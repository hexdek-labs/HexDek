package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 OUTCOME residual pins.
//
// TestETBCounters_ExactlyPrinted — the generic AST etb_with_counters
// static is THE counter path; the per_card counter-only OnETB
// duplicates (Ghave/Reyhan/Yorvo) were deleted after the OUTCOME sweep
// caught both paths running (Ghave entered with 10). Real registry
// active: any reintroduced duplicate doubles the count and fails here.
func TestETBCounters_ExactlyPrinted(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Ghave, Guru of Spores", 5},
		{"Reyhan, Last of the Abzan", 3},
		{"Yorvo, Lord of Garenbrig", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gs := gameengine.NewGameState(2, nil, nil)
			perm := &gameengine.Permanent{
				Card: &gameengine.Card{
					Name: c.name, Owner: 0, Types: []string{"creature"},
					AST: &gameast.CardAST{Abilities: []gameast.Ability{
						&gameast.Static{Modification: &gameast.Modification{
							ModKind: "etb_with_counters",
							Args:    []interface{}{c.want, "+1/+1"},
						}},
					}},
				},
				Controller: 0, Owner: 0,
				Flags: map[string]int{}, Counters: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
			gameengine.FirePermanentETBTriggers(gs, perm)
			if got := perm.Counters["+1/+1"]; got != c.want {
				t.Fatalf("%s entered with %d +1/+1 counters, want exactly %d", c.name, got, c.want)
			}
		})
	}
}

// TestDinaEssenceBrewer_NoPhantomDrain pins the misregistration fix:
// Essence Brewer has NO life-gain drain; gaining life must not touch
// opponents.
func TestDinaEssenceBrewer_NoPhantomDrain(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	perm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Dina, Essence Brewer", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0, Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	before := gs.Seats[1].Life
	gameengine.GainLife(gs, 0, 3, "test")
	gameengine.FireCardTrigger(gs, "life_gained", map[string]interface{}{"seat": 0, "amount": 3})
	if gs.Seats[1].Life != before {
		t.Fatalf("Essence Brewer must not drain on life gain (Soul Steeper copy-paste bug): opponent %d -> %d",
			before, gs.Seats[1].Life)
	}
}
