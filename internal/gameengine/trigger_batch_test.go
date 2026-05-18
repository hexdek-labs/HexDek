package gameengine

// R38 stack-priority fix: tests for CR §603.3b batched simultaneous-trigger
// placement. Each test exercises one of the three properties called out in
// the audit follow-up (docs/stack-priority-r38-fix.md):
//
//   (a) multiple simultaneous triggers from a single event get batched
//   (b) controller orders them per §603.3b within each player's bucket
//   (c) ordering is preserved across resolution (APNAP push order ⇒ LIFO
//       resolution ⇒ NAP triggers resolve before AP triggers)

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// noopEffect builds a non-nil gameast.Effect that resolves to nothing —
// avoids the need to wire real spell effects when the test only cares
// about stack ordering, not state mutation. Sequence with empty Items is
// a no-op under ResolveEffect.
func noopEffect() gameast.Effect {
	return gameast.Sequence{Items: nil}
}

// dummyTriggerPerm builds a minimal *Permanent owned/controlled by seat
// with a named card, suitable as the source argument to
// PushTriggeredAbility. The card name appears in the trace log so failing
// tests are easy to read.
func dummyTriggerPerm(seat int, name string) *Permanent {
	return &Permanent{
		Card:       &Card{Name: name, Owner: seat},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
}

// (countEventsOfKind is provided by activation_test.go in this package.)

// stackPushSources walks EventLog and returns the Source field of every
// stack_push event in the order they were logged.
func stackPushSources(gs *GameState) []string {
	var out []string
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_push" {
			out = append(out, ev.Source)
		}
	}
	return out
}

// stackResolveSources walks EventLog and returns the Source field of every
// stack_resolve event in resolution order.
func stackResolveSources(gs *GameState) []string {
	var out []string
	for _, ev := range gs.EventLog {
		if ev.Kind == "stack_resolve" {
			out = append(out, ev.Source)
		}
	}
	return out
}

// installStubHats puts a no-op GreedyHatStub on every seat so PriorityRound
// has a Hat to ask for ChooseResponse (returns nil → pass). Without this,
// the priority round can early-return in surprising ways.
func installStubHats(gs *GameState) {
	for _, s := range gs.Seats {
		if s != nil && s.Hat == nil {
			s.Hat = &GreedyHatStub{}
		}
	}
}

// ---------------------------------------------------------------------------
// (a) Multiple simultaneous triggers from a single event get batched.
// ---------------------------------------------------------------------------

// TestTriggerBatch_DefersUntilEnd_R38 verifies that inside a batch frame,
// PushTriggeredAbility appends to pendingTriggers instead of pushing onto
// the stack. The trigger does not appear on gs.Stack until EndTriggerBatch
// runs. Without batching (BUG-37-A) each PushTriggeredAbility would push
// AND resolve before returning — pendingTriggers would always be empty
// and the stack would briefly hold one item per push.
func TestTriggerBatch_DefersUntilEnd_R38(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	installStubHats(gs)
	gs.Active = 0

	pA := dummyTriggerPerm(0, "AP-Trig")
	pB := dummyTriggerPerm(1, "NAP1-Trig")
	pC := dummyTriggerPerm(2, "NAP2-Trig")

	opened := BeginTriggerBatch(gs)
	if !opened {
		t.Fatal("first BeginTriggerBatch should return opened=true")
	}

	PushTriggeredAbility(gs, pA, noopEffect())
	PushTriggeredAbility(gs, pB, noopEffect())
	PushTriggeredAbility(gs, pC, noopEffect())

	if got := len(gs.Stack); got != 0 {
		t.Fatalf("inside batch, stack must remain empty; got %d items", got)
	}
	if got := len(gs.pendingTriggers); got != 3 {
		t.Fatalf("expected 3 pending triggers, got %d", got)
	}
	if countEventsOfKind(gs, "stack_push") != 0 {
		t.Fatal("no stack_push events should fire while batch is open")
	}

	EndTriggerBatch(gs, opened)

	// After End, the batch was ordered, pushed, and drained.
	if got := len(gs.Stack); got != 0 {
		t.Fatalf("after End the stack must be drained; got %d items", got)
	}
	if got := len(gs.pendingTriggers); got != 0 {
		t.Fatalf("pendingTriggers must be cleared after End; got %d", got)
	}
	if countEventsOfKind(gs, "triggers_ordered") != 1 {
		t.Fatalf("expected exactly one triggers_ordered event from the batched push, got %d",
			countEventsOfKind(gs, "triggers_ordered"))
	}
	if got := countEventsOfKind(gs, "stack_push"); got != 3 {
		t.Fatalf("expected 3 stack_push events from the drained batch, got %d", got)
	}
	if got := countEventsOfKind(gs, "stack_resolve"); got != 3 {
		t.Fatalf("expected 3 stack_resolve events from the drained batch, got %d", got)
	}
}

