package gameengine

import "testing"

func ringBearerCount(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["ring_bearer"] == 1 {
			n++
		}
	}
	return n
}

// (a)/(d) First tempt: get the Ring (level 1) + designate a single Ring-bearer.
func TestRing_FirstTemptDesignatesBearer(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Bearer", 2, 2, "creature")
	TheRingTemptsYou(gs, 0)
	if GetRingLevel(gs, 0) != 1 {
		t.Fatalf("first tempt should set ring level 1, got %d", GetRingLevel(gs, 0))
	}
	if c.Flags["ring_bearer"] != 1 {
		t.Fatal("first tempt should designate the controller's creature as Ring-bearer")
	}
	if ringBearerCount(gs, 0) != 1 {
		t.Fatalf("exactly one Ring-bearer at a time, got %d", ringBearerCount(gs, 0))
	}
}

// (b) Cumulative levels 1→2→3→4, capped at 4.
func TestRing_LevelsCumulativeCappedAt4(t *testing.T) {
	gs := newCombatGame(t)
	addBattlefield(gs, 0, "Bearer", 2, 2, "creature")
	want := []int{1, 2, 3, 4, 4}
	for i, w := range want {
		TheRingTemptsYou(gs, 0)
		if got := GetRingLevel(gs, 0); got != w {
			t.Fatalf("tempt %d: ring level want %d, got %d", i+1, w, got)
		}
	}
}

// (e)/(a) THE BUG: if the Ring-bearer leaves, the next tempt designates a NEW
// bearer (pre-r63 the sticky ring_bearer_set gate never re-chose).
func TestRing_BearerLeaves_NextTemptRedesignates(t *testing.T) {
	gs := newCombatGame(t)
	a := addBattlefield(gs, 0, "Bearer A", 2, 2, "creature")
	TheRingTemptsYou(gs, 0)
	if a.Flags["ring_bearer"] != 1 {
		t.Fatal("A should be the Ring-bearer after the first tempt")
	}

	// A leaves the battlefield.
	gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
	b := addBattlefield(gs, 0, "Bearer B", 3, 3, "creature")

	// Next tempt must designate a live bearer (B), since A is gone.
	TheRingTemptsYou(gs, 0)
	if b.Flags["ring_bearer"] != 1 {
		t.Fatal("after the Ring-bearer left, the next tempt must designate a NEW bearer (B)")
	}
	if ringBearerCount(gs, 0) != 1 {
		t.Fatalf("still exactly one Ring-bearer, got %d", ringBearerCount(gs, 0))
	}
}

// (a) An existing live bearer is kept across tempts (you may keep the same one).
func TestRing_KeepsLiveBearerAcrossTempts(t *testing.T) {
	gs := newCombatGame(t)
	a := addBattlefield(gs, 0, "Bearer A", 2, 2, "creature")
	addBattlefield(gs, 0, "Other", 1, 1, "creature")
	TheRingTemptsYou(gs, 0)
	TheRingTemptsYou(gs, 0)
	if a.Flags["ring_bearer"] != 1 {
		t.Fatal("the live Ring-bearer should be kept across tempts")
	}
	if ringBearerCount(gs, 0) != 1 {
		t.Fatalf("exactly one Ring-bearer, got %d", ringBearerCount(gs, 0))
	}
}

// (f) "whenever the Ring tempts you" trigger fires once per tempt.
func TestRing_TemptTriggerFires(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "ring_tempt" {
			fired++
		}
	}
	gs := newCombatGame(t)
	addBattlefield(gs, 0, "Bearer", 2, 2, "creature")
	TheRingTemptsYou(gs, 0)
	TheRingTemptsYou(gs, 0)
	if fired != 2 {
		t.Fatalf("ring_tempt trigger should fire once per tempt (2), fired %d", fired)
	}
}
