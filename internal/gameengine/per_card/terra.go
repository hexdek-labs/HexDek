package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTerra wires Terra, Magical Adept // Esper Terra (DFC).
//
// Front face — Terra, Magical Adept:
//
//	Flying
//	When Terra enters, mill five cards. Put up to one enchantment card
//	milled this way into your hand.
//	Trance — {4}{R}{G}, {T}: Exile Terra, then return it to the
//	battlefield transformed under its owner's control. Activate only
//	as a sorcery.
//
// Back face — Esper Terra (Saga):
//
//	(As this Saga enters and after your draw step, add a lore counter.)
//	I, II, III — Create a token that's a copy of target nonlegendary
//	enchantment you control. It gains haste. If it's a Saga, put up to
//	three lore counters on it. Sacrifice it at the beginning of your
//	next end step.
//	IV — Add {W}{W}, {U}{U}, {B}{B}, {R}{R}, and {G}{G}. Exile Esper
//	Terra, then return it to the battlefield (front face up).
//
// Implementation:
//
//   - ETB (front): mill five cards from controller's library, then pull
//     up to one enchantment from those milled cards into hand. Greedy
//     pick: prefers highest-CMC enchantment milled.
//   - OnActivated (front): Trance transform via TransformPermanent. The
//     "exile then return transformed" wording is mechanically equivalent
//     in this engine to a flip in place; emitPartial flags the exile
//     round-trip nuance.
//   - Saga back face chapter abilities and color burst (IV) are
//     emitPartial — Saga lore-counter scheduling and copy-token semantics
//     for nonlegendary enchantments aren't expressible here without
//     considerable scaffolding.
func registerTerra(r *Registry) {
	// Register on the full DFC name (DisplayName before transform) and
	// the front-face split, since dispatch keys on the printed name and
	// loaders vary in which form they store.
	r.OnETB("Terra, Magical Adept // Esper Terra", terraETB)
	r.OnETB("Terra, Magical Adept", terraETB)
	r.OnActivated("Terra, Magical Adept // Esper Terra", terraTrance)
	r.OnActivated("Terra, Magical Adept", terraTrance)
}

func terraETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "terra_magical_adept_mill_five_pick_enchantment"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	var milled []*gameengine.Card
	for i := 0; i < 5; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "graveyard", "terra_mill")
		milled = append(milled, top)
	}

	// Greedy pick: highest-CMC enchantment among milled cards.
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range milled {
		if c == nil || !cardHasType(c, "enchantment") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pick = c
		}
	}
	picked := ""
	if pick != nil {
		gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "hand", "terra_etb_pick")
		picked = pick.DisplayName()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"milled":        len(milled),
		"enchantment":   picked,
	})
}

func terraTrance(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "terra_trance_transform"
	if gs == nil || src == nil {
		return
	}
	cardName := src.Card.DisplayName()
	if !gameengine.TransformPermanent(gs, src, "terra_trance_activate") {
		emitPartial(gs, slug, cardName,
			"transform_failed_face_data_missing")
		return
	}
	emit(gs, slug, cardName, map[string]interface{}{
		"seat": src.Controller,
		"to":   "Esper Terra",
	})
	emitPartial(gs, slug, cardName,
		"exile_then_return_transformed_round_trip_and_saga_chapter_abilities_unmodeled")
}
