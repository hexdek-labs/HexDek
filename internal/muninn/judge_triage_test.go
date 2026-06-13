package muninn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

func writeTriageLog(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "grinder-violations.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Same bug at different seeds/turns/timestamps must collapse into ONE
// cluster: the fingerprint keys on dimension+rule+object, never the
// repro context.
func TestJudgeTriage_DedupeAcrossSeeds(t *testing.T) {
	log := writeTriageLog(t,
		`{"ts":"2026-06-10T01:00:00Z","game_seed":41,"sampled_game":7,"deck_keys":["a","b"],"dimension":"conservation","surface":"invariants","rule":"ZoneConservation","severity":"critical","turn":12,"seat":3,"detail":"seat 3 census reads 4 cards low"}`,
		`{"ts":"2026-06-12T09:00:00Z","game_seed":99,"sampled_game":31,"deck_keys":["c","d"],"dimension":"conservation","surface":"invariants","rule":"ZoneConservation","severity":"critical","turn":47,"seat":1,"detail":"seat 1 census reads 2 cards low"}`,
	)
	tr, err := TriageJudgeLog(log, "")
	if err != nil {
		t.Fatal(err)
	}
	if tr.TotalRecords != 2 {
		t.Fatalf("TotalRecords = %d, want 2", tr.TotalRecords)
	}
	if len(tr.Clusters) != 1 {
		t.Fatalf("clusters = %d, want 1 (same bug, different seed must dedupe)", len(tr.Clusters))
	}
	c := tr.Clusters[0]
	if c.Count != 2 {
		t.Fatalf("Count = %d, want 2", c.Count)
	}
	if c.FirstSeen != "2026-06-10T01:00:00Z" || c.LastSeen != "2026-06-12T09:00:00Z" {
		t.Fatalf("first/last seen = %s / %s", c.FirstSeen, c.LastSeen)
	}
	// Representative repro = the first occurrence.
	if c.ReproSeed != 41 || c.ReproGame != 7 || c.ReproTurn != 12 {
		t.Fatalf("repro = seed %d game %d turn %d, want 41/7/12", c.ReproSeed, c.ReproGame, c.ReproTurn)
	}
	if !strings.Contains(c.Fingerprint, "conservation|ZoneConservation|") {
		t.Fatalf("fingerprint = %q", c.Fingerprint)
	}
	if strings.ContainsAny(c.Fingerprint, "0123456789") {
		t.Fatalf("fingerprint carries raw repro digits: %q", c.Fingerprint)
	}
}

// Distinct cards under the same rule must stay distinct clusters — the
// card IS the object signature when present.
func TestJudgeTriage_CardsSeparateClusters(t *testing.T) {
	log := writeTriageLog(t,
		`{"ts":"2026-06-10T01:00:00Z","game_seed":1,"dimension":"progression","rule":"ally_etb/fire","card":"Ambuscade Shaman","detail":"expected fire, got silent"}`,
		`{"ts":"2026-06-10T02:00:00Z","game_seed":2,"dimension":"progression","rule":"ally_etb/fire","card":"Kalastria Healer","detail":"expected fire, got silent"}`,
		`{"ts":"2026-06-10T03:00:00Z","game_seed":3,"dimension":"progression","rule":"ally_etb/fire","card":"Ambuscade Shaman","detail":"expected fire, got silent"}`,
	)
	tr, err := TriageJudgeLog(log, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2 (one per card)", len(tr.Clusters))
	}
	// Ranked by count within the dimension: Ambuscade (2) first.
	if tr.Clusters[0].Object != "Ambuscade Shaman" || tr.Clusters[0].Count != 2 {
		t.Fatalf("top cluster = %q ×%d", tr.Clusters[0].Object, tr.Clusters[0].Count)
	}
}

// All six dimensions classify and present in canonical order; an empty
// dimension defaults to state_integrity (the watchdog's own default).
func TestJudgeTriage_DimensionClassification(t *testing.T) {
	log := writeTriageLog(t,
		`{"dimension":"liveness","rule":"wall_clock","detail":"hung"}`,
		`{"dimension":"outcome","rule":"etb_with_counters","card":"X","detail":"got 0"}`,
		`{"dimension":"progression","rule":"etb/fire","card":"Y","detail":"silent"}`,
		`{"dimension":"","rule":"LegacyRow","detail":"no dimension stamped"}`,
		`{"dimension":"state_integrity","rule":"StaleAttachment","detail":"aura points off-battlefield"}`,
		`{"dimension":"conservation","rule":"ZoneConservation","detail":"seat 2 low"}`,
		`{"dimension":"legality","rule":"704.5f","detail":"illegal cast"}`,
	)
	tr, err := TriageJudgeLog(log, "")
	if err != nil {
		t.Fatal(err)
	}
	if tr.ByDimension[judge.DimensionStateIntegrity] != 2 {
		t.Fatalf("state_integrity = %d, want 2 (incl. empty-dimension default)", tr.ByDimension[judge.DimensionStateIntegrity])
	}
	var got []string
	for _, c := range tr.Clusters {
		got = append(got, c.Dimension)
	}
	want := []string{"legality", "conservation", "state_integrity", "state_integrity", "progression", "outcome", "liveness"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("dimension order = %v, want %v", got, want)
	}
}

// Missing stream + no archive = zero-record triage, and the artifact
// writer must NO-OP (not clobber prior artifacts with emptiness).
func TestJudgeTriage_EmptyStreamNoOp(t *testing.T) {
	dir := t.TempDir()
	tr, err := TriageJudgeLog(filepath.Join(dir, "does-not-exist.jsonl"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if tr.TotalRecords != 0 || len(tr.Clusters) != 0 {
		t.Fatalf("empty stream produced records: %+v", tr)
	}
	md, js, err := WriteJudgeTriage(dir, tr)
	if err != nil {
		t.Fatal(err)
	}
	if md != "" || js != "" {
		t.Fatalf("zero-record triage wrote artifacts: %q %q", md, js)
	}
	if _, err := os.Stat(filepath.Join(dir, "judge-triage.md")); !os.IsNotExist(err) {
		t.Fatal("judge-triage.md exists after no-op")
	}
}

// Malformed lines are counted and skipped, never fatal.
func TestJudgeTriage_MalformedSkipped(t *testing.T) {
	log := writeTriageLog(t,
		`{"dimension":"legality","rule":"704.5f","detail":"ok line"}`,
		`{not json at all`,
	)
	tr, err := TriageJudgeLog(log, "")
	if err != nil {
		t.Fatal(err)
	}
	if tr.TotalRecords != 1 || tr.Malformed != 1 {
		t.Fatalf("records/malformed = %d/%d, want 1/1", tr.TotalRecords, tr.Malformed)
	}
}

// Muninn's own invariant_violations.json archive folds into the SAME
// cluster space — one triage view over both bookkeeping paths, and the
// archive row now carries the canonical Dimension through.
func TestJudgeTriage_ArchiveUnification(t *testing.T) {
	dir := t.TempDir()
	v := judge.ValidationViolation{
		Surface: judge.SurfaceFeynman, Name: "ZoneConservation",
		Severity: judge.SeverityCritical, Message: "seat 2 census reads 3 cards low",
		Dimension: judge.DimensionConservation,
	}
	av := NewArchivedViolation(77, [4]string{"deckA", "deckB"}, "2026-06-09T00:00:00Z", v)
	if av.Dimension != judge.DimensionConservation {
		t.Fatalf("NewArchivedViolation dropped Dimension: %+v", av)
	}
	if err := PersistInvariantViolations(dir, []ArchivedViolation{av}); err != nil {
		t.Fatal(err)
	}
	// The live stream and the archive use different renderings of the
	// same violation (Detail vs v.String()) — they land in the same
	// DIMENSION+RULE space; digit-normalization keeps each shape stable.
	log := writeTriageLog(t,
		`{"ts":"2026-06-11T00:00:00Z","game_seed":41,"dimension":"conservation","rule":"ZoneConservation","severity":"critical","detail":"seat 3 census reads 4 cards low"}`,
	)
	tr, err := TriageJudgeLog(log, dir)
	if err != nil {
		t.Fatal(err)
	}
	if tr.TotalRecords != 2 || tr.ArchiveFolded != 1 {
		t.Fatalf("records/folded = %d/%d, want 2/1", tr.TotalRecords, tr.ArchiveFolded)
	}
	for _, c := range tr.Clusters {
		if c.Dimension != judge.DimensionConservation || c.Rule != "ZoneConservation" {
			t.Fatalf("archive row mis-classified: %+v", c)
		}
	}
}

// The two artifacts: markdown grouped by dimension + ranked by count,
// JSON round-trips the full report.
func TestJudgeTriage_ArtifactEmission(t *testing.T) {
	dir := t.TempDir()
	log := writeTriageLog(t,
		`{"ts":"2026-06-10T01:00:00Z","game_seed":41,"sampled_game":7,"deck_keys":["a","b"],"dimension":"conservation","rule":"ZoneConservation","severity":"critical","turn":12,"detail":"seat 3 census reads 4 cards low"}`,
		`{"ts":"2026-06-10T02:00:00Z","game_seed":99,"dimension":"conservation","rule":"ZoneConservation","severity":"critical","detail":"seat 1 census reads 2 cards low"}`,
		`{"ts":"2026-06-10T03:00:00Z","game_seed":5,"dimension":"progression","rule":"etb/fire","card":"Ambuscade Shaman","severity":"critical","detail":"expected fire, got silent"}`,
	)
	tr, err := TriageJudgeLog(log, "")
	if err != nil {
		t.Fatal(err)
	}
	mdPath, jsonPath, err := WriteJudgeTriage(dir, tr)
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## conservation (2 record(s))",
		"## progression (1 record(s))",
		"ZoneConservation — 2×",
		"repro: seed 41 (sampled game 7) turn 12 · decks: a, b",
		"Ambuscade Shaman",
	} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
	// conservation must render before progression (canonical order).
	if strings.Index(string(md), "## conservation") > strings.Index(string(md), "## progression") {
		t.Fatal("markdown dimension order wrong")
	}
	var back JudgeTriage
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.TotalRecords != 3 || len(back.Clusters) != 2 {
		t.Fatalf("json round-trip: records=%d clusters=%d", back.TotalRecords, len(back.Clusters))
	}
}

// The wire-format contract: a record produced with the watchdog's exact
// JSON field names must parse field-for-field, and Violation() renders
// it back as the canonical vocabulary type.
func TestJudgeLogRecord_WireFormatAndVocabulary(t *testing.T) {
	line := `{"ts":"2026-06-10T01:00:00Z","game_seed":41,"sampled_game":7,"deck_keys":["a"],"dimension":"legality","surface":"legality","rule":"704.5f","severity":"critical","turn":3,"seat":2,"card":"X","detail":"illegal cast"}`
	var rec JudgeLogRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.GameSeed != 41 || rec.SampledID != 7 || rec.Dimension != "legality" ||
		rec.Rule != "704.5f" || rec.Turn != 3 || rec.Seat != 2 || rec.Card != "X" {
		t.Fatalf("wire parse mismatch: %+v", rec)
	}
	v := rec.Violation()
	if v.Name != "704.5f" || v.Dimension != judge.DimensionLegality ||
		v.Surface != judge.SurfaceLegality || v.Seat != 2 || v.Message != "illegal cast" {
		t.Fatalf("Violation() mismatch: %+v", v)
	}
}
