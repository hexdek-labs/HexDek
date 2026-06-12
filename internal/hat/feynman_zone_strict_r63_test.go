package hat

import (
	"fmt"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 — zone_accounting false-positive flood fix.
//
// The InstanceID strict census is now the single production authority
// for zone conservation in CheckGame; the owner-count heuristic runs
// only when no InstanceIDs were minted. These tests pin the dispatch
// and the headline fix: a game where cards LEGALLY left (CR §800.4
// elimination, ceased IDs) must NOT warn, even though the count
// heuristic reads every such seat as cards-light.

// mintFeynmanGame builds the standard feynman fixture and mints a
// valid OG-provenance InstanceID for every library card (the census
// reads provenance from id[2:4], so "h<seat>OG-..." is the real shape).
func mintFeynmanGame(t *testing.T, nSeats int) *gameengine.GameState {
	t.Helper()
	gs := newFeynmanGame(t, nSeats)
	gs.MintedInstanceIDs = map[string]struct{}{}
	gs.CeasedInstanceIDs = map[string]struct{}{}
	gs.MintedInstanceIDNames = map[string]string{}
	for i, s := range gs.Seats {
		for j, c := range s.Library {
			id := fmt.Sprintf("h%dOG-%03d", i, j)
			c.InstanceID = id
			gs.MintedInstanceIDs[id] = struct{}{}
			gs.MintedInstanceIDNames[id] = c.Name
		}
	}
	return gs
}

func zoneViolations(result OracleResult) []OracleViolation {
	var out []OracleViolation
	for _, v := range result.Violations {
		if v.Rule == "zone_accounting" || v.Rule == "zone_conservation" {
			out = append(out, v)
		}
	}
	return out
}

// TestFeynman_StrictCensus_EliminationNoFalsePositive is the headline
// regression: cards that legally left the game (ceased IDs — the CR
// §800.4 elimination shape) make seats read 10+ cards light to the
// count heuristic, which warned on nearly every production game. The
// strict census knows the difference: ceased IDs leave the expectation,
// so the game is CLEAN.
func TestFeynman_StrictCensus_EliminationNoFalsePositive(t *testing.T) {
	gs := mintFeynmanGame(t, 4)
	gs.Turn = 15
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true

	// Seat 1 is eliminated per §800.4a: every owned card ceases and its
	// zones empty (HandleSeatElimination shape).
	gs.Seats[1].Lost = true
	gs.Seats[1].LeftGame = true
	for _, c := range gs.Seats[1].Library {
		gs.CeasedInstanceIDs[c.InstanceID] = struct{}{}
	}
	gs.Seats[1].Library = nil

	// Seat 0 legally lost 10 cards too (e.g. they were owned by the
	// leaver via an exchange, or exiled outside the game): removed from
	// the zone AND ceased. The count heuristic reads seat 0 at 90/100
	// (diff -10, well past its -3 tolerance) — the exact production
	// false-positive shape.
	for _, c := range gs.Seats[0].Library[90:] {
		gs.CeasedInstanceIDs[c.InstanceID] = struct{}{}
	}
	gs.Seats[0].Library = gs.Seats[0].Library[:90]

	result := CheckGame(gs)
	if vs := zoneViolations(result); len(vs) != 0 {
		t.Errorf("legally-departed cards must not warn under the strict census; got %v", vs)
	}
}

// TestFeynman_StrictCensus_RealDisappearanceFlagged pins the other
// direction: a card removed from its zone WITHOUT being ceased is a
// real conservation bug and must surface as a critical violation.
func TestFeynman_StrictCensus_RealDisappearanceFlagged(t *testing.T) {
	gs := mintFeynmanGame(t, 2)
	gs.Turn = 10
	gs.Seats[1].Lost = true

	// Vanish one card: out of the zone, still minted, not ceased.
	gs.Seats[0].Library = gs.Seats[0].Library[:99]

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "zone_conservation" && v.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Errorf("a minted-not-ceased card absent from every zone must flag zone_conservation critical; got %v", result.Violations)
	}
}

// TestFeynman_StrictCensus_FabricationFlagged — a card present with an
// ID that was never minted is a fabrication and must flag.
func TestFeynman_StrictCensus_FabricationFlagged(t *testing.T) {
	gs := mintFeynmanGame(t, 2)
	gs.Turn = 10
	gs.Seats[1].Lost = true

	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		&gameengine.Card{Name: "Forged Card", Owner: 0, InstanceID: "h0OG-forged"})

	result := CheckGame(gs)
	found := false
	for _, v := range result.Violations {
		if v.Rule == "zone_conservation" && v.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Errorf("an unminted InstanceID in a zone must flag zone_conservation critical; got %v", result.Violations)
	}
}

// TestFeynman_StrictCensus_HeuristicSuppressedWhenAuthoritative pins
// the dispatch: when InstanceIDs are minted, the count heuristic does
// NOT run — its -5..-20 false-warning shape must not appear even when
// counts look wrong, as long as identity says clean.
func TestFeynman_StrictCensus_HeuristicSuppressedWhenAuthoritative(t *testing.T) {
	gs := mintFeynmanGame(t, 2)
	gs.Turn = 10
	gs.Seats[1].Lost = true

	// Counts read -20 for seat 0, but every removed card is ceased —
	// identity-clean.
	for _, c := range gs.Seats[0].Library[80:] {
		gs.CeasedInstanceIDs[c.InstanceID] = struct{}{}
	}
	gs.Seats[0].Library = gs.Seats[0].Library[:80]

	result := CheckGame(gs)
	for _, v := range result.Violations {
		if v.Rule == "zone_accounting" {
			t.Errorf("count heuristic must not run when the census is authoritative; got %v", v)
		}
	}
}
