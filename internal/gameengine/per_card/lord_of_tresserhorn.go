package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Lord of Tresserhorn — {2}{B}{B} 10/4 Legendary Creature — Zombie.
//
//   When Lord of Tresserhorn enters, you lose 2 life, you sacrifice
//   two creatures, and target opponent draws two cards.
//   {B}: Regenerate Lord of Tresserhorn.
//
// Three ETB rider effects, all mandatory. AI heuristic for the "sacrifice
// two creatures" — pick the two lowest-Power non-self creatures on
// controller's battlefield (the typical "sac fodder" pick); if only Lord
// of Tresserhorn is available, sacrifice him as the second pick (CR
// 701.16d — Lord can be one of the two). Target opponent for the draw:
// pick the lowest-life non-Lost opponent (don't fuel a runaway leader).
// Regenerate ability is engine keyword-pipeline territory.

func init() {
	registerLordOfTresserhorn(Global())
	AddResetHook(registerLordOfTresserhorn)
}

func registerLordOfTresserhorn(r *Registry) {
	r.OnETB("Lord of Tresserhorn", lordOfTresserhornETB)
}

func lordOfTresserhornETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lord_of_tresserhorn_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	gameengine.LoseLife(gs, seat, 2, perm.Card.DisplayName())
	picks := pickLowestPowerCreatures(s, perm, 2)
	for _, p := range picks {
		gameengine.SacrificePermanent(gs, p, "lord_of_tresserhorn_etb")
	}
	oppSeat := pickLowestLifeOpponent(gs, seat)
	if oppSeat >= 0 {
		drawOne(gs, oppSeat, perm.Card.DisplayName())
		drawOne(gs, oppSeat, perm.Card.DisplayName())
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"life_lost":     2,
		"sacrificed":    permsNames(picks),
		"target_opp":    oppSeat,
	})
	_ = gs.CheckEnd()
}

func pickLowestPowerCreatures(s *gameengine.Seat, exclude *gameengine.Permanent, n int) []*gameengine.Permanent {
	type candidate struct {
		p   *gameengine.Permanent
		pow int
	}
	cands := []candidate{}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		cands = append(cands, candidate{p: p, pow: p.Power()})
	}
	// Bubble-sort by power asc (small n — fine).
	for i := 0; i < len(cands); i++ {
		for j := i + 1; j < len(cands); j++ {
			if cands[j].pow < cands[i].pow {
				cands[i], cands[j] = cands[j], cands[i]
			}
		}
	}
	picks := []*gameengine.Permanent{}
	for _, c := range cands {
		if c.p == exclude && len(picks)+1 < n {
			continue // try to defer the source if other fodder is available
		}
		picks = append(picks, c.p)
		if len(picks) == n {
			break
		}
	}
	// If we still don't have N picks and exclude isn't in the list, add it.
	if len(picks) < n {
		for _, c := range cands {
			if c.p == exclude {
				picks = append(picks, c.p)
				break
			}
		}
	}
	return picks
}

func permsNames(perms []*gameengine.Permanent) []string {
	out := []string{}
	for _, p := range perms {
		if p != nil && p.Card != nil {
			out = append(out, p.Card.DisplayName())
		}
	}
	return out
}
