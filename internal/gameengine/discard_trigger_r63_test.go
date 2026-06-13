package gameengine

// r63 PROGRESSION saturation finding: DiscardCard fired card_discarded only
// through the per_card registry — no walk consulted the AST, so "whenever you
// discard a card," payoffs were silent unless the card had a bespoke handler.
// FireDiscardASTTriggers is now driven from DiscardCard; these tests pin the
// real chokepoint.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func discardPayoffBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "discard_filtered"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Discard Payoff", 2, 2, "enchantment")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestDiscard_FiresThroughDiscardCard(t *testing.T) {
	gs := newFixtureGame(t)
	discardPayoffBearer(gs, 0, "whenever you discard a card, you gain 1 life")
	card := &Card{Name: "Junk", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)

	before := gs.Seats[0].Life
	DiscardCard(gs, card, 0)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("you-discard payoff must fire through DiscardCard: gained %d life, want 1", got)
	}
}

func TestDiscard_ControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	discardPayoffBearer(gs, 0, "whenever you discard a card, you gain 1 life")
	card := &Card{Name: "Opp Junk", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, card)

	before := gs.Seats[0].Life
	DiscardCard(gs, card, 1) // the opponent discards
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("you-discard payoff must not fire on an opponent's discard: gained %d", got)
	}
}
