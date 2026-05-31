package main

import (
	"bytes"
	"strings"
	"testing"
)

// interactive_test.go — scripted-input tests for the --interactive Q&A
// REPL. Each test builds an InteractiveContext, feeds a bytes.Buffer
// of newline-separated questions (terminated by "exit"), and asserts
// substrings on the captured output buffer.
//
// The injectable io.Reader / io.Writer plumbing in runInteractive
// keeps the loop testable without a terminal. Test coverage spans
// each intent handler: help, list rules/invariants, explain, cite,
// show state/seat, what-SBAs (with passing + failing snapshots), is
// combat legal, the "why didn't X" pattern, and unknown-question
// graceful path.

// runScript captures the output of feeding `input` (a single string
// of newline-separated questions) into runInteractive with the
// supplied context. The "exit\n" terminator is appended so the loop
// halts deterministically.
func runScript(t *testing.T, ctx *InteractiveContext, input string) string {
	t.Helper()
	if !strings.HasSuffix(input, "exit\n") {
		input += "\nexit\n"
	}
	var out bytes.Buffer
	if err := runInteractive(strings.NewReader(input), &out, ctx); err != nil {
		t.Fatalf("runInteractive: %v", err)
	}
	return out.String()
}

// emptyCtx is the simplest no-snapshot context — used by tests that
// exercise rule-citation-only intents (list rules, cite, explain).
func emptyCtx() *InteractiveContext {
	return &InteractiveContext{
		Snapshot: &SBASnapshot{Seats: []SBASeat{{Idx: 0, Life: 40}}},
	}
}

// ---------------------------------------------------------------------------
// help — printed on `help` / `?`
// ---------------------------------------------------------------------------

func TestInteractive_Help(t *testing.T) {
	out := runScript(t, emptyCtx(), "help\n")
	for _, want := range []string{
		"Question patterns",
		"why did <X> not",
		"is this combat legal",
		"what SBAs are pending",
		"explain <invariant>",
		"cite <topic>",
		"list invariants",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// list rules / list invariants — full citation catalog
// ---------------------------------------------------------------------------

func TestInteractive_ListInvariants(t *testing.T) {
	out := runScript(t, emptyCtx(), "list invariants\n")
	for _, name := range []string{
		"LifeConsistency", "SBACompleteness", "ZoneConservation",
		"AttachmentConsistency", "TriggerCompleteness", "WinCondition",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("list invariants missing %q in:\n%s", name, out)
		}
	}
}

func TestInteractive_ListRules(t *testing.T) {
	out := runScript(t, emptyCtx(), "list rules\n")
	// Every list-rules output must include at least the load-bearing
	// CR sub-sections — 704.5a (life loss), 400.6 (zone identity),
	// 506.2 (combat), 614.1 (replacement).
	for _, rule := range []string{
		"704.5a", "400.6", "506.2", "614.1",
	} {
		if !strings.Contains(out, "§"+rule) {
			t.Errorf("list rules missing §%s in:\n%s", rule, out)
		}
	}
}

// ---------------------------------------------------------------------------
// explain <invariant> — CR citations + summary
// ---------------------------------------------------------------------------

func TestInteractive_ExplainKnownInvariant(t *testing.T) {
	out := runScript(t, emptyCtx(), "explain LifeConsistency\n")
	for _, want := range []string{
		"LifeConsistency",
		"§704.5a",
		"life total 0 or less",
		"§119",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("explain LifeConsistency missing %q in:\n%s", want, out)
		}
	}
}

