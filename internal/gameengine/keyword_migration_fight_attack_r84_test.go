package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r84 — batch 8: fight/bite damage riders (resolve.go) and the attack-
// trigger attacker filters (attack_trigger_dispatch.go).
//
// Both are rules enforcement. dealFightDamage decides whether damage is
// deathtouch-lethal and whether its controller gains life; matchesAttacker
// decides whether a triggered ability FIRES. Neither is AI policy.
//
// Two widenings past the keyword axis, both under the standing rule that a
// layer-blind read in the same function is migrated when it enforces a rule:
//
//   resolveFight     a.Power()      -> gs.PowerOf(a)      CR §701.12c
//   matchesAttacker  atk.Power()    -> gs.PowerOf(atk)
//   matchesAttacker  atk.IsCreature -> gs.IsCreatureOf    (TYPE axis — see
//                                                          the site comment)

func r84Fighter(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	return addBattlefield(gs, seat, name, pow, tough, "creature")
}

// §702.2 deathtouch on a fight — grant and strip.
func TestKeywordMigrationR84_FightDeathtouchGrantAndStrip(t *testing.T) {
	// GRANT: a layer-granted deathtouch must mark the target.
	gs := newFixtureGame(t)
	a := r84Fighter(gs, 0, "Granted Toucher", 1, 1)
	b := r84Fighter(gs, 1, "Victim", 4, 4)
	r70Grant(gs, a, "deathtouch", "r84_dt_grant")
	if a.HasKeyword("deathtouch") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	dealFightDamage(gs, a, b, 1)
	if b.Flags["deathtouch_damaged"] != 1 {
		t.Error("layer-granted deathtouch did not reach fight damage")
	}

	// STRIP: a stamped deathtouch wiped by Humility must not mark.
	gs2 := newFixtureGame(t)
	c := r84Fighter(gs2, 0, "Humbled Toucher", 1, 1)
	d := r84Fighter(gs2, 1, "Survivor", 4, 4)
	c.Flags["kw:deathtouch"] = 1
	gs2.InvalidateCharacteristicsCache()
	if !c.HasKeyword("deathtouch") {
		t.Fatal("precondition: the stamp should be raw-visible")
	}
	r70Strip(gs2, c, "r84_dt_strip")
	dealFightDamage(gs2, c, d, 1)
	if d.Flags["deathtouch_damaged"] == 1 {
		t.Error("deathtouch survived a layer-6 ability strip on a fight")
	}
}

// §702.15 lifelink on a fight — grant and strip.
func TestKeywordMigrationR84_FightLifelinkGrantAndStrip(t *testing.T) {
	gs := newFixtureGame(t)
	a := r84Fighter(gs, 0, "Granted Linker", 3, 3)
	b := r84Fighter(gs, 1, "Target", 5, 5)
	r70Grant(gs, a, "lifelink", "r84_ll_grant")
	before := gs.Seats[0].Life
	dealFightDamage(gs, a, b, 3)
	if gs.Seats[0].Life != before+3 {
		t.Errorf("layer-granted lifelink did not gain life on a fight: %d -> %d", before, gs.Seats[0].Life)
	}

	gs2 := newFixtureGame(t)
	c := r84Fighter(gs2, 0, "Humbled Linker", 3, 3)
	d := r84Fighter(gs2, 1, "Target", 5, 5)
	c.Flags["kw:lifelink"] = 1
	gs2.InvalidateCharacteristicsCache()
	r70Strip(gs2, c, "r84_ll_strip")
	before2 := gs2.Seats[0].Life
	dealFightDamage(gs2, c, d, 3)
	if gs2.Seats[0].Life != before2 {
		t.Errorf("lifelink survived a layer-6 ability strip on a fight: %d -> %d", before2, gs2.Seats[0].Life)
	}
}

// §701.12c — a fight deals damage equal to EFFECTIVE power. Driven through
// ResolveEffect/resolveFight, which is where the power is actually read;
// calling dealFightDamage with a number computed in the test would prove
// nothing about the migration (it passed pre-migration when written that
// way, which is how I caught it).
func TestKeywordMigrationR84_FightUsesEffectivePower(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Prey Upon", 0, 0, "sorcery")
	a := r84Fighter(gs, 0, "Pumped Fighter", 2, 2)
	b := r84Fighter(gs, 1, "Punching Bag", 1, 9)
	r84Anthem(gs, a, 3, "r84_fight_anthem")
	if a.Power() != 2 || gs.PowerOf(a) != 5 {
		t.Fatalf("precondition: raw power should be 2 and effective 5; got %d / %d",
			a.Power(), gs.PowerOf(a))
	}

	ResolveEffect(gs, src, &gameast.Fight{
		A: gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		B: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	})

	if b.MarkedDamage != 5 {
		t.Errorf("a fight should deal EFFECTIVE power 5; dealt %d", b.MarkedDamage)
	}
}

