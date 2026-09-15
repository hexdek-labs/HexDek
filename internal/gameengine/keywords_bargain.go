package gameengine

// keywords_bargain.go — Bargain (CR §702.166, Wilds of Eldraine /
// Lost Caverns of Ixalan 2023) as a real explicit-target additional
// cost wrapping the cast pipeline.
//
// CR §702.166a: Bargain is an optional additional cost on a spell.
//                As you cast a spell with bargain, you may sacrifice
//                an artifact, enchantment, or token. If you do, the
//                spell may have a bonus effect (per-card "if you
//                bargained" rider).
// CR §702.166b: The sacrifice is part of casting the spell. It happens
//                BEFORE the spell resolves; cards that count graveyards
//                / dies-triggers see the sacrificed permanent's
//                arrival at the graveyard before the bargain spell's
//                effect runs.
// CR §702.166c: Whether bargain was paid is recorded on the stack
//                item so resolution-time effects can branch on it.
//
// Engine surface (canonical):
//
//   - HasBargain(card) bool
//       AST keyword detector. CR §702.166a — the parser emits
//       "Bargain —" as a Keyword ability named "bargain".
//
//   - CanBargain(gs, seat) bool
//       Whether `seat` controls at least one artifact, enchantment,
//       or token they could sacrifice as the bargain cost.
//
//   - EligibleBargainTargets(gs, seat) []*Permanent
//       All permanents the seat controls that satisfy §702.166a's
//       "artifact, enchantment, or token" filter. The Hat picks one
//       (e.g. via findBargainCandidate's cheapest-first policy) and
//       passes it to CastWithBargain.
//
//   - CastWithBargain(gs, seat, card, sacTarget) error
//       Atomic validation + sacrifice + stack-item push. Validates
//       the card has bargain, the sacTarget is non-nil + controlled
//       by `seat` + an artifact/enchantment/token. On success:
//         1. Sacrifices the target via SacrificePermanent (CR §702.166b
//            — sacrifice fires the canonical dies/LTB triggers).
//         2. Pushes a StackItem flagged CostMeta["bargained"]=true and
//            CostMeta["bargain_target"]=<card name> so resolution-time
//            handlers can read it.
//         3. Emits a "bargain" log event.
//
//       The actual MANA payment for the spell is NOT handled here —
//       this helper is for the bargain-side cost only. Cast pipelines
//       that want a full cast-with-bargain flow combine CastWithBargain
//       with their normal cast-cost payment (the existing
//       BargainAdditionalCost factory in costs.go feeds the legacy
//       CastSpellWithCosts path; this canonical helper is the
//       hand-rolled equivalent for callers that want fine control).
//
// CastWithoutBargain is implicit — a caller that wants to cast a bargain
// spell without paying the optional cost simply uses the normal cast
// path; CostMeta["bargained"] is then absent (or false), which is the
// state that "if you didn't bargain" riders read.

// ---------------------------------------------------------------------------
// HasBargain
// ---------------------------------------------------------------------------

// HasBargain returns true if the card has the bargain keyword.
// CR §702.166a — the parser emits "Bargain —" preambles as a Keyword
// ability with name "bargain".
func HasBargain(card *Card) bool {
	return cardHasKeywordByName(card, "bargain")
}

// ---------------------------------------------------------------------------
// CanBargain / EligibleBargainTargets
// ---------------------------------------------------------------------------

// CanBargain reports whether `seat` controls at least one permanent
// they could sacrifice to satisfy a bargain cost (CR §702.166a:
// artifact, enchantment, or token). False when no eligible permanent
// is on the battlefield — used by cast-policy code to decide whether
// the optional cost is even available.
func CanBargain(gs *GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	for _, p := range seat.Battlefield {
		if isBargainEligible(p, seatIdx) {
			return true
		}
	}
	return false
}

// EligibleBargainTargets returns every permanent `seat` controls that
// qualifies as a bargain sacrifice. The Hat layer picks one (the
// existing findBargainCandidate uses cheapest-CMC greedy policy);
// callers that want to surface all options for player choice can use
// this list directly.
func EligibleBargainTargets(gs *GameState, seatIdx int) []*Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	var out []*Permanent
	for _, p := range seat.Battlefield {
		if isBargainEligible(p, seatIdx) {
			out = append(out, p)
		}
	}
	return out
}

