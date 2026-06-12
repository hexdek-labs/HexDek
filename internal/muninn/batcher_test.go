package muninn

import (
	"github.com/hexdek/hexdek/internal/judge"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBatcher_FlushOnClose(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: time.Hour})

	b.AddParserGaps(map[string]int{"snippet-a": 3, "snippet-b": 2})
	b.AddCrash("panic: boom", []string{"Tinybones"}, 100, 4)
	b.AddDeadTrigger("triggered_ability", "Rhystic Study", 5, 1)
	b.AddConcessions([]ConcessionRecord{{Commander: "Atraxa", Turn: 7, Life: 12}})

	// Nothing should be on disk yet (interval is 1h, batch size 1000).
	if _, err := os.Stat(filepath.Join(dir, parserGapsFile)); !os.IsNotExist(err) {
		t.Errorf("expected no parser_gaps.json before close, got err=%v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	gaps, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	if len(gaps) != 2 {
		t.Errorf("expected 2 parser gaps, got %d", len(gaps))
	}

	crashes, err := ReadCrashLogs(dir)
	if err != nil {
		t.Fatalf("ReadCrashLogs: %v", err)
	}
	if len(crashes) != 1 || crashes[0].StackTrace != "panic: boom" {
		t.Errorf("crashes: got %+v", crashes)
	}

	dead, err := ReadDeadTriggers(dir)
	if err != nil {
		t.Fatalf("ReadDeadTriggers: %v", err)
	}
	if len(dead) != 1 || dead[0].Count != 5 || dead[0].GamesSeen != 1 {
		t.Errorf("dead triggers: got %+v", dead)
	}

	cons, err := ReadConcessions(dir)
	if err != nil {
		t.Fatalf("ReadConcessions: %v", err)
	}
	if len(cons) != 1 || cons[0].Commander != "Atraxa" {
		t.Errorf("concessions: got %+v", cons)
	}
}

func TestBatcher_FlushOnBatchSize(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 3, FlushInterval: time.Hour})
	t.Cleanup(func() { _ = b.Close() })

	// Add 3 parser-gap entries; the 3rd should trigger a synchronous flush.
	b.AddParserGaps(map[string]int{"a": 1})
	b.AddParserGaps(map[string]int{"b": 1})

	// Before threshold, file should not exist.
	if _, err := os.Stat(filepath.Join(dir, parserGapsFile)); !os.IsNotExist(err) {
		t.Errorf("expected no parser_gaps.json before threshold, got err=%v", err)
	}

	b.AddParserGaps(map[string]int{"c": 1})

	// Synchronous flush from inside Add — file should exist now.
	gaps, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	if len(gaps) != 3 {
		t.Errorf("expected 3 parser gaps after batch flush, got %d (%+v)", len(gaps), gaps)
	}
}

func TestBatcher_FlushOnInterval(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: 50 * time.Millisecond})
	t.Cleanup(func() { _ = b.Close() })

	b.AddParserGaps(map[string]int{"interval-snippet": 1})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		gaps, err := ReadParserGaps(dir)
		if err != nil {
			t.Fatalf("ReadParserGaps: %v", err)
		}
		if len(gaps) == 1 && gaps[0].Snippet == "interval-snippet" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("interval-driven flush did not occur within 2s")
}

func TestBatcher_DeadTriggerMerge(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: time.Hour})

	// Two pre-existing entries — verify merge into existing file.
	if err := PersistDeadTriggersRaw(dir, []DeadTrigger{
		{TriggerName: "triggered_ability", CardName: "Rhystic Study", Count: 10, GamesSeen: 5},
	}); err != nil {
		t.Fatalf("seed dead_triggers: %v", err)
	}

	// Same key + new key, repeated calls accumulate.
	b.AddDeadTrigger("triggered_ability", "Rhystic Study", 1, 1)
	b.AddDeadTrigger("triggered_ability", "Rhystic Study", 1, 1)
	b.AddDeadTrigger("triggered_ability", "Smothering Tithe", 3, 1)

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ReadDeadTriggers(dir)
	if err != nil {
		t.Fatalf("ReadDeadTriggers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d (%+v)", len(got), got)
	}
	byCard := map[string]DeadTrigger{}
	for _, dt := range got {
		byCard[dt.CardName] = dt
	}
	if r := byCard["Rhystic Study"]; r.Count != 12 || r.GamesSeen != 7 {
		t.Errorf("Rhystic Study: got Count=%d GamesSeen=%d, want 12/7", r.Count, r.GamesSeen)
	}
	if s := byCard["Smothering Tithe"]; s.Count != 3 || s.GamesSeen != 1 {
		t.Errorf("Smothering Tithe: got Count=%d GamesSeen=%d, want 3/1", s.Count, s.GamesSeen)
	}
}

