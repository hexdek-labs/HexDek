package gameengine

// keywords_demonstrate.go — Demonstrate (CR §702.144, Strixhaven 2021).
//
// CR §702.144a: Demonstrate is a triggered ability. "Demonstrate" means
//               "When you cast this spell, you may copy it. If you do,
//               choose an opponent to also copy it."
// CR §702.144b: A copy made via demonstrate is created on the stack
//               above the original spell. Per CR §707.10, the copy is
//               not "cast" — it's created directly on the stack. A
//               copy of a permanent spell becomes a token when it
//               resolves; a copy of an instant or sorcery resolves and
//               then ceases to exist.
// CR §707.10c: The controller of each copy may choose new targets for
//               that copy. This implementation keeps the simple
//               default (same targets as the original) per the task
//               brief.
//
// API shape:
//
//   created := ApplyDemonstrate(
//       gs, originalStackItem, controllerSeat,
//       func() bool { /* opt-in? */ return true },
//       func() int  { /* which opponent? */ return 2 },
//   )
//
// Behaviour summary:
//   - optInCallback == nil → treated as "don't opt in" (no copies, returns 0)
//   - optInCallback returns false → no copies, returns 0
//   - optInCallback returns true →
//       1. mint a controller-controlled copy (StackItem on the stack
//          above the original)
//       2. resolve the opponent seat:
//            opponentChoiceCallback == nil → first living opponent in
//                                            turn order from controller's left
//            otherwise → callback's return value, validated as a
//                        living opponent of controllerSeat
//            no living opponent (all eliminated) → skip the opponent
//            copy; the controller copy still goes through
//       3. mint an opponent-controlled copy
//
// Returns the number of copies actually placed on the stack (0, 1, or
// 2). Each copy carries StackItem.IsCopy=true and Card.IsCopy=true
// per CR §707.10. Same Effect pointer and same Targets slice (cloned
// to avoid aliasing) as the original spell.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// DemonstrateOptIn is the per-cast opt-in decision. Returns true to
// copy the spell (which mints both controller and opponent copies).
type DemonstrateOptIn func() bool

// DemonstrateOpponentChoice returns the seat index of the opponent
// chosen to also copy the spell. The returned seat is validated as a
// living opponent of the controller; if invalid, ApplyDemonstrate
// falls back to the first living opponent.
type DemonstrateOpponentChoice func() int

// ---------------------------------------------------------------------------
// HasDemonstrate
// ---------------------------------------------------------------------------

// HasDemonstrate returns true if the card has the demonstrate keyword
// in its AST.
func HasDemonstrate(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "demonstrate") {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ApplyDemonstrate
// ---------------------------------------------------------------------------

// ApplyDemonstrate runs the demonstrate trigger for a freshly cast
// spell (`spell` is the original spell's StackItem already on the
// stack). CR §702.144a.
//
// Returns the number of copies created (0, 1, or 2). Side effects:
//   - emits "demonstrate_decline" when the controller opts out
//   - emits "demonstrate_opt_in" when the controller opts in
//   - emits "demonstrate_copy" per copy created (controller + opponent)
//   - pushes 0–2 StackItems onto gs.Stack above the original
func ApplyDemonstrate(
	gs *GameState,
	spell *StackItem,
	controllerSeat int,
	optInCallback DemonstrateOptIn,
	opponentChoiceCallback DemonstrateOpponentChoice,
) int {
	if gs == nil || spell == nil || spell.Card == nil {
		return 0
	}
	if controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return 0
	}
	if gs.Seats[controllerSeat] == nil || gs.Seats[controllerSeat].Lost {
		return 0
	}

	// Opt-in: nil callback ≡ decline (conservative default).
	if optInCallback == nil || !optInCallback() {
		gs.LogEvent(Event{
			Kind:   "demonstrate_decline",
			Seat:   controllerSeat,
			Source: spell.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule": "702.144a",
			},
		})
		return 0
	}

	gs.LogEvent(Event{
		Kind:   "demonstrate_opt_in",
		Seat:   controllerSeat,
		Source: spell.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.144a",
		},
	})

	created := 0

	// (1) Mint the controller copy.
	if pushDemonstrateCopy(gs, spell, controllerSeat, "controller") != nil {
		created++
	}

	// (2) Resolve the opponent seat. Per CR §702.144a, the controller
	// MUST choose an opponent if they opt in — but if all opponents are
	// eliminated, the choice has no legal target and the opponent copy
	// is skipped (the spell still copies for the controller).
	opps := gs.LivingOpponents(controllerSeat)
	if len(opps) == 0 {
		gs.LogEvent(Event{
			Kind:   "demonstrate_no_opponent",
			Seat:   controllerSeat,
			Source: spell.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule":   "702.144a",
				"reason": "no_living_opponents",
			},
		})
		return created
	}

	chosen := -1
	if opponentChoiceCallback != nil {
		choice := opponentChoiceCallback()
		if isLivingOpponent(opps, choice) {
			chosen = choice
		}
	}
	if chosen < 0 {
		// Default: first living opponent in turn order from controller's
		// left (matches gs.LivingOpponents semantics).
		chosen = opps[0]
	}

	if pushDemonstrateCopy(gs, spell, chosen, "opponent") != nil {
		created++
	}

	return created
}

