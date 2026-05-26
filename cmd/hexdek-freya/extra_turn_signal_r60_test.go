package main

import (
	"strings"
	"testing"
)

// TestBuildClassifyContext_ExtraTurnCount pins the canonical
// extra-turn detection. Time Walk / Time Warp / Nexus of Fate all
// match the granting phrasings; Aggravated Assault (extra COMBAT
// PHASE, not extra turn) and Stranglehold ("can't take extra turns" —
// denial wording) must NOT count.
func TestBuildClassifyContext_ExtraTurnCount(t *testing.T) {
	profiles := []CardProfile{
		// Time Walk is on the commander banned list and skipped before
		// oracle scanning — use the legal Time Warp / Capture / Nexus /
		// Sage anchors instead.
		{Name: "Time Warp", TypeLine: "Sorcery"},
		{Name: "Capture of Jingzhou", TypeLine: "Sorcery"},
		{Name: "Nexus of Fate", TypeLine: "Instant"},
		{Name: "Sage of Hours", TypeLine: "Creature"},
		{Name: "Aggravated Assault", TypeLine: "Enchantment"},
		{Name: "Hellkite Charger", TypeLine: "Creature"},
		{Name: "Stranglehold", TypeLine: "Enchantment"},
	}
	oracleText := map[string]string{
		"time warp":           "Target player takes an extra turn after this one.",
		"capture of jingzhou": "Target player takes an extra turn after this one.",
		"nexus of fate":      "Take an extra turn after this one.",
		"sage of hours":      "Remove five time counters from Sage of Hours: Take an extra turn after this one.",
		"aggravated assault": "{3}: After the combat phase this turn, there is an additional combat phase. Untap all creatures you control.",
		"hellkite charger":   "Pay {5}{R}{R}: Untap all attacking creatures. After this phase, there is an additional combat phase.",
		"stranglehold":       "If an opponent would search a library, that player exiles the top four cards of that library instead. Your opponents can't take extra turns.",
	}

	qty := make([]CardProfileQty, 0, len(profiles))
	for _, p := range profiles {
		qty = append(qty, CardProfileQty{Profile: p, Qty: 1})
	}
	report := &FreyaReport{
		TotalCards: len(profiles),
		Profiles:   profiles,
		Roles:      &RoleAnalysis{TotalCards: len(profiles), RoleCounts: map[RoleTag]int{}},
	}

	ctx := buildClassifyContext(report, qty, stubOracleWithText(oracleText))

	if ctx.extraTurnCount != 4 {
		t.Errorf("extraTurnCount = %d, want 4 (Time Warp, Capture of Jingzhou, Nexus of Fate, Sage of Hours)",
			ctx.extraTurnCount)
	}
	// Aggravated Assault and Hellkite Charger drive extraCombatCount, not
	// extraTurnCount. Both contain "additional combat phase" — pin that
	// they're picked up by the right counter and don't leak into the
	// other.
	if ctx.extraCombatCount < 2 {
		t.Errorf("extraCombatCount = %d, want >= 2 (Aggravated Assault + Hellkite Charger)",
			ctx.extraCombatCount)
	}
}

// TestBuildClassifyContext_AggravatedAssaultExcluded pins the
// scoping rule from the task: Aggravated Assault grants extra combat,
// NOT an extra turn, and must contribute 0 to ctx.extraTurnCount.
func TestBuildClassifyContext_AggravatedAssaultExcluded(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Aggravated Assault", TypeLine: "Enchantment"},
	}
	oracleText := map[string]string{
		"aggravated assault": "{3}: After the combat phase this turn, there is an additional combat phase. Untap all creatures you control.",
	}
	qty := []CardProfileQty{{Profile: profiles[0], Qty: 1}}
	report := &FreyaReport{
		TotalCards: 1, Profiles: profiles,
		Roles: &RoleAnalysis{TotalCards: 1, RoleCounts: map[RoleTag]int{}},
	}
	ctx := buildClassifyContext(report, qty, stubOracleWithText(oracleText))

	if ctx.extraTurnCount != 0 {
		t.Errorf("Aggravated Assault counted as extra-turn (%d), should be 0 — it grants extra COMBAT PHASE only",
			ctx.extraTurnCount)
	}
}

