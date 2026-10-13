package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// hand_discard_snapshot_r88.go — the canonical "discards their hand" shape.
//
// THE DEFECT THIS REPLACES (caught live on prod .206, 2026-10-13, filed
// 1e9fbc70). Seven handlers emptied a hand with
//
//	for len(s.Hand) > 0 { gameengine.DiscardCard(gs, s.Hand[0], i) }
//
// which is an unbounded loop whose exit condition its own body can undo:
// DiscardCard fires the card_discarded trigger at resolve.go:1179, and
// anything that trigger resolves which puts a card back into that same
// hand keeps len(Hand) > 0 forever. Both grinder workers were found
// wedged in exactly this loop in Whispering Madness, each with the same
// 17-frame FLAT stack and both runnable — burning CPU, producing nothing,
// for 70+ minutes.
//
// NEITHER ENGINE LOOP GUARD BOUNDS IT. The stacks are flat rather than
// recursive, so maxPerCardInlineResolveDepth's _percard_inline_depth
// never sees it; and the outer loop is not a trigger dispatch, so the
// per-turn trigger_total budget never sees it either. Both guards
// measure the wrong dimension for this shape — which is the note already
// written in maxPerCardInlineResolveDepth's own doc comment about its
// sibling guards.
//
// THE FIX IS ALSO THE RULES-CORRECT READING. CR §701.8a: discarding a
// hand means discarding the cards that are in that hand as the effect is
// applied. A card drawn or returned to hand by a trigger that this very
// discard caused was never part of "their hand" for this effect, so it
// must not be discarded. Snapshotting is not a loop guard bolted on top
// of the wrong semantics — it is the semantics.
//
// MEMBERSHIP MUST BE RE-CHECKED PER CARD, and this is the non-obvious
// half. DiscardCard does NOT verify the card is still in hand: it calls
// MoveCard(…, "hand", dest, …), whose removeCardFromZone silently
// no-ops when the card is not found and then APPENDS to the destination
// zone. So discarding a stale snapshot entry — one a trigger already
// moved elsewhere — fabricates a second copy of that card in the
// graveyard, which is a CardIdentity invariant violation. A snapshot
// without cardStillInHand trades a hang for a duplication bug.

// handSnapshot copies seat's hand as it stands right now. Callers iterate
// the copy so that cards arriving in hand mid-effect are not swept into
// it, and re-check each entry with cardStillInHand before moving it.
func handSnapshot(gs *gameengine.GameState, seat int) []*gameengine.Card {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seat]
	if s == nil || len(s.Hand) == 0 {
		return nil
	}
	out := make([]*gameengine.Card, len(s.Hand))
	copy(out, s.Hand)
	return out
}

// cardStillInHand reports whether card is in seat's hand at this instant.
// A snapshot entry can leave the hand before its turn comes — a madness
// exile, a trigger that mills or reveals it, an opponent's effect — and
// moving it again would duplicate it (see the file header).
func cardStillInHand(gs *gameengine.GameState, seat int, card *gameengine.Card) bool {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	if s == nil {
		return false
	}
	for _, c := range s.Hand {
		if c == card {
			return true
		}
	}
	return false
}

// discardHandSnapshot discards the cards that are in seat's hand at the
// moment of the call and returns how many were actually discarded.
//
// The return value is the number to use for any "equal to the number of
// cards discarded this way" clause (Whispering Madness, Windfall). It is
// the count of cards that really changed zones, not the pre-pass hand
// size, so a card a trigger removed from hand before its turn came is
// correctly excluded.
func discardHandSnapshot(gs *gameengine.GameState, seat int) int {
	discarded := 0
	for _, c := range handSnapshot(gs, seat) {
		if !cardStillInHand(gs, seat, c) {
			continue
		}
		gameengine.DiscardCard(gs, c, seat)
		discarded++
	}
	return discarded
}
