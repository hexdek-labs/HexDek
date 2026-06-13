package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// withering_curse_r63.go — per_card handler for Withering Curse.
//
// Oracle text (Sorcery, {1}{B}{B}):
//
//	All creatures get -2/-2 until end of turn.
//	Infusion — If you gained life this turn, destroy all creatures instead.
//
// Replacement-orphan tail (r63 census): the base "all creatures get -2/-2"
// parses to a structured Buff node but the Infusion upgrade ("destroy all
// creatures INSTEAD if you gained life this turn") dumped to an inert
// parsed_tail. So the spell could never escalate to a full board wipe. A
// bespoke OnResolve handler checks the caster's life-gained-this-turn and
// either destroys every creature or applies the -2/-2 sweep. Symmetric
// (affects all players' creatures); no damage pipeline.
func init() {
	registerWitheringCurseR63(Global())
	AddResetHook(registerWitheringCurseR63)
}

func registerWitheringCurseR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Withering Curse", witheringCurseResolve)
}

func witheringCurseResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "withering_curse"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	// Infusion — the conditional that was inert pre-r63.
	infusion := gs.Seats[seat] != nil && gs.Seats[seat].Turn.LifeGained > 0

	if infusion {
		// Destroy all creatures. Snapshot first — DestroyPermanent mutates
		// the battlefield slices.
		var victims []*gameengine.Permanent
		for _, s := range gs.Seats {
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p != nil && p.Card != nil && p.IsCreature() {
					victims = append(victims, p)
				}
			}
		}
		destroyed := 0
		for _, p := range victims {
			if gameengine.DestroyPermanent(gs, p, nil) {
				destroyed++
			}
		}
		emit(gs, slug, "Withering Curse", map[string]interface{}{
			"seat": seat, "mode": "infusion_destroy_all", "destroyed": destroyed,
		})
		_ = gs.CheckEnd()
		return
	}

	// Base — all creatures get -2/-2 until end of turn.
	affected := 0
	ts := gs.NextTimestamp()
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			p.Modifications = append(p.Modifications, gameengine.Modification{
				Power: -2, Toughness: -2, Duration: "until_end_of_turn", Timestamp: ts,
			})
			affected++
		}
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, "Withering Curse", map[string]interface{}{
		"seat": seat, "mode": "minus_two_sweep", "affected": affected,
	})
	_ = gs.CheckEnd()
}
