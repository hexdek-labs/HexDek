package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Phelddagrif — {1}{G}{W}{U} 3/3 Legendary Creature — Hippo.
//
//   {G}: Phelddagrif gains trample until end of turn. Target opponent
//   creates a 1/1 green Hippo creature token.
//   {W}: Phelddagrif gains flying until end of turn. Target opponent
//   gains 2 life.
//   {U}: Return Phelddagrif to its owner's hand. Target opponent may
//   draw a card.
//
// Group-hug commander. Each activation has a giveaway rider — opponent
// is picked from ctx["target_seat"] (defaults to first non-Lost opponent
// if missing). UEOT keyword grants via perm.Flags + GrantedAbilities
// pattern (Akroma's Will + Squall, SeeD Mercenary references).

func init() {
	registerPhelddagrif(Global())
	AddResetHook(registerPhelddagrif)
}

func registerPhelddagrif(r *Registry) {
	r.OnActivated("Phelddagrif", phelddagrifActivate)
}

func phelddagrifActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		phelddagrifTrampleAndHippo(gs, src, ctx)
	case 1:
		phelddagrifFlyingAndLife(gs, src, ctx)
	case 2:
		phelddagrifBounceAndDraw(gs, src, ctx)
	}
}

func pickPhelddagrifOpponent(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) int {
	if ctx != nil {
		if v, ok := ctx["target_seat"].(int); ok && v >= 0 && v < len(gs.Seats) && v != src.Controller {
			if gs.Seats[v] != nil && !gs.Seats[v].Lost {
				return v
			}
		}
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == src.Controller {
			continue
		}
		return i
	}
	return -1
}

func phelddagrifTrampleAndHippo(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phelddagrif_trample_and_hippo"
	src.GrantedAbilities = append(src.GrantedAbilities, "trample")
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:trample"] = 1
	src.Flags["trample_eot"] = 1
	gs.InvalidateCharacteristicsCache()
	oppSeat := pickPhelddagrifOpponent(gs, src, ctx)
	if oppSeat >= 0 {
		gameengine.CreateCreatureToken(gs, oppSeat, "Hippo Token",
			[]string{"creature", "hippo", "pip:G"}, 1, 1)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"opp_seat": oppSeat,
		"gave":     "1/1 Hippo token",
	})
}

func phelddagrifFlyingAndLife(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phelddagrif_flying_and_life"
	src.GrantedAbilities = append(src.GrantedAbilities, "flying")
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:flying"] = 1
	src.Flags["flying_eot"] = 1
	gs.InvalidateCharacteristicsCache()
	oppSeat := pickPhelddagrifOpponent(gs, src, ctx)
	if oppSeat >= 0 {
		gameengine.GainLife(gs, oppSeat, 2, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     src.Controller,
		"opp_seat": oppSeat,
		"gave":     "2 life",
	})
}

func phelddagrifBounceAndDraw(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phelddagrif_bounce_and_draw"
	ownerSeat := src.Card.Owner
	gameengine.BouncePermanent(gs, src, src, "hand")
	oppSeat := pickPhelddagrifOpponent(gs, src, ctx)
	if oppSeat >= 0 {
		drawOne(gs, oppSeat, src.Card.DisplayName())
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         src.Controller,
		"owner_seat":   ownerSeat,
		"opp_seat":     oppSeat,
		"gave":         "draw 1",
	})
}
