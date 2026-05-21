package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlpharaelStonechosen wires Alpharael, Stonechosen.
//
// Oracle text:
//
//	Ward—Discard a card at random.
//	Void — Whenever Alpharael attacks, if a nonland permanent left
//	the battlefield this turn or a spell was warped this turn,
//	defending player loses half their life, rounded up.
//
// Implementation:
//   - Ward: handled by the AST keyword pipeline.
//   - "permanent_ltb" trigger: arms a turn flag whenever a nonland
//     permanent leaves the battlefield. The flag lives on the gs.Flags
//     map so all attack triggers in the same turn can read it.
//   - "creature_attacks" trigger: gate on attacker == Alpharael self.
//     If the void condition is armed (nonland permanent left this
//     turn — the only condition we can check; "spell was warped" is
//     not yet engine-visible), the defender loses ceil(life/2).
//
// emitPartial: warp-spell-tracking is engine-side TODO.
func registerAlpharaelStonechosen(r *Registry) {
	r.OnETB("Alpharael, Stonechosen", alpharaelETBWardStamp)
	r.OnTrigger("Alpharael, Stonechosen", "permanent_ltb", alpharaelTrackVoid)
	r.OnTrigger("Alpharael, Stonechosen", "creature_attacks", alpharaelAttacks)
}

// alpharaelETBWardStamp (R52 batch K) stamps the "Ward—Discard a card
// at random" cost on Alpharael as the canonical ward + ward_kind perm
// flags. The target dispatcher reads ward_kind to route Ward to the
// random-discard executor instead of the default mana cost.
func alpharaelETBWardStamp(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "alpharael_ward_discard_random"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["ward"] = 1                       // canonical "ward is present" surface
	perm.Flags["ward_discard_random"] = 1        // ward-kind variant
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"ward_kind": "discard_random",
	})
}

func alpharaelTrackVoid(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["card"].(*gameengine.Card)
	if leaving == nil {
		return
	}
	if cardHasType(leaving, "land") {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["alpharael_void_armed_turn"] = gs.Turn
}

func alpharaelAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alpharael_void_half_life"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	armed := gs.Flags != nil && gs.Flags["alpharael_void_armed_turn"] == gs.Turn
	if !armed {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"void_condition_not_armed_or_warp_tracking_missing")
		return
	}
	defender := -1
	if v, ok := ctx["defender"].(int); ok {
		defender = v
	} else if v, ok := ctx["defender_seat"].(int); ok {
		defender = v
	}
	if defender < 0 || defender >= len(gs.Seats) || gs.Seats[defender] == nil {
		return
	}
	life := gs.Seats[defender].Life
	if life <= 0 {
		return
	}
	loss := (life + 1) / 2 // ceiling of life/2
	gameengine.LoseLife(gs, defender, loss, "Alpharael, Stonechosen")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"defender_seat": defender,
		"life_lost":     loss,
	})
}
