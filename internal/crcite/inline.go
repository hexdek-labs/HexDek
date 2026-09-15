package crcite

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// parentRule strips a subrule letter: "702.27b" -> "702.27".
func parentRule(rule string) string {
	i := len(rule)
	for i > 0 && rule[i-1] >= 'a' && rule[i-1] <= 'z' {
		i--
	}
	return rule[:i]
}

var nonWordRE = regexp.MustCompile(`[^a-z0-9]+`)

// labelWindow is how far either side of the rule number a keyword name may
// sit and still count as labelling it rather than merely co-occurring.
const labelWindow = 26

// indexEntryRE matches a comment line that is an index entry: optional
// leading "//" and padding, then the rule number and the keyword name
// adjacent in either order, and nothing much else before an optional
// em-dash gloss.
var indexEntryRE = regexp.MustCompile(`^\s*(?://+)?\s*(?:[-*]\s*)?(?:(?:CR\s*)?§?\d{3}\.\d+[a-z]*\s*[:.\-–—]?\s*[A-Za-z][A-Za-z' !]{2,26}|[A-Za-z][A-Za-z' !]{2,26}\s*(?:N\s*)?[-–—:]\s*(?:CR\s*)?§?\d{3}\.\d+[a-z]*)\s*(?:[-–—(].*)?$`)

// isIndexEntry reports whether a source line is a declaration of the form
// "<rule> <Keyword>" or "<Keyword> — <rule>", rather than prose.
func isIndexEntry(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) > 96 {
		return false
	}
	return indexEntryRE.MatchString(t)
}

// InlineMismatch is a citation whose own SOURCE LINE names a keyword, where
// that keyword's real rule is not the rule cited.
//
// This exists because the filename heuristic is blind to catch-all files.
// `keywords_stubs_tail.go` names no keyword, so a header line reading
// "§702.173  Space Sculptor" sails past FindMismatches — yet Space Sculptor
// is §702.158 and §702.173 is Freerunning. Everything in keywords_batch*.go,
// keywords_misc.go and the stubs files sits in that blind spot, which is a
// large share of the keyword surface.
type InlineMismatch struct {
	File    string
	Line    int
	Rule    string // as cited
	Cited   string // heading of the rule actually cited
	Named   string // keyword named on the same line
	Correct string // that keyword's real rule
	Source  string // the line itself, trimmed
}

// FindInlineMismatches reports citations contradicted by their own line.
//
// It is deliberately conservative: the line must name exactly one keyword,
// that keyword must not be the one cited, and single-word keyword names
// shorter than six characters are ignored because they collide with ordinary
// English ("gift", "ward", "goad") and with card names.
func FindInlineMismatches(c *Corpus, cites []Citation, root string) []InlineMismatch {
	// §702 only. §701 keyword ACTIONS are named with ordinary English verbs
	// — destroy, counter, sacrifice, create, attach, double — which appear in
	// normal engine prose constantly. Matching them produces noise, not
	// findings: a comment on §704.5g that says "destroy" is correct English,
	// not a claim to be citing §701.8.
	byName := map[string]string{}
	for rule, h := range c.headings {
		if !strings.HasPrefix(rule, "702.") {
			continue
		}
		if n, ok := KeywordName(h); ok && len(squash(n)) >= 6 {
			byName[strings.ToLower(n)] = rule
		}
	}

	var out []InlineMismatch
	seen := map[string]bool{}
	for _, ct := range cites {
		if ct.Text == "" || !c.Exists(ct.Rule) {
			continue
		}
		// Both sides must be keyword rules. Citing §510.1c on a line that says
		// "trample", or §903.5c on a line that says "partner", is correct —
		// those rules are ABOUT those keywords. The error we are hunting is an
		// index entry that files one keyword under another keyword's number.
		if !strings.HasPrefix(ct.Rule, "701.") && !strings.HasPrefix(ct.Rule, "702.") {
			continue
		}
		// Normalise the line so "Space Sculptor" matches "space sculptor"
		// and "Living Weapon" matches "living_weapon".
		// Only an INDEX-ENTRY line counts — the shape catch-all files use to
		// declare what they implement:
		//     //   §702.158  Space Sculptor  — partitioned battlefield
		//     // Boast — CR §702.142
		// A short comment line that is essentially just "number + keyword".
		// Anything longer is prose discussing a mechanic, where naming a
		// related keyword is normal and says nothing about the citation.
		if !isIndexEntry(ct.Text) {
			continue
		}
		// The keyword must sit next to the citation, as in
		// "// Boast — CR §702.142" or "//   §702.158  Space Sculptor". A
		// keyword mentioned elsewhere in a sentence is prose about a related
		// mechanic, not a claim about what this number is.
		low := strings.ToLower(ct.Text)
		at := strings.Index(low, strings.ToLower(ct.Rule))
		if at < 0 {
			continue
		}
		lo, hi := at-labelWindow, at+len(ct.Rule)+labelWindow
		if lo < 0 {
			lo = 0
		}
		if hi > len(low) {
			hi = len(low)
		}
		flat := " " + nonWordRE.ReplaceAllString(low[lo:hi], " ") + " "
		var named, correct string
		n := 0
		for kw, rule := range byName {
			if strings.Contains(flat, " "+nonWordRE.ReplaceAllString(kw, " ")+" ") {
				named, correct, n = kw, rule, n+1
			}
		}
		if n != 1 {
			continue // no keyword named, or ambiguous
		}
		if parentRule(ct.Rule) == correct {
			continue // the line names the keyword it cites: consistent
		}
		key := ct.File + "|" + ct.Rule + "|" + named
		if seen[key] {
			continue
		}
		seen[key] = true
		rel, err := filepath.Rel(root, ct.File)
		if err != nil {
			rel = ct.File
		}
		h, _ := c.Heading(ct.Rule)
		out = append(out, InlineMismatch{
			File: filepath.ToSlash(rel), Line: ct.Line, Rule: ct.Rule,
			Cited: h, Named: named, Correct: correct,
			Source: strings.TrimSpace(ct.Text),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
