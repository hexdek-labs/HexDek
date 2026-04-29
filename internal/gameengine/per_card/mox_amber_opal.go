package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Mox Amber
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   {T}: Add one mana of any color among legendary creatures and
//   planeswalkers you control.
//
// 0-mana legendary artifact. Conditional fast mana — requires a
// legendary creature or planeswalker to produce mana.

func registerMoxAmber(r *Registry) {
	r.OnActivated("Mox Amber", moxAmberActivated)
}

func moxAmberActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mox_amber_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Mox Amber", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Find colors among legendary creatures and planeswalkers you control.
	availableColors := map[string]bool{}
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsLegendary() {
			continue
		}
		if !p.IsCreature() && !p.IsPlaneswalker() {
			continue
		}
		for _, c := range p.Card.Colors {
			availableColors[strings.ToUpper(c)] = true
		}
		// Also check pip markers.
		for _, t := range p.Card.Types {
			if strings.HasPrefix(t, "pip:") {
				availableColors[strings.ToUpper(strings.TrimPrefix(t, "pip:"))] = true
			}
		}
	}

	if len(availableColors) == 0 {
		emitFail(gs, slug, "Mox Amber", "no_legendary_creature_or_planeswalker", nil)
		return
	}

	src.Tapped = true
	// Pick the first available color (alphabetical for determinism).
	color := "C"
	for _, c := range []string{"B", "G", "R", "U", "W"} {
		if availableColors[c] {
			color = c
			break
		}
	}
	gameengine.AddMana(gs, gs.Seats[seat], color, 1, "Mox Amber")

	emit(gs, slug, "Mox Amber", map[string]interface{}{
		"seat":  seat,
		"color": color,
	})
}

// -----------------------------------------------------------------------------
// Mox Opal
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   Metalcraft — {T}: Add one mana of any color. Activate only if you
//   control three or more artifacts.
//
// 0-mana legendary artifact. Requires metalcraft (3+ artifacts).

func registerMoxOpal(r *Registry) {
	r.OnActivated("Mox Opal", moxOpalActivated)
}

func moxOpalActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mox_opal_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Mox Opal", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Check metalcraft: 3+ artifacts.
	artifactCount := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsArtifact() {
			artifactCount++
		}
	}
	if artifactCount < 3 {
		emitFail(gs, slug, "Mox Opal", "metalcraft_not_met", map[string]interface{}{
			"artifacts": artifactCount,
		})
		return
	}

	src.Tapped = true
	// Add one mana of any color. MVP: pick based on commander color identity
	// or default to the first color available. For simplicity, add "any".
	gameengine.AddMana(gs, gs.Seats[seat], "any", 1, "Mox Opal")

	emit(gs, slug, "Mox Opal", map[string]interface{}{
		"seat":      seat,
		"artifacts": artifactCount,
		"color":     "any",
	})
}

// -----------------------------------------------------------------------------
// Gemstone Caverns
// -----------------------------------------------------------------------------
//
// Oracle text:
//
//   If Gemstone Caverns is in your opening hand and you're not the
//   starting player, you may begin the game with Gemstone Caverns on
//   the battlefield with a luck counter on it. If you do, exile a card
//   from your hand.
//   {T}: Add {C}. If Gemstone Caverns has a luck counter on it,
//   instead add one mana of any color.
//
// Utility land for non-starting-player advantage.

func registerGemstoneCaverns(r *Registry) {
	r.OnActivated("Gemstone Caverns", gemstoneCavernsActivated)
}

func gemstoneCavernsActivated(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "gemstone_caverns_tap"
	if gs == nil || src == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Gemstone Caverns", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	src.Tapped = true

	// Check for luck counter.
	hasLuck := false
	if src.Counters != nil && src.Counters["luck"] > 0 {
		hasLuck = true
	}

	if hasLuck {
		// Add one mana of any color.
		gameengine.AddMana(gs, gs.Seats[seat], "any", 1, "Gemstone Caverns")
		emit(gs, slug, "Gemstone Caverns", map[string]interface{}{
			"seat":  seat,
			"color": "any",
			"luck":  true,
		})
	} else {
		// Add {C} (colorless).
		gameengine.AddMana(gs, gs.Seats[seat], "C", 1, "Gemstone Caverns")
		emit(gs, slug, "Gemstone Caverns", map[string]interface{}{
			"seat":  seat,
			"color": "C",
			"luck":  false,
		})
	}
}
