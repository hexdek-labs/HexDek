package crcite

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

const inlineBaseline = "inline_baseline.txt"

// Key identifies an inline mismatch for the baseline.
func inlineKey(m InlineMismatch) string {
	return fmt.Sprintf("%s:%d|%s|%s", m.File, m.Line, m.Rule, m.Named)
}

// TestCRCitations_InlineIndexEntries catches the class the filename gate
// structurally cannot see.
//
// FindMismatches keys on the FILE NAME, so it only works where the file is
// named after the keyword it is about. It found none of the errors in
// keywords_stubs_tail.go, keywords_batch*.go or keywords_misc.go — catch-all
// files that implement many keywords and declare them in index-entry
// comments. That is a large share of the keyword surface.
//
// This gate reads the citing LINE instead. Where a line is an index entry
// naming exactly one keyword next to a rule number, the keyword's real rule
// is knowable and the citation can be checked against it:
//
//	zone_cast.go:7   //   §702.33   Flashback     (702.33 is Kicker)
//	zone_cast.go:16  //   §702.34   Madness       (702.34 is Flashback)
//
// Two consecutive lines, each one row off — an index transcribed out of step.
//
// Unlike the topic-mismatch ratchet, these have no legitimate form: the line
// names the keyword, and the CR gives exactly one number for that keyword.
// The baseline exists only so the existing set can be fixed deliberately
// rather than in one unreviewed sweep. It only shrinks.
func TestCRCitations_InlineIndexEntries(t *testing.T) {
	c, cites, root := loadTree(t)
	found := FindInlineMismatches(c, cites, root)

	if os.Getenv("CRCITE_UPDATE_INLINE") != "" {
		var b strings.Builder
		b.WriteString("# Index-entry comments filing a keyword under another keyword's rule number.\n")
		b.WriteString("# Every entry is wrong: the line names the keyword, and the CR gives exactly\n")
		b.WriteString("# one number for it. This file only ever SHRINKS — fix a line, delete it.\n")
		for _, m := range found {
			fmt.Fprintf(&b, "%s\t# says %q, cites §%s (%s), correct is §%s\n",
				inlineKey(m), m.Named, m.Rule, m.Cited, m.Correct)
		}
		if err := os.WriteFile(inlineBaseline, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s with %d entries", inlineBaseline, len(found))
		return
	}

	raw, err := os.ReadFile(inlineBaseline)
	if err != nil {
		t.Fatalf("read inline baseline: %v", err)
	}
	base := map[string]bool{}
	for _, ln := range strings.Split(string(raw), "\n") {
		if i := strings.IndexAny(ln, "\t#"); i >= 0 {
			ln = ln[:i]
		}
		if ln = strings.TrimSpace(ln); ln != "" {
			base[ln] = true
		}
	}

	live := map[string]bool{}
	for _, m := range found {
		k := inlineKey(m)
		live[k] = true
		if !base[k] {
			t.Errorf("NEW mis-filed index entry: %s:%d cites §%s (%s) on a line that says %q.\n"+
				"    %q\n"+
				"    %s is §%s. Cite that, or name the keyword the number belongs to.",
				m.File, m.Line, m.Rule, m.Cited, m.Named, m.Source, m.Named, m.Correct)
		}
	}
	var stale []string
	for k := range base {
		if !live[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("stale inline baseline entry %q — fixed; delete the line from %s.", k, inlineBaseline)
	}
	t.Logf("inline index entries: %d live, %d baselined", len(found), len(base))
}
