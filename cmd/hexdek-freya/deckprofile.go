package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// isWizardsPrecon reports whether deckPath points at a file under
// data/decks/wizards/ — the corpus of unedited WotC precons. Matched by
// substring on the cleaned path so callers can pass either absolute or
// relative paths; the wizards/ segment is unambiguous because no other
// directory in the data tree shares that name.
func isWizardsPrecon(deckPath string) bool {
	if deckPath == "" {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(deckPath))
	return strings.Contains(clean, "/data/decks/wizards/") ||
		strings.HasPrefix(clean, "data/decks/wizards/") ||
		strings.Contains(clean, "/decks/wizards/")
}

type DeckProfile struct {
	DeckName      string
	Commander     string
	ColorIdentity []string
	CardCount     int

	AvgCMC         float64
	LandCount      int
	RecommendedLands int
	LandVerdict    string
	RampCount      int
	DrawCount      int

	// CMC-distribution health check counts (r60). OneDropCount is the
	// number of nonland cards at CMC 1; TwoDropCount is at CMC 2. The
	// distribution warnings ("too many 1-drops" / "too few 2-drops")
	// are surfaced via CurveWarnings on the report and Weaknesses on
	// the profile; these counters are exposed for downstream renderers
	// that want the underlying metric.
	OneDropCount   int
	TwoDropCount   int

	RoleCounts map[RoleTag]int
	TopRoles   []RoleCount

	PrimaryArchetype    string
	SecondaryArchetype  string
	ArchetypeConfidence float64
	// Bracket is the rubber-stamp / declared bracket: what the deck
	// identifies as. Defaults to MeasuredBracket; overridden to 2 for
	// any deck under data/decks/wizards/ (unedited WotC precons whose
	// product-stated intent is B2 regardless of how the bracket signals
	// score). User-editable downstream.
	Bracket             int
	BracketLabel        string
	// MeasuredBracket is Freya's signal-computed bracket — the canonical
	// felt-power measurement, surfaced in the UI as "Estimated Bracket".
	// Diverges from Bracket when a declared override is applied (e.g.
	// precon claims B2, measures B3 — tells you Atraxa is hotter than
	// the box says).
	MeasuredBracket      int
	MeasuredBracketLabel string
	GameChangerCount    int
	GameChangerCards    []string
	Intent              string
	// BracketRationale documents how the bracket call was derived —
	// which density / card-list signals contributed, raw score, and
	// any ceiling/floor/gate adjustments. Surfaced to JSON + report so
	// builders can see why the deck landed where it did.
	BracketRationale *BracketRationale

	PrimaryWinLine    string
	WinLineCount      int
	BackupCount       int
	HasTutorAccess    bool
	SinglePointCount  int
	// WeightedWinLineScore sums per-line WinLine.Weight values so the
	// power-percentile scorer can reward win-line QUALITY (2-card true
	// infinite scores high, 4-card grindy lines score modestly) rather
	// than just counting raw line presence.
	WeightedWinLineScore int

	Strengths       []string
	Weaknesses      []string
	GameplanSummary string

	CommanderSynergy    float64  // 0.0-1.0 ratio of cards synergizing with commander
	CommanderThemes     []string // detected themes from commander oracle text
	SynergyCount        int      // cards in 99 that match commander themes

	InteractionQuality  float64 // avg CMC of interaction spells (lower = faster)
	CheapInteraction    int     // interaction at CMC 0-2
	ExpensiveInteraction int    // interaction at CMC 4+
	// AdjustedInteractionQuality is InteractionQuality with a downside
	// penalty layered in: each removal piece that grants the opponent a
	// meaningful resource (Path ramp, Generous Gift 3/3, Oblation card
	// advantage) costs +0.5 effective CMC. Path-to-Exile feels more like
	// CMC 1.5 than CMC 1 because the basic-land search is a real cost in
	// a non-green-opponent meta. See computeInteractionQuality and
	// interactionDownsides for the curated table.
	AdjustedInteractionQuality float64
	// InteractionDownsides is the per-card breakdown of which interaction
	// spells carry an opponent-advantage downside, and what the downside
	// is. Surfaced to the report so builders see WHICH pieces to consider
	// swapping for cleaner replacements (StP, Anguished Unmaking,
	// Counterspell). Sorted alphabetically for stable rendering.
	InteractionDownsides []InteractionDownside
	ProtectedKeyPieces  int     // key pieces with built-in protection
	UnprotectedKeyPieces int    // key pieces without built-in protection

	// VulnerableComboPieces lists each RoleCombo piece in the deck that
	// lacks built-in protection (hexproof / shroud / ward / indestructible
	// / can't-be-countered / phase-out). Combo piece losses are catastrophic
	// — a single Path / Pongify / Imprisoned in the Moon resets the kill —
	// where threat losses are recoverable. Surfaced as a per-card callout
	// (not just the aggregate Unprotected count) so builders see which
	// specific cards to back up with Lightning Greaves / Heroic Intervention
	// / Veil of Summer. Sorted CMC-descending (higher CMC = bigger tempo
	// loss to recast); capped at 8 entries. See computeProtectionDensity.
	VulnerableComboPieces []VulnerableComboPiece

	// Mana base grading
	ManaBaseGrade    string // A/B/C/D/F
	ManaBaseNotes    []string
	TaplandCount     int
	FetchCount       int
	UtilityLandCount int

	// Deck cost tier — "Budget" (<$100), "Mid" ($100-$500),
	// "High-end" (>$500). Empty when fewer than 80% of the deck's
	// cards have resolvable USD prices (insufficient data for a
	// meaningful classification — would mislead more than inform).
	// EstimatedTotalUSD is the sum of resolved card prices; the
	// real total may be higher when unpriced cards include
	// reserved-list / out-of-print premiums. See computeDeckCostTier.
	DeckCostTier      string
	EstimatedTotalUSD float64
	PricedCardCount   int
	UnpricedCardCount int
	DeckCostNote      string // human-readable summary or coverage warning

	// Threat assessment
	VulnerableTo []string // specific hosers this deck fears

	// Opening hand simulation
	KeepableHandPct  float64 // % of hands with 2-5 lands + action
	AvgTurnToFourMana float64

	// Commander-centric hand evaluation: for decks where the gameplan IS the
	// commander (Voltron, high commander-synergy engines), the standard
	// keepable heuristic over-penalizes hands that lack standalone threats.
	// KeepableHandPctAdjusted relaxes the action requirement when the
	// commander itself supplies the engine.
	IsCommanderCentric       bool
	CommanderCentricReason   string
	KeepableHandPctAdjusted  float64
	AvgTurnToCommander       float64
	CommanderCMC             int

	// Vancouver free-mulligan variants — Commander's house rule that the
	// first mulligan is free (re-draw 7 with no card penalty). Counts a
	// trial as keepable if EITHER the initial hand OR the free-mulligan
	// hand passes the keepability check. Always computed alongside the
	// no-mulligan baseline so consumers can compare; the no-mulligan
	// variant remains canonical for power-percentile scoring (matches
	// pre-r60 calibration) and the mulligan variant surfaces the real-
	// table accept rate. Invariant: FreeMull >= no-mulligan baseline
	// (1 - (1-p)^2 >= p for all p in [0,1], with equality only at p=1).
	KeepableHandPctFreeMull         float64
	KeepableHandPctAdjustedFreeMull float64

	// Synergy clusters
	SynergyClusters []SynergyCluster

	// Alt-build suggestions — when ≥2 clusters are above the pivot
	// threshold, surface each non-primary as a "you could refocus
	// around X" candidate so the Decks screen can show "this deck is
	// trying to do two things; pick one." See computeAltBuildSuggestions.
	AltBuildSuggestions []AltBuildSuggestion

	// Meta positioning
	MetaMatchups []MetaMatchup

	// "Favored against" reverse-lookup — archetypes this deck reliably
	// beats, combined from both forward (X's own favored matchups) and
	// reverse (other archetypes that list themselves unfavored vs X)
	// directions of the matchup DB. See MetaStrongAgainst for details.
	// Surfaced to the Decks screen as the "good against" summary.
	StrongAgainst []MetaAdvantage

	// Card quality tiers — star = clear keepers tied to wincon / value
	// chain; solid = okay-but-replaceable, scoring above the cut floor
	// but below the star threshold (worth flagging for builders shopping
	// upgrades); cuttable = bottom of the deck on the synergy evaluator.
	CuttableCards []CardQuality
	SolidCards    []CardQuality
	StarCards     []CardQuality

	// CardPowerLevels is the full 0-100 power rating for every non-land
	// card in the deck, sorted high → low. See CardPowerLevel for the
	// three-component breakdown (archetype fit + CMC efficiency + synergy
	// contribution). Each tier entry above (StarCards / SolidCards /
	// CuttableCards) echoes the same Power on its CardQuality.Power field.
	CardPowerLevels []CardPowerLevel

	// PowerTierCounts is the per-tier (S/A/B/C/D) card distribution
	// summary derived from CardPowerLevels — surfaces "this deck has
	// 3 S, 5 A, 12 B, 28 C, 50 D" so builders can pace upgrade
	// purchases. Map keys are always in PowerTierOrder; absent tiers
	// are present with value 0 for stable rendering.
	PowerTierCounts map[string]int

	// PetCards are low-tier (C/D) creatures whose role tags don't match
	// the deck's primary-archetype fingerprint — the deckbuilder kept
	// them despite the suboptimal fit, which strongly signals a personal-
	// taste / flavor pick rather than a card they'd cut on review. The
	// report surfaces these as "keep if you love it" so the upgrade
	// coaching respects personal taste instead of just hammering "cut
	// this." See computePetCards for the exact detection signals.
	PetCards []PetCard

	// Color weight suggestions
	LandSwapSuggestions []string

	// CurveArchetypeWarnings flag mismatches between the deck's
	// PrimaryArchetype and its average CMC. Each archetype has an
	// expected curve band (Combo decks want <2.7 to assemble fast,
	// Control wants >3.0 to support late-game inevitability, etc).
	// When AvgCMC sits more than 0.2 outside the archetype's band,
	// we emit a warning so builders see "you classified as Combo but
	// your curve plays like Midrange — consider cutting top-end
	// cards" without having to derive the mismatch themselves. Empty
	// when the curve fits the archetype, or when the archetype has
	// no firm curve expectation (Midrange / generic fallbacks).
	CurveArchetypeWarnings []string

	// Deck personality
	PersonalityBlurb string

	// Deck coaching — 3-5 prioritized actionable suggestions synthesized
	// across mana base, interaction, ramp/draw, win-line redundancy,
	// protection density, cuts, and commander-synergy signals. Thresholds
	// flex with archetype + bracket goal so a B2 casual deck isn't
	// lectured about tutor density. See computeCoachingTips.
	CoachingTips []CoachingTip

	// Power ranking
	PowerPercentile int    // 0-100 estimated percentile within archetype
	PowerFactors    []string
}

