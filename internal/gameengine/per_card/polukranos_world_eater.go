package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPolukranosWorldEater wires Polukranos, World Eater.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Polukranos%2C%20World%20Eater):
//
//	{2}{G}{G}
//	Legendary Creature — Hydra
//	5/5
//	{X}{X}{G}: Polukranos, World Eater fights up to X target creatures
//	your opponents control as long as those creatures don't share a
//	controller. X can't be greater than the number of creatures
//	Polukranos, World Eater could fight this way.
//
// The "don't share a controller" clause means each fight target must
// be from a distinct opponent — Polukranos can fight one creature per
// opponent, up to X.
//
// Implementation:
//   - OnActivated. ctx["x"] carries the chosen X; falls back to 1
//     when not threaded (engine still pays the cost, the handler just
//     can't see it from this hook).
//   - Pick the highest-power creature from each opponent's battlefield
//     (one per opponent), up to X targets total. Sort by power desc so
//     when X < #opponents, we hit the biggest threats first.
//   - Each fight: Polukranos and the target each take damage equal to
//     the other's power, marked via "damage_marked" counter so SBAs
//     resolve lethals later (matches the Nessian Wilds Ravager pattern
//     in this codebase). Polukranos may die after a single fight if
//     the target's power >= 5 — that's the rules-correct outcome.
//   - Hexproof / shroud target legality is engine-side; this handler
//     trusts the activation passed legality at cast time.
func registerPolukranosWorldEater(r *Registry) {
	r.OnActivated("Polukranos, World Eater", polukranosActivate)
}

func polukranosActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "polukranos_world_eater_fight"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	x := 1
	if v, ok := ctx["x"].(int); ok && v > 0 {
		x = v
	}

	// Pick the highest-power creature from each opponent (one per opp).
	type pick struct {
		seat int
		perm *gameengine.Permanent
		pow  int
	}
	var picks []pick
	for _, opp := range gs.Opponents(src.Controller) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		var best *gameengine.Permanent
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
		if best != nil {
			picks = append(picks, pick{opp, best, best.Power()})
		}
	}

	// Sort picks by power desc, truncate to X (selection-sort scale OK at ≤3 opps).
	for i := 0; i < len(picks); i++ {
		for j := i + 1; j < len(picks); j++ {
			if picks[j].pow > picks[i].pow {
				picks[i], picks[j] = picks[j], picks[i]
			}
		}
	}
	if len(picks) > x {
		picks = picks[:x]
	}

	if len(picks) == 0 {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      src.Controller,
			"x":         x,
			"triggered": false,
			"reason":    "no_legal_targets",
		})
		return
	}

	polukranosPower := src.Power()
	var targets []string
	for _, p := range picks {
		tgtPower := p.perm.Power()
		p.perm.AddCounter("damage_marked", polukranosPower)
		src.AddCounter("damage_marked", tgtPower)
		targets = append(targets, p.perm.Card.DisplayName())
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":             src.Controller,
		"x":                x,
		"targets":          targets,
		"polukranos_power": polukranosPower,
	})
}
