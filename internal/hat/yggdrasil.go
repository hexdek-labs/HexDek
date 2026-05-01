package hat

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// YggdrasilHat — the unified player-decision engine.
//
// One brain, tunable personality. Every decision flows through the same
// evaluation pipeline. Budget controls depth, archetype tunes weights,
// politics handles multiplayer dynamics.
//
// Replaces the Greedy→Poker→MCTS delegation chain with a single
// implementation that has native multi-seat awareness.

var _ gameengine.Hat = (*YggdrasilHat)(nil)

// YggdrasilHat implements gameengine.Hat.
type YggdrasilHat struct {
	Evaluator  *GameStateEvaluator
	Strategy   *StrategyProfile
	Budget     int     // 0=heuristic, 1-199=evaluator-guided, 200+=rollout
	Noise      float64 // gaussian σ applied to targeting scores (0=deterministic, 0.2=default)
	TurnRunner TurnRunnerFunc

	DecisionLog *[]string

	noiseRNG *rand.Rand

	// UCB1 tracking (turn-scoped keys).
	actionStats  map[string]*actionStat
	totalVisits  int

	// Per-opponent observation for politics.
	damageDealtTo     []int
	damageReceivedFrom []int
	spellsCastBy      []int
	perceivedArchetype []string

	seatCount int

	// Eval cache — keyed on (turn, seatIdx). Cleared when the turn
	// changes. Board state only changes on resolution, not stack push,
	// so this stays valid across an entire priority round.
	evalCache     map[evalCacheKey]float64
	evalCacheTurn int

	// -- 3rd Eye: Omniscient Intelligence System --

	// cardsSeen tracks every card name observed per opponent seat.
	// Key: seat index. Populated from cast, dies, exile, zone_change events.
	cardsSeen []map[string]int

	// threatTrajectory tracks per-opponent board power snapshots over time.
	// Used to detect momentum (growing vs stable vs collapsing boards).
	threatTrajectory [][]int

	// politicalGraph tracks damage dealt between ALL seat pairs (not just us).
	// politicalGraph[attacker][defender] = cumulative damage.
	politicalGraph [][]int

	// lastTurnBoardPower caches each seat's board power from the previous
	// turn for trajectory delta computation.
	lastTurnBoardPower []int

	// opponentColors tracks which mana colors each opponent has produced,
	// for estimating interaction probability (blue/black = instant-speed danger).
	opponentColors []map[string]bool

	// kingmakerTurn records the first turn each seat's eval exceeds the
	// "about to win" threshold. 0 = not yet detected.
	kingmakerTurn []int

	// lastAttackedUsTurn records the last turn each opponent dealt damage
	// to us. Used for détente: opponents who leave us alone get left alone.
	lastAttackedUsTurn []int

	// Pre-computed lookup sets for O(1) card relevance checks.
	comboPieceSet    map[string]bool
	valueEngineSet   map[string]bool
	tutorTargetSet   map[string]bool
	finisherSet      map[string]bool
	starCardSet      map[string]bool
	cuttableSet      map[string]bool
	vulnerableToSet  map[string]bool
	isTempoComboVal  bool
	lookupsBuilt     bool

	// Threat cache — per-turn memoization of assessAllThreats.
	threatCache     []seatThreat
	threatCacheTurn int

	// Pooled map for comboUrgency to avoid per-call allocation.
	availablePool map[string]bool

	// Conviction concession — sliding window of recent position evals.
	convictionScores [convictionWindowSize]float64
	convictionCount  int
	convictionFull   bool

	// Diagnostic: track the lowest relative position ever seen.
	MinRelPos float64

	// Per-turn evaluation budget. TurnBudget > 0 enables the system:
	// each evaluator-path decision costs 1 point, each rollout costs
	// rolloutEvalCost points. When exhausted, remaining decisions in
	// the turn degrade to heuristic. 0 = legacy per-action mode.
	TurnBudget     int
	turnEvalsSpent int
	turnBudgetTurn int
}

const (
	convictionWindowSize = 4
	convictionThreshold  = -0.35 // relative to best opponent — consistently worst at the table
	convictionMinTurn    = 10

	// Adaptive budget: degrade to heuristic when board is too complex.
	// 60 permanents across 4 seats = ~15 each = a developed mid-game board.
	adaptiveBudgetComplexityThreshold = 60

	// Per-turn budget costs. A rollout is ~10x more expensive than a
	// single evaluator-path decision (clone + forward sim + eval).
	rolloutEvalCost = 10
)

type evalCacheKey struct {
	seat int
}

func NewYggdrasilHat(strategy *StrategyProfile, budget int) *YggdrasilHat {
	return NewYggdrasilHatWithNoise(strategy, budget, 0.2)
}

// BudgetForELO returns an adjusted budget based on ELO confidence.
// Decks with more games get higher budget (deeper search is worthwhile
// when strategy data is reliable).
func BudgetForELO(baseBudget int, gamesPlayed int) int {
	if gamesPlayed < 20 {
		return baseBudget
	}
	if gamesPlayed < 100 {
		return baseBudget + baseBudget/4
	}
	return baseBudget + baseBudget/2
}

// BudgetForPower adjusts budget based on Freya's power percentile.
// High-percentile decks have more complex lines worth searching.
func BudgetForPower(baseBudget int, powerPercentile int) int {
	if powerPercentile <= 0 {
		return baseBudget
	}
	if powerPercentile >= 80 {
		return baseBudget + baseBudget/3
	}
	if powerPercentile >= 60 {
		return baseBudget + baseBudget/6
	}
	return baseBudget
}

func (h *YggdrasilHat) EvalsSpent() int { return h.turnEvalsSpent }

func NewYggdrasilHatWithNoise(strategy *StrategyProfile, budget int, noise float64) *YggdrasilHat {
	h := &YggdrasilHat{
		Evaluator:     NewEvaluator(strategy),
		Strategy:      strategy,
		Budget:        budget,
		Noise:         noise,
		noiseRNG:      rand.New(rand.NewSource(rand.Int63())),
		actionStats:   make(map[string]*actionStat),
		availablePool: make(map[string]bool, 32),
	}
	h.buildLookupSets()
	return h
}

func (h *YggdrasilHat) buildLookupSets() {
	h.comboPieceSet = make(map[string]bool)
	h.valueEngineSet = make(map[string]bool)
	h.tutorTargetSet = make(map[string]bool)
	h.finisherSet = make(map[string]bool)
	h.starCardSet = make(map[string]bool)
	h.cuttableSet = make(map[string]bool)
	h.vulnerableToSet = make(map[string]bool)
	if h.Strategy != nil {
		for _, cp := range h.Strategy.ComboPieces {
			for _, piece := range cp.Pieces {
				h.comboPieceSet[piece] = true
			}
		}
		for _, vk := range h.Strategy.ValueEngineKeys {
			h.valueEngineSet[vk] = true
		}
		for _, tt := range h.Strategy.TutorTargets {
			h.tutorTargetSet[tt] = true
		}
		for _, fc := range h.Strategy.FinisherCards {
			h.finisherSet[fc] = true
		}
		for _, sc := range h.Strategy.StarCards {
			h.starCardSet[sc] = true
		}
		for _, cc := range h.Strategy.CuttableCards {
			h.cuttableSet[cc] = true
		}
		for _, v := range h.Strategy.VulnerableTo {
			h.vulnerableToSet[strings.ToLower(v)] = true
		}
	}
	h.lookupsBuilt = true
}

func (h *YggdrasilHat) isStarCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return h.starCardSet[c.DisplayName()]
}

func (h *YggdrasilHat) isCuttable(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return h.cuttableSet[c.DisplayName()]
}

func (h *YggdrasilHat) matchupRating(oppArchetype string) string {
	if h.Strategy == nil || h.Strategy.MetaMatchups == nil {
		return ""
	}
	return h.Strategy.MetaMatchups[oppArchetype]
}

// freyaRole returns the Freya-assigned role for a card, or "" if not available.
func (h *YggdrasilHat) freyaRole(c *gameengine.Card) string {
	if h.Strategy == nil || h.Strategy.CardRoles == nil || c == nil {
		return ""
	}
	return h.Strategy.CardRoles[c.DisplayName()]
}

// categorizeWithFreya uses Freya's role classification if available,
// falling back to the heuristic categorizeCard.
func (h *YggdrasilHat) categorizeWithFreya(c *gameengine.Card) CardCategory {
	role := h.freyaRole(c)
	switch role {
	case "Ramp":
		return CatRamp
	case "Draw":
		return CatDraw
	case "Removal", "BoardWipe":
		return CatRemoval
	case "Counterspell":
		return CatCounter
	case "Combo":
		return CatCombo
	case "Threat":
		return CatThreat
	case "Tutor":
		return CatUtility
	case "Protection", "Stax", "Utility", "Land":
		return CatUtility
	}
	return categorizeCard(c)
}

// isFinisher returns true if the card is a Freya-classified game finisher.
func (h *YggdrasilHat) isFinisher(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return h.finisherSet[c.DisplayName()]
}

// -- Politics: multi-seat threat assessment --

type seatThreat struct {
	Seat            int
	EvalScore       float64 // their position strength
	BoardPower      int
	Life            int
	HandSize        int
	ManaSources     int
	DamageToUs      int     // how much they've hurt us
	RetaliationRisk float64 // risk they'll focus us if we attack them
	Momentum        float64 // board power trend (positive = growing)
	InteractionProb float64 // probability of holding instant-speed answers
	IsKingmaker     bool    // dangerously close to winning
	PoliticalEnemy  int     // seat they're most likely to retaliate against (-1 = none)
	TurnsToKill     int     // estimated turns until this seat kills us (0 = unknown, 1 = imminent)
}

func (h *YggdrasilHat) assessAllThreats(gs *gameengine.GameState, seatIdx int) []seatThreat {
	if gs.Turn == h.threatCacheTurn && h.threatCache != nil {
		return h.threatCache
	}
	threats := make([]seatThreat, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		st := seatThreat{
			Seat:       i,
			BoardPower: boardPower(s),
			Life:       s.Life,
			HandSize:   len(s.Hand),
			ManaSources: countManaRocksAndLands(s),
		}
		st.EvalScore = h.Evaluator.Evaluate(gs, i)
		if i < len(h.damageReceivedFrom) {
			st.DamageToUs = h.damageReceivedFrom[i]
		}

		// Retaliation risk: stronger opponents with more board presence
		// are more dangerous to provoke. Scale by their board power
		// relative to ours.
		myPow := boardPower(gs.Seats[seatIdx])
		if myPow > 0 {
			st.RetaliationRisk = float64(st.BoardPower) / float64(myPow)
		} else if st.BoardPower > 0 {
			st.RetaliationRisk = 2.0
		}
		// Grudge factor: opponents we've already hit are more likely
		// to retaliate. Decay-weighted: recent damage (from politicalGraph)
		// matters more than ancient grudges.
		if i < len(h.damageDealtTo) {
			dealt := h.damageDealtTo[i]
			if dealt > 0 && s.Life > 0 {
				st.RetaliationRisk += float64(dealt) / float64(s.Life) * 0.5
			}
		}

		// 3rd Eye enrichment.
		st.Momentum = h.threatMomentum(i)
		st.InteractionProb = h.opponentHasInteraction(gs, i)
		st.IsKingmaker = h.isKingmaker(gs, i)
		st.PoliticalEnemy = h.tablePoliticalEnemy(i)

		// Threat timeline: estimate turns until this opponent kills us.
		myLife := gs.Seats[seatIdx].Life
		if st.BoardPower > 0 && myLife > 0 {
			st.TurnsToKill = myLife / st.BoardPower
			if st.TurnsToKill < 1 {
				st.TurnsToKill = 1
			}
			// Momentum adjustment: growing boards kill faster.
			if st.Momentum > 1.0 && st.TurnsToKill > 1 {
				st.TurnsToKill--
			}
		}

		threats = append(threats, st)
	}
	h.threatCache = threats
	h.threatCacheTurn = gs.Turn
	return threats
}

// applyNoise adds gaussian noise (Box-Muller) scaled by h.Noise to a score.
// Returns the score unchanged when Noise <= 0.
func (h *YggdrasilHat) applyNoise(score float64) float64 {
	if h.Noise <= 0 || h.noiseRNG == nil {
		return score
	}
	u1 := h.noiseRNG.Float64()
	u2 := h.noiseRNG.Float64()
	if u1 < 1e-15 {
		u1 = 1e-15
	}
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return score + z*h.Noise
}

