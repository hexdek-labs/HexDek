package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKorvoldFaeCursedKing wires Korvold, Fae-Cursed King.
//
// Oracle text:
//
//	Flying
//	Whenever Korvold, Fae-Cursed King enters or attacks, sacrifice
//	another permanent.
//	Whenever you sacrifice a permanent, put a +1/+1 counter on Korvold,
//	Fae-Cursed King and draw a card.
//
// Implementation:
//   - Flying — AST keyword pipeline.
//   - OnETB and OnTrigger("creature_attacks") — fire the "sacrifice
//     another permanent" trigger. Picks the cheapest expendable
//     fodder (token first, then lowest-CMC nonland non-Korvold).
//   - OnTrigger("permanent_sacrificed") — when a permanent controlled
//     by Korvold's controller is sacrificed, place a +1/+1 counter on
//     Korvold and draw a card. The sacrifice from Korvold's own
//     ETB/attack trigger chains into this one organically; the engine's
//     trigger depth cap prevents runaway recursion.
func registerKorvoldFaeCursedKing(r *Registry) {
	r.OnETB("Korvold, Fae-Cursed King", korvoldETB)
	r.OnTrigger("Korvold, Fae-Cursed King", "creature_attacks", korvoldAttack)
	r.OnTrigger("Korvold, Fae-Cursed King", "permanent_sacrificed", korvoldOnSacrifice)
}

func korvoldETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	korvoldSacAnother(gs, perm, "etb")
}

func korvoldAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	korvoldSacAnother(gs, perm, "attack")
}

func korvoldSacAnother(gs *gameengine.GameState, perm *gameengine.Permanent, source string) {
	const slug = "korvold_fae_cursed_king_sac_trigger"
	if gs == nil || perm == nil {
		return
	}
	victim := chooseKorvoldSacFodder(gs, perm.Controller, perm)
	if victim == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_fodder", map[string]interface{}{
			"seat":    perm.Controller,
			"trigger": source,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "korvold_trigger")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"trigger":      source,
		"sacrificed":   victimName,
	})
}

func korvoldOnSacrifice(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "korvold_fae_cursed_king_growth_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	seat := gs.Seats[perm.Controller]
	drawn := false
	if seat != nil && !seat.Lost && len(seat.Library) > 0 {
		top := seat.Library[0]
		if top != nil {
			gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "korvold_draw")
			drawn = true
		}
	}

	sacrificedName := ""
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		sacrificedName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"counters":   perm.Counters["+1/+1"],
		"drew":       drawn,
		"sacrificed": sacrificedName,
	})
}

// chooseKorvoldSacFodder returns the best permanent to sacrifice for the
// Korvold trigger. Priority:
//
//  1. Token nonland permanents (Treasure, Food, Clue, generic tokens).
//  2. Lowest-CMC nonland non-Korvold permanent (cheap creature fodder).
//
// Returns nil if no other permanent is on the battlefield.
func chooseKorvoldSacFodder(gs *gameengine.GameState, seat int, src *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil {
		return nil
	}

	// Pass 1: tokens.
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "token") {
			return p
		}
	}

	// Pass 2: lowest-CMC nonland.
	var best *gameengine.Permanent
	bestCMC := 999
	for _, p := range s.Battlefield {
		if p == nil || p == src || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			bestCMC = cmc
			best = p
		}
	}
	return best
}
