package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// batch_test.go — pins the --batch mode contract.

// writeSnapFile marshals a snapshot to JSON at dir/name.
func writeSnapFile(t *testing.T, dir, name string, snap SBASnapshot) {
	t.Helper()
	body, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeRawFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Empty dir — zero files, valid=true
// ---------------------------------------------------------------------------

func TestBatch_EmptyDir_ValidTrue(t *testing.T) {
	dir := t.TempDir()
	rep, err := runBatch(dir, "")
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !rep.Valid {
		t.Errorf("empty dir should produce Valid=true; got %+v", rep)
	}
	if rep.FilesScanned != 0 {
		t.Errorf("FilesScanned = %d, want 0", rep.FilesScanned)
	}
}

// ---------------------------------------------------------------------------
// Single clean snapshot
// ---------------------------------------------------------------------------

func TestBatch_OneCleanSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeSnapFile(t, dir, "clean.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	rep, err := runBatch(dir, "")
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if !rep.Valid {
		t.Errorf("expected Valid=true; got %+v", rep)
	}
	if rep.FilesScanned != 1 || rep.SnapshotsClean != 1 || rep.SnapshotsWithViolations != 0 {
		t.Errorf("counts off: scanned=%d clean=%d violating=%d",
			rep.FilesScanned, rep.SnapshotsClean, rep.SnapshotsWithViolations)
	}
	if rep.TotalViolations != 0 {
		t.Errorf("TotalViolations = %d, want 0", rep.TotalViolations)
	}
}

// ---------------------------------------------------------------------------
// Single violating snapshot
// ---------------------------------------------------------------------------

func TestBatch_OneViolatingSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeSnapFile(t, dir, "bad.json", SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 0, PoisonCounters: 11,
			Battlefield: []SBAPermanent{
				{Name: "Wisp", Types: []string{"creature"}, BaseToughness: 0},
			},
		}},
	})
	rep, err := runBatch(dir, "")
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	if rep.Valid {
		t.Fatal("expected Valid=false on violating snapshot")
	}
	if rep.SnapshotsWithViolations != 1 || rep.SnapshotsClean != 0 {
		t.Errorf("counts off: violating=%d clean=%d",
			rep.SnapshotsWithViolations, rep.SnapshotsClean)
	}
	// Expect 704.5a + 704.5c + 704.5f (3 rules tickled).
	for _, want := range []string{"704.5a", "704.5c", "704.5f"} {
		if rep.ViolationsByRule[want] != 1 {
			t.Errorf("ViolationsByRule[%s] = %d, want 1", want, rep.ViolationsByRule[want])
		}
	}
}

// ---------------------------------------------------------------------------
// Mixed: 2 clean + 1 violating → counts split correctly + top_offenders
// ---------------------------------------------------------------------------

func TestBatch_MixedCleanAndViolating(t *testing.T) {
	dir := t.TempDir()
	writeSnapFile(t, dir, "a_clean.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	writeSnapFile(t, dir, "b_clean.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 30}},
	})
	writeSnapFile(t, dir, "c_bad.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 0}},
	})
	rep, _ := runBatch(dir, "")
	if rep.Valid {
		t.Fatal("expected Valid=false")
	}
	if rep.SnapshotsClean != 2 {
		t.Errorf("SnapshotsClean = %d, want 2", rep.SnapshotsClean)
	}
	if rep.SnapshotsWithViolations != 1 {
		t.Errorf("SnapshotsWithViolations = %d, want 1", rep.SnapshotsWithViolations)
	}
	if len(rep.TopOffenders) != 1 {
		t.Errorf("TopOffenders should contain the 1 offender; got %d", len(rep.TopOffenders))
	}
	if rep.TopOffenders[0].File != "c_bad.json" {
		t.Errorf("top offender = %q, want c_bad.json", rep.TopOffenders[0].File)
	}
}

// ---------------------------------------------------------------------------
// Top-offenders sorting — multiple violating files, sorted desc by count
// ---------------------------------------------------------------------------

func TestBatch_TopOffendersSortedDesc(t *testing.T) {
	dir := t.TempDir()
	// Three offenders with 1, 3, 2 violations respectively.
	writeSnapFile(t, dir, "one.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 0}}, // 704.5a × 1
	})
	writeSnapFile(t, dir, "three.json", SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 0, PoisonCounters: 11,
			Battlefield: []SBAPermanent{
				{Name: "Wisp", Types: []string{"creature"}, BaseToughness: 0},
			},
		}}, // 704.5a + 704.5c + 704.5f
	})
	writeSnapFile(t, dir, "two.json", SBASnapshot{
		Seats: []SBASeat{{
			Idx: 0, Life: 0, PoisonCounters: 11,
		}}, // 704.5a + 704.5c
	})
	rep, _ := runBatch(dir, "")
	if len(rep.TopOffenders) < 3 {
		t.Fatalf("expected ≥3 top offenders, got %d", len(rep.TopOffenders))
	}
	wantOrder := []string{"three.json", "two.json", "one.json"}
	for i, want := range wantOrder {
		if rep.TopOffenders[i].File != want {
			t.Errorf("TopOffenders[%d] = %q, want %q", i, rep.TopOffenders[i].File, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Malformed JSON — surfaces in FilesFailedParse + per_file.error,
// doesn't crash, doesn't flip Valid by itself
// ---------------------------------------------------------------------------

func TestBatch_MalformedJSONIsTriaged(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "broken.json", "{ not valid json")
	writeSnapFile(t, dir, "clean.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	rep, err := runBatch(dir, "")
	if err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	// Parse-fail alone does NOT trigger non-Valid.
	if !rep.Valid {
		t.Errorf("parse failure shouldn't flip Valid=false (no §704 violations); got %+v", rep)
	}
	if rep.FilesFailedParse != 1 {
		t.Errorf("FilesFailedParse = %d, want 1", rep.FilesFailedParse)
	}
	// per_file.error must surface the failure.
	var found bool
	for _, fr := range rep.PerFile {
		if fr.File == "broken.json" && fr.Error != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected broken.json to surface in per_file with error; got %+v", rep.PerFile)
	}
}

// ---------------------------------------------------------------------------
// Non-.json files ignored (no parse attempt, no surface in per_file)
// ---------------------------------------------------------------------------

func TestBatch_NonJSONFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "notes.txt", "some notes")
	writeRawFile(t, dir, "README.md", "# readme")
	writeSnapFile(t, dir, "snap.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	rep, _ := runBatch(dir, "")
	if len(rep.PerFile) != 1 {
		t.Errorf("expected 1 per_file entry (snap.json only); got %d: %+v",
			len(rep.PerFile), rep.PerFile)
	}
	if rep.PerFile[0].File != "snap.json" {
		t.Errorf("only json should be processed; got %q", rep.PerFile[0].File)
	}
}

// ---------------------------------------------------------------------------
// Subdirectories are NOT recursed
// ---------------------------------------------------------------------------

func TestBatch_SubdirectoriesNotRecursed(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeSnapFile(t, sub, "deep.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	writeSnapFile(t, dir, "shallow.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 40}},
	})
	rep, _ := runBatch(dir, "")
	if rep.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1 (subdirs not recursed)", rep.FilesScanned)
	}
	if rep.PerFile[0].File != "shallow.json" {
		t.Errorf("expected only shallow.json; got %+v", rep.PerFile)
	}
}

