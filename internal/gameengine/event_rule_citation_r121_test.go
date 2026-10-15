package gameengine

import "testing"

// r121 — the rule number an event carries must name the action the event IS.
//
// The engine stamps a CR citation into Details["rule"] on keyword-action
// events, and that string reaches the event log, so it is outcome data, not
// a comment. Thirty-four of them named the wrong rule: `scry` carried
// §701.18 (Play), `sacrifice` carried §701.17 (Mill), `regenerate` carried
// §701.15a (Goad), `surveil` carried §701.46 (Adapt).
//
// The cause is the same shape as r120's §704.5 shift — §701 keyword actions
// are inserted ALPHABETICALLY, so every new action pushes the numbers after
// it up, while §702 keyword abilities are append-only and largely survived.
//
// This test drives the real entry points rather than asserting on a table,
// because a table would only restate the fix. The static companion is
// TestCRCitations_NoWrongNumberForTopic_R94 in internal/crcite.

// r121Rule returns the Details["rule"] of the last event of `kind`.
func r121Rule(t *testing.T, gs *GameState, kind string) string {
	t.Helper()
	for i := len(gs.EventLog) - 1; i >= 0; i-- {
		if gs.EventLog[i].Kind != kind {
			continue
		}
		v, ok := gs.EventLog[i].Details["rule"]
		if !ok {
			t.Fatalf("%q event carries no \"rule\" detail", kind)
		}
		s, _ := v.(string)
		return s
	}
	t.Fatalf("no %q event in the log", kind)
	return ""
}

func r121Want(t *testing.T, gs *GameState, kind, want string) {
	t.Helper()
	if got := r121Rule(t, gs, kind); got != want {
		t.Errorf("%s event cites §%s, want §%s", kind, got, want)
	}
}

// §701.22 Scry / §701.25 Surveil — NOT §701.18 (Play) / §701.46 (Adapt).
func TestEventRuleCitationR121_ScryAndSurveil(t *testing.T) {
	gs := newFixtureGame(t)
	for i := 0; i < 8; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Filler"})
	}
	Scry(gs, 0, 2)
	r121Want(t, gs, "scry", "701.22")

	Surveil(gs, 0, 2)
	r121Want(t, gs, "surveil", "701.25")
}

// §701.8 Destroy — NOT §701.7 (Create).
func TestEventRuleCitationR121_Destroy(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "r121 Victim", 2, 2, "creature")
	src := addBattlefield(gs, 1, "r121 Source", 1, 1, "creature")
	if !DestroyPermanent(gs, p, src) {
		t.Fatalf("fixture: DestroyPermanent did not remove the permanent")
	}
	r121Want(t, gs, "destroy", "701.8")
}

// §701.21 Sacrifice — NOT §701.17 (Mill).
func TestEventRuleCitationR121_Sacrifice(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "r121 Fodder", 1, 1, "creature")
	SacrificePermanent(gs, p, "r121")
	r121Want(t, gs, "sacrifice", "701.21")
}
