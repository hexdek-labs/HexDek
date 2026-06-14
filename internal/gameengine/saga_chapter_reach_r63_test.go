package gameengine

// r63 — Saga chapter-reach probe (CR §714.2b).
//
// Two bugs fixed:
//   (c) a lore add that crosses MULTIPLE chapters at once fired only the final
//       chapter (AdvanceSagaChapter hardcoded +1 / single fire, no loop).
//   (f) proliferating a Saga's lore counter bumped perm.Counters["lore"]
//       silently without firing the newly-reached chapter ability.
//
// Both now route through AddLoreCounters, which fires one chapter trigger per
// chapter number newly reached, in ascending order.

import "testing"

func sagaChapterAmounts(gs *GameState) []int {
	var out []int
	for _, ev := range gs.EventLog {
		if ev.Kind == "saga_chapter" {
			out = append(out, ev.Amount)
		}
	}
	return out
}

func newTestSaga(gs *GameState, seat, lore, final int) *Permanent {
	p := addBattlefield(gs, seat, "Test Saga", 0, 0, "enchantment", "saga")
	p.Counters["lore"] = lore
	p.Counters["saga_final_chapter"] = final
	return p
}

// (c) A 2-counter add from lore 1 reaches chapters II and III — both must fire,
// in order, exactly once each.
func TestSaga_MultiChapterReachFiresEachInOrder(t *testing.T) {
	gs := newCombatGame(t)
	saga := newTestSaga(gs, 0, 1, 3)

	AddLoreCounters(gs, saga, 2) // lore 1 → 3

	if saga.Counters["lore"] != 3 {
		t.Fatalf("lore after add-2 = %d, want 3", saga.Counters["lore"])
	}
	got := sagaChapterAmounts(gs)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("chapters fired = %v, want [2 3] (each newly-reached chapter, in order)", got)
	}
}

// A single +1 tick still fires exactly one chapter (AdvanceSagaChapter parity —
// no double-fire from the refactor).
func TestSaga_SingleTickFiresOneChapter(t *testing.T) {
	gs := newCombatGame(t)
	saga := newTestSaga(gs, 0, 1, 3)

	ch := AdvanceSagaChapter(gs, saga)
	if ch != 2 {
		t.Errorf("AdvanceSagaChapter returned %d, want 2", ch)
	}
	got := sagaChapterAmounts(gs)
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("chapters fired = %v, want [2] (exactly one)", got)
	}
}

// (f) Proliferating a Saga's lore counter fires the newly-reached chapter.
func TestSaga_ProliferateFiresChapter(t *testing.T) {
	gs := newCombatGame(t)
	saga := newTestSaga(gs, 0, 1, 3)

	applied, err := Proliferate(gs, 0, []ProliferateTarget{
		{Permanent: saga, CounterType: "lore"},
	})
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if saga.Counters["lore"] != 2 {
		t.Errorf("lore after proliferate = %d, want 2", saga.Counters["lore"])
	}
	got := sagaChapterAmounts(gs)
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("proliferate must fire chapter II; chapters fired = %v, want [2]", got)
	}
}
