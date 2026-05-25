package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// keywords_sweep.go — CR §702.32 Sweep (Saviors of Kamigawa, 2005).
//
// Sweep is an ADDITIONAL optional cost paid at cast time:
//
//   "Sweep — As an additional cost to cast this spell, return any
//    number of [land type] you control to their owner's hand. This
//    spell's effect scales with the number returned."
//
// Per-card scaling effects read CostMeta["sweep_count"] off the
// StackItem to know how many lands were returned.
//
// Architecture (mirrors CastWithStrive — strive is also an additional
// per-count cost helper, §601.2g):
//
//   - HasSweep(card, landType)        detector with land-type filter
//                                     so a Plains-sweep spell doesn't
//                                     accidentally match a Mountain-
//                                     sweep query. Two paths:
//                                       1. AST Keyword "sweep" whose
//                                          first arg matches landType
//                                          (case-insensitive).
//                                       2. Oracle text contains the
//                                          literal "sweep —" / "sweep -"
//                                          reminder prefix AND mentions
//                                          landType.
//
//   - HasSweepKeyword(card)           type-agnostic detector for
//                                     analyzers that don't care which
//                                     land type.
//
//   - CastWithSweep(gs, seat, card,   full cast entry point.
//     landType, returned)             Greedy pre-validation: every
//                                     `returned` perm must be a basic
//                                     of landType controlled by seat;
//                                     first invalid aborts before any
//                                     side effect. On success, each
//                                     return routes through MoveCard
//                                     battlefield→hand (LTB triggers
//                                     fire), the spell pops off
//                                     `seat`'s hand, a StackItem is
//                                     pushed with CostMeta:
//                                       sweep=true
//                                       alt_cost="sweep"
//                                       sweep_land_type=landType
//                                       sweep_count=len(returned)
//                                     and CastZone=ZoneHand. Empty
//                                     `returned` is LEGAL per "any
//                                     number" — sweep_count=0 still
//                                     casts.

// HasSweep reports whether `card` prints a sweep clause for
// `landType`. landType is matched case-insensitively. Returns false
// for nil card / empty landType / no sweep clause / wrong land-type.
func HasSweep(card *Card, landType string) bool {
	if card == nil || landType == "" {
		return false
	}
	wantLT := strings.ToLower(strings.TrimSpace(landType))
	if wantLT == "" {
		return false
	}

	// Path 1: AST Keyword "sweep" with a string arg matching landType.
	if card.AST != nil {
		for _, ab := range card.AST.Abilities {
			kw, ok := ab.(*gameast.Keyword)
			if !ok || !strings.EqualFold(kw.Name, "sweep") {
				continue
			}
			for _, a := range kw.Args {
				if s, ok := a.(string); ok && strings.EqualFold(s, wantLT) {
					return true
				}
			}
		}
	}

	// Path 2: oracle-text fallback — must contain a "sweep" prefix
	// AND the landType. Both substrings required so a card that
	// merely mentions a land type in flavor text (without a sweep
	// clause) doesn't match.
	text := OracleTextLower(card)
	if text == "" {
		return false
	}
	if !strings.Contains(text, "sweep —") && !strings.Contains(text, "sweep -") {
		return false
	}
	return strings.Contains(text, wantLT)
}

// HasSweepKeyword reports whether `card` has any sweep clause,
// regardless of land type.
func HasSweepKeyword(card *Card) bool {
	if card == nil {
		return false
	}
	if cardHasKeywordByName(card, "sweep") {
		return true
	}
	text := OracleTextLower(card)
	return strings.Contains(text, "sweep —") || strings.Contains(text, "sweep -")
}

