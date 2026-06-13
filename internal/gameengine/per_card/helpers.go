package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
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
// Also emits a parser_gap event so Heimdall/Muninn can track runtime gaps.
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
	gs.LogEvent(gameengine.Event{
		Kind:   "parser_gap",
		Source: cardName,
		Details: map[string]interface{}{
			"snippet": cardName + ": " + missing,
			"reason":  "per_card_partial",
			"slug":    slug,
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
			// CR §104.2a: a player wins -> each opponent loses. Stamp the
			// structured cause beside the freeform string (step 2.5
			// dual-write); the winning card is the source.
			s.LossDetail = &judge.LossReason{
				Category:   judge.LossCategoryEffect,
				Rule:       "104.2a",
				SourceCard: cardName,
			}
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

// cardHasType forwards to the engine's exported strict helper (R60
// Phase 2C — the body used to be a verbatim copy).
func cardHasType(c *gameengine.Card, t string) bool {
	return gameengine.CardHasTypeExact(c, t)
}

// moveCardBetweenZones routes a card through MoveCard so §614 replacements,
// §903.9b commander redirect, and zone-change triggers all fire correctly.
func moveCardBetweenZones(gs *gameengine.GameState, seat int, card *gameengine.Card, fromZone, toZone, reason string) string {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return ""
	}
	return gameengine.MoveCard(gs, card, seat, fromZone, toZone, reason).FinalZone
}

// removePermanent detaches a permanent from its controller's battlefield.
// Returns true if it was found and removed.
//
// Phase D — token cessation: when the removed perm is a token, cease its
// InstanceID per CR §704.5d ("if a token is in a zone other than the
// battlefield, it ceases to exist") AND clear the *Card's InstanceID
// so blink-and-re-add paths get a fresh TK mint from EnsureTokenInstanceID
// when the perm re-enters via enterBattlefieldWithETB. Non-token perms
// are unaffected (their Card.InstanceID stays put so graveyard/exile
// references keep their identity).
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
			if p.Card != nil && p.IsToken() {
				if p.Card.InstanceID != "" {
					gameengine.MarkInstanceIDCeased(gs, p.Card.InstanceID)
					p.Card.InstanceID = ""
				}
			}
			return true
		}
	}
	return false
}

