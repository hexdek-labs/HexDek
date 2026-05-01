package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFrodoAdventurousHobbit wires Frodo, Adventurous Hobbit.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Partner with Sam, Loyal Attendant
//	Vigilance
//	Whenever Frodo attacks, if you gained 3 or more life this turn, the
//	Ring tempts you. Then if Frodo is your Ring-bearer and the Ring has
//	tempted you two or more times this game, draw a card.
//
// Implementation:
//   - Partner with Sam, Loyal Attendant: declarative; partner mechanics
//     live in deck-building / lobby code, not in per-card runtime.
//   - Vigilance: AST keyword pipeline (no handler needed).
//   - "life_gained" trigger: tally life gained on Frodo's controller's
//     seat into a per-turn flag (`frodo_lg_t<turn>`). Mirrors Berta's
//     bookkeeping shape so that prior-turn keys are GC'd.
//   - "creature_attacks" trigger gated on attacker == Frodo:
//       1. Read the per-turn life-gained tally; if < 3, skip.
//       2. Call TheRingTemptsYou(gs, controller). This advances the
//          ring level (CR §701.52) and designates a ring-bearer if none
//          exists.
//       3. After the tempt, check (a) Frodo's Flags["ring_bearer"] == 1
//          and (b) seat.Flags["ring_level"] >= 2. The ring level is
//          monotonically increasing across the whole game, so the
//          post-tempt level is the right "tempted you N times" measure.
//       4. If both conditions hold, draw one card.
//
// The "tempted N times" count and ring level are 1:1 in this engine
// (every tempt increments level by 1, capped at 4). For the ≥2 check
// that's a faithful read of the printed text.
func registerFrodoAdventurousHobbit(r *Registry) {
	r.OnTrigger("Frodo, Adventurous Hobbit", "life_gained", frodoLifeGained)
	r.OnTrigger("Frodo, Adventurous Hobbit", "creature_attacks", frodoAttackTrigger)
}

func frodoLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, ok := ctx["seat"].(int)
	if !ok || gainSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags[frodoLifeKey(gs.Turn)] += amount
}

func frodoAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "frodo_attack_ring_tempt_and_maybe_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	frodoPruneLifeKeys(seat, gs.Turn)

	gained := seat.Flags[frodoLifeKey(gs.Turn)]
	if gained < 3 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          seatIdx,
			"life_gained":   gained,
			"required":      3,
			"tempted":       false,
			"drew_card":     false,
		})
		return
	}

	gameengine.TheRingTemptsYou(gs, seatIdx)
	ringLevel := gameengine.GetRingLevel(gs, seatIdx)

	isRingBearer := perm.Flags != nil && perm.Flags["ring_bearer"] == 1
	drewCard := false
	if isRingBearer && ringLevel >= 2 {
		if drawn := drawOne(gs, seatIdx, perm.Card.DisplayName()); drawn != nil {
			drewCard = true
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"life_gained":  gained,
		"tempted":      true,
		"ring_level":   ringLevel,
		"ring_bearer":  isRingBearer,
		"drew_card":    drewCard,
	})
}

func frodoLifeKey(turn int) string {
	return fmt.Sprintf("frodo_lg_t%d", turn+1)
}

func frodoPruneLifeKeys(seat *gameengine.Seat, currentTurn int) {
	if seat == nil || seat.Flags == nil {
		return
	}
	prefix := "frodo_lg_t"
	cutoff := currentTurn + 1
	for k := range seat.Flags {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		n := 0
		_, err := fmt.Sscanf(k[len(prefix):], "%d", &n)
		if err != nil {
			continue
		}
		if n < cutoff {
			delete(seat.Flags, k)
		}
	}
}
