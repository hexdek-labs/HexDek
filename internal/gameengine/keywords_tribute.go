package gameengine

// keywords_tribute.go — Tribute N (CR §702.104, Born of the Gods 2014)
// as a real two-callback ETB choice with per-card payoff routing.
//
// CR §702.104a: "Tribute N" is a static ability that modifies the
//                creature's ETB. As the creature enters the battlefield,
//                the controller chooses an opponent. That opponent
//                chooses whether N +1/+1 counters are put on the
//                creature.
// CR §702.104b: A card with tribute also has an additional ability
//                that begins "When this creature enters the battlefield,
//                if tribute wasn't paid …". That ability triggers when
//                tribute was refused — the "tribute-failed" payoff.
//
// Engine surface (three pieces):
//
//   1. ApplyTribute(gs, perm, controllerSeat, chooseOpp, decide) bool
//        Drives the §702.104a ETB choice. Returns true if tribute was
//        accepted (counters added). False if refused; per-card handlers
//        read WasTributeAccepted == false and fire their punishment
//        effect.
//
//   2. WasTributeAccepted(perm) bool / WasTributeRefused(perm) bool
//        Cards with tribute carry both an accept-path and a refuse-path
//        ability; the printed body is per-card and routed through the
//        per-card handler registry. These helpers expose the recorded
//        decision so handlers can branch.
//
//   3. HasTribute(card) / PermHasTribute(perm) / TributeAmount(card)
//        Keyword + amount detection.
//
// State is stored on perm.Flags:
//   - "tribute_resolved":  1 once ApplyTribute has run for this ETB
//   - "tribute_accepted":  1 when tribute was paid (counters added)
//   - "tribute_opponent":  opponent seat index + 1 (0 means "unset"
//                          since flags are int; we encode +1 to keep
//                          seat 0 distinguishable from "not set")
//
// The +1 encoding for tribute_opponent keeps the integer-only Flags
// shape; readers should use TributeOpponent(perm) which decodes it.

// ---------------------------------------------------------------------------
// HasTribute / TributeAmount
// ---------------------------------------------------------------------------

// HasTribute returns true if the card has the tribute keyword.
func HasTribute(card *Card) bool {
	return cardHasKeywordByName(card, "tribute")
}

// PermHasTribute is the battlefield-side counterpart.
func PermHasTribute(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	return HasTribute(perm.Card)
}

// TributeAmount returns the N value of the card's "Tribute N" keyword.
// 0 when the keyword is absent.
func TributeAmount(card *Card) int {
	return keywordArgCost(card, "tribute")
}

// ---------------------------------------------------------------------------
// State accessors
// ---------------------------------------------------------------------------

// WasTributeAccepted reports whether the opponent accepted tribute
// (added the N +1/+1 counters) when this permanent ETB'd. False both
// for "tribute was refused" AND for "ApplyTribute hasn't run yet" —
// callers that need to distinguish those should also check
// TributeResolved.
func WasTributeAccepted(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["tribute_accepted"] > 0
}

// WasTributeRefused reports whether the opponent refused tribute. This
// is the gate the per-card "tribute-failed" effect handler reads —
// it's true exactly when ApplyTribute ran AND the opponent said no.
func WasTributeRefused(perm *Permanent) bool {
	return TributeResolved(perm) && !WasTributeAccepted(perm)
}

// TributeResolved reports whether ApplyTribute has run for this
// permanent's current ETB. Distinguishes "never resolved" from
// "resolved as refused."
func TributeResolved(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["tribute_resolved"] > 0
}

// TributeOpponent returns the seat index of the opponent the
// controller chose to decide tribute, or -1 if tribute hasn't been
// resolved.
func TributeOpponent(perm *Permanent) int {
	if perm == nil || perm.Flags == nil {
		return -1
	}
	encoded := perm.Flags["tribute_opponent"]
	if encoded == 0 {
		return -1
	}
	return encoded - 1
}

// ---------------------------------------------------------------------------
// ApplyTribute — the §702.104a ETB choice
// ---------------------------------------------------------------------------

// TributeChooseOpponent is the callback the controller uses to pick
// which opponent will decide tribute. Returns the seat index. A return
// value that isn't a valid opponent (the controller themselves, an
// out-of-range seat, or a Lost seat) falls back to the leftmost living
// opponent.
type TributeChooseOpponent func() int

// TributeDecide is the callback the chosen opponent uses to accept
// (true → put N counters on the creature) or refuse (false → don't,
// and let the printed "tribute-failed" effect fire via the per-card
// handler).
type TributeDecide func(opponentSeat int) bool

