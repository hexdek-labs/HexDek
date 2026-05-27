package gameengine

import (
	"testing"
)

// TestApplyMutate_CountersInheritedWhenSlidingUnder pins the r60 fix to
// ApplyMutate's counter-asymmetry bug. Per CR §702.140, the merged
// creature is a single object — counters live on the permanent, so any
// counters on either component must transfer to the survivor.
//
// The onTop=true branch already copies counters from the dying target
// into the surviving mutating perm (keywords_batch6.go:283-291). The
// onTop=false branch did NOT have the symmetric copy, so when the
// mutating creature slid UNDER (target keeps characteristics, mutating
// goes to graveyard / off-battlefield), any +1/+1 / loyalty / charge /
// time counters on the mutating perm were silently dropped on the floor.
//
// This is observable when a mutating creature has accumulated counters
// since ETB (Brokkos with Hardened Scales +1/+1s, a Pollywog Symbiote
// with counters from earlier mutates, a creature being reanimated with
// counters baked in) and is then mutated under another perm — the
// merged creature should be the size of the sum, not just the target.
func TestApplyMutate_CountersInheritedWhenSlidingUnder(t *testing.T) {
	gs := newMutateGame(t)

	mutating := addNonHumanCreature(gs, 0, "Pollywog Symbiote", 2, 2)
	mutating.Counters = map[string]int{
		"+1/+1":  3,
		"charge": 2,
	}

	target := addNonHumanCreature(gs, 0, "Brokkos Apex of Forever", 6, 6)
	target.Counters = map[string]int{
		"+1/+1": 1,
	}

	// Slide mutating UNDER target (onTop=false): target keeps Card
	// identity, mutating's *Permanent leaves the battlefield.
	ApplyMutate(gs, mutating, target, false /*onTop*/)

	// Survivor must be target (mutating goes away when onTop=false).
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == target {
			stillOnBF = true
		}
		if p == mutating {
			t.Fatalf("mutating perm should be off battlefield when sliding under (onTop=false)")
		}
	}
	if !stillOnBF {
		t.Fatalf("target perm should still be on battlefield as the survivor")
	}

	// Survivor must carry the SUM of both components' counters.
	if got := target.Counters["+1/+1"]; got != 4 {
		t.Fatalf("expected +1/+1 counters to sum (3 from mutating + 1 from target = 4), got %d", got)
	}
	if got := target.Counters["charge"]; got != 2 {
		t.Fatalf("expected charge counters (2) to transfer from mutating to target, got %d", got)
	}
}

// TestApplyMutate_CountersInheritedWhenOnTop guards the pre-existing
// onTop=true counter copy so the symmetry is bit-stable — defends
// against a future cleanup that removes the wrong branch's copy.
func TestApplyMutate_CountersInheritedWhenOnTop(t *testing.T) {
	gs := newMutateGame(t)

	target := addNonHumanCreature(gs, 0, "Sea-Dasher Octopus", 1, 2)
	target.Counters = map[string]int{
		"+1/+1": 4,
	}

	mutating := addNonHumanCreature(gs, 0, "Vadrok Apex of Thunder", 3, 3)
	mutating.Counters = map[string]int{
		"+1/+1": 1,
	}

	// Put mutating ON TOP (onTop=true): mutating keeps Card identity,
	// target's *Permanent leaves the battlefield.
	ApplyMutate(gs, mutating, target, true /*onTop*/)

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == mutating {
			stillOnBF = true
		}
		if p == target {
			t.Fatalf("target perm should be off battlefield when mutating on top")
		}
	}
	if !stillOnBF {
		t.Fatalf("mutating perm should still be on battlefield as the survivor")
	}

	if got := mutating.Counters["+1/+1"]; got != 5 {
		t.Fatalf("expected +1/+1 counters to sum (1 mutating + 4 target = 5), got %d", got)
	}
}

// TestApplyMutate_EmptyCountersNoOp guards the defensive nil-map handling
// on the sliding-under branch — neither perm carrying counters must not
// panic or spuriously initialize an empty map on the survivor.
func TestApplyMutate_EmptyCountersNoOp(t *testing.T) {
	gs := newMutateGame(t)

	mutating := addNonHumanCreature(gs, 0, "Lore Drakkis", 2, 3)
	mutating.Counters = nil

	target := addNonHumanCreature(gs, 0, "Insatiable Hemophage", 1, 4)
	target.Counters = nil

	ApplyMutate(gs, mutating, target, false /*onTop*/)

	if target.Counters != nil && len(target.Counters) != 0 {
		t.Fatalf("expected no counters when neither component had any, got %v", target.Counters)
	}
}
