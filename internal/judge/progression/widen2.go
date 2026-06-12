package progression

// widen2.go — PROGRESSION phase 2 (r63): wider trigger vocabulary +
// deeper check classes.
//
// New vocabulary (same independence contract — expectations from AST +
// raw oracle text, firings observed as state deltas):
//
//	upkeep    "at the beginning of your/each upkeep"   — 828 corpus
//	end_step  "at the beginning of your/each end step" — 466
//	ltb       "when this leaves the battlefield"       —  24
//
// New check axes:
//
//	CONTROLLER GATE — "your upkeep" triggers must fire on the
//	controller's upkeep and must NOT fire on an opponent's upkeep (the
//	phantom axis phase 1 could not reach; controller-keyed gates have a
//	prior history of silent short-circuits, goldilocks r37).
//
//	INTERVENING-IF (CR §603.4) — synthetic-only: the corpus carries ZERO
//	intervening_if nodes (parser gap, reported) and the engine ignores
//	the field entirely (engine gap, fixed this pass at the trigger-fire
//	chokepoints). The scenario suite pins: condition false at fire ⇒ no
//	fire; true at fire but false at resolution (flipped inside an open
//	trigger batch) ⇒ no effect.
//
//	DELAYED (CR §603.7) — a registered next-end-step delayed trigger
//	fires exactly once at the next end step and never again.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// phaseScope classifies an upkeep/end-step trigger's controller scope
// from the raw oracle text: "your" / "each" / "" (unknown → skip).
func phaseScope(raw string) string {
	r := strings.ToLower(raw)
	switch {
	case strings.Contains(r, "your upkeep"), strings.Contains(r, "your end step"):
		return "your"
	case strings.Contains(r, "each upkeep"), strings.Contains(r, "each player's upkeep"),
		strings.Contains(r, "each end step"), strings.Contains(r, "each player's end step"):
		return "each"
	}
	return ""
}

// InScopePhaseTrigger reports whether t is an in-scope upkeep/end-step
// trigger and returns (step, scope).
func InScopePhaseTrigger(t *gameast.Triggered) (string, string, bool) {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return "", "", false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil {
		return "", "", false
	}
	var step string
	switch {
	case tr.Event == "phase" && tr.Phase == "upkeep", tr.Event == "upkeep":
		step = "upkeep"
	case tr.Event == "phase" && (tr.Phase == "end_step" || tr.Phase == "end"), tr.Event == "end_step":
		step = "end_step"
	default:
		return "", "", false
	}
	// Conditional phase triggers ("at the beginning of each end step,
	// IF ...") lose their condition in the parse (intervening-if parser
	// gap, see report) — unverifiable either way.
	if strings.Contains(strings.ToLower(t.Raw), " if ") {
		return "", "", false
	}
	scope := phaseScope(t.Raw)
	if scope == "" {
		return "", "", false
	}
	return step, scope, true
}

// InScopeLTBTrigger reports whether t is a bare self-LTB trigger.
func InScopeLTBTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	switch tr.Event {
	case "ltb", "leaves_battlefield", "leave_battlefield":
	default:
		return false
	}
	raw := strings.ToLower(t.Raw)
	if strings.Contains(raw, "other") || strings.Contains(raw, "another") ||
		strings.Contains(raw, "you control") {
		return false
	}
	return true
}

// CheckPhaseTrigger runs FIRE (controller's step, plus opponent's step
// for "each" scope) and CONTROLLER-GATE PHANTOM (opponent's step for
// "your" scope) scenarios.
func CheckPhaseTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	step, scope, ok := InScopePhaseTrigger(t)
	if !ok {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}
	phase := "beginning"
	if step == "end_step" {
		phase = "ending"
	}

	var findings []*Finding
	run := func(activeSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		gs.Active = activeSeat
		before := outcome.Snap(gs)
		gameengine.FirePhaseTriggers(gs, phase, step)
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	// FIRE: the controller's own step — every scope fires.
	actual := run(0)
	matched := false
	for _, exp := range expectedSet {
		if actual.Equal(exp) {
			matched = true
		}
	}
	if !matched {
		findings = append(findings, &Finding{
			CardName: cardName, Event: step + "/" + scope, Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// Opponent's step: "your" must be SILENT (controller gate), "each"
	// must fire again.
	actualOpp := run(1)
	if scope == "your" {
		if !actualOpp.Equal(outcome.NewDelta()) {
			findings = append(findings, &Finding{
				CardName: cardName, Event: step + "/" + scope, Check: "controller_gate_phantom",
				Expected: "no change (opponent's " + step + ")", Actual: actualOpp.String(), Raw: t.Raw,
			})
		}
	} else {
		matchedOpp := false
		for _, exp := range expectedSet {
			if actualOpp.Equal(exp) {
				matchedOpp = true
			}
		}
		if !matchedOpp {
			findings = append(findings, &Finding{
				CardName: cardName, Event: step + "/" + scope, Check: "each_scope_fire",
				Expected: describeSet(expectedSet), Actual: actualOpp.String(), Raw: t.Raw,
			})
		}
	}
	return findings, true
}

// CheckLTBTrigger runs FIRE (bearer bounced) and PHANTOM (vanilla
// bounced) scenarios for self-LTB triggers.
func CheckLTBTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeLTBTrigger(t) {
		return nil, false
	}
	spec := progressionSpec()
	// LTB effects resolve after the bearer left — post-departure board.
	effSpec := spec
	effSpec.SrcIsCreature = false
	expectedSet, ok := expectedFireSet(effSpec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	// FIRE: bounce the bearer (battlefield → hand: bf-1 pow-4 tough-4
	// hand+1 movement subtracted).
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	before := outcome.Snap(gs)
	gameengine.BouncePermanent(gs, bearer, nil, "hand")
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	subtractBounce(actual, 0, 4)
	matched := false
	for _, exp := range expectedSet {
		if actual.Equal(exp) {
			matched = true
		}
	}
	if !matched {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "ltb", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// PHANTOM: a vanilla bystander bounces instead.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	v := vanillaPerm(gs2, 0)
	place(gs2, v)
	before2 := outcome.Snap(gs2)
	gameengine.BouncePermanent(gs2, v, nil, "hand")
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	subtractBounce(actual2, 0, 1)
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "ltb", Check: "phantom",
			Expected: "no change (another permanent left)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	return findings, true
}

// subtractBounce removes the bare battlefield→hand movement of a
// pt-bodied creature owned by seat from the delta.
func subtractBounce(d *outcome.Delta, seat, pt int) {
	d.BattlefieldBySeat[seat]++
	if d.BattlefieldBySeat[seat] == 0 {
		delete(d.BattlefieldBySeat, seat)
	}
	d.HandBySeat[seat]--
	if d.HandBySeat[seat] == 0 {
		delete(d.HandBySeat, seat)
	}
	d.PowerSum += pt
	d.ToughSum += pt
}
