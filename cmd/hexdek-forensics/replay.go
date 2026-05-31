package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Replay is the on-disk schema for a single Loki game's event log +
// invariant violations + final-state card index, designed for offline
// forensics analysis (mint-bypass bisect, LTB-path tracing, etc.).
//
// Producer side (future Loki integration): a `--replay-game-out <path>
// --replay-game-idx N` flag pair on hexdek-loki will dump this struct
// to JSON for the specified game index. Today, replays are either
// hand-rolled fixtures (see testdata/) or produced by a small ad-hoc
// dump that the Loki main loop can be patched to emit.
//
// The CardIndex is the (InstanceID → display name) snapshot at violation
// time, harvested from gs.MintedInstanceIDNames + a walk of every seat's
// zones. It lets the forensics tool decode an InstanceID from a
// violation message back to a human-readable card name even if the
// underlying *Card pointer has moved zones since first mint.
//
// JSON wire format: field names are lowercase + snake_case via struct
// tags so the schema is hand-editable and friendly to jq.
type Replay struct {
	GameIdx    int                `json:"game_idx"`
	Seed       int64              `json:"seed"`
	DeckSeed   int64              `json:"deck_seed"`
	Commanders []string           `json:"commanders,omitempty"`
	Events     []gameengine.Event `json:"events"`
	Violations []ViolationRecord  `json:"violations"`
	CardIndex  map[string]string  `json:"card_index"`
}

// ViolationRecord mirrors hexdek-loki's chaosViolation but trimmed to
// the fields the forensics tool needs (loki's full struct includes
// per-game commander lists and state summaries we don't consume here).
type ViolationRecord struct {
	Turn      int    `json:"turn"`
	Phase     string `json:"phase,omitempty"`
	Step      string `json:"step,omitempty"`
	Invariant string `json:"invariant"`
	Message   string `json:"message"`
}

// LoadReplay reads and decodes a JSON replay file. Returns a typed
// error so the CLI can distinguish missing-file from bad-JSON.
func LoadReplay(path string) (*Replay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read replay %q: %w", path, err)
	}
	var r Replay
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse replay %q: %w", path, err)
	}
	return &r, nil
}
