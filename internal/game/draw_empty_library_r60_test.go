package game

import (
	"context"
	"database/sql"
	"testing"

	hexdekdb "github.com/hexdek/hexdek/internal/db"
)

// R60 — closes half-finished-features-r48 #6: combat.go's empty libCount
// dead branch is replaced with a proper CR §119.5 implementation.
// DrawCards sets Player.AttemptedEmptyDraw when called with a non-zero
// count against an already-empty library; CheckGameEnd then eliminates
// flagged seats on the next sweep ("next time a player would receive
// priority").
//
// Tests pin five corners:
//   1. Drawing from an empty library sets the flag.
//   2. Drawing the LAST card (library was non-empty at entry) does NOT
//      set the flag — only the NEXT draw call does.
//   3. Drawing zero cards from an empty library is a no-op (no flag).
//   4. CheckGameEnd eliminates a flagged seat (down to 1 alive → finish).
//   5. CheckGameEnd keeps the seat alive when life>0/poison<10/no flag.

func setupTwoSeatGame(t *testing.T, ctx context.Context) (db *sql.DB, _ string, _ string) {
	t.Helper()
	d, err := hexdekdb.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	deckID := seedPartyForGameTest(t, ctx, d, "party-r60-empty-lib", "dev-r60-empty-lib")
	g, err := CreateGame(ctx, d, "party-r60-empty-lib", "cafefacef00dbabe")
	if err != nil {
		d.Close()
		t.Fatalf("create game: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := CreateGamePlayer(ctx, d, &Player{
			GameID:       g.ID,
			SeatPosition: i,
			DeviceID:     "dev-r60-empty-lib",
			DeckID:       deckID,
			Life:         40,
		}); err != nil {
			d.Close()
			t.Fatalf("create player %d: %v", i, err)
		}
	}
	// Turn state with seat 0 as active in the draw step, so DrawCards
	// accepts manual draws without the override flag.
	if err := CreateTurnState(ctx, d, &TurnState{
		GameID:     g.ID,
		ActiveSeat: 0,
		Phase:      PhaseDraw,
		TurnNumber: 1,
	}); err != nil {
		d.Close()
		t.Fatalf("turn state: %v", err)
	}
	return d, g.ID, deckID
}

// Helper: stamp `n` library cards into seat 0. Used to seed the
// non-empty-then-empty cases.
func seedLibraryForSeat(t *testing.T, ctx context.Context, d *sql.DB, gameID string, seat int, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := CreateGameCard(ctx, d, &Card{
			GameID:       gameID,
			InstanceID:   "lib-" + string(rune('a'+seat)) + "-" + string(rune('0'+i)),
			Name:         "Filler Card",
			OwnerSeat:    seat,
			Zone:         ZoneLibrary,
			ZonePosition: i,
		}); err != nil {
			t.Fatalf("seed library card %d: %v", i, err)
		}
	}
}

// -----------------------------------------------------------------------
// 1. Drawing from an empty library sets the flag.
// -----------------------------------------------------------------------

func TestDrawCards_EmptyLibrarySetsAttemptedFlag(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Library is empty at entry — no setup needed (no cards seeded).
	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("draw from empty: %v", err)
	}
	p, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	if !p.AttemptedEmptyDraw {
		t.Fatalf("seat 0 should have AttemptedEmptyDraw=true after drawing from empty library; got false")
	}
}

// -----------------------------------------------------------------------
// 2. Drawing the LAST card does NOT set the flag.
// -----------------------------------------------------------------------