// isBargainEligible centralizes the §702.166a "artifact, enchantment,
// or token controlled by you" filter. Used by all the bargain helpers
// so the rules stay in one place.
func isBargainEligible(p *Permanent, seatIdx int) bool {
	if p == nil || p.Card == nil {
		return false
	}
	if p.Controller != seatIdx {
		return false
	}
	return p.IsArtifact() || p.IsEnchantment() || p.IsToken()
}

// ---------------------------------------------------------------------------
// CastWithBargain
// ---------------------------------------------------------------------------

// CastWithBargain runs the §702.166a bargain-cost activation for a
// spell `card` being cast by `seatIdx`. CR §702.166b — the sacrifice
// is part of paying the cost, fires the canonical dies/LTB triggers,
// and the resulting stack item is flagged CostMeta["bargained"] = true
// so resolution-time per-card handlers can branch.
//
// Validation (atomic — no mutation happens on failure):
//
//   - card non-nil and carries the bargain keyword
//   - sacTarget non-nil
//   - sacTarget controlled by `seatIdx` (§702.166a "you may sacrifice")
//   - sacTarget is an artifact, enchantment, or token
//
// On success:
//
//   - sacTarget is sacrificed via SacrificePermanent (fires
//     dies/LTB triggers, handles tokens vanishing, emits sacrifice
//     event with reason="bargain_cost")
//   - A StackItem is pushed with CostMeta["bargained"] = true,
//     CostMeta["bargain_target"] = <sacrificed card name>,
//     CastZone = "hand", Effect = collectSpellEffect(card)
//   - A "bargain" log event is emitted
//   - FireCardTrigger("bargain_paid", ctx) fires for per-card observers
//
// Mana cost for the spell itself is NOT handled here — the caller's
// cast pipeline pays the mana before/around this helper. CastWithBargain
// is the bargain-side cost only; it produces a StackItem that the
// caller can finish wiring into the cast pipeline.
//
// Returns nil on success, *CastError on failure.
func CastWithBargain(gs *GameState, seatIdx int, card *Card, sacTarget *Permanent) error {
	if gs == nil {
		return &CastError{Reason: "nil_game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid_seat"}
	}
	if card == nil {
		return &CastError{Reason: "nil_card"}
	}
	if !HasBargain(card) {
		return &CastError{Reason: "no_bargain_keyword"}
	}
	if sacTarget == nil {
		return &CastError{Reason: "nil_bargain_target"}
	}
	if !isBargainEligible(sacTarget, seatIdx) {
		// Distinguish the controller mismatch from the type mismatch in
		// the error so callers can surface useful diagnostics.
		if sacTarget.Controller != seatIdx {
			return &CastError{Reason: "bargain_target_not_controlled"}
		}
		return &CastError{Reason: "bargain_target_not_artifact_enchantment_or_token"}
	}

	sacName := ""
	if sacTarget.Card != nil {
		sacName = sacTarget.Card.DisplayName()
	}

	// 1. Sacrifice (CR §702.166b — part of the cost, runs before the
	// spell resolves so dies/LTB triggers + graveyard-counting observers
	// see the cost-payment artifact's exit first).
	SacrificePermanent(gs, sacTarget, "bargain_cost")

	// 2. Push stack item flagged as bargained.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"bargained":      true,
			"bargain_target": sacName,
		},
	}
	PushStackItem(gs, item)

	// 3. Log + trigger fan-out.
	name := card.DisplayName()
	gs.LogEvent(Event{
		Kind:   "bargain",
		Seat:   seatIdx,
		Source: name,
		Details: map[string]interface{}{
			"sacrificed": sacName,
			"rule":       "702.166a",
		},
	})
	FireCardTrigger(gs, "bargain_paid", map[string]interface{}{
		"card":            card,
		"card_name":       name,
		"controller_seat": seatIdx,
		"sacrificed":      sacName,
	})

	return nil
}
