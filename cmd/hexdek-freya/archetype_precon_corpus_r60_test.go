package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Counter detection — unit-level tests on buildClassifyContext for the
// four post-precon-audit archetypes: Group Hug, Cycling, Toxic, Vehicles.
// ---------------------------------------------------------------------------

// TestBuildClassifyContext_GroupHugCounter pins the canonical
// Group-Hug phrasings: "each player draws" (Howling Mine / Font of
// Mythos / Temple Bell), "each opponent draws" (Phelddagrif activation),
// "each player gains" (Kynaios trigger), "each player may search"
// (Veteran Explorer / Collective Voyage). A generic "you draw a card"
// cantrip (Ponder) should NOT count.
func TestBuildClassifyContext_GroupHugCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Howling Mine", TypeLine: "Artifact"},
		{Name: "Font of Mythos", TypeLine: "Artifact"},
		{Name: "Temple Bell", TypeLine: "Artifact"},
		{Name: "Kynaios and Tiro of Meletis", TypeLine: "Legendary Creature"},
		{Name: "Veteran Explorer", TypeLine: "Creature"},
		{Name: "Ponder", TypeLine: "Sorcery"},
	}
	oracleText := map[string]string{
		"Howling Mine":                "at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card. each player draws an extra card.",
		"Font of Mythos":              "at the beginning of each player's draw step, that player draws two additional cards. each player draws an extra card.",
		"Temple Bell":                 "{t}: each player draws a card.",
		"Kynaios and Tiro of Meletis": "whenever kynaios and tiro of meletis attacks, each player gains 1 life and draws a card. each player may put a land card from their hand onto the battlefield.",
		"Veteran Explorer":            "when veteran explorer dies, each player may search their library for up to two basic land cards, put them onto the battlefield, then shuffle.",
		"Ponder":                      "look at the top three cards of your library, then put them back in any order. you may shuffle. draw a card.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.groupHugCount < 5 {
		t.Errorf("groupHugCount=%d, want >=5 (Howling Mine, Font of Mythos, Temple Bell, Kynaios, Veteran Explorer)", ctx.groupHugCount)
	}
	// Solo cantrip control: Ponder by itself must not count.
	soloCtx := buildContextFor(
		[]CardProfile{{Name: "Ponder", TypeLine: "Sorcery"}},
		map[string]string{"Ponder": "look at the top three cards of your library, then put them back in any order. you may shuffle. draw a card."},
	)
	if soloCtx.groupHugCount != 0 {
		t.Errorf("Ponder should not count as group hug, got %d", soloCtx.groupHugCount)
	}
}

// TestBuildClassifyContext_CyclingCounters pins BOTH cycling counters
// — the keyword-cost counter (cyclingCount: any card with "cycling
// {N}") and the payoff counter (cyclingPayoffCount: cards that trigger
// when you cycle). The two are tracked separately because the
// fingerprint Require needs both to fire, not just one.
func TestBuildClassifyContext_CyclingCounters(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Lonely Sandbar", TypeLine: "Land"},
		{Name: "Forgotten Cave", TypeLine: "Land"},
		{Name: "Decree of Justice", TypeLine: "Sorcery"},
		{Name: "Astral Drift", TypeLine: "Enchantment"},
		{Name: "Drake Haven", TypeLine: "Enchantment"},
		{Name: "New Perspectives", TypeLine: "Enchantment"},
		{Name: "Fluctuator", TypeLine: "Artifact"},
		{Name: "Lightning Bolt", TypeLine: "Instant"},
	}
	oracleText := map[string]string{
		"Lonely Sandbar":     "lonely sandbar enters tapped. {t}: add {u}. cycling {2}",
		"Forgotten Cave":     "forgotten cave enters tapped. {t}: add {r}. cycling {2}",
		"Decree of Justice":  "create x 4/4 angel creature tokens with flying. cycling {2}{w}",
		"Astral Drift":       "whenever you cycle a card, you may exile target creature.",
		"Drake Haven":        "whenever you cycle or discard a card, you may pay {1}. if you do, create a 2/2 blue drake creature token with flying.",
		"New Perspectives":   "you have no maximum hand size. as long as you have seven or more cards in hand, cycling abilities you activate cost {0}.",
		"Fluctuator":         "activated abilities of cards with cycling you activate cost up to {2} less to activate. this effect can't reduce the mana in that cost to less than one mana.",
		"Lightning Bolt":     "lightning bolt deals 3 damage to any target.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.cyclingCount < 3 {
		t.Errorf("cyclingCount=%d, want >=3 (Lonely Sandbar, Forgotten Cave, Decree of Justice)", ctx.cyclingCount)
	}
	if ctx.cyclingPayoffCount < 3 {
		t.Errorf("cyclingPayoffCount=%d, want >=3 (Astral Drift, Drake Haven, New Perspectives)", ctx.cyclingPayoffCount)
	}
	// Lightning Bolt is neither.
	soloCtx := buildContextFor(
		[]CardProfile{{Name: "Lightning Bolt", TypeLine: "Instant"}},
		map[string]string{"Lightning Bolt": "lightning bolt deals 3 damage to any target."},
	)
	if soloCtx.cyclingCount != 0 || soloCtx.cyclingPayoffCount != 0 {
		t.Errorf("Lightning Bolt should not count as cycling, got count=%d payoff=%d",
			soloCtx.cyclingCount, soloCtx.cyclingPayoffCount)
	}
}

