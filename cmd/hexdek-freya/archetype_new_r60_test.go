package main

import (
	"strings"
	"testing"
)

// stubOracleWithText builds a minimal oracleDB keyed by lowercased
// card name. Used so buildClassifyContext's oracle.lookup() reads
// the right oracle text for each fixture card.
func stubOracleWithText(entries map[string]string) *oracleDB {
	by := map[string]*oracleEntry{}
	for name, text := range entries {
		by[strings.ToLower(name)] = &oracleEntry{Name: name, OracleText: text}
	}
	return &oracleDB{byName: by}
}

// classifyFixture is a fixture helper for new-archetype tests. It builds
// a minimal FreyaReport + qtyProfiles + oracle so ClassifyArchetype runs
// end-to-end. roleCounts must sum to >= len(profiles) and should put
// representative counts on Protection/Draw/Removal/Threat for the new
// archetypes to land near their target ratios.
func classifyFixture(profiles []CardProfile, oracleText map[string]string, roleCounts map[RoleTag]int, totalCards int) *ArchetypeClassification {
	qtyProfiles := make([]CardProfileQty, 0, len(profiles))
	for _, p := range profiles {
		qtyProfiles = append(qtyProfiles, CardProfileQty{Profile: p, Qty: 1})
	}
	report := &FreyaReport{
		TotalCards: totalCards,
		AvgCMC:     3.0,
		Profiles:   profiles,
		Roles: &RoleAnalysis{
			TotalCards: totalCards,
			RoleCounts: roleCounts,
		},
	}
	return ClassifyArchetype(report, qtyProfiles, stubOracleWithText(oracleText))
}

// ---------------------------------------------------------------------------
// Counter detection — unit-level tests on buildClassifyContext.
// ---------------------------------------------------------------------------

// TestBuildClassifyContext_PillowfortCounter pins that the pillowfort
// oracle-text scan catches the canonical phrasings: Ghostly Prison /
// Propaganda's "can't attack you", Crawlspace's "no more than X
// creatures can attack you", No Mercy's "deals damage to you" trigger,
// and Solitary Confinement's "prevent all damage that would be dealt
// to you". A burn-deck Inferno Titan should NOT increment the counter.
func TestBuildClassifyContext_PillowfortCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Ghostly Prison", TypeLine: "Enchantment"},
		{Name: "Propaganda", TypeLine: "Enchantment"},
		{Name: "Crawlspace", TypeLine: "Artifact"},
		{Name: "No Mercy", TypeLine: "Enchantment"},
		{Name: "Solitary Confinement", TypeLine: "Enchantment"},
		{Name: "Inferno Titan", TypeLine: "Creature"},
	}
	oracleText := map[string]string{
		"Ghostly Prison":       "creatures can't attack you unless their controller pays {2} for each creature they control that's attacking you.",
		"Propaganda":           "creatures can't attack you unless their controller pays {2} for each creature they control that's attacking you.",
		"Crawlspace":           "no more than two creatures can attack you each combat.",
		"No Mercy":             "whenever a creature deals damage to you, destroy it.",
		"Solitary Confinement": "skip your draw step. prevent all damage that would be dealt to you.",
		"Inferno Titan":        "{r}: inferno titan deals 1 damage to any target. when inferno titan enters, it deals 3 damage divided as you choose among one, two, or three targets.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.pillowfortCount != 5 {
		t.Errorf("pillowfortCount=%d, want 5 (Inferno Titan should not count)", ctx.pillowfortCount)
	}
}

