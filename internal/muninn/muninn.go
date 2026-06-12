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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
)

var fileMu sync.Mutex

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

// ConcessionRecord captures the board state at the moment a player
// conceded. Used to diagnose whether conviction thresholds are
// calibrated or if the hat is conceding too early.
type ConcessionRecord struct {
	Commander  string `json:"commander"`
	Turn       int    `json:"turn"`
	BoardPower int    `json:"board_power"`
	Life       int    `json:"life"`
	HandSize   int    `json:"hand_size"`
	Opponents  int    `json:"opponents_alive"`
	Timestamp  string `json:"timestamp"`
}

// InvariantViolation records a Feynman/Odin-detected invariant violation
// during a game. The seed + deck keys make the offending game replayable
// from the regression runner.
//
// Consolidation step 4 note: this is a PERSISTED JSON row schema
// (invariant_violations.json), not a violation vocabulary — the
// violations it archives were already routed through
// validation.LogViolation at their origin (RunAllInvariants / Feynman
// CheckGame). Do not re-log here; do not grow this into a vocabulary.
type InvariantViolation struct {
	GameSeed      int64     `json:"game_seed"`
	DeckKeys      [4]string `json:"deck_keys"`
	ViolationType string    `json:"violation_type"`
	Message       string    `json:"message"`
	Timestamp     string    `json:"timestamp"`
	Turn          int       `json:"turn,omitempty"`
}

// RegressionFailure records a parity test failure.
type RegressionFailure struct {
	TestName  string `json:"test_name"`
	Expected  string `json:"expected"`
	Got       string `json:"got"`
	GameSeed  int64  `json:"game_seed"`
	Timestamp string `json:"timestamp"`
}

// --------------------------------------------------------------------
// File names
// --------------------------------------------------------------------

const (
	parserGapsFile          = "parser_gaps.json"
	crashesFile             = "crashes.json"
	deadTriggersFile        = "dead_triggers.json"
	concessionsFile         = "concessions.json"
	invariantViolationsFile = "invariant_violations.json"
	regressionFailuresFile  = "regression_failures.json"

	maxInvariantViolations = 10000
	maxCrashLogs           = 1000
	maxConcessions         = 10000
	maxRegressionFailures  = 5000
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

	fileMu.Lock()
	defer fileMu.Unlock()

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

	fileMu.Lock()
	defer fileMu.Unlock()

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

	existing = capEntries(existing, maxCrashLogs)
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

	fileMu.Lock()
	defer fileMu.Unlock()

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

// PersistConcessions appends new concession records to the persistent
// concessions.json file. Never overwrites existing entries.
func PersistConcessions(dir string, records []ConcessionRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	existing, err := ReadConcessions(dir)
	if err != nil {
		return err
	}

	existing = capEntries(append(existing, records...), maxConcessions)
	return atomicWriteJSON(filepath.Join(dir, concessionsFile), existing)
}

// PersistDeadTriggersRaw writes a pre-formed slice of DeadTrigger records
// to dead_triggers.json. Used by adapters that bridge from other subsystems
// (e.g. Heimdall) where the triggers have already been merged.
func PersistDeadTriggersRaw(dir string, triggers []DeadTrigger) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}
	fileMu.Lock()
	defer fileMu.Unlock()
	return atomicWriteJSON(filepath.Join(dir, deadTriggersFile), triggers)
}

