package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r109 — the variable-quantity add_mana_effect fix, measured at the
// production entry point (ActivateAbility) from the exporter's own arg text.
//
// r108 filed this as "emit the colours first, the count is the hard half".
// That ordering was wrong, and TestAnyManaWildcardProbe_r109 is why: the
// fallback's one mana lands in the typed pool's `Any` bucket, and `Any`
// pays EVERY coloured pip. Emitting {U} in place of Any:1 while leaving the
// count at 1 is a strict downgrade. So the count is resolved first and the
// colours ride along with it.
//
// Covered shapes:
//
//	literal count   "add two mana in any combination of {u}, {b}, and/or {r}"
//	self's power    "… where x is <self>'s power"            (Vivi, A-Vivi)
//	creature count  "… where x is the number of creatures you control" (Xenagos)
func r109ManaFixture(t *testing.T, name, argText string, pow int) (*GameState, *Permanent) {
	t.Helper()
	gs := newFixtureGame(t)
	p := addCreature(gs, 0, name, pow, pow)
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost:   gameast.Cost{},
				Effect: &gameast.ModificationEffect{ModKind: "add_mana_effect", Args: []any{argText}},
			},
		},
	}
	gs.Active = 0
	gs.Seats[0].ManaPool = 0
	gs.Seats[0].Mana = &ColoredManaPool{}
	return gs, p
}

func TestAddManaVariableCount_r109(t *testing.T) {
	cases := []struct {
		label, text string
		pow         int
		// extraCreatures are added to seat 0 BEFORE activation, for the
		// "number of creatures you control" basis.
		extraCreatures int
		wantTotal      int
		wantColors     map[string]int
	}{
		{
			// POSITIVE CONTROL: the fixed-pip filter-land path must be
			// untouched by this change. If it moves, the fix leaked.
			label:      "CONTROL filter land add {u}{r}",
			text:       "add {u}{r}",
			wantTotal:  2,
			wantColors: map[string]int{"U": 1, "R": 1},
		},
		{
			// Relic of Sauron. A LITERAL count that was bailing only
			// because "any combination" sits in a quantity guard's
			// marker list.
			label:      "Relic of Sauron (literal two)",
			text:       "add two mana in any combination of {u}, {b}, and/or {r}",
			wantTotal:  2,
			wantColors: map[string]int{"U": 1, "B": 1},
		},
		{
			label:      "Vivi Ornitier (power 4)",
			text:       "add x mana in any combination of {u} and/or {r}, where x is ~'s power",
			pow:        4,
			wantTotal:  4,
			wantColors: map[string]int{"U": 2, "R": 2},
		},
		{
			label:      "Vivi Ornitier (power 1)",
			text:       "add x mana in any combination of {u} and/or {r}, where x is ~'s power",
			pow:        1,
			wantTotal:  1,
			wantColors: map[string]int{"U": 1},
		},
		{
			label:          "Xenagos (3 creatures)",
			text:           "add x mana in any combination of {r} and/or {g}, where x is the number of creatures you control",
			pow:            2,
			extraCreatures: 2, // +the source itself = 3
			wantTotal:      3,
			wantColors:     map[string]int{"R": 2, "G": 1},
		},
	}

	for _, c := range cases {
		gs, p := r109ManaFixture(t, c.label, c.text, c.pow)
		for i := 0; i < c.extraCreatures; i++ {
			addCreature(gs, 0, c.label+" extra", 1, 1)
		}
		if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
			t.Errorf("%s: ActivateAbility: %v", c.label, err)
			continue
		}
		s := gs.Seats[0]
		EnsureTypedPool(s)
		got := map[string]int{"W": s.Mana.W, "U": s.Mana.U, "B": s.Mana.B,
			"R": s.Mana.R, "G": s.Mana.G, "C": s.Mana.C}
		t.Logf("%-34s total=%d typed=%s", c.label, s.Mana.Total(), manaSummary(s))

		if s.Mana.Total() != c.wantTotal {
			t.Errorf("%s: total mana = %d, want %d (typed %s)",
				c.label, s.Mana.Total(), c.wantTotal, manaSummary(s))
		}
		// STAMP THE RAW FLAG: assert Any is EMPTY. If the fallback ran,
		// the count assertion alone could pass vacuously on a 1-mana case.
		if s.Mana.Any != 0 {
			t.Errorf("%s: Any = %d, want 0 — the one-generic fallback ran",
				c.label, s.Mana.Any)
		}
		for col, want := range c.wantColors {
			if got[col] != want {
				t.Errorf("%s: %s = %d, want %d (typed %s)",
					c.label, col, got[col], want, manaSummary(s))
			}
		}
	}
}

// TestAddManaVariableCount_r109_StillBails pins the OTHER direction: the
// shapes whose count is genuinely not derivable must KEEP the legacy
// one-generic fallback rather than get a narrowed colour.
func TestAddManaVariableCount_r109_StillBails(t *testing.T) {
	cases := []struct{ label, text string }{
		// Storage lands — X is the number of storage counters removed as a
		// COST, which this resolution path never sees.
		{"Saltcrusted Steppe", "add x mana in any combination of {g} and/or {w}"},
		// Grand Warlord Radha — "that much" binds to a preceding clause.
		{"Grand Warlord Radha", "add that much mana in any combination of {r} and/or {g}"},
		// Leyline Immersion — a literal five, but CR §106.6 spend
		// restriction is not modelled, so decline rather than mint more
		// unrestricted mana than the fallback already does.
		{"Leyline Immersion", "add five mana in any combination of {w}, {u}, {b}, {r} and/or {g}. spend this mana only to cast spells."},
	}
	for _, c := range cases {
		gs, p := r109ManaFixture(t, c.label, c.text, 3)
		if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
			t.Errorf("%s: ActivateAbility: %v", c.label, err)
			continue
		}
		s := gs.Seats[0]
		EnsureTypedPool(s)
		t.Logf("%-24s typed=%s  (expect the legacy Any:1 fallback)", c.label, manaSummary(s))
		if s.Mana.Any != 1 || s.Mana.Total() != 1 {
			t.Errorf("%s: want the untouched fallback Any:1, got %s", c.label, manaSummary(s))
		}
	}
}
