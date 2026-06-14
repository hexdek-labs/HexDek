package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdelineResplendentCathar wires Adeline, Resplendent Cathar.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{1}{W}{W}
//	Legendary Creature — Human Knight
//	Vigilance
//	Adeline's power is equal to the number of creatures you control.
//	Whenever you attack, for each opponent, create a 1/1 white Human
//	creature token that's tapped and attacking that player or a
//	planeswalker they control.
//
// Implementation:
//   - ETB: refresh Adeline's temp_power to creature count (recompute on
//     ETB; layered static would need full layers wiring — emitPartial).
//   - declare_attackers gated to controller: mint one 1/1 W Human token
//     per living opponent, tapped + attacking.
func registerAdelineResplendentCathar(r *Registry) {
	r.OnETB("Adeline, Resplendent Cathar", adelineETB)
	r.OnTrigger("Adeline, Resplendent Cathar", "declare_attackers", adelineAttacks)
}

func adelineETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "adeline_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// R55: power = creatures you control. Layer 7b CDA via dynamic
	// power-only primitive (toughness stays at printed 3).
	gameengine.RegisterDynamicSetPower(gs, perm, adelineCountCreatures,
		gameengine.DurationUntilSourceLeaves, "Adeline, Resplendent Cathar", "cda_power")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func adelineCountCreatures(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			n++
		}
	}
	return n
}

func adelineAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "adeline_attack_token_per_opponent"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// "Whenever YOU attack" — a single player-level attack-declaration event
	// that fires EXACTLY ONCE per combat regardless of how many creatures
	// attack. The engine dispatches creature_attacks ONCE PER DECLARED ATTACKER
	// with attacker_seat in ctx (NOT active_seat — that key is never populated
	// for attacks, so the old read returned 0: Adeline was inert for every
	// non-seat-0 controller AND minted one full batch per attacker for seat 0).
	// Read canonical attacker_seat + gate to the first hit per turn
	// (Caesar/Raffine once-per-turn dedup). r63 you-attack cardinality audit.
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["adeline_attack_fired"] >= gs.Turn {
		return
	}
	perm.Flags["adeline_attack_fired"] = gs.Turn
	tokens := 0
	for i, opp := range gs.Seats {
		if opp == nil || opp.Lost || i == perm.Controller {
			continue
		}
		token := &gameengine.Card{
			Name:          "Human Token",
			Owner:         perm.Controller,
			BasePower:     1,
			BaseToughness: 1,
			Types:         []string{"token", "creature", "human"},
			Colors:        []string{"W"},
			TypeLine:      "Token Creature — Human",
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, true)
		tokens++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"tokens": tokens,
	})
}
