package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHeartlessHidetsugu wires Heartless Hidetsugu.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Heartless%20Hidetsugu):
//
//	{T}: Each player is dealt damage equal to half that player's life
//	total, rounded up.
//
// {2}{R}{R} Legendary Creature — Ogre Shaman 4/4. The "halve life
// totals" tap ability is a deterministic finisher: in a 4-player game
// where everyone starts at 40 (or 20 in a 1v1), two activations from
// 40 → 20 → 10 → 5 → 3 → 2 → 1 closes a table. Hidetsugu damages its
// own controller too — combine with a damage-prevention shield
// (Sulfuric Vortex anti-, Worship, indestructible) or accept the
// symmetric loss as part of the calculation.
//
// Implementation:
//   - OnActivated index 0: tap, deal ceil(life/2) damage to each
//     non-Lost seat including own controller. Round up via
//     (life + 1) / 2 — matches printed "rounded up."
//   - Damage to each player via DealDamage in seat order
//     (deterministic — log readability). Each call may trip SBA
//     704.5a if a seat drops to 0/-N; CheckEnd at the end picks up
//     simultaneous-elim per CR §104.3.
//   - Negative-life players still take damage (the printed rule
//     doesn't gate on "life > 0"). A seat at -3 takes 1 more damage
//     (ceil(-3/2) = -1, but DealDamage clamps non-positive amounts
//     to 0 internally). Defensive: skip Lost seats and life <= 0
//     seats to avoid double-fire log noise.
func registerHeartlessHidetsugu(r *Registry) {
	r.OnActivated("Heartless Hidetsugu", heartlessHidetsuguActivate)
}

func heartlessHidetsuguActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "heartless_hidetsugu_halve"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Heartless Hidetsugu", "already_tapped", nil)
		return
	}
	if src.SummoningSick && src.IsCreature() {
		emitFail(gs, slug, "Heartless Hidetsugu", "summoning_sick", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	src.Tapped = true

	damaged := []map[string]interface{}{}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Life <= 0 {
			continue
		}
		halfRoundedUp := (s.Life + 1) / 2
		preLife := s.Life
		gameengine.DealDamage(gs, i, halfRoundedUp, "Heartless Hidetsugu")
		damaged = append(damaged, map[string]interface{}{
			"seat":     i,
			"pre_life": preLife,
			"damage":   halfRoundedUp,
		})
	}

	emit(gs, slug, "Heartless Hidetsugu", map[string]interface{}{
		"seat":    seat,
		"damaged": damaged,
	})
	_ = gs.CheckEnd()
}
