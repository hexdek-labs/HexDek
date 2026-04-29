package main

import (
	"fmt"
	"math"
	"strings"
)

// DeckStatistics holds the Phase 1 statistics analysis output.
type DeckStatistics struct {
	// 1. Mana curve histogram (CMC 0–6 and 7+).
	ManaCurve       [8]int
	AvgCMC          float64
	AvgCMCWithLands float64

	// 2. Color pip demand by turn bracket.
	// Keys: "W","U","B","R","G"; values: [3]int for [T1-4, T5-8, T9+].
	PipDemandByBracket map[string][3]int

	// 3. Color production from lands.
	ColorSources map[string]int

	// 4. Demand vs supply gap warnings.
	ColorGaps []string

	// 5. Land count evaluation (Frank Karsten formula).
	LandCount        int
	NonlandCount     int
	RecommendedLands int
	LandVerdict      string // "ok", "too_few", "too_many"
	LandNote         string

	// 6. Ramp pieces.
	RampCount       int
	RampCards       []RampCard
	LandSearchCount int
	ManaDorkCount   int
	ManaRockCount   int

	// 7. Draw engine density.
	DrawSourceCount int
	DrawCards       []string
}

// RampCard classifies a single ramp source.
type RampCard struct {
	Name     string
	Category string // "land_search", "mana_dork", "mana_rock", "other"
}

// ComputeDeckStatistics runs all 7 stat modules over the quantity-aware
// card profiles.
func ComputeDeckStatistics(qtyProfiles []CardProfileQty) *DeckStatistics {
	s := &DeckStatistics{
		PipDemandByBracket: map[string][3]int{
			"W": {}, "U": {}, "B": {}, "R": {}, "G": {},
		},
		ColorSources: map[string]int{"W": 0, "U": 0, "B": 0, "R": 0, "G": 0},
	}

	totalCMCNoLands := 0
	totalCMCAll := 0
	nonlandCount := 0
	totalCards := 0

	// Deduplicate draw/ramp card names across quantities.
	seenRamp := map[string]bool{}
	seenDraw := map[string]bool{}

	for _, qp := range qtyProfiles {
		for q := 0; q < qp.Qty; q++ {
			totalCards++
			if qp.Profile.IsLand {
				s.LandCount++
				for _, c := range qp.Profile.LandColors {
					s.ColorSources[c]++
				}
				totalCMCAll += qp.Profile.CMC
			} else {
				nonlandCount++
				cmc := qp.Profile.CMC
				if cmc >= 7 {
					s.ManaCurve[7]++
				} else if cmc >= 0 {
					s.ManaCurve[cmc]++
				}
				totalCMCNoLands += cmc
				totalCMCAll += cmc
				addPipDemandByBracket(qp.Profile.ManaCost, cmc, s.PipDemandByBracket)
			}
		}

		// Ramp and draw: count once per card name, but multiply by qty
		// for total count metrics.
		if !qp.Profile.IsLand {
			rampCat := classifyRampCategory(qp.Profile)
			if rampCat != "" {
				s.RampCount += qp.Qty
				switch rampCat {
				case "land_search":
					s.LandSearchCount += qp.Qty
				case "mana_dork":
					s.ManaDorkCount += qp.Qty
				case "mana_rock":
					s.ManaRockCount += qp.Qty
				}
				if !seenRamp[qp.Profile.Name] {
					s.RampCards = append(s.RampCards, RampCard{
						Name:     qp.Profile.Name,
						Category: rampCat,
					})
					seenRamp[qp.Profile.Name] = true
				}
			}

			if producesCards(qp.Profile) {
				s.DrawSourceCount += qp.Qty
				if !seenDraw[qp.Profile.Name] {
					s.DrawCards = append(s.DrawCards, qp.Profile.Name)
					seenDraw[qp.Profile.Name] = true
				}
			}
		}
	}

	s.NonlandCount = nonlandCount
	if nonlandCount > 0 {
		s.AvgCMC = float64(totalCMCNoLands) / float64(nonlandCount)
	}
	if totalCards > 0 {
		s.AvgCMCWithLands = float64(totalCMCAll) / float64(totalCards)
	}

	computeColorGaps(s)
	computeLandEvaluation(s)

	return s
}

