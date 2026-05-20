package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKudoKingAmongBears wires Kudo, King Among Bears.
//
// Oracle text:
//
//	Other creatures have base power and toughness 2/2 and are Bears in
//	addition to their other types.
//
// Implementation (R46 stub port):
//   - Register a layer-7b continuous effect that SETS base power and
//     toughness to 2/2 on every creature other than Kudo herself.
//   - Register a layer-4 continuous effect that ADDS "bear" to the
//     subtype line of every other creature (additive — doesn't strip
//     existing subtypes).
//   - Both effects share SourcePerm = Kudo, so
//     UnregisterContinuousEffectsForPermanent on LTB tears them down.
//
// Notes:
//   - "Other" excludes Kudo specifically; the predicate compares
//     against the captured permanent pointer.
//   - The 2/2 base is the §613.4b base SET (sublayer b), not the
//     §613.4c modify (sublayer c). +1/+1 counters and anthems still
//     stack on top in the normal sublayer order.
func registerKudoKingAmongBears(r *Registry) {
	r.OnETB("Kudo, King Among Bears", kudoKingAmongBearsRegister)
}

func kudoKingAmongBearsRegister(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kudo_king_among_bears_static"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	src := perm
	others := func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
		if t == nil || t == src {
			return false
		}
		return t.IsCreature()
	}
	ts := perm.Timestamp
	suffix := strconv.Itoa(ts)

	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "b",
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Kudo, King Among Bears",
		ControllerSeat: perm.Controller,
		HandlerID:      "Kudo, King Among Bears:base_pt_2_2:" + suffix,
		Predicate:      others,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			chars.Power = 2
			chars.Toughness = 2
			chars.BasePower = 2
			chars.BaseToughness = 2
		},
	})

	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerType,
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Kudo, King Among Bears",
		ControllerSeat: perm.Controller,
		HandlerID:      "Kudo, King Among Bears:add_bear:" + suffix,
		Predicate:      others,
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			for _, t := range chars.Subtypes {
				if t == "bear" || t == "Bear" {
					return
				}
			}
			chars.Subtypes = append(chars.Subtypes, "bear")
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"base_pt":  "2/2",
		"add_type": "bear",
	})
}
