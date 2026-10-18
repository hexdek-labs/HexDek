package hexapi

import (
	"testing"

	"github.com/hexdek/hexdek/internal/heimdall"
	"github.com/hexdek/hexdek/internal/judge"
)

// r170 — the muninn adapter must be nil-safe in both directions.
//
// Found by driving the real grinder game loop for the seed work:
// sm.muninnSink is a typed nil *muninnAdapter in any Showmatch built
// without full init, so EndGame panicked on the `a.batcher == nil`
// check itself. Three of the six methods guarded the batcher but not
// the receiver; RecordParserGaps, RecordDeadTriggers and RecordCrash
// guarded neither.
//
// RecordCrash is the one that mattered: it runs inside the grinder's
// recover(), where a second panic is not recoverable and takes the
// worker — one of two — with it.

func TestR170_NilAdapterSurvivesEveryMethod(t *testing.T) {
	var a *muninnAdapter
	// Each of these dereferenced a nil receiver before r170.
	if err := a.Close(); err != nil {
		t.Fatalf("Close on nil adapter: %v", err)
	}
	a.RecordParserGaps([]string{"gap"}, "g1")
	a.RecordDeadTriggers([]heimdall.DeadTrigger{{TriggerType: "etb", CardName: "X"}}, "g1")
	a.RecordCrash("boom", "stack", []string{"a/b"})
	a.AutoArchive(42, [4]string{"a/b"}, []judge.ValidationViolation{{}})
	a.EndGame()
}

// The other direction: a real adapter with a nil batcher must also be
// inert rather than panicking. This is the state the three guarded
// methods were already written for, and the three unguarded ones were
// not.
func TestR170_AdapterWithNilBatcherIsInert(t *testing.T) {
	a := &muninnAdapter{}
	a.RecordParserGaps([]string{"gap"}, "g1")
	a.RecordDeadTriggers([]heimdall.DeadTrigger{{TriggerType: "etb", CardName: "X"}}, "g1")
	a.RecordCrash("boom", "stack", []string{"a/b"})
	a.AutoArchive(42, [4]string{"a/b"}, []judge.ValidationViolation{{}})
	a.EndGame()
}

// A grinder game must now complete without a recovered panic at all.
// This is the assertion that would have caught the original defect:
// the crash was swallowed by recover(), so every existing test still
// reported PASS while printing a stack trace into the gate.
func TestR170_AGrinderGameCompletesWithoutARecoveredPanic(t *testing.T) {
	out := r170Grind(t, 0)
	if containsAny(out, "game crashed", "nil pointer dereference", "panic:") {
		t.Fatalf("a grinder game is panicking and having it swallowed by "+
			"recover(); the log says:\n%s", out)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
