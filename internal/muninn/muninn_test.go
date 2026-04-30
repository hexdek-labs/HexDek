package muninn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
)

func TestPersistParserGaps_NewFile(t *testing.T) {
	dir := t.TempDir()
	gaps := map[string]int{
		"Whenever this creature deals combat damage...": 5,
		"At the beginning of your upkeep...":            3,
	}

	if err := PersistParserGaps(dir, gaps); err != nil {
		t.Fatalf("PersistParserGaps: %v", err)
	}

	got, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(got))
	}

	bySnippet := map[string]ParserGap{}
	for _, g := range got {
		bySnippet[g.Snippet] = g
	}

	if g, ok := bySnippet["Whenever this creature deals combat damage..."]; !ok || g.Count != 5 {
		t.Errorf("combat damage gap: got count=%d, want 5", g.Count)
	}
	if g, ok := bySnippet["At the beginning of your upkeep..."]; !ok || g.Count != 3 {
		t.Errorf("upkeep gap: got count=%d, want 3", g.Count)
	}
}

func TestPersistParserGaps_Merge(t *testing.T) {
	dir := t.TempDir()

	// First write.
	if err := PersistParserGaps(dir, map[string]int{"foo": 10}); err != nil {
		t.Fatalf("first persist: %v", err)
	}

	// Second write with same + new snippet.
	if err := PersistParserGaps(dir, map[string]int{"foo": 5, "bar": 3}); err != nil {
		t.Fatalf("second persist: %v", err)
	}

	got, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}

	bySnippet := map[string]ParserGap{}
	for _, g := range got {
		bySnippet[g.Snippet] = g
	}

	if bySnippet["foo"].Count != 15 {
		t.Errorf("foo count: got %d, want 15", bySnippet["foo"].Count)
	}
	if bySnippet["bar"].Count != 3 {
		t.Errorf("bar count: got %d, want 3", bySnippet["bar"].Count)
	}
}

func TestPersistParserGaps_Empty(t *testing.T) {
	dir := t.TempDir()
	// Empty map should be a no-op.
	if err := PersistParserGaps(dir, map[string]int{}); err != nil {
		t.Fatalf("PersistParserGaps(empty): %v", err)
	}
	// File should not exist.
	if _, err := os.Stat(filepath.Join(dir, parserGapsFile)); !os.IsNotExist(err) {
		t.Error("expected no file for empty gaps")
	}
}

func TestPersistCrashLogs_AppendOnly(t *testing.T) {
	dir := t.TempDir()

	// First batch.
	if err := PersistCrashLogs(dir, []string{"panic: nil pointer"}, []string{"Tinybones"}, 100, 4); err != nil {
		t.Fatalf("first persist: %v", err)
	}

	// Second batch.
	if err := PersistCrashLogs(dir, []string{"panic: index out of range", "timeout: 3m"}, []string{"K'rrik"}, 200, 4); err != nil {
		t.Fatalf("second persist: %v", err)
	}

	got, err := ReadCrashLogs(dir)
	if err != nil {
		t.Fatalf("ReadCrashLogs: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 crash logs, got %d", len(got))
	}

	// Verify first entry preserved.
	if got[0].StackTrace != "panic: nil pointer" {
		t.Errorf("first entry stack: got %q", got[0].StackTrace)
	}
	if len(got[0].Decks) != 1 || got[0].Decks[0] != "Tinybones" {
		t.Errorf("first entry decks: got %v", got[0].Decks)
	}
}

