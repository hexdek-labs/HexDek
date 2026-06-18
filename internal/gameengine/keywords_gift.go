package gameengine

// keywords_gift.go — Gift (CR §702.192, Bloomburrow 2024) as a real
// optional-promise cast-time choice with resolution-time token creation
// in the recipient's zone.
//
// CR §702.192a: "Gift a [token type]" is an optional additional choice
//                you make as you cast a spell with gift. As you cast
//                the spell, you may promise that gift to a chosen
//                opponent. If you do, the spell's effect text changes
//                — typically you get a bonus AND the chosen opponent
//                receives the gift token on resolution.
// CR §702.192b: The chosen opponent must be an OPPONENT — you cannot
//                gift yourself. Cards print "an opponent of your
//                choice"; the cast-time choice records who.
// CR §702.192c: The gift is created on resolution, NOT on cast. It
//                goes to the chosen opponent's battlefield. Whether
//                the gift was promised is recorded on the stack item
//                so per-card resolution handlers can read it AND
//                drive the bonus effect.
//
// Engine surface (canonical):
//
//   - HasGift(card) bool
//       AST keyword detector. The parser emits "Gift a [token type]"
//       preambles as a Keyword ability with name "gift" and the token
//       type as the keyword arg.
//
//   - GiftType(card) string
//       The token type promised. Returns the lowercased name of the
//       token ("treasure", "clue", "food", "blood", "map", "junk",
//       "powerstone", "gold") OR an empty string for non-gift cards.
//       Per-card handlers that print non-standard gifts (e.g. a 1/1
//       creature token) parse beyond this engine MVP.
//
//   - CastWithGift(gs, seat, card, recipient) error
//       Records the §702.192a cast-time promise. Atomic validation:
//       card has gift, recipient is a valid opponent (not the caster,
//       not a Lost seat). On success: pushes a StackItem with
//       CostMeta["gift_recipient"]=recipient and
//       CostMeta["gift_type"]=<token type>, emits a gift_promised
//       event.
//
//   - ResolveGift(gs, item) bool
//       The §702.192c resolution-time payoff. Called by per-card
//       resolve handlers after the spell's base effect resolves.
//       Reads CostMeta["gift_recipient"] + CostMeta["gift_type"];
//       creates the appropriate token in the recipient's battlefield;
//       emits a gift_delivered event. Returns true if a token was
//       created (i.e. the spell was cast with a gift promised).
//
// Token-type dispatch:
//   - "treasure"   → CreateTreasureToken
//   - "clue"       → CreateClueToken
//   - "food"       → CreateFoodToken
//   - "blood"      → CreateBloodToken
//   - "map"        → CreateMapToken
//   - "gold"       → CreateGoldToken
//   - "powerstone" → CreatePowerstoneToken
//   - "junk"       → CreateJunkToken
//
// Non-standard gifts (creature tokens with specific stats, custom
// token types) are dispatched by the per-card resolve handler — the
// engine surface intentionally covers only the canonical artifact
// token types. ResolveGift returns false when the gift_type isn't
// one of the canonical eight; the per-card handler should call its
// own token-creation primitive in that case.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasGift / GiftType
// ---------------------------------------------------------------------------

// HasGift returns true if the card has the gift keyword.
func HasGift(card *Card) bool {
	return cardHasKeywordByName(card, "gift")
}

