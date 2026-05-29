package gameengine

import "testing"

// Phase 7 Counter DB — behavioral tests for Saga chapter ticking
// (CR §714) and Battle damage routing + defeat trigger (CR §310).
// Companion to internal/gameengine/counters/sagas_battles_test.go,
// which holds the registry-shape assertions.

// ---------------------------------------------------------------------------
// Saga: TickSagaChapters precombat-main wiring
// ---------------------------------------------------------------------------

// TestPhase7_TickSagaChapters_AdvancesAllControllerSagas pins the
// precombat-main tick: every Saga the active seat controls gets
// exactly one lore counter per call. Phased-out sagas and sagas at
// or past their final chapter are skipped.
func TestPhase7_TickSagaChapters_AdvancesAllControllerSagas(t *testing.T) {
	gs := newMiscGame(t)

	s1 := addMiscBattlefield(gs, 0, "Saga A", 0, 0, "enchantment", "saga")
	s1.Counters["saga_final_chapter"] = 3
	s1.Counters["lore"] = 0

	s2 := addMiscBattlefield(gs, 0, "Saga B", 0, 0, "enchantment", "saga")
	s2.Counters["saga_final_chapter"] = 3
	s2.Counters["lore"] = 1

	// Opponent's saga — must NOT advance on seat-0 precombat main.
	other := addMiscBattlefield(gs, 1, "Saga C", 0, 0, "enchantment", "saga")
	other.Counters["saga_final_chapter"] = 3
	other.Counters["lore"] = 1

	ticked := TickSagaChapters(gs, 0)
	if ticked != 2 {
		t.Errorf("expected 2 sagas ticked, got %d", ticked)
	}
	if s1.Counters["lore"] != 1 {
		t.Errorf("Saga A lore = %d, want 1", s1.Counters["lore"])
	}
	if s2.Counters["lore"] != 2 {
		t.Errorf("Saga B lore = %d, want 2", s2.Counters["lore"])
	}
	if other.Counters["lore"] != 1 {
		t.Errorf("opponent Saga C lore = %d, want 1 (unchanged)", other.Counters["lore"])
	}
}

// TestPhase7_TickSagaChapters_SkipsFinalChapter prevents the tick from
// over-advancing a saga whose lore already equals its final chapter
// (which would double-fire the final chapter ability before SBA
// §704.5s can sacrifice the permanent).
func TestPhase7_TickSagaChapters_SkipsFinalChapter(t *testing.T) {
	gs := newMiscGame(t)

	s := addMiscBattlefield(gs, 0, "History of Benalia", 0, 0, "enchantment", "saga")
	s.Counters["saga_final_chapter"] = 3
	s.Counters["lore"] = 3

	ticked := TickSagaChapters(gs, 0)
	if ticked != 0 {
		t.Errorf("expected 0 sagas ticked, got %d", ticked)
	}
	if s.Counters["lore"] != 3 {
		t.Errorf("lore = %d, want 3 (final-chapter saga must not advance)", s.Counters["lore"])
	}
}

// TestPhase7_TickSagaChapters_SkipsPhasedOut confirms a phased-out
// saga (CR §702.26) doesn't tick — it isn't "on the battlefield" in
// any rules-meaningful way during its controller's precombat main.
func TestPhase7_TickSagaChapters_SkipsPhasedOut(t *testing.T) {
	gs := newMiscGame(t)

	s := addMiscBattlefield(gs, 0, "Saga X", 0, 0, "enchantment", "saga")
	s.Counters["saga_final_chapter"] = 3
	s.Counters["lore"] = 1
	s.PhasedOut = true

	ticked := TickSagaChapters(gs, 0)
	if ticked != 0 {
		t.Errorf("expected 0 sagas ticked, got %d", ticked)
	}
	if s.Counters["lore"] != 1 {
		t.Errorf("phased-out saga lore = %d, want 1 (unchanged)", s.Counters["lore"])
	}
}

// ---------------------------------------------------------------------------
// Saga: chapter ticks I → II → III + final-chapter SBA sacrifice
// ---------------------------------------------------------------------------

