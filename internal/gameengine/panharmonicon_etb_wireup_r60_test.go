package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Panharmonicon (and every other `would_fire_etb_trigger` handler —
// Yarok, Clara Oswald, Cloud Midgar Mercenary, Katara) registers a §614
// replacement effect that should fire each ETB trigger one additional
// time. The handler is fine; it's covered by
// TestRepl_Panharmonicon_DoublesETBTriggers which calls
// FireETBTriggerEvent directly.
//
// The bug surfaced by this R60 sweep: NO PRODUCTION CODE PATH calls
// FireETBTriggerEvent during real ETB processing. The self-trigger
// loops in stack.go:1486-1499 (spell ETB) and etb_dispatch.go:24-38
// (token/flicker ETB) push the trigger exactly once via
// PushTriggeredAbility, ignoring the replacement framework. Same for
// the observer-trigger loop in observer_triggers.go:fireObserverETBTriggers.
//
// Effect: Panharmonicon / Yarok / Cloud / Clara / Katara are silently
// inert in real games — the replacement registry has the right entries
// but nobody asks it the question.
//
// These tests pin the FIXED behavior: with Panharmonicon on the
// battlefield, FirePermanentETBTriggers must push self-ETB triggers
// TWICE, and observer ETB triggers fired through fireObserverETBTriggers
// must likewise double.

// makeETBDrawCreature builds a creature with one ETB triggered ability
// "When ~ enters the battlefield, draw a card". The Effect body is a
// minimal Draw — the tests don't resolve the stack, they just count
// trigger pushes.
func makeETBDrawCreature(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "etb",
				},
				Effect: &gameast.Draw{
					Count:  *gameast.NumInt(1),
					Target: gameast.Filter{Base: "controller"},
				},
			},
		},
	}
}

// makeObserverETBCreature builds a creature with "Whenever another
// creature enters under your control, draw a card" — an observer trigger.
func makeObserverETBCreature(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{
					Event: "etb",
					Actor: &gameast.Filter{
						Base:       "another_creature",
						Quantifier: "you_control",
					},
					Controller: "you",
				},
				Effect: &gameast.Draw{
					Count:  *gameast.NumInt(1),
					Target: gameast.Filter{Base: "controller"},
				},
			},
		},
	}
}

// triggerPushCount returns gs.Flags["_trigger_fires_this_turn"], which
// PushTriggeredAbility increments unconditionally per push (regardless of
// whether the trigger immediately resolves and pops off the stack). The
// resolved-and-popped path is what gs.Stack looks like after
// FirePermanentETBTriggers returns, which is why a Stack-length count
// would read zero even on a successful push.
func triggerPushCount(gs *GameState) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	return gs.Flags["_trigger_fires_this_turn"]
}

// TestPanharmonicon_SelfETB_DoublesWithoutManualFire pins the FIXED
// behavior for self-ETB triggers via FirePermanentETBTriggers.
// Pre-fix: 1 stack item. Post-fix: 2 stack items (Panharmonicon adds +1).
func TestPanharmonicon_SelfETB_DoublesWithoutManualFire(t *testing.T) {
	gs := newFixtureGame(t)

	// Place Panharmonicon and register its replacement.
	panh := addBattlefield(gs, 0, "Panharmonicon", 0, 0, "artifact")
	RegisterPanharmonicon(gs, panh)

	// Enter a creature with a stock "ETB → draw a card" trigger.
	ast := makeETBDrawCreature("Cloudblazer")
	entering := addBattlefieldWithAST(gs, 0, "Cloudblazer", 2, 2, ast, "creature")

	// Top up library so the Draw effect can resolve without an SBA loss.
	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	FirePermanentETBTriggers(gs, entering)
	pushed := triggerPushCount(gs)

	if pushed != 2 {
		t.Fatalf("expected Panharmonicon to double the self-ETB trigger (want 2 pushes), got %d", pushed)
	}
}