// ---------------------------------------------------------------------------
// Object missing seats key → triaged as parse failure
// ---------------------------------------------------------------------------

func TestBatch_EmptySeatsTreatedAsParseFailure(t *testing.T) {
	dir := t.TempDir()
	writeRawFile(t, dir, "no_seats.json", `{"format":"commander"}`)
	rep, _ := runBatch(dir, "")
	if rep.FilesFailedParse != 1 {
		t.Errorf("FilesFailedParse = %d, want 1 (no seats → parse failure)",
			rep.FilesFailedParse)
	}
	if rep.FilesScanned != 0 {
		t.Errorf("FilesScanned = %d, want 0 (the no-seats file shouldn't count as scanned)",
			rep.FilesScanned)
	}
}

// ---------------------------------------------------------------------------
// JSON output schema lock
// ---------------------------------------------------------------------------

func TestBatch_JSONSchemaLock(t *testing.T) {
	dir := t.TempDir()
	writeSnapFile(t, dir, "bad.json", SBASnapshot{
		Seats: []SBASeat{{Idx: 0, Life: 0}},
	})
	out := filepath.Join(t.TempDir(), "report.json")
	if _, err := runBatch(dir, out); err != nil {
		t.Fatalf("runBatch: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"rule"`, `"batch_dir"`, `"files_scanned"`, `"files_failed_parse"`,
		`"snapshots_clean"`, `"snapshots_with_violations"`,
		`"total_violations"`, `"violations_by_rule"`, `"per_file"`,
		`"top_offenders"`, `"valid"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing top-level key %s in JSON output", key)
		}
	}
	for _, key := range []string{
		`"file"`, `"valid"`, `"violation_count"`,
	} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("missing per_file field %s in JSON output", key)
		}
	}
	// Round-trip.
	var rt BatchReport
	if err := json.Unmarshal(raw, &rt); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if rt.Rule != "CR §704 (batch)" {
		t.Errorf("Rule = %q, want CR §704 (batch)", rt.Rule)
	}
}

// ---------------------------------------------------------------------------
// Error paths — empty dir arg + non-existent dir + file-not-dir
// ---------------------------------------------------------------------------

func TestBatch_EmptyDirArgErrors(t *testing.T) {
	if _, err := runBatch("", ""); err == nil {
		t.Errorf("expected error for empty --batch arg")
	}
}

func TestBatch_NonexistentDirErrors(t *testing.T) {
	if _, err := runBatch("/nonexistent/path/xyz", ""); err == nil {
		t.Errorf("expected error for non-existent dir")
	}
}

func TestBatch_FileNotDirErrors(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "afile.json")
	writeRawFile(t, tmp, "afile.json", "{}")
	if _, err := runBatch(path, ""); err == nil {
		t.Errorf("expected error when --batch points at a file not a dir")
	}
}

// ---------------------------------------------------------------------------
// Deterministic per-file ordering across runs (sorted alphabetically)
// ---------------------------------------------------------------------------

func TestBatch_PerFileOrderingIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"c.json", "a.json", "b.json"} {
		writeSnapFile(t, dir, name, SBASnapshot{
			Seats: []SBASeat{{Idx: 0, Life: 40}},
		})
	}
	rep1, _ := runBatch(dir, "")
	for i := 0; i < 5; i++ {
		rep2, _ := runBatch(dir, "")
		if len(rep1.PerFile) != len(rep2.PerFile) {
			t.Fatalf("PerFile length drift across runs")
		}
		for j := range rep1.PerFile {
			if rep1.PerFile[j].File != rep2.PerFile[j].File {
				t.Errorf("iter %d: PerFile[%d] = %q, want %q",
					i, j, rep2.PerFile[j].File, rep1.PerFile[j].File)
			}
		}
	}
	// Order must be alphabetical.
	wantOrder := []string{"a.json", "b.json", "c.json"}
	for i, want := range wantOrder {
		if rep1.PerFile[i].File != want {
			t.Errorf("PerFile[%d] = %q, want %q (alphabetical)",
				i, rep1.PerFile[i].File, want)
		}
	}
}
