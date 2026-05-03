package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMuldrothaTheGravetide wires Muldrotha, the Gravetide.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	During each of your turns, you may play a land and cast a permanent
//	spell of each other type from your graveyard. (If a spell has
//	multiple permanent types, choose one as you cast it.)
//
// Implementation:
//   - OnETB: sets `seat.Flags["muldrotha_graveyard_cast"] = 1` so the
//     engine / hat / Freya know this seat has graveyard-cast permission
//     for permanents. Also initializes the per-type tracking flags on
//     the permanent itself so the upkeep reset has something to clear.
//   - OnTrigger("upkeep_controller"): at the start of Muldrotha's
//     controller's turn, resets the per-type tracking flags on perm.Flags.
//     These flags record whether the player has already used their one
//     graveyard cast for each permanent type this turn:
//       perm.Flags["muldrotha_cast_creature"]     = 0
//       perm.Flags["muldrotha_cast_artifact"]      = 0
//       perm.Flags["muldrotha_cast_enchantment"]   = 0
//       perm.Flags["muldrotha_cast_planeswalker"]  = 0
//       perm.Flags["muldrotha_cast_land"]           = 0
//   - The actual graveyard casting enforcement (checking these flags in
//     CastSpell and blocking a second creature-from-graveyard cast, etc.)
//     requires engine-level hooks that are not yet wired. emitPartial
//     flags this coverage gap so Heimdall/Muninn can track it.
//
// Note: the legacy registerMuldrotha in commanders_batch.go is replaced by
// this handler. That stub only set the seat flag; this handler adds the
// upkeep reset for per-type tracking and the emitPartial for the missing
// engine-level CastSpell integration.
func registerMuldrothaTheGravetide(r *Registry) {
	r.OnETB("Muldrotha, the Gravetide", muldrothaTheGravetideETB)
	r.OnTrigger("Muldrotha, the Gravetide", "upkeep_controller", muldrothaUpkeepReset)
}

// muldrothaTheGravetideETB sets the seat-level graveyard-cast permission
// flag and initializes per-type tracking flags on the permanent.
func muldrothaTheGravetideETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "muldrotha_etb_graveyard_cast"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Set seat-level flag so the engine / hat knows this seat can cast
	// permanents from its graveyard.
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["muldrotha_graveyard_cast"] = 1

	// Initialize per-type tracking on the permanent. These start at 0
	// (unused) and get set to 1 when the player casts from graveyard
	// using that type slot. The upkeep trigger resets them each turn.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["muldrotha_cast_creature"] = 0
	perm.Flags["muldrotha_cast_artifact"] = 0
	perm.Flags["muldrotha_cast_enchantment"] = 0
	perm.Flags["muldrotha_cast_planeswalker"] = 0
	perm.Flags["muldrotha_cast_land"] = 0

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "graveyard_cast_permission_set_with_per_type_tracking",
	})

	// The actual enforcement of graveyard casting (allowing CastSpell from
	// graveyard, checking per-type limits, and consuming the flag) requires
	// engine-level hooks in CastSpell / PlayLand that are not yet wired.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"engine_level_castspell_graveyard_integration_not_wired")
}

// muldrothaUpkeepReset fires at the beginning of Muldrotha's controller's
// upkeep and resets all per-type tracking flags so the player can cast one
// permanent of each type from their graveyard again this turn.
func muldrothaUpkeepReset(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "muldrotha_upkeep_reset"
	if gs == nil || perm == nil {
		return
	}

	// Only fire on Muldrotha's controller's own upkeep.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	// Reset per-type tracking flags.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["muldrotha_cast_creature"] = 0
	perm.Flags["muldrotha_cast_artifact"] = 0
	perm.Flags["muldrotha_cast_enchantment"] = 0
	perm.Flags["muldrotha_cast_planeswalker"] = 0
	perm.Flags["muldrotha_cast_land"] = 0

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "per_type_graveyard_cast_tracking_reset",
	})
}
