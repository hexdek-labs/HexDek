package gameengine

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// Confidence-aware AST routing.
//
// The engine previously routed any AST node it saw, regardless of how
// well the parser understood it. With per-node confidence scores
// available from internal/gameast/confidence.go, low-confidence
// abilities can be intercepted before their dispatch switch — emitting
// a structured `low_confidence_fallback` event instead of executing a
// potentially-wrong scaffold path.
//
// The threshold here is deliberately LOWER than the default 0.7 gate
// used by freya / hat / corpus-health tooling. A 0.5 floor means:
//
//   - Score 1.0 (fully structured) — routes normally.
//   - Score 0.7 (single -0.3 condition penalty)  — routes normally.
//   - Score 0.5 (single -0.5 modkind/effect-kind penalty) — routes
//     normally (AT the boundary; at-or-above gate).
//   - Score 0.2 (stacked -0.5 + -0.3 penalties)  — INTERCEPTED. The
//     engine logs the gap and degrades gracefully rather than feeding
//     a noisy AST into a dispatch table that wasn't built for it.
//
// The asymmetric thresholds are intentional: a downstream consumer
// surveying the deck (freya / hat) should be conservative (0.7), but
// the engine is the last line of defense — gating at 0.5 catches the
// genuinely-broken parses while leaving "imperfect but workable"
// (single-penalty) abilities on the structured path.

// LowConfidenceFallbackThreshold is the score at-or-above which the
// engine WILL route the ability through its normal dispatch. Below
// this, the gate intercepts and emits a structured skip event.
//
// Tuned to 0.5 per the r60 engine-routing design: single-penalty
// abilities (e.g. a Static with a fallback-modkind body but no
// condition penalty, scoring 0.5) still route — the parser saw a
// recognisable shape and the dispatch's `default:` branches handle
// long-tail kinds gracefully. Two-or-more stacked penalties (scoring
// 0.2 or 0.0) signal the parser was confused enough that the
// dispatch is more likely to mis-route than help.
const LowConfidenceFallbackThreshold = 0.5

// IsLowConfidenceAbility returns true if a's confidence score is
// strictly below the engine's fallback threshold. Centralised so the
// inequality direction stays consistent across all gating call sites.
func IsLowConfidenceAbility(a gameast.Ability) bool {
	if a == nil {
		return false // nil fails open — gameast.AbilityConfidence(nil) == 1.0
	}
	return gameast.AbilityConfidence(a) < LowConfidenceFallbackThreshold
}

// IsLowConfidenceModificationEffect returns true if a Triggered /
// Activated *ModificationEffect's confidence (computed by treating it
// as if it were the standalone payload of a synthetic Triggered) is
// below the engine fallback threshold. Useful at chokepoints like
// resolveModificationEffect where the caller has an Effect, not an
// Ability.
func IsLowConfidenceModificationEffect(e *gameast.ModificationEffect) bool {
	if e == nil {
		return false
	}
	// A bare ModificationEffect with a fallback ModKind scores 0.5 on
	// the gameast confidence scale (synthetic Triggered, -0.5 effect
	// penalty). Strictly less-than 0.5 requires stacked signals — but
	// the synthetic wrapping doesn't carry condition info, so the
	// score floors at 0.5 for any single fallback. Treat "ModKind in
	// FallbackModKinds AND ArgsLooksLikeRaw" as the engine-level low-
	// confidence trigger — the structured kind alone is too coarse.
	if !gameast.IsFallbackModKind(e.ModKind) {
		return false
	}
	// If args is empty / contains only a single raw string, the parser
	// gave us essentially no semantic payload. That's the engine-level
	// "skip and log" case.
	if len(e.Args) == 0 {
		return true
	}
	if len(e.Args) == 1 {
		if _, isStr := e.Args[0].(string); isStr {
			return true
		}
	}
	return false
}

// LogLowConfidenceFallback emits a structured event recording that the
// engine declined to route an ability because its score was below the
// threshold. The event is sibling to the existing `parser_gap` shape
// so Heimdall post-game analytics can extract these cleanly.
//
// Caller-supplied `where` identifies the call site (e.g.
// "resolveModificationEffect", "etb_dispatch") so the log can be
// filtered by interception point.
func LogLowConfidenceFallback(gs *GameState, src *Permanent, a gameast.Ability, where string) {
	score := gameast.AbilityConfidence(a)
	reasons := gameast.LowConfidenceReasons(a)
	details := map[string]interface{}{
		"score":   score,
		"reasons": reasons,
		"where":   where,
	}
	if a != nil {
		details["ability_kind"] = a.Kind()
	}
	gs.LogEvent(Event{
		Kind:    "low_confidence_fallback",
		Seat:    controllerSeat(src),
		Source:  sourceName(src),
		Details: details,
	})
	// Also bump the parser-gap counter on the permanent so existing
	// Heimdall extractors that scan for parser_gap pick this up.
	if src != nil {
		if src.Flags == nil {
			src.Flags = map[string]int{}
		}
		src.Flags["parser_gap"]++
		src.Flags["low_confidence_fallback"]++
	}
}

// LogLowConfidenceModificationEffect emits the same structured event
// for ModificationEffect-level interceptions where the caller has an
// Effect (not an Ability) in hand.
func LogLowConfidenceModificationEffect(gs *GameState, src *Permanent, e *gameast.ModificationEffect, where string) {
	details := map[string]interface{}{
		"mod_kind": "",
		"where":    where,
		"reason":   "low_confidence_modification_effect",
	}
	if e != nil {
		details["mod_kind"] = e.ModKind
		details["args"] = e.Args
	}
	gs.LogEvent(Event{
		Kind:    "low_confidence_fallback",
		Seat:    controllerSeat(src),
		Source:  sourceName(src),
		Details: details,
	})
	if src != nil {
		if src.Flags == nil {
			src.Flags = map[string]int{}
		}
		src.Flags["parser_gap"]++
		src.Flags["low_confidence_fallback"]++
	}
}

// RouteAbilityWithConfidence is the canonical gating helper: it calls
// `structured(a)` if a's confidence is at-or-above the threshold;
// otherwise it logs a fallback event (via LogLowConfidenceFallback at
// `where`) and returns false. Returns true if structured was called.
//
// Designed for the "call structured handler if we trust the parse,
// else degrade gracefully" pattern. Callers can pass a no-op closure
// as `structured` and use the return value as a routing signal.
func RouteAbilityWithConfidence(
	gs *GameState,
	src *Permanent,
	a gameast.Ability,
	where string,
	structured func(gameast.Ability),
) bool {
	if IsLowConfidenceAbility(a) {
		LogLowConfidenceFallback(gs, src, a, where)
		return false
	}
	if structured != nil {
		structured(a)
	}
	return true
}
