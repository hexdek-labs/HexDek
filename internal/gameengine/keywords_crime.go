package gameengine

// keywords_crime.go — Crime (CR §701.71, Murders at Karlov Manor 2024) as a
// real keyword surface backing "whenever you commit a crime" and
// "when you commit your first crime each turn" triggers.
//
// CR §701.71a: A player "commits a crime" when they cast a spell or
//                activate an ability that targets at least one of:
//                  • a permanent an opponent controls
//                  • a spell or ability an opponent controls (on the
//                    stack)
//                  • a card in an opponent's graveyard, exile, hand,
//                    or library
// CR §701.71b: An ability that triggers when a player commits a crime
//                triggers ONCE per crime, even if the spell/ability
//                targets multiple legal opponent-controlled objects.
//                "When you commit your first crime each turn" only
//                fires on the FIRST crime in a given turn.
//
// Engine surface:
//
//   - HasCommitsCrimeTrigger(card) bool
//       Oracle-text detector. True when the card's oracle contains a
//       "whenever you commit a crime" or "when you commit your first
//       crime each turn" pattern.
//
//   - FireCommitsCrimeTriggers(gs, seatIdx, source, target)
//       Public entry point. Called from the spell/ability targeting
//       path when a target qualifies as "opponent-controlled." Bumps
//       Turn.CommittedCrimes, emits a "commit_crime" event, fires
//       FireCardTrigger("crime", ctx) with structured context, and
//       fires "first_crime" once per turn for the first crime only.
//
//   - SeatHasCommittedCrimeThisTurn(gs, seatIdx) bool
//   - SeatCrimeCountThisTurn(gs, seatIdx) int
//       Accessors for "did you commit a crime this turn"-gated cards.
//
//   - IsCrimeTarget(gs, seatIdx, target) bool
//       Predicate that tells callers whether a single Target qualifies
//       as a crime when targeted by `seatIdx`. Used by the cast/activation
//       targeting pipeline to decide whether to call
//       FireCommitsCrimeTriggers.
//
// Integration is the responsibility of the cast/activation targeting
// pipeline (resolve.go's maybeFireCrime + counterspell branch + future
// activated-ability targeting paths). This file provides the canonical
// API and the per-turn book-keeping; the existing maybeFireCrime helper
// now delegates to FireCommitsCrimeTriggers so legacy callers continue
// to work.

import "strings"

// ---------------------------------------------------------------------------
// HasCommitsCrimeTrigger
// ---------------------------------------------------------------------------

