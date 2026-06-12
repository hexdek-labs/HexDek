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

	// PruneThreshold (R60 round 5) cuts candidate branches whose cheap
	// heuristic falls below baseScore + PruneThreshold BEFORE the
	// expensive rollout / UCB1 evaluation. Must be <= 0 (a negative
	// delta from the current position score). Default -0.5 means
	// "skip candidates that look worse than passing by half a unit or
	// more" — clearly losing branches no longer consume rollout budget.
	// Set to 0 to disable. Pass is never pruned (always a safe option).
	// If pruning would empty the candidate pool, the highest-heuristic
	// candidate is retained as a safety floor.
	PruneThreshold float64

	// PrunedCount tracks how many candidates the threshold filtered out
	// across this hat's lifetime. Exposed for diagnostics and tests.
	PrunedCount int

	// actionStats tracks cumulative UCB1 statistics per action key.
	// Keys are decision-specific (e.g. card name for cast decisions).
	// Reset at the start of each game via ObserveEvent("game_start").
	actionStats map[string]*actionStat
	totalVisits int

	// rolloutSeed is bumped per rollout invocation to give each candidate
	// a distinct RNG stream within a decision. PER-HAT (not a package
	// global) so that hats running in parallel — or multiple hats in
	// the same process — don't share seed state. Reset on game_start
	// so each game starts from a known seed sequence (reproducible
	// replays + test stability).
	rolloutSeed int64
}

// DefaultPruneThreshold is the default cut for MCTSHat candidate
// pruning. -0.5 means "a candidate whose heuristic is worse than the
// current state by half a unit or more is skipped." Calibrated so
// `evaluateCandidate`'s baseline tempo bonus (+0.15) is still well
// above the cut — only candidates the heuristic actively dislikes get
// pruned.
const DefaultPruneThreshold = -0.5

type actionStat struct {
	visits int
	value  float64
}

// NewMCTSHat constructs an MCTSHat with the given inner policy,
// strategy profile, and search budget.
func NewMCTSHat(inner gameengine.Hat, sp *StrategyProfile, budget int) *MCTSHat {
	return &MCTSHat{
		Inner:          inner,
		Evaluator:      NewEvaluator(sp),
		Budget:         budget,
		PruneThreshold: DefaultPruneThreshold,
		actionStats:    map[string]*actionStat{},
	}
}