// "whenever a creature with flying attacks" — grant and strip.
func TestKeywordMigrationR84_AttackTriggerFlyingGrantAndStrip(t *testing.T) {
	spec := attackTriggerSpec{class: atkAnyCreature, flying: 1}

	gs := newFixtureGame(t)
	atk := r84Fighter(gs, 0, "Granted Flier", 2, 2)
	r70Grant(gs, atk, "flying", "r84_fly_grant")
	if atk.HasKeyword("flying") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	if !spec.matchesAttacker(gs, atk) {
		t.Error("a layer-granted flying attacker did not match a with-flying trigger")
	}

	gs2 := newFixtureGame(t)
	atk2 := r84Fighter(gs2, 0, "Humbled Flier", 2, 2)
	atk2.Flags["kw:flying"] = 1
	gs2.InvalidateCharacteristicsCache()
	if !spec.matchesAttacker(gs2, atk2) {
		t.Fatal("precondition: the stamped flier should match before the strip")
	}
	r70Strip(gs2, atk2, "r84_fly_strip")
	if spec.matchesAttacker(gs2, atk2) {
		t.Error("flying survived a layer-6 ability strip in the attack-trigger filter")
	}
}

// The negative form — "whenever a creature WITHOUT flying attacks" — has to
// move in the opposite direction, which is the half a one-sided test misses.
func TestKeywordMigrationR84_AttackTriggerWithoutFlyingInverts(t *testing.T) {
	spec := attackTriggerSpec{class: atkAnyCreature, flying: -1}

	gs := newFixtureGame(t)
	atk := r84Fighter(gs, 0, "Grounded", 2, 2)
	if !spec.matchesAttacker(gs, atk) {
		t.Fatal("precondition: a plain creature should match a without-flying trigger")
	}
	r70Grant(gs, atk, "flying", "r84_nofly_grant")
	if spec.matchesAttacker(gs, atk) {
		t.Error("a layer-granted flying attacker still matched a WITHOUT-flying trigger")
	}
}

// Power filters gate whether the trigger fires, so they must read effective
// power.
func TestKeywordMigrationR84_AttackTriggerPowerIsEffective(t *testing.T) {
	gs := newFixtureGame(t)
	atk := r84Fighter(gs, 0, "Anthem Attacker", 2, 2)
	r84Anthem(gs, atk, 2, "r84_power_anthem")

	atLeast4 := attackTriggerSpec{class: atkAnyCreature, powerAtLeast: 4}
	if !atLeast4.matchesAttacker(gs, atk) {
		t.Errorf("an anthem-pumped 4/4 did not match a power>=4 trigger (raw power %d, effective %d)",
			atk.Power(), gs.PowerOf(atk))
	}

	atMost2 := attackTriggerSpec{class: atkAnyCreature, powerAtMost: 2}
	if atMost2.matchesAttacker(gs, atk) {
		t.Error("an anthem-pumped 4/4 still matched a power<=2 trigger")
	}
}

// Type axis: an animated noncreature permanent attacking is a creature by
// layer 4 and must reach the filter at all.
func TestKeywordMigrationR84_AttackTriggerSeesAnimatedPermanent(t *testing.T) {
	gs := newFixtureGame(t)
	land := addBattlefield(gs, 0, "Animated Land", 0, 0, "land")
	if land.IsCreature() {
		t.Fatal("precondition: the printed type should not be creature")
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerType, HandlerID: "r84_animate", Duration: DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == land },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Types = append(c.Types, "creature")
			c.Power, c.Toughness = 3, 3
		},
	})
	gs.InvalidateCharacteristicsCache()
	if !gs.IsCreatureOf(land) {
		t.Fatal("precondition: the land should be a creature by layer")
	}

	spec := attackTriggerSpec{class: atkAnyCreature}
	if !spec.matchesAttacker(gs, land) {
		t.Error("an animated land attacking did not match an any-creature attack trigger")
	}
}

// r84Anthem registers a +N/+N continuous effect. LayerPT WITH a sublayer —
// applyLayer filters by sublayer, so an effect with none is silently never
// applied (the trap that made two correct r72 fixes look broken).
func r84Anthem(gs *GameState, target *Permanent, n int, id string) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", HandlerID: id, Duration: DurationPermanent,
		Predicate: func(_ *GameState, p *Permanent) bool { return p == target },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Power += n
			c.Toughness += n
		},
	})
	gs.InvalidateCharacteristicsCache()
}
