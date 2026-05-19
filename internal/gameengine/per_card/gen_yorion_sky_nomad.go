package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYorionSkyNomad wires Yorion, Sky Nomad.
//
// Oracle text (Scryfall, verified):
//
//	Companion — Your starting deck contains at least twenty cards more
//	than the minimum deck size.
//	Flying
//	When Yorion enters, exile any number of other nonland permanents
//	you own and control. Return those cards to the battlefield at the
//	beginning of the next end step.
//
// Implementation (R41 stub port):
//   - Companion alt-cast: outside-the-game zone surface; emitPartial.
//   - Flying: AST keyword pipeline.
//   - ETB blink: AI policy is greedy-upside — exile every nonland,
//     non-token permanent the controller owns and controls (excluding
//     Yorion itself). Tokens are skipped because token cards cease to
//     exist in exile and never come back (CR §111.10). Each exiled card
//     is captured into a delayed "next_end_step" trigger that returns
//     it from exile to the battlefield via MoveCard, firing ETB hooks
//     again — which is the entire reason to run a Yorion deck.
func registerYorionSkyNomad(r *Registry) {
	r.OnETB("Yorion, Sky Nomad", yorionSkyNomadETB)
}

func yorionSkyNomadETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yorion_sky_nomad_blink"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"companion_alt_cast_from_outside_the_game_not_modeled")

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Snapshot the blink set first — ExilePermanent mutates the
	// battlefield slice, so iterating live would skip neighbors.
	var targets []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if p.IsLand() {
			continue
		}
		if p.Owner != perm.Controller {
			continue
		}
		if p.IsToken() {
			continue
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"blinked": 0,
		})
		return
	}

	exiledCards := make([]*gameengine.Card, 0, len(targets))
	exiledNames := make([]string, 0, len(targets))
	for _, p := range targets {
		card := p.Card
		if !gameengine.ExilePermanent(gs, p, perm) {
			continue
		}
		if card == nil {
			continue
		}
		exiledCards = append(exiledCards, card)
		exiledNames = append(exiledNames, card.DisplayName())
	}

	if len(exiledCards) > 0 {
		owner := perm.Controller
		captured := make([]*gameengine.Card, len(exiledCards))
		copy(captured, exiledCards)
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: owner,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				yorionReturnFromExile(gs, owner, captured)
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"blinked":  len(exiledCards),
		"cards":    exiledNames,
	})
}

// yorionReturnFromExile pulls each captured card from the owner's exile
// back to the battlefield, firing ETB triggers. Cards that have already
// left exile (player cast them via some other zone-cast grant, or LTB
// triggers shipped them elsewhere) are skipped.
func yorionReturnFromExile(gs *gameengine.GameState, owner int, cards []*gameengine.Card) {
	const slug = "yorion_sky_nomad_return"
	if gs == nil || owner < 0 || owner >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[owner]
	if seat == nil {
		return
	}
	returned := 0
	for _, c := range cards {
		if c == nil {
			continue
		}
		idx := -1
		for i, e := range seat.Exile {
			if e == c {
				idx = i
				break
			}
		}
		if idx < 0 {
			continue
		}
		gameengine.MoveCard(gs, c, owner, "exile", "battlefield", "yorion_return")
		returned++
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "yorion_return_eot",
		Seat:   owner,
		Source: "Yorion, Sky Nomad",
		Amount: returned,
		Details: map[string]interface{}{
			"queued":   len(cards),
			"returned": returned,
		},
	})
	emit(gs, slug, "Yorion, Sky Nomad", map[string]interface{}{
		"seat":     owner,
		"returned": returned,
		"queued":   len(cards),
	})
}
