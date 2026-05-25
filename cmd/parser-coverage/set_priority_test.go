package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestDeck creates a deck file in dir with the given lines.
// Returns the absolute path.
func writeTestDeck(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDeckLine_Shapes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"# comment", ""},
		{"// also comment", ""},
		{"COMMANDER: Edgar Markov", "Edgar Markov"},
		{"COMMANDER:Edgar Markov", "Edgar Markov"},
		{"1 Sol Ring", "Sol Ring"},
		{"4 Lightning Bolt", "Lightning Bolt"},
		{"4x Lightning Bolt", "Lightning Bolt"},
		{"  2 Forest  ", "Forest"},
		{"Junk no count", ""},
		{"abc Lightning Bolt", ""}, // non-numeric prefix
	}
	for _, c := range cases {
		if got := parseDeckLine(c.in); got != c.want {
			t.Errorf("parseDeckLine(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

func TestScanDeckFrequencies_CountsAcrossDecks(t *testing.T) {
	dir := t.TempDir()
	writeTestDeck(t, dir, "a.txt",
		"COMMANDER: Edgar Markov",
		"1 Sol Ring",
		"1 Lightning Bolt",
	)
	writeTestDeck(t, dir, "b.txt",
		"COMMANDER: Atraxa",
		"1 Sol Ring",
	)
	writeTestDeck(t, dir, "c.txt",
		"1 Sol Ring",
		"1 Lightning Bolt",
		"4 Forest",
	)
	freq, count, err := scanDeckFrequencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("deck count: want 3, got %d", count)
	}
	if freq["Sol Ring"] != 3 {
		t.Errorf("Sol Ring: want 3, got %d", freq["Sol Ring"])
	}
	if freq["Lightning Bolt"] != 2 {
		t.Errorf("Lightning Bolt: want 2, got %d", freq["Lightning Bolt"])
	}
	if freq["Forest"] != 1 {
		t.Errorf("Forest: want 1, got %d", freq["Forest"])
	}
	if freq["Edgar Markov"] != 1 {
		t.Errorf("commander Edgar Markov: want 1, got %d", freq["Edgar Markov"])
	}
}

func TestScanDeckFrequencies_DedupesWithinADeck(t *testing.T) {
	// A pathological deck listing the same card twice (rare but
	// possible in hand-written .txt) shouldn't inflate frequency.
	dir := t.TempDir()
	writeTestDeck(t, dir, "weird.txt",
		"1 Sol Ring",
		"1 Sol Ring", // same card again
		"2 Forest",
		"2 Forest", // and again
	)
	freq, _, err := scanDeckFrequencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	if freq["Sol Ring"] != 1 {
		t.Errorf("intra-deck dedup: Sol Ring should count 1, got %d", freq["Sol Ring"])
	}
	if freq["Forest"] != 1 {
		t.Errorf("intra-deck dedup: Forest should count 1, got %d", freq["Forest"])
	}
}

func TestScanDeckFrequencies_RecursesIntoSubdirs(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "moxfield")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestDeck(t, dir, "top.txt", "1 Card A")
	writeTestDeck(t, subdir, "sub.txt", "1 Card A", "1 Card B")
	freq, count, err := scanDeckFrequencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("want 2 decks (top + sub), got %d", count)
	}
	if freq["Card A"] != 2 {
		t.Errorf("Card A appears in both: want 2, got %d", freq["Card A"])
	}
}

func TestScanDeckFrequencies_IgnoresNonTxtFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestDeck(t, dir, "deck.txt", "1 Real Card")
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("1 Fake Card"), 0o644); err != nil {
		t.Fatal(err)
	}
	freq, count, err := scanDeckFrequencies(dir)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("want 1 deck (txt only), got %d", count)
	}
	if _, ok := freq["Fake Card"]; ok {
		t.Error(".md file should not be scanned")
	}
}

func TestScanDeckFrequencies_MissingDirReturnsEmpty(t *testing.T) {
	freq, count, err := scanDeckFrequencies("/this/path/does/not/exist/either")
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if count != 0 {
		t.Errorf("missing dir: want 0 decks, got %d", count)
	}
	if len(freq) != 0 {
		t.Errorf("missing dir: want empty freq map, got %v", freq)
	}
}

