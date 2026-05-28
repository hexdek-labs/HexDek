package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBeastWithin wires Beast Within.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Beast%20Within):
//
//	Destroy target permanent. Its controller creates a 3/3 green Beast
//	creature token.
//
// {2}{G} Instant. The premier off-color answer to anything — green's
// universal Vindicate. Lands, planeswalkers, indestructible permanents
// (it destroys, not exiles, so Avacyn/Eldrazi are safe but the rest
// of the board isn't), commanders. The 3/3 Beast given back is the
// cost — usually negligible against a key Game Changer, occasionally
// awkward against a high-value engine.
//
// Implementation:
//   - OnResolve. Target picker prefers the highest-EV opp permanent:
//     planeswalker > commander > creature with power >= 5 > artifact
//     mana source > land (utility) > any opp permanent. Avoids own
//     permanents — Beast Within can technically hit your own
//     permanent, but the hat policy never sacrifices a useful piece
//     for a 3/3 trade.
//   - DestroyPermanent through the canonical path — fires §704.5g
//     would-die replacements (Rest in Peace, regen substitutes,
//     indestructible Counter), §603.6c LTB triggers, and routes the
//     Card to the graveyard.
//   - 3/3 green Beast token to the destroyed permanent's controller
//     via CreateCreatureToken; the controller may be the caster
//     themselves on a self-Beast-Within (we still create the token
//     on the controller's battlefield per oracle text).
func registerBeastWithin(r *Registry) {
	r.OnResolve("Beast Within", beastWithinResolve)
}

func beastWithinResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "beast_within"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	target := pickBeastWithinTarget(gs, seat)
	if target == nil {
		emitFail(gs, slug, "Beast Within", "no_valid_target", nil)
		return
	}
	targetName := target.Card.DisplayName()
	targetController := target.Controller

	gameengine.DestroyPermanent(gs, target, nil)

	// Token enters under the destroyed permanent's controller per
	// oracle text — "its controller creates."
	gameengine.CreateCreatureToken(gs, targetController, "Beast",
		[]string{"creature", "beast"}, 3, 3)

	emit(gs, slug, "Beast Within", map[string]interface{}{
		"seat":              seat,
		"destroyed":         targetName,
		"destroyed_seat":    targetController,
		"token":             "3/3 green Beast",
		"token_controller":  targetController,
	})
}

// pickBeastWithinTarget chooses an opponent permanent by EV tier:
// planeswalker > commander > big creature > mana rock > land > anything.
func pickBeastWithinTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	tier := func(p *gameengine.Permanent) int {
		switch {
		case cardHasType(p.Card, "planeswalker"):
			return 5
		case p.IsCreature() && p.Power() >= 5:
			return 4
		case cardHasType(p.Card, "artifact") && !p.IsCreature():
			return 3
		case p.IsLand():
			return 2
		default:
			return 1
		}
	}
	var best *gameengine.Permanent
	bestTier := 0
	for _, opp := range gs.Opponents(seat) {
		for _, p := range gs.Seats[opp].Battlefield {
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
