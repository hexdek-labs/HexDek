package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYennaRedtoothRegentCustom replaces the auto-generated Yenna
// stub (which spawned a generic 1/1 token) with the actual enchantment
// copy + scry-2-untap-on-aura behavior.
//
// Oracle text:
//
//	{2}, {T}: Choose target enchantment you control that doesn't have
//	the same name as another permanent you control. Create a token
//	that's a copy of it, except it isn't legendary. If the token is an
//	Aura, untap Yenna, then scry 2. Activate only as a sorcery.
//
// We pick the highest-CMC unique-named enchantment we control as the
// copy target; that maximizes the value of the copy in MCTS. Aura
// targeting (the "attached to" choice) is not modeled — we untap +
// scry-2 if the source was an Aura.
func registerYennaRedtoothRegentCustom(r *Registry) {
	r.OnActivated("Yenna, Redtooth Regent", yennaCopyEnchantment)
}

func yennaCopyEnchantment(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yenna_copy_enchantment"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Cost-availability gates checked before target selection so we
	// don't lock the player into an illegal activation: sorcery-speed,
	// untapped, ≥2 mana. We pay AFTER confirming a legal target.
	if !isSorcerySpeed(gs, src.Controller) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	if seat.ManaPool < 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  2,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	// Build a multiset of perm names we control to enforce the
	// "different name from any permanent you control" gate.
	names := map[string]int{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		names[p.Card.DisplayName()]++
	}
	// Pick best enchantment we control whose name is unique on our
	// battlefield (i.e. only Yenna-target itself owns the name).
	var pick *gameengine.Permanent
	bestCMC := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsEnchantment() {
			continue
		}
		if names[p.Card.DisplayName()] > 1 {
			continue
		}
		c := cardCMC(p.Card)
		if c > bestCMC {
			pick = p
			bestCMC = c
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_enchantment_target", nil)
		return
	}
	// Pay the cost now that the target check has passed.
	seat.ManaPool -= 2
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true
	// Create a token copy. Strip "legendary" by ensuring the type list
	// excludes it; we add "token" and copy other types.
	types := append([]string{"token"}, pick.Card.Types...)
	tokenCard := &gameengine.Card{
		Name:          pick.Card.DisplayName() + " (Yenna token)",
		Owner:         src.Controller,
		Types:         types,
		BasePower:     pick.Card.BasePower,
		BaseToughness: pick.Card.BaseToughness,
	}
	tokenPerm := enterBattlefieldWithETB(gs, src.Controller, tokenCard, false)
	if tokenPerm == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "token_create_failed", nil)
		return
	}
	// If source was an Aura, untap Yenna and scry 2.
	if cardSubtypeMatches(pick.Card, "aura") {
		src.Tapped = false
		gameengine.Scry(gs, src.Controller, 2)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"copied":    pick.Card.DisplayName(),
		"was_aura":  cardSubtypeMatches(pick.Card, "aura"),
	})
}
