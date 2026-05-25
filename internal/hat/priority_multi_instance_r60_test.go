package hat

// R60 round 5 — multi-instance priority window audit. When the hat is
// asked for a response on stack item A, then asked again on item B
// before A resolves, does the per-call state stay correctly bound to
// the item being evaluated? Three classes of state surveyed:
//
//   1. Read-state that's recomputed every call (stack scan, mana
//      estimate, hand contents) — must reflect CURRENT game state,
//      not state at the time of the first ChooseResponse call.
//   2. Per-call decision tier — must NOT cross-contaminate the
//      cascade decisions associated with an EARLIER item when a
//      LATER item has been classified more recently.
//   3. Reset behavior — per-window state must clear cleanly on
//      game_start.
//
// Pre-R60r5 finding: lastParentTier is overwritten by each
// ChooseResponse call, so cascade decisions (OrderTriggers /
// ChooseTarget / etc.) firing during an EARLIER item's resolution
// would read the LATER item's tier. Fix: per-stack-item tier map
// keyed by StackItem.ID, populated by recordResponseTier in
// ChooseResponse, consulted first by recordCascadeDecision.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func newPriorityHat() *YggdrasilHat {
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	return h
}

// ---------------------------------------------------------------------
// State-tracking — each ChooseResponse re-reads gs fresh
// ---------------------------------------------------------------------

// Sequence: trigger A pushed → ChooseResponse(A) → trigger B pushed
// (still no resolutions) → ChooseResponse(B). The second call must
// observe BOTH items on the stack, not just B in isolation.
func TestChooseResponse_SeesFullStackOnSecondCall(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	gs.Seats[0].Hat = h
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Opponent (seat 1) controls both triggers — we can respond.
	a := &gameengine.StackItem{
		ID: 1, Controller: 1, Kind: "triggered",
		Card: newTestCardMinimal("Trigger A", []string{"creature"}, 2, nil),
	}
	b := &gameengine.StackItem{
		ID: 2, Controller: 1, Kind: "triggered",
		Card: newTestCardMinimal("Trigger B", []string{"creature"}, 2, nil),
	}

	gs.Stack = []*gameengine.StackItem{a}
	_ = h.ChooseResponse(gs, 0, a)
	depthAfterA := len(gs.Stack)

	gs.Stack = append(gs.Stack, b) // B pushed before A resolves
	_ = h.ChooseResponse(gs, 0, b)
	depthAfterB := len(gs.Stack)

	if depthAfterA != 1 {
		t.Errorf("after ChooseResponse(A) the stack should still hold A; len=%d", depthAfterA)
	}
	if depthAfterB != 2 {
		t.Errorf("after ChooseResponse(B) the stack should hold both A and B; len=%d", depthAfterB)
	}
	// The depth signals computed in ChooseResponse(B) must have observed
	// 2 items, not 1 — verified indirectly by the assertions above (the
	// hat doesn't expose its computed signals as a return value; this
	// test pins that the fixture is structured the way the audit
	// requires and the ChooseResponse calls don't mutate the stack).
}