func TestComputeSetPriority_WeightsByDeckFreq(t *testing.T) {
	// Set A has 2 uncovered cards: one in 100 decks, one in 50 → score 150.
	// Set B has 5 uncovered cards each in 0 decks → score 0.
	// Set C has 1 uncovered card in 75 decks → score 75.
	// Expected order: A (150), C (75), B (0) — and the writer should
	// drop B from the report.
	results := []result{
		{Name: "A1", Class: classMissing, SetName: "Alpha"},
		{Name: "A2", Class: classEmptyAST, SetName: "Alpha"},
		{Name: "B1", Class: classMissing, SetName: "Beta"},
		{Name: "B2", Class: classMissing, SetName: "Beta"},
		{Name: "B3", Class: classMissing, SetName: "Beta"},
		{Name: "B4", Class: classMissing, SetName: "Beta"},
		{Name: "B5", Class: classMissing, SetName: "Beta"},
		{Name: "C1", Class: classPartial, SetName: "Charlie"},
	}
	freq := map[string]int{"A1": 100, "A2": 50, "C1": 75}
	got := computeSetPriority(results, freq, 5)
	if len(got) != 3 {
		t.Fatalf("want 3 SetPriority entries, got %d", len(got))
	}
	if got[0].SetName != "Alpha" || got[0].Score != 150 {
		t.Errorf("rank 1: want Alpha/150, got %+v", got[0])
	}
	if got[1].SetName != "Charlie" || got[1].Score != 75 {
		t.Errorf("rank 2: want Charlie/75, got %+v", got[1])
	}
	if got[2].SetName != "Beta" || got[2].Score != 0 {
		t.Errorf("rank 3: want Beta/0 (kept for caller to filter), got %+v", got[2])
	}
	if got[2].UncoveredCount != 5 {
		t.Errorf("Beta uncovered count: want 5, got %d", got[2].UncoveredCount)
	}
}

func TestComputeSetPriority_OmitsCoveredResults(t *testing.T) {
	// OK and OK_VANILLA results must not contribute to any set's
	// score — they're not scaffold targets.
	results := []result{
		{Name: "A1", Class: classOK, SetName: "Alpha"},
		{Name: "A2", Class: classOKVanilla, SetName: "Alpha"},
		{Name: "B1", Class: classMissing, SetName: "Beta"},
	}
	freq := map[string]int{"A1": 100, "A2": 100, "B1": 1}
	got := computeSetPriority(results, freq, 5)
	if len(got) != 1 {
		t.Fatalf("OK rows should not produce a SetPriority entry, got %d entries", len(got))
	}
	if got[0].SetName != "Beta" {
		t.Errorf("only uncovered Beta should appear, got %q", got[0].SetName)
	}
}

func TestComputeSetPriority_TopCardsSortedByFrequencyDescending(t *testing.T) {
	results := []result{
		{Name: "low", Class: classMissing, SetName: "S"},
		{Name: "high", Class: classMissing, SetName: "S"},
		{Name: "mid", Class: classMissing, SetName: "S"},
	}
	freq := map[string]int{"low": 1, "high": 100, "mid": 50}
	got := computeSetPriority(results, freq, 5)
	if len(got) != 1 {
		t.Fatalf("expected 1 set, got %d", len(got))
	}
	names := make([]string, len(got[0].TopCards))
	for i, c := range got[0].TopCards {
		names[i] = c.Name
	}
	want := []string{"high", "mid", "low"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("top cards order: want %v, got %v", want, names)
			break
		}
	}
}

func TestComputeSetPriority_TopNCapsCardList(t *testing.T) {
	results := []result{
		{Name: "a", Class: classMissing, SetName: "S"},
		{Name: "b", Class: classMissing, SetName: "S"},
		{Name: "c", Class: classMissing, SetName: "S"},
		{Name: "d", Class: classMissing, SetName: "S"},
	}
	freq := map[string]int{"a": 4, "b": 3, "c": 2, "d": 1}
	got := computeSetPriority(results, freq, 2)
	if len(got[0].TopCards) != 2 {
		t.Errorf("topN=2 should cap card list at 2, got %d", len(got[0].TopCards))
	}
	if got[0].TopCards[0].Name != "a" || got[0].TopCards[1].Name != "b" {
		t.Errorf("topN cap should keep the highest-freq cards: got %v", got[0].TopCards)
	}
}