type RoleCount struct {
	Role  RoleTag
	Count int
}

type SynergyCluster struct {
	Name        string
	Cards       []string // capped at 8 for display
	Theme       string
	Score       int // number of pairwise synergies within the cluster
	MemberCount int // full deduped count (uncapped) — used by alt-build threshold
	// AllMembers is the full deduped member list (uncapped). Cards is the
	// display-capped subset; AllMembers feeds the structured export
	// (cluster_export.go) so downstream deck-builder integrations get
	// the complete membership for each theme cluster, not just the
	// 8-card preview.
	AllMembers []string
	// HighDensity flags clusters with MemberCount >= HighDensityClusterFloor
	// (5). At this size, a theme stops being "the deck happens to enable
	// X" and becomes "X is a real subsystem of the gameplan" — surfaced
	// in the report with a ★ prefix and in JSON as high_density so
	// downstream consumers can highlight the theme without recomputing
	// the threshold.
	HighDensity bool
}

// HighDensityClusterFloor is the MemberCount at which a synergy cluster
// is flagged as a deliberate subsystem of the deck rather than incidental
// overlap. Tuned to 5 because 4 (the cluster floor itself) is the
// minimum-viable theme presence and the discriminator should sit one
// step above that.
const HighDensityClusterFloor = 5

// MinimumClusterMembers is the defensive floor below which a "cluster"
// is just one or two coincidentally-overlapping cards and should not
// surface as a theme. The pair-scoring math also degrades to noise at
// that size. Both computeSynergyClusters paths (themed clusters and the
// separate land_value cluster) already enforce a stricter floor of 4 —
// this constant pins the absolute lower bound in case either path's
// floor is ever relaxed.
const MinimumClusterMembers = 2

// AltBuildSuggestion is a "you could re-focus the deck around X" hint
// surfaced when the deck has multiple synergy clusters above the
// pivotable-size threshold — the deckbuilder is splitting slot
// priority across two engines and could commit to one. See
// computeAltBuildSuggestions.
type AltBuildSuggestion struct {
	Cluster     string // theme key (e.g. "tokens", "recursion")
	ClusterName string // display name ("Token Engine")
	MemberCount int    // cards already aligned with the theme
	Score       int    // pair-weighted score from the cluster
	Pivot       string // "You could refocus around X — ..."
	Trade       string // "Currently splits with Y — picking one frees N slots"
}

type MetaMatchup struct {
	Archetype string
	Rating    string // "favored", "neutral", "unfavored"
	Reason    string
	// Strength is the magnitude qualifier on a directional rating:
	// "strong" (mechanical hard-lock), "moderate" (default favored/unfavored
	// edge), "slight" (draw/curve-dependent lean). Empty when Rating is
	// "neutral" — a neutral matchup has no magnitude to report.
	// Surfaces in JSON as "strength" so Decks-screen consumers can
	// down-weight "slight" entries vs amplifying "strong" ones.
	Strength string `json:"strength,omitempty"`
}

// MetaAdvantage is the per-entry shape of the "favored against" list
// produced by MetaStrongAgainst. Reason is the headline why-line
// (forward perspective if available — "X beats Y because Z"); when
// Source is "both", OpponentReason carries the corroborating reverse
// perspective ("Y can't answer X because W").
type MetaAdvantage struct {
	Archetype      string
	Reason         string
	OpponentReason string // populated only when Source == "both"
	Source         string // "forward" | "reverse" | "both"
}

// CoachingTip is one prioritized, actionable suggestion for improving
// the deck. Coach surfaces ≤5 sorted by Priority (descending).
//
//   - Category groups tips for the Decks-screen filter rail
//     ("cut" | "add" | "manabase" | "ratio" | "consistency").
//   - Priority is the impact score (1-10). 10 = deck won't function
//     without this fix (severe land shortage); 5 = stylistic
//     improvement. Coach uses this for the sort + truncation pass.
//   - Title is a one-line headline rendered as the tip bullet.
//   - Detail is 1-3 sentences naming the specific signal that
//     triggered the tip (counts, percentages, named themes).
//   - Action is the concrete next step — what to actually do, with
//     named candidate cards where applicable.
//   - Tags carry archetype/bracket affinity for downstream filtering.
type CoachingTip struct {
	Category string
	Priority int
	Title    string
	Detail   string
	Action   string
	Tags     []string
}

type CardQuality struct {
	Name             string
	Tier             string // "star" | "solid" | "cuttable" (synergy-tier classification)
	Reason           string
	Power            int    // 0-100 power level — see CardPowerLevel for components
	PowerTier        string // "S" | "A" | "B" | "C" | "D" — see PowerTierFor
	PowerExplanation string // human-readable "S — wincon piece + 4-role at CMC 1 + Combo-archetype fit"
	// Rationale fields (populated for cuttable tier).
	Detected  string   // what stat/pattern triggered the recommendation
	WhyCut    string   // why cutting it is recommended
	Effect    string   // resulting effect on the deck if cut
	Suggested []string // suggested swap candidates
}