// TestPhase7_Saga_TicksToFinalChapter_SBASacs runs the full lifecycle:
// 1 lore counter on ETB (seeded explicitly via the test fixture), then
// 3 precombat ticks → SBA §704.5s sacrifices. Pins the integration of
// the precombat-tick → AdvanceSagaChapter → lore_counter_added →
// saga_final_chapter SBA chain.
func TestPhase7_Saga_TicksToFinalChapter_SBASacs(t *testing.T) {
	gs := newMiscGame(t)

	// Simulate Saga ETB: initSagaLoreCounters places 1 lore counter and
	// stamps saga_final_chapter. We replicate that state directly so
	// the test doesn't depend on full AST plumbing.
	s := addMiscBattlefield(gs, 0, "History of Benalia", 0, 0, "enchantment", "saga")
	s.Counters["saga_final_chapter"] = 3
	s.Counters["lore"] = 1

	// Precombat tick #1 — should land lore at 2.
	if n := TickSagaChapters(gs, 0); n != 1 {
		t.Fatalf("tick #1 expected 1, got %d", n)
	}
	if s.Counters["lore"] != 2 {
		t.Fatalf("after tick #1 lore = %d, want 2", s.Counters["lore"])
	}

	// Precombat tick #2 — lore at 3 (final chapter reached).
	if n := TickSagaChapters(gs, 0); n != 1 {
		t.Fatalf("tick #2 expected 1, got %d", n)
	}
	if s.Counters["lore"] != 3 {
		t.Fatalf("after tick #2 lore = %d, want 3", s.Counters["lore"])
	}

	// Precombat tick #3 — saga is at final chapter, skip.
	if n := TickSagaChapters(gs, 0); n != 0 {
		t.Fatalf("tick #3 expected 0 (final-chapter skip), got %d", n)
	}

	// SBA §704.5s sacrifices the saga.
	StateBasedActions(gs)
	for _, p := range gs.Seats[0].Battlefield {
		if p == s {
			t.Fatal("saga should have been sacrificed by SBA §704.5s after reaching final chapter")
		}
	}
}

// TestPhase7_Saga_LoreCounterAddedFiresWithChapterContext pins the
// trigger contract that per_card chapter dispatchers (e.g. History of
// Benalia, Elspeth Conquers Death) depend on: AdvanceSagaChapter must
// fire "lore_counter_added" with ctx["chapter"] = new lore count.
func TestPhase7_Saga_LoreCounterAddedFiresWithChapterContext(t *testing.T) {
	gs := newMiscGame(t)

	s := addMiscBattlefield(gs, 0, "History of Benalia", 0, 0, "enchantment", "saga")
	s.Counters["saga_final_chapter"] = 3
	s.Counters["lore"] = 0

	ch := AdvanceSagaChapter(gs, s)
	if ch != 1 {
		t.Errorf("first advance returned %d, want 1", ch)
	}

	// Verify the saga_chapter event was logged with Amount = chapter.
	foundChapter := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "saga_chapter" && ev.Amount == 1 && ev.Seat == 0 {
			foundChapter = true
			break
		}
	}
	if !foundChapter {
		t.Error("expected saga_chapter event with Amount=1 to be logged")
	}
}

// ---------------------------------------------------------------------------
// Battle: defense counter damage routing
// ---------------------------------------------------------------------------

// TestPhase7_Battle_TakesDamageDefenseDecrements pins the
// ApplyCombatDamageToBattle → RemoveDefenseCounters contract: 3
// combat damage drops defense from 4 to 1 with no defeat flip.
func TestPhase7_Battle_TakesDamageDefenseDecrements(t *testing.T) {
	gs := newMiscGame(t)

	b := addMiscBattlefield(gs, 0, "Invasion of Ravnica", 0, 0, "battle", "siege")
	b.Counters["defense"] = 4

	atk := addMiscBattlefield(gs, 1, "Attacker", 3, 3, "creature")

	ApplyCombatDamageToBattle(gs, atk, 3, b)

	if got := BattleDefenseCounters(b); got != 1 {
		t.Errorf("defense = %d, want 1 after 3 damage", got)
	}
	if IsBattleDefeated(b) {
		t.Error("battle should not be defeated at defense=1")
	}
	if b.Flags["battle_defeated"] > 0 {
		t.Error("battle_defeated flag should not be set yet")
	}
}

