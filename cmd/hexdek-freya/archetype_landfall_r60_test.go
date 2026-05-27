package main

import (
	"strings"
	"testing"
)

// archetype_landfall_r60_test.go — pins the r60 Landfall archetype.
// Four surfaces tested:
//
//   1. landIsFetchOrRamp correctly classifies true fetches, slow-
//      fetches, multi-fetch lands, and rejects mana-fix lands /
//      basic lands / utility lands that don't search for another land.
//   2. buildClassifyContext's second land-pass increments
//      fetchRampLandCount on the right cards.
//   3. ClassifyArchetype routes a Lotus Cobra / Omnath / Tireless
//      Tracker-shape deck (4+ landfall + 3+ fetch/ramp lands) to
//      "Landfall" rather than the broader "Lands Matter" fallback.
//      Counterfactuals: a deck with landfall triggers but no fetches
//      (just basics) routes off Landfall; a fetch-heavy deck with
//      no landfall payoffs routes off Landfall.
//   4. Eval-weight registration: "landfall" must exist in
//      defaultWeights so ComputeEvalWeights doesn't fall back to
//      midrange for the new fingerprint.

// -----------------------------------------------------------------------------
// 1. landIsFetchOrRamp — classification helper
// -----------------------------------------------------------------------------

func TestLandIsFetchOrRamp_CanonicalPatterns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// True positives — fetch / slow-fetch / multi-fetch lands.
		{"true fetch land Polluted Delta",
			"{t}, pay 1 life, sacrifice this: search your library for an island or swamp card, put it onto the battlefield, then shuffle.", true},
		{"true fetch land Wooded Foothills",
			"{t}, pay 1 life, sacrifice this: search your library for a mountain or forest card, put it onto the battlefield, then shuffle.", true},
		{"slow-fetch Evolving Wilds",
			"{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.", true},
		{"slow-fetch Terramorphic Expanse",
			"{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.", true},
		{"Fabled Passage",
			"{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle. then if you control four or more lands, untap that land.", true},
		{"multi-fetch Myriad Landscape",
			"{2}, {t}, sacrifice this: search your library for up to two basic land cards that share a land type, put them onto the battlefield tapped, then shuffle.", true},
		{"shuffler Krosan Verge",
			"{2}, {t}, sacrifice this: search your library for a forest card and a plains card, put them onto the battlefield tapped, then shuffle.", true},
		// False positives — mana-fix / utility / basic lands that do
		// NOT fetch additional lands.
		{"shockland Hallowed Fountain is NOT a fetch",
			"as this enters, you may pay 2 life. if you don't, it enters tapped. {t}: add {w} or {u}.", false},
		{"basic Plains is NOT a fetch",
			"{t}: add {w}.", false},
		{"utility land Reliquary Tower is NOT a fetch",
			"you have no maximum hand size. {t}: add {c}.", false},
		{"Wasteland is NOT a fetch (destroys, doesn't search)",
			"{t}: add {c}. {t}, sacrifice this: destroy target nonbasic land.", false},
		{"empty oracle", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := landIsFetchOrRamp(strings.ToLower(c.text))
			if got != c.want {
				t.Errorf("landIsFetchOrRamp(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// 2. buildClassifyContext — fetchRampLandCount fires on the right lands
// -----------------------------------------------------------------------------

func TestBuildClassifyContext_FetchRampLandCounter(t *testing.T) {
	// 4 fetch/ramp-effect lands + 1 shockland (non-fetch) + 1 basic
	// (non-fetch). Counter should land at exactly 4.
	profiles := []CardProfile{
		{Name: "Polluted Delta", TypeLine: "Land", IsLand: true},
		{Name: "Evolving Wilds", TypeLine: "Land", IsLand: true},
		{Name: "Fabled Passage", TypeLine: "Land", IsLand: true},
		{Name: "Myriad Landscape", TypeLine: "Land", IsLand: true},
		{Name: "Hallowed Fountain", TypeLine: "Land — Plains Island", IsLand: true},
		{Name: "Plains", TypeLine: "Basic Land — Plains", IsLand: true},
	}
	oracleText := map[string]string{
		"Polluted Delta":    "{t}, pay 1 life, sacrifice this: search your library for an island or swamp card, put it onto the battlefield, then shuffle.",
		"Evolving Wilds":    "{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		"Fabled Passage":    "{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		"Myriad Landscape":  "{2}, {t}, sacrifice this: search your library for up to two basic land cards that share a land type, put them onto the battlefield tapped, then shuffle.",
		"Hallowed Fountain": "as this enters, you may pay 2 life. if you don't, it enters tapped. {t}: add {w} or {u}.",
		"Plains":            "{t}: add {w}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.fetchRampLandCount != 4 {
		t.Errorf("fetchRampLandCount = %d, want 4 (Polluted Delta + Evolving Wilds + Fabled Passage + Myriad Landscape; shockland + basic don't count)",
			ctx.fetchRampLandCount)
	}
}

func TestBuildClassifyContext_FetchRampLandCounter_NonLandsIgnored(t *testing.T) {
	// Spells that search for a land (Cultivate, Rampant Growth) are
	// NOT counted — the fetch/ramp-LAND signal is specifically about
	// LANDS that fetch, because those multiply per-turn landfall
	// triggers (a fetch land enters → landfall #1 → cracks → searches
	// → fetched land enters → landfall #2). Cultivate-class ramp
	// spells trigger landfall only ONCE per cast.
	profiles := []CardProfile{
		{Name: "Cultivate", TypeLine: "Sorcery"},
		{Name: "Rampant Growth", TypeLine: "Sorcery"},
		{Name: "Sakura-Tribe Elder", TypeLine: "Creature — Snake Shaman"},
	}
	oracleText := map[string]string{
		"Cultivate":          "search your library for up to two basic land cards, reveal them, put one onto the battlefield tapped and the other into your hand, then shuffle.",
		"Rampant Growth":     "search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		"Sakura-Tribe Elder": "sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.fetchRampLandCount != 0 {
		t.Errorf("fetchRampLandCount = %d, want 0 (non-land ramp spells are not fetch/ramp LANDS)",
			ctx.fetchRampLandCount)
	}
}

// -----------------------------------------------------------------------------
// 3. ClassifyArchetype — Landfall fingerprint routing
// -----------------------------------------------------------------------------

// makeLandfallShapeFixture builds a Lotus Cobra / Omnath / Tireless
// Tracker-shape deck. 4 landfall trigger cards (via Triggers:[]
// containing "landfall") + 4 fetch/ramp-effect lands.
func makeLandfallShapeFixture() (profiles []CardProfile, oracleText map[string]string, roleCounts map[RoleTag]int, totalCards int) {
	profiles = []CardProfile{
		// 4 landfall payoffs (Triggers list contains "landfall" —
		// that's the ctx.landfallCount path in buildClassifyContext).
		{Name: "Lotus Cobra", TypeLine: "Creature — Snake", Triggers: []string{"landfall"}},
		{Name: "Tireless Tracker", TypeLine: "Creature — Human Scout", Triggers: []string{"landfall"}},
		{Name: "Omnath, Locus of Creation", TypeLine: "Legendary Creature — Elemental", Triggers: []string{"landfall"}},
		{Name: "Roil Elemental", TypeLine: "Creature — Elemental", Triggers: []string{"landfall"}},
		// 4 fetch / slow-fetch / multi-fetch lands
		{Name: "Polluted Delta", TypeLine: "Land", IsLand: true},
		{Name: "Evolving Wilds", TypeLine: "Land", IsLand: true},
		{Name: "Fabled Passage", TypeLine: "Land", IsLand: true},
		{Name: "Myriad Landscape", TypeLine: "Land", IsLand: true},
		// Padding for realistic deck shape
		{Name: "Azusa, Lost but Seeking", TypeLine: "Legendary Creature — Human Monk"},
		{Name: "Cultivate", TypeLine: "Sorcery"},
		{Name: "Kodama's Reach", TypeLine: "Sorcery"},
	}
	oracleText = map[string]string{
		"Lotus Cobra":               "landfall — whenever a land you control enters, you may add one mana of any color.",
		"Tireless Tracker":          "landfall — whenever a land you control enters, create a clue token.",
		"Omnath, Locus of Creation": "landfall — whenever a land you control enters, draw a card. if it's the fourth land you've played this turn, omnath deals 4 damage to each opponent and you gain 4 life.",
		"Roil Elemental":            "flying. landfall — whenever a land you control enters, you may gain control of target creature.",
		"Polluted Delta":            "{t}, pay 1 life, sacrifice this: search your library for an island or swamp card, put it onto the battlefield, then shuffle.",
		"Evolving Wilds":            "{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		"Fabled Passage":            "{t}, sacrifice this: search your library for a basic land card, put it onto the battlefield tapped, then shuffle.",
		"Myriad Landscape":          "{2}, {t}, sacrifice this: search your library for up to two basic land cards that share a land type, put them onto the battlefield tapped, then shuffle.",
		"Azusa, Lost but Seeking":   "you may play two additional lands on each of your turns.",
		"Cultivate":                 "search your library for up to two basic land cards, reveal them, put one onto the battlefield tapped and the other into your hand, then shuffle.",
		"Kodama's Reach":            "search your library for up to two basic land cards, reveal them, put one onto the battlefield tapped and the other into your hand, then shuffle.",
	}
	// Landfall fingerprint targets RoleRamp 0.18, RoleThreat 0.10,
	// RoleDraw 0.10. Set role counts so the deck shape sits near
	// that target (denominator = totalCards).
	roleCounts = map[RoleTag]int{
		RoleRamp:    18,
		RoleThreat:  10,
		RoleDraw:    10,
		RoleRemoval: 6,
	}
	totalCards = 100
	return
}

func TestClassifyArchetype_LandfallShapeRoutesToLandfall(t *testing.T) {
	profiles, oracleText, roleCounts, totalCards := makeLandfallShapeFixture()
	ac := classifyFixture(profiles, oracleText, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Landfall" {
		t.Errorf("Primary = %q, want %q (4 landfall triggers + 4 fetch/ramp-lands should win the structural fingerprint)",
			ac.Primary, "Landfall")
	}
}

func TestClassifyArchetype_NoFetchesFallsOffLandfall(t *testing.T) {
	// Counterfactual: 4 landfall triggers but ZERO fetch/ramp-lands
	// (only basics). Landfall fingerprint's fetchRampLandCount ≥ 3
	// gate fails, so we route to a different archetype. Lands
	// Matter's gate ALSO fails because landfallCount=4 < 5, so the
	// deck likely routes to a generic archetype based on role
	// ratios. The pin is that it must NOT be "Landfall".
	profiles, oracleText, roleCounts, totalCards := makeLandfallShapeFixture()
	// Strip the 4 fetch lands.
	keep := []CardProfile{}
	for _, p := range profiles {
		switch p.Name {
		case "Polluted Delta", "Evolving Wilds", "Fabled Passage", "Myriad Landscape":
			continue
		}
		keep = append(keep, p)
	}
	profiles = keep
	delete(oracleText, "Polluted Delta")
	delete(oracleText, "Evolving Wilds")
	delete(oracleText, "Fabled Passage")
	delete(oracleText, "Myriad Landscape")
	ac := classifyFixture(profiles, oracleText, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Landfall" {
		t.Errorf("Primary = %q — Landfall must NOT match without ≥3 fetch/ramp-lands (the structural gate is load-bearing)",
			ac.Primary)
	}
}

func TestClassifyArchetype_NoLandfallTriggersFallsOffLandfall(t *testing.T) {
	// Counterfactual: 4 fetch/ramp-lands but ZERO landfall triggers
	// (just a ramp deck that happens to run fetch lands for mana
	// fixing). landfallCount ≥ 4 gate fails. Same load-bearing
	// rationale — the new fingerprint must not poach every
	// fetch-heavy deck regardless of whether it has landfall
	// payoffs.
	profiles, oracleText, roleCounts, totalCards := makeLandfallShapeFixture()
	// Strip the 4 landfall payoffs.
	keep := []CardProfile{}
	for _, p := range profiles {
		switch p.Name {
		case "Lotus Cobra", "Tireless Tracker", "Omnath, Locus of Creation", "Roil Elemental":
			continue
		}
		keep = append(keep, p)
	}
	profiles = keep
	delete(oracleText, "Lotus Cobra")
	delete(oracleText, "Tireless Tracker")
	delete(oracleText, "Omnath, Locus of Creation")
	delete(oracleText, "Roil Elemental")
	ac := classifyFixture(profiles, oracleText, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Landfall" {
		t.Errorf("Primary = %q — Landfall must NOT match without ≥4 landfall triggers (the trigger-density gate is load-bearing)",
			ac.Primary)
	}
}

// -----------------------------------------------------------------------------
// 4. Eval-weight profile — Landfall key exists in defaultWeights table
// -----------------------------------------------------------------------------

func TestLandfallWeights_RegisteredInDefaultsTable(t *testing.T) {
	w, ok := defaultWeights["landfall"]
	if !ok || w == nil {
		t.Fatal("defaultWeights[\"landfall\"] missing — eval-weight fallback to midrange would apply for Landfall-classified decks")
	}
	mid := defaultWeights["midrange"]
	// Landfall optimizes for ramp-heavy multi-trigger-per-turn play.
	// ManaAdvantage > midrange (1.6 vs 0.8) is the signature divergence;
	// CardAdvantage > midrange (0.9 vs 1.0 → actually slightly LOWER,
	// because landfall lifts via triggers not pure draws — skip the
	// assertion on CardAdvantage to avoid pinning a brittle direction).
	if w.ManaAdvantage <= mid.ManaAdvantage {
		t.Errorf("Landfall ManaAdvantage = %v, want > midrange %v (ramp-heavy plan)", w.ManaAdvantage, mid.ManaAdvantage)
	}
}

func TestComputeEvalWeights_LandfallClassifiedDeckGetsLandfallProfile(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Landfall",
		RampCount:        4,
		WinLineCount:     0,
	}
	r := &FreyaReport{
		NonLandTutorCount: 0,
		RemovalCount:      10, // > 5 so ThreatExposure adjustment doesn't fire
		Roles:             &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
	}
	got := ComputeEvalWeights(dp, r)
	want := defaultWeights["landfall"]
	if got == nil || want == nil {
		t.Fatal("nil weights returned from ComputeEvalWeights")
	}
	if *got != *want {
		t.Errorf("Landfall deck got non-Landfall weights:\n  got  = %+v\n  want = %+v", *got, *want)
	}
}