// bestTarget picks the optimal attack target considering threat level,
// retaliation risk, and whether we can finish someone off.
func (h *YggdrasilHat) bestTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) == 0 {
		return -1
	}
	if len(legalDefenders) == 1 {
		return legalDefenders[0]
	}

	threats := h.assessAllThreats(gs, seatIdx)
	myScore := h.Evaluator.Evaluate(gs, seatIdx)
	relPos := h.relativePosition(gs, seatIdx)
	focusFire := relPos < -0.3

	// Spite / Dying Wish: when we're about to die (low life, worst position),
	// stop optimizing to win and instead target the strongest opponent.
	// This kingmakes the underdog — a real human would do the same.
	myLife := 0
	if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
		myLife = gs.Seats[seatIdx].Life
	}
	if myLife > 0 && myLife <= 5 && relPos < -0.4 && len(threats) > 1 {
		bestEval := -2.0
		spiteTarget := legalDefenders[0]
		for _, th := range threats {
			isLegal := false
			for _, d := range legalDefenders {
				if d == th.Seat {
					isLegal = true
					break
				}
			}
			if isLegal && th.EvalScore > bestEval {
				bestEval = th.EvalScore
				spiteTarget = th.Seat
			}
		}
		return spiteTarget
	}

	type candidate struct {
		seat  int
		score float64
	}
	candidates := make([]candidate, 0, len(legalDefenders))

	for _, def := range legalDefenders {
		var threat *seatThreat
		for i := range threats {
			if threats[i].Seat == def {
				threat = &threats[i]
				break
			}
		}
		if threat == nil {
			candidates = append(candidates, candidate{def, 0})
			continue
		}

		score := 0.0

		// 1. Kill-shot detection: always prioritize lethal attacks.
		if attacker != nil && attacker.Power() >= threat.Life && threat.Life > 0 {
			score += 8.0
		}

		// 2. Scaled low-life bonus — linear ramp as life drops below 20.
		// At 20 life: +0. At 10 life: +1.5. At 1 life: +3.0.
		if threat.Life < 20 {
			score += 3.0 * (1.0 - float64(threat.Life)/20.0)
		}

		// 3. Target the leader (highest eval score).
		leaderWeight := 2.0
		if focusFire {
			leaderWeight = 3.5
		}
		score += threat.EvalScore * leaderWeight

		// 4. Prefer open defenders (fewer untapped blockers).
		if attacker != nil && isOpenForAttacker(attacker, gs.Seats[def]) {
			score += 1.5
		}

		// 5. Retaliation risk penalty — skip when behind (focus fire).
		if !focusFire && myScore < 0.2 && threat.RetaliationRisk > 1.0 {
			score -= threat.RetaliationRisk * 0.8
		}

		// 6. Grudge factor — if they've been hitting us, hit back.
		if threat.DamageToUs > 0 {
			score += float64(threat.DamageToUs) / 40.0
		}

		// 6b. Détente: opponents who haven't attacked us in 4+ turns get a
		// targeting discount. Mutual non-aggression emerges organically.
		if gs != nil && def < len(h.lastAttackedUsTurn) && !threat.IsKingmaker {
			lastHit := h.lastAttackedUsTurn[def]
			peaceTurns := gs.Turn - lastHit
			if lastHit == 0 {
				peaceTurns = gs.Turn
			}
			if peaceTurns >= 4 {
				discount := float64(peaceTurns-3) * 0.15
				if discount > 1.0 {
					discount = 1.0
				}
				score -= discount
			}
		}

		// 7. Spread damage penalty — skip when behind (focus fire).
		if !focusFire && myScore < -0.1 {
			if seatIdx < len(h.damageDealtTo) && h.damageDealtTo[def] > 20 {
				score -= 0.5
			}
		}

		// 8. 3rd Eye: Kingmaker priority — always pressure the runaway leader.
		if threat.IsKingmaker {
			score += 3.0
		}

		// 9. 3rd Eye: Momentum bonus — target opponents whose boards are
		// growing fastest (they'll be harder to stop later).
		if threat.Momentum > 2.0 {
			score += threat.Momentum * 0.3
		}

		// 10. 3rd Eye: Political exploitation — if this opponent's primary
		// enemy is someone else, they're less likely to retaliate against us.
		if threat.PoliticalEnemy >= 0 && threat.PoliticalEnemy != seatIdx {
			score += 0.5
		}

		// 11. 3rd Eye: Interaction avoidance — when attacking into someone
		// likely holding tricks, apply a small penalty (not large enough to
		// override kill-shots or kingmaker priority).
		if threat.InteractionProb > 0.4 && !focusFire {
			score -= threat.InteractionProb * 0.5
		}

		// 13. Meta matchup: prioritize opponents we're unfavored against —
		// eliminate bad matchups early before they stabilize.
		if h.Strategy != nil && h.Strategy.MetaMatchups != nil {
			oppArch := h.inferOpponentArchetype(gs, def)
			if rating := h.matchupRating(oppArch); rating != "" {
				switch rating {
				case "unfavored":
					score += 1.0
				case "favored":
					score -= 0.5
				}
			}
		}

		// 12. 3rd Eye: Threat timeline urgency — opponents who can kill
		// us in 1-2 turns get deprioritized as attack targets (we need
		// to block them) UNLESS we can kill them first.
		if threat.TurnsToKill > 0 && threat.TurnsToKill <= 2 && attacker != nil {
			if attacker.Power() < threat.Life {
				score -= 1.0
			}
		}

		candidates = append(candidates, candidate{def, h.applyNoise(score)})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].seat
}

// -- Evaluation helpers --

func (h *YggdrasilHat) evalPosition(gs *gameengine.GameState, seatIdx int) float64 {
	if h.evalCache == nil || gs.Turn != h.evalCacheTurn {
		h.evalCache = make(map[evalCacheKey]float64, len(gs.Seats))
		h.evalCacheTurn = gs.Turn
	}
	key := evalCacheKey{seat: seatIdx}
	if v, ok := h.evalCache[key]; ok {
		return v
	}
	v := h.Evaluator.Evaluate(gs, seatIdx)
	h.evalCache[key] = v
	return v
}

func (h *YggdrasilHat) evalDetailed(gs *gameengine.GameState, seatIdx int) EvalResult {
	return h.Evaluator.EvaluateDetailed(gs, seatIdx)
}

// effectiveBudget returns the budget to use for this decision, degrading
// to heuristic on complex boards or when the per-turn budget is exhausted.
func (h *YggdrasilHat) effectiveBudget(gs *gameengine.GameState) int {
	if h.Budget == 0 {
		return 0
	}
	total := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		total += len(s.Battlefield)
	}
	if total >= adaptiveBudgetComplexityThreshold {
		return 0
	}
	if h.TurnBudget > 0 && h.turnRemaining(gs) <= 0 {
		return 0
	}
	return h.Budget
}

// turnRemaining returns how many eval points are left this turn.
// Returns TurnBudget (full) on the first call of a new turn.
// Returns a large number when TurnBudget is disabled (0).
func (h *YggdrasilHat) turnRemaining(gs *gameengine.GameState) int {
	if h.TurnBudget <= 0 {
		return 1<<30 - 1
	}
	if gs.Turn != h.turnBudgetTurn {
		h.turnEvalsSpent = 0
		h.turnBudgetTurn = gs.Turn
	}
	rem := h.TurnBudget - h.turnEvalsSpent
	if rem < 0 {
		return 0
	}
	return rem
}

// spendTurnBudget deducts cost from the per-turn evaluation budget.
func (h *YggdrasilHat) spendTurnBudget(gs *gameengine.GameState, cost int) {
	if h.TurnBudget <= 0 {
		return
	}
	if gs.Turn != h.turnBudgetTurn {
		h.turnEvalsSpent = 0
		h.turnBudgetTurn = gs.Turn
	}
	h.turnEvalsSpent += cost
}

// relativePosition returns how our score compares to the strongest opponent.
// Positive = we're ahead, negative = we're behind.
func (h *YggdrasilHat) relativePosition(gs *gameengine.GameState, seatIdx int) float64 {
	myScore := h.evalPosition(gs, seatIdx)
	bestOpp := -1.0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		oppScore := h.evalPosition(gs, i)
		if oppScore > bestOpp {
			bestOpp = oppScore
		}
	}
	return myScore - bestOpp
}

// cardHeuristic scores a castable card for the evaluator path.
func (h *YggdrasilHat) cardHeuristic(gs *gameengine.GameState, seatIdx int, c *gameengine.Card) float64 {
	base := 0.35
	cmc := gameengine.ManaCostOf(c)
	avail := gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])

	// Mana efficiency: spending more of available mana is better.
	if avail > 0 {
		base += float64(cmc) / float64(avail) * 0.15
	}

	cat := h.categorizeWithFreya(c)

	// Archetype-specific bonuses.
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}

	switch arch {
	case ArchetypeAggro:
		if cat == CatThreat || (typeLineContains(c, "creature") && cmc <= 3) {
			base += 0.15
		}
	case ArchetypeControl, ArchetypeStax:
		if cat == CatDraw || cat == CatRemoval {
			base += 0.15
		}
	case ArchetypeRamp:
		if cat == CatRamp {
			base += 0.20
		}
	case ArchetypeReanimator:
		if cat == CatRamp || cat == CatDraw {
			base += 0.10
		}
	case ArchetypeSpellslinger:
		if cat == CatDraw {
			base += 0.20
		}
	case ArchetypeTribal:
		if typeLineContains(c, "creature") {
			base += 0.15
		}
	case ArchetypeAristocrats:
		ot := gameengine.OracleTextLower(c)
		if (strings.Contains(ot, "sacrifice") && !strings.Contains(ot, "when")) ||
			(strings.Contains(ot, "whenever") && strings.Contains(ot, "dies")) {
			base += 0.25
		}
		if typeLineContains(c, "creature") && cmc <= 2 {
			base += 0.10
		}
	case ArchetypeSelfmill:
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "mill") || strings.Contains(ot, "graveyard") {
			base += 0.20
		}
	}

	// Combo piece bonus — applies to ALL archetypes. Every deck has
	// win lines from Freya; combo pieces should always be prioritized.
	if h.isComboRelevant(c) {
		bonus, _ := h.comboUrgency(gs, seatIdx, c)
		if bonus > 0 {
			base += bonus
		} else {
			comboFlat := 0.25
			if arch == ArchetypeCombo {
				comboFlat = 0.35
			}
			base += comboFlat
		}
	}

	if h.valueEngineSet[c.DisplayName()] {
		vkBonus := 0.15
		if arch == ArchetypeStax {
			vkBonus = 0.25
		}
		base += vkBonus
	}

	if h.tutorTargetSet[c.DisplayName()] {
		base += 0.10
	}

	// Finisher awareness: finisher cards get a bonus, scaled by board
	// readiness. A mass pump spell is much better when we have creatures.
	if h.isFinisher(c) {
		finBonus := 0.15
		if gs != nil {
			seat := gs.Seats[seatIdx]
			creatureCount := 0
			for _, p := range seat.Battlefield {
				if p != nil && p.IsCreature() {
					creatureCount++
				}
			}
			if creatureCount >= 3 {
				finBonus = 0.35
			} else if creatureCount >= 1 {
				finBonus = 0.20
			}
		}
		base += finBonus
	}

	// Star card bonus — Freya's highest-impact cards get priority.
	if h.isStarCard(c) {
		base += 0.15
	}

	// Cuttable card penalty — low-impact filler deprioritized.
	if h.isCuttable(c) {
		base -= 0.10
	}

	// Interaction speed: decks with cheap interaction can afford to hold mana.
	// Expensive interaction decks should cast proactively instead.
	if h.Strategy != nil && h.Strategy.InteractionAvgCMC > 3.0 && cat == CatRemoval {
		base += 0.05
	}

	// 3rd Eye: Interaction-aware sequencing — if opponents likely have
	// counters/removal (blue mana open, cards in hand), downweight key
	// pieces slightly to encourage baiting with lesser spells first.
	// Only applies to high-value pieces where losing them hurts.
	if gs != nil {
		intRisk := h.tableInteractionRisk(gs, seatIdx)
		isHighValue := h.isComboRelevant(c) || h.isValueEngineKey(c) || h.isStarCard(c)
		if isHighValue && intRisk > 0.4 {
			base -= (intRisk - 0.3) * 0.25
		}
	}

	// Sandbagging: if casting a high-value piece would make us the tallest
	// blade of grass (highest eval at the table), delay to avoid becoming
	// archenemy — unless we can win this turn or it's late enough to commit.
	if gs != nil && gs.Turn < 30 {
		isHighValue := h.isComboRelevant(c) || h.isValueEngineKey(c)
		if isHighValue {
			myEval := h.evalPosition(gs, seatIdx)
			bestOpp := -1.0
			for i, s := range gs.Seats {
				if i == seatIdx || s == nil || s.Lost || s.LeftGame {
					continue
				}
				e := h.evalPosition(gs, i)
				if e > bestOpp {
					bestOpp = e
				}
			}
			if myEval > bestOpp+0.1 {
				penalty := (myEval - bestOpp) * 0.3
				if penalty > 0.25 {
					penalty = 0.25
				}
				base -= penalty
			}
		}
	}

	// Synergy amplification: doublers on board boost related strategies.
	if gs != nil {
		seat := gs.Seats[seatIdx]
		ot := gameengine.OracleTextLower(c)
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			pn := strings.ToLower(p.Card.DisplayName())
			switch {
			case pn == "doubling season" || pn == "parallel lives" || pn == "anointed procession":
				if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
					base += 0.25
				}
				if strings.Contains(ot, "counter") {
					base += 0.15
				}
			case pn == "panharmonicon" || pn == "yarok the desecrated":
				if typeLineContains(c, "creature") || typeLineContains(c, "artifact") {
					base += 0.15
				}
			case pn == "hardened scales" || pn == "branching evolution":
				if strings.Contains(ot, "+1/+1") || strings.Contains(ot, "counter") {
					base += 0.20
				}
			}
		}
	}

	// Reactive penalty — stax/control/combo should be reluctant to cast
	// cards that aren't part of their strategy (non-engine, non-combo,
	// non-removal filler). This makes pass competitive against weak casts.
	if h.Strategy != nil {
		isStrategic := h.isComboRelevant(c) || h.isValueEngineKey(c) || h.tutorTargetSet[c.DisplayName()]
		if !isStrategic && cat != CatRemoval && cat != CatDraw {
			switch arch {
			case ArchetypeStax:
				base -= 0.15
			case ArchetypeControl:
				base -= 0.10
			case ArchetypeCombo:
				base -= 0.10
			case ArchetypeTribal:
				if !typeLineContains(c, "creature") {
					base -= 0.10
				}
			}
		}
	}

	return base
}

