package hat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Freya JSON mirror types (hat-local, cannot import cmd/hexdek-freya)
// ---------------------------------------------------------------------------

type freyaJSON struct {
	Archetype   *freyaArchetype   `json:"archetype,omitempty"`
	WinLines    *freyaWinLines    `json:"win_lines,omitempty"`
	ValueChains []freyaValueChain `json:"value_chains,omitempty"`
	FullProfile *freyaDeckProfile `json:"unified_profile,omitempty"`
}

type freyaArchetype struct {
	Primary   string  `json:"primary"`
	Bracket   int     `json:"bracket"`
	Secondary string  `json:"secondary,omitempty"`
	Confidence float64 `json:"confidence"`
}

type freyaWinLines struct {
	Lines        []freyaWinLine `json:"lines"`
	SinglePoints []string       `json:"single_points_of_failure,omitempty"`
}

type freyaWinLine struct {
	Pieces     []string          `json:"pieces"`
	Type       string            `json:"type"`
	Class      string            `json:"class,omitempty"`
	Desc       string            `json:"description,omitempty"`
	TutorPaths []freyaTutorChain `json:"tutor_paths,omitempty"`
}

type freyaTutorChain struct {
	Tutor    string `json:"tutor"`
	Finds    string `json:"finds"`
	Delivery string `json:"delivery"`
}

type freyaValueChain struct {
	Name           string                `json:"name"`
	Steps          []freyaValueChainStep `json:"steps"`
	BridgeCards    []string              `json:"bridge_cards,omitempty"`
	RecursionDepth string                `json:"recursion_depth,omitempty"`
}

type freyaValueChainStep struct {
	Cards []string `json:"cards"`
}

type freyaDeckProfile struct {
	PrimaryArchetype string `json:"primary_archetype"`
	Bracket          int    `json:"bracket"`
	GameplanSummary  string `json:"gameplan_summary"`
	// PowerTier carries the 5-tier cEDH classification from Freya's
	// ClassifyCEDHPowerTier (PRs #714/#715). 1-2 = Casual, 3 =
	// Upgraded Precon, 4 = High Power, 5 = cEDH. Drives
	// ApplyPowerTierRouting in the hat strategy loader.
	PowerTier        int     `json:"cedh_power_tier,omitempty"`
	// PowerTierConfidence — see CEDHPowerTier.Confidence in
	// cmd/hexdek-freya/power_tier_cedh.go. [0, 1].
	PowerTierConfidence float64 `json:"cedh_power_tier_confidence,omitempty"`
}

// ---------------------------------------------------------------------------
// Strategy JSON mirror types (compact machine format from Freya)
// ---------------------------------------------------------------------------

