package hexapi

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r182 — the capswallow= / capfirst= columns, tested through the real
// transport, which is the only hop r167 and r169 could not test and
// the only hop that has twice been broken in this file.
//
// The counters themselves are driven by the real fireTrigger in
// internal/gameengine/per_card; hexapi cannot import that without a
// cycle, so these drive the engine-side primitives and assert on the
// bytes renderGrinderLine actually produces.
//
// These mutate a process-global census and must not run concurrently.

func r182LineReset(t *testing.T) {
	t.Helper()
	reset := func() {
		gameengine.ResetDepthCapForTest()
		gameengine.ResetLoopGuardLabelsForTest()
	}
	reset()
	t.Cleanup(reset)
}

func TestR182_CapSwallowFieldsReachTheRenderedGrinderLine(t *testing.T) {
	r182LineReset(t)

	before := renderGrinderLine(grinderWindow{games: 1})
	if strings.Contains(before, "capswallow=") || strings.Contains(before, "capfirst=") {
		t.Fatalf("premise broken — the columns are present before anything "+
			"was swallowed: %s", before)
	}

	// One loud game and one quiet one — the exact shape the field
	// exists to separate.
	loud := &gameengine.GameState{}
	loud.NoteCascadeRoot()
	for i := 0; i < 9; i++ {
		loud.NoteDepthCapSwallow("life_gained")
	}
	quiet := &gameengine.GameState{}
	quiet.NoteCascadeRoot()
	quiet.NoteDepthCapSwallow("creature_dies")

	line := renderGrinderLine(grinderWindow{games: 1})
	// r187 appended cascadesmax: between cascademax: and mixed:. Both
	// games here run exactly one swallowing cascade, so the per-game
	// maximum is 1.
	if !strings.Contains(line, " capswallow=games:2,max:9,cascademax:9,cascadesmax:1,mixed:0,hist[1:2,2:1,4:1,8:1]") {
		t.Fatalf("capswallow= did not carry the real per-game shape; got: %s", line)
	}
	if !strings.Contains(line, " capfirst=creature_dies:1,life_gained:1") {
		t.Fatalf("capfirst= did not carry the per-game attribution; got: %s", line)
	}
	if !strings.Contains(line, "grinder: 1 games") {
		t.Fatalf("the new columns displaced the line's prefix: %s", line)
	}
}

// TestR182_QuietWindowKeepsTheLineUnchanged — a column that always
// prints is a column nobody reads, and the depth cap does not fire in
// most windows.
func TestR182_QuietWindowKeepsTheLineUnchanged(t *testing.T) {
	r182LineReset(t)
	line := renderGrinderLine(grinderWindow{games: 7})
	if strings.Contains(line, "capswallow=") || strings.Contains(line, "capfirst=") {
		t.Fatalf("the columns printed with nothing swallowed: %s", line)
	}
}
