package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Firespot r63 (loki seed 1337 game 741): an overloaded spell whose overload
// cost is changed by a §601.2f cost modifier must record the EFFECTIVE overload
// cost the engine actually paid, so the §601.2f-h legality check sees
// announced == spent instead of flagging a false under-payment. Electrickery's
// overload {1}{R} (=2) was reduced to 1 by Artist's Talent; the engine
// correctly paid 1 but the checker re-derived the expected from the RAW
// OverloadCost (=2) and flagged "announced 2 but 1 spent".
func TestOverload_CostModifier_LegalityMatchesEffective(t *testing.T) {
	gs := newKWCombatGame4P(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Legality = NewLegalityValidator(7)
	gs.Seats[0].ManaPool = 5

	// A reducer the caster (seat 0) controls: instants cost {1} less.
	reducer := addKWCombatBattlefield(gs, 0, "Cost Reducer", 0, 1, "enchantment")
	reducer.Card.AST = &gameast.CardAST{
		Name: "Cost Reducer",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "colored_cost_reduce",
				Args:    []interface{}{"instant", "{1}"},
			}},
		},
	}

	a := addKWCombatBattlefield(gs, 1, "A", 1, 2, "creature")
	b := addKWCombatBattlefield(gs, 2, "B", 1, 2, "creature")
	c := addKWCombatBattlefield(gs, 3, "C", 1, 2, "creature")

	dmg := &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
		Target: gameast.Filter{Base: "creature", Targeted: true},
	}
	card := &Card{
		Name:           "Electrickery",
		Owner:          0,
		ManaCostString: "{R}",
		Types:          []string{"instant", "cost:1"},
		AST: &gameast.CardAST{
			Name: "Electrickery",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "overload", Args: []interface{}{"{1}{r}"}},
				&gameast.Activated{Effect: dmg},
			},
		},
	}
	gs.Seats[0].Hand = []*Card{card}

	poolBefore := gs.Seats[0].ManaPool
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Overload {1}{R}=2 reduced by {1} → exactly 1 actually deducted.
	if spent := poolBefore - gs.Seats[0].ManaPool; spent != 1 {
		t.Fatalf("expected effective overload cost 1 (overload 2 minus {1} reduction) deducted, pool spent %d", spent)
	}

	// Confirm the cast was OVERLOADED: the single-target damage fanned out to
	// every creature (CR §702.96), not just one.
	for _, p := range []*Permanent{a, b, c} {
		if p.MarkedDamage != 1 {
			t.Fatalf("overload should fan damage to %s (got %d) — cast was not overloaded", p.Card.Name, p.MarkedDamage)
		}
	}

	// THE FIX: no false §601.2f-h under-payment. The validator must compare
	// against the EFFECTIVE overload cost (1, stamped as overload_cost), not
	// the raw printed OverloadCost (2). Before the fix this flagged
	// "announced total 2 but pool delta shows 1 spent".
	if v := violationsByRule(gs.Legality, "601.2f-h"); len(v) != 0 {
		t.Fatalf("§601.2f-h false positive on modifier-reduced overload: %+v", v)
	}
}
