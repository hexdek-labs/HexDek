package gameengine

import (
	"fmt"
	"strings"
)

// keywords_storm_rider.go — modern surface for Storm (CR §702.40) that
// pairs with the existing storm.go primitive (HasStormKeyword +
// ApplyStormCopies). Adds:
//
//   - HasStorm(card)             oracle-text-aware detector. Catches
//                                "Storm —" reminder-prefix variants,
//                                storm-flavored cards whose AST omits
//                                the Keyword node (older corpus
//                                dumps), and the "copy it for each
//                                other spell cast before it this
//                                turn" generic phrasing. Falls back to
//                                HasStormKeyword (the name-list + AST
//                                Keyword detector in storm.go) so the
//                                two surfaces agree.
//
//   - StormCount(gs, seat)       PER-SEAT count of spells `seat` has
//                                cast this turn (= the number of
//                                copies a storm spell would make
//                                MINUS the storm spell itself, i.e.
//                                "prior spells"). Reads
//                                seat.Turn.SpellsCast minus 1 when
//                                the storm spell is already counted;
//                                callers that need raw spell count
//                                use seat.Turn.SpellsCast directly.
//
//   - ApplyStormCopy(gs, item,   primitive: pushes EXACTLY `count`
//     count)                     copies of `item` onto the stack
//                                without consulting any cast counter.
//                                The existing ApplyStormCopies
//                                (storm.go) is now a thin convenience
//                                wrapper that derives `count` from
//                                gs.SpellsCastThisTurn - 1.
//
// Why both? ApplyStormCopies' "read SpellsCastThisTurn - 1" semantics
// is correct for the §702.40a "When you cast this spell, copy it for
// each OTHER spell cast before it this turn" path. ApplyStormCopy is
// the lower-level primitive that callers can use when they've
// computed the desired copy count via a different rule (e.g. a card
// that copies "for each spell cast by an opponent" or a per_card
// snowflake that scales storm differently). It also makes the
// "count = 0 → 0 copies" base case explicit at the API surface
// instead of buried in the SpellsCastThisTurn <= 1 guard.

