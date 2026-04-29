package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase 9 — multiplayer N-seat generalization (CR §800, §101.4).
//
// This file extends the 2-player MVP decision sites (attack targeting,
// end-of-game detection, APNAP iteration) to handle arbitrary seat counts
// (3, 4, 5, +). It mirrors the Python reference at scripts/playloop.py:
//
//   - Game.opp / Game.opponents / Game.living_opponents / Game.apnap_order
//   - Game.check_end          → CheckEnd here
//   - _handle_seat_elimination → HandleSeatElimination here
//
// §800.4 spans multiple sub-rules (§800.4a–§800.4p). We implement the
// hot-path subset relevant to the 4-player EDH gauntlet:
//
//   §800.4a  — leave-the-game cleanup (objects owned leave; controlled
//              objects on stack cease/exile; continuous/replacement
//              effects this seat sourced are dropped).
//   §800.4b  — control-change to a left player doesn't happen.
//   §800.4e  — combat damage to a left player isn't assigned (handled
//              via CheckEnd short-circuit + living-only attacker pick).
//
// §101.4    — APNAP: simultaneous choices resolve in turn order starting
//              from the active player.
//
// §104.2a / §104.3b — game ends when 1 or 0 living seats remain.
//
// state.go is the type-definition file; this module is pure behavior.

// -----------------------------------------------------------------------------
// Opponent / APNAP helpers (additive on GameState)
// -----------------------------------------------------------------------------

// OpponentsOf returns the seat indices of every non-source seat,
// INCLUDING dead ones, in APNAP order anchored at `seatIdx` (the seat
// AFTER seatIdx in turn order comes first). Mirrors Python
// Game.opponents — dead-inclusive so the caller can filter or not based
// on context. For living-only use LivingOpponents.
//
// Used by: each-opponent effect fan-out, threat-score iteration, §800.4
// cleanup ("next player in turn order").
func (gs *GameState) OpponentsOf(seatIdx int) []int {
	if gs == nil {
		return nil
	}
	n := len(gs.Seats)
	if n == 0 {
		return nil
	}
	out := make([]int, 0, n-1)
	for k := 1; k < n; k++ {
		cand := (seatIdx + k) % n
		if cand == seatIdx {
			continue
		}
		out = append(out, cand)
	}
	return out
}

// LivingOpponents returns non-source seats that aren't Lost, in APNAP
// order from seatIdx. CR §104.2a — a player "in the game" is one who
// hasn't lost. Mirrors Python Game.living_opponents.
//
// Combat target selection, "each opponent" fan-out, and policy/threat
// scoring should all use this rather than OpponentsOf, so effects don't
// leak onto eliminated seats (§800.4b / §800.4e).
func (gs *GameState) LivingOpponents(seatIdx int) []int {
	if gs == nil {
		return nil
	}
	all := gs.OpponentsOf(seatIdx)
	out := make([]int, 0, len(all))
	for _, i := range all {
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		out = append(out, i)
	}
	return out
}

// APNAPOrder returns every seat (living + dead) in APNAP order starting
// from `fromSeat`. CR §101.4a — "starting with the active player and
// proceeding in turn order." If fromSeat is out of range, anchors at
// gs.Active. Dead seats are included because some CR corners (§800.4h
// last-known-info fallback) still reference them; callers that want
// respondent polling should filter Lost themselves.
func (gs *GameState) APNAPOrder(fromSeat int) []int {
	if gs == nil {
		return nil
	}
	n := len(gs.Seats)
	if n == 0 {
		return nil
	}
	anchor := fromSeat
	if anchor < 0 || anchor >= n {
		anchor = gs.Active
	}
	out := make([]int, 0, n)
	for k := 0; k < n; k++ {
		out = append(out, (anchor+k)%n)
	}
	return out
}

// APNAPOrder returns seat indices in APNAP order starting from the
// active player, then clockwise through non-active players, skipping
// eliminated (Lost) seats.
// Per CR §101.4: "If multiple players would make choices and/or take
// actions at the same time, the active player makes any choices first,
// then each other player in turn order makes choices."
//
// This is a package-level convenience function for trigger ordering;
// the method GameState.APNAPOrder(fromSeat) includes dead seats and
// anchors at an arbitrary seat — use that for full-seat enumeration.
func APNAPOrder(gs *GameState) []int {
	if gs == nil {
		return nil
	}
	nSeats := len(gs.Seats)
	if nSeats == 0 {
		return nil
	}
	order := make([]int, 0, nSeats)
	for i := 0; i < nSeats; i++ {
		idx := (gs.Active + i) % nSeats
		if gs.Seats[idx] != nil && !gs.Seats[idx].Lost {
			order = append(order, idx)
		}
	}
	return order
}

