package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTasigurTheGoldenFangCustom replaces the auto-gen mill-only stub
// with the full activated ability. The previous gen_*.go body paid {4}
// + milled 2 then stopped, dropping the return-from-graveyard half (the
// entire reason to play Tasigur). The auto-gen function is neutered in
// the gen file so this is the sole registration site.
//
// Oracle text (Khans of Tarkir, {2}{S}{B}{G}; alt {1}{B}{G} via delve):
//
//	Delve (Each card you exile from your graveyard while casting this
//	spell pays for {1}.)
//	{2}{G/U}{G/U}: Mill two cards, then return a nonland card of an
//	opponent's choice from your graveyard to your hand.
//
// Delve cast-time cost reduction lives in
// internal/gameengine/keywords_delve_cast.go (CastWithDelve). The
// auto-cast Hat does not yet choose delve counts — Heimdall replays
// and Muninn parity DO pick it up via the IsDelveCast predicate, and
// the per_card cast pipeline (when the Hat is wired) can call it
// directly.
//
// Activated ability:
//   - Cost gate: {2}{G/U}{G/U} = 4 generic, hybrid pips paid out of
//     the generic mana pool (matches the gen stub's PayGenericCost
//     shape, since the engine doesn't track G-vs-U-vs-hybrid pip
//     differentiation at this layer).
//   - Mill two cards from the activator's library to graveyard.
//   - Pick an opponent (CR §603.3a APNAP — for simulation we use the
//     next-living-opponent-clockwise; a real game would offer the
//     choice to the activator, but the opponent that actually CHOOSES
//     is canonically "an opponent" with no further specification).
//   - That opponent's adversarial heuristic: among nonland cards in
//     the activator's graveyard, return the LOWEST-CMC card to the
//     activator's hand. (Returning the strongest card would be a
//     gift; the opponent gives Tasigur's controller the weakest
//     recursion target available.)
//   - If only lands are in graveyard OR the graveyard is empty after
//     mill, emit a "no_legal_return" partial and skip the return half.
func registerTasigurTheGoldenFangCustom(r *Registry) {
	r.OnActivated("Tasigur, the Golden Fang", tasigurTheGoldenFangActivateCustom)
}

func tasigurTheGoldenFangActivateCustom(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "tasigur_the_golden_fang_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seatIdx]
	if s == nil || s.Lost {
		return
	}

	// Cost gate. The gen stub paid {2}{G/U}{G/U} = 4 generic; preserve
	// that arithmetic exactly so existing tests that don't track hybrid
	// color identity still pass.
	const manaCost = 4
	if !gameengine.PayGenericCost(gs, s, manaCost, "activated", "tasigur_activate", src.Card.DisplayName()) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": s.ManaPool,
			"mana_cost": manaCost,
		})
		return
	}

	// Mill 2.
	milled := 0
	for i := 0; i < 2; i++ {
		if len(s.Library) == 0 {
			break
		}
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seatIdx, "library", "graveyard", "mill")
		milled++
	}

	// "An opponent" chooses — pick the first living opponent
	// clockwise. Skipping lost seats matches every other adversarial-
	// choice site (Gisa, The Reaper King, etc.).
	opponentSeat := -1
	for offset := 1; offset < len(gs.Seats); offset++ {
		candidate := (seatIdx + offset) % len(gs.Seats)
		opp := gs.Seats[candidate]
		if opp == nil || opp.Lost {
			continue
		}
		opponentSeat = candidate
		break
	}
	if opponentSeat < 0 {
		// No opponents alive — activator's about to win anyway.
		// Skip the return half cleanly.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seatIdx,
			"mana_paid": manaCost,
			"milled":    milled,
			"returned":  false,
			"reason":    "no_living_opponent",
		})
		return
	}

	// Opponent picks the LOWEST-CMC nonland card from activator's
	// graveyard. Tie-break: earliest-added (most-buried) so the
	// opponent's pick is stable and reproducible. Lands are ineligible
	// per the oracle wording.
	var pick *gameengine.Card
	pickIdx := -1
	for i, c := range s.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			continue
		}
		if pick == nil || c.CMC < pick.CMC {
			pick = c
			pickIdx = i
		}
	}
	if pick == nil {
		// Only lands (or empty) — no legal return target.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":          seatIdx,
			"mana_paid":     manaCost,
			"milled":        milled,
			"opponent_seat": opponentSeat,
			"returned":      false,
			"reason":        "no_nonland_in_graveyard",
		})
		return
	}
	_ = pickIdx // MoveCard locates the card by pointer; index is for diagnostic logs only

	gameengine.MoveCard(gs, pick, seatIdx, "graveyard", "hand", "tasigur_return")

	gs.LogEvent(gameengine.Event{
		Kind:   "return_to_hand",
		Seat:   seatIdx,
		Target: seatIdx,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"card":            pick.DisplayName(),
			"from":            "graveyard",
			"reason":          "tasigur_activate_return",
			"chooser_seat":    opponentSeat,
			"return_cmc":      pick.CMC,
			"return_strategy": "opponent_picks_lowest_cmc_nonland",
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seatIdx,
		"mana_paid":     manaCost,
		"milled":        milled,
		"opponent_seat": opponentSeat,
		"returned":      true,
		"returned_card": pick.DisplayName(),
		"return_cmc":    pick.CMC,
	})
}
