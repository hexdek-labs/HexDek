package per_card

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMangaraTheDiplomat wires Mangara, the Diplomat.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Lifelink
//	Whenever an opponent attacks with creatures, if two or more of those
//	creatures are attacking you and/or planeswalkers you control, draw a card.
//	Whenever an opponent casts their second spell each turn, draw a card.
//
// Why a per-card handler: the second-spell clause parses to the same inert
// `untyped_effect` scaffold as Monologue Tax ("each turn, draw a card" — the
// "opponent's second spell" condition is stripped), so the generic dispatch
// produces NOTHING. Mangara is a premier white card-advantage engine; leaving
// it inert silently removes its whole draw engine.
//
// Scope of THIS handler:
//   - Implements the "opponent's second spell each turn → draw a card" clause
//     (CR §603), mirroring the proven Monologue Tax tally: a per-opponent
//     spell count stored in the permanent's Flags, reset each turn, firing a
//     single draw when an opponent casts their second spell.
//   - Lifelink is a keyword handled by the engine's combat layer (not inert).
//   - The combat clause ("opponent attacks YOU/your planeswalkers with 2+
//     creatures → draw") is DEFERRED: the per-card "creature_attacks" trigger
//     ctx does not currently expose each attacker's chosen defender, so the
//     "attacking you and/or your planeswalkers" predicate cannot be evaluated
//     reliably. Logged as a deferred follow-up rather than implemented
//     half-correctly. This handler does not touch combat, so it cannot
//     regress existing behavior.
//
// Self-registers via init() + AddResetHook so it survives test-driven Reset()
// without editing the shared registry.go default table.
func init() {
	registerMangaraTheDiplomat(Global())
	AddResetHook(registerMangaraTheDiplomat)
}

func registerMangaraTheDiplomat(r *Registry) {
	r.OnTrigger("Mangara, the Diplomat", "spell_cast_by_opponent", mangaraDiplomatOnOpponentCast)
}

func mangaraDiplomatOnOpponentCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mangara_diplomat_second_spell_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	if casterSeat == perm.Controller {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["mangara_turn_stamp"] != gs.Turn {
		for k := range perm.Flags {
			if strings.HasPrefix(k, "mangara_seat_") {
				delete(perm.Flags, k)
			}
		}
		perm.Flags["mangara_turn_stamp"] = gs.Turn
	}

	key := "mangara_seat_" + strconv.Itoa(casterSeat)
	perm.Flags[key]++
	if perm.Flags[key] != 2 {
		return
	}

	drawOne(gs, perm.Controller, perm.Card.DisplayName())

	spellName, _ := ctx["spell_name"].(string)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"opponent":    casterSeat,
		"spell_name":  spellName,
		"second_cast": true,
	})
}
