package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Live-grinder §601.2f-h firespot (same class as Hashaton 47ce48ac): activating
// Glen Elendra Archmage's "{U}, Sacrifice this creature: counter target
// noncreature spell" over-pays. Glen Elendra is a WIZARD with persist; when it
// sacrifices itself as the activation cost it dies and persist returns it
// INLINE (mid-cost-payment, inside the activation's legality window). The
// re-entry fires Inalla, Archmage Ritualist's eminence trigger, which pays {1}
// for a token copy — and that {1} was deducted without crediting
// gs.Legality.NoteManaSpend, so the activation window's pool delta read 2 spent
// against an announced {U}=1 ⇒ false "over-paid (double-deduction)".
func TestGlenElendra_InallaEminence_NotFlaggedAsOverpay(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn — they have a spell on the stack
	gs.Phase = "main"
	gs.Legality = gameengine.NewLegalityValidator(1)

	addPerm(gs, 0, "Inalla, Archmage Ritualist", "legendary", "creature", "wizard")

	// Glen Elendra Archmage — Wizard with persist + the {U}+sac counter ability.
	glen := &gameengine.Card{
		Name:          "Glen Elendra Archmage",
		Owner:         0,
		Types:         []string{"creature", "faerie", "wizard"},
		BasePower:     2,
		BaseToughness: 1,
		AST: &gameast.CardAST{
			Name: "Glen Elendra Archmage",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flying"},
				&gameast.Activated{
					Cost: gameast.Cost{
						Mana:      &gameast.ManaCost{Symbols: []gameast.ManaSymbol{{Color: []string{"U"}}}},
						Sacrifice: &gameast.Filter{Base: "this"},
					},
					Effect: &gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}},
					Raw:    "{u}, sacrifice this creature: counter target noncreature spell",
				},
				&gameast.Keyword{Name: "persist"},
			},
		},
	}
	glenPerm := &gameengine.Permanent{Card: glen, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{}, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, glenPerm)

	gs.Seats[0].ManaPool = 3
	gameengine.EnsureTypedPool(gs.Seats[0])

	target := &gameengine.StackItem{Controller: 1, Card: &gameengine.Card{Name: "Divination", Owner: 1, Types: []string{"sorcery"}}}
	gameengine.PushStackItem(gs, target)

	if err := gameengine.ActivateAbility(gs, 0, glenPerm, 1,
		[]gameengine.Target{{Kind: gameengine.TargetKindStackItem, Stack: target}}); err != nil {
		t.Fatalf("activate Glen Elendra failed: %v", err)
	}

	// Inalla must actually have fired and paid (else the test passes vacuously).
	paid := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "inalla_eminence_copy" || (ev.Source == "Inalla, Archmage Ritualist" && ev.Kind == "percard") {
			paid = true
		}
	}
	if !paid {
		// Fall back: detect the token copy on the battlefield.
		for _, p := range gs.Seats[0].Battlefield {
			if p != nil && p.Card != nil && p.IsToken() {
				paid = true
			}
		}
	}
	if !paid {
		t.Fatal("Inalla eminence did not fire on Glen Elendra's persist re-entry — fixture did not exercise the aux-payment path")
	}

	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "601.2f-h" {
			t.Errorf("Inalla's eminence {1} payment misread as Glen Elendra activation over-pay: %s", lv.Detail)
		}
	}
}
