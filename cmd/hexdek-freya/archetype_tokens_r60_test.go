package main

import (
	"strings"
	"testing"
)

// archetype_tokens_r60_test.go — pins the r60 Tokens archetype. Three
// surfaces are tested in parallel:
//
//   1. The detection helpers cardCreatesTokens + cardHasAnthem
//      correctly classify the canonical patterns and reject the
//      false-positive shapes ("create a copy" / single-target buffs).
//   2. buildClassifyContext correctly increments tokenCreatorCount
//      and anthemCount when the corresponding oracle text fires.
//   3. ClassifyArchetype routes a Krenko/Adeline-shape deck (8+
//      token creators + 3+ anthems) to "Tokens" rather than the
//      generic "Aggro" fallback. Counterfactual: a token-only deck
//      WITHOUT the anthem density still routes to Aggro/Midrange.
//   4. Eval-weight registration: "tokens" must exist in
//      defaultWeights so ComputeEvalWeights doesn't fall back to
//      midrange for the new fingerprint.

// -----------------------------------------------------------------------------
// 1. Detection helpers — cardCreatesTokens
// -----------------------------------------------------------------------------

func TestCardCreatesTokens_CanonicalPatterns(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		cardName string // when set, exercises the doubler-name-lookup arm in addition to oracle-text matching
		want     bool
	}{
		// True positives — token-creation phrasings (oracle-text arm)
		{"create a creature token",
			"create a 1/1 white soldier creature token.", "", true},
		{"create two creature tokens",
			"create two 1/1 green saproling creature tokens.", "", true},
		{"create three creature tokens",
			"create three 1/1 white human soldier creature tokens.", "", true},
		{"create X tokens",
			"create x 1/1 colorless thopter artifact creature tokens.", "", true},
		{"creates a token (third person)",
			"at the beginning of your end step, this creature creates a 1/1 red goblin creature token.", "", true},
		{"create that many tokens (Selvala's Stampede shape)",
			"create that many 3/3 green beast creature tokens.", "", true},
		// True positives — token doubler names (replacement effects).
		// These intentionally route through the NAME-lookup arm
		// because the doubler oracle phrasing ("twice that many of
		// those tokens are created instead") doesn't match any
		// creation-verb in tokenCreationPhrases — production callers
		// always have the card name available, so the name-lookup
		// arm is the canonical detection path for doublers.
		{"Anointed Procession doubler",
			"if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
			"Anointed Procession", true},
		{"Doubling Season doubler (counters AND tokens)",
			"if an effect would create one or more tokens under your control, it creates twice that many of those tokens instead. if an effect would put one or more counters on a permanent you control, it puts twice that many of those counters on that permanent instead.",
			"Doubling Season", true},
		// False positives — "create a copy" / "put a counter" must NOT match
		{"spell copy is not token creation",
			"copy target instant or sorcery spell. you may choose new targets for the copy.", "", false},
		{"put a +1/+1 counter is not token creation",
			"put a +1/+1 counter on target creature you control.", "", false},
		{"put a card into hand is not token creation",
			"search your library for a card, put it into your hand, then shuffle.", "", false},
		// False positive — "create" without "token" is not token creation
		{"create an emblem is not token creation",
			"-7: you get an emblem with \"creatures you control have flying.\"", "", false},
		// Empty
		{"empty oracle returns false", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cardCreatesTokens(strings.ToLower(c.text), c.cardName)
			if got != c.want {
				t.Errorf("cardCreatesTokens(text=%q, name=%q) = %v, want %v", c.text, c.cardName, got, c.want)
			}
		})
	}
}

func TestCardCreatesTokens_DoublerNameLookup(t *testing.T) {
	// Doublers are detected by NAME (not by oracle text containing a
	// creation phrase) since their effect is a replacement, not a
	// per-turn creation event. Pin the four canonical entries.
	doublers := []string{
		"Anointed Procession",
		"Parallel Lives",
		"Doubling Season",
		"Mondrak, Glory Dominus",
		"Primal Vigor",
	}
	for _, name := range doublers {
		t.Run(name, func(t *testing.T) {
			// Empty oracle so we test the name-lookup arm in isolation.
			if !cardCreatesTokens("", name) {
				t.Errorf("doubler %q must be flagged as a token creator (name-lookup arm)", name)
			}
			// Case-insensitive lookup.
			if !cardCreatesTokens("", strings.ToUpper(name)) {
				t.Errorf("doubler %q (uppercase) must be case-insensitive in name lookup", name)
			}
		})
	}
	// A non-doubler name with empty oracle text must NOT match.
	if cardCreatesTokens("", "Lightning Bolt") {
		t.Errorf("Lightning Bolt must not be flagged as a token creator")
	}
}

// -----------------------------------------------------------------------------
// 2. Detection helpers — cardHasAnthem
// -----------------------------------------------------------------------------

