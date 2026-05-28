package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTalrand wires Talrand, Sky Summoner.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Talrand%2C%20Sky%20Summoner):
//
//	Whenever you cast an instant or sorcery spell, create a 2/2 blue
//	Drake creature token with flying.
//
// {2}{U}{U} 2/2 Legendary Creature — Merfolk Wizard. The canonical
// Spellslinger commander — every cantrip / counter / removal spell
// stacks a 2/2 flier on the board. Pairs with Brainstorm-style
// 1-mana draws to grow an air force while still developing the
// gameplan.
//
// Implementation:
//   - OnTrigger("instant_or_sorcery_cast"). Subscribing to the
//     pre-aliased event (stack.go:1890) is tighter than reading
//     spell_cast + filtering on type — the engine already does the
//     instant/sorcery gate before firing this event.
//   - Filter: caster_seat == perm.Controller ("Whenever YOU cast").
//   - Action: create a 2/2 blue flying Drake token.
func registerTalrand(r *Registry) {
	r.OnTrigger("Talrand, Sky Summoner", "instant_or_sorcery_cast", talrandOnCast)
}

func talrandOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "talrand_sky_summoner"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	token := &gameengine.Card{
		Name:          "Drake Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "drake", "flying", "pip:U"},
		Colors:        []string{"U"},
		BasePower:     2,
		BaseToughness: 2,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Drake Token",
			"reason": "talrand_sky_summoner",
			"power":  2,
			"tough":  2,
			"flying": true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
