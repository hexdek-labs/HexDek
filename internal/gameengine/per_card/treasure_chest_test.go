package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func gameWithSeededRNG(t *testing.T, seats int, seed int64) *gameengine.GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	return gameengine.NewGameState(seats, rng, nil)
}

func TestTreasureChest_Roll20PutsArtifactOnBattlefield(t *testing.T) {
	// Seed chosen so rand.Intn(20)+1 = 20 on first call. Verified empirically
	// via gs.Rng.Intn(20)+1; iterate until found.
	var seed int64
	for s := int64(1); s < 5000; s++ {
		r := rand.New(rand.NewSource(s))
		if r.Intn(20)+1 == 20 {
			seed = s
			break
		}
	}
	if seed == 0 {
		t.Fatal("could not find a seed producing roll=20 in [1,5000)")
	}
	gs := gameWithSeededRNG(t, 2, seed)
	chest := addPerm(gs, 0, "Treasure Chest", "artifact")
	artifact := newCardWithTypes("Sol Ring", 0, "artifact")
	noncreature := newCardWithTypes("Forest", 0, "land")
	gs.Seats[0].Library = []*gameengine.Card{noncreature, artifact}
	treasureChestActivate(gs, chest, 0, nil)

	foundSolRing := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Sol Ring" {
			foundSolRing = true
		}
	}
	if !foundSolRing {
		t.Fatalf("expected Sol Ring on battlefield from roll=20, got %v", permNames(gs.Seats[0].Battlefield))
	}
}

func TestTreasureChest_Roll10To19GainsLifeAndDraws(t *testing.T) {
	var seed int64
	for s := int64(1); s < 5000; s++ {
		r := rand.New(rand.NewSource(s))
		roll := r.Intn(20) + 1
		if roll >= 10 && roll <= 19 {
			seed = s
			break
		}
	}
	gs := gameWithSeededRNG(t, 2, seed)
	chest := addPerm(gs, 0, "Treasure Chest", "artifact")
	preLife := gs.Seats[0].Life
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("A", 0, "creature"),
		newCardWithTypes("B", 0, "creature"),
		newCardWithTypes("C", 0, "creature"),
	}
	treasureChestActivate(gs, chest, 0, nil)
	if gs.Seats[0].Life != preLife+3 {
		t.Errorf("expected life %d→%d, got %d", preLife, preLife+3, gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected 3 cards drawn into hand, got %d", len(gs.Seats[0].Hand))
	}
}

func TestTreasureChest_Roll2To9CreatesFiveTreasureTokens(t *testing.T) {
	var seed int64
	for s := int64(1); s < 5000; s++ {
		r := rand.New(rand.NewSource(s))
		roll := r.Intn(20) + 1
		if roll >= 2 && roll <= 9 {
			seed = s
			break
		}
	}
	gs := gameWithSeededRNG(t, 2, seed)
	chest := addPerm(gs, 0, "Treasure Chest", "artifact")
	preCount := len(gs.Seats[0].Battlefield)
	treasureChestActivate(gs, chest, 0, nil)
	added := len(gs.Seats[0].Battlefield) - preCount
	if added != 5 {
		t.Errorf("expected 5 new permanents (treasures) on battlefield, got %d (total %v)",
			added, permNames(gs.Seats[0].Battlefield))
	}
}

func TestTreasureChest_Roll1LosesThreeLife(t *testing.T) {
	var seed int64
	for s := int64(1); s < 5000; s++ {
		r := rand.New(rand.NewSource(s))
		if r.Intn(20)+1 == 1 {
			seed = s
			break
		}
	}
	gs := gameWithSeededRNG(t, 2, seed)
	chest := addPerm(gs, 0, "Treasure Chest", "artifact")
	preLife := gs.Seats[0].Life
	treasureChestActivate(gs, chest, 0, nil)
	if gs.Seats[0].Life != preLife-3 {
		t.Errorf("expected life %d→%d on roll=1, got %d", preLife, preLife-3, gs.Seats[0].Life)
	}
}