// TestBuildClassifyContext_ToxicInfectCounter pins detection of the
// poison-axis cards: explicit "infect" keyword wording, modern toxic-N
// keyword, and "poison counter" payoffs. A vanilla +1/+1-counter card
// (Hardened Scales) must NOT count — that's the Counters Matter axis.
func TestBuildClassifyContext_ToxicInfectCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Glistener Elf", TypeLine: "Creature"},
		{Name: "Phyrexian Crusader", TypeLine: "Creature"},
		{Name: "Ichor Rats", TypeLine: "Creature"},
		{Name: "Tainted Strike", TypeLine: "Instant"},
		{Name: "Ixhel, Scion of Atraxa", TypeLine: "Legendary Creature"},
		{Name: "Vraska, Betrayal's Sting", TypeLine: "Planeswalker"},
		{Name: "Hardened Scales", TypeLine: "Enchantment"},
	}
	oracleText := map[string]string{
		"Glistener Elf":          "infect (this creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters.)",
		"Phyrexian Crusader":     "first strike, protection from red and from white, infect (this creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters.)",
		"Ichor Rats":             "infect (this creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters.) when ichor rats enters, each opponent gets a poison counter.",
		"Tainted Strike":         "target creature gets +1/+0 and gains infect until end of turn.",
		"Ixhel, Scion of Atraxa": "at the beginning of your end step, for each opponent who has three or more poison counters, you draw a card and lose 1 life. corrupted — whenever a creature deals combat damage to an opponent who has three or more poison counters, that opponent gets a poison counter.",
		"Vraska, Betrayal's Sting": "+1: target opponent gets two poison counters. -3: destroy target creature or planeswalker. compleated. -8: target opponent who has three or more poison counters loses the game.",
		"Hardened Scales":        "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on that creature instead.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.toxicInfectCount < 5 {
		t.Errorf("toxicInfectCount=%d, want >=5 (5 of the 6 poison-axis cards minimum)", ctx.toxicInfectCount)
	}
	// Hardened Scales (+1/+1 counters, not poison) must not count.
	soloCtx := buildContextFor(
		[]CardProfile{{Name: "Hardened Scales", TypeLine: "Enchantment"}},
		map[string]string{"Hardened Scales": "if one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on that creature instead."},
	)
	if soloCtx.toxicInfectCount != 0 {
		t.Errorf("Hardened Scales (+1/+1 counters) should not count as toxic, got %d", soloCtx.toxicInfectCount)
	}
}

// TestBuildClassifyContext_VehicleCounter pins that the Vehicle (and
// Spacecraft) typeline drives the counter directly — most vehicle
// decks ship 10+ vehicles and the typeline is the load-bearing signal.
// A vanilla artifact (Sol Ring) must NOT count even though it's an
// artifact.
func TestBuildClassifyContext_VehicleCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Smuggler's Copter", TypeLine: "Artifact — Vehicle"},
		{Name: "Heart of Kiran", TypeLine: "Legendary Artifact — Vehicle"},
		{Name: "Cultivator's Caravan", TypeLine: "Artifact — Vehicle"},
		{Name: "Esika's Chariot", TypeLine: "Legendary Artifact — Vehicle"},
		{Name: "Parhelion II", TypeLine: "Legendary Artifact — Vehicle"},
		{Name: "Mobilizer Mech", TypeLine: "Artifact — Vehicle"},
		{Name: "Depala, Pilot Exemplar", TypeLine: "Legendary Creature — Dwarf Pilot"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Smuggler's Copter":      "flying. whenever smuggler's copter attacks or blocks, you may draw a card. then discard a card. crew 1",
		"Heart of Kiran":         "flying, vigilance. crew 3. you may remove a loyalty counter from a planeswalker you control rather than pay heart of kiran's crew cost.",
		"Cultivator's Caravan":   "vigilance. crew 3. {t}: add one mana of any color.",
		"Esika's Chariot":        "trample. when esika's chariot enters, create two 2/2 green cat creature tokens. whenever esika's chariot attacks, create a token that's a copy of target token you control. crew 4",
		"Parhelion II":           "flying. whenever parhelion ii attacks, create two 4/4 white angel creature tokens with flying and vigilance that are attacking. crew 4.",
		"Mobilizer Mech":         "trample, haste. crew 2",
		"Depala, Pilot Exemplar": "whenever depala, pilot exemplar attacks, put a +1/+1 counter on each other dwarf and vehicle you control. {x}{w}: reveal the top x cards of your library. put all dwarf and vehicle cards from among them into your hand and the rest on the bottom of your library in a random order.",
		"Sol Ring":               "{t}: add {c}{c}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	// 6 vehicles + Depala (no vehicle typeline but matches "vehicles
	// you control" payoff text) — Sol Ring should not contribute.
	if ctx.vehicleCount < 6 {
		t.Errorf("vehicleCount=%d, want >=6", ctx.vehicleCount)
	}
	soloCtx := buildContextFor(
		[]CardProfile{{Name: "Sol Ring", TypeLine: "Artifact"}},
		map[string]string{"Sol Ring": "{t}: add {c}{c}."},
	)
	if soloCtx.vehicleCount != 0 {
		t.Errorf("Sol Ring should not count as vehicle, got %d", soloCtx.vehicleCount)
	}
}

