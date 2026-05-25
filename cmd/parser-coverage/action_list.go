package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Action is a single TODO entry on the scaffold checklist for an
// uncovered card. Title is a short imperative ("Add triggered-ability
// handler for ETB"); Reason quotes the oracle/type-line snippet that
// motivated it so a developer can jump straight to the relevant clause.
type Action struct {
	Title  string
	Reason string
}

// generateActionList produces a checklist of concrete scaffold/handler
// TODOs that would need to land to move the given card from its current
// class to OK. OK/OK_VANILLA short-circuit to an empty list — those
// cards are already covered.
//
// The list combines:
//   - class-level actions (MISSING → re-ingest; PARTIAL → resolve each
//     parse_error; EMPTY_AST → extend parser)
//   - pattern-driven actions derived from oracle text + type line
//     (triggered/activated/static abilities, modal, replacement effect,
//     saga/class/battle/planeswalker, transform/meld/adventure, etc.)
//
// Output is sorted by title for deterministic rendering.
func generateActionList(r result, typeLine string) []Action {
	switch r.Class {
	case classOK, classOKVanilla:
		return nil
	}

	var actions []Action
	switch r.Class {
	case classMissing:
		actions = append(actions, Action{
			Title:  "Re-ingest card via Thor (no entry in ast_dataset.jsonl)",
			Reason: "MISSING: parser pipeline never produced an AST for this card",
		})
	case classPartial:
		if len(r.ParseErrors) == 0 {
			actions = append(actions, Action{
				Title:  "Investigate fully_parsed=false flag (no explicit parse_errors recorded)",
				Reason: "PARTIAL: corpus entry marked partial but ParseErrors is empty",
			})
		}
		for _, pe := range r.ParseErrors {
			actions = append(actions, Action{
				Title:  fmt.Sprintf("Resolve parser error: %s", pe),
				Reason: "from corpus.ParseErrors",
			})
		}
	case classEmptyAST:
		actions = append(actions, Action{
			Title:  "Extend Python parser to produce non-empty abilities for this oracle text",
			Reason: "EMPTY_AST: AST entry exists but contains zero abilities despite non-trivial oracle text",
		})
	}

	// Pattern-driven scaffolds (deduped within this call).
	actions = append(actions, scaffoldActions(r.OracleText, typeLine)...)

	// Sort by title for stable output across runs.
	sort.Slice(actions, func(i, j int) bool { return actions[i].Title < actions[j].Title })
	return actions
}

// scaffoldPattern is a single detection rule. match returns the
// snippet that fired the rule (used as the Reason) or "" if no match.
// The two-arg signature lets a rule key off oracle text, type line, or
// both — e.g., Saga handling is driven by type line, not oracle wording.
type scaffoldPattern struct {
	title string
	match func(text, typeLine string) string
}

