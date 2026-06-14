package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerElugeTheShorelessSea wires Eluge, the Shoreless Sea.
//
// Oracle text:
//
//	Eluge's power and toughness are each equal to the number of Islands
//	you control.
//	Whenever Eluge enters or attacks, put a flood counter on target
//	land. It's an Island in addition to its other types for as long as
//	it has a flood counter on it.
//	The first instant or sorcery spell you cast each turn costs {U}
//	(or {1}) less to cast for each land you control with a flood
//	counter on it.
//
// CDA p/t now wired via Layer 7b (R55) — see elugeFlood.
// Cost reduction is AST-resolved.
func registerElugeTheShorelessSea(r *Registry) {
	r.OnETB("Eluge, the Shoreless Sea", elugeFlood)
	r.OnTrigger("Eluge, the Shoreless Sea", "attacks", elugeFloodAttack)
}

func elugeFlood(gs *gameengine.GameState, perm *gameengine.Permanent) {
	// R55: P/T = Islands you control. Layer 7b CDA via dynamic primitive.
	// Lands with a flood counter count as Islands too, per the granted
	// type clause — we approximate by checking BOTH "island" type AND
	// the "flood" counter on the controller's lands.
	gameengine.RegisterDynamicSetPT(gs, perm, elugeCountIslands,
		gameengine.DurationUntilSourceLeaves, "Eluge, the Shoreless Sea", "cda_pt")
	gs.InvalidateCharacteristicsCache()
	elugePlaceFloodCounter(gs, perm)
}

func elugeCountIslands(gs *gameengine.GameState, perm *gameengine.Permanent) (int, int) {
	if gs == nil || perm == nil {
		return 0, 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0, 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		if cardHasType(p.Card, "island") {
			n++
			continue
		}
		// Flood-counter lands count as Islands per Eluge's own static
		// (printed text: "It's an Island in addition to its other types
		// for as long as it has a flood counter on it.").
		if p.Counters != nil && p.Counters["flood"] > 0 {
			n++
		}
	}
	return n, n
}

func elugeFloodAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
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
	elugePlaceFloodCounter(gs, perm)
}

func elugePlaceFloodCounter(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eluge_flood_counter"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick a non-Island land we control without a flood counter yet.
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if p.Counters != nil && p.Counters["flood"] > 0 {
			continue
		}
		if cardHasType(p.Card, "island") {
			continue
		}
		p.AddCounter("flood", 1)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"target": p.Card.DisplayName(),
		})
		return
	}
}