// CardPowerLevel is the 0-100 per-card power rating produced by
// computeCardPower. The total is the sum of three explicit components,
// each capped at its own max so the final Power is clamped to [0, 100].
//
//	ArchetypeFit         0-40 — how well the card's roles align with
//	                            the deck's primary-archetype fingerprint
//	CMCEfficiency        0-20 — curve placement (cheap multi-role = peak)
//	SynergyContribution  0-40 — win-line / value-chain / combo / cluster
//	                            participation
//
// Surfaced through DeckProfile.CardPowerLevels (sorted high → low) plus
// echoed onto each CardQuality entry's Power field so star / solid /
// cuttable lists show the rating inline.
type CardPowerLevel struct {
	Name                string
	CMC                 int
	Roles               []string
	Power               int
	PowerTier           string // "S" | "A" | "B" | "C" | "D" — see PowerTierFor
	Explanation         string // human-readable why-line — see buildPowerExplanation
	ArchetypeFit        int
	CMCEfficiency       int
	SynergyContribution int
}

// PetCard is a low-tier creature the deckbuilder kept despite its
// off-archetype fit — almost certainly a personal-taste / flavor pick.
// Surfaced as a "keep if you love it" hint so the report doesn't
// hammer "cut this" against cards the builder consciously included
// for flavor. See computePetCards for the detection criteria.
type PetCard struct {
	Name      string
	CMC       int
	Roles     []string
	Power     int
	PowerTier string
	Reason    string // "off-archetype legendary creature — signature flavor pick"
}

// PowerTierFor maps a 0-100 power score to a letter grade for "deck
// buy-it pacing" — S-tier first, A second, etc. Absolute bands
// (intentionally not percentile-based) so a casual deck with no S
// cards reads as "casual deck with no high-impact cards" instead of
// promoting its top filler. Boundaries are inclusive on the high side:
//
//	S = 70-100  must-keep wincons / load-bearing engines
//	A = 55-69   strong supports, the deck wants these specific cards
//	B = 38-54   solid utility, replaceable but not flex
//	C = 25-37   situational / role-only filler
//	D = 0-24    bottom of the deck, swap candidates
//
// Thresholds recalibrated from the original 75/60/40/25 split based on
// the cross-deck distribution from ComputePowerTierAggregate. The
// original 300-deck baseline showed 3.3% S / 11.4% A — too thin on
// both elite tiers, with the [70-74] band of strong-but-not-elite
// cards stuck in A and the [55-59] band of solid supports stuck in B.
// The retuned 70/55/38/25 thresholds shift the distribution toward
// the standard tier shape (target ~7% S / 18% A / 35% B / 30% C /
// 10% D — roughly normal peaked at B with thin tails). Lyon corpus
// (8 decks / 504 cards) under the new thresholds: 6.9% S / 19.4% A /
// 37.9% B / 28.6% C / 8.9% D. The C/D boundary at 25 is unchanged —
// D was already well-sized as "obvious cuts".
//
// See computeCardPower for how Power is computed.
func PowerTierFor(power int) string {
	switch {
	case power >= 70:
		return "S"
	case power >= 55:
		return "A"
	case power >= 38:
		return "B"
	case power >= 25:
		return "C"
	default:
		return "D"
	}
}

// PowerTierOrder is the deterministic ordering for tier iteration
// (high → low). Use this anywhere you print or count tiers so output
// always reads S/A/B/C/D, never C/A/S/B/D from map iteration order.
var PowerTierOrder = []string{"S", "A", "B", "C", "D"}

func BuildDeckProfile(report *FreyaReport, oracle *oracleDB) *DeckProfile {
	dp := &DeckProfile{
		DeckName:  report.DeckName,
		Commander: report.Commander,
		CardCount: report.TotalCards,
		AvgCMC:    report.AvgCMC,
		LandCount: report.LandCount,
	}

	if oracle != nil && report.Commander != "" {
		entry := oracle.lookup(report.Commander)
		if entry != nil {
			dp.ColorIdentity = entry.ColorIdentity
		}
	}

	if report.Stats != nil {
		dp.RecommendedLands = report.Stats.RecommendedLands
		dp.LandVerdict = report.Stats.LandVerdict
		dp.RampCount = report.Stats.RampCount
		dp.DrawCount = report.Stats.DrawSourceCount
	}

	if report.Roles != nil {
		dp.RoleCounts = report.Roles.RoleCounts
		dp.TopRoles = topNRoles(report.Roles.RoleCounts, 3)
	}

	if report.Archetype != nil {
		dp.PrimaryArchetype = report.Archetype.Primary
		dp.SecondaryArchetype = report.Archetype.Secondary
		dp.ArchetypeConfidence = report.Archetype.PrimaryConfidence
		dp.MeasuredBracket = report.Archetype.MeasuredBracket
		dp.MeasuredBracketLabel = report.Archetype.MeasuredBracketLabel
		// Declared bracket defaults to the measured value; the wizards/
		// override below stamps the precon corpus to B2.
		dp.Bracket = report.Archetype.MeasuredBracket
		dp.BracketLabel = report.Archetype.MeasuredBracketLabel
		dp.GameChangerCount = report.Archetype.GameChangerCount
		dp.GameChangerCards = report.Archetype.GameChangerCards
		dp.Intent = report.Archetype.Intent
		dp.BracketRationale = report.Archetype.BracketRationale
	}

	// WotC precon override: any deck file under data/decks/wizards/ is
	// stamped as B2 (Core) regardless of measured bracket. WotC publishes
	// product-stated bracket intent for every precon as B2; the measured
	// bracket can diverge upward (e.g. Atraxa Phyrexian Praetors measures
	// hotter than B2), but the rubber-stamp identity stays B2.
	if isWizardsPrecon(report.DeckPath) {
		dp.Bracket = 2
		dp.BracketLabel = "Core (precon)"
		if report.Archetype != nil {
			report.Archetype.Bracket = 2
			report.Archetype.BracketLabel = "Core (precon)"
		}
	}

	if report.WinLines != nil {
		dp.WinLineCount = len(report.WinLines.WinLines)
		dp.BackupCount = len(report.WinLines.BackupPlans)
		dp.SinglePointCount = len(report.WinLines.SinglePoints)
		dp.WeightedWinLineScore = report.WinLines.TotalWeightedScore
		if len(report.WinLines.WinLines) > 0 {
			wl := report.WinLines.WinLines[0]
			dp.PrimaryWinLine = strings.Join(wl.Pieces, " + ")
			dp.HasTutorAccess = len(wl.TutorPaths) > 0
		}
	}

	if oracle != nil && report.Commander != "" {
		computeCommanderSynergy(dp, report, oracle)
		// Now that commander themes are known, re-sort each card's
		// multi-tag role list so theme-aligned roles bubble up to the
		// primary (Roles[0]) position. Tie-breaks only roles sharing a
		// rolePriority value — never reorders across priority bands.
		RefineRolesByCommanderThemes(report.Roles, dp.CommanderThemes)
	}

	computeInteractionQuality(dp, report, oracle)
	computeProtectionDensity(dp, report, oracle)
	appendVulnerabilityBracketNote(dp)
	computeManaBaseGrade(dp, report, oracle)
	computeDeckCostTier(dp, report, oracle)
	// Opening hand sim runs first so dp.IsCommanderCentric is populated
	// before threat assessment reads it (commander-centric decks fear
	// commander-targeting hosers like Imprisoned in the Moon).
	computeOpeningHandSim(dp, report, oracle)
	computeThreatAssessment(dp, report)
	computeSynergyClusters(dp, report, oracle)
	computeAltBuildSuggestions(dp)
	computeMetaPositioning(dp)
	computeCardPower(dp, report)
	computeCardQualityTiers(dp, report, oracle)
	computePetCards(dp, report)
	computeLandSwapSuggestions(dp, report)
	computeCurveArchetypeFit(dp, report)
	dp.PersonalityBlurb = buildPersonalityBlurb(dp, report)
	dp.PowerPercentile, dp.PowerFactors = estimatePowerPercentile(dp, report)

	// CMC-distribution health check — populates OneDropCount /
	// TwoDropCount and appends warnings to report.CurveWarnings when
	// the deck is either 1-drop heavy or 2-drop starved. The
	// thresholds come from opening-hand hypergeometric math; see
	// computeCMCDistributionHealth for the calibration rationale.
	computeCMCDistributionHealth(dp, report)

	dp.Strengths = deriveStrengths(report, dp)
	dp.Weaknesses = deriveWeaknesses(report, dp)
	dp.GameplanSummary = buildGameplanSummary(dp, report)
	dp.CoachingTips = computeCoachingTips(dp, report)

	return dp
}

