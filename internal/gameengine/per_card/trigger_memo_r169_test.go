package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// r169 — the per-Card trigger-handler memo.
//
// EVERY test here starts at the production entry point,
// gameengine.FireCardTrigger, on a real board, and asserts that the
// HANDLER RAN. Not one of them calls triggerTableFor.
//
// That rule is r168's, and it was written because r167 shipped a field
// that never printed: all five of its tests hand-fed the value the
// feature existed to transport, so they asserted on the half that
// worked. A memo is exactly the same shape of risk — a test that asks
// the memo what it remembers will agree with the memo about everything,
// including being wrong.
// ---------------------------------------------------------------------------

// r169Perm puts a permanent of cardName on seat's battlefield with real
// power and toughness.
//
// The P/T is not decoration. addPerm leaves BasePower and
// BaseToughness at zero, and a 0/0 creature dies to state-based actions
// (CR §704.5f) the first time a dispatch resolves and runs SBAs. A test
// built on a 0/0 therefore sees its subject leave the battlefield after
// one event and every later fire find an empty board — which looks
// exactly like a memo that stopped dispatching.
func r169Perm(t *testing.T, gs *gameengine.GameState, seat int, cardName string) *gameengine.Permanent {
	t.Helper()
	p := addPerm(gs, seat, cardName, "creature")
	p.Card.BasePower = 2
	p.Card.BaseToughness = 2
	return p
}

// r169Register installs a trigger handler for the duration of one test
// and removes it afterwards.
//
// The cleanup bumps the generation by hand because it deletes from
// r.onTrigger directly rather than going through a registration method
// (per_card tests must not call Reset(), which would drop every ad-hoc
// registration made by sibling tests). Without the bump, a memo built
// during this test could outlive the handler it describes.
func r169Register(t *testing.T, cardName, event string, h TriggerHandler) {
	t.Helper()
	r := Global()
	r.OnTrigger(cardName, event, h)
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(cardName))
		r.mu.Unlock()
		bumpRegistryGeneration()
	})
}

// TestR169_HandlerFiresOnEveryEventNotJustTheFirst is the central
// claim: the memo must not turn a repeated dispatch into a single one.
// The first fire is a miss and builds the memo; every later fire is a
// hit and must still run the handler.
func TestR169_HandlerFiresOnEveryEventNotJustTheFirst(t *testing.T) {
	ResetTriggerMemoStatsForTest()
	fired := 0
	r169Register(t, "R169 Bell Ringer", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gs := newGame(t, 2)
	bell := r169Perm(t, gs, 0, "R169 Bell Ringer")
	// Bystanders with no handler at all — the case the memo exists for.
	r169Perm(t, gs, 0, "R169 Quiet Bystander")
	r169Perm(t, gs, 1, "R169 Other Bystander")

	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
			"controller_seat": 0,
			"perm":            bell,
		})
	}

	if fired != 3 {
		t.Fatalf("handler should run once per event, ran %d times in 3 events", fired)
	}
	hits, misses := TriggerMemoStats()
	if hits == 0 {
		t.Errorf("expected the memo to be hit after the first event; hits=%d misses=%d", hits, misses)
	}
}

// TestR169_RegistrationAfterTheMemoIsBuiltStillDispatches is THE
// MANDATORY CONTROL TARGET.
//
// The card is on the battlefield with no handler for the first event,
// so a memo recording "this card has nothing" is published. A handler
// is then registered and the same event fired again. If the generation
// key does not invalidate, the stale "nothing" is served forever and
// the handler never runs.
//
// This is the same invalidation path Reset() uses — Reset builds a new
// registry and replays registerDefaults plus every AddResetHook
// callback, and each of those is a registration. Driving it by
// registration rather than by Reset() keeps sibling tests' ad-hoc
// handlers intact; TestR169_ANewRegistryBumpsTheGeneration covers the
// swap itself.
func TestR169_RegistrationAfterTheMemoIsBuiltStillDispatches(t *testing.T) {
	gs := newGame(t, 2)
	latecomer := r169Perm(t, gs, 0, "R169 Latecomer")

	// First event: no handler exists yet. This publishes the memo.
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            latecomer,
	})

	fired := 0
	r169Register(t, "R169 Latecomer", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            latecomer,
	})

	if fired != 1 {
		t.Fatalf("handler registered after the memo was built must still dispatch; ran %d times", fired)
	}
}