func (h *YggdrasilHat) isComboRelevant(c *gameengine.Card) bool {
	return h.comboPieceSet[c.DisplayName()]
}

// comboUrgency checks how close the seat is to completing any combo.
// Returns the bonus for a specific card: +1.0 if it's the LAST piece
// needed, +0.5 if 1 of 2 missing, +0.0 otherwise. Also returns the
// best overall combo completeness ratio for pass/hold decisions.
//
// When all pieces are present, applies a readiness check: sacrifice-
// based combos need creatures to sacrifice, activated abilities need
// to not be summoning-sick. A "ready" combo gets an extra bonus.
func (h *YggdrasilHat) comboUrgency(gs *gameengine.GameState, seatIdx int, card *gameengine.Card) (cardBonus float64, bestRatio float64) {
	if len(h.comboPieceSet) == 0 || gs == nil {
		return 0, 0
	}
	seat := gs.Seats[seatIdx]
	for k := range h.availablePool {
		delete(h.availablePool, k)
	}
	available := h.availablePool
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = true
		}
	}
	// Track which pieces are on the battlefield (not just in hand).
	onBattlefield := map[string]bool{}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = true
			onBattlefield[p.Card.DisplayName()] = true
		}
	}

	cardName := ""
	if card != nil {
		cardName = card.DisplayName()
	}

	for _, cp := range h.Strategy.ComboPieces {
		if len(cp.Pieces) == 0 {
			continue
		}
		found := 0
		allOnField := true
		cardIsInCombo := false
		cardIsMissing := false
		for _, piece := range cp.Pieces {
			if available[piece] {
				found++
			}
			if !onBattlefield[piece] {
				allOnField = false
			}
			if piece == cardName {
				cardIsInCombo = true
				if !available[piece] {
					cardIsMissing = true
				}
			}
		}
		ratio := float64(found) / float64(len(cp.Pieces))
		if ratio > bestRatio {
			bestRatio = ratio
		}
		missing := len(cp.Pieces) - found
		if cardIsInCombo && cardIsMissing {
			if missing == 1 {
				cardBonus = 1.0
			} else if missing == 2 && cardBonus < 0.5 {
				cardBonus = 0.5
			}
		}
		// Combo readiness: all pieces present AND on battlefield.
		// Check if the combo can actually execute this turn.
		if found == len(cp.Pieces) && allOnField {
			ready := h.comboCanExecute(gs, seatIdx, cp.Pieces)
			if ready && cardBonus < 0.5 {
				cardBonus = 0.5
			}
			if ratio > bestRatio {
				bestRatio = ratio
			}
		}
	}
	return cardBonus, bestRatio
}

// comboCanExecute checks if a fully-assembled combo can actually fire.
// Verifies sacrifice fodder availability and that key permanents aren't
// summoning-sick when they need to activate.
func (h *YggdrasilHat) comboCanExecute(gs *gameengine.GameState, seatIdx int, pieces []string) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	needsSacFodder := false
	hasSacFodder := false
	for _, piece := range pieces {
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if strings.ToLower(p.Card.DisplayName()) != strings.ToLower(piece) {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "sacrifice a creature") || strings.Contains(ot, "sacrifice another") {
				needsSacFodder = true
			}
		}
	}
	if needsSacFodder {
		creatureCount := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				creatureCount++
			}
		}
		hasSacFodder = creatureCount > len(pieces)
	}
	if needsSacFodder && !hasSacFodder {
		return false
	}
	return true
}

func (h *YggdrasilHat) isValueEngineKey(c *gameengine.Card) bool {
	return h.valueEngineSet[c.DisplayName()]
}

// isTempoCombo returns true when the deck's combo pieces heavily overlap
// with value engine keys — meaning the pieces provide value on their own
// and should be cast aggressively rather than held for assembly.
func (h *YggdrasilHat) isTempoCombo() bool {
	if len(h.comboPieceSet) == 0 {
		return false
	}
	overlap := 0
	for p := range h.comboPieceSet {
		if h.valueEngineSet[p] {
			overlap++
		}
	}
	return float64(overlap)/float64(len(h.comboPieceSet)) >= 0.4
}

func isCommanderCard(gs *gameengine.GameState, seatIdx int, c *gameengine.Card) bool {
	if gs == nil || c == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	name := c.DisplayName()
	for _, cn := range seat.CommanderNames {
		if cn == name {
			return true
		}
	}
	return false
}

// -- UCB1 machinery (shared across all decision types) --

func (h *YggdrasilHat) ucb1(key string, baseValue float64) float64 {
	stat, ok := h.actionStats[key]
	if !ok || stat.visits == 0 {
		return baseValue + 2.0
	}
	avg := stat.value / float64(stat.visits)
	exploration := math.Sqrt(2.0) * math.Sqrt(math.Log(float64(h.totalVisits+1))/float64(stat.visits))
	return avg + exploration
}

func (h *YggdrasilHat) recordAction(key string, value float64) {
	stat, ok := h.actionStats[key]
	if !ok {
		stat = &actionStat{}
		h.actionStats[key] = stat
	}
	stat.visits++
	stat.value += value
	h.totalVisits++
}

func (h *YggdrasilHat) logf(format string, args ...interface{}) {
	if h.DecisionLog == nil {
		return
	}
	*h.DecisionLog = append(*h.DecisionLog, fmt.Sprintf(format, args...))
}

// turnPrefix returns a turn-scoped key prefix to prevent stale visit
// accumulation in multiplayer.
func turnPrefix(gs *gameengine.GameState) string {
	if gs == nil {
		return "t0:"
	}
	return fmt.Sprintf("t%d:", gs.Turn)
}

// roundTag formats the human-friendly round notation R{round}.{seat}.
// Seat is 1-indexed. Falls back to [T{turn}] if round tracking isn't set.
func roundTag(gs *gameengine.GameState, seatIdx int) string {
	if gs == nil {
		return "[R0.0]"
	}
	r := 0
	if gs.Flags != nil {
		r = gs.Flags["round"]
	}
	if r > 0 {
		return fmt.Sprintf("[R%d.%d]", r, seatIdx+1)
	}
	return fmt.Sprintf("[T%d]", gs.Turn)
}

// -- Interface: ChooseMulligan --

func (h *YggdrasilHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	landCount := 0
	comboCount := 0
	rampCount := 0
	cheapSpells := 0
	for _, c := range hand {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if t == "land" {
				landCount++
				break
			}
		}
		if h.isComboRelevant(c) {
			comboCount++
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc <= 2 {
			cheapSpells++
		}
		cat := h.categorizeWithFreya(c)
		if cat == CatRamp {
			rampCount++
		}
	}

	// Always mulligan 0-land hands.
	if landCount == 0 {
		return true
	}

	// Count value engine keys, star cards, and cuttable cards in hand.
	veCount := 0
	starCount := 0
	cuttableCount := 0
	for _, c := range hand {
		if c == nil {
			continue
		}
		if h.isValueEngineKey(c) {
			veCount++
		}
		if h.isStarCard(c) {
			starCount++
		}
		if h.isCuttable(c) {
			cuttableCount++
		}
	}

	// Color demand check: if Freya says we need heavy color commitment,
	// mulligan hands where lands don't provide our top 2 needed colors.
	if h.Strategy != nil && h.Strategy.ColorDemand != nil && len(hand) >= 7 && landCount >= 2 {
		handColors := make(map[string]bool)
		for _, c := range hand {
			if c == nil {
				continue
			}
			for _, t := range c.Types {
				if t == "land" {
					ot := gameengine.OracleTextLower(c)
					for _, col := range []struct{ name, sym string }{
						{"plains", "W"}, {"island", "U"}, {"swamp", "B"},
						{"mountain", "R"}, {"forest", "G"},
					} {
						tl := strings.ToLower(c.TypeLine)
						if strings.Contains(tl, col.name) || strings.Contains(ot, "add {"+strings.ToLower(col.sym)+"}") || strings.Contains(ot, "any color") {
							handColors[col.sym] = true
						}
					}
					break
				}
			}
		}
		// Find the top-demand color. If we have 0 sources for it, mulligan.
		topColor := ""
		topDemand := 0
		for col, demand := range h.Strategy.ColorDemand {
			if demand > topDemand {
				topDemand = demand
				topColor = col
			}
		}
		if topColor != "" && topDemand >= 5 && !handColors[topColor] {
			return true
		}
	}

	// Archetype-aware keepability on 7 cards.
	if len(hand) >= 7 {
		if landCount <= 1 {
			return true
		}
		if h.Strategy != nil {
			switch h.Strategy.Archetype {
			case ArchetypeAggro:
				if landCount >= 2 && cheapSpells >= 2 {
					return false
				}
				if landCount > 4 {
					return true
				}
			case ArchetypeCombo:
				if comboCount >= 1 && landCount >= 2 {
					return false
				}
			case ArchetypeRamp:
				if (rampCount >= 1 || landCount >= 3) && landCount >= 2 {
					return false
				}
			case ArchetypeControl, ArchetypeStax:
				if landCount >= 3 {
					return false
				}
			case ArchetypeReanimator:
				if landCount >= 2 {
					return false
				}
			case ArchetypeSpellslinger:
				if landCount >= 3 && cheapSpells >= 1 {
					return false
				}
			case ArchetypeTribal:
				creatureCount := 0
				for _, c := range hand {
					if c != nil && typeLineContains(c, "creature") {
						creatureCount++
					}
				}
				if landCount >= 2 && creatureCount >= 2 {
					return false
				}
				if landCount > 4 {
					return true
				}
			case ArchetypeAristocrats:
				if landCount >= 2 && cheapSpells >= 1 {
					return false
				}
			case ArchetypeSelfmill:
				if landCount >= 2 {
					return false
				}
			}
		}
		// Any archetype: a hand with 2+ lands and a VE key or star card is worth keeping.
		if (veCount >= 1 || starCount >= 1) && landCount >= 2 {
			return false
		}

		// Low keepable hand % from Freya Monte Carlo: be pickier with marginal hands.
		if h.Strategy != nil && h.Strategy.KeepableHandPct > 0 && h.Strategy.KeepableHandPct < 60 {
			if cuttableCount >= 3 && landCount <= 3 {
				return true
			}
		}
	}

	// On 6 or fewer: star cards make marginal hands keepable.
	if len(hand) <= 6 {
		if landCount == 0 {
			return true
		}
		if starCount >= 1 && landCount >= 1 {
			return false
		}
		return false
	}
	return false
}

// -- Interface: ChooseLandToPlay --

func (h *YggdrasilHat) ChooseLandToPlay(gs *gameengine.GameState, seatIdx int, lands []*gameengine.Card) *gameengine.Card {
	if len(lands) == 0 {
		return nil
	}
	if len(lands) == 1 {
		return lands[0]
	}

	type scored struct {
		card  *gameengine.Card
		score float64
	}
	candidates := make([]scored, 0, len(lands))
	for _, l := range lands {
		if l == nil {
			continue
		}
		sc := 0.0
		name := l.DisplayName()
		ot := gameengine.OracleTextLower(l)

		// Colored mana production.
		if l.AST != nil {
			for _, ab := range l.AST.Abilities {
				if a, ok := ab.(*gameast.Activated); ok && a.Effect != nil {
					if a.Effect.Kind() == "add_mana" {
						sc += 1.0
					}
				}
			}
		}

		// Enters-tapped penalty — untapped lands are better early game.
		if strings.Contains(ot, "enters tapped") || strings.Contains(ot, "enters the battlefield tapped") {
			if gs.Turn <= 4 {
				sc -= 2.0
			} else {
				sc -= 0.5
			}
		}

		// Utility land bonus.
		if strings.Contains(ot, "draw") || strings.Contains(ot, "scry") {
			sc += 0.5
		}
		if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
			sc += 0.5
		}

		// Strategy-aware: VE key lands are high priority.
		if h.isValueEngineKey(l) {
			sc += 2.0
		}

		// Color-fixing: boost lands that produce colors we need but lack.
		// Weak mana bases (C/D/F grade) get a larger color-fixing multiplier.
		if h.Strategy != nil && h.Strategy.ColorDemand != nil {
			fixMul := 1.5
			if h.Strategy.ManaBaseGrade == "D" || h.Strategy.ManaBaseGrade == "F" {
				fixMul = 2.5
			} else if h.Strategy.ManaBaseGrade == "C" {
				fixMul = 2.0
			}
			landColors := landProducesColors(l)
			for col, demand := range h.Strategy.ColorDemand {
				if demand < 3 {
					continue
				}
				if landColors[col] {
					have := fieldColorSources(gs.Seats[seatIdx], col)
					need := float64(demand) / 10.0
					deficit := need - float64(have)*0.3
					if deficit > 0 {
						sc += deficit * fixMul
					}
				}
			}
		}

		// Basic lands get a small baseline.
		if strings.Contains(strings.ToLower(name), "plains") || strings.Contains(strings.ToLower(name), "island") ||
			strings.Contains(strings.ToLower(name), "swamp") || strings.Contains(strings.ToLower(name), "mountain") ||
			strings.Contains(strings.ToLower(name), "forest") {
			sc += 0.5
		}

		candidates = append(candidates, scored{l, sc})
	}
	if len(candidates) == 0 {
		return lands[0]
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].card
}

// -- Interface: ChooseCastFromHand --

