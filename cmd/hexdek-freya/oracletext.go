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

// SplitUnless separates a clause at " unless " into the primary effect
// and the tax-out condition.
//
//	"destroy target creature unless its controller pays {3}"
//	→ primary="destroy target creature", condition="its controller pays {3}"
//
// The "unless" syntax in Magic is a CONDITIONAL — the primary effect is
// the DEFAULT outcome, and the condition is what the affected player can
// do (or what must be true) to avoid it. Distinguishing the two lets
// downstream removal classifiers flag soft removal (Mana Leak, Doom Blade
// at +2 mana) separately from hard removal (Swords to Plowshares).
//
// Returns (clause, "") when no " unless " is present. Case-insensitive
// match on " unless " (lowercased input expected, since this file's other
// scanners run on CleanForScan output).
//
// Edge cases:
//   - Multiple " unless " in one clause — splits at the FIRST occurrence.
//     "Counter target spell unless its controller pays {X} unless they
//     have hexproof" is not real Magic wording; the first-occurrence
//     behavior is a sensible default for the synthetic case.
//   - " unless " inside a sub-phrase ("...effects that say 'unless...'")
//     — caller's job to avoid; SplitUnless treats every occurrence as a
//     real split. None of the real-card scanners feed sub-quoted text
//     into SplitUnless.
func SplitUnless(clause string) (primary, condition string) {
	const sep = " unless "
	idx := strings.Index(clause, sep)
	if idx < 0 {
		return clause, ""
	}
	primary = strings.TrimSpace(clause[:idx])
	condition = strings.TrimSpace(clause[idx+len(sep):])
	// Drop a trailing period from the condition (the original clause's
	// terminator survives the SplitClauses pipeline).
	condition = strings.TrimRight(condition, ".")
	return primary, condition
}

// IsSoftRemoval reports whether a clause is a conditional / tax-style
// removal effect — a destroy/exile/counter wrapped in an unless-tax that
// the opponent can pay around.
//
//	"counter target spell unless its controller pays {3}"  → true (Mana Leak)
//	"destroy target creature unless its controller pays {3}"  → true (Edict tax)
//	"destroy target creature."  → false (Doom Blade — hard removal)
//	"counter target spell."  → false (Counterspell — hard counter)
//
// Used by removal-count tuning so "Mana Leak" doesn't count as a hard
// counter the way "Counterspell" does. The DEFAULT outcome must still
// be a removal/counter verb; "draw a card unless an opponent pays {1}"
// does NOT qualify.
func IsSoftRemoval(clause string) bool {
	primary, cond := SplitUnless(clause)
	if cond == "" {
		return false
	}
	// The primary must be a removal/counter shape.
	hasRemovalVerb := strings.Contains(primary, "destroy") ||
		strings.Contains(primary, "exile") ||
		strings.Contains(primary, "counter ")
	if !hasRemovalVerb {
		return false
	}
	// The condition must be a mana-payment / sacrifice / discard / life
	// tax — not an irrelevant condition like "unless it's a token".
	return strings.Contains(cond, "pay") ||
		strings.Contains(cond, "sacrifice") ||
		strings.Contains(cond, "discard")
}

// triggerHeaderRE matches the canonical "Whenever X" / "When X" / "At Y"
// trigger headers that introduce a triggered ability. The header runs up
// to the first comma — Magic's triggered-ability syntax always uses
// "<header>, <effect>" or "<header>, if <condition>, <effect>".
//
// Anchored at start-of-string OR after a newline (multi-paragraph
// printings) so we don't false-match a "whenever" buried inside an
// effect body (rare but possible: "Untap each creature you control.
// Whenever one of them attacks this turn, …" — second sentence has its
// own trigger header).
var triggerHeaderRE = regexp.MustCompile(
	`(?i)(?:^|\n)\s*(whenever|when|at)\s+([^,]+),\s*`)