// scaffoldPatterns is the catalog of oracle-text / type-line patterns
// the action-list emitter recognizes. Order doesn't matter for output
// (the emitter sorts by title) but each pattern fires independently so
// a card can collect several actions if its oracle text touches
// multiple concerns.
//
// Note: these are deliberately broad — false positives are cheap (a
// developer reads the snippet and skips) and false negatives are
// expensive (the checklist misses the real work).
var scaffoldPatterns = []scaffoldPattern{
	{
		title: "Add ETB triggered-ability handler (`When … enters the battlefield`)",
		match: firstLineMatching(`(?i)when\s+(this|cardname|[\w \-',]+)\s+enters( the battlefield)?`),
	},
	{
		title: "Add LTB / dies triggered-ability handler",
		match: firstLineMatching(`(?i)when\s+(this|cardname|[\w \-',]+)\s+(dies|leaves|is put into a graveyard)`),
	},
	{
		title: "Add `whenever` triggered-ability handler",
		match: firstLineMatching(`(?i)^\s*whenever\b`),
	},
	{
		title: "Add `at the beginning of …` phase-trigger handler",
		match: firstLineMatching(`(?i)at the beginning of`),
	},
	{
		title: "Add activated-ability handler (`{cost}: effect`)",
		match: firstLineMatching(`\{[^}]+\}[^:\n]*:\s*\S`),
	},
	{
		title: "Add mana-ability handler (`{T}: Add …`)",
		match: substring(`(?i)\{T\}:\s*Add\b`),
	},
	{
		title: "Add modal-spell handler (`Choose one/two/up to …`)",
		match: substring(`(?i)choose (one|two|three|up to)`),
	},
	{
		title: "Add replacement-effect handler (`if … would … instead`)",
		match: substring(`(?i)if [^.]* would [^.]* instead`),
	},
	{
		title: "Add ETB-with-counters replacement handler (`enters with N counter`)",
		match: substring(`(?i)enters with \w+ [\w/+\-]* ?counter`),
	},
	{
		title: "Add saga chapter handler",
		match: typeLineContains("saga"),
	},
	{
		title: "Add class level-up handler",
		match: func(text, tl string) string {
			low := strings.ToLower(tl)
			// "Class" the supertype, not "Creature — Class" (cleric/etc.).
			if strings.Contains(low, "enchantment") && strings.Contains(low, "class") {
				return "type_line: " + tl
			}
			return ""
		},
	},
	{
		title: "Add planeswalker loyalty-ability handler",
		match: typeLineContains("planeswalker"),
	},
	{
		title: "Add battle defense-counter handler",
		match: typeLineContains("battle"),
	},
	{
		title: "Add transform / double-faced handler",
		match: func(text, _ string) string {
			low := strings.ToLower(text)
			if strings.Contains(low, "transform") {
				return "oracle mentions transform"
			}
			if strings.Contains(text, "//") {
				return "card name contains '//' (DFC)"
			}
			return ""
		},
	},
	{
		title: "Add meld handler",
		match: substring(`(?i)melds with`),
	},
	{
		title: "Add adventure cast-then-exile handler",
		match: substring(`Adventure`),
	},
	{
		title: "Add prototype alt-cost handler",
		match: substring(`(?i)\bprototype\b`),
	},
	{
		title: "Add mutate stack-merge handler",
		match: substring(`(?i)\bmutate\b`),
	},
	{
		title: "Add library-search handler (`Search your library`)",
		match: substring(`(?i)search your library`),
	},
	{
		title: "Add counter-spell handler (`Counter target …`)",
		match: substring(`(?i)counter target`),
	},
	{
		title: "Add token-creation handler",
		match: substring(`(?i)create (a|an|\w+) [^.]*token`),
	},
	{
		title: "Add +1/+1 counter-placement handler",
		match: substring(`(?i)\+1/\+1 counter`),
	},
	{
		title: "Add -1/-1 counter-placement handler",
		match: substring(`(?i)-1/-1 counter`),
	},
	{
		title: "Add card-draw handler",
		match: substring(`(?i)draw (a|\d+|that many|x|two|three|four|five) card`),
	},
	{
		title: "Add cost-reduction handler (`costs {N} less`)",
		match: substring(`(?i)costs?\s*\{[^}]+\}\s*less`),
	},
	{
		title: "Add sacrifice-cost handler",
		match: substring(`(?i)sacrifice (a|an|another|\d+|two|three)`),
	},
	{
		title: "Add bounce handler (`return … to … hand`)",
		match: substring(`(?i)return [^.]* to (its|their|your) owner'?s? hand`),
	},
	{
		title: "Add exile-target handler",
		match: substring(`(?i)exile target`),
	},
	{
		title: "Add damage-dealing handler",
		match: substring(`(?i)deals?\s+\d+\s+damage`),
	},
	{
		title: "Add lifegain handler",
		match: substring(`(?i)gain (\d+|x|that much) life`),
	},
	{
		title: "Add mill handler",
		match: substring(`(?i)mill[s]?\s+\w+\s+card`),
	},
	{
		title: "Add discard handler",
		match: substring(`(?i)discard (a|\d+|your|two|three) card`),
	},
	{
		title: "Add scry handler",
		match: substring(`(?i)scry \d+`),
	},
	{
		title: "Add surveil handler",
		match: substring(`(?i)surveil \d+`),
	},
	{
		title: "Add keyword-stew scaffold (single-line keyword list)",
		match: func(text, _ string) string {
			t := strings.TrimSpace(text)
			if t == "" || strings.Contains(t, "\n") {
				return ""
			}
			if len(t) >= 60 {
				return ""
			}
			// A keyword stew rarely contains a sentence-ending period
			// followed by more clauses. A short comma-separated list
			// like "Flying, vigilance, lifelink" is the target.
			if !strings.Contains(t, ",") && !looksLikeKeyword(t) {
				return ""
			}
			return t
		},
	},
}

