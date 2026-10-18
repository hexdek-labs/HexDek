package hexapi

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r176 — the condunk= column, tested end to end through the real
// transport.
//
// The stated defect: evalCondition's fail-closed tail promises the
// unhandled kinds are "a measurable burn-down list", but it publishes
// them with gs.LogEvent into gs.EventLog, and the grinder runs
// EventLogNone. `grep -c condition_unrecognized server.log` over all
// of production returns 0. The list has never been readable.
//
// The stated risk: this project has twice shipped a counter that
// compiled, looked right at its source, and emitted nothing, because
// the tests hand-fed a value at the destination (r167, r169 — r169
// says so in its own comment and then asserts on os.ReadFile). So
// nothing here hand-feeds a number. The test drives the REAL resolve
// entry point with a REAL unrecognized condition and then asserts on
// the bytes renderGrinderLine actually produces.
//
// These tests mutate a process-global census and must not run
// alongside each other.

// drive resolves a conditional effect gated on the named kind through
// the production ResolveEffect path.
func driveCondition(t *testing.T, kind string, args ...interface{}) {
	t.Helper()
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(176)), nil)
	card := &gameengine.Card{Name: "R176 Census Probe", BasePower: 1, BaseToughness: 1,
		Types: []string{"creature"}}
	perm := &gameengine.Permanent{Card: card, Controller: 0, Owner: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	gameengine.ResolveEffect(gs, perm, &gameast.Conditional{
		Condition: &gameast.Condition{Kind: kind, Args: args},
		Body:      &gameast.Draw{},
	})
}

// TestR176_CondUnkFieldReachesTheRenderedGrinderLine is the transport
// test: real resolve -> process-global census -> renderGrinderLine ->
// the actual string the grinder logs. If any hop is broken the
// assertion fails, which is precisely what the source-text tests of
// r167/r169 could not do.
func TestR176_CondUnkFieldReachesTheRenderedGrinderLine(t *testing.T) {
	gameengine.ResetConditionUnrecognizedForTest()
	t.Cleanup(gameengine.ResetConditionUnrecognizedForTest)

	before := renderGrinderLine(grinderWindow{games: 1})
	if strings.Contains(before, "condunk=") {
		t.Fatalf("premise broken — condunk= is present before anything fired: %s", before)
	}

	// "if" is the dominant unrecognized kind in the corpus scan (107
	// cards) and evalCondition has no case for it.
	driveCondition(t, "if")
	driveCondition(t, "if")
	driveCondition(t, "etb_if")

	line := renderGrinderLine(grinderWindow{games: 1})
	if !strings.Contains(line, " condunk=") {
		t.Fatalf("condunk= never reached the rendered grinder line: %s", line)
	}
	// Non-zero, and the right non-zero: two "if" evaluations outrank
	// the one etb_if, so "if" must lead and must read 2.
	if !strings.Contains(line, " condunk=if:2,etb_if:1") {
		t.Fatalf("condunk= did not carry the real counts; got: %s", line)
	}
	// The column must not have displaced its neighbours.
	if !strings.Contains(line, "grinder: 1 games") {
		t.Fatalf("the grinder line lost its prefix: %s", line)
	}
}

// TestR176_CensusCountsOnlyUnrecognizedKinds proves BOTH directions:
// a kind evalCondition handles is not counted, a kind it does not
// handle is. Without the negative half the counter could be a
// "something was evaluated" tally and still look correct.
func TestR176_CensusCountsOnlyUnrecognizedKinds(t *testing.T) {
	gameengine.ResetConditionUnrecognizedForTest()
	t.Cleanup(gameengine.ResetConditionUnrecognizedForTest)

	// RECOGNIZED: life_threshold has a case in evalCondition.
	driveCondition(t, "life_threshold", ">=", 1)
	// "raw" is also recognized (and deliberately returns true).
	driveCondition(t, "raw", "some undecomposed text")

	if got := gameengine.ConditionUnrecognizedCounts(); len(got) != 0 {
		t.Fatalf("a recognized condition kind was counted as unrecognized: %v", got)
	}
	if s := gameengine.ConditionUnrecognizedTop(8); s != "" {
		t.Fatalf("the census rendered a field with no unrecognized kind: %q", s)
	}

	// UNRECOGNIZED: no case exists for this one.
	driveCondition(t, "no_mana_spent_to_cast")

	got := gameengine.ConditionUnrecognizedCounts()
	if got["no_mana_spent_to_cast"] != 1 {
		t.Fatalf("the unrecognized kind was not counted: %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("exactly one kind should be in the census; got %v", got)
	}
	if !strings.Contains(renderGrinderLine(grinderWindow{}), " condunk=no_mana_spent_to_cast:1") {
		t.Fatal("the unrecognized kind did not reach the rendered line")
	}
}