// HasStorm reports whether `card` triggers a storm-style copy fanout.
// Detection paths (descending order, first match wins):
//
//   1. HasStormKeyword(card)       — the existing name-list + AST
//                                    Keyword detector. Authoritative
//                                    for printed Storm cards (storm.go
//                                    catalog).
//   2. Oracle text contains the literal "storm —" / "storm -"
//      reminder prefix (em-dash + ASCII hyphen for older corpus
//      dumps).
//   3. Oracle text contains "storm count" — Tendrils-of-Agony-style
//      payload phrasing.
//   4. Oracle text contains "copy it for each other spell cast" —
//      the §702.40a effect phrasing in cards whose AST omits the
//      keyword tag.
//
// Returns false for nil cards.
func HasStorm(card *Card) bool {
	if card == nil {
		return false
	}
	if HasStormKeyword(card) {
		return true
	}
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	for _, needle := range []string{
		"storm —",
		"storm -",
		"storm count",
		"copy it for each other spell cast",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

// StormCount returns the number of copies a storm spell cast right now would
// produce: the number of OTHER spells cast this turn, BY ANY PLAYER, before it
// (CR §702.40a "for each other spell cast before it this turn"). It reads the
// GLOBAL gs.SpellsCastThisTurn (incremented at cast time for every player) and
// subtracts 1 for the storm spell itself, which the cast pipeline counts before
// storm fires.
//
// This MUST be the global count, not a per-seat count: storm counts spells by
// all players, so the old `seat.Turn.SpellsCast - 1` undercounted in
// multiplayer (it ignored opponents' spells) and disagreed with the live
// ApplyStormCopies path. seatIdx is retained for API stability and validity
// checks; the storm count is identical for whichever seat is casting.
//
// Returns 0 for nil gs / invalid seat / no prior spells.
func StormCount(gs *GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return 0
	}
	n := gs.SpellsCastThisTurn - 1
	if n < 0 {
		return 0
	}
	return n
}

// ApplyStormCopy pushes EXACTLY `count` copies of `original` onto the
// stack. Lower-level primitive than ApplyStormCopies — the caller
// supplies the copy count. count <= 0 is a no-op (returns 0 without
// logging the storm_trigger event).
//
// Copy semantics match ApplyStormCopies: each copy is a fresh
// StackItem pointing at a fresh Card with CMC=0, IsCopy=true (per CR
// §707.10 so SBAs sweep it from non-stack zones), Name suffixed with
// "(storm copy N)" for log distinguishability. Copies inherit the
// original's Effect, Targets, Controller, and AST pointer. Pushes
// land ABOVE the original on the stack so LIFO resolution gives
// copies priority — matches §405.2 trigger-ordering and the
// "triggered ability above the spell that triggered it" intuition.
//
// Returns the number of copies actually pushed (== count when count > 0,
// else 0).
//
// Does NOT trigger cast observers (storm copies aren't cast per
// §707.10), does NOT call IncrementCastCount, does NOT recursively
// re-fire storm.
func ApplyStormCopy(gs *GameState, original *StackItem, count int) int {
	if gs == nil || original == nil || original.Card == nil || count <= 0 {
		return 0
	}
	controller := original.Controller
	gs.LogEvent(Event{
		Kind:   "storm_trigger",
		Seat:   controller,
		Source: original.Card.DisplayName(),
		Amount: count,
		Details: map[string]interface{}{
			"copies": count,
			"rule":   "702.40a",
		},
	})
	baseName := original.Card.DisplayName()
	copies := make([]*Card, 0, count)
	for i := 0; i < count; i++ {
		copyCard := &Card{
			AST:            original.Card.AST,
			Name:           fmt.Sprintf("%s (storm copy %d)", baseName, i+1),
			Owner:          original.Card.Owner,
			BasePower:      original.Card.BasePower,
			BaseToughness:  original.Card.BaseToughness,
			Types:          append([]string(nil), original.Card.Types...),
			Colors:         append([]string(nil), original.Card.Colors...),
			CMC:            0,
			TypeLine:       original.Card.TypeLine,
			ManaCostString: "",
			IsCopy:         true,
		}
		MintCopyInstanceID(gs, copyCard, original.Card.InstanceID, currentMintEnablerID(gs))
		copyItem := &StackItem{
			Controller: controller,
			Card:       copyCard,
			Effect:     original.Effect,
			Targets:    append([]Target(nil), original.Targets...),
			IsCopy:     true,
			CastZone:   original.CastZone,
			// CR §707.10b — storm copies of an X spell copy the chosen X.
			ChosenX: original.ChosenX,
		}
		copyItem.ID = nextStackID(gs)
		gs.Stack = append(gs.Stack, copyItem)
		copies = append(copies, copyCard)
		gs.LogEvent(Event{
			Kind:   "stack_push_storm_copy",
			Seat:   controller,
			Source: copyCard.Name,
			Details: map[string]interface{}{
				"stack_id":   copyItem.ID,
				"stack_size": len(gs.Stack),
				"rule":       "702.40a+706.10",
			},
		})
	}
	// CR §702.137b — magecraft triggers "whenever you cast OR COPY an
	// instant or sorcery spell." Storm makes copies (§707.10), which the
	// dedicated resolveCopySpell magecraft hook never sees, so a magecraft
	// permanent (Archmage Emeritus, Storm-Kiln Artist, Symmetry Sage, …)
	// silently missed every storm copy — it triggered only on the single
	// original cast. Fire once per copy now. FireMagecraftTriggers filters
	// to instant/sorcery internally, so a storm copy of a non-i/s spell
	// (e.g. an artifact with storm) correctly triggers nothing. Done after
	// the push loop so the copies stay contiguous on the stack and the
	// magecraft triggers land above them.
	for _, cc := range copies {
		FireMagecraftTriggers(gs, controller, cc, true)
	}
	return count
}
