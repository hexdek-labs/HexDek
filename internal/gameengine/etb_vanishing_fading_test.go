package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Vanishing/Fading §614.1c ETB-counter coverage: a permanent with a bare
// Vanishing N / Fading N keyword now enters with N time / fade counters.
// Before ApplyVanishingFadingETBCounters these entered with zero counters
// (no path read the keyword).

func mkKeywordPerm(gs *GameState, kwName, kwRaw string, args []interface{}, extra ...gameast.Ability) *Permanent {
	abilities := []gameast.Ability{
		&gameast.Keyword{Name: kwName, Raw: kwRaw, Args: args},
	}
	abilities = append(abilities, extra...)
	card := &Card{
		Name: "Test VF Perm", Owner: 0, Types: []string{"creature"},
		BasePower: 2, BaseToughness: 2,
		AST: &gameast.CardAST{Abilities: abilities},
	}
	return &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
}

func TestVanishing_EntersWithTimeCounters(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkKeywordPerm(gs, "vanishing", "vanishing 3", nil)
	ApplyVanishingFadingETBCounters(gs, p)
	if got := p.Counters["time"]; got != 3 {
		t.Fatalf("Vanishing 3 must enter with 3 time counters, got %d", got)
	}
}

func TestFading_EntersWithFadeCounters(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkKeywordPerm(gs, "fading", "fading 2", nil)
	ApplyVanishingFadingETBCounters(gs, p)
	if got := p.Counters["fade"]; got != 2 {
		t.Fatalf("Fading 2 must enter with 2 fade counters, got %d", got)
	}
	if p.Counters["time"] != 0 {
		t.Fatalf("Fading must not place time counters")
	}
}

func TestVanishing_CountFromArgs(t *testing.T) {
	gs := newTestGame(t, 2)
	// Structured Args take precedence over raw parsing.
	p := mkKeywordPerm(gs, "vanishing", "vanishing", []interface{}{4})
	ApplyVanishingFadingETBCounters(gs, p)
	if got := p.Counters["time"]; got != 4 {
		t.Fatalf("Vanishing with Args[0]=4 must enter with 4 time counters, got %d", got)
	}
}

func TestVanishing_FullETBPath(t *testing.T) {
	gs := newTestGame(t, 2)
	// Through the real ETB chokepoint (FirePermanentETBTriggers), not just the
	// helper, to prove the wireup.
	p := mkKeywordPerm(gs, "vanishing", "vanishing 1", nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	if got := p.Counters["time"]; got != 1 {
		t.Fatalf("Vanishing 1 via ETB dispatch must place 1 time counter, got %d", got)
	}
}

func TestVanishingFading_StructuredCounterGateNoDouble(t *testing.T) {
	gs := newTestGame(t, 2)
	// A card carrying BOTH a vanishing keyword AND a structured
	// etb_with_counters static (Thor parsed the reminder) must NOT get the
	// keyword counters too — ApplyStaticETBCounters owns it. We only assert
	// the keyword fallback stands down (it adds nothing here).
	p := mkKeywordPerm(gs, "vanishing", "vanishing 3", nil,
		&gameast.Static{Modification: &gameast.Modification{
			ModKind: "etb_with_counters", Args: []interface{}{3, "time"}}})
	ApplyVanishingFadingETBCounters(gs, p)
	if got := p.Counters["time"]; got != 0 {
		t.Fatalf("keyword fallback must stand down when etb_with_counters static present, got %d time counters", got)
	}
}

func TestVanishingFading_NonKeywordNoCounters(t *testing.T) {
	gs := newTestGame(t, 2)
	card := &Card{Name: "Bear", Owner: 0, Types: []string{"creature"},
		BasePower: 2, BaseToughness: 2,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	p := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{}}
	ApplyVanishingFadingETBCounters(gs, p)
	if len(p.Counters) != 0 {
		t.Fatalf("a permanent without vanishing/fading must get no counters, got %v", p.Counters)
	}
}
