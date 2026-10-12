package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheArchimandrite wires The Archimandrite.
//
// Oracle text:
//
//	{2}{U}{R}{W}
//	Legendary Creature — Human Advisor
//	At the beginning of your upkeep, you gain X life, where X is the
//	  number of cards in your hand minus 4.
//	Whenever you gain life, each Advisor, Artificer, and Monk you control
//	  gains vigilance and gets +X/+0 until end of turn, where X is the
//	  amount of life you gained.
//	Tap three untapped Advisors, Artificers, and/or Monks you control:
//	  Draw a card.
//
// Implementation:
//   - "upkeep_controller" trigger: gains hand_size - 4 life.
//   - "life_gained" trigger: gated to the gainer being the controller.
//     Pumps each Advisor/Artificer/Monk the controller controls by +X/+0
//     UEOT and grants vigilance UEOT (perm.Flags["temp_power"] += X,
//     perm.Flags["temp_vigilance"] = 1).
//   - Activated tap-three: emitPartial — engine activated path.
func registerTheArchimandrite(r *Registry) {
	r.OnETB("The Archimandrite", theArchimandriteETB)
	r.OnTrigger("The Archimandrite", "upkeep_controller", theArchimandriteUpkeep)
	r.OnTrigger("The Archimandrite", "life_gained", theArchimandriteLifeGained)
}

func theArchimandriteETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "the_archimandrite_etb", perm.Card.DisplayName(),
		"activated_tap_three_advisors_artificers_monks_draw_card_partial")
}

func theArchimandriteUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_archimandrite_upkeep_lifegain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	x := len(seat.Hand) - 4
	if x <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"x":    x,
		})
		return
	}
	gameengine.GainLife(gs, perm.Controller, x, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"x":    x,
	})
}

func theArchimandriteLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_archimandrite_lifegain_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainerSeat, _ := ctx["seat"].(int)
	if gainerSeat != perm.Controller {
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
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !theArchimandriteIsAdvArtMonk(p.Card) {
			continue
		}
		pumpUntilEOT(gs, p, amount, 0)
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		grantUntilEOT(p, "vigilance")
		p.Flags["temp_vigilance"] = 1
		pumped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"amount": amount,
		"pumped": pumped,
	})
}

func theArchimandriteIsAdvArtMonk(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		switch strings.ToLower(t) {
		case "advisor", "artificer", "monk":
			return true
		}
	}
	return false
}
