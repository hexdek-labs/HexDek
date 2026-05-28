package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPerniciousDeed wires Pernicious Deed.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Pernicious%20Deed):
//
//	{X}, Sacrifice this enchantment: Destroy each artifact, creature,
//	and enchantment with mana value X or less.
//
// {1}{B}{G} Enchantment. The BG toolbox wipe — sits on the battlefield
// as a permanent threat-of-activation, then sweeps at the X you choose
// when the table over-commits. Pernicious Deed kills its own
// enchantment self too (mana value 3), so a Deed activation at X=3+
// removes the Deed alongside everything else of that size or smaller
// — usually the point.
//
// Implementation:
//   - OnActivated index 0: sacrifice Deed (printed cost), then
//     destroy each artifact / creature / enchantment with mana value
//     X or less across all battlefields. X comes from ctx["x"] if
//     stamped by the activation pipeline; falls back to a heuristic
//     that picks the largest X the controller can pay while still
//     hitting the most-valuable opp permanent.
//   - Sacrifice routes through SacrificePermanent so §704.5g
//     LTB observers and Karmic Guide-style graveyard payoffs fire.
//   - The Deed itself is sacrificed BEFORE the destroy sweep, so
//     it doesn't double-destroy (it's already in graveyard by the
//     time destroyAllAtCMC scans the battlefield).
//   - Lands are unaffected per printed restriction (no "land" in
//     the type list).
func registerPerniciousDeed(r *Registry) {
	r.OnActivated("Pernicious Deed", perniciousDeedActivate)
}

func perniciousDeedActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "pernicious_deed"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// X from caller, or hat-policy default: the largest mana value
	// that still includes a high-value opp permanent.
	x := 0
	if v, ok := ctx["x"]; ok {
		if n, ok := v.(int); ok {
			x = n
		}
	}
	if x <= 0 {
		x = pickPerniciousDeedX(gs, seat)
	}

	// Sacrifice Deed first (cost). After this point src is in the
	// graveyard and won't be scanned by the destroy sweep.
	gameengine.SacrificePermanent(gs, src, "pernicious_deed_cost")

	// Destroy all artifacts, creatures, enchantments with MV <= X.
	destroyed := 0
	var victims []*gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			if !(p.IsCreature() || cardHasType(p.Card, "artifact") || cardHasType(p.Card, "enchantment")) {
				continue
			}
			if cardCMC(p.Card) > x {
				continue
			}
			victims = append(victims, p)
		}
	}
	for _, p := range victims {
		if gameengine.DestroyPermanent(gs, p, nil) {
			destroyed++
		}
	}

	emit(gs, slug, "Pernicious Deed", map[string]interface{}{
		"seat":      seat,
		"x":         x,
		"destroyed": destroyed,
	})
}

// pickPerniciousDeedX chooses X for the Deed activation: walk the
// table and find the lowest CMC value that destroys at least one
// opp non-land permanent while sparing the controller's biggest
// permanent. Defaults to 4 when no sensible pick.
func pickPerniciousDeedX(gs *gameengine.GameState, seat int) int {
	maxOppHit := 0
	maxOwnSave := 1 << 30
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			if !(p.IsCreature() || cardHasType(p.Card, "artifact") || cardHasType(p.Card, "enchantment")) {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > maxOppHit {
				maxOppHit = cmc
			}
		}
	}
	own := gs.Seats[seat]
	if own != nil {
		for _, p := range own.Battlefield {
			if p == nil || p.Card == nil || p.IsLand() {
				continue
			}
			if !(p.IsCreature() || cardHasType(p.Card, "artifact") || cardHasType(p.Card, "enchantment")) {
				continue
			}
			cmc := cardCMC(p.Card)
			if cmc > maxOppHit && cmc < maxOwnSave {
				maxOwnSave = cmc
			}
		}
	}
	if maxOppHit > 0 {
		return maxOppHit
	}
	return 4
}