func TestDrawCards_LastCardDoesNotSetFlag(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	seedLibraryForSeat(t, ctx, d, gameID, 0, 1)

	// Library has 1 card; draw it. After this, library is empty but
	// AttemptedEmptyDraw should still be false — the §119.5 trigger is
	// "draw FROM empty," not "draw THE LAST card."
	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("draw last card: %v", err)
	}
	p, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	if p.AttemptedEmptyDraw {
		t.Fatalf("drawing the last card should NOT set AttemptedEmptyDraw (CR §119.5 fires on NEXT draw); got true")
	}

	// Now the library IS empty — drawing again should set the flag.
	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("draw after last-card: %v", err)
	}
	p2, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get player after second draw: %v", err)
	}
	if !p2.AttemptedEmptyDraw {
		t.Fatalf("second draw (library now empty) should set AttemptedEmptyDraw; got false")
	}
}

// -----------------------------------------------------------------------
// 3. Drawing zero from empty library = no-op.
// -----------------------------------------------------------------------

func TestDrawCards_ZeroDrawFromEmptyDoesNotSetFlag(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// "Draw zero" is a real edge case — happens for cards like Stroke
	// of Genius at X=0 or scripts that conditionally compute n=0. No
	// instruction-to-draw means no §119.5 trigger.
	if _, err := DrawCards(ctx, d, gameID, 0, 0, false); err != nil {
		t.Fatalf("draw zero: %v", err)
	}
	p, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	if p.AttemptedEmptyDraw {
		t.Fatalf("draw-zero from empty library should NOT set the flag; got true")
	}
}

// -----------------------------------------------------------------------
// 4. CheckGameEnd eliminates a flagged seat.
// -----------------------------------------------------------------------

func TestCheckGameEnd_AttemptedEmptyDrawEliminatesSeat(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Manually flag seat 1; seat 0 untouched.
	p1, err := GetGamePlayer(ctx, d, gameID, 1)
	if err != nil {
		t.Fatalf("get seat 1: %v", err)
	}
	p1.AttemptedEmptyDraw = true
	if err := UpdateGamePlayer(ctx, d, p1); err != nil {
		t.Fatalf("flag seat 1: %v", err)
	}

	// CheckGameEnd should eliminate seat 1 → seat 0 alone → finish.
	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}

	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("seat 0 should have been declared winner (DeviceID=dev-r60-empty-lib); got %q",
			g.Winner)
	}
	if g.FinishedAt == 0 {
		t.Errorf("game should be marked finished; FinishedAt=0")
	}
}

// -----------------------------------------------------------------------
// 5. CheckGameEnd keeps seat alive when no loss condition met.
// -----------------------------------------------------------------------

func TestCheckGameEnd_NoLossConditionKeepsAlive(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Both seats start at life 40, poison 0, no empty-draw flag.
	// CheckGameEnd should be a no-op (no elimination, no finish).
	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.FinishedAt != 0 {
		t.Errorf("game should still be active; FinishedAt=%d", g.FinishedAt)
	}
}

// -----------------------------------------------------------------------
// 6. End-to-end: DrawCards → CheckGameEnd flow eliminates seat.
// -----------------------------------------------------------------------

func TestDrawCardsThenCheckGameEnd_EliminatesEmptyLibrarySeat(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	// Seat 0 has 1 card; seat 1 has none. Draw the last card from seat
	// 0 (no flag), then draw again (flag set). Subsequent CheckGameEnd
	// should eliminate seat 0 → seat 1 wins.
	seedLibraryForSeat(t, ctx, d, gameID, 0, 1)

	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("first draw (last card): %v", err)
	}
	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("second draw (from empty): %v", err)
	}
	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}

	// Seat 1 should be the winner.
	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.Winner != "dev-r60-empty-lib" {
		t.Errorf("seat 1 should win after seat 0 attempts empty-library draw; winner=%q",
			g.Winner)
	}
}

// -----------------------------------------------------------------------
// 7. Replacement-prevented draw: when n is reduced to 0 before DrawCards
//    is reached (e.g. "if you would draw a card, instead..."), the §119.5
//    flag must NOT fire — the player was never instructed-to-draw at all.
//    The MVP engine has no replacement layer yet, so this is modeled by
//    calling DrawCards with n=0 against an empty library, which is the
//    state any replacement effect would normalize the call to.
// -----------------------------------------------------------------------

