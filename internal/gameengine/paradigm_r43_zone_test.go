package gameengine

// r43 — Krark / Decorum-Dissertation zone-conservation cluster.
//
// 824 ZoneConservation "extra real cards appeared" hits in Loki r41 game 181
// traced to ResolveParadigmCopies pushing a copy StackItem without
// StackItem.IsCopy=true. Per CR §707.10 a copy of a non-permanent spell
// ceases to exist on resolution; without the IsCopy flag, ResolveStackTop
// routed each Decorum Dissertation paradigm copy into the controller's
// graveyard, inflating the real-card census by one per cast.
//
// Test pins the contract: a paradigm copy must (a) carry IsCopy on its
// StackItem and (b) leave the graveyard untouched after resolution.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestResolveParadigmCopies_CopyStackItemIsFlaggedAndCeases(t *testing.T) {
	// Paradigm-tracked sorcery with a real effect (Draw 1) so the copy goes
	// through ResolveEffect and then through the "non-permanent spell after
	// resolve" branch in ResolveStackTop.
	original := &Card{
		Name:  "Paradigm Sorcery",
		Owner: 0,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: "Paradigm Sorcery",
			Abilities: []gameast.Ability{
				&gameast.Activated{Effect: &gameast.Draw{Count: *gameast.NumInt(1)}},
			},
		},
	}

	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].Exile = []*Card{original}
	gs.ParadigmExile = map[int][]*Card{0: {original}}

	preReal := countRealCards(gs.Seats[0].Library) +
		countRealCards(gs.Seats[0].Hand) +
		countRealCards(gs.Seats[0].Graveyard) +
		countRealCards(gs.Seats[0].Exile) +
		countRealCards(gs.Seats[0].CommandZone)

	ResolveParadigmCopies(gs, 0)

	// Original must remain in exile (not moved by the copy resolution).
	stillInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == original {
			stillInExile = true
			break
		}
	}
	if !stillInExile {
		t.Fatal("paradigm original must remain in exile after copy resolution")
	}

	// Graveyard must not receive the copy. CR §707.10 — copy ceases.
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name == original.Name {
			t.Fatalf("copy must cease to exist on resolution, not land in graveyard (found %q)", c.Name)
		}
	}

	// And the per-seat real-card total is unchanged — the copy didn't
	// inflate zone conservation.
	postReal := countRealCards(gs.Seats[0].Library) +
		countRealCards(gs.Seats[0].Hand) +
		countRealCards(gs.Seats[0].Graveyard) +
		countRealCards(gs.Seats[0].Exile) +
		countRealCards(gs.Seats[0].CommandZone)
	if postReal != preReal {
		t.Fatalf("zone conservation broken by paradigm copy: pre=%d post=%d (Δ=%d)",
			preReal, postReal, postReal-preReal)
	}

	// And the cast event must be flagged as a paradigm copy.
	sawCopyCast := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "paradigm_copy_cast" {
			sawCopyCast = true
			break
		}
	}
	if !sawCopyCast {
		t.Fatal("expected paradigm_copy_cast event")
	}
}

// TestResolveParadigmCopies_RepeatsDontLeak — drives the real failure mode
// from r41 game 181: the same paradigm-tracked card is cast as a copy
// repeatedly (one per main phase). Pre-fix this leaked one card to the
// graveyard per tick. Post-fix, ten ticks leaves the per-seat real-card
// count exactly unchanged.
func TestResolveParadigmCopies_RepeatsDontLeak(t *testing.T) {
	original := &Card{
		Name:  "Decorum Dissertation",
		Owner: 0,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: "Decorum Dissertation",
			Abilities: []gameast.Ability{
				&gameast.Activated{Effect: &gameast.Draw{Count: *gameast.NumInt(1)}},
			},
		},
	}

	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].Exile = []*Card{original}
	gs.ParadigmExile = map[int][]*Card{0: {original}}

	pre := countRealCards(gs.Seats[0].Library) +
		countRealCards(gs.Seats[0].Hand) +
		countRealCards(gs.Seats[0].Graveyard) +
		countRealCards(gs.Seats[0].Exile) +
		countRealCards(gs.Seats[0].CommandZone)

	for i := 0; i < 10; i++ {
		ResolveParadigmCopies(gs, 0)
	}

	post := countRealCards(gs.Seats[0].Library) +
		countRealCards(gs.Seats[0].Hand) +
		countRealCards(gs.Seats[0].Graveyard) +
		countRealCards(gs.Seats[0].Exile) +
		countRealCards(gs.Seats[0].CommandZone)
	if post != pre {
		t.Fatalf("zone conservation drift after 10 paradigm ticks: pre=%d post=%d Δ=%d",
			pre, post, post-pre)
	}
}
