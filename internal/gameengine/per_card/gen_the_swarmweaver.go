package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheSwarmweaver wires The Swarmweaver.
//
// Oracle text (DSK, {2}{B}{G}, 2/3 Legendary Artifact Creature — Scarecrow):
//
//	When The Swarmweaver enters, create two 1/1 black and green Insect
//	creature tokens with flying.
//	Delirium — As long as there are four or more card types among
//	cards in your graveyard, Insects and Spiders you control get +1/+1
//	and have deathtouch.
//
// Implementation (R50 batch F):
//   - ETB creates two 1/1 BG Insect tokens with flying via
//     CreateCreatureToken.
//   - Delirium anthem: evaluated at ETB and refreshed on upkeep + end
//     step. Counts distinct card types among cards in the controller's
//     graveyard (artifact, creature, enchantment, instant, land,
//     planeswalker, sorcery, battle, tribal). When ≥4, every Insect
//     and Spider the controller controls gets the swarmweaver_anthem
//     flag and kw:deathtouch stamped. Falls off cleanly when the
//     count drops below 4. The AST engine handles the as-long-as
//     static at layer 7 for scanners that consult it; this runtime
//     refresh ensures combat math sees the boost on every refresh.
func registerTheSwarmweaver(r *Registry) {
	r.OnETB("The Swarmweaver", theSwarmweaverETB)
	r.OnTrigger("The Swarmweaver", "upkeep_controller", theSwarmweaverRefresh)
	r.OnTrigger("The Swarmweaver", "end_step", theSwarmweaverRefresh)
}

func theSwarmweaverETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_swarmweaver_etb_insect_tokens"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 2; i++ {
		gameengine.CreateCreatureToken(gs, seat, "Insect Token",
			[]string{"creature", "insect", "pip:B", "pip:G", "flying"}, 1, 1)
	}
	theSwarmweaverApplyDelirium(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": 2,
	})
}

func theSwarmweaverRefresh(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	theSwarmweaverApplyDelirium(gs, perm)
}

func theSwarmweaverApplyDelirium(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_swarmweaver_delirium_apply"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	types := theSwarmweaverDistinctYardTypes(seat)
	active := types >= 4
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasSubtype(p.Card, "insect") && !cardHasSubtype(p.Card, "spider") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if active {
			p.Flags["swarmweaver_anthem"] = 1
			p.Flags["kw:deathtouch"] = 1
			stamped++
		} else {
			delete(p.Flags, "swarmweaver_anthem")
			delete(p.Flags, "kw:deathtouch")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"yard_types":     types,
		"delirium":       active,
		"insects_buffed": stamped,
	})
}

func theSwarmweaverDistinctYardTypes(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		for _, t := range []string{"artifact", "creature", "enchantment", "instant", "land", "planeswalker", "sorcery", "battle", "tribal"} {
			if cardHasType(c, t) {
				seen[t] = true
			}
		}
	}
	return len(seen)
}
