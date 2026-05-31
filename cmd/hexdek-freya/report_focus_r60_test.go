package main

import (
	"bytes"
	"strings"
	"testing"
)

// fixtureFocusCombo builds a cEDH Combo deck with named infinite loops,
// star cards, and a single-card finisher win line.
func fixtureFocusCombo() *FreyaReport {
	r := &FreyaReport{
		DeckName:     "thrasios_tymna_thoracle",
		Commander:    "Thrasios, Triton Hero",
		NonlandCount: 64,
		TrueInfinites: []ComboResult{
			{Cards: []string{"Thassa's Oracle", "Demonic Consultation"}, Class: ComboClassLibraryExileWin, Confirmed: true,
				Description: "Cast Consultation naming a card not in deck, exile library, then Thoracle ETB wins on empty library."},
			{Cards: []string{"Dramatic Reversal", "Isochron Scepter"}, Class: ComboClassInfiniteMana, Confirmed: true,
				Description: "Imprint Reversal, infinite mana from rocks."},
		},
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite", Tier: "S"},
				{Pieces: []string{"Dramatic Reversal", "Isochron Scepter"}, Type: "infinite", Tier: "S"},
			},
		},
		Profile: &DeckProfile{
			Commander:            "Thrasios, Triton Hero",
			PrimaryArchetype:     "Combo",
			Bracket:              5,
			BracketLabel:         "cEDH",
			ProtectedKeyPieces:   6,
			UnprotectedKeyPieces: 1,
			CheapInteraction:     12,
			ExpensiveInteraction: 1,
			InteractionQuality:   1.4,
			ManaBaseGrade:        "A",
			PersonalityTagline:   "All the time in the world.",
		},
	}
	return r
}

// fixtureFocusVoltron builds a casual Voltron deck — commander-centric,
// strong commander synergy, low protection, and a single-card commander-
// damage finisher.
func fixtureFocusVoltron() *FreyaReport {
	return &FreyaReport{
		DeckName:     "uril_aura_voltron",
		Commander:    "Uril, the Miststalker",
		NonlandCount: 64,
		Profile: &DeckProfile{
			Commander:              "Uril, the Miststalker",
			PrimaryArchetype:       "Voltron",
			Bracket:                2,
			BracketLabel:           "Core",
			CommanderSynergy:       0.65,
			SynergyCount:           39,
			CommanderThemes:        []string{"aura attached", "hexproof", "+X/+X buff"},
			IsCommanderCentric:     true,
			CommanderCentricReason: "Voltron — 65% of deck synergizes with commander",
			CheapInteraction:       6,
			ExpensiveInteraction:   3,
			InteractionQuality:     2.5,
			ManaBaseGrade:          "B",
			VulnerableTo:           []string{"Imprisoned in the Moon (critical) — turns commander into a colorless land"},
			ProtectedKeyPieces:     3,
			UnprotectedKeyPieces:   5,
			PersonalityTagline:     "Hand Uril the blade.",
		},
	}
}

// fixtureFocusAristocrats builds an Aristocrats deck with named synergy
// clusters (sac outlet + drain payoffs) and a Blood Artist + Phyrexian
// Altar + Gravecrawler win line.
func fixtureFocusAristocrats() *FreyaReport {
	return &FreyaReport{
		DeckName:     "yawgmoth_aristocrats",
		Commander:    "Yawgmoth, Thran Physician",
		NonlandCount: 63,
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Blood Artist", "Phyrexian Altar", "Gravecrawler"}, Type: "infinite", Tier: "A"},
			},
		},
		Profile: &DeckProfile{
			Commander:        "Yawgmoth, Thran Physician",
			PrimaryArchetype: "Aristocrats",
			Bracket:          3,
			BracketLabel:     "Upgraded",
			SynergyClusters: []SynergyCluster{
				{Name: "Death payoffs", Theme: "death_value", Score: 18, MemberCount: 9, Cards: []string{"Blood Artist", "Zulaport Cutthroat", "Pitiless Plunderer"}},
				{Name: "Sac outlets", Theme: "sacrifice_outlet", Score: 12, MemberCount: 6, Cards: []string{"Phyrexian Altar", "Yawgmoth", "Ashnod's Altar"}},
			},
			VulnerableTo:       []string{"Torpor Orb (critical) — shuts down ETB triggers"},
			PersonalityTagline: "Death is the engine.",
		},
	}
}

