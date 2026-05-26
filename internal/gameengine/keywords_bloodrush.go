package gameengine

// keywords_bloodrush.go — Bloodrush (CR §702.99) as an activated-from-hand
// pump mechanic.
//
// CR §702.99a: Bloodrush is an activated ability that functions only
//              while the card with bloodrush is in a player's hand.
//              "Bloodrush — [cost], Discard this card: [effect]" means
//              "[cost], Discard this card: [effect]. Activate only if
//              a creature you control is attacking."
// CR §702.99b: Bloodrush triggered/static-style effects are NOT a cast —
//              the source card never goes on the stack as a spell. The
//              ability itself uses the stack as a normal activated
//              ability would, but the discarded card lands in its
//              owner's graveyard via the activation cost, not via spell
//              resolution.
//
// Architecture note: bloodrush is an activated ability, NOT an
// alternative cast. The card never becomes a spell. The flow is:
//
//   pay mana cost → discard source from hand → resolve pump effect
//
// The pump (P/+T and any granted ability words like trample) is applied
// to the targeted attacking creature using:
//   - Permanent.Modifications (Power/Toughness deltas with
//     Duration="until_end_of_turn") — so Power()/Toughness() report
//     the buffed values for combat damage
//   - Permanent.Flags["temp_power"] / ["temp_toughness"] — the stamped
//     marker the user explicitly requested, useful for diagnostics,
//     per_card readers, and any downstream tooling
//   - Permanent.GrantedAbilities — any ability words named in the
//     keyword (e.g. "trample") — cleared at end-of-turn cleanup
//
// Keyword argument shape (gameast.Keyword.Args):
//
//   args[0]  string  — mana cost ("{1}{R}", "{R}{R}", etc.)
//   args[1]  float64 — power delta
//   args[2]  float64 — toughness delta
//   args[3]  string  — optional comma-separated ability words granted
//                       (e.g. "trample", "flying,first strike")
//
// Numeric values are also accepted as int for hand-authored tests.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasBloodrush / BloodrushCost / BloodrushPump
// ---------------------------------------------------------------------------

// HasBloodrush returns true if the card has the bloodrush keyword in
// its AST.
func HasBloodrush(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "bloodrush") {
			return true
		}
	}
	return false
}

// bloodrushKeyword returns the card's bloodrush Keyword node, or nil.
func bloodrushKeyword(card *Card) *gameast.Keyword {
	if card == nil || card.AST == nil {
		return nil
	}
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "bloodrush") {
			return kw
		}
	}
	return nil
}

