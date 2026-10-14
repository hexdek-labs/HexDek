package gameengine

import (
	"math/rand"
	"testing"
)

// ---------------------------------------------------------------------------
// Suspect tests — CR §701.60
// ---------------------------------------------------------------------------

func newSuspectGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(53)), nil)
}

// ---------------------------------------------------------------------------
// (a) SuspectCreature stamps flag + grants menace
// ---------------------------------------------------------------------------

func TestSuspectCreature_StampsFlagAndGrantsMenace(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	if IsSuspected(p) {
		t.Fatal("permanent should not be suspected at game start")
	}
	if p.HasKeyword("menace") {
		t.Fatal("vanilla 2/2 should not have menace")
	}

	SuspectCreature(gs, p)

	if !IsSuspected(p) {
		t.Fatal("SuspectCreature should set IsSuspected")
	}
	if p.Flags["suspected"] != 1 {
		t.Fatalf("Flags[suspected] = %d, want 1", p.Flags["suspected"])
	}
	if !p.HasKeyword("menace") {
		t.Fatal("SuspectCreature should grant menace (via Flags[kw:menace])")
	}
	if p.Flags["kw:menace"] != 1 {
		t.Fatalf("Flags[kw:menace] = %d, want 1", p.Flags["kw:menace"])
	}
}

func TestSuspectCreature_EmitsEvent(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	before := len(gs.EventLog)
	SuspectCreature(gs, p)
	found := false
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "suspect" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("SuspectCreature should emit a suspect event")
	}
}

func TestSuspectCreature_Idempotent(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	SuspectCreature(gs, p)
	before := len(gs.EventLog)
	SuspectCreature(gs, p)
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "suspect" {
			t.Fatal("repeat SuspectCreature should not emit a second suspect event")
		}
	}
	if !IsSuspected(p) {
		t.Fatal("permanent should remain suspected after repeat call")
	}
}

func TestSuspectCreature_NonCreatureIsNoOp(t *testing.T) {
	gs := newSuspectGame(t)
	land := addBattlefield(gs, 0, "Plains", 0, 0, "land")
	SuspectCreature(gs, land)
	if IsSuspected(land) {
		t.Fatal("suspecting a non-creature must be a no-op per §701.60a")
	}
	if land.HasKeyword("menace") {
		t.Fatal("non-creature must not gain menace")
	}
}

// ---------------------------------------------------------------------------
// (b) Suspected creature cannot be declared as blocker
// ---------------------------------------------------------------------------

func TestSuspectCreature_CannotBlock(t *testing.T) {
	gs := newSuspectGame(t)
	atk := addBattlefield(gs, 0, "Attacker", 3, 3, "creature")
	atk.Flags["attacking"] = 1
	blk := addBattlefield(gs, 1, "Blocker", 4, 4, "creature")

	// Baseline: untapped 4/4 blocker can block a 3/3 attacker.
	if !canBlockGS(gs, atk, blk) {
		t.Fatal("baseline: 4/4 should be able to block a 3/3")
	}

	SuspectCreature(gs, blk)

	if canBlockGS(gs, atk, blk) {
		t.Fatal("§701.60c: a suspected creature must be rejected as a blocker")
	}
}