// ---------------------------------------------------------------------------
// 2. Color pip demand by turn bracket
// ---------------------------------------------------------------------------

func addPipDemandByBracket(manaCost string, cmc int, demand map[string][3]int) {
	bracket := cmcToBracket(cmc)
	for _, c := range manaCost {
		var color string
		switch c {
		case 'W':
			color = "W"
		case 'U':
			color = "U"
		case 'B':
			color = "B"
		case 'R':
			color = "R"
		case 'G':
			color = "G"
		default:
			continue
		}
		arr := demand[color]
		arr[bracket]++
		demand[color] = arr
	}
}

func cmcToBracket(cmc int) int {
	if cmc <= 4 {
		return 0
	}
	if cmc <= 8 {
		return 1
	}
	return 2
}

// ---------------------------------------------------------------------------
// 4. Demand vs supply gap
// ---------------------------------------------------------------------------

func computeColorGaps(s *DeckStatistics) {
	totalDemand := 0
	totalSupply := 0
	for _, color := range []string{"W", "U", "B", "R", "G"} {
		arr := s.PipDemandByBracket[color]
		totalDemand += arr[0] + arr[1] + arr[2]
		totalSupply += s.ColorSources[color]
	}
	if totalDemand == 0 || totalSupply == 0 {
		return
	}

	for _, color := range []string{"W", "U", "B", "R", "G"} {
		arr := s.PipDemandByBracket[color]
		demand := arr[0] + arr[1] + arr[2]
		if demand == 0 {
			continue
		}
		supply := s.ColorSources[color]

		if supply == 0 {
			s.ColorGaps = append(s.ColorGaps,
				fmt.Sprintf("%s: %d pips demanded, 0 sources — no supply", color, demand))
			continue
		}

		demandPct := float64(demand) / float64(totalDemand) * 100
		supplyPct := float64(supply) / float64(totalSupply) * 100

		if demandPct-supplyPct > 20 {
			s.ColorGaps = append(s.ColorGaps,
				fmt.Sprintf("%s: %.0f%% of pip demand but only %.0f%% of sources (%d pips, %d sources)",
					color, demandPct, supplyPct, demand, supply))
		}
	}
}

// ---------------------------------------------------------------------------
// 5. Land count evaluation (Frank Karsten formula)
// ---------------------------------------------------------------------------

func computeLandEvaluation(s *DeckStatistics) {
	recommended := int(math.Round(30 + s.AvgCMC*2))
	s.RecommendedLands = recommended

	diff := s.LandCount - recommended
	switch {
	case diff < -3:
		s.LandVerdict = "too_few"
		s.LandNote = fmt.Sprintf("Running %d lands with avg CMC %.1f — Karsten recommends ~%d. Consider adding %d lands or more ramp.",
			s.LandCount, s.AvgCMC, recommended, -diff)
	case diff > 4:
		s.LandVerdict = "too_many"
		s.LandNote = fmt.Sprintf("Running %d lands with avg CMC %.1f — Karsten recommends ~%d. Could cut %d lands for more spells.",
			s.LandCount, s.AvgCMC, recommended, diff)
	default:
		s.LandVerdict = "ok"
		s.LandNote = fmt.Sprintf("Running %d lands with avg CMC %.1f — Karsten recommends ~%d. On target.",
			s.LandCount, s.AvgCMC, recommended)
	}
}

// ---------------------------------------------------------------------------
// 6. Ramp piece classification
// ---------------------------------------------------------------------------

func classifyRampCategory(p CardProfile) string {
	if p.IsLand {
		return ""
	}

	tl := strings.ToLower(p.TypeLine)
	isCreature := strings.Contains(tl, "creature")
	isArtifact := strings.Contains(tl, "artifact")

	// Land search ramp (Rampant Growth, Cultivate, etc.)
	for _, eff := range p.Effects {
		if eff == "land_fetch" {
			return "land_search"
		}
	}

	// Mana production (rocks, dorks, rituals)
	for _, r := range p.Produces {
		if r == ResMana {
			if isCreature {
				return "mana_dork"
			}
			if isArtifact {
				return "mana_rock"
			}
			return "other"
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// 7. Draw engine detection
// ---------------------------------------------------------------------------

func producesCards(p CardProfile) bool {
	for _, r := range p.Produces {
		if r == ResCard {
			return true
		}
	}
	return false
}
