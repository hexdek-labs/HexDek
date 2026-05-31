package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBloodchiefAscension wires Bloodchief Ascension.
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	At the beginning of each end step, if an opponent lost 2 or more
//	life this turn, you may put a quest counter on this enchantment.
//	(Damage causes loss of life.)
//	Whenever a card enters an opponent's graveyard from any source and
//	this enchantment has three or more quest counters, you may have
//	that player lose 2 life and you gain 2 life.
//
// Implementation (Muninn gap #4 — 223,731 hits):
//   - OnTrigger("end_step"): scan every opponent's Seat.Flags
//     ["life_lost_this_turn"] (maintained by LoseLife / DealDamage). If any
//     opponent crossed the 2-life threshold this turn, add a "quest"
//     counter. Fires on every player's end step ("each end step").
//   - OnTrigger("zone_change"): when a card lands in an opponent's
//     graveyard and the enchantment carries 3+ quest counters, drain 2
//     life from that opponent and gain 2 for the controller. AI policy
//     always opts in — the upside is monotone and the cost is zero.
//   - "you may" choice is auto-accepted both halves — this is a
//     deliberate AI policy choice (quest counter is strictly upside;
//     drain is monotone life-swing), NOT a missing engine feature.
//     R60 batch 4: dropped the stale "may_choice_auto_accepted"
//     emitPartials that read as gaps in audit tooling.
func registerBloodchiefAscension(r *Registry) {
	r.OnTrigger("Bloodchief Ascension", "end_step", bloodchiefAscensionEndStep)
	r.OnTrigger("Bloodchief Ascension", "zone_change", bloodchiefAscensionDrain)
}

func bloodchiefAscensionEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bloodchief_ascension_end_step_quest"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	threshold := false
	for _, oppSeat := range gs.Opponents(controller) {
		if oppSeat < 0 || oppSeat >= len(gs.Seats) {
			continue
		}
		os := gs.Seats[oppSeat]
		if os == nil || os.Lost {
			continue
		}
		if os.Turn.LifeLost >= 2 {
			threshold = true
			break
		}
		if os.Flags != nil && os.Flags["life_lost_this_turn"] >= 2 {
			threshold = true
			break
		}
	}
	if !threshold {
		return
	}

	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.AddCounter("quest", 1)

	emit(gs, slug, "Bloodchief Ascension", map[string]interface{}{
		"seat":           controller,
		"quest_counters": perm.Counters["quest"],
		"active_seat":    ctx["active_seat"],
	})
}

func bloodchiefAscensionDrain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "bloodchief_ascension_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if perm.Counters == nil || perm.Counters["quest"] < 3 {
		return
	}

	toZone, _ := ctx["to_zone"].(string)
	if toZone != "graveyard" {
		return
	}
	ownerSeat, _ := ctx["seat"].(int)
	if ownerSeat == perm.Controller {
		return
	}
	if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
		return
	}
	os := gs.Seats[ownerSeat]
	if os == nil || os.Lost {
		return
	}

	gameengine.LoseLife(gs, ownerSeat, 2, "Bloodchief Ascension")
	gameengine.GainLife(gs, perm.Controller, 2, "Bloodchief Ascension")

	cardName, _ := ctx["card"].(string)
	emit(gs, slug, "Bloodchief Ascension", map[string]interface{}{
		"seat":         perm.Controller,
		"target_seat":  ownerSeat,
		"card":         cardName,
		"life_drained": 2,
		"from_zone":    ctx["from_zone"],
	})
}
