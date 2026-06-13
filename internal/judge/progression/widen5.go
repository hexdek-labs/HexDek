package progression

// widen5.go — PROGRESSION phase 3c (r63): the next band from the
// phase-3b report's still-unmeasured map. Same independence contract:
// expectations derive from the AST trigger + raw oracle wording, firings
// are observed purely as state deltas, and the engine's dispatch is only
// ever DRIVEN as the stimulus — never consulted to FORM an expectation.
//
//	becomes_blocked  "whenever this creature becomes blocked," (90 corpus)
//	                 — block-conditional combat triggers. Stimulus: the
//	                 bearer attacks and a blocker is declared against it
//	                 (FireBecomesBlockedTriggers, CR §509.4).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

// CheckAny3c chains the phase-3c families; CheckAny calls it after 3b.
func checkAnyPhase3c(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if f, ran := CheckBecomesBlockedTrigger(cardName, t); ran {
		return f, true
	}
	if f, ran := CheckOrdinalFirstMainTrigger(cardName, t); ran {
		return f, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// beginning_of_ordinal_step — "at the beginning of your first main phase,"
// ---------------------------------------------------------------------------
//
// The parser emits Event="beginning_of_ordinal_step" with an EMPTY Phase
// field — the ordinal ("first main" / "second main" / "next end step" /
// "next upkeep") survives only in the raw clause. The recurring, statically-
// observable subset is "at the beginning of your first main phase," on a
// battlefield permanent (Coalition Relic, Altar of Shadows, Abstract
// Paintmage, Four Knocks, …). The "next end step / next upkeep" wordings are
// one-shot DELAYED triggers (CR §603.7) registered during a spell's
// resolution — not battlefield-permanent triggers, so out of this scope.

// InScopeOrdinalFirstMainTrigger keeps controller-scoped "your first main
// phase" wordings only.
func InScopeOrdinalFirstMainTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	if tr.Event != "beginning_of_ordinal_step" {
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	return strings.Contains(raw, "at the beginning of your first main phase,")
}

// CheckOrdinalFirstMainTrigger: FIRE — the controller's precombat (first)
// main phase boundary fires the trigger; CONTROLLER-GATE PHANTOM — an
// opponent's first main phase must leave the controller's trigger silent.
// Driven through the same FirePhaseTriggers chokepoint the chaos game
// runner uses for the main phase (phase="precombat_main", step="main").
func CheckOrdinalFirstMainTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeOrdinalFirstMainTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "first_main", "precombat_main", "main_phase") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding
	run := func(activeSeat int) *outcome.Delta {
		gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
		bearer.Card.AST = wrapSingle(t)
		gs.Active = activeSeat
		before := outcome.Snap(gs)
		gameengine.FirePhaseTriggers(gs, "precombat_main", "main")
		return outcome.DiffSnapshots(before, outcome.Snap(gs))
	}

	// FIRE: the controller's own first main phase.
	actual := run(0)
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "ordinal_first_main", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// CONTROLLER-GATE PHANTOM: an opponent's first main must be silent.
	actualOpp := run(1)
	if !actualOpp.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "ordinal_first_main", Check: "controller_gate_phantom",
			Expected: "no change (opponent's first main)", Actual: actualOpp.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}

// ---------------------------------------------------------------------------
// becomes_blocked — "whenever this creature becomes blocked,"
// ---------------------------------------------------------------------------