func TestInteractive_ExplainUnknownInvariant(t *testing.T) {
	out := runScript(t, emptyCtx(), "explain NotARealInvariant\n")
	if !strings.Contains(out, "no invariant named") {
		t.Errorf("expected 'no invariant named' message; got:\n%s", out)
	}
	// Suggestion to use list invariants.
	if !strings.Contains(out, "list invariants") {
		t.Errorf("explain unknown invariant should suggest 'list invariants' in:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// cite <topic> — fuzzy search across descriptions + names
// ---------------------------------------------------------------------------

func TestInteractive_CiteCombat(t *testing.T) {
	out := runScript(t, emptyCtx(), "cite combat\n")
	// CombatLegality has §506.2, §508, §509. At least one must surface.
	matched := strings.Contains(out, "§506.2") ||
		strings.Contains(out, "§508") ||
		strings.Contains(out, "§509")
	if !matched {
		t.Errorf("cite combat missed all of §506.2/§508/§509 in:\n%s", out)
	}
}

func TestInteractive_CiteLife(t *testing.T) {
	out := runScript(t, emptyCtx(), "cite life\n")
	// Any of 704.5a (life loss), 105 (life total), 119 (life bookkeeping)
	// is a valid hit.
	matched := strings.Contains(out, "§704.5a") ||
		strings.Contains(out, "§105") ||
		strings.Contains(out, "§119")
	if !matched {
		t.Errorf("cite life missed life-related citations in:\n%s", out)
	}
}

func TestInteractive_CiteNoMatch(t *testing.T) {
	out := runScript(t, emptyCtx(), "cite xyzzy_no_such_topic\n")
	if !strings.Contains(out, "no citations match") {
		t.Errorf("expected 'no citations match' for unknown topic; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// show state / show seat N — snapshot inspection
// ---------------------------------------------------------------------------

func TestInteractive_ShowState(t *testing.T) {
	ctx := &InteractiveContext{
		Snapshot: &SBASnapshot{
			Format: "commander", Turn: 7,
			Seats: []SBASeat{
				{Idx: 0, Life: 20, PoisonCounters: 3},
				{Idx: 1, Life: 30},
			},
		},
	}
	out := runScript(t, ctx, "show state\n")
	if !strings.Contains(out, "format=\"commander\"") {
		t.Errorf("show state missing format in:\n%s", out)
	}
	if !strings.Contains(out, "turn=7") {
		t.Errorf("show state missing turn in:\n%s", out)
	}
	if !strings.Contains(out, "seat 0") || !strings.Contains(out, "life=20") {
		t.Errorf("show state missing seat 0 details in:\n%s", out)
	}
	if !strings.Contains(out, "poison=3") {
		t.Errorf("show state missing seat 0 poison count in:\n%s", out)
	}
}

func TestInteractive_ShowSeat(t *testing.T) {
	ctx := &InteractiveContext{
		Snapshot: &SBASnapshot{
			Seats: []SBASeat{{
				Idx: 0, Life: 12,
				Battlefield: []SBAPermanent{
					{Name: "Edgar Markov", Types: []string{"creature"}, BasePower: 4, BaseToughness: 4, Tapped: true},
					{Name: "Sol Ring", Types: []string{"artifact"}, Tapped: false},
				},
			}},
		},
	}
	out := runScript(t, ctx, "show seat 0\n")
	if !strings.Contains(out, "Edgar Markov") || !strings.Contains(out, "P/T=4/4") {
		t.Errorf("show seat 0 missing Edgar P/T in:\n%s", out)
	}
	if !strings.Contains(out, "[T]") {
		t.Errorf("show seat 0 missing tapped marker for Edgar in:\n%s", out)
	}
	if !strings.Contains(out, "Sol Ring") {
		t.Errorf("show seat 0 missing Sol Ring in:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// what SBAs — runs §704 probe and reports findings
// ---------------------------------------------------------------------------

func TestInteractive_WhatSBAs_NoViolations(t *testing.T) {
	ctx := &InteractiveContext{
		Snapshot: &SBASnapshot{
			Seats: []SBASeat{{Idx: 0, Life: 40}},
		},
	}
	out := runScript(t, ctx, "what SBAs are pending\n")
	if !strings.Contains(out, "post-SBA clean") {
		t.Errorf("expected post-SBA clean message; got:\n%s", out)
	}
}

func TestInteractive_WhatSBAs_WithViolations(t *testing.T) {
	ctx := &InteractiveContext{
		Snapshot: &SBASnapshot{
			Seats: []SBASeat{{
				Idx: 0, Life: 0, PoisonCounters: 11,
				Battlefield: []SBAPermanent{
					{Name: "Wisp", Types: []string{"creature"}, BaseToughness: 0},
				},
			}},
		},
	}
	out := runScript(t, ctx, "what SBAs are pending\n")
	if !strings.Contains(out, "§704.5a") {
		t.Errorf("expected §704.5a in SBA output; got:\n%s", out)
	}
	if !strings.Contains(out, "§704.5c") {
		t.Errorf("expected §704.5c (poison) in SBA output; got:\n%s", out)
	}
	if !strings.Contains(out, "§704.5f") {
		t.Errorf("expected §704.5f (Wisp toughness 0) in SBA output; got:\n%s", out)
	}
	if !strings.Contains(out, "Wisp") {
		t.Errorf("expected Wisp by name in SBA output; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// is this combat legal — pure citation intent
// ---------------------------------------------------------------------------

func TestInteractive_IsCombatLegal(t *testing.T) {
	out := runScript(t, emptyCtx(), "is this combat legal\n")
	for _, want := range []string{
		"§506", "declare attackers", "declare blockers", "CombatLegality",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("is combat legal answer missing %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// why didn't X ... — fuzzy CR map search on action words
// ---------------------------------------------------------------------------

func TestInteractive_WhyDidNotSteal(t *testing.T) {
	// "steal" doesn't map to a specific CR in the index, but the
	// handler should still emit guidance rather than crashing.
	out := runScript(t, emptyCtx(), "why didn't Tergrid steal that\n")
	// Either we got candidate CR rules (the matcher hit something) OR
	// we got the "no direct CR citation matched" fallback. Either way,
	// the response must be informational — no panic, no silence.
	if !strings.Contains(out, "CR") && !strings.Contains(out, "no direct CR citation") {
		t.Errorf("why-didn't handler didn't emit CR-related guidance:\n%s", out)
	}
	if !strings.Contains(out, "judge>") {
		t.Errorf("transcript missing post-answer prompt:\n%s", out)
	}
}

func TestInteractive_WhyDidntTriggerFire(t *testing.T) {
	// "trigger" matches TriggerCompleteness in the citation map.
	out := runScript(t, emptyCtx(), "why didn't this trigger fire\n")
	if !strings.Contains(out, "§603.2") && !strings.Contains(out, "§603.3") &&
		!strings.Contains(out, "TriggerCompleteness") {
		t.Errorf("why-didn't-trigger missing trigger-completeness citation:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Unknown question — graceful path
// ---------------------------------------------------------------------------

func TestInteractive_UnknownQuestion(t *testing.T) {
	out := runScript(t, emptyCtx(), "the weather is nice today\n")
	if !strings.Contains(out, "don't recognize") {
		t.Errorf("expected 'don't recognize' for unparseable input; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// list violations — Loki replay summary
// ---------------------------------------------------------------------------

func TestInteractive_ListViolations(t *testing.T) {
	ctx := &InteractiveContext{
		Replay: &LokiReplay{
			Meta: LokiReplayMeta{TotalGames: 5000},
			Violations: []LokiReplayViolation{
				{GameIdx: 1, InvariantName: "LifeConsistency", Message: "x"},
				{GameIdx: 2, InvariantName: "LifeConsistency", Message: "y"},
				{GameIdx: 3, InvariantName: "ZoneConservation", Message: "z"},
			},
		},
	}
	out := runScript(t, ctx, "list violations\n")
	if !strings.Contains(out, "LifeConsistency") || !strings.Contains(out, "× 2") {
		t.Errorf("list violations didn't tally LifeConsistency × 2 in:\n%s", out)
	}
	if !strings.Contains(out, "ZoneConservation") {
		t.Errorf("list violations missing ZoneConservation in:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// exit / EOF — clean termination
// ---------------------------------------------------------------------------

func TestInteractive_ExitTerminates(t *testing.T) {
	out := runScript(t, emptyCtx(), "exit\n")
	// Banner + the prompt printed once before exit.
	if !strings.Contains(out, "Interactive Q&A mode") {
		t.Errorf("banner missing; got:\n%s", out)
	}
	// One prompt line at the start.
	if strings.Count(out, "judge>") != 1 {
		t.Errorf("expected exactly 1 judge> prompt before exit, got %d:\n%s",
			strings.Count(out, "judge>"), out)
	}
}
