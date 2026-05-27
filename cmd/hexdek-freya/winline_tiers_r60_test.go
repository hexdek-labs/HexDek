package main

import (
	"strings"
	"testing"
)

// winline_tiers_r60_test.go — pins the S/A/B/C/D win-line tier ladder
// added to WinLine + the "Tier X:" prefix surfaced in
// WinLineRationale.Resolves. Three surfaces tested:
//
//   1. classifyWinLineTier returns the expected letter for each of
//      the 5 tiers across 5+ canonical example shapes per tier.
//   2. addComboWinLines stamps Tier on every WinLine and the
//      rationale Resolves carries the "Tier X:" prefix.
//   3. Non-combo win lines (alt_wincon / commander_damage / combat)
//      all land at tier D with the prefix surfaced.

// -----------------------------------------------------------------------------
// 1. classifyWinLineTier — 5+ canonical examples per tier
// -----------------------------------------------------------------------------

func TestClassifyWinLineTier_S_TwoCardTrueInfinite(t *testing.T) {
	// S = 2-card true-infinite combos. The shape that matters is
	// Type=="infinite" + len(Pieces)<=2. Class doesn't matter for
	// S-tier — once both pieces hit the table the game closes.
	cases := []struct {
		name   string
		pieces []string
		class  string
	}{
		{"Thoracle + Consultation",
			[]string{"Thassa's Oracle", "Demonic Consultation"},
			ComboClassLibraryExileWin},
		{"Heliod + Walking Ballista",
			[]string{"Heliod, Sun-Crowned", "Walking Ballista"},
			ComboClassInfiniteDamage},
		{"Isochron Scepter + Dramatic Reversal",
			[]string{"Isochron Scepter", "Dramatic Reversal"},
			ComboClassInfiniteMana},
		{"Splinter Twin + Pestermite",
			[]string{"Splinter Twin", "Pestermite"},
			ComboClassInfiniteTokens},
		{"Kiki-Jiki + Felidar Guardian",
			[]string{"Kiki-Jiki, Mirror Breaker", "Felidar Guardian"},
			ComboClassInfiniteTokens},
		{"Devoted Druid + Vizier of Remedies",
			[]string{"Devoted Druid", "Vizier of Remedies"},
			ComboClassInfiniteMana},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wl := &WinLine{Type: "infinite", Pieces: c.pieces, Class: c.class}
			if got := classifyWinLineTier(wl); got != "S" {
				t.Errorf("classifyWinLineTier(%q) = %q, want %q", c.name, got, "S")
			}
		})
	}
}

func TestClassifyWinLineTier_A_ThreePlusCardTrueInfinite(t *testing.T) {
	// A = 3+-card true-infinite combos. Same Type=="infinite" but
	// requires an extra piece (usually sac outlet, mana producer,
	// or targeting piece) on top of the 2-card core.
	cases := []struct {
		name   string
		pieces []string
		class  string
	}{
		{"Worldgorger Dragon + Animate Dead + sac outlet",
			[]string{"Worldgorger Dragon", "Animate Dead", "Viscera Seer"},
			ComboClassInfiniteMana},
		{"Reveillark + Karmic Guide + Viscera Seer",
			[]string{"Reveillark", "Karmic Guide", "Viscera Seer"},
			ComboClassInfiniteETB},
		{"persist creature + Melira + sac outlet",
			[]string{"Murderous Redcap", "Melira, Sylvok Outcast", "Goblin Bombardment"},
			ComboClassInfiniteDamage},
		{"Sanguine Bond + Exquisite Blood + life-gain enabler",
			[]string{"Sanguine Bond", "Exquisite Blood", "Soul's Attendant"},
			ComboClassInfiniteDrain},
		{"4-card grindy ETB infinite",
			[]string{"Eldrazi Displacer", "Cloudblazer", "Sanctum Weaver", "Bear Umbra"},
			ComboClassInfiniteETB},
		{"5-card mass-resurrection chain",
			[]string{"Mikaeus, the Unhallowed", "Triskelion", "Phyrexian Altar", "Reassembling Skeleton", "Necropotence"},
			ComboClassInfiniteDamage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wl := &WinLine{Type: "infinite", Pieces: c.pieces, Class: c.class}
			if got := classifyWinLineTier(wl); got != "A" {
				t.Errorf("classifyWinLineTier(%q, %d pieces) = %q, want %q",
					c.name, len(c.pieces), got, "A")
			}
		})
	}
}

