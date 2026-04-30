package hat

// StrategyProfile is the hat's view of Freya deck analysis. It provides
// the PokerHat with deck-specific intelligence: known combos, tutor
// priorities, value engine keys, and archetype classification.
//
// Populated from Freya's JSON output by the tournament runner via
// LoadStrategyFromFreya. When nil, PokerHat falls back to its hardcoded
// knownCombos and generic heuristics (backward compatible).
type StrategyProfile struct {
	// Archetype is Freya's primary archetype classification:
	// "Aggro", "Midrange", "Combo", "Control", "Ramp", etc.
	Archetype string

	// ComboPieces are the deck's known win lines from Freya analysis.
	// Ordered by priority (infinite > determined > finisher).
	ComboPieces []ComboPlan

	// TutorTargets is a prioritized list of card names the hat should
	// prefer when selecting tutor targets or evaluating hand quality.
	// Derived from combo pieces + value engine keys.
	TutorTargets []string

	// ValueEngineKeys are cards critical to the deck's value engines
	// (e.g., Smothering Tithe, Rhystic Study, Phyrexian Arena). These
	// cards should be protected and prioritized for casting.
	ValueEngineKeys []string

	// GameplanSummary is Freya's one-line gameplan description.
	GameplanSummary string

	// Bracket is the power bracket (1-5) from Freya's classification.
	Bracket int

	// Weights are the MCTS evaluator weights for this deck. Computed by
	// Freya from deck analysis or defaulted per archetype. When nil, the
	// evaluator uses DefaultWeightsForArchetype(Archetype).
	Weights *EvalWeights

	// CardRoles maps card names to their primary Freya role tag:
	// "Ramp", "Draw", "Removal", "BoardWipe", "Counterspell", "Tutor",
	// "Threat", "Combo", "Protection", "Stax", "Utility", "Land".
	CardRoles map[string]string

	// FinisherCards are card names that Freya classified as game finishers.
	// Includes mass pump, extra combat, damage doublers, X-burn, infect, etc.
	FinisherCards []string

	// ColorDemand tracks how many pips of each color the deck needs by CMC bracket.
	// Key is color (W/U/B/R/G), value is total pip count across the deck.
	ColorDemand map[string]int

	// StarCards are the deck's highest-impact cards from Freya quality tiers.
	// These get tutor priority and mulligan keep weight.
	StarCards []string

	// CuttableCards are low-impact cards Freya identified as potential cuts.
	// Deprioritized for tutor targets, discard last.
	CuttableCards []string

	// CommanderThemes are synergy themes extracted from the commander's oracle text.
	CommanderThemes []string

	// CommanderSynergy is the ratio (0-1) of non-land cards that synergize
	// with commander themes. High synergy = commander is more critical.
	CommanderSynergy float64

	// VulnerableTo lists specific hosers the deck fears (e.g., "Rest in Peace",
	// "Blood Moon"). Informs play-around decisions and threat assessment.
	VulnerableTo []string

	// InteractionAvgCMC is the average CMC of the deck's interaction suite.
	// Lower = faster interaction = can afford to hold up mana more often.
	InteractionAvgCMC float64

	// CheapInteraction is the count of interaction spells at CMC 0-2.
	CheapInteraction int

	// ManaBaseGrade is the Freya-assigned mana base quality (A/B/C/D/F).
	// Weak grades = conservative land sequencing.
	ManaBaseGrade string

	// KeepableHandPct is the Monte Carlo simulated % of keepable opening hands.
	// Low % = more aggressive mulliganing.
	KeepableHandPct float64

	// PowerPercentile is this deck's estimated power level within its archetype (0-100).
	// Scales hat budget: stronger decks warrant deeper search.
	PowerPercentile int

	// MetaMatchups maps opponent archetype → matchup rating ("favored"/"neutral"/"unfavored").
	// Used by 3rd Eye to adjust targeting when opponent archetype is inferred.
	MetaMatchups map[string]string

	// EmergentSynergies are card pairs discovered by Huginn from game
	// observations. Tier 2+ interactions provide soft eval weight bumps
	// to combo proximity — NOT treated as hard combo plans.
	EmergentSynergies []EmergentSynergy

	// Weakness is an optional cross-game weakness profile derived from
	// Heimdall analytics. When set, the hat adjusts play to compensate.
	Weakness *WeaknessProfile

	// ELOGamesPlayed is how many games this deck's ELO is based on.
	// Used to scale hat budget: more games = more confidence = deeper search.
	ELOGamesPlayed int
}

// WeaknessProfile captures patterns from game analytics that indicate
// recurring vulnerabilities. Derived by aggregating Heimdall data.
type WeaknessProfile struct {
	VulnerableToWipes   float64 // 0-1: how often losses follow board wipes
	VulnerableToCounter float64 // 0-1: how often key spells get countered in losses
	SlowToClose         float64 // 0-1: how often the deck stalls with a winning position
	ManaScrew           float64 // 0-1: how often losses correlate with low land count
	OverExtends         float64 // 0-1: how often the deck dumps hand then loses to wipe
}

// EmergentSynergy is a card pair interaction discovered by Huginn.
// Softer than ComboPlan — provides a small eval weight bump, not a
// hard combo sequence.
type EmergentSynergy struct {
	Cards         []string
	EffectPattern string
	Tier          int
	AvgImpact     float64
}

// ComboPlan describes a single win line or combo the deck can assemble.
type ComboPlan struct {
	// Pieces are the card names required for this combo.
	Pieces []string

	// Type is the combo classification: "infinite", "determined", "finisher".
	Type string

	// CastOrder is the preferred casting sequence. The first element
	// should be cast/resolved first. If empty, falls back to Pieces order.
	CastOrder []string
}

// comboPlansToKnownCombos converts ComboPlan entries into comboPair format
// for compatibility with the existing combo detection machinery. Only
// two-card combos are converted; larger combos are tracked separately.
func comboPlansToKnownCombos(plans []ComboPlan) []comboPair {
	var out []comboPair
	for _, cp := range plans {
		if len(cp.Pieces) != 2 {
			continue
		}
		first := cp.Pieces[0]
		if len(cp.CastOrder) > 0 {
			first = cp.CastOrder[0]
		}
		out = append(out, comboPair{
			piece1:    cp.Pieces[0],
			piece2:    cp.Pieces[1],
			castFirst: first,
		})
	}
	return out
}

// allComboPieceNames returns a deduplicated set of all card names
// appearing in any ComboPlan.
func allComboPieceNames(plans []ComboPlan) map[string]bool {
	out := make(map[string]bool)
	for _, cp := range plans {
		for _, p := range cp.Pieces {
			out[p] = true
		}
	}
	return out
}
