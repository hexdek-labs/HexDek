package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// percard_cover_af_batch4_test.go — regression pins for A–F batch 4:
// Evacuation, Aetherize, Dingus Egg, Cathartic Reunion.

func afCreaturesOn(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsCreature() {
			n++
		}
	}
	return n
}

func TestEvacuation_ReturnsAllCreaturesToHands(t *testing.T) {
	gs := newGame(t, 2)
	a := addPerm(gs, 0, "Bear", "creature", "bear")
	a.Card.BasePower, a.Card.BaseToughness = 2, 2
	b := addPerm(gs, 1, "Elf", "creature", "elf")
	b.Card.BasePower, b.Card.BaseToughness = 1, 1
	addPerm(gs, 0, "Sol Ring", "artifact") // noncreature stays

	evacuationResolve(gs, stackItem("Evacuation", 0))

	if afCreaturesOn(gs, 0) != 0 || afCreaturesOn(gs, 1) != 0 {
		t.Fatalf("all creatures should be bounced; s0=%d s1=%d", afCreaturesOn(gs, 0), afCreaturesOn(gs, 1))
	}
	if !afCardInZone(gs.Seats[0].Hand, a.Card) || !afCardInZone(gs.Seats[1].Hand, b.Card) {
		t.Error("creatures should be in their owners' hands")
	}
	// Noncreature artifact untouched.
	artStill := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && !p.IsCreature() {
			artStill = true
		}
	}
	if !artStill {
		t.Error("Evacuation must only bounce creatures; the artifact should remain")
	}
}

func TestAetherize_ReturnsOnlyAttackingCreatures(t *testing.T) {
	gs := newGame(t, 2)
	atk1 := addPerm(gs, 1, "Attacker1", "creature", "beast")
	atk1.Card.BasePower, atk1.Card.BaseToughness = 3, 3
	atk1.Flags["attacking"] = 1
	atk2 := addPerm(gs, 1, "Attacker2", "creature", "beast")
	atk2.Card.BasePower, atk2.Card.BaseToughness = 4, 4
	atk2.Flags["attacking"] = 1
	idle := addPerm(gs, 1, "Idle", "creature", "wall")
	idle.Card.BasePower, idle.Card.BaseToughness = 0, 4

	aetherizeResolve(gs, stackItem("Aetherize", 0))

	if afOnBF(gs, 1, atk1.Card) || afOnBF(gs, 1, atk2.Card) {
		t.Error("attacking creatures should be bounced")
	}
	if !afOnBF(gs, 1, idle.Card) {
		t.Error("non-attacking creature should remain on the battlefield")
	}
}

func TestDingusEgg_LandToGraveyard_Deals2(t *testing.T) {
	gs := newGame(t, 2)
	egg := addPerm(gs, 0, "Dingus Egg", "artifact")
	gs.Seats[1].Life = 40

	dingusEggLandDies(gs, egg, map[string]interface{}{"from_zone": "battlefield", "owner_seat": 1})
	if gs.Seats[1].Life != 38 {
		t.Errorf("land's controller should take 2 damage (40→38), got %d", gs.Seats[1].Life)
	}

	// Not from the battlefield (e.g. hand→graveyard discard) → no damage.
	gs.Seats[1].Life = 40
	dingusEggLandDies(gs, egg, map[string]interface{}{"from_zone": "hand", "owner_seat": 1})
	if gs.Seats[1].Life != 40 {
		t.Errorf("only battlefield→graveyard should trigger Dingus Egg, got %d", gs.Seats[1].Life)
	}
}

func TestCatharticReunion_DrawsThree(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 5; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, addCard(gs, 0, "Filler", "land"))
	}
	handBefore := len(gs.Seats[0].Hand)
	catharticReunionResolve(gs, stackItem("Cathartic Reunion", 0))
	if len(gs.Seats[0].Hand) != handBefore+3 {
		t.Errorf("expected to draw 3, hand %d→%d", handBefore, len(gs.Seats[0].Hand))
	}
}
