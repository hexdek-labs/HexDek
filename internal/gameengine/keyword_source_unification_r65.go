package gameengine

import (
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine/counters"
)

// r65 — keyword-source unification (foundation).
//
// THE SPLIT. Until this file, "does this permanent have keyword X?" had
// two answers that disagreed, and which one you got depended on which
// query you happened to call:
//
//   - Permanent.HasKeyword (combat.go) reads FOUR runtime surfaces: AST
//     keywords, GrantedAbilities, Flags["kw:<name>"] and keyword
//     counters. It is layer-BLIND — a keyword granted by a §613
//     continuous effect is invisible to it, and a keyword STRIPPED by a
//     layer-6 effect (Humility) is still reported.
//   - GameState.HasKeywordOf (layers.go) reads the layer-computed
//     chars.Keywords. It is layer-correct, but BaseCharacteristics never
//     folded in the Flags or counter surfaces, so it MISSED keywords
//     that genuinely exist.
//
// Neither was a superset of the other, so neither could safely replace
// the other, and the engine used the layer-blind one at 81 call sites
// against 6 for the layer-aware one.
//
// This was already diagnosed in-repo — see the note at invariants.go
// on the defender check, which keeps the raw p.HasKeyword precisely
// because "Flags[...] and keyword counters are real positive sources
// that BaseCharacteristics does NOT fold into chars.Keywords, so
// gs.HasKeywordOf would MISS them", and names unifying the keyword
// sources into the layer base as the prerequisite. This is that
// prerequisite.
//
// The consequence of the split is not academic. Because the layer-aware
// grant was invisible to combat, every per_card handler that wanted a
// keyword to actually DO something had to stamp Flags["kw:<name>"] as a
// "fast path" (the comment on Avacyn, Angel of Hope says exactly this,
// and later handlers copied it). A Flags stamp has no duration and no
// owner, so nothing ever took it away — which is the r64b "until end of
// turn grants were permanent" cluster and the static anthems that
// outlive their own source.
//
// WHAT THIS CHANGES. Only BaseCharacteristics, and only additively: the
// runtime keyword surfaces are folded into the layer BASE, so they now
// compose under the layers instead of sitting beside them. That makes
// gs.HasKeywordOf a superset of p.HasKeyword, which is what lets
// call sites migrate to the layer-correct query. (r65 claimed that was
// already TRUE as shipped; it was not — four sources still escaped it,
// three of them closed in keyword_source_unification_r66.go, which
// documents the fourth as a defect in HasKeyword.) Folding at the base
// (rather than after the layers) is deliberate: a layer-6 "loses all
// abilities" effect must strip a flag-granted keyword too, which is the
// Humility phantom the invariants note describes.
//
// NOT changed here: the 81 p.HasKeyword call sites. Migrating them is a
// behaviour change per call site and is deliberately a separate step.

