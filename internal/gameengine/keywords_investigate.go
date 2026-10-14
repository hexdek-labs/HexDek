package gameengine

// keywords_investigate.go — Investigate keyword surface (CR §701.16
// in current Comprehensive Rules numbering; the engine's existing
// resolve_helpers.go cites it as §701.36, which was the Shadows over
// Innistrad-era number — the rule text is the same).
//
// Printed pattern: "Investigate." or "Investigate N." Whenever an
// effect tells a player to investigate, that player creates one (or
// N) Clue token(s). Clue is an artifact token with "{2}, Sacrifice
// this artifact: Draw a card."
//
// The engine has shipped CreateClueToken as the low-level mint since
// the token-tagging pass, and the AST-driven keyword-action dispatch
// in resolve_helpers.go handles the canonical "investigate" effect
// node. This file adds the missing PERMANENT-level surface so:
//
//   1. Per-card handlers and AI evaluators can ask
//      HasInvestigate(card) without re-implementing oracle scans.
//   2. A uniform FireInvestigateTriggers entry point lets the engine
//      (or per_card code) emit "investigate" card triggers so handlers
//      listening on the "investigate" event (Bygone Bishop, Lonis,
//      Cryptozoologist, Tireless Tracker, etc.) can layer their own
//      payoff (draw, +1/+1 counters, free spells).
//   3. ApplyInvestigateEffect collapses the "mint N clue tokens" loop
//      into a single helper so callers don't have to spin their own
//      for-loop around CreateClueToken.
//
// Wiring: this file is intentionally pure surface — no call sites are
// modified here. The AST-driven dispatch in resolve_helpers.go remains
// authoritative for executing the "investigate" keyword action. Per
// card handlers that want trigger-style behaviour should call
// FireInvestigateTriggers themselves at the point of the investigate
// event, the same way per_card/the_rani.go and bygone_bishop.go would.
//
// Public API:
//
//   HasInvestigate(card) → bool
//   FireInvestigateTriggers(gs, seatIdx, source, extraCtx) → void
//   ApplyInvestigateEffect(gs, seatIdx, n) → clues created (int)

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasInvestigate
// ---------------------------------------------------------------------------

// HasInvestigate returns true if the card carries an investigate
// keyword or its oracle text references investigate. Detection paths,
// in priority order:
//
//  1. Keyword AST node with name "investigate" — the cleanest signal.
//     Investigate is technically a keyword-ACTION (CR §701.16) rather
//     than a keyword-ABILITY, but corpora that pre-process triggered
//     abilities into keyword nodes (e.g. "Bygone Bishop's "Whenever
//     you cast a creature spell with mana value 3 or less, investigate")
//     sometimes mint a Keyword{Name:"investigate"} alongside the
//     Triggered ability — honor either form.
//  2. OracleTextLower contains the substring "investigate". This
//     catches both static text ("When CARDNAME enters the battlefield,
//     investigate.") and triggered abilities whose Raw still mentions
//     the word.
//
// Returns false for nil cards or cards with no AST. The match is
// substring-based and case-insensitive (via OracleTextLower's cached
// lowercased text), which correctly handles "investigate" and
// "investigate twice" and "investigate. Investigate." etc.
func HasInvestigate(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	// (1) Direct keyword node.
	for _, ab := range card.AST.Abilities {
		if kw, ok := ab.(*gameast.Keyword); ok && keywordNameEquals(kw, "investigate") {
			return true
		}
	}
	// (2) Oracle-text substring match.
	if strings.Contains(OracleTextLower(card), "investigate") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// FireInvestigateTriggers
// ---------------------------------------------------------------------------

// FireInvestigateTriggers fires the "investigate" card trigger for a
// single investigate event. CR §701.16 — investigating IS the action;
// triggers like "When you investigate, ..." (Lonis, Cryptozoologist;
// Tireless Tracker's clue-on-LTB doubler; Hard Evidence-adjacent
// effects) listen on this event.
//
// The caller is the code that PERFORMED the investigate (typically
// the per_card handler that emitted the investigate action, or the
// resolve_helpers.go keyword-action dispatcher). `source` is the
// permanent whose effect caused the investigate, used by per_card
// handlers to scope "if I investigated" vs "if some other permanent
// investigated."
//
// ctx keys forwarded to handlers (extraCtx is merged in last; callers
// may overwrite the defaults if needed):
//
//	"source": *Permanent  — the permanent that caused the investigate
//	"seat":   int          — seatIdx (the player who investigates)
//
// Side effects:
//   - emits "investigate_trigger" event so the EventLog records the
//     fan-out even when no listener is registered (auditable replay)
//   - calls FireCardTrigger("investigate", ctx) — listeners check
//     ctx["seat"] / ctx["source"] to decide whether to fire
//
// Nil-safe: nil gs, nil source, or out-of-range seatIdx → no-op.
func FireInvestigateTriggers(
	gs *GameState,
	seatIdx int,
	source *Permanent,
	extraCtx map[string]interface{},
) {
	if gs == nil {
		return
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if gs.Seats[seatIdx] == nil {
		return
	}

	ctx := map[string]interface{}{
		"source": source,
		"seat":   seatIdx,
	}
	for k, v := range extraCtx {
		ctx[k] = v
	}

	sourceName := ""
	if source != nil && source.Card != nil {
		sourceName = source.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "investigate_trigger",
		Seat:   seatIdx,
		Source: sourceName,
		Details: map[string]interface{}{
			"rule": "701.16",
		},
	})
	FireCardTrigger(gs, "investigate", ctx)
}

// ---------------------------------------------------------------------------
// ApplyInvestigateEffect
// ---------------------------------------------------------------------------

// ApplyInvestigateEffect mints `n` Clue tokens for `seatIdx`. CR
// §701.16a — "To investigate, create a colorless artifact token
// named Clue with '{2}, Sacrifice this artifact: Draw a card.'"
//
// Returns the number of clues actually created (always n on success;
// 0 on nil/invalid-seat inputs).
//
// The CreateClueToken path already handles ETB triggers, token-created
// triggers, and zone accounting per-mint; this helper is a thin loop
// so callers don't have to spin their own. For n <= 0 the function
// is a no-op and returns 0.
func ApplyInvestigateEffect(gs *GameState, seatIdx int, n int) int {
	if gs == nil {
		return 0
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	if gs.Seats[seatIdx] == nil {
		return 0
	}
	if n <= 0 {
		return 0
	}
	for i := 0; i < n; i++ {
		CreateClueToken(gs, seatIdx)
	}
	gs.LogEvent(Event{
		Kind:   "investigate",
		Seat:   seatIdx,
		Amount: n,
		Details: map[string]interface{}{
			"rule":  "701.16a",
			"clues": n,
		},
	})
	return n
}
