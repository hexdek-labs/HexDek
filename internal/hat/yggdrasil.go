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
	NeuralEval *NeuralEvaluator // optional neural position evaluator (Level 5)
	MicroNet   *MicroNet        // optional per-deck micro neural net (Level 6)
	Strategy   *StrategyProfile
	Budget     int     // 0=heuristic, 1-199=evaluator-guided, 200+=rollout
	Noise      float64 // gaussian σ applied to targeting scores (0=deterministic, 0.2=default)
	TurnRunner TurnRunnerFunc

	DecisionLog *[]string

	noiseRNG *rand.Rand

	// noiseSeededFor records the gs.Seed value the noiseRNG was last
	// (re)seeded from. Constructors seed noiseRNG from the global rand
	// (nondeterministic — fine for unseeded casual games); the first
	// ObserveEvent of a game whose GameState carries a real Seed reseeds
	// the stream deterministically from (gs.Seed, seatIdx) so seeded
	// games replay identically (r62 seed-determinism fix). 0 = not yet
	// deterministically seeded.
	noiseSeededFor int64

	// Combo sequencer: evaluates whether a combo win is available.
	// nil when the deck has no combo lines (from Freya).
	comboSeq *ComboSequencer

	// Plan state machine: tracks strategic intent and transitions.
	planState PlanState

	// UCB1 tracking (turn-scoped keys).
	actionStats map[string]*actionStat
	totalVisits int

	// rolloutSeed is bumped per rollout invocation to give each candidate
	// a distinct RNG stream within a decision. PER-HAT (not a package
	// global) so that hats running in parallel — or multiple hats in
	// the same process — don't share seed state. Reset on game_start
	// so each game starts from a known seed sequence (reproducible
	// replays + test stability).
	rolloutSeed int64

	// Per-opponent observation for politics.
	damageDealtTo      []int
	damageReceivedFrom []int
	spellsCastBy       []int
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

	// poisonReceivedFrom tracks cumulative poison counters received from
	// each opponent seat. Mirrors damageReceivedFrom but for infect/toxic.
	poisonReceivedFrom []int

	// -- 3rd Eye: Shannon Entropy Hand Tracking --

	// opponentHandEntropy is a heuristic [0,1] estimate of how much we
	// know about each opponent's hand. 0 = fully known, 1 = total unknown.
	opponentHandEntropy []float64

	// opponentHeldMana tracks consecutive turns where an opponent passed
	// with 2+ mana untapped. High values in interactive colors (U/B)
	// strongly suggest they're holding instant-speed answers.
	opponentHeldMana []int

	// opponentMaxHeldMana is the largest available-mana count we've ever
	// observed at this opponent's upkeep. The streak counter above is
	// binary at the 2+ threshold; this is the magnitude signal. A 4+
	// reading suggests Cryptic Command / Force of Negation / Mystic
	// Confluence territory — a different counterspell threshold than
	// a 2-mana Spell Pierce / Mana Leak rep. Read by classifyOpponent
	// to bump combo/control confidence on the Cryptic-class reading.
	// Reset to 0 alongside opponentHeldMana on game_start.
	opponentMaxHeldMana []int

	// opponentTutored records whether each opponent has tutored this game.
	// After a tutor resolves they almost certainly have a specific answer
	// or combo piece — near-zero entropy for one hand slot.
	opponentTutored []bool

	// R60: per-opponent magnitude counters for the three "what they've
	// cast so far" 3rd Eye signals. The existing opponentTutored bool
	// captures whether they've EVER tutored — these counts capture HOW
	// MANY TIMES, so the hat can predict "they've already cast 3 of
	// their ~10 counters; the next bluff is materially more likely to
	// be empty" and similar. Symmetrically populated for each cast
	// event in ObserveEvent via the isCounterspellText / isMassRemovalText
	// / isTutorText classifiers. Reset on game_start.
	opponentCounterspellsCast []int
	opponentMassRemovalCast   []int
	opponentTutorsCast        []int

	// opponentKnownCards tracks cards we know are in an opponent's hand
	// (from reveal effects). These are zero-entropy slots.
	opponentKnownCards []map[string]bool

	// -- 3rd Eye: Bluff detection (R60 round 5) --
	//
	// opponentLoadedSilentTurns counts consecutive turns where an
	// opponent's perceived interaction probability was >= 0.4 but they
	// did NOT cast any interactive spell that turn. High values
	// indicate they're more likely bluffing (or genuinely empty) — the
	// hat dampens `opponentHasInteraction` by a factor derived from
	// this count via `perceivedInteractionThreat`. Capped at
	// bluffMaxStreak so a single counter cast meaningfully resets the
	// signal.
	opponentLoadedSilentTurns []int

	// opponentFiredInteractionThisRound is a per-opponent bool that
	// flips true when we observe them casting an interactive spell
	// (counter / targeted removal / instant-speed answer); reset on
	// each upkeep where we evaluate the bluff signal.
	opponentFiredInteractionThisRound []bool

	// Pre-computed lookup sets for O(1) card relevance checks.
	comboPieceSet   map[string]bool
	valueEngineSet  map[string]bool
	tutorTargetSet  map[string]bool
	finisherSet     map[string]bool
	starCardSet     map[string]bool
	cuttableSet     map[string]bool
	vulnerableToSet map[string]bool
	isTempoComboVal bool
	lookupsBuilt    bool

	// Threat cache — per-turn memoization of assessAllThreats.
	threatCache     []seatThreat
	threatCacheTurn int

	// Reactive-removal one-shot guard (r61 PR-7). Keyed by the *incoming*
	// opponent stack item's ID; records that we already fired a reactive
	// non-counter instant-speed play in response to it. Loop-safety: the
	// AI never responds to its OWN stack item (PriorityRound's
	// top.Controller==seat guard) AND fires at most one reactive removal
	// per distinct opponent stack item, so a priority round can't chain
	// the AI's own responses indefinitely. Cleared each turn.
	reactiveFiredAgainst    map[int]bool
	reactiveFiredAgainstTrn int

	// Pooled map for comboUrgency to avoid per-call allocation.
	availablePool map[string]bool

	// Conviction concession — sliding window of recent position evals.

	// DNA — optional Curse personality parameters that nudge weights,
	// attack thresholds, combo patience, and political behavior.
	// nil means no DNA influence (default behavior).
	DNA *CurseDNA

	// confidenceThreshold controls how picky the hat is about which
	// action to take. Set from StrategyProfile.Bracket:
	//   B1=0.30 (casual, "good enough"), B5=0.90 (near-optimal only).
	// When the gap between best and second-best candidate is less than
	// (1 - confidenceThreshold), the hat picks randomly among top
	// candidates. Higher threshold = more deterministic selection.
	confidenceThreshold float64

	// T3.2: per-game dimension accumulator. Updated on each Evaluate call,
	// exposed via DimensionMeans() for post-game outcome correlation.
	dimAccum [NumDimensions]float64
	dimCount int

	// Cached exploration factor for UCB1 — recomputed per turn.
	explorationC     float64
	explorationCTurn int

	// Per-turn evaluation budget. TurnBudget > 0 enables the system:
	// each evaluator-path decision costs 1 point, each rollout costs
	// rolloutEvalCost points. When exhausted, remaining decisions in
	// the turn degrade to heuristic. 0 = legacy per-action mode.
	TurnBudget     int
	turnEvalsSpent int
	turnBudgetTurn int

	// tierCounts tracks per-game decision routing. Indexed by DecisionTier.
	// Reset by ResetMjolnirStats() and exposed via MjolnirStats().
	tierCounts [numDecisionTiers]int

	// R60 cascading-decisions audit. Parent-decision tier propagation:
	// when ChooseCastFromHand / ChooseActivation / ChooseAttackers /
	// ChooseResponse classifies a decision at a given tier, that tier
	// is also stamped here as the "ambient" tier. Downstream cascading
	// decisions (ChooseTarget called during the cast's resolution,
	// ChooseMode for modal triggers, ChooseSacrifice for an X-cost
	// payment, OrderTriggers for same-event APNAP ordering) read
	// `lastParentTier` so they can escalate their evaluation when the
	// parent decision earned Ragnarok-tier compute. Without this
	// propagation, a Ragnarok-rollout-evaluated Demonic Tutor cast
	// would resolve into a TierMjolnir first-match target pick — the
	// expensive parent decision priced the WRONG thing.
	lastParentTier     DecisionTier
	lastParentTierTurn int

	// stackItemTiers — R60 round 5 multi-instance priority window
	// audit. Per-stack-item record of the DecisionTier classified by
	// the ChooseResponse call for that item. Survives ChooseResponse
	// overwrites of lastParentTier (which only holds the MOST RECENT
	// classification), so cascade decisions firing during an earlier
	// item's resolution can recover the correct tier by ID lookup.
	// Reset on game_start; lazily cleared on first stamp of a new
	// turn via stackItemTiersTurn.
	stackItemTiers     map[int]DecisionTier
	stackItemTiersTurn int

	// R60 round 5 — multi-mode follow-up detection state. When the
	// engine's resolveChoice loops ChooseMode `pick` times for a
	// pick-2 modal spell (Cryptic Command, charms, …), each iteration
	// shows a slice that's the previous one minus the prior pick. We
	// remember the previous pick + slice so the next call can detect
	// the subset relationship and apply complement scoring (counter
	// → bounce, bounce → draw, etc.).
	lastChooseModePick  gameast.Effect
	lastChooseModeSlice []gameast.Effect
	lastChooseModeTurn  int

	// -- Zone-cast grant tracking (flashback / escape / impulse / etc.) --

	// myZoneCastGrants counts how many active zone-cast permissions the
	// engine has registered for *our* seat, keyed by keyword
	// ("flashback", "escape", "free_exile_cast", ...). Decremented when
	// the engine emits zone_cast_grant_expired. The authoritative source
	// remains gs.ZoneCastGrants — this is a fast inbound-event signal.
	myZoneCastGrants map[string]int

	// -- Linked-exile awareness (CR §406.7 — O-Ring style effects) --

	// linkedExilesByMe is the count of cards we have currently exiled with
	// a permanent we control. Higher = more value tied up in fragile
	// permanents that need protection.
	linkedExilesByMe int

	// linkedExilesByOpponent[seat] is the count of cards each opponent has
	// currently exiled via linked permanents. High values flag good
	// targets for our removal: killing the source returns the exiled
	// cards (often our own creatures or a key threat).
	linkedExilesByOpponent []int

	// opponentProfiles[seat] is the rolling, per-opponent classification
	// (aggro / combo / control / midrange / unknown) plus per-event
	// tallies. Updated by recordOpponentPlay during ObserveEvent and
	// read by classifyOpponent from decision functions. See
	// opponent_profile.go for shape and rules.
	opponentProfiles []*OpponentProfile

	// convictionDiag is the non-acting concession diagnostic. ShouldConcede
	// always returns false, but each turn we record what a candidate
	// trigger *would* have decided so post-game analysis can compare
	// trigger fire vs. actual game outcome. See conviction.go and
	// docs/conviction-reassessment-2026-05-17.md.
	convictionDiag *convictionDiagnostic
}

const (
	// Adaptive budget: degrade to heuristic when board is too complex.
	// 60 permanents across 4 seats = ~15 each = a developed mid-game board.
	adaptiveBudgetComplexityThreshold = 60

	// Per-turn budget costs. A rollout is ~10x more expensive than a
	// single evaluator-path decision (clone + forward sim + eval).
	rolloutEvalCost = 10

	// R60 high-stakes overrides for adaptive degradation.
	//   highStakesLifeThreshold: any seat at or below this life makes
	//     the next priority window decision game-deciding.
	//   highStakesStackDepth: stack at or above this depth indicates a
	//     counter war or triggered-ability chain where resolution-order
	//     decisions materially change the outcome.
	highStakesLifeThreshold = 8
	highStakesStackDepth    = 3

	// r61 PR-8 graceful budget degradation. Replaces the pre-r61 hard
	// `return 0` cliff at adaptiveComplexityThreshold with a linear taper
	// so the hat keeps doing SOME evaluation on big boards instead of
	// dropping to pure heuristic the moment the table crosses ~60
	// permanents (a routine turn-8+ 4-player Commander board).
	//
	//   degradedBudgetTaperPerPerm: each permanent past the complexity
	//     threshold shaves this fraction off the budget multiplier. At
	//     0.02, the budget hits the floor factor ~42 permanents over the
	//     threshold (e.g. threshold 60 → floor at ~102 permanents).
	//   degradedBudgetFloorFactor: the multiplier never falls below this,
	//     so even a pathological 120-permanent board still spends ~15% of
	//     base budget on evaluation. With base Budget=50 that is ~7
	//     (evaluator-guided, NOT rollout); with base Budget=200 (rollout)
	//     the taper drops effective budget below rolloutBudgetGe well
	//     before the floor, so rollouts cut out first and only the cheaper
	//     evaluator path runs on huge boards — bounding per-decision cost.
	//   degradedBudgetAbsoluteFloor: hard minimum so the degraded budget
	//     is always >= this (never 0). Keeps the hat off the pure-
	//     heuristic Mjolnir path on complex boards.
	degradedBudgetTaperPerPerm  = 0.02
	degradedBudgetFloorFactor   = 0.15
	degradedBudgetAbsoluteFloor = 5
)

// DecisionTier names the compute path a decision takes. The hat already
// has three implicit tiers; this type makes the routing explicit so the
// distribution can be observed and tuned.
type DecisionTier int

const (
	// TierMjolnir: cheap, deterministic heuristic. No evaluator, no rollout.
	TierMjolnir DecisionTier = iota
	// TierGungnir: evaluator-guided scoring with UCB1 exploration.
	TierGungnir
	// TierRagnarok: MCTS-style rollout (clone + forward sim + evaluate).
	TierRagnarok

	numDecisionTiers = 3
)

func (t DecisionTier) String() string {
	switch t {
	case TierMjolnir:
		return "Mjolnir"
	case TierGungnir:
		return "Gungnir"
	case TierRagnarok:
		return "Ragnarok"
	default:
		return "Unknown"
	}
}

// MjolnirStats reports per-tier decision counts for one game.
type MjolnirStats struct {
	Mjolnir  int
	Gungnir  int
	Ragnarok int
}

// Total returns the sum of decisions across all tiers.
func (s MjolnirStats) Total() int { return s.Mjolnir + s.Gungnir + s.Ragnarok }

type evalCacheKey struct {
	seat int
}

func NewYggdrasilHat(strategy *StrategyProfile, budget int) *YggdrasilHat {
	return NewYggdrasilHatWithNoise(strategy, budget, 0.2)
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

func (h *YggdrasilHat) EvalsSpent() int       { return h.turnEvalsSpent }
func (h *YggdrasilHat) PlanCurrent() GamePlan { return h.planState.Current }

func NewYggdrasilHatWithNoise(strategy *StrategyProfile, budget int, noise float64) *YggdrasilHat {
	h := &YggdrasilHat{
		Evaluator:     NewEvaluator(strategy),
		Strategy:      strategy,
		Budget:        budget,
		Noise:         noise,
		noiseRNG:      rand.New(rand.NewSource(rand.Int63())),
		actionStats:   make(map[string]*actionStat),
		availablePool: make(map[string]bool, 32),
		comboSeq:      NewComboSequencer(strategy),
	}
	h.applyBracketDial(strategy)
	h.buildLookupSets()
	return h
}

// NewYggdrasilHatWithDNA creates a YggdrasilHat whose behavior is nudged
// by Curse DNA parameters. DNA values are [0,1] floats centered at 0.5
// (neutral); values above/below 0.5 push the hat's personality in the
// corresponding direction without replacing the archetype-tuned baseline.
//
// Mapping:
//   - Aggression → lowers attack thresholds (stored as DNA, read in ChooseAttackers)
//   - ComboPat   → increases pass/hold combo patience (stored as DNA, read in priority decisions)
//   - ThreatParanoia → scales ThreatExposure eval weight (+/- 40% from center)
//   - ResourceGreed  → shifts CardAdvantage up and BoardPresence down (or vice versa)
//   - PoliticalMemory → slows détente discount decay (stored as DNA, read in targeting)
func NewYggdrasilHatWithDNA(dna *CurseDNA, sp *StrategyProfile, budget int) *YggdrasilHat {
	h := NewYggdrasilHatWithNoise(sp, budget, 0.2)
	if dna == nil {
		return h
	}
	h.DNA = dna

	// --- Weight nudges ---
	// DNA values are [0,1]; 0.5 is neutral. We compute a signed offset
	// in [-0.5, +0.5] and use it to scale the baseline weight.

	// ThreatParanoia: high → increase ThreatExposure weight by up to 40%.
	threatShift := (dna.ThreatParanoia - 0.5) * 0.8 // [-0.4, +0.4]
	h.Evaluator.Weights.ThreatExposure *= 1.0 + threatShift

	// ResourceGreed: high → favor CardAdvantage over BoardPresence.
	// Low → favor BoardPresence over CardAdvantage. Max swing +/- 30%.
	greedShift := (dna.ResourceGreed - 0.5) * 0.6 // [-0.3, +0.3]
	h.Evaluator.Weights.CardAdvantage *= 1.0 + greedShift
	h.Evaluator.Weights.BoardPresence *= 1.0 - greedShift

	// DNA Aggression nudges confidence threshold: aggressive DNA lowers
	// the threshold (more impulsive play), cautious DNA raises it.
	// Max swing: +/- 0.10 from the bracket baseline.
	aggroShift := (dna.Aggression - 0.5) * 0.2 // [-0.10, +0.10]
	h.confidenceThreshold -= aggroShift
	if h.confidenceThreshold < 0.1 {
		h.confidenceThreshold = 0.1
	}
	if h.confidenceThreshold > 0.95 {
		h.confidenceThreshold = 0.95
	}

	// ComboPat: high patience → longer Assemble before Pivot (3-8 turns).
	h.planState.ComboPatience = 3 + int(dna.ComboPat*5)

	// DrainAffinity: high → increase DrainEngine weight by up to 40%.
	drainShift := (dna.DrainAffinity - 0.5) * 0.8
	h.Evaluator.Weights.DrainEngine *= 1.0 + drainShift

	// ArtifactAffinity: high → increase ArtifactSynergy weight by up to 40%.
	artifactShift := (dna.ArtifactAffinity - 0.5) * 0.8
	h.Evaluator.Weights.ArtifactSynergy *= 1.0 + artifactShift

	// LandGreed: high → favor mana development. Scales ManaAdvantage up
	// and BoardPresence down (vice versa for low). Max swing ±30%.
	landShift := (dna.LandGreed - 0.5) * 0.6
	h.Evaluator.Weights.ManaAdvantage *= 1.0 + landShift
	h.Evaluator.Weights.BoardPresence *= 1.0 - landShift*0.5

	// EquipmentAffinity: high → care more about ArtifactSynergy on the
	// eval side AND scale equipment-specific scoring inline at equip
	// option sites (see equipmentAffinityMult). Max ±20% on the
	// archetype weight, since EquipmentAffinity stacks with
	// ArtifactAffinity for true Voltron decks.
	equipShift := (dna.EquipmentAffinity - 0.5) * 0.4
	h.Evaluator.Weights.ArtifactSynergy *= 1.0 + equipShift

	// GraveyardExploitation: high → prize GraveyardValue (recursion,
	// reanimator targets) and amplify the discount on cards with
	// graveyard recursion potential when discarding from hand. Max ±40%
	// on the eval weight; the inline discount lives in card-pick
	// helpers via graveyardExploitationMult().
	gyShift := (dna.GraveyardExploitation - 0.5) * 0.8
	h.Evaluator.Weights.GraveyardValue *= 1.0 + gyShift

	// CounterplayTiming: high → hold up interaction (boost
	// StackInteraction); low → slam threats (suppress StackInteraction,
	// nudge ThreatTrajectory). Max ±40% on StackInteraction, ±20% on
	// ThreatTrajectory.
	cpShift := (dna.CounterplayTiming - 0.5) * 0.8
	h.Evaluator.Weights.StackInteraction *= 1.0 + cpShift
	h.Evaluator.Weights.ThreatTrajectory *= 1.0 - cpShift*0.5

	// TokenPressure: high → go-wide bias. Boost BoardPresence and
	// ActivationTempo (token producers tend to be activated abilities);
	// low → go-tall, boost CommanderProgress. Max ±30%.
	tokShift := (dna.TokenPressure - 0.5) * 0.6
	h.Evaluator.Weights.BoardPresence *= 1.0 + tokShift
	h.Evaluator.Weights.ActivationTempo *= 1.0 + tokShift*0.5
	h.Evaluator.Weights.CommanderProgress *= 1.0 - tokShift*0.3

	return h
}

// equipmentAffinityMult returns a multiplier applied at equipment-scoring
// sites to scale equip-target value by the hat's EquipmentAffinity DNA.
// Returns 1.0 when DNA is absent, [0.6, 1.4] otherwise.
func (h *YggdrasilHat) equipmentAffinityMult() float64 {
	if h == nil || h.DNA == nil {
		return 1.0
	}
	return 1.0 + (h.DNA.EquipmentAffinity-0.5)*0.8
}

// graveyardExploitationMult returns a multiplier for inline graveyard-
// recursion discounts when picking discards / surveils / mill targets.
// High DNA → bigger discount on recursion-bearing cards (route them to
// the yard); low DNA → smaller discount. Returns 1.0 without DNA.
func (h *YggdrasilHat) graveyardExploitationMult() float64 {
	if h == nil || h.DNA == nil {
		return 1.0
	}
	return 1.0 + (h.DNA.GraveyardExploitation-0.5)*0.8
}

// commanderOnBattlefield reports whether any of seat's commander names
// is currently on its battlefield, returning the matched permanent so
// callers can read its types for tribal / theme matching. Match is
// case-insensitive and DFC-aware via DisplayName.
func commanderOnBattlefield(seat *gameengine.Seat) (bool, *gameengine.Permanent) {
	if seat == nil {
		return false, nil
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		name := p.Card.DisplayName()
		for _, cn := range seat.CommanderNames {
			if strings.EqualFold(cn, name) {
				return true, p
			}
		}
	}
	return false, nil
}

// commanderInCommandZone reports whether any of seat's commanders is
// currently sitting in the command zone (i.e., uncast or returned).
func commanderInCommandZone(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, c := range seat.CommandZone {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		for _, cn := range seat.CommanderNames {
			if strings.EqualFold(cn, name) {
				return true
			}
		}
	}
	return false
}

// minCommanderCMC returns the lowest CMC across the seat's commanders
// currently in the command zone. Returns 0 when no commanders are
// accessible (free-form games, edge tests) so callers can short-circuit
// the commander-castability check rather than mistreat 0 as a "free
// commander". For partner decks (multiple commanders) returns the
// cheaper one — you can cast either to start executing the deck's plan.
func minCommanderCMC(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	minCMC := 0
	for _, c := range seat.CommandZone {
		if c == nil {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc <= 0 {
			continue
		}
		if minCMC == 0 || cmc < minCMC {
			minCMC = cmc
		}
	}
	return minCMC
}

// projectedManaAtTurn estimates the mana available to cast a single
// spell at the given turn given an opening hand's land and ramp count.
// Heuristic — NOT a Monte Carlo simulator. Two components:
//
//  1. Land drops: min(turn, landsInHand + (turn-1) * 0.4) — we play one
//     land per turn (capped at one drop) and draw ~0.4 lands per draw
//     step (typical Commander deck runs 36-38 lands / 99 ≈ 38% land
//     density). The cap prevents an opener with 6 lands from reading
//     as 6 mana on turn 4 — you can only play one a turn.
//  2. Ramp acceleration: each ramp piece adds ~1 net mana per turn
//     once active. Typical ramp pieces (Signet, Talisman, Llanowar
//     Elves, Sol Ring) cost 1-2 and produce 1 starting the turn after
//     they ETB. Cast turn 2 at the earliest given typical cost, then
//     productive turn 3 onward — so the bonus only applies when
//     turn >= 3. Capped at 2 ramp pieces — beyond that you'd run out
//     of curve windows to cast them all before turn 4.
//
// Returns a float so the caller can compare against integer commander
// CMC with the implicit half-mana margin of error baked in (e.g.
// projected 5.2 vs CMC 5 reads as castable; 4.8 vs CMC 5 reads as not).
func projectedManaAtTurn(landsInHand, rampInHand, turn int) float64 {
	if turn <= 0 {
		return 0
	}
	draws := float64(turn - 1)
	landsByT := float64(landsInHand) + draws*0.4
	if landsByT > float64(turn) {
		landsByT = float64(turn)
	}
	rampBonus := 0.0
	if turn >= 3 {
		r := rampInHand
		if r > 2 {
			r = 2
		}
		rampBonus = float64(r)
	}
	return landsByT + rampBonus
}

// sharesCreatureSubtype returns true when commander and other share at
// least one creature subtype (case-insensitive), using Types as the
// canonical subtype container per this engine's convention. The base
// "creature" type itself is excluded so two random creatures don't
// match each other tribally.
func sharesCreatureSubtype(commander, other *gameengine.Card) bool {
	if commander == nil || other == nil {
		return false
	}
	skip := map[string]bool{
		"creature": true, "legendary": true, "token": true,
		"artifact": true, "enchantment": true, "land": true,
		"instant": true, "sorcery": true, "planeswalker": true, "tribal": true,
	}
	cmdrSubs := make(map[string]bool, len(commander.Types))
	for _, t := range commander.Types {
		lt := strings.ToLower(strings.TrimSpace(t))
		if lt == "" || skip[lt] || strings.HasPrefix(lt, "cost:") {
			continue
		}
		cmdrSubs[lt] = true
	}
	for _, t := range other.Types {
		lt := strings.ToLower(strings.TrimSpace(t))
		if lt == "" || skip[lt] || strings.HasPrefix(lt, "cost:") {
			continue
		}
		if cmdrSubs[lt] {
			return true
		}
	}
	return false
}

// isCommanderProtectionCard returns true when c plausibly protects the
// commander on cast or while attacking — counterspells, hexproof /
// indestructible / shroud / ward grants, and "your commander" /
// "target creature you control" protection effects. Heuristic only;
// good enough for a +0.15 nudge.
func isCommanderProtectionCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if gameengine.CardHasCounterSpell(c) {
		return true
	}
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	// Oracle-text fallback for counterspells whose AST shape isn't
	// reachable via CardHasCounterSpell (e.g. test fixtures or
	// partially-modeled cards) — and for built-in protection grants
	// the engine doesn't surface as activated abilities.
	if strings.Contains(ot, "counter target") ||
		strings.Contains(ot, "hexproof") || strings.Contains(ot, "indestructible") ||
		strings.Contains(ot, "shroud") || strings.Contains(ot, "ward ") ||
		strings.Contains(ot, "protection from") {
		return true
	}
	return false
}

// curseAxis returns the value of a DNA axis, or fallback if no DNA is
// attached. Use the [0,1] axis directly with this helper, or for a
// neutral-centered shift use the result minus 0.5. Keeps decision-site
// reads concise and nil-safe so callers don't have to repeat
// `if h.DNA != nil` everywhere.
func (h *YggdrasilHat) curseAxis(getter func(*CurseDNA) float64, fallback float64) float64 {
	if h == nil || h.DNA == nil {
		return fallback
	}
	return getter(h.DNA)
}

// NewYggdrasilHatWithPool creates a hat using DNA + learned dimension
// corrections from the pool's outcome-correlation statistics (T3.2).
func NewYggdrasilHatWithPool(dna *CurseDNA, sp *StrategyProfile, budget int, ds *DimensionStats) *YggdrasilHat {
	h := NewYggdrasilHatWithDNA(dna, sp, budget)
	if ds == nil || ds.N < dimStatsMinN {
		return h
	}
	corr := ds.WeightCorrections()
	w := &h.Evaluator.Weights
	arr := w.AsArray()
	for d := 0; d < NumDimensions; d++ {
		arr[d] *= corr[d]
	}
	w.BoardPresence = arr[0]
	w.CardAdvantage = arr[1]
	w.ManaAdvantage = arr[2]
	w.LifeResource = arr[3]
	w.ComboProximity = arr[4]
	w.ThreatExposure = arr[5]
	w.CommanderProgress = arr[6]
	w.GraveyardValue = arr[7]
	w.DrainEngine = arr[8]
	w.ArtifactSynergy = arr[9]
	w.EnchantmentSynergy = arr[10]
	w.OpponentGraveyardThreat = arr[11]
	w.PartnerSynergy = arr[12]
	w.ActivationTempo = arr[13]
	w.ToolboxBreadth = arr[14]
	w.ThreatTrajectory = arr[15]
	w.StackInteraction = arr[16]
	w.PlaneswalkerProgress = arr[17]
	w.ExileZoneAssets = arr[18]
	w.StaxLockProgress = arr[19]
	return h
}

// applyBracketDial sets the confidence threshold and noise based on the
// strategy profile's power bracket. This is the Watts dial — same code
// path, different sensitivity. Low brackets produce warm, varied play;
// high brackets produce cold, precise play.
func (h *YggdrasilHat) applyBracketDial(sp *StrategyProfile) {
	// Confidence threshold: how picky when picking among candidates.
	switch {
	case sp != nil && sp.Bracket >= 5:
		h.confidenceThreshold = 0.9
	case sp != nil && sp.Bracket >= 4:
		h.confidenceThreshold = 0.75
	case sp != nil && sp.Bracket >= 3:
		h.confidenceThreshold = 0.6
	case sp != nil && sp.Bracket >= 2:
		h.confidenceThreshold = 0.45
	default:
		h.confidenceThreshold = 0.3
	}

	// Noise override: bracket-scaled noise replaces the caller-provided
	// value when a bracket is known. Lower brackets get more noise
	// (varied, natural play), higher brackets get less (deterministic).
	if sp != nil && sp.Bracket >= 1 && sp.Bracket <= 5 {
		bracketNoise := [6]float64{0, 0.35, 0.25, 0.15, 0.10, 0.05}
		h.Noise = bracketNoise[sp.Bracket]
	}
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

// -----------------------------------------------------------------------------
// Game phase model — continuous awareness of where we are in the game.
//
// Replaces ad-hoc `gs.Turn > 15` checks. Real Commander games have three
// distinct phases where optimal play looks different:
//
//   PhaseDeploy   (turns 1–4 baseline): ramp up, deploy commander, fix mana.
//                                       Conservative attacks; expensive threats
//                                       are mostly dead in hand.
//   PhaseDevelop  (turns 5–9 baseline): build the board, run value engines,
//                                       gather card advantage.
//   PhaseExecute  (turns 10+ baseline): push wins, deploy combo finishers,
//                                       cast commander even at high tax.
//
// Detection is multi-signal: turn number is the spine but board state, mana
// availability, commander deployment, and hand exhaustion can pull the phase
// forward (or, rarely, hold it back).
// -----------------------------------------------------------------------------

// GamePhase classifies the current strategic moment of the game.
type GamePhase int

const (
	PhaseDeploy  GamePhase = iota // turns 1-4: ramp, deploy commander, fix mana
	PhaseDevelop                  // turns 5-9: build board, value engine, card advantage
	PhaseExecute                  // turns 10+: push win conditions, close the game
)

// String returns a short label for logging.
func (p GamePhase) String() string {
	switch p {
	case PhaseDeploy:
		return "Deploy"
	case PhaseDevelop:
		return "Develop"
	case PhaseExecute:
		return "Execute"
	}
	return "?"
}

// detectPhase blends turn number with board / mana / hand signals so a deck
// that ramps to 7 mana on turn 3 is treated as past Deploy, and a deck that
// hits turn 12 with an empty hand is treated as Execute regardless of the
// turn count being "only" 12.
//
// Signals (in order of weight):
//
//  1. Turn number (primary). 1-4 → Deploy, 5-9 → Develop, 10+ → Execute.
//  2. Mana availability. avail >= 9 OR avail >= 7 with a commander on the
//     battlefield → bump Deploy → Develop. avail >= 12 → Execute.
//  3. Commander on battlefield. Deploy is "deploy commander"; once it's out,
//     bump to at least Develop.
//  4. Board size. 5+ controlled creatures or 8+ permanents = past Deploy.
//  5. Cards in hand. Hand size <= 1 with avail mana spent = Execute (no fuel
//     left, only thing to do is push).
//
// All bumps are monotonic (never demote past the turn-spine baseline) — a
// conservative deck that didn't ramp still gets the turn-based phase.
func (h *YggdrasilHat) detectPhase(gs *gameengine.GameState, seatIdx int) GamePhase {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return PhaseDeploy
	}
	turn := gs.Turn
	phase := PhaseDeploy
	switch {
	case turn >= 10:
		phase = PhaseExecute
	case turn >= 5:
		phase = PhaseDevelop
	}

	seat := gs.Seats[seatIdx]
	avail := gameengine.AvailableManaEstimate(gs, seat)

	// Commander on battlefield → past initial Deploy goal.
	commanderOut := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if gameengine.IsCommanderCard(gs, seatIdx, p.Card) {
			commanderOut = true
			break
		}
	}

	// Board size — count creatures + total nonland permanents.
	creatureCount := 0
	nonlandCount := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			creatureCount++
		}
		if !p.IsLand() {
			nonlandCount++
		}
	}

	bumpToDevelop := func() {
		if phase < PhaseDevelop {
			phase = PhaseDevelop
		}
	}
	bumpToExecute := func() {
		if phase < PhaseExecute {
			phase = PhaseExecute
		}
	}

	// Mana / commander signals → at least Develop.
	if avail >= 9 {
		bumpToDevelop()
	}
	if avail >= 7 && commanderOut {
		bumpToDevelop()
	}
	if commanderOut {
		bumpToDevelop()
	}
	if creatureCount >= 5 || nonlandCount >= 8 {
		bumpToDevelop()
	}

	// Heavy mana / hand exhaustion / huge board → Execute.
	if avail >= 12 {
		bumpToExecute()
	}
	if len(seat.Hand) <= 1 && turn >= 6 {
		bumpToExecute()
	}
	if creatureCount >= 8 || nonlandCount >= 12 {
		bumpToExecute()
	}

	return phase
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

	// Alt-wincon threat fields (poison, PW loyalty, mill, commander damage).
	HasInfect       bool    // controls creatures with infect or toxic
	PoisonToUs      int     // cumulative poison counters dealt to us
	PWLoyaltyThreat float64 // max planeswalker-ultimate proximity [0,1]
	MillThreat      float64 // threat from library depletion [0,1]
	CmdrDmgToUs     int     // max commander damage from this seat to us

	// Shannon entropy hand-tracking fields.
	HandEntropy     float64 // heuristic [0,1]: 0=fully known, 1=total unknown
	HeldManaTurns   int     // consecutive turns with 2+ mana untapped
	Tutored         bool    // did they tutor this game?
	LikelyHasAnswer bool    // composite flag: tutored + held mana + interactive colors

	// Wrath risk: probability [0,1] that this opponent is holding a board wipe.
	WrathRisk float64
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
			Seat:        i,
			BoardPower:  boardPower(gs, s),
			Life:        s.Life,
			HandSize:    len(s.Hand),
			ManaSources: CountManaRocksAndLands(s),
		}
		st.EvalScore = h.Evaluator.Evaluate(gs, i)
		if i < len(h.damageReceivedFrom) {
			st.DamageToUs = h.damageReceivedFrom[i]
		}

		// Retaliation risk: stronger opponents with more board presence
		// are more dangerous to provoke. Scale by their board power
		// relative to ours.
		myPow := boardPower(gs, gs.Seats[seatIdx])
		if myPow > 0 {
			st.RetaliationRisk = float64(st.BoardPower) / float64(myPow)
		} else if st.BoardPower > 0 {
			st.RetaliationRisk = 2.0
		}
		// Grudge factor: opponents we've already hit are more likely
		// to retaliate. Decay rate modulated by Curse PoliticalMemory:
		// high memory (→1) = slow decay (long grudge), low (→0) = fast decay.
		if i < len(h.damageDealtTo) {
			dealt := h.damageDealtTo[i]
			if dealt > 0 && s.Life > 0 {
				grudge := float64(dealt) / float64(s.Life) * 0.5
				if h.DNA != nil && gs != nil {
					lastHitTurn := 0
					if i < len(h.lastAttackedUsTurn) {
						lastHitTurn = h.lastAttackedUsTurn[i]
					}
					turnsSince := gs.Turn - lastHitTurn
					if turnsSince > 0 {
						decayRate := 0.85 + h.DNA.PoliticalMemory*0.14
						grudge *= math.Pow(decayRate, float64(turnsSince))
					}
				}
				st.RetaliationRisk += grudge
			}
		}

		// 3rd Eye enrichment.
		st.Momentum = h.threatMomentum(i)
		st.InteractionProb = h.opponentHasInteraction(gs, i)
		st.IsKingmaker = h.isKingmaker(gs, i)
		st.PoliticalEnemy = h.tablePoliticalEnemy(i)

		// 3rd Eye: Shannon entropy hand tracking.
		if i < len(h.opponentHandEntropy) {
			st.HandEntropy = h.opponentHandEntropy[i]
		} else {
			st.HandEntropy = 1.0
		}
		if i < len(h.opponentHeldMana) {
			st.HeldManaTurns = h.opponentHeldMana[i]
		}
		if i < len(h.opponentTutored) {
			st.Tutored = h.opponentTutored[i]
		}

		// Composite "likely has answer" flag: tutored recently AND holding
		// mana open for 2+ turns AND in interactive colors (U or B).
		hasInteractiveColors := false
		if i < len(h.opponentColors) {
			hasInteractiveColors = h.opponentColors[i]["U"] || h.opponentColors[i]["B"]
		}
		st.LikelyHasAnswer = st.Tutored && st.HeldManaTurns >= 2 && hasInteractiveColors

		// Entropy-based threat adjustment: opponents who tutored and are
		// sitting on mana get a threat boost — they are loaded and waiting.
		if st.Tutored && st.HeldManaTurns >= 3 && hasInteractiveColors {
			st.EvalScore += 0.15
		} else if st.Tutored && st.HeldManaTurns >= 2 {
			st.EvalScore += 0.08
		}
		// High held-mana turns in interactive colors (even without tutor)
		// suggest sandbagging with countermagic or removal.
		if st.HeldManaTurns >= 4 && hasInteractiveColors {
			st.EvalScore += 0.10
		}

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

		// -- Alt-wincon threat assessment --

		// Poison: check if opponent controls infect/toxic creatures.
		myPoison := gs.Seats[seatIdx].PoisonCounters
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if p.HasKeyword("infect") || p.HasKeyword("toxic") {
				st.HasInfect = true
				break
			}
		}
		if i < len(h.poisonReceivedFrom) {
			st.PoisonToUs = h.poisonReceivedFrom[i]
		}
		// Poison proximity: boost threat score when we're close to 10.
		if st.HasInfect && myPoison >= 7 {
			// Any infect creature is lethal — treat like TurnsToKill=1.
			st.EvalScore += 0.4
		} else if st.HasInfect && myPoison >= 5 {
			st.EvalScore += 0.2
		}

		// Planeswalker loyalty: scan for PWs approaching ultimate.
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsPlaneswalker() {
				continue
			}
			loyalty := 0
			if p.Counters != nil {
				loyalty = p.Counters["loyalty"]
			}
			// Estimate ultimate cost from oracle text. Typical pattern:
			// "−7:" or "−8:". Heuristic: scan for largest negative number.
			ultimateCost := estimatePWUltimateCost(p.Card)
			if ultimateCost > 0 && loyalty > 0 {
				// How close are they? 1.0 = can ult this turn.
				proximity := float64(loyalty) / float64(ultimateCost)
				if proximity > 1.0 {
					proximity = 1.0
				}
				if proximity > st.PWLoyaltyThreat {
					st.PWLoyaltyThreat = proximity
				}
			}
		}
		// PWs within 1-2 activations of ultimate are high threat.
		if st.PWLoyaltyThreat >= 0.8 {
			st.EvalScore += 0.3
		} else if st.PWLoyaltyThreat >= 0.6 {
			st.EvalScore += 0.15
		}

		// Mill threat: check opponent for mill permanents + our library depth.
		myLibSize := len(gs.Seats[seatIdx].Library)
		hasMill := false
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			ot := gameengine.OracleTextLower(p.Card)
			if (strings.Contains(ot, "mill") || strings.Contains(ot, "cards from the top of") ||
				(strings.Contains(ot, "library") && strings.Contains(ot, "graveyard"))) &&
				(strings.Contains(ot, "opponent") || strings.Contains(ot, "target player") || strings.Contains(ot, "each player")) {
				hasMill = true
				break
			}
		}
		if hasMill && myLibSize > 0 {
			// Threat scales inversely with library size.
			if myLibSize < 10 {
				st.MillThreat = 1.0
			} else if myLibSize < 20 {
				st.MillThreat = 0.7
			} else if myLibSize < 40 {
				st.MillThreat = 0.3
			}
			st.EvalScore += st.MillThreat * 0.3
		}

		// Commander damage: check if this seat's commanders have dealt
		// significant combat damage to us (21 = lethal per §704.6c).
		if gs.CommanderFormat {
			mySeat := gs.Seats[seatIdx]
			if mySeat.CommanderDamage != nil {
				if cmdMap, ok := mySeat.CommanderDamage[i]; ok {
					for _, dmg := range cmdMap {
						if dmg > st.CmdrDmgToUs {
							st.CmdrDmgToUs = dmg
						}
					}
				}
			}
			if st.CmdrDmgToUs >= 15 {
				st.EvalScore += 0.3
			} else if st.CmdrDmgToUs >= 10 {
				st.EvalScore += 0.15
			}
		}

		// Wrath risk: probability opponent is holding a board wipe.
		st.WrathRisk = h.opponentLikelyHasWrath(gs, i)

		threats = append(threats, st)
	}
	h.threatCache = threats
	h.threatCacheTurn = gs.Turn
	return threats
}

// selectAmongTop picks from the top N candidates using the confidence
// threshold. scores must be sorted descending. Returns the selected index.
//
// When the gap between best and second-best is large relative to the
// threshold, the best is chosen deterministically. When scores are close
// (gap < 1 - threshold), we pick randomly among candidates within that
// gap — producing varied, natural-looking play at low thresholds and
// precise play at high thresholds.
func (h *YggdrasilHat) selectAmongTop(scores []float64) int {
	if len(scores) <= 1 {
		return 0
	}
	best := scores[0]

	// How close does a candidate need to be to qualify? The margin
	// shrinks as confidence rises: B1 (0.3) → margin 0.7, B5 (0.9) → margin 0.1.
	margin := 1.0 - h.confidenceThreshold
	if margin < 0.05 {
		margin = 0.05
	}

	// Count how many candidates fall within the margin of the best.
	topN := 1
	for i := 1; i < len(scores); i++ {
		if best-scores[i] <= margin {
			topN = i + 1
		} else {
			break
		}
	}
	if topN == 1 {
		return 0
	}
	if h.noiseRNG == nil {
		return 0
	}
	return h.noiseRNG.Intn(topN)
}

// estimatePWUltimateCost scans a planeswalker's oracle text for its ultimate
// cost. Looks for the largest "−N:" pattern (loyalty cost). Returns 0 if no
// pattern found, meaning we can't estimate.
func estimatePWUltimateCost(card *gameengine.Card) int {
	ot := gameengine.OracleTextLower(card)
	// Scan for patterns like "−7:", "−8:", "-12:" etc.
	// The ultimate is typically the largest negative loyalty ability.
	maxCost := 0
	for i := 0; i < len(ot); i++ {
		// Match '−' (unicode minus U+2212) or '-' (ASCII hyphen).
		isNeg := false
		if ot[i] == '-' {
			isNeg = true
		} else if i+2 < len(ot) && ot[i] == '\xe2' && ot[i+1] == '\x88' && ot[i+2] == '\x92' {
			// UTF-8 for U+2212 MINUS SIGN
			isNeg = true
			i += 2
		}
		if !isNeg {
			continue
		}
		// Parse following digits.
		j := i + 1
		num := 0
		for j < len(ot) && ot[j] >= '0' && ot[j] <= '9' {
			num = num*10 + int(ot[j]-'0')
			j++
		}
		// Must be followed by ':' (loyalty ability pattern).
		if num > 0 && j < len(ot) && ot[j] == ':' && num > maxCost {
			maxCost = num
		}
	}
	return maxCost
}

// noiseSeedFor derives a per-seat noise-RNG seed from the game seed.
// SplitMix64-style avalanche so adjacent (seed, seat) pairs produce
// decorrelated streams — without it, four seats sharing one game seed
// would draw identical noise values at correlated decision points.
func noiseSeedFor(gameSeed int64, seatIdx int) int64 {
	z := uint64(gameSeed) + uint64(seatIdx+1)*0x9E3779B97F4A7C15
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return int64(z ^ (z >> 31))
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

	// Unconditional-evasion hijack: when the attacker has shadow,
	// horsemanship, or is unblockable, blockers are irrelevant — there
	// is no "easier" opponent. Aim it at the biggest threat. Kingmakers
	// outrank raw eval here because runaway leaders are the priority.
	if hasUnconditionalEvasion(attacker) && len(threats) > 1 {
		bestEval := -2.0
		hijackTarget := -1
		bestKingmaker := false
		for _, th := range threats {
			isLegal := false
			for _, d := range legalDefenders {
				if d == th.Seat {
					isLegal = true
					break
				}
			}
			if !isLegal {
				continue
			}
			if th.IsKingmaker && !bestKingmaker {
				bestEval = th.EvalScore
				hijackTarget = th.Seat
				bestKingmaker = true
				continue
			}
			if bestKingmaker && !th.IsKingmaker {
				continue
			}
			if th.EvalScore > bestEval {
				bestEval = th.EvalScore
				hijackTarget = th.Seat
			}
		}
		if hijackTarget >= 0 {
			return hijackTarget
		}
	}

	// Archenemy avoidance: if we have already focused damage on one
	// opponent (>50% of total damage we've dealt) and they aren't a
	// runaway leader, we'd be making ourselves the table's archenemy
	// by piling on. Compute the share once for use as a per-candidate
	// penalty below.
	totalDealt := 0
	for _, d := range h.damageDealtTo {
		if d > 0 {
			totalDealt += d
		}
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
		if attacker != nil && gs.PowerOf(attacker) >= threat.Life && threat.Life > 0 {
			score += 8.0
		}

		// 1b. Commander-damage kill-shot + clock progression
		// (CR §704.6c — 21 commander damage from a single source loses
		// the game regardless of life). When OUR attacker is our own
		// commander, the existing CmdrDmg clock + this swing's damage
		// is the relevant kill condition, not target.Life. Pre-r60 the
		// picker was blind to this and would aim our 5-power commander
		// at the lowest-life opponent even when a 30-life target with
		// 17 cmdr damage from us would die to a redirect — throwing
		// away one of Voltron's primary win lines. Mirrors the block-
		// side check in AssignBlockers so the two sides of combat
		// reason about the same clock.
		//
		// Two bonuses:
		//   - Kill-shot (clock + swing >= 21): +8, matching the life
		//     kill-shot magnitude.
		//   - Closing-clock graded (clock > 0, not lethal): +clock/7,
		//     so investment we've already made compounds into focus
		//     on the same target. At clock=14 (one big swing from
		//     lethal) → +2.0; at clock=7 → +1.0; at clock=20 → +2.86
		//     (still below the +8 kill-shot bump so adding 1 more
		//     point of damage flips us into the kill-shot tier
		//     instead). Capped implicitly at clock=21.
		//
		// Honors double strike on the swing projection — DS doubles
		// the damage delivered in one attack and so doubles the clock
		// advance.
		if attacker != nil && attacker.Card != nil &&
			gameengine.IsCommanderCard(gs, attacker.Controller, attacker.Card) &&
			def >= 0 && def < len(gs.Seats) && gs.Seats[def] != nil &&
			gs.Seats[def].CommanderDamage != nil {
			if byDealer, ok := gs.Seats[def].CommanderDamage[attacker.Controller]; ok {
				clock := byDealer[attacker.Card.DisplayName()]
				if clock > 0 {
					swing := gs.PowerOf(attacker)
					if attacker.HasKeyword("double strike") || attacker.HasKeyword("double_strike") {
						swing *= 2
					}
					if swing < 0 {
						swing = 0
					}
					if clock+swing >= 21 {
						score += 8.0
					} else {
						score += float64(clock) / 7.0
					}
				}
			}
		}

		// 2. Scaled low-life bonus — linear ramp as life drops below 20.
		// At 20 life: +0. At 10 life: +1.5. At 1 life: +3.0.
		if threat.Life < 20 {
			score += 3.0 * (1.0 - float64(threat.Life)/20.0)
		}
		// 2a. Step focus-fire — finishing-off bonus once a player is in
		// closing range. The linear ramp keeps pressure scaling smoothly
		// across the 19→1 range; these step bumps capture the qualitative
		// shift at the "could die this turn" thresholds (10 = ~one big
		// swing, 5 = lethal to almost any combat phase).
		if threat.Life < 10 {
			score += 2.0
		}
		if threat.Life < 5 {
			score += 4.0
		}

		// 3. Target the leader (highest eval score).
		leaderWeight := 2.0
		if focusFire {
			leaderWeight = 3.5
		}
		// DNA ThreatParanoia: high paranoia inflates the leader-eval
		// term so high-eval seats become bigger targets. Low paranoia
		// flattens the term so the hat distributes damage more evenly.
		// Max swing ±50% on leaderWeight at the extremes.
		paranoiaShift := (h.curseAxis(func(d *CurseDNA) float64 { return d.ThreatParanoia }, 0.5) - 0.5) * 1.0
		leaderWeight *= 1.0 + paranoiaShift
		score += threat.EvalScore * leaderWeight

		// 4. Prefer open defenders (fewer untapped blockers). Bumped
		// when the attacker has evasion the defender can't answer
		// (already detected by isOpenForAttacker — flying without reach,
		// menace into a single blocker, etc.).
		// Flying lane: when the attacker has flying and this defender
		// has no flying or reach blockers, double the bonus to +3.0.
		// Open sky against a vulnerable seat is the strongest signal
		// for "swing here" the targeting layer has.
		if attacker != nil && isOpenForAttacker(gs, attacker, gs.Seats[def]) {
			openBonus := 1.5
			if attacker.HasKeyword("flying") && !seatHasReachOrFlyingBlocker(gs.Seats[def]) {
				openBonus = 3.0
			}
			score += openBonus
		}

		// 4a. Protection lane: even if the defender has untapped creatures,
		// prefer them as a target when none of those creatures can legally
		// block this attacker (protection-from-color, flying-without-reach,
		// landwalk, etc.). Engine's CanBlockGS encodes the full ruleset.
		if attacker != nil && noLegalBlockerOnSeat(gs, attacker, gs.Seats[def]) {
			score += 1.5
		}

		// 4b. Evasion match: graded bonus for defenders whose blocker
		// pool can only partially block our attacker (e.g. flyer into a
		// pod with one flier-blocker, skulk into all-large blockers).
		// Rewards aiming evasive creatures at low-defense pods even
		// when some blockers still exist. Stacks with 4 / 4a — those
		// are binary "fully clear" signals, this is a graded fallback.
		if attacker != nil {
			es := evasionScore(gs, attacker, gs.Seats[def])
			if es > 0.5 {
				score += (es - 0.5) * 2.0
			}
		}

		// 4c. Archenemy avoidance: if we've already dealt >50% of our
		// total damage to this seat AND they aren't a kingmaker, ease
		// off — continued focus paints us as the archenemy. The
		// existing spread-damage penalty (#7) only triggers when behind;
		// this one runs unconditionally so a winning hat doesn't make
		// itself the table's target either.
		if totalDealt > 30 && def < len(h.damageDealtTo) && !threat.IsKingmaker {
			share := float64(h.damageDealtTo[def]) / float64(totalDealt)
			if share > 0.5 {
				penalty := (share - 0.5) * 3.0 // up to -1.5 at share=1.0
				if penalty > 1.5 {
					penalty = 1.5
				}
				score -= penalty
			}
		}

		// 4b. Damage-swing keyword targeting:
		//   - Lifelink → finish off low-life opponents (each point heals
		//     us AND removes them, so the closer they are to 0 the bigger
		//     the win-condition swing).
		//   - Infect/Toxic/Poisonous → pile on opponents already deep in
		//     poison (10 = lethal, no life-gain mitigation).
		//   - Annihilator → strip the opponent with the most permanents
		//     (highest expected value per attack trigger).
		if attacker != nil && def >= 0 && def < len(gs.Seats) && gs.Seats[def] != nil {
			d := gs.Seats[def]
			if attacker.HasKeyword("lifelink") && d.Life > 0 && d.Life < 20 {
				score += 1.5 * (1.0 - float64(d.Life)/20.0)
			}
			if attacker.HasKeyword("infect") || attacker.HasKeyword("toxic") || attacker.HasKeyword("poisonous") {
				if d.PoisonCounters > 0 {
					score += float64(d.PoisonCounters) * 0.4
				}
				// First infect hit anywhere is also worth biasing.
				score += 0.5
			}
			if n := gameengine.GetAnnihilatorN(attacker); n > 0 {
				perms := 0
				for _, p := range d.Battlefield {
					if p != nil {
						perms++
					}
				}
				score += float64(n) * float64(perms) * 0.05
			}

			// 4c. First/double-strike blocker penalty. A defender's
			// untapped first-striker can kill our attacker before we
			// get to deal damage — effectively a much bigger blocker
			// from the swing's POV. Skip when our attacker also has
			// first/double strike (they trade in the first-strike step
			// together) or when evasion lets us bypass ground blockers.
			atkFS := attacker.HasKeyword("first strike") ||
				attacker.HasKeyword("first_strike") ||
				attacker.HasKeyword("double strike") ||
				attacker.HasKeyword("double_strike")
			if !atkFS {
				atkPow := gs.PowerOf(attacker)
				atkTou := gs.ToughnessOf(attacker) - attacker.MarkedDamage
				atkFlying := attacker.HasKeyword("flying")
				atkUnblockable := attacker.HasKeyword("unblockable")
				dangerousFS := 0
				for _, b := range d.Battlefield {
					if b == nil || !b.IsCreature() || b.Tapped {
						continue
					}
					bFS := b.HasKeyword("first strike") || b.HasKeyword("first_strike") ||
						b.HasKeyword("double strike") || b.HasKeyword("double_strike")
					if !bFS {
						continue
					}
					if atkFlying && !b.HasKeyword("flying") && !b.HasKeyword("reach") {
						continue
					}
					if atkUnblockable {
						continue
					}
					bPow := gs.PowerOf(b)
					bDT := b.HasKeyword("deathtouch")
					kills := bPow >= atkTou || (bDT && bPow >= 1)
					if attacker.HasKeyword("indestructible") && !bDT {
						kills = false
					}
					if kills {
						dangerousFS++
					}
				}
				if dangerousFS > 0 {
					// Calibration: 0.4 per power of swing we'd lose,
					// floored at 0.5 and capped at 1.5 per blocker —
					// sized to sit between the open-defender bonus
					// (+1.5) and the threat-eval term (×2-3.5). Total
					// penalty capped at -3.0 so a wall of FS blockers
					// can't fully veto a target that's otherwise lethal.
					per := 0.4 * float64(atkPow)
					if per > 1.5 {
						per = 1.5
					}
					if per < 0.5 {
						per = 0.5
					}
					penalty := per * float64(dangerousFS)
					if penalty > 3.0 {
						penalty = 3.0
					}
					score -= penalty
				}
			}
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
		// DNA PoliticalMemory: high memory → grudges persist longer (shorter
		// peace window before détente kicks in, and slower discount growth).
		// Low memory → quick forgiveness (détente kicks in sooner and grows
		// faster). Neutral (0.5) = default 4-turn window, 0.15 per turn.
		if gs != nil && def < len(h.lastAttackedUsTurn) && !threat.IsKingmaker {
			lastHit := h.lastAttackedUsTurn[def]
			peaceTurns := gs.Turn - lastHit
			if lastHit == 0 {
				peaceTurns = gs.Turn
			}
			peaceThreshold := 4
			discountRate := 0.15
			if h.DNA != nil {
				// High memory (1.0) → threshold 6, rate 0.08 (slow to forgive)
				// Low memory (0.0) → threshold 2, rate 0.25 (quick to forgive)
				memShift := (h.DNA.PoliticalMemory - 0.5) * 2.0 // [-1.0, +1.0]
				peaceThreshold = 4 + int(memShift*2.0)
				if peaceThreshold < 1 {
					peaceThreshold = 1
				}
				discountRate = 0.15 - memShift*0.07 // [0.08, 0.22]
				if discountRate < 0.05 {
					discountRate = 0.05
				}
			}
			if peaceTurns >= peaceThreshold {
				discount := float64(peaceTurns-peaceThreshold+1) * discountRate
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

		// 12b. Opponent-archetype targeting bias — bumps tied to the
		// rolling OpponentProfile classifier. R60r5 — bias is now
		// scaled by archetypeBiasMultiplier so high-confidence reads
		// (tutored 3 turns in a row → 0.90+) commit harder than
		// moderate-confidence reads, instead of the prior linear
		// pass-through that treated 0.45 and 0.90 as scaling-equivalent.
		//
		// Combo seats get pressured (disrupt their setup); control
		// seats get a small avoidance penalty (they have removal aimed
		// at attackers); aggro seats get pressured too at high
		// confidence (race the racer — closing their clock first beats
		// trading with their second wave).
		if prof := h.classifyOpponent(gs, def); prof != nil {
			// R60 round 5 — `effectiveArchetypeBias` folds in
			// MetaConfidence so a thin-sample / contradiction-laden
			// classification dampens the bias even at moderate raw
			// confidence. The bias-threshold gates (>0, >0.75) stay
			// the same; the value flowing through them is just
			// meta-corrected.
			mult := effectiveArchetypeBias(prof)
			if mult > 0 {
				switch prof.Archetype {
				case "combo":
					score += mult * 1.5
				case "control":
					score -= mult * 0.6
				case "aggro":
					// Only commits at high confidence (mult > 0.75) —
					// at moderate confidence the existing momentum /
					// kingmaker scoring is doing the work and we
					// shouldn't double-count.
					if mult > 0.75 {
						score += (mult - 0.75) * 1.0
					}
				}
			}
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
			if gs.PowerOf(attacker) < threat.Life {
				score -= 1.0
			}
		}

		candidates = append(candidates, candidate{def, h.applyNoise(score)})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Confidence threshold: at low brackets, pick randomly among
	// similarly-scored targets for less predictable attack patterns.
	tgtScores := make([]float64, len(candidates))
	for i, c := range candidates {
		tgtScores[i] = c.score
	}
	pick := h.selectAmongTop(tgtScores)

	// Persist the candidate pool — top 3 seats with scores — so the
	// analyzer can see *why* a target was chosen over the alternatives
	// (kingmaker bias, threat sniffing, evasion hijack all surface in
	// the score deltas). Keeping it to 3 since 4-player Commander caps
	// the pool there; if seats < 3 we just log whatever was there.
	topN := 3
	if len(candidates) < topN {
		topN = len(candidates)
	}
	topSeats := make([]int, topN)
	topScores := make([]float64, topN)
	for i := 0; i < topN; i++ {
		topSeats[i] = candidates[i].seat
		topScores[i] = candidates[i].score
	}
	h.emitDecisionEvent(gs, seatIdx, "attack_target", map[string]interface{}{
		"chosen_seat":  candidates[pick].seat,
		"chosen_score": candidates[pick].score,
		"top_seats":    topSeats,
		"top_scores":   topScores,
		"candidate_n":  len(candidates),
	})
	h.logf("ATTACK_TARGET seat=%d -> def=%d score=%.3f (top: %v scores=%v)",
		seatIdx, candidates[pick].seat, candidates[pick].score, topSeats, topScores)
	return candidates[pick].seat
}

// -- Combat keyword awareness helpers --

// hasAnyProtection reports whether the permanent has a "prot:X" runtime
// flag (color or universal). Used as a quick "this attacker dodges some
// removal/blockers" signal in attack valuation.
func hasAnyProtection(p *gameengine.Permanent) bool {
	if p == nil || p.Flags == nil {
		return false
	}
	for k := range p.Flags {
		if strings.HasPrefix(k, "prot:") {
			return true
		}
	}
	return false
}

// noLegalBlockerOnSeat returns true when the defender has at least one
// untapped creature, but none of them can legally block the attacker
// (engine CanBlockGS encodes flying/reach, protection, landwalk, evasion,
// etc.). Distinguished from isOpenForAttacker by accepting the legality
// side only — leaves size-based judgments to other heuristics.
func noLegalBlockerOnSeat(gs *gameengine.GameState, attacker *gameengine.Permanent, defender *gameengine.Seat) bool {
	if attacker == nil || defender == nil {
		return false
	}
	any := false
	for _, b := range defender.Battlefield {
		if b == nil || !b.IsCreature() || b.Tapped {
			continue
		}
		any = true
		if gameengine.CanBlockGS(gs, attacker, b) {
			return false
		}
	}
	// "no untapped blockers at all" is handled by isOpenForAttacker; only
	// claim a protection lane when blockers exist but are all illegal.
	return any
}

// anyOpponentHasReachOrFlyingBlocker reports whether any opponent of
// isCombatFirstArchetype returns true for archetypes whose plan IS
// early-game damage: Aggro, Burn, Voltron, Tribal. These archetypes
// should not pay the uniform PhaseDeploy +0.15 conservative bump on
// attack threshold — holding back a turn-2 1/1 because "we should
// develop more board" is the exact early-game-too-defensive bug the
// R60 round 5 aggression audit surfaced. nil-safe.
func isCombatFirstArchetype(sp *StrategyProfile) bool {
	if sp == nil {
		return false
	}
	switch sp.Archetype {
	case ArchetypeAggro, ArchetypeBurn, ArchetypeVoltron, ArchetypeTribal:
		return true
	}
	return false
}

// anyOpponentHasOpenLane reports whether at least one opponent has
// ZERO untapped creatures available as potential blockers. When true,
// every attacker (even a vanilla 1/1) gets a "damage is free" bonus
// — chip damage into an undefended seat adds up, and refusing to
// swing on an open table is exactly the defensive failure mode aggro
// audits flagged. Skips dead / left-game seats. The check is
// pessimistic: a tapped creature is treated as no blocker even though
// it may untap before combat resumes; that matches what the swinger
// actually faces in this combat step.
func anyOpponentHasOpenLane(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		untappedBlockers := 0
		for _, b := range s.Battlefield {
			if b == nil || !b.IsCreature() || b.Tapped {
				continue
			}
			if gs.PowerOf(b) <= 0 && gs.ToughnessOf(b) <= 0 {
				// 0/0 creatures are SBA fodder, not blockers.
				continue
			}
			untappedBlockers++
		}
		if untappedBlockers == 0 {
			return true
		}
	}
	return false
}

// seatIdx controls an untapped creature with reach or flying. Used to
// downgrade flying's evasion bonus when the lane isn't actually open.
func anyOpponentHasReachOrFlyingBlocker(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		if seatHasReachOrFlyingBlocker(s) {
			return true
		}
	}
	return false
}

// seatHasReachOrFlyingBlocker reports whether the seat has at least one
// untapped creature with reach or flying. Per-seat granularity; used by
// bestTarget to scale the open-lane bonus when a flying attacker has a
// truly clear sky against this specific defender.
func seatHasReachOrFlyingBlocker(seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	for _, b := range seat.Battlefield {
		if b == nil || !b.IsCreature() || b.Tapped {
			continue
		}
		if b.HasKeyword("reach") || b.HasKeyword("flying") {
			return true
		}
	}
	return false
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

	var v float64
	if h.MicroNet != nil {
		detailed := h.Evaluator.EvaluateDetailed(gs, seatIdx)
		v = detailed.Score
		microInput := EncodeMicroInput(gs, seatIdx, detailed, h.planState.Current)
		mv := h.MicroNet.Forward(microInput)
		if h.NeuralEval != nil {
			nv := h.NeuralEval.Evaluate(EncodeState(gs, seatIdx))
			v = v*0.6 + mv*0.2 + nv*0.2
		} else {
			v = v*0.7 + mv*0.3
		}
	} else {
		v = h.Evaluator.Evaluate(gs, seatIdx)
		if h.NeuralEval != nil {
			nv := h.NeuralEval.Evaluate(EncodeState(gs, seatIdx))
			v = v*0.8 + nv*0.2
		}
	}

	h.evalCache[key] = v
	return v
}

func (h *YggdrasilHat) evalDetailed(gs *gameengine.GameState, seatIdx int) EvalResult {
	r := h.Evaluator.EvaluateDetailed(gs, seatIdx)
	h.accumulateDims(r)
	return r
}

func (h *YggdrasilHat) accumulateDims(r EvalResult) {
	arr := r.AsArray()
	h.dimCount++
	for d := 0; d < NumDimensions; d++ {
		h.dimAccum[d] += arr[d]
	}
}

func (h *YggdrasilHat) DimensionMeans() [NumDimensions]float64 {
	var means [NumDimensions]float64
	if h.dimCount == 0 {
		return means
	}
	n := float64(h.dimCount)
	for d := 0; d < NumDimensions; d++ {
		means[d] = h.dimAccum[d] / n
	}
	return means
}

// effectiveBudget returns the budget to use for this decision, degrading
// to heuristic on complex boards or when the per-turn budget is exhausted.
// High-stakes turns (combo assembly, low-life opponents, deep stacks)
// bypass the complexity degrade — the whole point of the budget is to
// spend it on decisions that matter, and "60+ permanents on the board"
// IS a decision that matters when someone is about to die.
func (h *YggdrasilHat) effectiveBudget(gs *gameengine.GameState) int {
	if h.Budget == 0 {
		return 0
	}
	highStakes := h.isHighStakesDecision(gs)

	// r61 PR-8 combo carve-out, made explicit in the real gate. The
	// advisory classifyDecision already keeps full compute on a combo-
	// assembly/execution turn via isHighStakesDecision → comboAssembling,
	// but the live budget gate previously only got the carve-out implicitly
	// through `highStakes`. Naming it here guarantees the hat NEVER degrades
	// to a reduced budget on a turn it is assembling or executing its own
	// combo — the most decision-sensitive turn in the game. comboAssembling
	// reports Executable (resolves THIS turn) or Assembling (one tutor away),
	// matching the comboPriority condition classifyDecision keys on.
	comboTurn := h.comboAssembling(gs)
	if comboTurn {
		highStakes = true
	}

	total := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		total += len(s.Battlefield)
	}

	// r61 PR-8 graceful degradation. Pre-r61 this was a hard `return 0`
	// cliff: the instant the table crossed adaptiveComplexityThreshold
	// (~60 permanents — a routine turn-8+ 4-player Commander board) the
	// hat went brain-dead (pure heuristic, no evaluator, no rollout)
	// exactly when decisions are hardest. Now we taper the budget down as
	// complexity rises (applied last, below) so the hat keeps thinking
	// SOME, with a sensible floor. High-stakes turns (combo / low life /
	// deep stack / pressure-bridge) still bypass the degrade entirely.
	thresh := adaptiveComplexityThreshold(gs)
	degraded := total >= thresh && !highStakes

	if h.TurnBudget > 0 && h.turnRemaining(gs) <= 0 && !highStakes {
		return 0
	}

	// r60-cedh-planstate: budget lift when actively assembling/executing
	// a combo. PR #826's null result diagnosed that the cast-order
	// priors steer MCTS toward combo branches but the default
	// h.Budget=50 can't reach terminal wincon visibility on those
	// branches — search prunes before convergence to a win state. The
	// lift trades wall-clock for actually finding the wincon when the
	// plan says we should be looking for it. PlanAssemble (one or more
	// missing pieces, tutors in hand) gets +50%; PlanExecute (combo
	// resolves THIS turn, no missing pieces) gets +100% because the
	// stakes are highest and the branch we want is the literal cast-
	// resolve-win sequence. PlanDevelop / Disrupt / Pivot / Defend are
	// unchanged. Lift is applied last so it stacks cleanly on the
	// high-stakes complexity bypass above (a combo-assembly turn on a
	// 60+ permanent board now gets BOTH the complexity bypass AND the
	// budget lift).
	budget := h.Budget
	switch h.planState.Current {
	case PlanAssemble:
		budget = budget * 3 / 2
	case PlanExecute:
		budget = budget * 2
	}

	// R60: graduated game-state pressure lift on the non-high-stakes
	// path. With all 8 hat eval dimensions r60-tuned and Freya
	// integration solid, mid-pressure states (turn 8+ life 15 vs an
	// opp with 12 board power — none of the binary high-stakes
	// signals fire, but the position is real) deserve more compute
	// than the flat baseline Budget. Skipped on the high-stakes path
	// because that path already returns the unmodified Budget (the
	// existing test contract pins exact == Budget values); the lift
	// graduates the MIDDLE ground between "low pressure: base Budget"
	// and "high stakes: bypass degrade, full Budget." Up to +50% at
	// saturated pressure (combined turn + low-life + opp-board signals).
	if !highStakes {
		if pressure := gameStatePressure(gs); pressure > 0 {
			budget = int(float64(budget) * (1.0 + pressure*0.5))
		}
	}

	// r61 PR-8 graceful complexity taper (applied last). When the board is
	// past the complexity threshold on a non-high-stakes turn, scale the
	// budget down linearly with the permanent overage instead of cliffing
	// to 0. The floor factor keeps a minimum fraction of base budget, and
	// the absolute floor guarantees a non-zero result so the hat always
	// does SOME evaluation. For a base rollout budget (>=200) the taper
	// drops the effective budget below rolloutBudgetGe long before the
	// floor, so rollouts cut out first and only the cheaper evaluator runs
	// on huge boards — keeping per-decision cost bounded.
	if degraded {
		over := total - thresh
		factor := 1.0 - float64(over)*degradedBudgetTaperPerPerm
		if factor < degradedBudgetFloorFactor {
			factor = degradedBudgetFloorFactor
		}
		budget = int(float64(budget) * factor)
		if budget < degradedBudgetAbsoluteFloor {
			budget = degradedBudgetAbsoluteFloor
		}
	}
	return budget
}

// adaptiveComplexityThreshold returns the per-turn-scaled board
// complexity threshold above which the budget degrades to heuristic.
// Pre-r60 a flat 60 was applied regardless of turn; late-game boards
// naturally grow past 60 (multiple seats with 15+ permanents each),
// so the static threshold over-degraded T15+ states where decisions
// genuinely matter. Scales +3 permanents per turn past 10 (so T15
// reads as 75, T20 reads as 90). Capped to prevent the threshold
// from sliding past observable game-state complexity ceilings.
func adaptiveComplexityThreshold(gs *gameengine.GameState) int {
	threshold := adaptiveBudgetComplexityThreshold
	if gs == nil || gs.Turn <= 10 {
		return threshold
	}
	scaled := threshold + (gs.Turn-10)*3
	if scaled > 120 {
		scaled = 120
	}
	return scaled
}

// gameStatePressure returns a 0.0..1.0 estimate of overall game-state
// pressure. Three additive components, each capped at ~0.35 so any
// single saturated signal contributes meaningfully but no single
// signal can saturate the total alone:
//
//   - Turn pressure: late game = compounding stakes. 0 below turn 6,
//     ramps linearly to 0.35 at turn 12 (game-defining mid-late
//     window — combos online, mana plentiful, threats deployed).
//   - Life pressure: minimum live-seat life ratio inverted. Any
//     seat at 20/40 contributes ~0.18; any seat at 4/40 contributes
//     ~0.32. The "any seat" framing matches isHighStakesDecision's
//     existing rationale — whether they're about to die or we are,
//     the next decisions are game-deciding.
//   - Board pressure: max effective board power across live seats,
//     normalized so 20 power ≈ 0.30, capped at 0.30. A large board
//     on the table compresses the answer-or-die window for whoever
//     it's pointed at.
//
// Used by effectiveBudget's non-high-stakes lift and by
// isHighStakesDecision's pressure-bridge (pressure >= 0.6 → high
// stakes even when no individual binary signal fires).
func gameStatePressure(gs *gameengine.GameState) float64 {
	if gs == nil {
		return 0
	}
	pressure := 0.0
	switch {
	case gs.Turn >= 12:
		pressure += 0.35
	case gs.Turn >= 6:
		pressure += 0.05 * float64(gs.Turn-6)
	}
	minLifeRatio := 1.0
	for _, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		starting := float64(s.StartingLife)
		if starting <= 0 {
			starting = 40
		}
		r := float64(s.Life) / starting
		if r < 0 {
			r = 0
		}
		if r < minLifeRatio {
			minLifeRatio = r
		}
	}
	pressure += (1.0 - minLifeRatio) * 0.35
	var maxBoardPow float64
	for _, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		bp := float64(effectiveBoardPower(gs, s))
		if bp > maxBoardPow {
			maxBoardPow = bp
		}
	}
	boardPressure := maxBoardPow / 70.0
	if boardPressure > 0.30 {
		boardPressure = 0.30
	}
	pressure += boardPressure
	if pressure > 1.0 {
		pressure = 1.0
	}
	return pressure
}

// isHighStakesDecision returns true when the current game state is one
// where we want to spend evaluator budget regardless of board complexity:
//
//   - Combo is one piece away or executable now (existing comboPriority
//     signal — preserve game-winning lines through the degrade).
//   - Any seat is at low life (≤ highStakesLifeThreshold). Whether
//     they're about to die or we are, the wrong decision here ends the
//     game; the right one wins it.
//   - Stack has ≥ highStakesStackDepth items. Deep stacks mean active
//     counter wars / triggered-ability chains where each priority
//     window's decision changes the resolution order — the kind of
//     position where evaluator-guided play meaningfully outscores the
//     fast-path heuristic.
func (h *YggdrasilHat) isHighStakesDecision(gs *gameengine.GameState) bool {
	if h == nil || gs == nil {
		return false
	}
	if h.comboAssembling(gs) {
		return true
	}
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Life <= highStakesLifeThreshold {
			return true
		}
	}
	if len(gs.Stack) >= highStakesStackDepth {
		return true
	}
	// R60: pressure bridge. When combined game-state pressure (turn +
	// life + board) crosses 0.6 without any individual binary signal
	// firing, the position is genuinely high-stakes — a turn-12 board
	// with one opp at 14 life and 12 board power is the canonical case
	// (turn 0.30 + life 0.23 + board 0.17 = 0.70). Pre-r60 such a
	// state degraded silently to heuristic on complex boards because
	// no single signal tripped. The 0.6 floor is calibrated to NOT
	// trigger on baseline-stacked states (e.g., life 9 alone gives
	// 0.27 — below the floor).
	if gameStatePressure(gs) >= 0.6 {
		return true
	}
	return false
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

// classifyDecision predicts which DecisionTier a decision at the current
// game state will use. It is purely advisory — the existing decision
// pipeline still makes the final budget choice. Logic mirrors
// effectiveBudget plus the rollout precondition in ChooseCastFromHand.
//
// Inputs considered:
//   - h.Budget: hard ceiling on tier (Budget=0 → Mjolnir only).
//   - board complexity: total permanents across all seats.
//   - per-turn budget: exhaustion forces Mjolnir.
//   - combo assembly: when a combo is one piece away or executable,
//     the routing prefers more compute and ignores the complexity
//     degrade so we don't punt on the winning turn.
func (h *YggdrasilHat) classifyDecision(gs *gameengine.GameState) DecisionTier {
	if h == nil || h.Budget <= 0 {
		return TierMjolnir
	}

	// R60: superset of the previous combo-only override — also lets
	// low-life and deep-stack windows keep evaluator/rollout compute.
	// Matches the gate inside effectiveBudget so the tier report no
	// longer drifts from the actual compute path.
	highStakes := h.isHighStakesDecision(gs)

	total := 0
	if gs != nil {
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			total += len(s.Battlefield)
		}
	}
	// r61 PR-8: the complexity branch no longer cliffs to Mjolnir (pure
	// heuristic). effectiveBudget now tapers to a degraded-but-non-zero
	// evaluator budget on complex non-high-stakes boards, so the tier
	// report degrades to Gungnir (evaluator-guided) here instead of
	// drifting to Mjolnir. Rollout (Ragnarok) is intentionally not offered
	// on a degraded board: the budget taper drops effective budget below
	// rolloutBudgetGe well before its floor, so rollouts cut out first.
	if total >= adaptiveComplexityThreshold(gs) && !highStakes {
		return TierGungnir
	}
	if h.TurnBudget > 0 && h.turnRemaining(gs) <= 0 && !highStakes {
		return TierMjolnir
	}

	canRagnarok := h.Budget >= rolloutBudgetGe && h.TurnRunner != nil
	if canRagnarok {
		if h.TurnBudget > 0 && h.turnRemaining(gs) < rolloutEvalCost {
			return TierGungnir
		}
		return TierRagnarok
	}
	return TierGungnir
}

// comboAssembling reports whether the active seat has a combo line that
// is executable now or one tutor away. Used by classifyDecision to bias
// toward more compute on combo-critical turns.
func (h *YggdrasilHat) comboAssembling(gs *gameengine.GameState) bool {
	if h == nil || h.comboSeq == nil || gs == nil {
		return false
	}
	seat := gs.Active
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return false
	}
	a := h.comboSeq.Evaluate(gs, seat)
	return a.Executable || a.Assembling
}

// refreshPlanState re-evaluates the combo sequencer + threat assessment
// and lets PlanState transition mid-turn. Pre-r60-cedh-planstate this
// was only called at upkeep (see the trigger block around line 9590),
// which meant that when a tutor resolves in main phase and the
// Assembling gate flips, the next cast decision THIS turn still ran
// under the old plan. That defeats the cast-order bias on the exact
// turn the reach pattern arrives — a fast-cEDH-critical loss.
//
// Now called from cast-decision entry points (ChooseCastFromHand) so a
// mid-turn state change is visible to the next decision. The refresh
// is cheap: comboSeq.Evaluate is O(plans × pieces) and assessAllThreats
// is the same call the upkeep block already makes. Safe to call when
// comboSeq is nil (no-op) or planState was never seeded.
func (h *YggdrasilHat) refreshPlanState(gs *gameengine.GameState, seatIdx int) {
	if h == nil || gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if h.comboSeq == nil {
		return
	}
	ca := h.comboSeq.Evaluate(gs, seatIdx)
	maxThreat := 0.0
	for _, t := range h.assessAllThreats(gs, seatIdx) {
		if t.EvalScore > maxThreat {
			maxThreat = t.EvalScore
		}
	}
	h.planState.Evaluate(&ca, maxThreat)
	pm := h.planState.PlanWeightMultipliers()
	h.Evaluator.PlanMultiplier = &pm
}

// recordDecisionTier increments the per-tier counter. Safe to call from
// any decision entry point.
func (h *YggdrasilHat) recordDecisionTier(t DecisionTier) {
	if h == nil {
		return
	}
	if t < 0 || int(t) >= numDecisionTiers {
		return
	}
	h.tierCounts[t]++
}

// recordParentTier wraps recordDecisionTier for the PARENT decision
// entry points (ChooseCastFromHand, ChooseActivation, ChooseAttackers,
// ChooseResponse). In addition to bumping tierCounts, it stamps
// `lastParentTier` so cascading downstream decisions (ChooseTarget,
// ChooseMode, ChooseX, ChooseSacrifice, OrderTriggers, ...) can read
// it and escalate their own evaluation.
//
// `turn` is the current game turn — used to invalidate the parent
// tier when a new turn starts (priority windows don't span turns,
// and stale Ragnarok context from last turn would over-escalate
// downstream decisions on the new turn).
func (h *YggdrasilHat) recordParentTier(t DecisionTier, turn int) {
	if h == nil {
		return
	}
	h.recordDecisionTier(t)
	h.lastParentTier = t
	h.lastParentTierTurn = turn
}

// recordResponseTier is the ChooseResponse-specific variant of
// recordParentTier. It stamps lastParentTier (legacy compat) AND
// records the tier keyed by the stack item's ID in stackItemTiers
// so that downstream cascade decisions firing during THIS specific
// item's resolution can recover the correct tier even after a
// LATER ChooseResponse call has overwritten lastParentTier.
//
// R60 round 5 — multi-instance priority window audit. Pre-fix the
// sequence:
//
//  1. Trigger A pushed. ChooseResponse → lastParentTier = T_A.
//  2. Hat declines to counter A.
//  3. Trigger B pushed (before A resolves). ChooseResponse →
//     lastParentTier = T_B (overwrites T_A).
//  4. B resolves, its cascades read T_B. Correct.
//  5. A resolves, its cascades read T_B. STALE — should be T_A.
//
// stackItemTiers gives us a per-item record so step 5's cascade
// can lookup T_A by stack item ID instead of falling back to the
// overwritten lastParentTier. Map is turn-scoped via
// stackItemTiersTurn — entries from prior turns are cleared
// lazily on the first stamp of a new turn.
func (h *YggdrasilHat) recordResponseTier(t DecisionTier, turn int, top *gameengine.StackItem) {
	if h == nil {
		return
	}
	h.recordParentTier(t, turn)
	if top == nil || top.ID == 0 {
		return
	}
	if h.stackItemTiers == nil || h.stackItemTiersTurn != turn {
		h.stackItemTiers = make(map[int]DecisionTier, 4)
		h.stackItemTiersTurn = turn
	}
	h.stackItemTiers[top.ID] = t
}

// tierForStackItem returns the DecisionTier that was stamped for a
// stack item via recordResponseTier this turn. Returns
// (TierMjolnir, false) if no stamp exists.
func (h *YggdrasilHat) tierForStackItem(turn, stackID int) (DecisionTier, bool) {
	if h == nil || h.stackItemTiers == nil || h.stackItemTiersTurn != turn {
		return TierMjolnir, false
	}
	t, ok := h.stackItemTiers[stackID]
	return t, ok
}

// recordCascadeDecision is the downstream-method analogue of
// recordDecisionTier: bumps the tier counter to reflect the cascade
// (using the parent's tier when still in the same turn, else
// defaulting to TierMjolnir). Keeps MjolnirStats accurate when the
// real work happens in downstream Choose* — without this, a
// Ragnarok cast followed by 3 ChooseTarget calls under-counts
// Ragnarok 3-to-1.
func (h *YggdrasilHat) recordCascadeDecision(gs *gameengine.GameState) DecisionTier {
	if h == nil {
		return TierMjolnir
	}
	t := TierMjolnir
	// R60r5 — prefer the per-stack-item tier when the current stack top
	// (the item closest to resolving / triggering this cascade) has a
	// stamped tier. This avoids the multi-instance priority window bug
	// where consecutive ChooseResponse calls overwrite lastParentTier
	// and an earlier item's cascade reads the latest item's tier.
	if gs != nil && len(gs.Stack) > 0 {
		topID := gs.Stack[len(gs.Stack)-1].ID
		if stamped, ok := h.tierForStackItem(gs.Turn, topID); ok {
			t = stamped
			h.recordDecisionTier(t)
			return t
		}
	}
	if gs != nil && h.lastParentTierTurn == gs.Turn {
		t = h.lastParentTier
	}
	h.recordDecisionTier(t)
	return t
}

// parentTierIs reports whether the most recently classified parent
// decision (within the current turn) is at-or-above `min`. Lets
// cascading decisions gate "do extra work" branches without having
// to inspect tierCounts directly.
func (h *YggdrasilHat) parentTierIs(gs *gameengine.GameState, min DecisionTier) bool {
	if h == nil || gs == nil {
		return false
	}
	if h.lastParentTierTurn != gs.Turn {
		return false
	}
	return h.lastParentTier >= min
}

// MjolnirStats returns the per-tier decision distribution for this hat
// since the last reset (or hat construction).
func (h *YggdrasilHat) MjolnirStats() MjolnirStats {
	if h == nil {
		return MjolnirStats{}
	}
	return MjolnirStats{
		Mjolnir:  h.tierCounts[TierMjolnir],
		Gungnir:  h.tierCounts[TierGungnir],
		Ragnarok: h.tierCounts[TierRagnarok],
	}
}

// ResetMjolnirStats zeroes the per-tier counters. Tests use this to
// isolate samples; production code can call it at game-start.
func (h *YggdrasilHat) ResetMjolnirStats() {
	if h == nil {
		return
	}
	h.tierCounts = [numDecisionTiers]int{}
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

// politicsThreatAdjustment returns a score adjustment for hitting
// targetSeat with a hostile effect (removal, burn, etc.) given the
// table's threat profile and our relative position. Pure function so
// the politics rules are testable in isolation.
//
// Encodes two table-politics signals:
//
//   - "Hit the leader" — when we're competitive (relPos >= -0.3) AND
//     there's a clear table leader (EvalScore lead >= 3.0 over the
//     runner-up), boost the leader's seat by +3.0. This makes removal
//     and direct damage gravitate to the seat that's actually winning,
//     overriding the existing flashier-card bonuses (combo piece on
//     low-threat seat).
//
//   - "Dodge the kingmaker" — when we're meaningfully behind
//     (relPos < -0.3) AND a clear leader exists, hitting them just
//     speeds our death (they retaliate with their bigger board). The
//     leader is demoted by -3.0 and the runner-up boosted by +2.0;
//     net effect is the kingmaker dodge — burn the contender, not
//     the leader.
//
// Returns 0 when there's no clear leader (gap < 3.0) or fewer than
// two living opponents, so the existing scoring dominates.
//
// `threats` is the assessAllThreats result for the asking seat; it
// already excludes self, lost, and left-game seats.
func politicsThreatAdjustment(threats []seatThreat, relPos float64, targetSeat int) float64 {
	if len(threats) < 2 {
		return 0
	}
	// Find the two highest EvalScore seats.
	leaderIdx, runnerupIdx := -1, -1
	leaderScore, runnerupScore := -1e18, -1e18
	for i, th := range threats {
		switch {
		case th.EvalScore > leaderScore:
			runnerupIdx, runnerupScore = leaderIdx, leaderScore
			leaderIdx, leaderScore = i, th.EvalScore
		case th.EvalScore > runnerupScore:
			runnerupIdx, runnerupScore = i, th.EvalScore
		}
	}
	if leaderIdx < 0 || runnerupIdx < 0 {
		return 0
	}
	// "Clear leader" requires a meaningful EvalScore gap. Below this
	// threshold the existing additive bonuses dominate and politics
	// shouldn't override them.
	const leadGap = 3.0
	if leaderScore-runnerupScore < leadGap {
		return 0
	}
	leaderSeat := threats[leaderIdx].Seat
	runnerupSeat := threats[runnerupIdx].Seat
	if relPos < -0.3 {
		// We're behind — kingmaker dodge.
		if targetSeat == leaderSeat {
			return -3.0
		}
		if targetSeat == runnerupSeat {
			return 2.0
		}
		return 0
	}
	// Competitive — hit the leader.
	if targetSeat == leaderSeat {
		return 3.0
	}
	return 0
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

// cardProducesRealMana returns true when the card genuinely produces mana
// when it resolves — i.e. its AST carries an activated mana ability whose
// AddMana output is a concrete, non-conditional amount (a fixed Pool or a
// positive AnyColorCount). This deliberately excludes the Everflowing-Chalice
// blind-spot family: counter-scaled / multikicker-scaled "rocks" whose printed
// output is zero until kicked or whose mana scales with charge counters (an
// unkicked Chalice taps for nothing). Used to gate the type/text-based
// "mana-rock" and "produces mana" cast bonuses on actual effect data instead
// of merely matching a type line or an "add {" substring.
func cardProducesRealMana(c *gameengine.Card) bool {
	if c == nil || c.AST == nil {
		return false
	}
	// Counter-scaled / multikicker producers print no guaranteed mana on a
	// vanilla cast — reject by oracle text before trusting the AST shape.
	ot := gameengine.OracleTextLower(c)
	if strings.Contains(ot, "multikicker") ||
		strings.Contains(ot, "for each charge counter") ||
		strings.Contains(ot, "for each counter") {
		return false
	}
	for _, ab := range c.AST.Abilities {
		act, ok := ab.(*gameast.Activated)
		if !ok || act.Effect == nil {
			continue
		}
		if am, ok := act.Effect.(*gameast.AddMana); ok {
			if len(am.Pool) > 0 || am.AnyColorCount > 0 {
				return true
			}
			continue
		}
		// Sequence-wrapped mana abilities (cost-then-add shapes).
		if seq, ok := act.Effect.(*gameast.Sequence); ok {
			for _, sub := range seq.Items {
				if am, ok := sub.(*gameast.AddMana); ok {
					if len(am.Pool) > 0 || am.AnyColorCount > 0 {
						return true
					}
				}
			}
		}
	}
	return false
}

// cardHeuristic scores a castable card for the evaluator path.
func (h *YggdrasilHat) cardHeuristic(gs *gameengine.GameState, seatIdx int, c *gameengine.Card) float64 {
	// Early-guard: a downstream block at ~line 2511 re-checks gs != nil
	// (staticcheck SA5011), implying the function may be called with nil
	// gs from at least one caller. Match the established guards used by
	// sibling helpers at lines 833 / 2889 / 2945 / 3080.
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return 0
	}
	base := 0.35
	cmc := gameengine.ManaCostOf(c)
	avail := gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])

	// Mana efficiency: spending more of available mana is better.
	if avail > 0 {
		base += float64(cmc) / float64(avail) * 0.15
	}

	cat := h.categorizeWithFreya(c)

	// Phase-of-game shaping. Different cards matter at different stages:
	//   Deploy  → ramp / mana rocks early; expensive threats are dead weight.
	//   Develop → value engines, draw, removal — out-resource opponents.
	//   Execute → finishers, combo pieces, mass removal — close the game.
	// The shifts are additive on top of the archetype switch below; they
	// don't override archetype identity, just sharpen the curve.
	switch h.detectPhase(gs, seatIdx) {
	case PhaseDeploy:
		switch cat {
		case CatRamp:
			base += 0.20
		case CatThreat:
			if cmc >= 5 {
				base -= 0.10 // top-end is dead in hand on turn 2
			}
		}
		// Mana-rock heuristic: low-CMC artifact that ACTUALLY taps for mana.
		// Gated on real effect output (not just the artifact type line) so a
		// 2-CMC artifact with no mana ability — or an unkicked Everflowing
		// Chalice that taps for nothing — doesn't earn the ramp bonus.
		if typeLineContains(c, "artifact") && cmc <= 2 && cat != CatThreat &&
			cardProducesRealMana(c) {
			base += 0.15
		}
		// Commander-color enabler: any sub-3 spell that produces mana. The
		// mana-producer branch is gated on actual non-conditional output
		// (excludes counter-scaled / multikicker producers and cards whose
		// text merely contains "add {"); the land-fetch ramp branch stands on
		// its own (a fetch is real ramp regardless of mana-ability shape).
		if cmc <= 3 {
			ot := gameengine.OracleTextLower(c)
			fetchesLand := strings.Contains(ot, "search your library for a") &&
				strings.Contains(ot, "land")
			if cardProducesRealMana(c) || fetchesLand {
				base += 0.10
			}
		}
	case PhaseDevelop:
		if h.isValueEngineKey(c) {
			base += 0.15
		}
		switch cat {
		case CatDraw:
			base += 0.10
		case CatRemoval:
			base += 0.10
		}
	case PhaseExecute:
		if h.isFinisher(c) {
			base += 0.25
		}
		if h.isComboRelevant(c) {
			base += 0.20
		}
		// Mass removal / sweeper detection.
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all") ||
			strings.Contains(ot, "each creature") && strings.Contains(ot, "damage") {
			base += 0.15
		}
		// Late ramp is mostly wasted draws.
		if cat == CatRamp {
			base -= 0.10
		}
	}

	// DNA ResourceGreed: high → bias toward card draw + ramp (resource
	// hoarding); low → bias toward CatThreat (board presence). Applied
	// before archetype bonuses so the same nudge stacks coherently with
	// the archetype's own draw/ramp bias for ramp/control decks.
	// Max swing ±0.15 per relevant card category.
	greedShift := (h.curseAxis(func(d *CurseDNA) float64 { return d.ResourceGreed }, 0.5) - 0.5) * 0.3
	switch cat {
	case CatDraw, CatRamp:
		base += greedShift
	case CatThreat:
		base -= greedShift
	}

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
	case ArchetypeEnchantress:
		if typeLineContains(c, "enchantment") || h.valueEngineSet[c.DisplayName()] {
			base += 0.25
		}
		if typeLineContains(c, "aura") {
			base += 0.15
		}
	case ArchetypeArtifacts:
		if typeLineContains(c, "artifact") {
			base += 0.20
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "treasure token") {
			base += 0.15
		}
	}

	// R60 round 7+ defend-vs-aggro signals (see defend_vs_aggro_signals_r60.go).
	// Only fire when projected single-turn opp damage crosses 33% of life;
	// scale by urgency so the bias grows as pressure climbs. Both signals
	// use the SAME urgency value so they pull in the same direction.
	if urgency := h.defenseUrgencyVsAggro(gs, seatIdx); urgency >= 0.33 {
		if h.isDefensiveCard(c) {
			base += 0.20 * urgency
		} else if cmc >= 5 {
			base -= 0.15 * urgency
		}
	}

	// R60 round 14+ ETB-trigger value bonus (see etb_trigger_value_r60.go).
	// Cards whose value lives in their ETB (Mulldrifter, Stoneforge,
	// Reclamation Sage, Eternal Witness) were underweighted because
	// Freya's CatDraw / CatRamp / CatRemoval bonuses are small and the
	// multi-mode / tutor / recursion ETB families don't fit a single
	// Freya category cleanly. Graduated bonus by ETB payoff (tutor +0.30,
	// draw/removal +0.20, ramp +0.15, token/recursion +0.10). Stacks
	// with the existing category bonuses — they're complementary signals.
	base += etbTriggerBonus(c)

	// Lovelace Composer Intent: boost cards matching commander themes.
	if h.Strategy != nil {
		cName := c.DisplayName()
		if h.starCardSet[cName] {
			base += 0.20
		}
		if len(h.Strategy.CommanderThemes) > 0 {
			ot := gameengine.OracleTextLower(c)
			tl := strings.ToLower(c.TypeLine)
			for _, theme := range h.Strategy.CommanderThemes {
				lt := strings.ToLower(theme)
				if strings.Contains(ot, lt) || strings.Contains(tl, lt) {
					base += 0.12
					break
				}
			}
		}
	}

	// Commander Synergy Amplifier — the Lovelace boost above is
	// commander-AGNOSTIC (it fires whether the commander is in play or
	// not). This block shifts scoring based on WHERE the commander is
	// right now: if it's on the battlefield, cards that feed its engine
	// or finish its win line are concretely better; if it's still in
	// the command zone, we want to deploy the commander first and pick
	// protection over generic value while we wait. Skipped when no
	// strategy profile or commander metadata is available.
	if gs != nil && h.Strategy != nil && seatIdx >= 0 && seatIdx < len(gs.Seats) {
		seat := gs.Seats[seatIdx]
		if seat != nil && len(seat.CommanderNames) > 0 {
			cName := c.DisplayName()
			cmdrOnField, cmdrOnFieldPerm := commanderOnBattlefield(seat)
			cmdrInCommandZone := commanderInCommandZone(seat)

			if cmdrOnField {
				if h.valueEngineSet[cName] {
					base += 0.25
				}
				if h.comboPieceSet[cName] {
					base += 0.30
				}
				if len(h.Strategy.CommanderThemes) > 0 {
					ot := gameengine.OracleTextLower(c)
					for _, theme := range h.Strategy.CommanderThemes {
						lt := strings.ToLower(strings.TrimSpace(theme))
						if lt == "" {
							continue
						}
						if strings.Contains(ot, lt) {
							base += 0.15
							break
						}
					}
				}
				if cmdrOnFieldPerm != nil && cmdrOnFieldPerm.Card != nil &&
					typeLineContains(c, "creature") &&
					sharesCreatureSubtype(cmdrOnFieldPerm.Card, c) {
					base += 0.10
				}
			} else if cmdrInCommandZone {
				// Bias toward deploying the commander first: small
				// across-the-board penalty on non-ramp / non-land cards
				// so ramp and lands keep their priority while generic
				// value gets nudged behind the commander.
				if !typeLineContains(c, "land") && cat != CatRamp {
					base -= 0.05
				}
				if isCommanderProtectionCard(c) {
					base += 0.15
				}
			}
		}
	}

	// Partner commander priority: when a partner pair is in the command zone,
	// deploying the second partner is high-value — it unlocks the deck's full
	// identity. Boost if the other partner is already on the battlefield.
	if gs != nil && len(gs.Seats[seatIdx].CommanderNames) == 2 {
		seat := gs.Seats[seatIdx]
		cName := c.DisplayName()
		isCommander := false
		otherPartner := ""
		for _, cn := range seat.CommanderNames {
			if strings.EqualFold(cn, cName) {
				isCommander = true
			} else {
				otherPartner = cn
			}
		}
		if isCommander {
			base += 0.20
			for _, p := range seat.Battlefield {
				if p != nil && p.Card != nil && strings.EqualFold(p.Card.DisplayName(), otherPartner) {
					base += 0.25
					break
				}
			}
		}
	}

	// DFC/MDFC recognition: modal double-faced cards offer flexibility.
	// Score both faces and use the better one. Back-face lands are
	// especially valuable as they're never dead draws.
	if c.IsMDFC() {
		base += 0.10 // inherent flexibility bonus
		// Back face is a land = never a dead card.
		for _, t := range c.BackFaceTypes {
			if t == "land" {
				base += 0.10
				break
			}
		}
		// If the back face has a lower CMC and we're mana-constrained,
		// the flexibility is even more valuable.
		if avail > 0 && c.BackFaceCMC > 0 && c.BackFaceCMC < cmc && c.BackFaceCMC <= avail {
			base += 0.08 // can cast back face when front is too expensive
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

	// R60: WinConPursuit aggressive-tutoring bonus. At high pursuit
	// (>= 0.7 — combo is materially close, mana to deploy is up),
	// boost tutors and combo pieces by an additional pursuit-scaled
	// bonus so the hat picks them over safer plays. Pure additive on
	// top of the comboUrgency / tutorTargetSet bonuses above; no
	// dampening of non-combo cards (would destabilize early-game),
	// relying instead on the relative ranking to push safer plays
	// down naturally.
	if pursuit := h.WinConPursuit(gs, seatIdx); pursuit >= 0.7 {
		ot := gameengine.OracleTextLower(c)
		isTutorCard := strings.Contains(ot, "search your library for") ||
			strings.Contains(strings.ToLower(c.DisplayName()), "tutor")
		if isTutorCard {
			base += pursuit * 0.30 // +0.21 at gate, +0.30 at saturation
		} else if h.isComboRelevant(c) {
			base += pursuit * 0.20
		}
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

	// Synergy cluster cohesion: when a cluster (Freya themed grouping —
	// Tokens / Recursion / +1/+1 Counters / etc.) already has ≥2 members
	// on the seat's battlefield, prioritize playing other members of
	// the same cluster so the engine compounds. HighDensity clusters
	// (Freya's marker for "real subsystem of the gameplan", member
	// count ≥5) boost 2x because they're load-bearing engines, not
	// incidental overlap. Wave-4 freya-hat integration audit
	// (2026-05-30) — see synergyClusterCohesionBoost docstring.
	base += h.synergyClusterCohesionBoost(gs, seatIdx, c)

	// Huginn prediction boost: cards appearing in a Huginn-generated
	// combo prediction (dev-19's --predict output) get a small bonus
	// scaled by the prediction's confidence. Biases the hat toward
	// drawing / casting predicted combo pieces while the prediction is
	// fresh, which produces a stronger predicted-vs-actual fire signal
	// for the post-game feedback loop
	// (huginn.ComputePredictionOutcomes). Worker D — Huginn 2.0 freya
	// integration (2026-05-31) — see huginnPredictionBoost docstring.
	base += huginnPredictionBoost(h.Strategy, c)

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
		// 3rd Eye: Entropy-enhanced caution — if any opponent has the
		// LikelyHasAnswer flag (tutored + held mana + interactive colors),
		// apply extra downweight on combo pieces. They almost certainly
		// have a counterspell or removal spell waiting.
		if isHighValue && h.isComboRelevant(c) {
			for i := range gs.Seats {
				if i == seatIdx {
					continue
				}
				if h.opponentLikelyHasAnswer(i) {
					base -= 0.15
					break
				}
			}
		}
	}

	// Sandbagging: if casting a high-value piece would make us the tallest
	// blade of grass (highest eval at the table), delay to avoid becoming
	// archenemy — unless we can win this turn or it's late enough to commit.
	// Engine archetypes (aristocrats, combo, enchantress, artifacts) get a
	// reduced penalty because deploying engine pieces IS their win condition.
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
				switch arch {
				case ArchetypeAristocrats, ArchetypeCombo, ArchetypeEnchantress, ArchetypeArtifacts:
					penalty *= 0.3
				case ArchetypeSelfmill, ArchetypeReanimator:
					penalty *= 0.5
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

	// R60 Second Main Phase audit — Signal B: precombat main bonus
	// for cards whose value-on-the-table evaporates if they sit in
	// hand through combat (haste creatures, attack-trigger commanders,
	// anthems, equipment). Boost is +0.30 in precombat_main so the
	// cast ranking tilts ahead of generic CatDraw / CatRamp picks;
	// gated off in postcombat_main (too late for this turn's combat).
	// Sized to outweigh the typical CatDraw PhaseDevelop bonus
	// (+0.10) and the mana-efficiency advantage of higher-CMC
	// alternatives so a 1-mana haste creature actually wins over a
	// 3-mana Divination in main1.
	if gs != nil && gs.Phase == "precombat_main" && cardPrefersMain1(c) {
		base += 0.30
	}

	// Political "deal" heuristic — goad-effect cards become MUCH more
	// valuable when an opponent's creature could lethal another opponent
	// if redirected AND is also a threat to us. The hat trades a slot
	// in hand for letting the table eliminate a player on our behalf,
	// keeping our removal in reserve and preserving political capital
	// (we're not the one casting the kill spell). See goadDealOpportunity
	// for the trigger conditions and magnitude (0 to +0.45). Sized so
	// the bonus outweighs the typical -0.10 reactive penalty applied to
	// non-strategic cards under stax / control / combo archetypes — a
	// goad redirect against a winning swing is good in every archetype.
	if cardIsGoad(c) {
		base += h.goadDealOpportunity(gs, seatIdx)
	}

	// r60-cedh-sequencer: combo-priority cast-order bias. PlanAssemble
	// and PlanExecute already boost the ComboProximity *evaluator*
	// weight at the MCTS leaves (see PlanWeightMultipliers in
	// gameplan.go), but the priors that decide WHICH spell to cast
	// first this turn run through cardHeuristic — and that path
	// previously treated combo pieces and value engines as roughly
	// interchangeable. The cEDH gauntlet rerun (docs/cedh-gauntlet-
	// rerun-r60.md, PR #808) confirmed this gap: PR #793's multi-tutor
	// leaf-eval lift fired (5-7× throughput drop proves the MCTS
	// expanded combo subtrees deeper) but didn't change the actual
	// cast-order outcome, so avg game length stayed at 47-50 turns.
	//
	// When the plan is PlanAssemble or PlanExecute, push combo pieces
	// and tutors strictly ahead of generic value engines so the cast
	// queue tries to assemble the wincon before re-investing in mid-
	// board state. Sizes (+0.40 combo / +0.35 tutor / -0.15 value
	// engine) are tuned so a combo piece outranks a same-CMC value
	// engine by ~0.55 — larger than any single archetype/category
	// bonus above, so the bias is decisive when combo decks have a
	// reach line in hand.
	if h.planState.Current == PlanAssemble || h.planState.Current == PlanExecute {
		cName := c.DisplayName()
		switch {
		case h.comboPieceSet[cName]:
			base += 0.40
		case isTutor(c) || strings.Contains(gameengine.OracleTextLower(c), "search your library"):
			// Mirror hasTutorInHand's detection: AST effect-kind OR
			// the canonical "search your library" oracle phrase. The
			// AST-only check misses Static-raw-text tutors (Demonic
			// Tutor parses as Activated with a Tutor effect node when
			// the parser succeeds, but some cards and the test corpus
			// reach cardHeuristic with only Static.Raw populated).
			base += 0.35
		case h.isValueEngineKey(c):
			base -= 0.15
		}
	}

	return base
}

func (h *YggdrasilHat) isComboRelevant(c *gameengine.Card) bool {
	return h.comboPieceSet[c.DisplayName()]
}

// WinConPursuit returns a 0.0..1.0 score quantifying how aggressively
// the hat should pursue win-condition assembly given current game state.
// Built from the same availability map scoreCombo uses (hand=1.0,
// battlefield=1.0, graveyard=0.5 OR 0.9 with recursion engine,
// command zone=0.8), folding in tutor reach in hand and a mana-
// availability bonus when the combo is genuinely close to deploying.
//
// Returns 0 when no Strategy / no combo plans are loaded — the
// signal is undefined for non-combo decks and downstream callers
// must gate on > 0.
//
// Calibration:
//
//	0.0..0.4  — early/no progress; no pursuit signal
//	0.4..0.7  — assembling but not close; cardHeuristic priors don't fire
//	>= 0.7    — high pursuit: cardHeuristic boosts tutors (+pursuit*0.30)
//	            and combo pieces (+pursuit*0.20), pushing safer plays
//	            down the ranking
//	~1.0      — single-cast-from-winning; tutors maximally promoted
//
// The 0.7 threshold matches the close-to-closing gate in scoreCombo's
// stack-window penalty (PR #893) so the same "we're close" framing
// drives both signals consistently.
func (h *YggdrasilHat) WinConPursuit(gs *gameengine.GameState, seatIdx int) float64 {
	if h == nil || h.Strategy == nil || len(h.Strategy.ComboPieces) == 0 {
		return 0
	}
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}

	graveyardWeight := 0.5
	if seatHasComboGraveyardRecursion(seat) {
		graveyardWeight = 0.9
	}
	available := make(map[string]float64, len(seat.Hand)+len(seat.Battlefield)+len(seat.Graveyard)+len(seat.CommandZone))
	for _, c := range seat.Hand {
		if c != nil {
			available[c.DisplayName()] = 1.0
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card != nil {
			available[p.Card.DisplayName()] = 1.0
		}
	}
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if available[c.DisplayName()] < graveyardWeight {
			available[c.DisplayName()] = graveyardWeight
		}
	}
	for _, c := range seat.CommandZone {
		if c == nil {
			continue
		}
		if available[c.DisplayName()] < 0.8 {
			available[c.DisplayName()] = 0.8
		}
	}

	tutors := seatTutorsInHand(seat)
	bestRatio := 0.0
	for _, cp := range h.Strategy.ComboPieces {
		if len(cp.Pieces) == 0 {
			continue
		}
		foundWeight := 0.0
		realPieces := 0
		missing := 0
		for _, piece := range cp.Pieces {
			if w := available[piece]; w > 0 {
				foundWeight += w
				realPieces++
			} else {
				missing++
			}
		}
		// Tutor credit mirrors scoreCombo: capped at realPieces+1 so a
		// tutor-only hand (no real anchor) never claims more than 1
		// soft-piece; one real anchor lets credit grow to 2, etc.
		if tutors > 0 && missing > 0 {
			credit := tutors
			if credit > missing {
				credit = missing
			}
			if cap := realPieces + 1; credit > cap {
				credit = cap
			}
			foundWeight += float64(credit)
		}
		if foundWeight > float64(len(cp.Pieces)) {
			foundWeight = float64(len(cp.Pieces))
		}
		ratio := foundWeight / float64(len(cp.Pieces))
		if ratio > bestRatio {
			bestRatio = ratio
		}
	}

	pursuit := bestRatio

	// Mana-availability bonus: combo is close AND mana to deploy is up.
	// Only fires when bestRatio is already meaningful (>= 0.5) so an
	// empty board with 10 lands doesn't claim pursuit.
	if bestRatio >= 0.5 {
		if CountUntappedManaSources(seat) >= 3 {
			pursuit += 0.10
		}
	}

	if pursuit > 1.0 {
		pursuit = 1.0
	}
	return pursuit
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
			if !strings.EqualFold(p.Card.DisplayName(), piece) {
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

// cheapInteractionPassAdjust returns the passBoost delta driven by the
// deck's cheap-interaction density (StrategyProfile.CheapInteraction,
// count of CMC 0-2 interaction spells). Refines the archetype-based
// passBoost with per-deck-shape signal:
//
//   - 4+ cheap interaction pieces (a "stocked interaction package"
//     by Freya's RoleCounterspell threshold): +0.10 to passBoost.
//     The deck has meaningful instant-speed plays on opp turns that
//     the archetype switch alone doesn't credit (e.g. a Midrange
//     deck with 8 cheap interaction won't get any hold-mana boost
//     from the archetype lookup since Midrange isn't in the switch).
//   - 0 cheap interaction pieces (deck cannot interact at all):
//     -0.15 to passBoost. A Control archetype labeled deck with no
//     cheap interaction shouldn't get the +0.30 hold-mana boost
//     because there's literally nothing to hold mana for — the
//     archetype stance encodes intent, the count encodes capability,
//     and capability vetoes.
//   - 1-3 cheap interaction: 0 (neutral middle).
//
// Magnitudes deliberately small relative to the archetype-stance
// boosts above so this signal refines rather than replaces. Returns
// 0 when sp is nil (legacy / no-strategy hat).
//
// Closes the freya-hat integration audit gap (2026-05-30):
// CheapInteraction was loaded by strategy_loader.go but never read
// by any decision code despite the field docstring claiming it
// drives hold-mana behavior. Tested via
// TestCheapInteractionPassAdjust in cheap_interaction_pass_r60_test.go.
func cheapInteractionPassAdjust(sp *StrategyProfile) float64 {
	if sp == nil {
		return 0
	}
	switch {
	case sp.CheapInteraction >= 4:
		return 0.10
	case sp.CheapInteraction == 0:
		return -0.15
	}
	return 0
}

// synergyClusterCohesionBoost returns a small additive cardHeuristic
// bonus when the candidate card is a member of a Freya synergy cluster
// that has already activated (≥2 OTHER cluster members on the seat's
// battlefield). Closes the wave-4 freya-hat integration audit gap
// (2026-05-30): Freya's themed SynergyClusters (Tokens, Recursion,
// Lifegain, +1/+1 Counters, Spellslinger, etc.) tracked the deck's
// engine groupings but never reached the strategy.json wire — the hat
// couldn't sequence "play the second token-maker because Anointed
// Procession is already out" or "cast the +1/+1 anthem now because
// Hardened Scales + Cathars' Crusade are already in play".
//
// Per-cluster contribution:
//   - HighDensity cluster (Freya marks MemberCount ≥ 5 as a real
//     subsystem of the gameplan, not incidental overlap): +0.10
//   - Regular cluster: +0.05
//
// Cumulative cap +0.25 so multi-cluster decks don't stack the boost
// past the magnitude of star/cuttable adjustments.
//
// Cluster "activation" requires ≥2 OTHER members already on battlefield
// (i.e. the candidate is the third or later member). The "OTHER"
// guard prevents the candidate itself from counting toward
// activation — a creature about to be cast shouldn't be its own
// cohesion partner.
//
// Returns 0 for nil Strategy, empty SynergyClusters, nil gs, or when
// the card is not a member of any active cluster. Tested in
// synergy_cluster_cohesion_r60_test.go.
// huginnPredictionBoost returns a small additive cardHeuristic bonus
// when the candidate card appears in any Huginn-generated combo
// prediction on the deck's StrategyProfile.HuginnPredictions.
// Worker D — Huginn 2.0 freya integration (2026-05-31).
//
// Per-prediction contribution: 0.10 × Confidence (so a 0.8-confidence
// prediction contributes +0.08; a 0.3-confidence prediction
// contributes +0.03). A card that appears in multiple predictions
// sums all contributions. Cumulative cap +0.20 keeps the boost in
// the same magnitude as the SynergyCluster cohesion boost (+0.25
// cap) and the star-card bonus (+0.15) so prediction signal refines
// rather than dominates.
//
// Composes additively with the existing ComboPieces scoring path —
// a card that's in BOTH a confirmed ComboPlan AND a high-confidence
// Huginn prediction correctly gets both boosts. The two paths
// measure different things: ComboPieces is a hard win-line plan,
// HuginnPredictions is a speculative prior that wants to be
// validated.
//
// Returns 0 for nil sp, empty predictions, or candidates not
// appearing in any prediction. Case-insensitive name matching
// mirrors the synergyClusterCohesionBoost convention. Tested in
// huginn_prediction_boost_r60_test.go.
func huginnPredictionBoost(sp *StrategyProfile, c *gameengine.Card) float64 {
	if sp == nil || len(sp.HuginnPredictions) == 0 || c == nil {
		return 0
	}
	candidateLower := strings.ToLower(c.DisplayName())
	boost := 0.0
	for _, p := range sp.HuginnPredictions {
		for _, name := range p.Cards {
			if strings.ToLower(name) == candidateLower {
				boost += 0.10 * p.Confidence
				break
			}
		}
	}
	if boost > 0.20 {
		boost = 0.20
	}
	if boost < 0 {
		// Defensive: a negative Confidence value (out of [0, 1])
		// shouldn't produce a negative cardHeuristic boost — that
		// would actively deprioritize a predicted piece. Normalize
		// to 0 instead. The normalizeStrategyProfile clamp (wave-5)
		// would catch negative Confidence at load time, but this
		// defends against direct StrategyProfile construction in
		// tests / future call sites that bypass load.
		boost = 0
	}
	return boost
}

func (h *YggdrasilHat) synergyClusterCohesionBoost(gs *gameengine.GameState, seatIdx int, c *gameengine.Card) float64 {
	if h == nil || h.Strategy == nil || len(h.Strategy.SynergyClusters) == 0 ||
		gs == nil || c == nil {
		return 0
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}

	candidateLower := strings.ToLower(c.DisplayName())
	bfNames := make(map[string]bool, len(seat.Battlefield))
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		bfNames[strings.ToLower(p.Card.DisplayName())] = true
	}

	boost := 0.0
	for _, cluster := range h.Strategy.SynergyClusters {
		isMember := false
		activeOthers := 0
		for _, m := range cluster.Members {
			ml := strings.ToLower(m)
			if ml == candidateLower {
				isMember = true
				continue
			}
			if bfNames[ml] {
				activeOthers++
			}
		}
		if !isMember || activeOthers < 2 {
			continue
		}
		if cluster.HighDensity {
			boost += 0.10
		} else {
			boost += 0.05
		}
	}
	if boost > 0.25 {
		boost = 0.25
	}
	return boost
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

// hasAttackTriggerValue is true when the commander has an attack-trigger
// whose value (tutor / Etali-style impulse-cast / scry-and-look) is large
// enough that we should prioritize getting it into combat over preserving
// it from a clean trade. Matches the same substring shape used in the
// per-attacker value loop above — keep them in sync.
func hasAttackTriggerValue(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	ot := gameengine.OracleTextLower(c)
	if !strings.Contains(ot, "attacks") {
		return false
	}
	return strings.Contains(ot, "search") ||
		strings.Contains(ot, "exile the top") ||
		strings.Contains(ot, "look at the top")
}

// commanderClockNearLethal reports whether the named commander has
// already dealt enough damage to some single opponent that one more
// connect-or-near-connect resolves the 21-damage clock (CR §704.6c).
// Threshold is the minimum accumulated damage on any opponent — at 13
// every nontrivial commander (≥8 power, or buffed) can close the gap
// in one hit, which is the case where pruning the swing wastes a real
// kill opportunity. Returns false when the commander hasn't dealt any
// combat damage yet — early-game commanders fall through to the normal
// "strategic only if it does something" gate.
func commanderClockNearLethal(gs *gameengine.GameState, dealerSeat int, c *gameengine.Card, threshold int) bool {
	if gs == nil || c == nil {
		return false
	}
	name := c.DisplayName()
	for i, s := range gs.Seats {
		if i == dealerSeat || s == nil || s.Lost || s.LeftGame {
			continue
		}
		if gameengine.CommanderDamageFrom(s, dealerSeat, name) >= threshold {
			return true
		}
	}
	return false
}

// hasDeathPayoffValue reports whether a creature's death generates
// meaningful value — either via a self-referential death trigger
// ("when ~ dies"), a damage-taken trigger ("when ~ is dealt damage"),
// or a recursion keyword (persist / undying / unearth) that brings
// the creature back. These are the "trick attack" archetype: the
// creature WANTS to attack into a clean block because the trade is
// the engine.
//
// Examples this catches:
//   - Reassembling Skeleton (own dies, return on activation — paired
//     with an explicit sac outlet, but the recursion is the value)
//   - Hangarback Walker ("when ~ dies, create N Thopters")
//   - Murderous Redcap (persist — comes back 1/1 with damage trigger)
//   - Bloodghast ("landfall: return ~ from your graveyard")
//   - Doomed Traveler ("when ~ dies, create a Spirit token")
//   - Sakura-Tribe Elder, Solemn Simulacrum (sac/die for land + draw)
//
// Why oracle-text + keyword rather than a per-card list: the corpus
// is too big to hand-curate, but the textual shape of "death IS the
// win" effects is extremely consistent ("when this dies" / "whenever
// ~ dies" / "if ~ dies this turn" / "~ enters with ... persist /
// undying"). Same approach as the existing hasAttackTriggerValue
// helper — see the kept-in-sync comment there.
func hasDeathPayoffValue(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	// Persist / Undying / Unearth — strict recursion keywords. The
	// creature comes back after dying, so the swing-into-block trade
	// is at worst neutral (-1/-1 or +1/+1 counter restored) and
	// usually positive (Murderous Redcap re-triggers the damage on
	// re-ETB, Geralf's Messenger pings on the persist return).
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "persist") ||
		strings.Contains(ot, "undying") ||
		strings.Contains(ot, "unearth") {
		return true
	}
	// Self-referential death trigger. Matches "when ~ dies" and the
	// generic "creature dies" pattern only when paired with a
	// self-reference. "When a creature dies" without "this" / "~" is
	// a global trigger (Blood Artist) — those creatures get value
	// from OTHER deaths, not from attacking and dying themselves, so
	// they don't qualify as trick-attackers.
	if strings.Contains(ot, "when this dies") ||
		strings.Contains(ot, "when this creature dies") ||
		strings.Contains(ot, "if this dies") ||
		strings.Contains(ot, "whenever this dies") {
		return true
	}
	// Damage-taken trigger ("whenever ~ is dealt damage, ...") — the
	// creature converts incoming damage into value (Mardu Hateblade
	// family, Stuffy Doll, Boros Reckoner).
	if strings.Contains(ot, "is dealt damage") ||
		strings.Contains(ot, "when this is dealt damage") {
		return true
	}
	return false
}

// hasSacFuelValue reports whether the controller has a sac outlet on
// the battlefield AND this creature is cheap enough (≤ 2 mana) to
// serve as one-shot fuel. The combination means the attacker can
// swing into a block, get blocked (or not), and on a bad trade the
// outlet drains the creature into a triggered effect (Phyrexian
// Altar mana, Goblin Bombardment damage, Yawgmoth draw, Viscera
// Seer scry).
//
// This is the "intentional chump" signal: the attacker is a token /
// 1-drop / 2-drop the controller is happy to lose because the sac
// outlet converts the loss into value the block was supposed to
// deny. The block becomes a tempo trade where WE pick the timing.
func hasSacFuelValue(gs *gameengine.GameState, seatIdx int, p *gameengine.Permanent) bool {
	if gs == nil || p == nil || p.Card == nil ||
		seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if gameengine.ManaCostOf(p.Card) > 2 {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, other := range seat.Battlefield {
		if other == nil || other.Card == nil || other == p {
			continue
		}
		ot := gameengine.OracleTextLower(other.Card)
		// "sacrifice a creature:" with a colon is the activated-ability
		// shape ("Sacrifice a creature: Add {B}." / "Sacrifice a
		// creature: Target creature gets +2/+0."). Skip free-floating
		// "sacrifice a creature" in costs of one-shot spells — those
		// aren't on the battlefield as standing outlets.
		if strings.Contains(ot, "sacrifice a creature:") ||
			strings.Contains(ot, "sacrifice another creature:") ||
			strings.Contains(ot, "sacrifice another creature,") ||
			strings.Contains(ot, ", sacrifice a creature:") {
			return true
		}
	}
	return false
}

// cardPrefersMain1 reports whether a card wants to be cast in the
// precombat main phase (rather than postcombat_main) because its
// value is tied to THIS turn's combat:
//
//   - Haste creatures want to attack the turn they enter.
//   - Attack-trigger commanders (Etali, Zur, Narset, Goblin Guide,
//     Bonecrusher Giant) need to be in play before declare_attackers
//     to swing for their value trigger.
//   - Anthem effects ("creatures you control get +X/+X until end of
//     turn" / "until your next turn") only buff a combat that hasn't
//     happened yet; main2 is too late.
//   - Equipment / aura cards with "equip" / "enchant creature" want
//     to be deployed + attached before combat to make the swing
//     bigger.
//
// Used by ChooseCastFromHand to bias the cast-vs-pass decision in
// precombat_main toward cards whose value-on-the-table evaporates if
// they sit in hand through the combat phase.
func cardPrefersMain1(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	if strings.Contains(ot, "haste") {
		return true
	}
	// Attack-trigger value (shares the substring shape with the
	// existing hasAttackTriggerValue helper used by the attack-target
	// pipeline — keep in sync; the helper is on a Card already).
	if hasAttackTriggerValue(c) {
		return true
	}
	// Anthem family — only buffs this turn's combat. "until end of
	// turn" and "until your next turn" variants both qualify; static
	// anthems (Glorious Anthem, Honor of the Pure) are also main1-
	// preferring because the buff applies to the combat-step state.
	if strings.Contains(ot, "creatures you control get +") {
		return true
	}
	if strings.Contains(ot, "attacking creatures get +") {
		return true
	}
	if strings.Contains(ot, "creatures you control gain") &&
		(strings.Contains(ot, "haste") || strings.Contains(ot, "trample") ||
			strings.Contains(ot, "first strike")) {
		return true
	}
	// Equipment — the card itself wants to be deployed AND attached
	// pre-combat. Bias the cast toward main1 so the engine's equip
	// activation window in the same main phase can attach.
	if typeLineContains(c, "equipment") {
		return true
	}
	return false
}

// cardIsGoad detects spell or activated-ability oracle text that
// applies the Goad effect (CR §701.39). Matches the literal "goad"
// keyword which the parser uses uniformly for Magic's modern Goad
// implementations (Coastal Piracy of Smoke, Lure of the Wilds, Disrupt
// Decorum, Agitator Ant, Cleavemaul Knight, Maelstrom Wanderer's
// triggered grant, etc.). Cards whose oracle text only contains "goad"
// as part of a longer word (e.g. "goading" doesn't appear in any
// printed card — the literal verb match is safe). Used by cardHeuristic
// to elevate goad spells when a political "deal" redirect is available.
func cardIsGoad(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	return strings.Contains(ot, "goad")
}

// goadDealOpportunity scores the political "deal" value of casting a
// goad effect from our seat. Returns 0 when no opponent creature
// presents a useful redirect target; returns up to +0.45 when:
//
//   - Some other opponent (not the threat's controller) is at low
//     enough life to die to a redirected swing.
//   - The largest non-defender creature owned by a non-low-life-opp
//     opponent has power >= that lowest-life-opp's life AND is also
//     a meaningful threat to us (>= myLife/3 or >= 6 absolute).
//
// The "deal" framing: instead of casting removal that kills the
// threat (consuming our removal AND making us the table's answer-
// spender — both political costs), we redirect the threat at someone
// who can't survive it. The threat-holder still trades their creature
// in combat against the goaded target's defenses, we kept our removal
// in reserve, and the table eliminates an opponent for us.
//
// Three additive contributors, capped at +0.45:
//
//   - Lethal redirect available: +0.20 baseline.
//   - Threat also fast-clocks us (power*2 >= myLife): +0.15.
//   - We're behind on board (relPos < 0): +0.10 — goad is most useful
//     when we lack the resources to handle threats directly.
//
// Defenders are excluded — defender + goad is a no-op (CR §702.3b:
// defenders can't attack regardless of "must attack" riders).
// Hexproof / shroud creatures are not pre-filtered here; the engine's
// target legality check handles that at cast time.
func (h *YggdrasilHat) goadDealOpportunity(gs *gameengine.GameState, seatIdx int) float64 {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return 0
	}
	myLife := gs.Seats[seatIdx].Life
	if myLife <= 0 {
		return 0
	}
	threatThreshold := myLife / 3
	if threatThreshold < 6 {
		threatThreshold = 6
	}

	lowestLifeOpp := -1
	lowestLife := 9999
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		if s.Life > 0 && s.Life < lowestLife {
			lowestLife = s.Life
			lowestLifeOpp = i
		}
	}
	if lowestLifeOpp < 0 {
		return 0
	}

	for i, s := range gs.Seats {
		if i == seatIdx || i == lowestLifeOpp || s == nil || s.Lost || s.LeftGame {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() || p.HasKeyword("defender") {
				continue
			}
			pow := gs.PowerOf(p)
			if pow < lowestLife || pow < threatThreshold {
				continue
			}
			bonus := 0.20
			if pow*2 >= myLife {
				bonus += 0.15
			}
			if h.relativePosition(gs, seatIdx) < 0 {
				bonus += 0.10
			}
			if bonus > 0.45 {
				bonus = 0.45
			}
			return bonus
		}
	}
	return 0
}

// -- UCB1 machinery (shared across all decision types) --

func (h *YggdrasilHat) ucb1(key string, baseValue float64) float64 {
	stat, ok := h.actionStats[key]
	if !ok || stat.visits == 0 {
		return baseValue + 2.0
	}
	avg := stat.value / float64(stat.visits)
	c := h.explorationC
	if c <= 0 {
		c = math.Sqrt(2.0)
	}
	exploration := c * math.Sqrt(math.Log(float64(h.totalVisits+1))/float64(stat.visits))
	return avg + exploration
}

// explorationFactor returns the UCB1 exploration constant C for this turn,
// tuned by archetype, game stage, and current plan. The result is cached
// per-turn via h.explorationC / h.explorationCTurn.
//
// Archetype base values:
//
//	Aggro/Tribal  1.0  — execute the beatdown, don't get cute
//	Stax          1.2  — lock pieces are clear, minimal exploration
//	Ramp          1.3  — ramp targets are known
//	Midrange      1.4  — balanced
//	Control       1.5  — need to find the right answers
//	Combo         1.8  — explore lines early, converge once assembling
//	Others        sqrt(2) — standard UCB1 default
//
// Modulated by game stage (multiplicative):
//
//	Turn <= 5   +20%  — still learning the board
//	Turn > 12   -20%  — time to execute
//
// Modulated by current plan (multiplicative):
//
//	PlanExecute  x0.5  — near-zero exploration, just win
//	PlanAssemble x0.8  — focused but still some exploration
//	PlanDisrupt  x1.2  — explore to find the right answer
func (h *YggdrasilHat) explorationFactor(gs *gameengine.GameState, seatIdx int) float64 {
	_ = seatIdx // reserved for future per-seat tuning

	turn := 0
	if gs != nil {
		turn = gs.Turn
	}
	if turn == h.explorationCTurn && h.explorationC > 0 {
		return h.explorationC
	}
	h.explorationCTurn = turn

	// --- Archetype base C ---
	base := math.Sqrt(2.0)
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro, ArchetypeTribal:
			base = 1.0
		case ArchetypeStax:
			base = 1.2
		case ArchetypeRamp, ArchetypeAristocrats:
			base = 1.3
		case ArchetypeMidrange:
			base = 1.4
		case ArchetypeControl, ArchetypeReanimator, ArchetypeSpellslinger:
			base = 1.5
		case ArchetypeCombo:
			base = 1.8
		}
	}

	// --- Game stage modulation (multiplicative) ---
	if turn <= 5 {
		base *= 1.2
	} else if turn > 12 {
		base *= 0.8
	}

	// --- Plan modulation (multiplicative) ---
	switch h.planState.Current {
	case PlanExecute:
		base *= 0.5
	case PlanAssemble:
		base *= 0.8
	case PlanDisrupt:
		base *= 1.2
	}

	// Floor: never let exploration collapse completely.
	if base < 0.3 {
		base = 0.3
	}

	h.explorationC = base
	return base
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

// emitDecisionEvent persists a hat decision to the engine event log so the
// post-game analyzer (heimdall) can reconstruct *why* a choice was made
// without the in-memory DecisionLog (which is nil in tournament play). The
// hat's own archetype belief is stamped into every event so each row is
// self-describing — analysts shouldn't have to rejoin against the deck
// index just to know which strategy was driving the decision. Kind is the
// decision category ("mulligan", "block", "attack_target", "mode") and is
// prefixed with "hat_decision_" on the engine side.
func (h *YggdrasilHat) emitDecisionEvent(gs *gameengine.GameState, seatIdx int, kind string, details map[string]interface{}) {
	if gs == nil {
		return
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	if _, ok := details["archetype"]; !ok {
		if h.Strategy != nil {
			details["archetype"] = h.Strategy.Archetype
		} else {
			details["archetype"] = ""
		}
	}
	details["turn"] = gs.Turn
	gs.LogEvent(gameengine.Event{
		Kind:    "hat_decision_" + kind,
		Seat:    seatIdx,
		Target:  -1,
		Source:  "yggdrasil",
		Amount:  gs.Turn,
		Details: details,
	})
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

// handStatsForLog computes a compact hand summary for decision logging.
// Mirrors the categorization the mulligan heuristic does so post-game
// analysis can correlate the keep/mull call against hand composition
// without re-running the categorize loop.
func (h *YggdrasilHat) handStatsForLog(hand []*gameengine.Card) (lands, combo, valueEngines, stars, cuttables int) {
	for _, c := range hand {
		if c == nil {
			continue
		}
		for _, t := range c.Types {
			if t == "land" {
				lands++
				break
			}
		}
		if h.isComboRelevant(c) {
			combo++
		}
		if h.isValueEngineKey(c) {
			valueEngines++
		}
		if h.isStarCard(c) {
			stars++
		}
		if h.isCuttable(c) {
			cuttables++
		}
	}
	return
}

func (h *YggdrasilHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) (mulligan bool) {
	// Named return + defer so every existing return path participates in
	// the persisted decision event without threading a reason through ~8
	// return sites. Hand stats are computed once here; the inner heuristic
	// recomputes them locally (cheap — bounded hand size).
	defer func() {
		lands, combo, veCount, starCount, cuttableCount := h.handStatsForLog(hand)
		h.emitDecisionEvent(gs, seatIdx, "mulligan", map[string]interface{}{
			"hand_size":     len(hand),
			"lands":         lands,
			"combo":         combo,
			"value_engines": veCount,
			"stars":         starCount,
			"cuttables":     cuttableCount,
			"mulliganed":    mulligan,
		})
		h.logf("MULLIGAN seat=%d size=%d lands=%d ve=%d stars=%d cut=%d -> mull=%v",
			seatIdx, len(hand), lands, veCount, starCount, cuttableCount, mulligan)
	}()

	landCount := 0
	comboCount := 0
	rampCount := 0
	cheapSpells := 0
	// actionCount = nonland cards that advance an actual game plan (a threat,
	// removal, draw, counter, combo piece, or a value engine / star). Ramp on
	// its own is NOT a payoff — a hand of all lands + rocks does nothing. Used
	// by the archetype-agnostic flood ceiling and the final action-density
	// keep gate so we never keep a hand with no castable action.
	actionCount := 0
	for _, c := range hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				landCount++
				isLand = true
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
		if !isLand {
			switch cat {
			case CatThreat, CatRemoval, CatDraw, CatCounter, CatCombo:
				actionCount++
			default:
				if h.isValueEngineKey(c) || h.isStarCard(c) {
					actionCount++
				}
			}
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

	// Partner-aware mulligan: partner decks need enablers for both halves.
	// A hand with ramp/draw for only one color identity half is weak.
	if len(hand) >= 7 && gs != nil && len(gs.Seats[seatIdx].CommanderNames) >= 2 && h.Strategy != nil && h.Strategy.ColorDemand != nil {
		seat := gs.Seats[seatIdx]
		// Collect colors available from lands in hand.
		handColors := make(map[string]bool)
		for _, c := range hand {
			if c == nil {
				continue
			}
			isLand := false
			for _, t := range c.Types {
				if t == "land" {
					isLand = true
					break
				}
			}
			if !isLand {
				continue
			}
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
		}

		// Find each commander's colors and check if the hand supports both.
		type cmdProfile struct {
			name   string
			colors []string
		}
		var cmdProfiles []cmdProfile
		for _, cn := range seat.CommanderNames {
			var colors []string
			for _, c := range seat.CommandZone {
				if c != nil && c.DisplayName() == cn {
					colors = c.Colors
					break
				}
			}
			cmdProfiles = append(cmdProfiles, cmdProfile{cn, colors})
		}

		// Check if hand has enablers that work with both commanders.
		// Count cards relevant to each commander's color identity.
		enablersPerCmd := make([]int, len(cmdProfiles))
		for _, c := range hand {
			if c == nil {
				continue
			}
			isLand := false
			for _, t := range c.Types {
				if t == "land" {
					isLand = true
					break
				}
			}
			if isLand {
				continue
			}
			for i, cp := range cmdProfiles {
				for _, col := range cp.colors {
					for _, cc := range c.Colors {
						if cc == col {
							enablersPerCmd[i]++
							break
						}
					}
				}
			}
		}

		// If hand completely lacks enablers for one commander half, mulligan.
		if len(cmdProfiles) >= 2 && landCount >= 2 {
			minEnablers := enablersPerCmd[0]
			for _, e := range enablersPerCmd[1:] {
				if e < minEnablers {
					minEnablers = e
				}
			}
			// A hand with 0 non-land cards matching one commander's colors is
			// unbalanced for a partner deck.
			if minEnablers == 0 && len(hand) >= 7 {
				// But only mulligan if we also lack star cards / combo pieces.
				if starCount == 0 && comboCount == 0 {
					return true
				}
			}
		}
	}

	// Commander-castability check: the deck's primary plan is usually
	// "land the commander, then execute its text" — a hand that can't
	// reach commander CMC by turn 4 throws that plan away unless there's
	// an alternate engine to fall back on. Only fires on 7-card hands
	// (partial mulls accept thinner plans) and only when no engine
	// (star / VE / combo) provides a non-commander game plan. Without
	// engine backup, a hand stuck on a 6-CMC commander it can't cast
	// until turn 6+ gets run over by curve-based opponents before the
	// commander ever hits the board. For partner decks the check uses
	// the cheaper commander — you can cast either to start executing.
	// See projectedManaAtTurn for the land+ramp model.
	if len(hand) >= 7 && gs != nil {
		if cmdrCMC := minCommanderCMC(gs.Seats[seatIdx]); cmdrCMC > 0 {
			projected := projectedManaAtTurn(landCount, rampCount, 4)
			if projected < float64(cmdrCMC) {
				hasEngine := starCount >= 1 || veCount >= 1 || comboCount >= 1
				if !hasEngine {
					return true
				}
			}
		}
	}

	// Archetype-aware keepability on 7 cards.
	if len(hand) >= 7 {
		if landCount <= 1 {
			return true
		}
		// Archetype-AGNOSTIC flood ceiling. Previously only Aggro and Tribal
		// guarded against land floods (landCount > 4); Control / Combo /
		// Midrange / Ramp / Reanimator fell straight through and kept 6-7
		// land hands. Apply a conservative flood cap to EVERY archetype:
		//   - 6+ lands in a 7-card hand is a flood regardless of plan; mull.
		//   - exactly 5 lands with no early action/payoff (no threat, removal,
		//     draw, combo piece, value engine, or star) is a do-nothing flood;
		//     mull. A 5-land hand WITH an action is a fine Commander keep, so
		//     it's left alone.
		if landCount >= 6 {
			return true
		}
		if landCount == 5 && actionCount == 0 {
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
			case ArchetypeEnchantress:
				enchantmentCount := 0
				for _, c := range hand {
					if c == nil {
						continue
					}
					if typeLineContains(c, "enchantment") || h.valueEngineSet[c.DisplayName()] {
						enchantmentCount++
					}
				}
				if landCount >= 2 && enchantmentCount >= 1 {
					return false
				}
			case ArchetypeArtifacts:
				artifactCount := 0
				for _, c := range hand {
					if c == nil {
						continue
					}
					if typeLineContains(c, "artifact") || isManaRock(c) {
						artifactCount++
					}
				}
				if landCount >= 2 && artifactCount >= 1 {
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

		// Combo-threat tightening: if any opponent's commander looks like
		// a combo win condition (oracle text hits "infinite" / "win the
		// game" / "extra turn" / "untap all" / "create a copy"), demand
		// either interaction or an engine card in hand. A marginal hand
		// with only lands + ramp + cuttables vs a combo opponent gets
		// run over before turn 4 — better to dig for a counter or a
		// stax piece.
		if someOpponentLooksCombo(gs, seatIdx) {
			hasInteraction := handHasInteraction(hand)
			hasEngine := veCount >= 1 || starCount >= 1 || comboCount >= 1
			if !hasInteraction && !hasEngine {
				return true
			}
		}

		// Action-density final gate (7-card). Any hand that reached this
		// fall-through with ZERO castable action — no threat, removal, draw,
		// counter, combo piece, value engine, or star, just lands + ramp +
		// utility chaff — does nothing for several turns and should be
		// mulliganed. The per-archetype keeps above already returned for
		// hands with a real plan, so this only catches the genuinely empty
		// "lands and rocks" hands the old fall-through silently kept.
		if actionCount == 0 {
			return true
		}
	}

	// On 6 or fewer: star cards make marginal hands keepable. We tolerate
	// thinner hands after a mulligan (London mull cost), but still refuse a
	// hand with no castable action at all — a 6-card pile of lands + ramp
	// does nothing whether it's the first hand or the third.
	if len(hand) <= 6 {
		if landCount == 0 {
			return true
		}
		if starCount >= 1 && landCount >= 1 {
			return false
		}
		if actionCount == 0 {
			return true
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

	seat := gs.Seats[seatIdx]

	// Forward-looking color demand: scan spells in hand and tally color
	// pips needed. Spells castable next turn (CMC <= available mana + 1)
	// get double weight — they represent immediate sequencing pressure.
	handDemand := map[string]float64{}
	availMana := gameengine.AvailableManaEstimate(gs, seat) + 1
	for _, c := range seat.Hand {
		if c == nil || c.AST == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		weight := 1.0
		if c.CMC <= availMana {
			weight = 2.0
		}
		for _, ab := range c.AST.Abilities {
			act, ok := ab.(*gameast.Activated)
			if !ok || act.Cost.Mana == nil {
				continue
			}
			for _, sym := range act.Cost.Mana.Symbols {
				for _, col := range sym.Color {
					handDemand[col] += weight
				}
			}
		}
		for _, col := range c.Colors {
			handDemand[col] += weight * 0.5
		}
	}

	// Mana-curve "dead next turn" detection: is there a non-land card
	// in hand we could realistically cast at avail+1 mana? If not, an
	// ETB-tapped land costs no tempo this turn (we weren't deploying
	// anyway) — soften the penalty so a color-fixer or utility land
	// gets played over a basic when the immediate-tempo cost is zero.
	deadNextTurn := true
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		if gameengine.ManaCostOf(c) <= availMana {
			deadNextTurn = false
			break
		}
	}

	// Archetype shapes how harshly tempo loss bites. Aggro and combo
	// need to deploy on schedule; control and ramp can stomach a tapped
	// land. Multiplier rides on top of the early/late turn penalty.
	tappedMul := 1.0
	if h.Strategy != nil {
		switch h.Strategy.Archetype {
		case ArchetypeAggro:
			tappedMul = 1.5
		case ArchetypeCombo:
			tappedMul = 1.2
		case ArchetypeControl:
			tappedMul = 0.5
		case ArchetypeRamp:
			tappedMul = 0.7
		}
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
			base := 2.0
			if gs.Turn > 4 {
				base = 0.5
			}
			// Dead-next-turn override: collapse the early penalty to
			// the late-game floor when there's nothing to cast at
			// avail+1 anyway. Tempo loss is the only reason ETB-tapped
			// is bad early; no tempo to lose means no penalty to apply.
			if deadNextTurn && gs.Turn <= 4 {
				base = 0.5
			}
			sc -= base * tappedMul
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

		landMask := landProducesColorsMask(l)

		// Hand-aware color sequencing: boost lands that produce colors
		// matching spells in hand. Stronger boost for near-castable spells.
		for col, demand := range handDemand {
			bit := colorSymBit(col)
			if bit == 0 || landMask&bit == 0 {
				continue
			}
			have := float64(fieldColorSources(seat, col))
			if have < 2 {
				sc += demand * 0.8
			} else if have < 4 {
				sc += demand * 0.3
			}
		}

		// Deck-level color-fixing: boost lands that produce colors we need but lack.
		// Weak mana bases (C/D/F grade) get a larger color-fixing multiplier.
		if h.Strategy != nil && h.Strategy.ColorDemand != nil {
			fixMul := 1.5
			if h.Strategy.ManaBaseGrade == "D" || h.Strategy.ManaBaseGrade == "F" {
				fixMul = 2.5
			} else if h.Strategy.ManaBaseGrade == "C" {
				fixMul = 2.0
			}
			for col, demand := range h.Strategy.ColorDemand {
				if demand < 3 {
					continue
				}
				bit := colorSymBit(col)
				if bit == 0 || landMask&bit == 0 {
					continue
				}
				have := fieldColorSources(seat, col)
				need := float64(demand) / 10.0
				deficit := need - float64(have)*0.3
				if deficit > 0 {
					sc += deficit * fixMul
				}
			}
		}

		// Basic lands get a small baseline. Lowercase name once.
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, "plains") || strings.Contains(nameLower, "island") ||
			strings.Contains(nameLower, "swamp") || strings.Contains(nameLower, "mountain") ||
			strings.Contains(nameLower, "forest") {
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
	// r60-cedh-planstate: refresh plan state at the cast-decision entry
	// so a mid-turn tutor resolve / draw / recursion / gameengine event
	// that flipped the Assembling gate this turn is visible to this
	// decision (cardHeuristic combo-priority bias + effectiveBudget
	// lift). Without this hook, PlanState only re-evaluates on upkeep,
	// so the first cast decision in a fast-cEDH "draw tutor → cast →
	// fetch piece → cast" turn ran under the previous plan and the
	// bias never fired on the critical turn.
	h.refreshPlanState(gs, seatIdx)

	h.recordParentTier(h.classifyDecision(gs), gs.Turn)
	h.explorationFactor(gs, seatIdx)

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

	// Combo sequencer override: if a combo is executable this turn,
	// skip normal evaluation and cast the next combo piece immediately.
	// 3rd Eye: Entropy-gated combo — if an opponent with interactive
	// colors has tutored and is holding mana open, delay the combo
	// attempt unless we're in a must-win situation (low life / kingmaker
	// pressure). Jamming into a known counterspell is worse than waiting.
	if h.comboSeq != nil {
		assessment := h.comboSeq.Evaluate(gs, seatIdx)
		if assessment.Executable && assessment.NextAction != "" {
			entropyBlocked := false
			if gs != nil {
				myLife := 40
				if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
					myLife = gs.Seats[seatIdx].Life
				}
				mustWin := myLife <= 10 || h.relativePosition(gs, seatIdx) < -0.5
				if !mustWin {
					for i := range gs.Seats {
						if i == seatIdx {
							continue
						}
						if h.opponentLikelyHasAnswer(i) {
							h.logf("%s COMBO-DELAY seat=%d (opponent %d likely has answer: tutored=%v heldMana=%d)",
								roundTag(gs, seatIdx), seatIdx, i,
								h.opponentTutored[i], h.opponentHeldMana[i])
							entropyBlocked = true
							break
						}
					}
				}
			}
			if !entropyBlocked {
				for _, c := range pool {
					if c.DisplayName() == assessment.NextAction {
						h.logf("%s COMBO-CAST seat=%d %s (line: %s)",
							roundTag(gs, seatIdx), seatIdx, c.DisplayName(),
							assessment.BestLine.Name)
						return c
					}
				}
			}
		}
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
		// DNA ComboPat: high patience → larger comboMul (hold for combo),
		// low patience → smaller (play pieces for tempo). Max nudge: +/- 40%.
		if h.DNA != nil {
			patShift := (h.DNA.ComboPat - 0.5) * 0.8 // [-0.4, +0.4]
			comboMul *= 1.0 + patShift
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
		// DNA CounterplayTiming: high → strongly prefer holding the
		// counterspell (boost pass), low → prefer slamming proactive
		// plays even when a counter is in hand. Max swing ±0.20 from
		// the 0.25 baseline.
		cpBoost := 0.25
		cpShift := (h.curseAxis(func(d *CurseDNA) float64 { return d.CounterplayTiming }, 0.5) - 0.5) * 0.4
		cpBoost += cpShift
		passBoost += cpBoost
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
	case ArchetypeEnchantress:
		passBoost -= 0.05
	case ArchetypeArtifacts:
		passBoost -= 0.10
		if poolHasVE {
			passBoost -= 0.10
		}
	case ArchetypeSelfmill:
		passBoost -= 0.10
	}
	// CheapInteraction-driven adjustment per StrategyProfile.
	// CheapInteraction / InteractionAvgCMC docstring: "lower avg CMC =
	// faster interaction = can afford to hold mana more often". The
	// archetype switch above encodes a static stance; the deck's actual
	// cheap-interaction density should modulate it. A Control deck shipped
	// with 0 cheap interaction (rare but possible — minimally-rebuilt
	// precons) shouldn't get the full +0.30 hold-mana boost because there
	// is literally nothing to hold mana for. A Midrange / Tribal /
	// Ramp / Aggro deck with 4+ cheap interaction SHOULD lean toward
	// passing — the deck has meaningful instant-speed plays on opp turns
	// that the archetype switch doesn't credit.
	//
	// Thresholds chosen to match Freya's existing cheap-interaction
	// reporting: 4+ pieces is the canonical "stocked interaction package"
	// mark (mirrors RoleCounterspell density gate, line 2427 in
	// archetype.go); 0 pieces is the absolute floor (deck cannot interact
	// at all). Magnitudes (+0.10 / -0.15) are deliberately small — the
	// archetype stance stays load-bearing, this is a refinement.
	//
	// Closes the gap surfaced by the freya-hat integration audit
	// (2026-05-30): CheapInteraction was loaded by strategy_loader.go
	// but never consumed by any decision code despite the field's
	// docstring claiming it drives hold-mana behavior.
	passBoost += cheapInteractionPassAdjust(h.Strategy)
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
	// R60 Second Main Phase audit — Signal A: main2 deploy push.
	// In postcombat_main we're past the only meaningful combat
	// window this turn; unused mana doesn't carry over (mana pools
	// empty at end-of-each-step per CR §106.4). The archetype-based
	// passBoost above represents "save mana for instants / hold the
	// counterspell" — useful in main1 because we still have combat +
	// opponents' turns to spend mana into. In main2 the only window
	// left is our end step, then opp upkeep. Trim the boost so we
	// actually deploy before EOT instead of passing held mana into
	// the void. Counter-hold passBoost stays (still relevant for opp
	// end-step plays); this only relaxes the archetype-stance
	// component.
	if gs != nil && gs.Phase == "postcombat_main" {
		passBoost -= 0.20
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
	canISRollout := eb >= isRolloutBudgetGe && h.TurnRunner != nil &&
		h.turnRemaining(gs) >= isRolloutCost
	canRollout := eb >= rolloutBudgetGe && h.TurnRunner != nil &&
		h.turnRemaining(gs) >= rolloutEvalCost

	for _, c := range pool {
		cardKey := prefix + "cast:" + c.DisplayName()
		heurVal := h.cardHeuristic(gs, seatIdx, c)

		if canISRollout && h.turnRemaining(gs) >= isRolloutCost {
			h.spendTurnBudget(gs, isRolloutCost)
			rollVal := h.multiRolloutForCard(gs, seatIdx, c, isRolloutsPerCandidate)
			blended := rollVal*0.6 + heurVal*0.4
			ucb := h.ucb1(cardKey, blended)
			info := fmt.Sprintf("  candidate: %-35s is-rollout=%.3f heuristic=%.3f blended=%.3f ucb=%.3f",
				c.DisplayName(), rollVal, heurVal, blended, ucb)
			candidates = append(candidates, scored{c, ucb, info})
		} else if canRollout && h.turnRemaining(gs) >= rolloutEvalCost {
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

	if candidates[0].ucb <= passUCB {
		h.logf("  → PASS (pass ucb=%.3f beats best=%.3f)", passUCB, candidates[0].ucb)
		h.recordAction(passKey, pos)
		return nil
	}

	// Confidence threshold selection: at low thresholds (B1), pick
	// randomly among close candidates for varied play. At high
	// thresholds (B5), almost always pick the best.
	ucbs := make([]float64, len(candidates))
	for i, c := range candidates {
		ucbs[i] = c.ucb
	}
	pick := h.selectAmongTop(ucbs)
	best := candidates[pick]

	bestKey := prefix + "cast:" + best.card.DisplayName()
	tierLabel := "heuristic"
	if canISRollout {
		tierLabel = "is_mcts_rollout"
		h.logf("  → CAST %s (ucb=%.3f, beat pass by %.3f, IS-MCTS, pick=%d/%d)",
			best.card.DisplayName(), best.ucb, best.ucb-passUCB, pick, len(candidates))
	} else if canRollout {
		tierLabel = "rollout"
		h.logf("  → CAST %s (ucb=%.3f, beat pass by %.3f, pick=%d/%d)",
			best.card.DisplayName(), best.ucb, best.ucb-passUCB, pick, len(candidates))
	} else {
		h.logf("  → CAST %s (ucb=%.3f, pick=%d/%d)",
			best.card.DisplayName(), best.ucb, pick, len(candidates))
	}
	h.recordAction(bestKey, pos+h.cardHeuristic(gs, seatIdx, best.card))
	// R60 decision-replay surface — emit a structured event so post-
	// game "why did hat cast X?" analysis can read the candidate-vs-pass
	// scoring without re-running the eval. Best.ucb / passUCB / margin
	// are the three numbers that justify the pick; tier and pool size
	// give the context (was this an expensive Ragnarok decision over a
	// 12-card pool, or a cheap heuristic over 2?).
	h.emitDecisionEvent(gs, seatIdx, "cast", map[string]interface{}{
		"card":        best.card.DisplayName(),
		"ucb":         best.ucb,
		"pass_ucb":    passUCB,
		"margin":      best.ucb - passUCB,
		"pool_size":   len(pool),
		"candidates":  len(candidates),
		"tier":        tierLabel,
		"pos":         pos,
		"interaction": interactionRisk,
	})
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

	// Ramp > draw > threats. Always fires turn ≤ 12 (early-game default).
	// Beyond that, the rule still fires when ramping would unlock a hand
	// card the seat couldn't cast this turn (rampUnlocksHand) — late-game
	// ramp into a big finisher is the same tempo win as a turn-3 Cultivate.
	if turn <= 12 || h.rampUnlocksHand(gs, seatIdx) {
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
		if turn <= 12 && len(draw) > 0 {
			sort.SliceStable(draw, func(i, j int) bool {
				return gameengine.ManaCostOf(draw[i]) < gameengine.ManaCostOf(draw[j])
			})
			return draw[0]
		}
		pool = other
		// Re-include `draw` when we entered via the rampUnlocksHand
		// branch past turn 12 — we don't want to drop draw cards from
		// the pool just because they didn't beat ramp on this tick.
		if turn > 12 {
			pool = append(pool, draw...)
		}
	}

	if len(pool) == 0 {
		return nil
	}
	// Default: cardHeuristic-driven sort with a dead-card filter that
	// pushes spells requiring a creature we control to the end when the
	// seat has no creatures. Non-dead cards come first; within each
	// group, cardHeuristic decides.
	sort.SliceStable(pool, func(i, j int) bool {
		iDead := h.castIsDead(gs, seatIdx, pool[i])
		jDead := h.castIsDead(gs, seatIdx, pool[j])
		if iDead != jDead {
			return !iDead
		}
		return h.cardHeuristic(gs, seatIdx, pool[i]) > h.cardHeuristic(gs, seatIdx, pool[j])
	})
	return pool[0]
}

// rampUnlocksHand reports whether the seat holds a non-ramp non-land
// card in hand whose CMC is just out of reach now but would be castable
// after one more mana source. Used by castHeuristic to extend the
// turn ≤ 12 ramp-priority rule into the late game when ramping would
// directly enable a higher-CMC play next turn.
//
// The unlock window is [avail+1, avail+2] — a single +1 ramp source
// typically enables a single CMC bracket, and many ramp cards (Sol
// Ring, signets) effectively add 2 by ETB-untapping for that mana.
func (h *YggdrasilHat) rampUnlocksHand(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	avail := gameengine.AvailableManaEstimate(gs, seat)
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		isLand := false
		for _, t := range c.Types {
			if t == "land" {
				isLand = true
				break
			}
		}
		if isLand {
			continue
		}
		if h.categorizeWithFreya(c) == CatRamp {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > avail && cmc <= avail+2 {
			return true
		}
	}
	return false
}

// castIsDead reports whether casting `card` would have no useful effect
// in the current board state. Conservative substring scan — false
// positives are acceptable (the card just gets cast slightly later),
// false negatives would mis-deprioritize a usable card.
//
// Current rules:
//   - "Target creature you control" / "creatures you control" / "creature
//     you control gains" + the seat controls zero creatures → dead.
//
// Self-creature spells (a creature card that requires "target creature
// you control" for an ETB ability) are NOT marked dead — casting the
// card itself adds a creature to the board, so the requirement is met
// on resolution.
func (h *YggdrasilHat) castIsDead(gs *gameengine.GameState, seatIdx int, card *gameengine.Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	ot := gameengine.OracleTextLower(card)
	if ot == "" {
		return false
	}
	needsCreature := strings.Contains(ot, "target creature you control") ||
		strings.Contains(ot, "creatures you control") ||
		strings.Contains(ot, "creature you control gains")
	if !needsCreature {
		return false
	}
	for _, t := range card.Types {
		if t == "creature" {
			return false
		}
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if typeLineContains(p.Card, "creature") {
			return false
		}
	}
	return true
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
	h.explorationFactor(gs, seatIdx)

	if len(options) == 0 {
		return nil
	}
	h.recordParentTier(h.classifyDecision(gs), gs.Turn)

	// Combo sequencer override: if a combo is executable and the next
	// action matches an activation (already on battlefield), prefer it.
	if h.comboSeq != nil {
		assessment := h.comboSeq.Evaluate(gs, seatIdx)
		if assessment.Executable && assessment.NextAction != "" {
			for i := range options {
				opt := &options[i]
				if opt.Permanent != nil && opt.Permanent.Card != nil &&
					opt.Permanent.Card.DisplayName() == assessment.NextAction {
					h.logf("%s COMBO-ACTIVATE seat=%d %s (line: %s)",
						roundTag(gs, seatIdx), seatIdx,
						opt.Permanent.Card.DisplayName(),
						assessment.BestLine.Name)
					return opt
				}
			}
		}
	}

	if h.effectiveBudget(gs) == 0 {
		// Budget 0 (complex board) still ranks activations by the cheap
		// heuristic rather than blindly firing options[0] — which could be
		// a bad sacrifice, a pointless mana tap, or a life-loss ability.
		// Pick the highest-scoring option; if even the best scores at or
		// below the do-nothing baseline, pass instead of activating.
		bestIdx := -1
		bestScore := math.Inf(-1)
		for i := range options {
			s := h.activationHeuristic(gs, seatIdx, &options[i])
			if s > bestScore {
				bestScore = s
				bestIdx = i
			}
		}
		// 0.15 is the heuristic baseline (no positive signal). Require a
		// small positive margin over it before firing at zero budget.
		if bestIdx < 0 || bestScore <= 0.15 {
			return nil
		}
		return &options[bestIdx]
	}
	h.spendTurnBudget(gs, 1)

	prefix := turnPrefix(gs)
	pos := h.evalPosition(gs, seatIdx)

	passKey := prefix + "act_pass"
	passUCB := h.ucb1(passKey, pos)

	type scoredAct struct {
		opt *gameengine.Activation
		ucb float64
		key string
	}
	acts := make([]scoredAct, 0, len(options))
	for i := range options {
		opt := &options[i]
		name := "?"
		if opt.Permanent != nil && opt.Permanent.Card != nil {
			name = opt.Permanent.Card.DisplayName()
		}
		heurVal := h.activationHeuristic(gs, seatIdx, &options[i])
		key := prefix + fmt.Sprintf("act:%s:%d", name, opt.Ability)
		ucb := h.ucb1(key, pos+heurVal)
		if ucb > passUCB {
			acts = append(acts, scoredAct{opt, ucb, key})
		}
	}

	if len(acts) == 0 {
		h.recordAction(passKey, pos)
		return nil
	}

	sort.SliceStable(acts, func(i, j int) bool {
		return acts[i].ucb > acts[j].ucb
	})

	// Confidence threshold selection among qualifying activations.
	actUCBs := make([]float64, len(acts))
	for i, a := range acts {
		actUCBs[i] = a.ucb
	}
	pick := h.selectAmongTop(actUCBs)
	chosen := acts[pick]

	heurVal := h.activationHeuristic(gs, seatIdx, chosen.opt)
	h.recordAction(chosen.key, pos+heurVal)
	return chosen.opt
}

func (h *YggdrasilHat) activationHeuristic(gs *gameengine.GameState, seatIdx int, opt *gameengine.Activation) float64 {
	base := 0.15
	if opt.Permanent == nil || opt.Permanent.Card == nil {
		return base
	}
	c := opt.Permanent.Card

	// The One Ring — burden counters compound life loss each upkeep.
	// Activation adds another counter and draws N cards, so over a
	// full life cycle we lose 1 + 2 + 3 + ... per consecutive activation.
	// Heuristic: refuse to activate when life can't survive the next
	// upkeep tick after this one. life > burdens*2 from the user spec.
	if c.DisplayName() == "The One Ring" {
		burdens := 0
		if opt.Permanent.Counters != nil {
			burdens = opt.Permanent.Counters["burden"]
		}
		life := 0
		if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
			life = gs.Seats[seatIdx].Life
		}
		// After this activation: burdens+1 counters → next upkeep
		// drains burdens+1 life. We need to outlive at least one more
		// upkeep. Per spec: skip when life <= burdens*2 (proxy for
		// "won't survive the cumulative drain").
		if life > 0 && life <= burdens*2 {
			return -1.0 // forces ucb1 below pass threshold
		}
		// Each fresh draw is +1 cards − burden lost; positively scale
		// based on how cheap it is to take the next drain.
		base += 0.30
		if life > burdens*4 {
			base += 0.10
		}
	}

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
	gyMult := h.graveyardExploitationMult()
	if strings.Contains(ot, "graveyard") && (strings.Contains(ot, "onto the battlefield") || strings.Contains(ot, "return")) {
		gyTargets := 0
		for _, gc := range gs.Seats[seatIdx].Graveyard {
			if gc != nil && gameengine.ManaCostOf(gc) >= 4 {
				gyTargets++
			}
		}
		// DNA GraveyardExploitation: high → boost recursion-activation
		// score (route reanimator decks to fire activations sooner).
		base += (0.25 + float64(gyTargets)*0.10) * gyMult
		if base > 0.60 {
			base = 0.60
		}
	}
	if strings.Contains(ot, "haste") {
		sickCount := 0
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p != nil && p.IsCreature() && p.SummoningSick {
				sickCount++
			}
		}
		if sickCount > 0 {
			base += 0.20 + float64(sickCount)*0.05
		}
	}

	// Sacrifice outlets: score based on full engine density. Each death-payoff
	// type (drain, draw, ramp, recursion) stacks, and token fodder availability
	// multiplies the value. A fully assembled aristocrats engine (outlet + 3
	// payoffs + tokens) should aggressively sacrifice.
	if strings.Contains(ot, "sacrifice") {
		deathPayoffs := 0
		drainPayoffs := 0
		drawPayoffs := 0
		rampPayoffs := 0
		tokenCount := 0
		fodderCount := 0
		for _, p := range gs.Seats[seatIdx].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			pot := gameengine.OracleTextLower(p.Card)
			isDeath := strings.Contains(pot, "whenever") && (strings.Contains(pot, "dies") || strings.Contains(pot, "leaves"))
			if isDeath {
				deathPayoffs++
				if strings.Contains(pot, "lose") || strings.Contains(pot, "drain") || strings.Contains(pot, "life") {
					drainPayoffs++
				}
				if strings.Contains(pot, "draw") || strings.Contains(pot, "scry") {
					drawPayoffs++
				}
				if strings.Contains(pot, "add") || strings.Contains(pot, "treasure") || strings.Contains(pot, "mana") {
					rampPayoffs++
				}
			}
			isToken := false
			for _, t := range p.Card.Types {
				if t == "token" {
					isToken = true
					break
				}
			}
			if isToken {
				tokenCount++
			}
			if p.IsCreature() && (isToken || gameengine.ManaCostOf(p.Card) <= 2) {
				fodderCount++
			}
		}
		payoffBonus := float64(deathPayoffs) * 0.20
		if drainPayoffs > 0 {
			payoffBonus += 0.15
		}
		if drawPayoffs > 0 {
			payoffBonus += 0.15
		}
		if rampPayoffs > 0 {
			payoffBonus += 0.10
		}
		if fodderCount >= 2 {
			payoffBonus *= 1.5
		} else if tokenCount > 0 {
			payoffBonus += 0.10
		}
		if payoffBonus > 0.80 {
			payoffBonus = 0.80
		}
		// DNA DrainAffinity: high → lean harder into the aristocrats
		// payoff bonus (max +60% at the extreme); low → underweight it.
		// Operates in lockstep with the centralized DrainEngine eval-
		// weight nudge so the activation site biases the SAME direction
		// the planner is biased.
		drainShift := (h.curseAxis(func(d *CurseDNA) float64 { return d.DrainAffinity }, 0.5) - 0.5) * 1.2
		payoffBonus *= 1.0 + drainShift
		base += payoffBonus
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

	// Equipment equip: score based on best available target quality plus
	// recurrence-engine and connect-payoff signals. Cheap equip costs on
	// death-trigger equipment (Skullclamp pattern) and connect-trigger
	// equipment with evasive carriers (Sword cycle pattern) are loops we
	// want to activate every turn.
	ot2 := gameengine.OracleTextLower(c)
	if strings.Contains(ot2, "equip") && opt.Permanent != nil && opt.Permanent.IsEquipment() {
		equipMult := h.equipmentAffinityMult()
		equipScore := scoreEquipTarget(gs, seatIdx, opt.Permanent, nil)
		if equipScore > 0 {
			base += float64(equipScore) * 0.015 * equipMult
		}
		hasConnect := strings.Contains(ot2, "deals combat damage")
		hasDeath := strings.Contains(ot2, "equipped creature dies") ||
			strings.Contains(ot2, "whenever equipped creature dies")
		if hasConnect || hasDeath {
			base += 0.20 * equipMult
		}
		equipCost := equipCostFromText(ot2)
		// Skullclamp-style recurrence engine: cheap to equip + death
		// trigger = grindable loop. Bias us toward activating it
		// whenever there's a disposable target available.
		if hasDeath && equipCost >= 0 && equipCost <= 2 {
			base += 0.15
		}
		// Connect-payoff with cheap equip cost is a Sword-cycle loop;
		// we'd happily move the sword each turn to whichever attacker
		// connects.
		if hasConnect && equipCost >= 0 && equipCost <= 2 {
			base += 0.10
		}
	}

	if h.tutorTargetSet[c.DisplayName()] {
		base += 0.10
	}

	return base
}

// lethalAttackTarget returns the seat index of an opponent who can be killed
// this turn by attacking with `legal`, modelling the defender's MINIMUM
// rational blocks rather than the old over-optimistic / over-pessimistic
// approximations. Returns -1 when no opponent is lethal under optimal blocking.
//
// Damage classification per attacker (power × double-strike):
//   - "hard" evasion (unblockable / shadow / horsemanship) connects no matter
//     what the defender fields;
//   - "flying" connects unless the defender has a flying/reach blocker;
//   - everything else (incl. menace/fear, treated conservatively as
//     ground-blockable) can be chump-blocked by any untapped creature.
//
// The defender is assumed to block optimally to minimize damage that connects:
// it spends air blockers on the biggest fliers first, then any remaining
// untapped creatures on the biggest ground attackers. The residual that gets
// through must already kill for the seat to be a lethal target. This fixes
// both the old over-optimistic "all evasive power connects" (ignored
// flying/reach blockers) and the over-pessimistic "subtract every blocker's
// toughness" (a single chumped blocker doesn't absorb its toughness in power).
func lethalAttackTarget(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) int {
	type atk struct {
		dmg    int
		hard   bool // always connects
		flying bool // connects unless a flier/reacher is available
	}
	atks := make([]atk, 0, len(legal))
	for _, p := range legal {
		if p == nil {
			continue
		}
		pw := gs.PowerOf(p)
		if pw <= 0 {
			continue
		}
		mul := 1
		if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
			mul = 2
		}
		a := atk{dmg: pw * mul}
		if p.HasKeyword("unblockable") || p.HasKeyword("shadow") || p.HasKeyword("horsemanship") {
			a.hard = true
		} else if p.HasKeyword("flying") {
			a.flying = true
		}
		atks = append(atks, a)
	}
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		groundBlockers := 0 // any untapped creature can block a ground attacker
		airBlockers := 0    // fliers / reachers can additionally block fliers
		for _, bp := range s.Battlefield {
			if bp == nil || !bp.IsCreature() || bp.Tapped {
				continue
			}
			groundBlockers++
			if bp.HasKeyword("flying") || bp.HasKeyword("reach") {
				airBlockers++
			}
		}
		hardDmg := 0
		var flyingDmgs, groundDmgs []int
		for _, a := range atks {
			switch {
			case a.hard:
				hardDmg += a.dmg
			case a.flying:
				flyingDmgs = append(flyingDmgs, a.dmg)
			default:
				groundDmgs = append(groundDmgs, a.dmg)
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(flyingDmgs)))
		sort.Sort(sort.Reverse(sort.IntSlice(groundDmgs)))
		connected := hardDmg
		air := airBlockers
		for _, fd := range flyingDmgs {
			if air > 0 {
				air-- // this flier is blocked, deals 0 to the player
				continue
			}
			connected += fd
		}
		// Air blockers spent on fliers are also ground creatures, so they're
		// drawn from the shared untapped pool — subtract them from ground.
		spentOnFliers := airBlockers - air
		ground := groundBlockers - spentOnFliers
		if ground < 0 {
			ground = 0
		}
		for _, gd := range groundDmgs {
			if ground > 0 {
				ground-- // chump-blocked, deals 0 to the player
				continue
			}
			connected += gd
		}
		if connected > 0 && connected >= s.Life {
			return i
		}
	}
	return -1
}

// -- Interface: ChooseAttackers --

func (h *YggdrasilHat) ChooseAttackers(gs *gameengine.GameState, seatIdx int, legal []*gameengine.Permanent) []*gameengine.Permanent {
	h.explorationFactor(gs, seatIdx)

	if len(legal) == 0 {
		return nil
	}
	h.recordParentTier(h.classifyDecision(gs), gs.Turn)

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
	// DNA Aggression: high aggression lowers the thresholds for attacking
	// (more willing to swing), low aggression raises them (more cautious).
	// Neutral (0.5) = no change. Max nudge: +/- 0.15 on thresholds.
	if h.DNA != nil {
		aggroShift := (h.DNA.Aggression - 0.5) * 0.3 // [-0.15, +0.15]
		aheadThresh -= aggroShift                    // high aggro → lower threshold to trigger "ahead" attacks
		behindThresh -= aggroShift                   // high aggro → lower threshold to trigger "behind" selectivity
		aheadVal -= aggroShift * 0.5                 // high aggro → attack with less advantage needed
		behindVal -= aggroShift * 0.5                // high aggro → less cautious when behind
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

	// Phase-of-game shift on attack threshold:
	//   Deploy  → +0.15 (conservative; develop board over chip damage)
	//   Develop → 0.0   (default behavior)
	//   Execute → -0.10 (aggressive; lower bar to swing)
	//
	// R60 round 5 — combat-first archetypes (Aggro / Burn / Voltron /
	// Tribal) INVERT the deploy bump. Their gameplan IS early damage:
	// a turn-2 Goblin Guide HOLDing because "we should develop more
	// board" is exactly the bug the audit surfaced. They get a small
	// -0.05 shift instead so a 1/1 (val=0.10) on turn 2 still clears
	// the swing bar.
	combatFirst := isCombatFirstArchetype(h.Strategy)
	switch h.detectPhase(gs, seatIdx) {
	case PhaseDeploy:
		if combatFirst {
			threshold -= 0.05
			stance += "+DEPLOY-AGGRO"
		} else {
			threshold += 0.15
			stance += "+DEPLOY"
		}
	case PhaseExecute:
		threshold -= 0.10
		stance += "+EXECUTE"
	}

	// Opponent-archetype clock bias: race confident aggro pods (lower
	// the swing threshold so we keep pace) and stay cautious into
	// confident control pods (raise it so we don't dump value into
	// removal). Effect is bounded: ±0.05 per archetype found at the
	// table, scanned once.
	racingAggro := false
	dodgingControl := false
	for i := range gs.Seats {
		if i == seatIdx {
			continue
		}
		prof := h.classifyOpponent(gs, i)
		if prof == nil || prof.Confidence < 0.55 {
			continue
		}
		switch prof.Archetype {
		case "aggro":
			racingAggro = true
		case "control":
			dodgingControl = true
		}
	}
	if racingAggro {
		threshold -= 0.05
		stance += "/race-aggro"
	}
	if dodgingControl {
		threshold += 0.05
		stance += "/dodge-control"
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
		if i != seatIdx && h.opponentLikelyHasWrath(gs, i) > 0.35 {
			wrathSuspected = true
			break
		}
	}

	// Lethal detection — model the defender's ACTUAL blocks rather than the
	// old over-optimistic "all evasive power connects" / over-pessimistic
	// "subtract every blocker's toughness" approximations (r61 item #6).
	lethalTarget := lethalAttackTarget(gs, seatIdx, legal)
	if lethalTarget >= 0 {
		h.logf("%s LETHAL DETECTED on seat %d — sending everything",
			roundTag(gs, seatIdx), lethalTarget)
		var all []*gameengine.Permanent
		for _, p := range legal {
			if p != nil && gs.PowerOf(p) > 0 {
				all = append(all, p)
			}
		}
		return all
	}

	// Defender deathtouch density. canSwingProfitably already binary-prunes
	// attackers that have no safe lane, but the value loop didn't soften
	// the bonus when the worst-case opponent fields several untapped
	// deathtouch blockers. Against e.g. a Spider tribal or Vraska's
	// tokens, every non-deathtouch / non-first-strike / non-trample /
	// non-indestructible swing is a one-shot trade — borderline attackers
	// shouldn't get the same evasion/keyword bonuses that nudge them over
	// the threshold. We track the MAX untapped-DT count across opponents
	// because the attacker only swings into one defender, but bestTarget
	// will pick the lane that minimizes resistance, so density is the
	// pessimistic case and acts as a soft brake rather than a hard prune.
	maxDeathtouchDensity := 0
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		dt := 0
		for _, b := range s.Battlefield {
			if b != nil && b.IsCreature() && !b.Tapped && b.HasKeyword("deathtouch") &&
				gs.PowerOf(b) > 0 {
				dt++
			}
		}
		if dt > maxDeathtouchDensity {
			maxDeathtouchDensity = dt
		}
	}

	// R60 round 5 — open-lane detection. If at least one opponent has
	// ZERO untapped potential blockers, every attacker gets a "damage
	// is free" bonus. Bonus scales with archetype: combat-first decks
	// (Aggro / Burn / Voltron / Tribal) treat open lanes as their PLAN
	// and get the full +0.20; others get +0.10 because chip damage is
	// still positive but less central to their gameplan. Pre-R60r5 a
	// turn-3 vanilla 1/1 (val=0.10) staring at an empty seat held its
	// ground because the score sat below the deploy-phase threshold —
	// that's the early-game-too-defensive failure the audit flagged.
	openLaneBonus := 0.0
	if anyOpponentHasOpenLane(gs, seatIdx) {
		if combatFirst {
			openLaneBonus = 0.20
		} else {
			openLaneBonus = 0.10
		}
	}

	// R60 round 12+ chain-attack awareness (see chain_attack_signals_r60.go).
	// Pending-extra-combats: read the engine's FIFO queue. Anticipated:
	// scan the legal pool for an "additional combat" trigger source about
	// to swing this combat. Both bonuses encourage committing chip-damage
	// attackers when multiple combats are queued or imminent.
	chainBonus := chainAttackPendingBonus(gs) + chainAttackAnticipationBonus(legal)

	h.logf("%s ATTACK seat=%d pos=%.3f stance=%s threshold=%.2f legal=%d wrath=%v dt-density=%d open-lane=%.2f chain=%.2f",
		roundTag(gs, seatIdx), seatIdx, pos, stance, threshold, len(legal), wrathSuspected, maxDeathtouchDensity, openLaneBonus, chainBonus)

	var attackers []*gameengine.Permanent
	for _, p := range legal {
		if p == nil {
			continue
		}
		pw := gs.PowerOf(p)
		if pw <= 0 {
			continue
		}
		val := float64(pw) / 10.0
		// R60 round 5 — open-lane bonus. Applied universally to every
		// attacker (vanilla 1/1s included) when a defenseless seat
		// exists; the bonus is sized so it can lift a marginal attacker
		// over the deploy-phase threshold without overriding the
		// profitability prune that handles bad trades downstream.
		val += openLaneBonus
		// R60 round 12+ chain-attack horizon bonus. Same per-attacker
		// shape as openLaneBonus — fold the queue-aware and anticipated
		// extra-combat awareness into every attacker's commit score.
		val += chainBonus
		if p.HasKeyword("deathtouch") {
			val += 0.3
		}
		if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
			val += 0.2
		} else if p.HasKeyword("first strike") || p.HasKeyword("first_strike") {
			// First-strikers kill blockers in the §510.5 step before
			// taking return damage, so they're noticeably safer to send
			// into a contested board than vanilla creatures of the same
			// P/T. Less impact than double strike (no extra damage), but
			// still meaningfully tilts marginal attack decisions.
			val += 0.15
		}
		if p.HasKeyword("lifelink") {
			val += 0.1
		}
		// Vigilance — no defensive downside to attacking; the creature is
		// still up to block on opponents' turns.
		if p.HasKeyword("vigilance") {
			val += 0.10
		}
		// Indestructible — survives every combat trade. Removal-by-block
		// doesn't work, so the threat keeps reapplying turn after turn.
		if p.HasKeyword("indestructible") {
			val += 0.20
		}
		// Sustained-threat keywords — opponents can't snipe the attacker
		// with a targeted removal spell, so the value of getting it into
		// combat compounds across turns.
		if p.HasKeyword("hexproof") || p.HasKeyword("shroud") {
			val += 0.10
		} else if p.HasKeyword("ward") {
			val += 0.05
		}
		// Protection (from any color) — many opponents' removal/blockers
		// are colored, so attackers with protection live longer in combat
		// and are harder to interact with at instant speed.
		if hasAnyProtection(p) {
			val += 0.10
		}
		// Evasion bonus — creatures that connect reliably are worth sending.
		// Flying is downgraded if any opponent has a reach/flying blocker
		// available; the lane isn't actually open in that case.
		evasive := false
		if p.HasKeyword("unblockable") || p.HasKeyword("shadow") || p.HasKeyword("horsemanship") {
			val += 0.25
			evasive = true
		} else if p.HasKeyword("flying") || p.HasKeyword("fear") || p.HasKeyword("intimidate") || p.HasKeyword("skulk") {
			flyBonus := 0.15
			if p.HasKeyword("flying") && anyOpponentHasReachOrFlyingBlocker(gs, seatIdx) {
				flyBonus = 0.05
			}
			val += flyBonus
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
		// R60 — trick attacks. Creatures whose death IS the value (die-
		// triggers like Hangarback Walker / Doomed Traveler, persist /
		// undying / unearth recursion, damage-taken triggers) should
		// swing INTO clean blocks — the trade is the engine. Adds a
		// fixed bump so the value loop tilts them above the threshold;
		// the strategic-shield below also bypasses the lose-to-clean-
		// block prune for the same set.
		if p.Card != nil && hasDeathPayoffValue(p.Card) {
			val += 0.20
		}
		// R60 — intentional chump. Cheap (≤2 CMC) attackers when a sac
		// outlet is on the board: the block becomes a tempo trade we
		// choose to take (Phyrexian Altar mana, Goblin Bombardment
		// damage, Viscera Seer scry on the would-be casualty).
		if hasSacFuelValue(gs, seatIdx, p) {
			val += 0.15
		}
		// Commander damage matters — always worth sending.
		if p.Card != nil && isCommanderCard(gs, seatIdx, p.Card) {
			val += 0.10
			// Attack-trigger tutor commanders (Zur, Narset, etc.)
			// get a massive bonus — the tutor value far outweighs
			// combat risk. The whole deck is built around this trigger.
			ot := gameengine.OracleTextLower(p.Card)
			if strings.Contains(ot, "attacks") &&
				(strings.Contains(ot, "search") || strings.Contains(ot, "exile the top") ||
					strings.Contains(ot, "look at the top")) {
				val += 0.60
			}
		}
		// R60 round 9+ self-attack-trigger bonus (see
		// attack_trigger_value_r60.go). Covers the Hellrider / Maraxus /
		// Goldspan / Lord-of-the-Forsaken family — non-commander attack-
		// triggers and non-tutor commander attack-triggers (pump, draw,
		// damage, drain, treasure) that the audit found were all scoring 0.
		// Graduated bonus by payoff category; capped well below the
		// commander-tutor +0.60 so it lifts marginal attackers without
		// drowning out the profitability prune downstream.
		if p.Card != nil {
			val += attackTriggerBonus(p.Card)
		}
		// R60 round 10+ postcombat-trigger bonus (see
		// postcombat_trigger_value_r60.go). Disjoint family from
		// attackTriggerBonus: "deals combat damage to a player" (Toski,
		// Bident bearers), "additional combat phase" (Aurelia, Scourge,
		// Combat Celebrant), "at the end of combat" generic, and
		// "becomes blocked" pumps. Both helpers can fire on the same
		// attacker when the card has both an attack trigger AND a
		// postcombat trigger (rare — e.g. Maelstrom Wanderer), which
		// correctly compounds the value.
		if p.Card != nil {
			val += postcombatTriggerBonus(p.Card)
		}

		// 3rd Eye: When a wrath is suspected, hold back VE key creatures
		// to preserve board presence post-wipe. Only applies when we're
		// not desperate (ahead or neutral).
		if wrathSuspected && relPos > -0.2 && p.Card != nil && h.isValueEngineKey(p.Card) {
			val -= 0.15
		}
		// R60 round 11+ counter-balance (see attack_into_wipe_signals_r60.go):
		// the inverse case — sometimes attacking INTO the wipe is correct.
		// Expendable creatures generate free chip damage before dying;
		// triggered creatures capture their use-it-or-lose-it payoff
		// before the wipe takes the body. Both gated on the same
		// wrathSuspected + relPos > -0.2 envelope as the hold-back above.
		val += h.wipeBaitExpendableBonus(gs, seatIdx, p, wrathSuspected, relPos)
		val += useItOrLoseItTriggerBonus(p, wrathSuspected, relPos)

		// Deathtouch density brake. A single untapped DT blocker is one
		// safe lane lost; two or more start to gate the entire turn. The
		// attacker dodges the penalty if it ignores DT outright —
		// deathtouch (we trade up by definition), first/double strike
		// (kills the DT body in 510.5 before being bitten), trample
		// (excess leaks past the chump), or indestructible (DT damage
		// can't kill us). Caps at -0.20 — even a board of 4 DT spiders
		// shouldn't push a real threat off the table by itself; this
		// nudges marginal attackers below the swing threshold.
		ignoresDT := p.HasKeyword("deathtouch") || p.HasKeyword("trample") ||
			p.HasKeyword("indestructible") ||
			p.HasKeyword("first strike") || p.HasKeyword("first_strike") ||
			p.HasKeyword("double strike") || p.HasKeyword("double_strike")
		if maxDeathtouchDensity > 0 && !ignoresDT {
			penalty := 0.05 * float64(maxDeathtouchDensity)
			if penalty > 0.20 {
				penalty = 0.20
			}
			val -= penalty
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

	// Profitability prune: drop attackers that lose to a clean block on
	// every legal target. Skip in desperation mode (relPos very low),
	// when stance is ALL-IN, or when the creature is a strategic asset
	// (commander, value engine, combo piece). Lethal-swing path returns
	// early above, so we don't gate on it here.
	if relPos > -0.5 && !strings.Contains(stance, "ALL-IN") && len(attackers) > 0 {
		opponents := make([]*gameengine.Seat, 0, len(gs.Seats)-1)
		for i, s := range gs.Seats {
			if i == seatIdx || s == nil || s.Lost || s.LeftGame {
				continue
			}
			opponents = append(opponents, s)
		}
		if len(opponents) > 0 {
			kept := attackers[:0]
			for _, p := range attackers {
				keep := true
				if p != nil && p.Card != nil {
					// Commanders used to be auto-strategic (never pruned).
					// That blanket protection lost games: a vanilla 2/2
					// commander clean-trading into a 5/5 blocker also
					// costs {2} extra on recast (CR §903.8 — commander
					// tax compounds on every recast from the command
					// zone), so the swing is *worse* than for a non-
					// commander attacker — we get 0 damage AND owe two
					// more mana to redeploy. Keep the strategic shield
					// only when the commander is doing something
					// non-vanilla: an attack-trigger (Zur, Narset,
					// Etali) where the trigger value outweighs the
					// trade, or a commander-damage clock that's close
					// enough to lethal that one more poke matters.
					strategic := h.isValueEngineKey(p.Card) ||
						h.isComboRelevant(p.Card)
					if isCommanderCard(gs, seatIdx, p.Card) {
						if hasAttackTriggerValue(p.Card) ||
							commanderClockNearLethal(gs, seatIdx, p.Card, 13) {
							strategic = true
						}
					}
					// R60 trick-attack shield. Death-payoff and sac-fuel
					// attackers WANT a clean block — pruning them as
					// "would die on every target" loses the engine. Both
					// signals are oracle / battlefield scoped; see
					// hasDeathPayoffValue + hasSacFuelValue for the shape.
					if hasDeathPayoffValue(p.Card) || hasSacFuelValue(gs, seatIdx, p) {
						strategic = true
					}
					if !strategic && !canSwingProfitably(gs, p, opponents) {
						keep = false
						reason := "would die to clean block on every target"
						if isCommanderCard(gs, seatIdx, p.Card) {
							reason += " (+{2} commander tax)"
						}
						h.logf("  PRUNE: %s %s",
							p.Card.DisplayName(), reason)
					}
				}
				if keep {
					kept = append(kept, p)
				}
			}
			attackers = kept
		}
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
				// Never hold back attack-trigger tutor commanders.
				ot := gameengine.OracleTextLower(p.Card)
				if strings.Contains(ot, "attacks") &&
					(strings.Contains(ot, "search") || strings.Contains(ot, "exile the top")) {
					rv = -10.0
				}
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

// simulateBlockerTrade resolves a single attacker-vs-blocker trade
// taking first strike, double strike, deathtouch, and indestructible
// into account, and returns whether each side dies. Used by tests
// that need to verify combat outcomes without spinning up the full
// engine combat phase.
//
// Damage steps:
//  1. First-strike step: any creature with FS or DS deals damage. If
//     either side dies here, it deals no damage in step 2.
//  2. Regular step: surviving creatures with no FS (or with DS) deal
//     damage. Deathtouch makes any non-zero damage lethal.
//
// Indestructible immunity is applied at the end of each step.
func simulateBlockerTrade(gs *gameengine.GameState, atk, blk *gameengine.Permanent) (attackerDies, blockerDies bool) {
	if atk == nil || blk == nil {
		return false, false
	}
	atkPow := gs.PowerOf(atk)
	if atkPow < 0 {
		atkPow = 0
	}
	blkPow := gs.PowerOf(blk)
	if blkPow < 0 {
		blkPow = 0
	}
	atkTou := gs.ToughnessOf(atk) - atk.MarkedDamage
	blkTou := gs.ToughnessOf(blk) - blk.MarkedDamage

	atkFS := atk.HasKeyword("first strike") || atk.HasKeyword("first_strike")
	atkDS := atk.HasKeyword("double strike") || atk.HasKeyword("double_strike")
	atkDT := atk.HasKeyword("deathtouch")
	atkIndest := atk.HasKeyword("indestructible")

	blkFS := blk.HasKeyword("first strike") || blk.HasKeyword("first_strike")
	blkDS := blk.HasKeyword("double strike") || blk.HasKeyword("double_strike")
	blkDT := blk.HasKeyword("deathtouch")
	blkIndest := blk.HasKeyword("indestructible")

	// Step 1: first-strike damage.
	atkStep1, blkStep1 := 0, 0
	if atkFS || atkDS {
		blkStep1 = atkPow
	}
	if blkFS || blkDS {
		atkStep1 = blkPow
	}
	atkLost := atkStep1
	blkLost := blkStep1
	atkKilled := !atkIndest && (atkLost >= atkTou || (blkDT && atkLost >= 1))
	blkKilled := !blkIndest && (blkLost >= blkTou || (atkDT && blkLost >= 1))

	// Step 2: regular damage. Dead creatures deal no damage. Creatures
	// with DS swing again; creatures with only FS do not.
	if !atkKilled && (!atkFS || atkDS) {
		blkLost += atkPow
	}
	if !blkKilled && (!blkFS || blkDS) {
		atkLost += blkPow
	}
	if !atkIndest {
		atkKilled = atkLost >= atkTou || (blkDT && atkLost >= 1)
	}
	if !blkIndest {
		blkKilled = blkLost >= blkTou || (atkDT && blkLost >= 1)
	}
	return atkKilled, blkKilled
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

	// Calculate incoming damage and effective life-swing pressure.
	// Lifelink doubles the swing (we lose N + they gain N). Infect
	// converts damage into poison counters — treat 1 poison ≈ 2 life
	// (10 poison kills, 20 life kills). Annihilator forces sacrifices
	// that aren't life damage but ARE catastrophic; we surface that as
	// a "must-block" flag rather than mixing it into the life math.
	// Commander damage (CR §704.6c — 21 from a single commander = loss)
	// gets the same must-block treatment per attacker.
	incoming := 0
	myPoison := seat.PoisonCounters
	addedPoison := 0
	existentialCommander := false
	for _, a := range attackers {
		if a == nil {
			continue
		}
		mul := 1
		if a.HasKeyword("double strike") || a.HasKeyword("double_strike") {
			mul = 2
		}
		dmg := gs.PowerOf(a) * mul
		if dmg < 0 {
			dmg = 0
		}
		switch {
		case a.HasKeyword("infect") || a.HasKeyword("toxic") || a.HasKeyword("poisonous"):
			// Damage from infect goes to poison instead of life.
			// Convert into a life-equivalent so the survival check still
			// fires (1 poison ≈ 2 life since 10 kills vs 20 kills).
			addedPoison += dmg
			incoming += dmg * 2
		case a.HasKeyword("lifelink"):
			// Effective swing is 2x — they gain, we lose.
			incoming += dmg * 2
		default:
			incoming += dmg
		}
		// Commander-damage clock — would this swing reach 21?
		if a.Card != nil && gameengine.IsCommanderCard(gs, a.Controller, a.Card) {
			clock := 0
			if byDealer, ok := seat.CommanderDamage[a.Controller]; ok {
				clock = byDealer[a.Card.DisplayName()]
			}
			if clock+dmg >= 21 {
				existentialCommander = true
			}
		}
	}
	// Treat being one poison hit from death the same as being lethaled.
	if myPoison+addedPoison >= 10 {
		incoming = seat.Life + 1
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
	// DNA Aggression (inverse): high aggression → less willing to
	// block (prefer racing). Lower aheadNoBlock means a smaller lead
	// suffices to skip blocks; lower survivalFrac means a larger
	// damage budget before forced to block. Both push toward racing.
	// Max swing: aheadNoBlock ±0.15, survivalFrac ±1 (clamped to ≥1).
	aggroShift := (h.curseAxis(func(d *CurseDNA) float64 { return d.Aggression }, 0.5) - 0.5) * 0.3
	aheadNoBlock -= aggroShift
	if aheadNoBlock < 0.05 {
		aheadNoBlock = 0.05
	}
	if aheadNoBlock > 0.95 {
		aheadNoBlock = 0.95
	}
	// Aggression > 0.5 → reduce survivalFrac (skip more); < 0.5 →
	// increase (block more). Translate ±0.5 axis to ±1 integer step.
	survivalFrac -= int((h.curseAxis(func(d *CurseDNA) float64 { return d.Aggression }, 0.5) - 0.5) * 2.0)
	if survivalFrac < 1 {
		survivalFrac = 1
	}
	// "Comfortably ahead" no longer skips ALL blocking. The old guard
	// returned an empty map whenever relPos was high enough and total
	// incoming was below life/survivalFrac, so a 40-life, slightly-ahead
	// hat would just eat 4/4 (or 19-damage) hits even when a free chump
	// trade was available. We now keep this as a per-attacker hint:
	// individual attackers get skipped further down only when there's
	// no survivor AND no favorable trade — favorable trades (blocker
	// strictly lighter than the attacker) ALWAYS go through, even when
	// ahead. The `existentialCommander` flag computed above is still
	// honored implicitly: it forces willDie=true so value creatures
	// stay in the pool for must-block trades against a 21-clock
	// commander.
	aheadAndComfortable := relPos > aheadNoBlock && incoming < seat.Life/survivalFrac
	_ = aheadAndComfortable // currently advisory; per-attacker logic below
	// is already conservative enough on its own. Retained as a named
	// expression so future skip heuristics can reuse it without
	// re-deriving the threshold.

	// Pool of legal blockers — exclude combo/value creatures from trades
	// unless we'll die without blocking (life or commander-damage path).
	willDie := seat.Life-incoming <= 0 || existentialCommander
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
		ranks = append(ranks, rank{a, -attackerRank(gs, a)})
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
			if !used[b] && gameengine.CanBlockGS(gs, atk, b) {
				legal = append(legal, b)
			}
		}
		if len(legal) == 0 {
			continue
		}

		willDieIfUnblocked := life-incoming <= 0
		atkDT := atk.HasKeyword("deathtouch")
		atkDS := atk.HasKeyword("double strike") || atk.HasKeyword("double_strike")
		// First/double strike on the attacker — used by the deathtouch
		// trade-up + trample chump-skip branches below.
		atkFS := atk.HasKeyword("first strike") || atk.HasKeyword("first_strike") || atkDS
		atkPow := gs.PowerOf(atk)
		atkTou := gs.ToughnessOf(atk)
		atkInfect := atk.HasKeyword("infect") || atk.HasKeyword("toxic") || atk.HasKeyword("poisonous")
		// Annihilator and infect attackers are effectively must-block (any
		// unblocked hit is catastrophic). Force the must-block path even
		// when raw life damage isn't lethal.
		mustBlock := false
		if gameengine.GetAnnihilatorN(atk) > 0 {
			mustBlock = true
		}
		if atkInfect {
			mustBlock = true
		}
		// Commander-damage check (CR §704.6c). If THIS attacker is a
		// commander whose clock + this swing's damage hits 21, treat as
		// lethal regardless of remaining life. Uses the post-double-strike
		// damage projection to match how the engine accumulates damage.
		if atk.Card != nil && gameengine.IsCommanderCard(gs, atk.Controller, atk.Card) {
			clock := 0
			if byDealer, ok := seat.CommanderDamage[atk.Controller]; ok {
				clock = byDealer[atk.Card.DisplayName()]
			}
			swing := atkPow
			if atkDS {
				swing = atkPow * 2
			}
			if clock+swing >= 21 {
				mustBlock = true
			}
		}

		// Find survivors (blockers that outlive the attacker after the
		// trade resolves). With double strike on the attacker, the
		// blocker effectively eats `power * 2` damage unless our blocker
		// has first/double strike and can kill the attacker before the
		// regular damage step. Likewise, our deathtouch blocker survives
		// any attacker without first/double strike.
		//
		// R60 round 8+ combat-trick awareness (see
		// block_priority_signals_r60.go). Two signals reshape the survivor
		// pool: hasTrick → rescue otherwise-dead blockers; oppTrick →
		// require a 1-toughness buffer for marginal survivors.
		hasTrick := h.hasAffordableDefensiveTrick(gs, seatIdx)
		oppTrick := h.oppHasCombatTrickMana(gs, seatIdx)
		var survivors []*gameengine.Permanent
		for _, b := range legal {
			if b == nil {
				continue
			}
			bTou := gs.ToughnessOf(b) - b.MarkedDamage
			bPow := gs.PowerOf(b)
			bDT := b.HasKeyword("deathtouch")
			bFS := b.HasKeyword("first strike") || b.HasKeyword("first_strike")
			bDS := b.HasKeyword("double strike") || b.HasKeyword("double_strike")

			// Does the blocker kill the attacker in the first-strike
			// step? (Blocker has FS/DS and attacker lacks FS/DS, AND
			// blocker's damage suffices — deathtouch makes 1 damage do.)
			killsInFirstStrike := false
			if (bFS || bDS) && !atkDS {
				if bPow >= atkTou || (bDT && bPow >= 1) {
					killsInFirstStrike = true
				}
			}

			// How much damage does the blocker take?
			incomingToBlocker := atkPow
			if atkDS {
				incomingToBlocker = atkPow * 2
			}
			if killsInFirstStrike {
				incomingToBlocker = 0
			}

			// Deathtouch attacker kills any blocker that takes ≥1 damage,
			// EXCEPT when the blocker is indestructible — CR §702.12b says
			// damage doesn't destroy indestructible permanents, including
			// damage from deathtouch sources (§702.2c only marks lethal
			// damage; SBAs don't destroy indestructibles).
			bIndestructible := b.HasKeyword("indestructible")
			if bIndestructible {
				survivors = append(survivors, b)
				continue
			}
			if atkDT && incomingToBlocker >= 1 {
				// Signal A doesn't rescue against deathtouch — pump tricks
				// don't escape the DT trigger, and regen-vs-DT is
				// case-specific. Skip to next candidate.
				continue
			}
			// Survivor margin: how many toughness over lethal. Signal B
			// requires margin ≥ 2 when opp has combat-trick mana up so a
			// trick that pumps the attacker +1/+1 doesn't kill the blocker.
			margin := bTou - incomingToBlocker
			survives := margin > 0
			requiredMargin := 1
			if oppTrick {
				requiredMargin = 2
			}
			if survives && margin >= requiredMargin {
				survivors = append(survivors, b)
				continue
			}
			// Signal A: trust our defensive trick to rescue a would-die
			// blocker. Gated on hasTrick + !atkDT (handled above).
			if !survives && hasTrick {
				survivors = append(survivors, b)
			}
		}
		// Sort: indestructible first (the unconditional survivor — never
		// dies, so always preferable), then by smallest power+toughness
		// (cheapest creature to commit).
		sort.SliceStable(survivors, func(i, j int) bool {
			si, sj := survivors[i], survivors[j]
			indI := si.HasKeyword("indestructible")
			indJ := sj.HasKeyword("indestructible")
			if indI != indJ {
				return indI
			}
			if gs.PowerOf(si)+gs.ToughnessOf(si) != gs.PowerOf(sj)+gs.ToughnessOf(sj) {
				return gs.PowerOf(si)+gs.ToughnessOf(si) < gs.PowerOf(sj)+gs.ToughnessOf(sj)
			}
			return gs.ToughnessOf(si) < gs.ToughnessOf(sj)
		})

		var chosen []*gameengine.Permanent
		if len(survivors) > 0 {
			chosen = []*gameengine.Permanent{survivors[0]}
		}

		// Favorable-trade fallback: even when no blocker survives and
		// we're not at lethal, throwing a strictly-lighter creature in
		// front of a heavier attacker is always correct (a 1/1 token
		// blocking a 5/5 trades 2 stat-points for 10). We pick the
		// lightest qualifying blocker so we burn the cheapest creature
		// possible. Skipped for 0-power attackers (they kill the
		// blocker for nothing). Combo / value-engine pieces were
		// already filtered out of the pool above when willDie==false.
		//
		// First/double strike caveat (CR §510.5): a non-FS blocker
		// thrown into an FS/DS attacker dies in the first-strike step
		// before delivering damage — the trade is just a creature
		// loss. Require the blocker to either kill the attacker or
		// survive (e.g. indestructible) when the attacker has FS/DS.
		if len(chosen) == 0 && atkPow > 0 {
			atkSum := atkPow + atkTou
			var best *gameengine.Permanent
			bestSum := atkSum
			for _, b := range legal {
				if b == nil {
					continue
				}
				bSum := gs.PowerOf(b) + gs.ToughnessOf(b)
				if bSum >= bestSum {
					continue
				}
				if atkFS {
					aDies, bDies := simulateBlockerTrade(gs, atk, b)
					if bDies && !aDies {
						continue
					}
				}
				best = b
				bestSum = bSum
			}
			if best != nil {
				chosen = []*gameengine.Permanent{best}
			}
		}

		// Lifelink-killshot: an unblocked lifelink attacker is a 2x life
		// swing (we lose N, opp gains N). The favorable-trade fallback
		// above only accepts STRICTLY lighter blockers, so a 4/4 vanilla
		// vs a 4/4 lifelink attacker falls through — we eat 4 damage AND
		// concede 4 life. A parity trade (both die, equal stats) is a
		// life-positive outcome against lifelink even when we lose the
		// body, because we're trading creature-for-creature instead of
		// creature-for-creature-AND-8-life. Require simulateBlockerTrade
		// to confirm the blocker actually kills the attacker; feeding the
		// lifelink with a non-killing block would be strictly worse.
		// Pick the smallest qualifying mutual-killer to minimize the
		// committed-stats loss.
		if len(chosen) == 0 && atkPow > 0 && atk.HasKeyword("lifelink") {
			var best *gameengine.Permanent
			bestSum := 1 << 30
			for _, b := range legal {
				if b == nil || gs.PowerOf(b) <= 0 {
					continue
				}
				aDies, _ := simulateBlockerTrade(gs, atk, b)
				if !aDies {
					continue
				}
				bSum := gs.PowerOf(b) + gs.ToughnessOf(b)
				if bSum < bestSum {
					best = b
					bestSum = bSum
				}
			}
			if best != nil {
				chosen = []*gameengine.Permanent{best}
			}
		}

		if len(chosen) == 0 && (willDieIfUnblocked || mustBlock) {
			// Deathtouch trade-up: prefer a deathtouch blocker that can
			// take down the attacker (any damage is lethal) over a chump.
			// CR §510.5 — if the attacker has first/double strike and our
			// DT blocker LACKS first/double strike, the blocker dies in
			// the FS step before it can deliver the deathtouch hit. So
			// against an FS attacker, only an FS/DS deathtouch blocker
			// is a real trade-up; otherwise it's just a chump.
			var dtTrader *gameengine.Permanent
			if !atkDT {
				for _, b := range legal {
					if b == nil || !b.HasKeyword("deathtouch") || gs.PowerOf(b) < 1 {
						continue
					}
					if atkFS {
						bFS := b.HasKeyword("first strike") || b.HasKeyword("first_strike") ||
							b.HasKeyword("double strike") || b.HasKeyword("double_strike")
						bIndestruct := b.HasKeyword("indestructible")
						// Indestructible DT blocker survives the FS hit
						// regardless and still gets to deliver DT.
						if !bFS && !bIndestruct {
							continue
						}
					}
					dtTrader = b
					break
				}
			}
			// Even when nothing in `legal` is a strict survivor, prefer an
			// indestructible chump — it eats the attack risk-free.
			var indChump *gameengine.Permanent
			for _, b := range legal {
				if b != nil && b.HasKeyword("indestructible") {
					indChump = b
					break
				}
			}
			switch {
			case indChump != nil:
				chosen = []*gameengine.Permanent{indChump}
			case dtTrader != nil:
				chosen = []*gameengine.Permanent{dtTrader}
			default:
				chump := bestChumpBlocker(gs, legal)
				useChump := chump != nil
				// CR §702.19c trample with first/double strike: if the
				// chump can't absorb enough damage to keep us alive, the
				// block burns a creature for nothing. Skip the chump.
				// (Simple-trample wastes are caught by the post-decision
				// trample-leak guard further down, which covers the
				// favorable-trade branch above too — that branch can
				// pick a chump before this one ever runs.)
				if useChump && atk.HasKeyword("trample") && atkFS &&
					!chump.HasKeyword("deathtouch") &&
					!chump.HasKeyword("first strike") && !chump.HasKeyword("first_strike") &&
					!chump.HasKeyword("double strike") && !chump.HasKeyword("double_strike") {
					ap := atkPow
					if atkDS {
						ap *= 2
					}
					absorbed := gs.ToughnessOf(chump) - chump.MarkedDamage
					if absorbed < 0 {
						absorbed = 0
					}
					leak := ap - absorbed
					if leak < 0 {
						leak = 0
					}
					// If the leak still kills us (and the attacker isn't
					// must-block), the chump didn't save us — preserve
					// the creature.
					if life-leak <= 0 && !mustBlock {
						useChump = false
					}
				}
				if useChump {
					chosen = []*gameengine.Permanent{chump}
				}
			}
		}

		// Trample-leak waste guard (CR §702.19, post-decision). Both
		// the favorable-trade fallback and the chump branch can pick a
		// single chump blocker against a trample attacker. When we're
		// at lethal-from-the-leak even AFTER the block (chump dies,
		// excess trample tramples over and kills us) the chump was
		// burned for nothing — the right play is to preserve the body
		// for instant-speed responses or next turn (if a fog/wipe is
		// in hand we'd rather not have committed). Skipped when:
		//   - we're under must-block (annihilator/infect/commander
		//     clock at 21 — those are catastrophic if unblocked even
		//     when we don't die from raw damage),
		//   - the chump survives (irrelevant — chump branch caps stats
		//     well below survivor pool, but defensive check),
		//   - the chosen blocker is gang-sized (handled below),
		//   - the chump has DT/FS/DS or is indestructible (a real
		//     trade-up, not waste — those kill the trampler or take
		//     no damage themselves).
		if len(chosen) == 1 && atk.HasKeyword("trample") && !mustBlock {
			chump := chosen[0]
			ignoresTrample := chump.HasKeyword("deathtouch") ||
				chump.HasKeyword("first strike") || chump.HasKeyword("first_strike") ||
				chump.HasKeyword("double strike") || chump.HasKeyword("double_strike") ||
				chump.HasKeyword("indestructible")
			if !ignoresTrample {
				ap := atkPow
				if atkDS {
					ap *= 2
				}
				absorbed := gs.ToughnessOf(chump) - chump.MarkedDamage
				if absorbed < 0 {
					absorbed = 0
				}
				leak := ap - absorbed
				if leak < 0 {
					leak = 0
				}
				if life-leak <= 0 {
					h.logf("  trample-waste: dropping %s vs %s (leak %d kills us at %d life)",
						chump.Card.DisplayName(), atk.Card.DisplayName(), leak, life)
					chosen = nil
				}
			}
		}

		// Gang-block — multiple blockers combining to kill a dangerous
		// attacker. Only fires when:
		//   - we MUST block (lethal incoming or must-block attacker)
		//   - the attacker is dangerous (raw 4+ power, lifelink/infect/
		//     annihilator/double strike, etc.)
		//   - the current single-blocker choice doesn't already kill
		//     the attacker
		//   - 2 or 3 creatures from the legal pool can combine to kill
		//     it (sum of powers ≥ attacker toughness, attacker not
		//     indestructible)
		// Trades 2-3 small creatures for a single big threat — the
		// classic "wall of bodies" answer to a Voltron commander or a
		// 6/6 trampler that nothing 1v1's profitably.
		gangAlreadyKills := false
		if len(chosen) >= 1 {
			aDies, _ := simulateBlockerTrade(gs, atk, chosen[0])
			gangAlreadyKills = aDies
		}
		if !gangAlreadyKills && (willDieIfUnblocked || mustBlock) {
			dangerous := atkPow >= 4 ||
				atk.HasKeyword("lifelink") ||
				atk.HasKeyword("infect") || atk.HasKeyword("toxic") || atk.HasKeyword("poisonous") ||
				atkDS ||
				gameengine.GetAnnihilatorN(atk) > 0
			if dangerous && !atk.HasKeyword("indestructible") {
				// Greedy: take blockers in descending power so we kill
				// the attacker with the fewest creatures lost. Cap at 3
				// — beyond that the trade gets too expensive even
				// against scary threats.
				byPow := make([]*gameengine.Permanent, 0, len(legal))
				for _, b := range legal {
					if b == nil || gs.PowerOf(b) <= 0 {
						continue
					}
					byPow = append(byPow, b)
				}
				sort.SliceStable(byPow, func(i, j int) bool {
					if gs.PowerOf(byPow[i]) != gs.PowerOf(byPow[j]) {
						return gs.PowerOf(byPow[i]) > gs.PowerOf(byPow[j])
					}
					// Tie-break on cheapest stat-sum so we lose less.
					return gs.PowerOf(byPow[i])+gs.ToughnessOf(byPow[i]) <
						gs.PowerOf(byPow[j])+gs.ToughnessOf(byPow[j])
				})
				gangSum := 0
				var gang []*gameengine.Permanent
				for _, b := range byPow {
					if len(gang) >= 3 {
						break
					}
					gang = append(gang, b)
					gangSum += gs.PowerOf(b)
					if gangSum >= atkTou {
						break
					}
				}
				if gangSum >= atkTou && len(gang) >= 2 {
					chosen = gang
				}
			}
		}

		// Menace: need a second blocker. Layer-aware (gs.HasKeywordOf) so
		// granted menace matches the engine's block sanitizer — see the
		// greedy.go note (deep loki r63b §702.110b granted-menace cluster).
		if len(chosen) > 0 && (atk.HasKeyword("menace") || gs.HasKeywordOf(atk, "menace")) {
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
					return gs.PowerOf(extras[i])+gs.ToughnessOf(extras[i]) < gs.PowerOf(extras[j])+gs.ToughnessOf(extras[j])
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

		// Update incoming for trample accounting. Scale the absorbed
		// damage by the same multiplier we used when adding it (lifelink
		// = 2x, infect = 2x life-equivalent) so partial blocks stay in
		// sync with the inflated `incoming` budget.
		atkDmg := gs.PowerOf(atk)
		if atk.HasKeyword("double strike") || atk.HasKeyword("double_strike") {
			atkDmg *= 2
		}
		swingMul := 1
		if atk.HasKeyword("lifelink") ||
			atk.HasKeyword("infect") || atk.HasKeyword("toxic") || atk.HasKeyword("poisonous") {
			swingMul = 2
		}
		var absorbed int
		if atk.HasKeyword("trample") {
			totalT := 0
			for _, b := range chosen {
				totalT += gs.ToughnessOf(b) - b.MarkedDamage
			}
			leak := atkDmg - totalT
			if leak < 0 {
				leak = 0
			}
			absorbed = atkDmg - leak
		} else {
			absorbed = atkDmg
		}
		incoming -= absorbed * swingMul
	}

	// Persist a single summary row covering the block math: how much was
	// coming in, how much we tanked, how many blockers we committed across
	// all attackers, residual life after the swing. Lets heimdall answer
	// "did the hat trade efficiently?" and "did it chump-block when it
	// shouldn't have?" without re-simulating the combat step.
	blockersCommitted := 0
	attackersBlocked := 0
	for _, blockers := range out {
		if len(blockers) > 0 {
			attackersBlocked++
			blockersCommitted += len(blockers)
		}
	}
	residualLife := seat.Life - incoming
	if incoming < 0 {
		residualLife = seat.Life
	}
	h.emitDecisionEvent(gs, seatIdx, "block", map[string]interface{}{
		"attackers":             len(attackers),
		"attackers_blocked":     attackersBlocked,
		"blockers_committed":    blockersCommitted,
		"residual_incoming":     incoming,
		"life_before":           seat.Life,
		"residual_life":         residualLife,
		"poison_added":          addedPoison,
		"existential_commander": existentialCommander,
	})
	h.logf("BLOCK seat=%d atkrs=%d/%d blockers=%d incoming_left=%d life=%d->%d",
		seatIdx, attackersBlocked, len(attackers), blockersCommitted, incoming, seat.Life, residualLife)
	return out
}

// -- Interface: ChooseResponse --

func (h *YggdrasilHat) ChooseResponse(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if top.Countered {
		return nil
	}
	// R60 round 15+ self-trigger response (see self_trigger_response_r60.go).
	// The general rule "skip self-controlled stack items" holds for
	// spells and activations, but the narrow edge case "my draw trigger
	// + my Underworld Dreams = I die" is correctly handled by countering
	// our own trigger. shouldCounterOwnTrigger gates this tightly —
	// requires a triggered ability source, a damage-on-draw punisher
	// on our board, and projected lethal damage.
	if top.Controller == seatIdx {
		if h.shouldCounterOwnTrigger(gs, seatIdx, top) {
			// Find an affordable counterspell and use it. Mirrors the
			// counter-cast logic below at line ~6402.
			seat := gs.Seats[seatIdx]
			colored := gameengine.AvailableColoredManaEstimate(gs, seat)
			for _, c := range seat.Hand {
				if c != nil && gameengine.CardHasCounterSpell(c) &&
					gameengine.CanPayColoredCost(colored, c) {
					h.logf("SELF-TRIGGER-COUNTER seat=%d top=%v (self-harm via punisher)",
						seatIdx, top.Kind)
					// Emit a structured decision event so post-game
					// audit logs can count self-counter fires (one of
					// the few cases where the hat goes against its own
					// stack item — rare enough that audit-derived
					// counts are the right observability surface).
					sourceName := ""
					if top.Source != nil && top.Source.Card != nil {
						sourceName = top.Source.Card.DisplayName()
					}
					counterName := ""
					if c != nil {
						counterName = c.DisplayName()
					}
					h.emitDecisionEvent(gs, seatIdx, "self_trigger_counter",
						map[string]interface{}{
							"source":  sourceName,
							"counter": counterName,
							"life":    seat.Life,
						})
					// Process-lifetime atomic counter for gauntlet
					// observability (see SelfTriggerCounterFires).
					selfTriggerCounterFires.Add(1)
					return &gameengine.StackItem{
						Card:       c,
						Controller: seatIdx,
					}
				}
			}
		}
		return nil
	}
	// R60r5 — stamp tier keyed by top.ID so that even after a LATER
	// ChooseResponse call overwrites lastParentTier, cascade decisions
	// firing during THIS item's resolution can still recover the
	// correct tier.
	h.recordResponseTier(h.classifyDecision(gs), gs.Turn, top)
	if gameengine.SplitSecondActive(gs) {
		return nil
	}
	if gameengine.OppRestrictsDefenderToSorcerySpeed(gs, seatIdx) {
		return nil
	}

	// R60 defensive signal — lethal-incoming combat with a fog / mass-
	// protection answer in hand. Checked BEFORE the counterspell fast-path
	// because no number of countered spells stops an already-declared
	// attack; the fog/protection IS the answer. The defender only has
	// this priority window because some opponent-controlled item (usually
	// an attack trigger) is on the stack — without that window, the
	// engine wouldn't poll for a response at all.
	if defResp := h.maybeCastDefensiveAnswer(gs, seatIdx); defResp != nil {
		return defResp
	}

	// Fast-path: scan for an affordable counterspell BEFORE running the
	// evaluator. Most seats most of the time have zero counters — this
	// skips the expensive relativePosition call entirely.
	seat := gs.Seats[seatIdx]
	var bestCounter *gameengine.Card
	// R60 color-aware mana gate: a Counterspell {U}{U} needs blue, not
	// just generic. CanPayColoredCost folds the seat's untapped lands
	// (mono-color → Fixed, dual → Flex bitmask) plus current pool into a
	// greedy match against the card's printed pip cost. Falls through to
	// the legacy generic-CMC check via Total when ManaCostString is empty
	// (engine-minted cards / tests without printed costs still work).
	colored := gameengine.AvailableColoredManaEstimate(gs, seat)
	// Among all affordable counterspells, pick the CHEAPEST sufficient one
	// (by CMC, then name for determinism) rather than the first in hand
	// order — so we don't burn a Cryptic Command when a plain Counterspell
	// would do the same job and keep the expensive flexible counter for a
	// situation that actually needs it.
	var bestCounterCMC int
	for _, c := range seat.Hand {
		if c != nil && gameengine.CardHasCounterSpell(c) {
			if gameengine.CanPayColoredCost(colored, c) {
				cmc := gameengine.ManaCostOf(c)
				if bestCounter == nil || cmc < bestCounterCMC ||
					(cmc == bestCounterCMC && c.DisplayName() < bestCounter.DisplayName()) {
					bestCounter = c
					bestCounterCMC = cmc
				}
			}
		}
	}
	if bestCounter == nil {
		// No counter available, but we may still have an instant-speed
		// removal answer for a genuine threat an opponent just deployed or
		// is attacking with (r61 PR-7). This is the proactive reactive game
		// the non-active seat was previously blind to — its only reactive
		// window is this response priority, so a removal instant fired here
		// is the least-invasive way to give the AI instant-speed interaction
		// on opponents' turns. Counters take priority (handled above); this
		// only fires when no counter is in hand. Loop-safe: see
		// maybeCastReactiveRemoval's one-shot guard + the top.Controller==seat
		// gate at the top of ChooseResponse.
		if rmResp := h.maybeCastReactiveRemoval(gs, seatIdx, top); rmResp != nil {
			return rmResp
		}
		return nil
	}

	// R60 priority-window audit — resolve the EFFECTIVE response card
	// once, then drive all downstream heuristics off it. For spells
	// this is identical to top.Card; for triggered / activated
	// abilities (top.Card == nil) it's top.Source.Card — the
	// permanent whose oracle text describes the trigger payload.
	// Pre-R60 every "top.Card != nil" branch silently short-circuited
	// on triggers, leaving the hat blind to Etali attack triggers,
	// Sheoldred drain triggers, Smothering Tithe draw triggers, and
	// the wider "what's resolving right now" question across complex
	// trigger stacks.
	respondCard := effectiveResponseCard(top)
	isTrigger := isAbilityOnStack(top)

	// R60 signal A — "nothing meaningful to interrupt" early pass.
	// Cantrip-shape only applies to SPELLS (a triggered ability that
	// draws a card is the entire payload of its source permanent, not
	// a low-impact value blip — those almost always indicate a value
	// engine worth countering when we have the window).
	if !isTrigger && isLowImpactCantripSpell(respondCard) && !h.isComboRelevant(respondCard) {
		return nil
	}

	score := stackItemScore(top)

	// R60 — stack-depth context. With the stack at 3+ items, the literal
	// top is rarely the meaningful decision: an opponent's counter is
	// aimed at our spell below, removal is timed to interrupt an
	// already-committed line, or a counter-war is mid-flight. The simple
	// `stackItemScore(top)` gate can't see any of that. See
	// `computeStackDepthSignals` for the three signals folded in here.
	depthSig := h.computeStackDepthSignals(gs, seatIdx, top)
	// R60 follow-up to #310 — multi-level stack-resolution awareness.
	// holdForBiggerBelow (defer counter when a higher-threat hostile
	// item sits below; single-counter holders save the response window)
	// and forceCounterShield (top is a Veil-of-Summer-shape protection
	// spell about to lock our counters out of the bigger threats below).
	h.stackResolutionSignals(gs, seatIdx, top, &depthSig)
	if depthSig.scoreBonus > 0 {
		score += depthSig.scoreBonus
	}
	// holdForBiggerBelow short-circuits to pass — saving the single
	// available counter for the larger threat's own priority window.
	// Skipped when mustCounter is set (the helper already gates this,
	// but defense-in-depth here keeps the contract explicit).
	if depthSig.holdForBiggerBelow && !depthSig.mustCounter {
		h.logf("STACK-RESOLUTION seat=%d depth=%d HOLD reason=%s",
			seatIdx, len(gs.Stack), depthSig.reason)
		return nil
	}

	// Always counter combo pieces / "win the game" / mass removal.
	mustCounter := false
	if depthSig.mustCounter {
		mustCounter = true
		topName := "<unknown>"
		if top.Card != nil {
			topName = top.Card.DisplayName()
		}
		h.logf("STACK-DEPTH-RESPONSE seat=%d depth=%d MUST-COUNTER reason=%s top=%s",
			seatIdx, len(gs.Stack), depthSig.reason, topName)
	}
	if respondCard != nil {
		if h.isComboRelevant(respondCard) {
			mustCounter = true
		}
		ot := gameengine.OracleTextLower(respondCard)
		if strings.Contains(ot, "win the game") {
			mustCounter = true
		}
		if (strings.Contains(ot, "destroy all") || strings.Contains(ot, "exile all")) && score >= 1 {
			mustCounter = true
		}
		// R60 signal B — eager counter on routinely game-deciding shapes
		// that the CMC-based score gate would otherwise let through.
		// Extra-turn spells are nearly always a combo finisher or lethal
		// swing setup; tutors at score ≥ 2 are fetching a key piece (cheap
		// 1-mana tutors like Vampiric / Mystical Tutor often slip past
		// the gate but materially advance the caster's win line).
		if strings.Contains(ot, "take an extra turn") ||
			strings.Contains(ot, "take an additional turn") ||
			strings.Contains(ot, "extra turn after this one") {
			mustCounter = true
		}
		// r60 — combo-tutor classifier supersedes the score-gated
		// "search your library" path. Cheap 1-CMC tutors (Vampiric,
		// Mystical, Imperial Seal, Worldly) scored 1 under the legacy
		// `score >= 2` gate and slipped past despite being the highest-
		// leverage combo enablers in Commander. isComboTutorOracle
		// matches the canonical non-land search clauses and mustCounters
		// regardless of CMC; ramp tutors (Cultivate, Three Visits, Crop
		// Rotation) fall through to the original score gate so an
		// expensive ramp tutor (e.g. 4-mana Skyshroud Claim) can still
		// trip the threshold via its CMC but a cheap one stays off the
		// mustCounter list.
		if isComboTutorOracle(ot) {
			mustCounter = true
		} else if strings.Contains(ot, "search your library") && score >= 2 {
			mustCounter = true
		}
		// R60 priority-window audit — high-value trigger payloads. When
		// the stack item is a triggered ability whose source's oracle
		// text matches a known game-impactful pattern (attack-trigger
		// value extraction, mass-draw, mass-drain, theft), force a
		// must-counter even at a low CMC-derived score. Etali, Zur,
		// Narset (attack-trigger search); Sheoldred / Toxic Deluge-on-
		// resolve (mass drain); Smothering Tithe / Esper Sentinel (mass
		// draw); Tergrid (theft) all match.
		if isTrigger {
			if strings.Contains(ot, "exile the top") && strings.Contains(ot, "of each") {
				mustCounter = true // Etali pattern
			}
			if strings.Contains(ot, "search your library") {
				mustCounter = true // Zur / Narset attack-trigger tutors
			}
			if strings.Contains(ot, "each opponent") && strings.Contains(ot, "loses") {
				mustCounter = true // Sheoldred drain shape
			}
			if strings.Contains(ot, "each opponent draws") ||
				strings.Contains(ot, "whenever an opponent draws") {
				mustCounter = true // Sheoldred / wheel-style trigger
			}
		}
		// 3rd Eye: Counter kingmaker's key plays more aggressively.
		if h.isKingmaker(gs, top.Controller) && score >= 2 {
			mustCounter = true
		}
		// Counter cards we're specifically vulnerable to (Freya threat assessment).
		if len(h.vulnerableToSet) > 0 {
			if h.vulnerableToSet[strings.ToLower(respondCard.DisplayName())] {
				mustCounter = true
			}
		}
		// Opponent-archetype bias: a confident-combo caster's
		// non-trivial spell is far more likely to be a key piece than
		// random midrange chaff. Lower minScore for them so we burn
		// counters earlier rather than save for a later wrath.
		if prof := h.classifyOpponent(gs, top.Controller); prof != nil &&
			prof.Archetype == "combo" && prof.Confidence > 0.55 && score >= 1 {
			mustCounter = true
		}
		// 3rd Eye: Counter cards we've seen wreck the board before.
		cardName := respondCard.DisplayName()
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

		// R60 — deep-stack commit-or-lose adjustment. When the stack
		// already has 2+ hostile items pending, holding the counter for
		// "a bigger threat later" usually fails to find a use — the
		// chain itself IS the bigger threat, and the resolved items
		// will close the window. Drop the gate to fire on what we have.
		if depthSig.minScoreDelta > 0 {
			minScore -= depthSig.minScoreDelta
			if minScore < 1 {
				minScore = 1
			}
		}

		if score < minScore {
			return nil
		}
	}

	// R60 decision-replay surface — emit a structured event so post-
	// game "why did hat counter X?" analysis can read the score gate,
	// the depth-aware signal that tipped it, and the resolved-target
	// card (effectiveResponseCard handles the trigger-source case).
	respondTarget := "<unknown>"
	if rc := effectiveResponseCard(top); rc != nil {
		respondTarget = rc.DisplayName()
	}
	h.emitDecisionEvent(gs, seatIdx, "response_counter", map[string]interface{}{
		"counter":     bestCounter.DisplayName(),
		"target":      respondTarget,
		"top_kind":    top.Kind,
		"score":       score,
		"must_reason": depthSig.reason,
		"must":        depthSig.mustCounter,
		"stack_depth": len(gs.Stack),
	})

	return &gameengine.StackItem{
		Card:       bestCounter,
		Controller: seatIdx,
	}
}

// stackDepthSignals carries context the literal stackItemScore can't
// capture. Populated by computeStackDepthSignals and consumed by
// ChooseResponse to escalate the counter-decision at deep stacks.
//
// The three signals address concrete misjudgements the pre-R60 hat
// would make at stack depth ≥ 3:
//
//   - mustCounter — top is a counter (or other "kill the spell"
//     effect) aimed at one of OUR spells already on the stack below.
//     The pre-fix path scored the counter at its raw CMC (≈ 2) which
//     fell below the standard minScore=3 gate, so we passed and
//     watched our committed spell evaporate. The fix forces a counter
//     in that exact window — the only window we'll get to save the
//     investment.
//
//   - scoreBonus — top targets one of our permanents (removal /
//     bounce / steal aimed at us), OR we have a non-copy spell of our
//     own pending below at deep stacks. Both raise the stake of
//     letting the top resolve, but not to MUST-counter level — a 1
//     damage ping at a 4/4 doesn't deserve a hard counter just
//     because it has a target.
//
//   - minScoreDelta — pile-up signal. Two+ hostile items below the
//     top mean we're in a counter war or trigger chain where holding
//     the counter for "the next big threat" is the wrong frame —
//     this IS the big moment. Drop the threshold by 1 so a borderline
//     top fires.
type stackDepthSignals struct {
	mustCounter   bool
	scoreBonus    int
	minScoreDelta int
	// R60 follow-up to #310 — stack-resolution awareness across levels.
	//
	// holdForBiggerBelow: a higher-threat hostile item sits BELOW the
	// top. Burning our counter on the smaller top wastes it — passing
	// lets the small top resolve, then the engine grants us a fresh
	// priority window over the bigger threat (the formerly-below item
	// becomes the new top). Single-counter holders gain a turn-level
	// payoff from this deferral; multi-counter holders fall through to
	// the normal mustCounter / score path because the higher-threat
	// item will get its own counter on its window.
	//
	// forceCounterShield: top is a counter-protection spell (Veil of
	// Summer / Autumn's Veil / Silence / Grand Abolisher activation)
	// that, if it resolves, locks our counters out of the bigger
	// threats below it on the stack. We have ONE window — counter
	// the shield now or lose the response chain.
	holdForBiggerBelow bool
	forceCounterShield bool
	reason             string // short tag for log lines
}

// computeStackDepthSignals scans gs.Stack relative to `top` and
// `seatIdx`, producing the depth-aware response signals. Safe at any
// stack depth; returns zero-value signals when nothing of interest is
// happening (the cheap fast-path for the depth=1 common case).
//
// Two passes over gs.Stack:
//  1. Examine top.Targets / oracle text for the mustCounter and
//     scoreBonus triggers — these fire at any depth (depth=2 with
//     top=counter aimed at our depth=1 spell still must-counters).
//  2. At depth ≥ highStakesStackDepth, scan items below top to
//     build the investment-protection and hostile-pile-up signals.
func (h *YggdrasilHat) computeStackDepthSignals(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) stackDepthSignals {
	var sig stackDepthSignals
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return sig
	}
	depth := len(gs.Stack)

	// ----- Pass 1: top-item target / effect inspection ----------------
	if top.Card != nil {
		// Resolved Targets — the engine populates these at cast time so
		// we have authoritative target IDs to match against.
		for _, tg := range top.Targets {
			switch tg.Kind {
			case gameengine.TargetKindStackItem:
				if tg.Stack != nil && tg.Stack.Controller == seatIdx && !tg.Stack.IsCopy {
					sig.mustCounter = true
					sig.reason = "top_counters_our_stack_item"
					return sig
				}
			case gameengine.TargetKindPermanent:
				if tg.Permanent != nil && tg.Permanent.Controller == seatIdx {
					sig.scoreBonus += 2
					if sig.reason == "" {
						sig.reason = "top_targets_our_permanent"
					}
				}
			}
		}
		// Heuristic fallback when Targets are empty (some engine paths
		// push spells with the Targets slice unpopulated, and tests
		// commonly do so). If top has counterspell oracle and any of
		// our items is below in the stack, treat as the same situation.
		if !sig.mustCounter && depth >= 2 {
			ot := gameengine.OracleTextLower(top.Card)
			isCounter := strings.Contains(ot, "counter target spell") ||
				strings.Contains(ot, "counter target activated") ||
				strings.Contains(ot, "counter target triggered") ||
				strings.Contains(ot, "counter that spell")
			if isCounter {
				for i := 0; i < depth-1; i++ {
					it := gs.Stack[i]
					if it == nil || it.IsCopy {
						continue
					}
					if it.Controller == seatIdx {
						sig.mustCounter = true
						sig.reason = "top_counters_our_stack_item_heuristic"
						return sig
					}
				}
			}
		}
	}

	if depth < highStakesStackDepth {
		return sig
	}

	// ----- Pass 2: deep-stack investment + pile-up scan ---------------
	hasOurSpellBelow := false
	hostileBelow := 0
	for i := 0; i < depth-1; i++ {
		it := gs.Stack[i]
		if it == nil || it.IsCopy {
			continue
		}
		if it.Controller == seatIdx {
			hasOurSpellBelow = true
		} else {
			hostileBelow++
		}
	}
	if hasOurSpellBelow {
		sig.scoreBonus += 2
		if sig.reason == "" {
			sig.reason = "investment_below_top_at_depth"
		}
	}
	if hostileBelow >= 2 {
		sig.minScoreDelta = 1
		if sig.reason == "" {
			sig.reason = "hostile_pile_up_at_depth"
		}
	}
	return sig
}

// stackResolutionSignals adds the cross-level signals on top of
// stackDepthSignals. Mutates the passed-in sig in place — separate
// helper so the multi-level reasoning is testable independently of
// the existing depth-context detection.
//
// holdForBiggerBelow fires when:
//   - the top is hostile AND scores below a higher-threat hostile
//     item somewhere below it on the stack;
//   - the seat has exactly ONE affordable counter in hand (more than
//     one means we can counter both; zero is moot — ChooseResponse
//     short-circuits before this runs).
//
// forceCounterShield fires when:
//   - the top is a counter-protection spell (oracle text matches
//     "can't be countered" / "hexproof from blue" / "spells you cast
//     this turn can't be countered" / "no player may cast spells");
//   - and a hostile item sits below the top (otherwise the shield
//     protects nothing material — let it resolve and save the
//     counter).
func (h *YggdrasilHat) stackResolutionSignals(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem, sig *stackDepthSignals) {
	if gs == nil || top == nil || sig == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	depth := len(gs.Stack)
	if depth < 2 {
		return
	}

	// ----- forceCounterShield ----------------------------------------
	respondCard := effectiveResponseCard(top)
	if respondCard != nil && top.Controller != seatIdx {
		ot := gameengine.OracleTextLower(respondCard)
		shieldShape := strings.Contains(ot, "can't be countered") ||
			strings.Contains(ot, "hexproof from blue") ||
			strings.Contains(ot, "your opponents can't cast spells") ||
			strings.Contains(ot, "no player may cast")
		if shieldShape {
			hostileBelow := false
			for i := 0; i < depth-1; i++ {
				it := gs.Stack[i]
				if it == nil || it.IsCopy {
					continue
				}
				if it.Controller != seatIdx {
					hostileBelow = true
					break
				}
			}
			if hostileBelow {
				sig.forceCounterShield = true
				sig.mustCounter = true
				if sig.reason == "" {
					sig.reason = "counter_shield_protects_opp_stack"
				}
			}
		}
	}

	// ----- holdForBiggerBelow ----------------------------------------
	// Only the deferral case — if mustCounter is already set (Etali /
	// must-counter / shield), we'll counter regardless and this signal
	// is irrelevant. Same if top is our own item.
	if sig.mustCounter || top.Controller == seatIdx {
		return
	}
	topScore := stackItemScore(top) + sig.scoreBonus
	maxBelowScore := 0
	for i := 0; i < depth-1; i++ {
		it := gs.Stack[i]
		if it == nil || it.IsCopy {
			continue
		}
		if it.Controller == seatIdx {
			continue
		}
		if s := stackItemScore(it); s > maxBelowScore {
			maxBelowScore = s
		}
	}
	if maxBelowScore <= topScore {
		return
	}
	// Count affordable counters in hand. Defer the counter only when
	// we have a SINGLE one — with multiple, we can spend on the top
	// AND the below.
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	colored := gameengine.AvailableColoredManaEstimate(gs, seat)
	affordable := 0
	for _, c := range seat.Hand {
		if c != nil && gameengine.CardHasCounterSpell(c) &&
			gameengine.CanPayColoredCost(colored, c) {
			affordable++
			if affordable >= 2 {
				break
			}
		}
	}
	if affordable >= 2 {
		return
	}
	sig.holdForBiggerBelow = true
	if sig.reason == "" {
		sig.reason = "hold_counter_for_bigger_threat_below"
	}
}

// maybeCastDefensiveAnswer returns a fog or mass-protection spell from
// hand when this combat phase has enough incoming damage to kill the
// defender. Returns nil unless ALL of:
//   - we're in the combat phase (no point fogging out of combat),
//   - blockers have NOT yet been declared (otherwise the blocker math
//     already absorbed some of the damage and the incoming estimate is
//     stale),
//   - sum of unblocked attacker power targeting seatIdx ≥ our life,
//   - an affordable instant-shaped fog/protection card is in hand.
//
// The fog-or-protection scan uses oracle-text patterns rather than a
// hard-coded card list so it picks up reprints and variants (Fog,
// Moment's Peace, Holy Day, Heroic Intervention, Teferi's Protection,
// Boros Charm-style indestructible grants).
func (h *YggdrasilHat) maybeCastDefensiveAnswer(gs *gameengine.GameState, seatIdx int) *gameengine.StackItem {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	if gs.Phase != "combat" {
		return nil
	}
	// Avoid double-counting: if blockers were already declared, the
	// ChooseBlockers path has already shaped the incoming numbers and
	// we'd be re-pricing a partial situation. Only fire in the windows
	// BEFORE declare_blockers (begin_of_combat, declare_attackers).
	if gs.Step == "declare_blockers" ||
		gs.Step == "first_strike_damage" ||
		gs.Step == "combat_damage" ||
		gs.Step == "end_of_combat" {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.Life <= 0 {
		return nil
	}
	incoming := unblockedCombatDamageTo(gs, seatIdx)
	if incoming < seat.Life {
		return nil
	}
	// R60 color-aware check: Fog {G} needs green; Heroic Intervention
	// {1}{G} needs at least one green plus one of anything. The generic
	// ManaCostOf gate let a colorless-only pool false-positive these.
	colored := gameengine.AvailableColoredManaEstimate(gs, seat)
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !gameengine.CanPayColoredCost(colored, c) {
			continue
		}
		if !isDefensiveInstantSpell(c) {
			continue
		}
		h.logf("DEFENSIVE-ANSWER seat=%d → %s (incoming=%d life=%d)",
			seatIdx, c.DisplayName(), incoming, seat.Life)
		return &gameengine.StackItem{Card: c, Controller: seatIdx}
	}
	return nil
}

// unblockedCombatDamageTo sums the raw power of every attacking
// permanent targeting seatIdx (per AttackerDefender). Doubles power for
// double_strike (their two damage steps both land). Trample and other
// keyword nuances are intentionally ignored — the helper is a pre-
// blocker estimate; the only question is whether the swing would
// outright kill us if nothing changes.
func unblockedCombatDamageTo(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil {
		return 0
	}
	total := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsAttacking() {
				continue
			}
			def, ok := gameengine.AttackerDefender(p)
			if !ok || def != seatIdx {
				continue
			}
			pow := gs.PowerOf(p)
			if pow <= 0 {
				continue
			}
			if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
				pow *= 2
			}
			total += pow
		}
	}
	return total
}

// isDefensiveInstantSpell returns true for fogs (prevent all combat
// damage), mass damage-prevention turn-effects, and mass protection
// grants (creatures-you-control gain indestructible until EOT, phase
// out, etc.). Pattern-matched on oracle text so variants and reprints
// are picked up automatically.
func isDefensiveInstantSpell(card *gameengine.Card) bool {
	if card == nil {
		return false
	}
	ot := gameengine.OracleTextLower(card)
	if ot == "" {
		return false
	}
	// Fog family.
	if strings.Contains(ot, "prevent all combat damage") {
		return true
	}
	if strings.Contains(ot, "prevent all damage") && strings.Contains(ot, "this turn") {
		return true
	}
	// Mass-protection family: indestructible/hexproof grant to our
	// permanents until end of turn. Requires both the keyword and the
	// "you control" + "until end of turn" anchors so a single-target
	// buff or a permanent-source static doesn't false-positive.
	hasGrant := strings.Contains(ot, "creatures you control") ||
		strings.Contains(ot, "permanents you control")
	hasEOT := strings.Contains(ot, "until end of turn")
	if hasGrant && hasEOT {
		if strings.Contains(ot, "indestructible") ||
			strings.Contains(ot, "hexproof") {
			return true
		}
	}
	// Phase-out protection (Teferi's Protection): "phase out" + "you control".
	if strings.Contains(ot, "phase out") &&
		(strings.Contains(ot, "you control") || strings.Contains(ot, "your life total")) {
		return true
	}
	return false
}

// maybeCastReactiveRemoval offers an instant-speed targeted-removal spell
// from hand in response to an opponent's stack item, but ONLY when there's
// a clearly-good target on an opponent's battlefield (a genuine threat:
// big creature, planeswalker, value engine, combo piece, commander).
//
// This is the conservative reactive-interaction win (r61 PR-7). The
// non-active seat's only reactive priority window is this ChooseResponse
// hook; before PR-7 it could ONLY return counterspells, so its entire
// instant-speed removal game on opponents' turns was dead. The cast itself
// is driven by the existing PriorityRound machinery (it pays manaCostOf,
// removes from hand, pushes the StackItem); the removal's target is chosen
// at resolution by ChooseTarget, which already scores permanent targets.
//
// LOOP SAFETY (the whole reason this is gated tightly):
//   - The AI never responds to its OWN stack item (top.Controller==seat
//     guard at the top of ChooseResponse). So a removal we cast can't
//     trigger another reactive removal in the same chain.
//   - One-shot per incoming stack item: we record top.ID and never offer
//     a second reactive non-counter play against the same item. Combined
//     with the per-cast mana/card depletion and PriorityRound's maxDepth=8,
//     a priority round terminates.
//   - Cleared each turn so the map can't grow unbounded.
//
// PERF: cheap filters first (turn guard → one-shot guard → "is there a
// real threat at all" via bestOpponentRemovalScore) BEFORE the hand scan;
// no rollout/MCTS work on this path.
//
// CONSERVATIVE: when uncertain, we pass (return nil). A missed reactive
// removal is status-quo; a bad/looping one is a regression.
func (h *YggdrasilHat) maybeCastReactiveRemoval(gs *gameengine.GameState, seatIdx int, top *gameengine.StackItem) *gameengine.StackItem {
	if gs == nil || top == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return nil
	}

	// Per-turn reset of the one-shot guard.
	if h.reactiveFiredAgainst == nil || h.reactiveFiredAgainstTrn != gs.Turn {
		h.reactiveFiredAgainst = make(map[int]bool)
		h.reactiveFiredAgainstTrn = gs.Turn
	}
	// One-shot guard: at most one reactive non-counter play per distinct
	// opponent stack item. top.ID is assigned by PushStackItem; a 0 ID
	// means an un-pushed/synthetic item — treat as "don't track" but still
	// allow exactly one fire (the guard below keys on 0 then, which is
	// fine because ChooseResponse is called per real stack push).
	if h.reactiveFiredAgainst[top.ID] {
		return nil
	}

	// Cheap threat gate: only bother scanning hand when a genuinely
	// threatening permanent exists to target. bestOpponentRemovalScore
	// returns >= 0.60 for a big creature / planeswalker / value engine /
	// combo piece / commander — a clearly-good removal target. Anything
	// lower (vanilla creatures, chaff) isn't worth a reactive removal.
	if h.bestOpponentRemovalScore(gs, seatIdx) < 0.60 {
		return nil
	}

	// Color-aware affordability against currently-available mana, same gate
	// the counterspell path and the active-player cast path use.
	colored := gameengine.AvailableColoredManaEstimate(gs, seat)
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		// Instant-speed only: an actual instant. We intentionally do NOT
		// offer flash permanents here (a flash creature isn't removal) —
		// this keeps the reactive surface to the single clear-win case.
		if !typeLineContains(c, "instant") {
			continue
		}
		// Targeted-removal shape: destroy/exile/bounce/damage at a
		// creature / planeswalker / nonland permanent / any-target. Reuses
		// the poker.go classifier (same package), which scans the parsed
		// spell-body Activated node — the AST shape instants resolve through.
		if !isTargetedRemoval(c) {
			continue
		}
		if !gameengine.CanPayColoredCost(colored, c) {
			continue
		}
		// Fire it. Record the one-shot guard so this incoming item can't
		// trigger a second reactive removal even if it remains top across
		// further priority passes in this round.
		h.reactiveFiredAgainst[top.ID] = true
		topName := "<spell>"
		if top.Card != nil {
			topName = top.Card.DisplayName()
		} else if top.Source != nil && top.Source.Card != nil {
			topName = top.Source.Card.DisplayName()
		}
		h.logf("REACTIVE-REMOVAL seat=%d → %s (in response to %s, threat=%.2f)",
			seatIdx, c.DisplayName(), topName, h.bestOpponentRemovalScore(gs, seatIdx))
		h.emitDecisionEvent(gs, seatIdx, "reactive_removal", map[string]interface{}{
			"card":           c.DisplayName(),
			"in_response_to": topName,
		})
		// Set Effect explicitly so ResolveStackTop resolves the removal
		// body (it gates effect resolution on item.Effect != nil). The
		// counterspell path can leave Effect nil because counters are
		// dispatched specially, but a Destroy/Exile/Bounce/Damage instant
		// must carry its spell body to do anything on resolution — its
		// target is then chosen by ChooseTarget at resolve time.
		return &gameengine.StackItem{
			Card:       c,
			Controller: seatIdx,
			Effect:     gameengine.CollectSpellEffectOf(c),
		}
	}
	return nil
}

// -- Interface: ChooseTarget --

func (h *YggdrasilHat) ChooseTarget(gs *gameengine.GameState, seatIdx int, filter gameast.Filter, legal []gameengine.Target) gameengine.Target {
	if len(legal) == 0 {
		return gameengine.Target{}
	}
	if len(legal) == 1 {
		return legal[0]
	}
	h.recordCascadeDecision(gs)

	// Combo sequencer tutor override: if we're assembling a combo and
	// this is a tutor resolving, prefer the missing piece above all else.
	if h.comboSeq != nil {
		assessment := h.comboSeq.Evaluate(gs, seatIdx)
		if assessment.Assembling && assessment.MissingPiece != "" {
			for _, t := range legal {
				if t.Kind == gameengine.TargetKindCard && t.Card != nil &&
					t.Card.DisplayName() == assessment.MissingPiece {
					h.logf("%s COMBO-TUTOR seat=%d → %s (assembling: %s)",
						roundTag(gs, seatIdx), seatIdx,
						assessment.MissingPiece,
						assessment.BestLine.Name)
					return t
				}
			}
		}
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

	// For permanent-targeting effects, score each target.
	hasPermanentTargets := false
	for _, t := range legal {
		if t.Kind == gameengine.TargetKindPermanent && t.Permanent != nil {
			hasPermanentTargets = true
			break
		}
	}

	// r61 PR-8 Fix B — beneficial-effect branch (strictly additional).
	// When the resolving effect is a single-target BENEFICIAL effect (pump
	// / +1/+1 / regen / protective keyword grant / damage prevention /
	// untap) whose filter admits both own and opponent creatures, point it
	// at the AI's OWN best creature instead of the strongest enemy. This
	// fires ONLY for genuinely-beneficial intent; for HARMFUL or UNKNOWN
	// intent the function falls through to the existing removal-scoring
	// loop below with byte-identical behavior — so the PR-7 reactive-
	// removal reliance on best-enemy targeting is completely untouched.
	if hasPermanentTargets && h.resolvingTargetIntent(gs, seatIdx, filter) == intentBeneficial {
		if own, ok := h.bestBeneficialOwnTarget(gs, seatIdx, legal); ok {
			ownName := "<creature>"
			if own.Permanent != nil && own.Permanent.Card != nil {
				ownName = own.Permanent.Card.DisplayName()
			}
			h.logf("%s BENEFICIAL-TARGET seat=%d → %s (own creature; buff/protect)",
				roundTag(gs, seatIdx), seatIdx, ownName)
			return own
		}
		// No own creature among the legal targets (e.g. the buff can only
		// hit an opponent's board). Fall through to the existing scorer —
		// targeting an enemy with a buff is bad, but it's a degenerate
		// legal-target set and we keep the conservative status-quo path.
	}

	if hasPermanentTargets {
		type scoredTarget struct {
			target gameengine.Target
			score  float64
		}
		var candidates []scoredTarget
		// relPos drives politics-aware threat bias: hit the leader when
		// competitive, dodge them when behind.
		relPos := h.relativePosition(gs, seatIdx)
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
				pow := gs.PowerOf(p)
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
					// Scale PW removal priority by loyalty proximity to ultimate.
					loyalty := 0
					if p.Counters != nil {
						loyalty = p.Counters["loyalty"]
					}
					ultCost := estimatePWUltimateCost(p.Card)
					if ultCost > 0 && loyalty > 0 {
						proximity := float64(loyalty) / float64(ultCost)
						if proximity >= 1.0 {
							sc += 5.0 // can ult NOW — critical removal target
						} else if proximity >= 0.7 {
							sc += 3.5 // 1-2 activations away
						} else {
							sc += 2.0 // base PW removal value
						}
					} else {
						sc += 2.0 // fallback: can't estimate, use flat bonus
					}
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
				// Opponent-archetype targeting bias.
				// Confident-combo opponent: every permanent they
				// control is suspect — bump combo pieces extra and
				// add a flat bias for any artifact/enchantment that
				// might be a combo enabler.
				// Confident-aggro opponent: prefer lord/anthem
				// removal (anything granting +1/+1 or "creatures you
				// control") over picking off lone creatures, since
				// the lord lifts their entire board.
				if prof := h.classifyOpponent(gs, p.Controller); prof != nil {
					// R60r5 — meta-confidence-folded bias. Matches the
					// attack-target site so removal selection and
					// attack target selection consume the same
					// composed signal.
					mult := effectiveArchetypeBias(prof)
					if mult > 0 {
						switch prof.Archetype {
						case "combo":
							if h.comboPieceSet[cardName] {
								sc += 1.5 * mult
							}
							if typeLineContains(p.Card, "artifact") || typeLineContains(p.Card, "enchantment") {
								sc += 0.6 * mult
							}
						case "aggro":
							lowOT := gameengine.OracleTextLower(p.Card)
							if strings.Contains(lowOT, "creatures you control") ||
								strings.Contains(lowOT, "other creatures get +") ||
								strings.Contains(lowOT, "+1/+1") {
								sc += 1.5 * mult
							}
						}
					}
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
			// Politics-aware bias: when there's a clear table leader,
			// either prefer hitting them (we're competitive) or dodge
			// them and bias toward the runner-up (we're behind).
			sc += politicsThreatAdjustment(threats, relPos, p.Controller)
			// R60 round 13+ removal-quality penalties (see
			// target_priority_signals_r60.go). Indestructible kills the
			// entire removal spell with a no-op; death-payoff creatures
			// hand the opp value on death.
			sc += removalTargetIndestructiblePenalty(p)
			sc += removalTargetDeathPayoffPenalty(p)
			candidates = append(candidates, scoredTarget{t, h.applyNoise(sc)})
		}
		if len(candidates) > 0 {
			sort.SliceStable(candidates, func(i, j int) bool {
				return candidates[i].score > candidates[j].score
			})
			// Confidence threshold: vary removal targets at low brackets.
			permScores := make([]float64, len(candidates))
			for i, c := range candidates {
				permScores[i] = c.score
			}
			pick := h.selectAmongTop(permScores)
			return candidates[pick].target
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
			relPos := h.relativePosition(gs, seatIdx)
			nt := make([]noisyThreat, len(threats))
			for i, th := range threats {
				// Politics bias stacks on the EvalScore. The kingmaker-
				// dodge branch flips the ranking: when we're behind, the
				// leader is heavily demoted and the runner-up promoted.
				bias := politicsThreatAdjustment(threats, relPos, th.Seat)
				nt[i] = noisyThreat{th.Seat, h.applyNoise(th.EvalScore + bias)}
			}
			sort.SliceStable(nt, func(i, j int) bool {
				return nt[i].score > nt[j].score
			})
			// Confidence threshold: vary player targets at low brackets.
			ntScores := make([]float64, len(nt))
			for i, n := range nt {
				ntScores[i] = n.score
			}
			pickIdx := h.selectAmongTop(ntScores)
			bestSeat := nt[pickIdx].seat
			for _, t := range legal {
				if t.Kind == gameengine.TargetKindSeat && t.Seat == bestSeat {
					return t
				}
			}
		}
	}

	// Equipment equip target selection: score friendly creatures for equip.
	// Try to recover the source equipment from the resolving stack item so
	// scoreEquipTarget can apply equipment-specific signals (recurrence,
	// connect-payoff, indestructible/protection riders). If we can't find
	// it, fall back to body-only scoring.
	if filter.Base == "creature" && filter.YouControl {
		var sourceEquip *gameengine.Permanent
		if n := len(gs.Stack); n > 0 {
			top := gs.Stack[n-1]
			if top != nil && top.Source != nil && top.Source.IsEquipment() &&
				top.Source.Controller == seatIdx {
				sourceEquip = top.Source
			}
		}
		// Heuristic fallback: if there's exactly one of our equipment on
		// the battlefield with an "equip" cost, assume it's the source.
		if sourceEquip == nil && seatIdx >= 0 && seatIdx < len(gs.Seats) {
			seat := gs.Seats[seatIdx]
			var equips []*gameengine.Permanent
			for _, p := range seat.Battlefield {
				if p == nil || !p.IsEquipment() || p.Card == nil {
					continue
				}
				if strings.Contains(gameengine.OracleTextLower(p.Card), "equip") {
					equips = append(equips, p)
				}
			}
			if len(equips) == 1 {
				sourceEquip = equips[0]
			}
		}
		type equipCandidate struct {
			target gameengine.Target
			score  int
		}
		var equipCandidates []equipCandidate
		for _, t := range legal {
			if t.Kind != gameengine.TargetKindPermanent || t.Permanent == nil {
				continue
			}
			if t.Permanent.Controller != seatIdx {
				continue
			}
			sc := scoreEquipTarget(gs, seatIdx, sourceEquip, t.Permanent)
			if sc > 0 {
				equipCandidates = append(equipCandidates, equipCandidate{t, sc})
			}
		}
		if len(equipCandidates) > 0 {
			sort.SliceStable(equipCandidates, func(i, j int) bool {
				return equipCandidates[i].score > equipCandidates[j].score
			})
			return equipCandidates[0].target
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
	h.recordCascadeDecision(gs)

	pos := h.evalPosition(gs, seatIdx)

	// R60 round 5 — multi-mode follow-up detection. The engine's
	// resolveChoice loops ChooseMode `pick` times for pick > 1
	// (Cryptic Command, Charms, etc.), narrowing the modes slice by
	// one between calls. We detect that "the slice we're being shown
	// now is the previous slice minus our previous pick" and apply a
	// synergy bonus that complements the prior pick — so a Cryptic
	// Command first picking "counter" naturally pairs with "bounce"
	// or "draw" on the second pick instead of forcing the same
	// best-effort scoring without context. See `complementBonus` for
	// the pairing rules.
	priorMode := h.priorChooseModePickIfFollowup(gs, modes)

	type scoredMode struct {
		idx   int
		score float64
	}
	scored := make([]scoredMode, len(modes))
	for i, m := range modes {
		s := h.scoreModeEffect(gs, seatIdx, m, pos)
		if priorMode != nil {
			s += complementBonus(priorMode, m)
		}
		scored[i] = scoredMode{i, s}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Confidence threshold: vary mode selection at low brackets.
	modeScores := make([]float64, len(scored))
	for i, s := range scored {
		modeScores[i] = s.score
	}
	pick := h.selectAmongTop(modeScores)

	// Persist mode scoring so the analyzer can answer "why did the hat
	// pick draw over damage on Cryptic Command?". top_indices / top_scores
	// are aligned and ordered by score desc; chosen_idx is the original
	// (mode-list) index of the pick, not the position in the sorted slice.
	topN := len(scored)
	if topN > 4 {
		topN = 4
	}
	topIdx := make([]int, topN)
	topScores := make([]float64, topN)
	for i := 0; i < topN; i++ {
		topIdx[i] = scored[i].idx
		topScores[i] = scored[i].score
	}
	h.emitDecisionEvent(gs, seatIdx, "mode", map[string]interface{}{
		"chosen_idx":   scored[pick].idx,
		"chosen_score": scored[pick].score,
		"top_indices":  topIdx,
		"top_scores":   topScores,
		"mode_count":   len(modes),
		"position":     pos,
	})
	h.logf("MODE seat=%d -> idx=%d score=%.3f (top: %v scores=%v)",
		seatIdx, scored[pick].idx, scored[pick].score, topIdx, topScores)
	// R60r5 — remember the pick + the full slice we just saw so the
	// next ChooseMode call (if it's a follow-up pick on the same
	// modal spell) can detect the subset relationship and apply
	// complement scoring.
	h.lastChooseModePick = modes[scored[pick].idx]
	h.lastChooseModeSlice = append(h.lastChooseModeSlice[:0], modes...)
	h.lastChooseModeTurn = gs.Turn
	return scored[pick].idx
}

// priorChooseModePickIfFollowup reports the previously-picked Effect
// when the current `modes` slice is a strict subset of the previous
// ChooseMode call's slice within the same turn — i.e., the engine
// stripped the prior pick and is asking us for the next mode of the
// same modal spell. Returns nil when the call isn't a follow-up.
//
// gameast.Effect concrete types contain Filter structs that aren't
// `==`-comparable at runtime (Filter has slices/maps internally), so
// we compare via Kind() string matching: the prior slice must
// multiset-match the current slice PLUS one of-the-prior-pick-kind
// entry. Pointer identity would be more precise but the engine
// re-builds the slice between calls so the same Effect can move
// addresses; Kind matching is safe and the false-positive risk
// (different spell happens to have identical kind-multiset minus one)
// is negligible in practice given the multi-mode-spell corpus.
func (h *YggdrasilHat) priorChooseModePickIfFollowup(gs *gameengine.GameState, modes []gameast.Effect) gameast.Effect {
	if h == nil || gs == nil || h.lastChooseModePick == nil {
		return nil
	}
	if h.lastChooseModeTurn != gs.Turn {
		return nil
	}
	if len(modes) != len(h.lastChooseModeSlice)-1 {
		return nil
	}
	priorKinds := make(map[string]int, len(h.lastChooseModeSlice))
	for _, m := range h.lastChooseModeSlice {
		if m != nil {
			priorKinds[m.Kind()]++
		}
	}
	currKinds := make(map[string]int, len(modes))
	for _, m := range modes {
		if m != nil {
			currKinds[m.Kind()]++
		}
	}
	pickKind := h.lastChooseModePick.Kind()
	// Multiset relation: prior == current + {pickKind: 1}.
	if priorKinds[pickKind] == 0 || priorKinds[pickKind] != currKinds[pickKind]+1 {
		return nil
	}
	for k, v := range priorKinds {
		if k == pickKind {
			continue
		}
		if currKinds[k] != v {
			return nil
		}
	}
	return h.lastChooseModePick
}

// complementBonus returns a small score bonus for picking `cand` as
// the SECOND (or later) mode of a modal spell that already picked
// `prior`. Encodes the canonical multi-mode-spell synergies in
// commander / cEDH:
//
//	counter   → bounce / draw : Cryptic Command's classic line
//	bounce    → draw / counter : tempo + refill
//	destroy   → draw / counter : "kill + replace card"
//	damage    → draw : burn + cantrip
//	draw      → counter / bounce : reactive mana left up for the
//	                               drawn answer
//	gain_life → draw / counter : stabilize and rebuild
//
// The bonus is small (0.10) — large enough to break ties between
// similarly-scored modes, small enough not to override a strongly-
// favored mode (e.g., a lethal `damage` should still win over the
// "synergistic" draw). Unknown / non-paired kinds return 0.
func complementBonus(prior, cand gameast.Effect) float64 {
	if prior == nil || cand == nil {
		return 0
	}
	pk := prior.Kind()
	ck := cand.Kind()
	if pk == ck {
		return -0.05 // mild penalty against picking the same kind twice
	}
	synergies := map[string]map[string]bool{
		"counter_spell": {"bounce": true, "draw": true},
		"bounce":        {"draw": true, "counter_spell": true},
		"destroy":       {"draw": true, "counter_spell": true},
		"exile":         {"draw": true, "counter_spell": true},
		"damage":        {"draw": true},
		"lose_life":     {"draw": true},
		"draw":          {"counter_spell": true, "bounce": true},
		"gain_life":     {"draw": true, "counter_spell": true},
	}
	if paired, ok := synergies[pk]; ok && paired[ck] {
		return 0.10
	}
	return 0
}

func (h *YggdrasilHat) scoreModeEffect(gs *gameengine.GameState, seatIdx int, eff gameast.Effect, pos float64) float64 {
	score := 0.0
	switch eff.Kind() {
	case "destroy", "exile":
		// Score by the value of the best legal opponent permanent we
		// could remove. A combo piece on the board pushes this to 0.95;
		// a vanilla creature only is 0.40; nothing legal is near-zero.
		score = h.bestOpponentRemovalScore(gs, seatIdx)
		if score == 0 {
			score = 0.05
		}
		// Control archetype leans into removal modes — a Cryptic
		// Command-style "counter" vs "destroy" choice should bias
		// toward the answer for the open threat. The bump is small
		// enough not to flip a near-zero removal score against a
		// strong alternative.
		if h.Strategy != nil && (h.Strategy.Archetype == ArchetypeControl || h.Strategy.Archetype == ArchetypeStax) {
			score += 0.05
		}

	case "damage", "lose_life":
		// Scale by how close to lethal the closest opponent is. The
		// effect's own amount is consulted when present — a "deal 3"
		// vs a 3-life opponent is lethal regardless of position.
		amount := 1
		switch e := eff.(type) {
		case *gameast.Damage:
			amount = effectAmount(e.Amount)
		case *gameast.LoseLife:
			amount = effectAmount(e.Amount)
		}
		score = lethalIncomingScore(gs, seatIdx, amount)
		// Aggression / position nudges ride on top of the kill-math.
		if pos > 0.3 {
			score += 0.10
		}
		// Aggro / spellslinger care more about damage modes than
		// other archetypes: damage IS their win condition. Small
		// bump so the mode-pick gravitates toward burn over a parity
		// effect like draw when lethal isn't on the line.
		if h.Strategy != nil && (h.Strategy.Archetype == ArchetypeAggro || h.Strategy.Archetype == ArchetypeSpellslinger) {
			score += 0.05
		}

	case "counter_spell":
		// Countering only matters if there's a hostile spell on the
		// stack to counter. With nothing to point at the mode is a
		// dead floor — pick something else. Spells we control are
		// excluded so a Cryptic Command pointed at our own draw
		// trigger doesn't read as "high value".
		hostileOnStack := false
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			if item.IsCopy {
				continue
			}
			if item.Controller != seatIdx {
				hostileOnStack = true
				break
			}
		}
		if hostileOnStack {
			score = 0.85
		} else {
			score = 0.05
		}
		if h.Strategy != nil && (h.Strategy.Archetype == ArchetypeControl || h.Strategy.Archetype == ArchetypeSpellslinger) {
			score += 0.05
		}

	case "draw":
		// Empty-hand draw is huge; full-hand draw is incremental and
		// risks hand-size discard at end of turn.
		seat := gs.Seats[seatIdx]
		hand := 0
		lib := 0
		if seat != nil {
			hand = len(seat.Hand)
			lib = len(seat.Library)
		}
		switch {
		case hand == 0:
			score = 0.90
		case hand <= 2:
			score = 0.75
		case hand <= 4:
			score = 0.55
		case hand <= 6:
			score = 0.40
		default:
			score = 0.30
		}
		if pos < -0.2 {
			score += 0.05
		}
		// Library-low decking penalty: drawing into a near-empty
		// library accelerates a §704.5b loss. Below ~7 cards the
		// expected draws-to-deckout meaningfully shrink with each
		// drawn card; below ~3 we're one trigger / cantrip from
		// drawing on empty. lib==0 falls through unchanged: the
		// game is effectively over (next draw is the loss), no
		// useful signal in scaling the mode score, and leaving the
		// base score deterministic helps edge-case test reproducibility.
		if lib > 0 {
			switch {
			case lib <= 3:
				score *= 0.25
			case lib <= 7:
				score *= 0.60
			}
		}

	case "create_token":
		// Go-wider value scales with our existing creature count — each
		// extra body compounds anthems, sacrifice density, and combat
		// pressure. Commanders that explicitly care about creatures
		// (Krenko, Edgar, Slimefoot) push it further.
		count := ourCreatureCount(gs, seatIdx)
		switch {
		case count == 0:
			score = 0.55
		case count <= 2:
			score = 0.65
		case count <= 5:
			score = 0.75
		default:
			score = 0.85
		}
		if commanderCaresAboutCreatures(gs, seatIdx) {
			score += 0.08
		}
		if h.Strategy != nil &&
			(h.Strategy.Archetype == ArchetypeTribal || h.Strategy.Archetype == ArchetypeAggro) {
			score += 0.05
		}

	case "counter_mod":
		// +1/+1 counters need a recipient. No creature → near-zero;
		// a vanilla creature → modest; a payoff card → strong.
		anyCreature, payoff := h.hasCounterPayoff(gs, seatIdx)
		switch {
		case payoff:
			score = 0.70
		case anyCreature:
			score = 0.45
		default:
			score = 0.10
		}

	case "gain_life":
		// Curve: at 5 life this is panic mode (0.90); at 30+ it's
		// nearly worthless (0.20).
		seat := gs.Seats[seatIdx]
		life := 40
		if seat != nil {
			life = seat.Life
		}
		switch {
		case life <= 5:
			score = 0.90
		case life <= 10:
			score = 0.70
		case life <= 20:
			score = 0.45
		case life <= 30:
			score = 0.30
		default:
			score = 0.20
		}

	case "bounce":
		// Tempo per swap scales with the highest-CMC permanent on
		// either board we could legally return.
		cmc := bestBounceCMC(gs, seatIdx)
		switch {
		case cmc >= 6:
			score = 0.80
		case cmc >= 4:
			score = 0.60
		case cmc >= 2:
			score = 0.45
		default:
			score = 0.30
		}

	case "tutor":
		// Tutoring is almost always strong, but only if we have a
		// known target list. Combo decks search first.
		score = 0.75
		if len(h.tutorTargetSet) > 0 {
			score = 0.85
		}
		if h.Strategy != nil && h.Strategy.Archetype == ArchetypeCombo {
			score += 0.05
		}

	case "reanimate", "recurse":
		// Value scales with graveyard size; reanimator decks treat it
		// as their best effect.
		seat := gs.Seats[seatIdx]
		gy := 0
		if seat != nil {
			gy = len(seat.Graveyard)
		}
		switch {
		case gy == 0:
			score = 0.30
		case gy <= 3:
			score = 0.50
		case gy <= 7:
			score = 0.70
		default:
			score = 0.85
		}
		if h.Strategy != nil && h.Strategy.Archetype == ArchetypeReanimator {
			score += 0.10
		}

	case "add_mana":
		// Early mana = ramp value; late mana = mostly empty-floor.
		switch {
		case gs.Turn <= 3:
			score = 0.65
		case gs.Turn <= 6:
			score = 0.45
		default:
			score = 0.25
		}

	case "sacrifice":
		// With aristocrats payoffs on the board, our own sacrifices
		// are net positive. Without them, it's a creature loss.
		if h.hasAristocratsPayoff(gs, seatIdx) {
			score = 0.70
		} else {
			score = 0.10
		}

	case "buff", "grant_ability":
		// Useful when we have at least one creature; better when the
		// board is wide enough for the buff to compound.
		count := ourCreatureCount(gs, seatIdx)
		switch {
		case count == 0:
			score = 0.10
		case count <= 2:
			score = 0.40
		default:
			score = 0.55
		}

	case "discard":
		score = 0.35
		if h.Strategy != nil && h.Strategy.Archetype == ArchetypeStax {
			score = 0.65
		}

	case "mill":
		// Mill against an opponent is tempo if their library is small;
		// for self-mill (graveyard decks), we're filling a resource —
		// the DNA nudge below covers the self-mill case for graveyard
		// decks. Here we score by minimum opponent library size.
		minLib := minOpponentLibrarySize(gs, seatIdx)
		switch {
		case minLib == 0:
			score = 0.10
		case minLib <= 5:
			score = 0.85
		case minLib <= 15:
			score = 0.55
		case minLib <= 30:
			score = 0.30
		default:
			score = 0.15
		}

	case "scry", "surveil":
		score = 0.45

	default:
		score = 0.30
	}
	score = h.applyDNANudge(eff, score)
	return score
}

// -- Interface: ShouldCastCommander --

// commanderUrgency returns a 0.0–1.0 score for how strategy-critical
// resolving the named commander is for this deck. Higher = the deck
// doesn't function without the commander on the battlefield, so the hat
// should pay through more tax and ignore interaction risk to land it.
//
// Tiers (highest match wins):
//
//	0.95 — Strategy.IsCommanderCentric set by Freya's
//	       detectCommanderCentric (Voltron / engine commanders / ≥45%
//	       commander synergy). The deck's gameplan IS the commander.
//	0.90 — Commander name appears in any ComboPlan piece list — the
//	       commander is a literal combo piece (Kiki, Niv-Mizzet, Thrasios).
//	0.80 — Strategy archetype is Tribal — the commander is the lord/
//	       anthem the rest of the tribe is built around.
//	0.70 — Strategy has ≥3 ValueEngineKeys — commander is a key enabler
//	       of a value-engine-dense deck.
//	0.40 — Default. Generic creature commander, not load-bearing.
//
// CommanderUrgency exposes commanderUrgency for callers outside the hat
// package (e.g. the tournament runner deciding whether to retry the
// commander cast inside the main-phase cast loop).
func (h *YggdrasilHat) CommanderUrgency(commanderName string) float64 {
	return h.commanderUrgency(commanderName)
}

func (h *YggdrasilHat) commanderUrgency(commanderName string) float64 {
	if h.Strategy == nil {
		return 0.4
	}
	if h.Strategy.IsCommanderCentric {
		return 0.95
	}
	for _, cp := range h.Strategy.ComboPieces {
		for _, p := range cp.Pieces {
			if p == commanderName {
				return 0.9
			}
		}
	}
	if h.Strategy.Archetype == ArchetypeTribal {
		return 0.8
	}
	if len(h.Strategy.ValueEngineKeys) >= 3 {
		return 0.7
	}
	return 0.4
}

func (h *YggdrasilHat) ShouldCastCommander(gs *gameengine.GameState, seatIdx int, commanderName string, tax int) bool {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	avail := gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])
	if avail <= 0 && tax > 0 {
		h.logf("CMDR-CAST decision=skip name=%s urgency=n/a reason=no_mana avail=%d tax=%d",
			commanderName, avail, tax)
		return false
	}

	urgency := h.commanderUrgency(commanderName)

	// Voltron / high-tax recommit guard (r61). A Voltron commander — or any
	// commander already taxed enough to imply it's been killed 2+ times
	// (tax >= 4 == cast from the command zone at least twice) — should not
	// blindly re-deploy into open removal with no mana left to protect it.
	// Without this, the PhaseDeploy / PhaseExecute branches below return
	// `true` unconditionally and walk the same Voltron commander straight
	// back into the same removal that just killed it. Gate ONLY when ALL of:
	//   - it's a Voltron deck OR the commander is already repeatedly-killed
	//     (tax >= 4), AND
	//   - it isn't strategically urgent (urgency < 0.8 — a must-have engine
	//     commander still goes), AND
	//   - there's real interaction risk at the table, AND
	//   - we can't both recast AND hold up an answer (avail < tax*2),
	// so a safe recast (open table, spare mana, or low tax) is unaffected.
	isVoltron := h.Strategy != nil && h.Strategy.Archetype == ArchetypeVoltron
	if (isVoltron || tax >= 4) && urgency < 0.8 && tax >= 2 {
		intRisk := h.tableInteractionRisk(gs, seatIdx)
		if intRisk > 0.5 && avail < tax*2 {
			h.logf("CMDR-CAST decision=skip name=%s urgency=%.2f reason=voltron_recommit_risk intRisk=%.2f tax=%d avail=%d",
				commanderName, urgency, intRisk, tax, avail)
			return false
		}
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
		// Strategy-enabler commander: when the deck has ≥3 ValueEngineKeys,
		// the commander is the load-bearing piece for those engines, so
		// paying through tax matters more than holding mana for protection.
		// Lower buffer (we'll spend our mana on the commander), raise the
		// max tax we'll pay (we keep recasting even when expensive).
		if len(h.Strategy.ValueEngineKeys) >= 3 {
			manaBuffer--
			maxTax += 2
			if manaBuffer < 0 {
				manaBuffer = 0
			}
		}
	}
	// High-urgency: the deck needs this commander out. Pay through more
	// tax and don't reserve mana for interaction.
	if urgency >= 0.8 {
		maxTax += 4
		manaBuffer = 0
	}

	// Phase-aware caps. Deploy is "deploy commander" — always cast when
	// affordable (the commander IS the deploy goal). Execute is the close
	// — pay any tax we can afford, the commander is too important to leave
	// in zone. Develop is the default tax-aware path.
	switch h.detectPhase(gs, seatIdx) {
	case PhaseDeploy:
		h.logf("CMDR-CAST decision=cast name=%s urgency=%.2f reason=deploy_phase tax=%d avail=%d",
			commanderName, urgency, tax, avail)
		return true
	case PhaseExecute:
		// Existing "always recast if affordable" semantics that the old
		// `gs.Turn > 15` branch covered.
		h.logf("CMDR-CAST decision=cast name=%s urgency=%.2f reason=execute_phase tax=%d avail=%d",
			commanderName, urgency, tax, avail)
		return true
	}

	// 3rd Eye: If high interaction risk and commander tax is already 2+,
	// wait until we have enough mana to also hold up protection, or until
	// the blue player taps out. Only enforced for low-urgency commanders;
	// strategy-critical commanders cast through the risk because the deck
	// can't function without them on the battlefield.
	if urgency < 0.6 && tax >= 2 {
		intRisk := h.tableInteractionRisk(gs, seatIdx)
		if intRisk > 0.5 && avail < tax*2+2 {
			h.logf("CMDR-CAST decision=skip name=%s urgency=%.2f reason=interaction_risk intRisk=%.2f tax=%d avail=%d",
				commanderName, urgency, intRisk, tax, avail)
			return false
		}
	}

	cast := tax <= maxTax || avail >= tax*2+manaBuffer
	if cast {
		h.logf("CMDR-CAST decision=cast name=%s urgency=%.2f tax=%d maxTax=%d avail=%d buffer=%d",
			commanderName, urgency, tax, maxTax, avail, manaBuffer)
	} else {
		h.logf("CMDR-CAST decision=skip name=%s urgency=%.2f reason=tax_above_cap tax=%d maxTax=%d avail=%d buffer=%d",
			commanderName, urgency, tax, maxTax, avail, manaBuffer)
	}
	return cast
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

// OrderReplacements implements §616.1: when multiple replacement effects
// apply to the same event AND tie within a category, the AFFECTED player
// chooses the order. Earlier-applied effects feed their modified payload
// into later-applied ones (CR §614.6), so the order changes the outcome
// when one effect's mutation makes another stop applying.
//
// Heuristic (deterministic, stable sort):
//
//  1. Self-controlled effects first — we benefit most from our own
//     replacements landing before the opponent's.
//  2. Within self-vs-opponent groups, score by event-type benefit:
//     - For events targeting US (TargetSeat == seatIdx):
//     "would_be_dealt_damage" / "would_lose_life" / "would_die" /
//     "would_be_put_into_graveyard" / "would_lose_game" → boost
//     (our replacement likely prevents/redirects, fire it first).
//     - For events benefiting US (TargetSeat == seatIdx, gain side):
//     "would_draw" / "would_gain_life" / "would_put_counter" /
//     "would_create_token" → boost (our replacement likely doubles).
//     - For events targeting OPPONENTS controlled by US:
//     "would_lose_life" / "would_be_dealt_damage" → boost
//     (our replacement likely amplifies — Torment of Hailfire path).
//     Low-life override: when our life is critical (≤ 1/4 starting), every
//     damage/lose-life replacement targeting us pulls to the top.
//  3. Timestamp ascending (older first) breaks remaining ties — matches
//     APNAP fallback in GreedyHat.
//
// Beneficial replacements applied first means the post-mutation payload
// fed to subsequent replacements reflects our gains, so a later opponent
// replacement that would penalize the original event has less to grab.
func (h *YggdrasilHat) OrderReplacements(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.ReplacementEffect) []*gameengine.ReplacementEffect {
	if len(candidates) <= 1 {
		return candidates
	}
	out := make([]*gameengine.ReplacementEffect, len(candidates))
	copy(out, candidates)

	lowLife := false
	if seatIdx >= 0 && seatIdx < len(gs.Seats) && gs.Seats[seatIdx] != nil {
		s := gs.Seats[seatIdx]
		threshold := s.StartingLife / 4
		if threshold < 5 {
			threshold = 5
		}
		lowLife = s.Life <= threshold
	}

	score := func(re *gameengine.ReplacementEffect) int {
		if re == nil {
			return -1 << 30
		}
		s := 0
		// Self-controlled goes first.
		if re.ControllerSeat == seatIdx {
			s += 100
		}
		// Event-type benefit. Without firing the candidate's Applies
		// predicate (would mutate cached event state) we approximate by
		// SourcePerm controller + event type semantics.
		switch re.EventType {
		case "would_be_dealt_damage",
			"would_lose_life",
			"would_die",
			"would_be_put_into_graveyard",
			"would_lose_game":
			// Self-replacement on a harm event = prevention/redirection;
			// fire first so the modified payload reaches downstream effects.
			if re.ControllerSeat == seatIdx {
				s += 30
			}
			if lowLife && re.ControllerSeat == seatIdx {
				s += 200 // existential — pull to top
			}
		case "would_draw", "would_gain_life", "would_put_counter",
			"would_create_token", "would_win_game":
			// Self-replacement on a gain event = amplification (Doubling
			// Season, Alhammarret's Archive, Boon Reflection). Earlier =
			// the doubled count flows into anything downstream.
			if re.ControllerSeat == seatIdx {
				s += 20
			}
		case "would_fire_etb_trigger":
			// Panharmonicon-style: fire amplifiers before suppressors.
			if re.ControllerSeat == seatIdx {
				s += 15
			}
		}
		return s
	}

	sort.SliceStable(out, func(i, j int) bool {
		si, sj := score(out[i]), score(out[j])
		if si != sj {
			return si > sj
		}
		// Older timestamp first — matches GreedyHat fallback for APNAP-
		// style determinism within an otherwise-equal group.
		if out[i] == nil || out[j] == nil {
			return out[i] != nil
		}
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
}

// someOpponentLooksCombo scans each opponent's command zone for a
// commander whose oracle text suggests a combo win condition. Used at
// mulligan time when perceivedArchetype isn't populated yet (no cards
// seen on turn 0). Substring scan only — over-broad rather than miss
// hits, mirroring isRemovalText's style.
//
// Triggers: "infinite", "win the game", "win a game", "extra turn",
// "untap all", "create a copy of", "additional combat", "doesn't
// untap" (e.g. Aetherflux/Niv-Mizzet-class combos use these phrases
// in the commander's own oracle text, not just the supporting deck).
func someOpponentLooksCombo(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost {
			continue
		}
		for _, c := range s.CommandZone {
			if c == nil {
				continue
			}
			ot := gameengine.OracleTextLower(c)
			if ot == "" {
				continue
			}
			if strings.Contains(ot, "infinite") ||
				strings.Contains(ot, "win the game") ||
				strings.Contains(ot, "win a game") ||
				strings.Contains(ot, "extra turn") ||
				strings.Contains(ot, "untap all") ||
				strings.Contains(ot, "create a copy of") ||
				strings.Contains(ot, "additional combat") ||
				strings.Contains(ot, "doesn't untap") {
				return true
			}
		}
	}
	return false
}

// handHasInteraction reports whether `hand` contains at least one card
// that smells like a counterspell or single-target/sweep removal. The
// removal half reuses isRemovalText (opponent_profile.go); the
// counter half does a separate substring scan because counter wording
// ("counter target spell", "counter target ... unless") isn't covered
// by isRemovalText.
func handHasInteraction(hand []*gameengine.Card) bool {
	for _, c := range hand {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if ot == "" {
			continue
		}
		if isRemovalText(ot) {
			return true
		}
		if strings.Contains(ot, "counter target spell") ||
			strings.Contains(ot, "counter target activated") ||
			strings.Contains(ot, "counter that spell") {
			return true
		}
	}
	return false
}

// hasGraveyardRecursionValue returns true if the card has intrinsic
// recursion potential from the graveyard — flashback, unearth, escape,
// disturb, embalm, eternalize, encore, jump-start, aftermath, retrace,
// or dredge. Such cards are strictly better in the graveyard than
// hopelessly stuck in hand when forced to discard. Deck-agnostic: any
// deck running these cards gets the bonus, not just reanimator.
func (h *YggdrasilHat) hasGraveyardRecursionValue(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	return strings.Contains(ot, "flashback") ||
		strings.Contains(ot, "unearth") ||
		strings.Contains(ot, "escape") ||
		strings.Contains(ot, "disturb") ||
		strings.Contains(ot, "embalm") ||
		strings.Contains(ot, "eternalize") ||
		strings.Contains(ot, "encore") ||
		strings.Contains(ot, "jump-start") ||
		strings.Contains(ot, "aftermath") ||
		strings.Contains(ot, "retrace") ||
		strings.Contains(ot, "dredge")
}

// ZoneCastCandidate is one ranked option for the cast-from-graveyard /
// cast-from-exile pipeline. Returned by RankZoneCastCandidates in
// score-descending order so the cast pipeline can take the head as
// the recommended pick.
//
// The current tournament turn loop only scans seat.Hand for castable
// spells — it does NOT consult gs.ZoneCastGrants — so flashback /
// escape / Iroh / Mizzix-shaped grants surfaced by per-card handlers
// are never offered to the hat as a cast choice. This helper is the
// prioritization machinery that becomes load-bearing once the turn
// loop is extended to enumerate zone-cast options (tracked as a
// follow-up in the commit body). Tests pin the ranking contract
// today so the integration only needs to wire the call-through.
type ZoneCastCandidate struct {
	Card       *gameengine.Card
	Permission *gameengine.ZoneCastPermission
	Score      float64
	// Reason carries a short string explaining the top scoring
	// signals — for the decision-replay surface and tests.
	Reason string
}

// RankZoneCastCandidates enumerates every (card, ZoneCastPermission)
// pair on the seat's side that's affordable RIGHT NOW (mana + life
// + colored-pip gates) and returns them sorted high-score-first.
//
// Prioritization signals (additive):
//
//   - Free casts (ManaCost == 0): Iroh's flashback grant, Past in
//     Flames, Yawgmoth's Will, Underworld Breach escape-without-cost
//     paths. +3.0 — pure value with no mana opportunity cost.
//   - Combo-relevant card: +2.5 (decides games — Past-in-Flames
//     re-buying a Demonic Tutor, flashback Tendrils chains).
//   - Value-engine key: +1.5.
//   - Tutor pattern in oracle ("search your library"): +1.5.
//   - Mass-effect ("destroy all" / "exile all" / "win the game"):
//     +2.0 — cast-from-yard board wipes are routinely game-defining.
//   - High-CMC original (CMC ≥ 5): +1.0 per CMC bucket above 5,
//     capped at +2.5. Bigger spells underneath the grant mean more
//     value extracted.
//   - Once-per-turn-per-source already consumed this turn: SKIP
//     (engine-side gate blocks the cast anyway, but the helper
//     should not surface the candidate at all — it'd waste a
//     ChooseCastFromHand window).
func (h *YggdrasilHat) RankZoneCastCandidates(gs *gameengine.GameState, seatIdx int) []ZoneCastCandidate {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) ||
		gs.ZoneCastGrants == nil || len(gs.ZoneCastGrants) == 0 {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	// Build a permission-list per card so CanCastFromZone can apply
	// the engine's affordability + once-per-turn checks uniformly.
	out := make([]ZoneCastCandidate, 0, len(gs.ZoneCastGrants))
	for card, perm := range gs.ZoneCastGrants {
		if card == nil || perm == nil {
			continue
		}
		if perm.RequireController >= 0 && perm.RequireController != seatIdx {
			continue
		}
		// Engine-side affordability + once-per-turn gate.
		approved := gameengine.CanCastFromZone(gs, seatIdx, card, perm.Zone,
			[]*gameengine.ZoneCastPermission{perm})
		if approved == nil {
			continue
		}
		score, reason := h.scoreZoneCastCandidate(gs, seatIdx, card, perm)
		out = append(out, ZoneCastCandidate{
			Card:       card,
			Permission: perm,
			Score:      score,
			Reason:     reason,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Stable tie-break on card name so tests are deterministic.
		return out[i].Card.DisplayName() < out[j].Card.DisplayName()
	})
	return out
}

// scoreZoneCastCandidate returns (score, reason) for a single
// zone-cast option. Pure function so the per-signal contribution is
// testable without spinning up the full Rank flow.
func (h *YggdrasilHat) scoreZoneCastCandidate(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, perm *gameengine.ZoneCastPermission) (float64, string) {
	score := 0.0
	reasons := make([]string, 0, 4)
	// Base: every candidate starts above zero so an unranked
	// affordable cast still surfaces.
	score = 1.0
	if perm.ManaCost == 0 {
		score += 3.0
		reasons = append(reasons, "free")
	}
	if h.isComboRelevant(card) {
		score += 2.5
		reasons = append(reasons, "combo")
	}
	if h.isValueEngineKey(card) {
		score += 1.5
		reasons = append(reasons, "value_engine")
	}
	ot := gameengine.OracleTextLower(card)
	if strings.Contains(ot, "search your library") {
		score += 1.5
		reasons = append(reasons, "tutor")
	}
	if strings.Contains(ot, "destroy all") ||
		strings.Contains(ot, "exile all") ||
		strings.Contains(ot, "win the game") {
		score += 2.0
		reasons = append(reasons, "mass_effect")
	}
	if cmc := gameengine.ManaCostOf(card); cmc >= 5 {
		bump := float64(cmc-4) * 0.5
		if bump > 2.5 {
			bump = 2.5
		}
		score += bump
		reasons = append(reasons, "big_spell")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "baseline")
	}
	return score, strings.Join(reasons, ",")
}

// hasActiveGraveyardCastGrant returns true if the seat has at least one
// ZoneCastGrant whose Zone is "graveyard" — i.e. some permanent (Underworld
// Breach, Yawgmoth's Will-style, Past in Flames residual) currently lets
// this seat cast cards out of their graveyard. The card-side filter (which
// card kinds the grant applies to) varies per source; this helper just
// tells callers "a graveyard cast path exists for this seat right now."
func (h *YggdrasilHat) hasActiveGraveyardCastGrant(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.ZoneCastGrants == nil {
		return false
	}
	for _, perm := range gs.ZoneCastGrants {
		if perm == nil {
			continue
		}
		if perm.RequireController >= 0 && perm.RequireController != seatIdx {
			continue
		}
		if strings.EqualFold(perm.Zone, "graveyard") {
			return true
		}
	}
	return false
}

// hasGraveyardRecursionPotential is the hand-side question this hat asks
// when valuing a card for discard / surveil / put-back / bottom: "is this
// card actually playable from the graveyard?" The answer is yes if EITHER
// the card has an intrinsic recursion keyword (flashback, unearth, etc.)
// OR the seat has an active graveyard-rooted zone-cast grant whose filter
// type (instant/sorcery for Underworld Breach, etc.) covers this card.
//
// We don't try to match the grant's filter precisely — most graveyard-cast
// grants in current MTG cover instants/sorceries, so we approximate by
// treating any non-permanent spell as covered when a grant exists.
// Permanents (creatures/artifacts/enchantments) without an intrinsic
// recursion keyword are not covered by this approximation, since most
// graveyard-cast grants do NOT let you cast permanents from the yard.
func (h *YggdrasilHat) hasGraveyardRecursionPotential(gs *gameengine.GameState, seatIdx int, c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if h.hasGraveyardRecursionValue(c) {
		return true
	}
	if !h.hasActiveGraveyardCastGrant(gs, seatIdx) {
		return false
	}
	if typeLineContains(c, "instant") || typeLineContains(c, "sorcery") {
		return true
	}
	return false
}

// hasGraveyardRecursionEnabler returns true if the seat controls a
// permanent that returns cards from the graveyard to the battlefield
// or hand (Muldrotha, Sun Titan, Meren, Karador, Sheoldred, etc.) or
// has any active zone-cast grant rooted in the graveyard. When this is
// true, putting cards in the graveyard becomes generally valuable for
// any deck — not just reanimator archetypes.
func (h *YggdrasilHat) hasGraveyardRecursionEnabler(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if !strings.Contains(ot, "graveyard") {
			continue
		}
		if !(strings.Contains(ot, "return") || strings.Contains(ot, "put")) {
			continue
		}
		if strings.Contains(ot, "to the battlefield") ||
			strings.Contains(ot, "to your hand") ||
			strings.Contains(ot, "to its owner's hand") {
			return true
		}
	}
	if gs.ZoneCastGrants != nil {
		for _, perm := range gs.ZoneCastGrants {
			if perm == nil {
				continue
			}
			if perm.RequireController >= 0 && perm.RequireController != seatIdx {
				continue
			}
			if perm.Zone == "graveyard" {
				return true
			}
		}
	}
	return false
}

// commanderNamesForSeat returns a copy of the seat's commander names
// (defensive copy — callers iterate without holding the seat). Returns
// nil if the seat has no commanders or the index is out of range.
func commanderNamesForSeat(gs *gameengine.GameState, seatIdx int) []string {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.CommanderNames) == 0 {
		return nil
	}
	out := make([]string, len(seat.CommanderNames))
	copy(out, seat.CommanderNames)
	return out
}

// isCommanderCardName reports whether card matches any of the supplied
// commander names. Case-sensitive match against DisplayName, mirroring
// how the engine compares commander names elsewhere (e.g. evaluator.go
// uses == against seat.CommanderNames entries).
func isCommanderCardName(commanderNames []string, card *gameengine.Card) bool {
	if card == nil || len(commanderNames) == 0 {
		return false
	}
	name := card.DisplayName()
	for _, cn := range commanderNames {
		if cn == name {
			return true
		}
	}
	return false
}

// sacrificePreferenceScore ranks a candidate permanent for forced
// sacrifice. Higher = more willing to feed it to a sac cost. The intent
// is that recurring permanents (persist, undying, unearth, "return to
// hand on death" triggers, dredge-style escape value) are net-positive
// to sacrifice; vanilla creatures are neutral; key threats and the
// commander are last-resort.
func (h *YggdrasilHat) sacrificePreferenceScore(gs *gameengine.GameState, seatIdx int, p *gameengine.Permanent) float64 {
	if p == nil || p.Card == nil {
		return -1e6
	}
	score := 0.0
	c := p.Card
	ot := gameengine.OracleTextLower(c)

	// Tokens have no card-form to lose — sacrificing them costs only the
	// board presence. They go on top.
	if p.IsToken() {
		score += 6.0
	}

	// Persist (CR §702.79) returns the creature with a -1/-1 counter, so
	// long as it doesn't have one already. Undying (§702.93) returns it
	// with a +1/+1 counter, so long as it doesn't have one already.
	if strings.Contains(ot, "persist") {
		if p.Counters == nil || p.Counters["-1/-1"] == 0 {
			score += 5.0
		}
	}
	if strings.Contains(ot, "undying") {
		if p.Counters == nil || p.Counters["+1/+1"] == 0 {
			score += 5.0
		}
	}

	// Unearth / escape / embalm / eternalize / encore: card returns or
	// can be re-cast from graveyard.
	if strings.Contains(ot, "unearth") {
		score += 3.5
	}
	if strings.Contains(ot, "escape") {
		score += 3.0
	}
	if strings.Contains(ot, "embalm") || strings.Contains(ot, "eternalize") {
		score += 2.5
	}
	if strings.Contains(ot, "encore") {
		score += 2.0
	}

	// "When ~ dies/leaves the battlefield, return it to your hand/owner's
	// hand" (Reassembling Skeleton, Bloodghast-style returns). Conservative
	// pattern: oracle text mentions both "dies" and "return ... to your
	// hand" (or "to its owner's hand").
	if strings.Contains(ot, "dies") &&
		(strings.Contains(ot, "return it to its owner's hand") ||
			strings.Contains(ot, "return it to your hand") ||
			strings.Contains(ot, "to your hand") ||
			strings.Contains(ot, "to your owner's hand")) {
		score += 3.0
	}

	// Dies-trigger that draws or otherwise pays off death (Bone Shards,
	// Bridge from Below proxy, blood artist, etc.).
	if strings.Contains(ot, "when") && strings.Contains(ot, "dies") &&
		(strings.Contains(ot, "draw") || strings.Contains(ot, "create") ||
			strings.Contains(ot, "each opponent")) {
		score += 1.5
	}

	// Penalize value: commanders, equipped/buffed key creatures, finishers.
	if gameengine.IsCommanderCard(gs, seatIdx, c) {
		score -= 5.0
	}
	if h.finisherSet != nil && h.finisherSet[c.DisplayName()] {
		score -= 3.0
	}
	if h.comboPieceSet != nil && h.comboPieceSet[c.DisplayName()] {
		score -= 4.0
	}
	if h.valueEngineSet != nil && h.valueEngineSet[c.DisplayName()] {
		score -= 2.0
	}

	// Mild penalty proportional to the creature's combat power so a vanilla
	// 6/6 isn't sacrificed before a vanilla 1/1 if neither has recursion.
	if p.IsCreature() {
		// p.Card.BasePower can be negative (mods); clamp.
		pow := c.BasePower
		if pow < 0 {
			pow = 0
		}
		score -= float64(pow) * 0.1
	} else {
		// Non-creature permanents (artifacts, enchantments) without
		// recursion are usually doing more work in play than in the
		// graveyard — small penalty.
		score -= 0.5
	}

	return score
}

// -- Interface: SacrificeChooser --

// ChooseSacrifice picks the permanent we'd most happily lose. See
// sacrificePreferenceScore for the priority schedule.
func (h *YggdrasilHat) ChooseSacrifice(gs *gameengine.GameState, seatIdx int, source *gameengine.Permanent, reason string, candidates []*gameengine.Permanent) *gameengine.Permanent {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	bestIdx := 0
	bestScore := h.sacrificePreferenceScore(gs, seatIdx, candidates[0])
	for i := 1; i < len(candidates); i++ {
		sc := h.sacrificePreferenceScore(gs, seatIdx, candidates[i])
		if sc > bestScore {
			bestScore = sc
			bestIdx = i
		}
	}
	return candidates[bestIdx]
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
		sources = CountManaRocksAndLands(gs.Seats[seatIdx])
	}
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}
	hasEnabler := h.hasGraveyardRecursionEnabler(gs, seatIdx)
	commanderNames := commanderNamesForSeat(gs, seatIdx)
	// Lands-in-hand vs board-mana balance + current-turn castability. Count
	// the lands actually in hand and estimate available mana so we can (a)
	// protect the LAST land when we're still short on board mana, and (b)
	// prefer pitching a stranded uncastable bomb over a castable action card.
	landsInHand := 0
	for _, c := range hand {
		if c != nil && typeLineContains(c, "land") {
			landsInHand++
		}
	}
	availMana := 0
	if seatIdx >= 0 && seatIdx < len(gs.Seats) {
		availMana = gameengine.AvailableManaEstimate(gs, gs.Seats[seatIdx])
	}
	for _, c := range hand {
		if c == nil {
			continue
		}
		v := h.cardHeuristic(gs, seatIdx, c)
		isLandCard := typeLineContains(c, "land")
		if isLandCard && sources >= 5 {
			v -= 0.5
		}
		// Last-land protection: never pitch our only land in hand while we're
		// still building board mana (mirrors the sources<3 starvation gate
		// below). Losing the last land while mana-light strands the whole
		// hand — protect it dominantly. Aligned to sources<3 so the existing
		// sources==3 boundary contract is preserved.
		if isLandCard && landsInHand == 1 && sources < 3 {
			v += 5.0
		}
		// Current-turn castability. A card we can actually cast this turn is
		// live; a stranded high-CMC bomb we can't cast for several turns is a
		// better pitch than a castable action. Lands are exempt (they're a
		// resource, not a "cast").
		if !isLandCard {
			cmc := gameengine.ManaCostOf(c)
			if cmc <= availMana && availMana > 0 {
				v += 0.4
			} else if cmc >= availMana+3 {
				// Uncastable for the foreseeable future — mild pitch nudge,
				// not dominating, so genuine bombs/engines still survive via
				// their combo/VE/star bonuses.
				v -= 0.3
			}
		}
		// Mana-starvation land protection. With fewer than three mana
		// sources the next land drop matters far more than any card in
		// hand — protect lands so they don't get discarded when we're
		// behind on mana. Mirror of the sources>=5 flood penalty above.
		if typeLineContains(c, "land") && sources < 3 {
			v += 3.0
		}
		// Commander-in-hand protection. Discarding the commander throws
		// away a card the deck is built around; the §903.9 replacement
		// only matters once the commander has been cast and is being
		// moved between non-hand zones. From hand, "discarded" is just
		// "into the graveyard" with no command-zone redirect. We add a
		// dominating positive bonus so the commander is the last card
		// considered for discard short of being the only option.
		if isCommanderCardName(commanderNames, c) {
			v += 10.0
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
		// Cards with graveyard recursion potential are better in the
		// graveyard than rotting in hand. Deck-agnostic. Triggers on either
		// (a) intrinsic keywords (flashback, unearth, escape, disturb...)
		// or (b) instants/sorceries when the seat has an active graveyard
		// cast grant in play (Underworld Breach, Past in Flames residual).
		if h.hasGraveyardRecursionPotential(gs, seatIdx, c) {
			v -= 0.4 * h.graveyardExploitationMult()
		}
		// Reanimator archetype OR any deck with a graveyard-recursion
		// enabler on the battlefield (Muldrotha, Sun Titan, Meren, etc.)
		// wants high-CMC creatures in the yard.
		if (arch == ArchetypeReanimator || hasEnabler) && typeLineContains(c, "creature") {
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
	h.recordCascadeDecision(gs)
	// Stack resolves LIFO — last item resolves first. Put highest-priority
	// triggers at the END so they resolve first.
	sort.SliceStable(triggers, func(i, j int) bool {
		return h.triggerPriority(triggers[i]) < h.triggerPriority(triggers[j])
	})
	return triggers
}

func (h *YggdrasilHat) triggerPriority(item *gameengine.StackItem) float64 {
	if item == nil {
		return 0
	}
	// R60 cascading-decisions audit. OrderTriggers exclusively receives
	// triggered abilities (per CR §603.3b same-event same-controller
	// stack), but pre-R60 this helper dereferenced item.Card — which is
	// always nil for triggers. Every priority returned 0 and the
	// sort.SliceStable was a silent no-op; trigger ordering was
	// effectively insertion-order. effectiveResponseCard resolves the
	// source permanent's card so the trigger payload's oracle text
	// drives the ordering.
	card := effectiveResponseCard(item)
	if card == nil {
		return 0
	}
	pri := 0.0
	if h.isComboRelevant(card) {
		pri += 3.0
	}
	if h.isValueEngineKey(card) {
		pri += 2.0
	}
	ot := gameengine.OracleTextLower(card)
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
	h.recordCascadeDecision(gs)
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

// -- Interface: ChooseKickCount --
//
// Cast-time optional additional cost (CR §702.33). The base spell has
// already been paid for by the time this hook fires, so kicking can never
// strand the base cast — the only question is whether the leftover mana is
// better spent on the kicker now or held for interaction.
//
// Multikicker (maxKicks > 1, e.g. Everflowing Chalice / Astral Cornucopia):
// every kick scales the permanent's payoff, so spend all affordable kicks —
// leftover mana is otherwise wasted this cast. Control/Stax shave one kick
// to hold a mana for a cheap response when not maxing out a critical engine.
//
// Single kicker (maxKicks == 1): kick once when affordable — almost every
// printed single-kicker grants a strictly-positive "if kicked" bonus, so
// taking it is the right baseline; Control/Stax skip it for a non-critical
// spell to keep interaction mana up.
func (h *YggdrasilHat) ChooseKickCount(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, kickerCost, maxKicks int) int {
	if maxKicks <= 0 {
		return 0
	}
	holdBack := false
	if h.Strategy != nil {
		arch := h.Strategy.Archetype
		critical := h.isComboRelevant(card) || h.isValueEngineKey(card)
		if !critical && (arch == ArchetypeControl || arch == ArchetypeStax) {
			holdBack = true
		}
	}
	if holdBack {
		if maxKicks > 1 {
			return maxKicks - 1 // keep one kick's worth of mana for interaction
		}
		return 0 // single kicker on a non-critical spell: hold the mana
	}
	return maxKicks
}

// -- Interface: ChooseOptionalCost --
//
// PR-5 cast-time cost-mechanic family (CR §601.2b/f). Yggdrasil refines the
// greedy baseline with archetype + board awareness:
//
//   - replicate: spend all affordable copies; Control/Stax shave one to keep
//     interaction mana up unless the spell is a combo/value-engine key.
//   - overload: take it whenever ≥2 targets exist on the battlefield (the
//     "each" fan-out only earns its cost when it hits multiple permanents);
//     otherwise decline so a one-target overload isn't overpaid.
//   - buyback: take it when mana-flush (≥2 leftover after the cost) so the
//     repeatable spell becomes a value engine; otherwise hold the mana.
//   - surge/spectacle/conspire: take the discount/free copy whenever offered.
//   - casualty/bargain: SACRIFICE costs — take only when the deck wants the
//     sacrifice (Aristocrats/Sacrifice/Combo archetypes), else decline so the
//     hat doesn't trade board presence for a single spell.
func (h *YggdrasilHat) ChooseOptionalCost(gs *gameengine.GameState, seatIdx int, card *gameengine.Card, kind string, cost, max int) int {
	if max <= 0 {
		return 0
	}
	critical := false
	arch := ""
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
		critical = h.isComboRelevant(card) || h.isValueEngineKey(card)
	}
	defensive := arch == ArchetypeControl || arch == ArchetypeStax

	switch kind {
	case "replicate":
		if defensive && !critical && max > 1 {
			return max - 1
		}
		return max
	case "overload":
		// "each" fan-out only pays for itself with ≥2 affected permanents.
		if h.overloadTargetCount(gs, seatIdx, card) >= 2 {
			return max
		}
		return 0
	case "buyback":
		if avail := h.availableMana(gs, seatIdx); avail-cost >= 2 || critical {
			return max
		}
		return 0
	case "casualty", "bargain":
		if arch == ArchetypeAristocrats || arch == ArchetypeCombo || critical {
			return max
		}
		return 0
	default: // surge, spectacle, conspire — discounts / free copies
		return max
	}
}

// overloadTargetCount estimates how many permanents an overloaded spell
// would hit by counting opponents' nonland permanents — the common targets
// of overloaded bounce/destroy effects (Cyclonic Rift, Vandalblast). A coarse
// proxy that is right for the canonical overload cards and conservative for
// the rest (declining a 0/1-target overload).
func (h *YggdrasilHat) overloadTargetCount(gs *gameengine.GameState, seatIdx int, card *gameengine.Card) int {
	if gs == nil {
		return 0
	}
	count := 0
	for i, seat := range gs.Seats {
		if seat == nil || i == seatIdx {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !p.IsLand() {
				count++
			}
		}
	}
	return count
}

// availableMana reports the seat's current untapped mana pool total.
func (h *YggdrasilHat) availableMana(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return 0
	}
	return gs.Seats[seatIdx].ManaPool
}

// -- Interface: ChooseBottomCards --

func (h *YggdrasilHat) ChooseBottomCards(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card, count int) []*gameengine.Card {
	if count <= 0 || len(hand) == 0 {
		return nil
	}
	if count >= len(hand) {
		return hand
	}
	// Bottom the worst cards by an enriched heuristic. cardHeuristic
	// alone misses the combo / value-engine / star / cuttable biases
	// that ChooseDiscard applies, so a combo piece on a London bottom
	// would sit at the same value as a vanilla creature. Mirror the
	// ChooseDiscard weights so the bottom pile is consistent with the
	// discard pile.
	type ranked struct {
		card   *gameengine.Card
		value  float64
		isLand bool
	}
	ranked_ := make([]ranked, 0, len(hand))
	landsInHand := 0
	// R60 round 6+ London-mulligan signals: compute hand-wide context
	// once, then apply per-card biases below. See
	// mulligan_bottom_signals_r60.go for the rationale.
	handColorMask := handProducedColorMask(hand)
	cheapPlayables := countCheapPlayables(hand)
	for _, c := range hand {
		if c == nil {
			continue
		}
		v := h.cardHeuristic(gs, seatIdx, c)
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
		// London signals — small biases that don't override the existing
		// scaffolding but shift tiebreakers toward keep-side cards the
		// hand can actually cast on curve.
		if cardColorDeadAgainstHand(c, handColorMask) {
			v -= 0.4
		}
		if isEarlyPlayAnchor(c, cheapPlayables) {
			v += 0.6
		}
		isLand := typeLineContains(c, "land")
		if isLand {
			landsInHand++
		}
		ranked_ = append(ranked_, ranked{c, v, isLand})
	}
	sort.SliceStable(ranked_, func(i, j int) bool {
		return ranked_[i].value < ranked_[j].value
	})

	// Land floor: if the original hand had at least 2 lands, never
	// bottom into a sub-2-land hand. We swap a land out of the bottom
	// pile for the next non-land candidate until the keep-side floor
	// holds. Applies only at the London 7→6 / 6→5 transition where
	// keeping at least 2 lands materially changes the keep value;
	// when the original hand already has <2 lands the floor is moot
	// (we don't have the supply to honor it).
	if landsInHand >= 2 {
		const landFloor = 2
		// Iteratively pull lands out of the bottom slice until either
		// (a) bottoming the chosen `count` leaves landFloor lands in
		// hand, or (b) there's no more non-land in the keep pile to
		// swap up. Bounded by `count` so always terminates.
		for attempt := 0; attempt < count; attempt++ {
			landsBottomed := 0
			lastLandBottomIdx := -1
			for i := 0; i < count; i++ {
				if ranked_[i].isLand {
					landsBottomed++
					lastLandBottomIdx = i
				}
			}
			if landsInHand-landsBottomed >= landFloor {
				break
			}
			// Need to swap a land out. Find the highest-priority
			// non-land card currently in the keep pile (i.e. the
			// first non-land at or after index `count`).
			swapIdx := -1
			for i := count; i < len(ranked_); i++ {
				if !ranked_[i].isLand {
					swapIdx = i
					break
				}
			}
			if swapIdx < 0 || lastLandBottomIdx < 0 {
				break
			}
			ranked_[lastLandBottomIdx], ranked_[swapIdx] = ranked_[swapIdx], ranked_[lastLandBottomIdx]
		}
	}

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
	arch := ArchetypeMidrange
	if h.Strategy != nil {
		arch = h.Strategy.Archetype
	}
	// Combo archetype additionally tightens the keep threshold so
	// filler cards default to the bottom and the next draw is more
	// likely to hit a combo piece or tutor. Mirror of the R60
	// ChooseSurveil bias; the per-archetype lift stacks on top of the
	// relPos-based threshold above.
	if arch == ArchetypeCombo && threshold < 0.50 {
		threshold = 0.50
	}
	for _, c := range cards {
		if c == nil {
			bottom = append(bottom, c)
			continue
		}
		val := h.cardHeuristic(gs, seatIdx, c)
		// Finisher preserve-mode: a card the deck wins with stays on
		// top regardless of val or archetype. This is the "card on top
		// is the only thing keeping us in" guard — when we're behind,
		// the next draw being our finisher is the lifeline; sub-
		// threshold-bottoming it is a strict mistake. Stacks on top of
		// the combo / value-engine / star keep branch (which already
		// covers cards Freya tagged as engine pieces) — finishers
		// aren't always in StarCards so this is the gap-closer.
		if h.isComboRelevant(c) || h.isValueEngineKey(c) || h.isStarCard(c) || h.isFinisher(c) {
			top = append(top, c)
			continue
		}
		// Control archetype creature filter — non-keeper creatures
		// (we've already filtered keepers above) go to the bottom.
		// Control decks don't need creature density.
		if arch == ArchetypeControl && typeLineContains(c, "creature") {
			bottom = append(bottom, c)
			continue
		}
		if h.isCuttable(c) {
			bottom = append(bottom, c)
			continue
		}
		if val >= threshold {
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
	hasEnabler := h.hasGraveyardRecursionEnabler(gs, seatIdx)
	for _, c := range cards {
		if c == nil {
			graveyard = append(graveyard, c)
			continue
		}
		val := h.cardHeuristic(gs, seatIdx, c)

		// Cards with graveyard recursion potential belong in the yard
		// regardless of deck — castable via intrinsic keyword (flashback,
		// unearth, escape...) or via an active graveyard cast grant on
		// the battlefield covering this card type.
		if h.hasGraveyardRecursionPotential(gs, seatIdx, c) {
			graveyard = append(graveyard, c)
			continue
		}

		// Reanimator (or any deck with a graveyard-recursion enabler on
		// battlefield) wants fatties in the graveyard.
		if (arch == ArchetypeReanimator || hasEnabler) && typeLineContains(c, "creature") {
			cmc := gameengine.ManaCostOf(c)
			if cmc >= 5 {
				graveyard = append(graveyard, c)
				continue
			}
		}

		if h.isComboRelevant(c) || h.isValueEngineKey(c) || h.isStarCard(c) {
			top = append(top, c)
			continue
		}
		// Archetype bias: control sends non-keeper creatures to the
		// graveyard. Control decks don't need creature density — what
		// reaches the battlefield is usually a single finisher and the
		// commander, and excess creatures rot in hand. We've already
		// filtered combo / VE / star keepers above so this only fires
		// on filler creatures.
		if arch == ArchetypeControl && typeLineContains(c, "creature") {
			graveyard = append(graveyard, c)
			continue
		}
		if h.isCuttable(c) {
			graveyard = append(graveyard, c)
			continue
		}
		// Archetype bias: combo decks want the top of library reserved
		// for combo pieces and tutors. Raise the keep threshold so
		// mid-quality filler is sent to the graveyard instead of held
		// at the top — the next draw should hit something that builds
		// toward the win condition. Other archetypes keep the 0.35
		// threshold so they don't accidentally bottom into card
		// disadvantage.
		topThreshold := 0.35
		if arch == ArchetypeCombo {
			topThreshold = 0.50
		}
		if val >= topThreshold {
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

// -- Interface: ShouldConcede --
// Disabled: score-based "conviction" concession was too aggressive —
// hat scooped at 38 life because it felt behind. Concession should only
// happen for infinite loops / resolver lockups, which the engine handles
// via SBA cap (infinite_loop_draw) and stack loopDetector.
// Everyone fights to the death.
//
// Non-acting diagnostic still runs: each turn we record what candidate
// conviction triggers (score window, win-line extinction) would have
// decided. The return value is unconditionally false; the samples are
// emitted as "conviction_diagnostic" events for post-game analysis per
// docs/conviction-reassessment-2026-05-17.md.

func (h *YggdrasilHat) ShouldConcede(gs *gameengine.GameState, seatIdx int) bool {
	h.recordConvictionSample(gs, seatIdx)
	return false
}

// -- Interface: ObserveEvent --

func (h *YggdrasilHat) ObserveEvent(gs *gameengine.GameState, seatIdx int, event *gameengine.Event) {
	if event == nil {
		return
	}

	// Deterministic noise seeding (r62): the constructors seed noiseRNG
	// from the global rand, which made every targeting/selection decision
	// nondeterministic and broke seed replay (Loki repro, ELO parity,
	// anticheat verify). The first event of a seeded game reaches the hat
	// before any noise consumer runs (setup/draw/mulligan events precede
	// all decisions that call applyNoise/selectAmongTop), so reseeding
	// here guarantees the whole noise stream is a pure function of
	// (gs.Seed, seatIdx). Unseeded games (Seed==0) keep the legacy
	// global-rand stream. Keying on the seed value (not a bool) also
	// reseeds correctly if a hat instance is reused across games.
	if gs != nil && gs.Seed != 0 && h.noiseSeededFor != gs.Seed {
		h.noiseSeededFor = gs.Seed
		h.noiseRNG = rand.New(rand.NewSource(noiseSeedFor(gs.Seed, seatIdx)))
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
		h.poisonReceivedFrom = make([]int, h.seatCount)
		h.opponentHandEntropy = make([]float64, h.seatCount)
		h.opponentHeldMana = make([]int, h.seatCount)
		h.opponentMaxHeldMana = make([]int, h.seatCount)
		h.opponentTutored = make([]bool, h.seatCount)
		h.opponentCounterspellsCast = make([]int, h.seatCount)
		h.opponentMassRemovalCast = make([]int, h.seatCount)
		h.opponentTutorsCast = make([]int, h.seatCount)
		h.opponentLoadedSilentTurns = make([]int, h.seatCount)
		h.opponentFiredInteractionThisRound = make([]bool, h.seatCount)
		h.opponentKnownCards = make([]map[string]bool, h.seatCount)
		h.linkedExilesByOpponent = make([]int, h.seatCount)
		h.myZoneCastGrants = make(map[string]int)
		h.opponentProfiles = make([]*OpponentProfile, h.seatCount)
		for i := 0; i < h.seatCount; i++ {
			h.cardsSeen[i] = make(map[string]int)
			h.politicalGraph[i] = make([]int, h.seatCount)
			h.opponentColors[i] = make(map[string]bool)
			h.opponentHandEntropy[i] = 1.0 // start fully unknown
			h.opponentKnownCards[i] = make(map[string]bool)
			h.opponentProfiles[i] = &OpponentProfile{Archetype: "unknown"}
		}
	}

	// Reset on game start.
	if event.Kind == "game_start" {
		h.actionStats = make(map[string]*actionStat)
		h.totalVisits = 0
		h.rolloutSeed = 0
		h.stackItemTiers = nil
		h.stackItemTiersTurn = 0
		h.lastChooseModePick = nil
		h.lastChooseModeSlice = nil
		h.lastChooseModeTurn = 0
		h.planState = PlanState{}
		h.Evaluator.PlanMultiplier = nil
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
			if i < len(h.poisonReceivedFrom) {
				h.poisonReceivedFrom[i] = 0
			}
			if i < len(h.opponentHandEntropy) {
				h.opponentHandEntropy[i] = 1.0
			}
			if i < len(h.opponentHeldMana) {
				h.opponentHeldMana[i] = 0
			}
			if i < len(h.opponentMaxHeldMana) {
				h.opponentMaxHeldMana[i] = 0
			}
			if i < len(h.opponentTutored) {
				h.opponentTutored[i] = false
			}
			if i < len(h.opponentCounterspellsCast) {
				h.opponentCounterspellsCast[i] = 0
			}
			if i < len(h.opponentMassRemovalCast) {
				h.opponentMassRemovalCast[i] = 0
			}
			if i < len(h.opponentTutorsCast) {
				h.opponentTutorsCast[i] = 0
			}
			if i < len(h.opponentLoadedSilentTurns) {
				h.opponentLoadedSilentTurns[i] = 0
			}
			if i < len(h.opponentFiredInteractionThisRound) {
				h.opponentFiredInteractionThisRound[i] = false
			}
			if i < len(h.opponentKnownCards) {
				h.opponentKnownCards[i] = make(map[string]bool)
			}
			if i < len(h.linkedExilesByOpponent) {
				h.linkedExilesByOpponent[i] = 0
			}
			if i < len(h.opponentProfiles) {
				h.opponentProfiles[i] = &OpponentProfile{Archetype: "unknown"}
			}
		}
		h.linkedExilesByMe = 0
		if h.myZoneCastGrants == nil {
			h.myZoneCastGrants = make(map[string]int)
		} else {
			for k := range h.myZoneCastGrants {
				delete(h.myZoneCastGrants, k)
			}
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

	// 3rd Eye: Track poison counters received per opponent.
	if event.Kind == "poison" && event.Amount > 0 {
		if event.Target == seatIdx && event.Seat >= 0 && event.Seat < len(h.poisonReceivedFrom) {
			h.poisonReceivedFrom[event.Seat] += event.Amount
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
			// Update the rolling per-opponent profile (creature
			// counts, removal/counter/tutor classification, combo
			// piece detection).
			card := findCardByName(gs, event.Seat, event.Source)
			h.recordOpponentPlay("cast", event.Source, event.Seat, card, gs.Turn)
			// R60: classify the cast against the three 3rd Eye
			// magnitude signals — counterspells / mass removal /
			// tutors. Lookups against the card's lower-cased oracle
			// via the shared isCounterspellText / isMassRemovalText /
			// isTutorText helpers. Counts feed downstream
			// "they ran out of counter mana" / "they have no wraths
			// left" / "they tutored AGAIN" predictions.
			if card != nil {
				ot := gameengine.OracleTextLower(card)
				if isCounterspellText(ot) && event.Seat < len(h.opponentCounterspellsCast) {
					h.opponentCounterspellsCast[event.Seat]++
				}
				if isMassRemovalText(ot) && event.Seat < len(h.opponentMassRemovalCast) {
					h.opponentMassRemovalCast[event.Seat]++
				}
				// NOTE: tutor increment lives in the explicit
				// tutor/search_library handler below, NOT here. Spell
				// tutors (Demonic Tutor) emit BOTH a cast event and a
				// resolve-side tutor event; counting in the explicit
				// handler avoids the double-count while also covering
				// activated/triggered tutors (Survival of the Fittest,
				// fetchland cracks) that don't emit a cast event.
			}
		}
	}

	// Track land plays + ETBs for opponent profiles.
	if event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		switch event.Kind {
		case "play_land":
			h.recordOpponentPlay("play_land", event.Source, event.Seat, nil, gs.Turn)
		case "permanent_etb", "creature_etb":
			card := findCardByName(gs, event.Seat, event.Source)
			h.recordOpponentPlay(event.Kind, event.Source, event.Seat, card, gs.Turn)
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

	// -- 3rd Eye: Shannon entropy hand tracking --

	// Tutor/search_library resolved → near-zero entropy for one slot.
	// They found exactly what they wanted.
	if (event.Kind == "tutor" || event.Kind == "search_library") &&
		event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		if event.Seat < len(h.opponentTutored) {
			h.opponentTutored[event.Seat] = true
		}
		// R60: increment the tutor magnitude counter. The cast-side
		// handler above also fires for tutors-as-spells, so this
		// branch is the catch-all for direct tutor/search_library
		// events that don't pass through the cast pipeline (e.g.,
		// activated abilities like Fetchland cracks, Survival of the
		// Fittest, Eladamri's Call from a permanent).
		if event.Seat < len(h.opponentTutorsCast) {
			h.opponentTutorsCast[event.Seat]++
		}
		h.recordOpponentPlay(event.Kind, event.Source, event.Seat, nil, gs.Turn)
		// Reduce entropy: they now hold a known-purpose card.
		if event.Seat < len(h.opponentHandEntropy) {
			h.opponentHandEntropy[event.Seat] *= 0.6
			if h.opponentHandEntropy[event.Seat] < 0.1 {
				h.opponentHandEntropy[event.Seat] = 0.1
			}
		}
	}

	// Draw events → increase entropy (more unknown cards in hand).
	if event.Kind == "draw" && event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		if event.Seat < len(h.opponentHandEntropy) {
			drawn := event.Amount
			if drawn < 1 {
				drawn = 1
			}
			// Each draw adds uncertainty. Scale: 1 card = +0.08, 3 cards = +0.24.
			increase := float64(drawn) * 0.08
			h.opponentHandEntropy[event.Seat] += increase
			if h.opponentHandEntropy[event.Seat] > 1.0 {
				h.opponentHandEntropy[event.Seat] = 1.0
			}
		}
	}

	// Reveal events → zero entropy for those specific cards (known).
	if event.Kind == "reveal" && event.Source != "" &&
		event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		if event.Seat < len(h.opponentKnownCards) {
			h.opponentKnownCards[event.Seat][event.Source] = true
		}
		// Knowing cards reduces overall hand entropy.
		if event.Seat < len(h.opponentHandEntropy) {
			h.opponentHandEntropy[event.Seat] -= 0.15
			if h.opponentHandEntropy[event.Seat] < 0.0 {
				h.opponentHandEntropy[event.Seat] = 0.0
			}
		}
	}

	// Cast events from opponents → remove from known cards if it was known,
	// and reset held-mana counter (they used their mana).
	if event.Kind == "cast" && event.Source != "" &&
		event.Seat >= 0 && event.Seat < h.seatCount && event.Seat != seatIdx {
		if event.Seat < len(h.opponentKnownCards) {
			delete(h.opponentKnownCards[event.Seat], event.Source)
		}
		if event.Seat < len(h.opponentHeldMana) {
			h.opponentHeldMana[event.Seat] = 0
		}
		// R60 round 5 — bluff signal. If this cast IS an interactive
		// spell, the opponent demonstrably has interaction (no bluff)
		// and we reset their loaded-silent streak. The flag is sticky
		// across the round until the next upkeep evaluation.
		if isInteractionSpellName(event.Source) {
			if event.Seat < len(h.opponentFiredInteractionThisRound) {
				h.opponentFiredInteractionThisRound[event.Seat] = true
			}
			if event.Seat < len(h.opponentLoadedSilentTurns) {
				h.opponentLoadedSilentTurns[event.Seat] = 0
			}
		}
	}

	// -- Zone-cast grant lifecycle --
	//
	// The engine emits zone_cast_grant_registered when a card gains a
	// permission to be cast from a non-hand zone (flashback, escape,
	// impulse-draw exile-cast, Misthollow Griffin, Bolas's Citadel).
	// We track our own grants by keyword for fast counts; the
	// authoritative store is gs.ZoneCastGrants and is read directly by
	// AvailableZoneCastGrants() during decision-making.
	switch event.Kind {
	case "zone_cast_grant_registered":
		if event.Seat == seatIdx {
			keyword := ""
			if event.Details != nil {
				if k, ok := event.Details["keyword"].(string); ok {
					keyword = k
				}
			}
			if h.myZoneCastGrants == nil {
				h.myZoneCastGrants = make(map[string]int)
			}
			h.myZoneCastGrants[keyword]++
		}
	case "zone_cast_grant_expired":
		if event.Seat == seatIdx && h.myZoneCastGrants != nil {
			keyword := ""
			if event.Details != nil {
				if k, ok := event.Details["keyword"].(string); ok {
					keyword = k
				}
			}
			if h.myZoneCastGrants[keyword] > 0 {
				h.myZoneCastGrants[keyword]--
				if h.myZoneCastGrants[keyword] == 0 {
					delete(h.myZoneCastGrants, keyword)
				}
			}
		}
	case "exile_linked_created":
		// Track who exiled cards via a linked permanent. For us: this
		// represents value tied up in a fragile permanent we should
		// protect. For opponents: high counts flag good removal targets
		// since killing the source returns the exiled card.
		if event.Seat == seatIdx {
			h.linkedExilesByMe++
		} else if event.Seat >= 0 && event.Seat < len(h.linkedExilesByOpponent) {
			h.linkedExilesByOpponent[event.Seat]++
		}
	case "exile_linked_returned":
		// Source permanent left the battlefield and returned its linked
		// cards. event.Seat is unset on this event; instead use Source
		// to identify whose perm returned. Best-effort: subtract Amount
		// from whichever bucket has any outstanding count for that
		// permanent. We don't track per-source granularly, so amortise
		// across all seats by clearing proportionally.
		amount := event.Amount
		if amount <= 0 {
			amount = 1
		}
		// Find the most-likely source seat: the one with the largest
		// outstanding count. Falls back gracefully if everything is 0.
		bestSeat := -1
		bestCount := 0
		if h.linkedExilesByMe > bestCount {
			bestSeat = seatIdx
			bestCount = h.linkedExilesByMe
		}
		for i, n := range h.linkedExilesByOpponent {
			if i == seatIdx {
				continue
			}
			if n > bestCount {
				bestSeat = i
				bestCount = n
			}
		}
		if bestSeat == seatIdx {
			h.linkedExilesByMe -= amount
			if h.linkedExilesByMe < 0 {
				h.linkedExilesByMe = 0
			}
		} else if bestSeat >= 0 && bestSeat < len(h.linkedExilesByOpponent) {
			h.linkedExilesByOpponent[bestSeat] -= amount
			if h.linkedExilesByOpponent[bestSeat] < 0 {
				h.linkedExilesByOpponent[bestSeat] = 0
			}
		}
	}

	// Upkeep: check if opponents passed the previous turn with mana open.
	// We piggyback on the upkeep event (already used for trajectory snapshots)
	// to evaluate each opponent's mana state at the start of a new turn cycle.
	if event.Kind == "upkeep" && gs != nil {
		for i, s := range gs.Seats {
			if i == seatIdx || s == nil || s.Lost || s.LeftGame {
				continue
			}
			if i >= len(h.opponentHeldMana) {
				continue
			}
			openMana := gameengine.AvailableManaEstimate(gs, s)
			if openMana >= 2 {
				h.opponentHeldMana[i]++
			} else {
				h.opponentHeldMana[i] = 0
			}
			// R60: track the largest open-mana value we've ever observed
			// at this opponent's upkeep. The streak counter above is
			// binary at 2+; this captures the magnitude — a 4+ reading
			// is Cryptic Command / Force of Negation / Mystic Confluence
			// territory, a different counterspell threshold than a
			// 2-mana Spell Pierce / Mana Leak rep. Doesn't decay — once
			// we've seen the opponent hold 4 mana, the deck is capable
			// of that rep even if they later spend down. (Future work
			// could add a recency window; magnitude-ever-seen is the
			// minimal first-pass signal.)
			if i < len(h.opponentMaxHeldMana) && openMana > h.opponentMaxHeldMana[i] {
				h.opponentMaxHeldMana[i] = openMana
			}

			// R60 round 5 — bluff signal tally. If this opponent looked
			// loaded at the start of the previous round but did NOT
			// cast any interaction during it, their loaded-silent
			// streak grows. The streak is what `perceivedInteractionThreat`
			// uses to dampen the raw `opponentHasInteraction` probability.
			if i < len(h.opponentLoadedSilentTurns) && i < len(h.opponentFiredInteractionThisRound) {
				if !h.opponentFiredInteractionThisRound[i] {
					currentProb := h.opponentHasInteraction(gs, i)
					if currentProb >= bluffLoadedThreshold {
						if h.opponentLoadedSilentTurns[i] < bluffMaxStreak {
							h.opponentLoadedSilentTurns[i]++
						}
					}
				}
				// Reset the per-round flag for the upcoming round.
				h.opponentFiredInteractionThisRound[i] = false
			}
		}
	}

	// Per-turn threat trajectory snapshot.
	if event.Kind == "upkeep" && gs != nil {
		for i, s := range gs.Seats {
			if s == nil || s.Lost || s.LeftGame {
				continue
			}
			bp := boardPower(gs, s)
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

		// Plan state machine: evaluate combo status and threat level,
		// then transition if conditions warrant.
		var comboAssess *ComboAssessment
		if h.comboSeq != nil {
			ca := h.comboSeq.Evaluate(gs, seatIdx)
			comboAssess = &ca
		}
		maxThreat := 0.0
		threats := h.assessAllThreats(gs, seatIdx)
		for _, t := range threats {
			if t.EvalScore > maxThreat {
				maxThreat = t.EvalScore
			}
		}
		prevPlan := h.planState.Current
		h.planState.Evaluate(comboAssess, maxThreat)
		if h.planState.Current != prevPlan {
			h.logf("%s PLAN seat=%d %s → %s (combo=%d/%d threat=%.2f)",
				roundTag(gs, seatIdx), seatIdx,
				prevPlan, h.planState.Current,
				h.planState.ComboReady, h.planState.ComboTotal, maxThreat)
		}

		// Apply plan weight multipliers to the evaluator for this turn.
		// PlanDevelop returns all-1.0 multipliers (no-op), so we can
		// always set it — no special-case needed.
		pm := h.planState.PlanWeightMultipliers()
		h.Evaluator.PlanMultiplier = &pm
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

// opponentLikelyHasAnswer returns the composite "loaded and waiting" flag
// for a given seat. True when they tutored, held mana for 2+ turns, and
// are in interactive colors (U/B).
func (h *YggdrasilHat) opponentLikelyHasAnswer(oppSeat int) bool {
	if oppSeat < 0 || oppSeat >= h.seatCount {
		return false
	}
	tutored := oppSeat < len(h.opponentTutored) && h.opponentTutored[oppSeat]
	heldMana := 0
	if oppSeat < len(h.opponentHeldMana) {
		heldMana = h.opponentHeldMana[oppSeat]
	}
	hasInteractiveColors := false
	if oppSeat < len(h.opponentColors) {
		hasInteractiveColors = h.opponentColors[oppSeat]["U"] || h.opponentColors[oppSeat]["B"]
	}
	return tutored && heldMana >= 2 && hasInteractiveColors
}

// OpponentMaxHeldMana returns the largest open-mana value ever observed
// at this opponent's upkeep. Read this when callers need the MAGNITUDE
// of counterspell representation, not just the consecutive-turns streak:
//
//   - 0–1: nothing meaningful seen.
//   - 2:   Spell Pierce / Mana Leak / Force Spike rep — soft counter
//     territory.
//   - 3:   Negate / Counterspell-class / Render Silent — hard counter.
//   - 4+:  Cryptic Command / Force of Negation / Mystic Confluence
//     territory — the opp has consistently held enough to fire
//     big-mana interaction, suggesting blue control or cEDH combo.
//
// Returns 0 for an unknown seat or before any upkeep has fired.
func (h *YggdrasilHat) OpponentMaxHeldMana(oppSeat int) int {
	if oppSeat < 0 || oppSeat >= len(h.opponentMaxHeldMana) {
		return 0
	}
	return h.opponentMaxHeldMana[oppSeat]
}

// ZoneCastGrantSummary describes a single zone-cast permission the hat
// can use right now. Returned by AvailableZoneCastGrants for use in
// hand evaluation, mulligan decisions, and tutor selection.
type ZoneCastGrantSummary struct {
	Card     *gameengine.Card // the card the permission attaches to
	CardName string           // pre-resolved name for hashing/logging
	Zone     string           // "graveyard", "exile", "library"
	Keyword  string           // "flashback", "escape", "free_exile_cast", ...
	ManaCost int              // -1 means use the card's normal cost
	Source   string           // permanent that granted the permission
	Duration string           // "until_end_of_turn" / "" / etc.
	Expiring bool             // grant is "until_end_of_turn" and turn is current
}

// AvailableZoneCastGrants reads gs.ZoneCastGrants and returns every
// active permission whose RequireController matches seatIdx (or is -1).
// The hat consults this when deciding whether casting a card from a
// non-hand zone is currently legal, how much it costs, and whether the
// permission will expire at end of turn (so it should be used now).
func (h *YggdrasilHat) AvailableZoneCastGrants(gs *gameengine.GameState, seatIdx int) []ZoneCastGrantSummary {
	if gs == nil || gs.ZoneCastGrants == nil || len(gs.ZoneCastGrants) == 0 {
		return nil
	}
	out := make([]ZoneCastGrantSummary, 0, len(gs.ZoneCastGrants))
	for card, perm := range gs.ZoneCastGrants {
		if card == nil || perm == nil {
			continue
		}
		if perm.RequireController >= 0 && perm.RequireController != seatIdx {
			continue
		}
		out = append(out, ZoneCastGrantSummary{
			Card:     card,
			CardName: card.DisplayName(),
			Zone:     perm.Zone,
			Keyword:  perm.Keyword,
			ManaCost: perm.ManaCost,
			Source:   perm.SourceName,
			Duration: perm.Duration,
			Expiring: perm.Duration == "until_end_of_turn" && perm.GrantTurn == gs.Turn,
		})
	}
	return out
}

// -- R60 round 5: Bluff detection constants --
//
// bluffLoadedThreshold is the perceived-interaction probability above
// which an opponent is considered "loaded" for the purposes of bluff
// tracking. A turn where they're loaded and don't fire any interactive
// spell adds one to their loaded-silent streak.
//
// bluffMaxStreak caps the streak counter so a long game doesn't drive
// the dampening factor past a sensible floor; with bluffStepPerTurn at
// 0.15 and the 0.4 floor, max=4 hits the floor exactly. Past the cap a
// single interaction cast still resets cleanly to 0.
//
// bluffStepPerTurn is the per-streak-tick reduction in the bluff
// factor returned by `perceivedInteractionThreat`.
//
// bluffFloor is the lowest multiplier the bluff factor can reach.
// Even at maximum bluffing, an opponent might still be holding the
// LAST interaction spell — we never fully discount.
const (
	bluffLoadedThreshold = 0.4
	bluffMaxStreak       = 4
	bluffStepPerTurn     = 0.15
	bluffFloor           = 0.4
)

// isInteractionSpellName returns true if a card-name string matches
// any of the known interactive patterns (counters / targeted removal /
// instant-speed answers). Mirrors the keyword check in
// `opponentHasInteraction`'s reveal scan so the "spell fired" reset
// uses the same definition of "interaction" as the "spell suspected"
// detection. Case-insensitive substring match.
func isInteractionSpellName(name string) bool {
	if name == "" {
		return false
	}
	n := strings.ToLower(name)
	return strings.Contains(n, "counter") ||
		strings.Contains(n, "negate") ||
		strings.Contains(n, "swan song") ||
		strings.Contains(n, "force of") ||
		strings.Contains(n, "pact of negation") ||
		strings.Contains(n, "swords to plowshares") ||
		strings.Contains(n, "path to exile") ||
		strings.Contains(n, "fatal push") ||
		strings.Contains(n, "go for the throat") ||
		strings.Contains(n, "doom blade") ||
		strings.Contains(n, "dispel") ||
		strings.Contains(n, "mana drain") ||
		strings.Contains(n, "mana leak") ||
		strings.Contains(n, "spell pierce")
}

// bluffFactor returns the [bluffFloor, 1.0] dampening multiplier the
// bluff signal applies to an opponent's perceived interaction
// probability. 1.0 = no bluff suspected (recent interaction observed
// or never looked loaded). Lower values = stronger suspicion they're
// running on empty.
func (h *YggdrasilHat) bluffFactor(oppSeat int) float64 {
	if oppSeat < 0 || oppSeat >= len(h.opponentLoadedSilentTurns) {
		return 1.0
	}
	streak := h.opponentLoadedSilentTurns[oppSeat]
	if streak <= 0 {
		return 1.0
	}
	factor := 1.0 - float64(streak)*bluffStepPerTurn
	if factor < bluffFloor {
		factor = bluffFloor
	}
	return factor
}

// perceivedInteractionThreat is the public "use this instead of
// opponentHasInteraction when bluff context matters" surface. It
// returns `opponentHasInteraction * bluffFactor` — the raw signal
// dampened by how long the opponent has been holding mana open with
// nothing to show for it. When deciding whether to walk into a
// suspected counter, callers should prefer this over the raw
// probability so a four-turn "I'm holding Counterspell" routine
// stops paralyzing the hat.
func (h *YggdrasilHat) perceivedInteractionThreat(gs *gameengine.GameState, oppSeat int) float64 {
	raw := h.opponentHasInteraction(gs, oppSeat)
	return raw * h.bluffFactor(oppSeat)
}

// opponentHasInteraction estimates whether an opponent seat is likely
// holding instant-speed interaction based on: open mana, known colors
// (blue/black = counters/removal), hand size, and entropy signals.
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

	// Shannon entropy enrichment: tutored opponents with held mana are
	// far more likely to have a specific answer ready.
	if oppSeat < len(h.opponentTutored) && h.opponentTutored[oppSeat] {
		prob += 0.15
	}
	if oppSeat < len(h.opponentHeldMana) && h.opponentHeldMana[oppSeat] >= 3 {
		prob += 0.10
	}
	// Known cards: if we've seen interaction via reveal, boost confidence.
	if oppSeat < len(h.opponentKnownCards) {
		for name := range h.opponentKnownCards[oppSeat] {
			nameLower := strings.ToLower(name)
			if strings.Contains(nameLower, "counter") ||
				strings.Contains(nameLower, "negate") ||
				strings.Contains(nameLower, "swan song") ||
				strings.Contains(nameLower, "force of") ||
				strings.Contains(nameLower, "pact of negation") ||
				strings.Contains(nameLower, "swords to plowshares") ||
				strings.Contains(nameLower, "path to exile") ||
				strings.Contains(nameLower, "fatal push") {
				prob += 0.25
				break
			}
		}
	}

	if prob > 0.95 {
		prob = 0.95
	}
	return prob
}

// tableInteractionRisk returns the maximum interaction probability across
// all opponents, used to decide whether to walk into countermagic.
//
// R60 round 5 — switched from `opponentHasInteraction` to
// `perceivedInteractionThreat` so the table-wide risk reflects the
// bluff signal. An opponent who has been "holding Counterspell" for
// 4+ turns without firing gets dampened by up to 60%, so the hat
// stops paralyzing on a stale threat signal.
func (h *YggdrasilHat) tableInteractionRisk(gs *gameengine.GameState, seatIdx int) float64 {
	maxRisk := 0.0
	for i := range gs.Seats {
		if i == seatIdx {
			continue
		}
		risk := h.perceivedInteractionThreat(gs, i)
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

// OpponentCounterspellsCast returns the running count of
// counterspell-shaped casts observed from oppSeat this game. Used by
// callers to estimate "have they run out of counter mana / counters in
// hand?" — typical EDH lists run 8-12 counters; a count of 4-5 means
// the remaining-deck counter density is meaningfully diminished.
// Returns 0 for an out-of-range seat or pre-game state.
func (h *YggdrasilHat) OpponentCounterspellsCast(oppSeat int) int {
	if oppSeat < 0 || oppSeat >= len(h.opponentCounterspellsCast) {
		return 0
	}
	return h.opponentCounterspellsCast[oppSeat]
}

// OpponentMassRemovalCast returns the running count of board-wipe
// casts observed from oppSeat. Typical 100-card decks run 2-4
// wraths; a count of 3 means the remaining sweep budget is near zero
// and an overextended board is less likely to get punished.
func (h *YggdrasilHat) OpponentMassRemovalCast(oppSeat int) int {
	if oppSeat < 0 || oppSeat >= len(h.opponentMassRemovalCast) {
		return 0
	}
	return h.opponentMassRemovalCast[oppSeat]
}

// OpponentTutorsCast returns the running count of tutor activations
// observed from oppSeat. Magnitude companion to the existing
// opponentTutored bool — multi-tutor reads strongly suggest the
// opponent is assembling a combo (cEDH-style) and the next windows
// are the critical disruption points.
func (h *YggdrasilHat) OpponentTutorsCast(oppSeat int) int {
	if oppSeat < 0 || oppSeat >= len(h.opponentTutorsCast) {
		return 0
	}
	return h.opponentTutorsCast[oppSeat]
}

// PredictedTutorTargetClass returns a coarse classification of what
// oppSeat most likely fetched with their last tutor, based on
// perceivedArchetype. Returns one of:
//
//	"combo_piece"  — opp archetype reads as combo (likely fetched a
//	                 missing piece)
//	"interaction"  — opp archetype reads as control (likely fetched a
//	                 counter / targeted removal)
//	"haste_threat" — opp archetype reads as aggro (likely fetched a
//	                 finisher or board-pressure body)
//	"value"        — opp archetype reads as midrange (likely fetched
//	                 a flexible engine or ramp piece)
//	""             — opp hasn't tutored yet, or archetype unknown.
//
// The signal lets callers (combo timing, response priority, blocker
// allocation) adjust their next decision based on what the opp is
// projecting onto their hand. Returns empty string if no tutor
// observed or perceived archetype is unknown.
func (h *YggdrasilHat) PredictedTutorTargetClass(oppSeat int) string {
	if oppSeat < 0 || oppSeat >= len(h.opponentTutorsCast) {
		return ""
	}
	if h.opponentTutorsCast[oppSeat] == 0 {
		return ""
	}
	if oppSeat >= len(h.perceivedArchetype) {
		return ""
	}
	switch h.perceivedArchetype[oppSeat] {
	case "combo":
		return "combo_piece"
	case "control":
		return "interaction"
	case "aggro":
		return "haste_threat"
	case "midrange":
		return "value"
	}
	return ""
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
						bp := boardPower(gs, s)
						if bp > maxOppBoard {
							maxOppBoard = bp
						}
					}
				}
				if maxOppBoard > boardPower(gs, seat) {
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

// opponentLikelyHasWrath returns a probability [0, 1] that an opponent is
// holding a sorcery-speed board wipe. Factors: mana availability (4+),
// hand size, W/B/R color signals, inferred archetype, cast cadence,
// known cards in hand, board-state asymmetry, and prior wrath history.
func (h *YggdrasilHat) opponentLikelyHasWrath(gs *gameengine.GameState, oppSeat int) float64 {
	return h.wrathProbability(gs, oppSeat)
}

func (h *YggdrasilHat) wrathProbability(gs *gameengine.GameState, oppSeat int) float64 {
	if gs == nil || oppSeat < 0 || oppSeat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[oppSeat]
	if s == nil || s.Lost || s.LeftGame {
		return 0
	}

	prob := 0.0

	// Base: hand size. More cards = more likely to hold a wrath.
	handSize := len(s.Hand)
	if handSize == 0 {
		return 0
	}
	prob += float64(handSize) * 0.04 // 7 cards = 0.28 base

	// Mana availability: need 4+ to cast most wraths.
	avail := gameengine.AvailableManaEstimate(gs, s)
	if avail < 4 {
		prob *= 0.3 // can't cast it = very unlikely
	} else if avail >= 5 {
		prob += 0.08 // comfortably castable
	}

	// Color signals: white and black have the most wraths.
	if oppSeat < len(h.opponentColors) {
		if h.opponentColors[oppSeat]["W"] {
			prob += 0.15 // Wrath of God, Day of Judgment, Farewell
		}
		if h.opponentColors[oppSeat]["B"] {
			prob += 0.10 // Damnation, Toxic Deluge
		}
		if h.opponentColors[oppSeat]["R"] {
			prob += 0.05 // Blasphemous Act, Chain Reaction
		}
	}

	// Archetype: control and stax decks run more wraths.
	arch := h.inferOpponentArchetype(gs, oppSeat)
	switch arch {
	case ArchetypeControl:
		prob += 0.15
	case ArchetypeStax:
		prob += 0.10
	case ArchetypeMidrange:
		prob += 0.05
	}

	// Cast cadence: opponent holding cards while having mana = saving
	// something. Low cast rate with full hand is suspicious.
	if s.SpellsCastThisTurn == 0 && handSize >= 4 && avail >= 4 {
		prob += 0.10
	}
	if s.SpellsCastLastTurn == 0 && s.SpellsCastThisTurn == 0 && handSize >= 3 {
		prob += 0.05 // two turns of nothing with cards = holding
	}

	// Known cards in hand: if we've seen a wrath via reveal effects,
	// that's near-certain information (only reduced by possible discard).
	if oppSeat < len(h.opponentKnownCards) {
		for name := range h.opponentKnownCards[oppSeat] {
			nl := strings.ToLower(name)
			if strings.Contains(nl, "wrath") || strings.Contains(nl, "damnation") ||
				strings.Contains(nl, "day of judgment") || strings.Contains(nl, "farewell") ||
				strings.Contains(nl, "blasphemous act") || strings.Contains(nl, "toxic deluge") ||
				strings.Contains(nl, "cyclonic rift") || strings.Contains(nl, "supreme verdict") ||
				strings.Contains(nl, "vanquish the horde") || strings.Contains(nl, "chain reaction") ||
				strings.Contains(nl, "kindred dominance") || strings.Contains(nl, "merciless eviction") ||
				strings.Contains(nl, "austere command") || strings.Contains(nl, "terminus") ||
				strings.Contains(nl, "living death") || strings.Contains(nl, "decree of pain") {
				prob += 0.45 // known wrath in hand — near certain
				break
			}
		}
	}

	// Prior wrath history: check if we've seen board wipes from this
	// opponent via the event stream (cardsSeen set).
	if oppSeat < len(h.cardsSeen) {
		for name := range h.cardsSeen[oppSeat] {
			nl := strings.ToLower(name)
			if strings.Contains(nl, "wrath") || strings.Contains(nl, "damnation") ||
				strings.Contains(nl, "day of judgment") || strings.Contains(nl, "farewell") ||
				strings.Contains(nl, "blasphemous act") || strings.Contains(nl, "toxic deluge") ||
				strings.Contains(nl, "cyclonic rift") || strings.Contains(nl, "supreme verdict") ||
				strings.Contains(nl, "vanquish the horde") || strings.Contains(nl, "chain reaction") ||
				strings.Contains(nl, "kindred dominance") || strings.Contains(nl, "merciless eviction") {
				prob += 0.20 // confirmed wrath deck
				break
			}
		}
	}

	// Board-state asymmetry: opponents with few creatures but a full hand
	// and plenty of mana are more likely holding wraths — they're not
	// developing board presence because they plan to reset it.
	oppCreatures := 0
	for _, p := range s.Battlefield {
		if p != nil && p.IsCreature() {
			oppCreatures++
		}
	}
	if oppCreatures <= 1 && handSize >= 4 && avail >= 4 {
		prob += 0.10
	}

	if prob > 0.95 {
		prob = 0.95
	}
	return prob
}

// -- Rollout simulation (reuses the same pattern as MCTSHat) --

func (h *YggdrasilHat) simulateRollout(gs *gameengine.GameState, seatIdx int, actionFn func(clone *gameengine.GameState)) float64 {
	// Per-hat seed counter — see rollout.go for the rationale on dropping
	// the package-global `rolloutSeedCounter`.
	h.rolloutSeed++
	rng := rand.New(rand.NewSource(int64(gs.Turn)*1000 + int64(seatIdx)*100 + h.rolloutSeed))
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

	depth := rolloutDepth
	if h.Strategy != nil {
		depth = rolloutDepthFor(h.Strategy.Archetype)
	}
	for i := 0; i < depth; i++ {
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
	bit  uint8
}{
	{"plains", "W", colorBitW},
	{"island", "U", colorBitU},
	{"swamp", "B", colorBitB},
	{"mountain", "R", colorBitR},
	{"forest", "G", colorBitG},
}

const (
	colorBitW uint8 = 1 << iota
	colorBitU
	colorBitB
	colorBitR
	colorBitG
	colorBitAll = colorBitW | colorBitU | colorBitB | colorBitR | colorBitG
)

func colorSymBit(sym string) uint8 {
	switch sym {
	case "W":
		return colorBitW
	case "U":
		return colorBitU
	case "B":
		return colorBitB
	case "R":
		return colorBitR
	case "G":
		return colorBitG
	}
	return 0
}

// landProducesColorsMask returns a bitmask of the colors this card can
// produce as mana. Result is cached on the Card; subsequent calls are
// O(1). Hot path: called per land per evaluation, and historically
// allocated a map[string]bool each time (top alloc-space hotspot per
// perf_round2 audit).
func landProducesColorsMask(c *gameengine.Card) uint8 {
	if c == nil {
		return 0
	}
	if c.ProducedColorsReady {
		return c.ProducedColorsMask
	}
	var mask uint8
	ot := gameengine.OracleTextLower(c)
	tl := gameengine.TypeLineLower(c)
	for _, col := range colorLandTypes {
		// Pre-built "add {x}" needle avoids per-call allocation.
		needle := colorAddNeedles[col.bit]
		if strings.Contains(tl, col.name) || strings.Contains(ot, needle) {
			mask |= col.bit
		}
	}
	if strings.Contains(ot, "any color") {
		mask = colorBitAll
	}
	c.ProducedColorsMask = mask
	c.ProducedColorsReady = true
	return mask
}

// colorAddNeedles maps a color bit to the lowercased "add {x}" oracle
// needle used in landProducesColorsMask. Module-level so we don't
// rebuild it on each call.
var colorAddNeedles = map[uint8]string{
	colorBitW: "add {w}",
	colorBitU: "add {u}",
	colorBitB: "add {b}",
	colorBitR: "add {r}",
	colorBitG: "add {g}",
}

func fieldColorSources(seat *gameengine.Seat, color string) int {
	bit := colorSymBit(color)
	if bit == 0 {
		return 0
	}
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
		if landProducesColorsMask(p.Card)&bit != 0 {
			count++
		}
	}
	return count
}

// untappedFieldColorSources mirrors fieldColorSources but only counts
// UNTAPPED lands producing the color. Used by the in-hand color-screw
// detector (handColorScrewPenalty) — a tapped Island can't help cast
// the {UU} spell in your hand THIS turn, so the binding constraint on
// playability is untapped-only count, not total count.
func untappedFieldColorSources(seat *gameengine.Seat, color string) int {
	bit := colorSymBit(color)
	if bit == 0 || seat == nil {
		return 0
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.Tapped {
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
		if landProducesColorsMask(p.Card)&bit != 0 {
			count++
		}
	}
	return count
}
