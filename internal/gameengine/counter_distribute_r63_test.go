package gameengine

// Regression for the silent distribute-counters no-op (r63 OUTCOME
// phase-5 audit, 31-card cluster).
//
// resolveCounterMod's op switch handled put/remove/double/move —
// op "distribute" ("distribute two +1/+1 counters among one or two
// target creatures") fell through entirely, so Ajani, Mentor of
// Heroes' +1, Armament Corps, Dueling Coach and ~28 more distribute
// effects resolved to NOTHING in live games. The fix maps distribute
// to a concentrated "put" on the first picked target (CR §601.2d —
// the split is a player choice; the total is the rules-visible
// aggregate), routing the §614 replacement chain identically.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestCounterMod_DistributePutsTotalCounters(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	src := &Permanent{
		Card:       &Card{Name: "Distributor", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	ally := &Permanent{
		Card:       &Card{Name: "Ally Bear", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src, ally)

	ResolveEffect(gs, src, &gameast.CounterMod{
		Op:          "distribute",
		Count:       *gameast.NumInt(2),
		CounterKind: "+1/+1",
		Target:      gameast.Filter{Base: "one or two target creatures you control", Quantifier: "any", Targeted: true},
	})

	total := 0
	for _, p := range gs.Seats[0].Battlefield {
		total += p.Counters["+1/+1"]
	}
	if total != 2 {
		t.Fatalf("distribute two +1/+1: total counters on battlefield = %d, want 2 (pre-fix: 0, silent no-op)", total)
	}
}