// strategyFileJSON is the on-wire schema Freya emits to .strategy.json.
//
// Field-by-field contract (audited in the freya-hat integration final
// polish, 2026-05-30 — wave-5):
//
//   - archetype: lowercase string, expected to be a known archetype
//     constant (see archetypes.go). Empty string / unknown values fall
//     through to DefaultWeightsForArchetype's default branch.
//   - bracket: int, expected [1, 5] (0 = unset). Clamped to [0, 5]
//     by normalizeStrategyProfile; consumers handle 0 as "unknown
//     bracket, use archetype defaults".
//   - cedh_power_tier: int, expected [1, 5] (0 = unset). Clamped to
//     [0, 5]. ApplyPowerTierRouting no-ops on tier=0 or out-of-band.
//   - cedh_power_tier_confidence: float in [0, 1]. The
//     effectiveConfidence helper already clamps and treats 0 as "no
//     confidence supplied" (legacy default of 1.0); normalizeStrategyProfile
//     clamps before that to keep the invariant explicit.
//   - gameplan_summary: free-text string (debug field, no decision impact).
//   - win_lines: array of {pieces, type, class, tutor_paths}. Empty
//     piece arrays skipped at load time. Type/class strings are
//     compared by exact match by downstream consumers.
//   - value_engine_keys / tutor_targets / finisher_cards / star_cards /
//     cuttable_cards / commander_themes / vulnerable_to: card-name
//     string arrays. Nil/empty treated as "no entries", consumers
//     gate on len()>0 before iterating.
//   - eval_weights: pointer to 8-dimension weight struct. nil → hat
//     uses DefaultWeightsForArchetype. When present, 12 secondary
//     dimensions (StaxLockProgress, DrainEngine, ArtifactSynergy, …)
//     fill in from archetype defaults so partial weights stay coherent.
//   - card_roles: map[card_name → role_tag]. Empty map / nil treated
//     as "no role info"; consumers fall back to oracle-text inference.
//   - color_demand: map[color → pip_count]. Nil tolerable.
//   - max_recursion_depth: string in {"", "shallow", "deep", "infinite"}.
//     Unknown values fall through to no-op.
//   - commander_synergy: float in [0, 1]. Clamped by
//     normalizeStrategyProfile; consumers multiply directly so an
//     out-of-range value would distort scoring.
//   - interaction_avg_cmc: float >= 0. Clamped at 0; an avg CMC
//     of 3.0+ triggers the cast-proactively branch in cardHeuristic.
//   - cheap_interaction: int >= 0. Clamped at 0; thresholds at 0 / 4+
//     drive cheapInteractionPassAdjust.
//   - mana_base_grade: string in {"", "A", "B", "C", "D", "F"}.
//     Empty / unknown falls through to default 1.5x color-fixing mult.
//   - keepable_hand_pct: float in [0, 100]. Clamped. Mulligan logic
//     gates on >0 and <60 so out-of-range values would mis-trigger.
//   - is_commander_centric: bool, no validation needed.
//   - protected_key_pieces / unprotected_key_pieces: int >= 0.
//     Clamped at 0. ProtectionRatio() handles sum<3 as -1 sentinel
//     (insufficient signal).
//   - power_percentile: int in [0, 100]. Clamped. BudgetForPower
//     scales at percentile thresholds 60 / 80.
//   - meta_matchups: array of {archetype, rating}. Rating expected
//     in {"favored", "neutral", "unfavored"}; unknown ratings fall
//     through to no-op in the chooseAttacker meta lookup.
//   - emergent_synergies: array of {cards, effect_pattern, tier,
//     avg_impact}. Tier in [1, 3]. applyEmergentSynergyBoost and
//     emergentSynergyBump both have explicit switch arms for tier
//     2 and 3 only — tier 1 / 0 / out-of-range falls through to no-op.
//   - synergy_clusters: array of {name, theme, members, high_density}.
//     Empty members skipped at load. Member matching is case-
//     insensitive in synergyClusterCohesionBoost.
//
// All scalar fields default to Go zero on missing JSON keys (per
// encoding/json semantics). normalizeStrategyProfile is called after
// both build paths (buildFromStrategyJSON and buildStrategyProfile) so
// every consumer sees clamped values regardless of source.
type strategyFileJSON struct {
	Archetype       string              `json:"archetype"`
	Bracket         int                 `json:"bracket"`
	// PowerTier — 5-tier cEDH classification from Freya
	// ClassifyCEDHPowerTier (PRs #714/#715). 1-5; 0 = unset.
	PowerTier       int                 `json:"cedh_power_tier,omitempty"`
	// PowerTierConfidence — see CEDHPowerTier.Confidence (PR #717).
	PowerTierConfidence float64 `json:"cedh_power_tier_confidence,omitempty"`
	GameplanSummary string              `json:"gameplan_summary"`
	WinLines        []freyaWinLine      `json:"win_lines,omitempty"`
	ValueEngineKeys []string            `json:"value_engine_keys,omitempty"`
	TutorTargets    []string            `json:"tutor_targets,omitempty"`
	Weights         *freyaEvalWeights   `json:"eval_weights,omitempty"`
	CardRoles       map[string]string   `json:"card_roles,omitempty"`
	FinisherCards   []string            `json:"finisher_cards,omitempty"`
	ColorDemand     map[string]int      `json:"color_demand,omitempty"`

	MaxRecursionDepth string              `json:"max_recursion_depth,omitempty"`

	StarCards         []string            `json:"star_cards,omitempty"`
	CuttableCards     []string            `json:"cuttable_cards,omitempty"`
	CommanderThemes   []string            `json:"commander_themes,omitempty"`
	CommanderSynergy  float64             `json:"commander_synergy,omitempty"`
	VulnerableTo      []string            `json:"vulnerable_to,omitempty"`
	InteractionAvgCMC float64             `json:"interaction_avg_cmc,omitempty"`
	CheapInteraction  int                 `json:"cheap_interaction,omitempty"`
	ManaBaseGrade     string              `json:"mana_base_grade,omitempty"`
	KeepableHandPct    float64             `json:"keepable_hand_pct,omitempty"`
	IsCommanderCentric bool                `json:"is_commander_centric,omitempty"`
	// ProtectedKeyPieces / UnprotectedKeyPieces are the count of
	// RoleCombo / RoleThreat cards with and without built-in protection.
	// Added in the wave-3 freya-hat integration audit (2026-05-30).
	// Consumed via StrategyProfile.ProtectionRatio() + the evaluator's
	// protectionThreatScalar helper which adjusts ThreatExposure weight.
	ProtectedKeyPieces   int                `json:"protected_key_pieces,omitempty"`
	UnprotectedKeyPieces int                `json:"unprotected_key_pieces,omitempty"`
	PowerPercentile    int                 `json:"power_percentile,omitempty"`
	MetaMatchups      []freyaMetaMatchup      `json:"meta_matchups,omitempty"`
	EmergentSynergies []freyaEmergentSynergy  `json:"emergent_synergies,omitempty"`
	// SynergyClusters carry Freya's themed cluster analysis (wave-4
	// freya-hat integration audit, 2026-05-30). Consumed via
	// StrategyProfile.SynergyClusters + the yggdrasil's
	// synergyClusterCohesionBoost helper which boosts cardHeuristic
	// for cards from already-active clusters (≥2 members on battlefield).
	SynergyClusters []freyaSynergyCluster  `json:"synergy_clusters,omitempty"`
	// HuginnPredictions carry dev-19's --predict CLI output:
	// speculative combo predictions Huginn generates from tier-3
	// patterns against the target deck. Each prediction has a unique
	// InstanceID so the post-game feedback loop
	// (huginn.ComputePredictionOutcomes + RecordPredictionOutcomes)
	// can route per-prediction confidence adjustments back to the
	// pattern catalog. Hat consumes via huginnPredictionBoost in
	// cardHeuristic. Worker D — Huginn 2.0 (2026-05-31).
	HuginnPredictions []freyaHuginnPrediction `json:"huginn_predictions,omitempty"`
}

