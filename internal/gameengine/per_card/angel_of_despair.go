package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// angel_of_despair.go — per_card handler for Angel of Despair.
//
// Oracle text (Creature — Angel, {3}{W}{W}{B}{B}):
//
//	Flying
//	When this creature enters, destroy target permanent.
//
// Why this needs a per_card handler: the ETB clause parsed to inert
// scaffold, so this premier reanimator target entered and destroyed
// nothing. Verified inert: no registered handler for "Angel of Despair".
//
// Implemented here (OnETB): destroy the highest-value permanent an
// opponent controls — a nonland permanent if any (creatures, artifacts,
// enchantments, planeswalkers), otherwise any opponent permanent
// (including a land) so the mandatory "destroy target permanent" always
// resolves against something.

func init() {
	registerAngelOfDespair(Global())
	AddResetHook(registerAngelOfDespair)
}

func registerAngelOfDespair(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Angel of Despair", angelOfDespairETB)
}

func angelOfDespairETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "angel_of_despair"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	target := pickBestOppNonlandPermanent(gs, seat)
	if target == nil {
		target = anyOpponentPermanent(gs, seat)
	}
	if target == nil {
		emitFail(gs, slug, "Angel of Despair", "no_opponent_permanent", map[string]interface{}{"seat": seat})
		return
	}
	name := target.Card.DisplayName()
	if gameengine.DestroyPermanent(gs, target, perm) {
		emit(gs, slug, "Angel of Despair", map[string]interface{}{
			"seat":      seat,
			"destroyed": name,
		})
	}
}

// anyOpponentPermanent returns any permanent controlled by an opponent
// of seat (lands included), or nil.
func anyOpponentPermanent(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil {
				return p
			}
		}
	}
	return nil
}