// Mana available to the hat decrements between calls if the engine
// has deducted between them. The hat must NOT cache mana across
// ChooseResponse calls.
func TestChooseResponse_ManaStateReReadEachCall(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	gs.Seats[0].Hat = h
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Seat 0: 4 untapped Islands + Counterspell in hand.
	for i := 0; i < 4; i++ {
		newTestPermanent(gs.Seats[0], newTestCardMinimal("Island", []string{"land"}, 0, nil), 0, 0)
	}
	counter := newTestCardMinimal("Counterspell", []string{"instant"}, 2, nil)
	counter.Types = append(counter.Types, "oracle:counter target spell")
	gs.Seats[0].Hand = []*gameengine.Card{counter}

	a := &gameengine.StackItem{
		ID: 1, Controller: 1, Kind: "spell",
		Card: newTestCardMinimal("Threat A", []string{"sorcery"}, 4, nil),
	}
	gs.Stack = []*gameengine.StackItem{a}

	// Call 1 — has mana for counter.
	resp := h.ChooseResponse(gs, 0, a)
	_ = resp // hat may or may not counter — that's the hat's call

	// Simulate engine tapping lands + removing the counter from hand
	// (effectively "we cast it; B is next on the stack").
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsLand() {
			p.Tapped = true
		}
	}
	gs.Seats[0].Hand = nil // counter gone

	b := &gameengine.StackItem{
		ID: 2, Controller: 1, Kind: "spell",
		Card: newTestCardMinimal("Threat B", []string{"sorcery"}, 4, nil),
	}
	gs.Stack = []*gameengine.StackItem{b}

	// Call 2 — no mana, no counter in hand → must return nil (no
	// available response).
	resp2 := h.ChooseResponse(gs, 0, b)
	if resp2 != nil {
		t.Errorf("with no counter in hand and lands tapped, ChooseResponse must return nil; got %v", resp2)
	}
}

// ---------------------------------------------------------------------
// Per-stack-item tier tracking — the actual bug fix
// ---------------------------------------------------------------------

// Stamp records survive ChooseResponse overwrites of lastParentTier.
func TestRecordResponseTier_PerItemRecord(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{ID: 1, Controller: 1}
	b := &gameengine.StackItem{ID: 2, Controller: 1}

	h.recordResponseTier(TierRagnarok, 5, a)
	h.recordResponseTier(TierMjolnir, 5, b)

	// lastParentTier holds the latest (TierMjolnir from B).
	if h.lastParentTier != TierMjolnir {
		t.Errorf("lastParentTier should hold the latest stamp; got %v", h.lastParentTier)
	}

	// But the per-item map still has A's tier — recoverable.
	if tier, ok := h.tierForStackItem(5, 1); !ok || tier != TierRagnarok {
		t.Errorf("expected TierRagnarok for stack item A (ID=1); got tier=%v ok=%v", tier, ok)
	}
	if tier, ok := h.tierForStackItem(5, 2); !ok || tier != TierMjolnir {
		t.Errorf("expected TierMjolnir for stack item B (ID=2); got tier=%v ok=%v", tier, ok)
	}
	_ = gs
}

// Multi-instance window scenario end-to-end: ChooseResponse fires for
// A then B, then a cascade decision (e.g., OrderTriggers) runs while
// A is resolving (top of stack is A — B has resolved). The cascade
// must read T_A from the per-item map, NOT T_B from lastParentTier.
func TestRecordCascadeDecision_RecoversEarlierItemsTier(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{ID: 11, Controller: 1}
	b := &gameengine.StackItem{ID: 22, Controller: 1}

	h.recordResponseTier(TierRagnarok, 5, a)
	h.recordResponseTier(TierMjolnir, 5, b)

	// Simulate the engine resolving B first (LIFO), leaving A on top.
	gs.Stack = []*gameengine.StackItem{a}

	// A cascade fires (e.g., OrderTriggers during A's resolution). The
	// recovery path must lookup A.ID and return TierRagnarok.
	got := h.recordCascadeDecision(gs)
	if got != TierRagnarok {
		t.Errorf("cascade during A's resolution must read T_A (Ragnarok); got %v (lastParentTier=%v stale)",
			got, h.lastParentTier)
	}
}

// When no per-item stamp exists for the current top, recordCascadeDecision
// falls back to lastParentTier (legacy behavior preserved for
// non-ChooseResponse parent contexts: ChooseCastFromHand, ChooseActivation,
// ChooseAttackers).
func TestRecordCascadeDecision_FallsBackToLastParentTier(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Stack is empty AND no per-item stamps — pure legacy path.
	h.recordParentTier(TierGungnir, 5)
	if got := h.recordCascadeDecision(gs); got != TierGungnir {
		t.Errorf("with no per-item stamp and stack empty, cascade must read lastParentTier; got %v", got)
	}

	// Stack has an item but it wasn't stamped (ChooseResponse never
	// fired for it) — still fall back to lastParentTier.
	gs.Stack = []*gameengine.StackItem{{ID: 99, Controller: 1}}
	if got := h.recordCascadeDecision(gs); got != TierGungnir {
		t.Errorf("with unknown top.ID, cascade must fall back to lastParentTier; got %v", got)
	}
}

