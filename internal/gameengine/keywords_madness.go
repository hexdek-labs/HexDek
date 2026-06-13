package gameengine

// keywords_madness.go — Madness (CR §702.34, Torment 2002).
//
// CR §702.34a: Madness is a keyword that represents two abilities.
//               The first is a static ability that functions while the
//               card with madness is in a player's hand. The second is
//               a triggered ability that functions when the first
//               ability is applied. "Madness [cost]" means "If a
//               player would discard this card, that player discards
//               it, but may exile it instead of putting it into their
//               graveyard" and "When this card is exiled this way,
//               its owner may cast it by paying [cost] rather than
//               putting it into their graveyard. If that player
//               doesn't, they put it into their graveyard."
//
// Three-stage flow
// ----------------
//
//   stage 1 — discard replacement (OnDiscardMadness)
//     The hand→graveyard discard is replaced by hand→exile. The card
//     is stamped Meta["madness_exile_turn"] = gs.Turn and
//     Meta["madness_exiled_by_seat"] = seat so the upcoming cast
//     window can find it deterministically. A per-seat side map
//     gs.MadnessExile[card] = MadnessWindow{...} mirrors the same
//     facts for handlers that prefer a flat map lookup over poking
//     Card.Meta.
//
//   stage 2 — cast-in-window (CastWithMadness)
//     The owner pays the madness cost (the alt cost — NOT the printed
//     mana cost, CR §702.34a "rather than putting it into their
//     graveyard"). The card moves exile→stack with CostMeta
//     {"madness_cast": true, "madness_cost": N}. The MadnessExile
//     entry is removed so ResolveMadnessWindow knows the window has
//     been consumed.
//
//   stage 3 — window close (ResolveMadnessWindow)
//     Called at the end of the immediate replacement window (i.e.
//     right after the triggered ability resolves; in single-thread
//     test flow callers run this at the discard's natural sequence
//     point, or eagerly at end of the resolution that caused the
//     discard). Any MadnessExile entry still present for `seat` is
//     routed exile→graveyard and cleared. Returns the count of cards
//     routed.

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// MadnessWindow — per-card discard-window bookkeeping
// ---------------------------------------------------------------------------

// MadnessWindow captures the per-card facts needed to close the cast
// window if the controller doesn't elect to cast. Stored on
// gs.MadnessExile (a *Card → *MadnessWindow map; see GameState
// declaration in state.go).
type MadnessWindow struct {
	Seat int // the seat that discarded (and may now cast)
	Turn int // gs.Turn at exile time
}

// ---------------------------------------------------------------------------
// MadnessCostParsed — mana-string-aware cost reader
// ---------------------------------------------------------------------------