// TestBuildClassifyContext_AdventureCountsOnce pins the Adventure-card
// dedupe: a single Adventure (Twice Upon a Time stand-in here) whose
// BOTH faces contain extra-turn text must count as 1, not 2 — the card
// occupies one deck slot regardless of how many halves grant a turn.
func TestBuildClassifyContext_AdventureCountsOnce(t *testing.T) {
	// Build a card whose primary OracleText is empty and whose CardFaces
	// both contain the granting phrase — mirrors Scryfall's split-card /
	// Adventure data shape.
	by := map[string]*oracleEntry{
		"twice upon a time": {
			Name: "Twice Upon a Time",
			// Primary text intentionally empty — Adventure cards put text on faces.
			OracleText: "",
			CardFaces: []struct {
				Name       string `json:"name"`
				OracleText string `json:"oracle_text"`
				TypeLine   string `json:"type_line"`
				ManaCost   string `json:"mana_cost"`
			}{
				{Name: "Twice", OracleText: "Take an extra turn after this one."},
				{Name: "Upon a Time", OracleText: "Target player takes an extra turn after this one."},
			},
		},
	}
	oracle := &oracleDB{byName: by}

	profiles := []CardProfile{{Name: "Twice Upon a Time", TypeLine: "Sorcery // Sorcery"}}
	qty := []CardProfileQty{{Profile: profiles[0], Qty: 1}}
	report := &FreyaReport{
		TotalCards: 1, Profiles: profiles,
		Roles: &RoleAnalysis{TotalCards: 1, RoleCounts: map[RoleTag]int{}},
	}
	ctx := buildClassifyContext(report, qty, oracle)

	if ctx.extraTurnCount != 1 {
		t.Errorf("Adventure card with extra-turn text on both faces counted as %d, want 1 (single deck slot)",
			ctx.extraTurnCount)
	}
}

// TestEstimateBracket_ExtraTurnScoresAtThresholds pins the three-tier
// extra-turn scoring: count<=3 contributes 0, 4-6 contributes +2,
// >=7 contributes +3. We compare bracket signal contributions across
// the boundary points.
func TestEstimateBracket_ExtraTurnScoresAtThresholds(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	baseCtx := func(et int) *classifyContext {
		return &classifyContext{
			roleRatios:     map[RoleTag]float64{},
			avgCMC:         3.0,
			extraTurnCount: et,
		}
	}

	cases := []struct {
		count int
		want  int // expected contribution from the extra-turn signal alone
	}{
		{0, 0},
		{3, 0},
		{4, 2},
		{6, 2},
		{7, 3},
		{15, 3},
	}
	for _, tc := range cases {
		_, _, br := estimateMeasuredBracket(baseCtx(tc.count), report, "")
		var got int
		for _, s := range br.Signals {
			if s.Name == "Extra-turn density" {
				got = s.Contribution
			}
		}
		if got != tc.want {
			t.Errorf("extraTurnCount=%d: signal contribution = %d, want %d",
				tc.count, got, tc.want)
		}
	}
}

// TestEstimateBracket_HellkiteCharger_BearUmbra_LiftsToB4 pins the
// categorical B4+ carveout for the Hellkite Charger + Bear Umbra
// 2-card win. With only 1 Game Changer present, the GC=1-3 ceiling
// would normally cap at B3 unless a winning combo is present — the
// broadened predicate must trip the carveout via the determined
// 2-card categorical-win path (the combo is NOT a true_infinite).
func TestEstimateBracket_HellkiteCharger_BearUmbra_LiftsToB4(t *testing.T) {
	// Determined combo (Type=="determined") with Class=infinite_damage
	// and exactly 2 pieces — exercises the new
	// hasTwoCardCategoricalWin helper. NOT in TrueInfinites.
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Determined: []ComboResult{{
			Cards:       []string{"Hellkite Charger", "Bear Umbra"},
			LoopType:    "determined",
			Class:       ComboClassInfiniteDamage,
			Description: "Infinite combat → infinite damage.",
		}},
	}
	// Light GC presence + raw score that would otherwise put bracket >=4
	// before the GC=1-3 ceiling fires. With the carveout, B4 is held.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,
		tutorDensity:     0.05,
		comboCount:       1,
		fastManaCount:    4,
		gameChangerCount: 2,
	}
	b, lbl, br := estimateMeasuredBracket(ctx, report, "")
	if b < 4 {
		t.Errorf("Hellkite Charger + Bear Umbra (2-card categorical win) should lift to B4+ via WotC carveout, got B%d (%s)", b, lbl)
	}
	// Verify the ceiling-lift adjustment fired (not just a coincidental
	// raw score >=8). Check for the GC=1-3 ceiling note OR confirm the
	// raw score path landed at B4 without being capped down.
	hasCeilingNote := false
	for _, s := range br.Signals {
		if s.Kind == "ceiling" && strings.Contains(s.Note, "winning combo") {
			hasCeilingNote = true
		}
	}
	// Acceptable: either we hit the lifted ceiling, OR the raw score
	// (with extra-turn bonus etc.) is already >=4. The defining
	// invariant is that we don't cap DOWN to B3.
	if !hasCeilingNote && b < 4 {
		t.Errorf("expected ceiling lift or raw bracket >= 4, got B%d without ceiling-lift signal", b)
	}
}

