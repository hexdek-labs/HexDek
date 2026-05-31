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
func LoadStrategyFromFreya(deckPath string) *StrategyProfile {
	dir := filepath.Dir(deckPath)
	base := filepath.Base(deckPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// Prefer compact strategy format.
	stratPath := filepath.Join(dir, "freya", base+".strategy.json")
	if data, err := os.ReadFile(stratPath); err == nil {
		var sj strategyFileJSON
		if err := json.Unmarshal(data, &sj); err == nil {
			return buildFromStrategyJSON(&sj)
		}
	}

	// Fallback to full Freya JSON.
	freyaPath := filepath.Join(dir, "freya", base+"_freya.json")
	data, err := os.ReadFile(freyaPath)
	if err != nil {
		return nil
	}

	var fj freyaJSON
	if err := json.Unmarshal(data, &fj); err != nil {
		return nil
	}

	return buildStrategyProfile(&fj)
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

	return sp
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
