package hat

import (
	"fmt"
	"math"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

var _ gameengine.Hat = (*MCTSHat)(nil)

// Deprecated: MCTSHat is superseded by YggdrasilHat, which integrates UCB1 search
// and rollout natively without the inner-hat delegation chain. Use NewYggdrasilHat instead.
//
// MCTSHat wraps an inner policy hat (typically PokerHat) and overrides
// high-impact decisions with evaluator-guided search. Low-impact
// decisions delegate directly to the inner hat.
//
// Phase 2 is "shallow MCTS": for each candidate action in a decision,
// score the current game state with the evaluator and use UCB1 to
// balance exploration vs exploitation across repeated calls. Full
// rollout simulation requires CloneGameState (Phase 5).
//
// The skill dial (Budget) controls search depth:
//   - 0:    pure inner-hat fallthrough (GreedyHat/PokerHat behavior)
//   - 1-50: evaluator-guided best-of-candidates (this phase)
//   - 200+: reserved for MCTS tree search (Phase 5)
type MCTSHat struct {
	Inner     gameengine.Hat
	Evaluator *GameStateEvaluator
	Budget    int

	// TurnRunner advances a GameState by one full turn (including SBAs).
	// Injected by the tournament package to break the hat→tournament
	// circular dependency. Required for Budget >= 200 rollouts; nil
	// degrades gracefully to evaluator-only mode.
	TurnRunner TurnRunnerFunc

	// DecisionLog accumulates human-readable decision traces when non-nil.
	// Set by diagnostic harnesses; nil in normal tournament play.
	DecisionLog *[]string

	// actionStats tracks cumulative UCB1 statistics per action key.
	// Keys are decision-specific (e.g. card name for cast decisions).
	// Reset at the start of each game via ObserveEvent("game_start").
	actionStats map[string]*actionStat
	totalVisits int
}

type actionStat struct {
	visits int
	value  float64
}

// NewMCTSHat constructs an MCTSHat with the given inner policy,
// strategy profile, and search budget.
func NewMCTSHat(inner gameengine.Hat, sp *StrategyProfile, budget int) *MCTSHat {
	return &MCTSHat{
		Inner:       inner,
		Evaluator:   NewEvaluator(sp),
		Budget:      budget,
		actionStats: map[string]*actionStat{},
	}
}

// ucb1Score returns the UCB1 selection score for an action.
// Balances exploitation (average value) with exploration (unvisited
// actions get bonus). C = sqrt(2) is the standard exploration constant.
func (h *MCTSHat) ucb1Score(key string, baseValue float64) float64 {
	stat := h.actionStats[key]
	if stat == nil || stat.visits == 0 {
		return baseValue + 2.0 // exploration bonus for unvisited
	}
	avgValue := stat.value / float64(stat.visits)
	exploration := math.Sqrt(2.0) * math.Sqrt(math.Log(float64(h.totalVisits+1))/float64(stat.visits))
	return avgValue + exploration
}

func (h *MCTSHat) logDecision(msg string) {
	if h.DecisionLog != nil {
		*h.DecisionLog = append(*h.DecisionLog, msg)
	}
}

func (h *MCTSHat) recordAction(key string, value float64) {
	if h.actionStats[key] == nil {
		h.actionStats[key] = &actionStat{}
	}
	h.actionStats[key].visits++
	h.actionStats[key].value += value
	h.totalVisits++
}

// -----------------------------------------------------------------------
// High-impact decisions — evaluator-guided
// -----------------------------------------------------------------------

func (h *MCTSHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	if h.Budget == 0 || len(castable) <= 1 {
		return h.Inner.ChooseCastFromHand(gs, seatIdx, castable)
	}

	// High-budget path: simulate each candidate via rollout.
	if h.canRollout() {
		return h.chooseCastViaRollout(gs, seatIdx, castable)
	}

	baseScore := h.Evaluator.Evaluate(gs, seatIdx)
	detail := h.Evaluator.EvaluateDetailed(gs, seatIdx)

	// Turn-scoped keys prevent pass UCB inflation across turns.
	turnPrefix := fmt.Sprintf("t%d:", gs.Turn)

	var bestCard *gameengine.Card
	bestUCB := math.Inf(-1)

	type scored struct {
		name string
		val  float64
		ucb  float64
	}
	var candidates []scored

	for _, c := range castable {
		if c == nil {
			continue
		}
		// All keys are turn-scoped so multiplayer games don't accumulate
		// stale visit counts between a player's widely-spaced turns.
		key := turnPrefix + "cast:" + c.DisplayName()

		cardValue := h.evaluateCandidate(gs, seatIdx, c, baseScore)
		ucb := h.ucb1Score(key, cardValue)
		candidates = append(candidates, scored{c.DisplayName(), cardValue, ucb})

		if ucb > bestUCB {
			bestUCB = ucb
			bestCard = c
		}
	}

	passKey := turnPrefix + "cast:__pass__"
	passUCB := h.ucb1Score(passKey, baseScore)
	if passUCB > bestUCB {
		bestCard = nil
	}

	if h.DecisionLog != nil {
		h.logDecision(fmt.Sprintf("[T%d] CAST eval seat=%d pos=%.3f (board=%.2f cards=%.2f mana=%.2f life=%.2f combo=%.2f threat=%.2f cmdr=%.2f yard=%.2f)",
			gs.Turn, seatIdx, baseScore,
			detail.BoardPresence, detail.CardAdvantage, detail.ManaAdvantage,
			detail.LifeResource, detail.ComboProximity, detail.ThreatExposure,
			detail.CommanderProgress, detail.GraveyardValue))
		for _, sc := range candidates {
			h.logDecision(fmt.Sprintf("  candidate: %-30s heuristic=%.3f ucb=%.3f", sc.name, sc.val, sc.ucb))
		}
		h.logDecision(fmt.Sprintf("  pass: ucb=%.3f", passUCB))
		if bestCard != nil {
			h.logDecision(fmt.Sprintf("  → CAST %s (ucb=%.3f)", bestCard.DisplayName(), bestUCB))
		} else {
			h.logDecision(fmt.Sprintf("  → PASS (ucb=%.3f)", passUCB))
		}
	}

	if bestCard != nil {
		key := turnPrefix + "cast:" + bestCard.DisplayName()
		h.recordAction(key, h.Evaluator.Evaluate(gs, seatIdx))
	} else {
		h.recordAction(passKey, baseScore)
	}

	return bestCard
}

func (h *MCTSHat) chooseCastViaRollout(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	baseScore := h.Evaluator.Evaluate(gs, seatIdx)
	detail := h.Evaluator.EvaluateDetailed(gs, seatIdx)

	h.logDecision(fmt.Sprintf("[T%d] ROLLOUT-CAST seat=%d pos=%.3f (board=%.2f cards=%.2f mana=%.2f life=%.2f combo=%.2f threat=%.2f cmdr=%.2f yard=%.2f)",
		gs.Turn, seatIdx, baseScore,
		detail.BoardPresence, detail.CardAdvantage, detail.ManaAdvantage,
		detail.LifeResource, detail.ComboProximity, detail.ThreatExposure,
		detail.CommanderProgress, detail.GraveyardValue))

	turnPrefix := fmt.Sprintf("t%d:", gs.Turn)
	passKey := turnPrefix + "cast:__pass__"

	// Simulate passing (casting nothing).
	passScore := h.simulateRollout(gs, seatIdx, func(clone *gameengine.GameState) {})
	passUCB := h.ucb1Score(passKey, passScore)

	var bestCard *gameengine.Card
	bestUCB := passUCB

	for _, c := range castable {
		if c == nil {
			continue
		}
		cardName := c.DisplayName()
		key := turnPrefix + "cast:" + cardName

		rolloutScore := h.simulateRollout(gs, seatIdx, func(clone *gameengine.GameState) {
			seat := clone.Seats[seatIdx]
			if seat == nil {
				return
			}
			for i, hc := range seat.Hand {
				if hc != nil && hc.DisplayName() == cardName {
					seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
					clone.Stack = append(clone.Stack, &gameengine.StackItem{
						ID:         len(clone.Stack) + 1,
						Controller: seatIdx,
						Card:       hc,
						Kind:       "spell",
					})
					break
				}
			}
		})

		heuristic := h.evaluateCandidate(gs, seatIdx, c, baseScore)
		// Blend: 50/50 rollout+heuristic. Rollout now resolves spells
		// but still doesn't model full targeting/ETB — heuristic stays
		// important for combo/value-engine awareness.
		blended := 0.5*rolloutScore + 0.5*heuristic
		ucb := h.ucb1Score(key, blended)

		h.logDecision(fmt.Sprintf("  candidate: %-30s rollout=%.3f heuristic=%.3f blended=%.3f ucb=%.3f",
			cardName, rolloutScore, heuristic, blended, ucb))

		if ucb > bestUCB {
			bestUCB = ucb
			bestCard = c
		}
	}

	h.logDecision(fmt.Sprintf("  pass: rollout=%.3f ucb=%.3f", passScore, passUCB))

	if bestCard != nil {
		h.logDecision(fmt.Sprintf("  → CAST %s (ucb=%.3f, beat pass by %.3f)", bestCard.DisplayName(), bestUCB, bestUCB-passUCB))
		h.recordAction(turnPrefix+"cast:"+bestCard.DisplayName(), bestUCB)
	} else {
		h.logDecision(fmt.Sprintf("  → PASS (ucb=%.3f)", passUCB))
		h.recordAction(passKey, passUCB)
	}

	return bestCard
}

// evaluateCandidate scores a card in the context of the current game
// state. Uses heuristic bonuses on top of the base evaluator score
// since we can't simulate the cast without CloneGameState.
func (h *MCTSHat) evaluateCandidate(gs *gameengine.GameState, seatIdx int, c *gameengine.Card, baseScore float64) float64 {
	bonus := 0.0

	// Combo piece bonus: if this card is part of an active combo, strongly prefer it.
	if h.Evaluator.Strategy != nil {
		for _, cp := range h.Evaluator.Strategy.ComboPieces {
			for _, piece := range cp.Pieces {
				if c.DisplayName() == piece {
					bonus += 0.5
					// Extra bonus if the other piece(s) are already available.
					available := countAvailablePieces(gs, seatIdx, cp)
					if available >= len(cp.Pieces)-1 {
						bonus += 1.0 // completing the combo
					}
				}
			}
		}
	}

	// Value engine key bonus.
	if h.Evaluator.Strategy != nil {
		for _, vek := range h.Evaluator.Strategy.ValueEngineKeys {
			if c.DisplayName() == vek {
				bonus += 0.3
				break
			}
		}
	}

	// Tempo bonus: developing the board is inherently valuable. Without
	// this, the evaluator penalizes casting (hand size drops by 1 = -0.25
	// CardAdvantage) but only gains ~0.1 BoardPresence for noncreatures.
	// The tempo bonus compensates: casting is almost always better than
	// holding when you have the mana.
	bonus += 0.15

	// Extra tempo for mana rocks: developing mana early compounds.
	if c.CMC <= 2 && isManaRock(c) {
		bonus += 0.2
	}

	// CMC efficiency: prefer casting on-curve.
	seat := gs.Seats[seatIdx]
	manaSources := countManaRocksAndLands(seat)
	if c.CMC > 0 && manaSources > 0 {
		efficiency := 1.0 - math.Abs(float64(c.CMC)-float64(manaSources))/float64(manaSources+1)
		bonus += efficiency * 0.2
	}

	return baseScore + bonus
}

func isManaRock(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if !typeLineContains(c, "artifact") {
		return false
	}
	ot := gameengine.OracleTextLower(c)
	return strings.Contains(ot, "mana") || strings.Contains(ot, "{t}: add")
}

func countAvailablePieces(gs *gameengine.GameState, seatIdx int, cp ComboPlan) int {
	seat := gs.Seats[seatIdx]
	available := make(map[string]bool)
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = true
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = true
		}
	}
	count := 0
	for _, piece := range cp.Pieces {
		if available[piece] {
			count++
		}
	}
	return count
}

