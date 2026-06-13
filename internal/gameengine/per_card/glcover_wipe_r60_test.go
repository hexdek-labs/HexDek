package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// glcover_wipe_r60_test.go — regression pins for shard G-L board/zone
// control spells that parsed to inert raw-text nodes and did NOTHING:
// Hallowed Burial (mass tuck), Hurkyl's Recall (mass artifact bounce),
// Hex (six-target destroy).

func TestHallowedBurial_TucksAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Grizzly Bears", "creature")
	addPerm(gs, 1, "Savannah Lions", "creature")
	addPerm(gs, 1, "Llanowar Elves", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact") // survives

	card := addCard(gs, 0, "Hallowed Burial", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	for seat := 0; seat < 2; seat++ {
		for _, p := range gs.Seats[seat].Battlefield {
			if p != nil && p.IsCreature() {
				t.Errorf("seat %d still has creature %s", seat, p.Card.Name)
			}
		}
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("seat 0 library = %d, want 1 (Grizzly tucked)", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[1].Library) != 2 {
		t.Errorf("seat 1 library = %d, want 2 (two tucked)", len(gs.Seats[1].Library))
	}
}

func TestHurkylsRecall_BouncesTargetOwnersArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Sol Ring", "artifact")
	addPerm(gs, 1, "Mana Vault", "artifact")
	addPerm(gs, 1, "Bear", "creature") // not bounced
	addPerm(gs, 0, "Mox", "artifact")  // caster's, not target's

	card := addCard(gs, 0, "Hurkyl's Recall", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	artsLeft := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsArtifact() {
			artsLeft++
		}
	}
	if artsLeft != 0 {
		t.Errorf("target still controls %d artifacts, want 0", artsLeft)
	}
	if len(gs.Seats[1].Hand) != 2 {
		t.Errorf("target hand = %d, want 2 bounced artifacts", len(gs.Seats[1].Hand))
	}
	// Caster's own artifact untouched.
	casterArts := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsArtifact() {
			casterArts++
		}
	}
	if casterArts != 1 {
		t.Errorf("caster artifacts = %d, want 1 (untouched)", casterArts)
	}
}

func TestHex_DestroysUpToSixOpponentCreatures(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 8; i++ {
		p := addPerm(gs, 1, "Token", "creature")
		p.Card.BaseToughness = 2 // survive SBA so the 6-cap is observable
	}
	myPerm := addPerm(gs, 0, "My Creature", "creature") // caster's, spared
	myPerm.Card.BaseToughness = 2

	card := addCard(gs, 0, "Hex", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	oppCreatures := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsCreature() {
			oppCreatures++
		}
	}
	if oppCreatures != 2 {
		t.Errorf("opponent creatures left = %d, want 2 (8 - 6 destroyed)", oppCreatures)
	}
	// Caster's creature spared.
	mine := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			mine++
		}
	}
	if mine != 1 {
		t.Errorf("caster creatures = %d, want 1 (spared)", mine)
	}
}
