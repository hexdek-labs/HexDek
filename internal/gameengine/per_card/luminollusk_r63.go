package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// luminollusk_r63.go — per_card handler for Luminollusk.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Deathtouch
//	Vivid — When this creature enters, you gain life equal to the number
//	of colors among permanents you control.
//
// {5}{C} Artifact Creature — Elemental. Deathtouch is a keyword (handled
// by the keyword layer); only the ETB life gain was inert (it parsed to a
// fallback node). Implementation: OnETB → count the distinct WUBRG colors
// across the controller's permanents (layer-aware gs.ColorsOf) and gain
// that much life.
func init() {
	registerLuminolluskR63(Global())
	AddResetHook(registerLuminolluskR63)
}

func registerLuminolluskR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Luminollusk", luminolluskETB)
}

func luminolluskETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "luminollusk"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	seen := map[string]bool{}
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		// p.Card.Colors is the runtime color cache (kept current by the
		// resolver / continuous color-setting effects). The layer
		// Characteristics.Colors is NOT populated from Card.Colors in the
		// base path, so ColorsOf would read empty for ordinary permanents.
		for _, c := range p.Card.Colors {
			u := strings.ToUpper(strings.TrimSpace(c))
			switch u {
			case "W", "U", "B", "R", "G":
				seen[u] = true
			}
		}
	}
	colors := len(seen)
	if colors > 0 {
		gameengine.GainLife(gs, seat, colors, "Luminollusk")
	}
	emit(gs, slug, "Luminollusk", map[string]interface{}{
		"seat": seat, "colors": colors,
	})
}