// fixtureFocusReanimator builds a Reanimator deck where threat assessment
// (vs Rest in Peace / Bojuka Bog) drives most coaching value.
func fixtureFocusReanimator() *FreyaReport {
	return &FreyaReport{
		DeckName:     "meren_reanimator",
		Commander:    "Meren of Clan Nel Toth",
		NonlandCount: 64,
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Razaketh, the Foulblooded"}, Type: "finisher", Tier: "C"},
				{Pieces: []string{"Worldgorger Dragon", "Animate Dead"}, Type: "infinite", Tier: "S"},
			},
		},
		Profile: &DeckProfile{
			Commander:        "Meren of Clan Nel Toth",
			PrimaryArchetype: "Reanimator",
			Bracket:          3,
			BracketLabel:     "Upgraded",
			VulnerableTo: []string{
				"Rest in Peace (critical) — exiles graveyard engine",
				"Leyline of the Void (critical) — prevents graveyard from filling",
				"Bojuka Bog (critical) — colorless one-sided graveyard wipe",
			},
			WorstMatchup: "Stax",
			TechSuggestions: []TechCardSuggestion{
				{Card: "Pithing Needle", Reason: "names a key activated combo piece", OpponentArchetype: "Stax", Severity: 2},
			},
			PersonalityTagline: "Meren remembers.",
		},
	}
}

// fixtureFocusAggro builds an aggro / go-wide deck where win lines + mana
// base lead and synergy clusters are absent.
func fixtureFocusAggro() *FreyaReport {
	return &FreyaReport{
		DeckName:     "krenko_goblins",
		Commander:    "Krenko, Mob Boss",
		NonlandCount: 63,
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Krenko, Mob Boss", "Skirk Prospector", "Goblin Bombardment"}, Type: "finisher", Tier: "C"},
			},
		},
		Profile: &DeckProfile{
			Commander:          "Krenko, Mob Boss",
			PrimaryArchetype:   "Aggro",
			Bracket:            2,
			BracketLabel:       "Core",
			LandCount:          35,
			RecommendedLands:   34,
			RampCount:          8,
			ManaBaseGrade:      "C",
			ManaBaseNotes:      []string{"Mono-red, all basics — no color screw but no fixing either."},
			TaplandCount:       2,
			FetchCount:         0,
			PersonalityTagline: "Faster than fear.",
		},
	}
}

// TestSelectFocusSections_PicksThreeOrFewer pins the count contract.
func TestSelectFocusSections_PicksThreeOrFewer(t *testing.T) {
	cases := []struct {
		name    string
		fixture func() *FreyaReport
	}{
		{"combo", fixtureFocusCombo},
		{"voltron", fixtureFocusVoltron},
		{"aristocrats", fixtureFocusAristocrats},
		{"reanimator", fixtureFocusReanimator},
		{"aggro", fixtureFocusAggro},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.fixture()
			s := SelectFocusSections(r)
			if len(s) > FocusSectionCount {
				t.Errorf("%s: %d sections picked, want <= %d", c.name, len(s), FocusSectionCount)
			}
			if len(s) == 0 {
				t.Errorf("%s: zero sections picked — fixture has content but selector returned nothing", c.name)
			}
		})
	}
}

