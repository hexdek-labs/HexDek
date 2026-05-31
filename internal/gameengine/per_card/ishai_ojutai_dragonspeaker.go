package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Ishai, Ojutai Dragonspeaker — {2}{W} 1/1 Legendary Creature — Bird
// Soldier.
//
//   Flying
//   Whenever an opponent casts a spell, put a +1/+1 counter on Ishai.
//   Partner
//
// Hooks the spell_cast_by_opponent fan-out fired unconditionally from
// stack.go (line 1955); handler scopes via caster_seat != controller.

func init() {
	registerIshaiOjutaiDragonspeaker(Global())
	AddResetHook(registerIshaiOjutaiDragonspeaker)
}

func registerIshaiOjutaiDragonspeaker(r *Registry) {
	r.OnTrigger("Ishai, Ojutai Dragonspeaker", "spell_cast_by_opponent", ishaiOppCast)
}

func ishaiOppCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ishai_grow_on_opp_cast"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"caster_seat":  casterSeat,
		"new_counters": perm.Counters["+1/+1"],
	})
}
