package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Synthetic AST fixtures for confidence-routing tests. Each builds a
// minimal Ability that exercises a specific score band:
//
//   - confidentStatic   → score 1.0 (structured anthem-like modkind)
//   - boundaryStatic    → score 0.5 (single -0.5 mod-kind penalty)
//   - lowConfStatic     → score 0.2 (stacked -0.5 mod + -0.3 cond)
//   - confidentTriggered→ score 1.0 (structured Effect)
//   - lowConfTriggered  → score 0.2 (UnknownEffect + fallback InterveningIf)

func confidentStaticFixture() *gameast.Static {
	return &gameast.Static{Modification: &gameast.Modification{ModKind: "anthem"}}
}

func boundaryStaticFixture() *gameast.Static {
	return &gameast.Static{Modification: &gameast.Modification{ModKind: "parsed_tail"}}
}

func lowConfStaticFixture() *gameast.Static {
	return &gameast.Static{
		Modification: &gameast.Modification{ModKind: "parsed_effect_residual"},
		Condition:    &gameast.Condition{Kind: "conditional"},
	}
}

func confidentTriggeredFixture() *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "etb"},
		Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}},
	}
}

func lowConfTriggeredFixture() *gameast.Triggered {
	return &gameast.Triggered{
		Trigger:       gameast.Trigger{Event: "etb"},
		Effect:        &gameast.UnknownEffect{RawText: "do something inscrutable"},
		InterveningIf: &gameast.Condition{Kind: "if"},
	}
}