// GiftType returns the token-type string promised by the gift keyword.
// Lowercased, trimmed. Empty string when the keyword is absent or
// the arg is malformed.
//
// Examples (from BLB printings):
//   - "Gift a Treasure" → "treasure"
//   - "Gift a Clue"     → "clue"
//   - "Gift a Food"     → "food"
//   - "Gift a Map"      → "map"
func GiftType(card *Card) string {
	if card == nil || card.AST == nil {
		return ""
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "gift") {
			continue
		}
		if len(kw.Args) == 0 {
			return ""
		}
		if s, ok := kw.Args[0].(string); ok {
			return strings.ToLower(strings.TrimSpace(s))
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// DetectGift — real-card (parsed-AST) gift detection + cast-path wiring
// ---------------------------------------------------------------------------
//
// Printed BLB/OTJ gift cards do NOT parse the "Gift a <thing>" preamble to
// a Keyword node — the parser emits a Static ability whose Raw is "gift a
// <thing>" (the body lands in a parsed_effect_residual scaffold). So the
// keyword-based HasGift / GiftType above match only synthetic unit
// fixtures, never a real card. DetectGift bridges both representations so
// the canonical cast pipeline (CastSpell) can offer the §702.192a promise.

// canonicalGiftTokens is the set of artifact-token gift types ResolveGift
// can mint directly. Non-token gifts (a card draw, a tapped creature
// token, an extra turn) map to "" — their delivery is the per_card
// handler's concern (or an open gap), but the gift_promised flag still
// gates the spell's bonus either way.
var canonicalGiftTokens = map[string]bool{
	"treasure": true, "clue": true, "food": true, "blood": true,
	"map": true, "gold": true, "powerstone": true, "junk": true,
}

// DetectGift reports whether a spell carries the gift mechanic (CR
// §702.192) and the canonical gift-token type promised ("" for a
// non-token gift). It recognizes BOTH the synthetic Keyword form
// (HasGift/GiftType, used by fixtures) and the real parsed form (a Static
// ability whose Raw is a "gift a <thing>" / "gift an <thing>" preamble).
func DetectGift(card *Card) (bool, string) {
	if card == nil {
		return false, ""
	}
	if HasGift(card) {
		return true, GiftType(card)
	}
	if card.AST == nil {
		return false, ""
	}
	for _, ab := range card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok {
			continue
		}
		if t, ok := parseGiftRaw(st.Raw); ok {
			return true, t
		}
	}
	return false, ""
}

// parseGiftRaw extracts the gift-token type from a "gift a <thing>" /
// "gift an <thing>" preamble. ok=false when raw isn't a gift preamble.
// The returned type is a canonical artifact-token name, or "" for a
// non-token gift (still a valid gift — ok=true).
func parseGiftRaw(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	var rest string
	switch {
	case strings.HasPrefix(s, "gift a "):
		rest = strings.TrimSpace(s[len("gift a "):])
	case strings.HasPrefix(s, "gift an "):
		rest = strings.TrimSpace(s[len("gift an "):])
	default:
		return "", false
	}
	word := rest
	if i := strings.IndexByte(rest, ' '); i >= 0 {
		word = rest[:i]
	}
	if canonicalGiftTokens[word] {
		return word, true
	}
	return "", true
}

// chooseGiftRecipient picks the opponent who receives the gift (CR
// §702.192b — must be an opponent, never the caster). Baseline policy:
// the first living opponent in seat order (the lone opponent at a
// 2-player table). Returns -1 when the caster has no living opponent
// (CR §800.4 — then no gift can be promised).
func chooseGiftRecipient(gs *GameState, caster int) int {
	if gs == nil {
		return -1
	}
	for _, opp := range gs.Opponents(caster) {
		if opp < 0 || opp >= len(gs.Seats) {
			continue
		}
		if s := gs.Seats[opp]; s != nil && !s.Lost {
			return opp
		}
	}
	return -1
}

// StampGiftPromise records the §702.192a cast-time promise on an
// already-pushed StackItem: sets CostMeta gift_promised/recipient/type,
// logs gift_promised, and fires the gift_promised trigger. Mirrors
// CastWithGift's stamping but operates on an existing item (the canonical
// CastSpell pipeline builds + pushes the item itself, then layers
// optional cast-time choices onto it).
func StampGiftPromise(gs *GameState, item *StackItem, caster, recipient int, giftType string) {
	if gs == nil || item == nil {
		return
	}
	if item.CostMeta == nil {
		item.CostMeta = map[string]interface{}{}
	}
	item.CostMeta["gift_promised"] = true
	item.CostMeta["gift_recipient"] = recipient
	item.CostMeta["gift_type"] = giftType

	name := giftStackItemName(item)
	gs.LogEvent(Event{
		Kind:   "gift_promised",
		Seat:   caster,
		Target: recipient,
		Source: name,
		Details: map[string]interface{}{
			"gift_type": giftType,
			"rule":      "702.192a",
		},
	})
	FireCardTrigger(gs, "gift_promised", map[string]interface{}{
		"card":            item.Card,
		"card_name":       name,
		"controller_seat": caster,
		"recipient_seat":  recipient,
		"gift_type":       giftType,
	})
}

// ---------------------------------------------------------------------------
// CastWithGift
// ---------------------------------------------------------------------------

// CastWithGift records the §702.192a cast-time promise. The caller is
// responsible for the surrounding cast pipeline (mana payment, regular
// targeting); this helper only handles the gift side of the cast and
// produces a StackItem flagged with the gift metadata.
//
// Validation (atomic — no mutation on failure):
//   - card non-nil and carries the gift keyword
//   - recipient in range and != caster (§702.192b — opponent only)
//   - recipient seat not Lost (CR §800.4 — can't gift an eliminated
//     player)
//
// On success:
//   - Pushes a StackItem with:
//       Card:       card
//       Controller: seatIdx
//       CastZone:   "hand"
//       Effect:     collectSpellEffect(card)
//       CostMeta:
//         "gift_promised"   = true
//         "gift_recipient"  = recipient seat index
//         "gift_type"       = GiftType(card) (lowercased)
//   - Emits a "gift_promised" log event with caster, recipient, type
//   - Fires FireCardTrigger("gift_promised", ctx)
//
// Mana cost for the spell is NOT handled here — this helper plumbs
// only the gift-side cost. Cast pipelines wrap CastWithGift around
// their normal mana payment.
//
// Returns nil on success, *CastError on failure.
func CastWithGift(gs *GameState, seatIdx int, card *Card, recipient int) error {
	if gs == nil {
		return &CastError{Reason: "nil_game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid_caster_seat"}
	}
	if card == nil {
		return &CastError{Reason: "nil_card"}
	}
	if !HasGift(card) {
		return &CastError{Reason: "no_gift_keyword"}
	}
	// §702.192b — recipient must be an opponent of the caster.
	if recipient == seatIdx {
		return &CastError{Reason: "gift_self_forbidden"}
	}
	if recipient < 0 || recipient >= len(gs.Seats) {
		return &CastError{Reason: "invalid_recipient_seat"}
	}
	recSeat := gs.Seats[recipient]
	if recSeat == nil || recSeat.Lost {
		return &CastError{Reason: "recipient_eliminated"}
	}

	giftType := GiftType(card)
	if giftType == "" {
		return &CastError{Reason: "gift_type_unspecified"}
	}

	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"gift_promised":  true,
			"gift_recipient": recipient,
			"gift_type":      giftType,
		},
	}
	PushStackItem(gs, item)

	name := card.DisplayName()
	gs.LogEvent(Event{
		Kind:   "gift_promised",
		Seat:   seatIdx,
		Target: recipient,
		Source: name,
		Details: map[string]interface{}{
			"gift_type": giftType,
			"rule":      "702.192a",
		},
	})
	FireCardTrigger(gs, "gift_promised", map[string]interface{}{
		"card":            card,
		"card_name":       name,
		"controller_seat": seatIdx,
		"recipient_seat":  recipient,
		"gift_type":       giftType,
	})
	return nil
}

