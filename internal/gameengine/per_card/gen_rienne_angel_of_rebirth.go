package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRienneAngelOfRebirth wires Rienne, Angel of Rebirth.
//
// Oracle text:
//
//	Flying
//	Other multicolored creatures you control get +1/+0.
//	Whenever another multicolored creature you control dies, return it
//	to its owner's hand at the beginning of the next end step.
//
// Implementation:
//   - +1/+0 anthem static handled by AST keyword/buff pipeline; partial.
//   - "creature_dies" trigger gated on (a) it isn't Rienne, (b) the
//     dead creature was multicolored, (c) the dying-creature controller
//     was Rienne's controller. Schedule a delayed trigger at the next
//     end step that returns the card from graveyard to its owner's hand.
func registerRienneAngelOfRebirth(r *Registry) {
	r.OnETB("Rienne, Angel of Rebirth", rienneAngelOfRebirthETB)
	r.OnTrigger("Rienne, Angel of Rebirth", "creature_dies", rienneAngelOfRebirthDies)
}

func rienneAngelOfRebirthETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rienne_angel_of_rebirth_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Multicolored +1/+0 anthem is wired in
	// custom_rienne_angel_of_rebirth.go (R50 batchH) via Modification
	// refresh on permanent_etb / permanent_ltb.
}

func rienneAngelOfRebirthDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rienne_multicolored_eos_return"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dyingCard == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	if dyingCard == perm.Card {
		return
	}
	if !cardIsMulticolored(dyingCard) {
		return
	}
	owner := dyingCard.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = perm.Controller
	}
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			// Card may have moved on by EOS (exile, second death, etc.).
			// We only return if it's still in the OWNER's graveyard.
			ownerSeat := gs.Seats[owner]
			if ownerSeat == nil {
				return
			}
			for i, c := range ownerSeat.Graveyard {
				if c == dyingCard {
					gameengine.MoveCard(gs, c, owner, "graveyard", "hand", "rienne_eos_return")
					_ = i
					return
				}
			}
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"dying":    dyingCard.DisplayName(),
		"return_at": "next_end_step",
	})
}

// cardIsMulticolored returns true when the card has 2+ colors via Colors
// or pip:* tags.
func cardIsMulticolored(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	colors := map[string]bool{}
	for _, col := range c.Colors {
		switch col {
		case "W", "U", "B", "R", "G":
			colors[col] = true
		}
	}
	for _, t := range c.Types {
		switch t {
		case "pip:W":
			colors["W"] = true
		case "pip:U":
			colors["U"] = true
		case "pip:B":
			colors["B"] = true
		case "pip:R":
			colors["R"] = true
		case "pip:G":
			colors["G"] = true
		}
	}
	return len(colors) >= 2
}
