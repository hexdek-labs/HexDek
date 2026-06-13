package muninn

// judge_triage.go — Muninn as the Hex Judge's triage clerk (r63
// cleanup round 2, step 3; owner direction: "Muninn should now be
// classifying and organizing Judge and Bug issues from the engine").
//
// The Judge watchdog streams every live-grinder violation as one JSONL
// row to data/judge/grinder-violations.jsonl (see
// internal/hexapi/judge_watchdog.go). Each row is the wire form of a
// canonical judge.ValidationViolation plus repro context (seed, deck
// keys, sampled-game id, turn) — JudgeLogRecord below mirrors that wire
// format field-for-field; it is a SERIALIZATION of the one vocabulary,
// not a second one.
//
// Triage: classify each record by Judge dimension, dedupe by a STABLE
// fingerprint (dimension + rule + object signature — never the raw
// repro, which varies by seed/turn), count occurrences with
// first-seen/last-seen, and emit an organized artifact for human
// review: data/muninn/judge-triage.md + judge-triage.json. Muninn's
// pre-existing invariant_violations.json archive (the tournament
// runner's bookkeeping for the same canonical violations) folds into
// the same clusters so there is one triage view, not two.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/judge"
)

// JudgeLogRecord is one grinder-violations.jsonl row. Field names are
// pinned to the watchdog's writer (hexapi judgeViolationRecord):
// changing either side breaks the stream contract.
type JudgeLogRecord struct {
	TS        string   `json:"ts"`
	GameSeed  int64    `json:"game_seed"`
	SampledID int64    `json:"sampled_game"`
	DeckKeys  []string `json:"deck_keys,omitempty"`
	Dimension string   `json:"dimension"`
	Surface   string   `json:"surface,omitempty"`
	Rule      string   `json:"rule"`
	Severity  string   `json:"severity,omitempty"`
	Turn      int      `json:"turn,omitempty"`
	Seat      int      `json:"seat,omitempty"`
	Card      string   `json:"card,omitempty"`
	Detail    string   `json:"detail"`
}

// Violation renders the record back as the canonical vocabulary type.
func (r JudgeLogRecord) Violation() judge.ValidationViolation {
	return judge.ValidationViolation{
		Surface:   r.Surface,
		Name:      r.Rule,
		Severity:  r.Severity,
		Message:   r.Detail,
		Dimension: r.Dimension,
		Seat:      r.Seat,
	}
}

// TriageCluster is one deduped bug signature with its occurrence stats
// and a representative repro (the first occurrence seen).
type TriageCluster struct {
	Fingerprint string `json:"fingerprint"`
	Dimension   string `json:"dimension"`
	Surface     string `json:"surface,omitempty"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	// Object is the card name when the violation names one, else the
	// normalized detail signature the fingerprint keyed on.
	Object    string `json:"object"`
	Count     int    `json:"count"`
	FirstSeen string `json:"first_seen,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
	// Representative repro — replay the same seed with the same decks.
	ReproSeed     int64    `json:"repro_seed"`
	ReproGame     int64    `json:"repro_game,omitempty"`
	ReproDeckKeys []string `json:"repro_deck_keys,omitempty"`
	ReproTurn     int      `json:"repro_turn,omitempty"`
	SampleDetail  string   `json:"sample_detail"`
}

// JudgeTriage is the full triage result.
type JudgeTriage struct {
	Source        string          `json:"source"`
	TotalRecords  int             `json:"total_records"`
	ArchiveFolded int             `json:"archive_records_folded,omitempty"`
	Malformed     int             `json:"malformed_lines_skipped,omitempty"`
	ByDimension   map[string]int  `json:"by_dimension"`
	Clusters      []TriageCluster `json:"clusters"`
}

// dimensionOrder is the canonical 6-dimension presentation order.
// Unknown dimensions sort after, alphabetically.
var dimensionOrder = map[string]int{
	judge.DimensionLegality:       0,
	judge.DimensionConservation:   1,
	judge.DimensionStateIntegrity: 2,
	judge.DimensionProgression:    3,
	judge.DimensionOutcome:        4,
	judge.DimensionLiveness:       5,
}

// classifyDimension buckets a record into the 6 Judge dimensions.
// Empty defaults to state_integrity — the same default the watchdog
// applies at write time — so legacy/archive rows land somewhere real.
// A nonstandard non-empty value is kept verbatim (it groups after the
// canonical six and is visible in the artifact rather than masked).
func classifyDimension(dim string) string {
	if dim == "" {
		return judge.DimensionStateIntegrity
	}
	return dim
}

