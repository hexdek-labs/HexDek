package gameengine

import (
	"fmt"
	"testing"
)

// r149Stocked builds a game with a realistic card population spread across
// the zones the sweep walks, so the map actually has to grow.
func r149Stocked(t *testing.T) *GameState {
	t.Helper()
	gs := newFixtureGame(t)
	for si, s := range gs.Seats {
		for i := 0; i < 90; i++ {
			s.Library = append(s.Library, &Card{Name: "Filler", InstanceID: fmt.Sprintf("h%dL%04d", si, i)})
		}
		for i := 0; i < 7; i++ {
			s.Hand = append(s.Hand, &Card{Name: "InHand", InstanceID: fmt.Sprintf("h%dH%04d", si, i)})
		}
		for i := 0; i < 5; i++ {
			s.Graveyard = append(s.Graveyard, &Card{Name: "InYard", InstanceID: fmt.Sprintf("h%dG%04d", si, i)})
		}
	}
	return gs
}

// The capacity hint must cover every zone the sweep actually walks. It is
// allowed to over-count (a nil card, a blank InstanceID, a duplicate) —
// over-counting costs a slightly larger map, under-counting costs the
// rehash this change exists to avoid.
func TestR149_HintCoversWhatTheSweepCollects(t *testing.T) {
	gs := r149Stocked(t)
	present := collectPresentInstanceIDsForSweep(gs)
	hint := countCardsForSweep(gs)
	if hint < len(present) {
		t.Errorf("hint %d is BELOW the %d ids actually collected — the map will still rehash", hint, len(present))
	}
}

// Capacity is the only thing that changed; the contents must be identical
// to what an unsized map would have produced.
func TestR149_ContentsUnchanged(t *testing.T) {
	gs := r149Stocked(t)
	got := collectPresentInstanceIDsForSweep(gs)

	want := map[string]struct{}{}
	for _, s := range gs.Seats {
		if s == nil || s.LeftGame {
			continue
		}
		for _, z := range [][]*Card{s.Library, s.Hand, s.Graveyard, s.Exile, s.CommandZone, s.ForetellExile} {
			for _, c := range z {
				if c != nil && c.InstanceID != "" {
					want[c.InstanceID] = struct{}{}
				}
			}
		}
	}
	for id := range want {
		if _, ok := got[id]; !ok {
			t.Fatalf("id %q collected by the reference walk is missing from the sweep", id)
		}
	}
	if len(got) < len(want) {
		t.Fatalf("sweep collected %d ids, reference walk found %d", len(got), len(want))
	}
}

// The measurement this change exists for. Unsized, the same population cost
// 11 allocations; sized it costs 4. The bound carries margin so it pins the
// pre-sizing without being brittle about Go's growth schedule.
func TestR149_SweepDoesNotRehashItsWayUp(t *testing.T) {
	gs := r149Stocked(t)
	got := testing.AllocsPerRun(50, func() { _ = collectPresentInstanceIDsForSweep(gs) })
	if got > 6 {
		t.Errorf("sweep allocates %.0f times — the map is growing from empty again (sized should be ~4, unsized was ~11)", got)
	}
}

// A seat that has left the game contributes nothing, in the hint and in the
// sweep alike. If the two disagreed here the hint would over- or
// under-count exactly when a player is eliminated.
func TestR149_LeftGameSeatIsSkippedByBoth(t *testing.T) {
	gs := r149Stocked(t)
	before := countCardsForSweep(gs)
	beforeIDs := len(collectPresentInstanceIDsForSweep(gs))

	gs.Seats[0].LeftGame = true
	after := countCardsForSweep(gs)
	afterIDs := len(collectPresentInstanceIDsForSweep(gs))

	if after >= before {
		t.Errorf("hint did not drop when a seat left: %d -> %d", before, after)
	}
	if afterIDs >= beforeIDs {
		t.Errorf("sweep did not drop the departed seat's cards: %d -> %d", beforeIDs, afterIDs)
	}
	if after < afterIDs {
		t.Errorf("hint %d below collected %d after a seat left", after, afterIDs)
	}
}
