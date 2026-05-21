package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAkiriLineSlinger wires Akiri, Line-Slinger.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{R}{W}
//	Legendary Creature — Kor Soldier Ally
//	First strike, vigilance
//	Akiri gets +1/+0 for each artifact you control.
//	Partner
//
// R51 batch J port (cheap CMC=2 commander; promote stub to real layer effect):
//   - Replace the ETB-snapshot + per-artifact-ETB recount approach with a
//     layer-7c continuous effect whose ApplyFn counts the controller's
//     artifacts at evaluation time and writes +N/+0 directly to chars.
//     This stays in sync without per-artifact triggers and survives
//     artifact LTBs that the prior implementation didn't react to.
//   - SourcePerm = Akiri so UnregisterContinuousEffectsForPermanent on
//     LTB tears down automatically. First strike + vigilance + Partner
//     stay on the AST keyword pipeline.
func registerAkiriLineSlinger(r *Registry) {
	r.OnETB("Akiri, Line-Slinger", akiriLineSlingerRegister)
}

func akiriLineSlingerRegister(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "akiri_line_slinger_layer_7c_artifact_buff"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	src := perm
	ts := perm.Timestamp
	suffix := strconv.Itoa(ts)
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "c",
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Akiri, Line-Slinger",
		ControllerSeat: perm.Controller,
		HandlerID:      "Akiri, Line-Slinger:artifact_buff:" + suffix,
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
			return t == src
		},
		ApplyFn: func(gs *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			if chars == nil {
				return
			}
			seat := gs.Seats[src.Controller]
			if seat == nil {
				return
			}
			count := 0
			for _, p := range seat.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if cardHasType(p.Card, "artifact") {
					count++
				}
			}
			chars.Power += count
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"layer": "7c",
	})
}