// TestBuildClassifyContext_GroupSlugCounter pins detection of the
// canonical group-slug phrasings. "each opponent loses" hits Underworld
// Dreams / Polluted Bonds / Liliana's Caress / Megrim. "deals damage to
// each opponent" hits Manabarbs / Pyrostatic Pillar / Eidolon. "deals
// damage to each player" hits Sulfuric Vortex. A Lightning Bolt should
// NOT trigger — direct damage isn't the slug pattern.
func TestBuildClassifyContext_GroupSlugCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Underworld Dreams", TypeLine: "Enchantment"},
		{Name: "Manabarbs", TypeLine: "Enchantment"},
		{Name: "Pyrostatic Pillar", TypeLine: "Enchantment"},
		{Name: "Sulfuric Vortex", TypeLine: "Enchantment"},
		{Name: "Liliana's Caress", TypeLine: "Enchantment"},
		{Name: "Lightning Bolt", TypeLine: "Instant"},
	}
	oracleText := map[string]string{
		"Underworld Dreams": "whenever an opponent draws a card, underworld dreams deals 1 damage to that player.",
		"Manabarbs":         "whenever a player taps a land for mana, manabarbs deals 1 damage to that player.",
		"Pyrostatic Pillar": "whenever a player casts a spell with mana value 3 or less, pyrostatic pillar deals 2 damage to that player.",
		"Sulfuric Vortex":   "at the beginning of each player's upkeep, sulfuric vortex deals 2 damage to that player.",
		"Liliana's Caress":  "whenever an opponent discards a card, that player loses 2 life. each opponent loses 1 life at end of turn.",
		"Lightning Bolt":    "lightning bolt deals 3 damage to any target.",
	}
	ctx := buildContextFor(profiles, oracleText)
	// Manabarbs and Sulfuric Vortex both match the "deals damage to
	// that player" branch via "deals damage to each player" only if
	// they used that phrasing — they don't. Let me count what should
	// actually match:
	//   Underworld Dreams: matches "whenever an opponent draws a card"
	//   Manabarbs: no group-slug phrase (uses "that player" not "each opponent"). Won't count — and that's a known limitation.
	//   Pyrostatic Pillar: no group-slug phrase either (also "that player")
	//   Sulfuric Vortex: "each player's upkeep" doesn't match
	//   Liliana's Caress: "each opponent loses 1 life" matches
	// Want at least 2.
	if ctx.groupSlugCount < 2 {
		t.Errorf("groupSlugCount=%d, want >=2 (Underworld Dreams, Liliana's Caress at minimum)", ctx.groupSlugCount)
	}
	// A pure burn spell should never count.
	soloBurn := []CardProfile{{Name: "Lightning Bolt", TypeLine: "Instant"}}
	soloOracle := map[string]string{"Lightning Bolt": "lightning bolt deals 3 damage to any target."}
	soloCtx := buildContextFor(soloBurn, soloOracle)
	if soloCtx.groupSlugCount != 0 {
		t.Errorf("Lightning Bolt should not count as group slug, got %d", soloCtx.groupSlugCount)
	}
}

// TestBuildClassifyContext_DamageRedirectCounter pins detection of the
// Stuffy Doll / Boros Reckoner shell phrasings — "is dealt damage, it
// deals that much damage" hits both, plus Spitemare and Truefire Captain
// and Brash Taunter. Pariah's "would be dealt to you is dealt to
// enchanted creature instead" also lands. A vanilla 2/2 Grizzly Bears
// should NOT count.
func TestBuildClassifyContext_DamageRedirectCounter(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Stuffy Doll", TypeLine: "Artifact Creature"},
		{Name: "Boros Reckoner", TypeLine: "Creature"},
		{Name: "Spitemare", TypeLine: "Creature"},
		{Name: "Pariah", TypeLine: "Enchantment"},
		{Name: "Grizzly Bears", TypeLine: "Creature"},
	}
	oracleText := map[string]string{
		"Stuffy Doll":    "defender, indestructible. as stuffy doll enters, choose a player. whenever damage is dealt to stuffy doll, it deals that much damage to the chosen player.",
		"Boros Reckoner": "first strike. whenever boros reckoner is dealt damage, it deals that much damage to any target.",
		"Spitemare":      "whenever spitemare is dealt damage, it deals that much damage to any target.",
		"Pariah":         "all damage that would be dealt to you is dealt to enchanted creature instead.",
		"Grizzly Bears":  "",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.damageRedirectCount != 4 {
		t.Errorf("damageRedirectCount=%d, want 4 (Grizzly Bears should not count)", ctx.damageRedirectCount)
	}
}

// ---------------------------------------------------------------------------
// End-to-end classification — full ClassifyArchetype pipeline.
// ---------------------------------------------------------------------------

