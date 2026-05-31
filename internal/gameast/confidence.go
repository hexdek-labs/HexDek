package gameast

// Confidence scoring for parsed AST nodes.
//
// Each node gets a confidence float in [0.0, 1.0] derived from the
// match-quality of its parse — high confidence means the parser
// produced a fully-structured semantic representation; low confidence
// means the parser identified the node shape (Static / Triggered /
// Activated / Condition) but fell back to a raw / typed-residual /
// untyped representation for the payload.
//
// Downstream consumers (engine dispatch, freya deck analysis, hat AI)
// can gate on the score:
//
//	if gameast.IsConfident(score, gameast.DefaultConfidenceThreshold) {
//	    // trust the parsed structure end-to-end
//	} else {
//	    // fall back to raw oracle text or a generic handler
//	}
//
// The scoring is computed on the fly — no JSON schema change, no
// dataset regeneration. The signals mirror the fallback definitions in
// scripts/ast_corpus_health.py and the era scaffold audits, so the
// confidence floor is consistent across the parser-health surface.
//
// Keyword abilities (Flying, Trample, etc.) are by-design raw stubs
// where the keyword name IS the semantic payload — they score 1.0.

import (
	"sort"
)

// DefaultConfidenceThreshold is the gate downstream consumers use by
// default to decide whether to trust a parsed node end-to-end. Tuned
// to match the 85% non-fallback share in the 2026-05-30 corpus health
// snapshot — a node at this score or above is in the structured
// majority; below it the parser fell back somewhere.
const DefaultConfidenceThreshold = 0.7

// FallbackModKinds — Modification.ModKind values that indicate the
// parser identified a static-ability body but fell back to a
// raw / typed-residual / untyped representation. Same set the era
// scaffold audits and scripts/ast_corpus_health.py use.
var FallbackModKinds = map[string]struct{}{
	"parsed_effect_residual": {},
	"parsed_tail":            {},
	"untyped_effect":         {},
	"if_intervening_tail":    {},
	"custom":                 {},
	"cast_trigger_tail":      {},
}

// FallbackEffectKinds — Effect.Kind() values for Triggered / Activated
// effects that indicate fallback. Note "conditional" is included here
// as a bare-wrapper sentinel: the parser saw "if X then Y" but didn't
// unbox Y. The set is consulted ONLY for Effects whose concrete type
// reports its kind directly; *ModificationEffect and *UnknownEffect
// are handled via dedicated paths in effectIsFallback below.
var FallbackEffectKinds = map[string]struct{}{
	"parsed_effect_residual": {},
	"untyped_effect":         {},
	"cast_trigger_tail":      {},
	"conditional":            {},
}

// effectIsFallback inspects an Effect's concrete type. Returns
// (isFallback, label):
//
//	*UnknownEffect              -> always fallback (carries raw text only)
//	*ModificationEffect         -> fallback if its ModKind is in
//	                                FallbackModKinds
//	other concrete Effect types -> fallback if Kind() is in
//	                                FallbackEffectKinds
//
// Centralised so the Triggered + Activated branches in AbilityConfidence
// stay consistent.
func effectIsFallback(e Effect) (bool, string) {
	switch v := e.(type) {
	case nil:
		return true, "<nil>"
	case *UnknownEffect:
		return true, "unknown_effect"
	case *ModificationEffect:
		if IsFallbackModKind(v.ModKind) {
			return true, v.ModKind
		}
		return false, ""
	default:
		if IsFallbackEffectKind(e.Kind()) {
			return true, e.Kind()
		}
		return false, ""
	}
}

// FallbackCondKinds — Condition.Kind values that indicate a raw /
// unbucketed condition. Same set as scripts/era*_scaffold_audit.py's
// RAW_KINDS.
var FallbackCondKinds = map[string]struct{}{
	"if":             {},
	"conditional":    {},
	"raw":            {},
	"intervening_if": {},
	"as_long_as":     {},
}

// IsFallbackModKind reports whether a Modification's kind is a fallback
// representation. Exported so downstream consumers can apply the same
// classification independently.
func IsFallbackModKind(kind string) bool {
	_, ok := FallbackModKinds[kind]
	return ok
}

// IsFallbackEffectKind reports whether an Effect's kind is a fallback
// representation.
func IsFallbackEffectKind(kind string) bool {
	_, ok := FallbackEffectKinds[kind]
	return ok
}

// IsFallbackCondKind reports whether a Condition's kind is raw /
// unbucketed.
func IsFallbackCondKind(kind string) bool {
	_, ok := FallbackCondKinds[kind]
	return ok
}

// AbilityConfidence returns a confidence score in [0.0, 1.0] for a
// single ability. Returns 1.0 for nil / unknown ability shapes so the
// gate fails open rather than closed on unexpected input.
//
// Penalty schedule:
//
//	-0.50  fallback Modification.ModKind (per occurrence)
//	-0.30  missing/empty Modification on Static
//	-0.50  fallback Effect kind on Triggered/Activated
//	-0.40  nil/empty Effect on Triggered/Activated
//	-0.30  fallback Condition (on Static.Condition or Triggered.InterveningIf)
//
// Penalties stack; the result is clamped to [0, 1]. A bare Keyword
// scores 1.0 unconditionally — those are intentional raw stubs.
func AbilityConfidence(a Ability) float64 {
	if a == nil {
		return 1.0
	}
	score := 1.0
	switch v := a.(type) {
	case *Keyword:
		// Keywords are by design raw stubs; the keyword name is the
		// semantic payload. Full confidence.
		return 1.0

	case *Static:
		if v.Modification == nil {
			score -= 0.30
		} else if IsFallbackModKind(v.Modification.ModKind) {
			score -= 0.50
		}
		if v.Condition != nil && IsFallbackCondKind(v.Condition.Kind) {
			score -= 0.30
		}

	case *Triggered:
		if v.Effect == nil {
			score -= 0.40
		} else if fb, _ := effectIsFallback(v.Effect); fb {
			score -= 0.50
		}
		if v.InterveningIf != nil && IsFallbackCondKind(v.InterveningIf.Kind) {
			score -= 0.30
		}

	case *Activated:
		if v.Effect == nil {
			score -= 0.40
		} else if fb, _ := effectIsFallback(v.Effect); fb {
			score -= 0.50
		}
	}
	return clamp01(score)
}

