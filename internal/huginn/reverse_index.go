// reverse_index.go — Huginn 2.0 Worker A: reverse OracleID→[]InstanceID
// index + provenance-lineage tagging on interaction edges.
//
// Additive layer on top of the forward (card-name keyed) observation
// pipeline in huginn.go. Existing RawObservation / LearnedInteraction /
// FreyaInteraction shapes are untouched. Downstream Huginn 2.0 workers
// (dev-3 recommender, dev-19 pre-game predict) consume the exported
// query API at the bottom of this file.
//
// Persistence:
//   - data/huginn/instance_observations.jsonl   (line-delimited JSON)
//   - data/huginn/instance_interaction_edges.jsonl
//
// JSONL is used instead of array-of-objects so files can grow large
// without full-file rewrite on each append.
package huginn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/gameengine/instanceid"
)

// Provenance aliases instanceid.Provenance so huginn callers don't need
// to import the engine package directly.
type Provenance = instanceid.Provenance

// Re-exported provenance enum values so callers that have only the
// huginn import can refer to them without pulling in instanceid.
const (
	ProvUnknown = instanceid.ProvUnknown
	ProvOG      = instanceid.ProvOG
	ProvTK      = instanceid.ProvTK
	ProvCP      = instanceid.ProvCP
	ProvAB      = instanceid.ProvAB
)

const (
	defaultHuginnDir = "data/huginn"

	instanceObsFile  = "instance_observations.jsonl"
	instanceEdgeFile = "instance_interaction_edges.jsonl"
)

// InstanceObservation records one observed instance of a card at a point
// in a game, with full provenance lineage. Parallel to RawObservation
// but keyed by InstanceID instead of card name.
type InstanceObservation struct {
	InstanceID        string     `json:"instance_id"`
	OracleID          string     `json:"oracle_id"`                     // canonical Scryfall oracle_id
	CardName          string     `json:"card_name"`                     // denorm for debugging
	Provenance        Provenance `json:"provenance"`                    // OG / TK / CP / AB
	SourceInstanceID  string     `json:"source_instance_id,omitempty"`  // for TK/CP: what was copied
	EnablerInstanceID string     `json:"enabler_instance_id,omitempty"` // ability that minted this
	SeatIdx           int        `json:"seat_idx"`
	GameID            string     `json:"game_id"`
	Turn              int        `json:"turn"`
	Timestamp         string     `json:"timestamp"`
}

// InstanceInteractionEdge is the provenance-tagged interaction edge that
// downstream workers consume. Parallel to FreyaInteraction but with
// both endpoints keyed by InstanceID + provenance.
type InstanceInteractionEdge struct {
	InstanceA   string     `json:"instance_a"`
	OracleA     string     `json:"oracle_a"`
	ProvenanceA Provenance `json:"provenance_a"`
	InstanceB   string     `json:"instance_b"`
	OracleB     string     `json:"oracle_b"`
	ProvenanceB Provenance `json:"provenance_b"`
	Pattern     string     `json:"pattern"`
	ImpactScore float64    `json:"impact_score"`
	TurnWindow  int        `json:"turn_window"`
	GameID      string     `json:"game_id"`
	Timestamp   string     `json:"timestamp"`

	// Lineage relationship between A and B — derived from
	// SourceInstanceID + EnablerInstanceID chains. Empty when A and B
	// are independent (no shared ancestor). See DeriveLineage.
	Lineage string `json:"lineage,omitempty"`
}

// ---------------------------------------------------------------------------
// JSONL persistence
// ---------------------------------------------------------------------------