// TestSelectFocusSections_ArchetypeSpecificOrdering verifies the lead
// section differs by archetype identity. This is the headline kanban
// requirement — different archetypes pick different priority orderings.
func TestSelectFocusSections_ArchetypeSpecificOrdering(t *testing.T) {
	cases := []struct {
		name    string
		fixture func() *FreyaReport
		wantLeads []string // any of these are acceptable as the lead section name
	}{
		{"combo leads with combo lines", fixtureFocusCombo, []string{"Combo Lines"}},
		{"voltron leads with commander synergy", fixtureFocusVoltron, []string{"Commander Synergy"}},
		{"aristocrats leads with synergy clusters", fixtureFocusAristocrats, []string{"Synergy Clusters"}},
		{"reanimator leads with win lines", fixtureFocusReanimator, []string{"Win Lines"}},
		{"aggro leads with win lines", fixtureFocusAggro, []string{"Win Lines"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := c.fixture()
			s := SelectFocusSections(r)
			if len(s) == 0 {
				t.Fatalf("%s: no sections picked", c.name)
			}
			got := s[0].Name
			match := false
			for _, w := range c.wantLeads {
				if got == w {
					match = true
				}
			}
			if !match {
				names := make([]string, len(s))
				for i, x := range s {
					names[i] = x.Name
				}
				t.Errorf("%s: lead section %q, want one of %v (full order: %v)", c.name, got, c.wantLeads, names)
			}
		})
	}
}

// TestSelectFocusSections_OrderingsDifferAcrossArchetypes verifies the
// FULL ordering is distinct across 5 archetypes, not just the lead.
// Five archetypes producing only 2-3 unique orderings means the
// prioritization is too coarse — the test enforces a minimum of 4
// distinct full orderings out of the 5 fixtures.
func TestSelectFocusSections_OrderingsDifferAcrossArchetypes(t *testing.T) {
	fixtures := map[string]func() *FreyaReport{
		"combo":       fixtureFocusCombo,
		"voltron":     fixtureFocusVoltron,
		"aristocrats": fixtureFocusAristocrats,
		"reanimator":  fixtureFocusReanimator,
		"aggro":       fixtureFocusAggro,
	}
	seen := map[string]string{}
	for name, fx := range fixtures {
		s := SelectFocusSections(fx())
		parts := make([]string, len(s))
		for i, x := range s {
			parts[i] = x.Name
		}
		ordering := strings.Join(parts, "|")
		if existing, ok := seen[ordering]; ok && existing != name {
			t.Logf("%s and %s share ordering %q (allowed; we require ≥4 distinct overall)", name, existing, ordering)
		}
		seen[ordering] = name
	}
	if len(seen) < 4 {
		t.Errorf("only %d distinct orderings across 5 archetypes, want >= 4 — prioritization is too coarse: %v", len(seen), seen)
	}
}

// TestPrintFocusText_Under25Lines pins the headline length contract.
// Every archetype fixture must render in 25 lines or fewer.
func TestPrintFocusText_Under25Lines(t *testing.T) {
	fixtures := map[string]func() *FreyaReport{
		"combo":       fixtureFocusCombo,
		"voltron":     fixtureFocusVoltron,
		"aristocrats": fixtureFocusAristocrats,
		"reanimator":  fixtureFocusReanimator,
		"aggro":       fixtureFocusAggro,
	}
	for name, fx := range fixtures {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			printFocusText(&buf, fx())
			text := buf.String()
			n := strings.Count(text, "\n")
			if n > 25 {
				t.Errorf("%s: focus report is %d lines, want <= 25:\n%s", name, n, text)
			}
			if n < 5 {
				t.Errorf("%s: focus report is only %d lines, suspiciously empty:\n%s", name, n, text)
			}
		})
	}
}

// TestPrintFocusText_SkipsEmptySections defends against the "empty
// section header" failure mode — a section that scored highest by
// archetype priority but has no data must drop out entirely.
func TestPrintFocusText_SkipsEmptySections(t *testing.T) {
	// Reanimator fixture has no SynergyClusters and no
	// InteractionPackage; the prioritization would otherwise score
	// some of those sections, but they should not appear.
	var buf bytes.Buffer
	printFocusText(&buf, fixtureFocusReanimator())
	text := buf.String()

	// Look for unexpected headers — any header that corresponds to a
	// section the fixture has no data for.
	forbidden := []string{
		"[Synergy Clusters]",      // empty in fixture
		"[Commander Synergy]",     // empty in fixture (no CommanderSynergy / Themes)
		"[Mana Base]",             // empty in fixture (no ManaBaseGrade)
		"[Interaction]",           // empty in fixture (no Cheap/Expensive)
		"[Protection]",            // empty in fixture (zero key pieces)
		"[Meta Positioning]",      // empty in fixture (no MetaMatchups)
		"[Combo Lines]",           // empty in fixture (no TrueInfinites / Determined)
	}
	for _, h := range forbidden {
		if strings.Contains(text, h) {
			t.Errorf("Reanimator focus report contains forbidden empty-section header %q:\n%s", h, text)
		}
	}
}