func (h *YggdrasilHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	pool := make([]*gameengine.Card, 0, len(castable))
	for _, c := range castable {
		if c == nil || gameengine.CardHasCounterSpell(c) {
			continue
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		return nil
	}

	if h.effectiveBudget(gs) == 0 {
		return h.castHeuristic(gs, seatIdx, pool)
	}
	h.spendTurnBudget(gs, 1)

	prefix := turnPrefix(gs)
	pos := h.evalPosition(gs, seatIdx)
	det := h.evalDetailed(gs, seatIdx)

	interactionRisk := h.tableInteractionRisk(gs, seatIdx)
	h.logf("%s CAST eval seat=%d pos=%.3f intRisk=%.2f (board=%.2f cards=%.2f mana=%.2f life=%.2f combo=%.2f threat=%.2f cmdr=%.2f yard=%.2f)",
		roundTag(gs, seatIdx), seatIdx, pos, interactionRisk,
		det.BoardPresence, det.CardAdvantage, det.ManaAdvantage,
		det.LifeResource, det.ComboProximity, det.ThreatExposure,
		det.CommanderProgress, det.GraveyardValue)

	passKey := prefix + "pass"
	passBoost := 0.0
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}
	_, comboRatio := h.comboUrgency(gs, seatIdx, nil)
	if comboRatio > 0 {
		comboMul := 0.2
		switch arch {
		case ArchetypeCombo:
			comboMul = 0.5
			// Tempo-combo: if most combo pieces are also value engine keys,
			// casting them IS the plan — reduce hold incentive.
			if h.Strategy != nil && h.isTempoCombo() {
				comboMul = 0.15
			}
		case ArchetypeControl, ArchetypeStax:
			comboMul = 0.4
		}
		passBoost = comboRatio * comboMul
	}
	seat := gs.Seats[seatIdx]
	hasCounter := false
	for _, c := range seat.Hand {
		if c != nil && gameengine.CardHasCounterSpell(c) {
			hasCounter = true
			break
		}
	}
	if hasCounter {
		passBoost += 0.25
	}
	// Mana bluffing: even without a counter, represent interaction by
	// leaving 2+ mana open if we're in blue/black. The threat of a
	// counterspell is as powerful as having one. Only bluff when we
	// have enough mana that passing doesn't waste our whole turn.
	if !hasCounter && seat != nil {
		myColors := make(map[string]bool)
		for _, p := range seat.Battlefield {
			if p != nil && p.Card != nil {
				for _, cl := range p.Card.Colors {
					myColors[cl] = true
				}
			}
		}
		avail := gameengine.AvailableManaEstimate(gs, seat)
		if (myColors["U"] || myColors["B"]) && avail >= 4 && gs.Turn >= 4 {
			passBoost += 0.15
		}
	}
	// Check if castable pool contains strategic cards — if so, casting
	// them is the plan, not holding mana open.
	poolHasVE := false
	poolHasCombo := false
	for _, c := range pool {
		if h.isValueEngineKey(c) {
			poolHasVE = true
		}
		if h.isComboRelevant(c) {
			poolHasCombo = true
		}
	}
	switch arch {
	case ArchetypeStax:
		passBoost += 0.45
		if poolHasVE {
			passBoost -= 0.20
		}
	case ArchetypeControl:
		passBoost += 0.30
		if poolHasVE {
			passBoost -= 0.10
		}
	case ArchetypeCombo:
		passBoost += 0.20
		if h.Strategy != nil && h.isTempoCombo() {
			passBoost -= 0.10
		}
		if poolHasCombo {
			passBoost -= 0.10
		}
	case ArchetypeTribal:
		passBoost += 0.05
	case ArchetypeSpellslinger:
		passBoost += 0.05
	case ArchetypeAristocrats:
		passBoost -= 0.10
		if poolHasCombo {
			passBoost -= 0.15
		}
	}
	// Game clock pressure: reduce pass incentive as the game drags past
	// the archetype's comfort zone. Aggro at turn 20 shouldn't be patient.
	if gs != nil {
		clockPressure := 0.0
		clockStart := 20
		if h.Strategy != nil {
			switch h.Strategy.Archetype {
			case ArchetypeAggro, ArchetypeTribal:
				clockStart = 12
			case ArchetypeCombo:
				clockStart = 15
			case ArchetypeControl, ArchetypeStax:
				clockStart = 35
			}
		}
		if gs.Turn > clockStart {
			clockPressure = float64(gs.Turn-clockStart) * 0.02
			if clockPressure > 0.3 {
				clockPressure = 0.3
			}
		}
		passBoost -= clockPressure
	}
	passUCB := h.ucb1(passKey, pos+passBoost)
	h.logf("  pass: ucb=%.3f (boost=%.2f)", passUCB, passBoost)

	type scored struct {
		card *gameengine.Card
		ucb  float64
		info string
	}
	candidates := make([]scored, 0, len(pool))

	eb := h.effectiveBudget(gs)
	canRollout := eb >= rolloutBudgetGe && h.TurnRunner != nil &&
		h.turnRemaining(gs) >= rolloutEvalCost

	for _, c := range pool {
		cardKey := prefix + "cast:" + c.DisplayName()
		heurVal := h.cardHeuristic(gs, seatIdx, c)

		if canRollout && h.turnRemaining(gs) >= rolloutEvalCost {
			h.spendTurnBudget(gs, rolloutEvalCost)
			rollVal := h.simulateRolloutForCard(gs, seatIdx, c)
			blended := rollVal*0.5 + heurVal*0.5
			ucb := h.ucb1(cardKey, blended)
			info := fmt.Sprintf("  candidate: %-35s rollout=%.3f heuristic=%.3f blended=%.3f ucb=%.3f",
				c.DisplayName(), rollVal, heurVal, blended, ucb)
			candidates = append(candidates, scored{c, ucb, info})
		} else {
			ucb := h.ucb1(cardKey, pos+heurVal)
			info := fmt.Sprintf("  candidate: %-35s heuristic=%.3f ucb=%.3f",
				c.DisplayName(), heurVal, ucb)
			candidates = append(candidates, scored{c, ucb, info})
		}
	}

	for _, s := range candidates {
		h.logf("%s", s.info)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].ucb > candidates[j].ucb
	})
	best := candidates[0]

	if best.ucb <= passUCB {
		h.logf("  → PASS (pass ucb=%.3f beats best=%.3f)", passUCB, best.ucb)
		h.recordAction(passKey, pos)
		return nil
	}

	bestKey := prefix + "cast:" + best.card.DisplayName()
	if canRollout {
		h.logf("  → CAST %s (ucb=%.3f, beat pass by %.3f)", best.card.DisplayName(), best.ucb, best.ucb-passUCB)
	} else {
		h.logf("  → CAST %s (ucb=%.3f)", best.card.DisplayName(), best.ucb)
	}
	h.recordAction(bestKey, pos+h.cardHeuristic(gs, seatIdx, best.card))
	return best.card
}

func (h *YggdrasilHat) castHeuristic(gs *gameengine.GameState, seatIdx int, pool []*gameengine.Card) *gameengine.Card {
	turn := 0
	if gs != nil {
		turn = gs.Turn
	}

	// Ultra-cheap ramp always first.
	var ultraRamp, rest []*gameengine.Card
	for _, c := range pool {
		if isUltraCheapRamp(c) {
			ultraRamp = append(ultraRamp, c)
		} else {
			rest = append(rest, c)
		}
	}
	if len(ultraRamp) > 0 {
		sort.SliceStable(ultraRamp, func(i, j int) bool {
			return gameengine.ManaCostOf(ultraRamp[i]) < gameengine.ManaCostOf(ultraRamp[j])
		})
		return ultraRamp[0]
	}
	pool = rest
	if len(pool) == 0 {
		return nil
	}

	// Strategy-aware: combo pieces and VE keys always take priority
	// over generic ramp/draw, even in budget=0 mode.
	if h.Strategy != nil {
		var strategic, nonStrategic []*gameengine.Card
		for _, c := range pool {
			if h.isComboRelevant(c) || h.isValueEngineKey(c) {
				strategic = append(strategic, c)
			} else {
				nonStrategic = append(nonStrategic, c)
			}
		}
		if len(strategic) > 0 {
			sort.SliceStable(strategic, func(i, j int) bool {
				si := h.cardHeuristic(gs, seatIdx, strategic[i])
				sj := h.cardHeuristic(gs, seatIdx, strategic[j])
				return si > sj
			})
			return strategic[0]
		}
		pool = nonStrategic
		if len(pool) == 0 {
			return nil
		}
	}

	// Early game: ramp > draw > threats.
	if turn <= 12 {
		var ramp, draw, other []*gameengine.Card
		for _, c := range pool {
			switch h.categorizeWithFreya(c) {
			case CatRamp:
				ramp = append(ramp, c)
			case CatDraw:
				draw = append(draw, c)
			default:
				other = append(other, c)
			}
		}
		if len(ramp) > 0 {
			sort.SliceStable(ramp, func(i, j int) bool {
				return gameengine.ManaCostOf(ramp[i]) < gameengine.ManaCostOf(ramp[j])
			})
			return ramp[0]
		}
		if len(draw) > 0 {
			sort.SliceStable(draw, func(i, j int) bool {
				return gameengine.ManaCostOf(draw[i]) < gameengine.ManaCostOf(draw[j])
			})
			return draw[0]
		}
		pool = other
	}

	if len(pool) == 0 {
		return nil
	}
	// Default: use cardHeuristic for archetype-aware scoring.
	sort.SliceStable(pool, func(i, j int) bool {
		return h.cardHeuristic(gs, seatIdx, pool[i]) > h.cardHeuristic(gs, seatIdx, pool[j])
	})
	return pool[0]
}

// simulateRolloutForCard runs a rollout simulation for casting a specific card.
func (h *YggdrasilHat) simulateRolloutForCard(gs *gameengine.GameState, seatIdx int, card *gameengine.Card) float64 {
	if h.TurnRunner == nil {
		return 0
	}
	return h.simulateRollout(gs, seatIdx, func(clone *gameengine.GameState) {
		if seatIdx < 0 || seatIdx >= len(clone.Seats) {
			return
		}
		seat := clone.Seats[seatIdx]
		for i, c := range seat.Hand {
			if c != nil && c.DisplayName() == card.DisplayName() {
				seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
				item := &gameengine.StackItem{
					Card:       c,
					Controller: seatIdx,
				}
				clone.Stack = append(clone.Stack, item)
				break
			}
		}
	})
}

// -- Interface: ChooseActivation --

func (h *YggdrasilHat) ChooseActivation(gs *gameengine.GameState, seatIdx int, options []gameengine.Activation) *gameengine.Activation {
	if len(options) == 0 {
		return nil
	}
	if h.effectiveBudget(gs) == 0 {
		return &options[0]
	}
	h.spendTurnBudget(gs, 1)

	prefix := turnPrefix(gs)
	pos := h.evalPosition(gs, seatIdx)

	passKey := prefix + "act_pass"
	passUCB := h.ucb1(passKey, pos)

	var best *gameengine.Activation
	bestUCB := passUCB

	for i := range options {
		opt := &options[i]
		name := "?"
		if opt.Permanent != nil && opt.Permanent.Card != nil {
			name = opt.Permanent.Card.DisplayName()
		}
		heurVal := h.activationHeuristic(gs, seatIdx, &options[i])
		key := prefix + fmt.Sprintf("act:%s:%d", name, opt.Ability)
		ucb := h.ucb1(key, pos+heurVal)
		if ucb > bestUCB {
			bestUCB = ucb
			best = opt
		}
	}

	if best != nil {
		name := "?"
		if best.Permanent != nil && best.Permanent.Card != nil {
			name = best.Permanent.Card.DisplayName()
		}
		heurVal := h.activationHeuristic(gs, seatIdx, best)
		key := prefix + fmt.Sprintf("act:%s:%d", name, best.Ability)
		h.recordAction(key, pos+heurVal)
	} else {
		h.recordAction(passKey, pos)
	}
	return best
}

func (h *YggdrasilHat) activationHeuristic(gs *gameengine.GameState, seatIdx int, opt *gameengine.Activation) float64 {
	base := 0.15
	if opt.Permanent == nil || opt.Permanent.Card == nil {
		return base
	}
	c := opt.Permanent.Card

	if h.isValueEngineKey(c) {
		base += 0.25
	}
	if h.isComboRelevant(c) {
		bonus, _ := h.comboUrgency(gs, seatIdx, c)
		if bonus > 0 {
			base += bonus * 0.5
		} else {
			base += 0.20
		}
	}

	ot := gameengine.OracleTextLower(c)
	if strings.Contains(ot, "draw") || strings.Contains(ot, "scry") {
		base += 0.10
	}
	if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
		base += 0.10
	}
	if strings.Contains(ot, "destroy") || strings.Contains(ot, "exile") {
		base += 0.15
	}
	if strings.Contains(ot, "add {") || strings.Contains(ot, "add one mana") {
		if gs.Turn <= 5 {
			base += 0.10
		}
	}

	// Sacrifice outlets are much more valuable when we have death-trigger
	// payoffs (aristocrats) or token fodder on board.
	if strings.Contains(ot, "sacrifice") {
		deathPayoffs := 0
		tokenCount := 0
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			pot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(pot, "whenever") && (strings.Contains(pot, "dies") || strings.Contains(pot, "leaves")) {
				deathPayoffs++
			}
			for _, t := range p.Card.Types {
				if t == "token" {
					tokenCount++
					break
				}
			}
		}
		if deathPayoffs > 0 {
			base += float64(deathPayoffs) * 0.15
		}
		if tokenCount > 0 {
			base += 0.10
		}
		// Mana-producing sacrifices (Ashnod's, Phyrexian Altar) are ramp.
		if strings.Contains(ot, "add") && (strings.Contains(ot, "mana") || strings.Contains(ot, "{")) {
			base += 0.15
		}
	}

	// Life-payment abilities are better when we can afford the life.
	// At 30+ life in Commander, paying 2-5 life is essentially free.
	if c.AST != nil && opt.Ability >= 0 && opt.Ability < len(c.AST.Abilities) {
		if act, ok := c.AST.Abilities[opt.Ability].(*gameast.Activated); ok && act.Cost.PayLife != nil && *act.Cost.PayLife > 0 {
			life := gs.Seats[seatIdx].Life
			cost := *act.Cost.PayLife
			lifeAfter := life - cost
			if lifeAfter > 20 {
				base += 0.20
			} else if lifeAfter > 10 {
				base += 0.10
			}
			if strings.Contains(ot, "draw") || strings.Contains(ot, "scry") || strings.Contains(ot, "search") {
				base += 0.15
			}
		}
	}

	if h.tutorTargetSet[c.DisplayName()] {
		base += 0.10
	}

	return base
}

