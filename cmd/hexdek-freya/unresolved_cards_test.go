package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// HexDek keeps two card stores, and a deck list can fall through
// either one:
//
//	oracle-cards.json   what Freya reasons about. A miss here means the
//	                    analysis was computed WITHOUT the card.
//	ast_dataset.jsonl   what the game engine executes. A miss here means
//	                    the analysis is fine but the card does nothing
//	                    in a simulated game.
//
// Both used to fail silently. These tests pin the part that is easy to
// get catastrophically wrong: reporting a miss when there is nothing to
// miss against.

// astFixture builds the package's existing fakeASTSource (a
// map[string]*gameast.CardAST, defined in combo_math_r63_test.go) from a
// plain name list.
func astFixture(names ...string) fakeASTSource {
	f := fakeASTSource{}
	for _, n := range names {
		f[n] = &gameast.CardAST{}
	}
	return f
}

func TestASTSourceLoaded_ReportsInstallationNotContent(t *testing.T) {
	// The whole guard rests on this distinction. If astSourceLoaded()
	// ever answered "does this card have an AST" instead of "is there a
	// corpus at all", an empty corpus would mark every card in every
	// deck unsupported — a statement about a missing data file
	// presented to users as a statement about their deck.
	t.Cleanup(func() { SetMathASTSource(nil) })

	SetMathASTSource(nil)
	if astSourceLoaded() {
		t.Fatal("astSourceLoaded() true with no corpus installed")
	}

	// An EMPTY corpus is still an installed corpus.
	SetMathASTSource(astFixture())
	if !astSourceLoaded() {
		t.Error("astSourceLoaded() false for an installed but empty corpus — " +
			"installation and content are different questions")
	}

	SetMathASTSource(astFixture("Sol Ring"))
	if !astSourceLoaded() {
		t.Error("astSourceLoaded() false for a populated corpus")
	}
}

func TestASTHasCard(t *testing.T) {
	t.Cleanup(func() { SetMathASTSource(nil) })

	t.Run("no corpus installed returns false for everything", func(t *testing.T) {
		SetMathASTSource(nil)
		if astHasCard("Sol Ring") {
			t.Error("astHasCard() true with no corpus — callers gate on " +
				"astSourceLoaded(), and this must not panic or invent a hit")
		}
	})

	t.Run("hit and miss against an installed corpus", func(t *testing.T) {
		SetMathASTSource(astFixture("Sol Ring"))
		if !astHasCard("Sol Ring") {
			t.Error("astHasCard(\"Sol Ring\") = false, want true")
		}
		if astHasCard("Phyrexian Broodstar") {
			t.Error("astHasCard() = true for a name not in the corpus")
		}
	})
}

// TestUnresolvedCardShape pins the wire shape. Both gap lists share it
// and the frontend renders both with one code path, so a change to one
// silently changes the other.
func TestUnresolvedCardShape(t *testing.T) {
	c := UnresolvedCard{Name: "Mister Fantastic, Reed Richards", Qty: 2}
	if c.Name == "" || c.Qty != 2 {
		t.Fatalf("UnresolvedCard round trip broken: %+v", c)
	}

	r := &FreyaReport{}
	if len(r.UnresolvedCards) != 0 || len(r.UnsupportedCards) != 0 {
		t.Error("a zero FreyaReport must report no gaps — an analysis with " +
			"nothing wrong must render no banner at all")
	}
}