func (h *MCTSHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	if h.Budget == 0 || len(legal) == 0 {
		return h.Inner.ChooseAttackers(gs, seatIdx, legal)
	}

	// Evaluate: attack all vs attack selectively vs don't attack.
	baseScore := h.Evaluator.Evaluate(gs, seatIdx)

	// Score each attacker individually by its threat contribution.
	type scored struct {
		perm  *gameengine.Permanent
		value float64
	}
	var candidates []scored
	for _, p := range legal {
		if p == nil {
			continue
		}
		power := float64(p.Power())
		// Bonus for evasion keywords.
		evasion := 0.0
		if p.HasKeyword("flying") || p.HasKeyword("unblockable") || p.HasKeyword("menace") || p.HasKeyword("trample") {
			evasion = 0.3
		}
		candidates = append(candidates, scored{perm: p, value: power/10.0 + evasion})
	}

	// Use evaluator: if we're ahead, attack more aggressively.
	threshold := 0.0
	if baseScore > 0.3 {
		threshold = -0.1 // attack almost everything when ahead
	} else if baseScore < -0.3 {
		threshold = 0.3 // be selective when behind
	}

	var attackers []*gameengine.Permanent
	for _, s := range candidates {
		if s.value >= threshold {
			attackers = append(attackers, s.perm)
		}
	}

	if h.DecisionLog != nil {
		stance := "neutral"
		if baseScore > 0.3 {
			stance = "AHEAD→aggressive"
		} else if baseScore < -0.3 {
			stance = "BEHIND→selective"
		}
		h.logDecision(fmt.Sprintf("[T%d] ATTACK seat=%d pos=%.3f stance=%s threshold=%.1f legal=%d",
			gs.Turn, seatIdx, baseScore, stance, threshold, len(legal)))
		for _, s := range candidates {
			selected := "skip"
			if s.value >= threshold {
				selected = "ATTACK"
			}
			h.logDecision(fmt.Sprintf("  %-30s pow=%d val=%.2f %s",
				s.perm.Card.DisplayName(), s.perm.Power(), s.value, selected))
		}
		h.logDecision(fmt.Sprintf("  → %d/%d creatures attacking", len(attackers), len(legal)))
	}

	if len(attackers) == 0 {
		return h.Inner.ChooseAttackers(gs, seatIdx, legal)
	}
	return attackers
}

