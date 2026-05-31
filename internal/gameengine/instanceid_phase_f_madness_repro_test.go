package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase F live-Loki repro: game 411 surfaced h1OGVR200096 (Distemper of
// the Blood — a Madness instant owned by seat 1) as a fabrication that
// persisted for 5+ turns after seat 1 was eliminated. This test asks the
// engine the structural question: discard a Madness card, leave it in
// MadnessExile, eliminate the owner — does checkZoneConservation stay
// clean? If it fires, MadnessExile reconciliation in HandleSeatElimination
// is leaking. If it stays clean, game 411's leak is from a different
// source.
func TestPhaseF_MadnessExile_OwnerEliminated_NoFabrication(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1

	// Build a Madness instant owned by seat 1.
	distemper := &Card{
		Name:   "Distemper of the Blood",
		Owner:  1,
		Colors: []string{"R"},
		CMC:    2,
		Types:  []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "madness", Args: []interface{}{float64(0)}},
			},
		},
	}
	MintOGInstanceID(gs, distemper)
	if distemper.InstanceID == "" {
		t.Fatal("expected OG mint to stamp an InstanceID")
	}
	distID := distemper.InstanceID

	// Put it in seat 1's hand and discard via madness path.
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, distemper)
	if !OnDiscardMadness(gs, 1, distemper) {
		t.Fatal("OnDiscardMadness returned false; expected the madness keyword to be honored")
	}
	if _, ok := gs.MadnessExile[distemper]; !ok {
		t.Fatal("expected gs.MadnessExile entry after OnDiscardMadness")
	}
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("census pre-elimination must be clean; got %v", err)
	}

	// Eliminate seat 1.
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	if !gs.Seats[1].LeftGame {
		t.Fatal("expected LeftGame=true after HandleSeatElimination")
	}

	// After elimination: Distemper's OG ID is ceased (the cease happens
	// somewhere in HandleSeatElimination — either the private-zone walk
	// or the sideband reconciliation).
	if _, ceased := gs.CeasedInstanceIDs[distID]; !ceased {
		t.Fatalf("expected Distemper ID %q to be ceased after seat-1 elim", distID)
	}
	// The MadnessExile entry must be removed; otherwise the census's
	// sideband walk counts the *Card as present but its ID is in
	// CeasedInstanceIDs → fabrication.
	if _, stillPending := gs.MadnessExile[distemper]; stillPending {
		t.Fatalf("Phase F leak fingerprint: gs.MadnessExile still has Distemper after owner-elim; "+
			"this causes a persistent fabrication. " +
			"Fix HandleSeatElimination's sideband-purge to walk MadnessExile.")
	}
	// And the invariant must be clean.
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("census post-elimination must be clean; got %v", err)
	}
}

// TestPhaseF_CleanupHandSize_LeftGame_NoDiscard is the actual leak shape
// surfaced by Loki r60 seed-42 game 411 (Distemper of the Blood, 46 of
// 52 fabrications). After seat-1 elimination, the turn-cleanup step
// still called CleanupHandSize(gs, seat=1, 7), which discarded the
// (post-elim) hand contents. For Madness-keyword cards in that hand the
// discard fired OnDiscardMadness → RegisterZoneCastGrant on a *Card
// whose InstanceID was already ceased by HandleSeatElimination → census
// fabrication. Fix: CleanupHandSize early-returns on LeftGame seats per
// CR §800.4a.
func TestPhaseF_CleanupHandSize_LeftGame_NoDiscard(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["instanceid_strict_census"] = 1

	// Madness card in seat 0's hand, after seat 0 has LeftGame.
	distemper := &Card{
		Name:   "Distemper of the Blood",
		Owner:  0,
		Colors: []string{"R"},
		CMC:    2,
		Types:  []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "madness", Args: []interface{}{float64(0)}},
			},
		},
	}
	MintOGInstanceID(gs, distemper)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, distemper, distemper, distemper, distemper, distemper, distemper, distemper, distemper)
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)
	if !gs.Seats[0].LeftGame {
		t.Fatal("expected LeftGame=true after HandleSeatElimination")
	}

	// CleanupHandSize on a LeftGame seat must be a no-op — even with
	// 8 cards over hand size.
	preGrants := len(gs.ZoneCastGrants)
	preMadness := len(gs.MadnessExile)
	CleanupHandSize(gs, 0, 7)
	if len(gs.ZoneCastGrants) != preGrants {
		t.Fatalf("CleanupHandSize on LeftGame seat must not register grants; "+
			"before=%d after=%d", preGrants, len(gs.ZoneCastGrants))
	}
	if len(gs.MadnessExile) != preMadness {
		t.Fatalf("CleanupHandSize on LeftGame seat must not fire MadnessExile; "+
			"before=%d after=%d", preMadness, len(gs.MadnessExile))
	}
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("census post-cleanup-on-leftgame must stay clean; got %v", err)
	}
}

// TestPhaseF_OnDiscardMadness_LeftGame_Rejected is the defense-in-depth
// pin: even if some other path (a hypothetical force-discard effect)
// reaches OnDiscardMadness on a LeftGame seat, the function must reject
// so no Madness sideband state is stamped for an already-ceased ID.
func TestPhaseF_OnDiscardMadness_LeftGame_Rejected(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	distemper := &Card{
		Name:   "Distemper of the Blood",
		Owner:  0,
		Colors: []string{"R"},
		CMC:    2,
		Types:  []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "madness", Args: []interface{}{float64(0)}},
			},
		},
	}
	MintOGInstanceID(gs, distemper)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, distemper)
	gs.Seats[0].LeftGame = true

	if OnDiscardMadness(gs, 0, distemper) {
		t.Fatal("OnDiscardMadness on a LeftGame seat must return false")
	}
	if _, ok := gs.MadnessExile[distemper]; ok {
		t.Fatal("MadnessExile must not be populated for a LeftGame seat")
	}
	if _, ok := gs.ZoneCastGrants[distemper]; ok {
		t.Fatal("ZoneCastGrants must not be populated for a LeftGame seat")
	}
}
