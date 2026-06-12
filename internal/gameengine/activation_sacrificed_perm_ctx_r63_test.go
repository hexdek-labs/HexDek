package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — sacrifice-cost victim threading into the per_card activation ctx.
//
// Lyzolda-class abilities ("{B}{R}, Sacrifice a creature: Lyzolda deals
// 2 damage to any target if the sacrificed creature was red...") need
// the sacrificed permanent's characteristics at resolution, but the
// dispatcher paid the cost in ActivateAbility and never told the
// handler what died. The victim now rides StackItem.CostMeta
// ["sacrificed_perm"] onto the resolution ctx (and the inline
// mana-ability ctx, for Ashnod's Altar-class hooks).

// sacCostPerm builds a battlefield permanent whose ability 0 sacrifices
// a creature as its activation cost.
func sacCostPerm(gs *GameState, seat int, name string, effect gameast.Effect) *Permanent {
	card := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{
						Sacrifice: &gameast.Filter{Base: "creature"},
					},
					Effect: effect,
				},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

func TestActivateAbility_SacrificeCost_VictimReachesHandlerCtx(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 10

	// Fodder is placed FIRST: FindSacrificeTarget's no-Hat fallback is
	// deterministic first-match over battlefield order, and this test pins
	// the ctx threading, not the victim-selection policy.
	fodder := addBattlefield(gs, 0, "Red Fodder", 1, 1, "creature")
	fodder.Card.Colors = []string{"R"}
	src := sacCostPerm(gs, 0, "Lyzolda, the Blood Witch", &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 2},
		Target: gameast.Filter{Base: "player"},
	})

	var gotCtx map[string]interface{}
	prev := ActivatedHook
	ActivatedHook = func(_ *GameState, hookSrc *Permanent, idx int, ctx map[string]interface{}) {
		if hookSrc == src && idx == 0 {
			if _, fromStack := ctx["from_stack"]; fromStack {
				gotCtx = ctx
			}
		}
	}
	defer func() { ActivatedHook = prev }()

	if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	if gotCtx == nil {
		t.Fatal("resolution-path ActivatedHook was never invoked")
	}
	v, ok := gotCtx["sacrificed_perm"].(*Permanent)
	if !ok || v == nil {
		t.Fatalf("ctx[\"sacrificed_perm\"] missing or wrong type: %#v", gotCtx["sacrificed_perm"])
	}
	if v.Card == nil || v.Card.Name != "Red Fodder" {
		t.Fatalf("expected sacrificed Red Fodder in ctx, got %v", v.Card)
	}
	if len(v.Card.Colors) != 1 || v.Card.Colors[0] != "R" {
		t.Fatalf("victim characteristics must survive into ctx, got colors %v", v.Card.Colors)
	}
}

func TestActivateAbility_NoSacrificeCost_CtxOmitsVictim(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].ManaPool = 10

	src := makeArtifactPerm(gs, 0, "Shock Rod", &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 3},
		Target: gameast.Filter{Base: "player"},
	}, true)

	var gotCtx map[string]interface{}
	prev := ActivatedHook
	ActivatedHook = func(_ *GameState, hookSrc *Permanent, idx int, ctx map[string]interface{}) {
		if hookSrc == src && idx == 0 {
			gotCtx = ctx
		}
	}
	defer func() { ActivatedHook = prev }()

	if err := ActivateAbility(gs, 0, src, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}
	if gotCtx == nil {
		t.Fatal("ActivatedHook was never invoked")
	}
	if _, present := gotCtx["sacrificed_perm"]; present {
		t.Fatal("ctx must not carry sacrificed_perm when no sacrifice cost was paid")
	}
}