// HasCommitsCrimeTrigger scans the card's oracle text (lowercased,
// cached via OracleTextLower) for a "commit a crime" or "commit your
// first crime each turn" trigger preamble. Used by tooling and tests to
// identify cards with the keyword without rerunning the full AST
// parser.
func HasCommitsCrimeTrigger(card *Card) bool {
	if card == nil {
		return false
	}
	text := strings.ToLower(OracleTextLower(card))
	if text == "" {
		return false
	}
	// Canonical Murders-at-Karlov-Manor / OTJ phrasings:
	patterns := []string{
		"whenever you commit a crime",
		"when you commit a crime",
		"when you commit your first crime each turn",
		"whenever you commit your first crime each turn",
		"if you committed a crime this turn",
		"if you've committed a crime this turn",
		"committed a crime this turn",
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// IsCrimeTarget
// ---------------------------------------------------------------------------

// IsCrimeTarget reports whether targeting `t` while `seatIdx` is the
// active caster qualifies as committing a crime under CR §701.71a.
// Returns false for self-targets, ally targets (in multiplayer team
// variants — currently equates to non-self by seat index), and empty
// targets.
func IsCrimeTarget(gs *GameState, seatIdx int, t Target) bool {
	if gs == nil {
		return false
	}
	switch t.Kind {
	case TargetKindSeat:
		// Targeting an opponent directly (player target).
		return t.Seat != seatIdx && t.Seat >= 0
	case TargetKindPermanent:
		if t.Permanent == nil {
			return false
		}
		// CR §701.71a: a permanent an opponent controls.
		return t.Permanent.Controller != seatIdx && t.Permanent.Controller >= 0
	case TargetKindStackItem:
		if t.Stack == nil {
			return false
		}
		// CR §701.71a: a spell or ability an opponent controls.
		return t.Stack.Controller != seatIdx && t.Stack.Controller >= 0
	case TargetKindCard:
		// CR §701.71a: a card in an opponent's graveyard, exile,
		// hand, or library. Target.Seat holds the OWNER of that zone.
		return t.Seat != seatIdx && t.Seat >= 0
	}
	return false
}

// ---------------------------------------------------------------------------
// FireCommitsCrimeTriggers
// ---------------------------------------------------------------------------

// FireCommitsCrimeTriggers is the canonical entry point for emitting a
// "you commit a crime" event. CR §701.71a/b — fires ONCE per crime
// regardless of how many opponent-controlled objects the spell/ability
// targets (callers gate the per-resolution dedup themselves; this helper
// records ONE crime per invocation).
//
// `source` is the human-readable name of the spell or ability that
// committed the crime (e.g. "Murder", "Vraska, Betrayal's Sting").
// `target` is a human-readable identifier for the opponent-controlled
// thing that was targeted — used for log attribution only.
//
// Effects:
//   - Bumps gs.Seats[seatIdx].Turn.CommittedCrimes (the per-turn count).
//   - Emits a "commit_crime" log event with seat, source, target, count.
//   - Fires FireCardTrigger("crime", ctx) for any "whenever you commit
//     a crime" observer in the per-card registry.
//   - On the FIRST crime of the turn (count == 1) ALSO fires
//     FireCardTrigger("first_crime", ctx) for the "first crime each
//     turn" modal — these cards trigger only once even if multiple
//     crimes happen.
//
// Returns the new per-turn crime count for convenience (so callers
// that want to gate on "this is the Nth crime" can branch off the
// return value).
func FireCommitsCrimeTriggers(gs *GameState, seatIdx int, source, target string) int {
	if gs == nil {
		return 0
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	seat.Turn.CommittedCrimes++
	count := seat.Turn.CommittedCrimes

	gs.LogEvent(Event{
		Kind:   "commit_crime",
		Seat:   seatIdx,
		Source: source,
		Amount: count,
		Details: map[string]interface{}{
			"target":       target,
			"crime_count":  count,
			"rule":         "701.71a",
		},
	})

	FireCardTrigger(gs, "crime", map[string]interface{}{
		"seat":        seatIdx,
		"source":      source,
		"target":      target,
		"crime_count": count,
	})

	// Generic AST dispatch for "whenever you commit a crime" triggers that
	// have no bespoke per_card handler (Hardbristle Bandit, Marauding Sphinx,
	// Vadmir, …). FireCardTrigger above reaches the per_card registry only;
	// without this walk the AST trigger's effect never resolved. The
	// dispatcher self-skips per_card-owned cards, so no double-fire.
	FireCommitCrimeASTTriggers(gs, seatIdx)

	if count == 1 {
		FireCardTrigger(gs, "first_crime", map[string]interface{}{
			"seat":   seatIdx,
			"source": source,
			"target": target,
		})
	}

	return count
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

// SeatHasCommittedCrimeThisTurn returns true if `seatIdx` has
// committed at least one crime since the most recent UntapAll. Backs
// "if you've committed a crime this turn"-gated card text.
func SeatHasCommittedCrimeThisTurn(gs *GameState, seatIdx int) bool {
	return SeatCrimeCountThisTurn(gs, seatIdx) > 0
}

// SeatCrimeCountThisTurn returns the per-turn crime count for
// `seatIdx`. 0 when the seat hasn't acted yet or the count was reset.
func SeatCrimeCountThisTurn(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return 0
	}
	return seat.Turn.CommittedCrimes
}

// ---------------------------------------------------------------------------
// Internal: bulk-target crime check
// ---------------------------------------------------------------------------

// FireCrimeIfTargetingOpponent is a convenience that scans a slice of
// resolved targets and fires ONE crime if any of them qualify. This
// matches the cast/ability resolution shape — a single spell with
// multiple targets commits at most one crime per resolution
// (CR §701.71b). Returns true if a crime fired.
//
// Used by the existing maybeFireCrime helper in resolve.go (refactored
// to delegate here) and by per-card handlers that want to share the
// targeting-driven crime detection.
func FireCrimeIfTargetingOpponent(gs *GameState, seatIdx int, sourceName string, targets []Target) bool {
	if gs == nil || len(targets) == 0 {
		return false
	}
	for _, t := range targets {
		if !IsCrimeTarget(gs, seatIdx, t) {
			continue
		}
		FireCommitsCrimeTriggers(gs, seatIdx, sourceName, crimeTargetName(t))
		return true
	}
	return false
}

// crimeTargetName produces a human-readable identifier for a Target,
// used in the commit_crime event's "target" detail.
func crimeTargetName(t Target) string {
	switch t.Kind {
	case TargetKindSeat:
		return "opponent_seat_" + itoa(t.Seat)
	case TargetKindPermanent:
		if t.Permanent != nil && t.Permanent.Card != nil {
			return t.Permanent.Card.DisplayName()
		}
		return "permanent"
	case TargetKindStackItem:
		if t.Stack != nil && t.Stack.Card != nil {
			return t.Stack.Card.DisplayName()
		}
		return "stack_item"
	case TargetKindCard:
		if t.Card != nil {
			return t.Card.DisplayName()
		}
		return "card"
	}
	return ""
}
