package gameengine

import "testing"

// r70 — batch 1 of the keyword call-site migration: combat.go.
//
// r65/r66 made gs.HasKeywordOf a verified superset of p.HasKeyword and
// collapsed keywordActive onto it. This batch moves combat's remaining
// rules-enforcing reads — first strike, double strike, vigilance,
// defender, trample, deathtouch, lifelink — off the layer-blind query.
//
// Each test proves BOTH directions, because a migration that only
// added keywords would be half a change:
//
//	GRANT  a §613 layer effect confers the keyword -> combat obeys it
//	       (it was invisible before)
//	STRIP  a layer-6 "loses all abilities" effect removes it -> combat
//	       obeys that too (the flag stamp underneath used to win)

func r70Game(t *testing.T) *GameState {
	t.Helper()
	return newFixtureGame(t)
}

// grantKW registers a layer-6 continuous effect conferring kw on target.
func r70Grant(gs *GameState, target *Permanent, kw, id string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerAbility, HandlerID: id, Duration: DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == target },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Keywords = append(c.Keywords, kw)
		},
	})
	gs.InvalidateCharacteristicsCache()
}

// r70Strip registers a Humility-style layer-6 ability wipe.
func r70Strip(gs *GameState, target *Permanent, id string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerAbility, HandlerID: id, Duration: DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == target },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Keywords = nil
			c.Abilities = nil
			c.LostAllAbilities = true
		},
	})
	gs.InvalidateCharacteristicsCache()
}

// §702.3 vigilance — DeclareAttackers taps attackers that lack it.
func TestKeywordMigrationR70_VigilanceGrantAndStrip(t *testing.T) {
	// GRANT: a layer-granted vigilance must stop the tap.
	gs := r70Game(t)
	a := addBattlefield(gs, 0, "Granted Vigilant", 2, 2, "creature")
	r70Grant(gs, a, "vigilance", "r70_vig_grant")
	DeclareAttackers(gs, 0)
	if a.Tapped {
		t.Error("layer-granted vigilance did not reach DeclareAttackers — attacker was tapped")
	}

	// STRIP: a stamped vigilance removed by a layer-6 wipe must tap.
	gs2 := r70Game(t)
	b := addBattlefield(gs2, 0, "Stamped Vigilant", 2, 2, "creature")
	b.Flags = map[string]int{"kw:vigilance": 1}
	gs2.InvalidateCharacteristicsCache()
	r70Strip(gs2, b, "r70_vig_strip")
	DeclareAttackers(gs2, 0)
	if !b.Tapped {
		t.Error("vigilance survived a layer-6 ability strip — attacker was not tapped")
	}
}

// §702.3 defender — a creature with defender can't attack (CR 702.3b).
func TestKeywordMigrationR70_DefenderGrantAndStrip(t *testing.T) {
	// GRANT: layer-granted defender must block the attack.
	gs := r70Game(t)
	a := addBattlefield(gs, 0, "Granted Wall", 2, 2, "creature")
	r70Grant(gs, a, "defender", "r70_def_grant")
	if canAttackGS(gs, a) {
		t.Error("layer-granted defender did not prevent attacking")
	}

	// STRIP: a stamped defender wiped by Humility must be able to attack.
	gs2 := r70Game(t)
	b := addBattlefield(gs2, 0, "Stamped Wall", 2, 2, "creature")
	b.Flags = map[string]int{"kw:defender": 1}
	gs2.InvalidateCharacteristicsCache()
	if canAttackGS(gs2, b) {
		t.Fatal("precondition: the stamped defender should not be able to attack")
	}
	r70Strip(gs2, b, "r70_def_strip")
	if !canAttackGS(gs2, b) {
		t.Error("defender survived a layer-6 ability strip — creature still cannot attack")
	}
}

