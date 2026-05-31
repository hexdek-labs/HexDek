package tournament

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// loadTelemetryFixture loads the canonical Swiss telemetry fixture
// from testdata/. Returns a parsed TournamentResult ready for
// ComputeTelemetry to consume.
func loadTelemetryFixture(t *testing.T) *TournamentResult {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "swiss_converged_fixture.json"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var r TournamentResult
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &r
}

func TestComputeTelemetry_NilSafe(t *testing.T) {
	if got := ComputeTelemetry(nil, nil); got != nil {
		t.Errorf("ComputeTelemetry(nil) should return nil, got %+v", got)
	}
}

func TestComputeTelemetry_BasicShape(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	if tel == nil {
		t.Fatal("ComputeTelemetry returned nil for valid result")
	}
	if tel.SchemaVersion != TelemetrySchemaVersion {
		t.Errorf("schema version: got %q, want %q", tel.SchemaVersion, TelemetrySchemaVersion)
	}
	if tel.SourceMode != ModeSwiss {
		t.Errorf("source_mode: got %q, want %q", tel.SourceMode, ModeSwiss)
	}
	if tel.Games != 12 {
		t.Errorf("games: got %d, want 12", tel.Games)
	}
	if len(tel.Commanders) != 4 {
		t.Errorf("commanders: got %d rows, want 4", len(tel.Commanders))
	}
}

func TestComputeTelemetry_CommandersSortedByELO(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	for i := 1; i < len(tel.Commanders); i++ {
		if tel.Commanders[i].ELO > tel.Commanders[i-1].ELO {
			t.Errorf("commanders not sorted by ELO desc: %s(%.0f) > %s(%.0f)",
				tel.Commanders[i].Name, tel.Commanders[i].ELO,
				tel.Commanders[i-1].Name, tel.Commanders[i-1].ELO)
		}
	}
	// Sanity: top deck is Atraxa per fixture.
	if tel.Commanders[0].Name != "Atraxa" {
		t.Errorf("top deck = %q, want Atraxa", tel.Commanders[0].Name)
	}
	if tel.Commanders[0].WinRate <= 0 {
		t.Errorf("Atraxa winrate should be > 0, got %f", tel.Commanders[0].WinRate)
	}
}

func TestComputeTelemetry_Convergence(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	c := tel.Convergence

	wantInitial := 25.0 / 3.0
	if math.Abs(c.InitialSigma-wantInitial) > 1e-9 {
		t.Errorf("initial_sigma: got %f, want %f", c.InitialSigma, wantInitial)
	}
	// Fixture has σ values 3.8, 4.0, 4.1, 4.2 → mean ≈ 4.025.
	wantMean := (3.8 + 4.0 + 4.1 + 4.2) / 4
	if math.Abs(c.MeanSigma-wantMean) > 1e-9 {
		t.Errorf("mean_sigma: got %f, want %f", c.MeanSigma, wantMean)
	}
	// (1 - 4.025/8.333) ≈ 0.517 — just above the 0.5 converged threshold.
	wantRatio := 1.0 - (wantMean / wantInitial)
	if math.Abs(c.ConvergenceRatio-wantRatio) > 1e-9 {
		t.Errorf("convergence_ratio: got %f, want %f", c.ConvergenceRatio, wantRatio)
	}
	if !c.Converged {
		t.Errorf("fixture should report Converged=true (ratio %f >= %f)", c.ConvergenceRatio, SigmaConvergedRatio)
	}
	if c.MinSigma != 3.8 {
		t.Errorf("min_sigma: got %f, want 3.8", c.MinSigma)
	}
	if c.MaxSigma != 4.2 {
		t.Errorf("max_sigma: got %f, want 4.2", c.MaxSigma)
	}
}

func TestComputeTelemetry_ELODistribution(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	d := tel.ELODistribution

	// Fixture: 1620, 1510, 1470, 1400.
	if d.Min != 1400 || d.Max != 1620 {
		t.Errorf("ELO min/max: got %f/%f, want 1400/1620", d.Min, d.Max)
	}
	wantMean := (1620.0 + 1510 + 1470 + 1400) / 4
	if math.Abs(d.Mean-wantMean) > 1e-9 {
		t.Errorf("ELO mean: got %f, want %f", d.Mean, wantMean)
	}
	if d.Spread != 220 {
		t.Errorf("ELO spread: got %f, want 220", d.Spread)
	}
	if d.StdDev <= 0 {
		t.Errorf("ELO std_dev should be > 0, got %f", d.StdDev)
	}
}