// BloodrushCost returns the converted mana cost of the bloodrush
// activation. Accepts arg[0] as either a mana string ("{1}{R}") or a
// plain numeric value. Returns 0 if the keyword is absent or args are
// malformed; callers should treat 0 as "free" only when they have
// positively confirmed HasBloodrush.
func BloodrushCost(card *Card) int {
	kw := bloodrushKeyword(card)
	if kw == nil || len(kw.Args) == 0 {
		return 0
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
	return 0
}

// BloodrushPump returns the (power, toughness) bonus and any granted
// ability words for the card's bloodrush keyword. Ability words are
// the raw lowercased names from arg[3] split on commas — callers may
// append them directly to target.GrantedAbilities.
//
// Returns (0, 0, nil) if the keyword is absent.
func BloodrushPump(card *Card) (int, int, []string) {
	kw := bloodrushKeyword(card)
	if kw == nil {
		return 0, 0, nil
	}
	p := readBloodrushInt(kw.Args, 1)
	t := readBloodrushInt(kw.Args, 2)
	var abilities []string
	if len(kw.Args) >= 4 {
		if s, ok := kw.Args[3].(string); ok && s != "" {
			abilities = splitCommaLower(s)
		}
	}
	return p, t, abilities
}

func readBloodrushInt(args []any, idx int) int {
	if idx >= len(args) {
		return 0
	}
	switch v := args[idx].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// splitCommaLower splits "trample,first strike" into
// ["trample","first strike"] with each element trimmed and lowercased.
func splitCommaLower(s string) []string {
	out := []string{}
	cur := make([]byte, 0, len(s))
	flush := func() {
		// trim leading/trailing ASCII whitespace
		for len(cur) > 0 && (cur[0] == ' ' || cur[0] == '\t') {
			cur = cur[1:]
		}
		for len(cur) > 0 && (cur[len(cur)-1] == ' ' || cur[len(cur)-1] == '\t') {
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			out = append(out, string(cur))
		}
		cur = cur[:0]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ',' {
			flush()
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		cur = append(cur, c)
	}
	flush()
	return out
}

// ---------------------------------------------------------------------------
// ActivateBloodrush
// ---------------------------------------------------------------------------

// ActivateBloodrush activates the bloodrush ability of `source` (a card
// in `seatIdx`'s hand) targeting `target` (an attacking creature on the
// battlefield). CR §702.99a–b.
//
// Preconditions enforced here:
//   - source has the bloodrush keyword
//   - source is in seatIdx's hand
//   - target is non-nil, on the battlefield, a creature, and IsAttacking()
//   - seatIdx can afford the bloodrush mana cost
//
// On success:
//  1. Mana cost is paid from seat.ManaPool.
//  2. Source is discarded from hand via DiscardCard (fires
//     card_discarded triggers per the activation cost).
//  3. The pump (+P/+T) is applied to target via:
//       - target.Flags["temp_power"]      += P
//       - target.Flags["temp_toughness"]  += T
//       - target.Modifications appends Modification{Power:P, Toughness:T,
//         Duration:"until_end_of_turn"} so Power()/Toughness() reflect
//         the buff for combat damage
//  4. Any ability words from the keyword (e.g. "trample") are appended
//     to target.GrantedAbilities (cleared at EOT alongside Modifications).
//  5. A "bloodrush" event is logged with Details["bloodrush_source"] =
//     the source Card pointer — the canonical stamp the user requested
//     as CostMeta["bloodrush_source"].
//
// Returns a CastError on any precondition failure. (CastError is the
// generic activation-error type — its name predates the
// activated-vs-cast split.)
func ActivateBloodrush(gs *GameState, seatIdx int, source *Card, target *Permanent) error {
	if gs == nil {
		return &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid seat"}
	}
	if source == nil {
		return &CastError{Reason: "nil source"}
	}
	if target == nil {
		return &CastError{Reason: "nil target"}
	}
	if !HasBloodrush(source) {
		return &CastError{Reason: "no_bloodrush"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return &CastError{Reason: "nil seat"}
	}
	// Source must be in the activating player's hand (CR §702.99a).
	inHand := false
	for _, c := range seat.Hand {
		if c == source {
			inHand = true
			break
		}
	}
	if !inHand {
		return &CastError{Reason: "source_not_in_hand"}
	}
	// Target must be an attacking creature (CR §702.99b — bloodrush
	// activation is gated on "a creature you control is attacking";
	// printed bloodrush spells additionally target "attacking creature").
	if !target.IsCreature() {
		return &CastError{Reason: "target_not_creature"}
	}
	if !target.IsAttacking() {
		return &CastError{Reason: "target_not_attacking"}
	}

	cost := BloodrushCost(source)
	if cost < 0 {
		return &CastError{Reason: "invalid_bloodrush_cost"}
	}
	if seat.ManaPool < cost {
		return &CastError{Reason: "insufficient_mana"}
	}

	// 1) Pay mana.
	seat.ManaPool -= cost
	SyncManaAfterSpend(seat)
	if cost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: cost,
			Source: source.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "bloodrush_activation",
				"keyword": "bloodrush",
				"rule":    "702.99a",
			},
		})
	}

	// 2) Discard the source card from hand (the activation cost).
	DiscardCard(gs, source, seatIdx)

	// 3) Apply pump.
	p, tgh, abilities := BloodrushPump(source)
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["temp_power"] += p
	target.Flags["temp_toughness"] += tgh
	target.Modifications = append(target.Modifications, Modification{
		Power:     p,
		Toughness: tgh,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})

	// 4) Grant ability words until end of turn.
	target.GrantedAbilities = append(target.GrantedAbilities, abilities...)
	gs.InvalidateCharacteristicsCache()

	// 5) Log with CostMeta-style stamp identifying the source card.
	gs.LogEvent(Event{
		Kind:   "bloodrush",
		Seat:   seatIdx,
		Source: source.DisplayName(),
		Details: map[string]interface{}{
			"bloodrush_source": source,
			"target":           target.Card.DisplayName(),
			"power_delta":      p,
			"toughness_delta":  tgh,
			"granted":          abilities,
			"rule":             "702.99a",
		},
	})
	return nil
}