// InteractionDownside records one removal/counter/wipe in the deck
// that hands the opponent a meaningful resource: ramp from a basic-land
// search (Path to Exile), a creature token (Generous Gift / Beast Within
// / Pongify / Rapid Hybridization / Crib Swap), or pure card advantage
// (Oblation, Chaos Warp's free play). Self-cost downsides (Anguished
// Unmaking life payment) are deliberately NOT in scope — they're a
// tempo tax on the caster, not opponent leverage, and a B3 deck running
// Unmaking shouldn't be penalized for it.
type InteractionDownside struct {
	Name string
	CMC  int
	// Kind is the downside category: "ramp" (basic-land search),
	// "creature_token" (X/X token to opponent), "card_advantage"
	// (opponent draws / cycles), "free_spell" (opponent gets a free
	// cast off the top), or "other" for one-offs.
	Kind string
	Note string
}

// interactionDownsides is the curated card-name table of removal that
// hands the opponent a meaningful resource. Keys are normalized
// lowercase (matches the oracleDB lookup convention and the per-card
// match in computeInteractionQuality). Each entry carries the downside
// Kind for filtering and a short human-readable Note for the report.
//
// Curation policy: only cards where the downside is the FIRST-ORDER
// reason a builder would consider a swap. "Path gives ramp" is the
// canonical example — turn-defining vs a non-green opponent. Cards
// whose downside is just life payment to YOU (Anguished Unmaking,
// Vindicate's color requirement, etc.) are out of scope.
var interactionDownsides = map[string]struct {
	Kind string
	Note string
}{
	"path to exile":        {Kind: "ramp", Note: "opponent searches for a basic land — turn-defining ramp vs a non-green deck"},
	"generous gift":        {Kind: "creature_token", Note: "opponent gets a 3/3 elephant token"},
	"beast within":         {Kind: "creature_token", Note: "opponent gets a 3/3 beast token"},
	"pongify":              {Kind: "creature_token", Note: "opponent gets a 3/3 ape token"},
	"rapid hybridization": {Kind: "creature_token", Note: "opponent gets a 3/3 frog lizard token"},
	"crib swap":            {Kind: "creature_token", Note: "opponent gets a 1/1 changeling token"},
	"reality shift":        {Kind: "creature_token", Note: "opponent manifests the top of their library as a 2/2"},
	"oblation":             {Kind: "card_advantage", Note: "owner draws 2 cards — net positive card swap for the opponent"},
	"chaos warp":           {Kind: "free_spell", Note: "opponent reveals top of library; could be a free threat"},
	"swan song":            {Kind: "creature_token", Note: "opponent gets a 2/2 flying bird token"},
}

func computeInteractionQuality(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.Roles == nil {
		return
	}

	totalCMC := 0
	count := 0
	downsideCount := 0
	for _, a := range report.Roles.Assignments {
		isInteraction := false
		for _, r := range a.Roles {
			if r == RoleRemoval || r == RoleCounterspell || r == RoleBoardWipe {
				isInteraction = true
				break
			}
		}
		if !isInteraction {
			continue
		}

		var cmc int
		for _, p := range report.Profiles {
			if p.Name == a.Name {
				cmc = p.CMC
				break
			}
		}

		count++
		totalCMC += cmc
		if cmc <= 2 {
			dp.CheapInteraction++
		} else if cmc >= 4 {
			dp.ExpensiveInteraction++
		}

		if d, ok := interactionDownsides[strings.ToLower(a.Name)]; ok {
			downsideCount++
			dp.InteractionDownsides = append(dp.InteractionDownsides, InteractionDownside{
				Name: a.Name, CMC: cmc, Kind: d.Kind, Note: d.Note,
			})
		}
	}

	if count > 0 {
		dp.InteractionQuality = float64(totalCMC) / float64(count)
		// Effective-CMC penalty: each downside-bearing piece adds 0.5 to
		// the numerator before averaging. A piece that hands the opponent
		// ramp / a 3/3 / a free spell "feels" slower than its mana cost
		// suggests because the swing in board state offsets the tempo
		// gain from removing the original threat.
		dp.AdjustedInteractionQuality = (float64(totalCMC) + 0.5*float64(downsideCount)) / float64(count)
	}
	sort.SliceStable(dp.InteractionDownsides, func(i, j int) bool {
		return dp.InteractionDownsides[i].Name < dp.InteractionDownsides[j].Name
	})
}

// VulnerableComboPiece names a single RoleCombo card in the deck that
// has zero built-in protection — a high-vulnerability slot because a
// single removal spell resets the combo line. Reason is a short human-
// readable rationale rendered in the text report.
type VulnerableComboPiece struct {
	Name   string
	CMC    int
	Reason string
}

const vulnerableComboPieceCap = 8

func computeProtectionDensity(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	if report.Roles == nil || oracle == nil {
		return
	}

	keyRoles := map[RoleTag]bool{RoleCombo: true, RoleThreat: true}
	protectionWords := []string{
		"hexproof", "shroud", "indestructible", "ward",
		"protection from", "can't be the target",
		"can't be countered", "can't be destroyed",
		"phase out", "phases out",
	}

	cmcByName := map[string]int{}
	for _, p := range report.Profiles {
		cmcByName[strings.ToLower(p.Name)] = p.CMC
	}

	for _, a := range report.Roles.Assignments {
		isKey := false
		isCombo := false
		for _, r := range a.Roles {
			if keyRoles[r] {
				isKey = true
			}
			if r == RoleCombo {
				isCombo = true
			}
		}
		if !isKey {
			continue
		}

		entry := oracle.lookup(a.Name)
		if entry == nil {
			dp.UnprotectedKeyPieces++
			if isCombo {
				dp.VulnerableComboPieces = append(dp.VulnerableComboPieces, VulnerableComboPiece{
					Name:   a.Name,
					CMC:    cmcByName[strings.ToLower(a.Name)],
					Reason: "combo piece — no built-in protection (single removal resets the line)",
				})
			}
			continue
		}

		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}

		hasProtection := false
		for _, word := range protectionWords {
			if strings.Contains(ot, word) {
				hasProtection = true
				break
			}
		}

		if hasProtection {
			dp.ProtectedKeyPieces++
			continue
		}
		dp.UnprotectedKeyPieces++

		if !isCombo {
			continue
		}
		cmc := cmcByName[strings.ToLower(a.Name)]
		if cmc == 0 && entry.CMC > 0 {
			cmc = int(entry.CMC + 0.5)
		}
		dp.VulnerableComboPieces = append(dp.VulnerableComboPieces, VulnerableComboPiece{
			Name:   a.Name,
			CMC:    cmc,
			Reason: "combo piece — no built-in protection (single removal resets the line)",
		})
	}

	// Higher CMC first (bigger tempo loss to recast); stable secondary by
	// name so output is deterministic.
	sort.SliceStable(dp.VulnerableComboPieces, func(i, j int) bool {
		if dp.VulnerableComboPieces[i].CMC != dp.VulnerableComboPieces[j].CMC {
			return dp.VulnerableComboPieces[i].CMC > dp.VulnerableComboPieces[j].CMC
		}
		return dp.VulnerableComboPieces[i].Name < dp.VulnerableComboPieces[j].Name
	})
	if len(dp.VulnerableComboPieces) > vulnerableComboPieceCap {
		dp.VulnerableComboPieces = dp.VulnerableComboPieces[:vulnerableComboPieceCap]
	}
}

