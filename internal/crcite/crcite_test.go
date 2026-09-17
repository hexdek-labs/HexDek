package crcite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot walks up from this package to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate module root")
	return ""
}

// crPath is the Comprehensive Rules edition the tree is validated against.
// Bumping this file is a deliberate act: a new edition can renumber whole
// sections (the Aug-2026 edition inserted §722 "Preparation Cards", shifting
// everything after it), so the citation sweep must land with it.
const crPath = "data/rules/MagicCompRules-20260819.txt"

func loadTree(t *testing.T) (*Corpus, []Citation, string) {
	t.Helper()
	root := repoRoot(t)
	c, err := LoadCorpus(filepath.Join(root, crPath))
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	var cites []Citation
	for _, sub := range []string{"internal", "cmd"} {
		found, err := Scan(filepath.Join(root, sub))
		if err != nil {
			t.Fatalf("scan %s: %v", sub, err)
		}
		cites = append(cites, found...)
	}
	return c, cites, root
}

// exempt lists the very few places where a rule-shaped string is deliberately
// NOT a citation. Each entry needs a reason, because this is the one hole in
// the hard gate and it must stay small enough to read at a glance.
var exempt = map[string]string{
	"cmd/hexdek-judge/explain_test.go|999.99z":           "negative test: asserts Judge reports 'no index entry' for a rule that does not exist",
	"internal/gameengine/keywords_stubs_tail.go|702.17x": "range notation in a file comment (\u00a7702.17x-\u00a7702.19x), not a citation",
	"internal/gameengine/keywords_stubs_tail.go|702.19x": "range notation in a file comment (\u00a7702.17x-\u00a7702.19x), not a citation",
}

// TestCRCitations_NoDanglingRules is the hard gate: a citation naming a rule
// that does not exist in the Comprehensive Rules was never correct. It cannot
// have drifted — it was written to look right. There is no baseline and no
// grandfathering for this class.
func TestCRCitations_NoDanglingRules(t *testing.T) {
	c, cites, root := loadTree(t)

	t.Logf("corpus: %s (%d rules)", c.Effective, c.RuleCount())
	t.Logf("citations scanned: %d", len(cites))

	bad := map[string][]Citation{}
	for _, ct := range cites {
		if c.Exists(ct.Rule) {
			continue
		}
		rel, err := filepath.Rel(root, ct.File)
		if err != nil {
			rel = ct.File
		}
		if exempt[filepath.ToSlash(rel)+"|"+ct.Rule] != "" {
			continue
		}
		bad[ct.Rule] = append(bad[ct.Rule], ct)
	}
	if len(bad) == 0 {
		return
	}
	rules := make([]string, 0, len(bad))
	for r := range bad {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool { return len(bad[rules[i]]) > len(bad[rules[j]]) })

	total := 0
	for _, r := range rules {
		total += len(bad[r])
	}
	t.Errorf("%d citations name %d rule numbers that do not exist in the Comprehensive Rules.\n"+
		"A rule number that exists in no edition was never correct — it was written to look\n"+
		"correct. Fix it or remove the number and describe the rule in words.", total, len(rules))
	for _, r := range rules {
		locs := bad[r]
		ex := locs[0]
		rel, _ := filepath.Rel(root, ex.File)
		t.Errorf("  §%s — %d citation(s), e.g. %s:%d", r, len(locs), rel, ex.Line)
	}
}

const mismatchBaseline = "mismatch_baseline.txt"

func loadBaseline(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(mismatchBaseline)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	set := map[string]bool{}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if i := strings.IndexAny(ln, "\t#"); i >= 0 {
			ln = strings.TrimSpace(ln[:i])
		}
		if ln == "" {
			continue
		}
		set[ln] = true
	}
	return set
}

// TestCRCitations_TopicMismatchRatchet is the soft gate. Unlike a dangling
// rule number, a topic mismatch can be legitimate — keywords_disguise.go
// citing §702.37 Morph is correct, because disguise is defined in terms of
// morph. So today's known-bad set is recorded in mismatch_baseline.txt and
// this test fails on anything NEW. The baseline only ever shrinks: when a
// citation is corrected, delete its line.
func TestCRCitations_TopicMismatchRatchet(t *testing.T) {
	c, cites, root := loadTree(t)
	found := FindMismatches(c, cites, root)
	if os.Getenv("CRCITE_UPDATE_BASELINE") != "" {
		var b strings.Builder
		b.WriteString("# Known CR citation topic mismatches, as of the run that wrote this file.\n")
		b.WriteString("# A ratchet: this file only ever SHRINKS. Fix a citation, delete its line.\n")
		b.WriteString("# Regenerate (only when consciously accepting new entries):\n")
		b.WriteString("#   CRCITE_UPDATE_BASELINE=1 go test ./internal/crcite/ -run Ratchet -count=1\n")
		for _, m := range found {
			fmt.Fprintf(&b, "%s\t# cites %s, file is %s (%s)\n", m.Key(), m.Heading, m.FileKW, m.FileRule)
		}
		if err := os.WriteFile(mismatchBaseline, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s with %d entries", mismatchBaseline, len(found))
		return
	}
	base := loadBaseline(t)

	live := map[string]bool{}
	var added []Mismatch
	for _, m := range found {
		live[m.Key()] = true
		if !base[m.Key()] {
			added = append(added, m)
		}
	}
	for _, m := range added {
		t.Errorf("NEW topic mismatch: %s cites §%s (%s) but the file is %s (§%s).\n"+
			"    Cite the rule that says what you implemented, or fix the number.\n"+
			"    If the cross-reference is deliberate, add %q to %s with a reason.",
			m.File, m.Rule, m.Heading, m.FileKW, m.FileRule, m.Key(), mismatchBaseline)
	}

	var stale []string
	for k := range base {
		if !live[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("stale baseline entry: %q no longer mismatches — delete the line from %s.\n"+
			"    The baseline is a ratchet; it only shrinks.", k, mismatchBaseline)
	}
	t.Logf("topic mismatches: %d live, %d baselined", len(found), len(base))
}
