package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMorophonTheBoundless wires Morophon, the Boundless.
//
// Oracle text:
//
//	Changeling
//	As Morophon enters, choose a creature type.
//	Spells of the chosen type you cast cost {W}{U}{B}{R}{G} less to
//	cast. This effect reduces only the amount of colored mana you pay.
//	Other creatures you control of the chosen type get +1/+1.
//
// Implementation:
//   - Choose-a-type defaults to the most-common creature subtype in the
//     controller's hand+battlefield (token-aware fallback to "human").
//   - +1/+1 anthem to other creatures of that type via Modifications,
//     refreshed on permanent_etb so creatures entering after Morophon
//     pick up the buff.
//   - The 5-color cost reduction is engine-deep (cost-modifier pipeline);
//     emit a partial breadcrumb.
func registerMorophonTheBoundless(r *Registry) {
	r.OnETB("Morophon, the Boundless", morophonETBChooseTribe)
	r.OnTrigger("Morophon, the Boundless", "permanent_etb", morophonRefreshAnthem)
}

func morophonETBChooseTribe(gs *gameengine.GameState, perm *gameengine.Permanent) {
	morophonChooseAndApply(gs, perm)
}

func morophonRefreshAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	morophonApplyAnthem(gs, perm)
}

func morophonChooseAndApply(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "morophon_choose_tribe"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	tribe := pickFavoriteCreatureType(seat)
	if tribe == "" {
		tribe = "human"
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	perm.Flags["morophon_tribe_chosen"] = 1
	seat.Flags["morophon_cost_reduction_active"] = 1
	// Stash tribe on the perm via type-line marker for cross-handler reads.
	perm.Card.Types = append(perm.Card.Types, "morophon_tribe:"+tribe)
	morophonApplyAnthem(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"tribe": tribe,
	})
	// 5-color cost reduction is wired in gameengine/cost_modifiers.go
	// (ScanCostModifiers reads the "morophon_tribe:" marker stamped on
	// perm.Card.Types above and applies a 5-generic CostModReduction
	// to spells of the chosen type the controller casts).
}

func morophonApplyAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	tribe := ""
	for _, t := range perm.Card.Types {
		if strings.HasPrefix(t, "morophon_tribe:") {
			tribe = strings.TrimPrefix(t, "morophon_tribe:")
			break
		}
	}
	if tribe == "" {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	buffed := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, tribe) {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["morophon_anthem_applied"] == 1 {
			continue
		}
		p.Flags["morophon_anthem_applied"] = 1
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  "while_source_on_battlefield",
			Timestamp: gs.NextTimestamp(),
		})
		buffed++
	}
	if buffed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
}

// pickFavoriteCreatureType returns the most-common creature subtype
// across the seat's hand and battlefield. Empty string when no
// creature subtype tally exists.
func pickFavoriteCreatureType(seat *gameengine.Seat) string {
	counts := map[string]int{}
	bump := func(c *gameengine.Card) {
		if c == nil {
			return
		}
		creature := false
		for _, t := range c.Types {
			if t == "creature" {
				creature = true
				break
			}
		}
		if !creature {
			return
		}
		for _, t := range c.Types {
			switch t {
			case "creature", "legendary", "token", "artifact", "enchantment", "land":
				continue
			}
			if strings.Contains(t, ":") {
				continue
			}
			counts[t]++
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil {
			bump(p.Card)
		}
	}
	for _, c := range seat.Hand {
		bump(c)
	}
	best := ""
	bestN := 0
	for t, n := range counts {
		if n > bestN {
			best = t
			bestN = n
		}
	}
	return best
}
