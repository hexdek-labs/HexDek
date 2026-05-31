package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/hexdek/hexdek/internal/muninn"
)

// silent_inert_audit.go — Genesis Chamber detection AUTOMATED.
//
// PR #815's scoping audit found Genesis Chamber accumulating 22,662
// triggered-ability hits across the per_card observation surface
// while contributing zero damage, kills, or wincon participation —
// the canonical "wired but dead" silent-inert signature. The fix
// (ctx-key `permanent` → `perm`) had already been shipped, but the
// *detection* of that 22,662-hit class was a manual scoping step
// during a coverage-sweep PR. This subcommand surfaces the same
// detection automatically over the live `dead_triggers.json`
// Muninn persistence (populated by `PersistDeadTriggers` from
// every tournament run with `--audit --analytics`).
//
// The InteractionGraph integration the dispatch envisioned is NOT
// landed in main as of 2026-05-30 — no `InteractionGraph` type
// exists in the engine and no in-flight branch builds it. When
// that graph does land, the per-handler `participated-in-edge`
// signal will fold in as a fifth gate to the dead-trigger
// classification in `muninn.go:PersistDeadTriggers` itself (so
// every downstream consumer benefits, not just this CLI). Until
// then, the existing four-gate signature (TriggeredCount > 0 +
// DamageDealt == 0 + KillsAttributed == 0 + !ContributedToWin +
// not-the-WinningCard + not-a-land/token) is the production-grade
// silent-inert detector — same one that caught Genesis Chamber.

// SilentInertSeverity classifies a dead-trigger entry by hit count.
// Genesis Chamber's 22,662 hits at scope time would sit deep in the
// critical tier. Below `watch`, the count is treated as noise (e.g.
// a turn-1 misregistration with a few stray hits from a single
// game).
type SilentInertSeverity string

const (
	SeverityCritical SilentInertSeverity = "critical"
	SeverityModerate SilentInertSeverity = "moderate"
	SeverityWatch    SilentInertSeverity = "watch"
	SeverityTrivial  SilentInertSeverity = "trivial"
)

// SilentInertAuditOpts captures the CLI knobs for the audit run.
type SilentInertAuditOpts struct {
	Dir               string
	Format            string // text / json / tsv
	CriticalThreshold int
	ModerateThreshold int
	WatchThreshold    int
	FailOnCritical    bool
}

// SilentInertCandidate is one entry in the audit output. JSON-
// serialized as-is when `--audit-format=json`.
type SilentInertCandidate struct {
	CardName    string              `json:"card_name"`
	TriggerName string              `json:"trigger_name"`
	HitCount    int                 `json:"hit_count"`
	GamesSeen   int                 `json:"games_seen"`
	LastSeen    string              `json:"last_seen"`
	Severity    SilentInertSeverity `json:"severity"`
	// PerGameAvg is hit_count / games_seen — a high per-game
	// average means the handler is firing many times per game
	// without any rules effect, the strongest signal that the
	// handler is mis-wired (Genesis Chamber's per-game average
	// at scope time was in the hundreds).
	PerGameAvg float64 `json:"per_game_avg"`
}

// SilentInertAuditResult is the top-level audit output.
type SilentInertAuditResult struct {
	Candidates    []SilentInertCandidate `json:"candidates"`
	TotalEntries  int                    `json:"total_entries"`
	CriticalCount int                    `json:"critical_count"`
	ModerateCount int                    `json:"moderate_count"`
	WatchCount    int                    `json:"watch_count"`
	TrivialCount  int                    `json:"trivial_count"`
	Thresholds    struct {
		Critical int `json:"critical"`
		Moderate int `json:"moderate"`
		Watch    int `json:"watch"`
	} `json:"thresholds"`
}

// classifySeverity maps a hit count to a severity tier. Pure
// function — boundary-checked: at exactly the threshold, the
// candidate lands in the higher tier (>= comparison). Inputs at or
// above critical are critical; between moderate and critical are
// moderate; between watch and moderate are watch; below watch are
// trivial. The trivial bucket exists so the audit output isn't
// flooded with one-off noise from a single game, while still
// preserving the count in the total for completeness.
func classifySeverity(hitCount, critical, moderate, watch int) SilentInertSeverity {
	switch {
	case hitCount >= critical:
		return SeverityCritical
	case hitCount >= moderate:
		return SeverityModerate
	case hitCount >= watch:
		return SeverityWatch
	default:
		return SeverityTrivial
	}
}

