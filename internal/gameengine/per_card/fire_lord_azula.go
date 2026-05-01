package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFireLordAzula wires Fire Lord Azula (Batch #30 rewrite).
//
// Oracle text (Scryfall, Avatar: The Last Airbender, verified 2026-05-01):
//
//	{1}{U}{B}{R}, 4/4 Legendary Creature — Human Noble
//	Firebending 2 (Whenever this creature attacks, add {R}{R}. This mana
//	  lasts until end of combat.)
//	Whenever you cast a spell while Fire Lord Azula is attacking, copy
//	that spell. You may choose new targets for the copy. (A copy of a
//	permanent spell becomes a token.)
//
// Implementation:
//   - Firebending 2: on creature_attacks (filtered to Azula herself) add
//     2 to controller's mana pool. The engine's pool is colorless (no
//     per-color tracking), so we add the count and emitPartial flagging
//     "color tracking lost — should be {R}{R} restricted to combat costs".
//     The combat-only restriction is also approximated by adding to the
//     general pool — pools drain at end of phase, so practically it's
//     usable for spells/abilities cast during the same combat phase.
//   - Spell-copy while attacking: on spell_cast, gate on caster_seat ==
//     Azula's controller AND Azula.IsAttacking(). Copy the spell using
//     the krark.go / alania.go pattern (deep-copy StackItem, mark IsCopy,
//     push above the original). Skip when Azula is the spell being cast
//     (defensive — she's not on the battlefield at her own cast).
func registerFireLordAzula(r *Registry) {
	r.OnTrigger("Fire Lord Azula", "creature_attacks", azulaFirebending)
	r.OnTrigger("Fire Lord Azula", "spell_cast", azulaSpellCopy)
}

func azulaFirebending(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fire_lord_azula_firebending"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	seat.ManaPool += 2
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 2,
		Details: map[string]interface{}{
			"slug":   slug,
			"colors": "RR",
			"reason": "firebending_2",
			"rule":   "702.189",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"added": 2,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"firebending_color_R_tracking_and_combat_only_lifetime_approximated_via_general_pool")
}

func azulaSpellCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "fire_lord_azula_spell_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if !perm.IsAttacking() {
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
	// Defensive: the trigger fires for every spell cast, including Azula
	// herself if cast in some unusual way (alt-cast). She isn't on the
	// battlefield at that moment, so IsAttacking() is false — but skip
	// explicitly anyway.
	if card == perm.Card {
		return
	}

	// Locate the spell's StackItem.
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
			"slug":    slug,
			"copied":  card.DisplayName(),
			"is_copy": true,
			"rule":    "707.2",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"spell": card.DisplayName(),
	})
}