func (h *MCTSHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	if h.Budget == 0 || len(options) <= 1 {
		return h.Inner.ChooseActivation(gs, seatIdx, options)
	}

	baseScore := h.Evaluator.Evaluate(gs, seatIdx)
	var bestOpt *gameengine.Activation
	bestUCB := math.Inf(-1)

	for i := range options {
		opt := &options[i]
		key := "activate:" + opt.Permanent.Card.DisplayName()
		ucb := h.ucb1Score(key, baseScore+0.1) // slight preference to activate
		if ucb > bestUCB {
			bestUCB = ucb
			bestOpt = opt
		}
	}

	passUCB := h.ucb1Score("activate:__pass__", baseScore)
	if passUCB > bestUCB {
		return nil
	}

	if bestOpt != nil {
		h.recordAction("activate:"+bestOpt.Permanent.Card.DisplayName(), baseScore)
	}
	return bestOpt
}

func (h *MCTSHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, stackTop *gameengine.StackItem) *gameengine.StackItem {
	if h.Budget == 0 {
		return h.Inner.ChooseResponse(gs, seatIdx, stackTop)
	}
	// Response decisions are time-critical and context-heavy.
	// PokerHat's mode-gated response logic is well-tuned — delegate.
	return h.Inner.ChooseResponse(gs, seatIdx, stackTop)
}

