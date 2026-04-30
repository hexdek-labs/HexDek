// Package muninn provides persistent memory for the HexDek tournament
// runner. Named after Odin's raven of memory, it accumulates parser gaps,
// crash logs, and dead triggers across tournament runs as append-only
// JSON files on disk.
//
// All persist functions are safe for concurrent use across sequential
// tournament runs. They read the existing file, merge/append new data,
// and write atomically via temp-file + rename.
package muninn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
)

// --------------------------------------------------------------------
// Types
// --------------------------------------------------------------------

// ParserGap tracks a single parser gap snippet across tournament runs.
type ParserGap struct {
	Snippet     string   `json:"snippet"`
	Count       int      `json:"count"`
	FirstSeen   string   `json:"first_seen"`   // RFC3339
	LastSeen    string   `json:"last_seen"`     // RFC3339
	GameConfigs []string `json:"game_configs"`  // optional context
}

// CrashLog records a single crash (panic or timeout) from a tournament game.
type CrashLog struct {
	StackTrace string                 `json:"stack_trace"`
	Decks      []string               `json:"decks"`
	GameConfig map[string]interface{} `json:"game_config"`
	TurnCount  int                    `json:"turn_count"`
	Timestamp  string                 `json:"timestamp"` // RFC3339
}

// DeadTrigger tracks triggers that fired but produced no measurable
// game state change — wired but dead.
type DeadTrigger struct {
	TriggerName string `json:"trigger_name"`
	CardName    string `json:"card_name"`
	Count       int    `json:"count"`
	GamesSeen   int    `json:"games_seen"`
	LastSeen    string `json:"last_seen"` // RFC3339
}

// --------------------------------------------------------------------
// File names
// --------------------------------------------------------------------

const (
	parserGapsFile   = "parser_gaps.json"
	crashesFile      = "crashes.json"
	deadTriggersFile = "dead_triggers.json"
)

// --------------------------------------------------------------------
// Persist functions
// --------------------------------------------------------------------

