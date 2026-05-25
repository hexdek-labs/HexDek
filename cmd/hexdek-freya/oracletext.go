package main

import (
	"regexp"
	"strings"
	"sync"
)

// oracletext — tiny token-/regex-based helpers for cleaning Scryfall
// oracle text before substring matching.
//
// Freya's analyzers run ~250 substring checks against raw oracle text.
// Two whole classes of false positives come from that:
//
//   - Reminder text in parentheses. Cascade's reminder reads
//     "(... exile cards from the top of your library until you exile a
//     nonland card that costs less.)" — that substring matches the
//     EmptiesLibrary / Doomsday classifier and tags every cascade card
//     as a cEDH finisher. Flashback / encore / embalm / eternalize /
//     aftermath all carry "exile this card from your graveyard" in their
//     reminder, which matches the graveyard_curate heuristic and tags
//     hundreds of cards as graveyard-engines.
//
//   - Modal cards. "Choose one — • A • B" is a mutex; the substring
//     matcher attributes both A and B simultaneously. Knowing the modes
//     are separable lets downstream callers (today: the docstring;
//     tomorrow: a mode-aware classifier) opt into per-mode dispatch.
//
// These helpers are intentionally minimal — no full NLP, no AST, no
// per-card hand-tuning. Pure string operations so they can be unit-tested
// without a Scryfall corpus.

var (
	// modeHeaderRE matches the "Choose one — " (or two/three/etc.) header
	// that introduces a modal spell's bullet list. Both the em-dash (—)
	// and the ASCII double-hyphen (--) printings are supported, since
	// Scryfall stores the modern em-dash but a few older bulk dumps keep
	// the legacy form.
	modeHeaderRE = regexp.MustCompile(
		`(?i)choose (one|two|three|four|one or more|any number)( of the following)?\s*[—\-]+\s*`)

	// modeBulletSep splits the bullets. Scryfall's bullet character is
	// U+2022 (•); some older text uses an ASCII bullet equivalent.
	modeBulletSep = regexp.MustCompile(`\s*[•●]\s*`)
)

// stripReminder removes parenthesized reminder text from oracle text,
// returning the unchanged string when no parentheses are present.
//
// Mana symbols use braces ({W}, {2}, {T}) and are unaffected. Reminder
// text in Magic oracle text is always parenthesized and never nests in
// real printings — but the implementation handles arbitrary nesting
// defensively (depth counter, mismatched-paren tolerant) so a future
// printing with nested parens or a Scryfall data quirk can't crash the
// classifier.
//
// Empty parens, mismatched closing parens, and trailing unclosed parens
// are tolerated: extra ')' characters are dropped, and an unclosed '('
// is treated as if the rest of the string were reminder text. This
// matches how a reader would skim the card.
func stripReminder(ot string) string {
	// No early return on missing '(' — we still want stray ')' characters
	// and whitespace collapsed on the way out.
	var b strings.Builder
	b.Grow(len(ot))
	depth := 0
	for _, r := range ot {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
			// Stray ')' (no matching open) is silently dropped.
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	// Collapse any double spaces / leading-trailing whitespace left
	// behind by removed parentheticals so downstream substring checks
	// like ", and you" still match.
	return collapseSpaces(b.String())
}

// collapseSpaces normalizes runs of whitespace to single spaces and
// trims edges. Run on stripReminder output to undo the gaps left by
// removed parentheticals.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true // suppress leading whitespace
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	out := b.String()
	return strings.TrimRight(out, " ")
}

// splitModes returns the modal bullets from a "Choose one — ..." style
// oracle text. The first element is the "preamble" — everything before
// the choose-header (typically empty for pure modal cards, but populated
// for cards like Cryptic Command where additional clauses precede the
// header). The remaining elements are the individual mode bodies, in
// declaration order.
//
// Returns nil when no modal header is present. Returns a single-element
// preamble + body when the header exists but no bullets follow (rare
// data error — degrade gracefully).
//
// Callers that want only the modes (not the preamble) should slice [1:].
// The first return value is preserved so the caller can re-assemble the
// card text for display purposes without losing the always-applied
// pre-mode clause.
func splitModes(ot string) []string {
	loc := modeHeaderRE.FindStringIndex(ot)
	if loc == nil {
		return nil
	}
	preamble := strings.TrimSpace(ot[:loc[0]])
	body := strings.TrimSpace(ot[loc[1]:])
	if body == "" {
		return []string{preamble}
	}
	// The first bullet may be missing (some Scryfall printings put the
	// first mode immediately after the em-dash with no leading •). Split
	// on the bullet separator; the first chunk is the first mode.
	parts := modeBulletSep.Split(body, -1)
	modes := make([]string, 0, len(parts)+1)
	modes = append(modes, preamble)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		modes = append(modes, p)
	}
	return modes
}