// appendVulnerabilityBracketNote surfaces a soft "vulnerability cap" note
// in the bracket rationale when a B4+ deck has unprotected combo pieces.
// Combo pieces with zero built-in protection mean a single removal spell
// (Path / Pongify / Imprisoned in the Moon) resets the win line — the
// deck's FELT power at the table is closer to B3 than its raw signal
// score suggests, because the kill is fragile.
//
// This is deliberately NOT a hard cap: the bracket itself is unchanged
// (FinalBracket / FinalLabel stay where the score / GC / combo signals
// placed them). The note is informational only, so builders see that
// adding Lightning Greaves / Heroic Intervention / Veil of Summer would
// realize the bracket's nominal power level. Only fires when measured
// bracket is strictly above 3 — at B3 and below, the cap-toward-B3 push
// is a no-op, and the per-card VulnerableComboPieces callout already
// covers the surface for casual decks.
func appendVulnerabilityBracketNote(dp *DeckProfile) {
	if dp == nil || dp.BracketRationale == nil {
		return
	}
	if dp.MeasuredBracket <= 3 {
		return
	}
	if len(dp.VulnerableComboPieces) == 0 {
		return
	}
	const evidenceCap = 3
	evidence := make([]string, 0, evidenceCap)
	for _, v := range dp.VulnerableComboPieces {
		if len(evidence) >= evidenceCap {
			break
		}
		evidence = append(evidence, v.Name)
	}
	piece := "piece"
	verb := "lacks"
	if len(dp.VulnerableComboPieces) != 1 {
		piece = "pieces"
		verb = "lack"
	}
	evidenceStr := strings.Join(evidence, ", ")
	if len(dp.VulnerableComboPieces) > len(evidence) {
		evidenceStr += fmt.Sprintf(", +%d more", len(dp.VulnerableComboPieces)-len(evidence))
	}
	note := fmt.Sprintf(
		"%d combo %s %s built-in protection (%s) — a single common removal spell can shut the win down, so at the table this deck plays closer to Bracket 3 than its Bracket-4+ label suggests. Adding protection (Lightning Greaves, Heroic Intervention, Veil of Summer) would let the deck reliably reach its full bracket power. The bracket call itself is unchanged; this is a heads-up, not a downgrade.",
		len(dp.VulnerableComboPieces), piece, verb, evidenceStr,
	)
	dp.BracketRationale.Signals = append(dp.BracketRationale.Signals, BracketSignal{
		Name:     "Vulnerability cap",
		Kind:     "note",
		Evidence: evidence,
		Note:     note,
	})
}

var commanderThemePatterns = []struct {
	Theme    string
	Patterns []string
}{
	{"sacrifice", []string{"sacrifice", "sacrificed", "sac a"}},
	{"tokens", []string{"create", "token", "populate", "embalm", "encore"}},
	{"counters", []string{"+1/+1 counter", "proliferate", "counter on"}},
	{"graveyard", []string{"graveyard", "mill", "dredge", "surveil", "reanimate", "return from your graveyard"}},
	{"lifegain", []string{"gain life", "lifelink", "whenever you gain life"}},
	{"spellcasting", []string{"instant", "sorcery", "whenever you cast", "magecraft", "prowess", "storm"}},
	{"combat", []string{"attacks", "combat damage", "combat phase", "first strike", "double strike"}},
	{"artifacts", []string{"artifact", "treasure", "equipment", "equip"}},
	{"enchantments", []string{"enchantment", "aura", "constellation", "enchanted"}},
	{"lands", []string{"landfall", "land enters", "search your library for a land"}},
	{"blink", []string{"exile", "return", "flicker", "exile target creature you control"}},
	{"discard", []string{"discard", "each opponent discards", "madness"}},
	{"tribal", []string{"creature type", "all creatures of the chosen type", "creatures you control get"}},
	{"drawing", []string{"draw", "draws a card", "whenever you draw"}},
}

func computeCommanderSynergy(dp *DeckProfile, report *FreyaReport, oracle *oracleDB) {
	cmdrEntry := oracle.lookup(report.Commander)
	if cmdrEntry == nil {
		return
	}

	cmdrOT := strings.ToLower(cmdrEntry.OracleText)
	if cmdrOT == "" && len(cmdrEntry.CardFaces) > 0 {
		cmdrOT = strings.ToLower(cmdrEntry.CardFaces[0].OracleText)
		if len(cmdrEntry.CardFaces) > 1 {
			cmdrOT += " " + strings.ToLower(cmdrEntry.CardFaces[1].OracleText)
		}
	}

	var themes []string
	for _, tp := range commanderThemePatterns {
		for _, pat := range tp.Patterns {
			if strings.Contains(cmdrOT, pat) {
				themes = append(themes, tp.Theme)
				break
			}
		}
	}
	dp.CommanderThemes = themes

	if len(themes) == 0 {
		return
	}

	synergyCount := 0
	nonlandCount := 0
	for _, p := range report.Profiles {
		if p.IsLand || p.Name == report.Commander {
			continue
		}
		nonlandCount++

		entry := oracle.lookup(p.Name)
		if entry == nil {
			continue
		}
		ot := strings.ToLower(entry.OracleText)
		if ot == "" && len(entry.CardFaces) > 0 {
			ot = strings.ToLower(entry.CardFaces[0].OracleText)
		}
		tl := strings.ToLower(p.TypeLine)

		for _, theme := range themes {
			matched := false
			for _, tp := range commanderThemePatterns {
				if tp.Theme != theme {
					continue
				}
				for _, pat := range tp.Patterns {
					if strings.Contains(ot, pat) || strings.Contains(tl, pat) {
						matched = true
						break
					}
				}
				break
			}
			if matched {
				synergyCount++
				break
			}
		}
	}

	dp.SynergyCount = synergyCount
	if nonlandCount > 0 {
		dp.CommanderSynergy = float64(synergyCount) / float64(nonlandCount)
	}
}

func topNRoles(counts map[RoleTag]int, n int) []RoleCount {
	var rcs []RoleCount
	for role, count := range counts {
		if count > 0 && role != RoleLand && role != RoleUtility {
			rcs = append(rcs, RoleCount{Role: role, Count: count})
		}
	}
	sort.Slice(rcs, func(i, j int) bool {
		return rcs[i].Count > rcs[j].Count
	})
	if len(rcs) > n {
		rcs = rcs[:n]
	}
	return rcs
}

func deriveStrengths(report *FreyaReport, dp *DeckProfile) []string {
	var s []string

	// Use NonLandTutorCount — basic-land ramp should not inflate the
	// "tutor package" headline. A 100-card deck with 10 ramp pieces and
	// 0 real tutors is not a tutor deck.
	if report.NonLandTutorCount >= 8 {
		s = append(s, fmt.Sprintf("deep tutor package (%d tutors)", report.NonLandTutorCount))
	} else if report.NonLandTutorCount >= 5 {
		s = append(s, fmt.Sprintf("strong tutor package (%d tutors)", report.NonLandTutorCount))
	}

	if dp.DrawCount >= 12 {
		s = append(s, fmt.Sprintf("excellent draw density (%d sources)", dp.DrawCount))
	} else if dp.DrawCount >= 8 {
		s = append(s, fmt.Sprintf("good draw density (%d sources)", dp.DrawCount))
	}

	if dp.RampCount >= 14 {
		s = append(s, fmt.Sprintf("heavy ramp package (%d pieces)", dp.RampCount))
	} else if dp.RampCount >= 10 {
		s = append(s, fmt.Sprintf("solid ramp package (%d pieces)", dp.RampCount))
	}

	if dp.WinLineCount >= 5 {
		s = append(s, fmt.Sprintf("diverse win lines (%d paths to victory)", dp.WinLineCount))
	} else if dp.WinLineCount >= 3 {
		s = append(s, fmt.Sprintf("multiple win lines (%d paths)", dp.WinLineCount))
	}

	if dp.SinglePointCount == 0 && dp.WinLineCount > 0 {
		s = append(s, "no single points of failure")
	}

	if dp.LandVerdict == "ok" {
		s = append(s, "land count on target")
	}

	if report.Stats != nil && len(report.Stats.ColorGaps) == 0 {
		s = append(s, "balanced mana base")
	}

	if dp.CommanderSynergy >= 0.60 {
		s = append(s, fmt.Sprintf("strong commander synergy (%.0f%% of cards match themes)", dp.CommanderSynergy*100))
	} else if dp.CommanderSynergy >= 0.40 {
		s = append(s, fmt.Sprintf("good commander synergy (%.0f%%)", dp.CommanderSynergy*100))
	}

	if dp.InteractionQuality > 0 && dp.InteractionQuality <= 2.0 {
		s = append(s, fmt.Sprintf("fast interaction (avg CMC %.1f, %d pieces at CMC ≤2)", dp.InteractionQuality, dp.CheapInteraction))
	}

	if dp.ProtectedKeyPieces > 0 {
		total := dp.ProtectedKeyPieces + dp.UnprotectedKeyPieces
		if total > 0 && float64(dp.ProtectedKeyPieces)/float64(total) >= 0.40 {
			s = append(s, fmt.Sprintf("well-protected key pieces (%d/%d have built-in protection)", dp.ProtectedKeyPieces, total))
		}
	}

	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction >= 12 {
		s = append(s, fmt.Sprintf("heavy interaction suite (%d pieces)", interaction))
	} else if interaction >= 8 {
		s = append(s, fmt.Sprintf("good interaction density (%d pieces)", interaction))
	}

	return s
}