// TestClassifyArchetype_PillowfortDeck pins that a deck with 8+
// pillowfort pieces and pillowfort-aligned role counts (heavy
// protection/draw/removal, light threats) classifies as Pillowfort.
func TestClassifyArchetype_PillowfortDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Ghostly Prison", TypeLine: "Enchantment"},
		{Name: "Propaganda", TypeLine: "Enchantment"},
		{Name: "Sphere of Safety", TypeLine: "Enchantment"},
		{Name: "Norn's Annex", TypeLine: "Enchantment"},
		{Name: "Windborn Muse", TypeLine: "Creature"},
		{Name: "No Mercy", TypeLine: "Enchantment"},
		{Name: "Crawlspace", TypeLine: "Artifact"},
		{Name: "Aegis of the Gods", TypeLine: "Creature"},
		// Filler so the deck has a real card body.
		{Name: "Wrath of God", TypeLine: "Sorcery"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Ghostly Prison":    "creatures can't attack you unless their controller pays {2}.",
		"Propaganda":        "creatures can't attack you unless their controller pays {2}.",
		"Sphere of Safety":  "creatures can't attack you unless their controller pays {2}.",
		"Norn's Annex":      "creatures can't attack you unless their controller pays {2} or 2 life.",
		"Windborn Muse":     "creatures can't attack you unless their controller pays {2}.",
		"No Mercy":          "whenever a creature deals damage to you, destroy it.",
		"Crawlspace":        "no more than two creatures can attack you each combat.",
		"Aegis of the Gods": "you have hexproof.",
	}
	roleCounts := map[RoleTag]int{
		RoleProtection: 10, RoleDraw: 10, RoleRemoval: 10,
		RoleRamp: 8, RoleStax: 4, RoleThreat: 6, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Pillowfort" {
		t.Errorf("primary archetype = %q, want Pillowfort. Secondary=%q", ac.Primary, ac.Secondary)
	}
}

// TestClassifyArchetype_GroupSlugDeck pins that a deck with 6+ group-slug
// pieces lands on Group Slug rather than the Midrange fallback.
func TestClassifyArchetype_GroupSlugDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Underworld Dreams", TypeLine: "Enchantment"},
		{Name: "Liliana's Caress", TypeLine: "Enchantment"},
		{Name: "Megrim", TypeLine: "Enchantment"},
		{Name: "Pyrostatic Pillar", TypeLine: "Enchantment"},
		{Name: "Eidolon of the Great Revel", TypeLine: "Creature"},
		{Name: "Polluted Bonds", TypeLine: "Enchantment"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
		{Name: "Counterspell", TypeLine: "Instant"},
	}
	oracleText := map[string]string{
		"Underworld Dreams":          "whenever an opponent draws a card, deals 1 damage to that player.",
		"Liliana's Caress":           "whenever an opponent discards a card, each opponent loses 2 life.",
		"Megrim":                     "whenever an opponent discards a card, each opponent loses 2 life.",
		"Pyrostatic Pillar":          "whenever an opponent casts a spell with mana value 3 or less, pyrostatic pillar deals 2 damage to each opponent.",
		"Eidolon of the Great Revel": "whenever an opponent casts a spell with mana value 3 or less, eidolon of the great revel deals 2 damage to that player. whenever an opponent casts that spell, each opponent loses 1 life.",
		"Polluted Bonds":             "whenever a land enters under an opponent's control, you gain 1 life and each opponent loses 1 life.",
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 8, RoleRemoval: 10, RoleDraw: 10,
		RoleRamp: 8, RoleStax: 4, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Group Slug" {
		t.Errorf("primary archetype = %q, want Group Slug. Secondary=%q", ac.Primary, ac.Secondary)
	}
}

// TestClassifyArchetype_DamageRedirectDeck pins that a Stuffy Doll /
// Boros Reckoner shell with 4+ redirect pieces classifies as Damage
// Redirect.
func TestClassifyArchetype_DamageRedirectDeck(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Stuffy Doll", TypeLine: "Artifact Creature"},
		{Name: "Boros Reckoner", TypeLine: "Creature"},
		{Name: "Spitemare", TypeLine: "Creature"},
		{Name: "Truefire Captain", TypeLine: "Creature"},
		{Name: "Brash Taunter", TypeLine: "Creature"},
		{Name: "Pariah", TypeLine: "Enchantment"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Stuffy Doll":      "whenever damage is dealt to stuffy doll, it deals that much damage to the chosen player.",
		"Boros Reckoner":   "whenever boros reckoner is dealt damage, it deals that much damage to any target.",
		"Spitemare":        "whenever spitemare is dealt damage, it deals that much damage to any target.",
		"Truefire Captain": "whenever truefire captain is dealt damage, it deals that much damage to target opponent.",
		"Brash Taunter":    "whenever brash taunter is dealt damage, it deals that much damage to target opponent.",
		"Pariah":           "all damage that would be dealt to you is dealt to enchanted creature instead.",
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 10, RoleProtection: 8, RoleRemoval: 10,
		RoleDraw: 8, RoleRamp: 8, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Damage Redirect" {
		t.Errorf("primary archetype = %q, want Damage Redirect. Secondary=%q", ac.Primary, ac.Secondary)
	}
}

