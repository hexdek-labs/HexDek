package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMysticRemora wires up Mystic Remora.
//
// Oracle text:
//
//	Cumulative upkeep {1}.
//	Whenever an opponent casts a noncreature spell, that player may
//	pay {4}. If the player doesn't, you draw a card.
//
// Same "pay unless / draw" pattern as Rhystic Study, but:
//   - Tax is 4 generic instead of 1.
//   - Only NONCREATURE spells trigger.
//   - Cumulative upkeep is a cost the CONTROLLER pays each upkeep —
//     tracked by a counter; if they can't/won't pay, sacrifice.
//
// Batch #2 additions (cumulative upkeep):
//   - OnTrigger("upkeep_controller", ctx={"active_seat":N}) —
//     CR §702.23a: at the beginning of the upkeep, put an age counter
//     on the permanent; then pay (cumulative_upkeep_cost × age_counters)
//     or sacrifice.
//   - Policy for "pay or sac": pay iff ManaPool ≥ total AND the
//     cumulative cost is ≤ 3 generic (typical cEDH threshold — beyond 3
//     the card advantage is no longer worth the tempo cost). In
//     addition, if the remora has drawn < 1 card this turn, pay (we'd
//     rather keep it up for at least one draw). Otherwise sacrifice.
func registerMysticRemora(r *Registry) {
	r.OnTrigger("Mystic Remora", "noncreature_spell_cast", mysticRemoraOnCast)
	r.OnTrigger("Mystic Remora", "upkeep_controller", mysticRemoraUpkeep)
}

func mysticRemoraOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mystic_remora"
	if gs == nil || perm == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster == perm.Controller {
		return // opponent-only trigger
	}
	opp := gs.Seats[caster]
	if opp == nil {
		return
	}
	// Greedy: pay {4} if affordable. cEDH usually DOESN'T pay Remora
	// (4 mana is a lot) but the engine default matches Rhystic's
	// policy until Hat-driven decisions land.
	if opp.ManaPool >= 4 {
		opp.ManaPool -= 4
		gameengine.SyncManaAfterSpend(opp)
		gs.LogEvent(gameengine.Event{
			Kind:   "pay_mana",
			Seat:   caster,
			Source: perm.Card.DisplayName(),
			Amount: 4,
			Details: map[string]interface{}{
				"reason": "mystic_remora_tax",
			},
		})
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"caster_seat": caster,
			"paid_tax":    true,
		})
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"caster_seat": caster,
		"paid_tax":    false,
	})
}

// mysticRemoraUpkeep implements the cumulative upkeep trigger
// (CR §702.23a). Age counters accumulate on the permanent; the
// controller pays (cost × counters) or sacrifices.
//
// Pay policy (cEDH-aligned): keep Remora for at most ~3 turns. After
// the third turn, sacrifice even if affordable — the card advantage is
// no longer worth the mana.
//
//   turn 1: 1 age counter → pay {1}  (cheap, keep)
//   turn 2: 2 age counters → pay {2}
//   turn 3: 3 age counters → pay {3} (marginal)
//   turn 4: 4 age counters → sacrifice
//
// Cumulative upkeep cost is read from the card's AST when present
// (Keyword(name="cumulative upkeep", args=("{1}", ))). For MVP we
// assume a hard-coded cost of 1 generic mana per counter, matching
// Mystic Remora's printed cost. Other cumulative-upkeep cards would
// need either a per-card handler of their own OR a generalized
// upkeep-cost inspector.
func mysticRemoraUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mystic_remora_cumulative_upkeep"
	if gs == nil || perm == nil {
		return
	}
	// Trigger fires for every seat's upkeep; only act on the
	// controller's own upkeep per CR §702.23a.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Step 1: put an age counter.
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["age"]++
	age := perm.Counters["age"]
	// Step 2: decide — pay or sac?
	// Cumulative upkeep cost is generic 1 per counter for Mystic Remora.
	const perCounter = 1
	dueCost := perCounter * age
	seat := gs.Seats[perm.Controller]
	// Policy: sacrifice after 3 age counters, or if unaffordable.
	// Paying a total of 6 generic (1+2+3) over three turns is the
	// typical cEDH Remora threshold.
	wantSac := age > 3 || seat.ManaPool < dueCost
	if wantSac {
		// Sacrifice via SacrificePermanent for proper zone-change handling:
		// replacement effects (Rest in Peace, etc.), dies/LTB triggers,
		// and commander redirect.
		gameengine.SacrificePermanent(gs, perm, "cumulative_upkeep_unpaid")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"age":           age,
			"cost_demanded": dueCost,
			"action":        "sacrifice",
		})
		return
	}
	// Pay: drain mana pool.
	seat.ManaPool -= dueCost
	gameengine.SyncManaAfterSpend(seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "pay_mana",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: dueCost,
		Details: map[string]interface{}{
			"reason": "cumulative_upkeep",
			"age":    age,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"age":           age,
		"cost_demanded": dueCost,
		"action":        "paid",
		"mana_after":    seat.ManaPool,
	})
}

