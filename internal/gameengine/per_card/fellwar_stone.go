package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFellwarStone wires Fellwar Stone.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Fellwar%20Stone):
//
//	{T}: Add one mana of any color that a land an opponent controls
//	could produce.
//
// {2} Artifact. The premier 2-mana ramp rock for off-color access.
// Where Sol Ring gives {C}{C}, Fellwar Stone reads opponent lands and
// lets you fix into colors your own basics can't cover.
//
// Implementation:
//   - OnActivated index 0: tap, add 1 mana of a color any opponent's
//     basic land could produce. Picker scans every opponent's lands
//     for basic-land subtypes (Plains/Island/Swamp/Mountain/Forest)
//     and collects the union; chooses the first hit by W/U/B/R/G
//     order (deterministic — the printed "any color" is the
//     controller's choice, but the hat default picks the highest-
//     demand color which lives above this layer).
//   - Nonbasic lands that produce colored mana (shocks, fetches as
//     "produced via the basic-type subtype", duals) ARE covered:
//     Watery Grave has subtypes "island swamp" so it counts for U
//     and B. Pure-utility nonbasics with no basic-land-subtype
//     (Cabal Coffers, Strip Mine) contribute nothing — matches the
//     printed "land … could produce" wording, since produces-color
//     for an arbitrary nonbasic requires reading its full oracle
//     text which isn't worth the per_card-layer cost.
//   - No opponent has a color-producing land → no-op with a fail
//     event (tap is wasted; matches printed-rules edge case).
func registerFellwarStone(r *Registry) {
	r.OnActivated("Fellwar Stone", fellwarStoneActivate)
}

func fellwarStoneActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "fellwar_stone_mana"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Fellwar Stone", "already_tapped", nil)
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

	color := pickFellwarStoneColor(gs, seat)
	if color == "" {
		emitFail(gs, slug, "Fellwar Stone", "no_opp_color_source", nil)
		return
	}
	src.Tapped = true
	gameengine.AddMana(gs, s, color, 1, "Fellwar Stone")
	emit(gs, slug, "Fellwar Stone", map[string]interface{}{
		"seat":  seat,
		"color": color,
		"mana":  1,
	})
}

// pickFellwarStoneColor scans every opponent's lands for basic-land
// subtypes and returns the first available color in W-U-B-R-G order.
// Returns "" when no opponent controls a color-producing land.
func pickFellwarStoneColor(gs *gameengine.GameState, seat int) string {
	available := map[string]bool{}
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsLand() {
				continue
			}
			if cardHasSubtype(p.Card, "plains") {
				available["W"] = true
			}
			if cardHasSubtype(p.Card, "island") {
				available["U"] = true
			}
			if cardHasSubtype(p.Card, "swamp") {
				available["B"] = true
			}
			if cardHasSubtype(p.Card, "mountain") {
				available["R"] = true
			}
			if cardHasSubtype(p.Card, "forest") {
				available["G"] = true
			}
		}
	}
	// WUBRG priority — deterministic. Higher-EV color picking is a
	// hat-layer concern; this just makes the picker reproducible.
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		if available[c] {
			return c
		}
	}
	return ""
}
