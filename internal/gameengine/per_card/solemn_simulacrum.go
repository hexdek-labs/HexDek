package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSolemnSimulacrum wires Solemn Simulacrum ("Sad Robot").
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Solemn%20Simulacrum):
//
//	When this creature enters, you may search your library for a basic
//	land card, put that card onto the battlefield tapped, then shuffle.
//	When this creature dies, you may draw a card.
//
// {4} Artifact Creature — Golem 2/2. Classic ramp-into-replace.
//
// Implementation:
//   - OnETB: tutor for basic land → battlefield tapped. Reuses
//     pickFirstBasicLand + shuffleLibraryPerCard helpers.
//   - OnETB also registers a `would_die` REPLACEMENT effect that
//     observes Solemn's own death and draws a card immediately. The
//     creature_dies trigger path (which fans out to per_card OnTrigger
//     handlers) walks seat.Battlefield; by the time it fires Solemn
//     has been removed, so the trigger would never reach a per_card
//     handler keyed to Solemn herself. The `would_die` replacement
//     fires BEFORE removePermanent (zone_change.go:84 and sba.go's
//     destroyPermSBA), so Solemn is still on the battlefield when
//     the observation lands. The replacement does not mutate the
//     event — it's purely an observation hook. UnregisterReplacements-
//     ForPermanent auto-cleans on LTB.
//
// Hat policy on both "you may" riders: always opt in. Both clauses
// are monotone upside across every archetype the engine scores.
func registerSolemnSimulacrum(r *Registry) {
	r.OnETB("Solemn Simulacrum", solemnSimulacrumETB)
}

func solemnSimulacrumETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "solemn_simulacrum_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Tutor: first basic land in library → battlefield tapped.
	land := pickFirstBasicLand(s.Library)
	if land != nil {
		gameengine.MoveCard(gs, land, seat, "library", "battlefield_tapped", "solemn_simulacrum_search")
	}
	shuffleLibraryPerCard(gs, seat)

	// Register the dies-draw via would_die replacement.
	registerSolemnSimulacrumDiesReplacement(gs, perm)

	if land != nil {
		emit(gs, slug, "Solemn Simulacrum", map[string]interface{}{
			"seat": seat,
			"land": land.DisplayName(),
		})
	} else {
		emitFail(gs, slug, "Solemn Simulacrum", "no_basic_land_in_library", map[string]interface{}{
			"seat": seat,
		})
	}
}

func registerSolemnSimulacrumDiesReplacement(gs *gameengine.GameState, sj *gameengine.Permanent) {
	if gs == nil || sj == nil {
		return
	}
	controller := sj.Controller
	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_die",
		HandlerID:      "solemn_simulacrum:dies_draw:" + sj.Card.DisplayName(),
		SourcePerm:     sj,
		ControllerSeat: controller,
		Timestamp:      sj.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			return ev != nil && ev.TargetPerm == sj
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			drawOne(gs, controller, "Solemn Simulacrum")
			emit(gs, "solemn_simulacrum_dies", "Solemn Simulacrum", map[string]interface{}{
				"seat": controller,
				"drew": 1,
			})
		},
	})
}
