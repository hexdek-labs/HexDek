package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYorionSkyNomadCustom implements Yorion's blink ETB. The
// auto-generated gen_*.go stub is a no-op.
//
// Oracle text:
//
//	Companion — Your starting deck contains at least twenty cards more
//	than the minimum deck size. (Companion deckbuilding constraint —
//	engine territory.)
//	Flying
//	When Yorion enters, exile any number of other nonland permanents
//	you own and control. Return those cards to the battlefield at the
//	beginning of the next end step.
//
// Implementation notes:
//   - Flying is a static keyword handled by the AST pipeline.
//   - We exile EVERY other nonland permanent we own and control (the
//     "any number" choice always picks the maximum because re-ETB
//     is virtually always positive value: triggers re-fire, P/T
//     resets to base, summoning sickness clears for tokens that
//     return from exile, etc.).
//   - Returns happen via a delayed trigger at next_end_step. Exiled
//     cards are tracked by Card pointer; the delayed trigger walks
//     the captured list and creates fresh permanents on the
//     controller's battlefield.
//   - Companion deckbuilding constraint is engine territory and is
//     emitPartial-flagged.
// R52 batch L: REGISTRATION NEUTERED. The gen_yorion_sky_nomad.go
// auto-gen body was promoted to a substantive blink implementation
// in R41 (registerYorionSkyNomad in batch_generated.go). This
// custom_*.go was added later via zz_handler_q45_register.go and
// duplicated the entire ETB blink, so every Yorion ETB exiled-and-
// returned the controller's nonland-permanent set TWICE. Neutering
// the custom-side registration keeps the canonical gen handler as
// the sole owner; the local helper functions are retained because
// other tests in the package import them, but the registry hook is
// dropped.
func registerYorionSkyNomadCustom(r *Registry) {
	_ = r
}

func yorionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yorion_blink"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	// Collect every other nonland permanent we own and control.
	var exileTargets []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		if p.Owner != seatIdx {
			continue
		}
		exileTargets = append(exileTargets, p)
	}

	exiled := make([]*gameengine.Card, 0, len(exileTargets))
	for _, p := range exileTargets {
		card := p.Card
		// Battlefield → exile: the engine requires callers to remove
		// the Permanent themselves; MoveCard's "battlefield" arm is a
		// no-op for source removal (see zone_move.go:239 comment).
		removePermanent(gs, p)
		gs.Seats[seatIdx].Exile = append(gs.Seats[seatIdx].Exile, card)
		gameengine.FireZoneChangeTriggers(gs, p, card, "battlefield", "exile")
		gameengine.DetachAll(gs, p)
		exiled = append(exiled, card)
	}

	// Schedule the return at next end step.
	if len(exiled) > 0 {
		captured := exiled
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: seatIdx,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, c := range captured {
					if c == nil {
						continue
					}
					enterBattlefieldWithETB(gs, seatIdx, c, false)
				}
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seatIdx,
		"exiled": len(exiled),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"companion deckbuilding constraint not enforced at engine level")
}
