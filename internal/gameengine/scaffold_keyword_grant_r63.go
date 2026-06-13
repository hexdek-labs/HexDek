package gameengine

import "strings"

// scaffold_keyword_grant_r63.go — generic coverage for the inert
// empty-ModKind static KEYWORD-GRANT scaffold kind (r63 keyword-ability
// sweep; see /tmp/fable-review/scaffold-claims.txt and
// docs report scaffold-kind-keywords-10).
//
// The card-AST parser routes static keyword grants of the shape
// "[all | (other) creatures you control] (have|has|gain) <kw>[, <kw>…]"
// (Levitation, Akroma's Memorial, Concordant Crossroads, Archetype of
// Imagination, Gaea's Anthem-style lords, etc.) to a `Static` whose
// `Modification.ModKind` is EMPTY — the keyword lives only in the raw
// text / first arg. registerASTStaticEffects' ModKind switch has no case
// for "", so these registered NOTHING: the grant was fully inert. ~2.6k
// corpus statics carry an empty ModKind, of which the unconditional
// keyword-grant shape is a large, evasion-relevant slice.
//
// This producer detects that shape from the raw text and registers a
// layer-6 continuous keyword grant (registerKeywordGrant) per keyword,
// scoped to the matching creatures. The grants then surface through
// gs.HasKeywordOf — and combat's evasion path honors them via the
// keywordActive() union (combat.go), so a creature a card says "has
// flying" can actually no longer be blocked by a non-flyer.
//
// SCOPE (conservative by design — the messy tail is left for follow-ups,
// see the report's plan): only UNCONDITIONAL grants of recognized
// keywords with a you-control / other-you-control / all scope. Loss
// ("lose flying"), can't-have, protection-from / hexproof-from,
// duration ("until end of turn" — those are GrantAbility, handled
// elsewhere), conditional ("as long as", "if"), and triggered
// ("whenever") shapes are REJECTED so this never mis-fires.

// grantableKeywordVocab is the set of keywords this producer will grant
// via a static. Restricted to evasion / combat / protection keywords
// whose grant is meaningful to the engine.
var grantableKeywordVocab = map[string]bool{
	"flying": true, "reach": true, "menace": true, "fear": true,
	"intimidate": true, "horsemanship": true, "skulk": true, "shadow": true,
	"trample": true, "deathtouch": true, "lifelink": true, "vigilance": true,
	"first strike": true, "double strike": true, "haste": true,
	"hexproof": true, "shroud": true, "indestructible": true, "defender": true,
	"flanking": true, "wither": true, "infect": true,
}

// keywordGrantRejectSubstrings veto a raw static from being treated as an
// unconditional keyword grant — these shapes need richer handling.
var keywordGrantRejectSubstrings = []string{
	"lose", "loses", "losing", "can't", "cannot", "until ", "as long as",
	"whenever", "when ", " if ", "protection from", " from ", "instead",
	"becomes", "where ", "rather than", "for each",
}

// detectKeywordGrantStatic recognizes an unconditional static keyword
// grant and returns its scope ("all" | "you_control" | "other_you_control")
// and the recognized keyword list. ok is false for any non-matching or
// rejected shape.
func detectKeywordGrantStatic(raw string) (scope string, kws []string, ok bool) {
	r := strings.ToLower(strings.TrimSpace(raw))
	if r == "" {
		return "", nil, false
	}
	for _, bad := range keywordGrantRejectSubstrings {
		if strings.Contains(r, bad) {
			return "", nil, false
		}
	}
	// Locate the grant verb (longest/most-specific first so " have " wins
	// before a stray " has " substring inside a word is considered).
	verbIdx, verb := -1, ""
	for _, v := range []string{" have ", " has ", " gains ", " gain "} {
		if i := strings.Index(r, v); i >= 0 {
			verbIdx, verb = i, v
			break
		}
	}
	if verbIdx < 0 {
		return "", nil, false
	}
	prefix := r[:verbIdx]
	tail := r[verbIdx+len(verb):]

	// The grant must be about creatures.
	if !strings.Contains(prefix, "creature") {
		return "", nil, false
	}
	switch {
	case strings.HasPrefix(prefix, "all creatures") || strings.HasPrefix(prefix, "each creature"):
		scope = "all"
	case strings.Contains(prefix, "you control"):
		if strings.HasPrefix(prefix, "other ") {
			scope = "other_you_control"
		} else {
			scope = "you_control"
		}
	default:
		// opponents / attacking / "the chosen player" etc. — follow-up.
		return "", nil, false
	}

	// Parse the keyword list from the tail up to the first clause boundary.
	if i := strings.IndexAny(tail, ".\n;"); i >= 0 {
		tail = tail[:i]
	}
	tail = strings.ReplaceAll(tail, " and ", ",")
	seen := map[string]bool{}
	for _, part := range strings.Split(tail, ",") {
		kw := strings.TrimSpace(part)
		if grantableKeywordVocab[kw] && !seen[kw] {
			seen[kw] = true
			kws = append(kws, kw)
		}
	}
	if len(kws) == 0 {
		return "", nil, false
	}
	return scope, kws, true
}

// registerKeywordGrantStatic registers a layer-6 continuous keyword grant
// for each detected keyword, scoped by the static's prefix. registerKeyword-
// Grant already gates its ApplyFn to creatures, matching "creatures … have".
func registerKeywordGrantStatic(gs *GameState, p *Permanent, scope string, kws []string) {
	if gs == nil || p == nil {
		return
	}
	src := p
	var pred func(*GameState, *Permanent) bool
	switch scope {
	case "all":
		pred = func(_ *GameState, _ *Permanent) bool { return true }
	case "you_control":
		pred = func(_ *GameState, t *Permanent) bool { return t != nil && t.Controller == src.Controller }
	case "other_you_control":
		pred = func(_ *GameState, t *Permanent) bool { return t != nil && t.Controller == src.Controller && t != src }
	default:
		return
	}
	for _, kw := range kws {
		registerKeywordGrant(gs, p, kw, "ast-kwgrant-"+scope+"-"+kw, pred)
	}
}

// maybeRegisterKeywordGrantStatic is the registerASTStaticEffects hook: if
// `raw` is an unconditional keyword-grant static, register the grants and
// return true (the caller then skips its empty-ModKind no-op path).
func maybeRegisterKeywordGrantStatic(gs *GameState, p *Permanent, raw string) bool {
	scope, kws, ok := detectKeywordGrantStatic(raw)
	if !ok {
		return false
	}
	registerKeywordGrantStatic(gs, p, scope, kws)
	return true
}
