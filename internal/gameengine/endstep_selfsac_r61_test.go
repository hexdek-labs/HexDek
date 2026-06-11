package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// endstep_selfsac_r61_test.go — r61: isolate the Skelemental "end-of-turn sac
// clause never fires" report. Skelemental's clause ("At the beginning of the
// end step, sacrifice it") parses to Triggered{phase:end_step → Sacrifice
// that_thing}. This test drives that exact AST shape through the live phase
// pipeline to confirm the ENGINE executes it — isolating whether the bug is
// engine logic (this fails) or corpus coverage / a missing AST (this passes,
// pointing at Skelemental simply not being in the parsed corpus).

func endStepSelfSacAST(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "phase", Phase: "end_step"},
				Effect:  &gameast.Sacrifice{Query: gameast.Filter{Base: "that_thing"}, Actor: "controller"},
				Raw:     "at the beginning of the end step, sacrifice it",
			},
		},
		FullyParsed: true,
	}
}

func TestEndStepSelfSac_GenericPipelineSacrifices(t *testing.T) {
	gs := newFixtureGame(t)
	seat := gs.Seats[0]

	card := &Card{Name: "Skelemental (synthetic)", Owner: 0, Types: []string{"creature"}}
	card.AST = endStepSelfSacAST(card.Name)
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	seat.Battlefield = append(seat.Battlefield, perm)

	// Fire the end step exactly as the turn engine does.
	FirePhaseTriggers(gs, "ending", "end_step")
	DrainStack(gs)
	StateBasedActions(gs)

	// The creature must be gone from the battlefield and in the graveyard.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == card.Name {
			t.Fatalf("end-step self-sacrifice did NOT fire: creature still on battlefield (engine-logic bug)")
		}
	}
	inYard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.Name == card.Name {
			inYard = true
		}
	}
	if !inYard {
		t.Errorf("creature left battlefield but is not in the graveyard (sacrifice routing bug)")
	}
}