// triageFingerprint is the stable dedupe key: dimension + rule + the
// object signature. The object is the card name when present;
// otherwise the detail message with digit runs collapsed, so the same
// bug surfacing at different seeds/turns/counts fingerprints
// identically while different cards or different shapes stay distinct.
func triageFingerprint(dim, rule, card, detail string) (fp, object string) {
	object = card
	if object == "" {
		object = normalizeDetailSignature(detail)
	}
	return dim + "|" + rule + "|" + object, object
}

// normalizeDetailSignature strips the volatile parts of a detail
// message: digit runs become '#' (seeds, turns, counts, instance IDs),
// whitespace collapses, and the result is capped.
func normalizeDetailSignature(detail string) string {
	var b strings.Builder
	b.Grow(len(detail))
	inDigits, inSpace := false, false
	for _, r := range detail {
		switch {
		case r >= '0' && r <= '9':
			if !inDigits {
				b.WriteByte('#')
			}
			inDigits, inSpace = true, false
		case r == ' ' || r == '\t' || r == '\n':
			if !inSpace {
				b.WriteByte(' ')
			}
			inDigits, inSpace = false, true
		default:
			b.WriteRune(r)
			inDigits, inSpace = false, false
		}
	}
	sig := strings.TrimSpace(b.String())
	if len(sig) > 160 {
		sig = sig[:160]
	}
	return sig
}

// severityRank orders severities for keeping the worst seen per cluster.
func severityRank(s string) int {
	switch s {
	case judge.SeverityCritical:
		return 3
	case judge.SeverityWarning:
		return 2
	case judge.SeverityInfo:
		return 1
	}
	return 0
}

// TriageJudgeLog reads the Judge's grinder violation stream at logPath
// and, when muninnDir is non-empty, folds muninn's own
// invariant_violations.json archive into the same clusters. A missing
// or empty stream is not an error — the result simply has
// TotalRecords 0 (the artifact writers no-op on that).
func TriageJudgeLog(logPath, muninnDir string) (*JudgeTriage, error) {
	t := &JudgeTriage{
		Source:      logPath,
		ByDimension: map[string]int{},
	}
	clusters := map[string]*TriageCluster{}

	add := func(rec JudgeLogRecord) {
		dim := classifyDimension(rec.Dimension)
		t.TotalRecords++
		t.ByDimension[dim]++
		fp, object := triageFingerprint(dim, rec.Rule, rec.Card, rec.Detail)
		c := clusters[fp]
		if c == nil {
			c = &TriageCluster{
				Fingerprint:   fp,
				Dimension:     dim,
				Surface:       rec.Surface,
				Rule:          rec.Rule,
				Severity:      rec.Severity,
				Object:        object,
				FirstSeen:     rec.TS,
				LastSeen:      rec.TS,
				ReproSeed:     rec.GameSeed,
				ReproGame:     rec.SampledID,
				ReproDeckKeys: rec.DeckKeys,
				ReproTurn:     rec.Turn,
				SampleDetail:  rec.Detail,
			}
			clusters[fp] = c
		}
		c.Count++
		// RFC3339 compares lexicographically; tolerate empty stamps.
		if rec.TS != "" && (c.FirstSeen == "" || rec.TS < c.FirstSeen) {
			c.FirstSeen = rec.TS
		}
		if rec.TS > c.LastSeen {
			c.LastSeen = rec.TS
		}
		if severityRank(rec.Severity) > severityRank(c.Severity) {
			c.Severity = rec.Severity
		}
	}

	if f, err := os.Open(logPath); err == nil {
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if line == "" {
				continue
			}
			var rec JudgeLogRecord
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Malformed++
				continue
			}
			add(rec)
		}
		scanErr := scan.Err()
		f.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("muninn: scan %s: %w", logPath, scanErr)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("muninn: open %s: %w", logPath, err)
	}

	// Fold the tournament runner's archived violations — same canonical
	// vocabulary, older bookkeeping path — into the same clusters.
	if muninnDir != "" {
		archived, err := ReadInvariantViolations(muninnDir)
		if err != nil {
			return nil, err
		}
		for _, av := range archived {
			var keys []string
			for _, k := range av.DeckKeys {
				if k != "" {
					keys = append(keys, k)
				}
			}
			add(JudgeLogRecord{
				TS:        av.Timestamp,
				GameSeed:  av.GameSeed,
				DeckKeys:  keys,
				Dimension: av.Dimension,
				Rule:      av.ViolationType,
				Turn:      av.Turn,
				Detail:    av.Message,
			})
			t.ArchiveFolded++
		}
	}

	t.Clusters = make([]TriageCluster, 0, len(clusters))
	for _, c := range clusters {
		t.Clusters = append(t.Clusters, *c)
	}
	sort.Slice(t.Clusters, func(i, j int) bool {
		a, b := t.Clusters[i], t.Clusters[j]
		ao, aKnown := dimensionOrder[a.Dimension]
		bo, bKnown := dimensionOrder[b.Dimension]
		switch {
		case aKnown != bKnown:
			return aKnown
		case aKnown && ao != bo:
			return ao < bo
		case !aKnown && a.Dimension != b.Dimension:
			return a.Dimension < b.Dimension
		case a.Count != b.Count:
			return a.Count > b.Count
		}
		return a.Fingerprint < b.Fingerprint
	})
	return t, nil
}

