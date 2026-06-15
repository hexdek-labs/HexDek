package gameengine

import "testing"

// saddle_behavior_r63_test.go — mechanic-probe regressions for SADDLE/MOUNT
// (CR §702.171) covering the properties the existing suite left unpinned:
//   (e) summoning-sick creatures CAN tap to pay the saddle cost (tapping for a
//       cost is not a {T} ability or an attack, so §302.6 sickness doesn't
//       apply — same as crew).
//   (a) the saddle cost is total POWER, not mana value — a high-CMC, low-power
//       creature can't meet it; power is what's summed.

// (e) A summoning-sick creature can be tapped to saddle.
func TestSaddleMount_SummoningSickCreatureCanTap(t *testing.T) {
	gs := newSaddleGame(t)
	mount := mountPerm(gs, 0, "Intrepid Stablemaster", 2)
	sick := creaturePerm(gs, 0, "Just-Cast Beast", 3)
	sick.SummoningSick = true // entered this turn

	if !SaddleMount(gs, 0, mount, []*Permanent{sick}) {
		t.Fatal("a summoning-sick creature must be allowed to tap for saddle (CR §702.171a)")
	}
	if !sick.Tapped {
		t.Fatal("the summoning-sick saddler should be tapped")
	}
	if !PermIsSaddled(mount) {
		t.Fatal("mount should be saddled")
	}
}

// (a) The saddle threshold is total POWER. A 1-power creature with a huge CMC
// cannot satisfy Saddle 3 alone; power — not mana value — is summed.
func TestSaddleMount_UsesPowerNotManaValue(t *testing.T) {
	gs := newSaddleGame(t)
	mount := mountPerm(gs, 0, "Roxanne", 3)
	// 1-power creature with a large mana value.
	weak := creaturePerm(gs, 0, "Expensive Wimp", 1)
	weak.Card.CMC = 9

	if SaddleMount(gs, 0, mount, []*Permanent{weak}) {
		t.Fatal("power 1 must NOT satisfy Saddle 3 even with CMC 9 (power, not mana)")
	}
	if PermIsSaddled(mount) {
		t.Fatal("mount must stay unsaddled when power is insufficient")
	}
	if weak.Tapped {
		t.Fatal("failed saddle must not tap any creature (atomic)")
	}
}
