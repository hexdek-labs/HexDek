package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// crackling_doom.go — per_card handler for Crackling Doom.
//
// Oracle text (Instant, {R}{W}{B}):
//
//	Crackling Doom deals 2 damage to each opponent. Each opponent
//	sacrifices a creature with the greatest power among creatures that
//	player controls.
//
// Why this needs a per_card handler: the spell body parsed to inert
// scaffold, so on resolution Crackling Doom — Mardu's premier
// indestructible-proof removal — dealt no damage and forced no
// sacrifice. Verified inert: no registered handler for "Crackling Doom".
//
// Implemented here (OnResolve): deal 2 to each opponent (LoseLife so
// §704.5a fires), then each opponent sacrifices their greatest-power
// creature (the sacrifice bypasses indestructible/hexproof — that is the
// card's whole point).

func init() {
	registerCracklingDoom(Global())
	AddResetHook(registerCracklingDoom)
}

func registerCracklingDoom(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Crackling Doom", cracklingDoomResolve)
}

func cracklingDoomResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "crackling_doom"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for _, opp := range gs.Opponents(seat) {
		gameengine.LoseLife(gs, opp, 2, "Crackling Doom")
		if victim := greatestPowerCreatureOf(gs, opp); victim != nil {
			name := victim.Card.DisplayName()
			gameengine.SacrificePermanent(gs, victim, "Crackling Doom")
			emit(gs, slug, "Crackling Doom", map[string]interface{}{
				"seat":       seat,
				"opponent":   opp,
				"sacrificed": name,
			})
		}
	}
	_ = gs.CheckEnd()
}

// greatestPowerCreatureOf returns the highest-power creature the seat
// controls (ties broken by first found), or nil.
func greatestPowerCreatureOf(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pw := p.Power(); pw > bestPow {
			bestPow = pw
			best = p
		}
	}
	return best
}