// pushDemonstrateCopy builds and pushes one copy of `spell` controlled
// by `seat`. Returns the pushed StackItem, or nil on failure.
//
// Per CR §707.10, the copy has the same characteristics as the
// original (name, mana cost, types, colors, P/T, text — modeled by
// sharing the AST pointer and snapshotting the runtime Card scalar
// fields). The copy's controller is `seat`. Targets are copied from
// the original (CR §707.10c default — "same targets unless the
// controller of the copy chooses new ones"; the task brief specifies
// keeping the simple same-targets path).
//
// `role` is a logging hint — "controller" or "opponent" — surfaced in
// the demonstrate_copy event so replay/analytics can attribute each
// copy.
func pushDemonstrateCopy(gs *GameState, spell *StackItem, seat int, role string) *StackItem {
	if gs == nil || spell == nil || spell.Card == nil {
		return nil
	}
	if seat < 0 || seat >= len(gs.Seats) {
		return nil
	}

	// Build the copy's Card. Mirrors the Replicate copy pattern in
	// keywords_batch3.go — snapshot runtime scalars + share the AST
	// pointer (the engine treats AST nodes as immutable). Mark
	// Card.IsCopy so SBAs and zone accounting know this card ceases to
	// exist outside the stack (CR §704.5e).
	copyCard := &Card{
		Name:          spell.Card.Name,
		Owner:         seat, // CR §707.10 — copies have no "real" owner; we record the copy's controller so any owner-conditioned logic routes to them
		BasePower:     spell.Card.BasePower,
		BaseToughness: spell.Card.BaseToughness,
		Types:         append([]string(nil), spell.Card.Types...),
		Colors:        append([]string(nil), spell.Card.Colors...),
		CMC:           spell.Card.CMC,
		AST:           spell.Card.AST,
		TypeLine:      spell.Card.TypeLine,
		IsCopy:        true,
	}
	MintCopyInstanceID(gs, copyCard, spell.Card.InstanceID, currentMintEnablerID(gs))

	copyItem := &StackItem{
		Controller: seat,
		Card:       copyCard,
		Effect:     spell.Effect,
		Targets:    append([]Target(nil), spell.Targets...),
		Kind:       spell.Kind,
		IsCopy:     true,
		CostMeta: map[string]interface{}{
			"demonstrate_copy":   true,
			"demonstrate_source": spell.Card,
			"demonstrate_role":   role,
		},
	}
	PushStackItem(gs, copyItem)

	gs.LogEvent(Event{
		Kind:   "demonstrate_copy",
		Seat:   seat,
		Source: spell.Card.DisplayName(),
		Details: map[string]interface{}{
			"role":       role,
			"controller": seat,
			"rule":       "702.144a",
		},
	})
	// CR §702.137a / "whenever you copy a spell" — each demonstrate copy fires
	// the canonical copy-trigger fan-out for ITS controller (`seat`): the
	// controller copy fires the controller's magecraft, the opponent copy
	// fires the chosen opponent's (that player "copies it too").
	FireSpellCopyTriggers(gs, seat, copyCard, spell.Card)
	return copyItem
}

// isLivingOpponent returns true if `candidate` is present in `opps`.
// Small helper to keep ApplyDemonstrate's intent readable.
func isLivingOpponent(opps []int, candidate int) bool {
	for _, s := range opps {
		if s == candidate {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Stack predicate
// ---------------------------------------------------------------------------

// IsDemonstrateCopy reports whether a StackItem is a demonstrate-minted
// copy. Useful for resolve-path branches that need to skip cast-only
// triggers (since CR §707.10 says copies are "created on the stack,"
// not "cast").
func IsDemonstrateCopy(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["demonstrate_copy"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}
