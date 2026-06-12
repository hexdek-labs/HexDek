package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Regressions for r61 review 03 bug C-2: evalCondition defaulted to
// TRUE for every condition kind without a case arm, so any
// intervening-if / as-long-as gate the engine couldn't evaluate was
// silently treated as satisfied — effects fired that shouldn't, at
// corpus scale (the Iroh phase_scoped_static Issue Log entry is this
// mechanism). The default is now fail-closed (false) and emits a
// "condition_unrecognized" event so the unhandled-kind set is a
// measurable burn-down list.

func ecfc_makeGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.RetainEvents = true
	return gs
}

func TestEvalCondition_UnrecognizedKindFailsClosed(t *testing.T) {
	gs := ecfc_makeGame(t)
	src := &Permanent{Controller: 0, Owner: 0, Card: &Card{Name: "Gate Test Source", Owner: 0, Types: []string{"creature"}}}

	cond := &gameast.Condition{Kind: "definitely_not_a_real_condition_kind_r61"}
	if evalCondition(gs, src, cond) {
		t.Fatal("evalCondition must return false for an unrecognized condition kind (fail-closed); default-true silently grants gated effects (r61 C-2)")
	}

	// The unrecognized kind must be logged so telemetry can burn it down.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind != "condition_unrecognized" {
			continue
		}
		if k, _ := ev.Details["condition_kind"].(string); k == "definitely_not_a_real_condition_kind_r61" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("evalCondition must emit a condition_unrecognized event naming the unhandled kind")
	}
}

// TestEvalCondition_RawKindRetainsDefaultTrue pins the deliberate
// carve-out: "raw" (the parser recognized a conditional but couldn't
// decompose its text) keeps the historical default-true polarity.
// Flipping raw is a separate, larger-blast-radius decision — this test
// exists so that change is made on purpose, not by accident.
func TestEvalCondition_RawKindRetainsDefaultTrue(t *testing.T) {
	gs := ecfc_makeGame(t)
	src := &Permanent{Controller: 0, Owner: 0, Card: &Card{Name: "Raw Test Source", Owner: 0, Types: []string{"creature"}}}

	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"some unparseable rider text"}}
	if !evalCondition(gs, src, cond) {
		t.Fatal("evalCondition(raw) must stay true — the raw carve-out is intentional (see evalCondition doc comment)")
	}
}

// TestEvalCondition_NilConditionStaysTrue — no condition means no gate.
func TestEvalCondition_NilConditionStaysTrue(t *testing.T) {
	gs := ecfc_makeGame(t)
	if !evalCondition(gs, nil, nil) {
		t.Fatal("evalCondition(nil) must remain true — absence of a condition is not a gate")
	}
}
