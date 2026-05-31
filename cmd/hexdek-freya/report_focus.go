package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Focus-mode text report — under-25-line prioritized summary.
//
// Selects the 3 most insight-bearing sections for a specific deck based on
// archetype identity + content presence, and renders each in a compact 4-6
// line form. Designed for terminal scanning and chat-channel pastes where
// the full text report (~200 lines) is too long.
// ---------------------------------------------------------------------------

// focusSection is one renderable section candidate. Score is the sort key
// (higher = render earlier); BodyLines is the compact body (each entry is
// one rendered line, NOT counting the header line). The renderer joins
// header + body lines + a blank separator.
type focusSection struct {
	Name      string
	Score     int
	BodyLines []string
}

// SectionMaxBodyLines bounds each focused section to keep the total
// report under the 25-line ceiling. Header + 5 body lines + blank
// separator = 7 lines per section; 3 sections = 21 lines + 4-line
// banner = 25 lines.
const SectionMaxBodyLines = 5

// FocusSectionCount is the number of sections picked per report.
const FocusSectionCount = 3

// focusPriorityByArchetype encodes each archetype's "what matters most"
// ranking. Archetype identity drives which sections lead — a Combo deck
// leads with combo lines; a Voltron deck leads with commander synergy.
// Numbers are base scores (0-100). Sections still need to have content
// to actually render; absentees score 0.
//
// Each archetype lists 4-5 candidate sections so that if the deck's
// top-priority section is empty (e.g. a Combo deck with no detected
// infinite loops), the next-best section takes its slot.
var focusPriorityByArchetype = map[string]map[string]int{
	"Combo":            {"combo_lines": 100, "win_lines": 90, "protection": 80, "interaction": 70, "tech": 60},
	"Storm":            {"combo_lines": 100, "win_lines": 90, "interaction": 75, "protection": 65, "mana_base": 50},
	"Voltron":          {"commander_synergy": 100, "threat": 85, "interaction": 75, "protection": 70, "tech": 60},
	"Aristocrats":      {"synergy_clusters": 100, "win_lines": 85, "threat": 75, "combo_lines": 70, "tech": 60},
	"Reanimator":       {"win_lines": 100, "threat": 90, "tech": 80, "synergy_clusters": 70, "combo_lines": 60},
	"Aggro":            {"win_lines": 100, "mana_base": 85, "threat": 75, "interaction": 60, "tech": 55},
	"Tribal":           {"synergy_clusters": 100, "commander_synergy": 85, "win_lines": 70, "mana_base": 65, "threat": 60},
	"Control":          {"interaction": 100, "win_lines": 85, "meta_positioning": 75, "tech": 70, "threat": 60},
	"Stax":             {"synergy_clusters": 100, "meta_positioning": 85, "mana_base": 75, "interaction": 65, "tech": 55},
	"Lands Matter":     {"win_lines": 100, "mana_base": 95, "synergy_clusters": 85, "threat": 60, "tech": 50},
	"Spellslinger":     {"interaction": 100, "win_lines": 90, "combo_lines": 75, "threat": 65, "tech": 60},
	"Enchantress":      {"synergy_clusters": 100, "threat": 90, "win_lines": 80, "interaction": 65, "tech": 70},
	"Lifegain":         {"synergy_clusters": 100, "win_lines": 80, "threat": 75, "interaction": 60, "tech": 55},
	"Counters Matter":  {"synergy_clusters": 100, "win_lines": 80, "threat": 75, "combo_lines": 65, "tech": 60},
	"Blink":            {"synergy_clusters": 100, "win_lines": 80, "threat": 75, "interaction": 65, "tech": 60},
	"Mill":             {"win_lines": 100, "threat": 85, "interaction": 80, "synergy_clusters": 70, "tech": 60},
	"Superfriends":     {"win_lines": 100, "interaction": 85, "synergy_clusters": 70, "protection": 65, "tech": 55},
	"Group Hug":        {"meta_positioning": 100, "interaction": 80, "win_lines": 70, "threat": 60, "tech": 50},
	"Group Slug":       {"win_lines": 100, "synergy_clusters": 80, "interaction": 65, "tech": 60, "mana_base": 50},
	"Pillowfort":       {"interaction": 100, "win_lines": 85, "threat": 70, "mana_base": 55, "tech": 50},
	"Toxic":            {"win_lines": 100, "synergy_clusters": 85, "threat": 70, "interaction": 60, "tech": 55},
	"Discard":          {"synergy_clusters": 100, "interaction": 85, "win_lines": 70, "threat": 60, "tech": 55},
	"Extra Combats":    {"win_lines": 100, "synergy_clusters": 85, "interaction": 70, "mana_base": 60, "tech": 55},
	"Theft / Clone":    {"synergy_clusters": 100, "interaction": 85, "win_lines": 75, "threat": 60, "tech": 55},
	"Artifacts":        {"synergy_clusters": 100, "win_lines": 85, "threat": 80, "combo_lines": 70, "tech": 65},
	"Damage Redirect":  {"interaction": 100, "synergy_clusters": 80, "win_lines": 70, "threat": 65, "tech": 55},
	"Selfmill":         {"synergy_clusters": 100, "win_lines": 85, "threat": 80, "combo_lines": 70, "tech": 60},
	"Midrange":         {"win_lines": 100, "interaction": 80, "mana_base": 70, "threat": 65, "tech": 55},
	"Vehicles":         {"win_lines": 100, "synergy_clusters": 80, "interaction": 70, "mana_base": 65, "tech": 55},
	"Cycling":          {"synergy_clusters": 100, "win_lines": 80, "interaction": 70, "threat": 60, "tech": 55},
}

