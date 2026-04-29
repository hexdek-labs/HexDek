package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// emit writes the canonical per-card log entry. Mirrors Python's
// per_card_runtime._emit contract: ("per_card_handler", slug, card, ...).
func emit(gs *gameengine.GameState, slug string, cardName string, details map[string]interface{}) {
	if gs == nil {
		return
	}
	d := map[string]interface{}{"slug": slug, "card": cardName}
	for k, v := range details {
		d[k] = v
	}
	gs.LogEvent(gameengine.Event{
		Kind:    "per_card_handler",
		Source:  cardName,
		Details: d,
	})
}

// emitFail writes a graceful-skip event.
func emitFail(gs *gameengine.GameState, slug, cardName, reason string, details map[string]interface{}) {
	if gs == nil {
		return
	}
	d := map[string]interface{}{"slug": slug, "card": cardName, "reason": reason}
	for k, v := range details {
		d[k] = v
	}
	gs.LogEvent(gameengine.Event{
		Kind:    "per_card_failed",
		Source:  cardName,
		Details: d,
	})
}

// emitPartial writes a "handler worked but clause N is unimplemented" event.
func emitPartial(gs *gameengine.GameState, slug, cardName, missing string) {
	if gs == nil {
		return
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "per_card_partial",
		Source: cardName,
		Details: map[string]interface{}{
			"slug":    slug,
			"card":    cardName,
			"missing": missing,
		},
	})
}

// emitWin writes the canonical win event, flipping the winner's seat +
// every other seat's Lost flag. Called by Thassa's Oracle.
func emitWin(gs *gameengine.GameState, winnerSeat int, slug, cardName, reason string) {
	if gs == nil || winnerSeat < 0 || winnerSeat >= len(gs.Seats) {
		return
	}
	winner := gs.Seats[winnerSeat]
	if winner == nil {
		return
	}
	winner.Won = true
	for i, s := range gs.Seats {
		if s == nil || i == winnerSeat {
			continue
		}
		if !s.Lost {
			s.Lost = true
			s.LossReason = reason
		}
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "per_card_win",
		Seat:   winnerSeat,
		Target: winnerSeat,
		Source: cardName,
		Details: map[string]interface{}{
			"slug":        slug,
			"winner_seat": winnerSeat,
			"reason":      reason,
		},
	})
	// Ensure CheckEnd runs so SBAs see the game-over state.
	_ = gs.CheckEnd()
}

// -----------------------------------------------------------------------------
// Devotion counting
// -----------------------------------------------------------------------------

// countDevotion returns the seat's devotion to a set of color symbols.
// Devotion = number of mana symbols of those colors across all mana costs
// of permanents you control (CR §700.5). Empty color slice → 0.
//
// Implementation: walks each permanent's Card and sums pip counts. We
// source pips from three places, in priority order:
//
//  1. Card.Types tokens of shape "pip:U" / "pip:B" etc. (test-friendly;
//     callers can hand-craft devotion values without loading a corpus).
//  2. A matching Card.AST and its Activated/Triggered abilities that
//     carry ManaCost info (Phase 14 — not implemented; we log partial).
//  3. Card.AST ability-cost aggregation (not applicable — CR §700.5
//     counts the CARD's cost, not ability costs).
//
// Hybrid cards (e.g. {U/B}) count as 1 toward EACH of their colors.
// Phyrexian mana ({U/P}) counts as one pip of U regardless of whether
// it was paid with life or mana (CR §107.4e).
func countDevotion(seat *gameengine.Seat, colors ...string) int {
	if seat == nil || len(colors) == 0 {
		return 0
	}
	want := map[string]bool{}
	for _, c := range colors {
		want[strings.ToUpper(strings.TrimSpace(c))] = true
	}
	total := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		total += countPipsOnCard(p.Card, want)
	}
	return total
}

// countPipsOnCard counts matching mana pips on a single card's cost.
// Reads from Card.Types ("pip:U", "pip:W" etc.) first since we don't
// have a CardAST.ManaCost field yet (Phase 14).
func countPipsOnCard(card *gameengine.Card, wantColors map[string]bool) int {
	if card == nil {
		return 0
	}
	n := 0
	for _, t := range card.Types {
		if !strings.HasPrefix(t, "pip:") {
			continue
		}
		color := strings.ToUpper(strings.TrimPrefix(t, "pip:"))
		if wantColors[color] {
			n++
		}
	}
	// Future: if card.AST carried an explicit ManaCost we'd loop
	// symbols here. Not implemented in the Phase 3 AST.
	return n
}

