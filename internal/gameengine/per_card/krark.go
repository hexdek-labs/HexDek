package per_card

import (
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

	// R55/R56: prefer the per-game RNG when available so tests aren't
	// at the mercy of global math/rand state from other tests; r62
	// routes the flip through the shared rngIntn helper (rng.go) so
	// per_card carries exactly ONE documented global-rand fallback and
	// the TestPerCardNoBareGlobalRand source gate covers this file too.
	//
	// Backstory: r55's Layer-7b CDA port batch (Sandman + Namor +
	// Multani) tripped TestKrark via global-rand pollution — adding
	// any code that consumed or shifted package-init ordering pushed
	// math/rand off-phase, so a globally-seeded flip no longer reliably
	// produced won=false. The gs.Rng preference fixed that; r62 made it
	// the package-wide rule.
	won := rngIntn(gs, 2) == 1

	if !won {
		// Lose: return the spell to its owner's hand.
		//
		// CR §608.2b — at resolution we must check that the trigger's target
		// (the cast spell) is still on the stack. If it isn't, the bounce
		// effect does nothing for that target. This matters because
		// PushPerCardTrigger defers our handler via gs.pendingTriggers when
		// the cast event fires inside another resolution frame (CR §608.2c).
		// By the time pendingTriggers drains, the original spell may have
		// already resolved into graveyard / exile / hand. Pre-r54, the lose
		// branch unconditionally called MoveCard(card, owner, "stack",
		// "hand"); removeCardFromZone("stack") is a no-op (zone_move.go:239
		// — battlefield/stack source removal is the caller's responsibility),
		// so the engine appended the card to hand without removing it from
		// its actual current zone. Result: the same *Card pointer appearing
		// in both graveyard and hand — Loki r53 lead 1 (Glyph of Destruction
		// game 490 with Krark, the Thumbless).
		if stackIdx < 0 {
			emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
				"seat":  perm.Controller,
				"flip":  "lose",
				"spell": card.DisplayName(),
				"noop":  "spell_no_longer_on_stack",
				"rule":  "608.2b",
			})
			return
		}
		owner := card.Owner
		gs.Stack = append(gs.Stack[:stackIdx], gs.Stack[stackIdx+1:]...)
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
			"seat":     perm.Controller,
			"flip":     "lose",
			"spell":    card.DisplayName(),
			"returned": true,
		})
		return
	}

	// Win: copy the spell. CR §707.2 — the copy is created on the
	// stack; per CR §707.10 it ceases to exist on resolution rather
	// than going to a graveyard.
	if stackItem == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "spell_not_on_stack", map[string]interface{}{
			"flip":  "win",
			"spell": card.DisplayName(),
		})
		return
	}
	copyCard := gameengine.MintSpellCopy(gs, card)
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
