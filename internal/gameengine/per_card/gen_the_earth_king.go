package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheEarthKing wires The Earth King.
//
// Oracle text (Scryfall, verified):
//
//	When The Earth King enters, create a 4/4 green Bear creature
//	token.
//	Whenever one or more creatures you control with power 4 or greater
//	attack, search your library for up to that many basic land cards,
//	put them onto the battlefield tapped, then shuffle.
//
// Implementation (R49 stub port):
//   - ETB: spawn a 4/4 green Bear via CreateCreatureToken; stamp G
//     color so anthem / ramp interactions see it as green.
//   - Attack trigger: count how many of the controller's declared
//     attackers have power >= 4 this attack step. The engine fires
//     "creature_attacks" once per attacker; tracking the per-step
//     batch requires a dedup window. Use a turn-keyed flag
//     ("_earth_king_attack_resolved") so the search only runs on the
//     FIRST big-power attacker. That first call scans the seat's
//     battlefield for Flags["attacking"]==1 permanents with power >= 4.
//   - Library fetch: tutor up to count basics tapped through MoveCard
//     so landfall observers fire. Shuffle once at the end iff any
//     land moved.
func registerTheEarthKing(r *Registry) {
	r.OnETB("The Earth King", theEarthKingETB)
	r.OnTrigger("The Earth King", "attack", theEarthKingAttackTrigger)
}

func theEarthKingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_earth_king_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, seat, "Bear Token",
		[]string{"creature", "bear"}, 4, 4)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"G"}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"token": "Bear Token",
	})
}

func theEarthKingAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_earth_king_attack_ramp"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	atkSeat, _ := ctx["attacker_seat"].(int)
	if atkSeat != perm.Controller {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	dedup := "_earth_king_attack_resolved"
	if gs.Flags[dedup] == gs.Turn+1 {
		return
	}
	gs.Flags[dedup] = gs.Turn + 1

	s := gs.Seats[perm.Controller]
	if s == nil {
		return
	}
	count := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Flags == nil || p.Flags["attacking"] == 0 {
			continue
		}
		if gs.PowerOf(p) >= 4 {
			count++
		}
	}
	if count == 0 {
		// The current attacker (ctx) may not have its Flags["attacking"]
		// set yet — count it directly.
		if atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent); atkPerm != nil {
			if gs.PowerOf(atkPerm) >= 4 {
				count = 1
			}
		}
	}
	if count == 0 {
		return
	}

	found := []string{}
	for i := 0; i < len(s.Library) && len(found) < count; {
		c := s.Library[i]
		if c == nil {
			i++
			continue
		}
		if !isBasicLand(c) {
			i++
			continue
		}
		gameengine.MoveCard(gs, c, perm.Controller, "library", "battlefield_tapped", "earth_king_search")
		found = append(found, c.DisplayName())
	}
	if len(found) > 0 {
		shuffleLibraryPerCard(gs, perm.Controller)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"count": count,
		"found": found,
	})
}
