package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestLegality_ExtortPayment_NotFlaggedAsOverpay replays the judge r62
// +1 over-pay finding (seed 840043 game 84 seat 2 ×8 casts) through the
// real pipeline: the caster controls an extort permanent (the repro
// seat's commander, Sorin of House Markov, carries extort on both
// faces), casts a spell, and the engine's cast-trigger observer pays
// the optional {W/B} extort cost from the same pool inside the
// validator's Begin..Finish window (cast_counts.go). That payment is a
// legitimate CR §702.101 trigger cost, NOT part of the spell's §601.2f
// announced total — pre-fix the cost-paid check read every such cast
// as "announced N, spent N+1, over-paid". The NoteManaSpend hook now
// credits auxiliary in-window payments symmetrically to NoteManaAdd.
func TestLegality_ExtortPayment_NotFlaggedAsOverpay(t *testing.T) {
	gs := legalityFixture(t)
	gs.Seats[0].ManaPool = 3 // spell {2} + extort {1}

	extortPerm := &Permanent{
		Card: &Card{
			Name: "Extort Cleric", Owner: 0,
			Types: []string{"creature", "cleric"},
			AST: &gameast.CardAST{Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "extort"},
			}},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, extortPerm)

	card := instantSpellCard("Taxed Bolt", 2, nil)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// The extort payment must actually have happened (pool fully spent:
	// 2 for the spell + 1 for extort) — this test must not pass by the
	// extort trigger silently not firing.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected pool 0 after spell{2}+extort{1}, got %d — extort did not pay, fixture broken",
			gs.Seats[0].ManaPool)
	}
	paid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "extort_trigger" {
			paid = true
		}
	}
	if !paid {
		t.Fatal("no extort_trigger event — fixture did not exercise the aux-payment path")
	}

	if got := violationsByRule(gs.Legality, "601.2f-h"); len(got) != 0 {
		t.Errorf("legitimate extort payment flagged as cost violation: %v", got)
	}
}

// TestLegality_NoteManaSpend_UnitArithmetic pins the check arithmetic
// directly: announced 2, pool delta 3, aux-spent 1 → clean; the same
// shape with aux-spent 0 must still flag (the hook must not blanket-
// suppress real over-pays).
func TestLegality_NoteManaSpend_UnitArithmetic(t *testing.T) {
	withAux := &LegalityObservation{
		Kind: "cast", Seat: 2, TurnAtAnnounce: 10,
		Card:                     &Card{Name: "Contested Game Ball", Types: []string{"artifact", "cost:2"}},
		PoolBefore:               3,
		PoolAfter:                0,
		BaseCostAtAnnounce:       2,
		AuxManaSpentDuringWindow: 1, // extort
		Item:                     &StackItem{},
	}
	if v := checkLegalityCostPaid(nil, withAux); len(v) != 0 {
		t.Errorf("aux-credited cast still flagged: %v", v)
	}

	withoutAux := &LegalityObservation{
		Kind: "cast", Seat: 2, TurnAtAnnounce: 10,
		Card:               &Card{Name: "Contested Game Ball", Types: []string{"artifact", "cost:2"}},
		PoolBefore:         3,
		PoolAfter:          0,
		BaseCostAtAnnounce: 2,
		Item:               &StackItem{},
	}
	if v := checkLegalityCostPaid(nil, withoutAux); len(v) != 1 {
		t.Errorf("real +1 over-pay no longer flagged — hook over-suppresses: %v", v)
	}
}