func TestDrawCards_ReplacementPreventedDrawDoesNotSetFlag(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	if _, err := DrawCards(ctx, d, gameID, 0, 0, false); err != nil {
		t.Fatalf("replacement-prevented draw: %v", err)
	}
	p, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get player: %v", err)
	}
	if p.AttemptedEmptyDraw {
		t.Fatalf("a replacement-reduced n=0 draw should not trigger §119.5; got AttemptedEmptyDraw=true")
	}
	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.FinishedAt != 0 {
		t.Errorf("game should still be active after a replacement-prevented draw; FinishedAt=%d", g.FinishedAt)
	}
}

// -----------------------------------------------------------------------
// 8. Two empty-draws in the same SBA round → both seats lose
//    simultaneously. CR §704.5b is checked all-at-once; if every alive
//    seat is flagged, the game ends in a draw (no winner).
// -----------------------------------------------------------------------

func TestCheckGameEnd_SimultaneousEmptyDrawDrawsTheGame(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	if _, err := DrawCards(ctx, d, gameID, 0, 1, false); err != nil {
		t.Fatalf("seat 0 draw: %v", err)
	}
	turn, err := GetTurnState(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get turn: %v", err)
	}
	turn.ActiveSeat = 1
	if err := UpdateTurnState(ctx, d, turn); err != nil {
		t.Fatalf("update turn: %v", err)
	}
	if _, err := DrawCards(ctx, d, gameID, 1, 1, false); err != nil {
		t.Fatalf("seat 1 draw: %v", err)
	}

	if err := CheckGameEnd(ctx, d, gameID); err != nil {
		t.Fatalf("check game end: %v", err)
	}
	g, err := GetGame(ctx, d, gameID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if g.FinishedAt == 0 {
		t.Errorf("game should be finished after both seats hit §704.5b; FinishedAt=0")
	}
	if g.Winner != "" {
		t.Errorf("simultaneous empty-draw should end as a draw (Winner=\"\"); got %q", g.Winner)
	}
	p0, _ := GetGamePlayer(ctx, d, gameID, 0)
	p1, _ := GetGamePlayer(ctx, d, gameID, 1)
	if p0 == nil || !p0.AttemptedEmptyDraw {
		t.Errorf("seat 0 should remain flagged AttemptedEmptyDraw=true post-CheckGameEnd")
	}
	if p1 == nil || !p1.AttemptedEmptyDraw {
		t.Errorf("seat 1 should remain flagged AttemptedEmptyDraw=true post-CheckGameEnd")
	}
}

// -----------------------------------------------------------------------
// 9. Round-trip: flag persists through storage layer.
// -----------------------------------------------------------------------

func TestAttemptedEmptyDraw_PersistsThroughStorage(t *testing.T) {
	ctx := context.Background()
	d, gameID, _ := setupTwoSeatGame(t, ctx)
	defer d.Close()

	p, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if p.AttemptedEmptyDraw {
		t.Fatal("fresh player should have AttemptedEmptyDraw=false (default)")
	}

	p.AttemptedEmptyDraw = true
	if err := UpdateGamePlayer(ctx, d, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Round-trip through both single-row and list reads to confirm both
	// scanners pick up the new column.
	got, err := GetGamePlayer(ctx, d, gameID, 0)
	if err != nil {
		t.Fatalf("re-get: %v", err)
	}
	if !got.AttemptedEmptyDraw {
		t.Errorf("GetGamePlayer should read AttemptedEmptyDraw=true; got false")
	}

	listed, err := ListGamePlayers(ctx, d, gameID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, lp := range listed {
		if lp.SeatPosition == 0 {
			found = true
			if !lp.AttemptedEmptyDraw {
				t.Errorf("ListGamePlayers should read AttemptedEmptyDraw=true for seat 0; got false")
			}
		}
	}
	if !found {
		t.Fatal("seat 0 not in ListGamePlayers result")
	}
}
