package crcite

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// r94 — the gap this closes.
//
// The mismatch gate (crcite_test.go) decides a cite is suspicious by
// comparing it against the keyword the FILE IS NAMED FOR. That works for
// keywords_disguise.go and is structurally blind everywhere else: a
// shield-counter definition inside counters/registry_init.go citing
// §701.58 (Cloak) has no filename to disagree with, so it sailed through
// every run. Same reason §706.2-for-copying and §702.166-for-disguise
// survived — and the ones the gate DID catch were suppressed into
// mismatch_baseline.txt with the correct number written in the comment
// beside them.
//
// This gate works the other way round: it pairs a rule number with a
// TOPIC WORD and fails if both appear on the same line. Each pair below
// is a specific wrong citation fixed in r94, verified one at a time
// against data/rules/MagicCompRules-20260819.txt. It is deliberately
// narrow — it pins what was wrong, it does not try to judge new cites.
var r94WrongByTopic = []struct {
	badRule string // the number that was there
	topic   string // a word that must not appear on the same line
	owner   string // what badRule actually is in the CR
	right   string // what the topic should cite
}{
	{"706.2", "copy", "Rolling a Die", "707.2"},
	{"706.10", "copy", "Rolling a Die", "707.10"},
	{"707.2", "face-down", "Copying Objects", "708.2"},
	{"708.4", "no abilities", "cast-face-down timing", "708.2"},
	{"702.166", "disguise", "Bargain", "702.168"},
	{"702.171", "cloak", "Saddle", "701.58"},
	{"702.36", "morph", "Fear", "702.37"},
	{"701.58", "shield", "Cloak", "122.1c"},
	{"701.55", "stun", "Face a Villainous Choice", "122.1d"},
	{"701.23", "proliferate", "Search", "701.34"},
	{"701.27", "proliferate", "Transform", "701.34"},
	{"702.61", "suspend", "Split Second", "702.62"},
	{"702.131", "vanishing", "Ascend", "702.63"},
	{"704.5r", "pair-removal", "the can't-have-more-than-N SBA", "704.5q"},
	{"702.110", "menace", "Exploit", "702.111"},
	{"702.27", "shadow", "Buyback", "702.28"},
	{"702.91", "infect", "Battle Cry", "702.90"},
	{"702.91", "living weapon", "Battle Cry", "702.92"},
	{"702.146", "decayed", "Disturb", "702.147"},
	{"702.19a", "ripple", "Trample", "702.60a"},
	{"701.8a", "discard", "Destroy", "701.9a"},
	{"702.3", "vigilance", "Defender", "702.20"},
	// Topic-word variants. The first sweep missed these because the line
	// said "copiable" rather than "copy" and "non-pairing" rather than
	// "pair-removal" — a reminder that the topic word is the weak half
	// of this gate, not the number.
	{"706.2", "copiable", "Rolling a Die", "707.2"},
	{"706.2", "clone", "Rolling a Die", "707.2"},
	{"708.4", "face-down", "cast-face-down timing", "708.2"},
	{"704.5r", "non-pairing", "the can't-have-more-than-N SBA", "704.5q"},
}

func TestCRCitations_NoWrongNumberForTopic_R94(t *testing.T) {
	root := repoRootForR94(t)
	type hit struct {
		file, line string
		n          int
		c          int
	}
	var hits []hit

	// Compile once. Doing this inside the per-line loop costs 22 regex
	// compiles per line of every .go file in the repo and takes minutes.
	res := make([]*regexp.Regexp, len(r94WrongByTopic))
	topics := make([]string, len(r94WrongByTopic))
	for i, c := range r94WrongByTopic {
		res[i] = regexp.MustCompile(`\b` + regexp.QuoteMeta(c.badRule) + `([^0-9a-z]|$)`)
		topics[i] = strings.ToLower(c.topic)
	}

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		// This file carries every bad number in its table by design.
		if filepath.Base(p) == "wrong_number_by_topic_r94_test.go" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for ln, line := range strings.Split(string(b), "\n") {
			low := strings.ToLower(line)
			for ci := range r94WrongByTopic {
				if strings.Contains(low, topics[ci]) && res[ci].MatchString(line) {
					hits = append(hits, hit{rel, strings.TrimSpace(line), ln + 1, ci})
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	for _, h := range hits {
		c := r94WrongByTopic[h.c]
		t.Errorf("%s:%d cites §%s on a line about %q — §%s is %s. %s is §%s.\n\t%s",
			h.file, h.n, c.badRule, c.topic, c.badRule, c.owner, c.topic, c.right, h.line)
	}
	t.Logf("r94 wrong-number-by-topic: %d pairs checked, %d violations", len(r94WrongByTopic), len(hits))
}

func repoRootForR94(t *testing.T) string {
	t.Helper()
	d, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatal("could not locate repo root (no go.mod above cwd)")
	return ""
}
