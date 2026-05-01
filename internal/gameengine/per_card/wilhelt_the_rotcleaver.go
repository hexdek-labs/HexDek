package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWilheltTheRotcleaver wires Wilhelt, the Rotcleaver.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Zombie Warrior (3/3, {2}{U}{B})
//	Whenever another Zombie you control dies, if it didn't have
//	decayed, create a 2/2 black Zombie creature token with decayed.
//	(It can't block. When it attacks, sacrifice it at end of combat.)
//	At the beginning of your end step, you may sacrifice a Zombie. If
//	you do, draw a card.
//
// Implementation:
//   - "creature_dies" (gated to controller_seat == Wilhelt's controller):
//     iff the dying card is a Zombie that didn't have decayed AND wasn't
//     Wilhelt himself ("another"), create a tapped 2/2 black Zombie
//     token with the decayed flag set.
//   - "end_step" (gated to active_seat == controller): may-sacrifice a
//     Zombie → draw a card. Hat opts in iff a positive-score Zombie
//     victim exists (we don't sac high-value Zombies just to cycle).
func registerWilheltTheRotcleaver(r *Registry) {
	r.OnTrigger("Wilhelt, the Rotcleaver", "creature_dies", wilheltZombieDies)
	r.OnTrigger("Wilhelt, the Rotcleaver", "end_step", wilheltEndStep)
}

func wilheltZombieDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wilhelt_zombie_dies_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	dyingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	// "Another" — exclude Wilhelt himself.
	if dyingPerm == perm {
		return
	}
	if !wilheltIsZombie(dyingCard) {
		return
	}
	// Skip non-token zombies that had decayed (per oracle: "if it didn't
	// have decayed"). Tokens with decayed are excluded for the same
	// reason — this prevents the trigger from chaining off a token's
	// own end-of-combat sacrifice into infinite token creation.
	if dyingPerm != nil && dyingPerm.Flags != nil && dyingPerm.Flags["decayed"] > 0 {
		return
	}

	seat := perm.Controller
	tok := gameengine.CreateCreatureToken(gs, seat, "Zombie Token",
		[]string{"creature", "zombie", "pip:B"}, 2, 2)
	if tok == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_create_failed", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	tok.Tapped = true
	gameengine.ApplyDecayed(tok)

	dyingName := ""
	if dyingCard != nil {
		dyingName = dyingCard.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"dying_zombie": dyingName,
		"token":        "2/2 Zombie (decayed, tapped)",
	})
}

func wilheltEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wilhelt_end_step_sac_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	victim := wilheltPickZombieToSac(gs, perm)
	if victim == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":       perm.Controller,
			"sacrificed": "",
			"drew":       0,
		})
		return
	}
	victimName := victim.Card.DisplayName()
	gameengine.SacrificePermanent(gs, victim, "wilhelt_end_step")
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": victimName,
		"drew":       1,
		"card":       drawnName,
	})
}

// wilheltPickZombieToSac chooses the best Zombie to feed to Wilhelt's
// end-step. Priority:
//
//  1. Decayed token zombies that already attacked this turn (they'd die
//     to the decayed end-of-combat trigger anyway — free fodder).
//  2. Any decayed token zombie (will die at end of next combat).
//  3. Lowest-CMC non-Wilhelt non-token Zombie (cheap fodder + recurs
//     into Wilhelt's death-trigger token loop).
//
// Returns nil if no Zombie other than Wilhelt himself exists.
func wilheltPickZombieToSac(gs *gameengine.GameState, src *gameengine.Permanent) *gameengine.Permanent {
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	var bestDecayed *gameengine.Permanent
	var bestNonToken *gameengine.Permanent
	bestNonTokenCMC := 999
	for _, p := range seat.Battlefield {
		if p == nil || p == src || !p.IsCreature() {
			continue
		}
		if !wilheltIsZombie(p.Card) {
			continue
		}
		if p.Flags != nil && p.Flags["decayed"] > 0 {
			bestDecayed = p
			continue
		}
		if cardHasType(p.Card, "token") {
			// Tokens without decayed: still good fodder, slightly worse
			// than decayed (decayed tokens are doomed anyway).
			if bestNonToken == nil {
				bestNonToken = p
			}
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestNonTokenCMC {
			bestNonTokenCMC = cmc
			bestNonToken = p
		}
	}
	if bestDecayed != nil {
		return bestDecayed
	}
	return bestNonToken
}

func wilheltIsZombie(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "zombie") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(c.TypeLine), "zombie")
}
