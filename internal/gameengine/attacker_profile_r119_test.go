package gameengine

import (
	"math/rand"
	"testing"
)

// r119 — DeclareBlockers now builds an attackerBlockProfile once per
// attacker and reuses it across that attacker's candidate blockers, instead
// of re-deriving flying / unblockable / landwalk for every blocker.
//
// The failure mode a hoist introduces is a profile LEAKING ACROSS
// ATTACKERS: build it for the first attacker and apply it to the second,
// and every attacker in the list inherits the first one's evasion. A
// per-pair test of canBlockGS cannot see that, because canBlockGS still
// builds its own profile. These drive the real entry point with TWO
// attackers whose evasion DIFFERS, which is the only shape that catches it.

func r119Game(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(119)), nil)
	gs.Active = 0
	gs.Phase = "combat"
	return gs
}

func r119Attack(gs *GameState, p *Permanent, defender int) {
	setPermFlag(p, flagAttacking, true)
	setAttackerDefender(p, defender)
}

// Direction 1: the flier is unblockable by a ground blocker.
// Direction 2: the ground attacker declared ALONGSIDE it still gets blocked.
// If the flier's profile leaked onto the ground attacker, direction 2 fails.
func TestAttackerProfileR119_FlyingDoesNotLeakToGroundAttacker(t *testing.T) {
	for _, order := range []string{"flier first", "ground first"} {
		gs := r119Game(t)
		flier := addCreature(gs, 0, "R119 Flier", 2, 2, "flying")
		ground := addCreature(gs, 0, "R119 Grunt", 2, 2)
		blocker := addCreature(gs, 1, "R119 Wall", 2, 4)
		r119Attack(gs, flier, 1)
		r119Attack(gs, ground, 1)

		attackers := []*Permanent{flier, ground}
		if order == "ground first" {
			attackers = []*Permanent{ground, flier}
		}
		out := DeclareBlockers(gs, attackers, 1)

		if len(out[flier]) != 0 {
			t.Errorf("[%s] a ground creature must not block a flier, got %d blockers",
				order, len(out[flier]))
		}
		if len(out[ground]) != 1 || out[ground][0] != blocker {
			t.Errorf("[%s] the ground attacker must still be blocked, got %d blockers",
				order, len(out[ground]))
		}
	}
}

// The same leak, on the member that actually costs: LandwalkType asks
// keywordActive five times and is the reason the profile exists.
func TestAttackerProfileR119_LandwalkDoesNotLeakToNonWalker(t *testing.T) {
	for _, order := range []string{"walker first", "walker second"} {
		gs := r119Game(t)
		// The defender controls a Swamp, so swampwalk is live.
		// DefenderControlsLandType matches on Card.Types or the card NAME,
		// so a basic named "Swamp" is the fixture the real path reads.
		swamp := &Card{Name: "Swamp", Owner: 1, Types: []string{"land", "swamp"}}
		gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield,
			&Permanent{Card: swamp, Controller: 1, Owner: 1,
				Counters: map[string]int{}, Flags: map[string]int{}})

		walker := addCreature(gs, 0, "R119 Bog Raider", 2, 2, "swampwalk")
		plain := addCreature(gs, 0, "R119 Footman", 2, 2)
		blocker := addCreature(gs, 1, "R119 Guard", 2, 4)
		r119Attack(gs, walker, 1)
		r119Attack(gs, plain, 1)

		attackers := []*Permanent{walker, plain}
		if order == "walker second" {
			attackers = []*Permanent{plain, walker}
		}
		out := DeclareBlockers(gs, attackers, 1)

		if len(out[walker]) != 0 {
			t.Errorf("[%s] swampwalk vs a Swamp controller must be unblockable, got %d",
				order, len(out[walker]))
		}
		if len(out[plain]) != 1 || out[plain][0] != blocker {
			t.Errorf("[%s] the non-walker must still be blocked, got %d",
				order, len(out[plain]))
		}
	}
}

// canBlockGS must stay behaviourally identical for its other ~20 callers,
// which still pass a bare (attacker, blocker) pair and never see a profile.
func TestAttackerProfileR119_CanBlockGSUnchangedForPairCallers(t *testing.T) {
	gs := r119Game(t)
	flier := addCreature(gs, 0, "R119 Pair Flier", 2, 2, "flying")
	ground := addCreature(gs, 0, "R119 Pair Grunt", 2, 2)
	wall := addCreature(gs, 1, "R119 Pair Wall", 2, 4)
	reacher := addCreature(gs, 1, "R119 Pair Reach", 2, 4, "reach")

	if canBlockGS(gs, flier, wall) {
		t.Error("ground blocker must not block a flier through canBlockGS")
	}
	if !canBlockGS(gs, flier, reacher) {
		t.Error("reach must block a flier through canBlockGS")
	}
	if !canBlockGS(gs, ground, wall) {
		t.Error("ground blocker must block a ground attacker through canBlockGS")
	}
	// And the profiled core agrees with the wrapper for the same inputs.
	prof := newAttackerBlockProfile(gs, flier)
	if canBlockProfiled(gs, prof, flier, wall) != canBlockGS(gs, flier, wall) {
		t.Error("canBlockProfiled must agree with canBlockGS for the same attacker")
	}
}
