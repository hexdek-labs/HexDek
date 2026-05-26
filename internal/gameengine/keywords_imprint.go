package gameengine

// keywords_imprint.go — Imprint (CR §702.40, Mirrodin 2003).
//
// CR §702.40a: Imprint is an ability word. It indicates that an
//              effect (typically a triggered ability that fires on
//              ETB) exiles one or more cards "with this permanent,"
//              and a separate static or activated ability on the
//              permanent references those exiled cards.
// CR §702.40b: A card exiled by an imprint effect stays in exile as
//              long as the imprint source is on the battlefield with
//              that imprint association intact. If the source leaves
//              the battlefield, the imprint association ends —
//              callers invoke ReleaseImprint at the LTB hook to
//              dissolve the link. The exiled card stays in exile
//              unless another effect retrieves it; the imprint
//              clause stops referencing it.
//
// Architecture: the existing per_card layer (Chrome Mox, etc.)
// rolled imprint state into ad-hoc Flags (Flags["imprinted"] = 1,
// Flags["imprint_color_R"] = 1) without storing the imprinted card
// POINTER. That works for Chrome Mox's color-only riff, but breaks
// for Isochron Scepter / Mirror of Fate / Soul Foundry / Crucible
// of Worlds-adjacent cards that need to read the actual imprinted
// card object (to copy a spell, to cast a spell, to clone an
// exiled creature, etc.).
//
// This file ships the canonical surface so per_card handlers and
// engine code can:
//
//   1. Detect imprint-bearing cards via HasImprint
//   2. Imprint a card via ApplyImprint — exiles the target and
//      stamps reciprocal metadata on both ends
//   3. Read the currently-imprinted card via GetImprintedCard
//   4. Release the imprint association via ReleaseImprint (called
//      from the LTB hook when the imprint source leaves play)
//
// Metadata convention:
//
//   - Permanent.LinkedExile (already shipped) holds the imprinted
//     Card pointer(s). Imprint reuses this slice rather than adding
//     a new field — the semantic is identical ("cards exiled with
//     this permanent"), and ReleaseImprint clears it explicitly so
//     no auto-return happens (imprint differs from Fiend Hunter
//     §406.7 in that respect).
//   - Permanent.Flags["imprinted"] = 1 marks the permanent as
//     carrying live imprint state. This is the same key Chrome Mox
//     and Misthollow Griffin handlers already read, so existing
//     per_card code keeps working.
//   - Card.Meta["imprinted_by"] = perm.Timestamp (int) gives the
//     exiled card a back-reference to its host. Timestamp is the
//     engine's canonical permanent identifier and is set at ETB —
//     persists across control changes (which imprint doesn't care
//     about, but it's the right invariant).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasImprint
// ---------------------------------------------------------------------------

// HasImprint returns true if the card carries the imprint ability
// word. Detection paths:
//
//  1. Keyword AST node named "imprint" — the corpus loader emits
//     this for canonically tagged cards.
//  2. Oracle text substring "imprint" — catches Static/Triggered
//     ability text on cards whose AST hasn't been keyword-tagged.
//
// Returns false for nil cards or AST-less cards.
func HasImprint(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "imprint") {
			return true
		}
	}
	return strings.Contains(OracleTextLower(card), "imprint")
}

// ---------------------------------------------------------------------------
// ApplyImprint
// ---------------------------------------------------------------------------

// ApplyImprint exiles `target` and binds it to `perm` as that
// permanent's imprinted card. CR §702.40a.
//
// Steps:
//  1. Locate `target` across all of perm's controller's zones (hand
//     first — the common case for Chrome Mox / Isochron Scepter —
//     then library, graveyard, battlefield). Returns an error if
//     the target cannot be located.
//  2. Move `target` to exile via MoveCard, so §614 replacement
//     effects and exile-zone triggers fire correctly.
//  3. Append `target` to perm.LinkedExile so callers can read it
//     back via GetImprintedCard.
//  4. Set perm.Flags["imprinted"] = 1.
//  5. Stamp target.Meta["imprinted_by"] = perm.Timestamp for
//     reciprocal lookup (e.g. an effect that needs to find the
//     host of a given exiled card).
//  6. Log an "imprint" event.
//
// Returns the exiled card (the same target on success) and any
// error. Nil-safe on all inputs.
func ApplyImprint(gs *GameState, perm *Permanent, target *Card) (*Card, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if perm == nil {
		return nil, &CastError{Reason: "nil permanent"}
	}
	if target == nil {
		return nil, &CastError{Reason: "nil target"}
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid_controller"}
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}

	fromZone, ok := findCardZone(seat, target)
	if !ok {
		return nil, &CastError{Reason: "target_not_found"}
	}

	MoveCard(gs, target, perm.Controller, fromZone, "exile", "imprint")

	// Reciprocal metadata.
	perm.LinkedExile = append(perm.LinkedExile, target)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["imprinted"] = 1
	if target.Meta == nil {
		target.Meta = map[string]any{}
	}
	target.Meta["imprinted_by"] = perm.Timestamp

	sourceName := ""
	if perm.Card != nil {
		sourceName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "imprint",
		Seat:   perm.Controller,
		Source: sourceName,
		Details: map[string]interface{}{
			"target":   target.DisplayName(),
			"from":     fromZone,
			"host_ts":  perm.Timestamp,
			"rule":     "702.40a",
		},
	})
	return target, nil
}

