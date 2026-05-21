package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArchmageAscension wires Archmage Ascension.
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	At the beginning of each end step, if you drew two or more cards
//	this turn, you may put a quest counter on this enchantment.
//	As long as this enchantment has six or more quest counters on it,
//	if you would draw a card, you may instead search your library for
//	a card, put that card into your hand, then shuffle.
//
// Implementation (Muninn gap #35 — 27K hits):
//   - OnTrigger("end_step"): "each end step" — fires for every player's
//     end step, gated on the controller having drawn 2+ this turn. AI
//     policy auto-accepts the "may" (monotone upside).
//   - The 6+ counter draw-replacement is a §614 replacement effect that
//     requires registering against the controller's draw event. The
//     engine's per-card replacement registration is limited to the
//     built-in table; emitPartial.
func registerArchmageAscension(r *Registry) {
	r.OnTrigger("Archmage Ascension", "end_step", archmageAscensionEndStep)
	// R52 batchM: 6+ quest counters → "draw a card" becomes "search
	// your library for a card, put it into your hand, then shuffle".
	// Wired via player_would_draw, matching the Chains of Mephistopheles
	// precedent for trigger-style draw replacements.
	r.OnTrigger("Archmage Ascension", "player_would_draw", archmageAscensionTutorOnDraw)
}

func archmageAscensionEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archmage_ascension_quest"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	drawn := seat.Turn.CardsDrawn
	if drawn < 2 {
		if seat.Flags != nil && seat.Flags["cards_drawn_this_turn"] >= 2 {
			drawn = seat.Flags["cards_drawn_this_turn"]
		} else {
			emitFail(gs, slug, perm.Card.DisplayName(), "drew_less_than_two_this_turn", map[string]interface{}{
				"seat":  seatIdx,
				"drawn": drawn,
			})
			return
		}
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.AddCounter("quest", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           seatIdx,
		"drawn":          drawn,
		"quest_counters": perm.Counters["quest"],
	})
	// Draw replacement at 6+ counters wired in
	// archmageAscensionTutorOnDraw (R52 batchM); end-step counter add
	// is the only thing this hook owns now.
}

// archmageAscensionTutorOnDraw replaces the controller's draw with a
// library tutor when Ascension has 6+ quest counters. Matches the
// Chains of Mephistopheles trigger-style replacement precedent — sets
// ctx["draw_replaced"] = true for downstream consumers + performs the
// tutor (highest-CMC card for upside, matching the AI's "always
// accept" stance on this monotone-good effect).
func archmageAscensionTutorOnDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "archmage_ascension_tutor_draw"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	if perm.Counters == nil || perm.Counters["quest"] < 6 {
		return
	}
	drawSeat, _ := ctx["draw_seat"].(int)
	if drawSeat != perm.Controller {
		return
	}
	seat := gs.Seats[drawSeat]
	if seat == nil || seat.Lost {
		return
	}
	if len(seat.Library) == 0 {
		return
	}
	// AI picks the highest-CMC card as the tutor target.
	var pick *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			pick = c
		}
	}
	if pick == nil {
		return
	}
	gameengine.MoveCard(gs, pick, drawSeat, "library", "hand", "archmage_ascension_tutor")
	// Shuffle the remaining library.
	if gs.Rng != nil && len(seat.Library) > 1 {
		gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
			seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
		})
	}
	ctx["draw_replaced"] = true
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   drawSeat,
		"picked": pick.DisplayName(),
		"cmc":    bestCMC,
	})
}