func newConfidenceTestGS() *GameState {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func newConfidenceTestSrc(gs *GameState) *Permanent {
	card := &Card{Name: "Test Source", Owner: 0, Types: []string{"creature"}}
	p := &Permanent{Card: card, Controller: 0, Owner: 0, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return p
}

// ---------------------------------------------------------------------------
// IsLowConfidenceAbility — per-band predicates.
// ---------------------------------------------------------------------------

func TestIsLowConfidenceAbility_NilFailsOpen(t *testing.T) {
	if IsLowConfidenceAbility(nil) {
		t.Errorf("nil ability: want fail-open (false), got true")
	}
}

func TestIsLowConfidenceAbility_HighScorePassesGate(t *testing.T) {
	if IsLowConfidenceAbility(confidentStaticFixture()) {
		t.Errorf("score=1.0 Static must NOT be low-confidence")
	}
	if IsLowConfidenceAbility(confidentTriggeredFixture()) {
		t.Errorf("score=1.0 Triggered must NOT be low-confidence")
	}
}

func TestIsLowConfidenceAbility_BoundaryPassesGate(t *testing.T) {
	// score=0.5 boundary — at-or-above threshold (0.5) means NOT low.
	if IsLowConfidenceAbility(boundaryStaticFixture()) {
		t.Errorf("score=0.5 boundary Static must NOT be low-confidence (gate is at-or-above)")
	}
}

func TestIsLowConfidenceAbility_StackedFallbackGated(t *testing.T) {
	// score=0.2 from stacked -0.5 + -0.3 — below 0.5 threshold.
	if !IsLowConfidenceAbility(lowConfStaticFixture()) {
		t.Errorf("score=0.2 stacked Static must be low-confidence")
	}
	if !IsLowConfidenceAbility(lowConfTriggeredFixture()) {
		t.Errorf("score=0.2 stacked Triggered must be low-confidence")
	}
}

// ---------------------------------------------------------------------------
// IsLowConfidenceModificationEffect — engine-effect variant.
// ---------------------------------------------------------------------------

func TestIsLowConfidenceModificationEffect_NilSafe(t *testing.T) {
	if IsLowConfidenceModificationEffect(nil) {
		t.Errorf("nil ModEffect: want false")
	}
}

func TestIsLowConfidenceModificationEffect_StructuredKindIsConfident(t *testing.T) {
	e := &gameast.ModificationEffect{ModKind: "investigate"}
	if IsLowConfidenceModificationEffect(e) {
		t.Errorf("structured 'investigate' must NOT be low-confidence")
	}
}

func TestIsLowConfidenceModificationEffect_FallbackKindNoArgs(t *testing.T) {
	e := &gameast.ModificationEffect{ModKind: "parsed_effect_residual"}
	if !IsLowConfidenceModificationEffect(e) {
		t.Errorf("fallback ModKind with no args: want low-confidence")
	}
}

func TestIsLowConfidenceModificationEffect_FallbackKindRawStringArg(t *testing.T) {
	// A single string arg is treated as raw payload — low confidence.
	e := &gameast.ModificationEffect{
		ModKind: "parsed_tail",
		Args:    []interface{}{"the rest of the oracle text we couldn't parse"},
	}
	if !IsLowConfidenceModificationEffect(e) {
		t.Errorf("fallback ModKind with raw string arg: want low-confidence")
	}
}

func TestIsLowConfidenceModificationEffect_FallbackKindStructuredArgs(t *testing.T) {
	// Args that include a Filter or nested structure indicate the
	// parser DID produce semantic payload alongside the fallback kind
	// label — don't gate these out.
	e := &gameast.ModificationEffect{
		ModKind: "parsed_effect_residual",
		Args: []interface{}{
			&gameast.Filter{Base: "creature"},
			"more text",
		},
	}
	if IsLowConfidenceModificationEffect(e) {
		t.Errorf("fallback ModKind WITH structured args must NOT gate out")
	}
}

// ---------------------------------------------------------------------------
// LogLowConfidenceFallback — event shape + permanent flag bumps.
// ---------------------------------------------------------------------------

func eventsByKind(gs *GameState, kind string) []Event {
	var out []Event
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func TestLogLowConfidenceFallback_EmitsStructuredEvent(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)
	ab := lowConfStaticFixture()

	LogLowConfidenceFallback(gs, src, ab, "test_call_site")

	events := eventsByKind(gs, "low_confidence_fallback")
	if len(events) != 1 {
		t.Fatalf("want 1 low_confidence_fallback event, got %d", len(events))
	}
	ev := events[0]
	if ev.Source != "Test Source" {
		t.Errorf("event Source: want %q, got %q", "Test Source", ev.Source)
	}
	if ev.Seat != 0 {
		t.Errorf("event Seat: want 0, got %d", ev.Seat)
	}
	if ev.Details["where"] != "test_call_site" {
		t.Errorf("event Details[where]: want %q, got %v", "test_call_site", ev.Details["where"])
	}
	// Score should be 0.2 (stacked penalties).
	score, ok := ev.Details["score"].(float64)
	if !ok {
		t.Fatalf("event Details[score]: want float64, got %T", ev.Details["score"])
	}
	if score >= LowConfidenceFallbackThreshold {
		t.Errorf("logged score %v not below threshold %v", score, LowConfidenceFallbackThreshold)
	}
	// Reasons should include both penalty labels.
	reasons, ok := ev.Details["reasons"].([]string)
	if !ok {
		t.Fatalf("event Details[reasons]: want []string, got %T", ev.Details["reasons"])
	}
	if len(reasons) != 2 {
		t.Errorf("want 2 reasons, got %v", reasons)
	}
	// Permanent flag bump.
	if src.Flags["parser_gap"] != 1 {
		t.Errorf("src.Flags[parser_gap]: want 1, got %d", src.Flags["parser_gap"])
	}
	if src.Flags["low_confidence_fallback"] != 1 {
		t.Errorf("src.Flags[low_confidence_fallback]: want 1, got %d", src.Flags["low_confidence_fallback"])
	}
}

// ---------------------------------------------------------------------------
// RouteAbilityWithConfidence — high-level gating helper.
// ---------------------------------------------------------------------------

func TestRouteAbilityWithConfidence_ConfidentRoutes(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)

	called := 0
	routed := RouteAbilityWithConfidence(gs, src, confidentStaticFixture(), "test",
		func(_ gameast.Ability) { called++ })

	if !routed {
		t.Errorf("confident ability: RouteAbilityWithConfidence returned false")
	}
	if called != 1 {
		t.Errorf("structured callback should fire once, got %d", called)
	}
	if len(eventsByKind(gs, "low_confidence_fallback")) != 0 {
		t.Errorf("confident ability should NOT emit fallback event")
	}
}

func TestRouteAbilityWithConfidence_LowConfBypassed(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)

	called := 0
	routed := RouteAbilityWithConfidence(gs, src, lowConfStaticFixture(), "test",
		func(_ gameast.Ability) { called++ })

	if routed {
		t.Errorf("low-conf ability: RouteAbilityWithConfidence returned true (should bypass)")
	}
	if called != 0 {
		t.Errorf("structured callback must NOT fire on low-conf ability, got %d calls", called)
	}
	if len(eventsByKind(gs, "low_confidence_fallback")) != 1 {
		t.Errorf("want exactly 1 fallback event, got %d", len(eventsByKind(gs, "low_confidence_fallback")))
	}
}

