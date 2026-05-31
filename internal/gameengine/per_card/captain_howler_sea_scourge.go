package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaptainHowlerSeaScourge wires Captain Howler, Sea Scourge.
//
// Oracle text (Scryfall, verified 2026-05-30, {2}{U}{R}, 5/4
// Legendary Creature — Shark Pirate):
//
//	Ward—{2}, Pay 2 life.
//	Whenever you discard one or more cards, target creature gets +2/+0
//	until end of turn for each card discarded this way. Whenever that
//	creature deals combat damage to a player this turn, you draw a
//	card.
//
// Implementation (R60 stub sweep):
//   - card_discarded trigger: pump the controller's biggest creature
//     by +2N/+0 UEOT (N = card count). Then stamp
//     Flags["captain_howler_drawer_until_turn"] = gs.Turn+1 on the
//     pumped creature so the combat-damage trigger below can identify
//     it as the "that creature" referent.
//   - combat_damage_player trigger: when ANY combat damage is dealt to
//     a player, scan our controller's battlefield for the perm with
//     the captain_howler_drawer_until_turn flag set to gs.Turn+1 and
//     matching ctx.source_perm. If found, drawOne for the controller.
//     "this turn" gating: the flag is stamped as gs.Turn+1 and we
//     check it matches gs.Turn+1 — at turn rollover the comparison
//     stops matching and the trigger goes silent without explicit
//     cleanup.
func registerCaptainHowlerSeaScourge(r *Registry) {
	r.OnTrigger("Captain Howler, Sea Scourge", "card_discarded", captainHowlerDiscard)
	r.OnTrigger("Captain Howler, Sea Scourge", "combat_damage_player", captainHowlerDamageDraw)
}

func captainHowlerDiscard(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "captain_howler_discard_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires card_discarded with discarder_seat (resolve.go:1034) and
	// does NOT populate a count (the trigger fires per-card). Legacy "seat"
	// + "count" reads kept as fallbacks for batched/synthetic callers.
	discarderSeat, ok := ctx["discarder_seat"].(int)
	if !ok {
		discarderSeat, _ = ctx["seat"].(int)
	}
	if discarderSeat != perm.Controller {
		return
	}
	count, _ := ctx["count"].(int)
	if count <= 0 {
		count = 1
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pow := p.Power(); pow > bestPow {
			best = p
			bestPow = pow
		}
	}
	if best == nil {
		return
	}
	best.Modifications = append(best.Modifications, gameengine.Modification{
		Power:     2 * count,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["captain_howler_drawer_until_turn"] = gs.Turn + 1
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
		"buff":   2 * count,
	})
}

func captainHowlerDamageDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "captain_howler_combat_damage_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Engine fires combat_damage_player with source_seat (int) + source_card
	// (string) — there is no source_perm. Resolve the pumped creature by
	// scanning the controller's battlefield for the captain_howler_drawer
	// flag, then matching by name against ctx["source_card"]. Legacy
	// ctx["source_perm"] kept as a fast-path fallback for Hat callers.
	var srcPerm *gameengine.Permanent
	if p, ok := ctx["source_perm"].(*gameengine.Permanent); ok && p != nil {
		srcPerm = p
	} else {
		srcSeat, _ := ctx["source_seat"].(int)
		srcName, _ := ctx["source_card"].(string)
		if srcSeat == perm.Controller && srcName != "" {
			if seat := gs.Seats[perm.Controller]; seat != nil {
				for _, bp := range seat.Battlefield {
					if bp != nil && bp.Card != nil && bp.Card.DisplayName() == srcName {
						srcPerm = bp
						break
					}
				}
			}
		}
	}
	if srcPerm == nil || srcPerm.Flags == nil {
		return
	}
	if srcPerm.Controller != perm.Controller {
		return
	}
	if srcPerm.Flags["captain_howler_drawer_until_turn"] != gs.Turn+1 {
		return
	}
	drew := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"source": srcPerm.Card.DisplayName(),
		"drew":   drew != nil,
	})
}