// Opp returns the first living non-source seat in APNAP order from
// seatIdx. Legacy 2-player compatibility shim (mirrors Python Game.opp).
// Callers needing ALL opponents should use LivingOpponents / OpponentsOf.
//
// If all opponents are dead, falls back to any non-source seat so
// `.Life` reads don't crash; the game should already be ended by then
// via CheckEnd.
func (gs *GameState) Opp(seatIdx int) int {
	if gs == nil || len(gs.Seats) == 0 {
		return seatIdx
	}
	living := gs.LivingOpponents(seatIdx)
	if len(living) > 0 {
		return living[0]
	}
	all := gs.OpponentsOf(seatIdx)
	if len(all) > 0 {
		return all[0]
	}
	return seatIdx
}

// LivingSeats returns the indices of every seat whose Lost flag is
// false. Used by CheckEnd + "each player" fan-outs that should exclude
// eliminated seats (per §800.4).
func (gs *GameState) LivingSeats() []int {
	if gs == nil {
		return nil
	}
	out := make([]int, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		out = append(out, i)
	}
	return out
}

// -----------------------------------------------------------------------------
// CheckEnd — CR §104.2a last-seat-standing + §800.4 elimination sweep
// -----------------------------------------------------------------------------

// CheckEnd flips gs.Flags["ended"] = 1 when at most one seat remains in
// the game. Also runs §800.4a cleanup on any seat whose Lost flag flipped
// true since the previous call (idempotent via Seat.LeftGame).
//
// Contract (mirrors Python Game.check_end):
//
//   - ≥2 living seats → game continues, returns false.
//   - 1  living seat  → game ends, that seat is the winner.
//     gs.Flags["winner"] = seat_idx.
//   - 0  living seats → simultaneous elimination draw. gs.Flags["winner"]
//     unset (absent = draw).
//
// Always safe to call multiple times per SBA pass. Callers receive the
// returned bool as "is the game over?" and should stop turn/phase
// progression when true.
func (gs *GameState) CheckEnd() bool {
	if gs == nil {
		return false
	}
	// §800.4a — run leave-the-game cleanup for newly-Lost seats. Order
	// matches Python: eliminate first, THEN count alive.
	for _, s := range gs.Seats {
		if s != nil && s.Lost && !s.LeftGame {
			HandleSeatElimination(gs, s.Idx)
		}
	}
	alive := gs.LivingSeats()
	if len(alive) > 1 {
		return false
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["ended"] == 1 {
		return true
	}
	gs.Flags["ended"] = 1
	// Reconcile mana pools — combat-phase game ends skip the normal
	// end-of-turn drain, leaving typed/legacy pools potentially out of sync.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Mana != nil {
			s.ManaPool = s.Mana.Total()
		}
	}
	if len(alive) == 1 {
		gs.Flags["winner"] = alive[0]
		gs.Seats[alive[0]].Won = true
		gs.LogEvent(Event{
			Kind: "game_end", Seat: alive[0], Target: -1,
			Details: map[string]interface{}{
				"rule":   "104.2a",
				"winner": alive[0],
				"reason": "last_seat_standing",
			},
		})
	} else {
		gs.LogEvent(Event{
			Kind: "game_end", Seat: -1, Target: -1,
			Details: map[string]interface{}{
				"rule":   "104.3b",
				"winner": -1,
				"reason": "simultaneous_elimination_draw",
			},
		})
	}
	return true
}

// -----------------------------------------------------------------------------
// HandleSeatElimination — CR §800.4a cleanup
// -----------------------------------------------------------------------------

