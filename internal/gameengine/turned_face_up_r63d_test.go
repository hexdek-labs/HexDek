package gameengine

// r63d PROGRESSION phase-3d finding: TurnFaceUp (the chokepoint for
// morph/megamorph, disguise, cloak, manifest, and "turn ~ face up" effects)
// flipped the face and logged the turn_face_up event but NEVER dispatched
// the "when this creature is turned face up," AST triggers — the same
// alias-to-nowhere shape as the gain_life / becomes_tapped families. All 109
// corpus "is turned face up" triggers were silent. FireTurnedFaceUpTriggers
// is now driven from the TurnFaceUp chokepoint.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func faceDownDrawBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "turned_face_up"},
		Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Morph Drawer", 2, 2, "creature")
	p.Card.FaceDown = true
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestTurnedFaceUp_FiresOnTurnFaceUp(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	p := faceDownDrawBearer(gs, 0, "when this creature is turned face up, draw a card")

	before := len(gs.Seats[0].Hand)
	if !TurnFaceUp(gs, p, "test") {
		t.Fatal("TurnFaceUp should succeed on a face-down permanent")
	}
	if got := len(gs.Seats[0].Hand) - before; got != 1 {
		t.Fatalf("turned-face-up trigger must fire through TurnFaceUp: drew %d, want 1", got)
	}
}

func TestTurnedFaceUp_DispatchSilentWithoutTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A")
	p := addBattlefield(gs, 0, "Plain Morph", 2, 2, "creature")
	p.Card.FaceDown = true

	before := len(gs.Seats[0].Hand)
	FireTurnedFaceUpTriggers(gs, p)
	if got := len(gs.Seats[0].Hand) - before; got != 0 {
		t.Fatalf("a creature with no turned-face-up trigger must stay silent: drew %d", got)
	}
}

func TestTurnedFaceUp_NoFireWhenAlreadyFaceUp(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A")
	p := faceDownDrawBearer(gs, 0, "when this creature is turned face up, draw a card")
	p.Card.FaceDown = false // already face up — TurnFaceUp is a no-op

	before := len(gs.Seats[0].Hand)
	if TurnFaceUp(gs, p, "test") {
		t.Fatal("TurnFaceUp on an already-face-up permanent should return false")
	}
	if got := len(gs.Seats[0].Hand) - before; got != 0 {
		t.Fatalf("no trigger should fire when the permanent was already face up: drew %d", got)
	}
}
