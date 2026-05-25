package heimdall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CompositionPriorReplayRecord is the on-disk shape persisted by
// Observer.RecordObservation for any game whose Observation carries
// CompositionPriorEffects. One file per game, named by the RNG seed
// (which uniquely identifies a game in showmatch + tournament
// contexts). Read back by cmd/hexdek-composition-replay for debug
// / tuning workflows.
type CompositionPriorReplayRecord struct {
	GameSeed GameSeed                 `json:"game_seed"`
	Effects  []CompositionPriorEffect `json:"effects"`
}

// compositionPriorLogMu serializes writes to the same file across
// goroutines. Acceptable contention because each game writes once
// to a unique filename — collisions only happen on the unlikely
// case of duplicate RNG seeds in flight at the same moment.
var compositionPriorLogMu sync.Mutex

// CompositionPriorLogDir returns the on-disk directory where
// per-game effect records live, relative to the Observer's dataDir.
// Exposed so the replay CLI can locate the file without re-importing
// the Observer.
func CompositionPriorLogDir(dataDir string) string {
	return filepath.Join(dataDir, "heimdall", "composition_prior")
}

// writeCompositionPriorRecord persists a single per-game effect
// record as JSON. Returns silently on any error — this is best-
// effort debug telemetry, never fatal to a game.
func writeCompositionPriorRecord(dataDir string, rec CompositionPriorReplayRecord) {
	if len(rec.Effects) == 0 {
		return
	}
	dir := CompositionPriorLogDir(dataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", rec.GameSeed.RNGSeed))
	compositionPriorLogMu.Lock()
	defer compositionPriorLogMu.Unlock()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// LoadCompositionPriorRecord loads a previously-persisted per-game
// effect record by RNG seed. Returns (nil, os.ErrNotExist) when the
// file doesn't exist (game never wrote effects, or pre-PR-#422 game).
// Exposed for the replay CLI; not used inside the runtime.
func LoadCompositionPriorRecord(dataDir string, rngSeed int64) (*CompositionPriorReplayRecord, error) {
	path := filepath.Join(CompositionPriorLogDir(dataDir), fmt.Sprintf("%d.json", rngSeed))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rec CompositionPriorReplayRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("decode replay record %s: %w", path, err)
	}
	return &rec, nil
}