// looksLikeKeyword returns true for a short single-word/phrase that
// resembles a bare keyword line ("Flying.", "Indestructible"). Used as
// a secondary check for keyword-stew detection on terse single-keyword
// cards so they still surface as keyword-only.
func looksLikeKeyword(t string) bool {
	low := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(t), "."))
	switch low {
	case "flying", "trample", "deathtouch", "lifelink", "first strike",
		"double strike", "vigilance", "haste", "menace", "reach",
		"defender", "indestructible", "hexproof", "shroud", "flash",
		"prowess", "skulk", "infect", "toxic", "horsemanship":
		return true
	}
	return false
}

// substring builds a match function that returns the first regex
// substring match in oracle text, or "" if no match.
func substring(pattern string) func(text, _ string) string {
	re := regexp.MustCompile(pattern)
	return func(text, _ string) string {
		if m := re.FindString(text); m != "" {
			return strings.TrimSpace(m)
		}
		return ""
	}
}

// firstLineMatching returns the first oracle-text line that matches
// the regex (whole-line scan), trimmed. Used for triggers/activated
// abilities that naturally live on their own line.
func firstLineMatching(pattern string) func(text, _ string) string {
	re := regexp.MustCompile(pattern)
	return func(text, _ string) string {
		for _, line := range strings.Split(text, "\n") {
			if re.MatchString(line) {
				return strings.TrimSpace(line)
			}
		}
		return ""
	}
}

// typeLineContains builds a match function keyed off the Scryfall
// type_line. The token must appear as a separate word (lowercased).
func typeLineContains(token string) func(text, typeLine string) string {
	return func(_, tl string) string {
		if strings.Contains(strings.ToLower(tl), token) {
			return "type_line: " + tl
		}
		return ""
	}
}

// scaffoldActions runs every pattern, dedupes by title, and returns
// the resulting Actions unsorted (generateActionList sorts the union
// later so class-level + scaffold actions interleave deterministically).
func scaffoldActions(text, typeLine string) []Action {
	var out []Action
	seen := map[string]bool{}
	for _, p := range scaffoldPatterns {
		snip := p.match(text, typeLine)
		if snip == "" {
			continue
		}
		if seen[p.title] {
			continue
		}
		seen[p.title] = true
		out = append(out, Action{Title: p.title, Reason: snip})
	}
	return out
}

// renderActionList produces the markdown checklist for one card.
// Format is intentionally lightweight — a single H1 with the card
// name, an oracle-text blockquote, and a checkbox list of TODOs. The
// "matched" snippet line under each item is what a developer reads to
// decide whether the action is real work or noise.
func renderActionList(name, typeLine string, r result, actions []Action) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Action list — %s\n\n", name)
	fmt.Fprintf(&b, "Class: **%s**  \n", r.Class)
	if typeLine != "" {
		fmt.Fprintf(&b, "Type: `%s`  \n", typeLine)
	}
	b.WriteString("\n")

	if strings.TrimSpace(r.OracleText) != "" {
		b.WriteString("## Oracle text\n\n")
		for _, line := range strings.Split(strings.TrimSpace(r.OracleText), "\n") {
			fmt.Fprintf(&b, "> %s\n", line)
		}
		b.WriteString("\n")
	}

	b.WriteString("## TODO\n\n")
	if r.Class == classOK || r.Class == classOKVanilla {
		fmt.Fprintf(&b, "_Already covered (class=%s). No action needed._\n", r.Class)
		return b.String()
	}
	if len(actions) == 0 {
		b.WriteString("_No scaffold patterns detected; manual inspection required._\n")
		return b.String()
	}
	for _, a := range actions {
		fmt.Fprintf(&b, "- [ ] %s\n", a.Title)
		if a.Reason != "" {
			fmt.Fprintf(&b, "  - _matched:_ %s\n", a.Reason)
		}
	}
	return b.String()
}