// WriteJudgeTriage writes the organized artifacts: judge-triage.md (the
// human review document) and judge-triage.json (the machine twin) under
// dir. A zero-record triage is a NO-OP — it returns empty paths and
// leaves any previous artifacts untouched rather than clobbering them
// with emptiness.
func WriteJudgeTriage(dir string, t *JudgeTriage) (mdPath, jsonPath string, err error) {
	if t == nil || t.TotalRecords == 0 {
		return "", "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("muninn: mkdir %s: %w", dir, err)
	}
	fileMu.Lock()
	defer fileMu.Unlock()

	jsonPath = filepath.Join(dir, "judge-triage.json")
	if err := atomicWriteJSON(jsonPath, t); err != nil {
		return "", "", err
	}

	mdPath = filepath.Join(dir, "judge-triage.md")
	md := renderTriageMarkdown(t)
	tmp := mdPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(md), 0o644); err != nil {
		return "", "", fmt.Errorf("muninn: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, mdPath); err != nil {
		os.Remove(tmp)
		return "", "", fmt.Errorf("muninn: rename %s: %w", tmp, err)
	}
	return mdPath, jsonPath, nil
}

// renderTriageMarkdown renders the human artifact: grouped by dimension
// in canonical order, clusters ranked by occurrence count.
func renderTriageMarkdown(t *JudgeTriage) string {
	var b strings.Builder
	b.WriteString("# Hex Judge Triage — Muninn clerk\n\n")
	fmt.Fprintf(&b, "Source: `%s` — %d records", t.Source, t.TotalRecords)
	if t.ArchiveFolded > 0 {
		fmt.Fprintf(&b, " (incl. %d folded from invariant_violations.json)", t.ArchiveFolded)
	}
	if t.Malformed > 0 {
		fmt.Fprintf(&b, ", %d malformed line(s) skipped", t.Malformed)
	}
	fmt.Fprintf(&b, " · %d cluster(s)\n\n", len(t.Clusters))
	b.WriteString("Clusters dedupe by `dimension|rule|object` — same bug at a different\nseed/turn is one cluster; the repro line replays the representative hit.\n")

	curDim := ""
	rank := 0
	for _, c := range t.Clusters {
		if c.Dimension != curDim {
			curDim = c.Dimension
			rank = 0
			fmt.Fprintf(&b, "\n## %s (%d record(s))\n", curDim, t.ByDimension[curDim])
		}
		rank++
		fmt.Fprintf(&b, "\n### %d. %s — %d×", rank, c.Rule, c.Count)
		if c.Severity != "" {
			fmt.Fprintf(&b, " [%s]", c.Severity)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "- fingerprint: `%s`\n", c.Fingerprint)
		fmt.Fprintf(&b, "- object: %s\n", c.Object)
		if c.Surface != "" {
			fmt.Fprintf(&b, "- surface: %s\n", c.Surface)
		}
		if c.FirstSeen != "" || c.LastSeen != "" {
			fmt.Fprintf(&b, "- first seen %s · last seen %s\n", c.FirstSeen, c.LastSeen)
		}
		fmt.Fprintf(&b, "- repro: seed %d", c.ReproSeed)
		if c.ReproGame != 0 {
			fmt.Fprintf(&b, " (sampled game %d)", c.ReproGame)
		}
		if c.ReproTurn != 0 {
			fmt.Fprintf(&b, " turn %d", c.ReproTurn)
		}
		if len(c.ReproDeckKeys) > 0 {
			fmt.Fprintf(&b, " · decks: %s", strings.Join(c.ReproDeckKeys, ", "))
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "- sample: %s\n", c.SampleDetail)
	}
	return b.String()
}
