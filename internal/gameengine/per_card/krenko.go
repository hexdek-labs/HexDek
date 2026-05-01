package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKrenkoMobBoss wires Krenko, Mob Boss.
//
// Oracle text:
//
//	{T}: Create X 1/1 red Goblin creature tokens, where X is the number
//	of Goblins you control.
//
// Doubling each turn (1 → 2 → 4 → 8 → 16…) is what makes Krenko a
// proven 4-turn lethal commander. The handler simply counts Goblins on
// Krenko's controller's battlefield (Krenko himself is a Goblin and
// counts) and mints that many 1/1 red Goblin tokens via the standard
// ETB cascade — Anointed Procession / Parallel Lives etc. trigger off
// `token_created` and double the count naturally.
func registerKrenkoMobBoss(r *Registry) {
	r.OnActivated("Krenko, Mob Boss", krenkoActivate)
}

func krenkoActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "krenko_goblin_tokens"
	if gs == nil || src == nil || src.Card == nil {
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

	x := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "goblin") {
			x++
		}
	}
	if x <= 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_goblins", map[string]interface{}{
			"seat": seat,
		})
		return
	}

	for i := 0; i < x; i++ {
		token := &gameengine.Card{
			Name:          "Goblin Token",
			Owner:         seat,
			Types:         []string{"creature", "token", "goblin", "pip:R"},
			Colors:        []string{"R"},
			BasePower:     1,
			BaseToughness: 1,
		}
		enterBattlefieldWithETB(gs, seat, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   seat,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Goblin Token",
				"reason": "krenko_mob_boss",
				"power":  1,
				"tough":  1,
			},
		})
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"goblins_before": x,
		"tokens_created": x,
	})
	_ = gs.CheckEnd()
}
