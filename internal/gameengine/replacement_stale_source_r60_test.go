package gameengine

import (
	"math/rand"
	"testing"
)

// Loki r60 extreme-stress / seed 31415 game 9111 turn 57 surfaced:
//
//   SBACompleteness x6 — seat 0 life=0 Lost=false, no loss-prevention,
//   SBA 704.5a missed
//
// Recent events showed `replacement_applied source=Platinum Angel`
// firing twice on seat 0, but NO Platinum Angel was on any seat's
// battlefield in the state snapshot — the replacement effect was
// PHANTOM, registered against a perm that had already left play
// without an `UnregisterReplacementsForPermanent` call. Platinum
// Angel's `would_lose_game` cancel kept perma-preventing seat 0
// from losing.
//
// Every canonical leave-play path (DestroyPermanent, ExilePermanent,
// sacrificePermanentImpl, BouncePermanent, destroyPermSBA,
// sacrificePermSBA, HandleSeatElimination) calls
// UnregisterReplacementsForPermanent. The leak source is one of the
// non-canonical paths that use `removePermanent` directly without
// the unregister:
//   - keywords_batch6.go:293 / 306 — mutate-eaten perm
//   - keywords_sweep.go:196 — sweep alt-cost return
//   - per_card flicker / blink / exile-self helpers
//
// Fix is a defensive backstop in `pickReplacement`: skip any
// replacement whose `SourcePerm` is non-nil AND not on any seat's
// battlefield. This catches phantoms regardless of which path leaked
// them. Real global replacements (SourcePerm == nil) still apply.

// TestPickReplacement_SkipsStaleSourceNotOnAnyBattlefield pins the
// bit-stable fix: a Platinum Angel replacement whose source is no
// longer on the battlefield must NOT prevent the seat from losing.
func TestPickReplacement_SkipsStaleSourceNotOnAnyBattlefield(t *testing.T) {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Construct a Platinum Angel Permanent that's NOT on any battlefield
	// (simulates the phantom-source state).
	stale := &Permanent{
		Card:       &Card{Name: "Platinum Angel", Owner: 0, Types: []string{"artifact", "creature"}},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
	}
	// Register Platinum Angel's would_lose_game cancel against the stale
	// perm. (Same call site RegisterPlatinumAngel uses.)
	RegisterPlatinumAngel(gs, stale)

	// Sanity: the replacement IS in the registry.
	foundReg := false
	for _, re := range gs.Replacements {
		if re != nil && re.EventType == "would_lose_game" && re.SourcePerm == stale {
			foundReg = true
			break
		}
	}
	if !foundReg {
		t.Fatal("precondition: Platinum Angel's would_lose_game replacement must be registered")
	}

	// Fire the would_lose_game event. Pre-fix: the replacement applies,
	// FireLoseGameEvent returns true (cancelled). Post-fix: the stale
	// source is detected and the replacement is skipped — cancelled
	// returns false.
	if FireLoseGameEvent(gs, 0) {
		t.Fatal("REGRESSION: stale Platinum Angel replacement should not cancel would_lose_game when its source is off-battlefield")
	}
}

// TestPickReplacement_StillAppliesWhenSourceOnBattlefield pins the
// legitimate path: a real Platinum Angel on the battlefield must
// still prevent the controller from losing.
func TestPickReplacement_StillAppliesWhenSourceOnBattlefield(t *testing.T) {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	live := &Permanent{
		Card:       &Card{Name: "Platinum Angel", Owner: 0, Types: []string{"artifact", "creature"}},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, live)
	RegisterPlatinumAngel(gs, live)

	if !FireLoseGameEvent(gs, 0) {
		t.Fatal("live Platinum Angel on seat 0's battlefield must cancel seat 0's would_lose_game")
	}
}

// TestPickReplacement_GlobalNilSourceStillApplies confirms the gate
// is scoped — replacements without a SourcePerm (global effects like
// Anointed Procession-style doublers) bypass the stale-source check.
func TestPickReplacement_GlobalNilSourceStillApplies(t *testing.T) {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Register a global cancel-this-loss with SourcePerm=nil — the gate
	// must NOT skip it.
	called := false
	gs.RegisterReplacement(&ReplacementEffect{
		EventType:      "would_lose_game",
		HandlerID:      "test_global_cancel_loss",
		SourcePerm:     nil, // global
		ControllerSeat: 0,
		Timestamp:      gs.NextTimestamp(),
		Category:       CategoryOther,
		Applies: func(gs *GameState, ev *ReplEvent) bool {
			return ev.TargetSeat == 0
		},
		ApplyFn: func(gs *GameState, ev *ReplEvent) {
			called = true
			ev.Cancelled = true
		},
	})

	if !FireLoseGameEvent(gs, 0) {
		t.Fatal("global (nil-source) replacement must still apply")
	}
	if !called {
		t.Fatal("global replacement's ApplyFn should have been called")
	}
}

// TestPickReplacement_RegisteredThenLeftPlayMidGame is the end-to-end
// shape: Platinum Angel ETBs (replacement registered), then leaves
// via a hypothetical non-canonical removal that skips the unregister.
// The pickReplacement backstop must keep the loss flowing once the
// perm is off-battlefield.
func TestPickReplacement_RegisteredThenLeftPlayMidGame(t *testing.T) {
	rng := rand.New(rand.NewSource(31415))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	live := &Permanent{
		Card:       &Card{Name: "Platinum Angel", Owner: 0, Types: []string{"artifact", "creature"}},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, live)
	RegisterPlatinumAngel(gs, live)

	// Confirm loss IS prevented while live.
	if !FireLoseGameEvent(gs, 0) {
		t.Fatal("precondition: live Platinum Angel must prevent loss")
	}

	// Simulate a non-canonical removal that DOESN'T unregister the
	// replacement (e.g. mutate-eaten perm, sweep alt-cost return, per_card
	// flicker that skipped the unregister call).
	for i, p := range gs.Seats[0].Battlefield {
		if p == live {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	// Deliberately DO NOT call gs.UnregisterReplacementsForPermanent(live)
	// — that's the pre-fix bug shape.

	// Post-fix: the stale replacement is skipped, loss is NOT prevented.
	if FireLoseGameEvent(gs, 0) {
		t.Fatal("REGRESSION: stale Platinum Angel replacement still cancels after the perm left play")
	}
}
