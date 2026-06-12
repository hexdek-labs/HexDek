package gameengine

// r63 goldilocks A-M round 2 — engine-half certification for the
// 41-card "target attacking or blocking creature" dead-effect cluster
// (D'Avenant Archer, Heavy Ballista, Gideon's Reproach, Burning Oil…).
//
// The parser emits these as Filter{Base:"or", Extra:["attacking"]} (the
// conjunction strands in Base — same junk-base family as the any-target
// shapes fixed in 9d68fd07). These tests prove the ENGINE half is fully
// wired: matchesPermanent treats Base "or" as untyped and enforces the
// attacking/blocking Extra constraints against combat flags, and a
// Damage resolve lands on the attacker. The goldilocks deadness is
// therefore PURELY the scaffold's missing combat board (no permanent
// has flagAttacking set) — hex-dev-3's lane. When the combat scaffold
// lands, these cards clear with no engine work; if anyone regresses the
// junk-base or Extra handling, these pins catch it.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func combatFilterFixture(t *testing.T) (*GameState, *Permanent, *Permanent) {
	t.Helper()
	gs := newFixtureGame(t)
	attacker := addBattlefield(gs, 1, "Charging Bear", 2, 2, "creature")
	attacker.Flags[flagAttacking] = 1
	bystander := addBattlefield(gs, 1, "Idle Bear", 2, 2, "creature")
	return gs, attacker, bystander
}

func archerFilter() gameast.Filter {
	return gameast.Filter{
		Base:       "or",
		Quantifier: "one",
		Targeted:   true,
		Extra:      []string{"attacking"},
	}
}

// PickTarget must select ONLY the attacking creature through the
// junk "or" base + attacking Extra.
func TestCombatFilter_PickTargetFindsOnlyAttacker(t *testing.T) {
	gs, attacker, bystander := combatFilterFixture(t)
	src := addBattlefield(gs, 0, "D'Avenant Archer", 1, 2, "creature")

	ts := PickTarget(gs, src, archerFilter())
	if len(ts) != 1 {
		t.Fatalf("want exactly 1 target (the attacker), got %d", len(ts))
	}
	if ts[0].Permanent != attacker {
		t.Errorf("picked %v, want the attacking creature", ts[0].Permanent.Card.DisplayName())
	}
	_ = bystander

	// No combat → no legal target → empty pick (the goldilocks scaffold
	// state). This is the engine behaving CORRECTLY on a combat-less
	// board, which is why the cluster reads dead in the battery.
	attacker.Flags[flagAttacking] = 0
	if ts := PickTarget(gs, src, archerFilter()); len(ts) != 0 {
		t.Errorf("with no attacker, want 0 targets, got %d", len(ts))
	}
}

// End-to-end: resolving the archer-shaped Damage effect marks damage on
// the attacking creature.
func TestCombatFilter_DamageResolvesOntoAttacker(t *testing.T) {
	gs, attacker, bystander := combatFilterFixture(t)
	src := addBattlefield(gs, 0, "D'Avenant Archer", 1, 2, "creature")

	f := archerFilter()
	ResolveEffect(gs, src, &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
		Target: f,
	})

	if attacker.MarkedDamage != 1 {
		t.Errorf("attacker should carry 1 marked damage, got %d", attacker.MarkedDamage)
	}
	if bystander.MarkedDamage != 0 {
		t.Errorf("bystander should be untouched, got %d", bystander.MarkedDamage)
	}
}

// Blocking variant — the other half of "attacking or blocking".
func TestCombatFilter_BlockingExtraMatches(t *testing.T) {
	gs := newFixtureGame(t)
	blocker := addBattlefield(gs, 1, "Wall Bear", 0, 4, "creature")
	blocker.Flags[flagBlocking] = 1
	src := addBattlefield(gs, 0, "Gravel Slinger", 1, 3, "creature")

	f := gameast.Filter{Base: "or", Quantifier: "one", Targeted: true, Extra: []string{"blocking"}}
	ts := PickTarget(gs, src, f)
	if len(ts) != 1 || ts[0].Permanent != blocker {
		t.Fatalf("blocking Extra should find the blocker, got %d targets", len(ts))
	}
}