// runtimeKeywordSources returns the keywords conferred on p by the
// runtime surfaces that live outside the AST and outside
// GrantedAbilities: the Flags["kw:<name>"] stamp and keyword counters
// (CR §122.1b) in BOTH counter stores.
//
// The result is SORTED. BaseCharacteristics feeds chars.Keywords, and
// the LayerIdempotency invariant compares two independently computed
// keyword slices for equality — an unsorted Go map range would make the
// order differ between passes and fail that check spuriously.
func runtimeKeywordSources(p *Permanent) []string {
	if p == nil {
		return nil
	}
	var out []string

	// Surface 1 — Flags["kw:<name>"].
	//
	// Presence, not value: Permanent.HasKeyword tests `_, ok :=
	// p.Flags["kw:"+want]` and removal is by delete(), so a key present
	// with value 0 still grants. Mirrored exactly to keep this a strict
	// superset of the flag surface.
	//
	// The suffix is folded VERBATIM rather than normalised. HasKeyword
	// looks up the exact key "kw:"+want, so a stamp written as
	// "kw:first_strike" and one written as "kw:first strike" are already
	// two different keys to it; rewriting underscores here would make
	// this query match keywords the flag surface does not, which is the
	// opposite of a faithful fold.
	//
	// CORRECTED r89 — this comment used to end by claiming the verbatim
	// fold "keeps provenance tags such as kw:menace_from_gisa from being
	// reinterpreted as a keyword". That is FALSE, and measured false:
	// the verbatim fold is exactly what puts "menace_from_gisa" into
	// chars.Keywords as a keyword of that name. What it avoids is the
	// tag being normalised into something that collides with the REAL
	// keyword, which is a smaller and different guarantee. No keyword
	// query answers wrongly as a result — "menace" stays false both raw
	// and layer-aware — so the harm is a fabricated entry in the layer's
	// keyword set, and a comment that would tell the next person a
	// provenance tag is safe inside the kw: namespace. It is not; that
	// is why r87's markers live under "kwsrc:". Pinned by
	// TestKeywordFoldR89_ProvenanceTagInKwNamespaceBecomesAKeyword.
	if len(p.Flags) > 0 {
		for k := range p.Flags {
			if !strings.HasPrefix(k, "kw:") {
				continue
			}
			if kw := strings.TrimSpace(k[len("kw:"):]); kw != "" {
				out = append(out, kw)
			}
		}
	}

	// Surface 2 — keyword counters (CR §122.1b). A counter grants its
	// keyword regardless of the permanent's card type (§122.6), so this
	// is folded unconditionally.
	//
	// Only counter types the registry categorises as KeywordGrant are
	// folded, resolved through the registry rather than by name. That is
	// deliberately STRICTER than HasKeyword's first counter check, which
	// tests `p.Counters[want] > 0` against the raw map and would answer
	// true for HasKeyword("+1/+1"); folding every counter kind as a
	// keyword would put "+1/+1" into chars.Keywords.
	if len(p.Counters) > 0 {
		for kind, n := range p.Counters {
			if n <= 0 {
				continue
			}
			def := counters.Lookup(kind)
			if def == nil || def.Category != counters.KeywordGrant {
				continue
			}
			// A divergent grant confers a keyword under a different name
			// than the counter's own (a phyresis counter grants infect,
			// CR §702.91); otherwise the canonical counter name IS the
			// keyword.
			if def.GrantedAbility != nil && def.GrantedAbility.Keyword != "" {
				out = append(out, def.GrantedAbility.Keyword)
			} else if def.Name != "" {
				out = append(out, def.Name)
			}
		}
	}

	// Surface 3 (r66) — keyword counters held in the CounterStacks
	// store rather than the legacy Counters map. These are two
	// SEPARATE stores: HasKeyword consults the legacy map directly and
	// then calls HasKeywordCounter, which reads CounterStacks. r65
	// folded only the first, so a keyword counter written through the
	// Counter DB path reached the layer-blind query and not this one.
	//
	// Resolved through the same registry lookup as surface 2, so a
	// divergent grant (phyresis -> infect) folds as the keyword it
	// confers and a non-KeywordGrant stack folds as nothing.
	for _, st := range p.CounterStacks {
		if st.Count <= 0 {
			continue
		}
		def := counters.Lookup(st.Type)
		if def == nil || def.Category != counters.KeywordGrant {
			continue
		}
		if def.GrantedAbility != nil && def.GrantedAbility.Keyword != "" {
			out = append(out, def.GrantedAbility.Keyword)
		} else if def.Name != "" {
			out = append(out, def.Name)
		}
	}

	if len(out) > 1 {
		sort.Strings(out)
	}
	return out
}

// foldRuntimeKeywordSources appends the runtime keyword surfaces to the
// base characteristics, skipping any the base already carries.
// Case-insensitive, matching the GrantedAbilities fold beside it.
func foldRuntimeKeywordSources(p *Permanent, c *Characteristics) {
	if p == nil || c == nil {
		return
	}
	for _, kw := range runtimeKeywordSources(p) {
		if !containsFold(c.Keywords, kw) {
			c.Keywords = append(c.Keywords, kw)
		}
	}
}