// TestPhase7_Battle_ReachesZeroDefenseFlipsAndDefeats pins the
// defeat pipeline: 4 damage to a defense-4 battle drives defense to
// 0, fires FireBattleZeroDefense exactly once (becomes_defeated +
// battle_defeated latch), and SBA §704.5v destroys the battle on
// the next pass.
func TestPhase7_Battle_ReachesZeroDefenseFlipsAndDefeats(t *testing.T) {
	gs := newMiscGame(t)

	b := addMiscBattlefield(gs, 0, "Invasion of Ravnica", 0, 0, "battle", "siege")
	b.Counters["defense"] = 4
	atk := addMiscBattlefield(gs, 1, "Attacker", 4, 4, "creature")

	ApplyCombatDamageToBattle(gs, atk, 4, b)

	if got := BattleDefenseCounters(b); got != 0 {
		t.Errorf("defense = %d, want 0 after 4 damage", got)
	}
	if !IsBattleDefeated(b) {
		t.Error("battle should be defeated at defense=0")
	}

	// One — and only one — battle_defeated event should have fired.
	defeats := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "battle_defeated" {
			defeats++
		}
	}
	if defeats != 1 {
		t.Errorf("battle_defeated fired %d times, want exactly 1", defeats)
	}

	// SBA §704.5v should destroy the battle on the next pass.
	StateBasedActions(gs)
	for _, p := range gs.Seats[0].Battlefield {
		if p == b {
			t.Fatal("battle should have been destroyed by SBA §704.5v")
		}
	}
}

// TestPhase7_Battle_OverkillStopsAtZero pins damage clamping: a
// battle at defense=2 takes 10 damage but only loses 2 counters
// (CR §310 / §704.5v: the defeat trigger fires once when defense
// reaches 0, regardless of overkill amount). The remaining 8
// damage is absorbed, not redirected, because the engine treats
// damage to battles as defense-counter removal (CR §310.5b) rather
// than life-loss-equivalent damage.
func TestPhase7_Battle_OverkillStopsAtZero(t *testing.T) {
	gs := newMiscGame(t)

	b := addMiscBattlefield(gs, 0, "Invasion of Ravnica", 0, 0, "battle", "siege")
	b.Counters["defense"] = 2
	atk := addMiscBattlefield(gs, 1, "Big Attacker", 10, 10, "creature")

	ApplyCombatDamageToBattle(gs, atk, 10, b)

	if got := BattleDefenseCounters(b); got != 0 {
		t.Errorf("defense = %d, want 0 after overkill damage", got)
	}
	if !IsBattleDefeated(b) {
		t.Error("battle should be defeated at defense=0")
	}

	defeats := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "battle_defeated" {
			defeats++
		}
	}
	if defeats != 1 {
		t.Errorf("battle_defeated fired %d times, want exactly 1 (overkill should not double-fire)", defeats)
	}
}

// TestPhase7_Battle_IncrementalDamageDefeatsOnExactlyZero confirms
// that multiple smaller damage applications correctly stack:
// 4-defense battle takes 3 damage (defense=1, no flip), then 1
// damage (defense=0, flip + becomes_defeated). Mirrors the spec
// test the user named: "3 combat damage, defense 4 → 1, no flip;
// 1 more, defense=0, flip, post-defeat trigger fires."
func TestPhase7_Battle_IncrementalDamageDefeatsOnExactlyZero(t *testing.T) {
	gs := newMiscGame(t)

	b := addMiscBattlefield(gs, 0, "Invasion of Ravnica", 0, 0, "battle", "siege")
	b.Counters["defense"] = 4
	atk := addMiscBattlefield(gs, 1, "Attacker", 3, 3, "creature")

	// First strike: 3 damage.
	ApplyCombatDamageToBattle(gs, atk, 3, b)
	if got := BattleDefenseCounters(b); got != 1 {
		t.Fatalf("after 3 damage, defense = %d, want 1", got)
	}
	if IsBattleDefeated(b) {
		t.Fatal("battle should NOT be defeated at defense=1")
	}

	// Second strike: 1 more damage.
	ApplyCombatDamageToBattle(gs, atk, 1, b)
	if got := BattleDefenseCounters(b); got != 0 {
		t.Fatalf("after 1 more damage, defense = %d, want 0", got)
	}
	if !IsBattleDefeated(b) {
		t.Fatal("battle should be defeated at defense=0")
	}

	defeats := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "battle_defeated" {
			defeats++
		}
	}
	if defeats != 1 {
		t.Errorf("battle_defeated fired %d times, want exactly 1 (idempotent latch)", defeats)
	}
}
