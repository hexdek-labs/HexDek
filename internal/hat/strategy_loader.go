package hat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Freya JSON mirror types (hat-local, cannot import cmd/mtgsquad-freya)
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
	Desc       string            `json:"description,omitempty"`
	TutorPaths []freyaTutorChain `json:"tutor_paths,omitempty"`
}

type freyaTutorChain struct {
	Tutor    string `json:"tutor"`
	Finds    string `json:"finds"`
	Delivery string `json:"delivery"`
}

type freyaValueChain struct {
	Name        string                `json:"name"`
	Steps       []freyaValueChainStep `json:"steps"`
	BridgeCards []string              `json:"bridge_cards,omitempty"`
}

type freyaValueChainStep struct {
	Cards []string `json:"cards"`
}

type freyaDeckProfile struct {
	PrimaryArchetype string `json:"primary_archetype"`
	Bracket          int    `json:"bracket"`
	GameplanSummary  string `json:"gameplan_summary"`
}

// ---------------------------------------------------------------------------
// Strategy JSON mirror types (compact machine format from Freya)
// ---------------------------------------------------------------------------

type strategyFileJSON struct {
	Archetype       string              `json:"archetype"`
	Bracket         int                 `json:"bracket"`
	GameplanSummary string              `json:"gameplan_summary"`
	WinLines        []freyaWinLine      `json:"win_lines,omitempty"`
	ValueEngineKeys []string            `json:"value_engine_keys,omitempty"`
	TutorTargets    []string            `json:"tutor_targets,omitempty"`
	Weights         *freyaEvalWeights   `json:"eval_weights,omitempty"`
	CardRoles       map[string]string   `json:"card_roles,omitempty"`
	FinisherCards   []string            `json:"finisher_cards,omitempty"`
	ColorDemand     map[string]int      `json:"color_demand,omitempty"`
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
		Archetype:       sj.Archetype,
		Bracket:         sj.Bracket,
		GameplanSummary: sj.GameplanSummary,
		TutorTargets:    sj.TutorTargets,
		ValueEngineKeys: sj.ValueEngineKeys,
		CardRoles:       sj.CardRoles,
		FinisherCards:   sj.FinisherCards,
		ColorDemand:     sj.ColorDemand,
	}

	for _, wl := range sj.WinLines {
		if len(wl.Pieces) == 0 {
			continue
		}
		cp := ComboPlan{
			Pieces: wl.Pieces,
			Type:   wl.Type,
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
		sp.Weights = &EvalWeights{
			BoardPresence:     sj.Weights.BoardPresence,
			CardAdvantage:     sj.Weights.CardAdvantage,
			ManaAdvantage:     sj.Weights.ManaAdvantage,
			LifeResource:      sj.Weights.LifeResource,
			ComboProximity:    sj.Weights.ComboProximity,
			ThreatExposure:    sj.Weights.ThreatExposure,
			CommanderProgress: sj.Weights.CommanderProgress,
			GraveyardValue:    sj.Weights.GraveyardValue,
		}
	}

	return sp
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
	// tutor targets.
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
	}

	return sp
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