// PersistInvariantViolations appends new Odin-detected invariant
// violations to the persistent invariant_violations.json file.
func PersistInvariantViolations(dir string, violations []InvariantViolation) error {
	if len(violations) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	existing, err := ReadInvariantViolations(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range violations {
		if violations[i].Timestamp == "" {
			violations[i].Timestamp = now
		}
	}

	existing = capEntries(append(existing, violations...), maxInvariantViolations)
	return atomicWriteJSON(filepath.Join(dir, invariantViolationsFile), existing)
}

// AutoArchiveViolation appends Feynman-detected invariant violations to
// invariant_violations.json in dir. Each violation string in the slice
// produces one record tagged with the game's RNG seed and deck keys so
// the regression runner can replay the offending game.
//
// Violation strings are expected to use OracleViolation.String() format:
//
//	[severity] rule (seat N): description
//
// The "rule" portion is parsed out as ViolationType; the full string is
// preserved in Message. Strings that don't match the format are stored
// verbatim with an empty ViolationType.
func AutoArchiveViolation(dir string, rngSeed int64, deckKeys [4]string, violations []string) error {
	if len(violations) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	existing, err := ReadInvariantViolations(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, v := range violations {
		vtype, msg := parseOracleViolation(v)
		existing = append(existing, InvariantViolation{
			GameSeed:      rngSeed,
			DeckKeys:      deckKeys,
			ViolationType: vtype,
			Message:       msg,
			Timestamp:     now,
		})
	}

	existing = capEntries(existing, maxInvariantViolations)
	return atomicWriteJSON(filepath.Join(dir, invariantViolationsFile), existing)
}

// parseOracleViolation extracts the rule identifier from a string formatted
// by hat.OracleViolation.String() ("[severity] rule (seat N): description").
// Returns the rule plus the original message. If the string doesn't match
// the format, returns ("", s).
func parseOracleViolation(s string) (vtype, msg string) {
	if !strings.HasPrefix(s, "[") {
		return "", s
	}
	end := strings.Index(s, "] ")
	if end < 0 {
		return "", s
	}
	rest := s[end+2:]
	if j := strings.Index(rest, " (seat "); j >= 0 {
		return rest[:j], s
	}
	if j := strings.Index(rest, ": "); j >= 0 {
		return rest[:j], s
	}
	return rest, s
}

// PersistRegressionFailures appends new parity test failures to the
// persistent regression_failures.json file.
func PersistRegressionFailures(dir string, failures []RegressionFailure) error {
	if len(failures) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	existing, err := ReadRegressionFailures(dir)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i := range failures {
		if failures[i].Timestamp == "" {
			failures[i].Timestamp = now
		}
	}

	existing = capEntries(append(existing, failures...), maxRegressionFailures)
	return atomicWriteJSON(filepath.Join(dir, regressionFailuresFile), existing)
}

// --------------------------------------------------------------------
// Read functions
// --------------------------------------------------------------------

// ReadConcessions reads the persistent concessions.json file.
func ReadConcessions(dir string) ([]ConcessionRecord, error) {
	var out []ConcessionRecord
	if err := readJSON(filepath.Join(dir, concessionsFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []ConcessionRecord{}
	}
	return out, nil
}

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

// ReadInvariantViolations reads the persistent invariant_violations.json file.
func ReadInvariantViolations(dir string) ([]InvariantViolation, error) {
	var out []InvariantViolation
	if err := readJSON(filepath.Join(dir, invariantViolationsFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []InvariantViolation{}
	}
	return out, nil
}

// ReadRegressionFailures reads the persistent regression_failures.json file.
func ReadRegressionFailures(dir string) ([]RegressionFailure, error) {
	var out []RegressionFailure
	if err := readJSON(filepath.Join(dir, regressionFailuresFile), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []RegressionFailure{}
	}
	return out, nil
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

// capEntries keeps the last max elements when a slice exceeds the limit.
func capEntries[T any](s []T, max int) []T {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

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

	tmp := path + ".tmp." + strconv.FormatInt(time.Now().UnixNano(), 36)
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

// ConcessionSummary aggregates concession counts by commander.
type ConcessionSummary struct {
	Commander string `json:"commander"`
	Count     int    `json:"count"`
	AvgTurn   float64 `json:"avg_turn"`
	AvgLife   float64 `json:"avg_life"`
}

// SortedConcessions aggregates concession records by commander and
// returns them sorted by count descending.
func SortedConcessions(records []ConcessionRecord) []ConcessionSummary {
	type accum struct {
		count    int
		turnSum  int
		lifeSum  int
	}
	byCmd := map[string]*accum{}
	for _, r := range records {
		a := byCmd[r.Commander]
		if a == nil {
			a = &accum{}
			byCmd[r.Commander] = a
		}
		a.count++
		a.turnSum += r.Turn
		a.lifeSum += r.Life
	}
	out := make([]ConcessionSummary, 0, len(byCmd))
	for cmd, a := range byCmd {
		out = append(out, ConcessionSummary{
			Commander: cmd,
			Count:     a.count,
			AvgTurn:   float64(a.turnSum) / float64(a.count),
			AvgLife:   float64(a.lifeSum) / float64(a.count),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Count > out[j].Count
	})
	return out
}