type freyaHuginnPrediction struct {
	InstanceID string   `json:"instance_id"`
	Cards      []string `json:"cards"`
	Pattern    string   `json:"pattern,omitempty"`
	Confidence float64  `json:"confidence"`
}

type freyaSynergyCluster struct {
	Name        string   `json:"name"`
	Theme       string   `json:"theme,omitempty"`
	Members     []string `json:"members"`
	HighDensity bool     `json:"high_density,omitempty"`
}

type freyaEmergentSynergy struct {
	Cards            []string `json:"cards"`
	EffectPattern    string   `json:"effect_pattern"`
	Tier             int      `json:"tier"`
	ObservationCount int      `json:"observation_count"`
	AvgImpact        float64  `json:"avg_impact"`
}

type freyaMetaMatchup struct {
	Archetype string `json:"archetype"`
	Rating    string `json:"rating"`
}

type freyaEvalWeights struct {
	BoardPresence     float64 `json:"board_presence"`
	CardAdvantage     float64 `json:"card_advantage"`
	ManaAdvantage     float64 `json:"mana_advantage"`
	LifeResource      float64 `json:"life_resource"`
	ComboProximity    float64 `json:"combo_proximity"`
	ThreatExposure    float64 `json:"threat_exposure"`
	CommanderProgress float64 `json:"commander_progress"`
	GraveyardValue    float64 `json:"graveyard_value"`
}

