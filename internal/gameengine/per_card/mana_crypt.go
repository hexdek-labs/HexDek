package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerManaCrypt wires up Mana Crypt.
//
// Oracle text:
//
//	At the beginning of your upkeep, flip a coin. If you lose the
//	flip, Mana Crypt deals 3 damage to you.
//	{T}: Add {C}{C}.
//
// Fast mana staple. The coin flip is a real drawback — over a long
// cEDH game Crypt accumulates 3-damage ticks, so decks factor it into
// their life cushion. {C}{C} is powerful mana filtering for the cost
// of (nothing at-cast, 3 damage per turn upkeep).
//
// Batch #2 scope:
//   - OnActivated(0, ...): {T}: Add {C}{C}. Adds 2 to controller's
//     mana pool (generic, since the engine's mana-pool is untyped MVP).
//     Caller is responsible for marking the tap.
//   - OnTrigger("upkeep_controller"): fire the flip-a-coin trigger.
//     Uses gs.Rng for determinism. Loss (50%) → 3 damage to controller.
//
// The upkeep trigger is dispatched from the upkeep-phase step in
// phases.go — the engine emits "upkeep_controller" per-seat at the
// beginning of each seat's upkeep step. Without that wiring landing
// (Day/Night agent owns phases.go tweaks), we still register the
// handler so dispatching is ready on the first call.
func registerManaCrypt(r *Registry) {
	r.OnActivated("Mana Crypt", manaCryptActivate)
	r.OnTrigger("Mana Crypt", "upkeep_controller", manaCryptUpkeep)
}

func manaCryptActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mana_crypt_tap_for_cc"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true
	seat := src.Controller
	s := gs.Seats[seat]
	s.ManaPool += 2
	gameengine.SyncManaAfterAdd(s, 2)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "mana_crypt_tap",
			"pool":   "CC",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"added":    2,
		"new_pool": s.ManaPool,
	})
}

func manaCryptUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mana_crypt_upkeep_flip"
	if gs == nil || perm == nil {
		return
	}
	// Only fires during the CONTROLLER's own upkeep.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	rng := gs.Rng
	// Heads: no damage. Tails: 3 damage. Float-based for readable bias:
	// rng.Intn(2) == 1 → tails (we lose). Deterministic under the game's
	// seeded RNG.
	if rng == nil {
		// Defensive: without RNG, pick arbitrarily. Choose heads (no
		// damage) — tests without RNG shouldn't crash the engine.
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"result":     "heads",
			"no_rng_set": true,
		})
		return
	}
	tails := rng.Intn(2) == 1
	if !tails {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"result": "heads",
		})
		return
	}
	// Tails: 3 damage to controller.
	seat := perm.Controller
	gs.Seats[seat].Life -= 3
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   seat,
		Target: seat,
		Source: perm.Card.DisplayName(),
		Amount: 3,
		Details: map[string]interface{}{
			"reason": "mana_crypt_flip_loss",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"result": "tails",
		"damage": 3,
	})
	_ = gs.CheckEnd()
}
