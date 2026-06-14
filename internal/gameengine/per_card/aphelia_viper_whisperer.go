package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerApheliaViperWhisperer wires Aphelia, Viper Whisperer.
//
// Oracle text:
//
//	Deathtouch
//	Whenever Aphelia attacks, you may pay {1}{B/G}. If you do, create
//	a 1/1 black Snake creature token with deathtouch.
//	{4}{B}: Until end of turn, whenever one or more Gorgons and/or
//	Snakes you control deal combat damage to a player, that player
//	loses half their life, rounded up.
//
// Implementation:
//   - attacks trigger: pay {1}{B/G} (2 mana) heuristically — always yes
//     if seat has >=2 available mana — and create a 1/1 deathtouch Snake.
//   - Activated tribal-damage trigger: emitPartial (delayed continuous
//     halving trigger is non-trivial and rarely activated in sim).
func registerApheliaViperWhisperer(r *Registry) {
	r.OnTrigger("Aphelia, Viper Whisperer", "attacks", apheliaAttacks)
}

func apheliaAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aphelia_attacks_snake_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever ~ attacks" fires only when this creature
	// itself is the attacker. The creature_attacks event is dispatched once per
	// declared attacker, so the old seat check (ctx["seat"] is never populated by
	// the attack fire → it only matched seat 0) let the effect multiply per
	// attacker. Mirrors the canonical gate and the Aang and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if !apheliaTryPay(gs, seat) {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"paid": false,
		})
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Snake Token",
		[]string{"creature", "snake", "pip:B"}, 1, 1)
	if tok != nil {
		if tok.Flags == nil {
			tok.Flags = map[string]int{}
		}
		tok.Flags["kw:deathtouch"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"paid": true,
	})
}

// apheliaTryPay spends 2 mana (1 generic + 1 B-or-G hybrid). Heuristic:
// require a B or G pip, plus any 1 generic. Falls back to legacy pool.
func apheliaTryPay(gs *gameengine.GameState, seat *gameengine.Seat) bool {
	if seat == nil {
		return false
	}
	if seat.Mana != nil {
		// Need at least 1 from {B,G,Any} and 1 generic from anywhere.
		total := seat.Mana.W + seat.Mana.U + seat.Mana.B + seat.Mana.R + seat.Mana.G + seat.Mana.C + seat.Mana.Any
		if total < 2 {
			return false
		}
		hybridPaid := false
		if seat.Mana.B > 0 {
			seat.Mana.B--
			hybridPaid = true
		} else if seat.Mana.G > 0 {
			seat.Mana.G--
			hybridPaid = true
		} else if seat.Mana.Any > 0 {
			seat.Mana.Any--
			hybridPaid = true
		}
		if !hybridPaid {
			return false
		}
		// Now pay 1 generic.
		switch {
		case seat.Mana.Any > 0:
			seat.Mana.Any--
		case seat.Mana.C > 0:
			seat.Mana.C--
		case seat.Mana.W > 0:
			seat.Mana.W--
		case seat.Mana.U > 0:
			seat.Mana.U--
		case seat.Mana.B > 0:
			seat.Mana.B--
		case seat.Mana.R > 0:
			seat.Mana.R--
		case seat.Mana.G > 0:
			seat.Mana.G--
		}
		gameengine.SyncManaAfterSpend(seat)
		return true
	}
	if seat.ManaPool >= 2 {
		seat.ManaPool -= 2
		gameengine.SyncManaAfterSpend(seat)
		return true
	}
	return false
}
