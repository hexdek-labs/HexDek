package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mind_whip.go — per_card handler for Mind Whip.
//
// Oracle text (Enchantment — Aura, {2}{U}):
//
//	Enchant creature
//	At the beginning of the upkeep of enchanted creature's controller,
//	that player may pay {3}. If they don't, this Aura deals 2 damage to
//	that player and you tap that creature.
//
// Why this needs a per_card handler: the parser emitted
// `untyped_effect` + `if_intervening_tail` + `prev_action_response`
// nodes (the raw "they / deny" decision fragment), none of which the
// engine acted on — so Mind Whip was an inert aura. Verified inert: no
// prior handler.
//
// Implementation (OnTrigger "upkeep"): on the enchanted creature's
// controller's upkeep, AI policy never pays the {3} tax (punisher
// downside), so the Aura deals 2 damage to that player (modelled via
// LoseLife so §704.5a life-total SBA bookkeeping fires) and taps the
// enchanted creature.

func init() {
	registerMindWhip(Global())
	AddResetHook(registerMindWhip)
}

func registerMindWhip(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Mind Whip", "upkeep", mindWhipUpkeep)
}

func mindWhipUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mind_whip"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enchanted := perm.AttachedTo
	if enchanted == nil {
		return // not attached to anything (shouldn't persist, but be safe)
	}
	victimSeat := enchanted.Controller
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != victimSeat {
		return // only the enchanted creature's controller's upkeep
	}
	if victimSeat < 0 || victimSeat >= len(gs.Seats) {
		return
	}

	// AI never pays the {3}: deal 2 damage to that player and tap the creature.
	gameengine.LoseLife(gs, victimSeat, 2, "Mind Whip")
	enchanted.Tapped = true

	emit(gs, slug, "Mind Whip", map[string]interface{}{
		"seat":   victimSeat,
		"damage": 2,
		"tapped": enchanted.Card.DisplayName(),
	})
	_ = gs.CheckEnd()
}
