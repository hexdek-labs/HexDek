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

	// r96 — the pairs r95 fixed, which were NEVER ADDED HERE.
	//
	// That omission is the whole reason this block exists. r94 built
	// this gate to cover what the filename-based mismatch check cannot
	// see, r95 then found 65 more wrong numbers by reading the
	// suppression file, and none of them were written into the gate. So
	// the gate that was built to close the blind spot still did not
	// cover the bugs found THROUGH it — and two of them survived into
	// r96 untouched, both in stack.go, both emitted as event DATA:
	// "701.5a" for a countered spell's destination (§701.5 is Cast) and
	// "702.33" for flashback's exile carveout (§702.33 is Kicker). r95
	// had fixed the second number 600 lines earlier in the same file
	// and not grepped for the rest of it — lesson 32.
	//
	// Each number below was read out of MagicCompRules-20260819.txt by
	// exact section-header match, one at a time, for this table.
	{"702.33", "flashback", "Kicker", "702.34a"},
	{"701.5a", "countered", "Cast", "701.6a"},
	{"702.128", "aftermath", "Embalm", "702.127a"},
	{"702.158", "cleave", "Space Sculptor", "702.148"},
	{"701.51", "discover", "Open an Attraction", "701.57a"},
	{"702.148", "escape", "Cleave", "702.138"},
	{"702.143", "escape", "Foretell", "702.138"},
	{"702.169", "freerunning", "Solved", "702.173a"},
	{"701.15", "investigate", "Goad", "701.16"},
	{"701.36", "investigate", "Populate", "701.16"},
	{"701.15", "regenerat", "Goad", "701.19a"},
	{"702.34", "madness", "Flashback", "702.35"},
	{"701.34", "manifest", "Proliferate", "701.40"},
	{"702.92", "ripple", "Living Weapon", "702.60a"},
	{"702.186", "solved", "the infinity keyword", "702.169"},
	{"701.62", "suspect", "Manifest Dread", "701.60"},
	{"702.121", "tribute", "Melee", "702.104"},
	{"702.99", "extort", "Cipher", "702.101"},
	{"702.176", "bargain", "Impending", "702.166a"},
	{"702.139", "mutate", "Companion", "702.140"},
	{"702.97", "soulbond", "Scavenge", "702.95"},
	{"701.32", "populate", "Set in Motion", "701.36a"},
	{"702.137", "magecraft", "Spectacle", "nothing — §207.2c, ability words have no CR entry"},
}

// r121 — an event-log citation and the Kind naming its action sit in the
// same composite literal but never on the same line.
const maxEventBlockLines = 45

var (
	ruleLiteralRE = regexp.MustCompile(`"rule"\s*:\s*"\d`)
	kindLineRE    = regexp.MustCompile(`Kind:\s*"`)
)

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
		// r121 — MEASURED, DELIBERATELY NOT APPLIED.
		//
		// A bare table entry does not match its own subrules: "701.15"
		// never matched the live "701.15a" in regeneration.go, so
		// {701.15, regenerat} sat here while the engine emitted
		// §701.15a (Goad) on every regenerate event. r121 fixed that
		// site by hand; this gate still cannot see it.
		//
		// Appending `[a-z]?` here closes the gap and takes the gate
		// from 0 violations to 51 — e.g. hat/greedy.go citing §702.110b
		// for menace and tournament/turn.go citing §702.34a for madness.
		// Those 51 are UNREAD. Baselining unread sites is how
		// mismatch_baseline.txt became a worklist, so the widening is
		// recorded here and left off until someone triages them.
		res[i] = regexp.MustCompile(`\b` + regexp.QuoteMeta(c.badRule) + `([^0-9a-z]|$)`)
		topics[i] = strings.ToLower(c.topic)
	}

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		// crcite's own files carry bad numbers by design: this table,
		// and the doc comments in inline.go / inline_test.go that quote
		// a real past mis-citation as the worked example.
		if strings.Contains(filepath.ToSlash(p), "/internal/crcite/") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		lines := strings.Split(string(b), "\n")
		for ln, line := range lines {
			// r121 — WIDEN THE WINDOW TO THE ENCLOSING EVENT BLOCK.
			//
			// This gate matched the number and the topic word on ONE
			// line. An event-log citation does not have that shape:
			//
			//	gs.LogEvent(Event{
			//		Kind: "proliferate",     <- the topic word
			//		Details: map[string]interface{}{
			//			"rule": "701.27",    <- the number
			//
			// so the pair {701.27, proliferate} sat in the table above
			// from r94 while yawgmoth_thran_physician.go emitted it
			// unchallenged. Six of the 34 wrong event-log citations
			// r121 fixed were already named in this table and were
			// invisible for exactly this reason. For a `"rule": "..."`
			// literal, scan back to the nearest Kind: line and treat
			// the whole block as the context.
			topicCtx := strings.ToLower(line)
			if ruleLiteralRE.MatchString(line) {
				for j := ln; j >= 0 && j > ln-maxEventBlockLines; j-- {
					topicCtx += "\n" + strings.ToLower(lines[j])
					if kindLineRE.MatchString(lines[j]) {
						break
					}
				}
			}
			for ci := range r94WrongByTopic {
				if strings.Contains(topicCtx, topics[ci]) && res[ci].MatchString(line) {
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