// ---------------------------------------------------------------------------
// ResolveGift
// ---------------------------------------------------------------------------

// ResolveGift drives the §702.192c resolution-time payoff. Called by
// per-card resolve handlers after the spell's base effect runs. Reads
// the gift metadata from item.CostMeta:
//
//   - CostMeta["gift_promised"]  == true  → a gift was promised
//   - CostMeta["gift_recipient"] int      → recipient seat index
//   - CostMeta["gift_type"]      string   → canonical token type
//
// If the metadata is absent or "gift_promised" is false/missing, this
// is a no-op (returns false). When present, dispatches to the
// canonical artifact-token creator for the named type and emits a
// gift_delivered event.
//
// Returns true if a token was created. False for:
//   - nil game/item, missing CostMeta, gift not promised
//   - recipient seat invalid or eliminated mid-resolution
//   - gift_type not in the canonical eight (per-card handlers cover
//     non-canonical gifts like creature tokens)
func ResolveGift(gs *GameState, item *StackItem) bool {
	if gs == nil || item == nil || item.CostMeta == nil {
		return false
	}
	promised, _ := item.CostMeta["gift_promised"].(bool)
	if !promised {
		return false
	}
	recipient, ok := item.CostMeta["gift_recipient"].(int)
	if !ok {
		return false
	}
	if recipient < 0 || recipient >= len(gs.Seats) {
		return false
	}
	recSeat := gs.Seats[recipient]
	if recSeat == nil || recSeat.Lost {
		gs.LogEvent(Event{
			Kind:   "gift_lost_recipient_eliminated",
			Source: giftStackItemName(item),
			Target: recipient,
			Details: map[string]interface{}{
				"rule": "702.192c",
			},
		})
		return false
	}
	giftType, _ := item.CostMeta["gift_type"].(string)
	giftType = strings.ToLower(strings.TrimSpace(giftType))

	created := false
	switch giftType {
	case "treasure":
		CreateTreasureToken(gs, recipient)
		created = true
	case "clue":
		CreateClueToken(gs, recipient)
		created = true
	case "food":
		CreateFoodToken(gs, recipient)
		created = true
	case "blood":
		CreateBloodToken(gs, recipient)
		created = true
	case "map":
		CreateMapToken(gs, recipient)
		created = true
	case "gold":
		CreateGoldToken(gs, recipient)
		created = true
	case "powerstone":
		CreatePowerstoneToken(gs, recipient)
		created = true
	case "junk":
		CreateJunkToken(gs, recipient)
		created = true
	}

	if created {
		gs.LogEvent(Event{
			Kind:   "gift_delivered",
			Seat:   item.Controller,
			Target: recipient,
			Source: giftStackItemName(item),
			Details: map[string]interface{}{
				"gift_type": giftType,
				"rule":      "702.192c",
			},
		})
		FireCardTrigger(gs, "gift_delivered", map[string]interface{}{
			"card_name":       giftStackItemName(item),
			"controller_seat": item.Controller,
			"recipient_seat":  recipient,
			"gift_type":       giftType,
		})
	}
	return created
}

// giftStackItemName returns a human-readable name for the stack item's
// source — the card name when available, "<unknown>" otherwise. Used
// only by ResolveGift logging.
func giftStackItemName(item *StackItem) string {
	if item == nil {
		return "<unknown>"
	}
	if item.Card != nil {
		return item.Card.DisplayName()
	}
	if item.Source != nil && item.Source.Card != nil {
		return item.Source.Card.DisplayName()
	}
	return "<unknown>"
}
