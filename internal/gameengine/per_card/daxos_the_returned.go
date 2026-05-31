package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Daxos the Returned — {1}{W}{B} 2/2 Legendary Creature — Zombie Soldier.
//
//   Whenever you cast an enchantment spell, you get an experience
//   counter.
//   {1}{W}{B}: Create a white and black Spirit enchantment creature
//   token. It has "This creature's power and toughness are each equal
//   to the number of experience counters you have."
//
// Implementation:
//   - OnTrigger spell_cast: filter caster_seat == controller +
//     ctx["card"] is enchantment; increment
//     gs.Seats[seat].Flags["experience_counters"].
//   - OnActivated: spawn a 0/0 Spirit Enchantment Creature token with
//     pip:W/B, then stamp a Modification with Power/Toughness equal to
//     current experience counters (the token's P/T scales with the XP
//     counter total at the time of creation — re-applied each cast).
//     The "each equal to the number of experience counters you have"
//     wording is a continuous self-referencing static effect; full §613
//     layer machinery would re-evaluate on every XP change. MVP: stamp
//     at creation time + emitPartial the dynamic re-evaluation.

func init() {
	registerDaxosTheReturned(Global())
	AddResetHook(registerDaxosTheReturned)
}

func registerDaxosTheReturned(r *Registry) {
	r.OnTrigger("Daxos the Returned", "spell_cast", daxosEnchantmentCast)
	r.OnActivated("Daxos the Returned", daxosActivate)
}

func daxosEnchantmentCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "daxos_xp_on_enchantment_cast"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	cardPtr, _ := ctx["card"].(*gameengine.Card)
	if cardPtr == nil || !cardHasType(cardPtr, "enchantment") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["experience_counters"]++
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"xp":   seat.Flags["experience_counters"],
	})
}

func daxosActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "daxos_spirit_token"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	xp := 0
	if seat.Flags != nil {
		xp = seat.Flags["experience_counters"]
	}
	token := gameengine.CreateCreatureToken(gs, src.Controller, "Spirit Token",
		[]string{"creature", "enchantment", "spirit", "pip:W", "pip:B"}, 0, 0)
	if token == nil {
		return
	}
	// Stamp current XP as a Modification so Power()/Toughness() report
	// xp/xp. Per-creation snapshot (MVP) — dynamic re-evaluation on XP
	// change is layer-7-static territory and is flagged below.
	if xp > 0 {
		token.Modifications = append(token.Modifications, gameengine.Modification{
			Power:     xp,
			Toughness: xp,
			Duration:  "permanent",
			Timestamp: gs.NextTimestamp(),
		})
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
		"xp":   xp,
		"pt":   [2]int{xp, xp},
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"spirit_token_pt_snapshot_at_creation_not_dynamic_to_xp_changes")
}
