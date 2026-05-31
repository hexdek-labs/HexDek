package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKalamaxTheStormsireCustom adds Kalamax's instant-copy trigger
// and the +1/+1 counter-on-copy trigger that the auto-generated static
// stub leaves as no-ops.
//
// Oracle text:
//
//	Whenever you cast your first instant spell each turn, if Kalamax
//	is tapped, copy that spell. You may choose new targets for the copy.
//	Whenever you copy an instant spell, put a +1/+1 counter on Kalamax.
//
// Implementation:
//   - Track first-instant-this-turn per Kalamax permanent via
//     perm.Flags["kalamax_first_instant_used"]; reset at upkeep.
//   - When the first instant is cast and Kalamax is tapped, deep-copy
//     the original spell (mirroring rootha.go / krark.go), push it as
//     a fresh StackItem above the original, then put a +1/+1 counter on
//     Kalamax for the second clause ("Whenever you copy an instant
//     spell, put a +1/+1 counter on Kalamax").
//   - The copy event keys off ctx["spell_card"] (a *Card) supplied by
//     fireCastTriggersFromZone in stack.go (key "card"). We locate the
//     matching StackItem on the stack to source the deep copy.
func registerKalamaxTheStormsireCustom(r *Registry) {
	r.OnETB("Kalamax, the Stormsire", kalamaxETBReset)
	r.OnTrigger("Kalamax, the Stormsire", "instant_or_sorcery_cast", kalamaxFirstInstantCopy)
}

func kalamaxETBReset(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["kalamax_first_instant_used"] = 0
	scheduleKalamaxReset(gs, perm)
}

func scheduleKalamaxReset(gs *gameengine.GameState, perm *gameengine.Permanent) {
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if perm.Flags != nil {
				perm.Flags["kalamax_first_instant_used"] = 0
			}
			seat := gs.Seats[perm.Controller]
			if seat == nil {
				return
			}
			for _, p := range seat.Battlefield {
				if p == perm {
					scheduleKalamaxReset(gs, perm)
					return
				}
			}
		},
	})
}

func kalamaxFirstInstantCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kalamax_first_instant_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, ok := ctx["caster_seat"].(int)
	if !ok || caster != perm.Controller {
		return
	}
	// instant_or_sorcery_cast fires for both — filter to instants by
	// inspecting the spell card's types when present, falling back to
	// ctx["is_instant"] for callers that pre-classified.
	spellCard, _ := ctx["card"].(*gameengine.Card)
	isInstant := false
	if spellCard != nil {
		isInstant = cardHasType(spellCard, "instant")
	} else if v, ok := ctx["is_instant"].(bool); ok {
		isInstant = v
	}
	if !isInstant {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["kalamax_first_instant_used"] > 0 {
		return
	}
	if !perm.Tapped {
		return
	}
	perm.Flags["kalamax_first_instant_used"] = 1

	// Locate the original spell's StackItem so we can deep-copy it.
	var stackItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card == nil {
			continue
		}
		if si.Card == spellCard {
			stackItem = si
			break
		}
	}
	copied := false
	if stackItem != nil {
		copyCard := gameengine.MintSpellCopy(gs, stackItem.Card)
		copyItem := &gameengine.StackItem{
			Controller: perm.Controller,
			Card:       copyCard,
			Effect:     stackItem.Effect,
			Kind:       stackItem.Kind,
			IsCopy:     true,
		}
		if len(stackItem.Targets) > 0 {
			copyItem.Targets = append([]gameengine.Target(nil), stackItem.Targets...)
		}
		gameengine.PushStackItem(gs, copyItem)
		gs.LogEvent(gameengine.Event{
			Kind:   "copy_spell",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":    slug,
				"copied":  stackItem.Card.DisplayName(),
				"is_copy": true,
				"rule":    "707.2",
			},
		})
		copied = true
	}

	// "Whenever you copy an instant spell, put a +1/+1 counter on
	// Kalamax." Apply on real copy; also apply in proxy mode (no card
	// supplied / no matching StackItem found) so the rider still
	// resolves at the engine boundary.
	perm.AddCounter("+1/+1", 1)
	spellName, _ := ctx["spell_name"].(string)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"copied":    spellName,
		"made_copy": copied,
		"new_count": perm.Counters["+1/+1"],
	})
}
