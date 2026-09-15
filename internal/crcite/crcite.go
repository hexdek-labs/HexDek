// Package crcite validates Comprehensive Rules citations appearing in the
// HexDek source tree.
//
// # WHY THIS EXISTS
//
// HexDek's second golden rule requires every mechanic to be traceable to a
// Comprehensive Rules section. That rule created an obligation to CITE but
// never a means to VERIFY, and the gap between those two is where fabricated
// citations live: if nothing checks the number, a plausible-looking wrong one
// costs the author nothing and is indistinguishable from a correct one.
//
// A 2026-09-15 audit found ~7,500 citations in the tree, of which 84 named
// rules that exist in NO edition of the Comprehensive Rules. Those were never
// stale — they were never real. A wrong citation is worse than no citation,
// because a reader assumes somebody checked it.
//
// This package makes that class of error mechanically impossible.
package crcite

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// citationRE matches the citation forms that actually occur in the tree
// (verified by survey, not assumed): "§704.5g", "CR 701.50a", and the
// `"rule": "701.50"` log field. The rule number itself is group 1.
var citationRE = regexp.MustCompile(`(?:§|CR\s+|"rule":\s*")(\d{3}\.\d+[a-z]*)`)

// ruleHeadingRE matches a numbered CR entry that carries a title, e.g.
// "702.97. Scavenge" or "722. Preparation Cards". Sub-rules ("702.97a …")
// are bodies, not headings, and are captured separately by ruleExistsRE.
var ruleHeadingRE = regexp.MustCompile(`^(\d{3}(?:\.\d+)?)\.\s+(\S.*)$`)

// ruleExistsRE matches any numbered CR line, heading or body.
var ruleExistsRE = regexp.MustCompile(`^(\d{3}\.\d+[a-z]*)`)

// Corpus is a parsed Comprehensive Rules document.
type Corpus struct {
	// Effective is the "These rules are effective as of ..." line, so a
	// caller can report which edition a verdict was reached against.
	Effective string
	// exists holds every rule number the document defines, heading or body.
	exists map[string]bool
	// headings maps a rule number to its title where it has one.
	headings map[string]string
	// text maps a rule number to its full body line. Used by the text lock
	// to notice a rule whose NUMBER survives an edition bump while its
	// MEANING changes underneath the citations that point at it.
	text map[string]string
}

// Citation is one CR reference found in the source tree.
type Citation struct {
	File string
	Line int
	Rule string
}

// LoadCorpus parses a Comprehensive Rules text file.
func LoadCorpus(path string) (*Corpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("crcite: open CR corpus: %w", err)
	}
	defer f.Close()

	c := &Corpus{exists: map[string]bool{}, headings: map[string]string{}, text: map[string]string{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimLeft(sc.Text(), string(rune(0xFEFF))))
		if c.Effective == "" && strings.HasPrefix(line, "These rules are effective as of") {
			c.Effective = line
		}
		if m := ruleExistsRE.FindStringSubmatch(line); m != nil {
			c.exists[m[1]] = true
			c.text[m[1]] = line
		}
		if m := ruleHeadingRE.FindStringSubmatch(line); m != nil {
			c.headings[m[1]] = strings.TrimSpace(m[2])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("crcite: scan CR corpus: %w", err)
	}
	if len(c.exists) == 0 {
		return nil, fmt.Errorf("crcite: %s parsed to zero rules — wrong file?", path)
	}
	return c, nil
}

// Exists reports whether the corpus defines this rule number.
func (c *Corpus) Exists(rule string) bool { return c.exists[rule] }

// Heading returns the title of the rule or of its parent rule, plus whether
// one was found. "702.97a" falls back to "702.97"'s heading.
func (c *Corpus) Heading(rule string) (string, bool) {
	if h, ok := c.headings[rule]; ok {
		return h, true
	}
	if i := strings.LastIndexAny(rule, "0123456789"); i >= 0 && i+1 < len(rule) {
		if h, ok := c.headings[rule[:i+1]]; ok {
			return h, true
		}
	}
	return "", false
}

// RuleCount reports how many distinct rule numbers the corpus defines.
func (c *Corpus) RuleCount() int { return len(c.exists) }

// Scan walks root and returns every CR citation found in .go files.
func Scan(root string) ([]Citation, error) {
	var out []Citation
	err := walkGoFiles(root, func(path string) error {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
		for n := 1; sc.Scan(); n++ {
			for _, m := range citationRE.FindAllStringSubmatch(sc.Text(), -1) {
				out = append(out, Citation{File: path, Line: n, Rule: m[1]})
			}
		}
		return sc.Err()
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

func walkGoFiles(root string, fn func(string) error) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		p := root + "/" + name
		if e.IsDir() {
			if name == ".git" || name == "node_modules" || name == "vendor" {
				continue
			}
			if err := walkGoFiles(p, fn); err != nil {
				return err
			}
			continue
		}
		if strings.HasSuffix(name, ".go") {
			if err := fn(p); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
	}
	return nil
}
