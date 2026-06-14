package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaitCageBrawler wires Cait, Cage Brawler.
//
// Oracle text:
//
//	During your turn, Cait has indestructible.
//	Whenever Cait attacks, you and defending player each draw a card,
//	then discard a card. Put two +1/+1 counters on Cait if you
//	discarded the card with the greatest mana value among those cards
//	or tied for greatest.
//
// Implementation: simulate symmetric loot. Both players draw 1 then
// discard 1. We always discard the highest-CMC card on Cait's controller
// side (favorable for trigger), and lowest on opponent. Compare the two
// discarded CMCs to decide if Cait gets +1/+1 counters.
func registerCaitCageBrawler(r *Registry) {
	r.OnTrigger("Cait, Cage Brawler", "attacks", caitAttacks)
}

func caitAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cait_attack_loot"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever Cait, Cage Brawler attacks" fires only when
	// Cait itself attacks. creature_attacks is dispatched once per declared
	// attacker, so without this the symmetric loot + counter check ran once per
	// attacker instead of once. See the Aang and Katara fix (5254f9cf). (The
	// attacker_seat check below is now subsumed.)
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	// Engine fires creature_attacks with attacker_perm + attacker_seat;
	// defender is stored on the attacker via setAttackerDefender. Legacy
	// "seat" / "defender_seat" reads kept as fallbacks for callers that
	// thread them explicitly.
	attackerSeat, ok := ctx["attacker_seat"].(int)
	if !ok {
		attackerSeat, _ = ctx["seat"].(int)
	}
	if attackerSeat != perm.Controller {
		return
	}
	defenderSeat := -1
	if atk, ok := ctx["attacker_perm"].(*gameengine.Permanent); ok && atk != nil {
		if d, found := gameengine.AttackerDefender(atk); found {
			defenderSeat = d
		}
	}
	if defenderSeat < 0 {
		defenderSeat, _ = ctx["defender_seat"].(int)
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	myCMC := caitLoot(gs, perm.Controller, perm.Card.DisplayName(), true)
	oppCMC := caitLoot(gs, defenderSeat, perm.Card.DisplayName(), false)
	winner := myCMC >= oppCMC && myCMC >= 0
	if winner {
		perm.AddCounter("+1/+1", 2)
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"my_discard_cmc":  myCMC,
		"opp_discard_cmc": oppCMC,
		"got_counters":   winner,
	})
}

// caitLoot draws a card then discards. If high=true, discards the
// highest-CMC card after the draw; otherwise the lowest. Returns the
// discarded card's CMC, or -1 if nothing was discarded.
func caitLoot(gs *gameengine.GameState, seatIdx int, source string, high bool) int {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return -1
	}
	s := gs.Seats[seatIdx]
	if s == nil || s.Lost {
		return -1
	}
	drawOne(gs, seatIdx, source)
	if len(s.Hand) == 0 {
		return -1
	}
	pickIdx := 0
	pickCMC := gameengine.ManaCostOf(s.Hand[0])
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if (high && cmc > pickCMC) || (!high && cmc < pickCMC) {
			pickIdx = i
			pickCMC = cmc
		}
	}
	c := s.Hand[pickIdx]
	gameengine.DiscardCard(gs, c, seatIdx)
	return pickCMC
}
