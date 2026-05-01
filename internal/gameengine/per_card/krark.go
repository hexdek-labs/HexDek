package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKrark wires Krark, the Thumbless.
//
// Oracle text:
//
//	Whenever you cast an instant or sorcery spell, flip a coin. If you
//	lose the flip, return that spell to its owner's hand. If you win
//	the flip, copy that spell, and you may choose new targets for the
//	copy.
//	Partner
//
// Implementation:
//   - Listens on "spell_cast"; gates on caster_seat == controller and
//     instant-or-sorcery.
//   - Coin flip via math/rand: heads (1) wins, tails (0) loses.
//   - Lose: locate the StackItem whose Card matches ctx["card"], remove
//     it from the stack, and route the card to its owner's hand via
//     MoveCard (preserves §614 replacements + commander redirect).
//   - Win: deep-copy the cast card, mark IsCopy, and push a new
//     StackItem on top, mirroring resolveCopySpell's CR §707.2 path.
func registerKrark(r *Registry) {
	r.OnTrigger("Krark, the Thumbless", "spell_cast", krarkTrigger)
}

func krarkTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "krark_thumbless_coin_flip"
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
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return
	}

	// Find the spell's StackItem.
	var (
		stackIdx  = -1
		stackItem *gameengine.StackItem
	)
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si.Card != card {
			continue
		}
		stackIdx = i
		stackItem = si
		break
	}

	won := rand.Intn(2) == 1

	if !won {
		// Lose: return the spell to its owner's hand.
		owner := card.Owner
		if stackIdx >= 0 {
			gs.Stack = append(gs.Stack[:stackIdx], gs.Stack[stackIdx+1:]...)
		}
		gameengine.MoveCard(gs, card, owner, "stack", "hand", "krark_bounce")
		gs.LogEvent(gameengine.Event{
			Kind:   "bounce",
			Seat:   perm.Controller,
			Target: owner,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":        slug,
				"target_card": card.DisplayName(),
				"from":        "stack",
				"flip":        "lose",
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"flip":      "lose",
			"spell":     card.DisplayName(),
			"returned":  true,
		})
		return
	}

	// Win: copy the spell. CR §707.2 — the copy is created on the
	// stack; per CR §706.10 it ceases to exist on resolution rather
	// than going to a graveyard.
	if stackItem == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "spell_not_on_stack", map[string]interface{}{
			"flip":  "win",
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
			"flip":    "win",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"flip":   "win",
		"spell":  card.DisplayName(),
		"copied": true,
	})
}
