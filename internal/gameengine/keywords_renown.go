package gameengine

// keywords_renown.go — Renown (CR §702.112, Magic Origins 2015).
//
// CR §702.112a: Renown is a triggered ability. "Renown N" means
//               "Whenever this creature deals combat damage to a
//               player, if it isn't renowned, put N +1/+1 counters
//               on it and it becomes renowned."
// CR §702.112b: "Renowned" is a state designation that, once granted
//               to a creature, persists for as long as that creature
//               remains on the battlefield. A creature that leaves
//               and re-enters loses the designation.
//
// Engine model
// ------------
// Renown is tracked on perm.Flags["renowned"] (0 = not renowned,
// 1 = renowned). ApplyRenownOnCombatDamage is the combat-damage hook
// wired into applyCombatDamageToPlayer; it runs after damage has
// been applied to a player and is a no-op when:
//
//   - the source doesn't have Renown,
//   - the source is already renowned,
//   - the source dealt zero damage to the player (e.g. blocked-to-zero
//     attackers — handler is invoked only with amount > 0, so this
//     guard is defense-in-depth for direct callers).
//
// On a fresh renown trigger the helper stamps perm.Flags["renowned"]
// = 1, adds N +1/+1 counters, invalidates the characteristics cache
// so the new P/T propagates, fires the per-card trigger so card
// handlers that key off "becomes renowned" (Akroan Hoplite-adjacent)
// receive the event, and logs a renown event for observability.
//
// Side-effects on damage-to-creature paths are intentionally NOT
// triggered here — CR §702.112a's printed wording restricts the
// event to "deals combat damage to a player." The damage-to-creature
// path in combat.go does not call ApplyRenownOnCombatDamage; only
// the player path does.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasRenown / RenownValue
// ---------------------------------------------------------------------------

// HasRenown reports whether the card has the renown keyword in its
// AST.
func HasRenown(card *Card) bool {
	return cardHasKeywordByName(card, "renown")
}

// RenownValue returns the N in "Renown N" — the number of +1/+1
// counters the trigger places. Bare "renown" with no argument defaults
// to 1 (matches printed cards like Eagle of the Watch and the
// Magic Origins reminder text). Returns 0 when the keyword is absent.
func RenownValue(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "renown") {
			continue
		}
		if len(kw.Args) == 0 {
			return 1
		}
		switch v := kw.Args[0].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// IsRenowned
// ---------------------------------------------------------------------------

// IsRenowned reports whether the permanent currently carries the
// renowned designation (perm.Flags["renowned"] > 0). Mirrors the
// flag CheckRenown writes in keywords_batch.go so callers that pre-
// date this file's helpers see consistent state.
func IsRenowned(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["renowned"] > 0
}

// ---------------------------------------------------------------------------
// ApplyRenownOnCombatDamage — the combat-damage hook
// ---------------------------------------------------------------------------

// ApplyRenownOnCombatDamage is the combat-damage-to-player hook for
// Renown. It is the no-op-when-not-applicable guard that lives at
// the head of every renown-relevant code path; combat.go calls it
// unconditionally for every creature that deals combat damage to a
// player. CR §702.112a.
//
// Returns true iff a renown trigger actually fired (the source was
// previously not renowned and is renowned now). False on every other
// path — no keyword, already renowned, nil inputs.
//
// `defenderSeat` is passed through to the per-card trigger context
// so handlers that care about which opponent was struck can read it
// directly without re-deriving from the source. It is NOT used to
// gate the trigger (the printed rule is symmetric across opponents).
func ApplyRenownOnCombatDamage(gs *GameState, perm *Permanent, defenderSeat int) bool {
	if gs == nil || perm == nil {
		return false
	}
	if !HasRenownPerm(perm) {
		return false
	}
	if IsRenowned(perm) {
		return false
	}
	n := renownValueForPerm(perm)
	if n <= 0 {
		return false
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["renowned"] = 1
	perm.AddCounter("+1/+1", n)
	gs.InvalidateCharacteristicsCache()

	cardName := ""
	if perm.Card != nil {
		cardName = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "renown",
		Seat:   perm.Controller,
		Source: cardName,
		Amount: n,
		Details: map[string]interface{}{
			"rule":          "702.112",
			"defender_seat": defenderSeat,
			"counters":      n,
		},
	})
	// Per-card "becomes renowned" trigger so card handlers that key
	// off the renown event (e.g. Anointer of Champions, Topan
	// Freeblade-adjacent) can subscribe.
	FireCardTrigger(gs, "becomes_renowned", map[string]interface{}{
		"source":          perm,
		"controller_seat": perm.Controller,
		"defender_seat":   defenderSeat,
		"counters_added":  n,
	})
	return true
}

// HasRenownPerm reports whether a permanent has renown via either
// its printed keyword OR a granted ability (perm.HasKeyword consults
// perm.GrantedAbilities). Mirrors HasInspiredPerm so granted-renown
// edge cases (e.g. a global "creatures you control have renown 1"
// static effect, when one ships) light up here automatically.
func HasRenownPerm(perm *Permanent) bool {
	if perm == nil {
		return false
	}
	if perm.HasKeyword("renown") {
		return true
	}
	return HasRenown(perm.Card)
}

// renownValueForPerm returns the renown N for a permanent, preferring
// the printed card's keyword arg and falling back to 1 when only a
// granted "renown" string is present without a numeric arg.
func renownValueForPerm(perm *Permanent) int {
	if perm == nil {
		return 0
	}
	if perm.Card != nil {
		if n := RenownValue(perm.Card); n > 0 {
			return n
		}
	}
	// Granted-renown without a numeric arg — assume Renown 1.
	if perm.HasKeyword("renown") {
		return 1
	}
	return 0
}