func TestComputeTelemetry_ArchetypeMatrix(t *testing.T) {
	r := loadTelemetryFixture(t)
	archetypes := []Archetype{
		ArchetypeComboInfinite, // Atraxa
		ArchetypeAggroGoWide,   // Krenko
		ArchetypeComboInfinite, // Yuriko (grouped with Atraxa)
		ArchetypeControl,       // Edric
	}
	tel := ComputeTelemetry(r, archetypes)
	if tel.ArchetypeMatrix == nil {
		t.Fatal("archetype matrix should be populated when archetypes provided")
	}
	combo := tel.ArchetypeMatrix[string(ArchetypeComboInfinite)]
	if combo.Decks != 2 {
		t.Errorf("ComboInfinite decks: got %d, want 2", combo.Decks)
	}
	// Atraxa 6 wins + Yuriko 2 wins = 8.
	if combo.Wins != 8 {
		t.Errorf("ComboInfinite wins: got %d, want 8", combo.Wins)
	}
	// 12 games per deck × 2 decks = 24.
	if combo.Games != 24 {
		t.Errorf("ComboInfinite games: got %d, want 24", combo.Games)
	}
	wantWR := 8.0 / 24
	if math.Abs(combo.WinRate-wantWR) > 1e-9 {
		t.Errorf("ComboInfinite winrate: got %f, want %f", combo.WinRate, wantWR)
	}
}

func TestComputeTelemetry_ArchetypeMatrix_SkippedWhenAbsent(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	if tel.ArchetypeMatrix != nil {
		t.Errorf("archetype matrix should be nil when archetypes not provided, got %+v", tel.ArchetypeMatrix)
	}
}

func TestComputeTelemetry_RoundsPopulated(t *testing.T) {
	r := loadTelemetryFixture(t)
	tel := ComputeTelemetry(r, nil)
	if len(tel.Rounds) != 3 {
		t.Fatalf("rounds: got %d, want 3", len(tel.Rounds))
	}
	// Round numbers must be 1, 2, 3 in order.
	for i, rt := range tel.Rounds {
		if rt.Round != i+1 {
			t.Errorf("round[%d].Round = %d, want %d", i, rt.Round, i+1)
		}
	}

	// Sigma must DECREASE across rounds (convergence). Fixture
	// round means: r1 ≈ 6.675, r2 ≈ 5.7, r3 = 4.025.
	for i := 1; i < len(tel.Rounds); i++ {
		if tel.Rounds[i].SigmaMean >= tel.Rounds[i-1].SigmaMean {
			t.Errorf("sigma mean should decrease across rounds; r%d=%f r%d=%f",
				tel.Rounds[i-1].Round, tel.Rounds[i-1].SigmaMean,
				tel.Rounds[i].Round, tel.Rounds[i].SigmaMean)
		}
	}

	// Mu spread must INCREASE across rounds (decks differentiating).
	for i := 1; i < len(tel.Rounds); i++ {
		if tel.Rounds[i].MuSpread <= tel.Rounds[i-1].MuSpread {
			t.Errorf("mu spread should increase across rounds; r%d=%f r%d=%f",
				tel.Rounds[i-1].Round, tel.Rounds[i-1].MuSpread,
				tel.Rounds[i].Round, tel.Rounds[i].MuSpread)
		}
	}

	// Final-round winrates must match the aggregate WinsByCommander /
	// GamesPlayedByCommander.
	final := tel.Rounds[len(tel.Rounds)-1]
	for _, c := range tel.Commanders {
		got := final.WinRates[c.Name]
		if math.Abs(got-c.WinRate) > 1e-9 {
			t.Errorf("final-round winrate(%s)=%f != aggregate winrate %f", c.Name, got, c.WinRate)
		}
	}
}

