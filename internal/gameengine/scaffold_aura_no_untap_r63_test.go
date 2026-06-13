package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 scaffold-kind regression: aura_no_untap. A lockdown Aura
// ("enchanted permanent doesn't untap during its controller's untap
// step") now actually keeps the enchanted permanent tapped through the
// untap step — previously inert (the static was logged and dropped, so
// the creature untapped normally and the aura did nothing lasting).

func attachNoUntapAura(gs *GameState, seat int, name string, target *Permanent) *Permanent {
	a := addBattlefield(gs, seat, name, 0, 0, "enchantment", "aura")
	a.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "aura_no_untap",
				Args: []interface{}{map[string]interface{}{
					"__ast_type__": "Filter", "base": "enchanted_creature",
				}},
			}},
		},
	}
	a.AttachedTo = target
	return a
}

func TestScaffold_AuraNoUntap_KeepsCreatureTapped(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Tapped = true
	attachNoUntapAura(gs, 1, "Waterknot", bear) // opponent's aura on my creature

	UntapAll(gs, 0)

	if !bear.Tapped {
		t.Fatalf("aura_no_untap: enchanted creature should remain tapped through untap step")
	}
}

func TestScaffold_AuraNoUntap_ControlUntapsNormally(t *testing.T) {
	// Guard the guard: an un-enchanted tapped creature untaps as usual.
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Tapped = true

	UntapAll(gs, 0)

	if bear.Tapped {
		t.Fatalf("normal creature should untap during the untap step")
	}
}

func TestScaffold_AuraNoUntap_DetachReleasesLock(t *testing.T) {
	// Once the aura is gone (detached / destroyed), the lock lifts — the
	// dynamic check means no stale flag persists.
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Tapped = true
	aura := attachNoUntapAura(gs, 1, "Waterknot", bear)

	UntapAll(gs, 0)
	if !bear.Tapped {
		t.Fatalf("should be locked while enchanted")
	}

	// Aura leaves play: clear its attachment (simulating detach/destroy).
	aura.AttachedTo = nil
	bear.Tapped = true
	UntapAll(gs, 0)
	if bear.Tapped {
		t.Fatalf("aura_no_untap lock should lift once the aura is detached")
	}
}

func TestScaffold_AuraNoUntap_AffectsControllersStepOnly(t *testing.T) {
	// The enchanted creature is controlled by seat 0; the lock applies on
	// seat 0's untap step. Seat 1's untap step shouldn't touch it.
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Tapped = true
	attachNoUntapAura(gs, 1, "Waterknot", bear)

	UntapAll(gs, 1) // opponent's untap step — does not untap seat 0's bear anyway
	if !bear.Tapped {
		t.Fatalf("seat 1 untap must not untap seat 0's creature")
	}
	UntapAll(gs, 0) // controller's untap step — still locked
	if !bear.Tapped {
		t.Fatalf("aura_no_untap should keep it tapped on the controller's untap step")
	}
}