// TestR169_RenamedCardDispatchesUnderItsNewName pins the other half of
// the key. Card.Name is mutable — the clone and rename paths rewrite it
// in place, which is why lookupCandidates exists. A memo keyed on the
// Card alone would keep answering for the old name.
func TestR169_RenamedCardDispatchesUnderItsNewName(t *testing.T) {
	fired := 0
	r169Register(t, "R169 True Name", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gs := newGame(t, 2)
	shifter := r169Perm(t, gs, 0, "R169 Blank Shifter")

	// Under its original name the card has no handler; this publishes
	// a memo saying so.
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            shifter,
	})
	if fired != 0 {
		t.Fatalf("handler must not run under the original name, ran %d times", fired)
	}

	shifter.Card.Name = "R169 True Name"

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            shifter,
	})
	if fired != 1 {
		t.Fatalf("after the rename the handler must dispatch; ran %d times", fired)
	}
}

// TestR169_ACardWithNoHandlerStaysSilent is the negative direction: the
// memo must not invent a dispatch. A test suite that only ever asserts
// "the handler ran" passes against a memo that runs everything.
func TestR169_ACardWithNoHandlerStaysSilent(t *testing.T) {
	fired := 0
	r169Register(t, "R169 Registered Elsewhere", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gs := newGame(t, 2)
	stranger := r169Perm(t, gs, 0, "R169 Unregistered Stranger")

	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
			"controller_seat": 0,
			"perm":            stranger,
		})
	}
	if fired != 0 {
		t.Fatalf("a card with no handler must dispatch nothing; ran %d times", fired)
	}
}

// TestR169_ADifferentEventOnTheSameCardIsNotServedTheWrongHandlers
// pins that the memo is a table, not a single answer. The card has a
// handler for one event only; a different event must find nothing even
// though the memo for that card is already warm.
func TestR169_ADifferentEventOnTheSameCardIsNotServedTheWrongHandlers(t *testing.T) {
	etb, dies := 0, 0
	r169Register(t, "R169 One Event Only", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		etb++
	})

	gs := newGame(t, 2)
	p := r169Perm(t, gs, 0, "R169 One Event Only")

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0, "perm": p,
	})
	gameengine.FireCardTrigger(gs, "creature_dies", map[string]interface{}{
		"controller_seat": 0, "perm": p,
	})

	if etb != 1 {
		t.Errorf("permanent_etb handler should have run once, ran %d", etb)
	}
	if dies != 0 {
		t.Errorf("no creature_dies handler is registered; ran %d", dies)
	}
}

// TestR169_ANewRegistryBumpsTheGeneration covers the Reset() swap
// itself. Reset() installs a brand-new registry object; a registry that
// is replaced and repopulated with an identical table is still a
// different registry, so the generation has to move on construction and
// not only in the registration methods.
//
// This one does NOT go through FireCardTrigger, because the thing it
// asserts is a property of registry construction rather than of
// dispatch. It is a supplement to the dispatch tests above, not a
// substitute for them.
func TestR169_ANewRegistryBumpsTheGeneration(t *testing.T) {
	before := RegistryGeneration()
	_ = newRegistry()
	if after := RegistryGeneration(); after == before {
		t.Fatalf("newRegistry must bump the generation; still %d", before)
	}
}

// TestR169_RegistrationBumpsTheGeneration pins the PREMISE of the test
// above as its own assertion (r162's lesson). If registration stopped
// moving the generation, the dispatch test would be the only thing
// standing between a stale memo and a silently dead handler, and a
// reader would have to infer the mechanism from a behaviour failure.
func TestR169_RegistrationBumpsTheGeneration(t *testing.T) {
	before := RegistryGeneration()
	r169Register(t, "R169 Generation Probe", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {})
	if after := RegistryGeneration(); after == before {
		t.Fatalf("OnTrigger must bump the generation; still %d", before)
	}
}

// TestR169_ParentheticalRiderStillFindsTheBaseCard pins that the memo
// preserved the CANDIDATE SEARCH and not just the exact-name lookup.
//
// lookupCandidates exists because the clone and copy paths append
// riders to the name ("Crown of Gondor (Urza copy)"). The memo caches
// the RESULT of that search; a version that cached only an exact-name
// probe would compile, pass every other test here, and silently stop
// dispatching every copied permanent in the game.
func TestR169_ParentheticalRiderStillFindsTheBaseCard(t *testing.T) {
	fired := 0
	r169Register(t, "R169 Base Card", "permanent_etb", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})

	gs := newGame(t, 2)
	copyPerm := r169Perm(t, gs, 0, "R169 Base Card (Urza copy)")

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            copyPerm,
	})
	if fired != 1 {
		t.Fatalf("a renamed copy must still dispatch the base card's handler; ran %d times", fired)
	}
}