func TestComputeSetPriority_EmptySetNameBucketsAsUnknown(t *testing.T) {
	results := []result{
		{Name: "x", Class: classMissing, SetName: ""},
		{Name: "y", Class: classEmptyAST, SetName: ""},
	}
	freq := map[string]int{"x": 5, "y": 3}
	got := computeSetPriority(results, freq, 5)
	if len(got) != 1 || got[0].SetName != "(unknown)" {
		t.Fatalf("empty set name should bucket as '(unknown)', got %+v", got)
	}
	if got[0].Score != 8 {
		t.Errorf("score: want 8, got %d", got[0].Score)
	}
}

func TestComputeSetPriority_ScoreTiebreakByName(t *testing.T) {
	// Two sets with identical scores → alphabetic ascending order.
	results := []result{
		{Name: "x", Class: classMissing, SetName: "Beta"},
		{Name: "y", Class: classMissing, SetName: "Alpha"},
	}
	freq := map[string]int{"x": 10, "y": 10}
	got := computeSetPriority(results, freq, 5)
	if got[0].SetName != "Alpha" || got[1].SetName != "Beta" {
		t.Errorf("tied scores should sort by name asc: got %v / %v", got[0].SetName, got[1].SetName)
	}
}

func TestWriteSetPriorityReport_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	priorities := []SetPriority{
		{SetName: "Alpha", Score: 150, UncoveredCount: 2, TopCards: []UncoveredCardEntry{{Name: "A1", Class: "MISSING", Frequency: 100}, {Name: "A2", Class: "EMPTY_AST", Frequency: 50}}},
		{SetName: "Beta", Score: 0, UncoveredCount: 5},
	}
	if err := writeSetPriorityReport(path, priorities, 1620, 0); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	for _, want := range []string{
		"# Parser Coverage — Set Priority (deck-weighted)",
		"| Decks scanned | 1620 |",
		"| Sets with at least one deck-relevant uncovered card | 1 |",
		"| Rank | Set | Score | Uncovered cards | Top cards",
		"| 1 | Alpha | 150 | 2 |",
		"A1 (100 · MISSING)",
		"A2 (50 · EMPTY_AST)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n---\n%s", want, out)
		}
	}
	// Beta has score 0 — it must NOT appear in the table.
	if strings.Contains(out, "| Beta |") {
		t.Error("zero-score Beta should be filtered from the report")
	}
}

func TestWriteSetPriorityReport_LimitTruncatesTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	priorities := []SetPriority{
		{SetName: "A", Score: 100, UncoveredCount: 1},
		{SetName: "B", Score: 50, UncoveredCount: 1},
		{SetName: "C", Score: 25, UncoveredCount: 1},
	}
	if err := writeSetPriorityReport(path, priorities, 0, 2); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "| 1 | A |") || !strings.Contains(out, "| 2 | B |") {
		t.Errorf("limit=2 should keep top 2 rows: \n%s", out)
	}
	if strings.Contains(out, "| 3 | C |") || strings.Contains(out, "| C |") {
		t.Error("limit=2 should drop the 3rd row")
	}
	if !strings.Contains(out, "Sets shown (top-N)") {
		t.Error("truncated report should call out top-N")
	}
}

func TestWriteSetPriorityReport_AllZeroScoreYieldsPlaceholder(t *testing.T) {
	// Pathological case: every set has score 0 (e.g., no decks
	// scanned). The report must still write a file with a placeholder
	// rather than an empty table.
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	priorities := []SetPriority{
		{SetName: "A", Score: 0, UncoveredCount: 1},
	}
	if err := writeSetPriorityReport(path, priorities, 0, 0); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := os.ReadFile(path)
	out := string(got)
	if !strings.Contains(out, "No uncovered cards intersect any scanned deck") {
		t.Errorf("all-zero case should emit explanatory placeholder, got:\n%s", out)
	}
}

func TestWriteSetPriorityReport_BadPathErrors(t *testing.T) {
	err := writeSetPriorityReport("/this/path/does/not/exist/out.md", nil, 0, 0)
	if err == nil {
		t.Fatal("expected error for unwritable path, got nil")
	}
}
