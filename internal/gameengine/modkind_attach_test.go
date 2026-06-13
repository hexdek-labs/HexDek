package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// modkind_attach_test.go — pins the generic `attach` handler: ETB-attach
// Equipment/Auras now latch onto a legal creature instead of entering
// unattached.

func attPerm(gs *GameState, seat int, name string, power int, types ...string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: append([]string{}, types...), BasePower: power, BaseToughness: power},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func runAttach(gs *GameState, src *Permanent, filter string) {
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "attach",
		Args:    []interface{}{filter},
	})
}

func TestAttach_CreatureYouControl_PicksBestFriendly(t *testing.T) {
	gs := newTestGameState(2)
	maul := attPerm(gs, 0, "Maul of the Skyclaves", 0, "artifact", "equipment")
	small := attPerm(gs, 0, "Squire", 1, "creature", "human")
	big := attPerm(gs, 0, "Serra Angel", 4, "creature", "angel")
	// Opponent creature must NOT be chosen for "you control".
	attPerm(gs, 1, "Big Bad", 9, "creature", "demon")

	runAttach(gs, maul, "creature you control")

	if maul.AttachedTo != big {
		t.Fatalf("Maul should attach to the highest-power creature YOU control (Serra Angel), got %v",
			attachedName(maul))
	}
	_ = small
}

func TestAttach_KnightSubtype_OnlyKnights(t *testing.T) {
	gs := newTestGameState(2)
	armor := attPerm(gs, 0, "Shining Armor", 0, "artifact", "equipment")
	attPerm(gs, 0, "Big Bear", 5, "creature", "bear") // higher power but not a Knight
	knight := attPerm(gs, 0, "Knight Token", 2, "creature", "knight")

	runAttach(gs, armor, "knight you control")

	if armor.AttachedTo != knight {
		t.Fatalf("Shining Armor must attach only to a Knight, got %v", attachedName(armor))
	}
}

func TestAttach_OpponentControls_AttachesToOpponent(t *testing.T) {
	gs := newTestGameState(2)
	blade := attPerm(gs, 0, "Bloodthirsty Blade", 0, "artifact", "equipment")
	attPerm(gs, 0, "My Guy", 6, "creature", "human") // friendly, must be ignored
	oppCreature := attPerm(gs, 1, "Their Guy", 3, "creature", "elf")

	runAttach(gs, blade, "creature an opponent controls")

	if blade.AttachedTo != oppCreature {
		t.Fatalf("Bloodthirsty Blade must attach to an opponent's creature, got %v", attachedName(blade))
	}
}

func TestAttach_LegendaryConstraint(t *testing.T) {
	gs := newTestGameState(2)
	coat := attPerm(gs, 0, "Mithril Coat", 0, "artifact", "equipment", "legendary")
	attPerm(gs, 0, "Vanilla", 7, "creature", "ogre") // nonlegendary, higher power
	legend := attPerm(gs, 0, "Legendary Hero", 3, "creature", "legendary", "human")

	runAttach(gs, coat, "legendary creature you control")

	if coat.AttachedTo != legend {
		t.Fatalf("Mithril Coat must attach only to a legendary creature, got %v", attachedName(coat))
	}
}

func TestAttach_NoLegalTarget_StaysUnattached(t *testing.T) {
	gs := newTestGameState(2)
	maul := attPerm(gs, 0, "Maul of the Skyclaves", 0, "artifact", "equipment")
	// No friendly creatures at all.
	attPerm(gs, 1, "Opponent Only", 4, "creature", "beast")

	runAttach(gs, maul, "creature you control")

	if maul.AttachedTo != nil {
		t.Fatalf("with no legal target the Equipment must stay unattached, got %v", attachedName(maul))
	}
}

func attachedName(p *Permanent) string {
	if p == nil || p.AttachedTo == nil || p.AttachedTo.Card == nil {
		return "<nil>"
	}
	return p.AttachedTo.Card.Name
}
