package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSecretsOfTheDead wires Secrets of the Dead.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever you cast a spell from your graveyard, draw a card.
//
// Why a per-card handler: the trigger parses to an inert `cast_trigger_tail`
// ("a spell from your graveyard, draw a card") — generic dispatch draws
// nothing. A graveyard/flashback/escape value engine; inert it does nothing.
//
// Implementation (CR §603 triggered ability):
//   - Listen on "spell_cast"; gate caster == controller AND the spell was
//     cast from the graveyard (ctx "cast_zone" == ZoneGraveyard — the engine
//     threads the source zone for exactly this kind of payoff).
//   - Draw one card via the shared DrawN primitive.
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerSecretsOfTheDead(Global())
	AddResetHook(registerSecretsOfTheDead)
}

func registerSecretsOfTheDead(r *Registry) {
	r.OnTrigger("Secrets of the Dead", "spell_cast", secretsOfTheDeadOnCast)
}

func secretsOfTheDeadOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "secrets_of_the_dead_graveyard_cast_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat != perm.Controller {
		return
	}
	zone, _ := ctx["cast_zone"].(string)
	if zone != "graveyard" {
		return
	}

	gameengine.DrawN(gs, perm.Controller, 1, perm)

	spellName, _ := ctx["spell_name"].(string)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"spell_name": spellName,
		"cast_zone":  zone,
	})
}