// SplitTriggerEffect decomposes a triggered-ability clause into
// (triggerKind, triggerCondition, ifCondition, effect):
//
//	"Whenever a creature you control dies, draw a card."
//	→ ("whenever", "a creature you control dies", "", "draw a card.")
//
//	"Whenever ~ attacks, if you control a Goblin, deal 2 damage to any target."
//	→ ("whenever", "~ attacks", "you control a Goblin", "deal 2 damage to any target.")
//
//	"At the beginning of your upkeep, draw a card."
//	→ ("at", "the beginning of your upkeep", "", "draw a card.")
//
// Returns all empty strings for non-trigger clauses. The triggerKind is
// lowercased ("whenever" / "when" / "at"). The optional ifCondition
// (intervening-if per CR §603.4) is extracted when the effect body
// starts with "if <X>,".
//
// This supersedes the hand-enumerated stripDrawTriggerClauses pattern in
// analysis.go for a small class of detectors that need to scope substring
// matches to the EFFECT body without bleeding into the trigger condition.
func SplitTriggerEffect(clause string) (kind, trigger, ifCond, effect string) {
	m := triggerHeaderRE.FindStringSubmatchIndex(clause)
	if m == nil {
		return "", "", "", ""
	}
	kind = strings.ToLower(clause[m[2]:m[3]])
	trigger = strings.TrimSpace(clause[m[4]:m[5]])
	effect = strings.TrimSpace(clause[m[1]:])

	// Intervening-if check — the effect body starts with "if <X>,".
	const ifPrefix = "if "
	if strings.HasPrefix(strings.ToLower(effect), ifPrefix) {
		// Find the comma that closes the if-clause. Bounded scan; the
		// if-condition is by convention a single comma-terminated phrase.
		rest := effect[len(ifPrefix):]
		if commaIdx := strings.Index(rest, ","); commaIdx >= 0 {
			ifCond = strings.TrimSpace(rest[:commaIdx])
			effect = strings.TrimSpace(rest[commaIdx+1:])
		}
	}
	return kind, trigger, ifCond, effect
}

// -----------------------------------------------------------------------------
// ScanOracle — unified tokenizing scanner.
//
// ScanOracle is the single entry point that prior callers re-implemented
// piecemeal: lowercase the text, strip reminder parens, split into clauses,
// AND tag each clause with which modal bullet (if any) it came from. Once a
// caller has a `*OracleScan` it can run per-clause substring checks without
// paying for the upstream string surgery on every call site, and gets
// modal-mode information for free.
//
// The scanner is intentionally conservative — no AST, no NLP. The clause
// splitter is sentence-and-colon based (see SplitClauses); the modal tagger
// re-uses splitModes (see above). Both have been false-positive-pinned
// against the Scryfall corpus.
//
// Resource cost: ~one allocation per clause; ScanOracle is fine to call
// per-card. Callers that want to reuse a scan across multiple classifier
// passes should hold the *OracleScan in a local variable, not call
// ScanOracle repeatedly.
// -----------------------------------------------------------------------------

// OracleScan is the structured result of tokenizing one card's oracle text.
//
//   - Clean: lowercased, reminder-stripped, whitespace-collapsed. The shape
//     analysis.go used to compute inline via strings.ToLower(stripReminder(...)).
//   - Clauses: clause-level splits (paragraph break / sentence period /
//     activation cost colon), each tagged with Mode (-1 for always-applied
//     text, 0..N for modal bullet index when the source was a "Choose one/
//     two/..." modal printing).
//
// Modal bullets become independent clauses with Mode >= 0. A non-modal card
// produces clauses with Mode == -1. A card with a preamble + bullets puts
// the preamble's clauses at Mode == -1 and each bullet at Mode == 0, 1, …
//
// Empty input yields a non-nil *OracleScan with empty fields so callers can
// `.Clean` and `.Clauses` without nil checks.
type OracleScan struct {
	Clean   string
	Clauses []ScanClause
}

// ScanClause is a clause + which mode it came from. Mode == -1 means the
// clause is unconditionally applied (no modal context); Mode >= 0 indexes
// into the modal bullet list (0 = first bullet, 1 = second, etc.).
type ScanClause struct {
	Text string
	Mode int
}

