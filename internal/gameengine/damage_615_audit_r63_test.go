package gameengine

import "testing"

// damage_615_audit_r63_test.go — CR §615 / §119 / §616 damage replacement
// + prevention interaction audit. Exercises the real damage pipelines
// (applyDamage / DealDamage / applyCombatDamageTo{Player,Creature}) with
// doublers, prevention shields, lifelink, deathtouch, infect/wither.

// doubleAllDamage registers a Furnace-of-Rath-shape DamageReplacement.
func doubleAllDamage(gs *GameState, src *Permanent) {
	gs.RegisterDamageReplacement(&DamageReplacement{
		SourcePerm: src,
		HandlerID:  "test_double",
		Fn: func(_ *GameState, ctx *DamageContext) {
			if ctx != nil && ctx.Amount > 0 {
				ctx.Amount *= 2
			}
		},
	})
}

// ---------------------------------------------------------------------------
// Doubler coverage across the three damage pipelines.
// ---------------------------------------------------------------------------

// Combat damage to a player must be doubled by a Furnace-shape replacement.
func TestDmg615_Doubler_CombatPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	furnace := addBattlefield(gs, 0, "Furnace of Rath", 0, 0, "enchantment")
	doubleAllDamage(gs, furnace)
	src := addBattlefield(gs, 0, "Bolt Drake", 3, 3, "creature")
	gs.Seats[1].Life = 40

	applyCombatDamageToPlayer(gs, src, 3, 1)
	if got := 40 - gs.Seats[1].Life; got != 6 {
		t.Errorf("Furnace must double combat damage to player: 3 -> 6; got %d", got)
	}
}

// Spell/ability damage to a player (applyDamage) must ALSO be doubled — a
// Furnace doubles burn spells, not just combat. (Pre-fix: applyDamage only
// consulted the ReplEvent registry, never DamageReplacements.)
func TestDmg615_Doubler_SpellPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	furnace := addBattlefield(gs, 0, "Furnace of Rath", 0, 0, "enchantment")
	doubleAllDamage(gs, furnace)
	bolt := addBattlefield(gs, 0, "Lightning Bolt Source", 0, 0, "creature")
	gs.Seats[1].Life = 40

	applyDamage(gs, bolt, Target{Kind: TargetKindSeat, Seat: 1}, 3)
	if got := 40 - gs.Seats[1].Life; got != 6 {
		t.Errorf("Furnace must double SPELL damage to player: 3 -> 6; got %d (doubler not wired into applyDamage)", got)
	}
}

// Spell/ability damage to a creature must be doubled too.
func TestDmg615_Doubler_SpellCreature(t *testing.T) {
	gs := newFixtureGame(t)
	furnace := addBattlefield(gs, 0, "Furnace of Rath", 0, 0, "enchantment")
	doubleAllDamage(gs, furnace)
	bolt := addBattlefield(gs, 0, "Bolt Source", 0, 0, "creature")
	target := addBattlefield(gs, 1, "Big Creature", 5, 5, "creature")

	applyDamage(gs, bolt, Target{Kind: TargetKindPermanent, Permanent: target}, 2)
	if target.MarkedDamage != 4 {
		t.Errorf("Furnace must double spell damage to a creature: 2 -> 4; got %d", target.MarkedDamage)
	}
}

// ---------------------------------------------------------------------------
// Prevention coverage across pipelines.
// ---------------------------------------------------------------------------

// Spell damage to a player respects a prevention shield. (applyDamage path.)
func TestDmg615_Prevention_SpellPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	bolt := addBattlefield(gs, 0, "Bolt Source", 0, 0, "creature")
	gs.Seats[1].Life = 40
	AddPreventionShield(gs, PreventionShield{TargetSeat: 1, Amount: 3, SourceCard: "Healing Salve", OneShot: true})

	applyDamage(gs, bolt, Target{Kind: TargetKindSeat, Seat: 1}, 5)
	if got := 40 - gs.Seats[1].Life; got != 2 {
		t.Errorf("prevention shield must reduce spell damage 5 -> 2; got %d", got)
	}
}

// Generic player damage (DealDamage — painlands, "deals N to you") must ALSO
// respect prevention shields. (Pre-fix: DealDamage skipped prevention.)
func TestDmg615_Prevention_DealDamage(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	AddPreventionShield(gs, PreventionShield{TargetSeat: 0, Amount: 3, SourceCard: "Healing Salve", OneShot: true})

	DealDamage(gs, 0, 5, "Painland")
	if got := 40 - gs.Seats[0].Life; got != 2 {
		t.Errorf("prevention shield must reduce DealDamage 5 -> 2; got %d (DealDamage skips prevention)", got)
	}
}

// ---------------------------------------------------------------------------
// §616 ordering: replacement (double) BEFORE prevention. A Furnace + a
// "prevent 3" shield on a 5-damage hit: the affected player applies
// prevention to the post-doubling amount in the engine's deterministic
// order. 5 -> double 10 -> prevent 3 -> 7. (The alternative ordering the
// player could choose is documented as an approximation.)
// ---------------------------------------------------------------------------

