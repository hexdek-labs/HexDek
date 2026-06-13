package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// counterspells_mr_r60.go — per_card handlers for a family of shard M-R
// counterspells that were GENUINELY INERT.
//
// Each of these parses to a bare `typed_spell_effect` Modification node
// with EMPTY args and no structured Counter — a kind the engine handles
// nowhere (verified: `typed_spell_effect` appears in no resolve switch
// case and no text-fallback shape). So at resolution they did NOTHING:
// the targeted spell resolved normally. For staple interaction like Mana
// Leak this is a serious functional gap.
//
// The shard already has the canonical counter machinery
// (findCounterableSpell → set StackItem.Countered → emitCounter, used by
// Counterspell / Negate / Mana Drain). These handlers reuse it with the
// appropriate target filter. For "counter unless its controller pays {N}"
// cards we follow the engine's established convention (resolve_helpers.go
// `counter_that_spell_unless_pay`): counter it, assuming the opponent
// can't/won't pay — the dominant line in the AI sim where opponents
// rarely hold the extra mana open through your counter window.
//
// One new self-registering file (init() + AddResetHook); no shared
// registry edits. Each card regression-pinned.

func init() {
	registerCounterspellsMRR60(Global())
	AddResetHook(registerCounterspellsMRR60)
}

func registerCounterspellsMRR60(r *Registry) {
	if r == nil {
		return
	}
	// Unconditional "counter unless controller pays {N}" — counter (engine
	// convention assumes no pay).
	r.OnResolve("Mana Leak", mrCounterFunc("Mana Leak", "mana_leak", nil, 3))
	r.OnResolve("Mana Tithe", mrCounterFunc("Mana Tithe", "mana_tithe", nil, 1))
	r.OnResolve("Quench", mrCounterFunc("Quench", "quench", nil, 2))
	// Type-restricted hard counters.
	r.OnResolve("Remove Soul", mrCounterFunc("Remove Soul", "remove_soul", isCreatureSpell, 0))
	r.OnResolve("Preemptive Strike", mrCounterFunc("Preemptive Strike", "preemptive_strike", isCreatureSpell, 0))
	r.OnResolve("Mystic Denial", mrCounterFunc("Mystic Denial", "mystic_denial", mrCreatureOrSorcery, 0))
	r.OnResolve("Minor Misstep", mrCounterFunc("Minor Misstep", "minor_misstep", mrManaValueLE1, 0))
}

// mrCounterFunc builds a ResolveHandler that counters the first legal
// opponent spell matching filter. unlessPay > 0 records the soft-counter
// cost in the event for log fidelity.
func mrCounterFunc(card, slug string, filter func(*gameengine.StackItem) bool, unlessPay int) func(*gameengine.GameState, *gameengine.StackItem) {
	return func(gs *gameengine.GameState, item *gameengine.StackItem) {
		if gs == nil || item == nil {
			return
		}
		target := findCounterableSpell(gs, item.Controller, filter)
		if target == nil {
			emitFail(gs, slug, card, "no_legal_spell_on_stack", nil)
			return
		}
		target.Countered = true
		emitCounter(gs, slug, card, item.Controller, target)
		if unlessPay > 0 {
			emit(gs, slug, card, map[string]interface{}{
				"mode": "unless_pay",
				"cost": unlessPay,
			})
		}
	}
}

func mrCreatureOrSorcery(si *gameengine.StackItem) bool {
	return isCreatureSpell(si) || isSorcerySpell(si)
}

func mrManaValueLE1(si *gameengine.StackItem) bool {
	if si == nil || si.Card == nil {
		return false
	}
	return si.Card.CMC <= 1
}