// -- Interface: ChooseAttackers --

func (h *YggdrasilHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	if len(legal) == 0 {
		return nil
	}

	pos := h.evalPosition(gs, seatIdx)
	relPos := h.relativePosition(gs, seatIdx)

	// Stance determination from relative position, tuned by archetype.
	aheadThresh := 0.3
	behindThresh := -0.3
	aheadVal := -0.1
	behindVal := 0.3
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro:
			aheadThresh = 0.15
			behindThresh = -0.5
			aheadVal = -0.2
			behindVal = 0.15
		case ArchetypeControl:
			aheadThresh = 0.5
			behindThresh = -0.2
			aheadVal = 0.0
			behindVal = 0.4
		case ArchetypeCombo:
			aheadThresh = 0.3
			behindThresh = -0.3
			aheadVal = -0.15
			behindVal = 0.10
		case ArchetypeMidrange:
			aheadThresh = 0.25
			behindThresh = -0.35
			aheadVal = -0.1
			behindVal = 0.2
		case ArchetypeRamp:
			aheadThresh = 0.3
			behindThresh = -0.4
			aheadVal = -0.1
			behindVal = 0.2
		case ArchetypeStax:
			aheadThresh = 0.4
			behindThresh = -0.2
			aheadVal = 0.0
			behindVal = 0.15
		case ArchetypeReanimator:
			aheadThresh = 0.25
			behindThresh = -0.35
			aheadVal = -0.1
			behindVal = 0.2
		case ArchetypeSpellslinger:
			aheadThresh = 0.35
			behindThresh = -0.3
			aheadVal = 0.0
			behindVal = 0.3
		case ArchetypeTribal:
			aheadThresh = 0.15
			behindThresh = -0.4
			aheadVal = -0.2
			behindVal = 0.15
		default:
			// tempo, voltron, aristocrats, etc. — combat-oriented,
			// treat like aggro-midrange blend.
			if h.Strategy.Archetype != "" {
				aheadThresh = 0.2
				behindThresh = -0.4
				aheadVal = -0.15
				behindVal = 0.15
			}
		}
	}
	threshold := 0.0
	stance := "neutral"
	if relPos > aheadThresh {
		threshold = aheadVal
		stance = "AHEAD→aggressive"
	} else if relPos < behindThresh {
		threshold = behindVal
		stance = "BEHIND→selective"
	}

	// Game clock awareness: archetype-shaped urgency. Aggro gets desperate
	// early, control stays patient, combo panics without assembly.
	urgencyStart := 20
	urgencyWindow := 30
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro, ArchetypeTribal:
			urgencyStart = 12
			urgencyWindow = 15
		case ArchetypeCombo:
			urgencyStart = 15
			urgencyWindow = 20
		case ArchetypeControl, ArchetypeStax:
			urgencyStart = 30
			urgencyWindow = 40
		case ArchetypeRamp:
			urgencyStart = 25
			urgencyWindow = 30
		case ArchetypeReanimator:
			urgencyStart = 18
			urgencyWindow = 25
		case ArchetypeMidrange:
			urgencyStart = 20
			urgencyWindow = 30
		}
	}
	if gs.Turn > urgencyStart && threshold > 0 {
		progress := float64(gs.Turn-urgencyStart) / float64(urgencyWindow)
		if progress > 1.0 {
			progress = 1.0
		}
		threshold *= (1.0 - progress)
		if progress >= 1.0 {
			stance += "→ALL-IN"
		}
	}

	// 3rd Eye: Wrath detection — if any opponent likely has a board wipe,
	// raise the attack threshold for value creatures (don't over-commit).
	wrathSuspected := false
	for i := range gs.Seats {
		if i != seatIdx && h.opponentLikelyHasWrath(gs, i) {
			wrathSuspected = true
			break
		}
	}

	// Lethal detection — compute total possible damage and check if any
	// opponent can be killed this turn. If so, go all-in.
	totalEvasivePower := 0
	totalPower := 0
	for _, p := range legal {
		if p == nil {
			continue
		}
		pw := p.Power()
		if pw <= 0 {
			continue
		}
		mul := 1
		if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
			mul = 2
		}
		totalPower += pw * mul
		if p.HasKeyword("unblockable") || p.HasKeyword("shadow") || p.HasKeyword("flying") ||
			p.HasKeyword("fear") || p.HasKeyword("menace") || p.HasKeyword("horsemanship") {
			totalEvasivePower += pw * mul
		}
	}
	lethalTarget := -1
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		// Evasive power alone can kill (minimal blocks possible).
		if totalEvasivePower >= s.Life {
			lethalTarget = i
			break
		}
		// Total power overkills by 2x their blockers' toughness.
		blockerTough := 0
		for _, bp := range s.Battlefield {
			if bp != nil && bp.IsCreature() && !bp.Tapped {
				blockerTough += bp.Toughness() - bp.MarkedDamage
			}
		}
		if totalPower >= s.Life+blockerTough {
			lethalTarget = i
			break
		}
	}
	if lethalTarget >= 0 {
		h.logf("%s LETHAL DETECTED on seat %d — sending everything",
			roundTag(gs, seatIdx), lethalTarget)
		var all []*gameengine.Permanent
		for _, p := range legal {
			if p != nil && p.Power() > 0 {
				all = append(all, p)
			}
		}
		return all
	}

	h.logf("%s ATTACK seat=%d pos=%.3f stance=%s threshold=%.2f legal=%d wrath=%v",
		roundTag(gs, seatIdx), seatIdx, pos, stance, threshold, len(legal), wrathSuspected)

	var attackers []*gameengine.Permanent
	for _, p := range legal {
		if p == nil {
			continue
		}
		pw := p.Power()
		if pw <= 0 {
			continue
		}
		val := float64(pw) / 10.0
		if p.HasKeyword("deathtouch") {
			val += 0.3
		}
		if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
			val += 0.2
		}
		if p.HasKeyword("lifelink") {
			val += 0.1
		}
		// Evasion bonus — creatures that connect reliably are worth sending.
		evasive := false
		if p.HasKeyword("unblockable") || p.HasKeyword("shadow") || p.HasKeyword("horsemanship") {
			val += 0.25
			evasive = true
		} else if p.HasKeyword("flying") || p.HasKeyword("fear") || p.HasKeyword("intimidate") || p.HasKeyword("skulk") {
			val += 0.15
			evasive = true
		} else if p.HasKeyword("menace") {
			val += 0.10
			evasive = true
		}
		// Value engine bonus — commanders and strategy-critical creatures
		// that trigger on combat damage are more valuable attacking.
		if p.Card != nil && h.isValueEngineKey(p.Card) {
			val += 0.15
		}
		// Commander damage matters — always worth sending.
		if p.Card != nil && isCommanderCard(gs, seatIdx, p.Card) {
			val += 0.10
		}

		// 3rd Eye: When a wrath is suspected, hold back VE key creatures
		// to preserve board presence post-wipe. Only applies when we're
		// not desperate (ahead or neutral).
		if wrathSuspected && relPos > -0.2 && p.Card != nil && h.isValueEngineKey(p.Card) {
			val -= 0.15
		}

		tag := "ATTACK"
		if val < threshold {
			tag = "HOLD (below threshold)"
		}
		evStr := ""
		if evasive {
			evStr = " [evasive]"
		}
		if tag == "ATTACK" {
			attackers = append(attackers, p)
		}
		h.logf("  %-30s pow=%d val=%.2f %s%s", p.Card.DisplayName(), pw, val, tag, evStr)
	}

	// Overcommitment guard: if committing 3+ creatures and we're not in a
	// lethal swing, hold back the single best value creature as insurance
	// against a board wipe. Don't put all eggs in one basket.
	if len(attackers) >= 3 && lethalTarget < 0 && relPos > -0.3 {
		bestReserveIdx := -1
		bestReserveVal := -1.0
		for i, p := range attackers {
			if p.Card == nil {
				continue
			}
			rv := 0.0
			if h.isValueEngineKey(p.Card) {
				rv += 2.0
			}
			if h.isComboRelevant(p.Card) {
				rv += 1.5
			}
			if isCommanderCard(gs, seatIdx, p.Card) {
				rv += 1.0
			}
			if rv > bestReserveVal {
				bestReserveVal = rv
				bestReserveIdx = i
			}
		}
		if bestReserveIdx >= 0 && bestReserveVal >= 1.0 {
			h.logf("  RESERVE: holding back %s (value=%.1f) as insurance",
				attackers[bestReserveIdx].Card.DisplayName(), bestReserveVal)
			attackers = append(attackers[:bestReserveIdx], attackers[bestReserveIdx+1:]...)
		}
	}

	h.logf("  → %d/%d creatures attacking", len(attackers), len(legal))
	return attackers
}

// -- Interface: ChooseAttackTarget (THE politics method) --

func (h *YggdrasilHat) ChooseAttackTarget(gs *gameengine.GameState, seatIdx int, attacker *gameengine.Permanent, legalDefenders []int) int {
	if len(legalDefenders) == 0 {
		return -1
	}
	if len(legalDefenders) == 1 {
		return legalDefenders[0]
	}
	return h.bestTarget(gs, seatIdx, attacker, legalDefenders)
}

// -- Interface: AssignBlockers --

func (h *YggdrasilHat) AssignBlockers(gs *gameengine.GameState, seatIdx int, attackers []*gameengine.Permanent) map[*gameengine.Permanent][]*gameengine.Permanent {
	out := make(map[*gameengine.Permanent][]*gameengine.Permanent, len(attackers))
	for _, a := range attackers {
		out[a] = nil
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return out
	}
	seat := gs.Seats[seatIdx]

	// Calculate incoming damage.
	incoming := 0
	for _, a := range attackers {
		if a == nil {
			continue
		}
		mul := 1
		if a.HasKeyword("double strike") || a.HasKeyword("double_strike") {
			mul = 2
		}
		incoming += a.Power() * mul
	}

	relPos := h.relativePosition(gs, seatIdx)
	aheadNoBlock := 0.3
	survivalFrac := 2
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro, ArchetypeTribal:
			aheadNoBlock = 0.2
			survivalFrac = 3
		case ArchetypeControl, ArchetypeStax:
			aheadNoBlock = 0.5
			survivalFrac = 2
		case ArchetypeReanimator:
			aheadNoBlock = 0.1
			survivalFrac = 4
		case ArchetypeSpellslinger:
			aheadNoBlock = 0.4
			survivalFrac = 2
		case ArchetypeCombo:
			aheadNoBlock = 0.3
			survivalFrac = 2
		}
	}
	if relPos > aheadNoBlock && incoming < seat.Life/survivalFrac {
		return out
	}

	// Pool of legal blockers — exclude combo/value creatures from trades
	// unless we'll die without blocking.
	willDie := seat.Life-incoming <= 0
	pool := make([]*gameengine.Permanent, 0, len(seat.Battlefield))
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Tapped {
			continue
		}
		if !willDie && p.Card != nil && (h.isComboRelevant(p.Card) || h.isValueEngineKey(p.Card)) {
			continue
		}
		pool = append(pool, p)
	}

	// Rank attackers by threat.
	type rank struct {
		a     *gameengine.Permanent
		score int
	}
	ranks := make([]rank, 0, len(attackers))
	for _, a := range attackers {
		if a == nil {
			continue
		}
		ranks = append(ranks, rank{a, -attackerRank(a)})
	}
	sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].score < ranks[j].score })

	used := make(map[*gameengine.Permanent]bool, len(pool))
	life := seat.Life

	for _, r := range ranks {
		atk := r.a
		if atk == nil {
			continue
		}
		legal := make([]*gameengine.Permanent, 0, len(pool))
		for _, b := range pool {
			if !used[b] && gameengine.CanBlock(atk, b) {
				legal = append(legal, b)
			}
		}
		if len(legal) == 0 {
			continue
		}

		willDieIfUnblocked := life-incoming <= 0
		atkDT := atk.HasKeyword("deathtouch")

		// Find survivors (blockers that outlive the attacker).
		var survivors []*gameengine.Permanent
		if !atkDT {
			for _, b := range legal {
				if b.Toughness()-b.MarkedDamage > atk.Power() {
					survivors = append(survivors, b)
				}
			}
		}
		sort.SliceStable(survivors, func(i, j int) bool {
			si, sj := survivors[i], survivors[j]
			if si.Power()+si.Toughness() != sj.Power()+sj.Toughness() {
				return si.Power()+si.Toughness() < sj.Power()+sj.Toughness()
			}
			return si.Toughness() < sj.Toughness()
		})

		var chosen []*gameengine.Permanent
		if len(survivors) > 0 {
			chosen = []*gameengine.Permanent{survivors[0]}
		} else if willDieIfUnblocked {
			chosen = []*gameengine.Permanent{bestChumpBlocker(legal)}
		}

		// Menace: need a second blocker.
		if len(chosen) > 0 && atk.HasKeyword("menace") {
			extras := make([]*gameengine.Permanent, 0, len(legal))
			for _, b := range legal {
				if b != chosen[0] {
					extras = append(extras, b)
				}
			}
			if len(extras) == 0 {
				chosen = nil
			} else {
				sort.SliceStable(extras, func(i, j int) bool {
					return extras[i].Power()+extras[i].Toughness() < extras[j].Power()+extras[j].Toughness()
				})
				chosen = append(chosen, extras[0])
			}
		}

		if len(chosen) == 0 {
			continue
		}
		for _, b := range chosen {
			used[b] = true
		}
		out[atk] = chosen

		// Update incoming for trample accounting.
		atkDmg := atk.Power()
		if atk.HasKeyword("double strike") || atk.HasKeyword("double_strike") {
			atkDmg *= 2
		}
		if atk.HasKeyword("trample") {
			totalT := 0
			for _, b := range chosen {
				totalT += b.Toughness() - b.MarkedDamage
			}
			leak := atkDmg - totalT
			if leak < 0 {
				leak = 0
			}
			incoming -= (atkDmg - leak)
		} else {
			incoming -= atkDmg
		}
	}
	return out
}

