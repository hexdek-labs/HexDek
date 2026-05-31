package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func makeArtifact(name string, owner, cmc int) *gameengine.Card {
	c := newCardWithTypes(name, owner, "artifact")
	c.Types = append(c.Types, "cmc:"+intToStr(cmc))
	return c
}

func TestSharuum_ETBReanimatesHighestCMCArtifact(t *testing.T) {
	gs := newGame(t, 2)
	sharuum := addPerm(gs, 0, "Sharuum the Hegemon", "artifact", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		makeArtifact("Sol Ring", 0, 1),
		makeArtifact("Wurmcoil Engine", 0, 6),
		makeArtifact("Lightning Greaves", 0, 2),
	}
	sharuumETB(gs, sharuum)
	foundWurm := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Wurmcoil Engine" {
			foundWurm = true
		}
	}
	if !foundWurm {
		t.Errorf("expected Wurmcoil Engine (highest CMC) reanimated, got %v", permNames(gs.Seats[0].Battlefield))
	}
}

func TestSharuum_ETBNoopWithNoArtifactInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	sharuum := addPerm(gs, 0, "Sharuum the Hegemon", "artifact", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		newCardWithTypes("Llanowar Elves", 0, "creature"),
	}
	preCount := len(gs.Seats[0].Battlefield)
	sharuumETB(gs, sharuum)
	if len(gs.Seats[0].Battlefield) != preCount {
		t.Errorf("expected no battlefield change with no artifacts in graveyard")
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("graveyard should be unchanged")
	}
}

func TestSharuum_ETBOnlyScansControllerGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	sharuum := addPerm(gs, 0, "Sharuum the Hegemon", "artifact", "creature")
	gs.Seats[1].Graveyard = []*gameengine.Card{makeArtifact("Sol Ring", 1, 1)}
	sharuumETB(gs, sharuum)
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Sol Ring" {
			t.Errorf("Sharuum must only reanimate from controller's graveyard, not opponent's")
		}
	}
}
