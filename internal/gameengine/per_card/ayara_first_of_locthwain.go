package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAyaraFirstOfLocthwain wires Ayara, First of Locthwain.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Whenever Ayara or another black creature you control enters,
//	each opponent loses 1 life and you gain 1 life.
//	{T}, Sacrifice another creature: Draw a card.
//
// Implementation:
//   - OnTrigger("nonland_permanent_etb"): fires whenever any nonland
//     permanent enters the battlefield. We gate on:
//     1. The entering permanent is controlled by Ayara's controller.
//     2. The entering permanent is a creature.
//     3. The entering permanent is black (at least one element of
//        Card.Colors is "B") OR is Ayara herself (so Ayara's own ETB
//        triggers the drain).
//     On trigger: each opponent loses 1 life, controller gains 1 life.
//   - OnActivated(0): the "{T}, Sacrifice another creature: Draw a card"
//     ability. Ayara must be untapped; tap her, pick the lowest-value
//     non-Ayara creature via chooseSacVictimNotSelf, sacrifice it (fires
//     creature_dies and all death-trigger observers), then draw one card.
func registerAyaraFirstOfLocthwain(r *Registry) {
	r.OnTrigger("Ayara, First of Locthwain", "nonland_permanent_etb", ayaraFirstOfLocthwainETBTrigger)
	r.OnActivated("Ayara, First of Locthwain", ayaraFirstOfLocthwainActivate)
}

// ayaraFirstOfLocthwainETBTrigger fires whenever a nonland permanent enters
// under any player's control. We scope to Ayara's controller's black creatures
// (including Ayara herself).
func ayaraFirstOfLocthwainETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ayara_first_of_locthwain_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// The entering permanent must be controlled by Ayara's controller.
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}

	// The entering permanent must be a creature.
	enteringCard, _ := ctx["card"].(*gameengine.Card)
	if enteringCard == nil {
		return
	}
	if !cardHasType(enteringCard, "creature") {
		return
	}

	// The entering creature must be black (Colors contains "B"), OR it
	// must be Ayara herself (her name matches — she triggers on her own
	// entry even if Colors is unpopulated in test fixtures).
	isBlack := false
	for _, col := range enteringCard.Colors {
		if strings.ToUpper(col) == "B" {
			isBlack = true
			break
		}
	}
	isAyara := strings.EqualFold(enteringCard.DisplayName(), "Ayara, First of Locthwain")
	if !isBlack && !isAyara {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	controllerSeat := gs.Seats[seat]
	if controllerSeat == nil || controllerSeat.Lost {
		return
	}

	// Each opponent loses 1 life.
	opps := gs.Opponents(seat)
	for _, opp := range opps {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		os.Life--
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   opp,
			Target: opp,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"reason":   "ayara_drain",
				"creature": enteringCard.DisplayName(),
			},
		})
	}

	// Controller gains 1 life.
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"creature":    enteringCard.DisplayName(),
		"opponents":   len(opps),
		"life_gained": 1,
	})

	_ = gs.CheckEnd()
}

// ayaraFirstOfLocthwainActivate handles Ayara's "{T}, Sacrifice another
// creature: Draw a card" ability (ability index 0).
func ayaraFirstOfLocthwainActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ayara_first_of_locthwain_sac_draw"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}

	// Ayara must be untapped to pay the {T} cost.
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// "Sacrifice another creature" — pick the lowest-value creature that
	// is not Ayara herself.
	victim := chooseSacVictimNotSelf(gs, seat, src, ctx)
	if victim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_other_creature", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	victimName := victim.Card.DisplayName()

	// Tap Ayara as part of the activation cost.
	src.Tapped = true

	// Sacrifice the chosen creature (fires creature_dies + observers).
	gameengine.SacrificePermanent(gs, victim, "ayara_first_of_locthwain_sac_draw")

	// Draw a card.
	drawn := drawOne(gs, seat, src.Card.DisplayName())
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"sacrificed": victimName,
		"drawn":      drawnName,
	})
}