func TestBatcher_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: time.Hour})
	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestBatcher_FlushOnGameCount(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{
		Dir:           dir,
		BatchSize:     1000,
		GamesPerFlush: 5,
		FlushInterval: time.Hour,
	})
	t.Cleanup(func() { _ = b.Close() })

	b.AddParserGaps(map[string]int{"per-game-snippet": 1})

	// 4 games: still under threshold, no flush expected.
	for i := 0; i < 4; i++ {
		b.EndGame()
	}
	if _, err := os.Stat(filepath.Join(dir, parserGapsFile)); !os.IsNotExist(err) {
		t.Errorf("expected no parser_gaps.json before game threshold, got err=%v", err)
	}

	// 5th game crosses threshold and triggers a synchronous flush.
	b.EndGame()

	gaps, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	if len(gaps) != 1 || gaps[0].Snippet != "per-game-snippet" {
		t.Errorf("expected per-game-snippet flushed by game count, got %+v", gaps)
	}
}

func TestBatcher_EndGameResetsCounter(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{
		Dir:           dir,
		BatchSize:     1000,
		GamesPerFlush: 3,
		FlushInterval: time.Hour,
	})
	t.Cleanup(func() { _ = b.Close() })

	// First batch of games triggers a flush at the 3rd EndGame.
	b.AddParserGaps(map[string]int{"first": 1})
	b.EndGame()
	b.EndGame()
	b.EndGame()

	// Second batch must also flush after exactly 3 more games — proving
	// the counter resets, not 6 games total.
	b.AddParserGaps(map[string]int{"second": 1})
	b.EndGame()
	b.EndGame()
	// 5 total EndGame calls; "second" should not yet be on disk.
	gaps, err := ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	hasSecond := false
	for _, g := range gaps {
		if g.Snippet == "second" {
			hasSecond = true
		}
	}
	if hasSecond {
		t.Errorf("second flushed too early: %+v", gaps)
	}

	b.EndGame() // 6th total / 3rd of second batch -> flush
	gaps, err = ReadParserGaps(dir)
	if err != nil {
		t.Fatalf("ReadParserGaps: %v", err)
	}
	if len(gaps) != 2 {
		t.Errorf("expected 2 snippets after second flush, got %+v", gaps)
	}
}

func TestBatcher_AddAutoArchive(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: time.Hour})

	b.AddAutoArchive(424242, [4]string{"deckA", "deckB", "deckC", "deckD"}, []judge.ValidationViolation{
		{Name: "permanent_type_consistency", Severity: judge.SeverityCritical, Seat: 2,
			Message: "creature has noncreature type"},
		{Name: "mana_pool_drain", Severity: judge.SeverityWarning, Seat: 0,
			Message: "floating mana not emptied"},
		{Name: "freeform_note", Message: "freeform note without prefix"},
	})
	// Empty input is a no-op (must not panic, must not flush).
	b.AddAutoArchive(0, [4]string{}, nil)

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := ReadInvariantViolations(dir)
	if err != nil {
		t.Fatalf("ReadInvariantViolations: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 violations, got %d (%+v)", len(got), got)
	}
	if got[0].GameSeed != 424242 || got[0].DeckKeys[0] != "deckA" {
		t.Errorf("seed/deck keys not propagated: %+v", got[0])
	}
	if got[0].ViolationType != "permanent_type_consistency" {
		t.Errorf("expected parsed ViolationType, got %q", got[0].ViolationType)
	}
	if got[2].ViolationType != "freeform_note" {
		t.Errorf("canonical Name carries through as ViolationType: %+v", got[2])
	}
}

func TestBatcher_EmptyFlushNoOp(t *testing.T) {
	dir := t.TempDir()
	b := NewBatcher(BatcherConfig{Dir: dir, BatchSize: 1000, FlushInterval: time.Hour})
	if err := b.Flush(); err != nil {
		t.Fatalf("Flush on empty: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// No files should have been created.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dir, got %d entries", len(entries))
	}
}
