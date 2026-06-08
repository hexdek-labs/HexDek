package hexapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// find_deck_files_test.go — pins that findDeckFiles ingests both .txt and
// .json decks (so the engine pool matches the deck-list/read endpoints),
// while skipping Freya/import sidecars and the version-history dir.

func TestFindDeckFiles_IncludesTxtAndJSON_SkipsSidecars(t *testing.T) {
	root := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// decks (should be found)
	write("josh/cleohaus_tergrid.json")
	write("josh/tergrid_cleo_b4.txt")
	write("hex/ninjas.txt")
	// sidecars + skipped dirs (should NOT be found)
	write("josh/cleohaus_tergrid.strategy.json")
	write("josh/cleohaus_tergrid.profile.json")
	write("josh/freya/cleohaus_tergrid.strategy.json")
	write("josh/.versions/cleohaus_tergrid.json")
	write("benched/old.txt")
	write("test/fixture.txt")

	paths, err := findDeckFiles(root)
	if err != nil {
		t.Fatalf("findDeckFiles: %v", err)
	}
	got := map[string]bool{}
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		got[filepath.ToSlash(rel)] = true
	}

	wantFound := []string{"josh/cleohaus_tergrid.json", "josh/tergrid_cleo_b4.txt", "hex/ninjas.txt"}
	for _, w := range wantFound {
		if !got[w] {
			t.Errorf("expected %q in pool, missing (got %v)", w, keys(got))
		}
	}
	wantSkipped := []string{
		"josh/cleohaus_tergrid.strategy.json",
		"josh/cleohaus_tergrid.profile.json",
		"josh/freya/cleohaus_tergrid.strategy.json",
		"josh/.versions/cleohaus_tergrid.json",
		"benched/old.txt",
		"test/fixture.txt",
	}
	for _, w := range wantSkipped {
		if got[w] {
			t.Errorf("expected %q to be skipped, but it was included", w)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return strings.Split(strings.Join(out, ","), ",")
}