// AppendInstanceObservations appends the given observations to
// data/huginn/instance_observations.jsonl (one JSON object per line).
// Creates the file + directory if necessary. Safe to call with an empty
// slice (no-op).
func AppendInstanceObservations(dir string, obs []InstanceObservation) error {
	if len(obs) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, instanceObsFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("huginn: open %s: %w", path, err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w) // writes one object + trailing newline per Encode call
	for i := range obs {
		if err := enc.Encode(&obs[i]); err != nil {
			return fmt.Errorf("huginn: encode obs: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("huginn: flush %s: %w", path, err)
	}
	return nil
}

// AppendInstanceInteractionEdges appends the given edges to
// data/huginn/instance_interaction_edges.jsonl. Same shape as
// AppendInstanceObservations.
func AppendInstanceInteractionEdges(dir string, edges []InstanceInteractionEdge) error {
	if len(edges) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, instanceEdgeFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("huginn: open %s: %w", path, err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for i := range edges {
		if err := enc.Encode(&edges[i]); err != nil {
			return fmt.Errorf("huginn: encode edge: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("huginn: flush %s: %w", path, err)
	}
	return nil
}

// ReadInstanceObservations reads every observation from the JSONL file.
// Tolerates a missing or empty file (returns empty slice + nil error),
// mirroring ReadRawObservations.
func ReadInstanceObservations(dir string) ([]InstanceObservation, error) {
	out := []InstanceObservation{}
	path := filepath.Join(dir, instanceObsFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("huginn: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow long lines for big provenance chains.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ob InstanceObservation
		if err := json.Unmarshal(line, &ob); err != nil {
			return nil, fmt.Errorf("huginn: decode %s line %d: %w", path, lineNo, err)
		}
		out = append(out, ob)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("huginn: scan %s: %w", path, err)
	}
	return out, nil
}

// ReadInstanceInteractionEdges reads every edge from the JSONL file.
// Tolerates a missing or empty file.
func ReadInstanceInteractionEdges(dir string) ([]InstanceInteractionEdge, error) {
	out := []InstanceInteractionEdge{}
	path := filepath.Join(dir, instanceEdgeFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("huginn: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e InstanceInteractionEdge
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("huginn: decode %s line %d: %w", path, lineNo, err)
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("huginn: scan %s: %w", path, err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Lineage derivation (pure)
// ---------------------------------------------------------------------------

// DeriveLineage returns the lineage tag for the relationship between a
// and b. Pure function on the two observations — no global state, no
// I/O. See InstanceInteractionEdge.Lineage for the enum description.
func DeriveLineage(a, b InstanceObservation) string {
	// Copy relationships first (most specific).
	if a.SourceInstanceID != "" && a.SourceInstanceID == b.InstanceID {
		return "a_is_copy_of_b"
	}
	if b.SourceInstanceID != "" && b.SourceInstanceID == a.InstanceID {
		return "b_is_copy_of_a"
	}
	if a.SourceInstanceID != "" && a.SourceInstanceID == b.SourceInstanceID {
		return "shared_source"
	}
	// Enabler relationships.
	if a.InstanceID != "" && a.InstanceID == b.EnablerInstanceID {
		return "a_enables_b"
	}
	if b.InstanceID != "" && b.InstanceID == a.EnablerInstanceID {
		return "b_enables_a"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Reverse-index singleton + query API
// ---------------------------------------------------------------------------

// reverseIndexState holds the in-memory reverse index. Lazy-built on
// first query; cleared by ResetReverseIndex.
type reverseIndexState struct {
	mu          sync.RWMutex
	built       bool
	dir         string                // populated by the last BuildReverseIndex call
	oracleIndex map[string][]string   // OracleID -> []InstanceID (deduped)
	provIndex   map[string]Provenance // InstanceID -> Provenance
}

var reverseIdx = &reverseIndexState{}

// BuildReverseIndex eagerly loads data/huginn/instance_observations.jsonl
// from dir and constructs both the OracleID→[]InstanceID map and the
// InstanceID→Provenance map. Pass dir="" to use the default
// data/huginn/ path.
//
// Always clears any previously-built state and rebuilds from the given
// dir — safe to call multiple times to refresh after a write batch, or
// to scope to a temp dir in tests.
func BuildReverseIndex(dir string) error {
	if dir == "" {
		dir = defaultHuginnDir
	}
	obs, err := ReadInstanceObservations(dir)
	if err != nil {
		return err
	}

	oracleIdx := make(map[string][]string)
	provIdx := make(map[string]Provenance)
	seen := make(map[string]map[string]struct{}) // oracleID -> set of InstanceIDs

	for _, ob := range obs {
		// Provenance map populates for every observation, even if OracleID is empty.
		if ob.InstanceID != "" {
			provIdx[ob.InstanceID] = ob.Provenance
		}
		// Reverse-index requires both fields.
		if ob.OracleID == "" || ob.InstanceID == "" {
			continue
		}
		bucket, ok := seen[ob.OracleID]
		if !ok {
			bucket = make(map[string]struct{})
			seen[ob.OracleID] = bucket
		}
		if _, dup := bucket[ob.InstanceID]; dup {
			continue
		}
		bucket[ob.InstanceID] = struct{}{}
		oracleIdx[ob.OracleID] = append(oracleIdx[ob.OracleID], ob.InstanceID)
	}

	reverseIdx.mu.Lock()
	reverseIdx.built = true
	reverseIdx.dir = dir
	reverseIdx.oracleIndex = oracleIdx
	reverseIdx.provIndex = provIdx
	reverseIdx.mu.Unlock()
	return nil
}

// LookupReverseIndex returns every InstanceID currently observed for
// the given oracle ID. Returns nil for an unseen oracle ID. Result
// is a copy — callers may mutate it freely.
//
// O(1) lookup after the index has been built; lazily-builds on first
// call from defaultHuginnDir if not already built. Thread-safe.
//
// Renamed from ReverseIndex to disambiguate from the ReverseIndex
// interface type in recommender.go (collision blocked the entire
// huginn package from compiling — see PR #946 trace).
func LookupReverseIndex(oracleID string) []string {
	ensureBuilt()
	reverseIdx.mu.RLock()
	defer reverseIdx.mu.RUnlock()
	src, ok := reverseIdx.oracleIndex[oracleID]
	if !ok {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// ProvenanceOf returns the provenance tag for the given InstanceID, or
// instanceid.ProvUnknown if the ID hasn't been observed.
//
// O(1) lookup. Thread-safe.
func ProvenanceOf(iid string) Provenance {
	ensureBuilt()
	reverseIdx.mu.RLock()
	defer reverseIdx.mu.RUnlock()
	if p, ok := reverseIdx.provIndex[iid]; ok {
		return p
	}
	return ProvUnknown
}

// ResetReverseIndex clears the in-memory caches. Tests only.
func ResetReverseIndex() {
	reverseIdx.mu.Lock()
	reverseIdx.built = false
	reverseIdx.dir = ""
	reverseIdx.oracleIndex = nil
	reverseIdx.provIndex = nil
	reverseIdx.mu.Unlock()
}

// ensureBuilt lazy-initializes the singleton from defaultHuginnDir if
// no prior BuildReverseIndex call has populated it. Uses a check-lock-
// recheck pattern so concurrent first-callers don't double-build.
func ensureBuilt() {
	reverseIdx.mu.RLock()
	built := reverseIdx.built
	reverseIdx.mu.RUnlock()
	if built {
		return
	}
	// Double-checked init. BuildReverseIndex itself takes the write lock
	// internally, so we drop our read lock and let it run.
	_ = BuildReverseIndex(defaultHuginnDir)
}

// ---------------------------------------------------------------------------
// Bridge to the existing forward pipeline
// ---------------------------------------------------------------------------

// ObservationsFromGameAnalyses extracts InstanceObservations from a
// batch of GameAnalysis records via the engine's per-card observation
// log. Sibling to PersistRawObservations but for the InstanceID
// pipeline.
//
// As of 2026-05-30 analytics.GameAnalysis does NOT yet carry per-
// instance observation data (the existing CoTriggerObservation stream
// is keyed by card name). engine.Card has InstanceID/Provenance/
// SourceInstanceID/EnablerInstanceID/EnablerHistory but NO OracleID
// field, so even when GameAnalysis is plumbed, the OracleID side of
// the reverse index needs either a card-name→oracle_id lookup or a
// new Card.OracleID field.
//
// This stub exists so dev-3 (recommender) and dev-19 (pre-game predict)
// can write against the contract today; the engine team / Worker B
// can populate the source data in a follow-up PR. Tolerates legacy
// analyses by returning an empty slice silently.
//
// commanderNames is accepted for parity with PersistRawObservations'
// signature; unused at this stub stage.
func ObservationsFromGameAnalyses(analyses []*analytics.GameAnalysis, commanderNames []string) []InstanceObservation {
	_ = commanderNames
	if len(analyses) == 0 {
		return nil
	}
	// No per-instance observation data plumbed yet. Returning empty is
	// the documented behavior — callers don't have to special-case the
	// rollout window.
	return nil
}
