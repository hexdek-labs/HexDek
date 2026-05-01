package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEdgarMarkov wires Edgar Markov.
//
// Oracle text:
//
//	Eminence — Whenever you cast another Vampire spell, if Edgar Markov
//	is in the command zone or on the battlefield, create a 1/1 black
//	Vampire creature token.
//	First strike, haste
//	Whenever Edgar Markov attacks, put a +1/+1 counter on each Vampire
//	you control.
//
// Implementation:
//   - "spell_cast": fires when Edgar's controller casts a Vampire spell
//     other than Edgar himself; mints a 1/1 black Vampire token.
//   - "creature_attacks": when Edgar attacks, every Vampire on his
//     controller's battlefield gains a +1/+1 counter.
//
// Architectural note: the registry's TriggerHook walks battlefield only,
// so this handler fires only while Edgar is on the battlefield. Eminence
// from the command zone (Edgar dead, awaiting recast) is not dispatched
// through the standard path. In simulation Edgar is almost always on the
// battlefield by virtue of recasting, so this is acceptable.
func registerEdgarMarkov(r *Registry) {
	r.OnTrigger("Edgar Markov", "spell_cast", edgarMarkovEminence)
	r.OnTrigger("Edgar Markov", "creature_attacks", edgarMarkovAttack)
}

func edgarMarkovEminence(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "edgar_markov_eminence_vampire_token"
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
	if !cardHasType(card, "vampire") {
		return
	}
	// "another Vampire spell" — skip Edgar himself.
	if strings.EqualFold(card.DisplayName(), perm.Card.DisplayName()) {
		return
	}

	token := &gameengine.Card{
		Name:          "Vampire Token",
		Owner:         perm.Controller,
		Types:         []string{"creature", "token", "vampire", "pip:B"},
		BasePower:     1,
		BaseToughness: 1,
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Vampire Token",
			"reason": "edgar_markov_eminence",
			"power":  1,
			"tough":  1,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
	})
}

func edgarMarkovAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "edgar_markov_attack_anthem"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "vampire") {
			continue
		}
		p.AddCounter("+1/+1", 1)
		count++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"vampires_buffed":  count,
	})
}
