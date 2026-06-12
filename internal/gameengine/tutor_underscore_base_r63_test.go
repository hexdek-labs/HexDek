package gameengine

// Regression for the underscore-compound tutor-base whiff (r63 OUTCOME
// widening, 70-card corpus cluster).
//
// The AST parser emits BOTH "basic land_card" (space) and
// "basic_land_card" (underscores) for "search your library for a basic
// land card". cardMatchesTutorFilter's fallback handled the space form
// via its per-word type check but reduced the underscore form to a
// literal cardHasType(c, "basic_land") that no card satisfies — so
// Beneath the Sands, Armillary Sphere, Blighted Woodland and ~67 other
// fetch/ramp effects FAILED TO FIND a basic land in a library full of
// them. The fix normalizes underscores to spaces before the per-word
// check.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func tutorTestState(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	src := &Permanent{
		Card:       &Card{Name: "Tutor Source", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	for i := 0; i < 3; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{
			Name: "Filler", Owner: 0, Types: []string{"creature"},
		})
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{
		Name: "Wastes", Owner: 0, Types: []string{"basic", "land"}, TypeLine: "basic land — wastes",
	})
	return gs, src
}

func TestTutor_UnderscoreCompoundBase_FindsBasicLand(t *testing.T) {
	for _, base := range []string{"basic_land_card", "basic land_card", "basic_land", "basic land"} {
		gs, src := tutorTestState(t)
		libBefore := len(gs.Seats[0].Library)
		bfBefore := len(gs.Seats[0].Battlefield)

		ResolveEffect(gs, src, &gameast.Tutor{
			Query:        gameast.Filter{Base: base, Quantifier: "one"},
			Destination:  "battlefield_tapped",
			Count:        *gameast.NumInt(1),
			ShuffleAfter: true,
		})

		if got := len(gs.Seats[0].Library); got != libBefore-1 {
			t.Errorf("base %q: library %d → %d, want -1 (tutor failed to find the basic land)",
				base, libBefore, got)
		}
		if got := len(gs.Seats[0].Battlefield); got != bfBefore+1 {
			t.Errorf("base %q: battlefield %d → %d, want +1", base, bfBefore, got)
		}
	}
}

func TestTutor_UnderscoreBase_StillRejectsNonMatches(t *testing.T) {
	// The normalization must not over-match: an artifact tutor finds
	// nothing in a creatures+basic-land library.
	gs, src := tutorTestState(t)
	libBefore := len(gs.Seats[0].Library)
	ResolveEffect(gs, src, &gameast.Tutor{
		Query:       gameast.Filter{Base: "artifact_card", Quantifier: "one"},
		Destination: "hand",
		Count:       *gameast.NumInt(1),
	})
	if got := len(gs.Seats[0].Library); got != libBefore {
		t.Errorf("artifact tutor moved a card from a library with no artifacts (%d → %d)", libBefore, got)
	}
}
