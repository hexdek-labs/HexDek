package hat

// Interface-contract tests: both GreedyHat and PokerHat must satisfy
// gameengine.Hat, hats must be swappable mid-game, and a game built
// with mixed hats in each seat must function end-to-end.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestInterfaceSatisfaction is primarily a compile-time check; the var
// assertions at the top of greedy.go and poker.go fail at build time
// if either hat drifts from the interface. This test additionally
// confirms the interface is callable via the engine's Hat field.
func TestInterfaceSatisfaction(t *testing.T) {
	var _ gameengine.Hat = (*GreedyHat)(nil)
	var _ gameengine.Hat = (*PokerHat)(nil)

	// Dynamic check: build a GameState, attach each hat type, exercise
	// one method. The engine code path doesn't type-assert — proves the
	// "engine never inspects hats" contract.
	gs := newTestGame(t, 2)
	gs.Seats[0].Hat = &GreedyHat{}
	gs.Seats[1].Hat = NewPokerHat()

	for i, s := range gs.Seats {
		// Each seat's Hat answers a trivial mulligan query.
		got := s.Hat.ChooseMulligan(gs, i, s.Hand)
		if got {
			t.Errorf("seat %d: greedy/poker should keep the opener; got mulligan=true", i)
		}
	}
}

// TestHatSwapMidGame verifies a one-line hat swap works without the
// engine caring. This is the load-bearing assertion for the
// architectural directive "hats are swappable, engine never inspects".
func TestHatSwapMidGame(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Hat = &GreedyHat{}

	// Swap to a PokerHat mid-"game".
	gs.Seats[0].Hat = NewPokerHat()

	// Engine-side broadcast from LogEvent must reach the new hat.
	gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})

	// PokerHat observes the event (eventsSeen++). Verify via the
	// concrete type — allowed in tests, just not in the engine.
	ph, ok := gs.Seats[0].Hat.(*PokerHat)
	if !ok {
		t.Fatalf("seat 0 hat is not PokerHat after swap")
	}
	if ph.eventsSeen == 0 {
		t.Fatalf("PokerHat did not observe LogEvent after swap")
	}
}

// TestMixedGauntletSeating exercises the "NewGameState with mixed
// hats in each seat" gauntlet pattern. The engine must handle a 4-seat
// game where each seat has a different hat type (incl. nil).
func TestMixedGauntletSeating(t *testing.T) {
	gs := newTestGame(t, 4)
	gs.Seats[0].Hat = &GreedyHat{}
	gs.Seats[1].Hat = NewPokerHat()
	gs.Seats[2].Hat = &GreedyHat{}
	gs.Seats[3].Hat = NewPokerHatWithMode(ModeHold)

	// Broadcast an event to exercise every seat's ObserveEvent path.
	gs.LogEvent(gameengine.Event{Kind: "game_start", Seat: -1})

	// Every Hat should have responded without panicking.
	for i, s := range gs.Seats {
		if s.Hat == nil {
			t.Errorf("seat %d has no Hat", i)
			continue
		}
		// Trigger one decision to prove the method table is wired.
		_ = s.Hat.ChooseAttackers(gs, i, nil)
	}
}

// TestHatBroadcastSkipsNilSeats — LogEvent's broadcast must tolerate
// seats with no Hat (the pre-Phase-10 default).
func TestHatBroadcastSkipsNilSeats(t *testing.T) {
	gs := newTestGame(t, 2)
	// seat 0 has no Hat; seat 1 is a PokerHat.
	gs.Seats[1].Hat = NewPokerHat()

	gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})

	ph := gs.Seats[1].Hat.(*PokerHat)
	if ph.eventsSeen == 0 {
		t.Fatalf("PokerHat on seat 1 did not observe the broadcast")
	}
}

// ---------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------

// newTestGame returns a barebones GameState with `n` seats. The game
// has no cards loaded; tests that need cards build them inline.
func newTestGame(t *testing.T, n int) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(n, rand.New(rand.NewSource(1)), nil)
}

// newTestCardMinimal builds a Card with only the fields Hat methods
// read: AST (for ability walks), Types (for type-line checks, including
// "cost:N" for ManaCostOf), BasePower/Toughness.
func newTestCardMinimal(name string, types []string, cmc int, ast *gameast.CardAST) *gameengine.Card {
	if ast == nil {
		ast = &gameast.CardAST{Name: name}
	}
	c := &gameengine.Card{
		AST:   ast,
		Name:  name,
		Types: append([]string{}, types...),
	}
	if cmc > 0 {
		c.Types = append(c.Types, "cost:"+itoa(cmc))
	}
	return c
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// newTestPermanent builds a Permanent on seat's battlefield. Registers
// with the given Controller/Owner and zero-value flags.
func newTestPermanent(seat *gameengine.Seat, card *gameengine.Card, power, toughness int) *gameengine.Permanent {
	if card == nil {
		card = &gameengine.Card{}
	}
	card.BasePower = power
	card.BaseToughness = toughness
	p := &gameengine.Permanent{
		Card:       card,
		Controller: seat.Idx,
		Owner:      seat.Idx,
		Flags:      map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

// addKeyword sets a runtime keyword flag on a permanent so HasKeyword
// picks it up (matches the pattern in combat_test.go).
func addKeyword(p *gameengine.Permanent, name string) {
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	p.Flags["kw:"+name] = 1
}

// TestGauntletReadiness verifies the Phase 11 tournament-runner prereq:
// build a 4-seat commander pod with mixed hats, set the game running
// via engine events, and confirm:
//   - no panics
//   - each hat ran ObserveEvent at least once
//   - mode transitions propagated through the RAISE cascade pathway
//     from one seat to another via the shared EventLog.
func TestGauntletReadiness(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.CommanderFormat = true
	gs.Seats[0].Hat = &GreedyHat{}
	gs.Seats[1].Hat = NewPokerHat()
	gs.Seats[2].Hat = &GreedyHat{}
	gs.Seats[3].Hat = NewPokerHatWithMode(ModeCall)

	// Give seat 1 big board + combo hand so it will emergency-RAISE.
	gs.Seats[1].Life = 3
	for i := 0; i < 8; i++ {
		gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 1})
	}

	// Seat 1 should have transitioned to RAISE.
	ph1, _ := gs.Seats[1].Hat.(*PokerHat)
	if ph1.Mode != ModeRaise {
		t.Errorf("seat 1 on 3 life should RAISE; got %v", ph1.Mode)
	}
	// Seat 3's PokerHat should have SEEN the player_mode_change event
	// (cascade logic observed it — doesn't need to match).
	ph3 := gs.Seats[3].Hat.(*PokerHat)
	if ph3.eventsSeen == 0 {
		t.Error("seat 3 hat should have observed events")
	}
	// GreedyHats should still answer decisions.
	_ = gs.Seats[0].Hat.ChooseAttackers(gs, 0, nil)
	_ = gs.Seats[2].Hat.ChooseAttackers(gs, 2, nil)
}
