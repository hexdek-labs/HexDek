package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// counterspells_mr_r60_test.go — regression pins for the shard M-R
// counterspell family that previously parsed to inert typed_spell_effect
// nodes and countered NOTHING.

func mrPushOppSpell(gs *gameengine.GameState, name string, types ...string) *gameengine.StackItem {
	si := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: name, Owner: 1, Types: append([]string{}, types...)},
	}
	gs.Stack = append(gs.Stack, si)
	return si
}

func mrResolve(gs *gameengine.GameState, counterName string) {
	card := addCard(gs, 0, counterName, "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
}

func TestManaLeak_CountersAnySpell(t *testing.T) {
	gs := newGame(t, 2)
	opp := mrPushOppSpell(gs, "Demonic Tutor", "sorcery")
	mrResolve(gs, "Mana Leak")
	if !opp.Countered {
		t.Errorf("Mana Leak should counter the opp spell")
	}
}

func TestManaTithe_CountersAnySpell(t *testing.T) {
	gs := newGame(t, 2)
	opp := mrPushOppSpell(gs, "Cultivate", "sorcery")
	mrResolve(gs, "Mana Tithe")
	if !opp.Countered {
		t.Errorf("Mana Tithe should counter the opp spell")
	}
}

func TestQuench_CountersAnySpell(t *testing.T) {
	gs := newGame(t, 2)
	opp := mrPushOppSpell(gs, "Bear", "creature")
	mrResolve(gs, "Quench")
	if !opp.Countered {
		t.Errorf("Quench should counter the opp spell")
	}
}

func TestRemoveSoul_OnlyCreatureSpells(t *testing.T) {
	gs := newGame(t, 2)
	sorc := mrPushOppSpell(gs, "Wrath of God", "sorcery")
	mrResolve(gs, "Remove Soul")
	if sorc.Countered {
		t.Errorf("Remove Soul must not counter a sorcery")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected fail when only a sorcery is on the stack")
	}
	// Now a creature spell.
	gs2 := newGame(t, 2)
	cre := mrPushOppSpell(gs2, "Tarmogoyf", "creature")
	mrResolve(gs2, "Remove Soul")
	if !cre.Countered {
		t.Errorf("Remove Soul should counter a creature spell")
	}
}

func TestMysticDenial_CreatureOrSorcery(t *testing.T) {
	gs := newGame(t, 2)
	cre := mrPushOppSpell(gs, "Bear", "creature")
	mrResolve(gs, "Mystic Denial")
	if !cre.Countered {
		t.Errorf("Mystic Denial should counter a creature spell")
	}
	gs2 := newGame(t, 2)
	inst := mrPushOppSpell(gs2, "Lightning Bolt", "instant")
	mrResolve(gs2, "Mystic Denial")
	if inst.Countered {
		t.Errorf("Mystic Denial must not counter an instant")
	}
}

func TestMinorMisstep_OnlyLowMV(t *testing.T) {
	gs := newGame(t, 2)
	cheap := mrPushOppSpell(gs, "Opt", "instant")
	cheap.Card.CMC = 1
	mrResolve(gs, "Minor Misstep")
	if !cheap.Countered {
		t.Errorf("Minor Misstep should counter a MV-1 spell")
	}
	gs2 := newGame(t, 2)
	pricey := mrPushOppSpell(gs2, "Cultivate", "sorcery")
	pricey.Card.CMC = 3
	mrResolve(gs2, "Minor Misstep")
	if pricey.Countered {
		t.Errorf("Minor Misstep must not counter a MV-3 spell")
	}
}
