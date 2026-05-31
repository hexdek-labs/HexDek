package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Watch mode tails a JSONL replay stream — Loki (future producer
// integration) appends one Replay JSON per game as it finishes; this
// tool reads each line, AnalyzeReplay's it, MergeReplay's the result
// into a running PatternAggregate, and re-renders the cluster table
// after each new line so the user gets incremental post-mortem signal
// without waiting for the full run to complete.
//
// Wire format: one Replay JSON per line, newline-terminated, no
// surrounding array brackets (line-oriented for tail-friendliness).
// Partial lines mid-write are tolerated — bytes that don't end in a
// newline stay in the partial-line buffer and re-attempt parse on
// the next poll once the writer flushes.
//
// Producer-side TODO (same flag pair as the one-shot replay-out):
// hexdek-loki gains `--replay-stream-out <path>` that opens the file
// for append and writes one Replay per finished game. Until that
// ships, users can hand-author JSONL files or pipe synthetic streams
// for testing.

// WatchOptions tunes the polling loop. PollInterval is the cadence
// for "has the file grown?" checks; 500ms is a reasonable default
// for human-watching-terminal speed and is light on disk.
type WatchOptions struct {
	PollInterval time.Duration
	// MaxLines, when > 0, stops the watch after processing N
	// replays. Used by tests to bound execution; production runs
	// leave it 0 (unbounded).
	MaxLines int
}

// WatchJSONL opens path for tail-style reading, parses each newline-
// terminated Replay, feeds it through onReplay (typically aggregate
// merge + re-render), and continues until ctx is canceled or
// opts.MaxLines hits the cap.
//
// Returns the number of replays processed and the first hard error
// encountered (nil on clean ctx-cancel / MaxLines exit). Malformed
// individual lines are surfaced via onError if non-nil but do NOT
// terminate the watch — a truncated JSON write at end-of-file is the
// common transient case and the tool keeps going.
//
// Tail semantics: starts from the file's current END so a long
// pre-existing JSONL isn't replayed in bulk (use one-shot --replay
// for that). To start-from-beginning, seek the file to 0 before
// calling (typical pattern: open + ftruncate? — no, just unset start
// behavior via opts in a future revision; today's MVP is tail-only).
func WatchJSONL(ctx context.Context, path string, opts WatchOptions, onReplay func(*Replay), onError func(error)) (int, error) {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	// Tail from current end-of-file. Test usage that wants to
	// process pre-existing content should seek back to 0 via opts
	// (see WatchOptions.MaxLines / future StartFromBeginning).
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return 0, fmt.Errorf("seek end %q: %w", path, err)
	}

	return tailLoop(ctx, f, opts, onReplay, onError)
}

// WatchJSONLFromStart is the start-from-beginning variant — used by
// the test suite to replay a static fixture deterministically and by
// users who want a full post-mortem on a completed JSONL file.
func WatchJSONLFromStart(ctx context.Context, path string, opts WatchOptions, onReplay func(*Replay), onError func(error)) (int, error) {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 500 * time.Millisecond
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()
	return tailLoop(ctx, f, opts, onReplay, onError)
}

// tailLoop is the shared inner loop for both seek-end (live watch)
// and seek-start (full replay) variants. Reads available bytes,
// splits on newlines, processes complete lines, buffers the partial
// trailing fragment for the next poll iteration.
func tailLoop(ctx context.Context, f *os.File, opts WatchOptions, onReplay func(*Replay), onError func(error)) (int, error) {
	var pending strings.Builder
	processed := 0
	buf := make([]byte, 64*1024)

	for {
		// Drain everything currently available from the offset
		// onwards, then poll-sleep before checking again.
		for {
			n, err := f.Read(buf)
			if n > 0 {
				pending.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return processed, fmt.Errorf("read: %w", err)
			}
			if n == 0 {
				break
			}
		}

		// Split the pending buffer into complete lines + a trailing
		// fragment. Process completes; replace pending with fragment.
		for {
			s := pending.String()
			nl := strings.IndexByte(s, '\n')
			if nl < 0 {
				break
			}
			line := strings.TrimRight(s[:nl], "\r ")
			fragment := s[nl+1:]
			pending.Reset()
			pending.WriteString(fragment)

			if line == "" {
				continue
			}
			processed = processOneLine(line, processed, onReplay, onError)
			if opts.MaxLines > 0 && processed >= opts.MaxLines {
				return processed, nil
			}
		}

		select {
		case <-ctx.Done():
			return processed, nil
		case <-time.After(opts.PollInterval):
			// Loop and try reading more.
		}
	}
}

// processOneLine parses a single trimmed JSON line and invokes
// onReplay if successful. Surfaces parse errors via onError without
// terminating. Returns the new processed count.
func processOneLine(line string, processed int, onReplay func(*Replay), onError func(error)) int {
	var r Replay
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		if onError != nil {
			onError(fmt.Errorf("malformed replay line: %w", err))
		}
		return processed
	}
	if onReplay != nil {
		onReplay(&r)
	}
	return processed + 1
}
