package per_card

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMonologueTax wires Monologue Tax.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever an opponent casts their second spell each turn, you create a
//	Treasure token.
//
// Why a per-card handler: the gameast parser collapses the trigger to an
// inert `untyped_effect` node whose only arg is the mangled raw string
// "each turn, you create a treasure token" — the "opponent's second spell"
// condition is lost entirely, so the generic dispatch produces NOTHING.
// (Confirmed: every Modification node on this card is a scaffold kind and no
// prior handler referenced it.) Monologue Tax is a popular Smothering-Tithe-
// adjacent treasure/stax payoff, so leaving it inert silently neuters the
// deck's mana engine.
//
// Self-registers via init() + AddResetHook so it survives test-driven
// Reset() without touching the shared registry.go default table.
//
// Implementation (CR §603 triggered ability):
//   - Listen on "spell_cast_by_opponent". The event fires for every cast;
//     we gate on casterSeat != controller (it IS an opponent's spell).
//   - Track a per-opponent spell tally for the current turn in the Monologue
//     Tax permanent's Flags, resetting whenever gs.Turn advances. When an
//     opponent's tally reaches EXACTLY 2 (their second spell this turn), mint
//     one Treasure for Monologue Tax's controller.
//   - Counting beyond 2 is intentionally ignored — the ability only triggers
//     on the second spell, not every subsequent spell.
func init() {
	registerMonologueTax(Global())
	AddResetHook(registerMonologueTax)
}

func registerMonologueTax(r *Registry) {
	r.OnTrigger("Monologue Tax", "spell_cast_by_opponent", monologueTaxOnOpponentCast)
}

func monologueTaxOnOpponentCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "monologue_tax_second_spell_treasure"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	// Only an opponent's spell counts.
	if casterSeat == perm.Controller {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	// Reset the per-opponent tally at each new turn.
	if perm.Flags["monologue_turn_stamp"] != gs.Turn {
		for k := range perm.Flags {
			if strings.HasPrefix(k, "monologue_seat_") {
				delete(perm.Flags, k)
			}
		}
		perm.Flags["monologue_turn_stamp"] = gs.Turn
	}

	key := "monologue_seat_" + strconv.Itoa(casterSeat)
	perm.Flags[key]++
	// Trigger fires only on the opponent's SECOND spell this turn.
	if perm.Flags[key] != 2 {
		return
	}

	gameengine.CreateTreasureToken(gs, perm.Controller)

	spellName, _ := ctx["spell_name"].(string)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"opponent":    casterSeat,
		"spell_name":  spellName,
		"token":       "Treasure",
		"second_cast": true,
	})
}
