package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestPerCardTriggers_CrossSeatResolveAPNAP pins CR §603.3b for the
// per_card dispatch path. When a single game event fires triggered
// abilities controlled by different players simultaneously, they are
// placed on the stack in APNAP order (active player's FIRST), so under
// the LIFO stack the active player's trigger resolves LAST and a
// non-active player's resolves FIRST.
//
// The per_card path (fireTrigger → PushPerCardTrigger) resolves each
// trigger INLINE as it is pushed rather than batching then draining
// LIFO. Before the r63 fix it iterated seats in APNAP order (active
// first) and inline-resolved, so the ACTIVE player's trigger resolved
// FIRST — backwards from §603.3b. The fix iterates in resolution order
// (reverse APNAP) so the inline resolution reproduces the LIFO outcome.
//
// Mechanism: an identical "OrderObserver" creature_dies handler sits on
// both the active seat (0) and a non-active seat (1). A third creature
// (seat 2) dies in one SBA pass, firing one creature_dies event that
// dispatches to BOTH observers simultaneously. The non-active seat's
// observer must resolve BEFORE the active seat's.
func TestPerCardTriggers_CrossSeatResolveAPNAP(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var resolveOrder []int
	Global().OnTrigger("OrderObserver", "creature_dies",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			// perm is the observer source; record WHICH seat's observer
			// resolved, in resolution order.
			if perm != nil {
				resolveOrder = append(resolveOrder, perm.Controller)
			}
		})

	gs := newGame(t, 3)
	gs.Active = 0 // active player = seat 0

	// Identical observer on the active seat (0) and a non-active seat (1).
	obs0 := addPerm(gs, 0, "OrderObserver", "creature")
	obs0.Card.BasePower = 1
	obs0.Card.BaseToughness = 1
	obs1 := addPerm(gs, 1, "OrderObserver", "creature")
	obs1.Card.BasePower = 1
	obs1.Card.BaseToughness = 1

	// Sacrificial dier on a third seat so neither observer is the dier.
	dier := addPerm(gs, 2, "Dier", "creature")
	dier.Card.BasePower = 1
	dier.Card.BaseToughness = 1
	dier.MarkedDamage = 1 // §704.5g lethal damage

	gameengine.StateBasedActions(gs)

	// Both observers must have resolved exactly once.
	if len(resolveOrder) != 2 {
		t.Fatalf("expected 2 observer resolutions (one per seat), got %d: %v",
			len(resolveOrder), resolveOrder)
	}

	// CR §603.3b: the non-active seat (1) resolves BEFORE the active seat
	// (0). apnap = [0,1,2]; the active player's trigger goes on the stack
	// first and resolves last.
	if resolveOrder[0] != 1 || resolveOrder[1] != 0 {
		t.Fatalf("CR §603.3b violation: expected non-active seat 1 to resolve "+
			"before active seat 0 (want [1,0]), got %v", resolveOrder)
	}
}
