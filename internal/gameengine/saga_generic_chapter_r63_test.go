package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Saga audit r63 — generic (non-per_card) Saga chapter abilities resolve from
// the AST. Before the fix, the ~187 corpus Sagas not in the phase-7 per_card
// list fired lore_counter_added into the void; their chapters were INERT.

func unlistedSaga(gs *GameState, seat int, name string, chapters []gameast.Ability) *Permanent {
	p := addMiscBattlefield(gs, seat, name, 0, 0, "enchantment", "saga")
	p.Card.AST = &gameast.CardAST{Name: name, Abilities: chapters}
	p.Counters["saga_final_chapter"] = 3
	return p
}

func saChapter(roman interface{}, eff gameast.Effect) gameast.Ability {
	return &gameast.Static{Modification: &gameast.Modification{
		ModKind: "saga_chapter",
		Args:    []interface{}{roman, eff},
	}}
}

func drainStackT(gs *GameState) {
	for i := 0; i < 60 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// (1)+(3): chapter I fires on ETB-seeded lore; chapter II on a tick; each fires
// when lore REACHES that value, from the canonical AST path.
func TestSaga_GenericChapter_FiresPerChapter(t *testing.T) {
	gs := newMiscGame(t)
	gs.Seats[0].Life = 20
	saga := unlistedSaga(gs, 0, "Probe Saga A", []gameast.Ability{
		saChapter("I", &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "controller"}}),
		saChapter("II", &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 5}, Target: gameast.Filter{Base: "controller"}}),
	})

	AddLoreCounters(gs, saga, 1) // reach chapter I
	drainStackT(gs)
	if gs.Seats[0].Life != 23 {
		t.Fatalf("chapter I (gain 3) did not fire: life=%d, want 23 — generic saga chapter still INERT", gs.Seats[0].Life)
	}
	AddLoreCounters(gs, saga, 1) // reach chapter II
	drainStackT(gs)
	if gs.Seats[0].Life != 28 {
		t.Fatalf("chapter II (gain 5) did not fire: life=%d, want 28", gs.Seats[0].Life)
	}
}

// (5): a double-lore / proliferate-style multi-counter add fires EACH crossed
// chapter once, in order (chapters I and II both resolve from one +2).
func TestSaga_GenericChapter_DoubleLoreFiresBothChapters(t *testing.T) {
	gs := newMiscGame(t)
	gs.Seats[0].Life = 20
	saga := unlistedSaga(gs, 0, "Probe Saga B", []gameast.Ability{
		saChapter("I", &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "controller"}}),
		saChapter("II", &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 5}, Target: gameast.Filter{Base: "controller"}}),
	})
	AddLoreCounters(gs, saga, 2) // cross I and II at once
	drainStackT(gs)
	if gs.Seats[0].Life != 28 {
		t.Fatalf("double-lore should fire BOTH chapter I (+3) and II (+5): life=%d, want 28", gs.Seats[0].Life)
	}
}

// Guard: a chapter shared across "I, II" (list arg) fires on both I and II.
func TestSaga_GenericChapter_ListedRomanFiresEachValue(t *testing.T) {
	gs := newMiscGame(t)
	gs.Seats[0].Life = 20
	saga := unlistedSaga(gs, 0, "Probe Saga C", []gameast.Ability{
		saChapter([]interface{}{"I", "II"}, &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 2}, Target: gameast.Filter{Base: "controller"}}),
	})
	AddLoreCounters(gs, saga, 1)
	drainStackT(gs)
	AddLoreCounters(gs, saga, 1)
	drainStackT(gs)
	if gs.Seats[0].Life != 24 {
		t.Fatalf("shared [I,II] chapter should fire on BOTH: life=%d, want 24 (20+2+2)", gs.Seats[0].Life)
	}
}
