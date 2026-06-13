package gameengine

import "testing"

// mod_kind_aura_grant_test.go — regressions for the generic aura_grant
// handler (worker hex-dev-5). aura_grant = keyword-only auras
// ("enchanted creature has <keyword>"); previously inert.

func TestAuraGrant_GrantsKeywordToEnchanted(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	// "Flight" — enchanted creature has flying.
	attachBuffPerm(gs, 0, "Flight", "aura_grant", bear, "flying", enchantedFilter("enchanted_creature"))
	if !gs.HasKeywordOf(bear, "flying") {
		t.Fatalf("aura_grant: bear should have flying")
	}
}

func TestAuraGrant_MultiKeyword(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	attachBuffPerm(gs, 0, "Twin Bolt Aura", "aura_grant", bear, "flying and hexproof", enchantedFilter("enchanted_creature"))
	if !gs.HasKeywordOf(bear, "flying") || !gs.HasKeywordOf(bear, "hexproof") {
		t.Fatalf("aura_grant: bear should have flying AND hexproof")
	}
}

func TestAuraGrant_KeywordHonoredAlongsideLosesClause(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Flyer", 2, 2, "creature")
	// Sky Tether: "defender and loses flying" — we grant the whitelisted
	// keyword (defender) and conservatively skip the "loses flying" removal.
	attachBuffPerm(gs, 0, "Sky Tether", "aura_grant", bear, "defender and loses flying", enchantedFilter("enchanted_creature"))
	if !gs.HasKeywordOf(bear, "defender") {
		t.Fatalf("aura_grant: bear should gain defender from Sky Tether")
	}
}

func TestAuraGrant_SkipsProtectionAndNonKeywords(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	// "protection from green and from blue" — not a bare evergreen keyword;
	// must be skipped, not injected as a garbage keyword string.
	attachBuffPerm(gs, 0, "Shield", "aura_grant", bear, "protection from green and from blue", enchantedFilter("enchanted_creature"))
	if gs.HasKeywordOf(bear, "protection from green") || gs.HasKeywordOf(bear, "protection") {
		t.Fatalf("aura_grant: protection clause must be skipped, not granted")
	}
}

func TestEquipGrant_GrantsKeywordToEquipped(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	// Quietus Spike-style: equipped creature has deathtouch.
	attachBuffPerm(gs, 0, "Gorgon's Head", "equip_grant", bear, "deathtouch", enchantedFilter("equipped_creature"))
	if !gs.HasKeywordOf(bear, "deathtouch") {
		t.Fatalf("equip_grant: equipped creature should have deathtouch")
	}
}

func TestEquipGrant_MultiKeywordAndExoticSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	attachBuffPerm(gs, 0, "Triple Blade", "equip_grant", bear, "first strike, trample, and haste", enchantedFilter("equipped_creature"))
	if !gs.HasKeywordOf(bear, "first strike") || !gs.HasKeywordOf(bear, "trample") || !gs.HasKeywordOf(bear, "haste") {
		t.Fatalf("equip_grant: should grant all three keywords")
	}
	// Exotic clause (P/T-set mislabelled) grants nothing.
	cat := addBattlefield(gs, 0, "Cat", 1, 1, "creature")
	attachBuffPerm(gs, 0, "Aettir", "equip_grant", cat, "base power and toughness x/x, where x is your life total", enchantedFilter("equipped_creature"))
	if gs.HasKeywordOf(cat, "base") {
		t.Fatalf("equip_grant: exotic non-keyword clause must be skipped")
	}
}

func TestAuraGrant_DynamicOnReattach(t *testing.T) {
	gs := newFixtureGame(t)
	a := addBattlefield(gs, 0, "A", 2, 2, "creature")
	b := addBattlefield(gs, 0, "B", 2, 2, "creature")
	aura := attachBuffPerm(gs, 0, "Flight", "aura_grant", a, "flying", enchantedFilter("enchanted_creature"))
	if !gs.HasKeywordOf(a, "flying") || gs.HasKeywordOf(b, "flying") {
		t.Fatalf("flying should be on A only")
	}
	// Re-attach to B — the dynamic predicate must follow.
	aura.AttachedTo = b
	gs.InvalidateCharacteristicsCache()
	if gs.HasKeywordOf(a, "flying") || !gs.HasKeywordOf(b, "flying") {
		t.Fatalf("after re-attach, flying should be on B only")
	}
}
