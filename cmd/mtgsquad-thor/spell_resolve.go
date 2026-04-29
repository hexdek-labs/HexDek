package main

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runSpellResolve(_ *astload.Corpus, oracleCards []*oracleCard) []failure {
	var fails []failure

	for _, oc := range oracleCards {
		if !isInstantOrSorcery(oc) {
			continue
		}
		if oc.ast == nil {
			continue
		}

		f := testSpellResolve(oc)
		if f != nil {
			fails = append(fails, *f)
		}
	}

	return fails
}

func isInstantOrSorcery(oc *oracleCard) bool {
	for _, t := range oc.Types {
		tl := strings.ToLower(t)
		if tl == "instant" || tl == "sorcery" {
			return true
		}
	}
	return false
}

func testSpellResolve(oc *oracleCard) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			result = &failure{
				CardName:    oc.Name,
				Interaction: "spell_resolve",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			}
		}
	}()

	gs := makeSpellGameState(oc)
	if gs == nil {
		return nil
	}

	// Build stack item from the card's AST.
	card := &gameengine.Card{
		Name:   oc.Name,
		Owner:  0,
		Types:  oc.Types,
		Colors: oc.Colors,
		CMC:    oc.CMC,
		AST:    oc.ast,
	}

	// Find the first effect in the AST to use as the spell's effect.
	var eff interface{}
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			eff = ab
			break
		}
	}
	_ = eff

	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       card,
	}

	gameengine.PushStackItem(gs, item)
	gameengine.ResolveStackTop(gs)
	gameengine.StateBasedActions(gs)

	violations := gameengine.RunAllInvariants(gs)
	if len(violations) > 0 {
		return &failure{
			CardName:    oc.Name,
			Interaction: "spell_resolve",
			Invariant:   violations[0].Name,
			Message:     violations[0].Message,
		}
	}

	return nil
}

func makeSpellGameState(oc *oracleCard) *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Step:   "",
		Flags:  map[string]int{},
	}

	for i := 0; i < 4; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Flags: map[string]int{},
		}
		// Library with filler.
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name:          fmt.Sprintf("Filler %d-%d", i, j),
				Owner:         i,
				Types:         []string{"creature"},
				BasePower:     2,
				BaseToughness: 2,
			})
		}
		// Hand.
		for j := 0; j < 3; j++ {
			seat.Hand = append(seat.Hand, &gameengine.Card{
				Name:  fmt.Sprintf("HandCard %d-%d", i, j),
				Owner: i,
				Types: []string{"creature"},
			})
		}
		// Creatures on battlefield for targeting.
		for j := 0; j < 2; j++ {
			perm := &gameengine.Permanent{
				Card: &gameengine.Card{
					Name:          fmt.Sprintf("Creature %d-%d", i, j),
					Owner:         i,
					Types:         []string{"creature"},
					BasePower:     3,
					BaseToughness: 3,
				},
				Controller: i,
				Owner:      i,
				Flags:      map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, perm)
		}
		gs.Seats = append(gs.Seats, seat)
	}

	gs.Snapshot()
	return gs
}
