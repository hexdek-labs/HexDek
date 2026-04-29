package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ---------------------------------------------------------------------------
// Report rendering
// ---------------------------------------------------------------------------

// PrintReport writes the Freya report in the specified format.
func PrintReport(w io.Writer, report *FreyaReport, format string) {
	switch format {
	case "json":
		printJSON(w, report)
	case "markdown":
		printMarkdown(w, report)
	default:
		printText(w, report)
	}
}

// ---------------------------------------------------------------------------
// Text output (default, with color indicators)
// ---------------------------------------------------------------------------

func printText(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "FREYA -- Combo & Synergy Analysis\n")
	fmt.Fprintf(w, "==================================\n")
	if r.DeckPath != "" {
		fmt.Fprintf(w, "Deck: %s\n", r.DeckPath)
	} else {
		fmt.Fprintf(w, "Deck: %s\n", r.DeckName)
	}
	fmt.Fprintf(w, "Cards: %d\n", r.TotalCards)
	if r.Commander != "" {
		fmt.Fprintf(w, "Commander: %s\n", r.Commander)
	}
	fmt.Fprintf(w, "\n")

	// Legality validation (always first).
	printLegalityText(w, r.Legality)

	// True infinites (mandatory loops).
	fmt.Fprintf(w, "[RED] TRUE INFINITES -- mandatory loops (%d found)\n", len(r.TrueInfinites))
	if len(r.TrueInfinites) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.TrueInfinites {
		prefix := "\xf0\x9f\x94\x8d" // magnifying glass for heuristic
		if c.Confirmed {
			prefix = "\xe2\x9c\x85" // checkmark for confirmed
		}
		fmt.Fprintf(w, "  %s %s -- mandatory trigger loop\n", prefix, strings.Join(c.Cards, " + "))
		// Split description on " | " to show outlets on a separate line.
		parts := strings.SplitN(c.Description, " | ", 2)
		fmt.Fprintf(w, "    %s\n", parts[0])
		if len(parts) > 1 {
			if strings.HasPrefix(parts[1], "OUTLETS IN DECK:") {
				fmt.Fprintf(w, "    %s\n", parts[1])
			} else if strings.HasPrefix(parts[1], "OUTLETS:") {
				fmt.Fprintf(w, "    OUTLETS IN DECK: %s\n", strings.TrimPrefix(parts[1], "OUTLETS: "))
			} else {
				fmt.Fprintf(w, "    !! %s\n", parts[1])
			}
		}
		if c.Resources != "" {
			fmt.Fprintf(w, "    Resources: %s\n", c.Resources)
		}
		if c.NonDeterministic {
			fmt.Fprintf(w, "    \xe2\x9a\xa0\xef\xb8\x8f  Non-deterministic: involves random selection\n")
		}
	}
	fmt.Fprintf(w, "\n")

	// Determined loops (player chooses count or has kill condition).
	fmt.Fprintf(w, "[GRN] DETERMINED LOOPS -- player chooses count (%d found)\n", len(r.Determined))
	if len(r.Determined) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.Determined {
		prefix := "\xf0\x9f\x94\x8d" // magnifying glass for heuristic
		if c.Confirmed {
			prefix = "\xe2\x9c\x85" // checkmark for confirmed
		}
		fmt.Fprintf(w, "  %s %s\n", prefix, strings.Join(c.Cards, " + "))
		// Split description on " | " to show outlets on a separate line.
		parts := strings.SplitN(c.Description, " | ", 2)
		fmt.Fprintf(w, "    %s\n", parts[0])
		if len(parts) > 1 {
			fmt.Fprintf(w, "    %s\n", parts[1])
		}
		if c.Resources != "" {
			fmt.Fprintf(w, "    Resources: %s\n", c.Resources)
		}
		if c.NonDeterministic {
			fmt.Fprintf(w, "    \xe2\x9a\xa0\xef\xb8\x8f  Non-deterministic: involves random selection\n")
		}
	}
	fmt.Fprintf(w, "\n")

	// Finishers.
	fmt.Fprintf(w, "[YLW] GAME FINISHERS (%d found)\n", len(r.Finishers))
	if len(r.Finishers) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.Finishers {
		fmt.Fprintf(w, "  * %s -- %s\n", strings.Join(c.Cards, " + "), c.Description)
	}
	fmt.Fprintf(w, "\n")

	// Synergies.
	fmt.Fprintf(w, "[BLU] SYNERGIES (%d found)\n", len(r.Synergies))
	if len(r.Synergies) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.Synergies {
		fmt.Fprintf(w, "  * %s\n", strings.Join(c.Cards, " + "))
		fmt.Fprintf(w, "    %s\n", c.Description)
	}
	fmt.Fprintf(w, "\n")

	// Combo potential (partial known combo matches).
	if len(r.ComboNotes) > 0 {
		fmt.Fprintf(w, "[CYN] COMBO POTENTIAL -- known pieces without full combo (%d noted)\n", len(r.ComboNotes))
		for _, note := range r.ComboNotes {
			fmt.Fprintf(w, "  - %s\n", note)
		}
		fmt.Fprintf(w, "\n")
	}

	// Mana curve.
	fmt.Fprintf(w, "MANA CURVE (avg %.1f -- %s)\n", r.AvgCMC, r.CurveShape)
	maxCount := 0
	for _, count := range r.ManaCurve {
		if count > maxCount {
			maxCount = count
		}
	}
	for i, count := range r.ManaCurve {
		label := fmt.Sprintf("  %d:", i)
		if i == 7 {
			label = "  7+:"
		}
		barLen := 0
		if maxCount > 0 {
			barLen = count * 30 / maxCount // scale to max 30 chars
		}
		bar := strings.Repeat("\u2588", barLen)
		fmt.Fprintf(w, "%-5s %-30s %d\n", label, bar, count)
	}
	fmt.Fprintf(w, "  Lands: %d  Nonlands: %d\n", r.LandCount, r.NonlandCount)
	fmt.Fprintf(w, "\n")

	// Color balance.
	totalDemand := 0
	totalSupply := 0
	for _, v := range r.ColorDemand {
		totalDemand += v
	}
	for _, v := range r.ColorSupply {
		totalSupply += v
	}
	if totalDemand > 0 || totalSupply > 0 {
		fmt.Fprintf(w, "COLOR BALANCE\n")
		colorNames := map[string]string{"W": "White", "U": "Blue", "B": "Black", "R": "Red", "G": "Green"}
		fmt.Fprintf(w, "  %-8s %6s %6s  %6s %6s  %s\n", "Color", "Pips", "Dem%", "Srcs", "Sup%", "Status")
		fmt.Fprintf(w, "  %-8s %6s %6s  %6s %6s  %s\n", "─────", "────", "────", "────", "────", "──────")
		for _, c := range []string{"W", "U", "B", "R", "G"} {
			demand := r.ColorDemand[c]
			supply := r.ColorSupply[c]
			dPct := 0.0
			sPct := 0.0
			if totalDemand > 0 {
				dPct = float64(demand) / float64(totalDemand) * 100
			}
			if totalSupply > 0 {
				sPct = float64(supply) / float64(totalSupply) * 100
			}
			status := "✅"
			if demand == 0 && supply == 0 {
				status = "—"
			} else if dPct-sPct > 5 {
				status = "⚠️ LOW"
			} else if sPct-dPct > 15 {
				status = "📈 HIGH"
			}
			if demand > 0 || supply > 0 {
				fmt.Fprintf(w, "  %-8s %6d %5.0f%%  %6d %5.0f%%  %s\n",
					colorNames[c], demand, dPct, supply, sPct, status)
			}
		}
		fmt.Fprintf(w, "  %-8s %6d %5s  %6d\n", "Total", totalDemand, "", totalSupply)
		if len(r.ColorMismatch) > 0 {
			fmt.Fprintf(w, "\n")
			for _, mismatch := range r.ColorMismatch {
				fmt.Fprintf(w, "  ⚠️  %s\n", mismatch)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	// Deck profile (Phase 5 unified output).
	printDeckProfileText(w, r)

	// Phase 1 statistics.
	printStatsText(w, r.Stats)

	// Phase 2 role tagging.
	printRolesText(w, r.Roles)

	// Phase 3 archetype classification.
	printArchetypeText(w, r.Archetype)

	// Phase 4 win line mapping.
	printWinLinesText(w, r.WinLines)

	// Value chains.
	if vcText := renderValueChainsText(r.ValueChains); vcText != "" {
		fmt.Fprintf(w, "%s", vcText)
	}
}

func printLegalityText(w io.Writer, lr *LegalityReport) {
	if lr == nil {
		return
	}

	fmt.Fprintf(w, "DECK LEGALITY\n")
	fmt.Fprintf(w, "=============\n")
	if lr.Valid {
		fmt.Fprintf(w, "  Status: LEGAL\n")
	} else {
		fmt.Fprintf(w, "  Status: ILLEGAL\n")
	}
	fmt.Fprintf(w, "\n")

	// Card count.
	if lr.CardCount.Valid {
		fmt.Fprintf(w, "  [OK]  Card count: %s\n", lr.CardCount.Message)
	} else {
		fmt.Fprintf(w, "  [!!]  Card count: %s\n", lr.CardCount.Message)
	}

	// Commander legality.
	if lr.CommanderOK.Valid {
		fmt.Fprintf(w, "  [OK]  Commander: %s\n", lr.CommanderOK.Message)
	} else {
		fmt.Fprintf(w, "  [!!]  Commander: %s\n", lr.CommanderOK.Message)
	}

	// Color identity.
	if lr.ColorID.Valid {
		fmt.Fprintf(w, "  [OK]  Color identity: all cards within [%s]\n", strings.Join(lr.ColorID.CommanderColors, ""))
	} else {
		fmt.Fprintf(w, "  [!!]  Color identity: %d violation(s)\n", len(lr.ColorID.Violations))
		for _, v := range lr.ColorID.Violations {
			fmt.Fprintf(w, "        - %s has [%s], allowed [%s]\n",
				v.CardName, strings.Join(v.CardColors, ""), strings.Join(v.AllowedColors, ""))
		}
	}

	// Singleton.
	if lr.Singleton.Valid {
		fmt.Fprintf(w, "  [OK]  Singleton: no duplicates\n")
	} else {
		fmt.Fprintf(w, "  [!!]  Singleton: %d violation(s)\n", len(lr.Singleton.Violations))
		for _, v := range lr.Singleton.Violations {
			fmt.Fprintf(w, "        - %s appears %d times\n", v.CardName, v.Count)
		}
	}

	// Banned cards.
	if lr.BannedCards.Valid {
		fmt.Fprintf(w, "  [OK]  Banned list: no banned cards\n")
	} else {
		fmt.Fprintf(w, "  [!!]  Banned list: %d banned card(s)\n", len(lr.BannedCards.BannedFound))
		for _, name := range lr.BannedCards.BannedFound {
			fmt.Fprintf(w, "        - %s\n", name)
		}
	}

	// Warnings.
	if len(lr.Warnings) > 0 {
		fmt.Fprintf(w, "\n")
		for _, w2 := range lr.Warnings {
			fmt.Fprintf(w, "  NOTE: %s\n", w2)
		}
	}
	fmt.Fprintf(w, "\n")
}

func printLegalityMarkdown(w io.Writer, lr *LegalityReport) {
	if lr == nil {
		return
	}

	fmt.Fprintf(w, "## Deck Legality\n\n")
	if lr.Valid {
		fmt.Fprintf(w, "**Status:** LEGAL\n\n")
	} else {
		fmt.Fprintf(w, "**Status:** ILLEGAL\n\n")
	}

	fmt.Fprintf(w, "| Check | Result | Details |\n")
	fmt.Fprintf(w, "|-------|--------|---------|\n")

	// Card count.
	if lr.CardCount.Valid {
		fmt.Fprintf(w, "| Card Count | OK | %s |\n", lr.CardCount.Message)
	} else {
		fmt.Fprintf(w, "| Card Count | FAIL | %s |\n", lr.CardCount.Message)
	}

	// Commander.
	if lr.CommanderOK.Valid {
		fmt.Fprintf(w, "| Commander | OK | %s |\n", lr.CommanderOK.Message)
	} else {
		fmt.Fprintf(w, "| Commander | FAIL | %s |\n", lr.CommanderOK.Message)
	}

	// Color identity.
	if lr.ColorID.Valid {
		fmt.Fprintf(w, "| Color Identity | OK | all cards within [%s] |\n", strings.Join(lr.ColorID.CommanderColors, ""))
	} else {
		fmt.Fprintf(w, "| Color Identity | FAIL | %d violation(s) |\n", len(lr.ColorID.Violations))
	}

	// Singleton.
	if lr.Singleton.Valid {
		fmt.Fprintf(w, "| Singleton | OK | no duplicates |\n")
	} else {
		fmt.Fprintf(w, "| Singleton | FAIL | %d violation(s) |\n", len(lr.Singleton.Violations))
	}

	// Banned.
	if lr.BannedCards.Valid {
		fmt.Fprintf(w, "| Banned List | OK | no banned cards |\n")
	} else {
		fmt.Fprintf(w, "| Banned List | FAIL | %d banned card(s) |\n", len(lr.BannedCards.BannedFound))
	}
	fmt.Fprintf(w, "\n")

	// Detail sections for violations.
	if !lr.ColorID.Valid && len(lr.ColorID.Violations) > 0 {
		fmt.Fprintf(w, "### Color Identity Violations\n\n")
		for _, v := range lr.ColorID.Violations {
			fmt.Fprintf(w, "- **%s** has [%s], allowed [%s]\n",
				v.CardName, strings.Join(v.CardColors, ""), strings.Join(v.AllowedColors, ""))
		}
		fmt.Fprintf(w, "\n")
	}

	if !lr.Singleton.Valid && len(lr.Singleton.Violations) > 0 {
		fmt.Fprintf(w, "### Singleton Violations\n\n")
		for _, v := range lr.Singleton.Violations {
			fmt.Fprintf(w, "- **%s** appears %d times\n", v.CardName, v.Count)
		}
		fmt.Fprintf(w, "\n")
	}

	if !lr.BannedCards.Valid && len(lr.BannedCards.BannedFound) > 0 {
		fmt.Fprintf(w, "### Banned Cards\n\n")
		for _, name := range lr.BannedCards.BannedFound {
			fmt.Fprintf(w, "- **%s**\n", name)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(lr.Warnings) > 0 {
		for _, w2 := range lr.Warnings {
			fmt.Fprintf(w, "> **Note:** %s\n\n", w2)
		}
	}
}

func printStatsText(w io.Writer, s *DeckStatistics) {
	if s == nil {
		return
	}

	fmt.Fprintf(w, "STATISTICS\n")
	fmt.Fprintf(w, "==========\n\n")

	// Pip demand by turn bracket.
	fmt.Fprintf(w, "COLOR PIP DEMAND BY TURN BRACKET\n")
	fmt.Fprintf(w, "  %-6s %6s %6s %6s %6s\n", "Color", "T1-4", "T5-8", "T9+", "Total")
	fmt.Fprintf(w, "  %-6s %6s %6s %6s %6s\n", "─────", "────", "────", "───", "─────")
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		arr := s.PipDemandByBracket[c]
		total := arr[0] + arr[1] + arr[2]
		if total == 0 {
			continue
		}
		fmt.Fprintf(w, "  %-6s %6d %6d %6d %6d\n", c, arr[0], arr[1], arr[2], total)
	}
	fmt.Fprintf(w, "\n")

	// Demand vs supply gap.
	if len(s.ColorGaps) > 0 {
		fmt.Fprintf(w, "DEMAND vs SUPPLY GAPS\n")
		for _, gap := range s.ColorGaps {
			fmt.Fprintf(w, "  ⚠️  %s\n", gap)
		}
		fmt.Fprintf(w, "\n")
	}

	// Land count evaluation.
	fmt.Fprintf(w, "LAND COUNT EVALUATION\n")
	verdict := "✅"
	if s.LandVerdict == "too_few" {
		verdict = "⚠️ TOO FEW"
	} else if s.LandVerdict == "too_many" {
		verdict = "📈 TOO MANY"
	}
	fmt.Fprintf(w, "  %s %s\n\n", verdict, s.LandNote)

	// Ramp pieces.
	fmt.Fprintf(w, "RAMP SOURCES (%d total)\n", s.RampCount)
	if s.RampCount > 0 {
		fmt.Fprintf(w, "  Land search: %d  |  Mana dorks: %d  |  Mana rocks: %d  |  Other: %d\n",
			s.LandSearchCount, s.ManaDorkCount, s.ManaRockCount,
			s.RampCount-s.LandSearchCount-s.ManaDorkCount-s.ManaRockCount)
		for _, rc := range s.RampCards {
			fmt.Fprintf(w, "  - %s [%s]\n", rc.Name, rc.Category)
		}
	} else {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	fmt.Fprintf(w, "\n")

	// Draw sources.
	fmt.Fprintf(w, "DRAW SOURCES (%d total)\n", s.DrawSourceCount)
	if s.DrawSourceCount > 0 {
		for _, name := range s.DrawCards {
			fmt.Fprintf(w, "  - %s\n", name)
		}
	} else {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	fmt.Fprintf(w, "\n")
}

func printWinLinesText(w io.Writer, wla *WinLineAnalysis) {
	if wla == nil {
		return
	}

	fmt.Fprintf(w, "WIN LINES\n")
	fmt.Fprintf(w, "=========\n\n")

	if len(wla.WinLines) == 0 {
		fmt.Fprintf(w, "  (no win lines detected)\n\n")
		return
	}

	for i, wl := range wla.WinLines {
		label := strings.Join(wl.Pieces, " + ")
		fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, strings.ToUpper(wl.Type), label)
		if wl.Desc != "" {
			fmt.Fprintf(w, "     %s\n", wl.Desc)
		}
		if len(wl.TutorPaths) > 0 {
			seen := map[string]bool{}
			fmt.Fprintf(w, "     Tutor paths:\n")
			for _, tp := range wl.TutorPaths {
				key := tp.Tutor + "→" + tp.Finds
				if seen[key] {
					continue
				}
				seen[key] = true
				fmt.Fprintf(w, "       %s → %s (to %s)\n", tp.Tutor, tp.Finds, tp.Delivery)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	if len(wla.BackupPlans) > 0 {
		fmt.Fprintf(w, "BACKUP PLANS\n")
		for _, bp := range wla.BackupPlans {
			fmt.Fprintf(w, "  - %s\n", bp)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(wla.SinglePoints) > 0 {
		fmt.Fprintf(w, "SINGLE POINTS OF FAILURE\n")
		for _, sp := range wla.SinglePoints {
			fmt.Fprintf(w, "  ⚠️  %s\n", sp)
		}
		fmt.Fprintf(w, "\n")
	}

	if len(wla.RedundancyMap) > 0 {
		fmt.Fprintf(w, "REDUNDANCY\n")
		roles := []string{"win_condition", "sacrifice_outlet", "tutor", "board_wipe", "draw_engine", "mana_source"}
		for _, role := range roles {
			count := wla.RedundancyMap[role]
			if count > 0 {
				fmt.Fprintf(w, "  %-20s %d cards\n", role, count)
			}
		}
		fmt.Fprintf(w, "\n")
	}
}

func printArchetypeText(w io.Writer, ac *ArchetypeClassification) {
	if ac == nil {
		return
	}

	fmt.Fprintf(w, "ARCHETYPE CLASSIFICATION\n")
	fmt.Fprintf(w, "========================\n\n")

	conf := ac.PrimaryConfidence * 100
	fmt.Fprintf(w, "  Primary:    %s (%.0f%% confidence)\n", ac.Primary, conf)
	if ac.Secondary != "" {
		fmt.Fprintf(w, "  Secondary:  %s\n", ac.Secondary)
	}
	fmt.Fprintf(w, "  Bracket:    %d/5 — %s\n", ac.Bracket, ac.BracketLabel)
	fmt.Fprintf(w, "\n")

	if len(ac.Signals) > 0 {
		fmt.Fprintf(w, "  Signals:\n")
		for _, s := range ac.Signals {
			fmt.Fprintf(w, "    - %s\n", s)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "  Intent: %s\n\n", ac.Intent)
}

func printRolesText(w io.Writer, ra *RoleAnalysis) {
	if ra == nil {
		return
	}

	fmt.Fprintf(w, "ROLE DISTRIBUTION\n")
	fmt.Fprintf(w, "=================\n\n")

	totalCards := ra.TotalCards

	fmt.Fprintf(w, "  %-14s %5s %6s\n", "Role", "Count", "Pct")
	fmt.Fprintf(w, "  %-14s %5s %6s\n", "──────────────", "─────", "──────")
	for _, role := range AllRoles {
		count := ra.RoleCounts[role]
		if count == 0 {
			continue
		}
		pct := 0.0
		if totalCards > 0 {
			pct = float64(count) / float64(totalCards) * 100
		}
		fmt.Fprintf(w, "  %-14s %5d %5.0f%%\n", role, count, pct)
	}
	fmt.Fprintf(w, "\n")

	if len(ra.Warnings) > 0 {
		fmt.Fprintf(w, "ROLE BALANCE WARNINGS\n")
		for _, w2 := range ra.Warnings {
			fmt.Fprintf(w, "  ⚠️  %s\n", w2)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "ROLE ASSIGNMENTS\n")
	for _, a := range ra.Assignments {
		tags := make([]string, len(a.Roles))
		for i, r := range a.Roles {
			tags[i] = string(r)
		}
		fmt.Fprintf(w, "  %-35s %s\n", a.Name, strings.Join(tags, ", "))
	}
	fmt.Fprintf(w, "\n")
}

func printDeckProfileText(w io.Writer, r *FreyaReport) {
	dp := r.Profile
	if dp == nil {
		fmt.Fprintf(w, "DECK PROFILE\n")
		fmt.Fprintf(w, "  Tutors:    %d cards\n", r.TutorCount)
		fmt.Fprintf(w, "  Removal:   %d cards\n", r.RemovalCount)
		fmt.Fprintf(w, "  Outlets:   %d sacrifice outlets\n", r.OutletCount)
		fmt.Fprintf(w, "  Win Cons:  %d win conditions\n", r.WinConCount)
		fmt.Fprintf(w, "\n")
		return
	}

	fmt.Fprintf(w, "DECK PROFILE\n")
	fmt.Fprintf(w, "============\n")
	fmt.Fprintf(w, "  Commander:  %s\n", dp.Commander)
	if len(dp.ColorIdentity) > 0 {
		fmt.Fprintf(w, "  Colors:     %s\n", strings.Join(dp.ColorIdentity, ""))
	}
	fmt.Fprintf(w, "  Archetype:  %s", dp.PrimaryArchetype)
	if dp.SecondaryArchetype != "" {
		fmt.Fprintf(w, " / %s", dp.SecondaryArchetype)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Bracket:    %d/5 (%s)\n", dp.Bracket, dp.BracketLabel)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Gameplan:   %s\n", dp.GameplanSummary)
	fmt.Fprintf(w, "\n")

	if len(dp.Strengths) > 0 {
		fmt.Fprintf(w, "  Strengths:\n")
		for _, s := range dp.Strengths {
			fmt.Fprintf(w, "    + %s\n", s)
		}
	}
	if len(dp.Weaknesses) > 0 {
		fmt.Fprintf(w, "  Weaknesses:\n")
		for _, w2 := range dp.Weaknesses {
			fmt.Fprintf(w, "    - %s\n", w2)
		}
	}
	fmt.Fprintf(w, "\n")
}

// ---------------------------------------------------------------------------
// Markdown output
// ---------------------------------------------------------------------------

func printMarkdown(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "# FREYA -- Combo & Synergy Analysis\n\n")
	if r.DeckPath != "" {
		fmt.Fprintf(w, "**Deck:** `%s`\n\n", r.DeckPath)
	} else {
		fmt.Fprintf(w, "**Deck:** %s\n\n", r.DeckName)
	}
	fmt.Fprintf(w, "**Cards:** %d\n\n", r.TotalCards)
	if r.Commander != "" {
		fmt.Fprintf(w, "**Commander:** %s\n\n", r.Commander)
	}

	// Legality validation (always first).
	printLegalityMarkdown(w, r.Legality)

	fmt.Fprintf(w, "## True Infinites -- Mandatory Loops (%d)\n\n", len(r.TrueInfinites))
	for _, c := range r.TrueInfinites {
		prefix := "\xf0\x9f\x94\x8d"
		if c.Confirmed {
			prefix = "\xe2\x9c\x85"
		}
		fmt.Fprintf(w, "- %s **%s** -- mandatory trigger loop\n", prefix, strings.Join(c.Cards, " + "))
		parts := strings.SplitN(c.Description, " | ", 2)
		fmt.Fprintf(w, "  - %s\n", parts[0])
		if len(parts) > 1 {
			if strings.HasPrefix(parts[1], "OUTLETS IN DECK:") {
				fmt.Fprintf(w, "  - **%s**\n", parts[1])
			} else if strings.HasPrefix(parts[1], "OUTLETS:") {
				fmt.Fprintf(w, "  - **OUTLETS IN DECK:** %s\n", strings.TrimPrefix(parts[1], "OUTLETS: "))
			} else {
				fmt.Fprintf(w, "  - **%s**\n", parts[1])
			}
		}
		if c.Resources != "" {
			fmt.Fprintf(w, "  - Resources: `%s`\n", c.Resources)
		}
		if c.NonDeterministic {
			fmt.Fprintf(w, "  - **Non-deterministic:** involves random selection\n")
		}
	}
	if len(r.TrueInfinites) == 0 {
		fmt.Fprintf(w, "_None detected._\n")
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "## Determined Loops (%d)\n\n", len(r.Determined))
	for _, c := range r.Determined {
		prefix := "\xf0\x9f\x94\x8d"
		if c.Confirmed {
			prefix = "\xe2\x9c\x85"
		}
		fmt.Fprintf(w, "- %s **%s**\n", prefix, strings.Join(c.Cards, " + "))
		parts := strings.SplitN(c.Description, " | ", 2)
		fmt.Fprintf(w, "  - %s\n", parts[0])
		if len(parts) > 1 {
			fmt.Fprintf(w, "  - **%s**\n", parts[1])
		}
		if c.Resources != "" {
			fmt.Fprintf(w, "  - Resources: `%s`\n", c.Resources)
		}
		if c.NonDeterministic {
			fmt.Fprintf(w, "  - **Non-deterministic:** involves random selection\n")
		}
	}
	if len(r.Determined) == 0 {
		fmt.Fprintf(w, "_None detected._\n")
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "## Game Finishers (%d)\n\n", len(r.Finishers))
	for _, c := range r.Finishers {
		fmt.Fprintf(w, "- **%s** -- %s\n", strings.Join(c.Cards, " + "), c.Description)
	}
	if len(r.Finishers) == 0 {
		fmt.Fprintf(w, "_None detected._\n")
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "## Synergies (%d)\n\n", len(r.Synergies))
	for _, c := range r.Synergies {
		fmt.Fprintf(w, "- **%s** -- %s\n", strings.Join(c.Cards, " + "), c.Description)
	}
	if len(r.Synergies) == 0 {
		fmt.Fprintf(w, "_None detected._\n")
	}
	fmt.Fprintf(w, "\n")

	// Combo potential.
	if len(r.ComboNotes) > 0 {
		fmt.Fprintf(w, "## Combo Potential (%d)\n\n", len(r.ComboNotes))
		for _, note := range r.ComboNotes {
			fmt.Fprintf(w, "- %s\n", note)
		}
		fmt.Fprintf(w, "\n")
	}

	// Mana curve.
	fmt.Fprintf(w, "## Mana Curve (avg %.1f -- %s)\n\n", r.AvgCMC, r.CurveShape)
	fmt.Fprintf(w, "```\n")
	maxCountMD := 0
	for _, count := range r.ManaCurve {
		if count > maxCountMD {
			maxCountMD = count
		}
	}
	for i, count := range r.ManaCurve {
		label := fmt.Sprintf("%d:", i)
		if i == 7 {
			label = "7+:"
		}
		barLen := 0
		if maxCountMD > 0 {
			barLen = count * 30 / maxCountMD
		}
		bar := strings.Repeat("\u2588", barLen)
		fmt.Fprintf(w, "%-4s %-30s %d\n", label, bar, count)
	}
	fmt.Fprintf(w, "```\n\n")
	fmt.Fprintf(w, "Lands: %d | Nonlands: %d\n\n", r.LandCount, r.NonlandCount)

	// Color balance.
	totalDemandMD := 0
	totalSupplyMD := 0
	for _, v := range r.ColorDemand {
		totalDemandMD += v
	}
	for _, v := range r.ColorSupply {
		totalSupplyMD += v
	}
	if totalDemandMD > 0 || totalSupplyMD > 0 {
		fmt.Fprintf(w, "## Color Balance\n\n")
		fmt.Fprintf(w, "| Color | Demand | Supply |\n")
		fmt.Fprintf(w, "|-------|--------|--------|\n")
		for _, c := range []string{"W", "U", "B", "R", "G"} {
			dPct := 0.0
			sPct := 0.0
			if totalDemandMD > 0 {
				dPct = float64(r.ColorDemand[c]) / float64(totalDemandMD) * 100
			}
			if totalSupplyMD > 0 {
				sPct = float64(r.ColorSupply[c]) / float64(totalSupplyMD) * 100
			}
			fmt.Fprintf(w, "| %s | %.0f%% | %.0f%% |\n", c, dPct, sPct)
		}
		fmt.Fprintf(w, "\n")
		for _, mismatch := range r.ColorMismatch {
			fmt.Fprintf(w, "> **Warning:** %s\n\n", mismatch)
		}
	}

	printDeckProfileMarkdown(w, r)

	printStatsMarkdown(w, r.Stats)

	printRolesMarkdown(w, r.Roles)

	printArchetypeMarkdown(w, r.Archetype)

	printWinLinesMarkdown(w, r.WinLines)

	if vcMd := renderValueChainsMarkdown(r.ValueChains); vcMd != "" {
		fmt.Fprintf(w, "%s", vcMd)
	}
}

func printDeckProfileMarkdown(w io.Writer, r *FreyaReport) {
	dp := r.Profile
	if dp == nil {
		return
	}

	fmt.Fprintf(w, "## Deck Profile\n\n")
	fmt.Fprintf(w, "**Commander:** %s", dp.Commander)
	if len(dp.ColorIdentity) > 0 {
		fmt.Fprintf(w, " (%s)", strings.Join(dp.ColorIdentity, ""))
	}
	fmt.Fprintf(w, "\n\n")
	fmt.Fprintf(w, "**Archetype:** %s", dp.PrimaryArchetype)
	if dp.SecondaryArchetype != "" {
		fmt.Fprintf(w, " / %s", dp.SecondaryArchetype)
	}
	fmt.Fprintf(w, "  \n")
	fmt.Fprintf(w, "**Bracket:** %d/5 (%s)\n\n", dp.Bracket, dp.BracketLabel)
	fmt.Fprintf(w, "> %s\n\n", dp.GameplanSummary)

	if len(dp.Strengths) > 0 {
		fmt.Fprintf(w, "**Strengths:**\n")
		for _, s := range dp.Strengths {
			fmt.Fprintf(w, "- %s\n", s)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(dp.Weaknesses) > 0 {
		fmt.Fprintf(w, "**Weaknesses:**\n")
		for _, w2 := range dp.Weaknesses {
			fmt.Fprintf(w, "- %s\n", w2)
		}
		fmt.Fprintf(w, "\n")
	}
}

func printStatsMarkdown(w io.Writer, s *DeckStatistics) {
	if s == nil {
		return
	}

	fmt.Fprintf(w, "## Statistics\n\n")

	// Pip demand by turn bracket.
	fmt.Fprintf(w, "### Color Pip Demand by Turn Bracket\n\n")
	fmt.Fprintf(w, "| Color | T1-4 | T5-8 | T9+ | Total |\n")
	fmt.Fprintf(w, "|-------|------|------|-----|-------|\n")
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		arr := s.PipDemandByBracket[c]
		total := arr[0] + arr[1] + arr[2]
		if total == 0 {
			continue
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d |\n", c, arr[0], arr[1], arr[2], total)
	}
	fmt.Fprintf(w, "\n")

	// Demand vs supply gap.
	if len(s.ColorGaps) > 0 {
		fmt.Fprintf(w, "### Demand vs Supply Gaps\n\n")
		for _, gap := range s.ColorGaps {
			fmt.Fprintf(w, "> **Warning:** %s\n\n", gap)
		}
	}

	// Land count evaluation.
	fmt.Fprintf(w, "### Land Count Evaluation\n\n")
	fmt.Fprintf(w, "%s\n\n", s.LandNote)

	// Ramp pieces.
	fmt.Fprintf(w, "### Ramp Sources (%d total)\n\n", s.RampCount)
	if s.RampCount > 0 {
		fmt.Fprintf(w, "| Category | Count |\n")
		fmt.Fprintf(w, "|----------|-------|\n")
		fmt.Fprintf(w, "| Land Search | %d |\n", s.LandSearchCount)
		fmt.Fprintf(w, "| Mana Dorks | %d |\n", s.ManaDorkCount)
		fmt.Fprintf(w, "| Mana Rocks | %d |\n", s.ManaRockCount)
		other := s.RampCount - s.LandSearchCount - s.ManaDorkCount - s.ManaRockCount
		if other > 0 {
			fmt.Fprintf(w, "| Other | %d |\n", other)
		}
		fmt.Fprintf(w, "\n")
		for _, rc := range s.RampCards {
			fmt.Fprintf(w, "- %s (%s)\n", rc.Name, rc.Category)
		}
		fmt.Fprintf(w, "\n")
	} else {
		fmt.Fprintf(w, "_None detected._\n\n")
	}

	// Draw sources.
	fmt.Fprintf(w, "### Draw Sources (%d total)\n\n", s.DrawSourceCount)
	if s.DrawSourceCount > 0 {
		for _, name := range s.DrawCards {
			fmt.Fprintf(w, "- %s\n", name)
		}
		fmt.Fprintf(w, "\n")
	} else {
		fmt.Fprintf(w, "_None detected._\n\n")
	}
}

func printWinLinesMarkdown(w io.Writer, wla *WinLineAnalysis) {
	if wla == nil || len(wla.WinLines) == 0 {
		return
	}

	fmt.Fprintf(w, "## Win Lines\n\n")

	for i, wl := range wla.WinLines {
		label := strings.Join(wl.Pieces, " + ")
		fmt.Fprintf(w, "### %d. %s — %s\n\n", i+1, label, wl.Type)
		if wl.Desc != "" {
			fmt.Fprintf(w, "%s\n\n", wl.Desc)
		}
		if len(wl.TutorPaths) > 0 {
			seen := map[string]bool{}
			fmt.Fprintf(w, "**Tutor paths:**\n")
			for _, tp := range wl.TutorPaths {
				key := tp.Tutor + "→" + tp.Finds
				if seen[key] {
					continue
				}
				seen[key] = true
				fmt.Fprintf(w, "- %s → %s (to %s)\n", tp.Tutor, tp.Finds, tp.Delivery)
			}
			fmt.Fprintf(w, "\n")
		}
	}

	if len(wla.SinglePoints) > 0 {
		fmt.Fprintf(w, "### Single Points of Failure\n\n")
		for _, sp := range wla.SinglePoints {
			fmt.Fprintf(w, "> **Warning:** %s\n\n", sp)
		}
	}

	if len(wla.RedundancyMap) > 0 {
		fmt.Fprintf(w, "### Redundancy\n\n")
		fmt.Fprintf(w, "| Role | Count |\n")
		fmt.Fprintf(w, "|------|-------|\n")
		roles := []string{"win_condition", "sacrifice_outlet", "tutor", "board_wipe", "draw_engine", "mana_source"}
		for _, role := range roles {
			count := wla.RedundancyMap[role]
			if count > 0 {
				fmt.Fprintf(w, "| %s | %d |\n", role, count)
			}
		}
		fmt.Fprintf(w, "\n")
	}
}

func printArchetypeMarkdown(w io.Writer, ac *ArchetypeClassification) {
	if ac == nil {
		return
	}

	fmt.Fprintf(w, "## Archetype Classification\n\n")

	conf := ac.PrimaryConfidence * 100
	fmt.Fprintf(w, "**Primary:** %s (%.0f%% confidence)\n\n", ac.Primary, conf)
	if ac.Secondary != "" {
		fmt.Fprintf(w, "**Secondary:** %s\n\n", ac.Secondary)
	}
	fmt.Fprintf(w, "**Bracket:** %d/5 — %s\n\n", ac.Bracket, ac.BracketLabel)

	if len(ac.Signals) > 0 {
		fmt.Fprintf(w, "**Signals:**\n")
		for _, s := range ac.Signals {
			fmt.Fprintf(w, "- %s\n", s)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "> %s\n\n", ac.Intent)
}

func printRolesMarkdown(w io.Writer, ra *RoleAnalysis) {
	if ra == nil {
		return
	}

	fmt.Fprintf(w, "## Role Distribution\n\n")

	totalCards := ra.TotalCards

	fmt.Fprintf(w, "| Role | Count | Pct |\n")
	fmt.Fprintf(w, "|------|-------|-----|\n")
	for _, role := range AllRoles {
		count := ra.RoleCounts[role]
		if count == 0 {
			continue
		}
		pct := 0.0
		if totalCards > 0 {
			pct = float64(count) / float64(totalCards) * 100
		}
		fmt.Fprintf(w, "| %s | %d | %.0f%% |\n", role, count, pct)
	}
	fmt.Fprintf(w, "\n")

	if len(ra.Warnings) > 0 {
		fmt.Fprintf(w, "### Role Balance Warnings\n\n")
		for _, w2 := range ra.Warnings {
			fmt.Fprintf(w, "> **Warning:** %s\n\n", w2)
		}
	}
}

// ---------------------------------------------------------------------------
// JSON output
// ---------------------------------------------------------------------------

type jsonReport struct {
	DeckName      string           `json:"deck_name"`
	DeckPath      string           `json:"deck_path,omitempty"`
	Commander     string           `json:"commander,omitempty"`
	TotalCards    int              `json:"total_cards"`
	Legality      *LegalityReport  `json:"legality,omitempty"`
	TrueInfinites []jsonCombo      `json:"true_infinites"`
	Determined    []jsonCombo   `json:"determined_loops"`
	Finishers     []jsonCombo   `json:"finishers"`
	Synergies     []jsonCombo   `json:"synergies"`
	ComboNotes    []string      `json:"combo_notes,omitempty"`
	ManaCurve     jsonManaCurve `json:"mana_curve"`
	ColorBalance  jsonColors    `json:"color_balance"`
	Profile       jsonProfile       `json:"deck_profile"`
	FullProfile   *jsonDeckProfile  `json:"unified_profile,omitempty"`
	Statistics    *jsonStats        `json:"statistics,omitempty"`
	Roles         *jsonRoles       `json:"roles,omitempty"`
	Archetype     *jsonArchetype   `json:"archetype,omitempty"`
	WinLines      *jsonWinLines    `json:"win_lines,omitempty"`
	ValueChains   []jsonValueChain `json:"value_chains,omitempty"`
}

type jsonManaCurve struct {
	Distribution [8]int  `json:"distribution"`
	AvgCMC       float64 `json:"avg_cmc"`
	CurveShape   string  `json:"curve_shape"`
	LandCount    int     `json:"land_count"`
	NonlandCount int     `json:"nonland_count"`
}

type jsonColors struct {
	Demand   map[string]int `json:"demand"`
	Supply   map[string]int `json:"supply"`
	Warnings []string       `json:"warnings,omitempty"`
}

type jsonCombo struct {
	Cards            []string `json:"cards"`
	LoopType         string   `json:"loop_type"`
	Resources        string   `json:"resources,omitempty"`
	Description      string   `json:"description"`
	Confirmed        bool     `json:"confirmed,omitempty"`
	NonDeterministic bool     `json:"non_deterministic,omitempty"`
}

type jsonProfile struct {
	Tutors  int `json:"tutors"`
	Removal int `json:"removal"`
	Outlets int `json:"sacrifice_outlets"`
	WinCons int `json:"win_conditions"`
}

type jsonDeckProfile struct {
	DeckName           string            `json:"deck_name"`
	Commander          string            `json:"commander"`
	ColorIdentity      []string          `json:"color_identity,omitempty"`
	CardCount          int               `json:"card_count"`
	AvgCMC             float64           `json:"avg_cmc"`
	LandCount          int               `json:"land_count"`
	RecommendedLands   int               `json:"recommended_lands"`
	LandVerdict        string            `json:"land_verdict"`
	RampCount          int               `json:"ramp_count"`
	DrawCount          int               `json:"draw_count"`
	TopRoles           []jsonRoleCount   `json:"top_roles,omitempty"`
	PrimaryArchetype   string            `json:"primary_archetype"`
	SecondaryArchetype string            `json:"secondary_archetype,omitempty"`
	Confidence         float64           `json:"archetype_confidence"`
	Bracket            int               `json:"bracket"`
	BracketLabel       string            `json:"bracket_label"`
	Intent             string            `json:"intent"`
	PrimaryWinLine     string            `json:"primary_win_line"`
	WinLineCount       int               `json:"win_line_count"`
	BackupCount        int               `json:"backup_count"`
	HasTutorAccess     bool              `json:"has_tutor_access"`
	SinglePointCount   int               `json:"single_point_count"`
	Strengths          []string          `json:"strengths,omitempty"`
	Weaknesses         []string          `json:"weaknesses,omitempty"`
	GameplanSummary    string            `json:"gameplan_summary"`
}

type jsonRoleCount struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

type jsonStats struct {
	AvgCMCWithLands    float64                `json:"avg_cmc_with_lands"`
	PipDemandByBracket map[string][3]int      `json:"pip_demand_by_bracket"`
	ColorSources       map[string]int         `json:"color_sources"`
	ColorGaps          []string               `json:"color_gaps,omitempty"`
	LandCount          int                    `json:"land_count"`
	RecommendedLands   int                    `json:"recommended_lands"`
	LandVerdict        string                 `json:"land_verdict"`
	LandNote           string                 `json:"land_note"`
	RampCount          int                    `json:"ramp_count"`
	LandSearchCount    int                    `json:"land_search_count"`
	ManaDorkCount      int                    `json:"mana_dork_count"`
	ManaRockCount      int                    `json:"mana_rock_count"`
	RampCards          []jsonRampCard         `json:"ramp_cards,omitempty"`
	DrawSourceCount    int                    `json:"draw_source_count"`
	DrawCards          []string               `json:"draw_cards,omitempty"`
}

type jsonRampCard struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

type jsonRoles struct {
	RoleCounts  map[string]int       `json:"role_counts"`
	Warnings    []string             `json:"warnings,omitempty"`
	Assignments []jsonRoleAssignment `json:"assignments"`
}

type jsonRoleAssignment struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

type jsonArchetype struct {
	Primary    string   `json:"primary"`
	Confidence float64  `json:"confidence"`
	Secondary  string   `json:"secondary,omitempty"`
	Bracket    int      `json:"bracket"`
	BracketLbl string   `json:"bracket_label"`
	Signals    []string `json:"signals,omitempty"`
	Intent     string   `json:"intent"`
}

type jsonWinLines struct {
	Lines        []jsonWinLine  `json:"lines"`
	BackupPlans  []string       `json:"backup_plans,omitempty"`
	SinglePoints []string       `json:"single_points_of_failure,omitempty"`
	Redundancy   map[string]int `json:"redundancy,omitempty"`
}

type jsonWinLine struct {
	Pieces     []string         `json:"pieces"`
	Type       string           `json:"type"`
	Desc       string           `json:"description,omitempty"`
	TutorPaths []jsonTutorChain `json:"tutor_paths,omitempty"`
}

type jsonTutorChain struct {
	Tutor    string `json:"tutor"`
	Finds    string `json:"finds"`
	Delivery string `json:"delivery"`
}

func printJSON(w io.Writer, r *FreyaReport) {
	demand := r.ColorDemand
	if demand == nil {
		demand = map[string]int{}
	}
	supply := r.ColorSupply
	if supply == nil {
		supply = map[string]int{}
	}
	jr := jsonReport{
		DeckName:      r.DeckName,
		DeckPath:      r.DeckPath,
		Commander:     r.Commander,
		TotalCards:    r.TotalCards,
		Legality:      r.Legality,
		TrueInfinites: comboSlice(r.TrueInfinites),
		Determined:    comboSlice(r.Determined),
		Finishers:     comboSlice(r.Finishers),
		Synergies:     comboSlice(r.Synergies),
		ComboNotes:    r.ComboNotes,
		ManaCurve: jsonManaCurve{
			Distribution: r.ManaCurve,
			AvgCMC:       r.AvgCMC,
			CurveShape:   r.CurveShape,
			LandCount:    r.LandCount,
			NonlandCount: r.NonlandCount,
		},
		ColorBalance: jsonColors{
			Demand:   demand,
			Supply:   supply,
			Warnings: r.ColorMismatch,
		},
		Profile: jsonProfile{
			Tutors:  r.TutorCount,
			Removal: r.RemovalCount,
			Outlets: r.OutletCount,
			WinCons: r.WinConCount,
		},
		FullProfile: buildJSONDeckProfile(r.Profile),
		Statistics:  buildJSONStats(r.Stats),
		Roles:       buildJSONRoles(r.Roles),
		Archetype:   buildJSONArchetype(r.Archetype),
		WinLines:    buildJSONWinLines(r.WinLines),
		ValueChains: buildJSONValueChains(r.ValueChains),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(jr)
}

func buildJSONStats(s *DeckStatistics) *jsonStats {
	if s == nil {
		return nil
	}
	ramps := make([]jsonRampCard, len(s.RampCards))
	for i, rc := range s.RampCards {
		ramps[i] = jsonRampCard{Name: rc.Name, Category: rc.Category}
	}
	return &jsonStats{
		AvgCMCWithLands:    s.AvgCMCWithLands,
		PipDemandByBracket: s.PipDemandByBracket,
		ColorSources:       s.ColorSources,
		ColorGaps:          s.ColorGaps,
		LandCount:          s.LandCount,
		RecommendedLands:   s.RecommendedLands,
		LandVerdict:        s.LandVerdict,
		LandNote:           s.LandNote,
		RampCount:          s.RampCount,
		LandSearchCount:    s.LandSearchCount,
		ManaDorkCount:      s.ManaDorkCount,
		ManaRockCount:      s.ManaRockCount,
		RampCards:          ramps,
		DrawSourceCount:    s.DrawSourceCount,
		DrawCards:          s.DrawCards,
	}
}

func buildJSONDeckProfile(dp *DeckProfile) *jsonDeckProfile {
	if dp == nil {
		return nil
	}
	var roles []jsonRoleCount
	for _, rc := range dp.TopRoles {
		roles = append(roles, jsonRoleCount{Role: string(rc.Role), Count: rc.Count})
	}
	return &jsonDeckProfile{
		DeckName:           dp.DeckName,
		Commander:          dp.Commander,
		ColorIdentity:      dp.ColorIdentity,
		CardCount:          dp.CardCount,
		AvgCMC:             dp.AvgCMC,
		LandCount:          dp.LandCount,
		RecommendedLands:   dp.RecommendedLands,
		LandVerdict:        dp.LandVerdict,
		RampCount:          dp.RampCount,
		DrawCount:          dp.DrawCount,
		TopRoles:           roles,
		PrimaryArchetype:   dp.PrimaryArchetype,
		SecondaryArchetype: dp.SecondaryArchetype,
		Confidence:         dp.ArchetypeConfidence,
		Bracket:            dp.Bracket,
		BracketLabel:       dp.BracketLabel,
		Intent:             dp.Intent,
		PrimaryWinLine:     dp.PrimaryWinLine,
		WinLineCount:       dp.WinLineCount,
		BackupCount:        dp.BackupCount,
		HasTutorAccess:     dp.HasTutorAccess,
		SinglePointCount:   dp.SinglePointCount,
		Strengths:          dp.Strengths,
		Weaknesses:         dp.Weaknesses,
		GameplanSummary:    dp.GameplanSummary,
	}
}

func buildJSONRoles(ra *RoleAnalysis) *jsonRoles {
	if ra == nil {
		return nil
	}
	counts := make(map[string]int, len(ra.RoleCounts))
	for role, count := range ra.RoleCounts {
		counts[string(role)] = count
	}
	assignments := make([]jsonRoleAssignment, len(ra.Assignments))
	for i, a := range ra.Assignments {
		roles := make([]string, len(a.Roles))
		for j, r := range a.Roles {
			roles[j] = string(r)
		}
		assignments[i] = jsonRoleAssignment{Name: a.Name, Roles: roles}
	}
	return &jsonRoles{
		RoleCounts:  counts,
		Warnings:    ra.Warnings,
		Assignments: assignments,
	}
}

func buildJSONArchetype(ac *ArchetypeClassification) *jsonArchetype {
	if ac == nil {
		return nil
	}
	return &jsonArchetype{
		Primary:    ac.Primary,
		Confidence: ac.PrimaryConfidence,
		Secondary:  ac.Secondary,
		Bracket:    ac.Bracket,
		BracketLbl: ac.BracketLabel,
		Signals:    ac.Signals,
		Intent:     ac.Intent,
	}
}

func buildJSONWinLines(wla *WinLineAnalysis) *jsonWinLines {
	if wla == nil {
		return nil
	}
	lines := make([]jsonWinLine, len(wla.WinLines))
	for i, wl := range wla.WinLines {
		seen := map[string]bool{}
		var paths []jsonTutorChain
		for _, tp := range wl.TutorPaths {
			key := tp.Tutor + "→" + tp.Finds
			if seen[key] {
				continue
			}
			seen[key] = true
			paths = append(paths, jsonTutorChain{
				Tutor:    tp.Tutor,
				Finds:    tp.Finds,
				Delivery: tp.Delivery,
			})
		}
		lines[i] = jsonWinLine{
			Pieces:     wl.Pieces,
			Type:       wl.Type,
			Desc:       wl.Desc,
			TutorPaths: paths,
		}
	}
	return &jsonWinLines{
		Lines:        lines,
		BackupPlans:  wla.BackupPlans,
		SinglePoints: wla.SinglePoints,
		Redundancy:   wla.RedundancyMap,
	}
}

func comboSlice(combos []ComboResult) []jsonCombo {
	if len(combos) == 0 {
		return []jsonCombo{}
	}
	out := make([]jsonCombo, len(combos))
	for i, c := range combos {
		out[i] = jsonCombo{
			Cards:            c.Cards,
			LoopType:         c.LoopType,
			Resources:        c.Resources,
			Description:      c.Description,
			Confirmed:        c.Confirmed,
			NonDeterministic: c.NonDeterministic,
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Summary for --all-decks mode
// ---------------------------------------------------------------------------

func PrintAllDecksSummary(w io.Writer, reports []*FreyaReport) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "FREYA -- Multi-Deck Summary\n")
	fmt.Fprintf(w, "============================\n")
	fmt.Fprintf(w, "Decks analyzed: %d\n\n", len(reports))

	fmt.Fprintf(w, "%-40s %5s %5s %5s %5s %5s %5s %5s %5s %5s %10s\n",
		"DECK", "CARDS", "INF", "DET", "FIN", "SYN", "TUT", "REM", "OUT", "AVG", "SHAPE")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 115))

	totalInf := 0
	totalDet := 0
	totalFin := 0
	totalSyn := 0

	for _, r := range reports {
		name := r.DeckName
		if len(name) > 38 {
			name = name[:38] + ".."
		}
		fmt.Fprintf(w, "%-40s %5d %5d %5d %5d %5d %5d %5d %5d %5.1f %10s\n",
			name,
			r.TotalCards,
			len(r.TrueInfinites),
			len(r.Determined),
			len(r.Finishers),
			len(r.Synergies),
			r.TutorCount,
			r.RemovalCount,
			r.OutletCount,
			r.AvgCMC,
			r.CurveShape,
		)
		totalInf += len(r.TrueInfinites)
		totalDet += len(r.Determined)
		totalFin += len(r.Finishers)
		totalSyn += len(r.Synergies)
	}

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 115))
	fmt.Fprintf(w, "%-40s %5s %5d %5d %5d %5d\n",
		"TOTALS", "", totalInf, totalDet, totalFin, totalSyn)
	fmt.Fprintf(w, "\n")
}
