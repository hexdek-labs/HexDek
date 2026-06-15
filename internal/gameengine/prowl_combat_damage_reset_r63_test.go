package gameengine

import (
	"math/rand"
	"testing"
)

// prowl_combat_damage_reset_r63_test.go — mechanic-probe regressions for the
// per-turn reset of the shared "dealt combat damage to a player this turn"
// trackers (CR §702.74 Prowl / §702.169 Freerunning).
//
// Both facts are populated in applyCombatDamageToPlayer (combat.go) ONLY for
// the seat whose creatures attacked — always the active player. But the
// turn-start reset (UntapAll → Turn.Reset) clears only the ACTIVE seat, so a
// seat that attacked on its own turn kept a stale "dealt damage this turn"
// record through every intervening opponent's turn. Both are game-turn-scoped
// ("THIS turn"), so they must clear for ALL seats at every turn start.

func newProwlMPGame() *GameState {
	gs := NewGameState(4, rand.New(rand.NewSource(73)), nil)
	gs.Active = 0
	gs.Step = "precombat_main"
	return gs
}

// (c) Prowl: a Rogue that hit a player on seat 0's turn must NOT keep prowl
// active for seat 0 once a later (opponent's) turn has begun.
func TestProwl_CombatDamageBy_ClearedAtNextTurnStart(t *testing.T) {
	gs := newProwlMPGame()
	rogue := &Card{Name: "Nightshade Stinger", Owner: 0, Types: []string{"creature", "rogue"}, BasePower: 1, BaseToughness: 1}
	attacker := &Permanent{Card: rogue, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, attacker)
	applyCombatDamageToPlayer(gs, attacker, 1, 1)

	spell := &Card{Name: "Stinkdrinker Bandit", Owner: 0, Types: []string{"creature", "rogue"}}
	if !CanPayProwl(gs, 0, spell) {
		t.Fatal("setup: prowl should be active on seat 0's own turn after a Rogue hit a player")
	}

	// A new game turn begins — seat 1's untap step.
	gs.Active = 1
	UntapAll(gs, 1)

	if got := len(gs.Seats[0].Turn.CombatDamageBy); got != 0 {
		t.Errorf("seat 0 CombatDamageBy must clear at the next turn start; got %d entries", got)
	}
	if CanPayProwl(gs, 0, spell) {
		t.Error("prowl must NOT remain active for seat 0 on a later turn (not 'this turn' anymore)")
	}
}

// (c) Freerunning: the seat-level combat-damage flag (same combat.go block) is
// never reset today; it must clear at the next turn start too.
func TestFreerunning_CombatDamageFlag_ClearedAtNextTurnStart(t *testing.T) {
	gs := newProwlMPGame()
	beast := &Card{Name: "Just Some Beast", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	attacker := &Permanent{Card: beast, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, attacker)
	applyCombatDamageToPlayer(gs, attacker, 2, 1)

	if !CanCastForFreerunning(gs, 0) {
		t.Fatal("setup: freerunning should be active on seat 0's own turn after combat damage")
	}

	gs.Active = 1
	UntapAll(gs, 1)

	if CanCastForFreerunning(gs, 0) {
		t.Error("freerunning must NOT remain active for seat 0 on a later turn (flag was never reset)")
	}
}
