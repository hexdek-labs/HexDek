package gameengine

import (
	"math/rand"
	"testing"
)

// seat_elimination_linked_exile_return_r63_test.go — firespot r63
// (sweep-currentmain #1, loki game 630 seed 6301338).
//
// Distinct from seat_elimination_linked_exile_r60_test.go (case 2: the
// source is STILL on the leaving seat's battlefield when HandleSeatElimination
// sweeps it). This is CASE 1: the "exile until ~ leaves" source already LEFT
// the battlefield (e.g. died in the same SBA cycle that eliminated its
// controller), so its permanent_ltb return trigger fires through
// PushPerCardTrigger — where the CR §800.4a "controller left the game" gate
// previously CEASED it, orphaning the prisoner in exile forever.
//
// Repro: Banisher Priest (controlled by the now-eliminated seat 1) had exiled
// A-Meria's Outrider (owned by seat 2). Seat 1's elimination removed Banisher
// Priest; the LTB return was ceased; A-Meria's Outrider was stranded in seat
// 2 exile with ExiledByTimestamp pointing at the gone source → 6
// ExileLinkageIntegrity hits. Fix: the return is performed directly (the card
// goes back to its OWNER's battlefield, a surviving player) rather than ceased.

// noopHandler is a stand-in per-card trigger handler — the §800.4a cease gate
// runs (and now performs the linked-exile return) BEFORE the handler is ever
// invoked, so its body is irrelevant to this test.
func noopHandler(_ *GameState, _ *Permanent, _ map[string]interface{}) {}

// TestSeatElimination_LinkedExileReturnsWhenSourceLTBCeased pins the fix:
// when an LTBReturn source's controller has left the game, the prisoner is
// returned to its (surviving) owner's battlefield instead of being orphaned.
func TestSeatElimination_LinkedExileReturnsWhenSourceLTBCeased(t *testing.T) {
	rng := rand.New(rand.NewSource(6301338))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Banisher Priest controlled+owned by seat 1, which is being eliminated.
	// It has ALREADY left the battlefield (not in any seat's Battlefield) —
	// this is its permanent_ltb dispatch.
	srcCard := &Card{
		Name:       "Banisher Priest",
		Owner:      1,
		InstanceID: "h1OGVW100115",
		Types:      []string{"creature"},
	}
	src := &Permanent{
		Card:       srcCard,
		Controller: 1,
		Owner:      1,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}

	// A-Meria's Outrider — owned by SURVIVING seat 2, exiled by the priest.
	preyCard := &Card{
		Name:       "A-Meria's Outrider",
		Owner:      2,
		InstanceID: "h2OGVG200042",
		Types:      []string{"creature"},
	}
	gs.MintedInstanceIDs = map[string]struct{}{
		srcCard.InstanceID:  {},
		preyCard.InstanceID: {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		srcCard.InstanceID:  srcCard.Name,
		preyCard.InstanceID: preyCard.Name,
	}
	preyCard.ExiledByTimestamp = src.Timestamp
	src.LinkedExile = []*Card{preyCard}
	src.ExiledByMe = []string{preyCard.InstanceID}
	src.LinkageKind = LTBReturn
	gs.Seats[2].Exile = append(gs.Seats[2].Exile, preyCard)

	// Seat 1 has left the game (its §800.4a sweep ran / is running).
	gs.Seats[1].Lost = true
	gs.Seats[1].LeftGame = true

	// Fire the source's LTB trigger through the bridge — this is the path
	// that previously ceased the return.
	PushPerCardTrigger(gs, src, noopHandler, map[string]interface{}{"perm": src})

	// 1. Prey returned to its owner's (seat 2) battlefield.
	onBF := false
	for _, p := range gs.Seats[2].Battlefield {
		if p != nil && p.Card == preyCard {
			onBF = true
			break
		}
	}
	if !onBF {
		t.Fatalf("A-Meria's Outrider must return to seat 2 battlefield, not be orphaned in exile")
	}
	// 2. No longer in seat 2 exile.
	for _, c := range gs.Seats[2].Exile {
		if c == preyCard {
			t.Fatalf("A-Meria's Outrider must have left seat 2 exile after the return")
		}
	}
	// 3. Linkage bookkeeping drained on both sides.
	if preyCard.ExiledByTimestamp != 0 {
		t.Fatalf("prey.ExiledByTimestamp=%d, want 0 after return", preyCard.ExiledByTimestamp)
	}
	if len(src.LinkedExile) != 0 || len(src.ExiledByMe) != 0 {
		t.Fatalf("source linkage must be drained; LinkedExile=%v ExiledByMe=%v", src.LinkedExile, src.ExiledByMe)
	}
	// 4. ExileLinkageIntegrity runs clean (game 630 signature gone).
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("ExileLinkageIntegrity must pass; got: %v", err)
	}
}

// TestSeatElimination_LinkedExileNotRoutedToDepartedOwner pins the guard: a
// prisoner whose OWNER has also left the game is NOT routed onto a departed
// seat's battlefield — its tag is cleared (it ceases via the owner's own
// §800.4a sweep) and the invariant still runs clean.
func TestSeatElimination_LinkedExileNotRoutedToDepartedOwner(t *testing.T) {
	rng := rand.New(rand.NewSource(6301338))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	srcCard := &Card{Name: "Oblivion Ring", Owner: 1, InstanceID: "h1OGVW100200", Types: []string{"enchantment"}}
	src := &Permanent{
		Card: srcCard, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	preyCard := &Card{Name: "Goblin Guide", Owner: 3, InstanceID: "h3OGVR100201", Types: []string{"creature"}}
	gs.MintedInstanceIDs = map[string]struct{}{srcCard.InstanceID: {}, preyCard.InstanceID: {}}
	gs.MintedInstanceIDNames = map[string]string{srcCard.InstanceID: srcCard.Name, preyCard.InstanceID: preyCard.Name}
	preyCard.ExiledByTimestamp = src.Timestamp
	src.LinkedExile = []*Card{preyCard}
	src.ExiledByMe = []string{preyCard.InstanceID}
	src.LinkageKind = LTBReturn
	gs.Seats[3].Exile = append(gs.Seats[3].Exile, preyCard)

	// BOTH the source's controller (seat 1) AND the prey's owner (seat 3)
	// have left the game.
	gs.Seats[1].Lost, gs.Seats[1].LeftGame = true, true
	gs.Seats[3].Lost, gs.Seats[3].LeftGame = true, true

	PushPerCardTrigger(gs, src, noopHandler, map[string]interface{}{"perm": src})

	// Prey must NOT be routed onto seat 3's (departed) battlefield.
	for _, p := range gs.Seats[3].Battlefield {
		if p != nil && p.Card == preyCard {
			t.Fatalf("prey must not be returned to a departed owner's battlefield")
		}
	}
	// Tag cleared and linkage drained regardless, so the invariant is clean.
	if preyCard.ExiledByTimestamp != 0 {
		t.Fatalf("prey.ExiledByTimestamp=%d, want 0", preyCard.ExiledByTimestamp)
	}
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("ExileLinkageIntegrity must pass; got: %v", err)
	}
}
