package main

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Deck coaching — synthesize 3-5 prioritized actionable suggestions across
// mana base, interaction, ramp/draw, win-line redundancy, protection, cuts,
// and synergy signals. Thresholds flex with bracket goal so a B2 casual deck
// isn't lectured about cEDH tutor density.
// ---------------------------------------------------------------------------

// computeCoachingTips emits the prioritized suggestion list for the
// deck. Each internal rule emits a candidate tip with a Priority score
// (1-10); the final list sorts descending by Priority and truncates at 5.
// A well-tuned deck may return fewer than 3 tips — Coach intentionally
// does not pad to a minimum count because fabricated "stylistic"
// suggestions would just be noise.
func computeCoachingTips(dp *DeckProfile, report *FreyaReport) []CoachingTip {
	tips := []CoachingTip{}
	bracket := dp.Bracket
	if bracket == 0 {
		bracket = 3
	}
	arch := strings.ToLower(dp.PrimaryArchetype)

	// ── Mana base — highest impact ──
	switch dp.LandVerdict {
	case "too_few":
		gap := dp.RecommendedLands - dp.LandCount
		if gap < 1 {
			gap = 1
		}
		tips = append(tips, CoachingTip{
			Category: "manabase",
			Priority: 10,
			Title:    fmt.Sprintf("Add %d land(s) — the deck will stall too often", gap),
			Detail: fmt.Sprintf(
				"Land count %d is under the %d recommended for an avg-CMC %.1f curve. Mulligan rates and turn-by-turn mana stability suffer materially below that floor.",
				dp.LandCount, dp.RecommendedLands, dp.AvgCMC),
			Action: fmt.Sprintf(
				"Add %d lands. If you don't want more basics, lean utility (Reliquary Tower, Field of Ruin, Bojuka Bog) or shock/fetch in colors.", gap),
			Tags: []string{"manabase", "critical"},
		})
	case "too_many":
		excess := dp.LandCount - dp.RecommendedLands
		if excess < 1 {
			excess = 1
		}
		tips = append(tips, CoachingTip{
			Category: "manabase",
			Priority: 7,
			Title:    fmt.Sprintf("Trim %d land(s) for ramp or interaction", excess),
			Detail: fmt.Sprintf(
				"Land count %d is over the %d recommended — you'll flood late game with no action.",
				dp.LandCount, dp.RecommendedLands),
			Action: fmt.Sprintf(
				"Cut %d lands and replace with low-CMC ramp (Arcane Signet, talismans) or interaction.", excess),
			Tags: []string{"manabase"},
		})
	}

	if dp.ManaBaseGrade == "D" || dp.ManaBaseGrade == "F" {
		detail := fmt.Sprintf("Mana base grades %s — %d taplands and %d fetchlands across the colored sources.",
			dp.ManaBaseGrade, dp.TaplandCount, dp.FetchCount)
		if len(dp.ManaBaseNotes) > 0 {
			detail += " " + strings.Join(dp.ManaBaseNotes, " ")
		}
		tips = append(tips, CoachingTip{
			Category: "manabase",
			Priority: 8,
			Title:    fmt.Sprintf("Upgrade the %s-grade mana base", dp.ManaBaseGrade),
			Detail:   detail,
			Action:   "Cut tap-on-ETB lands first. Add a fetch + shock or check-dual pair for each affected color combo, plus a colored-source utility land if your colors support one.",
			Tags:     []string{"manabase"},
		})
	}

	// ── Interaction — bracket-scaled minimums ──
	interaction := report.RemovalCount
	if report.Roles != nil {
		interaction += report.Roles.RoleCounts[RoleCounterspell]
	}
	minInteraction := 8
	switch {
	case bracket >= 5:
		minInteraction = 12
	case bracket >= 4:
		minInteraction = 10
	}
	if interaction < minInteraction {
		gap := minInteraction - interaction
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: 8,
			Title:    fmt.Sprintf("Add %d more interaction spells", gap),
			Detail: fmt.Sprintf(
				"Currently %d removal+counterspells. A bracket-%d deck typically wants ≥%d to navigate a 4-player table.",
				interaction, bracket, minInteraction),
			Action: "Add cheap interaction at CMC ≤2: Swords to Plowshares, Generous Gift, Counterspell, Swan Song, An Offer You Can't Refuse.",
			Tags:   []string{"interaction"},
		})
	}

	if report.Roles != nil && report.Roles.RoleCounts[RoleBoardWipe] == 0 {
		prio := 7
		if bracket >= 4 {
			prio = 8
		}
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: prio,
			Title:    "Add at least one board wipe",
			Detail:   "Zero board wipes — the deck can't reset against token swarms, hexproof boards, or a runaway opponent.",
			Action:   "Add 1-2 wipes in colors: Toxic Deluge, Cyclonic Rift, Damnation, Wrath of God, Farewell, Blasphemous Act.",
			Tags:     []string{"interaction"},
		})
	}

	// ── Draw — bracket-scaled ──
	minDraw := 7
	switch {
	case bracket >= 5:
		minDraw = 11
	case bracket >= 4:
		minDraw = 9
	}
	if dp.DrawCount < minDraw {
		gap := minDraw - dp.DrawCount
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: 7,
			Title:    fmt.Sprintf("Add %d more draw sources", gap),
			Detail: fmt.Sprintf(
				"Currently %d draw sources. Bracket-%d decks typically need ≥%d to keep gas flowing past turn 6.",
				dp.DrawCount, bracket, minDraw),
			Action: "Add Rhystic Study / Mystic Remora / Esper Sentinel / Necropotence in color, or wheel effects (Wheel of Fortune, Windfall) for explosive refills.",
			Tags:   []string{"consistency"},
		})
	}

	// ── Ramp — bracket-scaled ──
	minRamp := 8
	if bracket >= 4 {
		minRamp = 10
	}
	if dp.RampCount < minRamp {
		gap := minRamp - dp.RampCount
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: 6,
			Title:    fmt.Sprintf("Add %d more ramp pieces", gap),
			Detail: fmt.Sprintf(
				"Currently %d ramp. Bracket-%d decks typically want ≥%d for consistent turn-3 plays.",
				dp.RampCount, bracket, minRamp),
			Action: "Add Arcane Signet, color-matched talismans/signets, Sol Ring if missing, or green dorks (Llanowar Elves / Birds of Paradise) if your curve supports them.",
			Tags:   []string{"ramp"},
		})
	}

	// ── Tutors (combo/storm only, bracket 4+) ──
	if (strings.Contains(arch, "combo") || strings.Contains(arch, "storm")) && bracket >= 4 {
		minTutors := 5
		if bracket >= 5 {
			minTutors = 8
		}
		if report.NonLandTutorCount < minTutors {
			gap := minTutors - report.NonLandTutorCount
			tips = append(tips, CoachingTip{
				Category: "add",
				Priority: 8,
				Title:    fmt.Sprintf("Add %d more tutors to assemble the combo", gap),
				Detail: fmt.Sprintf(
					"Combo deck at bracket %d typically wants ≥%d non-land tutors; currently %d.",
					bracket, minTutors, report.NonLandTutorCount),
				Action: "Add color-appropriate tutors: Demonic Tutor, Vampiric Tutor, Imperial Seal, Mystical Tutor, Worldly Tutor, Enlightened Tutor, Survival of the Fittest.",
				Tags:   []string{"combo", "tutors"},
			})
		}
	}

	// ── Win lines & redundancy ──
	if dp.WinLineCount <= 1 {
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: 7,
			Title:    "Add a backup win line",
			Detail:   "Only 1 detected win line — if it gets answered, the deck has no plan B.",
			Action:   "Add a redundant finisher: a second combo line, a self-contained wincon (Approach of the Second Sun, Triumph of the Hordes, Craterhoof Behemoth), or convert a value engine into a closer.",
			Tags:     []string{"consistency", "wincon"},
		})
	}

	if dp.SinglePointCount > 0 {
		tips = append(tips, CoachingTip{
			Category: "add",
			Priority: 6,
			Title:    fmt.Sprintf("Add redundancy for %d single point(s) of failure", dp.SinglePointCount),
			Detail:   "Some combo pieces have no functional duplicates — if removed, the line can't reassemble.",
			Action:   "For each single-point piece, add 1-2 functional copies or recursion (Eternal Witness, Regrowth, Unburial Rites).",
			Tags:     []string{"redundancy"},
		})
	}

	// ── Protection — high impact for combo / voltron ──
	if dp.UnprotectedKeyPieces > 0 {
		total := dp.ProtectedKeyPieces + dp.UnprotectedKeyPieces
		if total >= 3 && float64(dp.UnprotectedKeyPieces)/float64(total) >= 0.70 {
			prio := 6
			if strings.Contains(arch, "voltron") || strings.Contains(arch, "combo") {
				prio = 7
			}
			tips = append(tips, CoachingTip{
				Category: "add",
				Priority: prio,
				Title:    fmt.Sprintf("Protect your key pieces — %d of %d are exposed", dp.UnprotectedKeyPieces, total),
				Detail:   "Most combo/threat pieces have no built-in protection. A single Path to Exile / Pongify / Imprisoned in the Moon resets your plan.",
				Action:   "Add Lightning Greaves, Swiftfoot Boots, Heroic Intervention, Veil of Summer, Teferi's Protection, or counter backup.",
				Tags:     []string{"protection"},
			})
		}
	}

	// ── Cuts ──
	if len(dp.CuttableCards) >= 3 {
		names := make([]string, 0, 3)
		for i, c := range dp.CuttableCards {
			if i >= 3 {
				break
			}
			names = append(names, c.Name)
		}
		tips = append(tips, CoachingTip{
			Category: "cut",
			Priority: 5,
			Title:    fmt.Sprintf("Cut underperformers (start with %d)", len(names)),
			Detail:   fmt.Sprintf("These cards scored bottom-of-deck on synergy: %s.", strings.Join(names, ", ")),
			Action:   fmt.Sprintf("Cut %s and replace with adds prioritized above.", strings.Join(names, " / ")),
			Tags:     []string{"cuts"},
		})
	}

	// ── Commander synergy ──
	if dp.CommanderSynergy > 0 && dp.CommanderSynergy < 0.25 {
		themes := "the commander's themes"
		if len(dp.CommanderThemes) > 0 {
			themes = strings.Join(dp.CommanderThemes, " / ")
		}
		tips = append(tips, CoachingTip{
			Category: "ratio",
			Priority: 5,
			Title:    fmt.Sprintf("Lean harder into %s (only %.0f%% synergy)", themes, dp.CommanderSynergy*100),
			Detail:   "Most cards don't reinforce the commander's gameplan, so the commander reads as filler instead of an engine. 40%+ synergy is the typical rule of thumb.",
			Action:   "Cut generic goodstuff (Sol Ring tier excepted) for theme-aligned cards that trigger or amplify the commander.",
			Tags:     []string{"synergy"},
		})
	}

	// ── Sort + cap ──
	sort.SliceStable(tips, func(i, j int) bool {
		return tips[i].Priority > tips[j].Priority
	})
	if len(tips) > 5 {
		tips = tips[:5]
	}
	return tips
}
