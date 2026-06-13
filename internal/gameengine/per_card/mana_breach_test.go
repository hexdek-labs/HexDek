package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Mana Breach: "Whenever a player casts a spell, that player returns a land
// they control to its owner's hand." Symmetric — fires for every caster.
// Pre-handler the effect parsed to an inert parsed_effect_residual and did
// nothing.

func fireSpellCast(gs *gameengine.GameState, casterSeat int, spellName string) {
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  spellName,
	})
}

func countLands(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsLand() {
			n++
		}
	}
	return n
}

func TestManaBreach_OpponentCastReturnsTheirLand(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mana Breach", "enchantment")
	opp := addPerm(gs, 1, "Forest", "land")
	opp.Card.TypeLine = "Basic Land — Forest"

	if countLands(gs, 1) != 1 {
		t.Fatalf("setup: opponent should control 1 land")
	}
	fireSpellCast(gs, 1, "Grizzly Bears")

	if got := countLands(gs, 1); got != 0 {
		t.Fatalf("expected opponent's land returned to hand, lands left=%d", got)
	}
	if len(gs.Seats[1].Hand) != 1 || gs.Seats[1].Hand[0].DisplayName() != "Forest" {
		t.Fatalf("expected Forest in opponent's hand, got %v", gs.Seats[1].Hand)
	}
}

func TestManaBreach_IsSymmetricHitsController(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mana Breach", "enchantment")
	own := addPerm(gs, 0, "Island", "land")
	own.Card.TypeLine = "Basic Land — Island"

	// Controller casting their own spell also returns one of their lands.
	fireSpellCast(gs, 0, "Ponder")
	if got := countLands(gs, 0); got != 0 {
		t.Fatalf("Mana Breach is symmetric — controller's own cast should bounce their land, lands left=%d", got)
	}
}

func TestManaBreach_PrefersTappedBasic(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mana Breach", "enchantment")

	// Untapped dual (least expendable) vs tapped basic (most expendable).
	dual := addPerm(gs, 1, "Tropical Island", "land")
	dual.Card.TypeLine = "Land — Forest Island"
	dual.Tapped = false

	basic := addPerm(gs, 1, "Forest", "land")
	basic.Card.TypeLine = "Basic Land — Forest"
	basic.Tapped = true

	fireSpellCast(gs, 1, "Brainstorm")

	// The tapped basic should be the one returned; the dual stays.
	if len(gs.Seats[1].Hand) != 1 || gs.Seats[1].Hand[0].DisplayName() != "Forest" {
		t.Fatalf("expected tapped basic Forest returned, hand=%v", gs.Seats[1].Hand)
	}
	if countLands(gs, 1) != 1 {
		t.Fatalf("expected the untapped dual to remain on battlefield")
	}
}

func TestManaBreach_NoLandNoOp(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Mana Breach", "enchantment")
	// Opponent controls no land — handler must no-op cleanly.
	fireSpellCast(gs, 1, "Lightning Bolt")
	if len(gs.Seats[1].Hand) != 0 {
		t.Fatalf("expected no bounce when caster has no land, hand=%v", gs.Seats[1].Hand)
	}
}
