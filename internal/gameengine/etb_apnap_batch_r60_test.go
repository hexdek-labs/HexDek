package gameengine

import (
	"testing"
)

// r60 — regression tests for the CR §603.3b APNAP-batching bypass in the
// FirePermanentETBTriggers cascade. Pre-r60, that helper (which fires
// self-AST ETB triggers + per_card snowflake hooks + observer ETB
// triggers + the "permanent_etb" / "nonland_permanent_etb"
// FireCardTrigger fan-outs) pushed each trigger inline via
// PushTriggeredAbility with no BeginTriggerBatch wrapper. The
// consequence: simultaneous triggers from a single ETB event resolved
// in AST-iteration order instead of being batched, ordered APNAP +
// controller-choice (via OrderTriggersAPNAP + Hat.OrderTriggers), and
// pushed together via PushSimultaneousTriggers.
//
// 13 production call sites bypassed §603.3b: resolve.go (5),
// resolve_helpers.go (7), keywords_batch4.go (1) — every token-mint
// path. Plus the 4 per_card flicker handlers (Brago, Deadeye Navigator,
// Displacer Kitten, Emiel). The stack.go:resolvePermanentSpellETB
// path is the only one that wrapped its own equivalent cascade in a
// batch — so the two ETB code paths had divergent §603.3b conformance.
//
// Fix opens the batch INSIDE FirePermanentETBTriggers so all 13+4
// callers get the §603.3b ordering at once. BeginTriggerBatch is
// re-entrant; the stack.go path's outer wrapper folds into a no-op
// nested frame.

// TestFirePermanentETBTriggers_OpensBatch pins the headline behavior:
// calling FirePermanentETBTriggers from outside any batch frame must
// elevate trigger handling INTO a batch — `triggers_ordered` event in
// the log proves PushSimultaneousTriggers ran (which only happens via
// the batch-drain path). Pre-r60 this event count was 0 because
// PushTriggeredAbility pushed inline.
func TestFirePermanentETBTriggers_OpensBatch(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	// Two observer-ETB creatures owned by seats 0 and 1 (each watches
	// for "another creature enters under your control"). When a new
	// creature enters under either of them, an observer trigger fires.
	addBattlefieldWithAST(gs, 0, "Observer-A", 1, 1, makeObserverETBCreature("Observer-A"), "creature")
	addBattlefieldWithAST(gs, 1, "Observer-B", 1, 1, makeObserverETBCreature("Observer-B"), "creature")

	// New creature enters under seat 0 — its ETB cascade should fire
	// Observer-A's observer trigger (seat 0's, "another creature you
	// control" matches because the entering perm is under seat 0).
	entering := addBattlefieldWithAST(gs, 0, "Newcomer", 2, 2, makeETBDrawCreature("Newcomer"), "creature")

	// Drive the cascade — Newcomer's own ETB self-trigger fires +
	// Observer-A's observer-ETB trigger fires. Two simultaneous
	// triggers from a single event — exactly the §603.3b shape.
	FirePermanentETBTriggers(gs, entering)

	if !eventLogged(gs, "triggers_ordered") {
		t.Errorf("expected at least one 'triggers_ordered' event proving §603.3b batching ran; got events: %v",
			eventKindList(gs))
	}
}

// TestFirePermanentETBTriggers_RespectsOpenBatch pins re-entrancy: when
// the caller has already opened a batch (mirroring the stack.go
// resolvePermanentSpellETB path), FirePermanentETBTriggers must NOT
// drain inside its own frame — it must let the outermost batch drain.
// This is the property that lets the existing stack.go wrapper fold
// into a no-op nested frame.
func TestFirePermanentETBTriggers_RespectsOpenBatch(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	addBattlefieldWithAST(gs, 0, "Observer-A", 1, 1, makeObserverETBCreature("Observer-A"), "creature")
	entering := addBattlefieldWithAST(gs, 0, "Newcomer", 2, 2, makeETBDrawCreature("Newcomer"), "creature")

	// Open an OUTER batch (caller-side, mirrors stack.go's wrapper).
	opened := BeginTriggerBatch(gs)
	if !opened {
		t.Fatalf("BeginTriggerBatch should have opened the outermost frame; another batch was already open")
	}

	FirePermanentETBTriggers(gs, entering)

	// The outermost batch is STILL open. The cascade's triggers must
	// be queued in pendingTriggers, NOT drained, until our caller-side
	// EndTriggerBatch fires.
	if len(gs.pendingTriggers) == 0 {
		t.Errorf("pendingTriggers should be non-empty while the outer batch is still open; cascade drained early")
	}
	if eventLogged(gs, "triggers_ordered") {
		t.Errorf("triggers_ordered fired while outer batch was still open — inner frame drained instead of deferring")
	}

	// Close the outer batch — now triggers_ordered should fire.
	EndTriggerBatch(gs, opened)
	if !eventLogged(gs, "triggers_ordered") {
		t.Errorf("triggers_ordered should have fired on outer-frame close; got: %v",
			eventKindList(gs))
	}
}

// TestFirePermanentETBTriggers_DepthAccountsForReEntrancy guards against
// a defer-leak bug: each BeginTriggerBatch/EndTriggerBatch pair must
// balance. After the cascade returns, the depth counter should be back
// to its pre-call value — otherwise nested cascades would silently
// stack up frames and never drain.
func TestFirePermanentETBTriggers_DepthAccountsForReEntrancy(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	entering := addBattlefieldWithAST(gs, 0, "Solo", 2, 2, makeETBDrawCreature("Solo"), "creature")

	depthBefore := gs.triggerBatchDepth
	FirePermanentETBTriggers(gs, entering)
	depthAfter := gs.triggerBatchDepth

	if depthAfter != depthBefore {
		t.Errorf("trigger batch depth leaked: before=%d after=%d", depthBefore, depthAfter)
	}
}

// eventLogged reports whether at least one event of the given kind
// landed in gs.EventLog. Helper local to this test file.
func eventLogged(gs *GameState, kind string) bool {
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

// eventKindList returns just the kind field of every logged event, for
// readable test-failure messages.
func eventKindList(gs *GameState) []string {
	out := make([]string, 0, len(gs.EventLog))
	for _, e := range gs.EventLog {
		out = append(out, e.Kind)
	}
	return out
}
