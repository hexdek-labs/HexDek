package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVandalblast wires Vandalblast.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Vandalblast):
//
//	Destroy target artifact an opponent controls.
//	Overload {4}{R}  (You may cast this spell for its overload cost. If
//	you do, change "target" to "each.")
//
// {R} Sorcery. Single-card answer to Sol Ring + a way to wipe the
// table's mana rocks at instant-overload. The asymmetry of "opp
// controls" — own artifacts survive even at overload — is the
// design feature that makes Vandalblast a cEDH staple.
//
// Implementation:
//   - OnResolve. Read IsOverloaded(item) to determine mode (mirrors
//     Cyclonic Rift shape from removal.go). Overload sweeps every
//     opp artifact; single-target picks the highest-EV opp artifact
//     (mana rock > equipment > random).
//   - Both modes destroy through DestroyPermanent so §704.5g
//     replacements (indestructible / Welding Jar protection) and
//     LTB observers fire correctly.
//   - Lands are unaffected (the printed text says "artifact" only —
//     artifact lands like Mishra's Factory are still artifacts and
//     fall under the sweep, but pure lands like Strip Mine don't).
//   - No-op clean when an opp board has zero artifacts.
func registerVandalblast(r *Registry) {
	r.OnResolve("Vandalblast", vandalblastResolve)
}

func vandalblastResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "vandalblast"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	overload := gameengine.IsOverloaded(item)

	if overload {
		// Sweep all opp artifacts.
		var victims []*gameengine.Permanent
		for _, opp := range gs.Opponents(seat) {
			s := gs.Seats[opp]
			if s == nil {
				continue
			}
			for _, p := range s.Battlefield {
				if p == nil || p.Card == nil {
					continue
				}
				if !cardHasType(p.Card, "artifact") {
					continue
				}
				victims = append(victims, p)
			}
		}
		destroyed := 0
		for _, p := range victims {
			if gameengine.DestroyPermanent(gs, p, nil) {
				destroyed++
			}
		}
		emit(gs, slug, "Vandalblast", map[string]interface{}{
			"seat":      seat,
			"mode":      "overload",
			"destroyed": destroyed,
		})
		return
	}

	// Single-target: pick highest-EV opp artifact.
	target := pickVandalblastSingleTarget(gs, seat)
	if target == nil {
		emitFail(gs, slug, "Vandalblast", "no_valid_target", nil)
		return
	}
	targetName := target.Card.DisplayName()
	gameengine.DestroyPermanent(gs, target, nil)
	emit(gs, slug, "Vandalblast", map[string]interface{}{
		"seat":      seat,
		"mode":      "single",
		"destroyed": targetName,
	})
}

// pickVandalblastSingleTarget picks an opp artifact by EV tier:
// mana rock > equipment > planeswalker-artifact > anything.
func pickVandalblastSingleTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	tier := func(p *gameengine.Permanent) int {
		// Mana rocks identified by being cheap (CMC <= 3) non-creature
		// artifacts that aren't equipment — Sol Ring / Arcane Signet /
		// Mana Crypt / medallions all fit.
		isArt := cardHasType(p.Card, "artifact")
		if !isArt {
			return 0
		}
		isEquip := cardHasType(p.Card, "equipment")
		switch {
		case !p.IsCreature() && !isEquip && cardCMC(p.Card) <= 3:
			return 4 // mana rock
		case isEquip:
			return 3
		case p.IsCreature():
			return 2 // artifact creature
		default:
			return 1
		}
	}
	var best *gameengine.Permanent
	bestTier := 0
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			tt := tier(p)
			if tt > bestTier {
				bestTier = tt
				best = p
			}
		}
	}
	return best
}
