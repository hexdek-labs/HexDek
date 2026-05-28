package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKrenkoTinStreetKingpin wires Krenko, Tin Street Kingpin.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Krenko%2C%20Tin%20Street%20Kingpin):
//
//	Whenever Krenko, Tin Street Kingpin attacks, put X +1/+1 counters
//	on it and create X 1/1 red Goblin creature tokens that are tapped
//	and attacking, where X is the number of opponents you have.
//
// {1}{R} 1/2 Legendary Creature — Goblin Rogue. Aggressive 2-drop
// Goblin commander. In a 4-player Commander game X = 3 each combat,
// so Krenko grows by 3 +1/+1 counters per swing and conjures 3
// tapped+attacking Goblins. The +1/+1 anthem on himself scales the
// next combat's tokens too if Krenko survives.
//
// Implementation:
//   - OnTrigger("creature_attacks"). Filter: attacker_perm == self
//     (mirrors the Edgar Markov attack pattern at edgar_markov.go:88).
//   - X = len(gs.Opponents(controller)). Skip when X==0 (1-player
//     fixture or all opponents lost — defensive).
//   - Add X +1/+1 counters via AddCounter; create X Goblin tokens via
//     enterBattlefieldWithETB and set Tapped=true on the resulting
//     permanents so the "tapped and attacking" flavor at least
//     manifests as tapped. The "attacking" status is fixture-only —
//     the engine's declared-attackers slice for this combat is already
//     locked by the time this trigger resolves, so the new tokens
//     can't legally be retro-added; treating them as tapped matches
//     the visible game state.
func registerKrenkoTinStreetKingpin(r *Registry) {
	r.OnTrigger("Krenko, Tin Street Kingpin", "creature_attacks", krenkoTinStreetAttack)
}

func krenkoTinStreetAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "krenko_tin_street_kingpin_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	x := len(gs.Opponents(perm.Controller))
	if x <= 0 {
		return
	}

	perm.AddCounter("+1/+1", x)

	for i := 0; i < x; i++ {
		token := &gameengine.Card{
			Name:          "Goblin Token",
			Owner:         perm.Controller,
			Types:         []string{"creature", "token", "goblin", "pip:R"},
			Colors:        []string{"R"},
			BasePower:     1,
			BaseToughness: 1,
		}
		tp := enterBattlefieldWithETB(gs, perm.Controller, token, false)
		if tp != nil {
			tp.Tapped = true
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Goblin Token",
				"reason": "krenko_tin_street_kingpin",
				"power":  1,
				"tough":  1,
				"tapped": true,
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"x":              x,
		"counters_added": x,
		"tokens_created": x,
	})
}
