package crcite

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var wordRE = regexp.MustCompile(`[a-z]+`)

// KeywordName reports whether a rule heading is a keyword NAME ("Cascade",
// "Start Your Engines!") rather than the opening paragraph of prose that
// heads a section (701.1, 702.1). Only named rules can be topic-checked:
// a citation to a prose rule says nothing about what it is about.
func KeywordName(h string) (string, bool) {
	h = strings.TrimSpace(h)
	if len(h) > 28 || strings.ContainsAny(h, ".,“”") {
		return "", false
	}
	if n := len(strings.Fields(h)); n == 0 || n > 3 {
		return "", false
	}
	return strings.ToLower(h), true
}

func squash(s string) string { return strings.Join(wordRE.FindAllString(strings.ToLower(s), -1), "") }

// Mismatch is a citation whose rule number exists but whose subject does not
// match the keyword its file is named for. It is a HEURISTIC: a file may cite
// a neighbouring keyword legitimately (keywords_disguise.go citing Morph).
// That is why mismatches ride a shrinking baseline instead of failing hard.
type Mismatch struct {
	File     string // repo-relative
	Rule     string // as cited
	Heading  string // what that rule is actually about
	FileKW   string // the keyword the file is named for
	FileRule string // that keyword's real rule number
}

// Key is the baseline identity of a mismatch: file and cited rule.
func (m Mismatch) Key() string { return m.File + "|" + m.Rule }

// FindMismatches pairs each citation against the keyword its file is named
// for. It reports only unambiguous cases: the cited rule is a named keyword
// rule, the filename names exactly one different keyword, and the two differ.
func FindMismatches(c *Corpus, cites []Citation, root string) []Mismatch {
	byName := map[string]string{}
	for rule, h := range c.headings {
		if !strings.HasPrefix(rule, "701.") && !strings.HasPrefix(rule, "702.") {
			continue
		}
		if n, ok := KeywordName(h); ok {
			byName[squash(n)] = rule
		}
	}

	var out []Mismatch
	seen := map[string]bool{}
	for _, ct := range cites {
		if !c.Exists(ct.Rule) {
			continue // dangling; the hard gate owns that class
		}
		h, ok := c.Heading(ct.Rule)
		if !ok {
			continue
		}
		cited, isKW := KeywordName(h)
		if !isKW {
			continue
		}
		base := squash(strings.TrimSuffix(filepath.Base(ct.File), ".go"))
		if strings.Contains(base, squash(cited)) {
			continue // the file names the keyword it cites
		}
		var name, rule string
		n := 0
		for kw, r := range byName {
			// >=6 chars: short names ("gift", "goad") collide with ordinary
			// English in filenames and produce noise, not findings.
			if len(kw) >= 6 && strings.Contains(base, kw) {
				name, rule, n = kw, r, n+1
			}
		}
		if n != 1 {
			continue // filename names no keyword, or several: not actionable
		}
		rel, err := filepath.Rel(root, ct.File)
		if err != nil {
			rel = ct.File
		}
		rel = filepath.ToSlash(rel)
		m := Mismatch{File: rel, Rule: ct.Rule, Heading: h, FileKW: name, FileRule: rule}
		if seen[m.Key()] {
			continue
		}
		seen[m.Key()] = true
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}
