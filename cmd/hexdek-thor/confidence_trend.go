package main

// Confidence trend — compare two corpus snapshots and report per-card
// confidence drift. Used to track parser quality changes across commits
// or releases.
//
// Workflow:
//
//   1. Snapshot the current corpus into a JSON file:
//        thor --confidence-snapshot-out data/conf-baseline.json
//   2. Make parser / scaffold changes.
//   3. Diff the new state against the baseline:
//        thor --confidence-trend --confidence-trend-from data/conf-baseline.json
//      (No --to flag means "snapshot current corpus and compare against
//      --from"; pass --to PATH for snapshot-vs-snapshot.)
//
// The report lists regressed cards (score went DOWN) ranked by drop
// size, improved cards (score went UP) ranked by gain size, plus
// summary stats (total regressed / improved / unchanged / new / dropped).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

// Snapshot is a JSON-serialisable capture of per-card confidence
// across the AST corpus at a point in time. Stored on disk as
// `data/...confidence-snapshot.json` (location operator's choice).
type Snapshot struct {
	// Version pins the serialisation shape. Bump on incompatible
	// changes; readers fall back gracefully if a key is absent.
	Version int `json:"version"`
	// TotalCards is the count of cards captured (including no-ability
	// cards, which still get an entry — score 1.0 by gameast contract).
	TotalCards int `json:"total_cards"`
	// Cards is keyed by card name; values carry score + ability detail.
	Cards map[string]SnapshotCard `json:"cards"`
}

// SnapshotCard is the per-card payload in a Snapshot.
type SnapshotCard struct {
	Score        float64 `json:"score"`
	MinScore     float64 `json:"min_score"`
	NumAbilities int     `json:"num_abilities"`
	NumFallback  int     `json:"num_fallback"`
}

// BuildSnapshot constructs a Snapshot from a freshly-loaded corpus.
// Skips cards whose AST is nil; iteration order is stable (Names() is
// deterministic).
func BuildSnapshot(corpus *astload.Corpus) Snapshot {
	if corpus == nil {
		return Snapshot{Version: 1, Cards: map[string]SnapshotCard{}}
	}
	cards := make([]*gameast.CardAST, 0, corpus.CardCount)
	for _, name := range corpus.Names() {
		if c, ok := corpus.Get(name); ok && c != nil {
			cards = append(cards, c)
		}
	}
	return BuildSnapshotFromCards(cards)
}

// BuildSnapshotFromCards is the pure core that the test harness
// exercises with synthetic *CardAST inputs.
func BuildSnapshotFromCards(cards []*gameast.CardAST) Snapshot {
	out := Snapshot{
		Version: 1,
		Cards:   make(map[string]SnapshotCard, len(cards)),
	}
	for _, card := range cards {
		if card == nil {
			continue
		}
		fallback := 0
		for _, ab := range card.Abilities {
			if len(gameast.LowConfidenceReasons(ab)) > 0 {
				fallback++
			}
		}
		out.Cards[card.Name] = SnapshotCard{
			Score:        gameast.CardConfidence(card),
			MinScore:     gameast.CardMinConfidence(card),
			NumAbilities: len(card.Abilities),
			NumFallback:  fallback,
		}
		out.TotalCards++
	}
	return out
}

// WriteSnapshot serialises s as indented JSON to path. Atomic via
// rename-from-temp so partial writes never corrupt an existing file.
func WriteSnapshot(path string, s Snapshot) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode snapshot: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s → %s: %w", tmp, path, err)
	}
	return nil
}

// ReadSnapshot loads a Snapshot from path. Returns a typed error on
// missing files so callers can distinguish "no baseline yet" from
// "baseline corrupt".
func ReadSnapshot(path string) (Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open snapshot %s: %w", path, err)
	}
	defer f.Close()
	var s Snapshot
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return Snapshot{}, fmt.Errorf("decode snapshot %s: %w", path, err)
	}
	if s.Cards == nil {
		s.Cards = map[string]SnapshotCard{}
	}
	return s, nil
}