func TestComputeTelemetry_RoundsEmptyWithoutSnapshots(t *testing.T) {
	r := loadTelemetryFixture(t)
	r.RoundSnapshots = nil
	tel := ComputeTelemetry(r, nil)
	if len(tel.Rounds) != 0 {
		t.Errorf("rounds should be empty when no snapshots; got %d", len(tel.Rounds))
	}
}

func TestComputeTelemetry_NoTrueSkillData(t *testing.T) {
	r := loadTelemetryFixture(t)
	r.TrueSkill = nil
	tel := ComputeTelemetry(r, nil)
	// Convergence should report zero stats when no TS data.
	if tel.Convergence.MeanSigma != 0 {
		t.Errorf("no-TS: mean_sigma should be 0, got %f", tel.Convergence.MeanSigma)
	}
	if tel.Convergence.Converged {
		t.Error("no-TS: should not report converged")
	}
	// ELO distribution should still populate.
	if tel.ELODistribution.Spread != 220 {
		t.Errorf("no-TS: ELO spread should still be 220, got %f", tel.ELODistribution.Spread)
	}
}

func TestComputeTelemetry_JSONRoundTrip(t *testing.T) {
	r := loadTelemetryFixture(t)
	archetypes := []Archetype{
		ArchetypeComboInfinite, ArchetypeAggroGoWide,
		ArchetypeComboInfinite, ArchetypeControl,
	}
	tel := ComputeTelemetry(r, archetypes)

	b, err := json.Marshal(tel)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(b)
	// Pin the canonical snake_case keys per the schema doc.
	for _, k := range []string{
		`"schema_version"`, `"source_mode"`, `"games"`,
		`"commanders"`, `"convergence"`, `"elo_distribution"`,
		`"archetype_matrix"`, `"rounds"`,
		`"initial_sigma"`, `"mean_sigma"`, `"convergence_ratio"`, `"converged"`,
		`"spread"`, `"std_dev"`, `"win_rate"`,
	} {
		if !strings.Contains(body, k) {
			t.Errorf("expected key %s in JSON, got: %s", k, body)
		}
	}

	// Round-trip back into a fresh Telemetry struct.
	var back Telemetry
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.SchemaVersion != tel.SchemaVersion || back.Games != tel.Games {
		t.Errorf("round-trip mismatch: orig sv=%q games=%d back sv=%q games=%d",
			tel.SchemaVersion, tel.Games, back.SchemaVersion, back.Games)
	}
}

// TestComputeTelemetry_SwissRealRunPopulatesRounds runs a real
// RunSwiss and confirms RoundSnapshots are populated end-to-end so
// the per-round telemetry actually fires in production. Skips when
// the corpus is unavailable.
func TestComputeTelemetry_SwissRealRunPopulatesRounds(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunSwiss(SwissConfig{
		Decks:         decks,
		NSeats:        4,
		Rounds:        2,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunSwiss: %v", err)
	}
	if len(r.RoundSnapshots) != 2 {
		t.Fatalf("expected 2 round snapshots, got %d", len(r.RoundSnapshots))
	}
	// Round numbers must be 1, 2 (not 0, 1).
	for i, snap := range r.RoundSnapshots {
		if snap.Round != i+1 {
			t.Errorf("round_snapshots[%d].round = %d, want %d", i, snap.Round, i+1)
		}
		if len(snap.Ratings) == 0 {
			t.Errorf("round %d ratings empty", snap.Round)
		}
	}
	tel := ComputeTelemetry(r, nil)
	if tel == nil {
		t.Fatal("ComputeTelemetry returned nil")
	}
	if len(tel.Rounds) != 2 {
		t.Errorf("derived rounds: got %d, want 2", len(tel.Rounds))
	}
}

// Ensure deterministic iteration when building schema-doc examples
// that list ArchetypeMatrix keys.
func TestComputeTelemetry_ArchetypeKeysSortable(t *testing.T) {
	r := loadTelemetryFixture(t)
	archetypes := []Archetype{
		ArchetypeComboInfinite, ArchetypeAggroGoWide,
		ArchetypeComboInfinite, ArchetypeControl,
	}
	tel := ComputeTelemetry(r, archetypes)
	keys := make([]string, 0, len(tel.ArchetypeMatrix))
	for k := range tel.ArchetypeMatrix {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) != 3 {
		t.Errorf("expected 3 archetypes, got %d", len(keys))
	}
}
