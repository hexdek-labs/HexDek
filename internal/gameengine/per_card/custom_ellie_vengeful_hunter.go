package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEllieVengefulHunterCustom implements Ellie's pay-life-and-sac
// activation. The auto-generated stub is a no-op.
//
// Oracle text:
//
//	Pay 2 life, Sacrifice another creature: Ellie deals 2 damage to
//	target player and gains indestructible until end of turn.
//	Partner—Survivors (You can have two commanders if both have this
//	ability.)
//
// Implementation notes:
//   - Cost: 2 life + sac another creature. Both portions enforced
//     defensively in-handler since the engine doesn't gate non-mana
//     activation costs before dispatch.
//   - Sac victim: smallest non-commander creature (preserve value).
//   - Damage target: opponent with lowest life (kill-shot priority).
//   - Indestructible UEOT: set Flags["kw:indestructible"]=1, schedule
//     a next_end_step delayed trigger to clear the flag.
//   - Partner clause is engine-side commander-zone wiring.
func registerEllieVengefulHunterCustom(r *Registry) {
	r.OnActivated("Ellie, Vengeful Hunter", ellieVengefulActivate)
}

func ellieVengefulActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ellie_vengeful_pay_sac_damage"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	if seat.Life <= 2 {
		emitFail(gs, slug, src.Card.DisplayName(), "life_too_low_to_pay", map[string]interface{}{
			"seat": seatIdx,
			"life": seat.Life,
		})
		return
	}

	// Pick smallest non-commander other creature to sacrifice.
	var victim *gameengine.Permanent
	bestPT := 1 << 30
	bestTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src || !p.IsCreature() {
			continue
		}
		if yawgmothIsCommander(gs, p) {
			continue
		}
		pt := gs.PowerOf(p) + gs.ToughnessOf(p)
		if pt < bestPT || (pt == bestPT && p.Timestamp > bestTS) {
			bestPT = pt
			bestTS = p.Timestamp
			victim = p
		}
	}
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_to_sacrifice", nil)
		return
	}

	// Pick lowest-life opponent.
	target := -1
	lowest := 1 << 30
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame {
			continue
		}
		if s.Life < lowest {
			lowest = s.Life
			target = i
		}
	}
	if target < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_legal_player_target", nil)
		return
	}

	// Pay costs.
	gameengine.LoseLife(gs, seatIdx, 2, src.Card.DisplayName())
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "ellie_sac_cost")

	// Effects.
	gameengine.DealDamage(gs, target, 2, src.Card.DisplayName())
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, src, "indestructible")
	captured := src
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			gameengine.ClearKeywordFlags(gs, captured, "indestructible")
		},
	})

	emitPartial(gs, slug, src.Card.DisplayName(),
		"Partner—Survivors clause: command-zone selection not modeled at per-card layer")

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":        seatIdx,
		"sacrificed":  victimName,
		"target_seat": target,
		"damage":      2,
	})
	_ = gs.CheckEnd()
}
