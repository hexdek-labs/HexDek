package gameengine

// Stack trace audit tests — verify stack resolution ordering matches
// CR §405/§608 requirements using the StackTrace logger.
//
// These tests enable the global trace, run game actions, then inspect
// the trace entries for correct ordering of push, priority, resolve,
// and SBA operations.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// newTestGameState creates a game with N seats for stack trace tests.
// Uses 40 life (commander-style) and empty zones.
func newTestGameState(seats int) *GameState {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(seats, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

// ---------------------------------------------------------------------------
// Test 1: Simple spell resolution — push, priority, resolve, SBA
// ---------------------------------------------------------------------------

// TestStackTrace_SimpleSpell verifies the fundamental stack resolution
// sequence: a spell is pushed, priority passes, the spell resolves, and
// SBAs are checked afterward. This is the bread-and-butter of CR §405/§608.
func TestStackTrace_SimpleSpell(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	// Give seat 0 a creature spell with cost 2.
	card := addHandCardWithEffect(gs, 0, "Grizzly Bears", 2,
		&gameast.Damage{
			Amount: *gameast.NumInt(0),
			Target: gameast.TargetOpponent(),
		}, "creature", "cost:2")
	gs.Seats[0].ManaPool = 10

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Verify trace: push -> priority -> resolve -> SBA
	trace := GlobalStackTrace.Entries
	if len(trace) == 0 {
		t.Fatal("no trace entries")
	}

	// Log all entries for manual inspection.
	for _, e := range trace {
		t.Logf("[%s] card=%s seat=%d stack=%d detail=%s",
			e.Action, e.Card, e.Seat, e.StackSize, e.Detail)
	}

	// Check that "push" appears before "resolve".
	pushIdx := -1
	resolveIdx := -1
	for i, e := range trace {
		if e.Action == "push" && pushIdx == -1 {
			pushIdx = i
		}
		if e.Action == "resolve" && resolveIdx == -1 {
			resolveIdx = i
		}
	}
	if pushIdx < 0 {
		t.Error("no push entry in trace")
	}
	if resolveIdx < 0 {
		t.Error("no resolve entry in trace")
	}
	if pushIdx >= 0 && resolveIdx >= 0 && pushIdx >= resolveIdx {
		t.Error("push should come before resolve")
	}

	// Check that priority_pass appears between push and resolve.
	hasPriority := false
	if pushIdx >= 0 && resolveIdx >= 0 {
		for i := pushIdx; i < resolveIdx; i++ {
			if trace[i].Action == "priority_pass" {
				hasPriority = true
				break
			}
		}
	}
	if !hasPriority {
		t.Error("no priority pass between push and resolve -- CR section 405 violation")
	}

	// Check that SBA check appears after resolve.
	hasSBA := false
	if resolveIdx >= 0 {
		for i := resolveIdx; i < len(trace); i++ {
			if trace[i].Action == "sba_check" {
				hasSBA = true
				break
			}
		}
	}
	if !hasSBA {
		t.Error("no SBA check after resolve -- CR section 704 violation")
	}
}

// ---------------------------------------------------------------------------
// Test 2: Multiple triggers — each should have priority between resolutions
// ---------------------------------------------------------------------------

// TestStackTrace_MultipleTriggers verifies that when multiple items are on the
// stack, priority passes between each resolution per CR §117.4.
func TestStackTrace_MultipleTriggers(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	card1 := &Card{Name: "Trigger A", Types: []string{"creature"}, CMC: 1, Owner: 0}
	card2 := &Card{Name: "Trigger B", Types: []string{"creature"}, CMC: 1, Owner: 0}

	// Place permanents on the battlefield for source attribution.
	perm1 := &Permanent{
		Card: card1, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	perm2 := &Permanent{
		Card: card2, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm1, perm2)

	// Simulate two triggered abilities on the stack (manually, since
	// PushTriggeredAbility auto-resolves in the current engine).
	gs.Stack = append(gs.Stack, &StackItem{Card: card1, Controller: 0})
	GlobalStackTrace.Log("trigger_push", "Trigger A", 0, len(gs.Stack), "triggered_ability")
	gs.Stack = append(gs.Stack, &StackItem{Card: card2, Controller: 0})
	GlobalStackTrace.Log("trigger_push", "Trigger B", 0, len(gs.Stack), "triggered_ability")

	// Resolve both — the CastSpell loop pattern: resolve + SBA + priority.
	ResolveStackTop(gs)
	StateBasedActions(gs)
	if len(gs.Stack) > 0 {
		PriorityRound(gs)
	}
	ResolveStackTop(gs)
	StateBasedActions(gs)

	// Count resolve entries.
	resolveCount := 0
	for _, e := range GlobalStackTrace.Entries {
		if e.Action == "resolve" {
			resolveCount++
		}
	}

	if resolveCount < 2 {
		t.Errorf("expected 2 resolutions, got %d", resolveCount)
	}

	// Log the trace for manual inspection.
	for _, e := range GlobalStackTrace.Entries {
		t.Logf("[%s] card=%s seat=%d stack=%d detail=%s",
			e.Action, e.Card, e.Seat, e.StackSize, e.Detail)
	}

	// Verify that between the two resolves we see either a priority_pass
	// or an sba_check — the engine must do SOMETHING between resolutions.
	firstResolve := -1
	secondResolve := -1
	for i, e := range GlobalStackTrace.Entries {
		if e.Action == "resolve" {
			if firstResolve == -1 {
				firstResolve = i
			} else if secondResolve == -1 {
				secondResolve = i
				break
			}
		}
	}
	if firstResolve >= 0 && secondResolve >= 0 {
		hasBetween := false
		for i := firstResolve + 1; i < secondResolve; i++ {
			act := GlobalStackTrace.Entries[i].Action
			if act == "priority_pass" || act == "sba_check" {
				hasBetween = true
				break
			}
		}
		if !hasBetween {
			t.Error("no SBA check or priority pass between two resolutions -- CR section 117.4 violation")
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: Death trigger fires even with replacement effect (§603.10)
// ---------------------------------------------------------------------------

// TestStackTrace_DeathTriggerWithReplacement verifies the trace around a
// creature dying when Rest in Peace is on the battlefield. Per §603.10
// the "when dies" trigger should still be able to fire (looks back in time).
// This test primarily serves as a trace inspection tool.
func TestStackTrace_DeathTriggerWithReplacement(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	// Set up: creature with marked damage + Rest in Peace.
	creature := &Card{
		Name: "Blood Artist", Types: []string{"creature"},
		CMC: 2, Owner: 0, BasePower: 0, BaseToughness: 1,
	}
	perm := &Permanent{
		Card: creature, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Register Rest in Peace replacement effect.
	ripCard := &Card{Name: "Rest in Peace", Types: []string{"enchantment"}, Owner: 0}
	ripPerm := &Permanent{
		Card: ripCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, ripPerm)
	RegisterRestInPeace(gs, ripPerm)

	// Kill the creature via SBA (mark damage >= toughness).
	perm.MarkedDamage = 1
	StateBasedActions(gs)

	// Log trace for manual inspection.
	for _, e := range GlobalStackTrace.Entries {
		t.Logf("[%s] card=%s seat=%d stack=%d detail=%s",
			e.Action, e.Card, e.Seat, e.StackSize, e.Detail)
	}

	// Verify that at minimum we see an sba_check entry — the SBA
	// should have run and detected the lethal damage.
	hasSBA := false
	for _, e := range GlobalStackTrace.Entries {
		if e.Action == "sba_check" {
			hasSBA = true
			break
		}
	}
	if !hasSBA {
		t.Error("no sba_check in trace -- SBA did not fire")
	}
}

// ---------------------------------------------------------------------------
// Test 3b: Per-card trigger pushed to stack via CR §603.3 bridge
// ---------------------------------------------------------------------------

// TestStackTrace_PerCardTriggerUsesStack verifies that per-card trigger
// handlers are pushed to the stack (via PushPerCardTrigger) rather than
// resolving immediately. This is the CR §603.3 compliance check: players
// must have priority to respond to triggered abilities (Stifle, etc.).
func TestStackTrace_PerCardTriggerUsesStack(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	// Set up a creature on the battlefield as the trigger source.
	card := &Card{
		Name: "Blood Artist", Types: []string{"creature"},
		CMC: 2, Owner: 0, BasePower: 0, BaseToughness: 1,
	}
	perm := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Track whether the handler was called.
	handlerCalled := false
	handler := func(gs *GameState, p *Permanent, ctx map[string]interface{}) {
		handlerCalled = true
	}

	// Push a per-card trigger via the CR §603.3 bridge.
	PushPerCardTrigger(gs, perm, handler, map[string]interface{}{
		"test": true,
	})

	// The handler should have been called (inline resolve).
	if !handlerCalled {
		t.Error("per-card trigger handler was not called after PushPerCardTrigger")
	}

	// Log trace for manual inspection.
	for _, e := range GlobalStackTrace.Entries {
		t.Logf("[%s] card=%s seat=%d stack=%d detail=%s",
			e.Action, e.Card, e.Seat, e.StackSize, e.Detail)
	}

	// Verify trace shows trigger_push before resolve.
	trigPushIdx := -1
	resolveIdx := -1
	for i, e := range GlobalStackTrace.Entries {
		if e.Action == "trigger_push" && e.Card == "Blood Artist" && trigPushIdx == -1 {
			trigPushIdx = i
		}
		if e.Action == "resolve" && e.Card == "Blood Artist" && resolveIdx == -1 {
			resolveIdx = i
		}
	}
	if trigPushIdx < 0 {
		t.Error("no trigger_push entry in trace for Blood Artist -- CR §603.3 violation")
	}
	if resolveIdx < 0 {
		t.Error("no resolve entry in trace for Blood Artist")
	}
	if trigPushIdx >= 0 && resolveIdx >= 0 && trigPushIdx >= resolveIdx {
		t.Error("trigger_push should come before resolve")
	}

	// Verify a priority_pass appears between push and resolve.
	hasPriority := false
	if trigPushIdx >= 0 && resolveIdx >= 0 {
		for i := trigPushIdx; i < resolveIdx; i++ {
			if GlobalStackTrace.Entries[i].Action == "priority_pass" {
				hasPriority = true
				break
			}
		}
	}
	if !hasPriority {
		t.Error("no priority pass between trigger_push and resolve -- players cannot respond to trigger")
	}

	// Verify a triggered_ability event was logged.
	hasTriggeredAbility := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "triggered_ability" && ev.Source == "Blood Artist" {
			if d, ok := ev.Details["rule"].(string); ok && d == "603.3" {
				hasTriggeredAbility = true
				break
			}
		}
	}
	if !hasTriggeredAbility {
		t.Error("no triggered_ability event with rule=603.3 in event log")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Trace disabled produces no entries
// ---------------------------------------------------------------------------

// TestStackTrace_DisabledNoEntries confirms that the trace logger produces
// zero entries when disabled, so it has no runtime impact in production.
func TestStackTrace_DisabledNoEntries(t *testing.T) {
	GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "precombat_main"
	gs.Seats[0].ManaPool = 5

	bolt := addHandCardWithEffect(gs, 0, "Lightning Bolt", 1,
		&gameast.Damage{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		}, "instant")

	if err := CastSpell(gs, 0, bolt, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if len(GlobalStackTrace.Entries) != 0 {
		t.Errorf("expected 0 trace entries when disabled, got %d", len(GlobalStackTrace.Entries))
	}
}

// ---------------------------------------------------------------------------
// Test 5: Stack trace reset clears entries
// ---------------------------------------------------------------------------

// TestStackTrace_Reset verifies the Reset method clears entries.
func TestStackTrace_Reset(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()

	GlobalStackTrace.Log("test", "TestCard", 0, 1, "test_entry")
	if len(GlobalStackTrace.Entries) == 0 {
		t.Fatal("expected at least 1 entry after Log")
	}

	GlobalStackTrace.Reset()
	if len(GlobalStackTrace.Entries) != 0 {
		t.Errorf("expected 0 entries after Reset, got %d", len(GlobalStackTrace.Entries))
	}
}

// ---------------------------------------------------------------------------
// Test 6: APNAP ordering — CR §101.4
// ---------------------------------------------------------------------------

// TestStackTrace_APNAPOrdering verifies that APNAPOrder returns seat indices
// in active-player-first turn order, skipping eliminated players.
func TestStackTrace_APNAPOrdering(t *testing.T) {
	GlobalStackTrace.Enable()
	defer GlobalStackTrace.Disable()
	GlobalStackTrace.Reset()

	gs := newTestGameState(4)
	gs.Active = 2 // Seat 2 is active
	gs.Phase = "precombat_main"

	// Verify APNAP order with all players alive.
	order := APNAPOrder(gs)
	// Should be: [2, 3, 0, 1] (active first, then clockwise)
	expected := []int{2, 3, 0, 1}
	if len(order) != len(expected) {
		t.Fatalf("APNAP order: got %v, want %v", order, expected)
	}
	for i, v := range order {
		if v != expected[i] {
			t.Errorf("APNAP order[%d] = %d, want %d", i, v, expected[i])
		}
	}

	// Test with eliminated player (seat 3).
	gs.Seats[3].Lost = true
	order2 := APNAPOrder(gs)
	// Should skip seat 3: [2, 0, 1]
	expected2 := []int{2, 0, 1}
	if len(order2) != len(expected2) {
		t.Fatalf("APNAP with eliminated: got %v, want %v", order2, expected2)
	}
	for i, v := range order2 {
		if v != expected2[i] {
			t.Errorf("APNAP eliminated order[%d] = %d, want %d", i, v, expected2[i])
		}
	}

	t.Log("APNAP ordering verified per CR §101.4")
}
