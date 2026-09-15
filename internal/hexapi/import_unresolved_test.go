package hexapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

func astDatasetPathForTest() string {
	for _, p := range []string{
		"../../data/rules/ast_dataset.jsonl",
		"../../../data/rules/ast_dataset.jsonl",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// A deck import that silently loses cards is the bug this exists to stop.
// The failure that would be WORSE than the original bug is reporting cards
// as unresolved when we simply could not check — a user would cut real
// cards from a real deck on our say-so. So the unloaded-pool path is
// tested first and hardest.
func TestUnresolvedImportFields_NoPoolReportsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(path, []byte("1 Sol Ring\n1 Definitely Not A Real Card\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nil Showmatch: no pool at all.
	if got := unresolvedImportFields(nil, path); got != nil {
		t.Errorf("nil Showmatch must report nothing, got %v", got)
	}
	// Non-nil Showmatch whose pool has never loaded (corpus/meta nil).
	if got := unresolvedImportFields(&Showmatch{}, path); got != nil {
		t.Errorf("unloaded pool must report nothing, got %v", got)
	}
}

func TestUnresolvedImportFields_ReportsDroppedCards(t *testing.T) {
	ast := astDatasetPathForTest()
	if ast == "" {
		t.Skip("no AST dataset available")
	}
	meta, err := deckparser.LoadMetaFromJSONL(ast)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	sm := &Showmatch{meta: meta}

	dir := t.TempDir()
	path := filepath.Join(dir, "d.txt")
	deck := "1 Sol Ring\n1 Zzzqqx The Nonexistent Card\n1 Arcane Signet\n"
	if err := os.WriteFile(path, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}

	got := unresolvedImportFields(sm, path)
	if got == nil {
		t.Fatal("expected the bogus card to be reported, got nothing")
	}
	names, _ := got["unresolved"].([]string)
	found := false
	for _, n := range names {
		if n == "Zzzqqx The Nonexistent Card" {
			found = true
		}
		if n == "Sol Ring" || n == "Arcane Signet" {
			t.Errorf("real card %q reported as unresolved — false positive", n)
		}
	}
	if !found {
		t.Errorf("bogus card missing from %v", names)
	}
	if c, _ := got["unresolved_count"].(int); c != len(names) {
		t.Errorf("unresolved_count %d != len(unresolved) %d", c, len(names))
	}
}

// A deck where everything resolves must produce no fields at all, so the
// client can treat presence of the key as "something was dropped".
func TestUnresolvedImportFields_CleanDeckIsSilent(t *testing.T) {
	ast := astDatasetPathForTest()
	if ast == "" {
		t.Skip("no AST dataset available")
	}
	meta, err := deckparser.LoadMetaFromJSONL(ast)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(path, []byte("1 Sol Ring\n1 Arcane Signet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := unresolvedImportFields(&Showmatch{meta: meta}, path); got != nil {
		t.Errorf("fully-resolving deck must report nothing, got %v", got)
	}
}
