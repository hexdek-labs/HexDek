package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheAncientOne wires The Ancient One.
//
// Oracle text:
//
//	{U}{B}
//	Legendary Creature — Spirit God
//	Descend 8 — The Ancient One can't attack or block unless there are
//	  eight or more permanent cards in your graveyard.
//	{2}{U}{B}: Draw a card, then discard a card. When you discard a card
//	  this way, target player mills cards equal to its mana value.
//
// Implementation (R49 stub port — batch A):
//   - Descend 8 attack/block restriction: ETB + upkeep + combat_begin +
//     end_step refresh perm.Flags["cant_attack"] / ["cant_block"] based
//     on a live count of permanent cards in the controller's graveyard
//     (creature/artifact/enchantment/land/planeswalker/battle). When
//     the count is ≥ 8 both flags are cleared; otherwise both are set.
//     This piggybacks on the engine's standard combat-restriction flag
//     pipeline rather than introducing a Descend-specific code path.
//   - Activated ability: {2}{U}{B} loot then mill target opponent for
//     the discarded card's MV.
func registerTheAncientOne(r *Registry) {
	r.OnETB("The Ancient One", theAncientOneETB)
	r.OnTrigger("The Ancient One", "upkeep_controller", theAncientOneRefreshRestriction)
	r.OnTrigger("The Ancient One", "combat_begin", theAncientOneRefreshRestriction)
	r.OnTrigger("The Ancient One", "end_step", theAncientOneRefreshRestriction)
	r.OnActivated("The Ancient One", theAncientOneActivated)
}

func theAncientOneETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	theAncientOneApplyDescendGate(gs, perm)
	emit(gs, "the_ancient_one_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"yard_perm_cards": theAncientOnePermanentCardsInYard(gs, perm.Controller),
	})
}

func theAncientOneRefreshRestriction(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	theAncientOneApplyDescendGate(gs, perm)
}

func theAncientOneApplyDescendGate(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	count := theAncientOnePermanentCardsInYard(gs, perm.Controller)
	if count >= 8 {
		delete(perm.Flags, "cant_attack")
		delete(perm.Flags, "cant_block")
		perm.Flags["descend_8_attack_block_restriction"] = 0
		return
	}
	perm.Flags["cant_attack"] = 1
	perm.Flags["cant_block"] = 1
	perm.Flags["descend_8_attack_block_restriction"] = 1
}

func theAncientOnePermanentCardsInYard(gs *gameengine.GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	if s == nil {
		return 0
	}
	n := 0
	for _, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") || cardHasType(c, "artifact") ||
			cardHasType(c, "enchantment") || cardHasType(c, "land") ||
			cardHasType(c, "planeswalker") || cardHasType(c, "battle") {
			n++
		}
	}
	return n
}

func theAncientOneActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_ancient_one_loot_mill"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	// Draw a card.
	drew := drawOne(gs, src.Controller, src.Card.DisplayName())
	// Discard a card. We don't have a clean per_card discard helper;
	// do it inline by grabbing the first hand card and moving to GY.
	discarded := (*gameengine.Card)(nil)
	if len(seat.Hand) > 0 {
		discarded = seat.Hand[0]
		moveCardBetweenZones(gs, src.Controller, discarded, "hand", "graveyard", "the_ancient_one_discard")
	}
	// Mill = MV of discarded.
	if discarded != nil {
		mv := cardCMC(discarded)
		if mv > 0 {
			oppSeat := -1
			for _, opp := range gs.Opponents(src.Controller) {
				oppSeat = opp
				break
			}
			if oppSeat >= 0 {
				target := gs.Seats[oppSeat]
				milled := 0
				for milled < mv && len(target.Library) > 0 {
					top := target.Library[0]
					moveCardBetweenZones(gs, oppSeat, top, "library", "graveyard", "the_ancient_one_mill")
					milled++
				}
				emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
					"seat":         src.Controller,
					"target_seat":  oppSeat,
					"discarded_mv": mv,
					"milled":       milled,
					"discarded":    discarded.DisplayName(),
					"drew":         theAncientOneCardName(drew),
				})
				return
			}
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"drew":      theAncientOneCardName(drew),
		"discarded": theAncientOneCardName(discarded),
	})
}

func theAncientOneCardName(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