// focusPriorityDefault is used when the deck's archetype is unknown or
// not present in focusPriorityByArchetype. A generic prioritization
// that surfaces win lines + mana base + threat — the three things that
// matter on essentially any commander deck.
var focusPriorityDefault = map[string]int{
	"win_lines": 100,
	"mana_base": 75,
	"threat":    65,
	"tech":      55,
}

// SelectFocusSections returns the top FocusSectionCount sections for
// this report, ranked by (archetype priority × content presence).
// Empty sections drop out — even at high priority, a section with no
// content scores 0 and is skipped. Returns a slice with Name+Body
// populated; the caller renders each one.
func SelectFocusSections(r *FreyaReport) []focusSection {
	if r == nil {
		return nil
	}
	priorities := focusPriorityDefault
	if r.Profile != nil {
		if p, ok := focusPriorityByArchetype[r.Profile.PrimaryArchetype]; ok {
			priorities = p
		}
	}

	builders := map[string]func(*FreyaReport) []string{
		"combo_lines":       focusComboLinesBody,
		"win_lines":         focusWinLinesBody,
		"protection":        focusProtectionBody,
		"commander_synergy": focusCommanderSynergyBody,
		"threat":            focusThreatBody,
		"interaction":       focusInteractionBody,
		"mana_base":         focusManaBaseBody,
		"meta_positioning":  focusMetaPositioningBody,
		"tech":              focusTechBody,
		"synergy_clusters":  focusSynergyClustersBody,
	}

	var candidates []focusSection
	for key, base := range priorities {
		build, ok := builders[key]
		if !ok {
			continue
		}
		body := build(r)
		if len(body) == 0 {
			continue
		}
		if len(body) > SectionMaxBodyLines {
			body = body[:SectionMaxBodyLines]
		}
		candidates = append(candidates, focusSection{
			Name:      focusSectionTitle(key),
			Score:     base,
			BodyLines: body,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].Name < candidates[j].Name
	})

	if len(candidates) > FocusSectionCount {
		candidates = candidates[:FocusSectionCount]
	}
	return candidates
}

// focusSectionTitle is the human-facing header for each section key.
// Kept short so the 25-line ceiling has room for content.
func focusSectionTitle(key string) string {
	switch key {
	case "combo_lines":
		return "Combo Lines"
	case "win_lines":
		return "Win Lines"
	case "protection":
		return "Protection"
	case "commander_synergy":
		return "Commander Synergy"
	case "threat":
		return "Threat Assessment"
	case "interaction":
		return "Interaction"
	case "mana_base":
		return "Mana Base"
	case "meta_positioning":
		return "Meta Positioning"
	case "tech":
		return "Tech Cards"
	case "synergy_clusters":
		return "Synergy Clusters"
	}
	return key
}