// TestTriggerBatch_ReentrantNoOp_R38 verifies that nested fire sites
// observe an existing batch and don't push prematurely. The inner Begin
// returns opened=false; only the outer End triggers the drain. This is
// the property that lets per-card handlers safely call FireCardTrigger
// while themselves inside another fire-site's batch frame without
// double-flushing.
func TestTriggerBatch_ReentrantNoOp_R38(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	installStubHats(gs)

	outer := BeginTriggerBatch(gs)
	if !outer {
		t.Fatal("outer BeginTriggerBatch should return true")
	}
	inner := BeginTriggerBatch(gs)
	if inner {
		t.Fatal("inner BeginTriggerBatch must return false (a batch was already open)")
	}

	PushTriggeredAbility(gs, dummyTriggerPerm(0, "T1"), noopEffect())
	EndTriggerBatch(gs, inner) // inner: no-op
	if got := len(gs.Stack); got != 0 {
		t.Fatalf("inner End must not flush; stack should be empty, got %d", got)
	}
	if got := len(gs.pendingTriggers); got != 1 {
		t.Fatalf("inner End must not flush; expected 1 pending, got %d", got)
	}

	PushTriggeredAbility(gs, dummyTriggerPerm(1, "T2"), noopEffect())
	EndTriggerBatch(gs, outer) // outer: flush + drain
	if got := len(gs.pendingTriggers); got != 0 {
		t.Fatalf("outer End must clear pendingTriggers; got %d", got)
	}
	if countEventsOfKind(gs, "triggers_ordered") != 1 {
		t.Fatal("outer End should emit exactly one triggers_ordered event")
	}
}

// ---------------------------------------------------------------------------
// (b) Controller orders triggers per §603.3b within each player's bucket.
// ---------------------------------------------------------------------------

// TestTriggerBatch_ControllerOrdersWithinBucket_R38 verifies that
// Hat.OrderTriggers is consulted to reorder a single controller's triggers
// within their bucket before pushing. Seat 0 owns two triggers; installing
// reverseTriggerHat on seat 0 reverses their order in the final stack.
// Seat 1's trigger sits above (resolves first) regardless.
func TestTriggerBatch_ControllerOrdersWithinBucket_R38(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	installStubHats(gs)
	// Install a reversing Hat on seat 0 — its OrderTriggers reverses the
	// intra-bucket arrival order.
	gs.Seats[0].Hat = &reverseTriggerHat{}

	pA1 := dummyTriggerPerm(0, "AP-First")
	pA2 := dummyTriggerPerm(0, "AP-Second")
	pB := dummyTriggerPerm(1, "NAP")

	opened := BeginTriggerBatch(gs)
	PushTriggeredAbility(gs, pA1, noopEffect())
	PushTriggeredAbility(gs, pA2, noopEffect())
	PushTriggeredAbility(gs, pB, noopEffect())
	EndTriggerBatch(gs, opened)

	pushed := stackPushSources(gs)
	if len(pushed) != 3 {
		t.Fatalf("expected 3 stack_push events, got %d: %v", len(pushed), pushed)
	}
	// Push order should be AP-bucket-reversed first, then NAP last.
	// APNAP rule says AP triggers are pushed first (resolve last under LIFO),
	// reverseTriggerHat reverses arrival within seat 0's bucket.
	wantPush := []string{"AP-Second", "AP-First", "NAP"}
	for i, name := range wantPush {
		if pushed[i] != name {
			t.Fatalf("push[%d]: want %q, got %q (full push order: %v)",
				i, name, pushed[i], pushed)
		}
	}
}

// ---------------------------------------------------------------------------
// (c) Ordering preserved across resolution (APNAP push ⇒ LIFO resolve).
// ---------------------------------------------------------------------------