// -----------------------------------------------------------------------------
// Battlefield search helpers
// -----------------------------------------------------------------------------

// findPermanentByName returns the first battlefield permanent whose Card
// name matches (case-insensitive) or nil. Scans across all seats.
func findPermanentByName(gs *gameengine.GameState, name string) *gameengine.Permanent {
	if gs == nil {
		return nil
	}
	want := strings.ToLower(name)
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if strings.ToLower(p.Card.DisplayName()) == want {
				return p
			}
		}
	}
	return nil
}

// permIsType returns true if the permanent has the named type. Wrapper
// around gameengine's private hasType via public predicates.
func permIsType(p *gameengine.Permanent, t string) bool {
	if p == nil {
		return false
	}
	switch strings.ToLower(t) {
	case "creature":
		return p.IsCreature()
	case "land":
		return p.IsLand()
	case "artifact":
		return p.IsArtifact()
	case "enchantment":
		return p.IsEnchantment()
	case "planeswalker":
		return p.IsPlaneswalker()
	case "battle":
		return p.IsBattle()
	}
	// Fallback: scan card types directly.
	if p.Card == nil {
		return false
	}
	for _, got := range p.Card.Types {
		if strings.EqualFold(got, t) {
			return true
		}
	}
	return false
}

// cardHasType mirrors the engine's internal predicate for Card.
func cardHasType(c *gameengine.Card, t string) bool {
	if c == nil {
		return false
	}
	want := strings.ToLower(t)
	for _, got := range c.Types {
		if strings.ToLower(got) == want {
			return true
		}
	}
	return false
}

// moveCardBetweenZones routes a card through MoveCard so §614 replacements,
// §903.9b commander redirect, and zone-change triggers all fire correctly.
func moveCardBetweenZones(gs *gameengine.GameState, seat int, card *gameengine.Card, fromZone, toZone, reason string) string {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return ""
	}
	return gameengine.MoveCard(gs, card, seat, fromZone, toZone, reason)
}

// removePermanent detaches a permanent from its controller's battlefield.
// Returns true if it was found and removed.
func removePermanent(gs *gameengine.GameState, p *gameengine.Permanent) bool {
	if gs == nil || p == nil {
		return false
	}
	if p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[p.Controller]
	for i, q := range seat.Battlefield {
		if q == p {
			seat.Battlefield = append(seat.Battlefield[:i], seat.Battlefield[i+1:]...)
			return true
		}
	}
	return false
}

// createPermanent puts a card onto seat's battlefield as a new Permanent.
// Summoning-sick defaults to true for creatures (haste is checked via AST).
func createPermanent(gs *gameengine.GameState, seat int, card *gameengine.Card, tapped bool) *gameengine.Permanent {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	sick := false
	if cardHasType(card, "creature") {
		sick = !cardHasKeyword(card, "haste")
	}
	perm := &gameengine.Permanent{
		Card:          card,
		Controller:    seat,
		Owner:         card.Owner,
		Tapped:        tapped,
		SummoningSick: sick,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

// enterBattlefieldWithETB creates a permanent and fires the full ETB
// cascade: replacement registration, per-card ETB hook, and observer
// ETB triggers. Use for any card entering the battlefield from a
// non-stack path (fetchlands, reanimation, Sneak Attack, token creation,
// etc.). For tokens, the card.Types must include "token".
func enterBattlefieldWithETB(gs *gameengine.GameState, seat int, card *gameengine.Card, tapped bool) *gameengine.Permanent {
	perm := createPermanent(gs, seat, card, tapped)
	if perm == nil {
		return nil
	}
	gameengine.RegisterReplacementsForPermanent(gs, perm)
	gameengine.FirePermanentETBTriggers(gs, perm)
	return perm
}

// cardHasKeyword mirrors the engine's internal helper.
func cardHasKeyword(c *gameengine.Card, name string) bool {
	if c == nil || c.AST == nil {
		return false
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for _, ab := range c.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) == want {
			return true
		}
	}
	return false
}