func TestPersistDeadTriggers(t *testing.T) {
	dir := t.TempDir()

	analyses := []*analytics.GameAnalysis{
		{
			WinningCard: "Craterhoof Behemoth",
			Players: []analytics.PlayerAnalysis{
				{
					CardsPlayed: []analytics.CardPerformance{
						{
							Name:            "Rhystic Study",
							TriggeredCount:  12,
							DamageDealt:     0,
							KillsAttributed: 0,
						},
						{
							Name:            "Craterhoof Behemoth",
							TriggeredCount:  1,
							DamageDealt:     40,
							KillsAttributed: 0,
							ContributedToWin: true,
						},
						{
							Name:            "Forest",
							TriggeredCount:  0,
							DamageDealt:     0,
							KillsAttributed: 0,
							IsLand:          true,
						},
					},
				},
			},
		},
	}

	if err := PersistDeadTriggers(dir, analyses); err != nil {
		t.Fatalf("PersistDeadTriggers: %v", err)
	}

	got, err := ReadDeadTriggers(dir)
	if err != nil {
		t.Fatalf("ReadDeadTriggers: %v", err)
	}

	// Should only have Rhystic Study (triggered but no impact, not winning card).
	// Craterhoof: ContributedToWin=true, so excluded.
	// Forest: IsLand, so excluded.
	if len(got) != 1 {
		t.Fatalf("expected 1 dead trigger, got %d", len(got))
	}
	if got[0].CardName != "Rhystic Study" {
		t.Errorf("expected Rhystic Study, got %q", got[0].CardName)
	}
	if got[0].Count != 12 {
		t.Errorf("count: got %d, want 12", got[0].Count)
	}
	if got[0].GamesSeen != 1 {
		t.Errorf("games_seen: got %d, want 1", got[0].GamesSeen)
	}
}

func TestPersistDeadTriggers_Merge(t *testing.T) {
	dir := t.TempDir()

	mkAnalysis := func(card string, trigCount int) []*analytics.GameAnalysis {
		return []*analytics.GameAnalysis{{
			WinningCard: "Something Else",
			Players: []analytics.PlayerAnalysis{{
				CardsPlayed: []analytics.CardPerformance{{
					Name:           card,
					TriggeredCount: trigCount,
				}},
			}},
		}}
	}

	if err := PersistDeadTriggers(dir, mkAnalysis("Rhystic Study", 5)); err != nil {
		t.Fatalf("first persist: %v", err)
	}
	if err := PersistDeadTriggers(dir, mkAnalysis("Rhystic Study", 3)); err != nil {
		t.Fatalf("second persist: %v", err)
	}

	got, err := ReadDeadTriggers(dir)
	if err != nil {
		t.Fatalf("ReadDeadTriggers: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 deduplicated trigger, got %d", len(got))
	}
	if got[0].Count != 8 {
		t.Errorf("count: got %d, want 8", got[0].Count)
	}
	if got[0].GamesSeen != 2 {
		t.Errorf("games_seen: got %d, want 2", got[0].GamesSeen)
	}
}

func TestReadParserGaps_NoFile(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestAtomicWrite_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, parserGapsFile)

	// Write corrupt JSON.
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	// ReadParserGaps should return an error for corrupt data.
	_, err := ReadParserGaps(dir)
	if err == nil {
		t.Fatal("expected error for corrupt JSON, got nil")
	}
}

func TestSortedParserGaps(t *testing.T) {
	gaps := []ParserGap{
		{Snippet: "a", Count: 1},
		{Snippet: "b", Count: 100},
		{Snippet: "c", Count: 50},
	}
	sorted := SortedParserGaps(gaps)
	if sorted[0].Snippet != "b" || sorted[1].Snippet != "c" || sorted[2].Snippet != "a" {
		t.Errorf("wrong sort order: %v", sorted)
	}
	// Original should be unchanged.
	if gaps[0].Snippet != "a" {
		t.Error("original slice was modified")
	}
}

func TestAtomicWriteJSON_Integrity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	data := []ParserGap{{Snippet: "test", Count: 42, FirstSeen: "2026-04-29T00:00:00Z", LastSeen: "2026-04-29T00:00:00Z"}}
	if err := atomicWriteJSON(path, data); err != nil {
		t.Fatalf("atomicWriteJSON: %v", err)
	}

	// Verify no temp file left behind.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file should not exist after successful rename")
	}

	// Verify file is valid JSON.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip []ParserGap
	if err := json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatalf("roundtrip unmarshal: %v", err)
	}
	if len(roundtrip) != 1 || roundtrip[0].Count != 42 {
		t.Errorf("roundtrip mismatch: %+v", roundtrip)
	}
}
