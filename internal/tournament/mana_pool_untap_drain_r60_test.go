package tournament

// r60 — regression tests for the §106.4 mana-pool-drain bypass at the
// start-of-untap boundary. Pre-fix, internal/tournament/turn.go zeroed
// the active seat's pool via a direct `seat.ManaPool = 0 ; seat.Mana
// .Clear()` pair, which silently overrode the CR §106.4a exemption set
// registered by Upwelling / Omnath / Cabal Coffers and friends. The
// fix routes the drain through gameengine.DrainAllPools(gs, "ending",
// "cleanup") — same canonical API the end-step drain already uses.
//
// CR §106.4a (Upwelling oracle: "You don't lose unspent mana as steps
// and phases end") explicitly carves out the §106.4 step/phase-end
// drain. Cleanup is the last step of the ending phase, so the drain
// that would normally fire at cleanup-end is the one Upwelling protects
// against. The pre-fix untap-start zero defeated this — Upwelling's
// pool got wiped at the cleanup → untap boundary every turn.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// buildManaDrainGame seeds a 2-seat game positioned at the very top of
// a turn (active=seat 0, phase=ending/cleanup of the prior turn). The
// caller adds whatever exemptions and mana state it wants to test, then
// calls TakeTurn which will transition into untap and exercise the
// drain.
func buildManaDrainGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Active = 0
	gs.Turn = 3
	// Position the game state at the boundary the test cares about:
	// the previous turn's cleanup step has just finished, and TakeTurn
	// is about to start the next turn at untap. The pool state we set
	// up below represents mana that survived the cleanup-end drain
	// (either because of an exemption or because there's no exemption
	// and the prior drain didn't fire — both shapes exercise the same
	// untap-start code path).
	gs.Phase = "beginning"
	gs.Step = "untap"

	// Stock libraries so TakeTurn has cards to draw / discard without
	// triggering library-empty losses.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}
	return gs
}

// TestUntapDrain_VanillaPoolDrainsToZero pins the dominant case: a
// seat with no exemption registered has its mana pool zeroed at the
// start-of-untap drain. Same behavior as the pre-fix manual zero —
// regression guard that the DrainAllPools route doesn't accidentally
// over-preserve mana in the vanilla case.
func TestUntapDrain_VanillaPoolDrainsToZero(t *testing.T) {
	gs := buildManaDrainGame(t)
	seat := gs.Seats[0]

	// Seed the seat with mana from the prior turn.
	gameengine.AddMana(gs, seat, "R", 3, "test")
	gameengine.AddMana(gs, seat, "any", 2, "test")
	if seat.Mana.Total() != 5 {
		t.Fatalf("pre-condition: expected 5 mana before TakeTurn; got %d", seat.Mana.Total())
	}

	TakeTurn(gs)

	if seat.Mana.Total() != 0 {
		t.Errorf("vanilla pool should drain at untap start; got total=%d (R=%d any=%d)",
			seat.Mana.Total(), seat.Mana.R, seat.Mana.Any)
	}
	if seat.ManaPool != 0 {
		t.Errorf("legacy ManaPool should mirror 0; got %d", seat.ManaPool)
	}
}

// TestUntapDrain_UpwellingRetainsAll pins the headline §106.4a
// behavior: when Upwelling registers an "any" exemption for all seats,
// the start-of-untap drain MUST NOT wipe the pool. Pre-fix the manual
// zero ran unconditionally and Upwelling silently lost its pool every
// turn boundary.
func TestUntapDrain_UpwellingRetainsAll(t *testing.T) {
	gs := buildManaDrainGame(t)
	seat := gs.Seats[0]

	// Stand in Upwelling on seat 0's battlefield and register its
	// all-seats / all-colors exemption (seatScope = -1 means every
	// seat; "any" color matches the registry's broad-exempt slug).
	upwelling := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Upwelling", TypeLine: "Enchantment",
			Types: []string{"enchantment"}, Owner: 0,
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	seat.Battlefield = append(seat.Battlefield, upwelling)
	gameengine.RegisterManaPoolExemption(gs, upwelling, -1, []string{"any"})

	gameengine.AddMana(gs, seat, "W", 2, "test")
	gameengine.AddMana(gs, seat, "R", 2, "test")
	gameengine.AddMana(gs, seat, "any", 2, "test")
	before := seat.Mana.Total()
	if before != 6 {
		t.Fatalf("pre-condition: expected 6 mana before TakeTurn; got %d", before)
	}

	TakeTurn(gs)

	if seat.Mana.Total() < before {
		t.Errorf("Upwelling should preserve pool across untap; before=%d after=%d (W=%d R=%d any=%d)",
			before, seat.Mana.Total(),
			seat.Mana.W, seat.Mana.R, seat.Mana.Any)
	}
}

// TestUntapDrain_OmnathRetainsGreenOnly pins the partial-exemption
// case: Omnath, Locus of Mana exempts green only. Non-green mana must
// drain at untap; green must survive. Pre-fix the manual zero wiped
// green too, defeating Omnath's whole game plan ("ramp up green that
// floats across turns to fuel a big X-spell next turn").
func TestUntapDrain_OmnathRetainsGreenOnly(t *testing.T) {
	gs := buildManaDrainGame(t)
	seat := gs.Seats[0]

	omnath := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Omnath, Locus of Mana", TypeLine: "Legendary Creature",
			Types: []string{"creature", "legendary", "elemental"}, Owner: 0,
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	seat.Battlefield = append(seat.Battlefield, omnath)
	gameengine.RegisterManaPoolExemption(gs, omnath, omnath.Controller, []string{"G"})

	gameengine.AddMana(gs, seat, "G", 5, "test")
	gameengine.AddMana(gs, seat, "R", 3, "test")
	gameengine.AddMana(gs, seat, "any", 2, "test")
	if seat.Mana.G != 5 {
		t.Fatalf("pre-condition: expected G=5 before TakeTurn; got %d", seat.Mana.G)
	}

	TakeTurn(gs)

	if seat.Mana.G < 5 {
		t.Errorf("Omnath should preserve green across untap; got G=%d (want >= 5)", seat.Mana.G)
	}
	if seat.Mana.R != 0 {
		t.Errorf("Omnath should NOT preserve red; got R=%d (want 0)", seat.Mana.R)
	}
	if seat.Mana.Any != 0 {
		t.Errorf("Omnath should NOT preserve generic; got Any=%d (want 0)", seat.Mana.Any)
	}
}