// TestTriggerBatch_ResolutionOrderAPNAP_R38 fires four triggers across all
// four seats in a single batch. Per §603.3b + LIFO stack resolution, the
// resolution order is the reverse of APNAP — the last non-active player's
// trigger resolves first; the active player's trigger resolves last.
// This is the property that observers, replacement effects, and watchers
// rely on for correctness.
func TestTriggerBatch_ResolutionOrderAPNAP_R38(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 1
	installStubHats(gs)

	// Push order will be: active=1 (AP), then NAP order 2, 3, 0.
	// Resolution order: reverse → 0, 3, 2, 1.
	perms := map[int]*Permanent{
		0: dummyTriggerPerm(0, "Seat0"),
		1: dummyTriggerPerm(1, "Seat1-AP"),
		2: dummyTriggerPerm(2, "Seat2"),
		3: dummyTriggerPerm(3, "Seat3"),
	}

	opened := BeginTriggerBatch(gs)
	// Intentionally push in arbitrary arrival order — the batch should
	// re-order to APNAP regardless of when we called Push.
	PushTriggeredAbility(gs, perms[3], noopEffect())
	PushTriggeredAbility(gs, perms[0], noopEffect())
	PushTriggeredAbility(gs, perms[1], noopEffect())
	PushTriggeredAbility(gs, perms[2], noopEffect())
	EndTriggerBatch(gs, opened)

	// Push order should be APNAP: seat 1, 2, 3, 0.
	pushed := stackPushSources(gs)
	wantPush := []string{"Seat1-AP", "Seat2", "Seat3", "Seat0"}
	if len(pushed) != len(wantPush) {
		t.Fatalf("expected %d pushes, got %d: %v", len(wantPush), len(pushed), pushed)
	}
	for i, name := range wantPush {
		if pushed[i] != name {
			t.Fatalf("push[%d]: want %q, got %q (full push order: %v)",
				i, name, pushed[i], pushed)
		}
	}

	// Resolution order is reversed under LIFO.
	resolved := stackResolveSources(gs)
	wantResolve := []string{"Seat0", "Seat3", "Seat2", "Seat1-AP"}
	if len(resolved) != len(wantResolve) {
		t.Fatalf("expected %d resolutions, got %d: %v", len(wantResolve), len(resolved), resolved)
	}
	for i, name := range wantResolve {
		if resolved[i] != name {
			t.Fatalf("resolve[%d]: want %q, got %q (full resolve order: %v)",
				i, name, resolved[i], resolved)
		}
	}
}

// TestTriggerBatch_PushTriggeredAbility_UnbatchedFallback_R38 verifies the
// legacy path remains intact: outside a batch frame, PushTriggeredAbility
// still pushes immediately and resolves inline. This preserves backward
// compatibility for test fixtures and single-trigger callers.
func TestTriggerBatch_PushTriggeredAbility_UnbatchedFallback_R38(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	installStubHats(gs)

	// No batch open.
	PushTriggeredAbility(gs, dummyTriggerPerm(0, "Solo"), noopEffect())

	// The trigger should have pushed AND resolved in this single call.
	if got := len(gs.Stack); got != 0 {
		t.Fatalf("expected stack empty after unbatched push+resolve, got %d", got)
	}
	if countEventsOfKind(gs, "stack_push") != 1 {
		t.Fatalf("expected 1 stack_push, got %d", countEventsOfKind(gs, "stack_push"))
	}
	if countEventsOfKind(gs, "stack_resolve") != 1 {
		t.Fatalf("expected 1 stack_resolve, got %d", countEventsOfKind(gs, "stack_resolve"))
	}
	// No batching event when there's no batch.
	if countEventsOfKind(gs, "triggers_ordered") != 0 {
		t.Fatal("unbatched path must not emit triggers_ordered")
	}
}

// ---------------------------------------------------------------------------
// R39: APNAP Hat-choice order + §608.2c during-resolution deferral.
// ---------------------------------------------------------------------------

// recordingHat records every Hat.OrderTriggers invocation into a shared
// slice (in call order). Used by TestTriggerBatch_APNAPHatChoiceOrder_R39
// to verify that the active player's controller-choice happens before the
// non-active players', per CR §603.3b.
type recordingHat struct {
	GreedyHatStub
	seat int
	log  *[]int
}

func (h *recordingHat) OrderTriggers(gs *GameState, seatIdx int, triggers []*StackItem) []*StackItem {
	if h.log != nil {
		*h.log = append(*h.log, seatIdx)
	}
	return triggers
}

