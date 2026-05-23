package gameengine

// Loki r59 / game 420 — ZoneConservation cluster (104 "2 real cards
// disappeared" hits in the Breya / Baron Bertram / Alela / SP-dr pod,
// seed 4200042, turn 10+). Root cause: an Unholy Indenture self-
// reanimate cycle drove StateBasedActions past its pass cap, setting
// gs.Flags["ended"]=1 + gs.Flags["game_draw"]=1 (CR §104.4b mandatory
// loop draw). CheckEnd did not react to game_draw so the chaos turn
// loop kept going; the next turn's CastSpell pushed a permanent spell
// onto gs.Stack, ResolveStackTop short-circuited on the ended flag,
// PriorityRound and DrainStack burned iterations until the §727
// loop-shortcut no-op detector fired, and projectAndApply nuked the
// stack via `gs.Stack = gs.Stack[:0]` — silently vanishing the Card.
//
// This test pins the fix in loop_shortcut.go: evacuateStackSpellsToGraveyard
// routes the underlying *Card to its owner's graveyard before the slice
// gets truncated, preserving zone conservation.

import "testing"

func TestEvacuateStackSpellsToGraveyard_NoOpLoopBranch(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Library = []*Card{}
	gs.Seats[0].Hand = []*Card{}
	gs.Seats[0].Graveyard = []*Card{}
	gs.Seats[1].Library = []*Card{}

	spell := &Card{Name: "Dragonborn Looter", Owner: 0, Types: []string{"creature"}}
	gs.Stack = append(gs.Stack, &StackItem{Card: spell, Controller: 0, ID: 99})

	pre := len(gs.Seats[0].Graveyard) + len(gs.Stack)
	evacuateStackSpellsToGraveyard(gs)
	gs.Stack = gs.Stack[:0]
	post := len(gs.Seats[0].Graveyard) + len(gs.Stack)

	if pre != post {
		t.Fatalf("zone conservation lost: pre=%d post=%d", pre, post)
	}
	if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0] != spell {
		t.Fatalf("expected spell in seat 0 graveyard, got %v", gs.Seats[0].Graveyard)
	}
}

func TestEvacuateStackSpellsToGraveyard_RoutesByOwnerNotController(t *testing.T) {
	// CR §608.2g — a non-permanent spell goes to its OWNER's graveyard
	// when it leaves the stack, not its controller's. Mirror that for
	// the evacuation path so a hijacked-control spell (Word of Command,
	// Telepathy plays) lands correctly.
	gs := NewGameState(2, nil, nil)
	spell := &Card{Name: "A Tale for the Ages", Owner: 1}
	gs.Stack = append(gs.Stack, &StackItem{Card: spell, Controller: 0, ID: 101})

	evacuateStackSpellsToGraveyard(gs)

	if len(gs.Seats[1].Graveyard) != 1 || gs.Seats[1].Graveyard[0] != spell {
		t.Fatalf("expected spell to land in OWNER seat 1 graveyard, got seat0=%v seat1=%v",
			gs.Seats[0].Graveyard, gs.Seats[1].Graveyard)
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatalf("seat 0 (controller) graveyard must remain empty, got %v", gs.Seats[0].Graveyard)
	}
}

func TestEvacuateStackSpellsToGraveyard_SkipsCopiesAndAbilities(t *testing.T) {
	// CR §707.10 — token copies of spells cease to exist on resolution
	// (or stack clear). Triggered/activated ability items have a Source
	// permanent and no spell-card identity. Both must be dropped without
	// zone placement.
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Graveyard = []*Card{}

	copyCard := &Card{Name: "Echocasting Symposium", Owner: 0}
	abilityCard := &Card{Name: "Unholy Indenture", Owner: 0}
	srcPerm := &Permanent{Card: abilityCard, Controller: 0, Owner: 0}

	gs.Stack = append(gs.Stack, &StackItem{Card: copyCard, Controller: 0, IsCopy: true, ID: 1})
	gs.Stack = append(gs.Stack, &StackItem{Card: abilityCard, Source: srcPerm, Controller: 0, ID: 2})

	evacuateStackSpellsToGraveyard(gs)

	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatalf("copies and ability items must not be placed in graveyard, got %v", gs.Seats[0].Graveyard)
	}
}

func TestProjectAndApply_NoOpLoop_PreservesSpellOnStack(t *testing.T) {
	// End-to-end: when projectAndApply detects a no-op loop with a real
	// spell on the stack, the spell's *Card must land in the owner's
	// graveyard — not vanish into a `gs.Stack[:0]` truncation.
	gs := NewGameState(2, nil, nil)
	spell := &Card{Name: "Dragonborn Looter", Owner: 0, Types: []string{"creature"}}
	gs.Stack = append(gs.Stack, &StackItem{Card: spell, Controller: 0, ID: 1})

	// Seed a no-op fingerprint sequence: identical snapshots, identical
	// stack-top fingerprints, repeated 3+ times. The detector requires
	// loopMinReps full repetitions of a period ≤ loopMaxPeriod.
	ld := newLoopDetector()
	fp := stackTopFingerprint(gs)
	for i := 0; i < loopMinReps*2; i++ {
		ld.record(gs, fp)
	}

	if !ld.projectAndApply(gs) {
		t.Fatal("expected projectAndApply to detect no-op loop and break")
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("expected stack cleared, got %d items", len(gs.Stack))
	}
	if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0] != spell {
		t.Fatalf("expected spell evacuated to seat 0 graveyard, got %v", gs.Seats[0].Graveyard)
	}
}
