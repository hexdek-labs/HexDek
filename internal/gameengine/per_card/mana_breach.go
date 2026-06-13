package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerManaBreach wires Mana Breach.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever a player casts a spell, that player returns a land they control
//	to its owner's hand.
//
// Why a per-card handler: the parser dumps the whole effect clause into an
// inert `parsed_effect_residual` node ("that player returns a land they
// control to its owner's hand") with no structured bounce, so the generic
// dispatch does NOTHING — the symmetric land-tax soft-lock never fires.
//
// Implementation (CR §603 triggered ability, symmetric):
//   - Listen on "spell_cast". The event fires for EVERY cast by any player,
//     including Mana Breach's own controller — the ability is symmetric.
//   - The casting player returns one of THEIR lands to its owner's hand. The
//     choice belongs to that player; the AI heuristic minimizes self-harm by
//     preferring an already-tapped basic land (least tempo lost), falling
//     back to tapped nonbasic, then any land.
//   - Bounce via the canonical BouncePermanent (handles auras/triggers/
//     commander redirect), sourced from the Mana Breach permanent.
//
// Self-registers via init() + AddResetHook so it survives test-driven
// Reset() without editing the shared registry.go default table.
func init() {
	registerManaBreach(Global())
	AddResetHook(registerManaBreach)
}

func registerManaBreach(r *Registry) {
	r.OnTrigger("Mana Breach", "spell_cast", manaBreachOnSpellCast)
}

func manaBreachOnSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mana_breach_return_land"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[casterSeat]
	if seat == nil {
		return
	}

	land := manaBreachPickLand(seat.Battlefield)
	if land == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_land_to_return", map[string]interface{}{
			"caster_seat": casterSeat,
		})
		return
	}

	landName := land.Card.DisplayName()
	if !gameengine.BouncePermanent(gs, land, perm, "hand") {
		emitFail(gs, slug, perm.Card.DisplayName(), "bounce_failed", map[string]interface{}{
			"caster_seat": casterSeat,
			"land":        landName,
		})
		return
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"caster_seat": casterSeat,
		"returned":    landName,
	})
}

// manaBreachPickLand chooses which land the casting player returns. Heuristic
// (the player minimizes their own tempo loss): prefer a tapped land, and
// among ties prefer a basic. Returns nil when the player controls no land.
func manaBreachPickLand(battlefield []*gameengine.Permanent) *gameengine.Permanent {
	var best *gameengine.Permanent
	bestScore := 1 << 30
	for _, p := range battlefield {
		if p == nil || p.Card == nil || !p.IsLand() {
			continue
		}
		// Lower score = more expendable. Tapped is cheaper to lose than
		// untapped; basic cheaper than nonbasic.
		score := 0
		if !p.Tapped {
			score += 2
		}
		if !strings.Contains(strings.ToLower(p.Card.TypeLine), "basic") {
			score += 1
		}
		if score < bestScore {
			bestScore = score
			best = p
		}
	}
	return best
}