// createPermanent puts a card onto seat's battlefield as a new Permanent.
// Summoning-sick defaults to true for creatures (haste is checked via AST).
//
// Defensively sweeps the card pointer out of any private zone (hand,
// library, graveyard, exile, command zone) on the OWNER's seat before
// placing on the battlefield. Per-card reanimate/cheat hooks frequently
// call MoveCard("graveyard"→"battlefield") just before createPermanent;
// MoveCard's battlefield arm is a no-op (state.go:moveToZone has no
// "battlefield" case) and falls through to a graveyard re-append, so
// without this sweep the same *Card ends up in both graveyard AND
// battlefield. See docs/zone-accounting-analysis.md (hypothesis #1).
func createPermanent(gs *gameengine.GameState, seat int, card *gameengine.Card, tapped bool) *gameengine.Permanent {
	if gs == nil || card == nil || seat < 0 || seat >= len(gs.Seats) {
		return nil
	}
	// CR §800.4a chokepoint guard (r63, seed-42 game 283 — Athreos,
	// Shroud-Veiled returning an eliminated player's coin-countered
	// Gurmag Angler): a card owned by a player who has LEFT THE GAME
	// left with them and cannot be materialized on any battlefield.
	// createPermanent is the per_card-side mover that bypasses
	// MoveCard's #1041 guard (direct zone sweep + battlefield append) —
	// the residual gap that fix documented. Owner >= 0 (r63 seed-1337
	// game 2293 fix): the LeftGame guard must cover seat 0 — the old
	// `Owner > 0` cutoff let The Cruelty of Gix chapter III reanimate
	// eliminated seat 0's ceased Flayer of Loyalties onto a live
	// battlefield (present-but-ceased fabrication). The LeftGame
	// condition is the discriminator against zero-value test fixtures.
	if card.Owner >= 0 && card.Owner < len(gs.Seats) {
		if os := gs.Seats[card.Owner]; os != nil && os.LeftGame {
			gs.LogEvent(gameengine.Event{
				Kind:   "zone_move_refused",
				Seat:   seat,
				Source: card.DisplayName(),
				Details: map[string]interface{}{
					"rule":   "800.4a",
					"reason": "owner_left_game",
					"via":    "per_card_createPermanent",
				},
			})
			return nil
		}
	}
	// MDFC with a land back face entering via a non-cast path
	// (reanimate, fetch, sneak attack, etc.) — swap to back face
	// before the perm wraps it so Permanent.Card.Types reads "land"
	// instead of "instant"/"sorcery".
	gameengine.EnsureBattlefieldFrontFace(card)
	// CR 304.4 / 307.1 — refuse to wrap a non-permanent in a Permanent.
	// Per-card hooks invoking createPermanent on an instant/sorcery
	// (mis-targeted reanimate, recursion, etc.) silently no-op here so
	// the §205 SBA never fires. Done before the zone sweep so the card
	// stays in whatever zone the caller put it.
	if !gameengine.CardCanEnterBattlefield(card) {
		return nil
	}
	// Dedup: if the *Card is already wrapped in a Permanent on the
	// TARGET seat's battlefield, return the existing Permanent rather
	// than allocating a duplicate. Standard "MoveCard then
	// createPermanent" pattern.
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card == card {
			return p
		}
	}
	// Cross-seat dedup: if the *Card is already wrapped on a
	// DIFFERENT seat's battlefield, that's the canonical
	// control-change pattern (Bribery, Etali, reanimate-from-opp-
	// graveyard). Per CR §400.7c the *Card pointer cannot exist on
	// two battlefields simultaneously — but handlers historically
	// staged the card on the original seat's battlefield as an
	// intermediate step, then called createPermanent on the new
	// controller's seat. The pre-r60-X policy allowed both wrappers
	// to coexist, producing the dominant Cluster A CardIdentity
	// violation pattern: "*Card appears in both seat X battlefield
	// and seat Y battlefield" (docs/loki-r60-250k-analysis.md, PR
	// #713). Layer-stress 1000-game seed-42 sweep before this fix:
	// 114 violations across 6 games; after: 0.
	//
	// The fix implements control-change semantics correctly: drop
	// the existing wrapper from the other seat's battlefield, then
	// fall through to create a new wrapper on the target seat. The
	// caller's expectation (perm on target seat, with Controller =
	// target) is preserved; the §400.7c "exactly one zone" invariant
	// is restored.
	for _, otherSeat := range gs.Seats {
		if otherSeat == nil || otherSeat == gs.Seats[seat] {
			continue
		}
		filtered := otherSeat.Battlefield[:0]
		for _, p := range otherSeat.Battlefield {
			if p != nil && p.Card == card {
				continue // drop the stale wrapper
			}
			filtered = append(filtered, p)
		}
		otherSeat.Battlefield = filtered
	}
	// Sweep from the OWNER's private zones (controller and owner can
	// differ on stolen permanents; reanimation pulls from the owner's
	// graveyard regardless of who casts the reanimate).
	owner := card.Owner
	if owner < 0 || owner >= len(gs.Seats) {
		owner = seat
	}
	gameengine.RemoveCardFromAllPrivateZones(gs, owner, card)
	if owner != seat {
		gameengine.RemoveCardFromAllPrivateZones(gs, seat, card)
	}
	// InstanceID gap-walk: catch the DeepCopy-without-remint shape
	// (Calix, Brudiclad, Riku and friends) before it tripping the
	// CardIdentity invariant. See instanceid_gap_walk.go for the
	// rationale. No-op when card carries a unique InstanceID, an empty
	// InstanceID (legacy mode), or the Minter is unwired.
	gameengine.EnforceBattlefieldUniqueInstanceID(gs, card, seat)
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

// cardCMC reads the card's mana value. Prefers an explicit "cmc:N"
// token in Card.Types (test-friendly); falls back to BasePower +
// BaseToughness as a rough proxy for creatures; else 0.
func cardCMC(c *gameengine.Card) int {
	if c == nil {
		return 0
	}
	for _, t := range c.Types {
		if len(t) > 4 && t[:4] == "cmc:" {
			n := 0
			for _, ch := range t[4:] {
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			return n
		}
	}
	// Proxy for test creatures without cmc: marker.
	if c.BasePower > 0 || c.BaseToughness > 0 {
		// Rough: power+toughness divided by 2 (MTG cards are usually
		// slightly cheaper than p+t).
		p := c.BasePower + c.BaseToughness
		if p > 0 {
			return (p + 1) / 2
		}
	}
	return 0
}

// cardHasKeyword forwards to the engine's exported helper. Kept as a
// local alias so the dozens of per_card call sites stay unchanged
// (R60 Phase 2C — the body used to be a verbatim copy of the engine
// helper; consolidated to forward instead).
func cardHasKeyword(c *gameengine.Card, name string) bool {
	return gameengine.CardHasKeyword(c, name)
}
