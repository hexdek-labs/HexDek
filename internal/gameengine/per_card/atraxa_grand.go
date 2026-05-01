package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAtraxaGrandUnifier wires Atraxa, Grand Unifier.
//
// Oracle text:
//
//	Flying, vigilance, deathtouch, lifelink
//	When Atraxa, Grand Unifier enters the battlefield, reveal the top
//	ten cards of your library. For each card type among those cards,
//	you may put a card of that type from among them into your hand.
//	Put the rest on the bottom of your library in a random order.
//
// Implementation (ETB):
//   - Take the top 10 (or fewer) cards from the library.
//   - For each of the eight card types (creature, instant, sorcery,
//     artifact, enchantment, planeswalker, land, battle), pick the
//     first revealed card of that type and route to hand. A revealed
//     card with multiple types only contributes to the first slot it
//     fills (greedy by the order above), but other cards may still
//     satisfy the remaining type buckets.
//   - Shuffle the remainder and append to the bottom of the library.
func registerAtraxaGrandUnifier(r *Registry) {
	r.OnETB("Atraxa, Grand Unifier", atraxaGrandUnifierETB)
}

var atraxaCardTypes = []string{
	"creature",
	"instant",
	"sorcery",
	"artifact",
	"enchantment",
	"planeswalker",
	"land",
	"battle",
}

func atraxaGrandUnifierETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "atraxa_grand_unifier_reveal_ten"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	n := 10
	if len(s.Library) < n {
		n = len(s.Library)
	}
	if n == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"revealed": 0,
		})
		return
	}

	revealed := make([]*gameengine.Card, n)
	copy(revealed, s.Library[:n])
	s.Library = s.Library[n:]

	taken := make(map[int]bool)
	var pulled []string
	for _, t := range atraxaCardTypes {
		for i, c := range revealed {
			if taken[i] || c == nil {
				continue
			}
			if !cardHasType(c, t) {
				continue
			}
			gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   seat,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug":      slug,
					"reason":    "atraxa_reveal_pick",
					"card":      c.DisplayName(),
					"card_type": t,
				},
			})
			pulled = append(pulled, c.DisplayName())
			taken[i] = true
			break
		}
	}

	var remainder []*gameengine.Card
	for i, c := range revealed {
		if taken[i] || c == nil {
			continue
		}
		remainder = append(remainder, c)
	}
	rand.Shuffle(len(remainder), func(i, j int) {
		remainder[i], remainder[j] = remainder[j], remainder[i]
	})
	s.Library = append(s.Library, remainder...)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"revealed":         n,
		"taken_count":      len(pulled),
		"taken_cards":      pulled,
		"bottomed_count":   len(remainder),
	})
}
