package per_card

import (
	"math/rand"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNivMizzetReborn wires Niv-Mizzet Reborn.
//
// Oracle text (Scryfall, WAR — verified 2026-04-30):
//
//	Flying
//	When Niv-Mizzet enters, reveal the top ten cards of your library.
//	For each color pair, choose a card that's exactly those colors from
//	among them. Put the chosen cards into your hand and the rest on the
//	bottom of your library in a random order.
//
// Implementation (ETB):
//   - Take the top 10 (or fewer) cards from the library.
//   - For each of the 10 guild pairs, scan revealed cards in order and
//     pick the first one whose Colors set is exactly that pair. A
//     revealed card can satisfy at most one pair (greedy by guild
//     order — Niv-Mizzet itself doesn't disambiguate, so any
//     deterministic order is acceptable in simulation).
//   - Shuffle the remainder and append to the bottom of the library.
//
// Card.Colors is the runtime color cache (state.go: populated by the
// corpus loader from the top-level "colors" JSON field). Cards with
// unknown colors (e.g. test fixtures without that cache populated)
// simply won't match any guild pair.
func registerNivMizzetReborn(r *Registry) {
	r.OnETB("Niv-Mizzet Reborn", nivMizzetRebornETB)
}

// The 10 two-color guild pairs. Order is conventional WUBRG-rotation;
// the actual order doesn't matter since each card can match at most one
// pair (set membership is exact).
var nivMizzetGuildPairs = [][2]string{
	{"W", "U"}, // Azorius
	{"U", "B"}, // Dimir
	{"B", "R"}, // Rakdos
	{"R", "G"}, // Gruul
	{"G", "W"}, // Selesnya
	{"W", "B"}, // Orzhov
	{"U", "R"}, // Izzet
	{"B", "G"}, // Golgari
	{"R", "W"}, // Boros
	{"G", "U"}, // Simic
}

func nivMizzetRebornETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "niv_mizzet_reborn_reveal_ten"
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
	var pulledPairs []string
	for _, pair := range nivMizzetGuildPairs {
		for i, c := range revealed {
			if taken[i] || c == nil {
				continue
			}
			if !cardHasExactlyColors(c, pair[0], pair[1]) {
				continue
			}
			s.Hand = append(s.Hand, c)
			gs.LogEvent(gameengine.Event{
				Kind:   "draw",
				Seat:   seat,
				Source: perm.Card.DisplayName(),
				Details: map[string]interface{}{
					"slug":   slug,
					"reason": "niv_mizzet_guild_pair_pick",
					"card":   c.DisplayName(),
					"pair":   pair[0] + pair[1],
				},
			})
			pulled = append(pulled, c.DisplayName())
			pulledPairs = append(pulledPairs, pair[0]+pair[1])
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
		"seat":           seat,
		"revealed":       n,
		"taken_count":    len(pulled),
		"taken_cards":    pulled,
		"taken_pairs":    pulledPairs,
		"bottomed_count": len(remainder),
	})
}

// cardHasExactlyColors returns true if the card's Colors set equals
// exactly {a, b}. Colorless cards, monocolor cards, and cards with 3+
// colors all return false. Case-insensitive on the WUBRG letter.
func cardHasExactlyColors(c *gameengine.Card, a, b string) bool {
	if c == nil || len(c.Colors) != 2 {
		return false
	}
	got := map[string]bool{
		strings.ToUpper(c.Colors[0]): true,
		strings.ToUpper(c.Colors[1]): true,
	}
	if len(got) != 2 {
		// Defensive: c.Colors had a duplicate entry.
		return false
	}
	return got[strings.ToUpper(a)] && got[strings.ToUpper(b)]
}
