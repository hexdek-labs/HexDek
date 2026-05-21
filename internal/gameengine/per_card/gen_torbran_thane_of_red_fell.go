package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTorbranThaneOfRedFell wires Torbran, Thane of Red Fell.
//
// Oracle text:
//
//	If a red source you control would deal damage to an opponent or a
//	permanent an opponent controls, it deals that much damage plus 2
//	instead.
//
// Implementation (R54 — damage replacement primitive):
//   - ETB registers a DamageReplacement closure on the engine's
//     gs.DamageReplacements registry. The closure filters on
//     (source is red AND source controlled by Torbran's controller
//     AND target is opponent or opponent-controlled permanent) and
//     adds +2 to ctx.Amount.
//   - LTB unregisters the closure via
//     UnregisterDamageReplacementsForPermanent so a bounced /
//     exiled Torbran stops applying the +2.
//   - Multiple Torbrans stack additively per §616 — registering
//     each replacement independently means a second Torbran adds
//     another +2 closure (total +4).
//   - The pre-R54 seat-flag breadcrumbs (torbran_red_damage_seat /
//     torbran_red_damage_bonus) are dropped — the engine now reads
//     the actual replacement registry, not the flag.
func registerTorbranThaneOfRedFell(r *Registry) {
	r.OnETB("Torbran, Thane of Red Fell", torbranETBRegisterReplacement)
	r.OnTrigger("Torbran, Thane of Red Fell", "permanent_ltb", torbranLTBUnregister)
}

func torbranETBRegisterReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "torbran_red_damage_plus_two"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "torbran_red_plus_two",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Source == nil || ctx.Source.Card == nil {
				return
			}
			if ctx.Source.Controller != controller {
				return
			}
			if !torbranIsRedSource(ctx.Source.Card) {
				return
			}
			// "an opponent or a permanent an opponent controls" —
			// TargetSeat is the seat of either the damaged player
			// (player-target damage) or the target permanent's
			// controller (creature/planeswalker-target damage).
			if ctx.TargetSeat == controller {
				return
			}
			ctx.Amount += 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  controller,
		"bonus": 2,
	})
}

func torbranLTBUnregister(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterDamageReplacementsForPermanent(perm)
}

// torbranIsRedSource returns true if the card has red among its
// colors or carries a "pip:R" type tag (matches the test-fixture
// convention used throughout per_card_test.go for color-by-tag).
func torbranIsRedSource(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, col := range c.Colors {
		if strings.EqualFold(col, "R") {
			return true
		}
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "pip:R") {
			return true
		}
	}
	return false
}