// ApplyTribute drives the §702.104a ETB choice for a tribute creature.
//
// Flow:
//   1. Read N from TributeAmount(perm.Card). If N <= 0 (keyword absent
//      or malformed) the function records "tribute_resolved" with the
//      accepted flag false — so per-card handlers don't dispatch their
//      tribute-failed effect for cards that don't actually have the
//      keyword.
//   2. Build the opponent list via gs.Opponents(controllerSeat),
//      filtering Lost seats. With no eligible opponents the function
//      records "resolved + refused" and returns false (tribute can't
//      be paid; the per-card "if tribute wasn't paid" rider may or
//      may not fire depending on §702.104b interpretation — the
//      printed cards all word it as "if tribute wasn't paid," which
//      includes the no-opponent case).
//   3. Call chooseOpp() (when non-nil) to pick the deciding opponent.
//      An invalid choice falls back to opponents[0].
//   4. Call decide(opponentSeat) — true accepts (counters added),
//      false refuses.
//   5. Stash the decision on perm.Flags; emit a "tribute" event;
//      fire FireCardTrigger("tribute_resolved", ctx) for any per-card
//      handler that wants a tribute-pivot hook.
//
// Returns the boolean the decide callback returned (or false when
// fallback paths short-circuit).
//
// CR §702.104b's "tribute wasn't paid" rider lives on the printed
// card body and is the per-card handler's responsibility — this
// function only RECORDS the decision. Per-card handlers gate on
// WasTributeRefused inside their ETB handler.
func ApplyTribute(
	gs *GameState,
	perm *Permanent,
	controllerSeat int,
	chooseOpp TributeChooseOpponent,
	decide TributeDecide,
) bool {
	if gs == nil || perm == nil || perm.Card == nil {
		return false
	}
	if controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return false
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}

	n := TributeAmount(perm.Card)
	// Mark resolved up-front so WasTributeRefused returns true even
	// when bail-out paths refuse (no opponent, keyword malformed).
	perm.Flags["tribute_resolved"] = 1
	perm.Flags["tribute_accepted"] = 0

	if n <= 0 {
		gs.LogEvent(Event{
			Kind:   "tribute_skipped",
			Seat:   controllerSeat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "no_tribute_keyword_or_zero_n",
				"rule":   "702.104a",
			},
		})
		return false
	}

	// Gather eligible opponents.
	var eligible []int
	for _, idx := range gs.Opponents(controllerSeat) {
		if idx < 0 || idx >= len(gs.Seats) {
			continue
		}
		if s := gs.Seats[idx]; s != nil && !s.Lost {
			eligible = append(eligible, idx)
		}
	}
	if len(eligible) == 0 {
		gs.LogEvent(Event{
			Kind:   "tribute",
			Seat:   controllerSeat,
			Source: perm.Card.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"resolved": true,
				"accepted": false,
				"reason":   "no_living_opponents",
				"rule":     "702.104a",
			},
		})
		fireTributeResolved(gs, perm, controllerSeat, -1, n, false)
		return false
	}

	// Controller chooses the deciding opponent (CR §702.104a — "an
	// opponent of your choice").
	chosen := eligible[0]
	if chooseOpp != nil {
		if c := chooseOpp(); c != controllerSeat && c >= 0 && c < len(gs.Seats) {
			if s := gs.Seats[c]; s != nil && !s.Lost {
				// Verify it's actually an opponent (Opponents already
				// filters dead seats but a custom callback may try to
				// pick a teammate in archenemy/two-headed-giant
				// variants — for the MVP we require gs.Opponents
				// membership).
				for _, opp := range eligible {
					if opp == c {
						chosen = c
						break
					}
				}
			}
		}
	}
	perm.Flags["tribute_opponent"] = chosen + 1 // +1 encoding (0 = unset)

	accepted := false
	if decide != nil {
		accepted = decide(chosen)
	}

	if accepted {
		perm.Flags["tribute_accepted"] = 1
		perm.AddCounter("+1/+1", n)
		gs.InvalidateCharacteristicsCache()
	}

	gs.LogEvent(Event{
		Kind:   "tribute",
		Seat:   controllerSeat,
		Target: chosen,
		Source: perm.Card.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"resolved":      true,
			"accepted":      accepted,
			"opponent_seat": chosen,
			"counters":      n,
			"rule":          "702.104a",
		},
	})

	fireTributeResolved(gs, perm, controllerSeat, chosen, n, accepted)
	return accepted
}

// fireTributeResolved fans out a per-card hook so per-card handlers
// (e.g. Fanatic of Xenagos's "when it ETBs, if tribute wasn't paid,
// you may have it deal damage equal to its power to target creature")
// can dispatch their tribute-failed effects through the same trigger
// channel they use for other ETBs.
func fireTributeResolved(gs *GameState, perm *Permanent, controllerSeat, opponentSeat, n int, accepted bool) {
	name := ""
	if perm != nil && perm.Card != nil {
		name = perm.Card.DisplayName()
	}
	FireCardTrigger(gs, "tribute_resolved", map[string]interface{}{
		"perm":            perm,
		"card_name":       name,
		"controller_seat": controllerSeat,
		"opponent_seat":   opponentSeat,
		"tribute_n":       n,
		"accepted":        accepted,
	})
}