// TestTriggerBatch_APNAPHatChoiceOrder_R39 verifies CR §603.3b's exact
// procedural rule: "each player, IN APNAP ORDER, puts the triggered
// abilities he or she controls on the stack in any order he or she
// chooses." The active player makes their intra-bucket choice first, then
// each non-active player in turn order. R38 wired the property by
// delegating to OrderTriggersAPNAP; R39 pins it with an end-to-end test
// that records the order Hat.OrderTriggers is invoked across all four
// seats and asserts the call sequence is APNAP, not seat-index order.
func TestTriggerBatch_APNAPHatChoiceOrder_R39(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 2 // pick a non-zero seat so APNAP ≠ seat order

	// Hat invocation log shared across all seats.
	var hatCallOrder []int
	for i := range gs.Seats {
		gs.Seats[i].Hat = &recordingHat{seat: i, log: &hatCallOrder}
	}

	// One trigger per seat, so every seat's Hat.OrderTriggers is invoked
	// exactly once during OrderTriggersAPNAP.
	opened := BeginTriggerBatch(gs)
	for seat := 0; seat < 4; seat++ {
		PushTriggeredAbility(gs, dummyTriggerPerm(seat, "Seat"+seatLabel(seat)), noopEffect())
	}
	EndTriggerBatch(gs, opened)

	// APNAP starting from active=2: 2 → 3 → 0 → 1. Hat.OrderTriggers must
	// be invoked in that order — active player first, then NAPs in turn
	// order.
	wantCallOrder := []int{2, 3, 0, 1}
	if len(hatCallOrder) != len(wantCallOrder) {
		t.Fatalf("expected 4 Hat.OrderTriggers calls (one per seat), got %d: %v",
			len(hatCallOrder), hatCallOrder)
	}
	for i, seat := range wantCallOrder {
		if hatCallOrder[i] != seat {
			t.Fatalf("Hat.OrderTriggers call[%d]: want seat %d (APNAP position), got seat %d; full order: %v",
				i, seat, hatCallOrder[i], hatCallOrder)
		}
	}
}

