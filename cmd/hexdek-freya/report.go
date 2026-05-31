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

// PrintReport writes the Freya report in the specified format. The
// text format respects the FocusMode package variable — when set to
// "focus", the under-25-line prioritized summary is emitted instead
// of the fixed-order full report. JSON and markdown ignore FocusMode.
func PrintReport(w io.Writer, report *FreyaReport, format string) {
	switch format {
	case "json":
		printJSON(w, report)
	case "markdown":
		printMarkdown(w, report)
	case "html":
		printHTML(w, report)
	default:
		if FocusMode == "focus" {
			printFocusText(w, report)
			return
		}
		printText(w, report)
	}
}

// FocusMode toggles the text renderer between "full" (default — fixed-
// order full report) and "focus" (under-25-line prioritized summary).
// Set once from main() based on the --mode flag; package-level rather
// than threading through every PrintReport call site since the existing
// 3-arg signature is the public surface used by tests and integrations.
var FocusMode = "full"

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
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " [" + label + "]"
		}
		fmt.Fprintf(w, "  %s %s -- mandatory trigger loop%s\n", prefix, strings.Join(c.Cards, " + "), classTag)
		// Split description on " | " to show outlets on a separate line.
		// Mana symbols inside the description (`{R}`, `{1}{U}`, etc.) are
		// rendered as emoji color discs for the text renderer.
		parts := strings.SplitN(RenderMana(c.Description, ManaText), " | ", 2)
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
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " [" + label + "]"
		}
		fmt.Fprintf(w, "  %s %s%s\n", prefix, strings.Join(c.Cards, " + "), classTag)
		// Split description on " | " to show outlets on a separate line.
		parts := strings.SplitN(RenderMana(c.Description, ManaText), " | ", 2)
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

	// Combo interaction matrix — piece overlap, fragility, redundancy.
	// Only renders when >= 2 combos exist (matrix is nil otherwise).
	printComboInteraction(w, r.ComboInteraction)

	// Land-cycle synergies. These are dual-cycle land pairs (Scattered
	// Groves + Irrigated Farmland, etc.) that the heuristic detector
	// flagged as determined loops via the cycling discard-cost +
	// draw-effect cycle. Real value, but only a deliberate wincon
	// component in Lands Matter / Reanimator / Selfmill — see
	// archetype.go for the gated bracket-lift contribution.
	if len(r.LandCycleSynergies) > 0 {
		fmt.Fprintf(w, "[GRN] LAND CYCLE SYNERGIES -- incidental fixing outside Lands Matter / Reanimator / Selfmill (%d found)\n",
			len(r.LandCycleSynergies))
		for _, c := range r.LandCycleSynergies {
			classTag := ""
			if label := ComboClassLabel(c.Class); label != "" {
				classTag = " [" + label + "]"
			}
			fmt.Fprintf(w, "  \xf0\x9f\x8c\xb1 %s%s\n", strings.Join(c.Cards, " + "), classTag)
			fmt.Fprintf(w, "    %s\n", c.Description)
		}
		fmt.Fprintf(w, "\n")
	}

	// Finishers.
	fmt.Fprintf(w, "[YLW] GAME FINISHERS (%d found)\n", len(r.Finishers))
	if len(r.Finishers) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.Finishers {
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " [" + label + "]"
		}
		fmt.Fprintf(w, "  * %s%s -- %s\n", strings.Join(c.Cards, " + "), classTag, RenderMana(c.Description, ManaText))
	}
	fmt.Fprintf(w, "\n")

	// Synergies.
	fmt.Fprintf(w, "[BLU] SYNERGIES (%d found)\n", len(r.Synergies))
	if len(r.Synergies) == 0 {
		fmt.Fprintf(w, "  (none detected)\n")
	}
	for _, c := range r.Synergies {
		fmt.Fprintf(w, "  * %s\n", strings.Join(c.Cards, " + "))
		fmt.Fprintf(w, "    %s\n", RenderMana(c.Description, ManaText))
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
	for _, cw := range r.CurveWarnings {
		fmt.Fprintf(w, "  ⚠️  %s\n", cw)
	}
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
		colorNames := map[string]string{"W": "White", "U": "Blue", "B": "Black", "R": "Red", "G": "Green", "C": "Colorless"}
		fmt.Fprintf(w, "  %-9s %6s %6s  %6s %6s  %s\n", "Color", "Pips", "Dem%", "Srcs", "Sup%", "Status")
		fmt.Fprintf(w, "  %-9s %6s %6s  %6s %6s  %s\n", "─────", "────", "────", "────", "────", "──────")
		for _, c := range []string{"W", "U", "B", "R", "G", "C"} {
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
				fmt.Fprintf(w, "  %-9s %6d %5.0f%%  %6d %5.0f%%  %s\n",
					colorNames[c], demand, dPct, supply, sPct, status)
			}
		}
		fmt.Fprintf(w, "  %-9s %6d %5s  %6d\n", "Total", totalDemand, "", totalSupply)
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
		if len(wl.DefendedBy) > 0 {
			fmt.Fprintf(w, "     Defended by: %s\n", strings.Join(wl.DefendedBy, ", "))
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

	if ac.BracketRationale != nil {
		printBracketRationaleText(w, ac.BracketRationale, "  ")
	}

	if len(ac.Signals) > 0 {
		fmt.Fprintf(w, "  Signals:\n")
		for _, s := range ac.Signals {
			fmt.Fprintf(w, "    - %s\n", s)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "  Intent: %s\n\n", ac.Intent)
}

// printBracketRationaleText renders the bracket-derivation breakdown
// under the bracket header: each scoring signal with its tier, evidence,
// and contribution; followed by any ceiling/floor/gate adjustments and
// the raw-vs-final score. indent is the leading whitespace per line.
func printBracketRationaleText(w io.Writer, br *BracketRationale, indent string) {
	if br == nil || len(br.Signals) == 0 {
		return
	}
	fmt.Fprintf(w, "%sBracket rationale (raw score %d → B%d %s):\n",
		indent, br.RawScore, br.FinalBracket, br.FinalLabel)
	for _, sig := range br.Signals {
		if sig.Kind == "score" {
			line := fmt.Sprintf("%s  [%+d] %s (%s): %s",
				indent, sig.Contribution, sig.Name, sig.Tier, sig.Measurement)
			if len(sig.Evidence) > 0 {
				ev := sig.Evidence
				if len(ev) > 6 {
					ev = append([]string{}, ev[:6]...)
					ev = append(ev, fmt.Sprintf("+%d more", len(sig.Evidence)-6))
				}
				line += " — " + strings.Join(ev, ", ")
			}
			fmt.Fprintln(w, line)
		} else {
			fmt.Fprintf(w, "%s  [%s] %s: %s\n", indent, sig.Kind, sig.Name, sig.Note)
		}
	}
	fmt.Fprintf(w, "\n")
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
		fmt.Fprintf(w, "  Tutors:    %d cards (%d real, %d land/ramp)\n",
			r.TutorCount, r.NonLandTutorCount, r.LandTutorCount)
		fmt.Fprintf(w, "  Removal:   %d cards (%d single-target)\n", r.RemovalCount, r.SingleTargetRemovalCount)
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
	if dp.IsBlend {
		fmt.Fprintf(w, "  Blend:      %s (primary confidence %.0f%%, secondary fit %.0f%%)\n",
			dp.BlendLabel, dp.ArchetypeConfidence*100, dp.SecondaryFit*100)
	}
	fmt.Fprintf(w, "  Bracket:    %d/5 (%s)\n", dp.Bracket, dp.BracketLabel)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Gameplan:   %s\n", dp.GameplanSummary)
	fmt.Fprintf(w, "\n")

	// Turn-by-turn script + branching decisions + degradation paths.
	// Empty for archetype-less decks (defensive — buildGameplanScript
	// returns nil); skipped silently in that case.
	if dp.GameplanScript != nil {
		renderGameplanScript(func(s string) { fmt.Fprint(w, s) }, dp.GameplanScript)
		fmt.Fprintf(w, "\n")
	}

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

	if dp.PersonalityBlurb != "" {
		fmt.Fprintf(w, "  %s\n", dp.PersonalityBlurb)
		if dp.PersonalityTagline != "" {
			fmt.Fprintf(w, "    — %s\n", dp.PersonalityTagline)
		}
		fmt.Fprintf(w, "\n")
	}

	if dp.CanonicalPreconOverlap != nil && dp.CanonicalPreconOverlap.SharedCount > 0 {
		fmt.Fprintf(w, "  Precon overlap: %s\n\n", FormatPreconOverlap(dp.CanonicalPreconOverlap))
	}

	if len(dp.CoachingTips) > 0 {
		fmt.Fprintf(w, "  Coaching (bracket %d, %s):\n", dp.Bracket, dp.PrimaryArchetype)
		for _, t := range dp.CoachingTips {
			fmt.Fprintf(w, "    [P%d %s] %s\n", t.Priority, t.Category, t.Title)
			if t.Detail != "" {
				fmt.Fprintf(w, "        %s\n", t.Detail)
			}
			if t.Action != "" {
				fmt.Fprintf(w, "        → %s\n", t.Action)
			}
		}
		fmt.Fprintf(w, "\n")
	}

	if dp.ManaBaseGrade != "" {
		fmt.Fprintf(w, "  Mana Base:  Grade %s", dp.ManaBaseGrade)
		if dp.FetchCount > 0 || dp.TaplandCount > 0 {
			fmt.Fprintf(w, " (%d fetch, %d tapland, %d utility)", dp.FetchCount, dp.TaplandCount, dp.UtilityLandCount)
		}
		fmt.Fprintf(w, "\n")
		for _, note := range dp.ManaBaseNotes {
			fmt.Fprintf(w, "    %s\n", note)
		}
	}

	if dp.DeckCostTier != "" {
		fmt.Fprintf(w, "  Deck Cost:  %s tier — %s\n", dp.DeckCostTier, dp.DeckCostNote)
	} else if dp.DeckCostNote != "" {
		fmt.Fprintf(w, "  Deck Cost:  %s\n", dp.DeckCostNote)
	}

	if dp.CountersMatterTheme {
		strength := "present"
		if dp.CountersMatterStrong {
			strength = "primary engine"
		}
		fmt.Fprintf(w, "  Counters Matter: %d producers + %d payoffs (%s)\n",
			len(dp.CounterProducers), len(dp.CounterPayoffs), strength)
	}

	if dp.KeepableHandPct > 0 {
		fmt.Fprintf(w, "  Opening Hands: %.0f%% keepable, avg turn to 4 mana: %.1f\n", dp.KeepableHandPct, dp.AvgTurnToFourMana)
		if dp.KeepableHandPctFreeMull > 0 {
			fmt.Fprintf(w, "    With Vancouver free mulligan: %.0f%% keepable (+%.0f%%)\n",
				dp.KeepableHandPctFreeMull, dp.KeepableHandPctFreeMull-dp.KeepableHandPct)
		}
		if dp.IsCommanderCentric && dp.KeepableHandPctAdjusted > 0 {
			extra := ""
			if dp.AvgTurnToCommander > 0 {
				extra = fmt.Sprintf(", avg turn to commander (CMC %d): %.1f", dp.CommanderCMC, dp.AvgTurnToCommander)
			}
			fmt.Fprintf(w, "    Commander-adjusted: %.0f%% keepable%s\n", dp.KeepableHandPctAdjusted, extra)
			if dp.KeepableHandPctAdjustedFreeMull > 0 {
				fmt.Fprintf(w, "    Commander-adjusted with free mulligan: %.0f%% keepable (+%.0f%%)\n",
					dp.KeepableHandPctAdjustedFreeMull, dp.KeepableHandPctAdjustedFreeMull-dp.KeepableHandPctAdjusted)
			}
			fmt.Fprintf(w, "    (commander-centric: %s)\n", dp.CommanderCentricReason)
		}
	}

	if dp.PowerPercentile > 0 {
		fmt.Fprintf(w, "  Power: ~%dth percentile within %s\n", dp.PowerPercentile, dp.PrimaryArchetype)
	}

	if dp.CommanderSynergy > 0 {
		fmt.Fprintf(w, "  Commander Synergy: %.0f%% (%d/%d cards match themes: %s)\n",
			dp.CommanderSynergy*100, dp.SynergyCount,
			dp.CardCount-dp.LandCount, strings.Join(dp.CommanderThemes, ", "))
	}

	if dp.InteractionQuality > 0 {
		fmt.Fprintf(w, "  Interaction Speed: avg CMC %.1f (%d cheap, %d expensive)\n",
			dp.InteractionQuality, dp.CheapInteraction, dp.ExpensiveInteraction)
		if len(dp.InteractionDownsides) > 0 {
			fmt.Fprintf(w, "    adjusted CMC %.1f after downsides (%d piece(s) grant opponent resources):\n",
				dp.AdjustedInteractionQuality, len(dp.InteractionDownsides))
			for _, d := range dp.InteractionDownsides {
				fmt.Fprintf(w, "      ↓ [CMC %d] %s — %s\n", d.CMC, d.Name, d.Note)
			}
		}
	}

	if pkg := dp.InteractionPackage; pkg.Score > 0 || len(pkg.Counterspells)+len(pkg.Protection)+len(pkg.OpponentInteraction)+len(pkg.ProtectionTutors) > 0 {
		fmt.Fprintf(w, "  Interaction Package: %.2f (%d counters / %d opp-int / %d protection / %d prot tutors)\n",
			pkg.Score, len(pkg.Counterspells), len(pkg.OpponentInteraction),
			len(pkg.Protection), len(pkg.ProtectionTutors))
		if len(pkg.Counterspells) > 0 {
			fmt.Fprintf(w, "    counters:    %s\n", strings.Join(pkg.Counterspells, ", "))
		}
		if len(pkg.OpponentInteraction) > 0 {
			fmt.Fprintf(w, "    opp-int:     %s\n", strings.Join(pkg.OpponentInteraction, ", "))
		}
		if len(pkg.Protection) > 0 {
			fmt.Fprintf(w, "    protection:  %s\n", strings.Join(pkg.Protection, ", "))
		}
		if len(pkg.ProtectionTutors) > 0 {
			fmt.Fprintf(w, "    prot tutors: %s\n", strings.Join(pkg.ProtectionTutors, ", "))
		}
	}

	if len(dp.PowerTierCounts) > 0 {
		parts := make([]string, 0, len(PowerTierOrder))
		for _, t := range PowerTierOrder {
			parts = append(parts, fmt.Sprintf("%d%s", dp.PowerTierCounts[t], t))
		}
		fmt.Fprintf(w, "\n  Power Tiers: %s (buy S→A first; D = cut candidates)\n",
			strings.Join(parts, " / "))
	}

	renderTierCard := func(glyph string, c CardQuality) {
		fmt.Fprintf(w, "    %s [%s %3d] %s — %s\n", glyph, c.PowerTier, c.Power, c.Name, c.Reason)
		if c.PowerExplanation != "" {
			fmt.Fprintf(w, "             why: %s\n", c.PowerExplanation)
		}
	}
	if len(dp.StarCards) > 0 {
		fmt.Fprintf(w, "\n  Star Cards:\n")
		for _, c := range dp.StarCards {
			renderTierCard("★", c)
		}
	}
	if len(dp.SolidCards) > 0 {
		fmt.Fprintf(w, "  Solid Picks:\n")
		for _, c := range dp.SolidCards {
			renderTierCard("●", c)
		}
	}
	if len(dp.FlexSlots) > 0 {
		fmt.Fprintf(w, "  Flex Slots (could swap for situational meta tech):\n")
		for _, c := range dp.FlexSlots {
			renderTierCard("⇄", c)
		}
	}
	if len(dp.CuttableCards) > 0 {
		fmt.Fprintf(w, "  Consider Cutting:\n")
		for _, c := range dp.CuttableCards {
			renderTierCard("✂", c)
		}
	}

	if len(dp.PetCards) > 0 {
		fmt.Fprintf(w, "  Pet Cards (flavor picks — personal taste outweighs optimization):\n")
		for _, pc := range dp.PetCards {
			fmt.Fprintf(w, "    ♥ [%s %3d] %s — %s\n", pc.PowerTier, pc.Power, pc.Name, pc.Reason)
		}
	}

	if len(dp.LandSwapSuggestions) > 0 {
		fmt.Fprintf(w, "\n  Land Swaps:\n")
		for _, s := range dp.LandSwapSuggestions {
			fmt.Fprintf(w, "    → %s\n", s)
		}
	}

	if len(dp.CurveArchetypeWarnings) > 0 {
		fmt.Fprintf(w, "\n  Curve vs. Archetype:\n")
		for _, s := range dp.CurveArchetypeWarnings {
			fmt.Fprintf(w, "    ⚠ %s\n", s)
		}
	}

	if len(dp.VulnerableTo) > 0 {
		fmt.Fprintf(w, "\n  Vulnerable To:\n")
		for _, v := range dp.VulnerableTo {
			fmt.Fprintf(w, "    ⚡ %s\n", v)
		}
	}

	if len(dp.VulnerableComboPieces) > 0 {
		fmt.Fprintf(w, "\n  Vulnerable Combo Pieces (no built-in protection — single removal resets the line):\n")
		for _, v := range dp.VulnerableComboPieces {
			fmt.Fprintf(w, "    ⚠ [CMC %d] %s — %s\n", v.CMC, v.Name, v.Reason)
		}
	}

	// Per-combo meta vulnerability — stax / graveyard / removal exposure.
	if dp.ComboMetaInteraction != nil && len(dp.ComboMetaInteraction.PerCombo) > 0 {
		fmt.Fprintf(w, "\n")
		printComboMetaInteraction(w, dp.ComboMetaInteraction)
	}

	// Interaction floor — minimum interaction the opposing pod must
	// resolve to shut each combo down, accounting for deck defense.
	if dp.InteractionFloor != nil && len(dp.InteractionFloor.PerCombo) > 0 {
		printComboInteractionFloor(w, dp.InteractionFloor)
	}

	// Combo timing — earliest reasonable turn each combo can assemble.
	if dp.ComboTiming != nil && len(dp.ComboTiming.PerCombo) > 0 {
		printComboTiming(w, dp.ComboTiming)
	}

	if len(dp.MetaMatchups) > 0 {
		fmt.Fprintf(w, "\n  Meta Positioning:\n")
		for _, m := range dp.MetaMatchups {
			icon := "≈"
			if m.Rating == "favored" {
				icon = "▲"
			} else if m.Rating == "unfavored" {
				icon = "▼"
			}
			fmt.Fprintf(w, "    %s vs %s: %s (%d%% expected) — %s\n",
				icon, m.Archetype, m.Rating, m.ExpectedWinPct, m.Reason)
			if m.EmpiricalGames > 0 {
				suffix := ""
				if m.Tilted {
					delta := m.TiltDelta
					if delta < 0 {
						delta = -delta
					}
					suffix = fmt.Sprintf(" — TILTED %s-performing by %d pp", m.TiltDirection, delta)
				}
				fmt.Fprintf(w, "        history: %d games, %d%% empirical vs %d%% baseline%s\n",
					m.EmpiricalGames, m.EmpiricalWinPct, m.BaselineWinPct, suffix)
			}
		}
	}

	if len(dp.TechSuggestions) > 0 && dp.WorstMatchup != "" {
		fmt.Fprintf(w, "\n  Tech Cards for worst matchup (vs %s):\n", dp.WorstMatchup)
		for _, s := range dp.TechSuggestions {
			tag := ""
			if s.AlreadyInDeck {
				tag = " ✓"
			}
			sev := "minor"
			switch s.Severity {
			case 2:
				sev = "major"
			case 3:
				sev = "critical"
			}
			fmt.Fprintf(w, "    %s%s [%s] — %s\n", s.Card, tag, sev, s.Reason)
		}
	}

	if len(dp.StrongAgainst) > 0 {
		fmt.Fprintf(w, "\n  Favored Against (reverse-lookup):\n")
		for _, a := range dp.StrongAgainst {
			tag := ""
			switch a.Source {
			case "both":
				tag = " [both directions]"
			case "reverse":
				tag = " [from opponent's perspective]"
			}
			fmt.Fprintf(w, "    ▲ %s%s — %s\n", a.Archetype, tag, a.Reason)
			if a.OpponentReason != "" {
				fmt.Fprintf(w, "        ↳ they say: %s\n", a.OpponentReason)
			}
		}
	}

	if len(dp.SynergyClusters) > 0 {
		fmt.Fprintf(w, "\n  Synergy Clusters:\n")
		for _, sc := range dp.SynergyClusters {
			prefix := ""
			if sc.HighDensity {
				prefix = "★ high-density — "
			}
			fmt.Fprintf(w, "    [%s] %s%s (%d pairwise synergies, %d members)\n",
				sc.Name, prefix, strings.Join(sc.Cards, ", "), sc.Score, sc.MemberCount)
		}
	}

	if len(dp.AltBuildSuggestions) > 0 {
		fmt.Fprintf(w, "\n  Alt-Build Suggestions (deck splits across multiple engines):\n")
		for _, a := range dp.AltBuildSuggestions {
			fmt.Fprintf(w, "    ◆ %s\n", a.Pivot)
			fmt.Fprintf(w, "        %s\n", a.Trade)
		}
	}

	fmt.Fprintf(w, "\n")
}

// ---------------------------------------------------------------------------
// Markdown output
// ---------------------------------------------------------------------------

func printMarkdown(w io.Writer, r *FreyaReport) {
	// Compact summary header (TL;DR) — designed for Discord previews
	// where readers see the first 8-12 lines without scrolling.
	// Archetype + bracket + win method + the gameplan one-liner land
	// in the visible part of the preview.
	printMarkdownSummaryHeader(w, r)
	if r.DeckPath != "" {
		fmt.Fprintf(w, "_Source: `%s`_\n\n", r.DeckPath)
	}

	// Legality validation (always first).
	printLegalityMarkdown(w, r.Legality)

	fmt.Fprintf(w, "## True Infinites -- Mandatory Loops (%d)\n\n", len(r.TrueInfinites))
	for _, c := range r.TrueInfinites {
		prefix := "\xf0\x9f\x94\x8d"
		if c.Confirmed {
			prefix = "\xe2\x9c\x85"
		}
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " _[" + label + "]_"
		}
		fmt.Fprintf(w, "- %s %s -- mandatory trigger loop%s\n", prefix, scryfallLinks(c.Cards, " + "), classTag)
		// Mana symbols inside the description render as emoji color discs
		// (Markdown viewers — Discord, GitHub, Reddit — render Unicode
		// emoji natively).
		parts := strings.SplitN(RenderMana(c.Description, ManaMarkdown), " | ", 2)
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
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " _[" + label + "]_"
		}
		fmt.Fprintf(w, "- %s %s%s\n", prefix, scryfallLinks(c.Cards, " + "), classTag)
		parts := strings.SplitN(RenderMana(c.Description, ManaMarkdown), " | ", 2)
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
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " _[" + label + "]_"
		}
		fmt.Fprintf(w, "- %s%s -- %s\n", scryfallLinks(c.Cards, " + "), classTag, RenderMana(c.Description, ManaMarkdown))
	}
	if len(r.Finishers) == 0 {
		fmt.Fprintf(w, "_None detected._\n")
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "## Synergies (%d)\n\n", len(r.Synergies))
	for _, c := range r.Synergies {
		fmt.Fprintf(w, "- %s -- %s\n", scryfallLinks(c.Cards, " + "), RenderMana(c.Description, ManaMarkdown))
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
		for _, c := range []string{"W", "U", "B", "R", "G", "C"} {
			dPct := 0.0
			sPct := 0.0
			if totalDemandMD > 0 {
				dPct = float64(r.ColorDemand[c]) / float64(totalDemandMD) * 100
			}
			if totalSupplyMD > 0 {
				sPct = float64(r.ColorSupply[c]) / float64(totalSupplyMD) * 100
			}
			if r.ColorDemand[c] > 0 || r.ColorSupply[c] > 0 {
				fmt.Fprintf(w, "| %s | %.0f%% | %.0f%% |\n", c, dPct, sPct)
			}
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
	fmt.Fprintf(w, "**Commander:** %s", scryfallLink(dp.Commander))
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

	// Structured turn-by-turn gameplan (PR #902). Sits inside the
	// Deck Profile section as a play-guide subsection — the
	// archetype-driven script reads naturally after the one-sentence
	// summary. Nil-safe for archetype-less decks.
	printGameplanScriptMarkdown(w, dp.GameplanScript)

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
	TrueInfinites      []jsonCombo `json:"true_infinites"`
	Determined         []jsonCombo `json:"determined_loops"`
	Finishers          []jsonCombo `json:"finishers"`
	Synergies          []jsonCombo `json:"synergies"`
	LandCycleSynergies []jsonCombo             `json:"land_cycle_synergies,omitempty"`
	GraveyardLoops     []jsonCombo             `json:"graveyard_loops,omitempty"`
	ComboInteraction   *jsonComboInteraction   `json:"combo_interaction,omitempty"`
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
	Distribution [8]int   `json:"distribution"`
	AvgCMC       float64  `json:"avg_cmc"`
	CurveShape   string   `json:"curve_shape"`
	Warnings     []string `json:"warnings,omitempty"`
	LandCount    int      `json:"land_count"`
	NonlandCount int      `json:"nonland_count"`
}

type jsonColors struct {
	Demand   map[string]int `json:"demand"`
	Supply   map[string]int `json:"supply"`
	Warnings []string       `json:"warnings,omitempty"`
}

type jsonCombo struct {
	Cards            []string            `json:"cards"`
	LoopType         string              `json:"loop_type"`
	Class            string              `json:"class,omitempty"`
	Resources        string              `json:"resources,omitempty"`
	Description      string              `json:"description"`
	Confirmed        bool                `json:"confirmed,omitempty"`
	NonDeterministic bool                `json:"non_deterministic,omitempty"`
	Annotation       *jsonLoopAnnotation `json:"annotation,omitempty"`
}

type jsonLoopAnnotation struct {
	PrimaryOutput   string         `json:"primary_output"`
	NetProduces     []ResourceType `json:"net_produces,omitempty"`
	ExternalEffects []string       `json:"external_effects,omitempty"`
	Classification  string         `json:"classification"`
	Summary         string         `json:"summary"`
}

type jsonComboInteraction struct {
	Combos                    []jsonComboMatrixEntry `json:"combos"`
	Overlap                   [][]int                `json:"overlap"`
	PieceFragility            []jsonPieceFragility   `json:"piece_fragility"`
	RedundancyOneCardRemoved  int                    `json:"redundancy_one_card_removed"`
	MostFragileComboIndex     int                    `json:"most_fragile_combo_index"`
	MostIndependentComboIndex int                    `json:"most_independent_combo_index"`
	IndependentComboCount     int                    `json:"independent_combo_count"`
}

type jsonComboMatrixEntry struct {
	Label    string   `json:"label"`
	Cards    []string `json:"cards"`
	LoopType string   `json:"loop_type"`
	Class    string   `json:"class,omitempty"`
	Source   string   `json:"source"`
}

type jsonPieceFragility struct {
	Card         string `json:"card"`
	ComboCount   int    `json:"combo_count"`
	ComboIndices []int  `json:"combo_indices"`
}

type jsonComboMetaInteraction struct {
	PerCombo              []jsonComboMetaVuln `json:"per_combo"`
	WorstStaxHoser        string              `json:"worst_stax_hoser,omitempty"`
	WorstStaxCount        int                 `json:"worst_stax_count,omitempty"`
	WorstGraveyardHoser   string              `json:"worst_graveyard_hoser,omitempty"`
	WorstGraveyardCount   int                 `json:"worst_graveyard_count,omitempty"`
	SurviveStaxCount      int                 `json:"survive_stax_count"`
	SurviveGraveyardCount int                 `json:"survive_graveyard_count"`
	FragileComboCount     int                 `json:"fragile_combo_count"`
}

type jsonComboTimingReport struct {
	PerCombo          []jsonComboTimingEntry `json:"per_combo"`
	MinTurn           int                    `json:"min_turn"`
	MaxTurn           int                    `json:"max_turn"`
	MedianTurn        int                    `json:"median_turn"`
	FastestComboIndex int                    `json:"fastest_combo_index"`
	FastestComboLabel string                 `json:"fastest_combo_label"`
	FastestTurn       int                    `json:"fastest_turn"`
	SlowestComboIndex int                    `json:"slowest_combo_index"`
	SlowestComboLabel string                 `json:"slowest_combo_label"`
	SlowestTurn       int                    `json:"slowest_turn"`
	BracketHint       string                 `json:"bracket_hint"`
}

type jsonComboTimingEntry struct {
	ComboIndex       int    `json:"combo_index"`
	Label            string `json:"label"`
	Source           string `json:"source"`
	TotalCMC         int    `json:"total_cmc"`
	MaxPieceCMC      int    `json:"max_piece_cmc"`
	NaturalTurn      int    `json:"natural_turn"`
	RampCompression  int    `json:"ramp_compression"`
	TutorCompression int    `json:"tutor_compression"`
	HandPenalty      int    `json:"hand_penalty"`
	EarliestTurn     int    `json:"earliest_turn"`
	Pacing           string `json:"pacing"`
}

type jsonComboInteractionFloor struct {
	PerCombo           []jsonComboInteractionFloorEntry `json:"per_combo"`
	MinFloor           int                              `json:"min_floor"`
	MaxFloor           int                              `json:"max_floor"`
	MedianFloor        int                              `json:"median_floor"`
	CounterspellCount  int                              `json:"counterspell_count"`
	ProtectionCount    int                              `json:"protection_count"`
	CheapestComboLabel string                           `json:"cheapest_combo_label"`
	CheapestComboIndex int                              `json:"cheapest_combo_index"`
	CheapestFloor      int                              `json:"cheapest_floor"`
	HardestComboLabel  string                           `json:"hardest_combo_label"`
	HardestComboIndex  int                              `json:"hardest_combo_index"`
	HardestFloor       int                              `json:"hardest_floor"`
	DeckbuildAdvice    string                           `json:"deckbuild_advice"`
}

type jsonComboInteractionFloorEntry struct {
	ComboIndex             int    `json:"combo_index"`
	Label                  string `json:"label"`
	Source                 string `json:"source"`
	RemovalAnswerCost      int    `json:"removal_answer_cost"`
	StaxAnswerCost         int    `json:"stax_answer_cost"`
	GraveyardAnswerCost    int    `json:"graveyard_answer_cost"`
	CounterspellAnswerCost int    `json:"counterspell_answer_cost"`
	CheapestAxis           string `json:"cheapest_axis"`
	CheapestAnswerCost     int    `json:"cheapest_answer_cost"`
	DefensiveLayerTax      int    `json:"defensive_layer_tax"`
	InteractionFloor       int    `json:"interaction_floor"`
}

type jsonComboMetaVuln struct {
	ComboIndex             int      `json:"combo_index"`
	Label                  string   `json:"label"`
	Source                 string   `json:"source"`
	Cards                  []string `json:"cards"`
	StaxScore              int      `json:"stax_score"`
	StaxHosers             []string `json:"stax_hosers,omitempty"`
	StaxReasons            []string `json:"stax_reasons,omitempty"`
	GraveyardScore         int      `json:"graveyard_score"`
	GraveyardHosers        []string `json:"graveyard_hosers,omitempty"`
	GraveyardReasons       []string `json:"graveyard_reasons,omitempty"`
	PermanentPieces        int      `json:"permanent_pieces"`
	ProtectedPieces        int      `json:"protected_pieces"`
	UnprotectedPieceNames  []string `json:"unprotected_piece_names,omitempty"`
	ProtectedPieceNames    []string `json:"protected_piece_names,omitempty"`
	RemovalRequiredToBreak int      `json:"removal_required_to_break"`
	DominantThreat         string   `json:"dominant_threat"`
}

type jsonProfile struct {
	Tutors        int `json:"tutors"`
	NonLandTutors int `json:"non_land_tutors"`
	LandTutors    int `json:"land_tutors"`
	WishTutors    int `json:"wish_tutors,omitempty"`
	Removal             int `json:"removal"`
	SingleTargetRemoval int `json:"single_target_removal"`
	Outlets             int `json:"sacrifice_outlets"`
	WinCons             int `json:"win_conditions"`
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
	SecondaryFit       float64           `json:"secondary_fit,omitempty"`
	IsBlend            bool              `json:"is_blend,omitempty"`
	BlendLabel         string            `json:"blend_label,omitempty"`
	// Bracket is the declared / rubber-stamp value (see DeckProfile.Bracket).
	// MeasuredBracket is Freya's signal-computed bracket; they diverge
	// for wizards/ precons (declared B2, may measure hotter).
	Bracket               int               `json:"bracket"`
	BracketLabel          string            `json:"bracket_label"`
	MeasuredBracket       int               `json:"measured_bracket"`
	MeasuredBracketLabel  string            `json:"measured_bracket_label,omitempty"`
	Intent             string            `json:"intent"`
	PrimaryWinLine     string            `json:"primary_win_line"`
	WinLineCount       int               `json:"win_line_count"`
	BackupCount        int               `json:"backup_count"`
	HasTutorAccess     bool              `json:"has_tutor_access"`
	SinglePointCount   int               `json:"single_point_count"`
	Strengths          []string          `json:"strengths,omitempty"`
	Weaknesses         []string          `json:"weaknesses,omitempty"`
	GameplanSummary    string            `json:"gameplan_summary"`
	GameplanScript     *GameplanScript   `json:"gameplan_script,omitempty"`
	PersonalityBlurb   string            `json:"personality_blurb,omitempty"`
	PersonalityTagline string            `json:"personality_tagline,omitempty"`
	ManaBaseGrade      string            `json:"mana_base_grade,omitempty"`
	ManaBaseNotes      []string          `json:"mana_base_notes,omitempty"`
	TaplandCount       int               `json:"tapland_count,omitempty"`
	FetchCount         int               `json:"fetch_count,omitempty"`
	UtilityLandCount   int               `json:"utility_land_count,omitempty"`
	DeckCostTier       string            `json:"deck_cost_tier,omitempty"`
	EstimatedTotalUSD  float64           `json:"estimated_total_usd,omitempty"`
	PricedCardCount    int               `json:"priced_card_count,omitempty"`
	UnpricedCardCount  int               `json:"unpriced_card_count,omitempty"`
	CounterProducers     []string        `json:"counter_producers,omitempty"`
	CounterPayoffs       []string        `json:"counter_payoffs,omitempty"`
	CountersMatterTheme  bool            `json:"counters_matter_theme,omitempty"`
	CountersMatterStrong bool            `json:"counters_matter_strong,omitempty"`
	DeckCostNote       string            `json:"deck_cost_note,omitempty"`
	VulnerableTo       []string          `json:"vulnerable_to,omitempty"`
	VulnerableComboPieces []jsonVulnerableComboPiece `json:"vulnerable_combo_pieces,omitempty"`
	ComboMetaInteraction  *jsonComboMetaInteraction  `json:"combo_meta_interaction,omitempty"`
	InteractionFloor      *jsonComboInteractionFloor `json:"interaction_floor,omitempty"`
	ComboTiming           *jsonComboTimingReport     `json:"combo_timing,omitempty"`
	KeepableHandPct                 float64 `json:"keepable_hand_pct,omitempty"`
	AvgTurnToFourMana               float64 `json:"avg_turn_to_four_mana,omitempty"`
	KeepableHandPctAdjusted         float64 `json:"keepable_hand_pct_adjusted,omitempty"`
	AvgTurnToCommander              float64 `json:"avg_turn_to_commander,omitempty"`
	KeepableHandPctFreeMull         float64 `json:"keepable_hand_pct_free_mull,omitempty"`
	KeepableHandPctAdjustedFreeMull float64 `json:"keepable_hand_pct_adjusted_free_mull,omitempty"`
	IsCommanderCentric      bool          `json:"is_commander_centric,omitempty"`
	CommanderCentricReason  string        `json:"commander_centric_reason,omitempty"`
	CommanderCMC            int           `json:"commander_cmc,omitempty"`
	SynergyClusters    []jsonCluster     `json:"synergy_clusters,omitempty"`
	// ClusterExport is the rich structured export — full membership,
	// per-card roles, score breakdown. Downstream deck-builder
	// integrations consume this; the display-oriented SynergyClusters
	// above stays for backward compatibility. See cluster_export.go.
	ClusterExport      *SynergyClusterExport `json:"cluster_export,omitempty"`
	AltBuildSuggestions []jsonAltBuild   `json:"alt_build_suggestions,omitempty"`
	MetaMatchups       []jsonMatchup     `json:"meta_matchups,omitempty"`
	StrongAgainst      []jsonStrongAgainst `json:"strong_against,omitempty"`
	WorstMatchup       string            `json:"worst_matchup,omitempty"`
	TechSuggestions    []TechCardSuggestion `json:"tech_suggestions,omitempty"`
	StarCards          []jsonCardQuality    `json:"star_cards,omitempty"`
	SolidCards         []jsonCardQuality    `json:"solid_cards,omitempty"`
	FlexSlots          []jsonCardQuality    `json:"flex_slots,omitempty"`
	CuttableCards      []jsonCardQuality    `json:"cuttable_cards,omitempty"`
	CardPowerLevels    []jsonCardPowerLevel `json:"card_power_levels,omitempty"`
	PowerTierCounts    map[string]int       `json:"power_tier_counts,omitempty"`
	PetCards           []jsonPetCard        `json:"pet_cards,omitempty"`
	LandSwapSuggestions    []string       `json:"land_swap_suggestions,omitempty"`
	CurveArchetypeWarnings []string       `json:"curve_archetype_warnings,omitempty"`
	CanonicalPreconOverlap *PreconOverlap `json:"canonical_precon_overlap,omitempty"`
	CommanderSynergy   float64           `json:"commander_synergy,omitempty"`
	CommanderThemes    []string          `json:"commander_themes,omitempty"`
	InteractionQuality float64           `json:"interaction_quality,omitempty"`
	AdjustedInteractionQuality float64   `json:"adjusted_interaction_quality,omitempty"`
	InteractionDownsides []jsonInteractionDownside `json:"interaction_downsides,omitempty"`
	InteractionPackage *jsonInteractionPackage `json:"interaction_package,omitempty"`
	PowerPercentile    int               `json:"power_percentile,omitempty"`
	PowerFactors       []string          `json:"power_factors,omitempty"`
	CoachingTips       []jsonCoachingTip `json:"coaching_tips,omitempty"`
}

type jsonInteractionDownside struct {
	Name string `json:"name"`
	CMC  int    `json:"cmc"`
	Kind string `json:"kind"`
	Note string `json:"note"`
}

type jsonInteractionPackage struct {
	Counterspells       []string `json:"counterspells,omitempty"`
	OpponentInteraction []string `json:"opponent_interaction,omitempty"`
	Protection          []string `json:"protection,omitempty"`
	ProtectionTutors    []string `json:"protection_tutors,omitempty"`
	Score               float64  `json:"interaction_package_score"`
}

type jsonVulnerableComboPiece struct {
	Name   string `json:"name"`
	CMC    int    `json:"cmc"`
	Reason string `json:"reason"`
}

type jsonCoachingTip struct {
	Category string   `json:"category"`
	Priority int      `json:"priority"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail,omitempty"`
	Action   string   `json:"action,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type jsonCluster struct {
	Name        string   `json:"name"`
	Cards       []string `json:"cards"`
	Theme       string   `json:"theme"`
	Score       int      `json:"synergy_count"`
	MemberCount int      `json:"member_count,omitempty"`
	HighDensity bool     `json:"high_density,omitempty"`
}

type jsonAltBuild struct {
	Cluster     string `json:"cluster"`
	ClusterName string `json:"cluster_name"`
	MemberCount int    `json:"member_count"`
	Score       int    `json:"score"`
	Pivot       string `json:"pivot"`
	Trade       string `json:"trade"`
}

type jsonMatchup struct {
	Archetype       string `json:"vs_archetype"`
	Rating          string `json:"rating"`
	Reason          string `json:"reason"`
	Strength        string `json:"strength,omitempty"`
	ExpectedWinPct  int    `json:"expected_win_pct"`
	BaselineWinPct  int    `json:"baseline_win_pct,omitempty"`
	EmpiricalGames  int    `json:"empirical_games,omitempty"`
	EmpiricalWinPct int    `json:"empirical_win_pct,omitempty"`
	Tilted          bool   `json:"tilted,omitempty"`
	TiltDirection   string `json:"tilt_direction,omitempty"`
	TiltDelta       int    `json:"tilt_delta_pp,omitempty"`
}

type jsonStrongAgainst struct {
	Archetype      string `json:"vs_archetype"`
	Reason         string `json:"reason"`
	OpponentReason string `json:"opponent_reason,omitempty"`
	Source         string `json:"source"` // "forward" | "reverse" | "both"
}

type jsonCardQuality struct {
	Name             string   `json:"name"`
	Tier             string   `json:"tier"`
	Reason           string   `json:"reason"`
	Power            int      `json:"power,omitempty"`
	PowerTier        string   `json:"power_tier,omitempty"`
	PowerExplanation string   `json:"power_explanation,omitempty"`
	Detected         string   `json:"detected,omitempty"`
	WhyCut           string   `json:"why_cut,omitempty"`
	Effect           string   `json:"effect,omitempty"`
	Suggested        []string `json:"suggested,omitempty"`
}

type jsonCardPowerLevel struct {
	Name                string   `json:"name"`
	CMC                 int      `json:"cmc"`
	Roles               []string `json:"roles,omitempty"`
	Power               int      `json:"power"`
	PowerTier           string   `json:"power_tier"`
	Explanation         string   `json:"explanation,omitempty"`
	ArchetypeFit        int      `json:"archetype_fit"`
	CMCEfficiency       int      `json:"cmc_efficiency"`
	SynergyContribution int      `json:"synergy_contribution"`
}

type jsonPetCard struct {
	Name      string   `json:"name"`
	CMC       int      `json:"cmc"`
	Roles     []string `json:"roles,omitempty"`
	Power     int      `json:"power"`
	PowerTier string   `json:"power_tier"`
	Reason    string   `json:"reason"`
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
	Primary    string             `json:"primary"`
	Confidence float64            `json:"confidence"`
	Secondary  string             `json:"secondary,omitempty"`
	// Bracket is the declared / rubber-stamp bracket; MeasuredBracket is
	// Freya's signal-computed value. They diverge for wizards/ precons.
	Bracket              int                `json:"bracket"`
	BracketLbl           string             `json:"bracket_label"`
	MeasuredBracket      int                `json:"measured_bracket"`
	MeasuredBracketLabel string             `json:"measured_bracket_label,omitempty"`
	Signals    []string           `json:"signals,omitempty"`
	Intent     string             `json:"intent"`
	Rationale  *jsonBracketRationale `json:"bracket_rationale,omitempty"`
}

type jsonBracketRationale struct {
	FinalBracket int                  `json:"final_bracket"`
	FinalLabel   string               `json:"final_label"`
	RawScore     int                  `json:"raw_score"`
	Signals      []jsonBracketSignal  `json:"signals"`
}

type jsonBracketSignal struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Tier         string   `json:"tier,omitempty"`
	Measurement  string   `json:"measurement,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	Contribution int      `json:"contribution"`
	Note         string   `json:"note,omitempty"`
}

type jsonWinLines struct {
	Lines              []jsonWinLine  `json:"lines"`
	BackupPlans        []string       `json:"backup_plans,omitempty"`
	SinglePoints       []string       `json:"single_points_of_failure,omitempty"`
	Redundancy         map[string]int `json:"redundancy,omitempty"`
	TotalWeightedScore int            `json:"total_weighted_score,omitempty"`
}

type jsonWinLine struct {
	Pieces     []string              `json:"pieces"`
	Type       string                `json:"type"`
	Class      string                `json:"class,omitempty"`
	Desc       string                `json:"description,omitempty"`
	Weight     int                   `json:"weight,omitempty"`
	TutorPaths []jsonTutorChain      `json:"tutor_paths,omitempty"`
	Rationale  *jsonWinLineRationale `json:"rationale,omitempty"`
	DefendedBy []string              `json:"defended_by,omitempty"`
}

type jsonWinLineRationale struct {
	Forms      []string `json:"forms,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
	Resolves   []string `json:"resolves,omitempty"`
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
		TrueInfinites:      comboSlice(r.TrueInfinites),
		Determined:         comboSlice(r.Determined),
		Finishers:          comboSlice(r.Finishers),
		Synergies:          comboSlice(r.Synergies),
		LandCycleSynergies: comboSlice(r.LandCycleSynergies),
		GraveyardLoops:     comboSlice(r.GraveyardLoops),
		ComboInteraction:   comboInteractionToJSON(r.ComboInteraction),
		ComboNotes:    r.ComboNotes,
		ManaCurve: jsonManaCurve{
			Distribution: r.ManaCurve,
			AvgCMC:       r.AvgCMC,
			CurveShape:   r.CurveShape,
			Warnings:     r.CurveWarnings,
			LandCount:    r.LandCount,
			NonlandCount: r.NonlandCount,
		},
		ColorBalance: jsonColors{
			Demand:   demand,
			Supply:   supply,
			Warnings: r.ColorMismatch,
		},
		Profile: jsonProfile{
			Tutors:              r.TutorCount,
			NonLandTutors:       r.NonLandTutorCount,
			LandTutors:          r.LandTutorCount,
			WishTutors:          r.WishTutorCount,
			Removal:             r.RemovalCount,
			SingleTargetRemoval: r.SingleTargetRemovalCount,
			Outlets:             r.OutletCount,
			WinCons:             r.WinConCount,
		},
		FullProfile: buildJSONDeckProfile(r.Profile, r),
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
		// jsonRampCard and RampCard share field shape; T(x) keeps the
		// converter trivially correct and survives field additions on
		// either side — staticcheck S1016.
		ramps[i] = jsonRampCard(rc)
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

func buildJSONDeckProfile(dp *DeckProfile, report *FreyaReport) *jsonDeckProfile {
	if dp == nil {
		return nil
	}
	var roles []jsonRoleCount
	for _, rc := range dp.TopRoles {
		roles = append(roles, jsonRoleCount{Role: string(rc.Role), Count: rc.Count})
	}
	var clusters []jsonCluster
	for _, sc := range dp.SynergyClusters {
		clusters = append(clusters, jsonCluster{
			Name: sc.Name, Cards: sc.Cards, Theme: sc.Theme, Score: sc.Score,
			MemberCount: sc.MemberCount, HighDensity: sc.HighDensity,
		})
	}
	var altBuilds []jsonAltBuild
	for _, a := range dp.AltBuildSuggestions {
		altBuilds = append(altBuilds, jsonAltBuild(a))
	}
	var matchups []jsonMatchup
	for _, m := range dp.MetaMatchups {
		matchups = append(matchups, jsonMatchup{
			Archetype:       m.Archetype,
			Rating:          m.Rating,
			Reason:          m.Reason,
			Strength:        m.Strength,
			ExpectedWinPct:  m.ExpectedWinPct,
			BaselineWinPct:  m.BaselineWinPct,
			EmpiricalGames:  m.EmpiricalGames,
			EmpiricalWinPct: m.EmpiricalWinPct,
			Tilted:          m.Tilted,
			TiltDirection:   m.TiltDirection,
			TiltDelta:       m.TiltDelta,
		})
	}
	var strongAgainst []jsonStrongAgainst
	for _, a := range dp.StrongAgainst {
		strongAgainst = append(strongAgainst, jsonStrongAgainst(a))
	}
	var stars, solid, flex, cuttable []jsonCardQuality
	for _, c := range dp.StarCards {
		// Stars/solid intentionally omit the cuttable-tier rationale
		// fields (Detected/WhyCut/Effect/Suggested) — kept as literal
		// to preserve the field selection. Cuttable's loop below uses
		// a T(x) conversion since it copies all fields anyway.
		stars = append(stars, jsonCardQuality{
			Name: c.Name, Tier: c.Tier, Reason: c.Reason,
			Power: c.Power, PowerTier: c.PowerTier,
			PowerExplanation: c.PowerExplanation,
		})
	}
	for _, c := range dp.SolidCards {
		solid = append(solid, jsonCardQuality{
			Name: c.Name, Tier: c.Tier, Reason: c.Reason,
			Power: c.Power, PowerTier: c.PowerTier,
			PowerExplanation: c.PowerExplanation,
		})
	}
	for _, c := range dp.FlexSlots {
		flex = append(flex, jsonCardQuality{
			Name: c.Name, Tier: c.Tier, Reason: c.Reason,
			Power: c.Power, PowerTier: c.PowerTier,
			PowerExplanation: c.PowerExplanation,
		})
	}
	for _, c := range dp.CuttableCards {
		cuttable = append(cuttable, jsonCardQuality(c))
	}
	var powerLevels []jsonCardPowerLevel
	for _, pl := range dp.CardPowerLevels {
		powerLevels = append(powerLevels, jsonCardPowerLevel(pl))
	}
	var petCards []jsonPetCard
	for _, pc := range dp.PetCards {
		petCards = append(petCards, jsonPetCard(pc))
	}
	var coaching []jsonCoachingTip
	for _, t := range dp.CoachingTips {
		coaching = append(coaching, jsonCoachingTip(t))
	}
	var vulnCombo []jsonVulnerableComboPiece
	for _, v := range dp.VulnerableComboPieces {
		vulnCombo = append(vulnCombo, jsonVulnerableComboPiece(v))
	}
	var downsides []jsonInteractionDownside
	for _, d := range dp.InteractionDownsides {
		downsides = append(downsides, jsonInteractionDownside(d))
	}
	var intPkg *jsonInteractionPackage
	if pkg := dp.InteractionPackage; pkg.Score > 0 || len(pkg.Counterspells)+len(pkg.OpponentInteraction)+len(pkg.Protection)+len(pkg.ProtectionTutors) > 0 {
		intPkg = &jsonInteractionPackage{
			Counterspells:       pkg.Counterspells,
			OpponentInteraction: pkg.OpponentInteraction,
			Protection:          pkg.Protection,
			ProtectionTutors:    pkg.ProtectionTutors,
			Score:               pkg.Score,
		}
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
		SecondaryFit:       dp.SecondaryFit,
		IsBlend:            dp.IsBlend,
		BlendLabel:         dp.BlendLabel,
		Bracket:               dp.Bracket,
		BracketLabel:          dp.BracketLabel,
		MeasuredBracket:       dp.MeasuredBracket,
		MeasuredBracketLabel:  dp.MeasuredBracketLabel,
		Intent:             dp.Intent,
		PrimaryWinLine:     dp.PrimaryWinLine,
		WinLineCount:       dp.WinLineCount,
		BackupCount:        dp.BackupCount,
		HasTutorAccess:     dp.HasTutorAccess,
		SinglePointCount:   dp.SinglePointCount,
		Strengths:          dp.Strengths,
		Weaknesses:         dp.Weaknesses,
		GameplanSummary:    dp.GameplanSummary,
		GameplanScript:     dp.GameplanScript,
		PersonalityBlurb:   dp.PersonalityBlurb,
		PersonalityTagline: dp.PersonalityTagline,
		ManaBaseGrade:      dp.ManaBaseGrade,
		ManaBaseNotes:      dp.ManaBaseNotes,
		TaplandCount:       dp.TaplandCount,
		FetchCount:         dp.FetchCount,
		UtilityLandCount:   dp.UtilityLandCount,
		DeckCostTier:       dp.DeckCostTier,
		EstimatedTotalUSD:  dp.EstimatedTotalUSD,
		PricedCardCount:    dp.PricedCardCount,
		UnpricedCardCount:  dp.UnpricedCardCount,
		DeckCostNote:       dp.DeckCostNote,
		CounterProducers:     dp.CounterProducers,
		CounterPayoffs:       dp.CounterPayoffs,
		CountersMatterTheme:  dp.CountersMatterTheme,
		CountersMatterStrong: dp.CountersMatterStrong,
		VulnerableTo:       dp.VulnerableTo,
		VulnerableComboPieces: vulnCombo,
		ComboMetaInteraction:  comboMetaInteractionToJSON(dp.ComboMetaInteraction),
		InteractionFloor:      comboInteractionFloorToJSON(dp.InteractionFloor),
		ComboTiming:           comboTimingToJSON(dp.ComboTiming),
		KeepableHandPct:                 dp.KeepableHandPct,
		AvgTurnToFourMana:               dp.AvgTurnToFourMana,
		KeepableHandPctAdjusted:         dp.KeepableHandPctAdjusted,
		AvgTurnToCommander:              dp.AvgTurnToCommander,
		KeepableHandPctFreeMull:         dp.KeepableHandPctFreeMull,
		KeepableHandPctAdjustedFreeMull: dp.KeepableHandPctAdjustedFreeMull,
		IsCommanderCentric:      dp.IsCommanderCentric,
		CommanderCentricReason:  dp.CommanderCentricReason,
		CommanderCMC:            dp.CommanderCMC,
		SynergyClusters:    clusters,
		ClusterExport:      BuildClusterExport(dp, report),
		AltBuildSuggestions: altBuilds,
		MetaMatchups:       matchups,
		StrongAgainst:      strongAgainst,
		WorstMatchup:       dp.WorstMatchup,
		TechSuggestions:    dp.TechSuggestions,
		StarCards:           stars,
		SolidCards:          solid,
		FlexSlots:           flex,
		CuttableCards:       cuttable,
		CardPowerLevels:     powerLevels,
		PowerTierCounts:     dp.PowerTierCounts,
		PetCards:            petCards,
		LandSwapSuggestions:    dp.LandSwapSuggestions,
		CurveArchetypeWarnings: dp.CurveArchetypeWarnings,
		CanonicalPreconOverlap: dp.CanonicalPreconOverlap,
		CommanderSynergy:   dp.CommanderSynergy,
		CommanderThemes:    dp.CommanderThemes,
		InteractionQuality: dp.InteractionQuality,
		AdjustedInteractionQuality: dp.AdjustedInteractionQuality,
		InteractionDownsides: downsides,
		InteractionPackage:   intPkg,
		PowerPercentile:    dp.PowerPercentile,
		PowerFactors:       dp.PowerFactors,
		CoachingTips:       coaching,
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
		Bracket:              ac.Bracket,
		BracketLbl:           ac.BracketLabel,
		MeasuredBracket:      ac.MeasuredBracket,
		MeasuredBracketLabel: ac.MeasuredBracketLabel,
		Signals:    ac.Signals,
		Intent:     ac.Intent,
		Rationale:  buildJSONBracketRationale(ac.BracketRationale),
	}
}

func buildJSONBracketRationale(br *BracketRationale) *jsonBracketRationale {
	if br == nil {
		return nil
	}
	out := &jsonBracketRationale{
		FinalBracket: br.FinalBracket,
		FinalLabel:   br.FinalLabel,
		RawScore:     br.RawScore,
		Signals:      make([]jsonBracketSignal, 0, len(br.Signals)),
	}
	for _, s := range br.Signals {
		out.Signals = append(out.Signals, jsonBracketSignal(s))
	}
	return out
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
		var rat *jsonWinLineRationale
		if wl.Rationale != nil {
			rat = &jsonWinLineRationale{
				Forms:      wl.Rationale.Forms,
				Conditions: wl.Rationale.Conditions,
				Resolves:   wl.Rationale.Resolves,
			}
		}
		lines[i] = jsonWinLine{
			Pieces:     wl.Pieces,
			Type:       wl.Type,
			Class:      wl.Class,
			Desc:       wl.Desc,
			Weight:     wl.Weight,
			TutorPaths: paths,
			Rationale:  rat,
			DefendedBy: wl.DefendedBy,
		}
	}
	return &jsonWinLines{
		Lines:              lines,
		BackupPlans:        wla.BackupPlans,
		SinglePoints:       wla.SinglePoints,
		Redundancy:         wla.RedundancyMap,
		TotalWeightedScore: wla.TotalWeightedScore,
	}
}

func comboSlice(combos []ComboResult) []jsonCombo {
	if len(combos) == 0 {
		return []jsonCombo{}
	}
	out := make([]jsonCombo, len(combos))
	for i, c := range combos {
		jc := jsonCombo{
			Cards:            c.Cards,
			LoopType:         c.LoopType,
			Class:            c.Class,
			Resources:        c.Resources,
			Description:      c.Description,
			Confirmed:        c.Confirmed,
			NonDeterministic: c.NonDeterministic,
		}
		if c.Annotation != nil {
			jc.Annotation = &jsonLoopAnnotation{
				PrimaryOutput:   c.Annotation.PrimaryOutput,
				NetProduces:     c.Annotation.NetProduces,
				ExternalEffects: c.Annotation.ExternalEffects,
				Classification:  c.Annotation.Classification,
				Summary:         c.Annotation.Summary,
			}
		}
		out[i] = jc
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

	// Corpus-wide stats — bracket / archetype / curve / density rollup
	// so the user gets a real-world context for the per-deck rows
	// above ("the corpus averages B3.2 / Midrange / 3.4 CMC").
	PrintCorpusStats(w, ComputeCorpusStats(reports))

	// Power-tier rollup — useful for calibrating the S/A/B/C/D
	// thresholds against a real-world corpus. See power_aggregate.go.
	PrintPowerTierAggregate(w, ComputePowerTierAggregate(reports))
}
