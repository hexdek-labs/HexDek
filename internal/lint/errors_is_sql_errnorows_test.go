// Package lint hosts CI-time lint tests that enforce code-style
// invariants beyond what `go vet` and `staticcheck` cover natively.
package lint

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestErrorsIsCatchesWrappedSqlErrNoRows is the "why this migration
// matters" pinning test: a wrapped sql.ErrNoRows that the old `==`
// form would have MISSED is correctly identified by errors.Is. If this
// test ever fails, the standard library has changed in a way that
// invalidates the migration premise and every caller in the tree
// needs to be re-audited.
func TestErrorsIsCatchesWrappedSqlErrNoRows(t *testing.T) {
	wrapped := fmt.Errorf("scan row: %w", sql.ErrNoRows)

	// The form the migration replaced — silently MISSES the wrap.
	if wrapped == sql.ErrNoRows { //nolint:errorlint
		t.Fatal("direct == should NOT match a wrapped error — this proves the bug existed pre-migration")
	}
	// The form the migration uses — CORRECTLY matches.
	if !errors.Is(wrapped, sql.ErrNoRows) {
		t.Fatal("errors.Is must match a wrapped sql.ErrNoRows; the migration's whole premise depends on it")
	}
}

// TestNoErrorEqualsSqlErrNoRows enforces that no non-test .go file in
// the repository uses `err == sql.ErrNoRows` or `err != sql.ErrNoRows`.
// Use `errors.Is(err, sql.ErrNoRows)` instead — the wrapped-error case
// (a `fmt.Errorf("...: %w", sql.ErrNoRows)` returned through a helper)
// is the entire reason errors.Is exists, and the engine pre-r48 cluster
// of "not found" branches that silently broke when callers wrapped is
// exactly what this lint protects against.
//
// Tracked in `half-finished-features-r48 #9 M2`: pre-r60 audit found
// the pattern at 6 call sites that had been migrated incompletely.
// This guard makes recurrence a test failure rather than a Code-review
// catch.
func TestNoErrorEqualsSqlErrNoRows(t *testing.T) {
	// Walk from the repo root. Tests live in `internal/lint/` two
	// levels deep, so back out to the module root before scanning.
	root := repoRoot(t)

	// Pattern: `err == sql.ErrNoRows` OR `err != sql.ErrNoRows`, allowing
	// arbitrary whitespace around the operator. Matches both `==` and `!=`
	// so the same lint catches both inverted and direct forms.
	pattern := regexp.MustCompile(`\berr\s*[!=]=\s*sql\.ErrNoRows\b`)

	type hit struct {
		path string
		line int
		text string
	}
	var hits []hit

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Prune common irrelevant trees.
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == ".claude" || name == "hexdek" /* frontend */ {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files: tests may want to assert the literal-equality
		// behavior of sql.ErrNoRows itself.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(body), "\n") {
			if pattern.MatchString(line) {
				hits = append(hits, hit{path: path, line: i + 1, text: strings.TrimSpace(line)})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(hits) > 0 {
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf(
			"found %d site(s) using `err == sql.ErrNoRows` / `err != sql.ErrNoRows`; "+
				"migrate to `errors.Is(err, sql.ErrNoRows)` so wrapped errors are handled:\n", len(hits)))
		for _, h := range hits {
			msg.WriteString(fmt.Sprintf("  %s:%d   %s\n", h.path, h.line, h.text))
		}
		t.Error(msg.String())
	}
}

// TestNoErrorEqualsSqlErrNoRows_RegexSanity asserts the regex used by
// the tree-walking guard catches every spelling we care about and
// rejects the canonical errors.Is form so future maintainers can be
// confident the lint is doing what its name says.
func TestNoErrorEqualsSqlErrNoRows_RegexSanity(t *testing.T) {
	pattern := regexp.MustCompile(`\berr\s*[!=]=\s*sql\.ErrNoRows\b`)

	mustMatch := []string{
		`if err == sql.ErrNoRows {`,
		`if err != sql.ErrNoRows {`,
		`	if err  ==  sql.ErrNoRows {`, // odd whitespace
		`return err == sql.ErrNoRows`,
		`for err != sql.ErrNoRows {`,
	}
	for _, line := range mustMatch {
		if !pattern.MatchString(line) {
			t.Errorf("regex should match %q", line)
		}
	}

	mustNotMatch := []string{
		`if errors.Is(err, sql.ErrNoRows) {`,
		`// err == sql.ErrNoRowsMaybe`,       // suffix differs (word boundary)
		`if myErr == sql.ErrNoRows {`,         // identifier prefix differs (word boundary)
		`if err == sql.ErrConnDone {`,         // different sentinel
	}
	for _, line := range mustNotMatch {
		if pattern.MatchString(line) {
			t.Errorf("regex should NOT match %q", line)
		}
	}
}

// repoRoot returns the absolute path to the module root by walking up
// from the test's working directory until a `go.mod` is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", dir)
		}
		dir = parent
	}
}