// seatLabel is a tiny zero-alloc int formatter used only to label test
// permanents (Seat0 / Seat1 / ...). Avoids pulling in fmt for one line.
func seatLabel(n int) string {
	switch n {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// §608.2c: triggers fired during resolution defer until the outer
// resolution completes.
// ---------------------------------------------------------------------------

// fireOnResolveEffect is a test-only effect node whose resolution side-
// effect is to fire one or more triggered abilities via the engine's
// PushTriggeredAbility entry point. It satisfies gameast.Effect's
// interface via the custom resolve-hook indirection we install below.
type fireOnResolveEffect struct {
	gameast.Sequence // embed to satisfy the Effect interface
	fire             func(gs *GameState)
}

// resolveFireOnResolve is registered as the ResolveHook to dispatch
// fireOnResolveEffect during a stack-item resolution. Returning >0 tells
// stack.go to skip stock Effect dispatch — this hook is authoritative.
func resolveFireOnResolve(gs *GameState, item *StackItem) int {
	if item == nil {
		return 0
	}
	fx, ok := item.Effect.(*fireOnResolveEffect)
	if !ok || fx == nil {
		return 0
	}
	if fx.fire != nil {
		fx.fire(gs)
	}
	return 1
}

// TestStack_TriggersFiredDuringResolutionDeferred_R39 verifies CR §608.2c:
// "If an ability's effect causes an event to happen during the resolution
// of an ability, any triggered abilities that trigger as a result of that
// event are placed on the stack after the resolving ability finishes
// resolving."
//
// The test:
//   1. Builds a stack item (the "outer" resolving ability) whose Effect
//      is fireOnResolveEffect — when it resolves, its hook calls
//      PushTriggeredAbility for two new triggers AND captures the stack
//      state observed mid-resolution.
//   2. Resolves the outer item via ResolveStackTop.
//   3. Asserts: while the outer item's effect ran, those new triggers
//      were NOT on the stack (deferred to pending). They appear on the
//      stack only after the outer ResolveStackTop's outermost-frame defer
//      fires drainPendingTriggers. By the time ResolveStackTop returns,
//      the deferred triggers have been ordered, pushed, and resolved.
func TestStack_TriggersFiredDuringResolutionDeferred_R39(t *testing.T) {
	prevHook := ResolveHook
	ResolveHook = resolveFireOnResolve
	defer func() { ResolveHook = prevHook }()

	gs := NewGameState(2, nil, nil)
	installStubHats(gs)
	gs.Active = 0

	// Mid-resolution observations.
	var stackLenDuringOuter int
	var pendingLenDuringOuter int
	var resolveDepthDuringOuter int

	innerA := dummyTriggerPerm(0, "Inner-A")
	innerB := dummyTriggerPerm(1, "Inner-B")

	fire := &fireOnResolveEffect{
		fire: func(gs *GameState) {
			// During the outer effect: fire two triggers and capture state.
			PushTriggeredAbility(gs, innerA, noopEffect())
			PushTriggeredAbility(gs, innerB, noopEffect())
			stackLenDuringOuter = len(gs.Stack)
			pendingLenDuringOuter = len(gs.pendingTriggers)
			resolveDepthDuringOuter = gs.Flags["_resolve_frame_depth"]
		},
	}

	outer := &StackItem{
		Controller: 0,
		Source:     dummyTriggerPerm(0, "Outer"),
		Card:       &Card{Name: "Outer"},
		Effect:     fire,
		Kind:       "triggered",
	}
	PushStackItem(gs, outer)
	ResolveStackTop(gs)

	// §608.2c assertions (the headline behavior R39 fixes).
	if resolveDepthDuringOuter != 1 {
		t.Fatalf("during outer resolution, _resolve_frame_depth should be 1, got %d",
			resolveDepthDuringOuter)
	}
	if stackLenDuringOuter != 0 {
		t.Fatalf("during outer resolution, triggers fired by the effect must NOT be on the stack; got len(Stack)=%d",
			stackLenDuringOuter)
	}
	if pendingLenDuringOuter != 2 {
		t.Fatalf("during outer resolution, both triggers must be in pendingTriggers; got %d",
			pendingLenDuringOuter)
	}

	// After ResolveStackTop returns, the outermost-frame defer drained
	// the deferred triggers — they should have been pushed AND resolved.
	if got := len(gs.Stack); got != 0 {
		t.Fatalf("after outer resolution + drain, stack should be empty; got %d", got)
	}
	if got := len(gs.pendingTriggers); got != 0 {
		t.Fatalf("pendingTriggers must be drained after outer ResolveStackTop returns; got %d", got)
	}
	// One stack_resolve for the outer item, plus one each for the two
	// deferred triggers = 3 total.
	if got := countEventsOfKind(gs, "stack_resolve"); got != 3 {
		t.Fatalf("expected 3 stack_resolve events (outer + 2 deferred), got %d", got)
	}
	// The outer item must have resolved before either deferred trigger.
	resolves := stackResolveSources(gs)
	if len(resolves) < 3 {
		t.Fatalf("expected at least 3 resolves, got %v", resolves)
	}
	if resolves[0] != "Outer" {
		t.Fatalf("§608.2c: outer ability must finish resolving FIRST; resolve order: %v", resolves)
	}
}

// TestStack_DeferredTriggers_BatchedAPNAP_R39 is the composite property:
// triggers fired during resolution defer per §608.2c AND when finally
// drained they are ordered APNAP per §603.3b. The outer effect fires four
// triggers (one per seat in a 4-seat game with Active=1). After
// resolution they must drain in APNAP push order — Seat1-AP first,
// then Seat2, Seat3, Seat0 — which under LIFO resolves in the reverse
// (Seat0, Seat3, Seat2, Seat1-AP).
func TestStack_DeferredTriggers_BatchedAPNAP_R39(t *testing.T) {
	prevHook := ResolveHook
	ResolveHook = resolveFireOnResolve
	defer func() { ResolveHook = prevHook }()

	gs := NewGameState(4, nil, nil)
	installStubHats(gs)
	gs.Active = 1

	perms := map[int]*Permanent{
		0: dummyTriggerPerm(0, "Seat0"),
		1: dummyTriggerPerm(1, "Seat1-AP"),
		2: dummyTriggerPerm(2, "Seat2"),
		3: dummyTriggerPerm(3, "Seat3"),
	}

	fire := &fireOnResolveEffect{
		fire: func(gs *GameState) {
			// Push in arbitrary order — drain must still order APNAP.
			PushTriggeredAbility(gs, perms[3], noopEffect())
			PushTriggeredAbility(gs, perms[0], noopEffect())
			PushTriggeredAbility(gs, perms[1], noopEffect())
			PushTriggeredAbility(gs, perms[2], noopEffect())
		},
	}
	outer := &StackItem{
		Controller: 1,
		Source:     dummyTriggerPerm(1, "Outer"),
		Card:       &Card{Name: "Outer"},
		Effect:     fire,
		Kind:       "triggered",
	}
	PushStackItem(gs, outer)
	ResolveStackTop(gs)

	resolved := stackResolveSources(gs)
	// Expected resolve order: Outer first, then deferred drain in
	// APNAP-LIFO order — Seat0, Seat3, Seat2, Seat1-AP.
	want := []string{"Outer", "Seat0", "Seat3", "Seat2", "Seat1-AP"}
	if len(resolved) != len(want) {
		t.Fatalf("expected %d resolves, got %d: %v", len(want), len(resolved), resolved)
	}
	for i, name := range want {
		if resolved[i] != name {
			t.Fatalf("resolve[%d]: want %q, got %q (full order: %v)",
				i, name, resolved[i], resolved)
		}
	}
}
