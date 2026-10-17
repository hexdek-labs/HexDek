package hexapi

import (
	"os"
	"strings"
	"testing"
)

// r167 — the wiring test, in the CALLER's package.
//
// r163's lesson, paid for the hard way: a control that deleted the
// production call site left the whole gate green, because every test
// exercised the helper directly and nothing asserted that the grinder
// line actually used it. LoopGuardLabelsFor is in exactly that
// position — four gameengine tests cover its behaviour and none of
// them would notice if showmatch stopped calling it.
//
// The grinder log line is emitted from deep inside a long-running
// loop that cannot be invoked in isolation, so this asserts on the
// source. That is cruder than driving the code, and it is stated here
// rather than hidden: its only job is to make the deletion control go
// red, and it does.
func TestR167_GrinderLineWiresTheDeadlineField(t *testing.T) {
	src, err := os.ReadFile("showmatch.go")
	if err != nil {
		t.Fatalf("reading showmatch.go: %v", err)
	}
	s := string(src)

	if !strings.Contains(s, `gameengine.LoopGuardLabelsFor("game_deadline")`) {
		t.Fatal("showmatch.go no longer calls LoopGuardLabelsFor(\"game_deadline\") — " +
			"the deadline's firing site is invisible in production again")
	}
	if !strings.Contains(s, `" deadlines=" + deadlines`) {
		t.Fatal("the deadlines= field is computed but never prefixed into the log line")
	}
	// Computed and prefixed is still not printed: r160 shipped nine
	// guards that were counted and then dropped on the floor.
	// r169 inserted tmemo= between deadlines and guardTop. The lock is
	// on deadlines still being IN the argument list in the right place,
	// not on the list never changing — this assertion correctly went red
	// when r169 edited the line, which is the whole point of it.
	if !strings.Contains(s, "ccInvalPerGame, guards, deadlines, tmemo, guardTop, tdepth, tpField)") {
		t.Fatal("deadlines is not passed to the grinder log.Printf argument list")
	}
}
