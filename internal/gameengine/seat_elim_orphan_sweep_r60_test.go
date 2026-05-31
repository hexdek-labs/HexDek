package gameengine

import (
	"testing"
)

// TestSeatElim_SidebandCleanup_CeasesIDs pins the four sideband-map
// cleanup paths (ZoneCastGrants, MadnessExile, PlotExile, MayhemDiscards)
// — they now MarkInstanceIDCeased before deleting the map entry,
// matching ParadigmExile's pre-existing pattern. Without the cease, any
// card whose ONLY zone reference was the sideband map leaks as
// "minted-but-absent" once the map entry is deleted.
//
// Loki r60 ZoneConservation disappearance cluster, 2026-05-30. The 35
// games across many turns (34-60) where tokens / real cards disappear
// after seat_eliminated trace back to this gap.
func TestSeatElim_SidebandCleanup_CeasesIDs(t *testing.T) {
	cases := []struct {
		name string
		stash func(gs *GameState, c *Card)
	}{
		{
			name: "ZoneCastGrants",
			stash: func(gs *GameState, c *Card) {
				if gs.ZoneCastGrants == nil {
					gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
				}
				gs.ZoneCastGrants[c] = &ZoneCastPermission{
					Zone:              ZoneExile,
					Keyword:           "test",
					RequireController: 1,
					SourceName:        "test_stash",
				}
			},
		},
		{
			name: "MadnessExile",
			stash: func(gs *GameState, c *Card) {
				if gs.MadnessExile == nil {
					gs.MadnessExile = map[*Card]*MadnessWindow{}
				}
				gs.MadnessExile[c] = &MadnessWindow{Seat: 1, Turn: 5}
			},
		},
		{
			name: "PlotExile",
			stash: func(gs *GameState, c *Card) {
				if gs.PlotExile == nil {
					gs.PlotExile = map[*Card]*PlotMeta{}
				}
				gs.PlotExile[c] = &PlotMeta{Seat: 1, Turn: 5}
			},
		},
		{
			name: "MayhemDiscards",
			stash: func(gs *GameState, c *Card) {
				if gs.MayhemDiscards == nil {
					gs.MayhemDiscards = map[*Card]int{}
				}
				gs.MayhemDiscards[c] = 5
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gs := newPhase4GameState(t)
			card := &Card{Name: "Test Card", Owner: 1, Colors: []string{"R"}}
			MintOGInstanceID(gs, card)
			if card.InstanceID == "" {
				t.Fatalf("expected MintOGInstanceID to stamp an ID")
			}
			id := card.InstanceID
			tc.stash(gs, card)

			HandleSeatElimination(gs, 1)

			if _, ceased := gs.CeasedInstanceIDs[id]; !ceased {
				t.Fatalf("%s: expected ID %q to be ceased after seat-elim", tc.name, id)
			}
		})
	}
}

// TestSeatElim_OrphanSweepBackstop pins the post-elim
// SweepOrphanedInstanceIDs call: a minted ID with no live *Card
// reference anywhere (the "card disappeared" shape) is ceased after
// elim. Without the backstop, mid-turn zone-leaks created BEFORE the
// elim survive the elim itself because explicit cease loops only walk
// the leaving seat's zones and rely on finding the *Card pointer.
func TestSeatElim_OrphanSweepBackstop(t *testing.T) {
	gs := newPhase4GameState(t)
	// Create a token with TK provenance, mint its ID, then DO NOT put
	// the *Card in any zone. Models the "token died via non-canonical
	// path before elim, its *Card was lost from every zone but the ID
	// was never ceased" shape.
	orphan := &Card{Name: "Orphan Token", Owner: 1, Types: []string{"creature", "token"}}
	MintTokenInstanceID(gs, orphan, "", "")
	if orphan.InstanceID == "" {
		t.Fatalf("expected MintTokenInstanceID to stamp an ID")
	}
	orphanID := orphan.InstanceID

	// Sanity: the seat-elim explicit cease loops cannot find this card
	// (it's in no zone), so without the orphan sweep its ID would stay
	// minted-but-not-ceased.
	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[orphanID]; !ceased {
		t.Fatalf("expected orphan ID %q to be ceased by post-elim SweepOrphanedInstanceIDs", orphanID)
	}
}

// TestSeatElim_OrphanSweep_DoesNotOverCeaseSurvivingZones pins the
// negative path for the orphan-sweep backstop. A non-leaving seat's
// own card (Owner != leaving seat) must NOT be ceased even if the
// minted IDs map contains its ID. The sweep walks the surviving seat's
// zones and finds the *Card present → no cease. Guards against the
// over-sweep regression where the post-elim hook would silently retire
// live cards on surviving seats.
func TestSeatElim_OrphanSweep_DoesNotOverCeaseSurvivingZones(t *testing.T) {
	gs := newPhase4GameState(t)
	seat0Card := &Card{Name: "Seat 0's Card", Owner: 0, Colors: []string{"W"}}
	MintOGInstanceID(gs, seat0Card)
	if seat0Card.InstanceID == "" {
		t.Fatalf("MintOGInstanceID returned empty")
	}
	id := seat0Card.InstanceID

	gs.Seats[0].Hand = append(gs.Seats[0].Hand, seat0Card)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
		t.Fatalf("seat 0's own card (Owner=0) over-ceased by seat-1 elim sweep")
	}
}
