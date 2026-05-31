package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestGlissa_OpponentCreatureDeathReturnsArtifactToHand(t *testing.T) {
	gs := newGame(t, 2)
	glissa := addPerm(gs, 0, "Glissa, the Traitor", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		makeArtifact("Sol Ring", 0, 1),
		makeArtifact("Wurmcoil Engine", 0, 6),
	}
	oppDying := addPerm(gs, 1, "Opp Creature", "creature")
	glissaCreatureDies(gs, glissa, map[string]interface{}{"perm": oppDying})
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.DisplayName() == "Wurmcoil Engine" {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("expected highest-CMC artifact (Wurmcoil) in hand, got %v", handNames(gs.Seats[0].Hand))
	}
}

func TestGlissa_DoesNotFireOnSameSideCreatureDeath(t *testing.T) {
	gs := newGame(t, 2)
	glissa := addPerm(gs, 0, "Glissa, the Traitor", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{makeArtifact("Sol Ring", 0, 1)}
	myDying := addPerm(gs, 0, "Llanowar Elves", "creature")
	glissaCreatureDies(gs, glissa, map[string]interface{}{"perm": myDying})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Glissa must not fire on same-side death (\"creature an opponent controls dies\")")
	}
}

func TestGlissa_NoopWithNoArtifactInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	glissa := addPerm(gs, 0, "Glissa, the Traitor", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		newCardWithTypes("Lightning Bolt", 0, "instant"),
	}
	oppDying := addPerm(gs, 1, "Opp Creature", "creature")
	glissaCreatureDies(gs, glissa, map[string]interface{}{"perm": oppDying})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Glissa should no-op with no artifact in graveyard")
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("graveyard should be unchanged")
	}
}

func TestGlissa_OnlyScansControllerGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	glissa := addPerm(gs, 0, "Glissa, the Traitor", "creature")
	gs.Seats[1].Graveyard = []*gameengine.Card{makeArtifact("Sol Ring", 1, 1)}
	oppDying := addPerm(gs, 1, "Opp Creature", "creature")
	glissaCreatureDies(gs, glissa, map[string]interface{}{"perm": oppDying})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Glissa must only return from her controller's graveyard, not opponent's")
	}
}