// CMCDistributionWarning is the threshold structure used by
// computeCMCDistributionHealth — surfaced as exported so tests can pin
// the calibration without re-running the warning pipeline.
//
// Calibration (opening-hand hypergeometric, deck of 99, opener of 7):
//
//   - OneDropMax = 15. With 15 one-drops in 99, P(opener has ≥2 one-
//     drops) ≈ 21% — high enough that the deck is regularly burning a
//     hand slot on a low-impact card. Past 15, mid-game tempo dilution
//     starts to outweigh the consistency of a turn-1 play. The cEDH
//     fast-mana corpus (~6-10 one-drops average per the lyon sample)
//     stays well under this floor; only dedicated 1-drop-tribal piles
//     (Glistener Elf infect, Slippery Bogle voltron variants) push past.
//
//   - TwoDropMin = 6. With only 6 two-drops in 99, P(opener has zero
//     two-drops) = C(93,7)/C(99,7) ≈ 64% — the deck whiffs a turn-2 play
//     in roughly two of every three games. By 8 two-drops the whiff
//     drops to ~55%, by 12 to ~38%. Six is the floor below which the
//     curve consistency degrades enough to warrant flagging.
//
// Both thresholds are deliberately tuned to fire on STRUCTURAL issues
// rather than minor curve quirks — they don't double-fire with the
// existing "few early plays" warning (which keys on CMC 0+1 totals, a
// different aggregate signal).
type CMCDistributionWarning struct {
	OneDropMax int // cards at CMC 1 above this count trigger a warning
	TwoDropMin int // cards at CMC 2 below this count trigger a warning
}

var defaultCMCDistribution = CMCDistributionWarning{
	OneDropMax: 15,
	TwoDropMin: 6,
}

// computeCMCDistributionHealth populates dp.OneDropCount /
// dp.TwoDropCount and appends warnings to report.CurveWarnings when the
// deck violates the calibrated thresholds in defaultCMCDistribution.
//
// Warnings include the expected opening-hand impact so builders see WHY
// the threshold matters: "16 one-drops — 7×16/99 ≈ 1.1 expected in
// opener, mid-game tempo may dilute".
//
// Skips when report.NonlandCount == 0 (corrupt / empty profile) or when
// ManaCurve is unpopulated. Idempotent — calling twice does NOT
// double-append (warnings are de-duped against an existing exact match).
func computeCMCDistributionHealth(dp *DeckProfile, report *FreyaReport) {
	if dp == nil || report == nil || report.NonlandCount == 0 {
		return
	}
	dp.OneDropCount = report.ManaCurve[1]
	dp.TwoDropCount = report.ManaCurve[2]

	thresh := defaultCMCDistribution

	if dp.OneDropCount > thresh.OneDropMax {
		expected := 7.0 * float64(dp.OneDropCount) / float64(report.TotalCards)
		w := fmt.Sprintf("1-drop heavy: %d cards at CMC 1 (>%d threshold) — %.1f expected in opener; mid-game tempo may dilute",
			dp.OneDropCount, thresh.OneDropMax, expected)
		report.CurveWarnings = appendUniqueCurveWarning(report.CurveWarnings, w)
	}

	if dp.TwoDropCount < thresh.TwoDropMin {
		// Hypergeometric P(no 2-drop in opener of 7) given K 2-drops in
		// deck of TotalCards. Computed inline so the message reports the
		// actual whiff rate for THIS deck, not a tabulated approximation.
		whiffPct := openingHandWhiffPct(dp.TwoDropCount, report.TotalCards, 7)
		w := fmt.Sprintf("2-drop starved: only %d cards at CMC 2 (<%d threshold) — %.0f%% of openers contain no 2-drop, turn-2 plays will whiff often",
			dp.TwoDropCount, thresh.TwoDropMin, whiffPct)
		report.CurveWarnings = appendUniqueCurveWarning(report.CurveWarnings, w)
	}
}

// openingHandWhiffPct returns the % chance of drawing zero copies of a
// K-card subset in an n-card opener from a deck of N. Computed as the
// product (N-K)/(N) * (N-K-1)/(N-1) * ... — the closed-form
// hypergeometric P(X=0) without factorials.
//
// Returns 0 when k <= 0 (every opener whiffs trivially — but we don't
// surface that meaningless case), 100 when k == 0 with positive draw
// count (impossible to draw what isn't there), and 0 for malformed
// inputs (n > deckSize, deckSize <= 0).
func openingHandWhiffPct(k, deckSize, n int) float64 {
	if k <= 0 {
		return 100
	}
	if deckSize <= 0 || n <= 0 || n > deckSize {
		return 0
	}
	if k > deckSize {
		return 0
	}
	p := 1.0
	for i := 0; i < n; i++ {
		num := float64(deckSize - k - i)
		den := float64(deckSize - i)
		if num <= 0 {
			return 0
		}
		p *= num / den
	}
	return p * 100
}

// appendUniqueCurveWarning appends w to warnings IFF an exact-string
// match is not already present. Defends against double-append when
// BuildDeckProfile is invoked twice on the same report (rare but
// possible in tests that mutate a shared report).
func appendUniqueCurveWarning(warnings []string, w string) []string {
	for _, existing := range warnings {
		if existing == w {
			return warnings
		}
	}
	return append(warnings, w)
}

func deriveWeaknesses(report *FreyaReport, dp *DeckProfile) []string {
	var w []string

	if report.NonLandTutorCount < 3 && report.NonLandTutorCount > 0 {
		w = append(w, fmt.Sprintf("thin tutor package (%d tutors)", report.NonLandTutorCount))
	} else if report.NonLandTutorCount == 0 {
		w = append(w, "no tutors")
	}

	if dp.DrawCount < 5 && dp.DrawCount > 0 {
		w = append(w, fmt.Sprintf("low draw density (%d sources)", dp.DrawCount))
	} else if dp.DrawCount == 0 {
		w = append(w, "no dedicated draw sources")
	}

	if dp.RampCount < 7 && dp.RampCount > 0 {
		w = append(w, fmt.Sprintf("light ramp (%d pieces)", dp.RampCount))
	}

	if dp.WinLineCount <= 1 {
		w = append(w, "limited win conditions")
	}

	if dp.SinglePointCount > 0 {
		w = append(w, fmt.Sprintf("%d single point(s) of failure", dp.SinglePointCount))
	}

	if dp.LandVerdict == "too_few" {
		w = append(w, fmt.Sprintf("low land count (%d vs %d recommended)", dp.LandCount, dp.RecommendedLands))
	} else if dp.LandVerdict == "too_many" {
		w = append(w, fmt.Sprintf("high land count (%d vs %d recommended)", dp.LandCount, dp.RecommendedLands))
	}

	if report.Stats != nil && len(report.Stats.ColorGaps) > 0 {
		w = append(w, fmt.Sprintf("%d color gap(s) in mana base", len(report.Stats.ColorGaps)))
	}

	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction < 5 {
		w = append(w, fmt.Sprintf("low interaction (%d removal + counterspells)", interaction))
	}

	if report.Roles != nil && report.Roles.RoleCounts[RoleBoardWipe] == 0 {
		w = append(w, "no board wipes")
	}

	if dp.CommanderSynergy > 0 && dp.CommanderSynergy < 0.25 {
		w = append(w, fmt.Sprintf("low commander synergy (%.0f%%) — many cards don't align with commander themes", dp.CommanderSynergy*100))
	}

	if dp.InteractionQuality > 3.5 {
		w = append(w, fmt.Sprintf("slow interaction (avg CMC %.1f) — may not answer fast threats in time", dp.InteractionQuality))
	}

	if dp.UnprotectedKeyPieces > 0 {
		total := dp.ProtectedKeyPieces + dp.UnprotectedKeyPieces
		if total >= 3 && float64(dp.UnprotectedKeyPieces)/float64(total) >= 0.80 {
			w = append(w, fmt.Sprintf("key pieces exposed (%d/%d combo/threat pieces lack built-in protection)", dp.UnprotectedKeyPieces, total))
		}
	}

	// CMC-distribution surface — short callouts that mirror the more
	// detailed CurveWarnings entries on the report. Builders reading the
	// Weaknesses list see the structural issue without having to cross-
	// reference CurveWarnings.
	if dp.OneDropCount > defaultCMCDistribution.OneDropMax {
		w = append(w, fmt.Sprintf("1-drop heavy (%d at CMC 1, >%d threshold)", dp.OneDropCount, defaultCMCDistribution.OneDropMax))
	}
	if dp.TwoDropCount > 0 && dp.TwoDropCount < defaultCMCDistribution.TwoDropMin {
		w = append(w, fmt.Sprintf("2-drop starved (%d at CMC 2, <%d threshold)", dp.TwoDropCount, defaultCMCDistribution.TwoDropMin))
	}

	return w
}

