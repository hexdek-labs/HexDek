package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYuriko wires Yuriko, the Tiger's Shadow.
//
// Commander ninjutsu {U}{B}: bypass commander tax, enter from command zone.
// Trigger: whenever a Ninja you control deals combat damage to a player,
// reveal top of library → hand, each opponent loses life = CMC.
func registerYuriko(r *Registry) {
	r.OnTrigger("Yuriko, the Tiger's Shadow", "combat_damage_player", yurikoTrigger)
}

func yurikoTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if perm == nil || gs == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	// The creature that dealt damage must be controlled by Yuriko's controller
	sourceSeat := -1
	sourceName := ""
	if ctx != nil {
		if ss, ok := ctx["source_seat"].(int); ok {
			sourceSeat = ss
		}
		if sn, ok := ctx["source_card"].(string); ok {
			sourceName = sn
		}
	}
	if sourceSeat != controller {
		return
	}

	// Check if the source creature is a Ninja (or Changeling)
	isNinja := false
	for _, p := range gs.Seats[controller].Battlefield {
		if p.Card == nil {
			continue
		}
		if p.Card.DisplayName() != sourceName {
			continue
		}
		for _, t := range p.Card.Types {
			tl := strings.ToLower(t)
			if tl == "ninja" || tl == "changeling" {
				isNinja = true
				break
			}
		}
		// Cards that entered via ninjutsu are ninjas by definition
		if p.Flags != nil {
			if _, ok := p.Flags["ninjutsu_entry"]; ok {
				isNinja = true
			}
		}
		break
	}
	if !isNinja {
		return
	}

	// Reveal top card → hand, each opponent loses life = CMC
	seat := gs.Seats[controller]
	if len(seat.Library) == 0 {
		return
	}

	topCard := seat.Library[0]
	gameengine.MoveCard(gs, topCard, controller, "library", "hand", "effect")
	cmc := topCard.CMC

	gs.LogEvent(gameengine.Event{
		Kind:   "yuriko_trigger",
		Seat:   controller,
		Source: "Yuriko, the Tiger's Shadow",
		Amount: cmc,
		Details: map[string]interface{}{
			"revealed_card": topCard.DisplayName(),
			"cmc":           cmc,
			"ninja_source":  sourceName,
		},
	})

	// Each opponent loses life equal to the revealed card's CMC
	for i := range gs.Seats {
		if i == controller || gs.Seats[i].Lost {
			continue
		}
		gs.Seats[i].Life -= cmc
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   i,
			Amount: -cmc,
			Source: "Yuriko, the Tiger's Shadow",
			Details: map[string]interface{}{
				"from":   gs.Seats[i].Life + cmc,
				"to":     gs.Seats[i].Life,
				"reason": "yuriko_trigger",
			},
		})
	}
}
