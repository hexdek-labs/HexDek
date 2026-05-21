package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheEarthKing wires The Earth King.
//
// Oracle text:
//
//	When The Earth King enters, create a 4/4 green Bear creature token.
//	Whenever one or more creatures you control with power 4 or greater
//	attack, search your library for up to that many basic land cards,
//	put them onto the battlefield tapped, then shuffle.
//
// Implementation (R49 stub port — batch A):
//   - ETB: 4/4 green Bear token via CreateCreatureToken so it picks up
//     pip:G and the Bear creature type properly (auto-gen mint was
//     untyped/uncolored).
//   - combat_attackers_declared: once-per-combat batch trigger. Count
//     attacking creatures controlled by The Earth King's controller
//     whose power ≥ 4 (Power() reads Modifications + counters, so
//     +1/+1 anthems are picked up). For each such attacker, search for
//     one basic land in the library and put it onto the battlefield
//     tapped. Shuffle once at the end (CR §701.19c — even on whiff).
func registerTheEarthKing(r *Registry) {
	r.OnETB("The Earth King", theEarthKingETB)
	r.OnTrigger("The Earth King", "combat_attackers_declared", theEarthKingAttack)
}

func theEarthKingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_earth_king_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.CreateCreatureToken(gs, seat, "Bear Token",
		[]string{"creature", "bear", "pip:G"}, 4, 4)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}

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
	// Per-turn dedup: combat_attackers_declared can fire once per attacker
	// in some legacy paths. Use a perm flag keyed on (turn, combat phase)
	// so the trigger fires only once per attack step.
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
			"found":          found,
			"big_attackers":  bigAttackers,
			"reason":         slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"big_attackers": bigAttackers,
		"found":         found,
	})
}
