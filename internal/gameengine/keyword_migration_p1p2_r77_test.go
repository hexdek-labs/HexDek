package gameengine

import "testing"

// r77 — batch 3 of the keyword call-site migration: keywords_p1p2.go.
//
// Same contract as r70/r72 (helpers r70Grant / r70Strip / r72Stamp are
// reused here): each rules-enforcing keyword read moves off the
// layer-blind p.HasKeyword, and each keyword gets a regression proving
// BOTH directions — a §613 layer grant becomes live, and a layer-6
// "loses all abilities" wipe actually strips it.
//
// The important one in this batch is TARGETING. HasShroud/HasHexproof
// feed CanBeTargetedBy -> CanBeTargetedByCombat, which is called from
// targets.go (x3), announce_targets.go and zone_change.go (x2) — the
// paths that decide whether a spell may target a permanent at all. A
// layer-granted hexproof was invisible to every one of them, and no
// layer-6 strip could remove a stamped one.
//
// NOT migrated, deliberately, each documented at its definition:
// HasDevoid and HasP1P2Keyword (zero production callers).

// §702.11 hexproof — opponents cannot target; the controller still can.
func TestKeywordMigrationR77_HexproofGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Hexproof", 2, 2, "creature")
	if !CanBeTargetedBy(gs, p, 1) {
		t.Fatal("precondition: an opponent can target a plain creature")
	}
	r70Grant(gs, p, "hexproof", "r77_hex_grant")
	if CanBeTargetedBy(gs, p, 1) {
		t.Error("layer-granted hexproof did not reach the targeting check")
	}
	if !CanBeTargetedBy(gs, p, 0) {
		t.Error("hexproof wrongly stopped the CONTROLLER from targeting")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Hexproof", 2, 2, "creature")
	r72Stamp(gs2, q, "hexproof")
	if CanBeTargetedBy(gs2, q, 1) {
		t.Fatal("precondition: stamped hexproof blocks an opponent")
	}
	r70Strip(gs2, q, "r77_hex_strip")
	if !CanBeTargetedBy(gs2, q, 1) {
		t.Error("hexproof survived a layer-6 ability strip")
	}
}

// §702.18 shroud — nobody can target, including the controller.
func TestKeywordMigrationR77_ShroudGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Shroud", 2, 2, "creature")
	r70Grant(gs, p, "shroud", "r77_shroud_grant")
	if CanBeTargetedBy(gs, p, 1) {
		t.Error("layer-granted shroud did not stop an opponent targeting")
	}
	if CanBeTargetedBy(gs, p, 0) {
		t.Error("layer-granted shroud did not stop the CONTROLLER targeting")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Shroud", 2, 2, "creature")
	r72Stamp(gs2, q, "shroud")
	if CanBeTargetedBy(gs2, q, 0) {
		t.Fatal("precondition: stamped shroud blocks the controller")
	}
	r70Strip(gs2, q, "r77_shroud_strip")
	if !CanBeTargetedBy(gs2, q, 0) {
		t.Error("shroud survived a layer-6 ability strip")
	}
}

// The chain that actually matters: the targeting entry point the
// resolution paths call (targets.go, announce_targets.go, zone_change.go
// all route through CanBeTargetedByCombat).
func TestKeywordMigrationR77_HexproofReachesTheCombatTargetingEntryPoint(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Hexproof", 2, 2, "creature")
	src := &Card{Name: "Some Spell", Colors: []string{"U"}}
	if !CanBeTargetedByCombat(gs, p, 1, src) {
		t.Fatal("precondition: an opponent's spell can target a plain creature")
	}
	r70Grant(gs, p, "hexproof", "r77_hex_combat_grant")
	if CanBeTargetedByCombat(gs, p, 1, src) {
		t.Error("layer-granted hexproof did not reach CanBeTargetedByCombat — " +
			"the path targets.go/zone_change.go actually use")
	}
}

// §702.80 wither — damage to creatures becomes -1/-1 counters.
func TestKeywordMigrationR77_WitherGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Wither", 2, 2, "creature")
	r70Grant(gs, p, "wither", "r77_wither_grant")
	if !HasWither(gs, p) {
		t.Error("layer-granted wither not seen by HasWither")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Wither", 2, 2, "creature")
	r72Stamp(gs2, q, "wither")
	if !HasWither(gs2, q) {
		t.Fatal("precondition: stamped wither reads true")
	}
	r70Strip(gs2, q, "r77_wither_strip")
	if HasWither(gs2, q) {
		t.Error("wither survived a layer-6 ability strip")
	}
}

// §702.45 bushido. The Flags["bushido_n"] short-circuit is deliberately
// not set, as it would mask the keyword read under test.
func TestKeywordMigrationR77_BushidoGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Bushido", 2, 2, "creature")
	r70Grant(gs, p, "bushido", "r77_bushido_grant")
	if GetBushidoN(gs, p) <= 0 {
		t.Error("layer-granted bushido not seen by GetBushidoN")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Bushido", 2, 2, "creature")
	r72Stamp(gs2, q, "bushido")
	if GetBushidoN(gs2, q) <= 0 {
		t.Fatal("precondition: stamped bushido reads > 0")
	}
	r70Strip(gs2, q, "r77_bushido_strip")
	if n := GetBushidoN(gs2, q); n != 0 {
		t.Errorf("bushido survived a layer-6 ability strip: N=%d", n)
	}
}

// §702.31 horsemanship — blockable only by creatures with horsemanship.
func TestKeywordMigrationR77_HorsemanshipGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Horseman", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Footsoldier", 2, 2, "creature")
	if !CanBlockP1P2(gs, atk, blk) {
		t.Fatal("precondition: a plain creature may block")
	}
	r70Grant(gs, atk, "horsemanship", "r77_horse_grant")
	if CanBlockP1P2(gs, atk, blk) {
		t.Error("layer-granted horsemanship did not reach the block check")
	}

	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Horseman", 2, 2, "creature")
	blk2 := addBattlefield(gs2, 1, "Footsoldier", 2, 2, "creature")
	r72Stamp(gs2, atk2, "horsemanship")
	if CanBlockP1P2(gs2, atk2, blk2) {
		t.Fatal("precondition: stamped horsemanship restricts the block")
	}
	r70Strip(gs2, atk2, "r77_horse_strip")
	if !CanBlockP1P2(gs2, atk2, blk2) {
		t.Error("horsemanship survived a layer-6 ability strip")
	}
}

// §702.25 flanking — a blocker without flanking gets -1/-1.
func TestKeywordMigrationR77_FlankingGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Flanker", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Blocker", 2, 2, "creature")
	r70Grant(gs, atk, "flanking", "r77_flank_grant")
	FireFlankingTriggers(gs, []*Permanent{atk},
		map[*Permanent][]*Permanent{atk: {blk}})
	if len(blk.Modifications) == 0 {
		t.Error("layer-granted flanking did not debuff the blocker")
	}

	gs2 := newFixtureGame(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Flanker", 2, 2, "creature")
	blk2 := addBattlefield(gs2, 1, "Blocker", 2, 2, "creature")
	r72Stamp(gs2, atk2, "flanking")
	r70Strip(gs2, atk2, "r77_flank_strip")
	FireFlankingTriggers(gs2, []*Permanent{atk2},
		map[*Permanent][]*Permanent{atk2: {blk2}})
	if len(blk2.Modifications) != 0 {
		t.Error("flanking survived a layer-6 ability strip — blocker was debuffed")
	}
}