// ScanOracle tokenizes a card's raw oracle text. See OracleScan / ScanClause.
func ScanOracle(raw string) *OracleScan {
	scan := &OracleScan{}
	if raw == "" {
		return scan
	}
	scan.Clean = CleanForScan(raw)
	if scan.Clean == "" {
		return scan
	}
	// Modal detection uses the cleaned text — modeHeaderRE is case-insensitive
	// and the bullet character survives lowercasing intact.
	modes := splitModes(scan.Clean)
	if modes == nil {
		// Non-modal card — every clause is Mode -1.
		for _, c := range SplitClauses(scan.Clean) {
			scan.Clauses = append(scan.Clauses, ScanClause{Text: c, Mode: -1})
		}
		return scan
	}
	// Modal card: modes[0] is the preamble (always-applied), modes[1:] are
	// the bullets in declaration order.
	for _, c := range SplitClauses(modes[0]) {
		scan.Clauses = append(scan.Clauses, ScanClause{Text: c, Mode: -1})
	}
	for i, body := range modes[1:] {
		for _, c := range SplitClauses(body) {
			scan.Clauses = append(scan.Clauses, ScanClause{Text: c, Mode: i})
		}
	}
	return scan
}

// ClauseCoOccurs reports whether some single clause in `scan` contains BOTH
// substrings. The canonical use case is a paired substring scanner like
// `strings.Contains(ot, "exile") && strings.Contains(ot, "from your graveyard")`
// — the raw form fires on Cabal Therapy because "exile" lives in the
// flashback reminder and "from your graveyard" also lives there, but it
// also fires on cards where the two substrings live in unrelated clauses
// across paragraph or modal boundaries. Narrowing the co-occurrence to a
// single clause kills both classes of leak in one helper.
//
// Both substrings are matched as raw substrings (NOT word-boundary) — the
// caller's job to pick distinctive enough anchors. For word-boundary
// matching, see HasKeywordIn.
//
// Empty `scan` or empty substrings return false.
func ClauseCoOccurs(scan *OracleScan, a, b string) bool {
	if scan == nil || a == "" || b == "" {
		return false
	}
	for _, c := range scan.Clauses {
		if strings.Contains(c.Text, a) && strings.Contains(c.Text, b) {
			return true
		}
	}
	return false
}

// ClauseContains reports whether some single clause in `scan` contains the
// given substring. Cheaper-than-CoOccurs single-substring scan with the
// same modal/reminder safety properties.
func ClauseContains(scan *OracleScan, s string) bool {
	if scan == nil || s == "" {
		return false
	}
	for _, c := range scan.Clauses {
		if strings.Contains(c.Text, s) {
			return true
		}
	}
	return false
}

// HasDependentTrigger returns true when `effect` contains a nested
// trigger that fires as a CONSEQUENCE of the outer trigger's resolution.
// The canonical shape is a follow-up sentence whose trigger condition
// references "that <thing>" — pointing back at a target named by the
// outer effect.
//
//	"Exile target creature that player controls. When that creature dies,
//	you draw a card."
//	→ true ("when that creature dies" is dependent on the exile)
//
//	"Exile target creature. Draw a card."
//	→ false (no nested trigger; just a follow-up effect)
//
// Used by trigger-completeness analyzers to recognize that an outer
// effect's target is load-bearing for a SECOND trigger — so a single
// trigger primer fixture isn't enough; the dependent leg needs its
// world too.
//
// Heuristic: looks for "when that <noun>" or "whenever that <noun>" as
// a substring of the effect body. False positives are rare in real
// Magic text — the "that <X>" demonstrative reference is a deliberate
// callback pattern used precisely for dependent triggers.
func HasDependentTrigger(effect string) bool {
	low := strings.ToLower(effect)
	return strings.Contains(low, "when that ") ||
		strings.Contains(low, "whenever that ")
}