// InScopeBecomesBlockedTrigger keeps self-blocked wordings only. The
// parser drops actor phrases, so "whenever a creature you control becomes
// blocked" parses to the same bare event — those stay out until carried.
// "becomes blocked BY <x>" wordings carry a condition on the blocker
// (flanking, color, …) that the parser drops, so they fail closed too.
func InScopeBecomesBlockedTrigger(t *gameast.Triggered) bool {
	if t == nil || t.Effect == nil || t.InterveningIf != nil {
		return false
	}
	tr := t.Trigger
	if tr.Actor != nil || tr.TargetFilter != nil || tr.Condition != nil ||
		tr.Phase != "" || tr.Controller != "" {
		return false
	}
	switch tr.Event {
	case "becomes_blocked", "blocked":
	default:
		return false
	}
	raw := strings.ToLower(t.Raw)
	if !cleanRider(raw) {
		return false
	}
	// "becomes blocked by …" restricts on the blocker — dropped by the
	// parser, unverifiable in the bare scenario.
	if strings.Contains(raw, "becomes blocked by") {
		return false
	}
	// Effects scoped to "each creature blocking it" / "the blocking
	// creature(s)" scale with the ACTUAL blocker set, which the
	// single-snapshot expectation (computed against the bare spec board's
	// generic creature count) cannot model — the engine is correct, it
	// affects exactly the declared blockers. Same unmodellable-magnitude
	// class as the documented board-scaling / "its counters" gates
	// (Battle-Scarred Goblin "deals 1 damage to each creature blocking it"
	// — PROGRESSION r63c FP). Fail closed.
	if strings.Contains(raw, "blocking") {
		return false
	}
	return strings.Contains(raw, "this creature becomes blocked,") ||
		strings.Contains(raw, "~ becomes blocked,")
}

// CheckBecomesBlockedTrigger: FIRE — the bearer (its controller's only
// creature) attacks and a vanilla blocker is declared against it; the
// "becomes blocked" trigger must produce its effect's delta exactly once.
// PHANTOM — the bearer stays home (summoning-sick) and a vanilla attacker
// is blocked instead; the bearer's trigger must stay silent.
func CheckBecomesBlockedTrigger(cardName string, t *gameast.Triggered) ([]*Finding, bool) {
	if !InScopeBecomesBlockedTrigger(t) {
		return nil, false
	}
	if perCardOwned(cardName, "blocked", "becomes_blocked") {
		return nil, false
	}
	spec := progressionSpec()
	expectedSet, ok := expectedFireSet(spec, t.Effect)
	if !ok {
		return nil, false
	}

	var findings []*Finding

	// FIRE: bearer attacks, then is blocked by a vanilla blocker.
	gs, bearer := outcome.BuildBoardForSpec(spec, cardName)
	bearer.Card.AST = wrapSingle(t)
	bearer.SummoningSick = false
	attackers := gameengine.DeclareAttackers(gs, 0)
	_ = attackers
	blocker := vanillaPerm(gs, 1) // defending seat's blocker
	place(gs, blocker)
	before := outcome.Snap(gs)
	gameengine.FireBecomesBlockedTriggers(gs, bearer)
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	if !matchSet(expectedSet, actual) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "becomes_blocked", Check: "fire",
			Expected: describeSet(expectedSet), Actual: actual.String(), Raw: t.Raw,
		})
	}

	// PHANTOM: a vanilla attacker is blocked; the bearer never attacked,
	// so its "becomes blocked" must not fire.
	gs2, bearer2 := outcome.BuildBoardForSpec(spec, cardName)
	bearer2.Card.AST = wrapSingle(t)
	bearer2.SummoningSick = true
	v := vanillaPerm(gs2, 0)
	v.SummoningSick = false
	place(gs2, v)
	gameengine.DeclareAttackers(gs2, 0)
	blocker2 := vanillaPerm(gs2, 1)
	place(gs2, blocker2)
	before2 := outcome.Snap(gs2)
	gameengine.FireBecomesBlockedTriggers(gs2, v)
	actual2 := outcome.DiffSnapshots(before2, outcome.Snap(gs2))
	if !actual2.Equal(outcome.NewDelta()) {
		findings = append(findings, &Finding{
			CardName: cardName, Event: "becomes_blocked", Check: "phantom",
			Expected: "no change (a bystander was blocked)", Actual: actual2.String(), Raw: t.Raw,
		})
	}
	emitAll(findings)
	return findings, true
}