func TestClassifyWinLineTier_B_TwoCardCategoricalWin(t *testing.T) {
	// B = 2-card categorical-win combo: Type=="determined" AND
	// len(Pieces)==2 AND Class is in twoCardCategoricalWinClasses
	// (InfiniteDamage, InfiniteDrain, InfiniteMill,
	// LibraryExileWin, CombatFinisher, StormFinisher,
	// InfiniteTokens). Deterministic kill within one combat step
	// but not strictly infinite-loop until SBAs resolve.
	cases := []struct {
		name   string
		pieces []string
		class  string
	}{
		{"Hellkite Charger + Bear Umbra",
			[]string{"Hellkite Charger", "Bear Umbra"},
			ComboClassCombatFinisher},
		{"Aggravated Assault + Sword of Feast and Famine",
			[]string{"Aggravated Assault", "Sword of Feast and Famine"},
			ComboClassCombatFinisher},
		{"Bloodchief Ascension + Mindcrank",
			[]string{"Bloodchief Ascension", "Mindcrank"},
			ComboClassInfiniteMill},
		{"Necrotic Ooze + Devoted Druid + Channeler Initiate",
			// Two-card simplification: Necrotic Ooze + Devoted Druid
			// closes via the InfiniteDamage class (Triskelion in GY)
			[]string{"Necrotic Ooze", "Triskelion"},
			ComboClassInfiniteDamage},
		{"Mike + Trike",
			[]string{"Mikaeus, the Unhallowed", "Triskelion"},
			ComboClassInfiniteDamage},
		{"Painter's Servant + Grindstone",
			[]string{"Painter's Servant", "Grindstone"},
			ComboClassInfiniteMill},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wl := &WinLine{Type: "determined", Pieces: c.pieces, Class: c.class}
			if got := classifyWinLineTier(wl); got != "B" {
				t.Errorf("classifyWinLineTier(%q, class %q) = %q, want %q",
					c.name, c.class, got, "B")
			}
		})
	}
}

func TestClassifyWinLineTier_C_NCardValuePileFinisher(t *testing.T) {
	// C = N-card value-pile finisher OR single-card finisher line.
	// Two sub-shapes:
	//   - Type=="determined" with len(Pieces)!=2 OR Class not in
	//     the twoCardCategoricalWinClasses set
	//   - Type=="finisher" (single-card finisher)
	cases := []struct {
		name   string
		typ    string
		pieces []string
		class  string
	}{
		// 3+ piece determined assemblies
		{"Aetherflux Reservoir + storm package (3-card grind)",
			"determined",
			[]string{"Aetherflux Reservoir", "Sensei's Divining Top", "Bolas's Citadel"},
			ComboClassStormFinisher},
		{"Doomsday + Thoracle + setup (3 pieces)",
			"determined",
			[]string{"Doomsday", "Thassa's Oracle", "Lim-Dul's Vault"},
			ComboClassLibraryExileWin},
		// 2-card determined but NOT in twoCardCategoricalWinClasses
		{"InfiniteETB pile (2 cards, not categorical-win)",
			"determined",
			[]string{"Peregrine Drake", "Deadeye Navigator"},
			ComboClassInfiniteETB},
		{"BlinkEngine value pile (not categorical-win class)",
			"determined",
			[]string{"Brago, King Eternal", "Strionic Resonator"},
			ComboClassBlinkEngine},
		// Single-card "finisher" cards
		{"Craterhoof Behemoth finisher",
			"finisher",
			[]string{"Craterhoof Behemoth"},
			""},
		{"Avenger of Zendikar finisher",
			"finisher",
			[]string{"Avenger of Zendikar"},
			""},
		{"Approach of the Second Sun finisher",
			"finisher",
			[]string{"Approach of the Second Sun"},
			""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wl := &WinLine{Type: c.typ, Pieces: c.pieces, Class: c.class}
			if got := classifyWinLineTier(wl); got != "C" {
				t.Errorf("classifyWinLineTier(%q, type %q, %d pieces, class %q) = %q, want %q",
					c.name, c.typ, len(c.pieces), c.class, got, "C")
			}
		})
	}
}

