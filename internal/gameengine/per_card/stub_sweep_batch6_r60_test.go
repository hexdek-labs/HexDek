package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestMagda_SacFiveTreasuresTutorsArtifactOrDragon pins the activated
// tutor: 5 Treasure tokens sacrificed → tutor highest-CMC Dragon or
// artifact onto the battlefield, library shuffled.
func TestMagda_SacFiveTreasuresTutorsArtifactOrDragon(t *testing.T) {
	gs := newGame(t, 2)
	magda := addPerm(gs, 0, "Magda, Brazen Outlaw", "legendary", "creature", "dwarf", "berserker")
	// Five Treasure tokens.
	for i := 0; i < 5; i++ {
		addPerm(gs, 0, "Treasure Token", "token", "artifact", "treasure")
	}
	// Library: a Dragon and an artifact.
	dragon := addCard(gs, 0, "Old Gnawbone", "creature", "dragon", "cmc:7")
	artifact := addCard(gs, 0, "Bolas's Citadel", "artifact", "legendary", "cmc:6")
	gs.Seats[0].Library = append(gs.Seats[0].Library, dragon, artifact)

	gameengine.InvokeActivatedHook(gs, magda, 0, nil)

	// All 5 treasures should be gone from battlefield.
	treasureCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil {
			for _, t := range p.Card.Types {
				if t == "treasure" {
					treasureCount++
				}
			}
		}
	}
	if treasureCount != 0 {
		t.Errorf("expected all 5 treasures sacrificed; %d remain", treasureCount)
	}
	// Dragon should be tutored (prefer Dragon over artifact at any CMC).
	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == dragon {
			foundOnBF = true
			break
		}
	}
	if !foundOnBF {
		t.Errorf("expected Dragon tutored onto battlefield; not found")
	}
}

// TestMagda_InsufficientTreasuresFails pins the cost-fail gate: <5
// treasures → emitFail, no sacrifice, no tutor.
func TestMagda_InsufficientTreasuresFails(t *testing.T) {
	gs := newGame(t, 2)
	magda := addPerm(gs, 0, "Magda, Brazen Outlaw", "legendary", "creature", "dwarf")
	// Only 3 treasures.
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Treasure Token", "token", "artifact", "treasure")
	}
	dragon := addCard(gs, 0, "Some Dragon", "creature", "dragon")
	gs.Seats[0].Library = append(gs.Seats[0].Library, dragon)

	gameengine.InvokeActivatedHook(gs, magda, 0, nil)

	// Treasures should still be there.
	treasureCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil {
			for _, t := range p.Card.Types {
				if t == "treasure" {
					treasureCount++
				}
			}
		}
	}
	if treasureCount != 3 {
		t.Errorf("expected 3 treasures preserved after fail; got %d", treasureCount)
	}
	// Dragon should NOT be on battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == dragon {
			t.Errorf("dragon should not be tutored when cost can't be paid")
		}
	}
}

// TestGrub_FirstMainPaysRToTransform pins the front-face upkeep proxy:
// front face with {R} available pays it and transforms.
func TestGrub_FirstMainPaysRToTransform(t *testing.T) {
	gs := newGame(t, 2)
	grub := addPerm(gs, 0, "Grub, Storied Matriarch", "legendary", "creature", "goblin")
	// Provide one R mana.
	if gs.Seats[0].Mana == nil {
		gs.Seats[0].Mana = &gameengine.ColoredManaPool{}
	}
	gs.Seats[0].Mana.R = 1
	gs.Seats[0].ManaPool = 1

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// Verify TransformPermanent was invoked (transform OR transform_noop).
	if hasEvent(gs, "transform")+hasEvent(gs, "transform_noop") < 1 {
		t.Errorf("expected TransformPermanent invoked after paying R; events=%+v", gs.EventLog)
	}
	// Verify R was spent.
	if gs.Seats[0].Mana.R != 0 {
		t.Errorf("expected R spent (0 remaining); got %d", gs.Seats[0].Mana.R)
	}
	_ = grub
}

// TestGrub_FirstMainSkipsWithoutMana pins the cost gate: no mana →
// no transform attempt, no event emitted.
func TestGrub_FirstMainSkipsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	grub := addPerm(gs, 0, "Grub, Storied Matriarch", "legendary", "creature", "goblin")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
	})

	// No transform event.
	if hasEvent(gs, "transform")+hasEvent(gs, "transform_noop") > 0 {
		t.Errorf("expected NO transform attempt without mana; events=%+v", gs.EventLog)
	}
	_ = grub
}

// TestGrub_FirstMainSkipsOnOpponentTurn pins the active_seat gate.
func TestGrub_FirstMainSkipsOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	grub := addPerm(gs, 0, "Grub, Storied Matriarch", "legendary", "creature", "goblin")
	if gs.Seats[0].Mana == nil {
		gs.Seats[0].Mana = &gameengine.ColoredManaPool{}
	}
	gs.Seats[0].Mana.R = 1
	gs.Seats[0].ManaPool = 1 // keep typed pool in sync with ManaPool

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 1, // opponent's turn
	})

	if hasEvent(gs, "transform")+hasEvent(gs, "transform_noop") > 0 {
		t.Errorf("expected NO transform on opponent's upkeep; events=%+v", gs.EventLog)
	}
	// Mana should NOT have been spent.
	if gs.Seats[0].Mana.R != 1 {
		t.Errorf("R should be preserved on opponent's turn; got %d", gs.Seats[0].Mana.R)
	}
	_ = grub
}