// HandleSeatElimination applies the §800.4a "when a player leaves the
// game" procedure for the seat at seatIdx. Idempotent via Seat.LeftGame.
//
// Steps (matching Python _handle_seat_elimination):
//
//  1. Remove from every battlefield each permanent OWNED by this seat
//     (CR §108.3 + §800.4a: "all objects owned by that player leave the
//     game"). Also remove permanents CONTROLLED by this seat that an
//     opponent happens to own — the control-effect ends (§800.4a second
//     clause) and without an active control effect the object returns
//     to its owner; our MVP simply exiles it by removing from play.
//     Unregister §614 replacement + §613 continuous effects keyed to
//     each removed permanent.
//  2. Purge stack items whose Controller == seat (§800.4a: "objects on
//     the stack not represented by cards ... cease to exist"; for
//     card-represented spells we drop them outright as a conservative
//     MVP — the rule says they're exiled, but we don't need the card
//     back in a zone for the simulator to proceed).
//  3. Drop §613 ContinuousEffects whose ControllerSeat == seat. Control-
//     change effects that gave this seat control of OTHER permanents
//     end now; the "restore to owner" clause is handled by step 1's
//     owner-based sweep.
//  4. Drop §614 Replacements whose ControllerSeat == seat.
//  5. Emit seat_eliminated event.
//
// §800.4b ("objects would enter under a left player's control don't")
// and §800.4e ("combat damage to a left player isn't assigned") are
// enforced at the decision sites (combat target pick + ResolveEffect
// controller checks), not here.
func HandleSeatElimination(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.LeftGame {
		return
	}
	seat.LeftGame = true

	// CR §106.4 — eliminated players hold no mana.
	if seat.Mana != nil {
		seat.Mana.Clear()
	}
	seat.ManaPool = 0

	// Track real (non-token) cards that leave the game so the
	// ZoneConservation invariant can adjust its baseline. Per CR §800.4a,
	// objects owned by the leaving player cease to exist — they are NOT
	// placed into any zone.
	realCardsLeaving := 0

	// Step 1: Walk EVERY seat's battlefield (the leaving player may
	// still own cards an opponent now controls — Gilded Drake trade).
	removed := 0
	for _, other := range gs.Seats {
		if other == nil || len(other.Battlefield) == 0 {
			continue
		}
		kept := other.Battlefield[:0]
		for _, p := range other.Battlefield {
			if p == nil {
				continue
			}
			if p.Controller == seatIdx || p.Owner == seatIdx {
				// Unregister any §614 / §613 hooks tied to this permanent.
				gs.UnregisterReplacementsForPermanent(p)
				gs.UnregisterContinuousEffectsForPermanent(p)
				// Count real cards for zone conservation tracking.
				if p.Card != nil && !p.IsToken() {
					realCardsLeaving++
				}
				removed++
				continue
			}
			kept = append(kept, p)
		}
		other.Battlefield = kept
	}

	// Step 2: purge stack items sourced from this seat. §800.4a:
	// abilities cease to exist; spells are exiled. MVP: drop.
	if len(gs.Stack) > 0 {
		purged := 0
		kept := gs.Stack[:0]
		for _, item := range gs.Stack {
			if item == nil {
				continue
			}
			if item.Controller == seatIdx {
				// Count real cards on the stack that are leaving.
				if item.Card != nil && !cardIsTokenForElim(item.Card) {
					realCardsLeaving++
				}
				purged++
				continue
			}
			kept = append(kept, item)
		}
		gs.Stack = kept
		if purged > 0 {
			gs.LogEvent(Event{
				Kind: "stack_purged_on_leave", Seat: seatIdx, Target: -1,
				Amount: purged,
				Details: map[string]interface{}{
					"rule":   "800.4a",
					"reason": "seat_left_game",
				},
			})
		}
	}

	// Adjust zone conservation baseline for cards leaving the game.
	// Also count cards remaining in the eliminated seat's private zones
	// (hand, library, graveyard, exile, command zone) that are now "out
	// of the game" per §800.4a. These cards remain in the zone data
	// structures (we don't nil them out) but they belong to a player who
	// has left — they should be excluded from the invariant's expected
	// count OR we leave them as-is (they're still counted). Since we
	// keep them in the data structures, only battlefield + stack removals
	// need adjustment.
	if realCardsLeaving > 0 && gs.Flags != nil {
		if baseline, ok := gs.Flags["_zone_conservation_total"]; ok {
			gs.Flags["_zone_conservation_total"] = baseline - realCardsLeaving
		}
	}

	// Step 3: drop §613 continuous effects controlled by this seat
	// (source already cleaned up in step 1; this catches effects whose
	// SourcePerm went nil or whose source was a resolved spell).
	if len(gs.ContinuousEffects) > 0 {
		before := len(gs.ContinuousEffects)
		kept := gs.ContinuousEffects[:0]
		for _, ce := range gs.ContinuousEffects {
			if ce == nil {
				continue
			}
			if ce.ControllerSeat == seatIdx {
				continue
			}
			kept = append(kept, ce)
		}
		gs.ContinuousEffects = kept
		if len(gs.ContinuousEffects) != before {
			gs.InvalidateCharacteristicsCache()
		}
	}

	// Step 4: drop §614 replacements controlled by this seat.
	if len(gs.Replacements) > 0 {
		kept := gs.Replacements[:0]
		for _, re := range gs.Replacements {
			if re == nil {
				continue
			}
			if re.ControllerSeat == seatIdx {
				continue
			}
			kept = append(kept, re)
		}
		gs.Replacements = kept
	}

	// Step 5: emit observation event.
	gs.LogEvent(Event{
		Kind: "seat_eliminated", Seat: seatIdx, Target: -1,
		Amount: removed,
		Details: map[string]interface{}{
			"rule":               "800.4a",
			"permanents_removed": removed,
			"reason":             seat.LossReason,
		},
	})
	// Fire per-card triggers for seat elimination (e.g. Davros, Dalek Creator).
	FireCardTrigger(gs, "seat_eliminated", map[string]interface{}{
		"eliminated_seat": seatIdx,
		"reason":          seat.LossReason,
	})

	// §800.4h: If the active player leaves the game, advance to the next
	// living player. "If the active player leaves the game during their
	// own turn, the turn continues without an active player until cleanup."
	// MVP: advance to next living seat to avoid TurnStructure invariant
	// violations in downstream checks.
	if gs.Active == seatIdx {
		for i := 1; i < len(gs.Seats); i++ {
			next := (seatIdx + i) % len(gs.Seats)
			if gs.Seats[next] != nil && !gs.Seats[next].Lost {
				gs.Active = next
				break
			}
		}
	}
}