// -- Interface: ChooseResponse --

func (h *YggdrasilHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if top.Controller == seatIdx || top.Countered {
		return nil
	}
	if gameengine.SplitSecondActive(gs) {
		return nil
	}
	if gameengine.OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil
	}

	// Fast-path: scan for an affordable counterspell BEFORE running the
	// evaluator. Most seats most of the time have zero counters — this
	// skips the expensive relativePosition call entirely.
	seat := gs.Seats[seatIdx]
	var bestCounter *gameengine.Card
	avail := gameengine.AvailableManaEstimate(gs, seat)
	for _, c := range seat.Hand {
		if c != nil && gameengine.CardHasCounterSpell(c) {
			if avail >= gameengine.ManaCostOf(c) {
				bestCounter = c
				break
			}
		}
	}
	if bestCounter == nil {
		return nil
	}

	score := stackItemScore(top)

	// Always counter combo pieces / "win the game" / mass removal.
	mustCounter := false
	if top.Card != nil {
		if h.isComboRelevant(top.Card) {
			mustCounter = true
		}
		ot := gameengine.OracleTextLower(top.Card)
		if strings.Contains(ot, "win the game") {
			mustCounter = true
		}
		if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") && score >= 1 {
			mustCounter = true
		}
		// 3rd Eye: Counter kingmaker's key plays more aggressively.
		if h.isKingmaker(gs, top.Controller) && score >= 2 {
			mustCounter = true
		}
		// Counter cards we're specifically vulnerable to (Freya threat assessment).
		if len(h.vulnerableToSet) > 0 {
			if h.vulnerableToSet[strings.ToLower(top.Card.DisplayName())] {
				mustCounter = true
			}
		}
		// 3rd Eye: Counter cards we've seen wreck the board before.
		cardName := top.Card.DisplayName()
		if top.Controller >= 0 && top.Controller < len(h.cardsSeen) {
			if h.cardsSeen[top.Controller][cardName] > 1 {
				score += 2
			}
		}
	}

	if !mustCounter {
		relPos := h.relativePosition(gs, seatIdx)

		minScore := 3
		if h.Strategy != nil {
			switch h.Strategy.Archetype {
			case ArchetypeControl, ArchetypeStax:
				minScore = 2
			case ArchetypeAggro, ArchetypeTribal:
				minScore = 4
			case ArchetypeCombo, ArchetypeSpellslinger:
				minScore = 3
			case ArchetypeMidrange, ArchetypeReanimator:
				minScore = 3
			default:
				minScore = 4
			}
		}
		if relPos > 0.3 {
			minScore += 2
		} else if relPos < -0.3 {
			minScore -= 1
			if minScore < 1 {
				minScore = 1
			}
		}

		// 3rd Eye: Political counter allocation — if this spell is from
		// the weakest opponent and targets the strongest, let it resolve
		// (it helps us). Save our counter for threats aimed at us or
		// that benefit the leader.
		caster := top.Controller
		if caster >= 0 && caster < len(gs.Seats) {
			threats := h.assessAllThreats(gs, seatIdx)
			casterIsWeakest := true
			casterIsKing := false
			casterImminentThreat := false
			for _, th := range threats {
				if th.Seat == caster {
					if th.IsKingmaker {
						casterIsKing = true
					}
					if th.TurnsToKill > 0 && th.TurnsToKill <= 2 {
						casterImminentThreat = true
					}
					continue
				}
				if th.EvalScore < 0 {
					casterIsWeakest = false
				}
			}
			if casterIsWeakest && !casterIsKing {
				minScore += 2
			}
			// Counter more aggressively from opponents about to kill us.
			if casterImminentThreat {
				minScore -= 2
				if minScore < 1 {
					minScore = 1
				}
			}
		}

		if score < minScore {
			return nil
		}
	}

	return &gameengine.StackItem{
		Card:       bestCounter,
		Controller: seatIdx,
	}
}

// -- Interface: ChooseTarget --

func (h *YggdrasilHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	if len(legal) == 0 {
		return gameengine.Target{}
	}
	if len(legal) == 1 {
		return legal[0]
	}

	// Strategy-aware tutor selection — context-dependent, not just first match.
	if h.Strategy != nil {
		type tutorCandidate struct {
			target gameengine.Target
			score  float64
		}
		var tutorCandidates []tutorCandidate
		for _, t := range legal {
			if t.Kind != gameengine.TargetKindCard || t.Card == nil {
				continue
			}
			if !h.tutorTargetSet[t.Card.DisplayName()] {
				continue
			}
			sc := h.tutorTargetScore(gs, seatIdx, t.Card)
			tutorCandidates = append(tutorCandidates, tutorCandidate{t, sc})
		}
		if len(tutorCandidates) > 0 {
			sort.SliceStable(tutorCandidates, func(i, j int) bool {
				return tutorCandidates[i].score > tutorCandidates[j].score
			})
			h.logf("%s TUTOR seat=%d → %s (score=%.2f)",
				roundTag(gs, seatIdx), seatIdx,
				tutorCandidates[0].target.Card.DisplayName(),
				tutorCandidates[0].score)
			return tutorCandidates[0].target
		}
	}

	// For permanent-targeting effects (removal), score each target.
	hasPermanentTargets := false
	for _, t := range legal {
		if t.Kind == gameengine.TargetKindPermanent && t.Permanent != nil {
			hasPermanentTargets = true
			break
		}
	}
	if hasPermanentTargets {
		type scoredTarget struct {
			target gameengine.Target
			score  float64
		}
		var candidates []scoredTarget
		for _, t := range legal {
			if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
				continue
			}
			p := t.Permanent
			if p.Controller == seatIdx {
				continue
			}
			sc := 1.0
			if p.Card != nil {
				pow := p.Power()
				if pow > 0 {
					sc += float64(pow) * 0.3
				}
				ot := gameengine.OracleTextLower(p.Card)
				if strings.Contains(ot, "draw") || strings.Contains(ot, "whenever") {
					sc += 2.0
				}
				if strings.Contains(ot, "each opponent") {
					sc += 1.5
				}
				if typeLineContains(p.Card, "planeswalker") {
					sc += 2.0
				}
				if typeLineContains(p.Card, "commander") {
					sc += 1.0
				}
				// Prioritize removing known combo pieces and finishers
				cardName := p.Card.DisplayName()
				if h.comboPieceSet[cardName] {
					sc += 3.0
				}
				if h.finisherSet[cardName] {
					sc += 2.0
				}
				if h.valueEngineSet[cardName] {
					sc += 1.5
				}
			}
			threats := h.assessAllThreats(gs, seatIdx)
			for _, th := range threats {
				if th.Seat == p.Controller {
					sc += th.EvalScore * 0.5
					if th.IsKingmaker {
						sc += 2.0
					}
					if th.Momentum > 2.0 {
						sc += th.Momentum * 0.2
					}
					break
				}
			}
			candidates = append(candidates, scoredTarget{t, h.applyNoise(sc)})
		}
		if len(candidates) > 0 {
			sort.SliceStable(candidates, func(i, j int) bool {
				return candidates[i].score > candidates[j].score
			})
			return candidates[0].target
		}
	}

	// For player-targeting effects, use threat assessment.
	if filter.Base == "player" || filter.Base == "opponent" || filter.Base == "any_target" {
		threats := h.assessAllThreats(gs, seatIdx)
		if len(threats) > 0 {
			type noisyThreat struct {
				seat  int
				score float64
			}
			nt := make([]noisyThreat, len(threats))
			for i, th := range threats {
				nt[i] = noisyThreat{th.Seat, h.applyNoise(th.EvalScore)}
			}
			sort.SliceStable(nt, func(i, j int) bool {
				return nt[i].score > nt[j].score
			})
			bestSeat := nt[0].seat
			for _, t := range legal {
				if t.Kind == gameengine.TargetKindSeat && t.Seat == bestSeat {
					return t
				}
			}
		}
	}

	// Default: first legal target.
	return legal[0]
}

// -- Interface: ChooseMode --

func (h *YggdrasilHat) ChooseMode(gs *gameengine.GameState, seatIdx int, modes []gameast.Effect) int {
	if len(modes) == 0 {
		return -1
	}
	if len(modes) == 1 {
		return 0
	}

	bestIdx := 0
	bestScore := -1.0
	pos := h.evalPosition(gs, seatIdx)

	for i, m := range modes {
		score := h.scoreModeEffect(gs, seatIdx, m, pos)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx
}

func (h *YggdrasilHat) scoreModeEffect(gs *gameengine.GameState, seatIdx int, eff gameast.Effect, pos float64) float64 {
	score := 0.0
	switch eff.Kind() {
	case "damage", "lose_life":
		score = 0.4
		if pos > 0.3 {
			score = 0.6
		}
	case "destroy", "exile":
		oppPerms := 0
		for i, s := range gs.Seats {
			if i != seatIdx && s != nil && !s.Lost {
				oppPerms += len(s.Battlefield)
			}
		}
		if oppPerms > 0 {
			score = 0.7
		}
	case "draw":
		score = 0.5
		if pos < -0.2 {
			score = 0.8
		}
	case "create_token":
		score = 0.5
		if h.Strategy != nil && (h.Strategy.Archetype == ArchetypeTribal || h.Strategy.Archetype == ArchetypeAggro) {
			score = 0.7
		}
	case "counter_mod":
		score = 0.4
	case "gain_life":
		seat := gs.Seats[seatIdx]
		if seat != nil && seat.Life < 15 {
			score = 0.6
		} else {
			score = 0.2
		}
	case "bounce":
		score = 0.5
	case "tutor":
		score = 0.7
	case "reanimate", "recurse":
		score = 0.6
		if h.Strategy != nil && h.Strategy.Archetype == ArchetypeReanimator {
			score = 0.9
		}
	case "add_mana":
		if gs.Turn <= 5 {
			score = 0.5
		} else {
			score = 0.2
		}
	case "sacrifice":
		score = 0.3
	case "buff", "grant_ability":
		score = 0.4
	case "discard":
		score = 0.3
		if h.Strategy != nil && h.Strategy.Archetype == ArchetypeStax {
			score = 0.6
		}
	case "mill":
		score = 0.2
	case "scry", "surveil":
		score = 0.4
	default:
		score = 0.3
	}
	return score
}

// -- Interface: ShouldCastCommander --

func (h *YggdrasilHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	avail := gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])
	if avail <= 0 {
		return false
	}

	maxTax := 6
	manaBuffer := 1
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro:
			maxTax = 8
			manaBuffer = 0
		case ArchetypeCombo:
			maxTax = 4
			manaBuffer = 2
		case ArchetypeControl:
			maxTax = 6
			manaBuffer = 1
		case ArchetypeRamp:
			maxTax = 8
			manaBuffer = 0
		case ArchetypeMidrange:
			maxTax = 6
			manaBuffer = 1
		case ArchetypeStax:
			maxTax = 6
			manaBuffer = 1
		case ArchetypeReanimator:
			maxTax = 6
			manaBuffer = 1
		case ArchetypeSpellslinger:
			maxTax = 5
			manaBuffer = 1
		case ArchetypeTribal:
			maxTax = 8
			manaBuffer = 0
		default:
			maxTax = 8
			manaBuffer = 0
		}
	}
	// Late-game: always recast if affordable — the commander is the deck.
	if gs.Turn > 15 {
		return true
	}
	// 3rd Eye: If high interaction risk and commander tax is already 2+,
	// wait until we have enough mana to also hold up protection, or until
	// the blue player taps out.
	if tax >= 2 {
		intRisk := h.tableInteractionRisk(gs, seatIdx)
		if intRisk > 0.5 && avail < tax*2+2 {
			return false
		}
	}
	return tax <= maxTax || avail >= tax*2+manaBuffer
}

// -- Interface: ShouldRedirectCommanderZone --