func TestCardHasAnthem_CanonicalPatterns(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		// True positives — broad anthems
		{"Glorious Anthem broad",
			"creatures you control get +1/+1.", true},
		{"Honor of the Pure other-only",
			"other creatures you control get +1/+1.", true},
		{"Crusade-style tribal anthem",
			"white creatures you control get +1/+1.", true},
		{"Goblin King tribal anthem",
			"other goblin creatures you control get +1/+1.", true},
		{"Elvish Champion tribal anthem",
			"other elf creatures you control get +1/+1 and have forestwalk.", true},
		{"+2/+2 anthem",
			"creatures you control get +2/+2 until end of turn.", true},
		{"creatures-you-control HAVE-shape",
			"creatures you control have +1/+1 and trample.", true},
		{"creature TOKENS get +N/+N",
			"creature tokens you control get +1/+1.", true},
		// False positives — single-target buffs
		{"single-target buff is NOT anthem",
			"target creature you control gets +3/+3 until end of turn.", false},
		{"giant growth is NOT anthem",
			"target creature gets +3/+3 until end of turn.", false},
		// False positive — keyword-only grants (no +X/+Y)
		{"flying-only grant is NOT a stat anthem",
			"creatures you control have flying.", false},
		// Empty
		{"empty oracle", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cardHasAnthem(strings.ToLower(c.text))
			if got != c.want {
				t.Errorf("cardHasAnthem(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// 3. buildClassifyContext — counters fire on a Krenko-shape fixture
// -----------------------------------------------------------------------------

func TestBuildClassifyContext_TokensCounters(t *testing.T) {
	// 8 token creators (5 oracle-text creators + 3 doublers) + 3 anthems
	// + 1 control card (Lightning Bolt) that must not increment either.
	profiles := []CardProfile{
		{Name: "Krenko, Mob Boss", TypeLine: "Legendary Creature — Goblin Warrior"},
		{Name: "Goblin Rabblemaster", TypeLine: "Creature — Goblin Warrior"},
		{Name: "Krenko's Command", TypeLine: "Sorcery"},
		{Name: "Dragon Fodder", TypeLine: "Sorcery"},
		{Name: "Hordeling Outburst", TypeLine: "Sorcery"},
		{Name: "Anointed Procession", TypeLine: "Enchantment"}, // doubler
		{Name: "Parallel Lives", TypeLine: "Enchantment"},      // doubler
		{Name: "Mondrak, Glory Dominus", TypeLine: "Legendary Creature — Phyrexian Horror"}, // doubler
		{Name: "Glorious Anthem", TypeLine: "Enchantment"},
		{Name: "Honor of the Pure", TypeLine: "Enchantment"},
		{Name: "Goblin King", TypeLine: "Creature — Goblin"},
		{Name: "Lightning Bolt", TypeLine: "Instant"}, // control — neither
	}
	oracleText := map[string]string{
		"Krenko, Mob Boss":        "{t}: create x 1/1 red goblin creature tokens, where x is the number of goblins you control.",
		"Goblin Rabblemaster":     "whenever this creature attacks, create a 1/1 red goblin creature token that's tapped and attacking.",
		"Krenko's Command":        "create two 1/1 red goblin creature tokens.",
		"Dragon Fodder":           "create two 1/1 red goblin creature tokens.",
		"Hordeling Outburst":      "create three 1/1 red goblin creature tokens.",
		"Anointed Procession":     "if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
		"Parallel Lives":          "if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
		"Mondrak, Glory Dominus":  "if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
		"Glorious Anthem":         "creatures you control get +1/+1.",
		"Honor of the Pure":       "other creatures you control get +1/+1.",
		"Goblin King":             "other goblin creatures you control get +1/+1 and have mountainwalk.",
		"Lightning Bolt":          "this spell deals 3 damage to any target.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.tokenCreatorCount != 8 {
		t.Errorf("tokenCreatorCount = %d, want 8 (5 oracle-text creators + 3 doublers; Lightning Bolt + anthems should not count)",
			ctx.tokenCreatorCount)
	}
	if ctx.anthemCount != 3 {
		t.Errorf("anthemCount = %d, want 3 (Glorious Anthem + Honor of the Pure + Goblin King)",
			ctx.anthemCount)
	}
}

func TestBuildClassifyContext_TokensCounters_NoFalsePositives(t *testing.T) {
	// Sanity that "create a copy" (spell-copy) and single-target buffs
	// don't increment the counters.
	profiles := []CardProfile{
		{Name: "Twincast", TypeLine: "Instant"},
		{Name: "Giant Growth", TypeLine: "Instant"},
	}
	oracleText := map[string]string{
		"Twincast":     "copy target instant or sorcery spell. you may choose new targets for the copy.",
		"Giant Growth": "target creature gets +3/+3 until end of turn.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.tokenCreatorCount != 0 {
		t.Errorf("tokenCreatorCount = %d, want 0 (spell copy + single-target buff are not token creation)",
			ctx.tokenCreatorCount)
	}
	if ctx.anthemCount != 0 {
		t.Errorf("anthemCount = %d, want 0 (single-target buff is not anthem)",
			ctx.anthemCount)
	}
}

// -----------------------------------------------------------------------------
// 4. ClassifyArchetype — Tokens-shape deck routes to "Tokens"
// -----------------------------------------------------------------------------

// makeKrenkoShapeFixture builds a deck with the Krenko-shape signature:
// 8 token creators + 3 anthems. The role counts mimic the Tokens
// fingerprint's Ratios so the euclidean distance favors Tokens over
// generic Aggro. totalCards inflates the role denominator the way a
// real 100-card deck would, so the Tokens fingerprint's RoleThreat=0.22
// target is meaningful (not artificially inflated by tiny denom).
func makeKrenkoShapeFixture() (profiles []CardProfile, oracleText map[string]string, roleCounts map[RoleTag]int, totalCards int) {
	profiles = []CardProfile{
		// 8 token creators
		{Name: "Krenko, Mob Boss", TypeLine: "Legendary Creature — Goblin Warrior"},
		{Name: "Goblin Rabblemaster", TypeLine: "Creature — Goblin Warrior"},
		{Name: "Krenko's Command", TypeLine: "Sorcery"},
		{Name: "Dragon Fodder", TypeLine: "Sorcery"},
		{Name: "Hordeling Outburst", TypeLine: "Sorcery"},
		{Name: "Mardu Ascendancy", TypeLine: "Enchantment"},
		{Name: "Anointed Procession", TypeLine: "Enchantment"},
		{Name: "Parallel Lives", TypeLine: "Enchantment"},
		// 3 anthems
		{Name: "Glorious Anthem", TypeLine: "Enchantment"},
		{Name: "Honor of the Pure", TypeLine: "Enchantment"},
		{Name: "Goblin King", TypeLine: "Creature — Goblin"},
		// 11 other threats / removal / draw / ramp / lands to round out
		// a realistic deck shape — totalCards 100 with these qty rolls
		// up to a Tokens-shape fingerprint match. The exact non-token
		// cards don't matter for the Require gate; they're padding to
		// make the role ratios realistic.
	}
	oracleText = map[string]string{
		"Krenko, Mob Boss":       "{t}: create x 1/1 red goblin creature tokens, where x is the number of goblins you control.",
		"Goblin Rabblemaster":    "whenever this creature attacks, create a 1/1 red goblin creature token that's tapped and attacking.",
		"Krenko's Command":       "create two 1/1 red goblin creature tokens.",
		"Dragon Fodder":          "create two 1/1 red goblin creature tokens.",
		"Hordeling Outburst":     "create three 1/1 red goblin creature tokens.",
		"Mardu Ascendancy":       "whenever a creature you control attacks, create a 1/1 red, white, and black warrior creature token tapped and attacking.",
		"Anointed Procession":    "if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
		"Parallel Lives":         "if one or more tokens would be created under your control, twice that many of those tokens are created instead.",
		"Glorious Anthem":        "creatures you control get +1/+1.",
		"Honor of the Pure":      "other creatures you control get +1/+1.",
		"Goblin King":            "other goblin creatures you control get +1/+1 and have mountainwalk.",
	}
	roleCounts = map[RoleTag]int{
		RoleThreat:  22, // Tokens fingerprint targets 0.22 * total
		RoleRamp:    6,
		RoleDraw:    8,
		RoleRemoval: 5,
	}
	totalCards = 100
	return
}

func TestClassifyArchetype_KrenkoShapeRoutesToTokens(t *testing.T) {
	profiles, oracleText, roleCounts, totalCards := makeKrenkoShapeFixture()
	ac := classifyFixture(profiles, oracleText, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Tokens" {
		t.Errorf("Primary = %q, want %q (8 creators + 3 anthems should win the fingerprint)", ac.Primary, "Tokens")
	}
}

func TestClassifyArchetype_NoAnthemsFallsBackOffTokens(t *testing.T) {
	// Counterfactual: same 8 token creators but ZERO anthems → Tokens
	// fingerprint Require gate (≥3 anthems) fails, so we route to
	// Aggro or Midrange instead. This pins that the anthem-density
	// gate is load-bearing — without it the new fingerprint would
	// poach every token-flood deck (Bishop of Wings + Wedding
	// Announcement + Battle Hymn etc.) that isn't actually a
	// "Tokens" archetype.
	profiles, oracleText, roleCounts, totalCards := makeKrenkoShapeFixture()
	// Strip out the 3 anthems (last 3 entries) plus their oracle.
	profiles = profiles[:len(profiles)-3]
	delete(oracleText, "Glorious Anthem")
	delete(oracleText, "Honor of the Pure")
	delete(oracleText, "Goblin King")
	// Lower avgCMC so Aggro's gate fires (creature ratio + low CMC).
	ac := classifyFixtureWithAvgCMC(profiles, oracleText, roleCounts, totalCards, 2.5)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Tokens" {
		t.Errorf("Primary = %q — Tokens fingerprint must NOT match without ≥3 anthems (the entire point of the gate)", ac.Primary)
	}
}

func TestClassifyArchetype_FewerThan8CreatorsFallsBackOffTokens(t *testing.T) {
	// Counterfactual: 3 anthems but only 5 token creators → Tokens
	// fingerprint Require gate (≥8 creators) fails. Same load-bearing
	// rationale as the no-anthems counterfactual.
	profiles, oracleText, roleCounts, totalCards := makeKrenkoShapeFixture()
	// Strip 3 of the 8 token creators (keeps Mardu Ascendancy, Anointed
	// Procession, Parallel Lives, and 2 oracle creators = 5 creators).
	keep := []CardProfile{}
	for _, p := range profiles {
		switch p.Name {
		case "Krenko's Command", "Dragon Fodder", "Hordeling Outburst":
			continue // remove these 3
		}
		keep = append(keep, p)
	}
	profiles = keep
	delete(oracleText, "Krenko's Command")
	delete(oracleText, "Dragon Fodder")
	delete(oracleText, "Hordeling Outburst")
	ac := classifyFixture(profiles, oracleText, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Tokens" {
		t.Errorf("Primary = %q — Tokens must NOT match with only 5 creators (< 8 gate)", ac.Primary)
	}
}

// classifyFixtureWithAvgCMC is identical to classifyFixture but lets
// the test override avgCMC. Used by the no-anthems counterfactual to
// drop CMC low enough that Aggro's `avgCMC < 3.0` Require fires.
func classifyFixtureWithAvgCMC(profiles []CardProfile, oracleText map[string]string, roleCounts map[RoleTag]int, totalCards int, avgCMC float64) *ArchetypeClassification {
	qtyProfiles := make([]CardProfileQty, 0, len(profiles))
	for _, p := range profiles {
		qtyProfiles = append(qtyProfiles, CardProfileQty{Profile: p, Qty: 1})
	}
	report := &FreyaReport{
		TotalCards: totalCards,
		AvgCMC:     avgCMC,
		Profiles:   profiles,
		Roles: &RoleAnalysis{
			TotalCards: totalCards,
			RoleCounts: roleCounts,
		},
	}
	return ClassifyArchetype(report, qtyProfiles, stubOracleWithText(oracleText))
}

// -----------------------------------------------------------------------------
// 5. Eval-weight profile — Tokens key exists in defaultWeights table
// -----------------------------------------------------------------------------

func TestTokensWeights_RegisteredInDefaultsTable(t *testing.T) {
	w, ok := defaultWeights["tokens"]
	if !ok || w == nil {
		t.Fatal("defaultWeights[\"tokens\"] missing — eval-weight fallback to midrange would apply for Tokens-classified decks")
	}
	// Sanity divergences vs midrange that the Tokens profile is
	// supposed to encode. If a future re-tune flips one of these,
	// the profile no longer behaves as a tokens deck.
	mid := defaultWeights["midrange"]
	if w.BoardPresence <= mid.BoardPresence {
		t.Errorf("Tokens BoardPresence = %v, want > midrange %v (tokens ARE the board)", w.BoardPresence, mid.BoardPresence)
	}
	if w.ThreatExposure <= mid.ThreatExposure {
		t.Errorf("Tokens ThreatExposure = %v, want > midrange %v (board wipes are catastrophic vs tokens)", w.ThreatExposure, mid.ThreatExposure)
	}
	if w.CommanderProgress <= mid.CommanderProgress {
		t.Errorf("Tokens CommanderProgress = %v, want > midrange %v (Krenko/Adeline IS the engine)", w.CommanderProgress, mid.CommanderProgress)
	}
}

func TestComputeEvalWeights_TokensClassifiedDeckGetsTokensProfile(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Tokens",
		RampCount:        4,
		WinLineCount:     0,
	}
	r := &FreyaReport{
		NonLandTutorCount: 0,
		RemovalCount:      10, // > 5 so ThreatExposure adjustment doesn't fire
		Roles:             &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
	}
	got := ComputeEvalWeights(dp, r)
	want := defaultWeights["tokens"]
	if got == nil || want == nil {
		t.Fatal("nil weights returned from ComputeEvalWeights")
	}
	if *got != *want {
		t.Errorf("Tokens deck got non-Tokens weights:\n  got  = %+v\n  want = %+v", *got, *want)
	}
}
