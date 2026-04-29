package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrimMonolith wires up Grim Monolith.
//
// Oracle text:
//
//	Grim Monolith doesn't untap during your untap step.
//	{T}: Add {C}{C}{C}.
//	{4}: Untap Grim Monolith.
//
// Identical to Basalt Monolith but with a {4} untap cost (vs Basalt's
// {3}). Net-mana-NEGATIVE alone, but with Kinnan: {C}{C}{C} + 1
// Kinnan = 4 mana → pay {4} to untap → break-even → requires an
// additional mana source for the combo. With Rings of Brighthearth
// (copy the untap trigger for {2}) Grim Monolith also goes infinite.
func registerGrimMonolith(r *Registry) {
	r.OnETB("Grim Monolith", grimMonolithETB)
	r.OnActivated("Grim Monolith", grimMonolithActivate)
}

func grimMonolithETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["skip_untap"] = 1
	perm.DoesNotUntap = true
	emit(gs, "grim_monolith_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func grimMonolithActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	switch abilityIdx {
	case 0:
		const slug = "grim_monolith_tap"
		if src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
			return
		}
		src.Tapped = true
		gameengine.AddManaFromPermanent(gs, s, src, "C", 3)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"added":    3,
			"color":    "C",
			"new_pool": s.ManaPool,
		})
	case 1:
		const slug = "grim_monolith_untap"
		if !src.Tapped {
			emitFail(gs, slug, src.Card.DisplayName(), "already_untapped", nil)
			return
		}
		src.Tapped = false
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat": seat,
		})
	}
}