// TestPanharmonicon_SelfETB_NoDoublerSingleFire is the negative — when
// Panharmonicon ISN'T on the battlefield, the self-trigger fires exactly
// once. Guards against the loop overshooting and double-firing without
// any replacement in play.
func TestPanharmonicon_SelfETB_NoDoublerSingleFire(t *testing.T) {
	gs := newFixtureGame(t)

	ast := makeETBDrawCreature("Cloudblazer")
	entering := addBattlefieldWithAST(gs, 0, "Cloudblazer", 2, 2, ast, "creature")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	FirePermanentETBTriggers(gs, entering)
	pushed := triggerPushCount(gs)

	if pushed != 1 {
		t.Fatalf("baseline (no Panharmonicon): expected 1 trigger push, got %d", pushed)
	}
}

// TestPanharmonicon_ObserverETB_DoublesWithoutManualFire pins the FIXED
// behavior for OBSERVER ETB triggers (the trigger is on a permanent
// reacting to someone else's ETB). Soul Warden-style.
// Pre-fix: 1 stack item. Post-fix: 2 stack items.
func TestPanharmonicon_ObserverETB_DoublesWithoutManualFire(t *testing.T) {
	gs := newFixtureGame(t)

	panh := addBattlefield(gs, 0, "Panharmonicon", 0, 0, "artifact")
	RegisterPanharmonicon(gs, panh)

	// Soul-Warden-style observer trigger on seat 0.
	observerAST := makeObserverETBCreature("Soul Warden")
	addBattlefieldWithAST(gs, 0, "Soul Warden", 1, 1, observerAST, "creature")

	// A second creature ETBs on seat 0 — the observer should react.
	enteringAST := &gameast.CardAST{Name: "Grizzly Bears"}
	entering := addBattlefieldWithAST(gs, 0, "Grizzly Bears", 2, 2, enteringAST, "creature")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	FirePermanentETBTriggers(gs, entering)
	pushed := triggerPushCount(gs)

	if pushed != 2 {
		t.Fatalf("expected observer ETB trigger to double under Panharmonicon (want 2 pushes), got %d", pushed)
	}
}

// TestPanharmonicon_ObserverETB_NoDoublerSingleFire pins the baseline
// observer-trigger count.
func TestPanharmonicon_ObserverETB_NoDoublerSingleFire(t *testing.T) {
	gs := newFixtureGame(t)

	observerAST := makeObserverETBCreature("Soul Warden")
	addBattlefieldWithAST(gs, 0, "Soul Warden", 1, 1, observerAST, "creature")

	enteringAST := &gameast.CardAST{Name: "Grizzly Bears"}
	entering := addBattlefieldWithAST(gs, 0, "Grizzly Bears", 2, 2, enteringAST, "creature")

	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	FirePermanentETBTriggers(gs, entering)
	pushed := triggerPushCount(gs)

	if pushed != 1 {
		t.Fatalf("baseline (no Panharmonicon): expected 1 observer trigger push, got %d", pushed)
	}
}

// TestPanharmonicon_OpponentETB_NotDoubled is the opponent-control
// guard: Panharmonicon doubles triggers on permanents YOU control, not
// on opponent permanents.
func TestPanharmonicon_OpponentETB_NotDoubled(t *testing.T) {
	gs := newFixtureGame(t)

	panh := addBattlefield(gs, 0, "Panharmonicon", 0, 0, "artifact")
	RegisterPanharmonicon(gs, panh)

	// Opponent's creature with ETB trigger.
	ast := makeETBDrawCreature("Mulldrifter")
	entering := addBattlefieldWithAST(gs, 1, "Mulldrifter", 2, 2, ast, "creature")

	addLibrary(gs, 1, "L1", "L2", "L3", "L4")
	FirePermanentETBTriggers(gs, entering)
	pushed := triggerPushCount(gs)

	if pushed != 1 {
		t.Fatalf("opponent's ETB must not be doubled by your Panharmonicon (want 1 push), got %d", pushed)
	}
}
