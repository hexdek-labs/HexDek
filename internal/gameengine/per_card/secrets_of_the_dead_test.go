package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Secrets of the Dead: cast a spell from your graveyard → draw. Previously
// inert (cast_trigger_tail).

func fireSpellCastFromZone(gs *gameengine.GameState, casterSeat int, spellName, zone string) {
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  spellName,
		"cast_zone":   zone,
	})
}

func TestSecretsOfTheDead_GraveyardCastDraws(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Secrets of the Dead", "enchantment")
	addLibrary(gs, 0, "A", "B")
	start := len(gs.Seats[0].Hand)

	fireSpellCastFromZone(gs, 0, "Flashback Spell", "graveyard")
	if len(gs.Seats[0].Hand) != start+1 {
		t.Fatalf("graveyard cast should draw 1, delta=%d", len(gs.Seats[0].Hand)-start)
	}
}

func TestSecretsOfTheDead_HandCastDoesNotDraw(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Secrets of the Dead", "enchantment")
	addLibrary(gs, 0, "A", "B")
	start := len(gs.Seats[0].Hand)

	fireSpellCastFromZone(gs, 0, "Normal Spell", "hand")
	if len(gs.Seats[0].Hand) != start {
		t.Fatalf("hand cast must not draw")
	}
	// Opponent's graveyard cast must not draw for the controller either.
	fireSpellCastFromZone(gs, 1, "Their Flashback", "graveyard")
	if len(gs.Seats[0].Hand) != start {
		t.Fatalf("opponent's graveyard cast must not draw for controller")
	}
}