// -----------------------------------------------------------------------
// Low-impact decisions — delegate to inner hat
// -----------------------------------------------------------------------

func (h *MCTSHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	return h.Inner.ChooseMulligan(gs, seatIdx, hand)
}

func (h *MCTSHat) ChooseLandToPlay(gs *gameengine.GameState, seatIdx int, lands []*gameengine.Card) *gameengine.Card {
	return h.Inner.ChooseLandToPlay(gs, seatIdx, lands)
}

func (h *MCTSHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	return h.Inner.ChooseAttackTarget(gs, seatIdx, attacker, legalDefenders)
}

func (h *MCTSHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	return h.Inner.AssignBlockers(gs, seatIdx, attackers)
}

func (h *MCTSHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	return h.Inner.ChooseTarget(gs, seatIdx, filter, legal)
}

func (h *MCTSHat) ChooseMode(gs *gameengine.GameState, seatIdx int, modes []gameast.Effect) int {
	return h.Inner.ChooseMode(gs, seatIdx, modes)
}

func (h *MCTSHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	return h.Inner.ShouldCastCommander(gs, seatIdx, commanderName, tax)
}

func (h *MCTSHat) ShouldRedirectCommanderZone(gs *gameengine.GameState, seatIdx int, commander *gameengine.Card, to string) bool {
	return h.Inner.ShouldRedirectCommanderZone(gs, seatIdx, commander, to)
}

func (h *MCTSHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	return h.Inner.OrderReplacements(gs, seatIdx, candidates)
}

func (h *MCTSHat) ChooseDiscard(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, n int) []*gameengine.Card {
	return h.Inner.ChooseDiscard(gs, seatIdx, hand, n)
}

func (h *MCTSHat) OrderTriggers(gs *gameengine.GameState, seatIdx int, triggers []*gameengine.StackItem) []*gameengine.StackItem {
	return h.Inner.OrderTriggers(gs, seatIdx, triggers)
}

func (h *MCTSHat) ChooseX(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, availableMana int) int {
	return h.Inner.ChooseX(gs, seatIdx, card, availableMana)
}

func (h *MCTSHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	return h.Inner.ChooseBottomCards(gs, seatIdx, hand, count)
}

func (h *MCTSHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) ([]*gameengine.Card, []*gameengine.Card) {
	return h.Inner.ChooseScry(gs, seatIdx, cards)
}

func (h *MCTSHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) ([]*gameengine.Card, []*gameengine.Card) {
	return h.Inner.ChooseSurveil(gs, seatIdx, cards)
}

func (h *MCTSHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	return h.Inner.ChoosePutBack(gs, seatIdx, hand, count)
}

func (h *MCTSHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool { return false }

func (h *MCTSHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {
	// Reset action stats at game start.
	if event.Kind == "game_start" {
		h.actionStats = map[string]*actionStat{}
		h.totalVisits = 0
	}
	// Forward to inner hat for its own tracking (PokerHat mode transitions etc).
	h.Inner.ObserveEvent(gs, seatIdx, event)
}
