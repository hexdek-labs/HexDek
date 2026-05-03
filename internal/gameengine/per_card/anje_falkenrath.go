package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnjeFalkenrath wires Anje Falkenrath.
//
// Oracle text (Scryfall, verified):
//
//	Haste.
//	{T}, Discard a card: Draw a card.
//	Whenever you discard a card, if it has madness, untap Anje Falkenrath.
//
// Haste is granted by the AST keyword pipeline; this handler does not
// implement it.
//
// Implementation:
//   - OnActivated(0, ...): the "{T}, Discard a card: Draw a card" loot.
//     Anje must be untapped; tap her, discard the lowest-CMC non-madness
//     card from hand (preserving madness cards so they can be cast via
//     their madness cost), then draw one card. If the controller's hand
//     is empty the activation is a no-op.
//   - OnTrigger("card_discarded"): whenever the controller discards a
//     card, if that card's oracle text contains "madness", untap Anje.
//     This enables the classic Anje madness-loot loop — each madness
//     discard untaps her for another activation.
func registerAnjeFalkenrath(r *Registry) {
	r.OnActivated("Anje Falkenrath", anjeFalkenrathActivate)
	r.OnTrigger("Anje Falkenrath", "card_discarded", anjeFalkenrathDiscardTrigger)
}

// anjeFalkenrathActivate handles Anje's "{T}, Discard a card: Draw a card"
// loot ability (ability index 0).
func anjeFalkenrathActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "anje_falkenrath_loot"
	if gs == nil || src == nil {
		return
	}
	// Only ability index 0 is defined.
	if abilityIdx != 0 {
		return
	}

	// Anje must be untapped to activate this ability.
	if src.Tapped {
		emitFail(gs, slug, "Anje Falkenrath", "already_tapped", nil)
		return
	}

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Hand must have at least one card to discard.
	if len(s.Hand) == 0 {
		emitFail(gs, slug, "Anje Falkenrath", "no_hand_to_discard", nil)
		return
	}

	// Tap Anje as part of the activation cost.
	src.Tapped = true

	// Pick the card to discard: prefer the lowest-CMC card that does NOT
	// have madness (preserve madness cards for activation loops). If every
	// card in hand has madness, fall back to the lowest-CMC card overall.
	pick := anjeFalkenrathPickDiscard(s.Hand)
	pickName := pick.DisplayName()

	gameengine.DiscardCard(gs, pick, seat)

	drawn := drawOne(gs, seat, "Anje Falkenrath")
	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}

	emit(gs, slug, "Anje Falkenrath", map[string]interface{}{
		"seat":      seat,
		"discarded": pickName,
		"drawn":     drawnName,
	})
}

// anjeFalkenrathDiscardTrigger fires whenever the controller discards a
// card. If the discarded card has madness in its oracle text, untap Anje.
func anjeFalkenrathDiscardTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anje_falkenrath_untap_on_madness"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Only fire for Anje's controller's discards.
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat != perm.Controller {
		return
	}

	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	// Check whether the discarded card has madness.
	oracleText := gameengine.OracleTextLower(card)
	if !strings.Contains(oracleText, "madness") {
		return
	}

	// Untap Anje.
	perm.Tapped = false

	emit(gs, slug, "Anje Falkenrath", map[string]interface{}{
		"seat":          perm.Controller,
		"madness_card":  card.DisplayName(),
		"untapped":      true,
	})
}

// anjeFalkenrathPickDiscard selects the card to discard for Anje's loot
// activation. Strategy:
//  1. Lowest-CMC card without madness — fodder, keeps madness cards for
//     the untap loop.
//  2. If every card in hand has madness (all are valuable loop pieces),
//     fall back to the lowest-CMC card overall — still useful because
//     discarding a madness card untaps Anje, enabling another activation.
//  3. If CMC is equal, take the first found (stable, deterministic).
func anjeFalkenrathPickDiscard(hand []*gameengine.Card) *gameengine.Card {
	// Pass 1: lowest-CMC non-madness card.
	var best *gameengine.Card
	bestCMC := 1<<31 - 1
	for _, c := range hand {
		if c == nil {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		if strings.Contains(ot, "madness") {
			continue
		}
		cmc := cardCMC(c)
		if cmc < bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best != nil {
		return best
	}

	// Pass 2: all cards have madness — lowest-CMC overall.
	bestCMC = 1<<31 - 1
	for _, c := range hand {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if cmc < bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best != nil {
		return best
	}

	// Final fallback: first non-nil card.
	for _, c := range hand {
		if c != nil {
			return c
		}
	}
	return nil
}
