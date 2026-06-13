package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUro wires Uro, Titan of Nature's Wrath.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Uro%2C%20Titan%20of%20Nature%27s%20Wrath):
//
//	{G}{U}
//	Legendary Creature — Elder Giant
//	6/6
//	When Uro, Titan of Nature's Wrath enters, sacrifice it unless it
//	escaped. Whenever Uro enters or attacks, you gain 3 life and draw
//	a card, then you may put a land card from your hand onto the
//	battlefield.
//	Escape — {G}{G}{U}{U}, Exile five other cards from your graveyard.
//
// Implementation:
//   - OnETB fires both clauses: gain-3-draw-land + sacrifice-unless-escaped.
//     Order matters per APNAP-controller-orders rules; we fire the
//     value clause first (gain 3 / draw / extra land) so it resolves
//     before Uro leaves on the sacrifice. Both triggers go on the stack
//     simultaneously per CR §603.3b; the controller chooses order, and
//     value-first is strictly better.
//   - OnTrigger("creature_attacks") gated on attacker_perm == self:
//     re-fire the value clause (gain 3 / draw / extra land).
//   - Escape cost/exile handled by the generic escape machinery.
func registerUro(r *Registry) {
	r.OnETB("Uro, Titan of Nature's Wrath", uroETB)
	r.OnTrigger("Uro, Titan of Nature's Wrath", "creature_attacks", uroAttack)
	// uroETB fully implements the printed self-ETB (gain-3 / draw / land,
	// then sacrifice-unless-escaped) — declare ownership so the engine's
	// generic AST push for the "sacrifice it unless it escaped" clause is
	// suppressed (Judge r63 double-fire gate; PROGRESSION r63 false positive).
	r.OwnsETBTrigger("Uro, Titan of Nature's Wrath")
}

func uroETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "uro_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// Value clause: gain 3 / draw 1 / extra land.
	uroValueClause(gs, perm)

	// Sacrifice-unless-escaped.
	escapedCast := perm.Flags != nil && perm.Flags["escape_cast"] == 1
	if !escapedCast {
		gameengine.SacrificePermanent(gs, perm, slug)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"escaped":    escapedCast,
		"sacrificed": !escapedCast,
	})
}

func uroAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "uro_attack_value"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	uroValueClause(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

// uroValueClause runs the "you gain 3 life, draw a card, then you may
// put a land card from your hand onto the battlefield" effect. Hat
// policy: always opt yes on the land placement — drops a land for free.
func uroValueClause(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "uro_value_clause"
	seatIdx := perm.Controller
	gameengine.GainLife(gs, seatIdx, 3, perm.Card.DisplayName())

	seat := gs.Seats[seatIdx]
	drew := false
	if seat != nil && !seat.Lost && len(seat.Library) > 0 {
		c := seat.Library[0]
		gameengine.MoveCard(gs, c, seatIdx, "library", "hand", slug)
		drew = true
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"reason": slug,
			},
		})
	}

	// Find a land in hand and put it onto the battlefield.
	landPlayed := ""
	if seat != nil {
		for _, c := range seat.Hand {
			if c == nil {
				continue
			}
			if cardHasType(c, "land") {
				gameengine.MoveCard(gs, c, seatIdx, "hand", "battlefield", slug)
				landPlayed = c.DisplayName()
				break
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seatIdx,
		"life_gained": 3,
		"drew":        drew,
		"land_played": landPlayed,
	})
}
