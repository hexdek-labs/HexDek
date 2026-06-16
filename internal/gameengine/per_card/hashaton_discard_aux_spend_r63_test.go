package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestHashaton_DiscardCostAuxSpend_NotFlaggedAsActivationOverpay replays the
// live grinder firespot (LEGALITY §601.2f-h): "activate Tireless Tribe#0
// over-paid (double-deduction) — announced total 0 but pool delta shows 3
// spent (before=4 after=1)".
//
// Mechanism: Tireless Tribe's activated ability is "Discard a card: this
// creature gets +0/+4" — a ZERO-mana cost. Paying that cost runs DiscardCard,
// which fires the "card_discarded" trigger INSIDE the activation's legality
// window. Hashaton, Scarab's Fist's card_discarded trigger ("whenever you
// discard a creature card, you may pay {2}{U}: copy it as a 4/4 Zombie")
// resolves inline and pays {2}{U} (3 mana) from the same pool. That payment
// is a legitimate CR §702 optional trigger cost, NOT part of Tireless Tribe's
// §601.2f announced total — but pre-fix hashatonDiscardTrigger debited the
// pool without crediting gs.Legality.NoteManaSpend, so the §601.2f-h cost-paid
// check read the activation's pool delta (4 -> 1) against its announced total
// (0) and false-flagged a double-deduction. Same extort/ward/Esper-Sentinel
// aux-spend class (legality.go NoteManaSpend).
func TestHashaton_DiscardCostAuxSpend_NotFlaggedAsActivationOverpay(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Legality = gameengine.NewLegalityValidator(1)

	addPerm(gs, 0, "Hashaton, Scarab's Fist", "legendary", "creature")

	// Tireless Tribe: real AST shape — "Discard a card: +0/+4 EOT", no mana.
	one := 1
	tribe := &gameengine.Card{
		Name:  "Tireless Tribe",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Tireless Tribe",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Discard: &one},
					Effect: &gameast.Buff{Power: 0, Toughness: 4, Duration: "until_end_of_turn"},
				},
			},
		},
	}
	tribePerm := &gameengine.Permanent{
		Card: tribe, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tribePerm)

	// A creature card in hand for the discard cost (Hashaton gates on creature).
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"},
	})

	// 4 floating mana: enough for Hashaton's {2}{U} (3) with 1 to spare.
	gameengine.AddMana(gs, gs.Seats[0], "C", 4, "Test Rock")

	before := gameengine.EnsureTypedPool(gs.Seats[0]).Total()
	if before != 4 {
		t.Fatalf("fixture: want pool 4 before activation, got %d", before)
	}

	if err := gameengine.ActivateAbility(gs, 0, tribePerm, 0, nil); err != nil {
		t.Fatalf("activate Tireless Tribe discard ability failed: %v", err)
	}

	// Hashaton must actually have fired and paid — the test must not pass by
	// the trigger silently not firing.
	paid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "create_token" && ev.Source == "Hashaton, Scarab's Fist" {
			paid = true
		}
	}
	if !paid {
		t.Fatal("Hashaton did not fire its discard-copy trigger — fixture did not exercise the aux-payment path")
	}
	if got := gs.Seats[0].ManaPool; got != 1 {
		t.Fatalf("want pool 1 after Hashaton's {2}{U}=3 aux payment from 4, got %d", got)
	}

	// The activation's announced cost is 0 (discard only). The 3-mana drain is
	// Hashaton's aux trigger cost, not the activation over-paying.
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("Hashaton aux discard-payment misread as activation over-pay: %s", v.Detail)
		}
	}
}
