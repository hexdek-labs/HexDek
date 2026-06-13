package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic aura_no_untap coverage: a creature enchanted by a tap-lock aura
// (Waterknot / Claustrophobia) stays tapped through its controller's untap
// step. Before this the kind was inert and the creature wrongly untapped.

func mkNoUntapAura(gs *GameState, seat int, name string, target *Permanent) *Permanent {
	a := addTestPerm(gs, seat, name, "enchantment", "aura")
	a.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{ModKind: "aura_no_untap"}},
	}}
	a.AttachedTo = target
	return a
}

func TestAuraNoUntap_EnchantedCreatureStaysTapped(t *testing.T) {
	gs := newTestGame(t, 2)
	creature := addTestPerm(gs, 0, "Tapped Beast", "creature")
	creature.Card.BasePower, creature.Card.BaseToughness = 3, 3
	creature.Tapped = true
	mkNoUntapAura(gs, 0, "Waterknot", creature)

	UntapAll(gs, 0)

	if !creature.Tapped {
		t.Fatalf("creature enchanted by aura_no_untap must stay TAPPED through untap step")
	}
}

func TestAuraNoUntap_UnenchantedUntapsNormally(t *testing.T) {
	gs := newTestGame(t, 2)
	creature := addTestPerm(gs, 0, "Free Beast", "creature")
	creature.Card.BasePower, creature.Card.BaseToughness = 3, 3
	creature.Tapped = true
	// No aura attached.
	UntapAll(gs, 0)
	if creature.Tapped {
		t.Fatalf("a creature with no tap-lock aura must untap normally")
	}
}

func TestAuraNoUntap_OtherCreaturesUnaffected(t *testing.T) {
	gs := newTestGame(t, 2)
	locked := addTestPerm(gs, 0, "Locked", "creature")
	locked.Card.BasePower, locked.Card.BaseToughness = 2, 2
	locked.Tapped = true
	free := addTestPerm(gs, 0, "Free", "creature")
	free.Card.BasePower, free.Card.BaseToughness = 2, 2
	free.Tapped = true
	mkNoUntapAura(gs, 0, "Claustrophobia", locked)

	UntapAll(gs, 0)

	if !locked.Tapped {
		t.Fatalf("enchanted creature must stay tapped")
	}
	if free.Tapped {
		t.Fatalf("the other creature (no aura) must untap")
	}
}

func TestAuraNoUntap_DetachedAuraNoLongerLocks(t *testing.T) {
	gs := newTestGame(t, 2)
	creature := addTestPerm(gs, 0, "Beast", "creature")
	creature.Card.BasePower, creature.Card.BaseToughness = 3, 3
	creature.Tapped = true
	aura := mkNoUntapAura(gs, 0, "Waterknot", creature)
	// Aura falls off (AttachedTo cleared) — creature should untap again.
	aura.AttachedTo = nil

	UntapAll(gs, 0)

	if creature.Tapped {
		t.Fatalf("once the aura is detached the creature must untap (read-only attachment check)")
	}
}