// ---------------------------------------------------------------------------
// End-to-end classification — full ClassifyArchetype pipeline reaches
// the new fingerprint for the new archetypes.
// ---------------------------------------------------------------------------

// TestClassifyArchetype_GroupHugDeck pins that 6+ group-hug cards in a
// Kynaios shell classify as Group Hug — and not as Pillowfort (which
// also targets defensive multi-player effects).
func TestClassifyArchetype_GroupHugDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Howling Mine", TypeLine: "Artifact"},
		{Name: "Font of Mythos", TypeLine: "Artifact"},
		{Name: "Temple Bell", TypeLine: "Artifact"},
		{Name: "Heartbeat of Spring", TypeLine: "Enchantment"},
		{Name: "Edric, Spymaster of Trest", TypeLine: "Legendary Creature"},
		{Name: "Kynaios and Tiro of Meletis", TypeLine: "Legendary Creature"},
		{Name: "Veteran Explorer", TypeLine: "Creature"},
	}
	oracleText := map[string]string{
		"Howling Mine":                "each player draws an extra card.",
		"Font of Mythos":              "each player draws two additional cards.",
		"Temple Bell":                 "{t}: each player draws a card.",
		"Heartbeat of Spring":         "whenever a player taps a land for mana, that player adds one mana of any type that land produced.",
		"Edric, Spymaster of Trest":   "whenever a creature deals combat damage to one of your opponents, its controller may draw a card.",
		"Kynaios and Tiro of Meletis": "whenever kynaios attacks, each player gains 1 life and draws a card. each player may put a land card from their hand onto the battlefield.",
		"Veteran Explorer":            "when veteran explorer dies, each player may search their library for up to two basic land cards.",
	}
	roleCounts := map[RoleTag]int{
		RoleDraw: 18, RoleRamp: 12, RoleProtection: 8,
		RoleRemoval: 6, RoleThreat: 4, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Group Hug" {
		t.Errorf("primary archetype = %q, want Group Hug", ac.Primary)
	}
}

// TestClassifyArchetype_CyclingDeck pins that a Gavi-shape cycling
// deck (3 payoffs + 12 cyclers) lands on Cycling.
func TestClassifyArchetype_CyclingDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Astral Drift", TypeLine: "Enchantment"},
		{Name: "Drake Haven", TypeLine: "Enchantment"},
		{Name: "New Perspectives", TypeLine: "Enchantment"},
	}
	oracleText := map[string]string{
		"Astral Drift":     "whenever you cycle a card, you may exile target creature.",
		"Drake Haven":      "whenever you cycle or discard a card, you may pay {1}. create a 2/2 drake.",
		"New Perspectives": "cycling abilities you activate cost {0}.",
	}
	// 12 cyclers so cyclingCount fires the Require threshold.
	for i := 0; i < 12; i++ {
		name := "Cycler" + string(rune('A'+i))
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature"})
		oracleText[name] = "cycling {2}"
	}
	roleCounts := map[RoleTag]int{
		RoleDraw: 16, RoleRamp: 8, RoleRemoval: 8, RoleThreat: 8, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Cycling" {
		t.Errorf("primary archetype = %q, want Cycling", ac.Primary)
	}
}