func TestClassifyWinLineTier_D_IncidentalFinish(t *testing.T) {
	// D = incidental finish: combat damage / commander damage /
	// alt-wincon enchantments with no assembled combo line.
	cases := []struct {
		name   string
		typ    string
		pieces []string
	}{
		{"21 commander damage Voltron",
			"commander_damage",
			[]string{"Uril, the Miststalker"}},
		{"21 commander damage partner pair",
			"commander_damage",
			[]string{"Wyleth, Soul of Steel"}},
		{"Combat damage with creature pressure",
			"combat",
			[]string{"15 threats + 3 pumps"}},
		{"Mass beats — high threat count",
			"combat",
			[]string{"20 threats + 0 pumps"}},
		{"Maze's End alt-wincon (no combo line)",
			"alt_wincon",
			[]string{"Maze's End"}},
		{"Mortal Combat alt-wincon",
			"alt_wincon",
			[]string{"Mortal Combat"}},
		{"Coalition Victory alt-wincon",
			"alt_wincon",
			[]string{"Coalition Victory"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wl := &WinLine{Type: c.typ, Pieces: c.pieces}
			if got := classifyWinLineTier(wl); got != "D" {
				t.Errorf("classifyWinLineTier(%q, type %q) = %q, want %q",
					c.name, c.typ, got, "D")
			}
		})
	}
}

func TestClassifyWinLineTier_NilSafe(t *testing.T) {
	if got := classifyWinLineTier(nil); got != "D" {
		t.Errorf("classifyWinLineTier(nil) = %q, want %q", got, "D")
	}
}

// -----------------------------------------------------------------------------
// 2. addComboWinLines wires Tier + prefixes rationale Resolves
// -----------------------------------------------------------------------------

func TestAddComboWinLines_TierStampedAndPrefixedInRationale(t *testing.T) {
	wla := &WinLineAnalysis{RedundancyMap: map[string]int{}}
	combos := []ComboResult{
		{
			Cards:       []string{"Thassa's Oracle", "Demonic Consultation"},
			Class:       ComboClassLibraryExileWin,
			Description: "Cast Consultation naming a card not in deck → empty library → Thoracle ETB wins.",
		},
		{
			Cards:       []string{"Hellkite Charger", "Bear Umbra"},
			Class:       ComboClassCombatFinisher,
			Description: "Combat untap loop via Bear Umbra mana refund.",
		},
	}
	addComboWinLines(wla, combos, "infinite", nil, nil, nil) // Thoracle goes through "infinite"
	addComboWinLines(wla, combos[1:], "determined", nil, nil, nil) // Hellkite as determined

	if len(wla.WinLines) != 3 {
		t.Fatalf("expected 3 win lines, got %d", len(wla.WinLines))
	}

	// First win line: Thoracle infinite — should be Tier S.
	wlS := wla.WinLines[0]
	if wlS.Tier != "S" {
		t.Errorf("Thoracle WinLine.Tier = %q, want %q", wlS.Tier, "S")
	}
	if wlS.Rationale == nil || len(wlS.Rationale.Resolves) == 0 {
		t.Fatal("Thoracle WinLine missing Rationale.Resolves")
	}
	if !strings.HasPrefix(wlS.Rationale.Resolves[0], "Tier S:") {
		t.Errorf("Thoracle Resolves[0] = %q, want prefix %q", wlS.Rationale.Resolves[0], "Tier S:")
	}

	// Second win line: Hellkite Charger initially classed as
	// "infinite" Type (passed via the first call) — should be S
	// because len(pieces)==2 and Type=="infinite". The tier ladder
	// doesn't gate on class for infinite; both 2-card infinite
	// shapes are S regardless of what they produce.
	wlSiblng := wla.WinLines[1]
	if wlSiblng.Tier != "S" {
		t.Errorf("Hellkite as infinite WinLine.Tier = %q, want %q (Type=infinite + 2 pieces)",
			wlSiblng.Tier, "S")
	}

	// Third win line: Hellkite Charger as "determined" — should be
	// Tier B because Class is CombatFinisher (in
	// twoCardCategoricalWinClasses).
	wlB := wla.WinLines[2]
	if wlB.Tier != "B" {
		t.Errorf("Hellkite as determined WinLine.Tier = %q, want %q", wlB.Tier, "B")
	}
	if !strings.HasPrefix(wlB.Rationale.Resolves[0], "Tier B:") {
		t.Errorf("Hellkite Resolves[0] = %q, want prefix %q", wlB.Rationale.Resolves[0], "Tier B:")
	}
}

