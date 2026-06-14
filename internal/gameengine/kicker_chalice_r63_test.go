package gameengine

import (
	"testing"
	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 KICKER / MULTIKICKER + Chalice X-counter audit.

func etbCounterCard(name, counterKind string, args ...interface{}) *Card {
	return &Card{
		Name:  name,
		Owner: 0,
		Types: []string{"artifact"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{ModKind: "etb_with_counters", Args: args},
					Raw:          "enters with counters",
				},
			},
		},
	}
}

func resolveEnterX(t *testing.T, card *Card, chosenX, multikick int) *Permanent {
	t.Helper()
	gs := newFixtureGame(t)
	item := &StackItem{Card: card, Controller: 0, ChosenX: chosenX}
	if multikick > 0 {
		item.CostMeta = map[string]interface{}{"kicked": true, "multikick_count": multikick}
	}
	return resolvePermanentSpellETB(gs, item)
}

// (c) THE REPORTED BUG: Chalice of the Void ({X}{X}) enters with X charge
// counters — read from the cast's chosen X, not from multikick_count.
func TestKicker_Chalice_EntersWithXChargeCounters(t *testing.T) {
	for _, x := range []int{0, 1, 2, 5} {
		card := etbCounterCard("Chalice of the Void", "charge", "x", "charge")
		perm := resolveEnterX(t, card, x, 0)
		if perm == nil {
			t.Fatalf("X=%d: nil perm", x)
		}
		if got := perm.Counters["charge"]; got != x {
			t.Fatalf("Chalice cast for X=%d must enter with %d charge counters, got %d", x, x, got)
		}
	}
}

// (b)/(e) Walking Ballista ({X}{X}) enters with X +1/+1 counters.
func TestKicker_WalkingBallista_EntersWithXPlusOne(t *testing.T) {
	card := etbCounterCard("Walking Ballista", "+1/+1", "x", "+1/+1")
	perm := resolveEnterX(t, card, 3, 0)
	if got := perm.Counters["+1/+1"]; got != 3 {
		t.Fatalf("Walking Ballista cast for X=3 must enter with 3 +1/+1 counters, got %d", got)
	}
}

// (b) MULTIKICKER var path still reads the times-kicked count (Everflowing
// Chalice: a charge counter for each time it was kicked).
func TestKicker_EverflowingChalice_VarReadsMultikick(t *testing.T) {
	card := etbCounterCard("Everflowing Chalice", "charge", "var", "charge", "for_each:time it was kicked")
	perm := resolveEnterX(t, card, 0, 3) // kicked 3 times, no X
	if got := perm.Counters["charge"]; got != 3 {
		t.Fatalf("Everflowing Chalice kicked 3x must enter with 3 charge counters, got %d", got)
	}
}

// (f-guard) a derived-X clause (where_x:) must NOT be read as the cast X — the
// fix is scoped to plain "x" only, so derived cards are unaffected (here: no
// chosen_x leakage; falls back to the legacy reading).
func TestKicker_DerivedX_NotReadAsChosenX(t *testing.T) {
	card := etbCounterCard("Voracious Wurm", "+1/+1", "x", "+1/+1", "where_x:the amount of life you've gained this turn")
	perm := resolveEnterX(t, card, 7, 0) // chosen X=7 must NOT leak in
	if got := perm.Counters["+1/+1"]; got == 7 {
		t.Fatalf("derived-X (where_x) card must not read the cast X (7); got %d", got)
	}
}
