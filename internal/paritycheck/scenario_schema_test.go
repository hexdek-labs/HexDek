package paritycheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scenario_schema_test.go — schema-validation pin for the
// cross-engine parity scenario corpus (Verification Phase 4 prep).
// See docs/cross-engine-parity-setup.md §3-4 for the corpus design
// and full schema spec.
//
// Catches scenario-file mistakes during PR review WITHOUT requiring
// an xmage adapter or actually running any games. Runs in CI on
// every push.

// scenarioFile mirrors the JSON shape every scenario in
// data/parity-scenarios/ must conform to. New fields go here;
// optional fields stay pointer-typed so a missing-field test is
// straightforward (== nil).
type scenarioFile struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Mode        string   `json:"mode"`
	Tags        []string `json:"tags"`
	// deck_seed mode fields.
	DeckPool  []string `json:"deck_pool,omitempty"`
	NSeats    int      `json:"n_seats,omitempty"`
	Seed      int64    `json:"seed,omitempty"`
	MaxTurns  int      `json:"max_turns,omitempty"`
	// state_inject mode fields (reserved — engine round-trip not
	// yet shipped).
	StatePath        string   `json:"state_path,omitempty"`
	EventsToExecute  []string `json:"events_to_execute,omitempty"`
	// Outcome envelope (both modes).
	ExpectedOutcome scenarioExpectedOutcome `json:"expected_outcome"`
}

type scenarioExpectedOutcome struct {
	WinnerSeatIn []int    `json:"winner_seat_in"`
	MinTurns     int      `json:"min_turns"`
	MaxTurns     int      `json:"max_turns"`
	EndReasonIn  []string `json:"end_reason_in"`
}

// allowedModes is the closed set of mode values. Unknown modes are
// rejected — the scenario corpus is small enough that we don't
// want silent acceptance of typos.
var allowedModes = map[string]bool{
	"deck_seed":    true,
	"state_inject": true,
}

// allowedTags is the closed set of category tags from
// docs/cross-engine-parity-setup.md §3. New tags need a doc update
// + this list update + a justification of what surface they cover.
var allowedTags = map[string]bool{
	"combat-fundamentals":   true,
	"mass-effects":          true,
	"etb-cascade":           true,
	"combo-wincons":         true,
	"replacement-effects":   true,
	"stack-interactions":    true,
	"multiplayer-dynamics":  true,
	"edge-cases":            true,
	// Cross-cutting tag — every Phase-4 bootstrap scenario carries
	// this so the CI entry point can filter to the bootstrap subset.
	"phase4-bootstrap": true,
}

// allowedEndReasons is the closed set of EndReason strings the
// scenario can list in `end_reason_in`. Must match the actual
// strings the engine writes to Outcome.EndReason at game end.
var allowedEndReasons = map[string]bool{
	"life_zero":           true,
	"commander_damage":    true,
	"poison":              true,
	"concede":             true,
	"library_empty":       true,
	"mandatory_loop_draw": true,
	"loss_effect":         true,
}

const scenarioDir = "../../data/parity-scenarios"

// loadScenarios walks scenarioDir, decoding every .json file
// (excluding the .baseline/ subdir for per-scenario Go baselines).
// Returns the file path + decoded struct for each, so test
// failures can name the offending file.
func loadScenarios(t *testing.T) []struct {
	path string
	f    scenarioFile
} {
	t.Helper()
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v (this PR ships the dir as part of the scaffolding; missing dir means a previous step regressed)", scenarioDir, err)
	}
	var out []struct {
		path string
		f    scenarioFile
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if filepath.Ext(ent.Name()) != ".json" {
			continue
		}
		p := filepath.Join(scenarioDir, ent.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var f scenarioFile
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("decode %s: %v", p, err)
		}
		out = append(out, struct {
			path string
			f    scenarioFile
		}{p, f})
	}
	return out
}

// TestScenarioCorpus_NonEmpty pins the scenario count floor at 15.
// docs/cross-engine-parity-setup.md §3 promises 17; floor at 15
// leaves room for two scenarios to be removed during a future audit
// without forcing test churn.
func TestScenarioCorpus_NonEmpty(t *testing.T) {
	scenarios := loadScenarios(t)
	if len(scenarios) < 15 {
		t.Errorf("scenario corpus has %d entries, want >= 15 — the Phase 4 prep PR promised 17 (see docs/cross-engine-parity-setup.md §3)", len(scenarios))
	}
}

// TestScenarioCorpus_NamesUnique pins the uniqueness of `name`
// across the corpus. Duplicate names break the
// data/parity-scenarios/.baseline/<name>.json indexing scheme.
func TestScenarioCorpus_NamesUnique(t *testing.T) {
	scenarios := loadScenarios(t)
	seen := map[string]string{}
	for _, s := range scenarios {
		if prior, dup := seen[s.f.Name]; dup {
			t.Errorf("duplicate scenario name %q (%s also has it)", s.f.Name, prior)
		}
		seen[s.f.Name] = s.path
	}
}

// TestScenarioCorpus_FilenameMatchesName pins the filename ↔ name
// invariant. Mismatches make `scripts/parity-test.sh --tag` filter
// behavior unpredictable.
func TestScenarioCorpus_FilenameMatchesName(t *testing.T) {
	scenarios := loadScenarios(t)
	for _, s := range scenarios {
		base := strings.TrimSuffix(filepath.Base(s.path), ".json")
		if base != s.f.Name {
			t.Errorf("%s: filename basename %q must match name field %q", s.path, base, s.f.Name)
		}
	}
}

