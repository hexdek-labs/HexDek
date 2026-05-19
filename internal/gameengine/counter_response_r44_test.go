package gameengine

// r44 game-404 follow-up to the Adric counter-response fix
// (loki_r44_adric_test.go). The Adric fix landed an isPermanentSpell
// early-return in counterSpellEffect, but Layout-1 (Activated AST node)
// would still match on instants/sorceries with a real activation cost —
// which would mean a hypothetical instant whose printed activated
// ability is a counter (rare in the corpus but parser-shaped that way
// for a handful of cards) would be picked as a hand-castable counter.
//
// Game 404 (seed 41) surfaced this with Disruptive Pitmage, a CREATURE
// with `{T}: Counter target spell unless its controller pays {1}`.
// The isPermanentSpell gate already covers Pitmage. This file pins the
// **strict refinement**: even for an instant, only Layout-1 nodes with
// an EMPTY cost (parser-artifact spell-body shape used by Summon the
// School, Divergent Growth, Eldrazi Confluence) qualify. A non-empty
// cost means the Activated ability is a real activation that only
// functions on the battlefield, even on an instant/sorcery — never a
// hand-castable counterspell.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestCounterSpellEffect_InstantWithRealCostNotPickable — hypothetical
// instant whose body is parsed as Activated-with-real-cost is not a
// hand-castable counterspell. Mirrors the empty-cost gate from r41's
// collectSpellEffect fix (Cerulean Sphinx).
func TestCounterSpellEffect_InstantWithRealCostNotPickable(t *testing.T) {
	hypothetical := &Card{
		Name:  "Hypothetical Tap Counterspell",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Hypothetical Tap Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Tap: true},
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}},
				},
			},
		},
	}
	if got := counterSpellEffect(hypothetical); got != nil {
		t.Fatalf("activated counterspell with a real cost must be nil; got %T", got)
	}
}

// TestCounterSpellEffect_InstantWithEmptyCostKeepsBody — instants
// whose body is parser-shaped as Activated-with-empty-cost still match.
// This is the legitimate use of Layout-1 (Summon the School-style
// parser artifacts).
func TestCounterSpellEffect_InstantWithEmptyCostKeepsBody(t *testing.T) {
	body := &gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}}
	card := &Card{
		Name:  "Test Empty-Cost Counterspell",
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Test Empty-Cost Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{Cost: gameast.Cost{}, Effect: body},
			},
		},
	}
	if got := counterSpellEffect(card); got == nil {
		t.Fatal("empty-cost Activated counterspell body must still match")
	}
}

// TestCounterSpellEffect_PitmageRejected — game 404's concrete carrier:
// a creature with `{T}: Counter target spell`. Covered by the
// isPermanentSpell early-return from the Adric fix, but pinned here so
// the Pitmage-shaped case stays nailed even if the gate is refactored.
func TestCounterSpellEffect_PitmageRejected(t *testing.T) {
	pitmage := &Card{
		Name:  "Disruptive Pitmage",
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Disruptive Pitmage",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{Tap: true},
					Effect: &gameast.CounterSpell{
						Target: gameast.Filter{Base: "spell"},
					},
				},
			},
		},
	}
	if got := counterSpellEffect(pitmage); got != nil {
		t.Fatalf("Disruptive Pitmage is a creature — must not be a hand-castable counter; got %T", got)
	}
}