// TestClassifyArchetype_CyclingDeck_NoPayoffsFallsThrough pins the
// distinction: 12 cyclers WITHOUT cycling payoffs is NOT Cycling — it's
// midrange-with-utility-lands. This guards against false positives on
// any deck running a few cycling lands plus generic Amonkhet cyclers.
func TestClassifyArchetype_CyclingDeck_NoPayoffsFallsThrough(t *testing.T) {
	var profiles []CardProfile
	oracleText := map[string]string{}
	for i := 0; i < 12; i++ {
		name := "Cycler" + string(rune('A'+i))
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Land"})
		oracleText[name] = "{t}: add {u}. cycling {2}"
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 12, RoleRemoval: 10, RoleDraw: 10, RoleRamp: 10, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Cycling" {
		t.Errorf("12 cyclers + 0 payoffs should NOT classify as Cycling, got %q", ac.Primary)
	}
}

// TestClassifyArchetype_ToxicDeck pins that 8+ infect/toxic cards
// classify as Toxic (the Ixhel/Atraxa-poison shape).
func TestClassifyArchetype_ToxicDeck(t *testing.T) {
	infectCards := []string{
		"Glistener Elf", "Plague Stinger", "Ichorclaw Myr", "Phyrexian Crusader",
		"Cystbearer", "Tangle Angler", "Ichor Rats", "Skithiryx, the Blight Dragon",
	}
	profiles := make([]CardProfile, 0, len(infectCards))
	oracleText := map[string]string{}
	for _, name := range infectCards {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Creature"})
		oracleText[name] = "infect (this creature deals damage to creatures in the form of -1/-1 counters and to players in the form of poison counters.)"
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 14, RoleRemoval: 8, RoleDraw: 8, RoleRamp: 8, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Toxic" {
		t.Errorf("primary archetype = %q, want Toxic", ac.Primary)
	}
}

// TestClassifyArchetype_VehiclesDeck pins the Kotori / Inspirit
// fix: 6+ vehicles classify as Vehicles, not Artifacts. Before this
// fingerprint they landed on Artifacts because the typeline contains
// "artifact" and there was no vehicle-specific rule.
func TestClassifyArchetype_VehiclesDeck(t *testing.T) {
	vehicles := []string{
		"Smuggler's Copter", "Heart of Kiran", "Cultivator's Caravan",
		"Esika's Chariot", "Parhelion II", "Mobilizer Mech",
		"Skysovereign, Consul Flagship", "Aerial Surveyor",
	}
	profiles := make([]CardProfile, 0, len(vehicles))
	oracleText := map[string]string{}
	for _, name := range vehicles {
		profiles = append(profiles, CardProfile{Name: name, TypeLine: "Artifact — Vehicle"})
		oracleText[name] = "crew 3"
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 16, RoleRamp: 10, RoleDraw: 8, RoleRemoval: 6, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Vehicles" {
		t.Errorf("primary archetype = %q, want Vehicles (Kotori/Inspirit precons were misclassified as Artifacts before this fingerprint)", ac.Primary)
	}
}

// ---------------------------------------------------------------------------
// Consistency: docs ↔ fingerprints ↔ buildIntent.
// ---------------------------------------------------------------------------

// TestPreconArchetypes_DocsMatchFingerprints extends the r60 consistency
// check to the four post-precon-audit archetypes. Every name must appear
// in BOTH archetypes.go (docs list) and archetype.go's
// archetypeFingerprints (live classifier).
func TestPreconArchetypes_DocsMatchFingerprints(t *testing.T) {
	required := []string{"Group Hug", "Cycling", "Toxic", "Vehicles"}

	docs := map[string]bool{}
	for _, a := range Archetypes {
		docs[a.Name] = true
	}
	fps := map[string]bool{}
	for _, fp := range archetypeFingerprints {
		fps[fp.Name] = true
	}
	for _, name := range required {
		if !docs[name] {
			t.Errorf("missing %q from Archetypes docs list", name)
		}
		if !fps[name] {
			t.Errorf("missing %q from archetypeFingerprints", name)
		}
	}
}

// TestPreconArchetypes_BuildIntentCovers pins that each new archetype
// has a tailored buildIntent gameplan line — never falling through to
// the generic "execute its game plan" default.
func TestPreconArchetypes_BuildIntentCovers(t *testing.T) {
	cases := []struct {
		archetype   string
		mustContain string
	}{
		{"Group Hug", "every player"},
		{"Cycling", "cycle"},
		{"Toxic", "poison"},
		{"Vehicles", "crew"},
	}
	for _, tc := range cases {
		ac := &ArchetypeClassification{Primary: tc.archetype}
		ctx := &classifyContext{avgCMC: 3.0}
		intent := buildIntent(ac, &FreyaReport{}, ctx)
		if !strings.Contains(strings.ToLower(intent), strings.ToLower(tc.mustContain)) {
			t.Errorf("%s intent should contain %q, got %q",
				tc.archetype, tc.mustContain, intent)
		}
		if strings.Contains(intent, "execute its game plan through incremental advantage") {
			t.Errorf("%s fell through to generic intent default: %q", tc.archetype, intent)
		}
	}
}