// -----------------------------------------------------------------------------
// 3. Non-combo win lines (alt_wincon / commander_damage / combat)
// -----------------------------------------------------------------------------

func TestAddNonComboWinLines_AltWinconGetsTierD(t *testing.T) {
	// IsWinCon profile should produce an alt_wincon win line with
	// Tier D and the prefix in Resolves.
	wla := &WinLineAnalysis{RedundancyMap: map[string]int{}}
	report := &FreyaReport{
		Commander: "",
		WinLines:  wla,
	}
	qtyProfiles := []CardProfileQty{
		{Profile: CardProfile{Name: "Maze's End", IsWinCon: true}, Qty: 1},
	}
	profileByName := map[string]CardProfile{"Maze's End": qtyProfiles[0].Profile}
	oracle := stubOracleWithText(map[string]string{
		"Maze's End": "{3}, {t}, sacrifice this: search your library for a gate card, put it onto the battlefield, then shuffle. then if you control ten or more gates with different names, you win the game.",
	})
	addNonComboWinLines(wla, report, qtyProfiles, oracle, profileByName)

	if len(wla.WinLines) == 0 {
		t.Fatal("expected at least 1 alt-wincon win line, got 0")
	}
	wl := wla.WinLines[0]
	if wl.Type != "alt_wincon" {
		t.Errorf("WinLine.Type = %q, want %q", wl.Type, "alt_wincon")
	}
	if wl.Tier != "D" {
		t.Errorf("alt_wincon WinLine.Tier = %q, want %q", wl.Tier, "D")
	}
	if wl.Rationale == nil || len(wl.Rationale.Resolves) == 0 {
		t.Fatal("alt_wincon WinLine missing Rationale.Resolves")
	}
	if !strings.HasPrefix(wl.Rationale.Resolves[0], "Tier D:") {
		t.Errorf("alt_wincon Resolves[0] = %q, want prefix %q", wl.Rationale.Resolves[0], "Tier D:")
	}
}

func TestAddNonComboWinLines_CommanderDamageGetsTierD(t *testing.T) {
	wla := &WinLineAnalysis{RedundancyMap: map[string]int{}}
	report := &FreyaReport{
		Commander: "Wyleth, Soul of Steel",
		WinLines:  wla,
	}
	addNonComboWinLines(wla, report, nil, nil, nil)

	// First (and only) win line should be commander_damage Tier D.
	var cdLine *WinLine
	for i := range wla.WinLines {
		if wla.WinLines[i].Type == "commander_damage" {
			cdLine = &wla.WinLines[i]
			break
		}
	}
	if cdLine == nil {
		t.Fatal("expected commander_damage win line")
	}
	if cdLine.Tier != "D" {
		t.Errorf("commander_damage Tier = %q, want %q", cdLine.Tier, "D")
	}
	if !strings.HasPrefix(cdLine.Rationale.Resolves[0], "Tier D:") {
		t.Errorf("commander_damage Resolves[0] = %q, want prefix %q", cdLine.Rationale.Resolves[0], "Tier D:")
	}
}
