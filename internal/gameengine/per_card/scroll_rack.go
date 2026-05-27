package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerScrollRack wires Scroll Rack.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Scroll%20Rack):
//
//	{1}, {T}: Exile any number of cards from your hand face down. Put
//	that many cards from the top of your library into your hand. Then
//	look at the exiled cards and put them on top of your library in
//	any order.
//
// {2} Artifact. Premier hand-sculpting tool — swaps stuck cards (lands
// or low-impact draws mid-game) for fresh draws from the top of the
// library. The swapped-out cards then queue up as the next N draws.
//
// Implementation:
//   - OnActivated(ability 0): {1}, {T} mana-then-tap cost; pick N
//     "worst" cards from hand using sylvanCardKeepScore as the cut
//     heuristic (lands and 1-CMC cantrips go first); exile them; draw
//     N from top of library; place the exiled N back on top.
//   - N is capped at min(handSize, librarySize, 4). The cap avoids
//     emptying the hand and keeps the swap behavior conservative for
//     fuzz / parity stability.
//   - Heuristic for "any order": exiled cards return in DRAW-ORDER
//     priority — best of the worst on top so the next draw is the
//     most useful of the cards we shipped out. In practice the cards
//     we exile are similar-quality so ordering is mostly cosmetic.
func registerScrollRack(r *Registry) {
	r.OnActivated("Scroll Rack", scrollRackActivate)
}

func scrollRackActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "scroll_rack"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, "Scroll Rack", "already_tapped", nil)
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	if s.ManaPool < 1 {
		emitFail(gs, slug, "Scroll Rack", "insufficient_mana", nil)
		return
	}
	if len(s.Hand) == 0 || len(s.Library) == 0 {
		emitFail(gs, slug, "Scroll Rack", "no_hand_or_library", nil)
		return
	}

	// Pay {1}, tap.
	s.ManaPool -= 1
	gameengine.SyncManaAfterSpend(s)
	src.Tapped = true

	// Pick N cards to exile. Cap conservatively at min(hand, library, 4).
	// Pick by ascending sylvanCardKeepScore — worst first.
	maxN := len(s.Hand)
	if len(s.Library) < maxN {
		maxN = len(s.Library)
	}
	if maxN > 4 {
		maxN = 4
	}

	// Rank hand cards by ascending keep-score (worst first).
	type ranked struct {
		card  *gameengine.Card
		score int
	}
	ranks := make([]ranked, 0, len(s.Hand))
	for _, c := range s.Hand {
		if c == nil {
			continue
		}
		ranks = append(ranks, ranked{card: c, score: sylvanCardKeepScore(c)})
	}
	// Selection sort ascending.
	for i := 0; i < len(ranks)-1; i++ {
		for j := i + 1; j < len(ranks); j++ {
			if ranks[j].score < ranks[i].score {
				ranks[i], ranks[j] = ranks[j], ranks[i]
			}
		}
	}

	// Choose N: take only cards that score <= 1 (lands + cheap cantrips),
	// capped at maxN. If none qualify, exile a minimum of 1 to still
	// dig (matches "scroll for a key card" play pattern).
	picks := []*gameengine.Card{}
	for _, r := range ranks {
		if len(picks) >= maxN {
			break
		}
		if r.score <= 1 {
			picks = append(picks, r.card)
		}
	}
	if len(picks) == 0 && maxN > 0 {
		picks = append(picks, ranks[0].card)
	}
	if len(picks) == 0 {
		emitFail(gs, slug, "Scroll Rack", "no_cards_to_exile", nil)
		return
	}

	// Exile picks from hand.
	for _, c := range picks {
		gameengine.MoveCard(gs, c, seat, "hand", "exile", "scroll_rack_exile")
	}

	// Draw N from top of library.
	drawn := []*gameengine.Card{}
	for i := 0; i < len(picks); i++ {
		c := drawOne(gs, seat, "Scroll Rack")
		if c != nil {
			drawn = append(drawn, c)
		}
	}

	// Put the exiled cards back on top of library. Ordering: highest
	// keep-score on top so the next draw is the best of what we sent
	// out. We need to MoveCard them in REVERSE order of desired top
	// position — MoveCard to "library_top" prepends, so the LAST move
	// ends up on top.
	sorted := append([]*gameengine.Card(nil), picks...)
	// Sort ascending by score so HIGHEST score ends up last → on top.
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sylvanCardKeepScore(sorted[j]) < sylvanCardKeepScore(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for _, c := range sorted {
		gameengine.MoveCard(gs, c, seat, "exile", "library_top", "scroll_rack_return")
	}

	emit(gs, slug, "Scroll Rack", map[string]interface{}{
		"seat":     seat,
		"swapped":  len(picks),
		"drew":     len(drawn),
		"hand_now": len(s.Hand),
	})
}
