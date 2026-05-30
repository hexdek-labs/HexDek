package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheGhoulGunslinger wires The Ghoul, Gunslinger.
//
// Oracle text:
//
//	{1}{B}{B}
//	Legendary Creature — Zombie Mutant Rogue
//	First strike
//	Whenever The Ghoul or another nontoken Zombie or Mutant you control
//	  dies, target player gets two rad counters. If that player is you,
//	  create a Treasure token.
//
// Implementation:
//   - First strike via AST keyword.
//   - "creature_dies" trigger: gated on dying creature controller ==
//     The Ghoul's controller, dying card is nontoken Zombie or Mutant
//     (or is The Ghoul itself). Targets the leftmost living opponent
//     with the rad counters; if no opponents available, targets the
//     controller and produces a Treasure.
func registerTheGhoulGunslinger(r *Registry) {
	r.OnTrigger("The Ghoul, Gunslinger", "creature_dies", theGhoulGunslingerDies)
}

func theGhoulGunslingerDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_ghoul_gunslinger_rad"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	ctrlSeat, _ := ctx["controller_seat"].(int)
	if ctrlSeat != perm.Controller {
		return
	}
	dying, _ := ctx["card"].(*gameengine.Card)
	if dying == nil {
		return
	}
	isGhoul := dying == perm.Card
	if !isGhoul {
		if cardHasType(dying, "token") {
			return
		}
		if !cardHasTypeAny(dying, "zombie", "mutant") {
			return
		}
	}

	// Pick first living opponent for rad counters.
	target := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		if gs.Seats[opp] != nil && !gs.Seats[opp].Lost {
			target = opp
			break
		}
	}
	if target < 0 {
		// No opponents; target self.
		target = perm.Controller
	}
	t := gs.Seats[target]
	if t.Flags == nil {
		t.Flags = map[string]int{}
	}
	t.Flags["rad_counters"] += 2
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_mod",
		Seat:   perm.Controller,
		Target: target,
		Source: perm.Card.DisplayName(),
		Amount: 2,
		Details: map[string]interface{}{
			"counter_kind": "rad",
			"op":           "put",
			"on_player":    true,
			"reason":       "the_ghoul_gunslinger",
		},
	})

	madeTreasure := false
	if target == perm.Controller {
		gameengine.CreateTreasureToken(gs, perm.Controller)
		madeTreasure = true
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target":       target,
		"madeTreasure": madeTreasure,
	})
}

func cardHasTypeAny(c *gameengine.Card, types ...string) bool {
	if c == nil {
		return false
	}
	for _, want := range types {
		w := strings.ToLower(want)
		for _, got := range c.Types {
			if strings.ToLower(got) == w {
				return true
			}
		}
	}
	return false
}
