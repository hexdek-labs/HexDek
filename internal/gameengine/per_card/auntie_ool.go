package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAuntieOol wires Auntie Ool, Cursewretch.
//
// Oracle text:
//
//	Ward—Blight 2. (To blight 2, a player puts two -1/-1 counters on a
//	creature they control.)
//	Whenever one or more -1/-1 counters are put on a creature, draw a
//	card if you control that creature. If you don't control it, its
//	controller loses 1 life.
//
// Implementation:
//   - ETB: emitPartial for ward—blight 2 (a non-mana alternative ward
//     payment that itself places counters; engine ward grants only
//     model mana costs cleanly).
//   - counter_placed (custom engine event fired from resolveCounterMod
//     when "put" succeeds): if -1/-1 went on a creature, either Auntie's
//     controller draws (creature is theirs) or the creature's controller
//     loses 1 life.
//
// Note: this trigger is "one or more counters at once", which is a
// per-EVENT trigger rather than per-counter. resolveCounterMod fires
// counter_placed once per target with the aggregate amount, which
// matches the rules wording for AST-driven counter applications.
// Counter placements outside resolveCounterMod (combat infect/wither)
// don't currently fire counter_placed; emitPartial flags the gap.
func registerAuntieOol(r *Registry) {
	r.OnETB("Auntie Ool, Cursewretch", auntieOolETB)
	r.OnTrigger("Auntie Ool, Cursewretch", "counter_placed", auntieOolCounterPlaced)
}

func auntieOolETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "auntie_ool_ward_blight"
	if gs == nil || perm == nil {
		return
	}
	// R60 closure of half-finished-features-r48 #4: route Blight 2 alt-
	// payment ward through ward_alt_payment.go's canonical handler.
	// Replaces the prior "ward_blight_2_alt_payment_unimplemented"
	// emitPartial. The engine puts 2 -1/-1 counters on the targeting
	// opponent's lowest-toughness creature; if they have no creatures
	// the targeting spell is countered per CR §702.21c.
	// r60 — migrated 2026-05-27 to unified WardCost primitive. Amount=2
	// is the Blight value (counters placed); payWardByBlight routes
	// accordingly.
	gameengine.SetWardCost(perm, gameengine.WardCost{
		Type:   gameengine.WardCostBlight,
		Amount: 2,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"ward_alt_kind": "blight",
		"counters":      2,
	})
	// Out-of-scope gap (NOT half-finished #4): combat infect/wither
	// counter placements don't currently fire the counter_placed event,
	// so Auntie's draw/drain reaction misses those sources. Engine-side
	// fan-out; kept as a partial for a future PR.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"counter_placed_not_fired_from_combat_infect_wither")
}

func auntieOolCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "auntie_ool_minus_counter_response"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	kind, _ := ctx["counter_kind"].(string)
	if kind != "-1/-1" {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		return
	}
	if !target.IsCreature() {
		return
	}
	targetSeat, _ := ctx["target_seat"].(int)
	if targetSeat == perm.Controller {
		drawOne(gs, perm.Controller, perm.Card.DisplayName())
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"target_card":   target.Card.DisplayName(),
			"effect":        "draw",
		})
		return
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[targetSeat]
	if s == nil || s.Lost {
		return
	}
	gameengine.LoseLife(gs, targetSeat, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target_card":  target.Card.DisplayName(),
		"target_seat":  targetSeat,
		"effect":       "drain",
	})
	_ = gs.CheckEnd()
}
