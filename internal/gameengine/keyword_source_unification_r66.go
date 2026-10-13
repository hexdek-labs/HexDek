package gameengine

import "strings"

// r66 — closing the keyword-source holes r65 left open.
//
// r65 folded the runtime keyword surfaces into the layer base and
// claimed, in this package's own header and in the issue log, that
// gs.HasKeywordOf was thereby "a true superset" of p.HasKeyword. THAT
// CLAIM WAS WRONG. Four positive sources still reached the layer-blind
// query and not the layer-aware one, each proven by probe before this
// file was written:
//
//	A  GrantedAbilities entries carrying the "level_bracket:" prefix.
//	   HasKeyword trims that prefix (combat.go) before comparing;
//	   BaseCharacteristics folded the raw string, so a level-up band
//	   keyword landed in chars.Keywords as "level_bracket:flying" and
//	   HasKeywordOf(p, "flying") answered false.
//	B  Keyword counters held in Permanent.CounterStacks. r65 folded
//	   only the legacy Permanent.Counters map; HasKeyword additionally
//	   consults HasKeywordCounter, which reads the CounterStacks store.
//	C  Divergent grants in that same store (a phyresis counter grants
//	   infect, CR §702.91) — same root as B, listed separately because
//	   it needs the registry rather than a name match.
//	D  HasKeyword's raw `p.Counters[want] > 0` check, which answers
//	   true for ANY counter kind queried by its own name — including
//	   HasKeyword("+1/+1"), verified true against a creature carrying
//	   +1/+1 counters.
//
// A, B and C are fixed here: they are real keywords the layer query was
// missing. D is NOT, and deliberately so — it is a defect in the
// layer-blind query, not a gap in the fold. "+1/+1" is not a keyword,
// and folding every counter kind to make the two agree would put it in
// chars.Keywords. The fold stays registry-resolved and stricter, and D
// is one of the things migrating a call site FIXES rather than
// preserves.
//
// The lesson is the one this cluster keeps teaching: r65's superset
// claim was written from reading the two functions, not from executing
// them. A probe found four counterexamples in minutes.

// normalizeGrantedKeyword maps a GrantedAbilities entry to the keyword
// name it actually confers, applying the same prefix handling
// Permanent.HasKeyword applies before comparing.
//
// Today that is the "level_bracket:" prefix (CR §711.2): level-up band
// keywords are stored prefixed so a band change can strip exactly its
// own grants without disturbing a same-named keyword the creature has
// from another source. The prefix is bookkeeping about WHERE the grant
// came from, not part of the keyword — the same distinction the
// "kw:<x>_from_<source>" provenance markers draw on the Flags surface.
//
// Keep this in step with HasKeyword: a prefix that query learns to trim
// must be trimmed here too, or the two disagree again.
func normalizeGrantedKeyword(g string) string {
	s := strings.TrimSpace(g)
	if s == "" {
		return ""
	}
	// Case-preserving trim: the fold's containsFold comparison is
	// case-insensitive, but the stored entry's own casing is kept so
	// chars.Keywords reads the way the grant was written.
	if i := len("level_bracket:"); len(s) > i &&
		strings.EqualFold(s[:i], "level_bracket:") {
		s = strings.TrimSpace(s[i:])
	}
	return s
}

// foldGrantedAbilities folds p.GrantedAbilities into the base
// characteristics through normalizeGrantedKeyword, skipping entries the
// base already carries. Both BaseCharacteristics paths (the §707.2
// face-down CDA override and the ordinary face-up path) route through
// this single helper so the two cannot drift apart — they previously
// held two hand-copied loops.
func foldGrantedAbilities(p *Permanent, c *Characteristics) {
	if p == nil || c == nil {
		return
	}
	for _, g := range p.GrantedAbilities {
		kw := normalizeGrantedKeyword(g)
		if kw == "" {
			continue
		}
		if !containsFold(c.Keywords, kw) {
			c.Keywords = append(c.Keywords, kw)
		}
	}
}
