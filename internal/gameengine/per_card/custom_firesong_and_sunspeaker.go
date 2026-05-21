package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFiresongAndSunspeakerCustom wires the "red instant and sorcery
// spells you control have lifelink" static. The auto-generated handler
// in gen_firesong_and_sunspeaker.go already covers the white-I/S
// life-gain → 3-damage trigger.
//
// The engine only honors lifelink in the combat damage path; non-combat
// damage from a red I/S resolution goes through resolve.go's applyDamage
// which doesn't consult lifelink. To honor the printed text without
// engine surgery, we listen to noncombat_damage_to_player and
// noncombat_damage_to_creature events. When the source is a red I/S the
// controller cast, we gain life equal to the damage amount.
//
// Casting is tracked via OnTrigger("instant_or_sorcery_cast"): when a
// red I/S is cast by Firesong's controller, we stamp
// gs.Flags["firesong_red_active_<seat>"] with the gs.Turn. The damage
// hook checks the flag against the current turn — turn-scoped so stale
// state doesn't bleed across turns (lifelink doesn't bleed past the
// spell anyway, but the turn gate is a defensive belt + suspenders for
// the rare case where a non-Firesong red I/S deals damage on the same
// turn after a Firesong red I/S resolved).
func registerFiresongAndSunspeakerCustom(r *Registry) {
	r.OnTrigger("Firesong and Sunspeaker", "instant_or_sorcery_cast", firesongTrackRedCast)
	r.OnTrigger("Firesong and Sunspeaker", "noncombat_damage_to_player", firesongLifelinkOnPlayer)
	r.OnTrigger("Firesong and Sunspeaker", "noncombat_damage_to_creature", firesongLifelinkOnCreature)
}

func firesongTrackRedCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
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
	if !firesongIsRedInstantOrSorcery(card) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["firesong_red_active_seat"] = perm.Controller + 1
	gs.Flags["firesong_red_active_turn"] = gs.Turn
	gs.Flags["firesong_red_active_name_hash"] = firesongNameHash(card.DisplayName())
}

func firesongLifelinkOnPlayer(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	firesongMaybeGainLife(gs, perm, ctx, "to_player")
}

func firesongLifelinkOnCreature(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	firesongMaybeGainLife(gs, perm, ctx, "to_creature")
}

func firesongMaybeGainLife(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}, where string) {
	const slug = "firesong_red_is_lifelink_gain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if gs.Flags == nil {
		return
	}
	seatBox := gs.Flags["firesong_red_active_seat"]
	if seatBox != perm.Controller+1 {
		return
	}
	if gs.Flags["firesong_red_active_turn"] != gs.Turn {
		return
	}
	srcName, _ := ctx["source"].(string)
	if srcName != "" && firesongNameHash(srcName) != gs.Flags["firesong_red_active_name_hash"] {
		// Damage source isn't the red I/S we tagged. Skip.
		return
	}
	amt, _ := ctx["amount"].(int)
	if amt <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	seat.Life += amt
	gameengine.FireCardTrigger(gs, "life_gained", map[string]interface{}{
		"seat":   perm.Controller,
		"amount": amt,
		"source": perm.Card.DisplayName(),
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"amount": amt,
		"where":  where,
	})
}

func firesongIsRedInstantOrSorcery(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
		return false
	}
	for _, col := range c.Colors {
		if strings.EqualFold(col, "R") {
			return true
		}
	}
	for _, t := range c.Types {
		if t == "pip:R" {
			return true
		}
	}
	return false
}

// firesongNameHash is a tiny deterministic hash for spell names so we
// can stash an identity check in gs.Flags (an int map). Collisions are
// possible but the gate is one more layer over the seat + turn match,
// not a security boundary.
func firesongNameHash(s string) int {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
		h &= 0x7fffffff
	}
	if h == 0 {
		return 1
	}
	return h
}
