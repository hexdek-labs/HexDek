package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSmellerbeeRebelFighter wires Smellerbee, Rebel Fighter.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	First strike
//	Other creatures you control have haste.
//	Whenever Smellerbee attacks, you may discard your hand. If you
//	do, draw cards equal to the number of attacking creatures.
//
// Implementation:
//   - First strike + haste-grant via AST keyword pipeline (haste is
//     applied as a static via the AST static-grant layer; we don't
//     need a runtime hook for it).
//   - "creature_attacks" trigger: when Smellerbee attacks, count attacking
//     creatures Smellerbee's controller controls; if that's strictly
//     more than current hand size we opt-in (net positive draw),
//     otherwise skip.
func registerSmellerbeeRebelFighter(r *Registry) {
	r.OnTrigger("Smellerbee, Rebel Fighter", "creature_attacks", smellerbeeAttacks)
}

func smellerbeeAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "smellerbee_attack_discard_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	attackers := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.IsAttacking() {
			attackers++
		}
	}
	handSize := len(seat.Hand)
	if attackers <= handSize {
		emitFail(gs, slug, perm.Card.DisplayName(), "discard_value_negative", map[string]interface{}{
			"seat":      perm.Controller,
			"hand":      handSize,
			"attackers": attackers,
		})
		return
	}
	// Route through DiscardCard per-card so §702.34a Madness /
	// §702.187 Mayhem / Necropotence reroute / card_discarded trigger
	// (Liliana's Caress / Waste Not / Tergrid) / Turn.Discarded all
	// see the discards. Pre-r60-normalize this path bulk-spliced
	// seat.Hand=nil + raw seat.Graveyard appends, silently bypassing
	// every one of those — discarding an entire hand of Madness
	// cards via Smellerbee should Madness-cast each one, not
	// graveyard-dump them. Snapshot the hand BEFORE the loop because
	// DiscardCard mutates seat.Hand via MoveCard.
	hand := append([]*gameengine.Card(nil), seat.Hand...)
	for _, c := range hand {
		if c == nil {
			continue
		}
		gameengine.DiscardCard(gs, c, perm.Controller)
		gs.LogEvent(gameengine.Event{
			Kind:   "discard",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":   c.DisplayName(),
				"reason": "smellerbee_attack",
			},
		})
	}
	drawn := 0
	for i := 0; i < attackers; i++ {
		if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"discarded": handSize,
		"drawn":     drawn,
		"attackers": attackers,
	})
}
