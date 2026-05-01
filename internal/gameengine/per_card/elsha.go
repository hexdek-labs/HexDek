package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerElsha wires Elsha of the Infinite.
//
// Oracle text:
//
//	Prowess
//	You may look at the top card of your library any time.
//	You may cast noncreature spells from the top of your library. If
//	you cast a spell this way, cast it as though it had flash.
//
// Implementation strategy:
//   - Prowess: explicit "noncreature_spell_cast" trigger that grants +1/+1
//     EOT to Elsha when its controller casts a noncreature spell. This
//     duplicates the engine's auto-prowess path for safety; the engine
//     keyword check is a no-op when the AST didn't surface a Keyword
//     ability with name "prowess".
//   - Cast-from-top-of-library: simulated by an upkeep_controller trigger
//     that peeks the top of the library and, if it's a noncreature card,
//     moves it to hand so the hat treats it as castable. This is a
//     simulation approximation — the real ability grants permission, not
//     a zone change — but it keeps the hat's decision-tree honest about
//     what's accessible without adding a new "virtual hand" pathway.
func registerElsha(r *Registry) {
	r.OnTrigger("Elsha of the Infinite", "noncreature_spell_cast", elshaProwess)
	r.OnTrigger("Elsha of the Infinite", "upkeep_controller", elshaTopOfLibrary)
}

func elshaProwess(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elsha_prowess"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	perm.Modifications = append(perm.Modifications, gameengine.Modification{
		Power:     1,
		Toughness: 1,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "+1/+1 until end of turn",
	})
}

func elshaTopOfLibrary(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elsha_cast_from_top"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	s := gs.Seats[perm.Controller]
	if s == nil || len(s.Library) == 0 {
		return
	}
	top := s.Library[0]
	if top == nil || cardHasType(top, "creature") || cardHasType(top, "land") {
		return
	}
	gameengine.MoveCard(gs, top, perm.Controller, "library", "hand", "elsha_cast_from_top")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"revealed": top.DisplayName(),
		"approx":   "moved_to_hand_to_simulate_cast_permission",
	})
}
