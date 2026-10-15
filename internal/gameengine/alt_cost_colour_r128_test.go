package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r128 — CR §202.1a / §107.4a / §601.2f: the coloured pips of an
// ALTERNATIVE cost must be payable in their own colours, exactly like
// the pips of a printed cost.
//
// r126 put this check on the printed cost only. The 36 corpus cards
// whose alternative cost demands MORE of some colour than their printed
// cost (Mizzium Mortars {1}{R} -> {3}{R}{R}{R}; every one of the 21
// prototype cards, printed colourless and prototyped coloured) sail
// through that check and then pay the alt cost out of any mana at all.
//
// Both directions, driven through the real entry point (CastSpell), with
// a Hat that always says yes so the only variable is the pool's colour.

// r128Overload builds an instant whose PRINTED cost is payable and whose
// overload cost is the given string — the real AST shape, a string arg
// ("{3}{R}{R}{R}"), not the bare number the older fixtures use.
func r128Overload(t *testing.T, name string, printed int, overload string) *Card {
	t.Helper()
	return &Card{
		Name:           name,
		Owner:          0,
		Colors:         []string{"R"},
		ManaCostString: "{1}{R}",
		Types:          []string{"instant", "cost:" + itoa(printed)},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "overload", Args: []interface{}{overload}},
			},
		},
	}
}

// r128Cast seats the card with an exact typed pool and casts it, and
// returns the mana actually spent. The legacy ManaPool mirror is set to
// the typed total exactly, so EnsureTypedPool credits nothing to Any —
// Any is a universal wildcard and would make every colour assertion
// here vacuous.
//
// Spent mana is the discriminator, not the stack: an instant with no
// body resolves inside CastSpell, so by the time it returns the stack
// is already empty. Printed cost paid = the overload was declined.
func r128Cast(t *testing.T, card *Card, pool *ColoredManaPool) int {
	t.Helper()
	gs := newKWCombatGame(t)
	seat := gs.Seats[0]
	seat.Hat = &payHat{}
	seat.Mana = pool
	seat.ManaPool = pool.Total()
	before := seat.ManaPool
	seat.Hand = append(seat.Hand, card)
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell should succeed (normally if not overloaded): %v", err)
	}
	return before - seat.ManaPool
}

const (
	r128Printed  = 2 // the printed cost of the fixture card
	r128OverloadCost = 6 // {3}{R}{R}{R}
)

// CATCHES: five red mana are enough in TOTAL for {3}{R}{R}{R}, but only
// one of them is red. Pre-r128 this overloaded.
func TestR128_Overload_DeclinedWhenColourUnavailable(t *testing.T) {
	card := r128Overload(t, "Mizzium Mortars", 2, "{3}{R}{R}{R}")
	if spent := r128Cast(t, card, &ColoredManaPool{R: 1, C: 5}); spent != r128Printed {
		t.Fatalf("overloaded {3}{R}{R}{R} on a pool holding one red (spent %d, printed cost %d): §202.1a requires three red", spent, r128Printed)
	}
}

// MUST NEVER BLOCK: the same total, the right colours.
func TestR128_Overload_TakenWhenColourAvailable(t *testing.T) {
	card := r128Overload(t, "Mizzium Mortars", 2, "{3}{R}{R}{R}")
	if spent := r128Cast(t, card, &ColoredManaPool{R: 3, C: 3}); spent != r128OverloadCost {
		t.Fatalf("three red and two generic pays {3}{R}{R}{R}; want %d spent, got %d", r128OverloadCost, spent)
	}
}

// MUST NEVER BLOCK: a pool that is all wildcard pays any colour.
func TestR128_Overload_AnyManaPaysEveryPip(t *testing.T) {
	card := r128Overload(t, "Mizzium Mortars", 2, "{3}{R}{R}{R}")
	if spent := r128Cast(t, card, &ColoredManaPool{Any: 6}); spent != r128OverloadCost {
		t.Fatalf("any-colour mana pays coloured pips; want %d spent, got %d", r128OverloadCost, spent)
	}
}

// CARVE-OUT: when the parser stored the overload cost as a bare number
// the colour of that cost is unknown, so the gate must not guess. This
// is also what keeps every pre-r128 fixture behaving as it did.
func TestR128_Overload_NumericCostArgIsNotJudged(t *testing.T) {
	card := r128Overload(t, "Cyclonic Rift", 2, "")
	card.AST.Abilities = []gameast.Ability{
		&gameast.Keyword{Name: "overload", Args: []interface{}{float64(5)}},
	}
	if spent := r128Cast(t, card, &ColoredManaPool{R: 1, C: 4}); spent != 5 {
		t.Fatalf("a numeric overload arg carries no pip information and must not be declined; want 5 spent, got %d", spent)
	}
}

// CARVE-OUT: §106.4a restricted mana may legally pay this very spell and
// the check cannot tell which, so it declines to judge.
func TestR128_Overload_RestrictedManaIsNotJudged(t *testing.T) {
	card := r128Overload(t, "Mizzium Mortars", 2, "{3}{R}{R}{R}")
	pool := &ColoredManaPool{R: 1, C: 5}
	pool.Restricted = append(pool.Restricted, RestrictedMana{Amount: 1})
	if spent := r128Cast(t, card, pool); spent != r128OverloadCost {
		t.Fatalf("a pool holding restricted mana must not be judged on colour; want %d spent, got %d", r128OverloadCost, spent)
	}
}

// The cost-string form is the shared implementation; pin it directly for
// the shapes the card form can never reach (an alt cost with no card).
func TestR128_CanSupplyColoredPipsForCost_BothDirections(t *testing.T) {
	cases := []struct {
		name string
		pool *ColoredManaPool
		cost string
		want bool
	}{
		{"one red cannot pay three", &ColoredManaPool{R: 1, C: 4}, "{3}{R}{R}{R}", false},
		{"three red can", &ColoredManaPool{R: 3, C: 2}, "{3}{R}{R}{R}", true},
		{"prototype colour absent", &ColoredManaPool{W: 2}, "{1}{B}", false},
		{"prototype colour present", &ColoredManaPool{B: 1, W: 1}, "{1}{B}", true},
		{"empty cost is not judged", &ColoredManaPool{}, "", true},
		{"lowercase pips are read", &ColoredManaPool{W: 2}, "{1}{b}", false},
	}
	for _, tc := range cases {
		if got := CanSupplyColoredPipsForCost(tc.pool, tc.cost); got != tc.want {
			t.Errorf("%s: CanSupplyColoredPipsForCost(%q) = %v, want %v", tc.name, tc.cost, got, tc.want)
		}
	}
}