// ComputeEvalWeights derives MCTS evaluator weights from the deck profile.
// Starts from archetype defaults and adjusts based on deck-specific signals
// (tutor density, graveyard recursion, ramp count, win line structure).
func ComputeEvalWeights(dp *DeckProfile, report *FreyaReport) *jsonEvalWeights {
	arch := strings.ToLower(dp.PrimaryArchetype)
	if arch == "" {
		arch = "midrange"
	}

	defaults := defaultWeights[arch]
	if defaults == nil {
		defaults = defaultWeights["midrange"]
	}

	w := *defaults

	// Tutor density boosts combo proximity weight. NonLandTutorCount only —
	// fetchlands/ramp don't help find combo pieces.
	if report.NonLandTutorCount >= 8 {
		w.ComboProximity += 0.3
	} else if report.NonLandTutorCount >= 5 {
		w.ComboProximity += 0.15
	}

	// Recursion-heavy decks get graveyard value boost. Raw IsRecursion
	// count is the floor signal — a deck with 5 reanimation bodies wants
	// graveyard play even with no detected loop. Value-chain recursion
	// depth (computed in classifyValueChains) is the ceiling signal — a
	// chain whose last step's To==first step's From forms a literal
	// graveyard loop (Eternal Witness + Strip Mine + Crucible style),
	// which is structurally stronger than 5 unrelated recursion cards.
	// Both stack so a loop-bearing deck with deep recursion bench gets
	// the full boost.
	recursionCount := 0
	for _, p := range report.Profiles {
		if p.IsRecursion {
			recursionCount++
		}
	}
	if recursionCount >= 5 {
		w.GraveyardValue += 0.4
	} else if recursionCount >= 3 {
		w.GraveyardValue += 0.2
	}
	switch MaxValueChainRecursionDepth(report.ValueChains) {
	case "infinite":
		w.GraveyardValue += 0.5
	case "deep":
		w.GraveyardValue += 0.25
	case "shallow":
		w.GraveyardValue += 0.1
	}

	// Heavy ramp package boosts mana advantage weight.
	if dp.RampCount >= 14 {
		w.ManaAdvantage += 0.3
	} else if dp.RampCount >= 10 {
		w.ManaAdvantage += 0.15
	}

	// Multiple win lines with tutor access boosts combo proximity.
	if dp.WinLineCount >= 3 && dp.HasTutorAccess {
		w.ComboProximity += 0.2
	}

	// Low interaction decks should weight threat exposure higher.
	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	if interaction < 5 {
		w.ThreatExposure += 0.3
	}

	return &w
}

