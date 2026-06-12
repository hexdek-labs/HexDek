package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOmnathLocusOfCreation wires Omnath, Locus of Creation.
//
// Oracle text:
//
//	When Omnath enters, draw a card.
//	Landfall — Whenever a land you control enters, you gain 4 life if
//	this is the first time this ability has resolved this turn. If it's
//	the second time, add {R}{G}{W}{U}. If it's the third time, Omnath
//	deals 4 damage to each opponent and each planeswalker you don't
//	control.
//
// Implementation:
//   - ETB: draw one card. (The auto-generated stub also gained 4 life,
//     conflating the ETB with the landfall payoff. Fixed here.)
//   - Landfall: track per-turn resolution count on Omnath via
//     perm.Flags["omnath_landfall_count"]; reset at upkeep / by counter
//     scheme keyed off gs.Active changes. Apply mode based on count:
//     1 → +4 life, 2 → +RGWU mana, 3 → 4 damage to each opponent.
//     4+ → no further triggers (Omnath text is silent on later resolves;
//     CR §603 — the trigger still goes on the stack but resolves with
//     no effect).
func registerOmnathLocusOfCreation(r *Registry) {
	r.OnETB("Omnath, Locus of Creation", omnathLocusOfCreationETB)
	r.OwnsETBTrigger("Omnath, Locus of Creation")
	r.OnTrigger("Omnath, Locus of Creation", "permanent_etb", omnathLandfall)
}

func omnathLocusOfCreationETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "omnath_locus_of_creation_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Tag the turn so we only reset the per-turn counter once per turn.
	perm.Flags["omnath_landfall_turn"] = gs.Turn
	perm.Flags["omnath_landfall_count"] = 0
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

func omnathLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "omnath_locus_of_creation_landfall"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || !entered.IsLand() {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Reset the per-turn counter on a new turn.
	if perm.Flags["omnath_landfall_turn"] != gs.Turn {
		perm.Flags["omnath_landfall_turn"] = gs.Turn
		perm.Flags["omnath_landfall_count"] = 0
	}
	perm.Flags["omnath_landfall_count"]++
	count := perm.Flags["omnath_landfall_count"]
	seat := perm.Controller
	switch count {
	case 1:
		gameengine.GainLife(gs, seat, 4, perm.Card.DisplayName())
	case 2:
		s := gs.Seats[seat]
		if s != nil {
			gameengine.AddMana(gs, s, "R", 1, perm.Card.DisplayName())
			gameengine.AddMana(gs, s, "G", 1, perm.Card.DisplayName())
			gameengine.AddMana(gs, s, "W", 1, perm.Card.DisplayName())
			gameengine.AddMana(gs, s, "U", 1, perm.Card.DisplayName())
		}
	case 3:
		// 4 damage to each opponent and each planeswalker you don't control.
		for _, opp := range gs.Opponents(seat) {
			os := gs.Seats[opp]
			if os == nil || os.Lost {
				continue
			}
			gameengine.LoseLife(gs, opp, 4, perm.Card.DisplayName())
			for _, p := range os.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if p.IsPlaneswalker() {
					p.AddCounter("loyalty", -4)
				}
			}
		}
		_ = gs.CheckEnd()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"count": count,
		"land":  cardName(entered.Card),
	})
}

func cardName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