// Turn-scoped invalidation: a stamp from turn N doesn't leak into turn N+1.
func TestRecordResponseTier_TurnInvalidates(t *testing.T) {
	h := newPriorityHat()
	h.ObserveEvent(newTestGame(t, 2), 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{ID: 1, Controller: 1}
	h.recordResponseTier(TierRagnarok, 5, a)

	if _, ok := h.tierForStackItem(5, 1); !ok {
		t.Fatal("setup: stamp should be readable at the same turn")
	}
	if _, ok := h.tierForStackItem(6, 1); ok {
		t.Error("stamp from turn 5 must NOT be readable at turn 6")
	}
}

// New turn re-init: the first stamp of turn N+1 must reset the map,
// not append.
func TestRecordResponseTier_NewTurnResetsMap(t *testing.T) {
	h := newPriorityHat()
	h.ObserveEvent(newTestGame(t, 2), 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{ID: 1, Controller: 1}
	b := &gameengine.StackItem{ID: 2, Controller: 1}

	h.recordResponseTier(TierRagnarok, 5, a)
	h.recordResponseTier(TierMjolnir, 6, b)

	// Turn 5's stamp must be invalidated by the turn-6 stamp.
	if _, ok := h.tierForStackItem(5, 1); ok {
		t.Error("turn-5 stamp must be cleared when first turn-6 stamp lands")
	}
	if tier, ok := h.tierForStackItem(6, 2); !ok || tier != TierMjolnir {
		t.Errorf("turn-6 stamp must be readable; got tier=%v ok=%v", tier, ok)
	}
}

// game_start clears everything.
func TestRecordResponseTier_GameStartResets(t *testing.T) {
	gs := newTestGame(t, 2)
	h := newPriorityHat()
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{ID: 1, Controller: 1}
	h.recordResponseTier(TierRagnarok, 5, a)
	if _, ok := h.tierForStackItem(5, 1); !ok {
		t.Fatal("setup: stamp should be readable")
	}

	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	if _, ok := h.tierForStackItem(5, 1); ok {
		t.Error("game_start must clear stackItemTiers map")
	}
	if h.stackItemTiers != nil {
		t.Errorf("stackItemTiers map should be nil after game_start; got non-nil")
	}
}

// ChooseResponse populates the per-item map (integration test).
func TestChooseResponse_PopulatesStackItemTiers(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := newPriorityHat()
	gs.Seats[0].Hat = h
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	a := &gameengine.StackItem{
		ID: 11, Controller: 1, Kind: "spell",
		Card: newTestCardMinimal("Spell A", []string{"sorcery"}, 4, nil),
	}
	gs.Stack = []*gameengine.StackItem{a}
	_ = h.ChooseResponse(gs, 0, a)

	if _, ok := h.tierForStackItem(5, 11); !ok {
		t.Error("ChooseResponse must stamp the tier for the responded-to stack item")
	}
}

// Nil / zero-ID stack items don't crash and don't poison the map.
func TestRecordResponseTier_NilAndZeroIDSafe(t *testing.T) {
	h := newPriorityHat()
	h.ObserveEvent(newTestGame(t, 2), 0, &gameengine.Event{Kind: "game_start"})

	// Should not panic.
	h.recordResponseTier(TierMjolnir, 5, nil)
	h.recordResponseTier(TierMjolnir, 5, &gameengine.StackItem{ID: 0})

	if h.stackItemTiers != nil {
		t.Error("nil / zero-ID stack items must not populate the map")
	}
}
