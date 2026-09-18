package gameengine

// keywords_cr_aug_r64.go — primitives for keyword actions / abilities and
// defined terms added in the Aug 7 2026 Comprehensive Rules edition. Each is a
// small, reusable rule primitive (the modular rule-focused approach); wiring
// the AST nodes that invoke them is tracked in docs/cr-aug-impact-scope.md.

// HealDamage removes marked damage from a permanent (CR §701.69a). "Heal N
// damage" removes up to n; n <= 0 removes ALL marked damage (the "damage ...
// is healed" form). Marked damage is wiped at end of turn regardless; this is
// the mid-turn heal action.
func HealDamage(perm *Permanent, n int) {
	if perm == nil {
		return
	}
	if n <= 0 || n >= perm.MarkedDamage {
		perm.MarkedDamage = 0
		return
	}
	perm.MarkedDamage -= n
}

// IsWorthy reports whether a card is a "worthy" creature (CR §700.16, Aug 7
// 2026 edition): a creature that is legendary, isn't a Villain, and is red
// and/or white.
//
// NOTE: the "isn't a Villain" clause is enforced but currently inert — the
// engine does not yet tag the Villain card type, so no card matches it. Once
// Villain typing exists this predicate excludes them with no further change.
func IsWorthy(c *Card) bool {
	if c == nil {
		return false
	}
	if !cardHasType(c, "creature") || !cardHasType(c, "legendary") {
		return false
	}
	if cardHasType(c, "villain") {
		return false
	}
	return cardHasColor(c, "R") || cardHasColor(c, "W")
}


// PerformRecruit executes the Recruit keyword action (CR §701.70a): draw a
// card, then discard a card; if a NONLAND card was discarded this way, create
// a 1/1 white Human Soldier creature token. Composes DrawN + the discard
// choice + CreateCreatureToken (the modular-reuse approach).
func PerformRecruit(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	DrawN(gs, seatIdx, 1, nil)
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Hand) == 0 {
		return
	}
	// Choose the card to discard: the Hat picks; fall back to the first card
	// in hand when no Hat is attached (CR §701.8 — the player chooses).
	var toDiscard *Card
	if seat.Hat != nil {
		if picks := seat.Hat.ChooseDiscard(gs, seatIdx, seat.Hand, 1); len(picks) > 0 {
			toDiscard = picks[0]
		}
	}
	if toDiscard == nil {
		toDiscard = seat.Hand[0]
	}
	wasNonland := !cardHasType(toDiscard, "land")
	DiscardCard(gs, toDiscard, seatIdx)
	if wasNonland {
		if tok := CreateCreatureToken(gs, seatIdx, "Human Soldier",
			[]string{"creature", "human", "soldier"}, 1, 1); tok != nil && tok.Card != nil {
			tok.Card.Colors = []string{"W"}
		}
	}
}


// TapCreaturesForTotalPower taps the given creatures to pay a "tap creatures
// with total power N or more" cost — the shared shape behind crew (§702.122),
// saddle (§702.171), and the new Teamwork keyword (§702.194a: "As an
// additional cost to cast this spell, you may tap any number of creatures you
// control with total power N or more"). It validates that every creature is an
// untapped creature the seat controls and that their combined power is >= need,
// then taps them all. Returns false and taps nothing if the set is invalid or
// insufficient. Summoning-sick creatures MAY be used — this is a cost-payment
// tap, not an attack (mirrors crew/saddle).
//
// Teamwork's cast-time wiring (offering this optional additional cost in the
// §601.2f payment step and exposing the tapped-count to the card's payoff) is
// the integration layer tracked in docs/cr-aug-impact-scope.md; this is the
// reusable primitive it will call.
func TapCreaturesForTotalPower(gs *GameState, seatIdx int, need int, chosen []*Permanent) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || need <= 0 {
		return false
	}
	total := 0
	for _, c := range chosen {
		if c == nil || !c.IsCreature() || c.Controller != seatIdx || c.Tapped {
			return false
		}
		total += c.Power()
	}
	if total < need {
		return false
	}
	for _, c := range chosen {
		c.Tapped = true
	}
	return true
}
