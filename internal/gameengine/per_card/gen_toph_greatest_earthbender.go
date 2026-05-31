package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophGreatestEarthbender wires Toph, Greatest Earthbender.
//
// Oracle text:
//
//	When Toph, Greatest Earthbender enters, earthbend X, where X is
//	the amount of mana spent to cast it. (To earthbend N, look at the
//	top N cards of your library. You may put a land card from among
//	them onto the battlefield. Put the rest on the bottom of your
//	library in a random order.)
//	Land creatures you control have double strike.
//
// Implementation:
//   - ETB earthbend X: peek top X (X = Toph CMC, since we don't track
//     "mana spent to cast" precisely yet). Find the highest-impact land
//     card (prefer lands with mana abilities + non-basics > basics).
//     Put it onto the battlefield; bury the rest.
//   - "Land creatures you control have double strike" — apply the
//     double_strike runtime keyword flag to all land creatures the
//     controller has on the battlefield, refreshable via permanent_etb.
func registerTophGreatestEarthbender(r *Registry) {
	r.OnETB("Toph, Greatest Earthbender", tophETBEarthbendAndAnthem)
	r.OnTrigger("Toph, Greatest Earthbender", "permanent_etb", tophRefreshLandCreatureAnthem)
}

func tophETBEarthbendAndAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_etb_earthbend_and_anthem"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	x := cardCMC(perm.Card)
	if x < 1 {
		x = 1
	}
	if x > len(seat.Library) {
		x = len(seat.Library)
	}
	// Snapshot top X (Wave 2 multi-step migration — no pre-splice).
	top := append([]*gameengine.Card(nil), seat.Library[:x]...)
	var pick *gameengine.Card
	pickIdx := -1
	for i, c := range top {
		if c == nil || !cardHasType(c, "land") {
			continue
		}
		if pick == nil {
			pick = c
			pickIdx = i
			continue
		}
		// Prefer non-basics (more impact).
		if !cardHasType(pick, "basic") && cardHasType(c, "basic") {
			continue
		}
		if cardHasType(pick, "basic") && !cardHasType(c, "basic") {
			pick = c
			pickIdx = i
		}
	}
	// enterBattlefieldWithETB → createPermanent sweeps the picked card
	// from library (RemoveCardFromAllPrivateZones). No pre-splice needed.
	if pick != nil {
		enterBattlefieldWithETB(gs, perm.Controller, pick, false)
	}
	// Non-picked top cards stay in library; rotate them to the bottom in
	// random order (within-zone reorder — no zone change).
	bottoms := make([]*gameengine.Card, 0, x-1)
	for i, c := range top {
		if i == pickIdx || c == nil {
			continue
		}
		if len(seat.Library) == 0 || seat.Library[0] != c {
			continue
		}
		seat.Library = seat.Library[1:]
		bottoms = append(bottoms, c)
	}
	if len(bottoms) > 1 && gs.Rng != nil {
		gs.Rng.Shuffle(len(bottoms), func(i, j int) {
			bottoms[i], bottoms[j] = bottoms[j], bottoms[i]
		})
	}
	seat.Library = append(seat.Library, bottoms...)

	tophApplyLandAnthem(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"earthbend":    x,
		"land_to_play": pick != nil,
	})
}

func tophRefreshLandCreatureAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	tophApplyLandAnthem(gs, perm)
}

func tophApplyLandAnthem(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") || !cardHasType(p.Card, "land") {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:double strike"] = 1
	}
}
