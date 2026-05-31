package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// hexdek-judge --batch <dir> — walk every .json file in a directory
// and run the §704 SBA probe against each as a game-state snapshot.
// Emits a summary report with per-file violation tallies, an
// aggregated "violations by rule" histogram, and a top-offenders
// ranking.
//
// Use case: a tournament-judge workflow accumulates many saved game
// states (Loki crash dumps, manual snapshots, replay-frame
// exports) into a directory. Rather than invoke --check-sba once per
// file, --batch runs the whole pile in one pass and produces a
// single JSON artifact suitable for CI / dashboards.
//
// Scope: SHALLOW (top-level .json only, no recursion) — keeps the
// semantics clear (one directory = one batch) and avoids accidentally
// pulling in unrelated JSONs from a subdirectory. Non-.json files are
// silently skipped. Files that fail to parse as SBASnapshot are
// recorded in FilesFailedParse + per_file.error so the user can
// triage them without the probe aborting.
//
// Exit status mirrors the other --check-* probes: 0 when every
// scanned snapshot is clean, 1 when ANY snapshot tickles a §704
// condition. Parse failures alone do NOT trigger exit=1 (they're
// surfaced in the report but don't represent a rules-correctness
// violation — they're input-data noise).

// BatchReport is the top-level JSON output shape.
type BatchReport struct {
	Rule                    string             `json:"rule"`
	BatchDir                string             `json:"batch_dir"`
	FilesScanned            int                `json:"files_scanned"`
	FilesFailedParse        int                `json:"files_failed_parse"`
	SnapshotsClean          int                `json:"snapshots_clean"`
	SnapshotsWithViolations int                `json:"snapshots_with_violations"`
	TotalViolations         int                `json:"total_violations"`
	ViolationsByRule        map[string]int     `json:"violations_by_rule"`
	PerFile                 []BatchFileResult  `json:"per_file"`
	TopOffenders            []BatchFileResult  `json:"top_offenders,omitempty"`
	Valid                   bool               `json:"valid"`
}

// BatchFileResult is the per-file summary entry.
type BatchFileResult struct {
	File            string         `json:"file"`
	Valid           bool           `json:"valid"`
	ViolationCount  int            `json:"violation_count"`
	ViolationsByRule map[string]int `json:"violations_by_rule,omitempty"`
	Error           string         `json:"error,omitempty"`
}

// runBatch walks dir's top-level .json files and runs detectSBAViolations
// against each, accumulating into a BatchReport.
func runBatch(dir, outPath string) (*BatchReport, error) {
	if dir == "" {
		return nil, fmt.Errorf("--batch requires a directory path")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("open batch dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("--batch path is not a directory: %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read batch dir: %w", err)
	}

	rep := &BatchReport{
		Rule:             "CR §704 (batch)",
		BatchDir:         dir,
		ViolationsByRule: map[string]int{},
		PerFile:          []BatchFileResult{},
		Valid:            true,
	}

	// Sort entries for deterministic output — os.ReadDir doesn't
	// promise alphabetical on all platforms.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, name)
		fr := scanOneFile(path)
		fr.File = name
		rep.PerFile = append(rep.PerFile, fr)
		if fr.Error != "" {
			rep.FilesFailedParse++
			continue
		}
		rep.FilesScanned++
		if fr.Valid {
			rep.SnapshotsClean++
		} else {
			rep.SnapshotsWithViolations++
			rep.TotalViolations += fr.ViolationCount
			for rule, n := range fr.ViolationsByRule {
				rep.ViolationsByRule[rule] += n
			}
			rep.Valid = false
		}
	}

	rep.TopOffenders = computeTopOffenders(rep.PerFile, 5)

	if err := writeBatchReport(rep, outPath); err != nil {
		return rep, err
	}
	return rep, nil
}

// scanOneFile loads a single snapshot and runs the §704 probe against
// it. Parse errors and "not actually a snapshot" cases (empty seats
// slice + nothing else recognizable) get surfaced via the Error
// field so the caller can tally them as FilesFailedParse without
// flipping Valid.
func scanOneFile(path string) BatchFileResult {
	out := BatchFileResult{Valid: true, ViolationsByRule: map[string]int{}}
	f, err := os.Open(path)
	if err != nil {
		out.Error = fmt.Sprintf("open: %v", err)
		return out
	}
	defer f.Close()
	var snap SBASnapshot
	if err := json.NewDecoder(f).Decode(&snap); err != nil {
		out.Error = fmt.Sprintf("decode: %v", err)
		return out
	}
	// Defensive — a JSON object missing the "seats" key still decodes
	// to an empty Seats slice. We don't want to mis-classify that as
	// a clean snapshot (it isn't one). Surface as parse failure.
	if len(snap.Seats) == 0 {
		out.Error = "no seats — not a recognizable SBASnapshot"
		return out
	}
	vs := detectSBAViolations(&snap)
	out.ViolationCount = len(vs)
	for _, v := range vs {
		out.ViolationsByRule[v.Rule]++
	}
	out.Valid = len(vs) == 0
	return out
}

// computeTopOffenders returns the top N files by ViolationCount,
// sorted descending. Files with zero violations are excluded —
// "top offenders" only makes sense for files with at least one hit.
func computeTopOffenders(per []BatchFileResult, n int) []BatchFileResult {
	var pool []BatchFileResult
	for _, fr := range per {
		if fr.ViolationCount > 0 {
			pool = append(pool, fr)
		}
	}
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].ViolationCount != pool[j].ViolationCount {
			return pool[i].ViolationCount > pool[j].ViolationCount
		}
		return pool[i].File < pool[j].File
	})
	if len(pool) > n {
		pool = pool[:n]
	}
	return pool
}

func writeBatchReport(rep *BatchReport, outPath string) error {
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
