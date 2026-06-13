package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// plow_under_make_a_wish_mr_r60.go — two shard M-R utility spells that
// parsed to inert parsed_effect_residual nodes (no structured effect) and
// did NOTHING.
//
//   - Plow Under: "Put two target lands on top of their owners'
//     libraries." A brutal tempo/stax play — sets an opponent back two
//     land drops. No "put N lands on top" shape in the text fallback.
//   - Make a Wish: "Return two cards at random from your graveyard to
//     your hand." Graveyard value/recursion. No matching shape.
//
// One new self-registering file (init() + AddResetHook); no shared
// registry edits.
func init() {
	registerPlowUnderMakeAWishMRR60(Global())
	AddResetHook(registerPlowUnderMakeAWishMRR60)
}

func registerPlowUnderMakeAWishMRR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Plow Under", plowUnderResolve)
	r.OnResolve("Make a Wish", makeAWishResolve)
}

// Plow Under — put two target lands on top of their owners' libraries.
// Hat policy: target the lands of opponents (set them back), preferring a
// single opponent so the tempo loss lands on one player; pick up to two.
func plowUnderResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "plow_under"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Gather opponent lands, grouped so we hit the opponent with the most
	// lands first (maximal setback on the strongest mana base).
	bestOpp, bestN := -1, -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		n := countControlled(gs, opp, func(p *gameengine.Permanent) bool { return p.IsLand() })
		if n > bestN {
			bestN = n
			bestOpp = opp
		}
	}
	if bestOpp < 0 || bestN == 0 {
		emitFail(gs, slug, "Plow Under", "no_target_lands", nil)
		return
	}
	var lands []*gameengine.Permanent
	for _, p := range gs.Seats[bestOpp].Battlefield {
		if p != nil && p.IsLand() {
			lands = append(lands, p)
			if len(lands) == 2 {
				break
			}
		}
	}
	tucked := 0
	for _, p := range lands {
		if gameengine.BouncePermanent(gs, p, nil, "library_top") {
			tucked++
		}
	}
	emit(gs, slug, "Plow Under", map[string]interface{}{"seat": seat, "target_seat": bestOpp, "tucked": tucked})
}

// Make a Wish — return two cards at random from your graveyard to hand.
// We take the two highest-mana-value cards as a deterministic stand-in for
// "at random" (the AI sim has no card-choice negotiation here; recovering
// the most impactful cards is the dominant line).
func makeAWishResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "make_a_wish"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	gy := gs.Seats[seat].Graveyard
	// Pick up to two by descending mana value (snapshot indices first).
	type c struct {
		card *gameengine.Card
		cmc  int
	}
	var cs []c
	for _, card := range gy {
		if card != nil {
			cs = append(cs, c{card, card.CMC})
		}
	}
	// simple selection of top-2 by cmc
	returned := 0
	for n := 0; n < 2 && len(cs) > 0; n++ {
		best := 0
		for i := 1; i < len(cs); i++ {
			if cs[i].cmc > cs[best].cmc {
				best = i
			}
		}
		gameengine.MoveCard(gs, cs[best].card, seat, "graveyard", "hand", "make-a-wish")
		returned++
		cs = append(cs[:best], cs[best+1:]...)
	}
	emit(gs, slug, "Make a Wish", map[string]interface{}{"seat": seat, "returned": returned})
}
