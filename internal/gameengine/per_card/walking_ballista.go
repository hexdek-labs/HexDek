package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWalkingBallista wires up Walking Ballista.
//
// Oracle text:
//
//	Walking Ballista enters the battlefield with X +1/+1 counters on
//	it.
//	{4}: Put a +1/+1 counter on Walking Ballista.
//	Remove a +1/+1 counter from Walking Ballista: It deals 1 damage
//	to any target.
//
// Kinnan wincon. Infinite mana (Kinnan + Basalt Monolith or Food Chain
// + Misthollow) plus Ballista on the battlefield wins the game: pump
// Ballista to absurd size via {4}, then remove counters for damage.
//
// The ETB-with-X-counters clause needs an X value. Oracle resolution
// reads X from gs.Flags["_ballista_x_"+seat] (test-friendly) or from a
// "etb_x:N" token in Card.Types. Defaults to 0 (Ballista enters as a
// 0/0 — engine-level SBA §704.5f then wants to kill it, but since the
// engine's SBA may not hit this before next call, we clamp).
//
// Handlers:
//   - OnETB: put X counters on Ballista.
//   - OnActivated(0, ...): pay {4} → add +1/+1 counter (ctx unused).
//   - OnActivated(1, ...): remove a counter → deal 1 damage to
//     ctx["target_seat"] or ctx["target_perm"].
func registerWalkingBallista(r *Registry) {
	r.OnETB("Walking Ballista", walkingBallistaETB)
	r.OnActivated("Walking Ballista", walkingBallistaActivate)
}

func walkingBallistaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "walking_ballista_etb_with_x"
	if gs == nil || perm == nil {
		return
	}
	// Read X from flags or Types.
	x := 0
	if v, ok := gs.Flags["_ballista_x_"+intToStr(perm.Controller)]; ok {
		x = v
		// Consume the flag so subsequent Ballistas don't inherit.
		delete(gs.Flags, "_ballista_x_"+intToStr(perm.Controller))
	} else if perm.Card != nil {
		for _, t := range perm.Card.Types {
			if len(t) > 6 && t[:6] == "etb_x:" {
				n := 0
				for _, ch := range t[6:] {
					if ch < '0' || ch > '9' {
						break
					}
					n = n*10 + int(ch-'0')
				}
				x = n
				break
			}
		}
	}
	if x < 0 {
		x = 0
	}
	perm.AddCounter("+1/+1", x)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"x_value": x,
	})
}

func walkingBallistaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		// {4}: Put a +1/+1 counter on Ballista.
		const slug = "walking_ballista_pump"
		src.AddCounter("+1/+1", 1)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"counters": src.Counters["+1/+1"],
		})
	case 1:
		// Remove a +1/+1 counter: deal 1 damage to any target.
		const slug = "walking_ballista_shoot"
		if src.Counters == nil || src.Counters["+1/+1"] <= 0 {
			emitFail(gs, slug, src.Card.DisplayName(), "no_counters_to_remove", nil)
			return
		}
		src.AddCounter("+1/+1", -1)
		// Damage target.
		if targetSeat, ok := ctx["target_seat"].(int); ok && targetSeat >= 0 && targetSeat < len(gs.Seats) {
			gs.Seats[targetSeat].Life -= 1
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   src.Controller,
				Target: targetSeat,
				Source: src.Card.DisplayName(),
				Amount: 1,
			})
			emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
				"target_seat": targetSeat,
				"damage":      1,
			})
			_ = gs.CheckEnd()
			return
		}
		if targetPerm, ok := ctx["target_perm"].(*gameengine.Permanent); ok && targetPerm != nil {
			targetPerm.MarkedDamage += 1
			gs.LogEvent(gameengine.Event{
				Kind:   "damage",
				Seat:   src.Controller,
				Target: targetPerm.Controller,
				Source: src.Card.DisplayName(),
				Amount: 1,
				Details: map[string]interface{}{
					"target_card": targetPerm.Card.DisplayName(),
				},
			})
			emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
				"target_card": targetPerm.Card.DisplayName(),
				"damage":      1,
			})
			return
		}
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
	}
}