// findCardZone locates `target` within `seat`'s zones and returns
// the matching zone string accepted by MoveCard. Order of search:
// hand → library → graveyard → battlefield (as a *Card lookup on
// permanents' Card pointer). Returns ("", false) if not found.
func findCardZone(seat *Seat, target *Card) (string, bool) {
	if seat == nil || target == nil {
		return "", false
	}
	for _, c := range seat.Hand {
		if c == target {
			return "hand", true
		}
	}
	for _, c := range seat.Library {
		if c == target {
			return "library", true
		}
	}
	for _, c := range seat.Graveyard {
		if c == target {
			return "graveyard", true
		}
	}
	for _, p := range seat.Battlefield {
		if p != nil && p.Card == target {
			return "battlefield", true
		}
	}
	for _, c := range seat.Exile {
		if c == target {
			return "exile", true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// GetImprintedCard
// ---------------------------------------------------------------------------

// GetImprintedCard returns the currently-imprinted card on `perm`,
// or nil if no imprint is active.
//
// Returns:
//   - the first card in perm.LinkedExile if perm has live imprint
//     state (Flags["imprinted"] == 1)
//   - nil if perm is nil, not imprinted, or has had its imprint
//     released via ReleaseImprint
//
// For permanents that imprint multiple cards (rare — most printed
// imprint cards exile exactly one), callers can iterate
// perm.LinkedExile directly. This helper returns the first slot
// because that matches the printed semantics of single-imprint
// cards (Chrome Mox, Isochron Scepter, Soul Foundry, etc.).
func GetImprintedCard(perm *Permanent) *Card {
	if perm == nil || perm.Flags == nil {
		return nil
	}
	if perm.Flags["imprinted"] != 1 {
		return nil
	}
	if len(perm.LinkedExile) == 0 {
		return nil
	}
	return perm.LinkedExile[0]
}

// GetImprintedCards returns all imprinted cards on `perm` in the
// order they were imprinted (oldest first). Empty slice (not nil)
// when no imprint state is active. Exposed for the rare multi-
// imprint case; single-imprint callers should prefer
// GetImprintedCard.
func GetImprintedCards(perm *Permanent) []*Card {
	if perm == nil || perm.Flags == nil || perm.Flags["imprinted"] != 1 {
		return []*Card{}
	}
	out := make([]*Card, len(perm.LinkedExile))
	copy(out, perm.LinkedExile)
	return out
}

// ---------------------------------------------------------------------------
// ReleaseImprint
// ---------------------------------------------------------------------------

// ReleaseImprint dissolves the imprint association on `perm`.
// Typically called from the LTB hook when the imprint host leaves
// the battlefield (CR §702.40b implicitly — the ability that
// references the imprinted card stops finding it).
//
// Side effects:
//   - clears perm.Flags["imprinted"]
//   - clears perm.LinkedExile (the slice — imprint cards do NOT
//     auto-return; that's a Fiend Hunter §406.7 thing)
//   - clears each exiled card's Meta["imprinted_by"]
//   - logs an "imprint_released" event
//
// The exiled cards STAY in exile after release — imprint only
// stops the imprint clause from finding them. If a future effect
// needs to retrieve them (e.g. a card that returns "a card you own
// in exile"), it can still pick them up through normal exile-zone
// reads.
func ReleaseImprint(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["imprinted"] != 1 {
		return
	}
	released := make([]string, 0, len(perm.LinkedExile))
	for _, c := range perm.LinkedExile {
		if c == nil {
			continue
		}
		released = append(released, c.DisplayName())
		if c.Meta != nil {
			delete(c.Meta, "imprinted_by")
		}
	}
	perm.LinkedExile = nil
	delete(perm.Flags, "imprinted")

	sourceName := ""
	if perm.Card != nil {
		sourceName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "imprint_released",
		Seat:   perm.Controller,
		Source: sourceName,
		Details: map[string]interface{}{
			"released_cards": released,
			"rule":           "702.40b",
		},
	})
}
