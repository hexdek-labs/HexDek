package crcite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const textLockFile = "text_lock.txt"

// TestCRCitations_TextLock is the gate that existence checking cannot be.
//
// The August 2026 CR inserts a new §704.5w without renumbering anything.
// Every letter that existed still exists — but from w onward each one now
// means what the PREVIOUS letter used to mean. Feb's §704.5y ("a permanent
// has more than one Role") is Aug's §704.5z; Aug's §704.5y is a different
// rule about battle protectors.
//
// So on an edition bump, a citation to §704.5y stays green under the
// dangling gate and silently starts pointing at another rule. This lock
// pins the TEXT of every rule the tree cites. Change the CR file and any
// cited rule whose text moved fails here, naming the citations to re-read.
// The edition bump and the citation sweep therefore cannot land apart.
func TestCRCitations_TextLock(t *testing.T) {
	c, cites, root := loadTree(t)
	rules := CitedRules(c, cites)

	if os.Getenv("CRCITE_UPDATE_LOCK") != "" {
		var b strings.Builder
		fmt.Fprintf(&b, "# Text fingerprints of every CR rule this tree cites.\n")
		fmt.Fprintf(&b, "# Corpus: %s\n", c.Effective)
		fmt.Fprintf(&b, "# A failure here means a cited rule's TEXT changed. That is not a\n")
		fmt.Fprintf(&b, "# merge conflict to resolve — it means re-read every citation of that\n")
		fmt.Fprintf(&b, "# rule, because the number may now point somewhere else entirely.\n")
		fmt.Fprintf(&b, "# Regenerate ONLY as part of a deliberate CR edition adoption:\n")
		fmt.Fprintf(&b, "#   CRCITE_UPDATE_LOCK=1 go test ./internal/crcite/ -run TextLock -count=1\n")
		for _, r := range rules {
			txt, _ := c.Text(r)
			h, _ := c.Heading(r)
			fmt.Fprintf(&b, "%s\t%s\t%s\n", r, Digest(txt), h)
		}
		if err := os.WriteFile(textLockFile, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s with %d cited rules", textLockFile, len(rules))
		return
	}

	raw, err := os.ReadFile(textLockFile)
	if err != nil {
		t.Fatalf("read text lock: %v", err)
	}
	locked := map[string]string{}
	for _, ln := range strings.Split(string(raw), "\n") {
		if ln = strings.TrimSpace(ln); ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		f := strings.Split(ln, "\t")
		if len(f) >= 2 {
			locked[f[0]] = f[1]
		}
	}

	// Where each rule is cited, so a failure points at real work.
	sites := map[string][]Citation{}
	for _, ct := range cites {
		sites[ct.Rule] = append(sites[ct.Rule], ct)
	}

	var drifted, unlocked []string
	for _, r := range rules {
		txt, _ := c.Text(r)
		switch want, ok := locked[r]; {
		case !ok:
			unlocked = append(unlocked, r)
		case want != Digest(txt):
			drifted = append(drifted, r)
		}
	}
	sort.Strings(drifted)

	for _, r := range drifted {
		txt, _ := c.Text(r)
		if len(txt) > 120 {
			txt = txt[:120] + "..."
		}
		t.Errorf("§%s text CHANGED under %d citation(s). It now reads %q.\n"+
			"    Do NOT just refresh the lock. The number surviving an edition bump does\n"+
			"    not mean it still says what the code claims — re-read every site below.",
			r, len(sites[r]), txt)
		for i, ct := range sites[r] {
			if i == 5 {
				t.Errorf("      ... and %d more", len(sites[r])-5)
				break
			}
			rel, _ := filepath.Rel(root, ct.File)
			t.Errorf("      %s:%d", filepath.ToSlash(rel), ct.Line)
		}
	}
	if len(unlocked) > 0 {
		t.Errorf("%d cited rule(s) absent from %s (new citations): %s\n"+
			"    Add them: CRCITE_UPDATE_LOCK=1 go test ./internal/crcite/ -run TextLock",
			len(unlocked), textLockFile, strings.Join(unlocked, " "))
	}
	t.Logf("text lock: %d cited rules pinned, %d drifted", len(rules), len(drifted))
}
