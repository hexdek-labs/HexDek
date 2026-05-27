package tournament

// r60 — end-to-end verification that an impulse_play ZoneCastPermission
// registered via resolveResidualByText (or its structured sibling) is
// actually cleaned up at the cleanup-step phase boundary by a real
// TakeTurn cycle.
//
// PR #554 stamped Duration + GrantTurn + SourceTimestamp on the grant
// so the reaper has something to reason about. This test closes the
// loop: drive a full turn through TakeTurn, then assert the grant is
// gone — verifying ScanExpiredDurations → ExpireZoneCastGrants is
// wired up at §514.2 and that the GrantTurn arithmetic actually fires
// at the right boundary (gs.Turn still == GrantTurn when cleanup runs).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// buildGrantedGame wires a 2-seat game with seat 0 holding an exiled
// card and a "source" permanent that registered the grant. Returns the
// game state and the exiled *Card pointer for later assertions.
func buildGrantedGame(t *testing.T) (*gameengine.GameState, *gameengine.Card) {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Active = 0
	gs.Turn = 3
	gs.Phase = "beginning"
	gs.Step = "untap"

	// Stock libraries so TakeTurn has cards to draw / discard.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}

	// Source permanent on seat 0's battlefield — this is the "Outpost
	// Siege" / heist source. Timestamp must be nonzero for the
	// SourceTimestamp stamp to be meaningful.
	srcCard := &gameengine.Card{Name: "Outpost Siege", Owner: 0}
	srcPerm := &gameengine.Permanent{
		Card:       srcCard,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, srcPerm)

	// Card that the source has just exiled — this is what the grant
	// keys on.
	exileCard := &gameengine.Card{Name: "Exiled Spell", Owner: 0}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, exileCard)

	// Register the impulse_play grant via the same path resolveResidualByText
	// uses. We can't call resolveResidualByText directly from the tournament
	// package (engine-internal), so we register the equivalent permission
	// shape with the exact fields PR #554 stamps.
	gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	gs.ZoneCastGrants[exileCard] = &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "impulse_play",
		ManaCost:          -1,
		ExileOnResolve:    false,
		RequireController: 0,
		SourceName:        "Outpost Siege",
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
		SourceTimestamp:   srcPerm.Timestamp,
	}
	return gs, exileCard
}

// TestImpulsePlayGrant_CleanedUpByTakeTurn drives a full turn cycle
// and verifies the grant is reaped by the cleanup-step phase boundary.
// This is the real end-to-end check that ScanExpiredDurations →
// ExpireZoneCastGrants fires at §514.2 — not just that the helper
// works in isolation.
func TestImpulsePlayGrant_CleanedUpByTakeTurn(t *testing.T) {
	gs, exileCard := buildGrantedGame(t)
	grantTurn := gs.Turn

	if _, ok := gs.ZoneCastGrants[exileCard]; !ok {
		t.Fatalf("precondition: grant should be registered before TakeTurn")
	}

	TakeTurn(gs)

	if _, still := gs.ZoneCastGrants[exileCard]; still {
		t.Errorf("impulse_play grant should be reaped by the cleanup-step phase boundary; "+
			"survived TakeTurn (GrantTurn=%d, gs.Turn now=%d, grants=%d)",
			grantTurn, gs.Turn, len(gs.ZoneCastGrants))
	}

	// The cleanup-step phase boundary should leave the invariant clean.
	// Other invariants may fire on synthetic test state; we only care
	// about ZoneCastGrantExpiry here.
	for _, v := range gameengine.RunAllInvariants(gs) {
		if v.Name == "ZoneCastGrantExpiry" {
			t.Errorf("ZoneCastGrantExpiry should be clean after TakeTurn: %s", v.Message)
		}
	}
}

// TestImpulsePlayGrant_GoneByEndStepEvent confirms the grant is gone by
// the time the cleanup_step event is logged — defends against a
// regression where someone moves ExpireZoneCastGrants out of
// ScanExpiredDurations or into a step that hasn't run yet.
func TestImpulsePlayGrant_GoneByEndStepEvent(t *testing.T) {
	gs, exileCard := buildGrantedGame(t)

	// Use the hook variant so we can snapshot the grant map after each
	// phase/step boundary.
	type snap struct {
		phase, step string
		present     bool
	}
	var snaps []snap
	TakeTurnWithHook(gs, func(g *gameengine.GameState) {
		_, present := g.ZoneCastGrants[exileCard]
		snaps = append(snaps, snap{g.Phase, g.Step, present})
	})

	// Find the last snapshot where the grant was present and the first
	// where it was absent.
	lastPresentAt := -1
	firstAbsentAt := -1
	for i, s := range snaps {
		if s.present && lastPresentAt < i {
			lastPresentAt = i
		}
		if !s.present && firstAbsentAt < 0 {
			firstAbsentAt = i
		}
	}

	if firstAbsentAt < 0 {
		t.Fatalf("grant never disappeared across %d phase snapshots", len(snaps))
	}
	if lastPresentAt >= 0 && lastPresentAt >= firstAbsentAt {
		t.Errorf("grant reappeared after disappearing — non-monotonic cleanup")
	}

	// The boundary at which it disappeared must be the cleanup step
	// (§514.2) — that is the contract ExpireZoneCastGrants documents.
	gone := snaps[firstAbsentAt]
	if gone.step != "cleanup" {
		t.Errorf("grant should be reaped at the cleanup step (§514.2); first absent at phase=%q step=%q",
			gone.phase, gone.step)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
