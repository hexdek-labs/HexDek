package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// suspend_cast_option_test.go — regressions for the generic "suspend from
// hand" entry point (slice 3): SuspendParams reads N + cost off the keyword,
// and a suspend-N hand card exiled via SuspendCard ticks down and free-casts
// on schedule (the slice-1/2 machinery). The tournament play-loop wiring
// (runSuspendPass) is exercised end-to-end by Loki; here we pin the gameengine
// read side + integration with the tick/free-cast path.

func mkSuspendHandCard(gs *GameState, seat int, name string, args []interface{}, raw string, types ...string) *Card {
	if len(types) == 0 {
		types = []string{"creature"}
	}
	card := &Card{
		Name: name, Owner: seat, Types: append([]string{}, types...),
		BasePower: 4, BaseToughness: 4,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "suspend", Raw: raw, Args: args},
		}},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, card)
	return card
}

// SuspendParams must read N (time counters) from the bare-integer arg and the
// suspend cost (CMC) from the {…}-shaped arg — NOT confuse the two.
func TestSuspendParams_ReadsNAndCostSeparately(t *testing.T) {
	gs := newTestGame(t, 2)

	// Errant Ephemeron: suspend 4-{1}{u} -> N=4, cost=2.
	e := mkSuspendHandCard(gs, 0, "Errant Ephemeron", []interface{}{"4", "{1}{u}"}, "suspend 4-{1}{u}")
	if n, cost, ok := SuspendParams(e); !ok || n != 4 || cost != 2 {
		t.Fatalf("Errant Ephemeron want N=4 cost=2 ok, got n=%d cost=%d ok=%v", n, cost, ok)
	}
	// Lotus Bloom: suspend 3-{0} -> N=3, cost=0.
	l := mkSuspendHandCard(gs, 0, "Lotus Bloom", []interface{}{"3", "{0}"}, "suspend 3-{0}", "artifact")
	if n, cost, ok := SuspendParams(l); !ok || n != 3 || cost != 0 {
		t.Fatalf("Lotus Bloom want N=3 cost=0 ok, got n=%d cost=%d ok=%v", n, cost, ok)
	}
	// Raw fallback only (no Args).
	r := mkSuspendHandCard(gs, 0, "Rift Bolt", nil, "suspend 1-{r}", "sorcery")
	if n, cost, ok := SuspendParams(r); !ok || n != 1 || cost != 1 {
		t.Fatalf("Rift Bolt raw fallback want N=1 cost=1 ok, got n=%d cost=%d ok=%v", n, cost, ok)
	}
	// A non-suspend card returns ok=false.
	plain := &Card{Name: "Bear", Owner: 0, Types: []string{"creature"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	if _, _, ok := SuspendParams(plain); ok {
		t.Fatalf("a card without suspend must return ok=false")
	}
	if HasSuspend(plain) {
		t.Fatalf("HasSuspend must be false for a non-suspend card")
	}
}

// The generic flow: a suspend-N card in hand is recognized (HasSuspend +
// SuspendParams), exiled with N time counters (SuspendCard), then ticks down
// over N upkeeps and is free-cast on the last tick — landing as a creature
// with haste, with no zone duplication.
func TestSuspend_GenericHandCardExilesTicksAndFreeCasts(t *testing.T) {
	gs := newTestGame(t, 2)
	card := mkSuspendHandCard(gs, 0, "Errant Ephemeron",
		[]interface{}{"3", "{1}{u}"}, "suspend 3-{1}{u}")

	// Discover the action the way the play loop does.
	if !HasSuspend(card) {
		t.Fatalf("HasSuspend must detect the keyword")
	}
	n, _, ok := SuspendParams(card)
	if !ok || n != 3 {
		t.Fatalf("want N=3, got n=%d ok=%v", n, ok)
	}

	// Take the special action: exile with N time counters.
	SuspendCard(gs, 0, card, n)
	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("suspended card must leave hand")
	}
	if len(gs.Seats[0].Exile) != 1 || gs.Seats[0].Exile[0] != card {
		t.Fatalf("suspended card must be in exile")
	}
	if got, _ := card.Meta["suspend_counters"].(int); got != 3 {
		t.Fatalf("suspended card must carry N=3 counters, got %d", got)
	}

	// Tick three controller upkeeps; the third free-casts.
	TickSuspendCounters(gs, 0) // 3 -> 2
	TickSuspendCounters(gs, 0) // 2 -> 1
	if len(gs.Seats[0].Exile) != 1 {
		t.Fatalf("card must remain suspended in exile until last counter removed")
	}
	TickSuspendCounters(gs, 0) // 1 -> 0, free-cast

	if len(gs.Seats[0].Exile) != 0 {
		t.Fatalf("free-cast must remove the card from exile (no duplication), exile=%d", len(gs.Seats[0].Exile))
	}
	var found *Permanent
	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			found = p
			count++
		}
	}
	if count != 1 {
		t.Fatalf("suspended creature must resolve onto the battlefield exactly once, got %d", count)
	}
	if found.Flags["kw:haste"] != 1 {
		t.Fatalf("a free-cast suspended creature must gain haste (§702.62g)")
	}
}