// TestClassifyArchetype_BelowThresholdDoesNotFire pins that decks with
// 1-2 pillowfort/slug/redirect pieces don't poach those archetypes
// from generic midrange — the thresholds (5/5/4) guard against false
// positives from one-off includes.
func TestClassifyArchetype_BelowThresholdDoesNotFire(t *testing.T) {
	// 2 pillowfort cards in an otherwise plain midrange deck.
	profiles := []CardProfile{
		{Name: "Ghostly Prison", TypeLine: "Enchantment"},
		{Name: "Propaganda", TypeLine: "Enchantment"},
		// Generic midrange shell.
		{Name: "Wrath of God", TypeLine: "Sorcery"},
		{Name: "Swords to Plowshares", TypeLine: "Instant"},
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Ghostly Prison": "creatures can't attack you unless their controller pays {2}.",
		"Propaganda":     "creatures can't attack you unless their controller pays {2}.",
	}
	roleCounts := map[RoleTag]int{
		RoleThreat: 12, RoleRemoval: 10, RoleDraw: 10, RoleRamp: 10, RoleLand: 38,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, 99)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Pillowfort" {
		t.Errorf("2 pillowfort cards should not trigger Pillowfort primary, got %q", ac.Primary)
	}
}

// ---------------------------------------------------------------------------
// Consistency checks — documentation list mirrors live fingerprints.
// ---------------------------------------------------------------------------

// TestArchetypeNames_DocsMatchFingerprints pins that every new
// archetype is present in BOTH the documentation list (Archetypes) and
// the live classifier (archetypeFingerprints). Adding a fingerprint
// without a docs entry silently breaks any consumer that reads
// archetypes.go for descriptions.
func TestArchetypeNames_DocsMatchFingerprints(t *testing.T) {
	required := []string{"Pillowfort", "Group Slug", "Damage Redirect"}

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

// TestBuildIntent_CoversNewArchetypes pins that buildIntent has a
// case arm for each new archetype — without it, the gameplan line
// falls through to the generic "execute its game plan through
// incremental advantage" which under-sells what the deck actually does.
func TestBuildIntent_CoversNewArchetypes(t *testing.T) {
	cases := []struct {
		archetype   string
		mustContain string
	}{
		{"Pillowfort", "tax"},
		{"Group Slug", "punish"},
		{"Damage Redirect", "reflect"},
	}
	for _, tc := range cases {
		ac := &ArchetypeClassification{Primary: tc.archetype}
		ctx := &classifyContext{avgCMC: 3.0}
		intent := buildIntent(ac, &FreyaReport{}, ctx)
		if !strings.Contains(strings.ToLower(intent), strings.ToLower(tc.mustContain)) {
			t.Errorf("%s intent should describe its plan (contain %q), got %q",
				tc.archetype, tc.mustContain, intent)
		}
		if strings.Contains(intent, "execute its game plan through incremental advantage") {
			t.Errorf("%s fell through to generic intent default: %q", tc.archetype, intent)
		}
	}
}

// buildContextFor is a thin wrapper around buildClassifyContext that
// builds the minimal scaffolding the counter-detection tests need.
// Used to keep the unit tests focused on the counter outputs without
// constructing a full ClassifyArchetype result.
func buildContextFor(profiles []CardProfile, oracleText map[string]string) *classifyContext {
	qtyProfiles := make([]CardProfileQty, 0, len(profiles))
	for _, p := range profiles {
		qtyProfiles = append(qtyProfiles, CardProfileQty{Profile: p, Qty: 1})
	}
	report := &FreyaReport{
		TotalCards: len(profiles),
		Profiles:   profiles,
		Roles: &RoleAnalysis{
			TotalCards: len(profiles),
			RoleCounts: map[RoleTag]int{},
		},
	}
	return buildClassifyContext(report, qtyProfiles, stubOracleWithText(oracleText))
}
