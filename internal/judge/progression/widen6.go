package progression

// widen6.go — PROGRESSION phase 3d (r63): the next band from the phase-3c
// report's still-unmeasured map. Same independence contract: expectations
// derive from the AST trigger + raw oracle wording, firings observed purely
// as state deltas, and the engine's dispatch is only ever DRIVEN as the
// stimulus — never consulted to FORM an expectation.
//
//	turned_face_up  "when this creature is turned face up," (109 corpus) —
//	                morph / megamorph / disguise / cloak / manifest. Stimulus:
//	                the bearer transitions face-down → face-up through the
//	                engine's TurnFaceUp chokepoint (CR §702.36e).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// CheckAny3d chains the phase-3d families; CheckAny calls it after 3c.
func checkAnyPhase3d(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckTurnedFaceUpTrigger(cardName, t); ran {
		return f, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// turned_face_up — "when this creature is turned face up,"
// ---------------------------------------------------------------------------

// InScopeTurnedFaceUpTrigger keeps self "is turned face up" wordings only.
// The parser drops actor phrases, so observer forms ("whenever a permanent
// is turned face up") parse to the same bare event — those stay out.
func InScopeTurnedFaceUpTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "turned_face_up" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	return strings.Contains(raw, "this creature is turned face up") ||
		strings.Contains(raw, "this permanent is turned face up") ||
		strings.Contains(raw, "~ is turned face up")
}

// CheckTurnedFaceUpTrigger: FIRE — the bearer (a face-down 2/2) is turned
// face up through the engine's TurnFaceUp chokepoint; the trigger must
// produce its effect's delta exactly once. PHANTOM — a vanilla face-down
// bystander is turned face up instead; the bearer's trigger must stay
// silent. The bearer is pinned to a 2/2 so the face-down → face-up
// transition is P/T-neutral (a face-down creature is a 2/2 by CR §707.2),
// isolating the trigger's own delta.
func CheckTurnedFaceUpTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeTurnedFaceUpTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "face_up", "turned_face_up") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding

	// FIRE: the bearer is face-down, then turned face up through the engine
	// chokepoint. A face-down creature is a 2/2 (CR §707.2) while the
	// harness source is a 4/4, so the face-down → face-up transition
	// understates the bearer's P/T contribution by exactly 2 versus the
	// 4/4 the effect-expectation assumes — subtract it so only the trigger's
	// own delta remains (mirrors subtractDeath / subtractTap).
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	bearer.Card.FaceDown = true
	before := outcome.Snap(gs)
	gameengine.TurnFaceUp(gs, bearer, "progression-stimulus")
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	actual.PowerSum -= 2
	actual.ToughSum -= 2
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "turned_face_up", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// PHANTOM: a vanilla face-down bystander is turned face up.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	bearer2.Card.FaceDown = false // the bearer stays face-up — its trigger is dormant
	v := vanillaPerm(gs2, 0)
	v.Card.FaceDown = true
	v.Card.BasePower = 2
	v.Card.BaseToughness = 2
	place(gs2, v)
	before2 := outcome.Snap(gs2)
	gameengine.TurnFaceUp(gs2, v, "progression-stimulus")
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "turned_face_up", Check: "phantom",
			Expected: "no change (a bystander was turned face up)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}
