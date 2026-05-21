package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheEarthKing wires The Earth King.
//
// Oracle text (Scryfall, verified):
//
//	When The Earth King enters, create a 4/4 green Bear creature token.
//	Whenever one or more creatures you control with power 4 or greater
//	attack, search your library for up to that many basic land cards,
//	put them onto the battlefield tapped, then shuffle.
//
// Implementation (R49 stub port — batch A merged with batch B):
//   - ETB: 4/4 green Bear token via CreateCreatureToken with pip:G type
//     tag and an explicit Card.Colors stamp so colorless-anthem and
//     color-matters scanners both pick it up.
//   - Attack trigger registered on BOTH the canonical
//     combat_attackers_declared event (batch A) and the legacy "attack"
//     alias (batch B). Each handler has its own dedup so we never
//     double-search per combat step. Both count the controller's
//     attacking creatures whose Power() ≥ 4 and tutor one basic land
//     per such attacker, tapped, then shuffle.
func registerTheEarthKing(r *Registry) {
	r.OnETB("The Earth King", theEarthKingETB)
	r.OnTrigger("The Earth King", "combat_attackers_declared", theEarthKingAttack)
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
		[]string{"creature", "bear", "pip:G"}, 4, 4)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"G"}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"token": "Bear Token",
	})
}

// theEarthKingAttack handles the canonical "combat_attackers_declared"
// once-per-combat batch event.
func theEarthKingAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_earth_king_attack_basic_land_search"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// Per-turn dedup so combat_attackers_declared (which may fire once
	// per attacker on some paths) only runs once per attack step.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["earth_king_attack_turn"] == gs.Turn {
		return
	}
	perm.Flags["earth_king_attack_turn"] = gs.Turn

	bigAttackers := 0
	for _, p := range s.Battlefield {
		if p == nil || !p.IsAttacking() || !p.IsCreature() {
			continue
		}
		if p.Power() >= 4 {
			bigAttackers++
		}
	}
	if bigAttackers == 0 {
		return
	}

	found := []string{}
	for i := 0; i < bigAttackers; i++ {
		var land *gameengine.Card
		for _, c := range s.Library {
			if c == nil {
				continue
			}
			if cardHasType(c, "basic") && cardHasType(c, "land") {
				land = c
				break
			}
		}
		if land == nil {
			break
		}
		gameengine.MoveCard(gs, land, seat, "library", "battlefield_tapped", slug)
		found = append(found, land.DisplayName())
	}
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":         found,
			"big_attackers": bigAttackers,
			"reason":        slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"big_attackers": bigAttackers,
		"found":         found,
	})
}

// theEarthKingAttackTrigger handles the legacy "attack" alias kept for
// engine paths that fire per-attacker. Dedups by game flag so it never
// re-runs the search within the same turn.
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
	// Also dedup against combat_attackers_declared if it already ran.
	if perm.Flags != nil && perm.Flags["earth_king_attack_turn"] == gs.Turn {
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