// MadnessCostParsed returns the converted mana cost of the madness
// keyword's alternative cost. Accepts the keyword arg as either a
// mana string ("{B}{R}") or a plain numeric value. Returns 0 if the
// keyword is absent or args are malformed; callers should treat 0
// as "free" only when they have positively confirmed HasMadness.
//
// Distinct from MadnessCost (defined in keywords_batch6.go) which
// uses the simpler keywordArgCost reader that does NOT parse mana
// strings. MadnessCostParsed matches the Flashback / Mayhem / Omen /
// Escape / Plot / Prowl reader pattern.
func MadnessCostParsed(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "madness") {
			continue
		}
		if len(kw.Args) == 0 {
			return card.CMC
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// OnDiscardMadness — stage-1 discard replacement
// ---------------------------------------------------------------------------

// OnDiscardMadness implements the §702.34a discard replacement.
// Called by DiscardCard for every discarded card; no-ops and
// returns false when the card lacks the madness keyword so the
// normal hand→graveyard path continues unchanged.
//
// On a fire:
//   - Card moves hand→exile via MoveCard (so card_discarded /
//     ETB-from-discard observers still see the right zone change
//     trail; the wrapping DiscardCard logs card_discarded itself).
//   - card.Meta["madness_exile_turn"] = gs.Turn and
//     card.Meta["madness_exiled_by_seat"] = seat are stamped for any
//     handler that wants per-card history.
//   - gs.MadnessExile[card] = &MadnessWindow{Seat, Turn} is set so
//     ResolveMadnessWindow can find pending casts.
//   - A ZoneCastPermission keyed on the card pointer is registered
//     (Zone=exile, Keyword="madness", ManaCost=MadnessCostParsed(card),
//     RequireController=seat) so AI/Hat and zone-cast observers can
//     see the cast option even though CastWithMadness has its own
//     dedicated entry point.
//
// Returns true iff the card had madness AND was successfully routed
// to exile.
func OnDiscardMadness(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	if !HasMadness(card) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	// CR §800.4a — a player who has left the game can't activate replacement
	// effects on their objects. The Madness "replace discard with exile"
	// chain is moot once the owner has left; skip so no MadnessExile +
	// ZoneCastGrant entry is registered for an already-ceased InstanceID
	// (Phase F §400.7c fabrication closure).
	if seat.LeftGame {
		return false
	}
	// Card must still be in hand when this is called — DiscardCard
	// invokes us before performing the hand→graveyard move.
	if !cardInZone(seat, card, ZoneHand) {
		return false
	}
	// Move hand → exile. MoveCard is the canonical zone-change path
	// so card-in-zone invariants stay consistent and any LTB/ETB
	// observers for exile fire normally.
	MoveCard(gs, card, seatIdx, ZoneHand, ZoneExile, "madness-exile")

	// Stamp per-card history on Card.Meta so external readers (replay,
	// AI) can ask "was this card madness-exiled this turn?" without
	// poking the game-level side map.
	if card.Meta == nil {
		card.Meta = map[string]any{}
	}
	card.Meta["madness_exile_turn"] = gs.Turn
	card.Meta["madness_exiled_by_seat"] = seatIdx

	// Mirror the same facts on the game-level side map so
	// ResolveMadnessWindow can iterate efficiently.
	if gs.MadnessExile == nil {
		gs.MadnessExile = map[*Card]*MadnessWindow{}
	}
	gs.MadnessExile[card] = &MadnessWindow{Seat: seatIdx, Turn: gs.Turn}

	// Register the cast-from-exile permission. ManaCost is the
	// parsed madness cost; if the parser couldn't extract a numeric
	// cost (corpus typo) the grant still appears but CastWithMadness
	// will reject it as invalid_madness_cost.
	madnessCost := MadnessCostParsed(card)
	RegisterZoneCastGrant(gs, card, &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "madness",
		ManaCost:          madnessCost,
		RequireController: seatIdx,
		SourceName:        "madness_exile",
		Duration:          "", // closed explicitly by ResolveMadnessWindow
		GrantTurn:         gs.Turn,
	})

	gs.LogEvent(Event{
		Kind:   "madness_exile",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule":         "702.34a",
			"madness_cost": madnessCost,
		},
	})
	return true
}

// ---------------------------------------------------------------------------
// CastWithMadness — stage-2 alt-cost cast from exile
// ---------------------------------------------------------------------------

// CastWithMadness casts a madness-exiled card from `seatIdx`'s exile
// for its madness cost. CR §702.34a, second ability.
//
// Preconditions enforced here:
//   - card has the madness keyword
//   - card is in seat's exile zone
//   - card has an open madness window for THIS seat (entry present
//     in gs.MadnessExile, MadnessWindow.Seat == seatIdx)
//   - seat can afford `madnessCost` (pass -1 to use the printed
//     MadnessCostParsed)
//
// On success: card removed from exile, mana paid, StackItem pushed
// with CostMeta{"madness_cast": true, "madness_cost": N,
// "zone_cast_keyword": "madness"} and CastZone=ZoneExile. The
// MadnessExile entry is removed and the ZoneCastPermission cleaned
// up — the madness window is single-use.
//
// Madness does NOT change resolution destination (the spell resolves
// to its normal post-resolve zone — battlefield for permanents,
// graveyard for non-permanents).
func CastWithMadness(gs *GameState, seatIdx int, card *Card, madnessCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasMadness(card) {
		return nil, &CastError{Reason: "no_madness_keyword"}
	}
	window, ok := gs.MadnessExile[card]
	if !ok || window == nil {
		return nil, &CastError{Reason: "no_madness_window"}
	}
	if window.Seat != seatIdx {
		return nil, &CastError{Reason: "wrong_madness_seat"}
	}
	if madnessCost < 0 {
		madnessCost = MadnessCostParsed(card)
	}
	if madnessCost < 0 {
		return nil, &CastError{Reason: "invalid_madness_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// Drannith Magistrate: opponents can't cast from non-hand zones.
	if drannithRestrictsZoneCast(gs, seatIdx) {
		gs.LogEvent(Event{
			Kind:   "cast_suppressed",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "drannith_magistrate",
				"zone":   ZoneExile,
				"rule":   "601.2a",
			},
		})
		return nil, &CastError{Reason: "drannith_magistrate"}
	}
	if seat.ManaPool < madnessCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !removeFromZone(seat, card, ZoneExile) {
		return nil, &CastError{Reason: "not_in_exile"}
	}
	seat.ManaPool -= madnessCost
	SyncManaAfterSpend(seat)
	if madnessCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: madnessCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "madness_cast",
				"keyword": "madness",
				"rule":    "601.2f",
			},
		})
	}
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneExile,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"madness_cast":      true,
			"madness_cost":      madnessCost,
			"zone_cast_keyword": "madness",
		},
	}
	PushStackItem(gs, item)

	// Window consumed — clear bookkeeping so ResolveMadnessWindow
	// doesn't double-route this card to graveyard later.
	delete(gs.MadnessExile, card)
	RemoveZoneCastGrant(gs, card)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_madness_cast_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "madness_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: madnessCost,
		Details: map[string]interface{}{
			"rule": "702.34a",
		},
	})
	return &CostPaymentResult{}, nil
}

