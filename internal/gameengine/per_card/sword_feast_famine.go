package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSwordOfFeastAndFamine wires up Sword of Feast and Famine.
//
// Oracle text:
//
//	Equipped creature gets +2/+2 and has protection from black and
//	from green.
//	Whenever equipped creature deals combat damage to a player,
//	that player discards a card and you untap all lands you control.
//
// For the engine, the critical clause is the combat-damage trigger:
// it untaps all lands the equipped creature's controller controls.
// This enables infinite combats with Aggravated Assault or similar
// "extra combat if you have mana" cards -- attack, deal damage, untap
// lands, use lands to pay for extra combat, repeat.
//
// Implementation:
//   - OnTrigger "combat_damage_dealt": untap all lands controlled by
//     the equipment holder, and force one opponent to discard.
func registerSwordOfFeastAndFamine(r *Registry) {
	r.OnTrigger("Sword of Feast and Famine", "combat_damage_player", swordFeastFamineTrigger)
}

func swordFeastFamineTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sword_feast_famine_untap"
	if gs == nil || perm == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if perm.AttachedTo == nil || perm.AttachedTo.Card == nil ||
		perm.AttachedTo.Card.DisplayName() != sourceName {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Untap all lands controlled by the seat.
	untapped := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsLand() || !p.Tapped {
			continue
		}
		p.Tapped = false
		untapped++
	}

	// Force one opponent to discard a card.
	for _, oppIdx := range gs.Opponents(seat) {
		opp := gs.Seats[oppIdx]
		if opp == nil || len(opp.Hand) == 0 {
			continue
		}
		gameengine.DiscardN(gs, oppIdx, 1, "")
		break
	}

	emit(gs, slug, "Sword of Feast and Famine", map[string]interface{}{
		"seat":          seat,
		"lands_untapped": untapped,
	})
}
