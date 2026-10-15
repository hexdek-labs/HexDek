package crcite

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// r122 — an event's Details["rule"] must name the action the event IS.
//
// The engine stamps a CR citation into Details["rule"] on keyword events.
// That string reaches the event log, so it is outcome data. The event's own
// Kind already says which action it is, so this is a join against the CR's
// §701/§702 headings rather than a table of hand-written pairs — it needs no
// maintenance when a new keyword is added, and it catches a wrong citation
// the first time one is written.
//
// Why this exists alongside TestCRCitations_NoWrongNumberForTopic_R94: that
// gate is a fixed list of known-bad (number, topic) pairs. It can only ever
// catch a mistake someone has already made somewhere else. This one derives
// the right answer from the CR.
//
// r121 fixed 36 of these but swept only citations beginning "701.", so a §701
// keyword action cited as a §702 rule stayed invisible: `adapt` carried
// §702.137 (Spectacle), `monstrosity` §702.94 (Miracle), `exert` §702.112
// (Renown). Keying on the Kind in BOTH directions is what found them.

// r122Legit — events whose Kind is a generic action but whose citation
// correctly names the specific ability that caused it. Each carries a
// "reason" field naming that ability, and each was read against the CR.
var r122Legit = map[string]string{
	// "partner with [name]" fetches and reveals the paired commander.
	"internal/gameengine/keywords_partner.go|reveal|702.124j": "§702.124j is the partner-with ability that performs this reveal; Kind is the generic action.",
	// Typecycling searches, reveals, and puts into hand.
	"internal/gameengine/keywords_p0.go|reveal|702.29e": "§702.29e is the typecycling ability that performs this reveal; Kind is the generic action.",
}

var (
	r122KindRE = regexp.MustCompile(`Kind:\s*"([^"]+)"`)
	r122RuleRE = regexp.MustCompile(`"rule"\s*:\s*"(70[12]\.\d+)([a-z]?)"`)
)

func TestCRCitations_EventRuleNamesItsAction_R122(t *testing.T) {
	root := repoRootForR94(t)
	c, err := LoadCorpus(filepath.Join(root, "data", "rules", "MagicCompRules-20260819.txt"))
	if err != nil {
		t.Fatalf("load CR: %v", err)
	}

	// action name -> rule number, from the CR's own §701/§702 headings.
	byAction := map[string]string{}
	for _, sec := range []string{"701", "702"} {
		for i := 1; i < 250; i++ {
			rule := sec + "." + itoa(i)
			head, ok := c.Heading(rule)
			if !ok {
				continue
			}
			head = strings.ToLower(strings.TrimSpace(head))
			if head == "" || strings.Contains(head, " ") {
				continue // multi-word headings are matched by Kind exactly below
			}
			byAction[head] = rule
		}
	}

	var bad []string
	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") ||
			strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		lines := strings.Split(string(b), "\n")
		for i, line := range lines {
			m := r122RuleRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			kind := ""
			for j := i; j >= 0 && j > i-45; j-- {
				if k := r122KindRE.FindStringSubmatch(lines[j]); k != nil {
					kind = k[1]
					break
				}
			}
			want, ok := byAction[strings.ToLower(kind)]
			if !ok || want == m[1] {
				continue
			}
			if _, allowed := r122Legit[rel+"|"+kind+"|"+m[1]+m[2]]; allowed {
				continue
			}
			owner, _ := c.Heading(m[1])
			bad = append(bad, rel+":"+itoa(i+1)+" the "+kind+" event cites §"+m[1]+m[2]+
				" ("+owner+") — "+kind+" is §"+want)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for _, b := range bad {
		t.Errorf("%s", b)
	}
	t.Logf("r122 event-rule/action join: %d actions known from the CR, %d violations",
		len(byAction), len(bad))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
