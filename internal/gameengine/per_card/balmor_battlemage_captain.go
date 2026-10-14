package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBalmorBattlemageCaptain wires Balmor, Battlemage Captain.
//
// Oracle text:
//
//	Flying
//	Whenever you cast an instant or sorcery spell, creatures you
//	control get +1/+0 and gain trample until end of turn.
func registerBalmorBattlemageCaptain(r *Registry) {
	r.OnTrigger("Balmor, Battlemage Captain", "instant_or_sorcery_cast", balmorTeamPump)
}

// balmorTrampleUEOTMarker is this trigger's source identity in the r87
// refcount (keyword_flag_grants_r87.go), replacing the raw shared
// Flags["kw:trample"] stamp.
const balmorTrampleUEOTMarker = "balmor_battlemage_trample_ueot"

func balmorTeamPump(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "balmor_team_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	ts := gs.NextTimestamp()
	count := 0
	changed := 0
	var granted []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		// r93: the guard here used to be `if p.Flags["kw:trample"] == 0`,
		// justified as "so we don't strip printed trample at end of
		// turn". Both halves are wrong. A printed keyword lives in the
		// AST, never in Flags, so the strip could never have reached it;
		// what the guard actually skipped was ANOTHER grant's shared
		// flag, making Balmor decline to record its own. That source
		// then strips, and the creature loses trample with Balmor's
		// until-end-of-turn grant still live. The refcount removes the
		// need for a guard at all.
		if gameengine.GrantKeywordFlag(p, "trample", balmorTrampleUEOTMarker) {
			changed++
		}
		granted = append(granted, p)
		count++
	}
	if count > 0 || changed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	if len(granted) > 0 {
		captured := granted
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				// r93: the strip side never invalidated. Neither
				// delayed-trigger runner (phases.go:639, state.go:692)
				// does it for the EffectFn, so the stale entry outlived
				// the grant it described.
				dropped := 0
				for _, p := range captured {
					if gameengine.RevokeKeywordFlag(p, "trample", balmorTrampleUEOTMarker) {
						dropped++
					}
				}
				if dropped > 0 {
					gs.InvalidateCharacteristicsCache()
				}
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"creatures": count,
	})
}
