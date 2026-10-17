package gameengine

import (
	"strings"
	"testing"
	"time"
)

// r168 — the test r167 should have had.
//
// r167 shipped LoopGuardLabelsFor and wired deadlines= into the
// grinder line. Four tests covered the renderer and one covered the
// wiring. Every one of them fed the label in by hand, via
// bumpLoopGuardLabel or by reading showmatch.go's source. NOT ONE
// drove CheckGameDeadline, so nothing noticed that game_deadline
// records no label at all: it passes "site", and the allow-list was
// {"event", "handler"}.
//
// The field therefore printed nothing through two real capped games.
// An instrument nobody has watched produce output is not an
// instrument, and reasoning about coverage is not coverage — both of
// which I had written down before shipping it.
//
// So these tests start at the production entry point and assert on
// the rendered string, with no hand-fed labels anywhere.

func r168Game(t *testing.T) *GameState {
	t.Helper()
	ResetLoopGuardCountsForTest()
	ResetLoopGuardLabelsForTest()
	t.Cleanup(func() {
		ResetLoopGuardCountsForTest()
		ResetLoopGuardLabelsForTest()
	})
	gs := newFixtureGame(t)
	gs.EventPolicy = EventLogNone // what the grinder really runs
	return gs
}

func TestR168_CheckGameDeadlineRendersItsSite(t *testing.T) {
	gs := r168Game(t)
	SetGameDeadline(gs, time.Now().Add(-time.Second))

	if !CheckGameDeadline(gs, "token_create") {
		t.Fatal("expired deadline did not report a breach")
	}

	// The whole point of r167, asserted end to end.
	if got := LoopGuardLabelsFor("game_deadline"); got != "token_create:1" {
		t.Fatalf("deadlines= would render %q, want %q — the firing is unattributed", got, "token_create:1")
	}
	// And the r160 counter still reads what production prints.
	if n := LoopGuardCounts()["game_deadline"]; n != 1 {
		t.Fatalf("game_deadline counter = %d, want 1", n)
	}
}

func TestR168_EverySiteNameSurvivesToTheField(t *testing.T) {
	// All four live check sites, each in its own game, so each one
	// gets a first breach. A site that never reached the field would
	// make the field lie by omission about which guard acted.
	sites := []string{"token_create", "stack_drain", "trigger_drain", "turn_phase_boundary"}
	ResetLoopGuardCountsForTest()
	ResetLoopGuardLabelsForTest()
	t.Cleanup(func() {
		ResetLoopGuardCountsForTest()
		ResetLoopGuardLabelsForTest()
	})

	for _, site := range sites {
		gs := newFixtureGame(t)
		gs.EventPolicy = EventLogNone
		SetGameDeadline(gs, time.Now().Add(-time.Second))
		CheckGameDeadline(gs, site)
	}

	got := LoopGuardLabelsFor("game_deadline")
	for _, site := range sites {
		if !strings.Contains(got, site+":1") {
			t.Fatalf("site %q missing from %q", site, got)
		}
	}
	if n := strings.Count(got, ",") + 1; n != len(sites) {
		t.Fatalf("rendered %d labels, want %d: %s", n, len(sites), got)
	}
}

func TestR168_UnexpiredDeadlineRecordsNoLabel(t *testing.T) {
	// The other direction: the field must stay empty on the normal
	// path, or it would print on all ~230 windows of a quiet segment.
	gs := r168Game(t)
	SetGameDeadline(gs, time.Now().Add(time.Hour))

	if CheckGameDeadline(gs, "token_create") {
		t.Fatal("unexpired deadline reported a breach")
	}
	if got := LoopGuardLabelsFor("game_deadline"); got != "" {
		t.Fatalf("unexpired deadline rendered %q, want empty", got)
	}
}

func TestR168_ExistingGuardsKeepTheirEventLabel(t *testing.T) {
	// "site" went LAST in the priority list. A guard that supplies
	// both must still be attributed by its event, or r162's existing
	// guardtop column silently changes meaning.
	ResetLoopGuardLabelsForTest()
	t.Cleanup(ResetLoopGuardLabelsForTest)

	if got := loopGuardLabel(map[string]interface{}{
		"event": "permanent_etb",
		"site":  "ast_trigger",
	}); got != "permanent_etb" {
		t.Fatalf("label = %q, want the event to win over the site", got)
	}
	// And a guard with only a site is now attributed where before it
	// was not attributed at all.
	if got := loopGuardLabel(map[string]interface{}{"site": "ast_trigger"}); got != "ast_trigger" {
		t.Fatalf("site-only label = %q, want %q", got, "ast_trigger")
	}
}
