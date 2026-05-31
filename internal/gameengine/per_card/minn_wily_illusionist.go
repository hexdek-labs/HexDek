package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMinnWilyIllusionist wires Minn, Wily Illusionist.
//
// Oracle text:
//
//	Whenever you draw your second card each turn, create a 1/1 blue
//	Illusion creature token with "This token gets +1/+0 for each other
//	Illusion you control."
//	Whenever an Illusion you control dies, you may put a permanent card
//	with mana value less than or equal to that creature's power from
//	your hand onto the battlefield.
//
// Implementation:
//   - opponent_drew_card / drew event tracking is handled at engine level;
//     we count draws via Flags["minn_drew_this_turn"] in the dies hook
//     using gs.TurnDraws if available, but the second-card trigger is
//     marked emitPartial because the runtime hook isn't wired here.
//   - On Illusion-dies, search hand for a permanent card with CMC <=
//     dying creature's power and put it onto the battlefield.
func registerMinnWilyIllusionist(r *Registry) {
	r.OnTrigger("Minn, Wily Illusionist", "creature_dies", minnIllusionDies)
	r.OnETB("Minn, Wily Illusionist", minnETB)
}

func minnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "minn_etb"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"second_card_drawn_token_trigger_not_wired")
}

func minnIllusionDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "minn_illusion_dies_cheat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadController, _ := ctx["controller_seat"].(int)
	if deadController != perm.Controller {
		return
	}
	deadCard, _ := ctx["card"].(*gameengine.Card)
	if deadCard == nil {
		return
	}
	if !minnIsIllusion(deadCard) {
		return
	}
	power := deadCard.BasePower
	deadPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if deadPerm != nil {
		power = deadPerm.Power()
	}
	if power < 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") && !cardHasType(c, "artifact") &&
			!cardHasType(c, "enchantment") && !cardHasType(c, "planeswalker") &&
			!cardHasType(c, "land") && !cardHasType(c, "battle") {
			continue
		}
		if cardCMC(c) > power {
			continue
		}
		// enterBattlefieldWithETB → createPermanent sweeps `c` from every
		// private zone (hand included); manual splice was redundant
		// (Wave 2 multi-step migration — same shape as Cluster 2 in
		// PR #924).
		_ = i
		enterBattlefieldWithETB(gs, perm.Controller, c, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"power":  power,
			"played": c.DisplayName(),
		})
		return
	}
	emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_card_in_hand", map[string]interface{}{
		"seat":  perm.Controller,
		"power": power,
	})
}

func minnIsIllusion(card *gameengine.Card) bool {
	if card == nil {
		return false
	}
	for _, t := range card.Types {
		if strings.EqualFold(t, "illusion") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(card.TypeLine), "illusion")
}
