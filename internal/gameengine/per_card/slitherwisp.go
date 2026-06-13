package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSlitherwisp wires Slitherwisp.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Flash
//	Whenever you cast another spell that has flash, you draw a card and each
//	opponent loses 1 life.
//
// Why a per-card handler: the trigger parses to an inert
// `parsed_effect_residual` ("another spell that has flash, you draw a card
// and each opponent loses 1 life") — generic dispatch neither draws nor
// drains. Slitherwisp is a popular mono-blue flash commander; inert its whole
// payoff is gone. (Flash itself is an engine keyword, not inert.)
//
// Implementation (CR §603 triggered ability):
//   - Listen on "spell_cast"; gate caster == controller.
//   - The cast spell must HAVE flash (cardHasKeyword). The "another" clause is
//     satisfied automatically: Slitherwisp is already a battlefield permanent
//     when it triggers, so the spell being cast is a different object.
//   - Draw one (DrawN), then each opponent loses 1 life.
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerSlitherwisp(Global())
	AddResetHook(registerSlitherwisp)
}

func registerSlitherwisp(r *Registry) {
	r.OnTrigger("Slitherwisp", "spell_cast", slitherwispOnCast)
}

func slitherwispOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "slitherwisp_flash_cast_draw_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat != perm.Controller {
		return
	}
	c, _ := ctx["card"].(*gameengine.Card)
	if c == nil || !cardHasKeyword(c, "flash") {
		return
	}

	gameengine.DrawN(gs, perm.Controller, 1, perm)

	drained := 0
	for _, opp := range gs.Opponents(perm.Controller) {
		if gs.Seats[opp] == nil || gs.Seats[opp].Lost {
			continue
		}
		gameengine.LoseLife(gs, opp, 1, perm.Card.DisplayName())
		drained++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"spell_name":   c.DisplayName(),
		"opps_drained": drained,
	})
}
