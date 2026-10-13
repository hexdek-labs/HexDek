package gameengine

import "testing"

// r81 — batch 5 of the keyword call-site migration: keywords_p0.go.
//
// Three reads, all rules enforcement: infect (CR §702.90, damage becomes
// poison counters / -1/-1 counters), landwalk (CR §702.13-style evasion,
// consulted by canBlockGS), and exalted (CR §702.82).
//
// Checked the other arm first, per the r79 lesson: all three call sites
// are single-predicate conditions, no second surface hiding behind them.
// Checked the hot path too, per r80: LandwalkType runs inside canBlockGS,
// but its five reads are one cache miss plus four hits, and canBlockGS
// already does a layer query immediately after via ColorsOf.

func TestKeywordMigrationR81_InfectGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Infector", 2, 2, "creature")
	r70Grant(gs, p, "infect", "r81_infect_grant")
	if !HasInfect(gs, p) {
		t.Error("layer-granted infect not seen by HasInfect")
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Infector", 2, 2, "creature")
	r72Stamp(gs2, q, "infect")
	if !HasInfect(gs2, q) {
		t.Fatal("precondition: stamped infect reads true")
	}
	r70Strip(gs2, q, "r81_infect_strip")
	if HasInfect(gs2, q) {
		t.Error("infect survived a layer-6 ability strip")
	}
}

func TestKeywordMigrationR81_LandwalkGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Walker", 2, 2, "creature")
	r70Grant(gs, p, "swampwalk", "r81_walk_grant")
	if lt := LandwalkType(gs, p); lt != "swamp" {
		t.Errorf("layer-granted swampwalk gave land type %q, want \"swamp\"", lt)
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Walker", 2, 2, "creature")
	r72Stamp(gs2, q, "swampwalk")
	if lt := LandwalkType(gs2, q); lt != "swamp" {
		t.Fatalf("precondition: stamped swampwalk gave %q", lt)
	}
	r70Strip(gs2, q, "r81_walk_strip")
	if lt := LandwalkType(gs2, q); lt != "" {
		t.Errorf("swampwalk survived a layer-6 ability strip: %q", lt)
	}
}

// Landwalk through the entry point canBlockGS actually uses — a granted
// swampwalk must make the block illegal when the defender controls a
// Swamp, which is the behaviour the migration exists for.
func TestKeywordMigrationR81_LandwalkReachesBlockLegality(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Granted Walker", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Blocker", 2, 2, "creature")
	swamp := addBattlefield(gs, 1, "Swamp", 0, 0, "land")
	swamp.Card.Types = []string{"land", "swamp"}
	gs.InvalidateCharacteristicsCache()
	if !canBlockGS(gs, atk, blk) {
		t.Fatal("precondition: a plain creature may block")
	}
	r70Grant(gs, atk, "swampwalk", "r81_walk_block_grant")
	if canBlockGS(gs, atk, blk) {
		t.Error("layer-granted swampwalk did not reach canBlockGS — " +
			"a defender controlling a Swamp still blocked")
	}
}

func TestKeywordMigrationR81_ExaltedGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Granted Exalted", 2, 2, "creature")
	r70Grant(gs, p, "exalted", "r81_exalted_grant")
	if n := CountExalted(gs, 0); n != 1 {
		t.Errorf("layer-granted exalted counted %d, want 1", n)
	}

	gs2 := newFixtureGame(t)
	q := addBattlefield(gs2, 0, "Stamped Exalted", 2, 2, "creature")
	r72Stamp(gs2, q, "exalted")
	if n := CountExalted(gs2, 0); n != 1 {
		t.Fatalf("precondition: stamped exalted counted %d, want 1", n)
	}
	r70Strip(gs2, q, "r81_exalted_strip")
	if n := CountExalted(gs2, 0); n != 0 {
		t.Errorf("exalted survived a layer-6 ability strip: counted %d", n)
	}
}
