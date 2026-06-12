package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSarcomancy wires Sarcomancy (Muninn parser-gap #70, ~9.5K hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{B}
//	Enchantment
//	When this enchantment enters, create a 2/2 black Zombie creature
//	token.
//	At the beginning of your upkeep, if there are no Zombies on the
//	battlefield, this enchantment deals 1 damage to you.
//
// Implementation:
//   - ETB: mint a 2/2 black Zombie token.
//   - upkeep_controller: if no battlefield permanent has the "zombie"
//     subtype, deal 1 damage to the controller.
func registerSarcomancy(r *Registry) {
	r.OnETB("Sarcomancy", sarcomancyETB)
	r.OwnsETBTrigger("Sarcomancy")
	r.OnTrigger("Sarcomancy", "upkeep_controller", sarcomancyUpkeep)
}

func sarcomancyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sarcomancy_etb_zombie_token"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	token := &gameengine.Card{
		Name:          "Zombie Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "zombie", "pip:B"},
		Colors:        []string{"B"},
		BasePower:     2,
		BaseToughness: 2,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "2/2 black Zombie",
	})
}

func sarcomancyUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sarcomancy_upkeep_zombie_check"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// "No Zombies on the battlefield" — scan every seat for any creature
	// permanent that has the Zombie subtype.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if cardHasType(p.Card, "zombie") {
				emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
					"seat":      perm.Controller,
					"triggered": false,
					"reason":    "zombie_present",
				})
				return
			}
		}
	}
	gameengine.DealDamage(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"damage":    1,
	})
}