// buildCandidates reads the persisted dead-trigger records from
// `dir`, applies severity tiering, and returns the sorted candidate
// list (critical first, then by hit-count descending within each
// tier). Separated from the run/print path so the test suite can
// drive classification logic without touching the filesystem.
func buildCandidates(triggers []muninn.DeadTrigger, opts SilentInertAuditOpts) []SilentInertCandidate {
	out := make([]SilentInertCandidate, 0, len(triggers))
	for _, dt := range triggers {
		perGame := 0.0
		if dt.GamesSeen > 0 {
			perGame = float64(dt.Count) / float64(dt.GamesSeen)
		}
		out = append(out, SilentInertCandidate{
			CardName:    dt.CardName,
			TriggerName: dt.TriggerName,
			HitCount:    dt.Count,
			GamesSeen:   dt.GamesSeen,
			LastSeen:    dt.LastSeen,
			Severity:    classifySeverity(dt.Count, opts.CriticalThreshold, opts.ModerateThreshold, opts.WatchThreshold),
			PerGameAvg:  perGame,
		})
	}
	// Sort: severity tier (critical > moderate > watch > trivial),
	// then hit_count desc, then card_name asc for determinism.
	tierRank := map[SilentInertSeverity]int{
		SeverityCritical: 0,
		SeverityModerate: 1,
		SeverityWatch:    2,
		SeverityTrivial:  3,
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := tierRank[out[i].Severity], tierRank[out[j].Severity]
		if ri != rj {
			return ri < rj
		}
		if out[i].HitCount != out[j].HitCount {
			return out[i].HitCount > out[j].HitCount
		}
		return out[i].CardName < out[j].CardName
	})
	return out
}

// summarize counts per-tier totals from the candidate slice and
// builds the result envelope.
func summarize(candidates []SilentInertCandidate, opts SilentInertAuditOpts) SilentInertAuditResult {
	r := SilentInertAuditResult{
		Candidates:   candidates,
		TotalEntries: len(candidates),
	}
	for _, c := range candidates {
		switch c.Severity {
		case SeverityCritical:
			r.CriticalCount++
		case SeverityModerate:
			r.ModerateCount++
		case SeverityWatch:
			r.WatchCount++
		case SeverityTrivial:
			r.TrivialCount++
		}
	}
	r.Thresholds.Critical = opts.CriticalThreshold
	r.Thresholds.Moderate = opts.ModerateThreshold
	r.Thresholds.Watch = opts.WatchThreshold
	return r
}

// runSilentInertAudit is the CLI entry point. Returns (exitCode,
// error). exitCode is non-zero only when FailOnCritical is set AND
// at least one critical-tier candidate surfaced — otherwise zero
// even when candidates exist (the audit is informational unless the
// CI flag is set).
func runSilentInertAudit(w io.Writer, opts SilentInertAuditOpts) (int, error) {
	triggers, err := muninn.ReadDeadTriggers(opts.Dir)
	if err != nil {
		return 0, fmt.Errorf("read dead triggers: %w", err)
	}
	candidates := buildCandidates(triggers, opts)
	result := summarize(candidates, opts)

	switch opts.Format {
	case "json":
		if err := writeJSON(w, result); err != nil {
			return 0, err
		}
	case "tsv":
		writeTSV(w, result)
	case "", "text":
		writeText(w, result)
	default:
		return 0, fmt.Errorf("unknown --audit-format %q (want text / json / tsv)", opts.Format)
	}

	if opts.FailOnCritical && result.CriticalCount > 0 {
		return 1, nil
	}
	return 0, nil
}

func writeJSON(w io.Writer, r SilentInertAuditResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func writeTSV(w io.Writer, r SilentInertAuditResult) {
	fmt.Fprintln(w, "severity\thit_count\tgames_seen\tper_game_avg\tcard_name\ttrigger_name\tlast_seen")
	for _, c := range r.Candidates {
		fmt.Fprintf(w, "%s\t%d\t%d\t%.2f\t%s\t%s\t%s\n",
			c.Severity, c.HitCount, c.GamesSeen, c.PerGameAvg, c.CardName, c.TriggerName, c.LastSeen)
	}
}

func writeText(w io.Writer, r SilentInertAuditResult) {
	fmt.Fprintln(w, "=== MUNINN SILENT-INERT AUDIT ===")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Total entries: %d  (thresholds: critical>=%d  moderate>=%d  watch>=%d)\n",
		r.TotalEntries, r.Thresholds.Critical, r.Thresholds.Moderate, r.Thresholds.Watch)
	fmt.Fprintf(w, "  CRITICAL: %d\n", r.CriticalCount)
	fmt.Fprintf(w, "  MODERATE: %d\n", r.ModerateCount)
	fmt.Fprintf(w, "  WATCH:    %d\n", r.WatchCount)
	fmt.Fprintf(w, "  TRIVIAL:  %d\n", r.TrivialCount)
	fmt.Fprintln(w)

	if r.TotalEntries == 0 {
		fmt.Fprintln(w, "No dead-trigger records found. Run a tournament with --audit --analytics first.")
		return
	}

	// Group by tier for readable output. Skip the trivial bucket
	// from the per-tier print (still counted in the total).
	currentTier := SilentInertSeverity("")
	printed := 0
	for _, c := range r.Candidates {
		if c.Severity == SeverityTrivial {
			continue
		}
		if c.Severity != currentTier {
			if currentTier != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "--- %s ---\n", c.Severity)
			currentTier = c.Severity
		}
		fmt.Fprintf(w, "  [%6d hits / %3d games / avg=%.1f]  %s  (%s)  last=%s\n",
			c.HitCount, c.GamesSeen, c.PerGameAvg, c.CardName, c.TriggerName, shortDate(c.LastSeen))
		printed++
	}
	if printed == 0 {
		fmt.Fprintln(w, "All entries are trivial-tier (below watch threshold). Likely noise from a single game.")
	}
}
