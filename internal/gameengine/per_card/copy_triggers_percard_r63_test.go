package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// copy_triggers_percard_r63_test.go — representative regression for the r63
// copy-trigger sweep's per_card half. The per_card copy-target-spell cards
// (Kalamax, Alania, Ivy, Krark, Aziza, Fire-Lord Azula, Mendicant Core) all
// add one identical line — gameengine.FireSpellCopyTriggers(...) right after
// PushStackItem. This drives Kalamax end-to-end: a magecraft creature on the
// copier's battlefield must see the Kalamax copy (CR §702.137a, copy branch),
// proving the per_card→canonical-helper wiring.

func magecraftCopyTriggerCount(gs *gameengine.GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "magecraft_trigger" {
			continue
		}
		if v, ok := ev.Details["is_copy"].(bool); ok && v {
			n++
		}
	}
	return n
}

func TestCopyTriggers_KalamaxCopyFiresMagecraft(t *testing.T) {
	gs := newGame(t, 2)
	kalamax := addPerm(gs, 0, "Kalamax, the Stormsire", "creature")
	kalamax.Tapped = true // first instant each turn copies only while tapped

	// A magecraft permanent the controller owns (AST keyword so the engine's
	// FireMagecraftTriggers fan-out detects it).
	mc := addPerm(gs, 0, "Storm-Kiln Artist", "creature")
	mc.Card.AST = &gameast.CardAST{
		Name:      "Storm-Kiln Artist",
		Abilities: []gameast.Ability{&gameast.Keyword{Name: "magecraft"}},
	}

	// The cast instant, on the stack, matched by ctx["card"].
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: bolt, Controller: 0, Kind: "spell"})

	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        bolt,
		"is_instant":  true,
	})

	if got := magecraftCopyTriggerCount(gs); got < 1 {
		t.Errorf("Kalamax copy must fire magecraft is_copy=true via FireSpellCopyTriggers; got %d", got)
	}
}