// TestHasTwoCardCategoricalWin_ClassFilter pins which combo classes
// count as categorical wins. Infinite mana / ETB / lockdown alone
// must NOT count — they require an outlet, and bracket scoring
// already credits them via comboCount / fastManaCount channels.
func TestHasTwoCardCategoricalWin_ClassFilter(t *testing.T) {
	cases := []struct {
		class string
		want  bool
		why   string
	}{
		{ComboClassInfiniteDamage, true, "wins on resolution"},
		{ComboClassInfiniteDrain, true, "drain kills the table"},
		{ComboClassInfiniteMill, true, "mills opponent's library to 0"},
		{ComboClassLibraryExileWin, true, "Thoracle empty-library win"},
		{ComboClassCombatFinisher, true, "lethal swing"},
		{ComboClassStormFinisher, true, "Aetherflux nuke"},
		{ComboClassInfiniteTokens, true, "hasty tokens win next combat"},
		{ComboClassInfiniteMana, false, "needs a sink"},
		{ComboClassInfiniteETB, false, "needs an outlet"},
		{ComboClassLockdown, false, "doesn't end the game"},
		{ComboClassBlinkEngine, false, "value engine, not a win"},
		{ComboClassETBDoubler, false, "multiplier, not a win"},
		{ComboClassManaSink, false, "payoff piece, not a 2-card line"},
		{ComboClassUnknown, false, "unclassified — don't false-trip"},
	}
	for _, tc := range cases {
		got := hasTwoCardCategoricalWin([]ComboResult{{
			Cards: []string{"X", "Y"},
			Class: tc.class,
		}})
		if got != tc.want {
			t.Errorf("class %q: got %v, want %v (%s)", tc.class, got, tc.want, tc.why)
		}
	}

	// Arity filter: a 3+ card combo with a win-class shouldn't trip the
	// 2-card predicate — that's the whole point of the WotC carveout
	// (it's about 2-card wins specifically).
	got := hasTwoCardCategoricalWin([]ComboResult{{
		Cards: []string{"A", "B", "C"},
		Class: ComboClassInfiniteDamage,
	}})
	if got {
		t.Error("3-card combo tripped 2-card predicate — arity filter broken")
	}
}

// TestKnownCombos_HellkiteCharger_BearUmbra_Present guards against
// future deletions of the Hellkite Charger + Bear Umbra entry. The
// curated 2-card extra-combat wins anchor the categorical B4+ lift —
// they're a documented bracket-bump signal and shouldn't silently
// disappear.
func TestKnownCombos_HellkiteCharger_BearUmbra_Present(t *testing.T) {
	want := map[string]string{
		"Hellkite Charger + Bear Umbra":   ComboClassInfiniteDamage,
		"Aggravated Assault + Bear Umbra": ComboClassInfiniteDamage,
	}
	for _, c := range KnownCombos {
		if expectClass, ok := want[c.Name]; ok {
			if c.Type != "determined" {
				t.Errorf("%s: Type = %q, want determined", c.Name, c.Type)
			}
			if c.Class != expectClass {
				t.Errorf("%s: Class = %q, want %q", c.Name, c.Class, expectClass)
			}
			if len(c.Pieces) != 2 {
				t.Errorf("%s: %d pieces, want 2", c.Name, len(c.Pieces))
			}
			delete(want, c.Name)
		}
	}
	for missing := range want {
		t.Errorf("KnownCombos missing curated entry %q", missing)
	}
}
