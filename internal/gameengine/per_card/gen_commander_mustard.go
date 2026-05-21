package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCommanderMustard wires Commander Mustard.
//
// Oracle text:
//
//	Vigilance
//	Other Soldiers you control have vigilance, trample, and haste.
//	{2}{R}{W}: Until end of turn, Soldiers you control gain "Whenever this
//	creature attacks, it deals 1 damage to defending player."
//
// Implementation:
//   - Activated ability gates on `seat.ManaPool >= 4` ({2}{R}{W} = 4
//     generic for the engine's colorless pool). Sets a per-seat
//     `mustard_soldier_attack_ping` flag for the combat layer to read
//     and queues a delayed trigger to clear the flag at end of turn.
//   - The "this creature attacks → 1 damage" rider itself relies on the
//     combat layer reading the flag during attacker resolution; the
//     handler tracks both the cost gate and the duration with a partial
//     breadcrumb so audits can find the wiring boundary.
func registerCommanderMustard(r *Registry) {
	r.OnActivated("Commander Mustard", commanderMustardActivate)
}

func commanderMustardActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "commander_mustard_soldier_attack_ping"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if !payManaFromPool(seat, 4) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  4,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["mustard_soldier_attack_ping"]++
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			s := gs.Seats[seatIdx]
			if s == nil || s.Flags == nil {
				return
			}
			if s.Flags["mustard_soldier_attack_ping"] > 0 {
				s.Flags["mustard_soldier_attack_ping"]--
			}
			if s.Flags["mustard_soldier_attack_ping"] <= 0 {
				delete(s.Flags, "mustard_soldier_attack_ping")
			}
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  seatIdx,
		"stack": seat.Flags["mustard_soldier_attack_ping"],
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"soldier-attacks-damage rider relies on combat layer reading mustard_soldier_attack_ping flag")
}