// TrendDelta categorises one card's drift between two snapshots.
type TrendDelta struct {
	Name       string
	From       SnapshotCard
	To         SnapshotCard
	DeltaScore float64
	Kind       string // "regressed", "improved", "unchanged", "new", "dropped"
}

// Trend holds the full diff result.
type Trend struct {
	FromTotal int
	ToTotal   int
	Regressed []TrendDelta // sorted by DeltaScore ascending (biggest drop first)
	Improved  []TrendDelta // sorted by DeltaScore descending (biggest gain first)
	Unchanged int          // count of cards with DeltaScore == 0
	New       []TrendDelta // present in `to`, absent in `from`
	Dropped   []TrendDelta // present in `from`, absent in `to`
}

// ComputeTrend diffs two snapshots. Score deltas smaller than epsilon
// are treated as unchanged (float comparison hygiene). All slices are
// sorted for deterministic output; the regression list is ordered by
// biggest-drop-first so the operator sees the worst regressions at the
// top of any report.
func ComputeTrend(from, to Snapshot) Trend {
	const epsilon = 1e-9
	tr := Trend{
		FromTotal: from.TotalCards,
		ToTotal:   to.TotalCards,
	}
	seen := make(map[string]struct{}, len(from.Cards)+len(to.Cards))
	for name, fromCard := range from.Cards {
		seen[name] = struct{}{}
		toCard, ok := to.Cards[name]
		if !ok {
			tr.Dropped = append(tr.Dropped, TrendDelta{
				Name: name, From: fromCard, Kind: "dropped",
				DeltaScore: -fromCard.Score, // negative: the score "fell" to absent
			})
			continue
		}
		d := toCard.Score - fromCard.Score
		switch {
		case d < -epsilon:
			tr.Regressed = append(tr.Regressed, TrendDelta{
				Name: name, From: fromCard, To: toCard,
				DeltaScore: d, Kind: "regressed",
			})
		case d > epsilon:
			tr.Improved = append(tr.Improved, TrendDelta{
				Name: name, From: fromCard, To: toCard,
				DeltaScore: d, Kind: "improved",
			})
		default:
			tr.Unchanged++
		}
	}
	for name, toCard := range to.Cards {
		if _, ok := seen[name]; ok {
			continue
		}
		tr.New = append(tr.New, TrendDelta{
			Name: name, To: toCard, Kind: "new",
			DeltaScore: toCard.Score, // positive: the score "rose" from absent
		})
	}
	// Sort: regressed ascending (most negative first), improved
	// descending (most positive first), name asc for ties.
	sort.Slice(tr.Regressed, func(i, j int) bool {
		if tr.Regressed[i].DeltaScore != tr.Regressed[j].DeltaScore {
			return tr.Regressed[i].DeltaScore < tr.Regressed[j].DeltaScore
		}
		return tr.Regressed[i].Name < tr.Regressed[j].Name
	})
	sort.Slice(tr.Improved, func(i, j int) bool {
		if tr.Improved[i].DeltaScore != tr.Improved[j].DeltaScore {
			return tr.Improved[i].DeltaScore > tr.Improved[j].DeltaScore
		}
		return tr.Improved[i].Name < tr.Improved[j].Name
	})
	sort.Slice(tr.New, func(i, j int) bool { return tr.New[i].Name < tr.New[j].Name })
	sort.Slice(tr.Dropped, func(i, j int) bool { return tr.Dropped[i].Name < tr.Dropped[j].Name })
	return tr
}

