package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerApheliaViperWhispererCustom wires Aphelia's
// "{4}{B}: Until end of turn, whenever one or more Gorgons and/or
// Snakes you control deal combat damage to a player, that player
// loses half their life, rounded up." tribal-damage activated.
//
// The hand-written aphelia_viper_whisperer.go covers the attack-
// trigger Snake-token spawn; its inline comment flags the activated
// half as emitPartial. We close the activated ability here:
//
//   - {4}{B} = 5 mana. Defensive deduction from ManaPool / Mana.
//   - Stamp seat.Flags["aphelia_tribal_damage_eot_turn"] = current
//     turn so the damage-doubler dispatcher recognizes the active
//     turn marker.
//   - Subscribe to creature_combat_damage_to_player while active.
//     On dispatch: if the source is a Gorgon or Snake controlled by
//     Aphelia's controller AND the marker is set for this turn,
//     halve the defender's life (rounded up via LoseLife).
const apheliaTribalDamageMarker = "aphelia_tribal_damage_eot_turn"

func registerApheliaViperWhispererCustom(r *Registry) {
	r.OnActivated("Aphelia, Viper Whisperer", apheliaActivateTribalHalving)
	r.OnTrigger("Aphelia, Viper Whisperer", "creature_combat_damage_to_player", apheliaTribalHalvingDispatch)
}

func apheliaActivateTribalHalving(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "aphelia_tribal_halving_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Defensive cost check: {4}{B} = 5 generic-equivalent. Engine
	// dispatcher usually pre-deducts; only deduct if pool covers it.
	if seat.ManaPool >= 5 {
		seat.ManaPool -= 5
	} else if seat.Mana != nil && (seat.Mana.W+seat.Mana.U+seat.Mana.B+seat.Mana.R+seat.Mana.G+seat.Mana.C+seat.Mana.Any) < 5 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", nil)
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags[apheliaTribalDamageMarker] = gs.Turn + 1
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          src.Controller,
		"active_turn":   gs.Turn,
		"tribal_filter": []string{"gorgon", "snake"},
	})
}

func apheliaTribalHalvingDispatch(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aphelia_tribal_halving_dispatch"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	// Only active on the turn we activated. Marker is stored as turn+1
	// (mirrors the gisa_hellraiser_fired_turn convention).
	if seat.Flags[apheliaTribalDamageMarker] != gs.Turn+1 {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Card == nil || atk.Controller != perm.Controller {
		return
	}
	if !cardHasSubtype(atk.Card, "gorgon") && !cardHasSubtype(atk.Card, "snake") {
		return
	}
	defSeat, _ := ctx["target_seat"].(int)
	if defSeat <= 0 || defSeat >= len(gs.Seats) {
		defSeat, _ = ctx["defending_seat"].(int)
	}
	if defSeat < 0 || defSeat == perm.Controller || defSeat >= len(gs.Seats) {
		return
	}
	d := gs.Seats[defSeat]
	if d == nil || d.Lost {
		return
	}
	// Halve life, rounded UP — printed text. life=21 → loses 11 (down to 10).
	loss := (d.Life + 1) / 2
	if loss < 0 {
		loss = 0
	}
	gameengine.LoseLife(gs, defSeat, loss, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_seat": defSeat,
		"life_lost":   loss,
	})
}
