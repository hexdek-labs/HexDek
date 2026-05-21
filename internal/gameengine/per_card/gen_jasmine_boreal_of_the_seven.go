package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJasmineBorealOfTheSeven wires Jasmine Boreal of the Seven.
//
// Oracle text (Scryfall, verified 2026-05-09):
//
//	{T}: Add {G}{W}. Spend this mana only to cast creature spells with
//	no abilities.
//	Creatures you control with no abilities can't be blocked by creatures
//	with abilities.
//
// Both abilities are engine-layer:
//   - The {T} mana ability with the "spend only on no-ability creature
//     spells" restriction is a mana-system tag (cf. Galazeth Prismari,
//     Cormela, Rivaz of the Claw — all left as engine-side partials).
//   - The "no-ability creatures can't be blocked by ability creatures"
//     static is a combat-blocker layer (cf. menace/skulk pipeline).
//
// We register the activation hook and ETB hook with breadcrumb partials
// so Heimdall/Muninn surfaces the gap. No effect is dispatched here —
// any logic would double-count what the engine already handles or fakes.
//
// TODO: engine support needed for ability-tagged restricted mana on the
// mana add path; engine support needed for static block-restriction by
// "has abilities" predicate on attacker/blocker.
func registerJasmineBorealOfTheSeven(r *Registry) {
	r.OnETB("Jasmine Boreal of the Seven", jasmineBorealETB)
	r.OnActivated("Jasmine Boreal of the Seven", jasmineBorealActivate)
}

func jasmineBorealETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jasmine_boreal_static_no_ability_unblockable"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_block_restriction_no_ability_vs_ability_creatures_engine_side")
}

// jasmineBorealActivate handles {T}: Add {G}{W}. The "spend only on
// creature spells with no abilities" restriction is typed-mana
// enforcement the untyped ManaPool can't track — we stamp a seat flag
// so future cast-legality scanners can recognize the restricted mana
// and emitPartial flags the gap (R51 batch H port).
func jasmineBorealActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jasmine_boreal_tap_for_gw_restricted"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	if src.SummoningSick {
		emitFail(gs, slug, src.Card.DisplayName(), "summoning_sick", nil)
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	src.Tapped = true
	gameengine.AddManaFromPermanent(gs, seat, src, "G", 1)
	gameengine.AddManaFromPermanent(gs, seat, src, "W", 1)
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["jasmine_no_ability_creature_mana"] += 2
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   src.Controller,
		"added":  2,
		"colors": []string{"G", "W"},
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"spend_only_on_no_ability_creature_spells_restriction_tracked_via_seat_flag_untyped_pool")
}