// ---------------------------------------------------------------------------
// ResolveMadnessWindow — stage-3 window close
// ---------------------------------------------------------------------------

// ResolveMadnessWindow closes any open madness windows for `seatIdx`
// by routing the still-exiled cards from exile to graveyard per
// CR §702.34a's "If that player doesn't, they put it into their
// graveyard." Cards whose owner declined to cast end up where the
// original discard would have placed them.
//
// Pass seatIdx = -1 to close ALL open madness windows across every
// seat (useful at end-of-turn cleanup or after a multi-discard
// resolution like Wheel of Fortune).
//
// Returns the number of cards routed.
func ResolveMadnessWindow(gs *GameState, seatIdx int) int {
	if gs == nil || gs.MadnessExile == nil {
		return 0
	}
	routed := 0
	// Collect candidates first so we can safely delete map entries
	// inside the loop.
	type pending struct {
		card *Card
		win  *MadnessWindow
	}
	var todo []pending
	for c, w := range gs.MadnessExile {
		if c == nil || w == nil {
			continue
		}
		if seatIdx >= 0 && w.Seat != seatIdx {
			continue
		}
		todo = append(todo, pending{card: c, win: w})
	}
	// Determinism (r63 seed-determinism audit): todo is built by ranging
	// the MadnessExile map, so its order is Go's randomized map-iteration
	// order. When a seat has >1 madness window open at once, the
	// madness_decline events below would be logged in a different order on
	// every run — diverging the event log that parity replays and the
	// repro baselines compare. Sort by the per-instance InstanceID (stable
	// and unique within a game; DisplayName as a legacy fallback for
	// unminted cards) to pin the resolution order.
	sort.SliceStable(todo, func(i, j int) bool {
		a, b := todo[i].card, todo[j].card
		if a.InstanceID != b.InstanceID {
			return a.InstanceID < b.InstanceID
		}
		return a.DisplayName() < b.DisplayName()
	})
	for _, p := range todo {
		seat := gs.Seats[p.win.Seat]
		if seat == nil {
			delete(gs.MadnessExile, p.card)
			RemoveZoneCastGrant(gs, p.card)
			continue
		}
		// Verify the card is still in exile; if a different effect
		// moved it (LTB, recursion) the rule no longer applies.
		if !cardInZone(seat, p.card, ZoneExile) {
			delete(gs.MadnessExile, p.card)
			RemoveZoneCastGrant(gs, p.card)
			continue
		}
		MoveCard(gs, p.card, p.win.Seat, ZoneExile, ZoneGraveyard, "madness-decline")
		gs.LogEvent(Event{
			Kind:   "madness_decline",
			Seat:   p.win.Seat,
			Source: p.card.DisplayName(),
			Details: map[string]interface{}{
				"rule":   "702.34a",
				"reason": "window_closed_without_cast",
			},
		})
		delete(gs.MadnessExile, p.card)
		RemoveZoneCastGrant(gs, p.card)
		routed++
	}
	return routed
}

// ---------------------------------------------------------------------------
// Stack / per-turn predicates
// ---------------------------------------------------------------------------

// IsMadnessCast reports whether a StackItem was cast via madness.
func IsMadnessCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["madness_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellMadnessCastThisTurn returns true if any spell was cast via
// madness by `seatIdx` during the current turn.
func SpellMadnessCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_madness_cast_this_turn:"+itoa(seatIdx)] > 0
}

// HasOpenMadnessWindow reports whether `card` has an open madness
// cast window for `seatIdx`. Useful for AI/Hat policy code that
// wants to surface the cast option to the planner.
func HasOpenMadnessWindow(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || gs.MadnessExile == nil {
		return false
	}
	w := gs.MadnessExile[card]
	if w == nil {
		return false
	}
	return w.Seat == seatIdx
}
