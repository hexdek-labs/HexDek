package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJhoiraWeatherlightCaptain wires Jhoira, Weatherlight Captain.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Human Artificer. 3/3.
//	Whenever you cast a historic spell, draw a card. (Artifacts,
//	legendaries, and Sagas are historic.)
//
// The Esper-blue artifact-historic engine. Every artifact / legendary /
// Saga cast becomes a cantrip, which makes Jhoira the canonical
// "spread-the-table" artifact value commander.
//
// Implementation:
//   - OnTrigger("spell_cast") gated on caster_seat == perm.Controller and
//     the cast card being historic. The "another" qualifier is absent in
//     the printed text, so Jhoira's own cast (if the card resolves a
//     trigger from the battlefield, which she can't — she's on the stack)
//     is technically eligible; in practice this never matters because
//     Jhoira isn't on the battlefield when she herself is being cast.
func registerJhoiraWeatherlightCaptain(r *Registry) {
	r.OnTrigger("Jhoira, Weatherlight Captain", "spell_cast", jhoiraHistoricCast)
}

func jhoiraHistoricCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jhoira_weatherlight_captain_historic_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardIsHistoric(card) {
		return
	}

	drewName := ""
	if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c != nil {
		drewName = c.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
		"drew":       drewName != "",
	})
}

// cardIsHistoric reports whether a card is "historic" per CR §700.10:
// historic = artifact, legendary, or Saga. Sagas are tagged with the
// "saga" subtype; we accept both lowercase "saga" and a raw type-line
// substring as a defensive backstop for cards loaded without explicit
// subtype tagging.
func cardIsHistoric(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if cardHasType(c, "artifact") {
		return true
	}
	if cardHasType(c, "legendary") {
		return true
	}
	if cardHasType(c, "saga") {
		return true
	}
	if c.TypeLine != "" && strings.Contains(strings.ToLower(c.TypeLine), "saga") {
		return true
	}
	return false
}
