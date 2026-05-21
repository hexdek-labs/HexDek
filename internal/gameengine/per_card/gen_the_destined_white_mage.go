package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheDestinedWhiteMage wires The Destined White Mage.
//
// Oracle text (Scryfall, verified):
//
//	Lifelink
//	{W}, {T}: Another target creature you control gains lifelink until
//	end of turn.
//	Whenever you gain life, put a +1/+1 counter on target creature you
//	control. If you have a full party, put three +1/+1 counters on that
//	creature instead.
//
// Implementation (R49 stub port):
//   - Lifelink: AST keyword pipeline.
//   - Life-gain trigger: gate on gain_seat == controller. Pick target —
//     the strongest creature the controller controls (maximises payoff
//     when the target is also the attacker), tiebreak by most-recent
//     timestamp. Add 1 counter, or 3 if CountParty == 4 (full party).
//   - Activated {W}{T}: pick a target other-creature (highest-power),
//     grant Flags["kw:lifelink"]=1 with a next_end_step cleanup. The
//     engine activation pipeline pays mana/tap; defensive guard kept
//     for fixture callers.
func registerTheDestinedWhiteMage(r *Registry) {
	r.OnTrigger("The Destined White Mage", "life_gained", theDestinedWhiteMageTrigger)
	r.OnActivated("The Destined White Mage", theDestinedWhiteMageActivate)
}

func theDestinedWhiteMageTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_destined_white_mage_lifegain_counter"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick strongest creature; tiebreak most recent.
	var target *gameengine.Permanent
	bestPow := -1
	bestTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		pow := gs.PowerOf(p)
		if pow > bestPow || (pow == bestPow && p.Timestamp > bestTS) {
			bestPow = pow
			bestTS = p.Timestamp
			target = p
		}
	}
	if target == nil {
		return
	}
	count := 1
	if gameengine.CountParty(gs, perm.Controller) >= 4 {
		count = 3
	}
	target.AddCounter("+1/+1", count)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"target":  target.Card.DisplayName(),
		"counters": count,
	})
}

func theDestinedWhiteMageActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_destined_white_mage_grant_lifelink"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	seatIdx := src.Controller
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.ManaPool < 1 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  1,
			"available": seat.ManaPool,
		})
		return
	}

	// Pick best other creature — highest power, tiebreak by NOT having
	// lifelink already (avoid wasted grant).
	var target *gameengine.Permanent
	bestPow := -1
	bestTS := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || p.Card == nil || !p.IsCreature() {
			continue
		}
		// Skip if already lifelinking.
		if p.Flags != nil && p.Flags["kw:lifelink"] == 1 {
			continue
		}
		pow := gs.PowerOf(p)
		if pow > bestPow || (pow == bestPow && p.Timestamp > bestTS) {
			bestPow = pow
			bestTS = p.Timestamp
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_other_creature", nil)
		return
	}

	seat.ManaPool -= 1
	gameengine.SyncManaAfterSpend(seat)
	src.Tapped = true

	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["kw:lifelink"] = 1
	captured := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:lifelink")
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"target": target.Card.DisplayName(),
	})
}