// TestScenarioCorpus_RequiredFieldsPresent pins schema conformance
// — every scenario must have name + description + mode + tags +
// expected_outcome.* populated. Catches truncated scenario files.
func TestScenarioCorpus_RequiredFieldsPresent(t *testing.T) {
	scenarios := loadScenarios(t)
	for _, s := range scenarios {
		f := s.f
		if f.Name == "" {
			t.Errorf("%s: name is empty", s.path)
		}
		if f.Description == "" {
			t.Errorf("%s: description is empty", s.path)
		}
		if !allowedModes[f.Mode] {
			t.Errorf("%s: mode = %q, want one of [deck_seed state_inject]", s.path, f.Mode)
		}
		if len(f.Tags) == 0 {
			t.Errorf("%s: tags must list at least one category tag", s.path)
		}
		for _, tag := range f.Tags {
			if !allowedTags[tag] {
				t.Errorf("%s: tag %q is not in the allowed set (see scenario_schema_test.allowedTags)", s.path, tag)
			}
		}
		if f.ExpectedOutcome.MinTurns < 0 || f.ExpectedOutcome.MaxTurns <= 0 {
			t.Errorf("%s: expected_outcome.min/max_turns = %d/%d (want non-negative min, positive max)", s.path, f.ExpectedOutcome.MinTurns, f.ExpectedOutcome.MaxTurns)
		}
		if f.ExpectedOutcome.MinTurns > f.ExpectedOutcome.MaxTurns {
			t.Errorf("%s: expected_outcome.min_turns (%d) > max_turns (%d)", s.path, f.ExpectedOutcome.MinTurns, f.ExpectedOutcome.MaxTurns)
		}
		for _, r := range f.ExpectedOutcome.EndReasonIn {
			if !allowedEndReasons[r] {
				t.Errorf("%s: end_reason_in includes %q which is not in the allowed set", s.path, r)
			}
		}
		if len(f.ExpectedOutcome.WinnerSeatIn) == 0 {
			t.Errorf("%s: expected_outcome.winner_seat_in must list at least one allowed seat (use [-1] for draw-only)", s.path)
		}
	}
}

// TestScenarioCorpus_DeckSeedModeFieldsPresent pins the deck_seed-
// specific schema. Required fields for that mode: deck_pool,
// n_seats, seed, max_turns. deck_pool length must match n_seats.
func TestScenarioCorpus_DeckSeedModeFieldsPresent(t *testing.T) {
	scenarios := loadScenarios(t)
	for _, s := range scenarios {
		if s.f.Mode != "deck_seed" {
			continue
		}
		f := s.f
		if f.NSeats < 2 || f.NSeats > 4 {
			t.Errorf("%s: n_seats = %d, want in [2, 4]", s.path, f.NSeats)
		}
		if len(f.DeckPool) < f.NSeats {
			t.Errorf("%s: deck_pool has %d entries, n_seats = %d — pool must be >= n_seats", s.path, len(f.DeckPool), f.NSeats)
		}
		if f.MaxTurns <= 0 {
			t.Errorf("%s: max_turns = %d, want > 0", s.path, f.MaxTurns)
		}
		// Decks must resolve to actual files. Repo paths are
		// rooted at the worktree, so the test runs from
		// internal/paritycheck/ — climb to repo root.
		for _, d := range f.DeckPool {
			repoPath := filepath.Join("..", "..", d)
			if _, err := os.Stat(repoPath); err != nil {
				t.Errorf("%s: deck %q does not resolve (tried %s): %v", s.path, d, repoPath, err)
			}
		}
	}
}

// TestScenarioCorpus_StateInjectModeNotShippedYet pins the
// post-engine-round-trip status. Any scenario shipping today as
// `mode: "state_inject"` is premature — the engine's
// MarshalGameState / UnmarshalGameState entry points don't exist
// yet. Section 2.3 of the setup doc explicitly flags this.
func TestScenarioCorpus_StateInjectModeNotShippedYet(t *testing.T) {
	scenarios := loadScenarios(t)
	for _, s := range scenarios {
		if s.f.Mode == "state_inject" {
			t.Errorf("%s: mode = state_inject but engine GameState JSON round-trip is not yet shipped (see docs/cross-engine-parity-setup.md §2.3). This scenario should be `mode: deck_seed` for now or reverted from the corpus.", s.path)
		}
	}
}

// TestScenarioCorpus_TagDistribution surfaces the per-tag count in
// the test log. Not a hard assertion — the log line lets a future
// auditor see the tag distribution at a glance without re-running
// the corpus.
func TestScenarioCorpus_TagDistribution(t *testing.T) {
	scenarios := loadScenarios(t)
	counts := map[string]int{}
	for _, s := range scenarios {
		for _, tag := range s.f.Tags {
			counts[tag]++
		}
	}
	t.Logf("parity scenario tag distribution (%d scenarios total):", len(scenarios))
	for tag, n := range counts {
		t.Logf("  %s: %d", tag, n)
	}
	// phase4-bootstrap is cross-cutting — every current scenario
	// carries it. Catches a scenario that forgets the bootstrap
	// tag and therefore wouldn't run under
	// `scripts/parity-test.sh --tag phase4-bootstrap`.
	if counts["phase4-bootstrap"] != len(scenarios) {
		t.Errorf("phase4-bootstrap tag: %d scenarios carry it, want all %d (cross-cutting CI filter tag)", counts["phase4-bootstrap"], len(scenarios))
	}
}

// ensure the err formatting helper is reachable even if not used.
var _ = fmt.Sprintf
