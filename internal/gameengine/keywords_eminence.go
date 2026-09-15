package gameengine

// keywords_eminence.go — Eminence (CR §702.107, Commander 2017).
//
// CR §702.107a: Eminence is an ability word that introduces a static
//               or triggered ability that functions while the card
//               with it is in the COMMAND ZONE in addition to (or
//               instead of) the battlefield. It's the rules-side
//               opt-in that lets commander-only abilities function
//               without the commander being cast.
// CR (ability word, no rule): Most abilities of cards in the command zone do NOT
//               function (CR (ability word, no rule)). Eminence is the named
//               exception — its presence on a card means "this
//               specific ability is allowed to function from the
//               command zone."
//
// Architecture: this is a passive GATING surface, not a triggered
// ability. Consumers check EminenceActive(gs, card) at the point
// they want to apply the gated effect (cost modifier scan, ETB
// scan, attack-trigger fan-out, etc.). The existing Ur-Dragon
// discount in cost_modifiers.go already implements this pattern by
// hand-checking each seat's CommandZone for "The Ur-Dragon" —
// EminenceActive collapses that lookup into a uniform helper so new
// eminence cards can reuse one canonical predicate.
//
// API:
//
//   HasEminence(card)              → bool
//   EminenceActive(gs, card)       → bool — true if card is in any
//                                   seat's CommandZone OR on any
//                                   seat's Battlefield (its Card
//                                   pointer matches a Permanent.Card)
//   EminenceController(gs, card)   → (seatIdx int, found bool) — the
//                                   seat that "owns" the eminence
//                                   activation (controller for
//                                   battlefield perms, owner for
//                                   command-zone cards)
//   EminenceZone(gs, card)         → "command_zone" / "battlefield" / ""
//                                   the zone the card is currently
//                                   in; "" if not in either eligible
//                                   zone (and the static ability is
//                                   therefore inactive)

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasEminence
// ---------------------------------------------------------------------------

// HasEminence returns true if the card carries an eminence ability.
// Detection paths:
//
//  1. Keyword AST node named "eminence" — the canonical corpus tag.
//  2. Oracle text contains the printed "Eminence —" prefix (the
//     reminder text on every printed eminence card opens with the
//     literal "Eminence —" em-dash separator).
//
// Returns false for nil / AST-less cards. Match is case-insensitive
// via OracleTextLower's cached lowercased text.
func HasEminence(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "eminence") {
			return true
		}
	}
	// "Eminence —" (em-dash) is the printed lead. Match the lowercased
	// word "eminence" — the em-dash itself isn't required because
	// OracleTextLower folds punctuation/spacing variations.
	if strings.Contains(OracleTextLower(card), "eminence") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// EminenceActive
// ---------------------------------------------------------------------------

// EminenceActive returns true if `card` is currently in any seat's
// command zone OR on any seat's battlefield. CR §702.107a — those
// are the two zones eminence abilities are explicitly allowed to
// function from. Any other zone (hand, graveyard, exile, library,
// stack) returns false: eminence abilities don't function there.
//
// Returns false for nil gs or nil card. Returns true even if the
// card itself doesn't bear the eminence keyword — the predicate
// asks "is this card in an eligible zone?", not "should eminence
// apply?". Callers typically pair the two:
//
//	if HasEminence(card) && EminenceActive(gs, card) { ... apply ... }
//
// For permanents the battlefield check matches via Permanent.Card —
// a card whose pointer matches the runtime Card on any seat's
// Battlefield slice is considered "on the battlefield." Phased-out
// permanents are excluded (§702.26 — "treated as though they don't
// exist") so their eminence abilities are correctly suspended while
// phased out.
func EminenceActive(gs *GameState, card *Card) bool {
	if gs == nil || card == nil {
		return false
	}
	_, zone := eminenceLocate(gs, card)
	return zone != ""
}

// EminenceZone returns the zone string ("command_zone" or
// "battlefield") where `card` currently resides among the eminence-
// eligible zones, or "" if it's not in either. Useful for log
// attribution ("Ur-Dragon (eminence, command zone)" vs
// "Ur-Dragon (eminence, battlefield)").
func EminenceZone(gs *GameState, card *Card) string {
	_, zone := eminenceLocate(gs, card)
	return zone
}

// EminenceController returns the seat that "controls" the eminence
// activation: the controller for battlefield permanents, or the owner
// (the seat whose CommandZone slice holds the card) for command-zone
// cards. Returns (-1, false) when the card isn't in an eligible zone.
//
// This is the seat eminence's "you" clauses refer to — "OTHER Dragon
// spells YOU cast cost {1} less" routes through the returned seat.
func EminenceController(gs *GameState, card *Card) (int, bool) {
	if gs == nil || card == nil {
		return -1, false
	}
	seat, zone := eminenceLocate(gs, card)
	if zone == "" {
		return -1, false
	}
	return seat, true
}

// eminenceLocate is the shared scanner. Returns the seat that "owns"
// the eminence activation and the zone name ("command_zone" or
// "battlefield"), or (-1, "") when card is not in an eligible zone.
//
// Order of search: command zones first (cheaper — usually 1–2
// entries), then battlefields. Phased-out permanents are skipped.
func eminenceLocate(gs *GameState, card *Card) (int, string) {
	if gs == nil || card == nil {
		return -1, ""
	}
	for i, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, c := range seat.CommandZone {
			if c == card {
				return i, "command_zone"
			}
		}
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.PhasedOut {
				continue
			}
			if p.Card == card {
				return p.Controller, "battlefield"
			}
		}
	}
	return -1, ""
}
