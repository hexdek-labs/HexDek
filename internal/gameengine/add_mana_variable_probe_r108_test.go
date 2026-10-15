package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r108 probe — the variable-quantity add_mana_effect bailout.
//
// Taking (N5), Vivi Ornitier. resolveAddManaEffect bails on any args text
// containing "x mana", "any combination", "any number", "for each" or
// "equal to" (addManaEffectVariable, modkind_add_mana_effect.go:35), and the
// caller's fallback at resolve_helpers.go:1568 adds ONE GENERIC mana.
//
// Measured here at the production entry point (ActivateAbility), from the
// REAL AST arg text the exporter emits, with a positive control:
//
//	CONTROL  add {u}{r}          -> 2 mana, U:1 R:1      correct
//	Vivi {0} (power 4)           -> 1 mana, Any:1        want 4x {U}/{R}
//	Saltcrusted Steppe           -> 1 mana, Any:1        want X in {G}/{W}
//	Grand Warlord Radha          -> 1 mana, Any:1        want N in {R}/{G}
//	Axebane Guardian             -> 1 mana, Any:1        want X in any colors
//
// BOTH halves are wrong, and they are independent: the COUNT is genuinely
// variable, but the COLORS are right there in the text and the structured
// handler already extracts them correctly — the bailout throws them away too.
// 25 of the 88 corpus cards carrying an add_mana_effect node land here.
//
// Characterization only: this probe asserts nothing but the control, so it
// records today's behaviour without freezing it. See issue 17.
func TestViviManaProbe_r108(t *testing.T) {
	mk := func(name, argText string, tap bool, pow int) (*GameState, *Permanent) {
		gs := newFixtureGame(t)
		p := addCreature(gs, 0, name, pow, pow)
		p.Card.AST = &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{Tap: tap},
					Effect: &gameast.ModificationEffect{
						ModKind: "add_mana_effect",
						Args:    []any{argText},
					},
				},
			},
		}
		gs.Active = 0
		gs.Seats[0].ManaPool = 0
		return gs, p
	}

	type row struct {
		label, text string
		pow         int
	}
	rows := []row{
		// POSITIVE CONTROL: a plain two-symbol filter-land shape the
		// structured handler is known to handle. If this one is wrong the
		// probe is dead and the rest means nothing.
		{"CONTROL  add {u}{r}", "add {u}{r}", 0},
		{"Vivi {0} (power 4)", "add x mana in any combination of {u} and/or {r}, where x is ~'s power", 4},
		{"Saltcrusted Steppe", "add x mana in any combination of {g} and/or {w}", 3},
		{"Grand Warlord Radha", "add that much mana in any combination of {r} and/or {g}", 3},
		{"Axebane Guardian", "add x mana in any combination of colors, where x is the number of creatures you control", 3},
	}

	for _, r := range rows {
		gs, p := mk(r.label, r.text, false, r.pow)
		before := gs.Seats[0].ManaPool
		err := ActivateAbility(gs, 0, p, 0, nil)
		s := gs.Seats[0]
		EnsureTypedPool(s)
		t.Logf("%-26s power=%d  err=%v  ManaPool %d -> %d   typed=%s",
			r.label, r.pow, err, before, s.ManaPool, manaSummary(s))
	}
}

func manaSummary(s *Seat) string {
	out := ""
	for _, pr := range []struct {
		c string
		n int
	}{{"W", s.Mana.W}, {"U", s.Mana.U}, {"B", s.Mana.B}, {"R", s.Mana.R}, {"G", s.Mana.G}, {"C", s.Mana.C}, {"Any", s.Mana.Any}} {
		if pr.n > 0 {
			out += pr.c + ":" + itoaBatch(pr.n) + " "
		}
	}
	if out == "" {
		return "(none)"
	}
	return out
}
