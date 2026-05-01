package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBladewing wires Bladewing the Risen.
//
// Oracle text:
//
//	Flying
//	When Bladewing the Risen enters the battlefield, you may return
//	target Dragon permanent card from your graveyard to the battlefield.
//	{B}{R}: Dragon creatures get +1/+1 until end of turn.
//
// Implementation:
//   - OnETB: scan controller's graveyard for the highest-CMC Dragon
//     permanent card and return it via the full ETB cascade.
//   - OnActivated (idx 0, {B}{R}): every Dragon creature controller
//     controls gets +1/+1 until end of turn.
func registerBladewing(r *Registry) {
	r.OnETB("Bladewing the Risen", bladewingETB)
	r.OnActivated("Bladewing the Risen", bladewingActivated)
}

func bladewingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bladewing_dragon_reanimate"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Find the best Dragon permanent card in the graveyard. Prefer
	// highest-CMC creature Dragons (biggest reanimation impact); fall
	// back to any Dragon permanent (artifact/enchantment dragons exist
	// like Henge of Ramos? — extremely rare, but the text says
	// permanent, not creature).
	bestIdx := -1
	bestCMC := -1
	for i, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "dragon") {
			continue
		}
		// Permanent card: creature, artifact, enchantment, planeswalker, land, battle.
		isPerm := cardHasType(c, "creature") || cardHasType(c, "artifact") ||
			cardHasType(c, "enchantment") || cardHasType(c, "planeswalker") ||
			cardHasType(c, "land") || cardHasType(c, "battle")
		if !isPerm {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_dragon_in_graveyard", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	card := s.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, seat, "graveyard", "battlefield", "bladewing_etb")
	enterBattlefieldWithETB(gs, seat, card, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"returned": card.DisplayName(),
		"cmc":      bestCMC,
	})
}

func bladewingActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bladewing_dragon_anthem"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	ts := gs.NextTimestamp()
	count := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "dragon") {
			continue
		}
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     1,
			Toughness: 1,
			Duration:  "until_end_of_turn",
			Timestamp: ts,
		})
		count++
	}
	if count > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"dragons_buffed": count,
	})
}