func (h *YggdrasilHat) ShouldRedirectCommanderZone(gs *gameengine.GameState, seatIdx int, commander *gameengine.Card, to string) bool {
	// Reanimator: if dying (going to graveyard), let it go — we can
	// reanimate it cheaper than paying commander tax.
	if h.Strategy != nil && h.Strategy.Archetype == ArchetypeReanimator && to == "graveyard" {
		if seatIdx >= 0 && seatIdx < len(gs.Seats) {
			seat := gs.Seats[seatIdx]
			if seat != nil {
				hasReanimate := false
				for _, c := range seat.Hand {
					if c != nil {
						ot := gameengine.OracleTextLower(c)
						if strings.Contains(ot, "return") && (strings.Contains(ot, "graveyard") || strings.Contains(ot, "battlefield")) {
							hasReanimate = true
							break
						}
					}
				}
				if hasReanimate {
					return false
				}
			}
		}
	}
	return true
}

// -- Interface: OrderReplacements --

func (h *YggdrasilHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	return candidates
}

// -- Interface: ChooseDiscard --

func (h *YggdrasilHat) ChooseDiscard(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, n int) []*gameengine.Card {
	if n <= 0 || len(hand) == 0 {
		return nil
	}
	if n >= len(hand) {
		return hand
	}
	type ranked struct {
		card  *gameengine.Card
		value float64
	}
	ranked_ := make([]ranked, 0, len(hand))
	sources := 0
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		sources = countManaRocksAndLands(gs.Seats[seatIdx])
	}
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}
	for _, c := range hand {
		if c == nil {
			continue
		}
		v := h.cardHeuristic(gs, seatIdx, c)
		if typeLineContains(c, "land") && sources >= 5 {
			v -= 0.5
		}
		if h.isComboRelevant(c) {
			v += 1.0
		}
		if h.isValueEngineKey(c) {
			v += 0.5
		}
		if h.isStarCard(c) {
			v += 0.75
		}
		if h.isCuttable(c) {
			v -= 0.5
		}
		// Reanimator: high-CMC creatures are BETTER in the graveyard.
		// Lower their keep-value so they get discarded first.
		if arch == ArchetypeReanimator && typeLineContains(c, "creature") {
			cmc := gameengine.ManaCostOf(c)
			if cmc >= 5 {
				v -= float64(cmc) * 0.15
			}
		}
		ranked_ = append(ranked_, ranked{c, v})
	}
	sort.SliceStable(ranked_, func(i, j int) bool {
		return ranked_[i].value < ranked_[j].value
	})
	out := make([]*gameengine.Card, 0, n)
	for i := 0; i < n && i < len(ranked_); i++ {
		out = append(out, ranked_[i].card)
	}
	return out
}

// -- Interface: OrderTriggers --

func (h *YggdrasilHat) OrderTriggers(gs *gameengine.GameState, seatIdx int, triggers []*gameengine.StackItem) []*gameengine.StackItem {
	if len(triggers) <= 1 {
		return triggers
	}
	// Stack resolves LIFO — last item resolves first. Put highest-priority
	// triggers at the END so they resolve first.
	sort.SliceStable(triggers, func(i, j int) bool {
		return h.triggerPriority(triggers[i]) < h.triggerPriority(triggers[j])
	})
	return triggers
}

func (h *YggdrasilHat) triggerPriority(item *gameengine.StackItem) float64 {
	if item == nil || item.Card == nil {
		return 0
	}
	pri := 0.0
	if h.isComboRelevant(item.Card) {
		pri += 3.0
	}
	if h.isValueEngineKey(item.Card) {
		pri += 2.0
	}
	ot := gameengine.OracleTextLower(item.Card)
	if strings.Contains(ot, "draw") {
		pri += 1.5
	}
	if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
		pri += 1.0
	}
	if strings.Contains(ot, "damage") || strings.Contains(ot, "lose life") {
		if h.Strategy != nil && (h.Strategy.Archetype == ArchetypeAggro || h.Strategy.Archetype == ArchetypeSpellslinger) {
			pri += 2.0
		} else {
			pri += 1.0
		}
	}
	return pri
}

// -- Interface: ChooseX --

func (h *YggdrasilHat) ChooseX(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, availableMana int) int {
	if availableMana <= 0 {
		return 0
	}
	// Control/stax: hold back 2 mana for potential interaction unless
	// this is a critical spell.
	if h.Strategy != nil {
		arch := h.Strategy.Archetype
		isCritical := h.isComboRelevant(card) || h.isValueEngineKey(card)
		if !isCritical && (arch == ArchetypeControl || arch == ArchetypeStax) {
			reserve := 2
			if availableMana > reserve {
				return availableMana - reserve
			}
			return 1
		}
	}
	return availableMana
}

// -- Interface: ChooseBottomCards --

func (h *YggdrasilHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	if count >= len(hand) {
		return hand
	}
	// Bottom the worst cards by heuristic.
	type ranked struct {
		card  *gameengine.Card
		value float64
	}
	ranked_ := make([]ranked, 0, len(hand))
	for _, c := range hand {
		if c == nil {
			continue
		}
		ranked_ = append(ranked_, ranked{c, h.cardHeuristic(gs, seatIdx, c)})
	}
	sort.SliceStable(ranked_, func(i, j int) bool {
		return ranked_[i].value < ranked_[j].value
	})
	out := make([]*gameengine.Card, 0, count)
	for i := 0; i < count && i < len(ranked_); i++ {
		out = append(out, ranked_[i].card)
	}
	return out
}

// -- Interface: ChooseScry --

func (h *YggdrasilHat) ChooseScry(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (top []*gameengine.Card, bottom []*gameengine.Card) {
	if len(cards) == 0 {
		return nil, nil
	}
	// Dynamic threshold: be more selective when ahead, less when behind.
	threshold := 0.35
	relPos := h.relativePosition(gs, seatIdx)
	if relPos > 0.3 {
		threshold = 0.45
	} else if relPos < -0.3 {
		threshold = 0.25
	}
	// Combo decks with high combo ratios want combo pieces on top.
	for _, c := range cards {
		if c == nil {
			bottom = append(bottom, c)
			continue
		}
		val := h.cardHeuristic(gs, seatIdx, c)
		if h.isComboRelevant(c) || h.isValueEngineKey(c) || h.isStarCard(c) {
			top = append(top, c)
		} else if h.isCuttable(c) {
			bottom = append(bottom, c)
		} else if val >= threshold {
			top = append(top, c)
		} else {
			bottom = append(bottom, c)
		}
	}
	if len(top) == 0 && len(cards) > 0 {
		top = append(top, cards[0])
		bottom = bottom[:len(bottom)-1]
	}
	return top, bottom
}

// -- Interface: ChooseSurveil --

func (h *YggdrasilHat) ChooseSurveil(gs *gameengine.GameState, seatIdx int, cards []*gameengine.Card) (graveyard []*gameengine.Card, top []*gameengine.Card) {
	if len(cards) == 0 {
		return nil, nil
	}
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}
	for _, c := range cards {
		if c == nil {
			graveyard = append(graveyard, c)
			continue
		}
		val := h.cardHeuristic(gs, seatIdx, c)

		// Reanimator wants fatties in the graveyard — send high-CMC
		// creatures to the yard where they can be reanimated.
		if arch == ArchetypeReanimator && typeLineContains(c, "creature") {
			cmc := gameengine.ManaCostOf(c)
			if cmc >= 5 {
				graveyard = append(graveyard, c)
				continue
			}
		}

		if h.isComboRelevant(c) || h.isValueEngineKey(c) || h.isStarCard(c) {
			top = append(top, c)
		} else if h.isCuttable(c) {
			graveyard = append(graveyard, c)
		} else if val >= 0.35 {
			top = append(top, c)
		} else {
			graveyard = append(graveyard, c)
		}
	}
	if len(top) == 0 && len(cards) > 0 {
		top = append(top, cards[0])
		graveyard = graveyard[:len(graveyard)-1]
	}
	return graveyard, top
}

// -- Interface: ChoosePutBack --

func (h *YggdrasilHat) ChoosePutBack(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	return h.ChooseBottomCards(gs, seatIdx, hand, count)
}

// -- Interface: ShouldConcede (conviction-based) --

func (h *YggdrasilHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return false
	}
	if gs.Turn < convictionMinTurn {
		return false
	}

	// Use relative position (my score - best opponent) not absolute.
	relScore := h.relativePosition(gs, seatIdx)
	if relScore < h.MinRelPos {
		h.MinRelPos = relScore
	}

	idx := h.convictionCount % convictionWindowSize
	h.convictionScores[idx] = relScore
	h.convictionCount++
	if h.convictionCount >= convictionWindowSize {
		h.convictionFull = true
	}

	if !h.convictionFull {
		return false
	}

	for _, s := range h.convictionScores {
		if s > convictionThreshold {
			return false
		}
	}
	return true
}

// -- Interface: ObserveEvent --

func (h *YggdrasilHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {
	if event == nil {
		return
	}

	// Initialize tracking arrays on first event.
	if h.seatCount == 0 && gs != nil {
		h.seatCount = len(gs.Seats)
		h.damageDealtTo = make([]int, h.seatCount)
		h.damageReceivedFrom = make([]int, h.seatCount)
		h.spellsCastBy = make([]int, h.seatCount)
		h.perceivedArchetype = make([]string, h.seatCount)
		h.cardsSeen = make([]map[string]int, h.seatCount)
		h.threatTrajectory = make([][]int, h.seatCount)
		h.politicalGraph = make([][]int, h.seatCount)
		h.lastTurnBoardPower = make([]int, h.seatCount)
		h.opponentColors = make([]map[string]bool, h.seatCount)
		h.kingmakerTurn = make([]int, h.seatCount)
		h.lastAttackedUsTurn = make([]int, h.seatCount)
		for i := 0; i < h.seatCount; i++ {
			h.cardsSeen[i] = make(map[string]int)
			h.politicalGraph[i] = make([]int, h.seatCount)
			h.opponentColors[i] = make(map[string]bool)
		}
	}

	// Reset on game start.
	if event.Kind == "game_start" {
		h.actionStats = make(map[string]*actionStat)
		h.totalVisits = 0
		for i := range h.damageDealtTo {
			h.damageDealtTo[i] = 0
			h.damageReceivedFrom[i] = 0
			h.spellsCastBy[i] = 0
			h.cardsSeen[i] = make(map[string]int)
			h.politicalGraph[i] = make([]int, h.seatCount)
			h.opponentColors[i] = make(map[string]bool)
			h.threatTrajectory[i] = nil
			h.lastTurnBoardPower[i] = 0
			h.kingmakerTurn[i] = 0
			h.lastAttackedUsTurn[i] = 0
		}
		return
	}

	// Track damage dealt/received — both personal AND global political graph.
	if event.Kind == "damage" && event.Amount > 0 {
		if event.Seat == seatIdx && event.Target >= 0 && event.Target < len(h.damageDealtTo) {
			h.damageDealtTo[event.Target] += event.Amount
		}
		if event.Target == seatIdx && event.Seat >= 0 && event.Seat < len(h.damageReceivedFrom) {
			h.damageReceivedFrom[event.Seat] += event.Amount
			if event.Seat < len(h.lastAttackedUsTurn) && gs != nil {
				h.lastAttackedUsTurn[event.Seat] = gs.Turn
			}
		}
		// Political graph: track ALL damage between ALL seats.
		if event.Seat >= 0 && event.Seat < h.seatCount &&
			event.Target >= 0 && event.Target < h.seatCount {
			h.politicalGraph[event.Seat][event.Target] += event.Amount
		}
	}

	// 3rd Eye: Track every card observed from any seat.
	if event.Source != "" && event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		switch event.Kind {
		case "cast", "dies", "exile", "sacrifice", "destroy", "zone_change":
			h.cardsSeen[event.Seat][event.Source]++
		}
	}

	// Track spells cast per seat + detect colors from mana spent.
	if event.Kind == "cast" && event.Seat >= 0 && event.Seat < h.seatCount {
		h.spellsCastBy[event.Seat]++
		// Infer color identity from cast events.
		if event.Seat != seatIdx && event.Source != "" {
			h.inferColorsFromCard(gs, event.Seat, event.Source)
		}
	}

	// Track mana production events for color inference.
	if event.Kind == "add_mana" && event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		if event.Details != nil {
			if colorStr, ok := event.Details["color"].(string); ok {
				h.opponentColors[event.Seat][colorStr] = true
			}
		}
	}

	// Per-turn threat trajectory snapshot.
	if event.Kind == "upkeep" && gs != nil {
		for i, s := range gs.Seats {
			if s == nil || s.Lost || s.LeftGame {
				continue
			}
			bp := boardPower(s)
			if i < len(h.threatTrajectory) {
				h.threatTrajectory[i] = append(h.threatTrajectory[i], bp)
			}
			// Kingmaker detection: if any opponent's eval spikes above 0.7.
			if i != seatIdx && i < len(h.kingmakerTurn) && h.kingmakerTurn[i] == 0 {
				eval := h.Evaluator.Evaluate(gs, i)
				if eval > 0.7 {
					h.kingmakerTurn[i] = gs.Turn
				}
			}
		}
	}
}

// inferColorsFromCard examines a card name on the battlefield or cast
// and records which mana colors that seat has access to.
func (h *YggdrasilHat) inferColorsFromCard(gs *gameengine.GameState, seat int, cardName string) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || p.Card.DisplayName() != cardName {
			continue
		}
		for _, c := range p.Card.Colors {
			h.opponentColors[seat][c] = true
		}
		return
	}
}

// -- 3rd Eye query methods --