func TestSuspectCreature_DeclareBlockersExcludesSuspect(t *testing.T) {
	gs := newSuspectGame(t)
	// Seat 0 attacks seat 1.
	atk := addBattlefield(gs, 0, "Attacker", 2, 2, "creature")
	atk.Flags["attacking"] = 1
	blocker := addBattlefield(gs, 1, "Suspected Blocker", 5, 5, "creature")
	SuspectCreature(gs, blocker)

	plan := DeclareBlockers(gs, []*Permanent{atk}, 1)
	for _, blkList := range plan {
		for _, b := range blkList {
			if b == blocker {
				t.Fatal("DeclareBlockers must skip suspected creatures even when they'd otherwise be optimal blockers")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// (c) UnsuspectCreature reverses both
// ---------------------------------------------------------------------------

func TestUnsuspectCreature_RemovesFlagAndMenace(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	SuspectCreature(gs, p)

	if !IsSuspected(p) || !p.HasKeyword("menace") {
		t.Fatal("setup: should be suspected with menace")
	}

	UnsuspectCreature(gs, p)

	if IsSuspected(p) {
		t.Fatal("UnsuspectCreature should clear the suspected designation")
	}
	if p.HasKeyword("menace") {
		t.Fatal("UnsuspectCreature should remove the menace flag stamped by Suspect")
	}
	if _, ok := p.Flags["suspected"]; ok {
		t.Fatal("Flags[suspected] should be deleted, not zeroed")
	}
	if _, ok := p.Flags["kw:menace"]; ok {
		t.Fatal("Flags[kw:menace] should be deleted, not zeroed")
	}
}

func TestUnsuspectCreature_NoOpOnNonSuspected(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Vanilla", 2, 2, "creature")
	before := len(gs.EventLog)
	UnsuspectCreature(gs, p)
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "unsuspect" {
			t.Fatal("UnsuspectCreature on a non-suspected perm must not emit an unsuspect event")
		}
	}
}

func TestUnsuspectCreature_RestoresBlockEligibility(t *testing.T) {
	gs := newSuspectGame(t)
	atk := addBattlefield(gs, 0, "Attacker", 3, 3, "creature")
	atk.Flags["attacking"] = 1
	blk := addBattlefield(gs, 1, "Blocker", 4, 4, "creature")

	SuspectCreature(gs, blk)
	if canBlockGS(gs, atk, blk) {
		t.Fatal("setup: suspected blocker should be rejected")
	}

	UnsuspectCreature(gs, blk)

	if !canBlockGS(gs, atk, blk) {
		t.Fatal("unsuspected creature should be eligible to block again")
	}
}

// ---------------------------------------------------------------------------
// (d) IsSuspected query correct
// ---------------------------------------------------------------------------

func TestIsSuspected_QueriesFlag(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	if IsSuspected(p) {
		t.Fatal("fresh permanent should not be suspected")
	}
	SuspectCreature(gs, p)
	if !IsSuspected(p) {
		t.Fatal("IsSuspected should be true after SuspectCreature")
	}
	UnsuspectCreature(gs, p)
	if IsSuspected(p) {
		t.Fatal("IsSuspected should be false after UnsuspectCreature")
	}
}

func TestIsSuspected_NilSafe(t *testing.T) {
	if IsSuspected(nil) {
		t.Fatal("IsSuspected(nil) should be false")
	}
	p := &Permanent{}
	if IsSuspected(p) {
		t.Fatal("IsSuspected on perm with nil Flags should be false")
	}
}

// ---------------------------------------------------------------------------
// (e) Suspected status persists across turns until investigated
// ---------------------------------------------------------------------------

func TestSuspectCreature_PersistsAcrossEOTCleanup(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	SuspectCreature(gs, p)
	if !IsSuspected(p) || !p.HasKeyword("menace") {
		t.Fatal("setup: should be suspected with menace")
	}

	// Run the end-of-turn cleanup that wipes GrantedAbilities +
	// Modifications. Flags-based storage must survive.
	ScanExpiredDurations(gs, "ending", "cleanup")

	if !IsSuspected(p) {
		t.Fatal("§701.60: suspected designation must persist across EOT cleanup")
	}
	if !p.HasKeyword("menace") {
		t.Fatal("menace granted by Suspect must persist across EOT cleanup (Flags channel)")
	}
}

func TestSuspectCreature_PersistsAcrossMultipleTurns(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	SuspectCreature(gs, p)

	// Drive three turn boundaries' worth of EOT cleanup.
	for turn := 0; turn < 3; turn++ {
		ScanExpiredDurations(gs, "ending", "cleanup")
	}

	if !IsSuspected(p) {
		t.Fatal("suspected designation must survive multiple turn boundaries")
	}
	if !p.HasKeyword("menace") {
		t.Fatal("menace must survive multiple turn boundaries")
	}
}

func TestSuspectCreature_UntapAllPreservesDesignation(t *testing.T) {
	gs := newSuspectGame(t)
	p := addBattlefield(gs, 0, "Suspect", 2, 2, "creature")
	SuspectCreature(gs, p)
	p.Tapped = true

	// Start-of-turn UntapAll runs the turn-start resets — must not
	// clear Flags entries unrelated to per-turn bookkeeping.
	UntapAll(gs, 0)

	if p.Tapped {
		t.Fatal("setup: UntapAll should untap the creature")
	}
	if !IsSuspected(p) {
		t.Fatal("UntapAll must preserve the suspected designation")
	}
	if !p.HasKeyword("menace") {
		t.Fatal("UntapAll must preserve menace stamped by Suspect")
	}
}

// ---------------------------------------------------------------------------
// Nil safety
// ---------------------------------------------------------------------------

func TestSuspectCreature_NilSafe(t *testing.T) {
	gs := newSuspectGame(t)
	SuspectCreature(nil, &Permanent{}) // must not panic
	SuspectCreature(gs, nil)            // must not panic
	UnsuspectCreature(nil, &Permanent{})
	UnsuspectCreature(gs, nil)
}
