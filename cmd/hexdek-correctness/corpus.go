package main

// corpus.go — the AST-corpus sweep feeding the OUTCOME and PROGRESSION
// dimensions. Walks every card in the dataset and drives the Judge's
// own checkers (internal/judge/outcome, internal/judge/progression);
// this driver adds nothing but counting.
//
// Units:
//
//	OUTCOME      one unit per in-scope (card, effect) — RunEffect — plus
//	             one per in-scope enters-with-counters static
//	             (CheckETBCounters). Passed = the engine's observed
//	             state delta matched the interpreter's expectation.
//	PROGRESSION  one unit per in-scope (card, trigger) — CheckTrigger /
//	             CheckPhaseTrigger / CheckLTBTrigger. Passed = zero
//	             findings (fired exactly once with the right delta AND
//	             no phantom fire).
//
// A panic inside a checker counts the unit as FAILED (an engine crash
// on a representative board is not correct behavior) under the kind
// "harness_panic", and never takes down the sweep.

import (
	"fmt"
	"os"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/judge/outcome"
	"github.com/hexdek/hexdek/internal/judge/progression"
)

// runOutcomePass sweeps the corpus through the OUTCOME checkers.
func runOutcomePass(corpus *astload.Corpus) DimensionScore {
	checked, passed, panics := 0, 0, 0
	byKind := map[string]int{}
	failedByKind := map[string]int{}

	for _, name := range corpus.Names() {
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}

		// Enters-with-counters statics (replacement-class arm).
		for _, ab := range ast.Abilities {
			st, ok := ab.(*gameast.Static)
			if !ok || st.Modification == nil {
				continue
			}
			fd, ran, panicked := safeCheckETBCounters(name, st.Modification)
			if !ran {
				continue
			}
			checked++
			byKind["etb_with_counters"]++
			switch {
			case panicked:
				panics++
				failedByKind["harness_panic"]++
			case fd == nil:
				passed++
			default:
				failedByKind["etb_with_counters"]++
			}
		}

		// Resolvable effect trees (triggered / activated / spell-body).
		for _, ex := range outcome.ExtractEffects(ast) {
			fd, ran, panicked := safeRunEffect(name, ex.Raw, ex.Effect)
			if !ran {
				continue
			}
			checked++
			kind := ex.Effect.Kind()
			byKind[kind]++
			switch {
			case panicked:
				panics++
				failedByKind["harness_panic"]++
			case fd == nil:
				passed++
			default:
				failedByKind[kind]++
			}
		}
	}

	return DimensionScore{
		Dimension: judge.DimensionOutcome,
		Unit:      "effects",
		Checked:   checked,
		Passed:    passed,
		Pct:       pct(passed, checked),
		Detail: map[string]interface{}{
			"in_scope_by_kind": byKind,
			"failed_by_kind":   failedByKind,
			"panics":           panics,
		},
	}
}

// runProgressionPass sweeps the corpus through the PROGRESSION
// trigger-correctness checkers (same walk as the corpus-audit test).
func runProgressionPass(corpus *astload.Corpus) DimensionScore {
	checked, passed, panics := 0, 0, 0
	byEvent := map[string]int{}
	failedByEvent := map[string]int{}

	for _, name := range corpus.Names() {
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}
		for _, ab := range ast.Abilities {
			tr, ok := ab.(*gameast.Triggered)
			if !ok {
				continue
			}
			findings, ran, evName, panicked := safeCheckAnyTrigger(name, tr)
			if !ran {
				continue
			}
			checked++
			byEvent[evName]++
			switch {
			case panicked:
				panics++
				failedByEvent["harness_panic"]++
			case len(findings) == 0:
				passed++
			default:
				failedByEvent[evName]++
				if os.Getenv("CORRECTNESS_DEBUG_VIOLATIONS") != "" {
					for _, f := range findings {
						fmt.Fprintf(os.Stderr, "PROG_DIV card=%q event=%s check=%s expected=%q actual=%q raw=%q\n",
							f.CardName, f.Event, f.Check, f.Expected, f.Actual, f.Raw)
					}
				}
			}
		}
	}

	return DimensionScore{
		Dimension: judge.DimensionProgression,
		Unit:      "triggers",
		Checked:   checked,
		Passed:    passed,
		Pct:       pct(passed, checked),
		Detail: map[string]interface{}{
			"in_scope_by_event": byEvent,
			"failed_by_event":   failedByEvent,
			"panics":            panics,
		},
	}
}

// ---------------------------------------------------------------------------
// Panic-isolating wrappers — one bad card must not kill the sweep.
// ---------------------------------------------------------------------------

func safeRunEffect(name, raw string, eff gameast.Effect) (fd *outcome.Finding, ran, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			fd, ran, panicked = nil, true, true
		}
	}()
	fd, ran = outcome.RunEffect(name, raw, eff)
	return fd, ran, false
}

func safeCheckETBCounters(name string, m *gameast.Modification) (fd *outcome.ETBCountersFinding, ran, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			fd, ran, panicked = nil, true, true
		}
	}()
	fd, ran = outcome.CheckETBCounters(name, m)
	return fd, ran, false
}

func safeCheckAnyTrigger(name string, tr *gameast.Triggered) (findings []*progression.Finding, ran bool, evName string, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			findings, ran, panicked = nil, true, true
			if evName == "" {
				evName = "unknown"
			}
		}
	}()
	if findings, ran = progression.CheckAny(name, tr); ran {
		switch {
		case func() bool { _, ok := progression.InScopeTrigger(tr); return ok }():
			evName, _ = progression.InScopeTrigger(tr)
		case func() bool { _, _, ok := progression.InScopePhaseTrigger(tr); return ok }():
			step, scope, _ := progression.InScopePhaseTrigger(tr)
			evName = fmt.Sprintf("%s/%s", step, scope)
		case progression.InScopeLTBTrigger(tr):
			evName = "ltb"
		case progression.InScopeCombatDamageTrigger(tr):
			evName = "combat_damage_player"
		case progression.InScopeYouAttackTrigger(tr):
			evName = "you_attack"
		default:
			if cs, ok := progression.InScopeCastTrigger(tr); ok {
				evName = "cast/" + cs.Who + "/" + cs.Filter
			} else if scope, ok := progression.InScopeCombatBeginTrigger(tr); ok {
				evName = "combat_begin/" + scope
			} else if _, ok := progression.InScopeAllyETBTrigger(tr); ok {
				evName = "ally_etb"
			} else if progression.InScopeBecomesBlockedTrigger(tr) {
				evName = "becomes_blocked"
			} else if progression.InScopeOrdinalFirstMainTrigger(tr) {
				evName = "ordinal_first_main"
			} else if progression.InScopeTurnedFaceUpTrigger(tr) {
				evName = "turned_face_up"
			} else {
				evName = "widened"
			}
		}
		return findings, true, evName, false
	}
	return nil, false, "", false
}
