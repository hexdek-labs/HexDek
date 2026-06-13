package gameengine

// r63 PROGRESSION saturation finding: the drawOne chokepoint fired
// card_drawn only through the per_card registry (FireCardTrigger) — no walk
// consulted the AST, so "whenever you draw a card," payoffs were silent
// unless the card had a bespoke handler. FireDrawCardASTTriggers is now
// driven from drawOne; these tests pin the real chokepoint.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func drawPayoffBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "draw_card"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Draw Payoff", 2, 2, "enchantment")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestDrawCard_FiresThroughDrawOne(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	drawPayoffBearer(gs, 0, "whenever you draw a card, you gain 1 life")

	before := gs.Seats[0].Life
	gs.drawOne(0)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("you-draw payoff must fire through drawOne: gained %d life, want 1", got)
	}
}

func TestDrawCard_ControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 1, "Opp Card")
	drawPayoffBearer(gs, 0, "whenever you draw a card, you gain 1 life")

	before := gs.Seats[0].Life
	gs.drawOne(1) // the OPPONENT draws
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("you-draw payoff must not fire on an opponent's draw: gained %d", got)
	}
}

func TestDrawCard_DispatchSilentWithoutTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A")
	addBattlefield(gs, 0, "Plain Permanent", 2, 2, "enchantment")

	before := gs.Seats[0].Life
	FireDrawCardASTTriggers(gs, 0)
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("no draw trigger present — must stay silent: life delta %d", got)
	}
}