var defaultWeights = map[string]*jsonEvalWeights{
	"aggro": {
		BoardPresence: 1.5, CardAdvantage: 0.4, ManaAdvantage: 0.3,
		LifeResource: 0.8, ComboProximity: 0.1, ThreatExposure: 0.6,
		CommanderProgress: 0.9, GraveyardValue: 0.2,
	},
	"aggro / go wide": {
		BoardPresence: 1.8, CardAdvantage: 0.4, ManaAdvantage: 0.3,
		LifeResource: 0.7, ComboProximity: 0.1, ThreatExposure: 0.5,
		CommanderProgress: 0.6, GraveyardValue: 0.2,
	},
	"combo": {
		BoardPresence: 0.4, CardAdvantage: 0.8, ManaAdvantage: 0.7,
		LifeResource: 0.3, ComboProximity: 2.0, ThreatExposure: 0.5,
		CommanderProgress: 0.6, GraveyardValue: 0.5,
	},
	"combo / infinite": {
		BoardPresence: 0.3, CardAdvantage: 0.9, ManaAdvantage: 0.8,
		LifeResource: 0.2, ComboProximity: 2.2, ThreatExposure: 0.5,
		CommanderProgress: 0.5, GraveyardValue: 0.6,
	},
	"control": {
		BoardPresence: 0.5, CardAdvantage: 1.5, ManaAdvantage: 0.8,
		LifeResource: 0.6, ComboProximity: 0.4, ThreatExposure: 1.2,
		CommanderProgress: 0.5, GraveyardValue: 0.4,
	},
	"midrange": {
		BoardPresence: 1.0, CardAdvantage: 1.0, ManaAdvantage: 0.8,
		LifeResource: 0.7, ComboProximity: 0.5, ThreatExposure: 0.8,
		CommanderProgress: 0.7, GraveyardValue: 0.5,
	},
	"ramp": {
		BoardPresence: 0.6, CardAdvantage: 0.7, ManaAdvantage: 1.8,
		LifeResource: 0.5, ComboProximity: 0.3, ThreatExposure: 0.6,
		CommanderProgress: 0.8, GraveyardValue: 0.3,
	},
	"voltron": {
		BoardPresence: 0.8, CardAdvantage: 0.5, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.2, ThreatExposure: 0.9,
		CommanderProgress: 2.0, GraveyardValue: 0.3,
	},
	"storm": {
		BoardPresence: 0.2, CardAdvantage: 1.3, ManaAdvantage: 1.5,
		LifeResource: 0.2, ComboProximity: 1.8, ThreatExposure: 0.3,
		CommanderProgress: 0.4, GraveyardValue: 0.3,
	},
	"aristocrats": {
		BoardPresence: 1.0, CardAdvantage: 0.8, ManaAdvantage: 0.5,
		LifeResource: 0.9, ComboProximity: 1.2, ThreatExposure: 0.7,
		CommanderProgress: 0.5, GraveyardValue: 1.2,
	},
	"artifacts": {
		BoardPresence: 1.2, CardAdvantage: 0.8, ManaAdvantage: 1.0,
		LifeResource: 0.4, ComboProximity: 1.0, ThreatExposure: 0.6,
		CommanderProgress: 0.6, GraveyardValue: 0.8,
	},
	"enchantress": {
		BoardPresence: 0.8, CardAdvantage: 1.4, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 0.8, ThreatExposure: 0.7,
		CommanderProgress: 0.6, GraveyardValue: 0.3,
	},
	"reanimator": {
		BoardPresence: 0.7, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.4, ComboProximity: 0.8, ThreatExposure: 0.5,
		CommanderProgress: 0.5, GraveyardValue: 2.0,
	},
	"lands matter": {
		BoardPresence: 0.7, CardAdvantage: 0.7, ManaAdvantage: 1.8,
		LifeResource: 0.6, ComboProximity: 0.4, ThreatExposure: 0.5,
		CommanderProgress: 0.7, GraveyardValue: 0.8,
	},
	"tribal": {
		BoardPresence: 1.6, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.7, ComboProximity: 0.3, ThreatExposure: 0.7,
		CommanderProgress: 0.8, GraveyardValue: 0.3,
	},
	"superfriends": {
		BoardPresence: 0.9, CardAdvantage: 1.2, ManaAdvantage: 0.7,
		LifeResource: 0.8, ComboProximity: 0.4, ThreatExposure: 1.0,
		CommanderProgress: 0.4, GraveyardValue: 0.3,
	},
	"mill": {
		BoardPresence: 0.4, CardAdvantage: 0.8, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 1.4, ThreatExposure: 0.8,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"lifegain": {
		BoardPresence: 0.9, CardAdvantage: 0.7, ManaAdvantage: 0.5,
		LifeResource: 1.8, ComboProximity: 0.8, ThreatExposure: 0.6,
		CommanderProgress: 0.6, GraveyardValue: 0.3,
	},
	"discard / hand attack": {
		BoardPresence: 0.6, CardAdvantage: 1.4, ManaAdvantage: 0.5,
		LifeResource: 0.5, ComboProximity: 0.6, ThreatExposure: 1.0,
		CommanderProgress: 0.7, GraveyardValue: 0.4,
	},
	"blink / flicker": {
		BoardPresence: 1.1, CardAdvantage: 1.0, ManaAdvantage: 0.6,
		LifeResource: 0.5, ComboProximity: 0.9, ThreatExposure: 0.6,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"spellslinger": {
		BoardPresence: 0.3, CardAdvantage: 1.4, ManaAdvantage: 1.0,
		LifeResource: 0.3, ComboProximity: 1.2, ThreatExposure: 0.4,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"counters matter": {
		BoardPresence: 1.4, CardAdvantage: 0.6, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.8, ThreatExposure: 0.6,
		CommanderProgress: 0.7, GraveyardValue: 0.3,
	},
	"stax": {
		BoardPresence: 0.5, CardAdvantage: 1.2, ManaAdvantage: 0.8,
		LifeResource: 0.4, ComboProximity: 0.6, ThreatExposure: 1.4,
		CommanderProgress: 0.5, GraveyardValue: 0.3,
	},
	"extra combats": {
		BoardPresence: 1.4, CardAdvantage: 0.5, ManaAdvantage: 0.5,
		LifeResource: 0.7, ComboProximity: 0.6, ThreatExposure: 0.6,
		CommanderProgress: 1.5, GraveyardValue: 0.2,
	},
	"theft / clone": {
		BoardPresence: 0.8, CardAdvantage: 1.0, ManaAdvantage: 0.7,
		LifeResource: 0.5, ComboProximity: 0.5, ThreatExposure: 1.0,
		CommanderProgress: 0.5, GraveyardValue: 0.4,
	},
	"ninjutsu / evasion": {
		BoardPresence: 1.0, CardAdvantage: 1.2, ManaAdvantage: 0.5,
		LifeResource: 0.6, ComboProximity: 0.3, ThreatExposure: 0.5,
		CommanderProgress: 0.8, GraveyardValue: 0.2,
	},

	// r60: Pillowfort — defensive deck that deters attacks via tax
	// effects (Ghostly Prison, Propaganda, Sphere of Safety),
	// prevention (Story Circle, Aegis of the Gods), and punishment
	// triggers (No Mercy, Solitary Confinement), then wins slowly via
	// inevitability rather than board pressure. Classified by the
	// archetypes.go fingerprint at line 234 (`Pillowfort`) since the
	// R60 archetype sweep but had no eval-weight entry — fell back to
	// midrange, which over-weighted BoardPresence (pillowfort
	// DELIBERATELY underdevelops to avoid painting a target) and
	// under-weighted LifeResource + ThreatExposure (the entire plan
	// is "survive the next turn cycle, grind out inevitability"). The
	// asymmetry is the same magnitude as the Voltron-vs-midrange
	// divergence (CommanderProgress 2.0 vs 0.7); without it the hat
	// mis-plays pillowfort decks as if they were midrange, casting
	// into pressure they should sandbag and ignoring tax-stack
	// maintenance they should prioritize.
	//
	//   - BoardPresence 0.4: don't develop threats that paint a
	//     target (vs midrange 1.0). Defensive permanents enter as
	//     non-creature tax stack.
	//   - CardAdvantage 1.3: grind value over time; outlast the
	//     fastest opponent (vs midrange 1.0).
	//   - ManaAdvantage 0.7: moderate; pillowfort needs mana for big
	//     control plays but isn't ramp-focused (vs midrange 0.8).
	//   - LifeResource 1.4: SURVIVAL is the plan. Much higher than
	//     midrange 0.7 — pillowfort decks track life like a clock.
	//   - ComboProximity 0.3: usually not a combo deck (vs midrange
	//     0.5). Pillowfort wincon is incremental, not assembly.
	//   - ThreatExposure 1.5: HIGH threat awareness. Pillowfort gets
	//     killed by under-respected board states it can't break
	//     parity with (vs midrange 0.8).
	//   - CommanderProgress 0.4: usually passive commander (Sram,
	//     Ghen, Norn-style). Don't tunnel-vision commander damage
	//     (vs midrange 0.7).
	//   - GraveyardValue 0.4: not graveyard-focused (vs midrange 0.5).
	"pillowfort": {
		BoardPresence: 0.4, CardAdvantage: 1.3, ManaAdvantage: 0.7,
		LifeResource: 1.4, ComboProximity: 0.3, ThreatExposure: 1.5,
		CommanderProgress: 0.4, GraveyardValue: 0.4,
	},
}

type jsonEvalWeights struct {
	BoardPresence     float64 `json:"board_presence"`
	CardAdvantage     float64 `json:"card_advantage"`
	ManaAdvantage     float64 `json:"mana_advantage"`
	LifeResource      float64 `json:"life_resource"`
	ComboProximity    float64 `json:"combo_proximity"`
	ThreatExposure    float64 `json:"threat_exposure"`
	CommanderProgress float64 `json:"commander_progress"`
	GraveyardValue    float64 `json:"graveyard_value"`
}

func buildGameplanSummary(dp *DeckProfile, report *FreyaReport) string {
	archetype := dp.PrimaryArchetype
	if archetype == "" {
		archetype = "Midrange"
	}

	var winMethod string
	if dp.WinLineCount > 0 && dp.PrimaryWinLine != "" {
		first := report.WinLines.WinLines[0]
		switch first.Type {
		case "infinite", "determined":
			winMethod = dp.PrimaryWinLine + " combo"
		case "finisher":
			winMethod = dp.PrimaryWinLine
		case "combat":
			winMethod = "combat damage"
		case "commander_damage":
			winMethod = "commander damage"
		default:
			winMethod = dp.PrimaryWinLine
		}
	} else {
		winMethod = "combat damage"
	}

	var backup string
	comboLines := 0
	if report.WinLines != nil {
		for _, wl := range report.WinLines.WinLines {
			if wl.Type == "infinite" || wl.Type == "determined" || wl.Type == "finisher" {
				comboLines++
			}
		}
	}
	if comboLines > 1 {
		backup = fmt.Sprintf(" %d backup lines available.", comboLines-1)
	} else if dp.BackupCount > 0 {
		backup = fmt.Sprintf(" %d backup plan(s).", dp.BackupCount)
	}

	var tutorNote string
	if dp.HasTutorAccess && report.NonLandTutorCount >= 5 {
		tutorNote = fmt.Sprintf(" Supported by %d tutors.", report.NonLandTutorCount)
	}

	var gcNote string
	if dp.GameChangerCount > 0 {
		gcNote = fmt.Sprintf(", %d GC", dp.GameChangerCount)
	}
	bracket := fmt.Sprintf(" Strict B%d (%s%s).", dp.Bracket, dp.BracketLabel, gcNote)

	return fmt.Sprintf("%s deck that wins via %s.%s%s%s",
		archetype, winMethod, backup, tutorNote, bracket)
}