// cardIsTokenForElim checks if a card is a token for elimination purposes.
// Mirrors cardIsTokenForInv in invariants.go.
func cardIsTokenForElim(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if t == "token" {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Partner legality — CR §702.124 / §903.3c
// -----------------------------------------------------------------------------

// PartnerInfo summarises the partner-relevant AST keywords on a card.
// Populated by ReadPartnerInfo from a CardAST. Empty/zero-valued when
// the card has no partner ability.
type PartnerInfo struct {
	Partner          bool   // bare "Partner" keyword (CR §702.124a)
	FriendsForever   bool   // "Friends forever" (functionally partner)
	ChooseBackground bool   // "Choose a Background" commander
	IsBackground     bool   // type-line includes "Background"
	PartnerWith      string // "Partner with X" — names the required pair (CR §702.124g)
	DoctorsCompanion bool   // "Doctor's companion" (pairs with a Doctor)
	IsDoctor         bool   // type-line includes "Time Lord ... Doctor"
}

// HasPartner returns true if the card has ANY partner-family keyword
// that legalises it in a two-commander pair.
func (p PartnerInfo) HasPartner() bool {
	return p.Partner || p.FriendsForever || p.ChooseBackground ||
		p.IsBackground || p.PartnerWith != "" ||
		p.DoctorsCompanion || p.IsDoctor
}

// ReadPartnerInfo walks a card's AST + Types slice and extracts which
// partner-family keyword(s) it carries. Safe on nil card or missing AST.
//
// The parser (scripts/parser.py) emits bare "partner" keywords and
// "partner with X" as Keyword nodes; we match on Name+Raw case-
// insensitively. Doctor's Companion / Friends Forever / Choose a
// Background land as Keyword nodes with raw text matching the tail of
// the keyword pattern in parser.py line ~1437.
//
// Type-line membership (Background subtype, Doctor subtype) is read
// from Card.Types; the deckparser populates that from Scryfall's
// type_line via parseTypes, lowercased.
func ReadPartnerInfo(card *Card) PartnerInfo {
	info := PartnerInfo{}
	if card == nil {
		return info
	}
	for _, t := range card.Types {
		low := strings.ToLower(t)
		switch low {
		case "background":
			info.IsBackground = true
		case "doctor":
			info.IsDoctor = true
		}
	}
	if card.AST == nil {
		return info
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok || kw == nil {
			continue
		}
		rawLow := strings.TrimSpace(kw.Raw)
		nameLow := strings.ToLower(strings.TrimSpace(kw.Name))
		switch {
		case rawLow == "partner" || (nameLow == "partner" && rawLow == ""):
			info.Partner = true
		case strings.HasPrefix(rawLow, "partner with "):
			// Keep everything after "partner with " up to comma / paren /
			// full stop. Names like "Partner with Kydele, Chosen of Kruphix"
			// are kept wholesale minus any reminder-text tail.
			name := strings.TrimSpace(kw.Raw[len("partner with "):])
			if idx := strings.IndexAny(name, ".("); idx >= 0 {
				name = strings.TrimSpace(name[:idx])
			}
			// Comma-terminated only if the comma isn't part of the card name
			// itself. Real cards like "Partner with Kydele, Chosen of
			// Kruphix" rely on full string comparison against the paired
			// commander's name so we keep the comma in. We only strip
			// reminder text after a sentence break above.
			info.PartnerWith = name
		case rawLow == "friends forever":
			info.FriendsForever = true
		case rawLow == "choose a background":
			info.ChooseBackground = true
		case rawLow == "doctor's companion" ||
			(strings.HasPrefix(rawLow, "doctor") && strings.Contains(rawLow, "companion")):
			info.DoctorsCompanion = true
		}
	}
	return info
}

// ValidatePartnerPair checks CR §702.124 / §903.3c partner legality
// for a commander slice. Returns nil if legal, a *CastError describing
// the violation otherwise.
//
// Valid configurations:
//  1. Both cards have bare Partner keyword (§702.124a).
//  2. "Partner with X" + the named X on each side (§702.124g — specific pair).
//  3. Both Friends Forever (functionally identical to Partner).
//  4. "Choose a Background" commander + Background-typed card.
//  5. A Doctor + Doctor's Companion.
//
// Mixing keywords across categories (Partner + Friends Forever, etc.)
// is ILLEGAL per CR — each keyword pairs only with its own kind.
//
// Single-commander decks pass (len(cards) == 1). Empty decks fail.
// More than 2 commanders always fails — no format allows triple
// commanders.
func ValidatePartnerPair(cards []*Card) error {
	if len(cards) == 0 {
		return &CastError{Reason: "no_commander"}
	}
	if len(cards) == 1 {
		return nil // single commander — trivially legal
	}
	if len(cards) > 2 {
		return &CastError{Reason: "too_many_commanders"}
	}
	a, b := cards[0], cards[1]
	if a == nil || b == nil {
		return &CastError{Reason: "nil_commander"}
	}
	ia := ReadPartnerInfo(a)
	ib := ReadPartnerInfo(b)
	aName := a.DisplayName()
	bName := b.DisplayName()

	// Case 1: both bare Partner.
	if ia.Partner && ib.Partner {
		return nil
	}
	// Case 2: "Partner with X" — names must cross-match.
	if ia.PartnerWith != "" && partnerNameMatch(ia.PartnerWith, bName) {
		if ib.PartnerWith == "" || partnerNameMatch(ib.PartnerWith, aName) {
			return nil
		}
	}
	if ib.PartnerWith != "" && partnerNameMatch(ib.PartnerWith, aName) {
		if ia.PartnerWith == "" || partnerNameMatch(ia.PartnerWith, bName) {
			return nil
		}
	}
	// Case 3: Friends Forever pair.
	if ia.FriendsForever && ib.FriendsForever {
		return nil
	}
	// Case 4: Choose-a-Background commander + Background card.
	if (ia.ChooseBackground && ib.IsBackground) ||
		(ib.ChooseBackground && ia.IsBackground) {
		return nil
	}
	// Case 5: Doctor + Doctor's Companion.
	if (ia.IsDoctor && ib.DoctorsCompanion) ||
		(ib.IsDoctor && ia.DoctorsCompanion) {
		return nil
	}
	return &CastError{Reason: "invalid_partner_pair"}
}

// partnerNameMatch normalises a "Partner with X" reference and a
// candidate commander display name for comparison. Case-fold + trim.
func partnerNameMatch(partnerWith, candidate string) bool {
	return strings.EqualFold(strings.TrimSpace(partnerWith),
		strings.TrimSpace(candidate))
}
