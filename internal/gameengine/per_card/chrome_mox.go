package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChromeMox wires up Chrome Mox.
//
// Oracle text:
//
//   Imprint — When Chrome Mox enters the battlefield, you may exile a
//   nonartifact, nonland card from your hand.
//   {T}: Add one mana of any of the exiled card's colors.
//
// 0-mana artifact. Fast mana staple in cEDH. The imprint mechanic
// requires choosing a card to exile from hand on ETB; the activated
// ability then taps for one mana of any of that card's colors.
//
// Implementation:
//   - OnETB: exile the first nonartifact, nonland card from hand.
//     Store the exiled card's colors in perm.Flags for the tap ability.
//   - OnActivated: tap for one mana of the imprinted card's color.

func registerChromeMox(r *Registry) {
	r.OnETB("Chrome Mox", chromeMoxETB)
	r.OnActivated("Chrome Mox", chromeMoxActivated)
}

// chromeMoxImprintColor stores the first imprinted color on the
// permanent's Flags map. We use "imprint_color_W"/"imprint_color_U" etc.
// as flag keys (value 1 = imprinted with that color).

func chromeMoxETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "chrome_mox_imprint"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Find the first nonartifact, nonland card in hand to exile.
	exileIdx := -1
	for i, c := range s.Hand {
		if c == nil {
			continue
		}
		isArt := false
		isLand := false
		for _, t := range c.Types {
			tl := strings.ToLower(t)
			if tl == "artifact" {
				isArt = true
			}
			if tl == "land" {
				isLand = true
			}
		}
		if !isArt && !isLand {
			exileIdx = i
			break
		}
	}
	if exileIdx < 0 {
		// No eligible card to imprint — Chrome Mox can still be played
		// but won't produce mana.
		emit(gs, slug, "Chrome Mox", map[string]interface{}{
			"seat":      seat,
			"imprinted": false,
			"reason":    "no_eligible_card",
		})
		return
	}

	card := s.Hand[exileIdx]
	gameengine.MoveCard(gs, card, seat, "hand", "exile", "exile-from-hand")

	// Determine the exiled card's colors and stamp them on the permanent.
	colors := extractCardColors(card)
	if len(colors) == 0 {
		colors = []string{"C"} // colorless if no colors found
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["imprinted"] = 1
	for _, c := range colors {
		perm.Flags["imprint_color_"+strings.ToUpper(c)] = 1
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "exile",
		Seat:   seat,
		Source: "Chrome Mox",
		Details: map[string]interface{}{
			"exiled_card": card.DisplayName(),
			"reason":      "imprint",
			"colors":      colors,
		},
	})
	emit(gs, slug, "Chrome Mox", map[string]interface{}{
		"seat":      seat,
		"imprinted": true,
		"card":      card.DisplayName(),
		"colors":    colors,
	})
}

func chromeMoxActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "chrome_mox_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Chrome Mox", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	if src.Flags == nil || src.Flags["imprinted"] == 0 {
		emitFail(gs, slug, "Chrome Mox", "no_imprinted_card", nil)
		return
	}

	// Find the first imprinted color.
	color := ""
	for _, c := range []string{"B", "G", "R", "U", "W", "C"} {
		if src.Flags["imprint_color_"+c] > 0 {
			color = c
			break
		}
	}
	if color == "" {
		emitFail(gs, slug, "Chrome Mox", "no_imprinted_color", nil)
		return
	}

	src.Tapped = true
	gameengine.AddMana(gs, gs.Seats[seat], color, 1, "Chrome Mox")

	emit(gs, slug, "Chrome Mox", map[string]interface{}{
		"seat":  seat,
		"color": color,
	})
}

// extractCardColors returns the color identities from a card's Colors
// slice or from pip: type markers.
func extractCardColors(c *gameengine.Card) []string {
	if c == nil {
		return nil
	}
	// Prefer the Colors slice if populated.
	if len(c.Colors) > 0 {
		return append([]string{}, c.Colors...)
	}
	// Fall back to pip: markers in Types.
	var colors []string
	seen := map[string]bool{}
	for _, t := range c.Types {
		if strings.HasPrefix(t, "pip:") {
			color := strings.ToUpper(strings.TrimPrefix(t, "pip:"))
			if !seen[color] {
				colors = append(colors, color)
				seen[color] = true
			}
		}
	}
	return colors
}
