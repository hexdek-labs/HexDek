package per_card

import (
	"testing"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func countCats(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Cat Soldier Token" {
			n++
		}
	}
	return n
}

func brimazAttacks(gs *gameengine.GameState, brimaz *gameengine.Permanent) {
	gameengine.MarkEnteredAttacking(brimaz)
	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": brimaz, "attacker_seat": brimaz.Controller, "attacker_card": brimaz.Card,
	})
}

// (a) Doubling Season doubles a per_card token-maker's creation event (Brimaz:
// 1 Cat -> 2). The raw CreateCreatureToken loop bypassed every token doubler.
func TestDoublerToken_Brimaz_DoublingSeasonMakesTwoCats(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	ds := addPerm(gs, 0, "Doubling Season", "enchantment")
	gameengine.RegisterDoublingSeason(gs, ds)
	brimaz := stampCreaturePT(addPerm(gs, 0, "Brimaz, King of Oreskos", "creature", "legendary", "cat"), 3, 4)

	brimazAttacks(gs, brimaz)

	if got := countCats(gs, 0); got != 2 {
		t.Fatalf("Brimaz under Doubling Season must create 2 Cat tokens, got %d", got)
	}
	// And both doubled cats are attacking (per-token setup applied to copies).
	attacking := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Cat Soldier Token" && p.IsAttacking() {
			attacking++
		}
	}
	if attacking != 2 {
		t.Fatalf("both doubled cats must enter attacking, got %d attacking", attacking)
	}
}

// (c) two token doublers MULTIPLY (Brimaz: 1 -> 4 with two Doubling Seasons).
func TestDoublerToken_Brimaz_TwoDoublersQuadruple(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	for i := 0; i < 2; i++ {
		ds := addPerm(gs, 0, "Doubling Season", "enchantment")
		gameengine.RegisterDoublingSeason(gs, ds)
	}
	brimaz := stampCreaturePT(addPerm(gs, 0, "Brimaz, King of Oreskos", "creature", "legendary", "cat"), 3, 4)
	brimazAttacks(gs, brimaz)
	if got := countCats(gs, 0); got != 4 {
		t.Fatalf("Brimaz under two Doubling Seasons must create 4 Cat tokens (1*2*2), got %d", got)
	}
}

// (a) baseline: no doubler -> exactly 1 cat (no over-fire from the migration).
func TestDoublerToken_Brimaz_NoDoublerOneCat(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	brimaz := stampCreaturePT(addPerm(gs, 0, "Brimaz, King of Oreskos", "creature", "legendary", "cat"), 3, 4)
	brimazAttacks(gs, brimaz)
	if got := countCats(gs, 0); got != 1 {
		t.Fatalf("Brimaz with no doubler must create exactly 1 Cat, got %d", got)
	}
}
