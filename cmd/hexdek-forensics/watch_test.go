package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWatchJSONLFromStart_ProcessesAllRecords(t *testing.T) {
	// Static fixture with 3 complete replays — start-from-beginning
	// must surface all 3 without waiting for new appends.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var (
		mu      sync.Mutex
		replays []*Replay
	)
	processed, err := WatchJSONLFromStart(ctx,
		"testdata/replay-stream.jsonl",
		WatchOptions{PollInterval: 20 * time.Millisecond, MaxLines: 3},
		func(r *Replay) {
			mu.Lock()
			defer mu.Unlock()
			replays = append(replays, r)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("WatchJSONLFromStart: %v", err)
	}
	if processed != 3 {
		t.Fatalf("processed: got %d, want 3", processed)
	}
	if len(replays) != 3 {
		t.Fatalf("onReplay invocations: got %d, want 3", len(replays))
	}
	wantGames := []int{411, 2762, 3589}
	for i, r := range replays {
		if r.GameIdx != wantGames[i] {
			t.Fatalf("replay %d game_idx: got %d, want %d", i, r.GameIdx, wantGames[i])
		}
	}
}

func TestWatchJSONLFromStart_AggregateAccumulates(t *testing.T) {
	// End-to-end: tail the fixture, merge each replay into a single
	// aggregate, assert the resulting cluster topology.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	agg := NewPatternAggregate()
	processed, err := WatchJSONLFromStart(ctx,
		"testdata/replay-stream.jsonl",
		WatchOptions{PollInterval: 20 * time.Millisecond, MaxLines: 3},
		func(r *Replay) {
			agg.MergeReplay(r)
		},
		nil,
	)
	if err != nil {
		t.Fatalf("WatchJSONLFromStart: %v", err)
	}
	if processed != 3 {
		t.Fatalf("processed: got %d, want 3", processed)
	}
	if agg.ReplaysSeen != 3 {
		t.Fatalf("ReplaysSeen: got %d, want 3", agg.ReplaysSeen)
	}
	// 3 replays, 1 fabrication each = 3 traces. 2 share the OG +
	// enter_battlefield + instance_id key; 1 is TK + creature_attacks
	// + card_name. So 2 clusters total.
	if agg.TotalTraces != 3 {
		t.Fatalf("TotalTraces: got %d, want 3", agg.TotalTraces)
	}
	if len(agg.ByKey) != 2 {
		t.Fatalf("clusters: got %d, want 2", len(agg.ByKey))
	}
}

func TestWatchJSONLFromStart_AppendedLinesPickedUp(t *testing.T) {
	// Dynamic append: write 1 line, start watch, append 2 more
	// concurrently, expect all 3 to surface before MaxLines stops us.
	dir := t.TempDir()
	path := filepath.Join(dir, "stream.jsonl")

	const replay1 = `{"game_idx":1,"events":[{"Kind":"enter_battlefield","Source":"A","Details":{"instance_id":"h0OGVR100001"}}],"violations":[{"turn":1,"invariant":"ZoneConservation","message":"ZoneConservation: InstanceID \"h0OGVR100001\" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry"}],"card_index":{"h0OGVR100001":"A"}}` + "\n"
	const replay2 = `{"game_idx":2,"events":[{"Kind":"enter_battlefield","Source":"B","Details":{"instance_id":"h0OGVR100002"}}],"violations":[{"turn":2,"invariant":"ZoneConservation","message":"ZoneConservation: InstanceID \"h0OGVR100002\" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry"}],"card_index":{"h0OGVR100002":"B"}}` + "\n"
	const replay3 = `{"game_idx":3,"events":[{"Kind":"enter_battlefield","Source":"C","Details":{"instance_id":"h0OGVR100003"}}],"violations":[{"turn":3,"invariant":"ZoneConservation","message":"ZoneConservation: InstanceID \"h0OGVR100003\" present in a zone but not in (Minted - Ceased) — fabrication or stale ceased entry"}],"card_index":{"h0OGVR100003":"C"}}` + "\n"

	if err := os.WriteFile(path, []byte(replay1), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var (
		mu      sync.Mutex
		gotIDs  []int
		started = make(chan struct{})
	)
	var processed int
	var watchErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		close(started)
		processed, watchErr = WatchJSONLFromStart(ctx, path,
			WatchOptions{PollInterval: 30 * time.Millisecond, MaxLines: 3},
			func(r *Replay) {
				mu.Lock()
				gotIDs = append(gotIDs, r.GameIdx)
				mu.Unlock()
			},
			nil,
		)
	}()
	<-started

	// Give the watcher one poll cycle to ingest the seed line,
	// then append the rest.
	time.Sleep(80 * time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString(replay2 + replay3); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	<-done
	if watchErr != nil {
		t.Fatalf("watch err: %v", watchErr)
	}
	if processed != 3 {
		t.Fatalf("processed: got %d, want 3", processed)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(gotIDs) != 3 || gotIDs[0] != 1 || gotIDs[1] != 2 || gotIDs[2] != 3 {
		t.Fatalf("got IDs %v, want [1 2 3]", gotIDs)
	}
}

func TestWatchJSONLFromStart_MalformedLineDoesNotKillWatch(t *testing.T) {
	// One valid replay, one corrupt line, one more valid replay —
	// the corrupt one should hit onError but not stop processing.
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.jsonl")
	contents := strings.Join([]string{
		`{"game_idx":1,"violations":[],"card_index":{}}`,
		`{not valid json}`,
		`{"game_idx":2,"violations":[],"card_index":{}}`,
		"", // trailing newline
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var (
		mu       sync.Mutex
		valid    []int
		errCount int
	)
	processed, err := WatchJSONLFromStart(ctx, path,
		WatchOptions{PollInterval: 20 * time.Millisecond, MaxLines: 2},
		func(r *Replay) {
			mu.Lock()
			valid = append(valid, r.GameIdx)
			mu.Unlock()
		},
		func(e error) {
			mu.Lock()
			errCount++
			mu.Unlock()
		},
	)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed: got %d, want 2 (corrupt line should not count)", processed)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(valid) != 2 || valid[0] != 1 || valid[1] != 2 {
		t.Fatalf("valid game_idx: got %v, want [1 2]", valid)
	}
	if errCount != 1 {
		t.Fatalf("errCount: got %d, want 1", errCount)
	}
}

func TestWatchJSONL_MissingFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := WatchJSONL(ctx, "testdata/does-not-exist.jsonl", WatchOptions{}, nil, nil)
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}
