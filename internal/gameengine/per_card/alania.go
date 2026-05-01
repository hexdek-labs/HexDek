package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlania wires Alania, Divergent Storm.
//
// Oracle text:
//
//	Whenever you cast a spell, if it's the first instant spell, the
//	first sorcery spell, or the first Otter spell other than Alania
//	you've cast this turn, you may have target opponent draw a card.
//	If you do, copy that spell. You may choose new targets for the
//	copy.
//
// Implementation:
//   - Listens on "spell_cast"; gates on caster_seat == perm.Controller.
//   - Tracks per-permanent first-spell-this-turn flags keyed by gs.Turn:
//     alania_first_instant_turn, alania_first_sorcery_turn,
//     alania_first_otter_turn. A spell that's both Otter and instant only
//     fires once (the trigger fires once per cast even when multiple
//     "first" conditions are simultaneously satisfied).
//   - AI policy: always opt YES on the may-draw clause — the spell copy
//     is worth far more than 1 card to one opponent, and refusing skips
//     the copy entirely.
//   - Target opponent: lowest-life living opponent (least likely to live
//     long enough to convert the extra card into a threat).
//   - Copy mirrors krark.go / resolveCopySpell — deep-copy the StackItem,
//     mark IsCopy, push above the original.
func registerAlania(r *Registry) {
	r.OnTrigger("Alania, Divergent Storm", "spell_cast", alaniaSpellCast)
}

func alaniaSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alania_divergent_storm_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	// Alania casting herself doesn't trigger her own conditions (she's
	// not on the battlefield yet at cast time anyway, but defensive).
	if card == perm.Card {
		return
	}

	isInstant := cardHasType(card, "instant")
	isSorcery := cardHasType(card, "sorcery")
	isOtter := cardHasType(card, "otter")

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := strconv.Itoa(gs.Turn)
	firedKind := ""
	switch {
	case isInstant && perm.Flags["alania_first_instant_turn"] != gs.Turn:
		perm.Flags["alania_first_instant_turn"] = gs.Turn
		firedKind = "instant"
	case isSorcery && perm.Flags["alania_first_sorcery_turn"] != gs.Turn:
		perm.Flags["alania_first_sorcery_turn"] = gs.Turn
		firedKind = "sorcery"
	case isOtter && perm.Flags["alania_first_otter_turn"] != gs.Turn:
		perm.Flags["alania_first_otter_turn"] = gs.Turn
		firedKind = "otter"
	}
	if firedKind == "" {
		return
	}

	// Pick a target opponent — lowest-life living opponent.
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent", map[string]interface{}{
			"turn": turnKey,
		})
		return
	}

	drawOne(gs, target, perm.Card.DisplayName())

	// Locate the spell's StackItem to copy.
	var stackItem *gameengine.StackItem
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card != card {
			continue
		}
		stackItem = si
		break
	}
	if stackItem == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "spell_not_on_stack", map[string]interface{}{
			"spell": card.DisplayName(),
			"kind":  firedKind,
		})
		return
	}

	copyCard := card.DeepCopy()
	copyCard.IsCopy = true
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
			"slug":         slug,
			"copied":       card.DisplayName(),
			"kind":         firedKind,
			"opp_drew":     target,
			"is_copy":      true,
			"rule":         "707.2",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"spell":        card.DisplayName(),
		"kind":         firedKind,
		"opp_drew":     target,
	})
}
