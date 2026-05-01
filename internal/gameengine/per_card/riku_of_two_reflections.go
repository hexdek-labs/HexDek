package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRikuOfTwoReflections wires Riku of Two Reflections.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Whenever you cast a creature spell, you may pay {2}{G/U}. If you
//	do, create a token that's a copy of that creature.
//	Whenever you cast an instant or sorcery spell, you may pay {2}{G/U}.
//	If you do, copy that spell. You may choose new targets for the copy.
//
// Implementation:
//   - Two triggers, both keyed on the cast event family:
//     "creature_spell_cast" and "instant_or_sorcery_cast".
//   - Cost {2}{G/U} = 3 mana. The engine's ManaPool is a single int (no
//     color tracking), so we approximate the hybrid pip as 1 generic and
//     gate on ManaPool >= 3. AI policy: always opt yes when affordable —
//     a token copy or spell copy is essentially always worth 3 mana.
//   - Creature path: deep-copy the cast card, mark it as a token (and as
//     a copy), then fire the full ETB cascade via enterBattlefieldWithETB.
//     The original creature spell still resolves normally and enters the
//     battlefield as a non-token, so we end up with one real creature
//     plus one token copy.
//   - Instant/sorcery path: locate the original StackItem, deep-copy the
//     card, push a new StackItem above it (mirrors mica.go / alania.go).
//     Targets are inherited; new-target choice is left to the resolver.
//   - Riku herself never triggers her own abilities (she's not on the
//     battlefield at her own cast time), but defensively we skip when
//     the cast card is Riku.
func registerRikuOfTwoReflections(r *Registry) {
	r.OnTrigger("Riku of Two Reflections", "creature_spell_cast", rikuTwoReflectionsCreatureCast)
	r.OnTrigger("Riku of Two Reflections", "instant_or_sorcery_cast", rikuTwoReflectionsSpellCast)
}

const rikuTwoReflectionsCost = 3 // {2}{G/U} ≈ 3 generic in this engine

func rikuTwoReflectionsCreatureCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "riku_two_reflections_creature_token_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || card == perm.Card {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.ManaPool < rikuTwoReflectionsCost {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"spell":  card.DisplayName(),
			"copied": false,
			"reason": "insufficient_mana",
		})
		return
	}
	seat.ManaPool -= rikuTwoReflectionsCost

	tokenCard := card.DeepCopy()
	tokenCard.IsCopy = true
	tokenCard.Owner = perm.Controller
	hasToken := false
	for _, t := range tokenCard.Types {
		if t == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		tokenCard.Types = append([]string{"token"}, tokenCard.Types...)
	}

	newPerm := enterBattlefieldWithETB(gs, perm.Controller, tokenCard, false)
	if newPerm == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_create_failed", map[string]interface{}{
			"spell": card.DisplayName(),
		})
		return
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token_copy",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"slug":    slug,
			"copy_of": card.DisplayName(),
			"rule":    "706.10a",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"spell":  card.DisplayName(),
		"paid":   rikuTwoReflectionsCost,
		"copied": true,
	})
}

func rikuTwoReflectionsSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "riku_two_reflections_spell_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || card == perm.Card {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.ManaPool < rikuTwoReflectionsCost {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"spell":  card.DisplayName(),
			"copied": false,
			"reason": "insufficient_mana",
		})
		return
	}

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

	seat.ManaPool -= rikuTwoReflectionsCost

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
		"seat":   perm.Controller,
		"spell":  card.DisplayName(),
		"paid":   rikuTwoReflectionsCost,
		"copied": true,
	})
}
