package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheWanderingMinstrel wires The Wandering Minstrel.
//
// Oracle text (Scryfall, verified):
//
//	Lands you control enter untapped.
//	The Minstrel's Ballad — At the beginning of combat on your turn,
//	if you control five or more Towns, create a 2/2 Elemental creature
//	token that's all colors.
//	{3}{W}{U}{B}{R}{G}: Other creatures you control get +X/+X until
//	end of turn, where X is the number of Towns you control.
//
// Implementation (R49 stub port):
//   - "Lands enter untapped" — engine-side AST static; emitPartial gap
//     breadcrumb (the per_card layer can't intercept land-ETB tap state).
//   - The Minstrel's Ballad (combat_begin trigger): gate on
//     active_seat == controller AND townCount(controller) >= 5. Spawns
//     a 2/2 Elemental token stamped with all five colors.
//   - {3}{W}{U}{B}{R}{G} activated pump: cost defensively enforced as
//     seat.ManaPool >= 8 (the engine activation dispatcher pays
//     before reaching here; this is a fixture-mode guard). X = the
//     town count when activated. Apply Modification{Power:X,
//     Toughness:X, Duration:until_end_of_turn} to every OTHER creature
//     the controller controls.
func registerTheWanderingMinstrel(r *Registry) {
	r.OnETB("The Wandering Minstrel", theWanderingMinstrelETB)
	r.OnActivated("The Wandering Minstrel", theWanderingMinstrelActivate)
	r.OnTrigger("The Wandering Minstrel", "combat_begin", theWanderingMinstrelCombatBegin)
}

func theWanderingMinstrelETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_wandering_minstrel_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"lands_enter_untapped_static_handled_by_ast_engine")
}

func minstrelTownCount(gs *gameengine.GameState, seatIdx int) int {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "town") {
			n++
			continue
		}
		if strings.Contains(strings.ToLower(p.Card.TypeLine), "town") {
			n++
		}
	}
	return n
}

func theWanderingMinstrelCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_wandering_minstrel_ballad"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	activeSeat := -1
	if ctx != nil {
		if v, ok := ctx["active_seat"].(int); ok {
			activeSeat = v
		}
	}
	if activeSeat != perm.Controller {
		return
	}
	if minstrelTownCount(gs, perm.Controller) < 5 {
		return
	}
	tok := gameengine.CreateCreatureToken(gs, perm.Controller, "Elemental Token",
		[]string{"creature", "elemental"}, 2, 2)
	if tok != nil && tok.Card != nil {
		tok.Card.Colors = []string{"W", "U", "B", "R", "G"}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"towns": minstrelTownCount(gs, perm.Controller),
	})
}

func theWanderingMinstrelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_wandering_minstrel_anthem"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	if seat.ManaPool < 8 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  8,
			"available": seat.ManaPool,
		})
		return
	}
	x := minstrelTownCount(gs, seatIdx)
	if x <= 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_towns", nil)
		return
	}

	seat.ManaPool -= 8
	gameengine.SyncManaAfterSpend(seat)

	ts := gs.NextTimestamp()
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == src || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     x,
			Toughness: x,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		pumped++
	}
	if pumped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"x":      x,
		"pumped": pumped,
	})
}
