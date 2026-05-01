package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAshlingRekindled wires Ashling, Rekindled // Ashling, Rimebound
// (modal DFC).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
// Front — Ashling, Rekindled (Legendary Creature — Elemental Sorcerer):
//
//	{1}{R}
//	Whenever this creature enters or transforms into Ashling,
//	Rekindled, you may discard a card. If you do, draw a card.
//	At the beginning of your first main phase, you may pay {U}. If
//	you do, transform Ashling.
//
// Back — Ashling, Rimebound (Legendary Creature — Elemental Wizard):
//
//	Whenever this creature transforms into Ashling, Rimebound and at
//	the beginning of your first main phase, add two mana of any one
//	color. Spend this mana only to cast spells with mana value 4 or
//	greater.
//	At the beginning of your first main phase, you may pay {R}. If
//	you do, transform Ashling.
//
// Implementation:
//   - OnETB: when entering as the front face, fire the loot trigger
//     (discard then draw; "may" — gated on hand having a worthwhile
//     discard target, otherwise skip).
//   - "upkeep_controller" stands in for "beginning of your first main
//     phase" — the engine has no first_main_phase trigger today, and
//     upkeep is the closest controller-active hook that fires once per
//     turn. The trigger handles BOTH the pay-to-transform clause AND
//     the back-face "add two mana" rider:
//       * If on front face: optionally pay {U}; on success, transform
//         to back. The transform-into-back static trigger then adds two
//         mana of any one color (we just call AddMana with restriction
//         flag on the typed pool — the engine has no MV-4+ filter today,
//         so emitPartial flags the restriction gap).
//       * If on back face: optionally pay {R}; on success, transform
//         back to front. The transform-into-front trigger fires the
//         loot. ALSO unconditionally add two mana on first-main when on
//         back face (per the "and at the beginning..." conjunction).
//   - Pay heuristic: always opt in when the controller has the mana,
//     since alternating sides nets a loot or a free 2-mana ramp. Decks
//     running Ashling are built around the alternation cadence.
//   - Restriction "spend only on MV>=4 spells" is unimplemented in the
//     mana pool; we tag the AddRestrictedMana with an "mv4_or_greater"
//     marker and emitPartial.
//
// DFC dispatch: register all three name forms (full DFC, front, back)
// per esika.go's pattern — perm.Card.Name swaps after TransformPermanent
// and the registry's " // " split fallback only catches pre-transform
// names.
func registerAshlingRekindled(r *Registry) {
	r.OnETB("Ashling, Rekindled // Ashling, Rimebound", ashlingRekindledETB)
	r.OnETB("Ashling, Rekindled", ashlingRekindledETB)
	r.OnTrigger("Ashling, Rekindled // Ashling, Rimebound", "upkeep_controller", ashlingRekindledUpkeep)
	r.OnTrigger("Ashling, Rekindled", "upkeep_controller", ashlingRekindledUpkeep)
	r.OnTrigger("Ashling, Rimebound", "upkeep_controller", ashlingRekindledUpkeep)
}

func ashlingRekindledETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Transformed {
		// Cast as the back face — Rimebound's transforms-into trigger
		// fires the mana add. Since we hit ETB instead of a transform
		// event, mirror the rider here.
		ashlingRekindledAddBackFaceMana(gs, perm, "etb_as_back_face")
		return
	}
	ashlingRekindledLoot(gs, perm, "etb")
}

func ashlingRekindledUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	if !perm.Transformed {
		// Front face: pay {U} to flip to back. Heuristic: pay if we have
		// a U pip available in the pool.
		if ashlingRekindledTryPayColored(gs, seat, "U") {
			if gameengine.TransformPermanent(gs, perm, "ashling_rekindled_pay_U") {
				ashlingRekindledAddBackFaceMana(gs, perm, "transform_to_rimebound")
				emit(gs, "ashling_rekindled_transform", perm.Card.DisplayName(), map[string]interface{}{
					"seat":  perm.Controller,
					"to":    "Ashling, Rimebound",
					"paid":  "U",
				})
			}
		}
		return
	}

	// Back face. The "add two mana of any one color" trigger fires both
	// on transform-into AND at the beginning of first main phase. The
	// transform-into branch is handled in the front-face block above; this
	// branch covers the steady-state first-main-phase add.
	ashlingRekindledAddBackFaceMana(gs, perm, "first_main_back_face")

	// Pay {R} to flip back to front.
	if ashlingRekindledTryPayColored(gs, seat, "R") {
		if gameengine.TransformPermanent(gs, perm, "ashling_rimebound_pay_R") {
			ashlingRekindledLoot(gs, perm, "transform_to_rekindled")
			emit(gs, "ashling_rekindled_transform", perm.Card.DisplayName(), map[string]interface{}{
				"seat": perm.Controller,
				"to":   "Ashling, Rekindled",
				"paid": "R",
			})
		}
	}
}

// ashlingRekindledLoot — front face "may discard a card. If you do,
// draw a card." Heuristic: only opt in if the controller has at least
// one card in hand to discard (otherwise the "may" is wasted).
func ashlingRekindledLoot(gs *gameengine.GameState, perm *gameengine.Permanent, cause string) {
	const slug = "ashling_rekindled_loot"
	seat := gs.Seats[perm.Controller]
	if seat == nil || len(seat.Hand) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_hand_to_discard", map[string]interface{}{
			"seat":  perm.Controller,
			"cause": cause,
		})
		return
	}
	// Discard the last card in hand (simple heuristic — matches Varina).
	discarded := seat.Hand[len(seat.Hand)-1]
	gameengine.DiscardCard(gs, discarded, perm.Controller)
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	discardedName := ""
	if discarded != nil {
		discardedName = discarded.DisplayName()
	}
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"cause":     cause,
		"discarded": discardedName,
		"drawn":     drawnName,
	})
}

// ashlingRekindledAddBackFaceMana — back-face Rimebound "add two mana
// of any one color. Spend this mana only to cast spells with mana value
// 4 or greater." We use AddRestrictedMana with restriction marker
// "mv4_or_greater"; the engine doesn't enforce that filter today, so we
// emitPartial to flag the gap.
func ashlingRekindledAddBackFaceMana(gs *gameengine.GameState, perm *gameengine.Permanent, cause string) {
	const slug = "ashling_rimebound_add_two_mana"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	gameengine.AddRestrictedMana(gs, seat, 2, "any", "mv4_or_greater",
		perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"cause": cause,
		"added": 2,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"mv_4_or_greater_spend_restriction_not_enforced_in_mana_pool")
}

// ashlingRekindledTryPayColored attempts to spend one mana of the named
// color from the seat's typed pool, falling back to Any. Returns true
// on success. Also drains the legacy ManaPool int via SyncManaAfterSpend
// so test fixtures that prime ManaPool directly stay in sync.
func ashlingRekindledTryPayColored(gs *gameengine.GameState, seat *gameengine.Seat, color string) bool {
	if seat == nil {
		return false
	}
	if seat.Mana != nil {
		switch color {
		case "U":
			if seat.Mana.U > 0 {
				seat.Mana.U--
				gameengine.SyncManaAfterSpend(seat)
				return true
			}
		case "R":
			if seat.Mana.R > 0 {
				seat.Mana.R--
				gameengine.SyncManaAfterSpend(seat)
				return true
			}
		}
		if seat.Mana.Any > 0 {
			seat.Mana.Any--
			gameengine.SyncManaAfterSpend(seat)
			return true
		}
	}
	if seat.ManaPool >= 1 {
		seat.ManaPool--
		gameengine.SyncManaAfterSpend(seat)
		return true
	}
	return false
}
