package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAtemsisAllSeeing wires Atemsis, All-Seeing.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Flying
//	{2}{U}, {T}: Draw two cards, then discard a card.
//	Whenever Atemsis deals damage to an opponent, you may reveal your
//	hand. If cards with at least six different mana values are revealed
//	this way, that player loses the game.
//
// Implementation:
//   - "combat_damage_player": gate on damage_seat == perm.Controller and
//     source_perm == perm. Count distinct mana values in controller's
//     hand; if >= 6, mark the damaged opponent as Lost.
//   - {2}{U},{T} loot (R60 stub sweep batch 4): tap, drawOne twice,
//     then DiscardCard one card from hand (pick the highest-CMC card
//     to mirror standard loot AI policy — keep cheap cards, dump
//     expensive ones we can't cast yet). Mana cost {2}{U} is enforced
//     by the activation cost pipeline upstream of this hook.
//   - Flying handled by AST keyword pipeline.
func registerAtemsisAllSeeing(r *Registry) {
	r.OnTrigger("Atemsis, All-Seeing", "combat_damage_player", atemsisCombatDamage)
	r.OnActivated("Atemsis, All-Seeing", atemsisActivate)
}

func atemsisCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "atemsis_six_mana_values_loss"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	src, _ := ctx["source_perm"].(*gameengine.Permanent)
	if src != perm {
		return
	}
	defenderSeat, ok := ctx["target_seat"].(int)
	if !ok {
		return
	}
	if defenderSeat == perm.Controller || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}

	seen := map[int]bool{}
	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		seen[gameengine.ManaCostOf(c)] = true
	}
	distinct := len(seen)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"target_seat":     defenderSeat,
		"distinct_values": distinct,
		"hand_size":       len(s.Hand),
	})

	if distinct >= 6 {
		// CR §104.3e: route the loss through the canonical helper
		// FIRST so §614 would_lose_game (Platinum Angel / Angel's
		// Grace) can cancel. Pre-r60-normalization the ordering was
		// inverted — emitWin direct-set s.Lost=true on all opp seats
		// before the helper ran, which made the helper short-circuit
		// at its `if s.Lost { return false }` guard and silently
		// bypass the §614 replacement chain. Helper-first preserves
		// §614 semantics; emitWin only fires if the loss wasn't
		// cancelled. Helper emits the canonical `lose_game` Event
		// with the source-name suffix preserving the mechanism
		// detail — no separate `player_loses` Event needed.
		applied := gameengine.MarkSeatLostByEffect(gs, defenderSeat,
			perm.Card.DisplayName()+" — six distinct mana values among cards in hand")
		if applied {
			emitWin(gs, perm.Controller, slug, perm.Card.DisplayName(),
				"opponent_loses_atemsis_six_distinct_mana_values")
			_ = gs.CheckEnd()
		}
	}
}

func atemsisActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "atemsis_loot_ability"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_seat", nil)
		return
	}
	src.Tapped = true
	drewBefore := len(seat.Hand)
	drawOne(gs, src.Controller, src.Card.DisplayName())
	drawOne(gs, src.Controller, src.Card.DisplayName())
	drew := len(seat.Hand) - drewBefore

	// Pick highest-CMC card from hand to discard. Standard loot AI:
	// dump expensive uncastables, keep cheap cards.
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pick = c
		}
	}
	discarded := ""
	if pick != nil {
		discardName := pick.DisplayName()
		gameengine.DiscardCard(gs, pick, src.Controller)
		discarded = discardName
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"drew":      drew,
		"discarded": discarded,
	})
}