func TestDmg615_DoublerThenPrevention_CombatPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	furnace := addBattlefield(gs, 0, "Furnace of Rath", 0, 0, "enchantment")
	doubleAllDamage(gs, furnace)
	src := addBattlefield(gs, 0, "Attacker", 5, 5, "creature")
	gs.Seats[1].Life = 40
	AddPreventionShield(gs, PreventionShield{TargetSeat: 1, Amount: 3, SourceCard: "Shield", OneShot: true})

	applyCombatDamageToPlayer(gs, src, 5, 1)
	if got := 40 - gs.Seats[1].Life; got != 7 {
		t.Errorf("double-then-prevent: 5 -> 10 -> 7; got %d", got)
	}
}

// ---------------------------------------------------------------------------
// §119.3 lifelink — life is gained equal to damage DEALT (post-prevention).
// ---------------------------------------------------------------------------

func TestDmg615_Lifelink_FullyPrevented_NoLifeGain(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Lifelinker", 3, 3, "creature")
	src.Flags["kw:lifelink"] = 1
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 40
	AddPreventionShield(gs, PreventionShield{TargetSeat: 1, Amount: -1, SourceCard: "Teferi's Protection"})

	applyCombatDamageToPlayer(gs, src, 3, 1)
	if gs.Seats[1].Life != 40 {
		t.Errorf("fully-prevented damage must deal 0; defender life %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("lifelink must gain 0 when all damage is prevented; attacker life %d (want 20)", gs.Seats[0].Life)
	}
}

func TestDmg615_Lifelink_PartiallyPrevented_GainsDealtOnly(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Lifelinker", 5, 5, "creature")
	src.Flags["kw:lifelink"] = 1
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 40
	AddPreventionShield(gs, PreventionShield{TargetSeat: 1, Amount: 2, SourceCard: "Shield", OneShot: true})

	applyCombatDamageToPlayer(gs, src, 5, 1) // 2 prevented -> 3 dealt
	if got := 40 - gs.Seats[1].Life; got != 3 {
		t.Errorf("defender should take 3 (2 of 5 prevented); got %d", got)
	}
	if got := gs.Seats[0].Life - 20; got != 3 {
		t.Errorf("lifelink should gain 3 (= damage dealt, not 5); got %d", got)
	}
}

// ---------------------------------------------------------------------------
// §702.2 deathtouch — prevented damage does not destroy.
// ---------------------------------------------------------------------------

func TestDmg615_Deathtouch_FullyPrevented_NoDestroy(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Deathtoucher", 1, 1, "creature")
	src.Flags["kw:deathtouch"] = 1
	target := addBattlefield(gs, 1, "Big Wall", 5, 5, "creature")
	AddPreventionShield(gs, PreventionShield{TargetPerm: target, TargetSeat: -1, Amount: -1, SourceCard: "Mark of Asylum"})

	applyCombatDamageToCreature(gs, src, 1, target)
	if target.MarkedDamage != 0 {
		t.Errorf("prevented deathtouch damage must not mark damage; got %d", target.MarkedDamage)
	}
	if target.Flags["deathtouch_damaged"] != 0 {
		t.Errorf("prevented deathtouch must not flag the target for lethal SBA")
	}
	StateBasedActions(gs)
	if !onBattlefield(gs, target) {
		t.Errorf("creature must survive when the deathtouch damage was fully prevented")
	}
}

func TestDmg615_Deathtouch_PartiallyPrevented_StillLethal(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Deathtoucher", 3, 3, "creature")
	src.Flags["kw:deathtouch"] = 1
	target := addBattlefield(gs, 1, "Big Wall", 6, 6, "creature")
	AddPreventionShield(gs, PreventionShield{TargetPerm: target, TargetSeat: -1, Amount: 1, SourceCard: "Shield", OneShot: true})

	applyCombatDamageToCreature(gs, src, 3, target) // 1 prevented -> 2 deathtouch damage dealt
	if target.Flags["deathtouch_damaged"] != 1 {
		t.Errorf("any unprevented deathtouch damage is lethal; target should be flagged")
	}
	StateBasedActions(gs)
	if onBattlefield(gs, target) {
		t.Errorf("creature must die to partially-prevented deathtouch (2 of 3 dealt)")
	}
}

// ---------------------------------------------------------------------------
// §702.90 infect / §702.80 wither — combat damage to a creature as -1/-1
// counters, not marked damage.
// ---------------------------------------------------------------------------

func TestDmg615_Infect_DealtAsMinusCounters(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Infecter", 3, 3, "creature")
	src.Flags["kw:infect"] = 1
	target := addBattlefield(gs, 1, "Bear", 4, 4, "creature")

	applyCombatDamageToCreature(gs, src, 3, target)
	if target.MarkedDamage != 0 {
		t.Errorf("infect deals -1/-1 counters, NOT marked damage; got %d marked", target.MarkedDamage)
	}
	if target.Counters["-1/-1"] != 3 {
		t.Errorf("infect must place 3 -1/-1 counters; got %d", target.Counters["-1/-1"])
	}
}
