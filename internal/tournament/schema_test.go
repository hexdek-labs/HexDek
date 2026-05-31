package tournament

import (
	"encoding/json"
	"strings"
	"testing"
)

// makeMinimalValid returns the smallest TournamentResult that
// passes ValidateSchema. Tests mutate one field at a time off this
// baseline to exercise individual validation rules.
func makeMinimalValid(mode string) *TournamentResult {
	r := &TournamentResult{
		SchemaVersion:   SchemaVersion,
		Mode:            mode,
		NSeats:          4,
		Games:           1,
		CommanderNames:  []string{"A", "B", "C", "D"},
		WinsByCommander: map[string]int{"A": 1},
	}
	switch mode {
	case ModeSwiss:
		r.SwissStandings = []SwissStanding{{CommanderName: "A", Points: 3}}
	case ModeDoubleElim:
		r.EliminationStandings = []EliminationStanding{{CommanderName: "A", FinalRank: 1}}
	case ModeBalancedPool:
		r.BalancedPodPlan = []BalancedPod{{DeckIndices: []int{0, 1, 2, 3}, CommanderNames: []string{"A", "B", "C", "D"}}}
	}
	return r
}

// TestValidateSchema_AllModesAcceptMinimal pins that the
// hand-built minimal-valid result passes for every mode. Future
// schema changes that add a required field surface as a test
// failure across every mode at once.
func TestValidateSchema_AllModesAcceptMinimal(t *testing.T) {
	for mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			if err := ValidateSchema(makeMinimalValid(mode)); err != nil {
				t.Errorf("minimal-valid %s should pass: %v", mode, err)
			}
		})
	}
}

func TestValidateSchema_NilRejected(t *testing.T) {
	if err := ValidateSchema(nil); err == nil {
		t.Fatal("nil result should fail")
	}
}

func TestValidateSchema_EmptyVersionRejected(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	r.SchemaVersion = ""
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "schema_version: empty") {
		t.Errorf("expected empty-version diagnostic, got: %v", err)
	}
}

func TestValidateSchema_VersionMismatchRejected(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	r.SchemaVersion = "0.0.1"
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "schema_version: got") {
		t.Errorf("expected version-mismatch diagnostic, got: %v", err)
	}
}

func TestValidateSchema_EmptyModeRejected(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	r.Mode = ""
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "mode: empty") {
		t.Errorf("expected empty-mode diagnostic, got: %v", err)
	}
}

func TestValidateSchema_UnknownModeRejected(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	r.Mode = "bogus_mode"
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "not in valid set") {
		t.Errorf("expected unknown-mode diagnostic, got: %v", err)
	}
}

func TestValidateSchema_NSeatsBelow2(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	r.NSeats = 1
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "n_seats: 1") {
		t.Errorf("expected nseats diagnostic, got: %v", err)
	}
}

func TestValidateSchema_WinsConservation(t *testing.T) {
	// sum(wins) + draws > games — corrupt accounting.
	r := makeMinimalValid(ModeRotate)
	r.Games = 1
	r.Draws = 0
	r.WinsByCommander = map[string]int{"A": 2}
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "wins_conservation") {
		t.Errorf("expected conservation diagnostic, got: %v", err)
	}
}

func TestValidateSchema_SwissMissingStandings(t *testing.T) {
	r := makeMinimalValid(ModeSwiss)
	r.SwissStandings = nil
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "swiss_standings: nil for mode=swiss") {
		t.Errorf("expected missing-standings diagnostic, got: %v", err)
	}
}

func TestValidateSchema_CrossModeLeakage(t *testing.T) {
	// Rotate mode carrying SwissStandings is a corrupt or
	// hand-merged payload — must be flagged.
	r := makeMinimalValid(ModeRotate)
	r.SwissStandings = []SwissStanding{{CommanderName: "A"}}
	err := ValidateSchema(r)
	if err == nil || !strings.Contains(err.Error(), "swiss_standings: populated under mode=rotate") {
		t.Errorf("expected leakage diagnostic, got: %v", err)
	}
}

func TestValidateSchema_AllErrorsJoined(t *testing.T) {
	// Multiple violations must surface in a single error message
	// (not fail-fast), so a CI run reports the whole gap list at once.
	r := makeMinimalValid(ModeRotate)
	r.SchemaVersion = ""
	r.Mode = ""
	r.NSeats = 0
	err := ValidateSchema(r)
	if err == nil {
		t.Fatal("expected multi-violation error")
	}
	// Each independent violation must appear in the joined message.
	for _, sub := range []string{"schema_version: empty", "mode: empty", "n_seats: 0"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("expected substring %q in error, got: %v", sub, err)
		}
	}
}

// TestSchema_JSONShape_SnakeCase pins that the canonical fields
// marshal with snake_case keys (not Go capitalization). Schema doc
// promises snake_case to downstream consumers.
func TestSchema_JSONShape_SnakeCase(t *testing.T) {
	r := makeMinimalValid(ModeRotate)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := []string{
		`"schema_version"`,
		`"mode"`,
		`"games"`,
		`"crashes"`,
		`"draws"`,
		`"avg_turns"`,
		`"commander_names"`,
		`"wins_by_commander"`,
		`"n_seats"`,
		`"duration_ns"`,
		`"games_per_second"`,
		`"turn_distribution"`,
	}
	body := string(b)
	for _, k := range want {
		if !strings.Contains(body, k) {
			t.Errorf("expected key %s in JSON, got: %s", k, body)
		}
	}
	// And the Go-default capitalized names must NOT appear.
	for _, k := range []string{`"SchemaVersion"`, `"Mode"`, `"Games"`, `"NSeats"`} {
		if strings.Contains(body, k) {
			t.Errorf("unexpected capitalized key %s leaked into JSON", k)
		}
	}
}

func TestSchema_RoundTripJSON(t *testing.T) {
	// Marshal → Unmarshal → re-validate. Confirms the JSON shape
	// is lossless across the canonical types we surface.
	orig := makeMinimalValid(ModeSwiss)
	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back TournamentResult
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ValidateSchema(&back); err != nil {
		t.Errorf("round-tripped result should still validate: %v", err)
	}
	if back.SchemaVersion != orig.SchemaVersion || back.Mode != orig.Mode || back.NSeats != orig.NSeats {
		t.Errorf("round-trip mismatch: orig=%+v back=%+v", orig, back)
	}
}