// hasKeyword reports whether `kw` appears in `ot` as a whole word.
// Substring matching false-fires when a keyword is a prefix/suffix of
// another word — "cast" matches "broadcast", "land" matches "highlander"
// (in flavor text), "mill" matches "milling" but also "millstone" if
// the card text quoted another card by name. Word-boundary regex avoids
// this without losing the simplicity of a string-based check.
//
// `kw` is treated as a literal pattern (regex metacharacters escaped),
// so callers don't have to think about regex syntax.
func hasKeyword(ot, kw string) bool {
	if kw == "" {
		return false
	}
	re := keywordRegexFor(kw)
	return re.MatchString(ot)
}

// keywordRegexCache memoises `\b<kw>\b` patterns so the per-card hot path
// (called 100s of times per deck × 100 cards) avoids regex re-compilation.
// The cache is tiny — Magic keyword vocabulary is bounded — so a plain map
// with a sync.Mutex is enough.
var (
	keywordRegexCache = map[string]*regexp.Regexp{}
	keywordRegexMu    sync.Mutex
)

func keywordRegexFor(kw string) *regexp.Regexp {
	keywordRegexMu.Lock()
	defer keywordRegexMu.Unlock()
	if re, ok := keywordRegexCache[kw]; ok {
		return re
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `\b`)
	keywordRegexCache[kw] = re
	return re
}

// CleanForScan returns oracle text lowercased, with parenthesized reminder
// text stripped and whitespace collapsed. This is the canonical input shape
// for substring-style scanners that should not leak on reminder text — every
// fix in the issue log (cascade EmptiesLibrary, flashback graveyard_curate,
// embalm/eternalize/encore graveyard engines) hinges on this normalization.
//
// Callers that want raw oracle text (e.g. for mana symbol scanning, where
// the reminder gloss *is* the structural cue) should not use this.
func CleanForScan(ot string) string {
	return strings.ToLower(stripReminder(ot))
}

// SplitClauses splits oracle text into sentence-sized clauses. Splits
// happen at:
//
//   - newlines (paragraph breaks between separate abilities)
//   - sentence-terminating "." followed by whitespace
//   - the activated-ability cost/effect divider ":" — the cost segment and
//     the effect segment are functionally separate clauses
//
// Trailing whitespace is trimmed; empty clauses are dropped. Mana symbols
// in braces ({1.5} doesn't exist but {2} and friends do) are preserved
// unchanged because the split is on `". "`, not on a bare period.
//
// Use this to bound substring matches to a single clause. Without it,
// "return target creature to its owner's hand. Then draw a card." in a
// bounce-then-draw spell can co-fire a graveyard-recursion classifier
// looking for `return` + `graveyard` (when the actual `graveyard` token
// appears in a completely unrelated reminder or ability line).
func SplitClauses(ot string) []string {
	if ot == "" {
		return nil
	}
	// Pre-normalize: newlines → " | " sentinel so the period-splitter can
	// also split on paragraph boundaries without losing the cue.
	withSentinels := strings.ReplaceAll(ot, "\n", " | ")
	// Split on `". "` first (sentence break with following whitespace).
	pieces := []string{}
	for _, sent := range clauseSplitRE.Split(withSentinels, -1) {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}
		// Then split each sentence at the sentinel.
		for _, piece := range strings.Split(sent, "|") {
			piece = strings.TrimSpace(piece)
			piece = strings.TrimRight(piece, ".")
			if piece == "" {
				continue
			}
			// Activation cost / effect split — only meaningful when a `:`
			// appears outside braces (mana costs are `{T}:` style, so the
			// `:` is fine to split on; the mana cost token itself stays
			// in the left clause).
			if idx := topLevelColon(piece); idx >= 0 {
				left := strings.TrimSpace(piece[:idx])
				right := strings.TrimSpace(piece[idx+1:])
				if left != "" {
					pieces = append(pieces, left)
				}
				if right != "" {
					pieces = append(pieces, right)
				}
				continue
			}
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

var clauseSplitRE = regexp.MustCompile(`\.\s+`)

// topLevelColon returns the index of the first `:` that is not inside a
// brace pair (so `{T}: draw a card` splits at the colon after `{T}`, but
// `{2}{U}{U}` won't surface a colon). Returns -1 when no top-level colon
// exists.
func topLevelColon(s string) int {
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// HasKeywordIn reports whether any clause in `ot` contains `kw` as a whole
// word. Useful when the caller wants to know `kw` appears AT ALL but cares
// only about word-boundary matches (so "storm" doesn't fire on "Storm
// Crow", "morph" doesn't fire on "polymorph", "infect" doesn't fire on
// "infectious", "transform" doesn't fire on "transformation").
//
// Equivalent to hasKeyword(ot, kw) but the explicit "In" name documents
// the boundary semantics at the call site, where the previous code reads
// strings.Contains(ot, kw).
func HasKeywordIn(ot, kw string) bool { return hasKeyword(ot, kw) }