func TestRouteAbilityWithConfidence_NilCallbackSafe(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)
	// Should not panic with nil structured callback.
	routed := RouteAbilityWithConfidence(gs, src, confidentStaticFixture(), "test", nil)
	if !routed {
		t.Errorf("nil callback + confident ability: routed should still be true")
	}
}

// ---------------------------------------------------------------------------
// resolveModificationEffect — wired gate behaviour.
// ---------------------------------------------------------------------------

func TestResolveModificationEffect_LowConfidenceSkipped(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)

	e := &gameast.ModificationEffect{
		ModKind: "parsed_effect_residual",
		Args:    []interface{}{"untyped tail text"},
	}
	resolveModificationEffect(gs, src, e)

	// Should NOT see the default-branch's "modification_effect" event
	// (the gate fired BEFORE the switch).
	if len(eventsByKind(gs, "modification_effect")) != 0 {
		t.Errorf("low-conf ModificationEffect should not reach the default switch branch")
	}
	if len(eventsByKind(gs, "low_confidence_fallback")) != 1 {
		t.Errorf("want 1 low_confidence_fallback event, got %d",
			len(eventsByKind(gs, "low_confidence_fallback")))
	}
	if src.Flags["low_confidence_fallback"] != 1 {
		t.Errorf("low_confidence_fallback flag should be bumped on src")
	}
}

func TestResolveModificationEffect_StructuredKindStillRoutes(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)

	// Use "phase_out_self" — a structured branch that flips a flag.
	e := &gameast.ModificationEffect{ModKind: "phase_out_self"}
	resolveModificationEffect(gs, src, e)

	if src.Flags["phased_out"] != 1 {
		t.Errorf("phase_out_self structured branch must still execute (flag not set)")
	}
	if len(eventsByKind(gs, "low_confidence_fallback")) != 0 {
		t.Errorf("structured kind must NOT trigger low_confidence_fallback")
	}
}

func TestResolveModificationEffect_FallbackKindWithStructuredArgsRoutes(t *testing.T) {
	gs := newConfidenceTestGS()
	gs.RetainEvents = true
	src := newConfidenceTestSrc(gs)

	// Fallback ModKind but with a structured arg — the engine should
	// route through the default branch (parser_gap log) rather than
	// the confidence-gate skip.
	e := &gameast.ModificationEffect{
		ModKind: "parsed_tail",
		Args: []interface{}{
			&gameast.Filter{Base: "creature"},
			"trailing text",
		},
	}
	resolveModificationEffect(gs, src, e)

	if len(eventsByKind(gs, "low_confidence_fallback")) != 0 {
		t.Errorf("fallback ModKind WITH structured args must NOT trigger the engine gate")
	}
	// The default branch will fire parser_gap; we don't check the
	// exact count here — just that the gate didn't pre-empt.
}

// ---------------------------------------------------------------------------
// Threshold sanity — pins the constant against silent drift.
// ---------------------------------------------------------------------------

func TestLowConfidenceFallbackThreshold_PinValue(t *testing.T) {
	// Engine fallback threshold is intentionally LOWER than the default
	// freya/hat threshold (0.7) — the engine is the last line of
	// defence and should be more permissive. Pin the relationship.
	if LowConfidenceFallbackThreshold >= gameast.DefaultConfidenceThreshold {
		t.Errorf("LowConfidenceFallbackThreshold (%.2f) must be < gameast.DefaultConfidenceThreshold (%.2f)",
			LowConfidenceFallbackThreshold, gameast.DefaultConfidenceThreshold)
	}
	if LowConfidenceFallbackThreshold != 0.5 {
		t.Errorf("LowConfidenceFallbackThreshold drifted to %.2f (expected 0.5)",
			LowConfidenceFallbackThreshold)
	}
}