// RenderTrendMarkdown writes a markdown report. limit caps the
// regressed + improved lists at N entries each (0 = unlimited).
func RenderTrendMarkdown(w io.Writer, tr Trend, limit int) error {
	if _, err := fmt.Fprintln(w, "# AST Confidence Trend"); err != nil {
		return err
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "**From snapshot:** %d cards\n", tr.FromTotal)
	fmt.Fprintf(w, "**To snapshot:**   %d cards\n", tr.ToTotal)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Summary")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| Bucket | Count |")
	fmt.Fprintln(w, "|--------|-------|")
	fmt.Fprintf(w, "| Regressed | %d |\n", len(tr.Regressed))
	fmt.Fprintf(w, "| Improved  | %d |\n", len(tr.Improved))
	fmt.Fprintf(w, "| Unchanged | %d |\n", tr.Unchanged)
	fmt.Fprintf(w, "| New (in `to` only) | %d |\n", len(tr.New))
	fmt.Fprintf(w, "| Dropped (in `from` only) | %d |\n", len(tr.Dropped))
	fmt.Fprintln(w)
	if err := renderTrendSection(w, "## Regressed (biggest drops first)", tr.Regressed, limit); err != nil {
		return err
	}
	if err := renderTrendSection(w, "## Improved (biggest gains first)", tr.Improved, limit); err != nil {
		return err
	}
	if len(tr.New) > 0 {
		if err := renderTrendSection(w, "## New cards (present in `to`, absent in `from`)", tr.New, limit); err != nil {
			return err
		}
	}
	if len(tr.Dropped) > 0 {
		if err := renderTrendSection(w, "## Dropped cards (present in `from`, absent in `to`)", tr.Dropped, limit); err != nil {
			return err
		}
	}
	return nil
}

func renderTrendSection(w io.Writer, header string, deltas []TrendDelta, limit int) error {
	if _, err := fmt.Fprintln(w, header); err != nil {
		return err
	}
	fmt.Fprintln(w)
	if len(deltas) == 0 {
		if _, err := fmt.Fprintln(w, "_(none)_"); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}
	show := deltas
	if limit > 0 && len(show) > limit {
		show = show[:limit]
	}
	fmt.Fprintln(w, "| Card | From | To | Δ |")
	fmt.Fprintln(w, "|------|------|----|---|")
	for _, d := range show {
		fmt.Fprintf(w, "| %s | %.2f | %.2f | %+.2f |\n",
			d.Name, d.From.Score, d.To.Score, d.DeltaScore)
	}
	if limit > 0 && len(deltas) > limit {
		fmt.Fprintf(w, "\n_(%d more not shown — pass --confidence-trend-limit 0 for all)_\n", len(deltas)-limit)
	}
	fmt.Fprintln(w)
	return nil
}

// ---------------------------------------------------------------------------
// CLI drivers
// ---------------------------------------------------------------------------

// runConfidenceSnapshot writes a snapshot of the current corpus to outPath.
func runConfidenceSnapshot(corpus *astload.Corpus, outPath string) error {
	s := BuildSnapshot(corpus)
	if err := WriteSnapshot(outPath, s); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "confidence-snapshot: wrote %d cards to %s\n", s.TotalCards, outPath)
	return nil
}

// runConfidenceTrend loads `fromPath`, loads or constructs `toPath`,
// computes the trend, and writes the report to outPath (or stdout if
// outPath is empty).
func runConfidenceTrend(corpus *astload.Corpus, fromPath, toPath, outPath string, limit int) error {
	from, err := ReadSnapshot(fromPath)
	if err != nil {
		return fmt.Errorf("load --confidence-trend-from: %w", err)
	}
	var to Snapshot
	if toPath != "" {
		to, err = ReadSnapshot(toPath)
		if err != nil {
			return fmt.Errorf("load --confidence-trend-to: %w", err)
		}
	} else {
		to = BuildSnapshot(corpus)
	}
	tr := ComputeTrend(from, to)
	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}
	if err := RenderTrendMarkdown(w, tr, limit); err != nil {
		return err
	}
	if outPath != "" {
		fmt.Fprintf(os.Stderr,
			"confidence-trend: %d regressed, %d improved, %d unchanged, %d new, %d dropped → %s\n",
			len(tr.Regressed), len(tr.Improved), tr.Unchanged, len(tr.New), len(tr.Dropped), outPath)
	}
	return nil
}
