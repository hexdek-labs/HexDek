package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// mana_vortex.go — per_card handler for Mana Vortex.
//
// Oracle text (Enchantment, {U}):
//
//	When you cast this spell, counter it unless you sacrifice a land.
//	At the beginning of each player's upkeep, that player sacrifices a
//	land of their choice.
//	When there are no lands on the battlefield, sacrifice this
//	enchantment.
//
// Why this needs a per_card handler: the parser emitted a
// `cast_trigger_tail` + `trigger_fragment_upkeep` + two `untyped_effect`
// nodes — none of which the engine acts on, so Mana Vortex sat on the
// battlefield doing nothing: a symmetric land-destruction engine running
// completely inert. Verified inert: no prior handler.
//
// Implemented here (the dominant, recurring board effect):
//   - each player's upkeep → that player sacrifices a land (AI: prefer a
//     basic so nonbasic utility lands survive longer);
//   - when no lands remain on any battlefield → sacrifice Mana Vortex.
//
// Deliberately omitted (one-shot entry cost, not a continuous effect):
// the cast-time "counter it unless you sacrifice a land." That is a
// downside paid at cast time; skipping it lets the enchantment enter
// without the toll — additive (strictly was-inert) and never regresses
// an existing behavior. Logged in the shard report.

func init() {
	registerManaVortex(Global())
	AddResetHook(registerManaVortex)
}

func registerManaVortex(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Mana Vortex", "upkeep", manaVortexUpkeep)
}

func manaVortexUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mana_vortex"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}

	// "That player sacrifices a land of their choice."
	if land := pickLandToSacrifice(gs, activeSeat); land != nil {
		gameengine.SacrificePermanent(gs, land, "Mana Vortex")
		emit(gs, slug, "Mana Vortex", map[string]interface{}{
			"seat": activeSeat,
			"land": land.Card.DisplayName(),
		})
	}

	// "When there are no lands on the battlefield, sacrifice this enchantment."
	if countAllLands(gs) == 0 {
		gameengine.SacrificePermanent(gs, perm, "Mana Vortex")
		emit(gs, slug, "Mana Vortex", map[string]interface{}{
			"seat":     perm.Controller,
			"self_sac": true,
			"no_lands": true,
		})
	}
}

// pickLandToSacrifice returns the land seat would least mind losing:
// prefer a basic land, otherwise the first land. nil if seat controls
// no lands.
func pickLandToSacrifice(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	var first *gameengine.Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if first == nil {
			first = p
		}
		if isBasicLandCard(p.Card) {
			return p
		}
	}
	return first
}

// countAllLands counts every land on every battlefield.
func countAllLands(gs *gameengine.GameState) int {
	n := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsLand() {
				n++
			}
		}
	}
	return n
}

// isBasicLandCard reports whether the card is a basic land (by basic
// supertype or by canonical basic name).
func isBasicLandCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "basic" {
			return true
		}
	}
	switch c.Name {
	case "Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes":
		return true
	}
	return false
}