// opponentHasInteraction estimates whether an opponent seat is likely
// holding instant-speed interaction based on: open mana, known colors
// (blue/black = counters/removal), and hand size.
func (h *YggdrasilHat) opponentHasInteraction(gs *gameengine.GameState, oppSeat int) float64 {
	if gs == nil || oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[oppSeat]
	if s == nil || s.Lost || s.LeftGame || len(s.Hand) == 0 {
		return 0
	}
	openMana := gameengine.AvailableManaEstimate(gs, s)
	if openMana == 0 {
		return 0
	}
	prob := 0.1
	if openMana >= 2 {
		prob += 0.15
	}
	if oppSeat < len(h.opponentColors) {
		if h.opponentColors[oppSeat]["U"] {
			prob += 0.30
		}
		if h.opponentColors[oppSeat]["B"] {
			prob += 0.15
		}
	}
	handFactor := float64(len(s.Hand)) / 7.0
	if handFactor > 1.0 {
		handFactor = 1.0
	}
	prob *= handFactor
	if prob > 0.9 {
		prob = 0.9
	}
	return prob
}

// tableInteractionRisk returns the maximum interaction probability across
// all opponents, used to decide whether to walk into countermagic.
func (h *YggdrasilHat) tableInteractionRisk(gs *gameengine.GameState, seatIdx int) float64 {
	maxRisk := 0.0
	for i := range gs.Seats {
		if i == seatIdx {
			continue
		}
		risk := h.opponentHasInteraction(gs, i)
		if risk > maxRisk {
			maxRisk = risk
		}
	}
	return maxRisk
}

// threatMomentum returns a delta for the opponent's board power trend.
// Positive = growing, negative = shrinking, zero = stable.
func (h *YggdrasilHat) threatMomentum(oppSeat int) float64 {
	if oppSeat < 0 || oppSeat >= len(h.threatTrajectory) {
		return 0
	}
	traj := h.threatTrajectory[oppSeat]
	if len(traj) < 3 {
		return 0
	}
	recent := traj[len(traj)-1]
	prev := traj[len(traj)-3]
	return float64(recent-prev) / 3.0
}

// isKingmaker returns true if seat has been flagged as dangerously ahead.
func (h *YggdrasilHat) isKingmaker(gs *gameengine.GameState, oppSeat int) bool {
	if oppSeat < 0 || oppSeat >= len(h.kingmakerTurn) {
		return false
	}
	return h.kingmakerTurn[oppSeat] > 0 && gs.Turn-h.kingmakerTurn[oppSeat] <= 5
}

// tablePoliticalEnemy returns the seat that has dealt the most damage to
// a given seat. Used to predict retaliation.
func (h *YggdrasilHat) tablePoliticalEnemy(seat int) int {
	if seat < 0 || seat >= h.seatCount {
		return -1
	}
	maxDmg := 0
	enemy := -1
	for i := 0; i < h.seatCount; i++ {
		if i == seat {
			continue
		}
		if i < len(h.politicalGraph) && seat < len(h.politicalGraph[i]) {
			if h.politicalGraph[i][seat] > maxDmg {
				maxDmg = h.politicalGraph[i][seat]
				enemy = i
			}
		}
	}
	return enemy
}

// cardsSeenFromOpponent returns the count of distinct cards observed from
// a specific opponent.
func (h *YggdrasilHat) cardsSeenFromOpponent(oppSeat int) int {
	if oppSeat < 0 || oppSeat >= len(h.cardsSeen) {
		return 0
	}
	return len(h.cardsSeen[oppSeat])
}

// opponentPlayedCard returns true if we've seen a specific card name
// from a given opponent.
func (h *YggdrasilHat) opponentPlayedCard(oppSeat int, cardName string) bool {
	if oppSeat < 0 || oppSeat >= len(h.cardsSeen) {
		return false
	}
	return h.cardsSeen[oppSeat][cardName] > 0
}

// tutorTargetScore evaluates which tutor target is best given the current
// game state. A superhuman tutor decision considers: combo proximity,
// survival urgency, board state needs, and opponent threats.
func (h *YggdrasilHat) tutorTargetScore(gs *gameengine.GameState, seatIdx int, card *gameengine.Card) float64 {
	if card == nil || gs == nil {
		return 0
	}
	score := 1.0
	name := card.DisplayName()
	seat := gs.Seats[seatIdx]
	relPos := h.relativePosition(gs, seatIdx)

	// 1. Combo completion priority — if this card completes a combo, it's
	// the #1 tutor target. The closer we are, the more urgent.
	bonus, bestRatio := h.comboUrgency(gs, seatIdx, card)
	if bonus >= 1.0 {
		score += 5.0 // This card COMPLETES a combo — always grab it.
	} else if bonus >= 0.5 {
		score += 3.0 // One piece away after this.
	} else if h.isComboRelevant(card) {
		score += 1.0 + bestRatio
	}

	// 2. Survival urgency — if life is low, prioritize removal/lifegain.
	if seat != nil && seat.Life <= 10 {
		ot := gameengine.OracleTextLower(card)
		if strings.Contains(ot, "destroy") || strings.Contains(ot, "exile") {
			score += 2.0
		}
		if strings.Contains(ot, "gain") && strings.Contains(ot, "life") {
			score += 1.5
		}
	}

	// 3. Behind → tutor for card advantage engines.
	if relPos < -0.2 {
		ot := gameengine.OracleTextLower(card)
		if strings.Contains(ot, "draw") || strings.Contains(ot, "whenever") {
			score += 1.5
		}
	}

	// 4. Ahead → tutor for protection or finishers.
	if relPos > 0.3 {
		ot := gameengine.OracleTextLower(card)
		if strings.Contains(ot, "hexproof") || strings.Contains(ot, "indestructible") || strings.Contains(ot, "counter") {
			score += 1.5
		}
		if h.isFinisher(card) {
			creatureCount := 0
			if seat != nil {
				for _, p := range seat.Battlefield {
					if p != nil && p.IsCreature() {
						creatureCount++
					}
				}
			}
			score += 2.0
			if creatureCount >= 3 {
				score += 1.5
			}
		}
	}

	// 4b. Even when not ahead, finishers are strong when board is developed.
	if relPos > -0.1 && h.isFinisher(card) {
		creatureCount := 0
		if seat != nil {
			for _, p := range seat.Battlefield {
				if p != nil && p.IsCreature() {
					creatureCount++
				}
			}
		}
		if creatureCount >= 4 {
			score += 2.0
		}
	}

	// 5. VE key bonus — engine pieces are always strong tutor targets.
	if h.isValueEngineKey(card) {
		score += 1.0
	}

	// 5b. Star card bonus — Freya's highest-impact cards.
	if h.isStarCard(card) {
		score += 1.5
	}

	// 5c. Cuttable card penalty — never tutor for filler.
	if h.isCuttable(card) {
		score -= 2.0
	}

	// 6. Archetype-specific tutor priorities.
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro, ArchetypeTribal:
			// Anthem effects and token generators.
			ot := gameengine.OracleTextLower(card)
			if strings.Contains(ot, "get +") || strings.Contains(ot, "anthem") {
				score += 1.5
			}
			if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
				score += 1.0
			}
		case ArchetypeReanimator:
			// Big creatures to reanimate.
			cmc := gameengine.ManaCostOf(card)
			if typeLineContains(card, "creature") && cmc >= 6 {
				score += 1.5
			}
			// Reanimate spells if we have targets in graveyard.
			ot := gameengine.OracleTextLower(card)
			if strings.Contains(ot, "return") && strings.Contains(ot, "graveyard") {
				if seat != nil && len(seat.Graveyard) > 3 {
					score += 2.0
				}
			}
		case ArchetypeControl, ArchetypeStax:
			// Board wipes and lock pieces.
			ot := gameengine.OracleTextLower(card)
			if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") {
				maxOppBoard := 0
				for i, s := range gs.Seats {
					if i != seatIdx && s != nil && !s.Lost {
						bp := boardPower(s)
						if bp > maxOppBoard {
							maxOppBoard = bp
						}
					}
				}
				if maxOppBoard > boardPower(seat) {
					score += 3.0
				}
			}
		case ArchetypeSpellslinger:
			ot := gameengine.OracleTextLower(card)
			if strings.Contains(ot, "copy") || strings.Contains(ot, "whenever you cast") {
				score += 1.5
			}
		}
	}

	// 7. Don't tutor for something we already have on battlefield.
	if seat != nil {
		for _, p := range seat.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == name {
				score -= 3.0
				break
			}
		}
	}

	return score
}

// inferOpponentArchetype classifies what an opponent is doing based on
// the cards we've observed them play. Updates perceivedArchetype.
func (h *YggdrasilHat) inferOpponentArchetype(gs *gameengine.GameState, oppSeat int) string {
	if oppSeat < 0 || oppSeat >= len(h.perceivedArchetype) {
		return ArchetypeMidrange
	}
	if h.perceivedArchetype[oppSeat] != "" {
		return h.perceivedArchetype[oppSeat]
	}
	if oppSeat >= len(h.cardsSeen) || len(h.cardsSeen[oppSeat]) < 3 {
		return ArchetypeMidrange
	}
	// Count signal cards from what we've observed.
	var creatures, instSorc, artifacts, enchantments int
	var rampSignals, drawSignals, tokenSignals, graveyardSignals int

	if gs != nil && oppSeat < len(gs.Seats) && gs.Seats[oppSeat] != nil {
		for _, p := range gs.Seats[oppSeat].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if typeLineContains(p.Card, "creature") {
				creatures++
			}
			if typeLineContains(p.Card, "artifact") {
				artifacts++
			}
			if typeLineContains(p.Card, "enchantment") {
				enchantments++
			}
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "add {") || strings.Contains(ot, "add one mana") {
				rampSignals++
			}
			if strings.Contains(ot, "draw") {
				drawSignals++
			}
			if strings.Contains(ot, "create") && strings.Contains(ot, "token") {
				tokenSignals++
			}
			if strings.Contains(ot, "graveyard") && strings.Contains(ot, "return") {
				graveyardSignals++
			}
		}
	}
	for name := range h.cardsSeen[oppSeat] {
		lower := strings.ToLower(name)
		_ = lower
		instSorc++
	}

	arch := ArchetypeMidrange
	if creatures >= 6 && tokenSignals >= 2 {
		arch = ArchetypeTribal
	} else if rampSignals >= 3 {
		arch = ArchetypeRamp
	} else if graveyardSignals >= 2 {
		arch = ArchetypeReanimator
	} else if drawSignals >= 3 && creatures <= 3 {
		arch = ArchetypeControl
	} else if instSorc >= 8 && creatures <= 4 {
		arch = ArchetypeSpellslinger
	} else if creatures >= 5 {
		arch = ArchetypeAggro
	}

	h.perceivedArchetype[oppSeat] = arch
	return arch
}

// opponentLikelyHasWrath returns true if we believe an opponent might be
// holding a board wipe based on: control archetype + high hand size + blue/white colors.
func (h *YggdrasilHat) opponentLikelyHasWrath(gs *gameengine.GameState, oppSeat int) bool {
	if gs == nil || oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[oppSeat]
	if s == nil || s.Lost || len(s.Hand) < 3 {
		return false
	}
	arch := h.inferOpponentArchetype(gs, oppSeat)
	if arch != ArchetypeControl && arch != ArchetypeStax {
		return false
	}
	avail := gameengine.AvailableManaEstimate(gs, s)
	if avail < 4 {
		return false
	}
	if oppSeat < len(h.opponentColors) {
		if h.opponentColors[oppSeat]["W"] || h.opponentColors[oppSeat]["B"] {
			return true
		}
	}
	return false
}

// -- Rollout simulation (reuses the same pattern as MCTSHat) --

func (h *YggdrasilHat) canRollout() bool {
	return h.Budget >= rolloutBudgetGe && h.TurnRunner != nil
}

func (h *YggdrasilHat) simulateRollout(gs *gameengine.GameState, seatIdx int, actionFn func(clone *gameengine.GameState)) float64 {
	rolloutSeedCounter++
	rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + rolloutSeedCounter))
	clone := gs.CloneForRollout(rng)
	if clone == nil {
		return 0
	}

	for _, s := range clone.Seats {
		if s != nil && s.Hat != nil {
			if yh, ok := s.Hat.(*YggdrasilHat); ok {
				light := NewYggdrasilHat(yh.Strategy, 0)
				s.Hat = light
			} else if mh, ok := s.Hat.(*MCTSHat); ok {
				s.Hat = mh.Inner
			}
		}
	}

	actionFn(clone)
	resolveStack(clone)
	gameengine.StateBasedActions(clone)

	for i := 0; i < rolloutDepth; i++ {
		if clone.CheckEnd() {
			break
		}
		clone.Active = advanceActive(clone)
		h.TurnRunner(clone)
		gameengine.StateBasedActions(clone)
	}

	return h.Evaluator.Evaluate(clone, seatIdx)
}

var colorLandTypes = []struct {
	name string
	sym  string
}{
	{"plains", "W"}, {"island", "U"}, {"swamp", "B"},
	{"mountain", "R"}, {"forest", "G"},
}

func landProducesColors(c *gameengine.Card) map[string]bool {
	out := make(map[string]bool)
	if c == nil {
		return out
	}
	ot := gameengine.OracleTextLower(c)
	tl := strings.ToLower(c.TypeLine)
	for _, col := range colorLandTypes {
		if strings.Contains(tl, col.name) || strings.Contains(ot, "add {"+strings.ToLower(col.sym)+"}") {
			out[col.sym] = true
		}
	}
	if strings.Contains(ot, "any color") {
		for _, col := range colorLandTypes {
			out[col.sym] = true
		}
	}
	return out
}

func fieldColorSources(seat *gameengine.Seat, color string) int {
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		isLand := false
		for _, t := range p.Card.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if !isLand {
			continue
		}
		cols := landProducesColors(p.Card)
		if cols[color] {
			count++
		}
	}
	return count
}

