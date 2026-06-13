package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWavebreakHippocamp wires Wavebreak Hippocamp.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever you cast your first spell during each opponent's turn, draw a
//	card.
//
// Why a per-card handler: the trigger parses to an inert `ordinal_trigger_tail`
// ("your first spell during each opponent's turn, draw a card") — generic
// dispatch draws nothing. A cEDH stax/tempo staple; inert it does nothing.
//
// Implementation (CR §603, mirrors Alela's first-spell-per-opponent-turn gate):
//   - Listen on "spell_cast"; gate caster == controller.
//   - Only during an opponent's turn (gs.Active != controller).
//   - Only the FIRST such spell each turn (per-turn flag stamp).
//   - Draw one card via the shared DrawN primitive (full §614.11 fidelity:
//     draw doublers, observers, draw counter).
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerWavebreakHippocamp(Global())
	AddResetHook(registerWavebreakHippocamp)
}

func registerWavebreakHippocamp(r *Registry) {
	r.OnTrigger("Wavebreak Hippocamp", "spell_cast", wavebreakHippocampOnCast)
}

func wavebreakHippocampOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wavebreak_hippocamp_first_spell_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat != perm.Controller {
		return
	}
	// Only during an opponent's turn.
	if gs.Active == perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// First spell each turn only (stamp the turn number when we fire).
	if perm.Flags["wavebreak_cast_turn"] == gs.Turn {
		return
	}
	perm.Flags["wavebreak_cast_turn"] = gs.Turn

	gameengine.DrawN(gs, perm.Controller, 1, perm)

	spellName, _ := ctx["spell_name"].(string)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"active":     gs.Active,
		"spell_name": spellName,
	})
}