// shouldPrune returns true if the candidate's heuristic value falls
// below baseScore + PruneThreshold. Pruning is disabled when
// PruneThreshold == 0 (the zero-value sentinel — safe for callers
// who construct MCTSHat directly without the constructor). Any
// non-zero value enables pruning: negative thresholds gate only on
// obvious losers, positive thresholds prune aggressively.
func (h *MCTSHat) shouldPrune(heuristicVal, baseScore float64) bool {
	if h.PruneThreshold == 0 {
		return false
	}
	return heuristicVal < baseScore+h.PruneThreshold
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

	type pruned struct {
		card *gameengine.Card
		name string
		val  float64
	}
	var safetyFloor pruned
	safetyFloor.val = math.Inf(-1)

	for _, c := range castable {
		if c == nil {
			continue
		}
		cardValue := h.evaluateCandidate(gs, seatIdx, c, baseScore)

		// Prune obvious losing branches before they consume UCB1 budget.
		// Track the best of the pruned in case ALL candidates fall under
		// the threshold — we still need a fallback better than passing
		// when passing is itself a slow loss.
		if h.shouldPrune(cardValue, baseScore) {
			h.PrunedCount++
			if cardValue > safetyFloor.val {
				safetyFloor = pruned{c, c.DisplayName(), cardValue}
			}
			h.logDecision(fmt.Sprintf("  pruned:    %-30s heuristic=%.3f (cut at %.3f)",
				c.DisplayName(), cardValue, baseScore+h.PruneThreshold))
			continue
		}

		// All keys are turn-scoped so multiplayer games don't accumulate
		// stale visit counts between a player's widely-spaced turns.
		key := turnPrefix + "cast:" + c.DisplayName()

		ucb := h.ucb1Score(key, cardValue)
		candidates = append(candidates, scored{c.DisplayName(), cardValue, ucb})

		if ucb > bestUCB {
			bestUCB = ucb
			bestCard = c
		}
	}

	// Safety floor: if pruning rejected every candidate, the highest-
	// heuristic pruned candidate re-enters as the only option so the
	// hat doesn't blindly pass on a forced-action turn.
	if bestCard == nil && len(candidates) == 0 && safetyFloor.card != nil {
		key := turnPrefix + "cast:" + safetyFloor.name
		ucb := h.ucb1Score(key, safetyFloor.val)
		candidates = append(candidates, scored{safetyFloor.name, safetyFloor.val, ucb})
		bestCard = safetyFloor.card
		bestUCB = ucb
		h.logDecision(fmt.Sprintf("  safety:    %-30s heuristic=%.3f (all candidates pruned, restored best)",
			safetyFloor.name, safetyFloor.val))
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

	type prunedRollout struct {
		card *gameengine.Card
		val  float64
	}
	var rolloutFloor prunedRollout
	rolloutFloor.val = math.Inf(-1)
	var evaluated []*gameengine.Card

	for _, c := range castable {
		if c == nil {
			continue
		}
		cardName := c.DisplayName()
		key := turnPrefix + "cast:" + cardName

		// Cheap heuristic gate BEFORE the expensive rollout. A rollout
		// costs a full turn simulation; the heuristic costs one
		// evaluator pass. Pruning here is the highest-leverage win.
		heuristicPre := h.evaluateCandidate(gs, seatIdx, c, baseScore)
		if h.shouldPrune(heuristicPre, baseScore) {
			h.PrunedCount++
			if heuristicPre > rolloutFloor.val {
				rolloutFloor = prunedRollout{c, heuristicPre}
			}
			h.logDecision(fmt.Sprintf("  pruned:    %-30s heuristic=%.3f (cut at %.3f, rollout skipped)",
				cardName, heuristicPre, baseScore+h.PruneThreshold))
			continue
		}
		evaluated = append(evaluated, c)

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

		// Blend: 50/50 rollout+heuristic. Rollout now resolves spells
		// but still doesn't model full targeting/ETB — heuristic stays
		// important for combo/value-engine awareness. heuristicPre was
		// already computed for the prune gate; reuse it.
		heuristic := heuristicPre
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

	// Safety floor: if pruning rejected every non-nil candidate and pass
	// is the only "option," restore the best-pruned card so the hat
	// can still take an action if the game state demands it (e.g. a
	// land-only hand on a turn the inner hat must play something).
	if bestCard == nil && len(evaluated) == 0 && rolloutFloor.card != nil {
		bestCard = rolloutFloor.card
		key := turnPrefix + "cast:" + bestCard.DisplayName()
		bestUCB = h.ucb1Score(key, rolloutFloor.val)
		h.logDecision(fmt.Sprintf("  safety:    %-30s heuristic=%.3f (all rollout candidates pruned, restored best)",
			bestCard.DisplayName(), rolloutFloor.val))
	}

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
	manaSources := CountManaRocksAndLands(seat)
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

func (h *MCTSHat) ChooseKickCount(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, kickerCost, maxKicks int) int {
	return h.Inner.ChooseKickCount(gs, seatIdx, card, kickerCost, maxKicks)
}

func (h *MCTSHat) ChooseOptionalCost(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, kind string, cost, max int) int {
	return h.Inner.ChooseOptionalCost(gs, seatIdx, card, kind, cost, max)
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
	// Reset action stats + rollout seed at game start so each game is
	// reproducible from its own starting state regardless of prior
	// games in the same process (test order, tournament gauntlet).
	if event.Kind == "game_start" {
		h.actionStats = map[string]*actionStat{}
		h.totalVisits = 0
		h.rolloutSeed = 0
	}
	// Forward to inner hat for its own tracking (PokerHat mode transitions etc).
	h.Inner.ObserveEvent(gs, seatIdx, event)
}