// §702.7 first strike / §702.4 double strike — which damage step a
// creature deals in.
func TestKeywordMigrationR70_FirstStrikeStepGrantAndStrip(t *testing.T) {
	gs := r70Game(t)
	p := addBattlefield(gs, 0, "Striker", 2, 2, "creature")

	if dealsInStep(gs, p, true) {
		t.Fatal("precondition: a vanilla creature must not deal in the first-strike step")
	}
	r70Grant(gs, p, "first strike", "r70_fs_grant")
	if !dealsInStep(gs, p, true) {
		t.Error("layer-granted first strike did not reach the damage-step split")
	}

	gs2 := r70Game(t)
	q := addBattlefield(gs2, 0, "Stamped Striker", 2, 2, "creature")
	q.Flags = map[string]int{"kw:first strike": 1}
	gs2.InvalidateCharacteristicsCache()
	if !dealsInStep(gs2, q, true) {
		t.Fatal("precondition: the stamped first-striker should deal in the first-strike step")
	}
	r70Strip(gs2, q, "r70_fs_strip")
	if dealsInStep(gs2, q, true) {
		t.Error("first strike survived a layer-6 ability strip")
	}
	if !dealsInStep(gs2, q, false) {
		t.Error("after losing first strike the creature must deal in the REGULAR step")
	}
}

// §702.2 deathtouch — lethal damage calculation.
func TestKeywordMigrationR70_DeathtouchGrantAndStrip(t *testing.T) {
	gs := r70Game(t)
	atk := addBattlefield(gs, 0, "Toucher", 4, 4, "creature")
	blk := addBattlefield(gs, 1, "Big Blocker", 6, 6, "creature")

	if got := lethalAmountGS(gs, atk, blk); got != 6 {
		t.Fatalf("precondition: lethal for a 6/6 is 6, got %d", got)
	}
	r70Grant(gs, atk, "deathtouch", "r70_dt_grant")
	if got := lethalAmountGS(gs, atk, blk); got != 1 {
		t.Errorf("layer-granted deathtouch did not reach lethal calculation: got %d, want 1", got)
	}

	gs2 := r70Game(t)
	atk2 := addBattlefield(gs2, 0, "Stamped Toucher", 4, 4, "creature")
	blk2 := addBattlefield(gs2, 1, "Big Blocker", 6, 6, "creature")
	atk2.Flags = map[string]int{"kw:deathtouch": 1}
	gs2.InvalidateCharacteristicsCache()
	if got := lethalAmountGS(gs2, atk2, blk2); got != 1 {
		t.Fatalf("precondition: stamped deathtouch should make lethal 1, got %d", got)
	}
	r70Strip(gs2, atk2, "r70_dt_strip")
	if got := lethalAmountGS(gs2, atk2, blk2); got != 6 {
		t.Errorf("deathtouch survived a layer-6 ability strip: lethal %d, want 6", got)
	}
}

// §702.15 lifelink — combat damage to a player gains that much life.
func TestKeywordMigrationR70_LifelinkGrantAndStrip(t *testing.T) {
	gs := r70Game(t)
	src := addBattlefield(gs, 0, "Linker", 3, 3, "creature")
	before := gs.Seats[0].Life
	applyCombatDamageToPlayer(gs, src, 3, 1)
	if gs.Seats[0].Life != before {
		t.Fatalf("precondition: no lifelink, life should be unchanged (%d -> %d)", before, gs.Seats[0].Life)
	}

	gs2 := r70Game(t)
	src2 := addBattlefield(gs2, 0, "Granted Linker", 3, 3, "creature")
	r70Grant(gs2, src2, "lifelink", "r70_ll_grant")
	before2 := gs2.Seats[0].Life
	applyCombatDamageToPlayer(gs2, src2, 3, 1)
	if gs2.Seats[0].Life != before2+3 {
		t.Errorf("layer-granted lifelink did not gain life: %d -> %d, want +3",
			before2, gs2.Seats[0].Life)
	}

	gs3 := r70Game(t)
	src3 := addBattlefield(gs3, 0, "Stamped Linker", 3, 3, "creature")
	src3.Flags = map[string]int{"kw:lifelink": 1}
	gs3.InvalidateCharacteristicsCache()
	r70Strip(gs3, src3, "r70_ll_strip")
	before3 := gs3.Seats[0].Life
	applyCombatDamageToPlayer(gs3, src3, 3, 1)
	if gs3.Seats[0].Life != before3 {
		t.Errorf("lifelink survived a layer-6 ability strip: %d -> %d, want unchanged",
			before3, gs3.Seats[0].Life)
	}
}