// PersistParserGaps merges new parser gap counts into the persistent
// parser_gaps.json file. Deduplicates by snippet text, increments count,
// and updates last_seen.
func PersistParserGaps(dir string, gaps map[string]int) error {
	if len(gaps) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	existing, err := ReadParserGaps(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Index existing by snippet for O(1) lookup.
	idx := make(map[string]int, len(existing))
	for i, g := range existing {
		idx[g.Snippet] = i
	}

	for snippet, count := range gaps {
		if i, ok := idx[snippet]; ok {
			existing[i].Count += count
			existing[i].LastSeen = now
		} else {
			existing = append(existing, ParserGap{
				Snippet:   snippet,
				Count:     count,
				FirstSeen: now,
				LastSeen:  now,
			})
		}
	}

	return atomicWriteJSON(filepath.Join(dir, parserGapsFile), existing)
}

// PersistCrashLogs appends new crash entries to the persistent
// crashes.json file. Never overwrites existing entries.
func PersistCrashLogs(dir string, crashes []string, commanderNames []string, nGames, nSeats int) error {
	if len(crashes) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	existing, err := ReadCrashLogs(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, stackTrace := range crashes {
		entry := CrashLog{
			StackTrace: stackTrace,
			Decks:      append([]string(nil), commanderNames...),
			GameConfig: map[string]interface{}{
				"n_games": nGames,
				"n_seats": nSeats,
			},
			Timestamp: now,
		}
		existing = append(existing, entry)
	}

	return atomicWriteJSON(filepath.Join(dir, crashesFile), existing)
}

// PersistDeadTriggers scans GameAnalysis results for triggers that fired
// (TriggeredCount > 0) but the card had zero DamageDealt, zero
// KillsAttributed, and was not the WinningCard. These are "wired but
// dead" — the trigger executed but produced no measurable impact.
func PersistDeadTriggers(dir string, analyses []*analytics.GameAnalysis) error {
	if len(analyses) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	// Collect dead triggers from this batch.
	type dtKey struct {
		triggerName string
		cardName    string
	}
	batch := make(map[dtKey]int)        // key -> total fire count
	batchGames := make(map[dtKey]int)   // key -> number of games seen

	for _, ga := range analyses {
		if ga == nil {
			continue
		}
		// Track which cards are dead in this game (deduplicate per game).
		seenThisGame := make(map[dtKey]bool)
		for _, pa := range ga.Players {
			for _, cp := range pa.CardsPlayed {
				if cp.TriggeredCount > 0 &&
					cp.DamageDealt == 0 &&
					cp.KillsAttributed == 0 &&
					!cp.ContributedToWin &&
					cp.Name != ga.WinningCard &&
					!cp.IsLand &&
					!cp.IsToken {
					key := dtKey{
						triggerName: "triggered_ability",
						cardName:    cp.Name,
					}
					batch[key] += cp.TriggeredCount
					if !seenThisGame[key] {
						seenThisGame[key] = true
						batchGames[key]++
					}
				}
			}
		}
	}

	if len(batch) == 0 {
		return nil
	}

	existing, err := ReadDeadTriggers(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Index existing by trigger_name + card_name.
	type existKey struct {
		triggerName string
		cardName    string
	}
	idx := make(map[existKey]int, len(existing))
	for i, dt := range existing {
		idx[existKey{dt.TriggerName, dt.CardName}] = i
	}

	for key, count := range batch {
		ek := existKey{key.triggerName, key.cardName}
		if i, ok := idx[ek]; ok {
			existing[i].Count += count
			existing[i].GamesSeen += batchGames[key]
			existing[i].LastSeen = now
		} else {
			existing = append(existing, DeadTrigger{
				TriggerName: key.triggerName,
				CardName:    key.cardName,
				Count:       count,
				GamesSeen:   batchGames[key],
				LastSeen:    now,
			})
		}
	}

	return atomicWriteJSON(filepath.Join(dir, deadTriggersFile), existing)
}

// --------------------------------------------------------------------
// Read functions
// --------------------------------------------------------------------

// ReadParserGaps reads the persistent parser_gaps.json file. Returns an
// empty slice if the file does not exist.
func ReadParserGaps(dir string) ([]ParserGap, error) {
	var out []ParserGap
	if err := readJSON(filepath.Join(dir, parserGapsFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []ParserGap{}
	}
	return out, nil
}

// ReadCrashLogs reads the persistent crashes.json file.
func ReadCrashLogs(dir string) ([]CrashLog, error) {
	var out []CrashLog
	if err := readJSON(filepath.Join(dir, crashesFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []CrashLog{}
	}
	return out, nil
}

// ReadDeadTriggers reads the persistent dead_triggers.json file.
func ReadDeadTriggers(dir string) ([]DeadTrigger, error) {
	var out []DeadTrigger
	if err := readJSON(filepath.Join(dir, deadTriggersFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []DeadTrigger{}
	}
	return out, nil
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

// readJSON reads a JSON file into dst. Returns nil if the file does not
// exist (dst is left at its zero value).
func readJSON(path string, dst interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("muninn: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("muninn: parse %s: %w", path, err)
	}
	return nil
}

// atomicWriteJSON writes data as indented JSON to a temp file then
// renames it to the target path. This prevents partial writes from
// corrupting the persistent file.
func atomicWriteJSON(path string, data interface{}) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("muninn: marshal: %w", err)
	}
	out = append(out, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("muninn: write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("muninn: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// SortedParserGaps returns parser gaps sorted by count descending.
func SortedParserGaps(gaps []ParserGap) []ParserGap {
	sorted := make([]ParserGap, len(gaps))
	copy(sorted, gaps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	return sorted
}

// SortedDeadTriggers returns dead triggers sorted by count descending.
func SortedDeadTriggers(triggers []DeadTrigger) []DeadTrigger {
	sorted := make([]DeadTrigger, len(triggers))
	copy(sorted, triggers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})
	return sorted
}

// SortedCrashLogs returns crash logs sorted by timestamp descending
// (most recent first).
func SortedCrashLogs(logs []CrashLog) []CrashLog {
	sorted := make([]CrashLog, len(logs))
	copy(sorted, logs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp > sorted[j].Timestamp
	})
	return sorted
}
