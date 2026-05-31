package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Lin Sivvi, Defiant Hero — {2}{W} 2/4 Legendary Creature — Human Rebel.
//
//   {X}, {T}: Search your library for a Rebel permanent card with mana
//   value X or less, put it onto the battlefield, then shuffle.
//   {3}: Put target Rebel card from your graveyard on the bottom of
//   your library.
//
// Implementation:
//   - Ability 0 ({X},{T}): read X from ctx["x"] / ctx["x_paid"] (Bruce
//     Banner pattern at bruce_banner.go:39); walk controller's library
//     for Rebel permanent cards with cmc <= X, pick highest-CMC (best
//     in slot); MoveCard library→battlefield (untapped), shuffle. Engine
//     handles the {T} + mana cost via the standard activation cost-pay
//     path before this handler fires.
//   - Ability 1 ({3}): target picked from ctx; must be a Rebel in
//     controller's graveyard; remove from graveyard and append to
//     library (bottom).

func init() {
	registerLinSivviDefiantHero(Global())
	AddResetHook(registerLinSivviDefiantHero)
}

func registerLinSivviDefiantHero(r *Registry) {
	r.OnActivated("Lin Sivvi, Defiant Hero", linSivviActivate)
}

func linSivviActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		linSivviTutorRebel(gs, src, ctx)
	case 1:
		linSivviBottomFromGraveyard(gs, src, ctx)
	}
}

func linSivviTutorRebel(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lin_sivvi_tutor_rebel"
	if ctx == nil {
		ctx = map[string]interface{}{}
	}
	x, _ := ctx["x"].(int)
	if x == 0 {
		x, _ = ctx["x_paid"].(int)
	}
	if x < 0 {
		x = 0
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	pick := pickRebelPermanentWithMVAtMost(s.Library, x)
	if pick == nil {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, src.Card.DisplayName(), "no_rebel_at_or_under_x", map[string]interface{}{
			"x": x,
		})
		return
	}
	gameengine.MoveCard(gs, pick, seat, "library", "battlefield", "lin_sivvi_tutor")
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":  []string{pick.DisplayName()},
			"reason": slug,
			"x":      x,
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"x":     x,
		"found": pick.DisplayName(),
	})
}

func linSivviBottomFromGraveyard(gs *gameengine.GameState, src *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lin_sivvi_bottom_rebel"
	if ctx == nil {
		ctx = map[string]interface{}{}
	}
	target, _ := ctx["target_card"].(*gameengine.Card)
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_target_in_ctx", nil)
		return
	}
	if !cardHasSubtype(target, "rebel") {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_rebel", nil)
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// Confirm target is in seat's graveyard.
	idx := -1
	for i, c := range s.Graveyard {
		if c == target {
			idx = i
			break
		}
	}
	if idx < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "target_not_in_graveyard",
			map[string]interface{}{"card": target.DisplayName()})
		return
	}
	s.Graveyard = append(s.Graveyard[:idx], s.Graveyard[idx+1:]...)
	s.Library = append(s.Library, target)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"bottomed_card": target.DisplayName(),
	})
}

// pickRebelPermanentWithMVAtMost walks library for a Rebel permanent card
// with cmc <= maxCMC. Returns highest-CMC match (Lin Sivvi typically wants
// the biggest Rebel she can fit).
func pickRebelPermanentWithMVAtMost(library []*gameengine.Card, maxCMC int) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range library {
		if c == nil {
			continue
		}
		if !cardHasSubtype(c, "rebel") {
			continue
		}
		if !isPermanentCard(c) {
			continue
		}
		cmc := cardCMC(c)
		if cmc > maxCMC {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}
