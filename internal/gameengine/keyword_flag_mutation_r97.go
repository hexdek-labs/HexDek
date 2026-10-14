package gameengine

import "strings"

// Plain mutators for the Flags["kw:<name>"] keyword surface (r97).
//
// WHY THESE EXIST
//
// Flags["kw:<name>"] is one of the surfaces runtimeKeywordSources folds
// into the layer BASE (keyword_source_unification_r65.go), so a stamp or
// a delete changes what gs.HasKeywordOf answers. Computed
// characteristics are cached per GameState behind charCacheEpoch, and
// nothing about writing to the Flags map bumps that epoch — so a handler
// that mutates the map by hand leaves every layer-aware reader holding
// the pre-mutation answer until some unrelated call happens to
// invalidate.
//
// That window is not theoretical. Measured on the r97 probe, with no
// intervening call:
//
//	grant  — Skithiryx pays {B} for haste:          raw=true  layer=false
//	expire — its end-step delayed trigger strips it: raw=false layer=true
//	mint   — a keyword stamped on a token after
//	         CreateCreatureToken (whose ETB and
//	         token_created triggers populate the cache
//	         first):                                 raw=true  layer=false
//
// Since r66 collapsed keywordActive's union down to the layer query
// alone (see combat.go), the LAYER answer is the one combat, evasion and
// the §508.1a legality validator consume. The raw flag being right is
// not enough.
//
// RELATIONSHIP TO THE r87 REFCOUNTED PAIR
//
// GrantKeywordFlag / RevokeKeywordFlag (keyword_flag_grants_r87.go) are
// the right tool when SEVERAL sources can grant the same keyword to the
// same permanent and each must be able to withdraw only its own grant —
// they keep a per-source marker so one source's revoke cannot take away
// another's grant. They deliberately do NOT invalidate; they return a
// "changed" bool and their doc puts the obligation on the caller. All 58
// call sites honour it (audited r97).
//
// These two are for the single-source case, which is the majority: one
// effect grants a keyword and the same effect (or its end-of-turn
// cleanup) takes it away. They own the invalidation so the caller cannot
// forget it. Prefer the r87 pair whenever a second grantor is possible.
//
// A NOTE ON EXPIRY TIMING, NOT FIXED HERE
//
// Many callers expire an "until end of turn" keyword from a
// next_end_step delayed trigger. That fires at the BEGINNING of the end
// step, whereas CR §514.2 ends such effects during the CLEANUP step —
// so the keyword is already gone while players still hold priority in
// the end step. ExpireKeywordGrantsEOT's opt-in marker
// (Flags["kw_eot:<name>"]) has the correct timing and only 5 adopters.
// These mutators do not change any caller's timing; they only make
// whichever timing a caller chose visible to the layer query.

// SetKeywordFlags stamps one or more keyword flags onto p and
// invalidates the characteristics cache so layer-aware readers see the
// grant immediately.
//
// Keywords are stored VERBATIM. Permanent.HasKeyword looks up the exact
// key "kw:"+want, so "first_strike" and "first strike" are two different
// keywords to it; pass whatever the reader on the other side expects,
// exactly as the hand-written stamp did.
//
// gs may be nil (construction paths that build a Permanent before it
// belongs to a game): the flags are still written, there is simply no
// cache to invalidate yet.
func SetKeywordFlags(gs *GameState, p *Permanent, keywords ...string) {
	if p == nil || len(keywords) == 0 {
		return
	}
	if p.Flags == nil {
		p.Flags = map[string]int{}
	}
	changed := false
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		key := "kw:" + kw
		if _, had := p.Flags[key]; !had {
			changed = true
		}
		p.Flags[key] = 1
	}
	if changed && gs != nil {
		gs.InvalidateCharacteristicsCache()
	}
}

// ClearKeywordFlags deletes one or more keyword flags from p and
// invalidates the characteristics cache.
//
// This is the arm that is easiest to get wrong, because an "until end of
// turn" grant is almost always withdrawn from a delayed-trigger
// EffectFn — and per the invalidation contract those closures must
// invalidate THEMSELVES. The runners (FireDelayedTriggers in phases.go,
// FireEventDelayedTriggers in state.go) do not do it for them, and the
// handler that registered the closure invalidated at registration time,
// which is a different turn's worth of reads ago.
//
// Presence, not value, is what grants (HasKeyword tests for the key's
// presence), so this deletes the key rather than zeroing it.
func ClearKeywordFlags(gs *GameState, p *Permanent, keywords ...string) {
	if p == nil || len(p.Flags) == 0 || len(keywords) == 0 {
		return
	}
	changed := false
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		key := "kw:" + kw
		if _, had := p.Flags[key]; had {
			delete(p.Flags, key)
			changed = true
		}
	}
	if changed && gs != nil {
		gs.InvalidateCharacteristicsCache()
	}
}
