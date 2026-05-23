package gameengine

// r59 — pin the expiry contract on impulse_play / cast-from-exile grants
// produced by resolveResidualByText and the "impulse_play" structured
// modification kind in resolve_helpers.go.
//
// Bug history: both sites built a ZoneCastPermission with no Duration /
// GrantTurn, so the grant outlived its oracle-declared "this turn"
// expiry. ExpireZoneCastGrants couldn't reclaim it and the
// ZoneCastGrantExpiry invariant could not reason about lifetime either.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func r59MakeGameState(turn int) *GameState {
	gs := &GameState{
		Turn:           turn,
		Phase:          "main1",
		Step:           "main1",
		Seats:          []*Seat{{Idx: 0, Life: 40}, {Idx: 1, Life: 40}},
		Flags:          map[string]int{},
		ZoneCastGrants: map[*Card]*ZoneCastPermission{},
	}
	return gs
}

func r59NewSourcePerm(gs *GameState, seat int, name string) *Permanent {
	c := &Card{Name: name, Owner: seat}
	p := &Permanent{
		Card:       c,
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// TestImpulsePlayStructured_ExpiresAtEndOfTurn verifies the "impulse_play"
// modification kind sets Duration="until_end_of_turn" + GrantTurn so the
// grant is reclaimed by ExpireZoneCastGrants at cleanup of the grant turn.
func TestImpulsePlayStructured_ExpiresAtEndOfTurn(t *testing.T) {
	gs := r59MakeGameState(5)
	libCard := &Card{Name: "Library Top", Owner: 0}
	gs.Seats[0].Library = []*Card{libCard}
	src := r59NewSourcePerm(gs, 0, "Outpost Siege")

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "impulse_play"})

	if len(gs.ZoneCastGrants) != 1 {
		t.Fatalf("impulse_play should register one grant, got %d", len(gs.ZoneCastGrants))
	}
	grant := gs.ZoneCastGrants[libCard]
	if grant == nil {
		t.Fatalf("grant for exiled top should be keyed on the original card pointer")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 5 {
		t.Errorf("GrantTurn: want 5, got %d", grant.GrantTurn)
	}

	// End-of-turn cleanup must reclaim the grant.
	ExpireZoneCastGrants(gs)
	if len(gs.ZoneCastGrants) != 0 {
		t.Errorf("impulse_play grant should expire at cleanup of its grant turn; %d left", len(gs.ZoneCastGrants))
	}
}

// TestResolveResidualByText_ImpulsePlay_ExpiresAtEndOfTurn pins the
// reResPlayThisTurn / reResExilePlay branch of resolveResidualByText.
func TestResolveResidualByText_ImpulsePlay_ExpiresAtEndOfTurn(t *testing.T) {
	gs := r59MakeGameState(7)
	exileCard := &Card{Name: "Exiled Card", Owner: 0}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Daxos of Meletis")

	if !resolveResidualByText(gs, src, "you may play that card this turn") {
		t.Fatalf("resolveResidualByText should handle the impulse-play phrase")
	}

	grant := gs.ZoneCastGrants[exileCard]
	if grant == nil {
		t.Fatalf("grant missing from ZoneCastGrants")
	}
	if grant.Duration != "until_end_of_turn" {
		t.Errorf("Duration: want until_end_of_turn, got %q", grant.Duration)
	}
	if grant.GrantTurn != 7 {
		t.Errorf("GrantTurn: want 7, got %d", grant.GrantTurn)
	}

	ExpireZoneCastGrants(gs)
	if _, still := gs.ZoneCastGrants[exileCard]; still {
		t.Errorf("residual-text impulse_play grant should be reclaimed at cleanup")
	}
}

// TestResolveResidualByText_AsLongAsExiled_NoTurnClock verifies the
// "for as long as it remains exiled" phrasing keeps Duration empty, so
// the grant persists until the card actually leaves exile (handled by
// RemoveZoneCastGrant on cast resolution).
func TestResolveResidualByText_AsLongAsExiled_NoTurnClock(t *testing.T) {
	gs := r59MakeGameState(3)
	exileCard := &Card{Name: "Heist Card", Owner: 1}
	gs.Seats[0].Exile = []*Card{exileCard}
	src := r59NewSourcePerm(gs, 0, "Gonti, Lord of Luxury")

	if !resolveResidualByText(gs, src, "you may play it for as long as it remains exiled") {
		t.Fatalf("resolveResidualByText should handle the as-long-as-exiled phrase")
	}

	grant := gs.ZoneCastGrants[exileCard]
	if grant == nil {
		t.Fatalf("grant missing")
	}
	if grant.Duration != "" {
		t.Errorf("Duration: want empty (no turn clock), got %q", grant.Duration)
	}

	// End-of-turn cleanup must NOT expire it — the card is still in exile.
	gs.Turn = 9
	ExpireZoneCastGrants(gs)
	if _, ok := gs.ZoneCastGrants[exileCard]; !ok {
		t.Errorf("as-long-as-exiled grant should survive EOT cleanup")
	}

	// But once the card leaves exile via RemoveZoneCastGrant it is gone.
	RemoveZoneCastGrant(gs, exileCard)
	if _, ok := gs.ZoneCastGrants[exileCard]; ok {
		t.Errorf("as-long-as-exiled grant should be reclaimed when card leaves exile")
	}
}

// TestImpulsePlayStructured_ZoneCastGrantExpiry_InvariantClean verifies
// that after the structured impulse_play handler runs and cleanup
// fires, the ZoneCastGrantExpiry invariant reports no violation.
func TestImpulsePlayStructured_ZoneCastGrantExpiry_InvariantClean(t *testing.T) {
	gs := r59MakeGameState(2)
	libCard := &Card{Name: "Library Top", Owner: 0}
	gs.Seats[0].Library = []*Card{libCard}
	src := r59NewSourcePerm(gs, 0, "Birgi, God of Storytelling")

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "impulse_play"})
	ExpireZoneCastGrants(gs)

	if err := checkZoneCastGrantExpiry(gs); err != nil {
		t.Errorf("ZoneCastGrantExpiry should be clean after cleanup: %v", err)
	}
}