// ---------------------------------------------------------------------------
// Public loader
// ---------------------------------------------------------------------------

// LoadStrategyFromFreya reads Freya analysis data for a deck and returns
// a StrategyProfile. Prefers the compact .strategy.json format; falls back
// to the full _freya.json. Returns nil if neither exists or can't be
// parsed (graceful degradation — hat runs without strategy).
//
// Search order for each format: <deckdir>/freya/<base>.strategy.json,
// then <deckdir>/../freya/<base>.strategy.json (parent-dir fallback).
//
// r60-cedh-bottleneck fix (docs/hat-bottleneck-r60.md): the parent-dir
// fallback covers the common gauntlet layout
//
//   /tmp/cedh-seat-bias/
//     freya/<deck>.strategy.json      ← analysis here
//     batch_a/<deck>.txt              ← deck staged here
//     batch_b/<deck>.txt
//
// where the analysis is staged once at the root and decks are partitioned
// into batch dirs. The pre-fix loader only checked the deck's immediate
// parent's freya/ subdir, so batch-staged decks silently got profile=nil
// and the hat played without any strategy intelligence. This drove the
// false-null result chain in PRs #793 + #826 + #848 — every architectural
// change in those PRs depended on Strategy.ComboPieces being populated,
// which it never was for these gauntlet runs.
func LoadStrategyFromFreya(deckPath string) *StrategyProfile {
	dir := filepath.Dir(deckPath)
	base := filepath.Base(deckPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Candidate roots: deck's immediate parent, then deck's grandparent.
	// Grandparent covers the staged-pool gauntlet layout above.
	roots := []string{dir, filepath.Dir(dir)}

	// Prefer compact strategy format across all roots before falling
	// back to the full Freya JSON — a parent-dir strategy.json should
	// still beat a deck-local _freya.json when both exist.
	for _, root := range roots {
		stratPath := filepath.Join(root, "freya", base+".strategy.json")
		if data, err := os.ReadFile(stratPath); err == nil {
			var sj strategyFileJSON
			if err := json.Unmarshal(data, &sj); err == nil {
				return buildFromStrategyJSON(&sj)
			}
		}
	}

	// Fallback to full Freya JSON.
	for _, root := range roots {
		freyaPath := filepath.Join(root, "freya", base+"_freya.json")
		data, err := os.ReadFile(freyaPath)
		if err != nil {
			continue
		}
		var fj freyaJSON
		if err := json.Unmarshal(data, &fj); err != nil {
			continue
		}
		return buildStrategyProfile(&fj)
	}

	return nil
}

func buildFromStrategyJSON(sj *strategyFileJSON) *StrategyProfile {
	sp := &StrategyProfile{
		Archetype:         sj.Archetype,
		Bracket:           sj.Bracket,
		PowerTier:         sj.PowerTier,
		PowerTierConfidence: sj.PowerTierConfidence,
		GameplanSummary:   sj.GameplanSummary,
		TutorTargets:      sj.TutorTargets,
		ValueEngineKeys:   sj.ValueEngineKeys,
		CardRoles:         sj.CardRoles,
		FinisherCards:     sj.FinisherCards,
		ColorDemand:       sj.ColorDemand,
		StarCards:         sj.StarCards,
		CuttableCards:     sj.CuttableCards,
		CommanderThemes:   sj.CommanderThemes,
		CommanderSynergy:  sj.CommanderSynergy,
		VulnerableTo:      sj.VulnerableTo,
		InteractionAvgCMC: sj.InteractionAvgCMC,
		CheapInteraction:  sj.CheapInteraction,
		ManaBaseGrade:      sj.ManaBaseGrade,
		KeepableHandPct:    sj.KeepableHandPct,
		IsCommanderCentric: sj.IsCommanderCentric,
		ProtectedKeyPieces:   sj.ProtectedKeyPieces,
		UnprotectedKeyPieces: sj.UnprotectedKeyPieces,
		PowerPercentile:    sj.PowerPercentile,
		MaxRecursionDepth:  sj.MaxRecursionDepth,
	}

	if len(sj.MetaMatchups) > 0 {
		sp.MetaMatchups = make(map[string]string, len(sj.MetaMatchups))
		for _, mm := range sj.MetaMatchups {
			sp.MetaMatchups[mm.Archetype] = mm.Rating
		}
	}

	for _, c := range sj.SynergyClusters {
		if len(c.Members) == 0 {
			continue
		}
		sp.SynergyClusters = append(sp.SynergyClusters, SynergyCluster{
			Name:        c.Name,
			Theme:       c.Theme,
			Members:     c.Members,
			HighDensity: c.HighDensity,
		})
	}

	for _, p := range sj.HuginnPredictions {
		if p.InstanceID == "" || len(p.Cards) == 0 {
			continue
		}
		sp.HuginnPredictions = append(sp.HuginnPredictions, HuginnPrediction{
			InstanceID: p.InstanceID,
			Cards:      p.Cards,
			Pattern:    p.Pattern,
			Confidence: p.Confidence,
		})
	}

	for _, wl := range sj.WinLines {
		if len(wl.Pieces) == 0 {
			continue
		}
		cp := ComboPlan{
			Pieces: wl.Pieces,
			Type:   wl.Type,
			Class:  wl.Class,
		}
		if len(wl.TutorPaths) > 0 && len(wl.Pieces) == 2 {
			cp.CastOrder = deriveCastOrder(wl.Pieces, wl.TutorPaths)
		}
		if len(cp.CastOrder) == 0 {
			cp.CastOrder = append([]string{}, wl.Pieces...)
		}
		sp.ComboPieces = append(sp.ComboPieces, cp)
	}

	if sj.Weights != nil {
		// Freya serializes only the 8 core dimensions. Start from the
		// archetype-appropriate full profile so the remaining 12 dims
		// (StaxLockProgress, DrainEngine, ArtifactSynergy, ...) keep
		// archetype-relevant values instead of being zeroed out.
		base := DefaultWeightsForArchetype(sp.Archetype)
		base.BoardPresence = sj.Weights.BoardPresence
		base.CardAdvantage = sj.Weights.CardAdvantage
		base.ManaAdvantage = sj.Weights.ManaAdvantage
		base.LifeResource = sj.Weights.LifeResource
		base.ComboProximity = sj.Weights.ComboProximity
		base.ThreatExposure = sj.Weights.ThreatExposure
		base.CommanderProgress = sj.Weights.CommanderProgress
		base.GraveyardValue = sj.Weights.GraveyardValue
		sp.Weights = &base
	}

	// Emergent synergies from Huginn — soft eval weight bumps.
	for _, es := range sj.EmergentSynergies {
		sp.EmergentSynergies = append(sp.EmergentSynergies, EmergentSynergy{
			Cards:         es.Cards,
			EffectPattern: es.EffectPattern,
			Tier:          es.Tier,
			AvgImpact:     es.AvgImpact,
		})
	}
	applyEmergentSynergyBoost(sp)
	// Power-tier routing runs AFTER emergent-synergy boost so the cEDH
	// ComboProximity multiplier amplifies any Huginn-discovered synergy
	// bump rather than getting masked by it.
	ApplyPowerTierRouting(sp)

	normalizeStrategyProfile(sp)
	return sp
}

// normalizeStrategyProfile clamps numeric fields to their documented
// valid ranges so downstream consumers see safe defaults regardless of
// what Freya emitted on the wire. Added in the freya-hat integration
// final-polish wave (2026-05-30) to formalize the implicit contracts
// between Freya output and hat consumption.
//
// Per the strategyFileJSON schema docstring:
//   - Bracket / PowerTier: clamped to [0, 5] (0 = unset sentinel)
//   - PowerTierConfidence / CommanderSynergy: clamped to [0, 1]
//   - InteractionAvgCMC: clamped >= 0
//   - CheapInteraction / ProtectedKeyPieces / UnprotectedKeyPieces:
//     clamped >= 0
//   - KeepableHandPct / PowerPercentile: clamped to [0, 100]
//
// Strings (Archetype, ManaBaseGrade, MaxRecursionDepth, MetaMatchups
// ratings, SynergyClusters Name/Theme) and string arrays are left
// untouched — invalid string values fall through to no-op in their
// consumers (validated case by case in the field's consumer site).
//
// No-op on nil sp (defensive). Idempotent — calling twice produces the
// same result as calling once (every operation is a clamp).
func normalizeStrategyProfile(sp *StrategyProfile) {
	if sp == nil {
		return
	}
	sp.Bracket = clampInt(sp.Bracket, 0, 5)
	sp.PowerTier = clampInt(sp.PowerTier, 0, 5)
	sp.PowerTierConfidence = clampFloat(sp.PowerTierConfidence, 0, 1)
	sp.CommanderSynergy = clampFloat(sp.CommanderSynergy, 0, 1)
	if sp.InteractionAvgCMC < 0 {
		sp.InteractionAvgCMC = 0
	}
	if sp.CheapInteraction < 0 {
		sp.CheapInteraction = 0
	}
	if sp.ProtectedKeyPieces < 0 {
		sp.ProtectedKeyPieces = 0
	}
	if sp.UnprotectedKeyPieces < 0 {
		sp.UnprotectedKeyPieces = 0
	}
	sp.KeepableHandPct = clampFloat(sp.KeepableHandPct, 0, 100)
	sp.PowerPercentile = clampInt(sp.PowerPercentile, 0, 100)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// applyEmergentSynergyBoost applies small ComboProximity weight bumps
// based on Huginn's emergent synergies. Tier 2 = +0.1, Tier 3 = +0.2.
// These are soft signals — they nudge the evaluator toward cards that
// co-trigger well, without overriding hard combo plans.
func applyEmergentSynergyBoost(sp *StrategyProfile) {
	if len(sp.EmergentSynergies) == 0 {
		return
	}
	boost := 0.0
	for _, es := range sp.EmergentSynergies {
		switch es.Tier {
		case 2:
			boost += 0.1
		case 3:
			boost += 0.2
		}
	}
	// Cap the total boost at 0.5 to prevent synergy stacking from
	// dominating the eval.
	if boost > 0.5 {
		boost = 0.5
	}
	if sp.Weights == nil {
		sp.Weights = &EvalWeights{}
		*sp.Weights = DefaultWeightsForArchetype(sp.Archetype)
	}
	sp.Weights.ComboProximity += boost
}

func buildStrategyProfile(fj *freyaJSON) *StrategyProfile {
	sp := &StrategyProfile{}

	// Archetype.
	if fj.Archetype != nil {
		sp.Archetype = strings.ToLower(fj.Archetype.Primary)
		sp.Bracket = fj.Archetype.Bracket
	}
	// Override from unified profile if available (more authoritative).
	if fj.FullProfile != nil {
		if fj.FullProfile.PrimaryArchetype != "" {
			sp.Archetype = strings.ToLower(fj.FullProfile.PrimaryArchetype)
		}
		if fj.FullProfile.Bracket > 0 {
			sp.Bracket = fj.FullProfile.Bracket
		}
		if fj.FullProfile.PowerTier > 0 {
			sp.PowerTier = fj.FullProfile.PowerTier
		}
		if fj.FullProfile.PowerTierConfidence > 0 {
			sp.PowerTierConfidence = fj.FullProfile.PowerTierConfidence
		}
		sp.GameplanSummary = fj.FullProfile.GameplanSummary
	}

	// Win lines -> ComboPieces.
	if fj.WinLines != nil {
		for _, wl := range fj.WinLines.Lines {
			if len(wl.Pieces) == 0 {
				continue
			}
			cp := ComboPlan{
				Pieces: wl.Pieces,
				Type:   wl.Type,
				Class:  wl.Class,
			}
			// Derive cast order: for 2-card combos, use tutor path hints
			// if available. Otherwise, use piece order as-is.
			if len(wl.TutorPaths) > 0 && len(wl.Pieces) == 2 {
				// Tutor paths tell us which piece the tutor finds — the
				// other piece should be cast first (it's the enabler).
				cp.CastOrder = deriveCastOrder(wl.Pieces, wl.TutorPaths)
			}
			if len(cp.CastOrder) == 0 {
				cp.CastOrder = append([]string{}, wl.Pieces...)
			}
			sp.ComboPieces = append(sp.ComboPieces, cp)
		}
	}

	// Tutor targets: all unique card names from combo pieces, ordered by
	// combo priority (infinite first, then determined, then finisher).
	seen := map[string]bool{}
	for _, cp := range sp.ComboPieces {
		for _, p := range cp.Pieces {
			if !seen[p] {
				seen[p] = true
				sp.TutorTargets = append(sp.TutorTargets, p)
			}
		}
	}

	// Value engine keys: cards from value chains that aren't already
	// tutor targets. Also captures the strongest recursion-depth signal
	// across chains — the _freya.json fallback path can't rely on
	// Freya's strategy.json max_recursion_depth field, so derive it
	// here.
	depthRank := map[string]int{"none": 0, "shallow": 1, "deep": 2, "infinite": 3}
	bestRank := 0
	for _, vc := range fj.ValueChains {
		for _, step := range vc.Steps {
			for _, card := range step.Cards {
				if !seen[card] {
					seen[card] = true
					sp.ValueEngineKeys = append(sp.ValueEngineKeys, card)
				}
			}
		}
		// Bridge cards are especially important for value chains.
		for _, card := range vc.BridgeCards {
			if !seen[card] {
				seen[card] = true
				sp.ValueEngineKeys = append(sp.ValueEngineKeys, card)
			}
		}
		if r, ok := depthRank[vc.RecursionDepth]; ok && r > bestRank {
			bestRank = r
			sp.MaxRecursionDepth = vc.RecursionDepth
		}
	}

	// Fallback-path GraveyardValue boost: when Freya didn't ship
	// eval_weights (legacy _freya.json), the strategy.json builder
	// would have folded recursion depth into Weights already. Apply
	// the equivalent boost here so the hat still benefits.
	applyRecursionDepthBoost(sp)
	// Power-tier routing on the _freya.json fallback path. Runs after
	// applyRecursionDepthBoost so the cEDH ComboProximity multiplier
	// applies on top of any GraveyardValue lift the legacy path
	// installed. No-op when PowerTier == 0 (legacy reports predating
	// the cEDH classifier).
	ApplyPowerTierRouting(sp)

	normalizeStrategyProfile(sp)
	return sp
}

// applyRecursionDepthBoost mirrors the GraveyardValue lift that Freya's
// ComputeEvalWeights applies when value-chain recursion depth is
// non-trivial. Only runs in the _freya.json fallback path — when
// strategy.json supplied pre-computed Weights, the boost is already
// baked in and we don't double-apply.
func applyRecursionDepthBoost(sp *StrategyProfile) {
	if sp.MaxRecursionDepth == "" {
		return
	}
	if sp.Weights != nil {
		return
	}
	base := DefaultWeightsForArchetype(sp.Archetype)
	switch sp.MaxRecursionDepth {
	case "infinite":
		base.GraveyardValue += 0.5
	case "deep":
		base.GraveyardValue += 0.25
	case "shallow":
		base.GraveyardValue += 0.1
	default:
		return
	}
	sp.Weights = &base
}

// deriveCastOrder uses tutor path information to determine which combo
// piece should be cast first. The piece that tutors FIND is typically
// the finisher; the other piece is the enabler (cast first).
func deriveCastOrder(pieces []string, paths []freyaTutorChain) []string {
	// Count how often each piece is the "finds" target — the most-found
	// piece is the finisher (cast second).
	findCount := map[string]int{}
	for _, tp := range paths {
		findCount[tp.Finds]++
	}

	// The piece that is found more often is the finisher.
	if findCount[pieces[0]] > findCount[pieces[1]] {
		// pieces[0] is the finisher, pieces[1] is the enabler
		return []string{pieces[1], pieces[0]}
	}
	if findCount[pieces[1]] > findCount[pieces[0]] {
		return []string{pieces[0], pieces[1]}
	}

	// Tie or no tutor data — return as-is.
	return nil
}
