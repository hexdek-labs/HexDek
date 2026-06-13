package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ire_of_kaminari_r63.go — per_card handler for Ire of Kaminari.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Ire of Kaminari deals damage to any target equal to the number of
//	Arcane cards in your graveyard.
//
// {2}{R} Instant — Arcane. A graveyard-Arcane burn payoff. Parses to an
// inert fallback node; the runtime text fallback has no "damage equal to
// Arcane cards in your graveyard" shape — fully DOA.
//
// Implementation: OnResolve (authoritative). Count Arcane cards in the
// caster's graveyard (the Arcane subtype appears on the printed type
// line), deal that much to the highest-life opponent (the player pick for
// an unstamped "any target").
func init() {
	registerIreOfKaminariR63(Global())
	AddResetHook(registerIreOfKaminariR63)
}

func registerIreOfKaminariR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Ire of Kaminari", ireOfKaminariResolve)
}

func ireOfKaminariResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "ire_of_kaminari"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	arcane := 0
	for _, c := range gs.Seats[seat].Graveyard {
		if c == nil {
			continue
		}
		if cardHasSubtypeArcane(c) {
			arcane++
		}
	}

	target := highestLifeOpponent(gs, seat)
	if target >= 0 && arcane > 0 {
		gameengine.DealDamage(gs, target, arcane, "Ire of Kaminari")
	}
	emit(gs, slug, "Ire of Kaminari", map[string]interface{}{
		"seat": seat, "arcane": arcane, "target": target,
	})
}

// cardHasSubtypeArcane reports whether a card is Arcane (subtype on the
// printed type line, e.g. "Instant — Arcane"). Types is the canonical
// source but Arcane is a subtype tracked on the type line.
func cardHasSubtypeArcane(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "arcane") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(c.TypeLine), "arcane")
}