// TestPrintFocusText_AlwaysShowsBanner verifies the banner is present
// and the personality tagline is rendered when populated. Without the
// banner the focus report would just be a wall of bracketed sections
// with no archetype/commander grounding.
func TestPrintFocusText_AlwaysShowsBanner(t *testing.T) {
	var buf bytes.Buffer
	printFocusText(&buf, fixtureFocusAristocrats())
	text := buf.String()

	if !strings.Contains(text, "FREYA -- Focus Mode") {
		t.Errorf("focus banner missing")
	}
	if !strings.Contains(text, "Yawgmoth, Thran Physician") {
		t.Errorf("commander not in banner")
	}
	if !strings.Contains(text, "Aristocrats") {
		t.Errorf("archetype not in banner")
	}
	if !strings.Contains(text, "Death is the engine.") {
		t.Errorf("personality tagline not in banner")
	}
}

// TestSelectFocusSections_NilReportSafe defends against panics on
// degenerate input — focus mode should produce empty output, not crash.
func TestSelectFocusSections_NilReportSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on nil report: %v", r)
		}
	}()
	s := SelectFocusSections(nil)
	if len(s) != 0 {
		t.Errorf("nil report should produce zero sections, got %d", len(s))
	}
}

// TestSelectFocusSections_BodyTrimmedToCap verifies each rendered
// section body is capped at SectionMaxBodyLines so a deck with 20
// vulnerable hosers doesn't blow the 25-line ceiling on a single
// section.
func TestSelectFocusSections_BodyTrimmedToCap(t *testing.T) {
	r := &FreyaReport{
		Profile: &DeckProfile{
			PrimaryArchetype: "Reanimator",
			VulnerableTo: []string{
				"hoser 1", "hoser 2", "hoser 3", "hoser 4",
				"hoser 5", "hoser 6", "hoser 7", "hoser 8",
			},
		},
	}
	s := SelectFocusSections(r)
	for _, sec := range s {
		if len(sec.BodyLines) > SectionMaxBodyLines {
			t.Errorf("section %s has %d body lines, want <= %d", sec.Name, len(sec.BodyLines), SectionMaxBodyLines)
		}
	}
}

// TestFocusMode_DispatchedFromPrintReport verifies the public PrintReport
// entry point honors the FocusMode toggle. Without this we could ship
// the focus renderer + wire the flag but accidentally leave the
// dispatch broken so the full report still emits.
func TestFocusMode_DispatchedFromPrintReport(t *testing.T) {
	saved := FocusMode
	t.Cleanup(func() { FocusMode = saved })

	r := fixtureFocusCombo()

	var fullBuf bytes.Buffer
	FocusMode = "full"
	PrintReport(&fullBuf, r, "text")

	var focusBuf bytes.Buffer
	FocusMode = "focus"
	PrintReport(&focusBuf, r, "text")

	if !strings.Contains(focusBuf.String(), "FREYA -- Focus Mode") {
		t.Errorf("focus mode banner missing from PrintReport output:\n%s", focusBuf.String())
	}
	if strings.Contains(fullBuf.String(), "FREYA -- Focus Mode") {
		t.Errorf("full-mode output should not contain focus banner")
	}
	// Sanity: focus should be much shorter than full.
	focusLines := strings.Count(focusBuf.String(), "\n")
	fullLines := strings.Count(fullBuf.String(), "\n")
	if focusLines >= fullLines {
		t.Errorf("focus (%d lines) not shorter than full (%d lines)", focusLines, fullLines)
	}
}
