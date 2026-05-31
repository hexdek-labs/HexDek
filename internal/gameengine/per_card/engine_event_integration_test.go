package per_card

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// engine_event_integration_test.go — engine-event integration test
// surface (PR #887 follow-up).
//
// The doc at docs/engine-event-registry.md enumerated the engine's
// 79-distinct-FireCardTrigger surface but the only regression
// pressure on the per_card handler base is the per-card unit tests
// (~one handler per file). Cross-handler regressions — a sibling
// refactor introducing a panic into a handler whose own test
// doesn't drive the right path — go uncaught until production /
// Loki fuzz / Goldilocks discovers them.
//
// This file walks EVERY (cardName, eventName) handler registration
// in the per_card global registry and fires the canonical event
// with a kitchen-sink ctx + a stub permanent of the registered
// card on seat 0's battlefield. Panics are recovered and logged;
// the test asserts zero panics across the full base.
//
// The baseline (registered pair count + zero panics) is pinned so
// any future regression introducing a panic into a previously-
// quiet handler trips this test immediately, regardless of whether
// that card has its own dedicated test driving the panic path.
//
// Cost: ~1ms per pair × 1189 pairs ≈ 1.2s wall-clock. Acceptable
// for the engine-test budget.

// engineEventIntegrationTestStub is a stub Permanent for the
// integration walk. Built with a minimal Card carrying just the
// name + a `creature` type so cardHasType / IsCreature checks pass
// (the registry uses normalizeName so the exact display name has
// to match the registered key — passing it through the
// addPermanent helper preserves the lookup contract).
func engineEventIntegrationTestStub(gs *gameengine.GameState, cardName string, seat int) *gameengine.Permanent {
	c := &gameengine.Card{
		Name:    cardName,
		Owner:   seat,
		Types:   []string{"creature"},
		Colors:  []string{},
		AST:     &gameast.CardAST{Name: cardName},
	}
	p := &gameengine.Permanent{
		Card:       c,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// engineEventIntegrationKitchenSinkCtx builds a ctx map populated
// with every key any handler in the per_card base might read,
// pointing perm-typed keys at the supplied stub perm + card-typed
// keys at its Card. Handlers that gate on a specific key shape
// will either find what they need or skip cleanly; handlers that
// happen to read an unrelated key will get a sensible default
// rather than a nil-deref panic.
//
// The key list is empirically derived from the OnTrigger ctx-read
// audit (see CLAUDE.md per_card ctx-key audit waves 1-5 Resolved
// row, 2026-05-30). New ctx keys added by future per_card handlers
// should be added here too — the test's value comes from coverage
// breadth.
func engineEventIntegrationKitchenSinkCtx(perm *gameengine.Permanent, seat, oppSeat int) map[string]interface{} {
	return map[string]interface{}{
		// perm-typed: every shape a handler might gate on.
		"perm":           perm,
		"permanent":      perm,
		"attacker_perm":  perm,
		"defender_perm":  perm,
		"source_perm":    perm,
		"damager_perm":   perm,
		"target_perm":    perm,
		"src":            perm,
		"entering":       perm,
		"leaving":        perm,
		// card-typed.
		"card":      perm.Card,
		"card_name": perm.Card.Name,
		// seat-typed: kitchen-sink populates every seat key with sensible
		// defaults (perm's seat for controller-side, opponent for
		// target/defender-side).
		"seat":             seat,
		"controller_seat":  seat,
		"controller":       seat,
		"attacker_seat":    seat,
		"source_seat":      seat,
		"discarder_seat":   seat,
		"drawer_seat":      seat,
		"defender_seat":    oppSeat,
		"target_seat":      oppSeat,
		"damaged_seat":     oppSeat,
		"opponent_seat":    oppSeat,
		"active_seat":      seat,
		// numeric.
		"amount":          1,
		"count":           1,
		"damage":          1,
		"life_amount":     1,
		"counter_amount":  1,
		"ability_index":   0,
		"x":               1,
		// phase / step / strings.
		"phase":         "beginning",
		"step":          "upkeep",
		"from_zone":     "battlefield",
		"to_zone":       "graveyard",
		"target_card":   perm.Card.Name,
		"source_card":   perm.Card.Name,
		"counter_kind":  "+1/+1",
		"mana_type":     "U",
		"event":         "synthetic",
		"reason":        "engine_event_integration_test",
		// booleans.
		"is_permanent": false,
		"exiled":       false,
		"madness_exile": false,
	}
}

// TestEngineEventIntegration_NoHandlerPanics_BaselinePin is the
// load-bearing pin for cross-handler regression detection. Walks
// every (cardName, eventName) pair registered in the global
// per_card registry, places a stub permanent of cardName on seat 0,
// fires the canonical event via FireCardTrigger with a kitchen-sink
// ctx, and asserts zero panics across the full base.
//
// On panic: logs the (cardName, eventName, recovered-value) tuple
// and counts toward the t.Errorf assertion at the end. The error
// message lists every panicking pair so a regression PR sees which
// handlers it broke.
//
// Baseline pin: registered_pairs >= 1100 (current count is ~1189;
// floor at 1100 to allow some natural shrinkage from handler
// consolidation without forcing test updates). Handler-count drops
// below 1100 will fail; the threshold can be ratcheted up as the
// per_card base grows.
func TestEngineEventIntegration_NoHandlerPanics_BaselinePin(t *testing.T) {
	regs := Global().EnumerateOnTriggerRegistrations()
	if len(regs) < 1100 {
		t.Errorf("baseline floor: expected >=1100 OnTrigger registrations, got %d — registry may have lost handlers (Reset hook gap, init() removal, etc.)", len(regs))
	}

	// Sort by (cardName, event) for deterministic test output. Any panic
	// reproduces under the same iteration order in a future run.
	sort.Slice(regs, func(i, j int) bool {
		if regs[i].CardName != regs[j].CardName {
			return regs[i].CardName < regs[j].CardName
		}
		return regs[i].Event < regs[j].Event
	})

	type panicReport struct {
		card     string
		event    string
		recovered interface{}
	}
	var panics []panicReport
	pairsWalked := 0
	handlersExercised := 0

	for _, reg := range regs {
		pairsWalked++
		handlersExercised += reg.HandlerCount
		func() {
			// Fresh GS per pair for isolation. Handlers that mutate
			// gs.Seats / gs.Stack / gs.Flags don't pollute the next pair.
			rng := rand.New(rand.NewSource(42))
			gs := gameengine.NewGameState(2, rng, nil)
			gs.Active = 0
			gs.Turn = 1

			perm := engineEventIntegrationTestStub(gs, reg.CardName, 0)
			ctx := engineEventIntegrationKitchenSinkCtx(perm, 0, 1)

			defer func() {
				if rec := recover(); rec != nil {
					panics = append(panics, panicReport{
						card:      reg.CardName,
						event:     reg.Event,
						recovered: rec,
					})
				}
			}()

			gameengine.FireCardTrigger(gs, reg.Event, ctx)
		}()
	}

	t.Logf("engine-event integration walk: %d (cardName, event) pairs walked, %d handlers exercised, %d panics", pairsWalked, handlersExercised, len(panics))

	if len(panics) > 0 {
		// Group by event for easier debugging.
		byEvent := map[string][]panicReport{}
		for _, p := range panics {
			byEvent[p.event] = append(byEvent[p.event], p)
		}
		var events []string
		for ev := range byEvent {
			events = append(events, ev)
		}
		sort.Strings(events)
		for _, ev := range events {
			t.Errorf("event %q: %d panicking handler(s):", ev, len(byEvent[ev]))
			for _, p := range byEvent[ev] {
				t.Errorf("  - %s — recovered=%v", p.card, p.recovered)
			}
		}
	}
}

// TestEngineEventIntegration_RegistryEnumerationIsStable pins the
// enumeration API itself — successive calls return the same set of
// pairs in (modulo iteration ordering) and the same handler counts.
// Guards against a future Registry refactor breaking the introspection
// API the baseline test depends on.
func TestEngineEventIntegration_RegistryEnumerationIsStable(t *testing.T) {
	a := Global().EnumerateOnTriggerRegistrations()
	b := Global().EnumerateOnTriggerRegistrations()
	if len(a) != len(b) {
		t.Errorf("EnumerateOnTriggerRegistrations: successive calls returned different lengths (%d vs %d)", len(a), len(b))
	}
	// Sum handler counts both ways — must match.
	sumA, sumB := 0, 0
	for _, r := range a {
		sumA += r.HandlerCount
	}
	for _, r := range b {
		sumB += r.HandlerCount
	}
	if sumA != sumB {
		t.Errorf("EnumerateOnTriggerRegistrations: handler-count sums differ between calls (%d vs %d)", sumA, sumB)
	}
}

// TestEngineEventIntegration_RegistryNonEmpty pins that the
// integration walk has SOMETHING to walk. Catches a registry-reset
// regression that wipes handlers before the test runs.
func TestEngineEventIntegration_RegistryNonEmpty(t *testing.T) {
	regs := Global().EnumerateOnTriggerRegistrations()
	if len(regs) == 0 {
		t.Fatalf("EnumerateOnTriggerRegistrations returned 0 pairs — registry was wiped before the test (Reset hook gap, init() removal, etc.)")
	}
	// Distinct event count must be reasonable — the doc audit found 66
	// canonical events listened-to, so the floor is conservative at 50
	// to allow some natural consolidation.
	events := map[string]bool{}
	for _, r := range regs {
		events[r.Event] = true
	}
	if len(events) < 50 {
		t.Errorf("distinct canonical events with handlers: got %d, expected >= 50 (per engine-event-registry.md §1)", len(events))
	}
	// Surface a few high-traffic events in the test output for sanity.
	// Note: OnTrigger stores under the post-NormalizeEventSingle key,
	// so the parser-spelling names get rewritten —
	// creature_attacks → attack, combat_damage_player →
	// deals_combat_damage, creature_dies → die. The check uses the
	// stored canonicals.
	highTraffic := []string{"permanent_etb", "attack", "spell_cast", "deals_combat_damage", "die", "permanent_ltb"}
	for _, ev := range highTraffic {
		if !events[ev] {
			t.Errorf("high-traffic canonical event %q has zero handlers — registry consolidation may have over-collapsed an alias", ev)
		}
	}
	_ = fmt.Sprintf // keep fmt import (used in panic-report formatting)
}