// CastWithSweep casts `card` from `seatIdx`'s hand and pays the sweep
// additional cost by returning `returned` lands to their owners'
// hands. CR §702.32a + §601.2g.
//
// Validation order (greedy pre-check, atomic abort):
//   1. Game / seat / card / keyword sanity.
//   2. For each Permanent in `returned`:
//        - non-nil + has a Card
//        - Controller == seatIdx (rejects opponent's land)
//        - has the "basic" supertype (rejects non-basic typed lands
//          like shocks, fetches, duals)
//        - has landType in Card.Types (rejects wrong-typed basics —
//          e.g. a Mountain doesn't satisfy a Plains-sweep)
//   3. Card must be in `seatIdx`'s hand.
//
// If any pre-check fails the call returns nil + a CastError and
// performs ZERO side effects. The lands stay on the battlefield, no
// card moves, no mana changes, no event fires. This matches CR
// §601.2g's "you must declare and pay the whole additional cost or
// none of it" discipline.
//
// On success:
//   - Each returned perm is removed from the battlefield via
//     gs.removePermanent and its Card is routed through MoveCard
//     battlefield→hand (reason "sweep") so LTB/zone-change triggers
//     fire normally.
//   - The card is removed from `seat`'s hand and a StackItem is
//     pushed with CostMeta:
//       sweep=true, alt_cost="sweep",
//       sweep_land_type=landType (lowercased),
//       sweep_count=len(returned)
//     and CastZone=ZoneHand.
//   - Per-turn flag "spell_sweep_this_turn:<seat>" is incremented.
//   - A sweep_cast event is logged with rule=702.32a + amount =
//     sweep_count.
//
// Returns the new StackItem on success or nil + CastError on failure.
// Errors: nil game / invalid seat / nil card / no_sweep_keyword /
// nil_target_perm / target_not_controller / target_not_basic /
// target_wrong_land_type / nil seat / not_in_hand.
func CastWithSweep(gs *GameState, seatIdx int, card *Card, landType string, returned []*Permanent) (*StackItem, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasSweep(card, landType) {
		return nil, &CastError{Reason: "no_sweep_keyword"}
	}
	wantLT := strings.ToLower(strings.TrimSpace(landType))

	for _, p := range returned {
		if p == nil || p.Card == nil {
			return nil, &CastError{Reason: "nil_target_perm"}
		}
		if p.Controller != seatIdx {
			return nil, &CastError{Reason: "target_not_controller"}
		}
		if !permIsBasic(p) {
			return nil, &CastError{Reason: "target_not_basic"}
		}
		if !permHasLandType(p, wantLT) {
			return nil, &CastError{Reason: "target_wrong_land_type"}
		}
	}

	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}

	for _, p := range returned {
		owner := p.Owner
		if owner < 0 || owner >= len(gs.Seats) {
			owner = p.Controller
		}
		gs.removePermanent(p)
		detachAll(gs, p)
		MoveCard(gs, p.Card, owner, "battlefield", "hand", "sweep")
	}

	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"sweep":           true,
			"alt_cost":        "sweep",
			"sweep_land_type": wantLT,
			"sweep_count":     len(returned),
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_sweep_this_turn:"+itoa(seatIdx)]++

	gs.LogEvent(Event{
		Kind:   "sweep_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: len(returned),
		Details: map[string]interface{}{
			"rule":            "702.32a",
			"sweep_land_type": wantLT,
			"sweep_count":     len(returned),
		},
	})
	return item, nil
}

// SpellSweepThisTurn returns the number of sweep spells `seatIdx` has
// cast this turn. Mirrors SpellStriveThisTurn / SpellSpectacleThisTurn.
func SpellSweepThisTurn(gs *GameState, seatIdx int) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	return gs.Flags["spell_sweep_this_turn:"+itoa(seatIdx)]
}

// permIsBasic reports whether `p` is a basic land. The runtime Card
// keeps both supertypes and types interleaved in Card.Types; we check
// for the "basic" supertype case-insensitively.
func permIsBasic(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if strings.EqualFold(t, "basic") {
			return true
		}
	}
	return false
}

// permHasLandType reports whether `p`'s Card.Types contains
// `wantLT` (case-insensitive). Used to verify a basic Plains matches
// landType="plains".
func permHasLandType(p *Permanent, wantLT string) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if strings.EqualFold(t, wantLT) {
			return true
		}
	}
	return false
}