// IsConfident returns true if score is at or above the threshold.
// Centralised so the inequality direction is consistent across the
// codebase ("at or above" — strict above would make 0.7-threshold-on-
// 0.7-score fail open).
func IsConfident(score, threshold float64) bool {
	return score >= threshold
}

// AbilityIsConfident is sugar for IsConfident(AbilityConfidence(a), threshold).
func AbilityIsConfident(a Ability, threshold float64) bool {
	return IsConfident(AbilityConfidence(a), threshold)
}

// LowConfidenceReasons returns short human-readable reason strings for
// the penalties currently applied to a. Empty slice = full confidence.
// Useful for diagnostics / logging in downstream consumers and for the
// corpus-health regression test below.
func LowConfidenceReasons(a Ability) []string {
	if a == nil {
		return nil
	}
	var reasons []string
	switch v := a.(type) {
	case *Keyword:
		return nil
	case *Static:
		if v.Modification == nil {
			reasons = append(reasons, "static_no_modification")
		} else if IsFallbackModKind(v.Modification.ModKind) {
			reasons = append(reasons, "static_fallback_mod_kind:"+v.Modification.ModKind)
		}
		if v.Condition != nil && IsFallbackCondKind(v.Condition.Kind) {
			reasons = append(reasons, "static_fallback_cond_kind:"+v.Condition.Kind)
		}
	case *Triggered:
		if v.Effect == nil {
			reasons = append(reasons, "triggered_no_effect")
		} else if fb, label := effectIsFallback(v.Effect); fb {
			reasons = append(reasons, "triggered_fallback_effect_kind:"+label)
		}
		if v.InterveningIf != nil && IsFallbackCondKind(v.InterveningIf.Kind) {
			reasons = append(reasons, "triggered_fallback_intervening_if:"+v.InterveningIf.Kind)
		}
	case *Activated:
		if v.Effect == nil {
			reasons = append(reasons, "activated_no_effect")
		} else if fb, label := effectIsFallback(v.Effect); fb {
			reasons = append(reasons, "activated_fallback_effect_kind:"+label)
		}
	}
	sort.Strings(reasons)
	return reasons
}

// CardConfidence returns a card-level confidence score. By default this
// is the mean across abilities — a single low-confidence ability
// shouldn't drag the whole card to 0, but a card with most abilities
// in fallback should land below the threshold.
//
// Cards with no abilities score 1.0 (vacuously confident — e.g. plain
// vanilla creatures). ParseErrors flag also matters: each parse error
// imposes a -0.4 penalty (clamped). A card with FullyParsed=false but
// no per-error rows still gets -0.4 from the !FullyParsed signal.
func CardConfidence(c *CardAST) float64 {
	if c == nil {
		return 1.0
	}
	score := 1.0
	if !c.FullyParsed {
		score -= 0.40
	}
	// Each parse error after the first contributes another -0.20.
	if extra := len(c.ParseErrors) - 1; extra > 0 {
		score -= 0.20 * float64(extra)
	} else if len(c.ParseErrors) > 0 {
		// First parse error contributes the initial -0.40 (subsumed by
		// the !FullyParsed branch if both fire, which is typical).
		if c.FullyParsed {
			// Edge: a parse error but FullyParsed=true. Still penalise.
			score -= 0.40
		}
	}
	if len(c.Abilities) == 0 {
		return clamp01(score)
	}
	sum := 0.0
	for _, ab := range c.Abilities {
		sum += AbilityConfidence(ab)
	}
	abilityMean := sum / float64(len(c.Abilities))
	// Combine card-level penalty with ability-level mean: multiply, so
	// a parse error on a card whose abilities all parse cleanly still
	// drags the score down proportionally.
	return clamp01(score * abilityMean)
}

// CardMinConfidence returns the lowest per-ability confidence on the
// card — the "weakest link" view. Useful when a downstream consumer
// needs to know if ANY ability is uncertain.
func CardMinConfidence(c *CardAST) float64 {
	if c == nil || len(c.Abilities) == 0 {
		return 1.0
	}
	mn := 1.0
	for _, ab := range c.Abilities {
		s := AbilityConfidence(ab)
		if s < mn {
			mn = s
		}
	}
	return mn
}

// CardIsConfident is sugar for IsConfident(CardConfidence(c), threshold).
func CardIsConfident(c *CardAST, threshold float64) bool {
	return IsConfident(CardConfidence(c), threshold)
}

// ConfidenceBucket classifies a score into one of four labels, useful
// for histograms and trace logs:
//
//	"clean"   score == 1.0
//	"high"    0.7 <= score < 1.0
//	"medium"  0.4 <= score < 0.7
//	"low"     score < 0.4
//
// The exact thresholds align with the parse-confidence histogram in
// docs/ast-corpus-health-r60.md.
func ConfidenceBucket(score float64) string {
	switch {
	case score >= 1.0:
		return "clean"
	case score >= DefaultConfidenceThreshold:
		return "high"
	case score >= 0.4:
		return "medium"
	default:
		return "low"
	}
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
