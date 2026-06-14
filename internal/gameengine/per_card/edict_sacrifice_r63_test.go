package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 EDICT audit (per_card surface): Plaguecrafter planeswalker mode + the
// Tergrid steal interaction.

// (a) Plaguecrafter: "each player sacrifices a creature OR PLANESWALKER, or
// discards." A player whose only nonland is a planeswalker must sacrifice the
// planeswalker — NOT fall through to a discard (the old handler ignored
// planeswalkers entirely).
func TestEdict_Plaguecrafter_SacrificesPlaneswalker(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	plague := addPerm(gs, 0, "Plaguecrafter", "creature")
	// Seat 1 has a planeswalker and NO creature, plus a card in hand.
	pw := addPerm(gs, 1, "Liliana of the Veil", "planeswalker")
	gs.Seats[1].Hand = []*gameengine.Card{{Name: "Some Card", Owner: 1}}
	handBefore := len(gs.Seats[1].Hand)

	plaguecrafterETB(gs, plague)

	for _, p := range gs.Seats[1].Battlefield {
		if p == pw {
			t.Fatal("Plaguecrafter must let a player with only a planeswalker sacrifice it")
		}
	}
	if len(gs.Seats[1].Hand) != handBefore {
		t.Fatalf("player sacrificed the planeswalker, so must NOT also discard; hand %d -> %d", handBefore, len(gs.Seats[1].Hand))
	}
}

// (g) Tergrid: when an OPPONENT sacrifices a nontoken permanent, it goes onto
// Tergrid's controller's battlefield.
func TestEdict_Tergrid_StealsSacrificedNontoken(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	stampCreaturePT(addPerm(gs, 0, "Tergrid, God of Fright", "creature", "legendary", "god"), 4, 4)
	victim := stampCreaturePT(addPerm(gs, 1, "Sengir Vampire", "creature"), 4, 4) // opponent nontoken

	gameengine.SacrificePermanent(gs, victim, "forced_sacrifice")
	gameengine.StateBasedActions(gs)

	onTergridSide := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Sengir Vampire" {
			onTergridSide = true
		}
	}
	if !onTergridSide {
		t.Fatal("Tergrid must put the opponent's sacrificed nontoken onto YOUR battlefield")
	}
	// And it must NOT be left in the opponent's graveyard.
	for _, c := range gs.Seats[1].Graveyard {
		if c.Name == "Sengir Vampire" {
			t.Fatal("stolen card must leave the opponent's graveyard")
		}
	}
}

// (g) Tergrid does NOT steal a TOKEN (it ceases on sacrifice; "nontoken" only).
func TestEdict_Tergrid_DoesNotStealToken(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	stampCreaturePT(addPerm(gs, 0, "Tergrid, God of Fright", "creature", "legendary", "god"), 4, 4)
	tok := gameengine.CreateCreatureToken(gs, 1, "Zombie Token", []string{"creature", "zombie"}, 2, 2)

	gameengine.SacrificePermanent(gs, tok, "forced_sacrifice")
	gameengine.StateBasedActions(gs)

	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Zombie Token" {
			t.Fatal("Tergrid must NOT steal a sacrificed TOKEN (nontoken only)")
		}
	}
}
