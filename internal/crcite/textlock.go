package crcite

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// The August 2026 CR inserted a new §704.5w. It did not renumber anything —
// every letter that existed before still exists. What changed is what they
// MEAN: Feb's 704.5w (battle has no protector) is Aug's 704.5x, Feb's 704.5y
// (more than one Role) is Aug's 704.5z, and so on down the tail.
//
// An existence check cannot see that. A citation to §704.5y stays "valid"
// across the edition bump while silently coming to mean a different rule.
// That is worse than a dangling citation, because a dangling one announces
// itself and this one does not.
//
// So we lock the TEXT of every rule the tree actually cites. If the CR file
// changes and a cited rule's text changes with it, the lock fails and names
// the citations that have to be re-read. The edition bump and the citation
// sweep then cannot be separated, which is the only safe way to do either.

// Digest is a short stable fingerprint of a rule's text. Whitespace is
// normalised so a reflow of the source file is not mistaken for a rules
// change; anything else that differs, differs.
func Digest(text string) string {
	sum := sha256.Sum256([]byte(strings.Join(strings.Fields(text), " ")))
	return hex.EncodeToString(sum[:])[:12]
}

// Text returns the full text of a rule as it appears in the corpus.
func (c *Corpus) Text(rule string) (string, bool) {
	t, ok := c.text[rule]
	return t, ok
}

// CitedRules returns every distinct rule number cited in the tree that
// actually exists in the corpus, sorted. Dangling numbers are excluded:
// they have no text to lock, and the hard gate owns them.
func CitedRules(c *Corpus, cites []Citation) []string {
	seen := map[string]bool{}
	var out []string
	for _, ct := range cites {
		if !c.Exists(ct.Rule) || seen[ct.Rule] {
			continue
		}
		if _, ok := c.Text(ct.Rule); !ok {
			continue
		}
		seen[ct.Rule] = true
		out = append(out, ct.Rule)
	}
	sort.Strings(out)
	return out
}