// printFocusText renders the under-25-line focus report. Banner +
// 3 sections + closing tagline.
func printFocusText(w io.Writer, r *FreyaReport) {
	dp := r.Profile
	// Banner — 3-4 lines.
	fmt.Fprintf(w, "FREYA -- Focus Mode\n")
	if dp != nil {
		fmt.Fprintf(w, "Deck: %s   Bracket %d (%s)   %s\n",
			focusDeckLabel(r), dp.Bracket, focusOrDash(dp.BracketLabel), focusOrDash(dp.PrimaryArchetype))
	} else {
		fmt.Fprintf(w, "Deck: %s\n", focusDeckLabel(r))
	}
	if r.Commander != "" {
		fmt.Fprintf(w, "Commander: %s\n", r.Commander)
	}
	if dp != nil && dp.PersonalityTagline != "" {
		fmt.Fprintf(w, "  — %s\n", dp.PersonalityTagline)
	}
	fmt.Fprintf(w, "\n")

	// Sections — 3 × (1 header + up to 5 body + 1 blank) = 21 lines.
	sections := SelectFocusSections(r)
	for _, s := range sections {
		fmt.Fprintf(w, "[%s]\n", s.Name)
		for _, line := range s.BodyLines {
			fmt.Fprintf(w, "  %s\n", line)
		}
		fmt.Fprintf(w, "\n")
	}
}

func focusDeckLabel(r *FreyaReport) string {
	if r.DeckName != "" {
		return r.DeckName
	}
	if r.DeckPath != "" {
		return r.DeckPath
	}
	return "(unnamed)"
}

func focusOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// --- Section body builders ---
//
// Each returns up to SectionMaxBodyLines strings (5). An empty slice
// means "no content; skip this section." The selector treats empty
// returns as score-0 and they don't appear in the output.

func focusComboLinesBody(r *FreyaReport) []string {
	var out []string
	for i, c := range r.TrueInfinites {
		if i >= 3 {
			break
		}
		classTag := ""
		if label := ComboClassLabel(c.Class); label != "" {
			classTag = " [" + label + "]"
		}
		out = append(out, fmt.Sprintf("∞ %s%s", strings.Join(c.Cards, " + "), classTag))
	}
	if len(out) < SectionMaxBodyLines {
		for i, c := range r.Determined {
			if len(out) >= SectionMaxBodyLines || i >= 3 {
				break
			}
			out = append(out, fmt.Sprintf("○ %s [determined]", strings.Join(c.Cards, " + ")))
		}
	}
	return out
}

func focusWinLinesBody(r *FreyaReport) []string {
	if r.WinLines == nil || len(r.WinLines.WinLines) == 0 {
		return nil
	}
	var out []string
	for i, wl := range r.WinLines.WinLines {
		if i >= 5 {
			break
		}
		pieces := strings.Join(wl.Pieces, " + ")
		if pieces == "" {
			continue
		}
		typeTag := wl.Type
		if wl.Tier != "" {
			typeTag = fmt.Sprintf("Tier %s/%s", wl.Tier, wl.Type)
		}
		out = append(out, fmt.Sprintf("• %s — %s", pieces, typeTag))
	}
	return out
}

func focusProtectionBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil {
		return nil
	}
	if dp.ProtectedKeyPieces == 0 && dp.UnprotectedKeyPieces == 0 {
		return nil
	}
	var out []string
	out = append(out, fmt.Sprintf("%d protected key pieces / %d unprotected",
		dp.ProtectedKeyPieces, dp.UnprotectedKeyPieces))
	for i, v := range dp.VulnerableComboPieces {
		if i >= 3 {
			break
		}
		out = append(out, fmt.Sprintf("⚠ %s — %s", v.Name, v.Reason))
	}
	if len(out) == 1 && r.NonlandCount > 0 {
		// Only the summary line — add a single follow-up so the section
		// isn't a one-liner.
		out = append(out, "(no vulnerable-piece notes — protection layer is uniform)")
	}
	return out
}

func focusCommanderSynergyBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil || dp.Commander == "" {
		return nil
	}
	if dp.CommanderSynergy == 0 && len(dp.CommanderThemes) == 0 {
		return nil
	}
	var out []string
	out = append(out, fmt.Sprintf("Synergy %d%% — %d / 99 cards align with %s",
		int(dp.CommanderSynergy*100), dp.SynergyCount, dp.Commander))
	if len(dp.CommanderThemes) > 0 {
		themes := dp.CommanderThemes
		if len(themes) > 4 {
			themes = themes[:4]
		}
		out = append(out, fmt.Sprintf("Themes: %s", strings.Join(themes, ", ")))
	}
	if dp.IsCommanderCentric {
		out = append(out, fmt.Sprintf("Commander-centric: %s", dp.CommanderCentricReason))
	}
	return out
}

func focusThreatBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil || len(dp.VulnerableTo) == 0 {
		return nil
	}
	var out []string
	for i, v := range dp.VulnerableTo {
		if i >= SectionMaxBodyLines {
			break
		}
		out = append(out, "▼ "+v)
	}
	return out
}

func focusInteractionBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil {
		return nil
	}
	if dp.CheapInteraction == 0 && dp.ExpensiveInteraction == 0 {
		return nil
	}
	var out []string
	out = append(out, fmt.Sprintf("Cheap (CMC ≤2): %d   Expensive (CMC ≥4): %d   Avg CMC: %.1f",
		dp.CheapInteraction, dp.ExpensiveInteraction, dp.InteractionQuality))
	pkg := dp.InteractionPackage
	if len(pkg.Counterspells) > 0 {
		counters := pkg.Counterspells
		if len(counters) > 3 {
			counters = counters[:3]
		}
		out = append(out, "Counterspells: "+strings.Join(counters, ", "))
	}
	if len(pkg.OpponentInteraction) > 0 {
		rem := pkg.OpponentInteraction
		if len(rem) > 3 {
			rem = rem[:3]
		}
		out = append(out, "Opponent interaction: "+strings.Join(rem, ", "))
	}
	for _, d := range dp.InteractionDownsides {
		if len(out) >= SectionMaxBodyLines {
			break
		}
		out = append(out, "⚠ "+d.Note)
	}
	return out
}

func focusManaBaseBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil {
		return nil
	}
	if dp.ManaBaseGrade == "" {
		return nil
	}
	var out []string
	out = append(out, fmt.Sprintf("Grade %s   Lands %d (rec %d)   Ramp %d   Taplands %d   Fetches %d",
		dp.ManaBaseGrade, dp.LandCount, dp.RecommendedLands, dp.RampCount, dp.TaplandCount, dp.FetchCount))
	for i, n := range dp.ManaBaseNotes {
		if i >= 3 {
			break
		}
		out = append(out, "• "+n)
	}
	return out
}

func focusMetaPositioningBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil || len(dp.MetaMatchups) == 0 {
		return nil
	}
	// Sort matchups by ExpectedWinPct ascending — worst-first so the
	// reader sees the gap they need to close.
	sorted := append([]MetaMatchup(nil), dp.MetaMatchups...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].ExpectedWinPct < sorted[j].ExpectedWinPct
	})
	var out []string
	for i, m := range sorted {
		if i >= 4 {
			break
		}
		icon := "≈"
		if m.Rating == "favored" {
			icon = "▲"
		} else if m.Rating == "unfavored" {
			icon = "▼"
		}
		tilt := ""
		if m.Tilted {
			delta := m.TiltDelta
			if delta < 0 {
				delta = -delta
			}
			tilt = fmt.Sprintf(" (tilted %s %dpp)", m.TiltDirection, delta)
		}
		out = append(out, fmt.Sprintf("%s vs %s: %d%%%s", icon, m.Archetype, m.ExpectedWinPct, tilt))
	}
	return out
}

func focusTechBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil || len(dp.TechSuggestions) == 0 {
		return nil
	}
	var out []string
	out = append(out, fmt.Sprintf("Worst matchup: %s", dp.WorstMatchup))
	for i, s := range dp.TechSuggestions {
		if i >= 4 {
			break
		}
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
		out = append(out, fmt.Sprintf("• %s%s [%s] — %s", s.Card, tag, sev, s.Reason))
	}
	return out
}

func focusSynergyClustersBody(r *FreyaReport) []string {
	dp := r.Profile
	if dp == nil || len(dp.SynergyClusters) == 0 {
		return nil
	}
	clusters := append([]SynergyCluster(nil), dp.SynergyClusters...)
	sort.SliceStable(clusters, func(i, j int) bool { return clusters[i].Score > clusters[j].Score })
	var out []string
	for i, c := range clusters {
		if i >= 3 || len(out) >= SectionMaxBodyLines {
			break
		}
		members := c.Cards
		if len(members) > 3 {
			members = members[:3]
		}
		theme := c.Theme
		if theme == "" {
			theme = c.Name
		}
		out = append(out, fmt.Sprintf("• %s (score %d, %d cards): %s",
			theme, c.Score, c.MemberCount, strings.Join(members, ", ")))
	}
	return out
}
