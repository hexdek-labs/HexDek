package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestMentalMisstep_CountersMV1Spell pins the CMC=1 filter: counters
// a 1-mana spell on the stack.
func TestMentalMisstep_CountersMV1Spell(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Cheap Cantrip",
			Owner: 1,
			Types: []string{"instant", "cost:1"},
			CMC:   1,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Mental Misstep", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Mental Misstep should counter MV=1 spell; %+v", oppSpell)
	}
}

// TestMentalMisstep_DoesNotCounterMV2Plus pins the negative filter:
// a 2-mana spell is NOT countered.
func TestMentalMisstep_DoesNotCounterMV2Plus(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Counterspell",
			Owner: 1,
			Types: []string{"instant"},
			CMC:   2,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Mental Misstep", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if oppSpell.Countered {
		t.Errorf("Mental Misstep should NOT counter MV=2 spell")
	}
}

// TestSpellSnare_CountersMV2Spell pins the CMC=2 filter.
func TestSpellSnare_CountersMV2Spell(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Counterspell",
			Owner: 1,
			Types: []string{"instant"},
			CMC:   2,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Spell Snare", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Spell Snare should counter MV=2 spell")
	}
}

// TestSpellSnare_DoesNotCounterMV3 pins the negative filter.
func TestSpellSnare_DoesNotCounterMV3(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Cancel",
			Owner: 1,
			Types: []string{"instant"},
			CMC:   3,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Spell Snare", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if oppSpell.Countered {
		t.Errorf("Spell Snare should NOT counter MV=3 spell")
	}
}

// TestMindbreakTrap_CountersAndExiles pins the exile rider.
func TestMindbreakTrap_CountersAndExiles(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Storm Spell",
			Owner: 1,
			Types: []string{"sorcery"},
			CMC:   3,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Mindbreak Trap", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Mindbreak Trap should counter target spell")
	}
	if oppSpell.CostMeta == nil {
		t.Fatalf("expected CostMeta stamped with exile_on_resolve")
	}
	if v, _ := oppSpell.CostMeta["exile_on_resolve"].(bool); !v {
		t.Errorf("expected exile_on_resolve=true; got %+v", oppSpell.CostMeta)
	}
}

// TestCounterflux_CannotBeCounteredFlagOnCast pins the OnCast hook
// stamps cannot_be_countered.
func TestCounterflux_CannotBeCounteredFlagOnCast(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Counterflux", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}

	gameengine.InvokeCastHook(gs, item)

	if item.CostMeta == nil {
		t.Fatalf("expected CostMeta stamped by counterfluxCast")
	}
	if v, _ := item.CostMeta["cannot_be_countered"].(bool); !v {
		t.Errorf("expected cannot_be_countered=true; got %+v", item.CostMeta)
	}
}

// TestCounterflux_CountersAnySpell pins the resolve.
func TestCounterflux_CountersAnySpell(t *testing.T) {
	gs := newGame(t, 2)
	oppSpell := &gameengine.StackItem{
		Controller: 1,
		Card: &gameengine.Card{
			Name:  "Big Threat",
			Owner: 1,
			Types: []string{"creature"},
			CMC:   8,
		},
	}
	gs.Stack = append(gs.Stack, oppSpell)

	card := addCard(gs, 0, "Counterflux", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !oppSpell.Countered {
		t.Errorf("Counterflux should counter any opponent spell")
	}
}

// TestCounterflux_NoSpellOnStackEmitsFail pins the no-target path.
func TestCounterflux_NoSpellOnStackEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Counterflux", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" && ev.Source == "Counterflux" {
			if reason, _ := ev.Details["reason"].(string); reason == "no_spell_on_stack" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_failed:Counterflux with no_spell_on_stack reason")
	}
}
