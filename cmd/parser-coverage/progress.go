package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProgressPoint is one HistoryEntry distilled down to the fields the
// chart actually plots. SuccessPct = (OK + OK_VANILLA) / OracleTotal —
// the headline coverage rate that "we went from 65% to 78%" reporting
// is about. Uncovered is the bucket sum (Missing + EmptyAST + Partial)
// kept for the table column.
type ProgressPoint struct {
	Timestamp   time.Time
	Label       string
	OracleTotal int
	OK          int
	Uncovered   int
	SuccessPct  float64
}

// entryToProgress derives a ProgressPoint from a HistoryEntry. Zero
// OracleTotal yields SuccessPct=0 rather than NaN so callers can plot
// the point without guarding.
func entryToProgress(e HistoryEntry) ProgressPoint {
	covered := e.OK + e.OKVanilla
	pct := 0.0
	if e.OracleTotal > 0 {
		pct = 100.0 * float64(covered) / float64(e.OracleTotal)
	}
	return ProgressPoint{
		Timestamp:   e.Timestamp,
		Label:       e.Label,
		OracleTotal: e.OracleTotal,
		OK:          covered,
		Uncovered:   e.Missing + e.EmptyAST + e.Partial,
		SuccessPct:  pct,
	}
}

// computeProgress maps a slice of HistoryEntries to the chart-ready
// ProgressPoint series. Order is preserved (history is append-only,
// so it's already chronological).
func computeProgress(entries []HistoryEntry) []ProgressPoint {
	out := make([]ProgressPoint, len(entries))
	for i, e := range entries {
		out[i] = entryToProgress(e)
	}
	return out
}

// ProgressSummary is the headline "X% → Y% (+Z pp)" data extracted
// from the first and last entries in the series.
type ProgressSummary struct {
	First, Last ProgressPoint
	DeltaPP     float64
	Span        time.Duration
	Points      int
}

func summarizeProgress(points []ProgressPoint) ProgressSummary {
	if len(points) == 0 {
		return ProgressSummary{}
	}
	first := points[0]
	last := points[len(points)-1]
	return ProgressSummary{
		First:   first,
		Last:    last,
		DeltaPP: last.SuccessPct - first.SuccessPct,
		Span:    last.Timestamp.Sub(first.Timestamp),
		Points:  len(points),
	}
}

// sparkBraille is the 8-level Unicode block-char palette used by the
// sparkline rendering. Lowest value maps to ▁, highest to █.
const sparkBraille = "▁▂▃▄▅▆▇█"

// sparkline maps a value series to a single-line Unicode block-char
// chart. Empty input returns "". A flat series (all values equal)
// renders as N copies of the middle block — visually neutral.
func sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	runes := []rune(sparkBraille)
	rng := max - min
	var b strings.Builder
	for _, v := range values {
		if rng == 0 {
			b.WriteRune(runes[len(runes)/2])
			continue
		}
		ratio := (v - min) / rng
		idx := int(ratio * float64(len(runes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(runes) {
			idx = len(runes) - 1
		}
		b.WriteRune(runes[idx])
	}
	return b.String()
}

// humanDuration renders a time.Duration as a coarse human-readable
// string ("3 days", "2 months", "5 hours"). Used in the chart's
// summary line where exact precision doesn't matter.
func humanDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days >= 60:
		return fmt.Sprintf("%d months", days/30)
	case days >= 1:
		return fmt.Sprintf("%d days", days)
	}
	if h := int(d.Hours()); h >= 1 {
		return fmt.Sprintf("%d hours", h)
	}
	if m := int(d.Minutes()); m >= 1 {
		return fmt.Sprintf("%d minutes", m)
	}
	return "<1 minute"
}

// renderProgressMarkdown produces a markdown report with a sparkline
// + per-run table. Suitable for pasting into a PR description, a
// release-notes section, or a docs/ file.
func renderProgressMarkdown(points []ProgressPoint, title string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	if len(points) == 0 {
		b.WriteString("_No history entries yet. Run `parser-coverage --history <path>` to record one._\n")
		return []byte(b.String())
	}
	sum := summarizeProgress(points)
	fmt.Fprintf(&b, "**%.2f%% → %.2f%%** (%+.2f pp over %d run%s",
		sum.First.SuccessPct, sum.Last.SuccessPct, sum.DeltaPP, sum.Points, pluralS(sum.Points))
	if sum.Span > 0 {
		fmt.Fprintf(&b, ", %s", humanDuration(sum.Span))
	}
	b.WriteString(")\n\n")

	// Sparkline.
	pcts := make([]float64, len(points))
	for i, p := range points {
		pcts[i] = p.SuccessPct
	}
	fmt.Fprintf(&b, "```\n%s\n```\n\n", sparkline(pcts))

	// Per-run table — chronological, with delta vs previous row.
	fmt.Fprintf(&b, "| # | When | Label | Oracle total | OK | Uncovered | Success%% | Δ |\n")
	fmt.Fprintf(&b, "|---:|---|---|---:|---:|---:|---:|---:|\n")
	for i, p := range points {
		delta := "—"
		if i > 0 {
			delta = fmt.Sprintf("%+.2f pp", p.SuccessPct-points[i-1].SuccessPct)
		}
		label := p.Label
		if label == "" {
			label = "—"
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %d | %d | %d | %.2f%% | %s |\n",
			i+1, p.Timestamp.Format("2006-01-02"), label, p.OracleTotal, p.OK, p.Uncovered, p.SuccessPct, delta)
	}
	return []byte(b.String())
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Chart layout constants. The plot area is the rectangle between
// (padL, padT) and (width-padR, height-padB). Values are pixels.
const (
	svgWidth = 720
	svgHeight = 360
	svgPadL  = 60
	svgPadR  = 24
	svgPadT  = 56
	svgPadB  = 50
)

// renderProgressSVG produces a self-contained SVG line chart of the
// success-rate curve. The Y-axis is pinned to 0-100% so charts from
// different runs compare visually at a glance — useful for the
// "65% to 78%" reporting use case where the absolute level matters
// more than zoomed-in variation.
//
// SVG is plain string-building, no external SVG libraries — the chart
// shape is simple enough (axes, gridlines, polyline, dots, labels)
// that a dedicated lib would be overkill.
func renderProgressSVG(points []ProgressPoint, title string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d">`,
		svgWidth, svgHeight, svgWidth, svgHeight)
	b.WriteString(`<rect width="100%" height="100%" fill="white"/>`)

	fmt.Fprintf(&b, `<text x="%d" y="24" font-family="-apple-system, sans-serif" font-size="16" font-weight="600" fill="#111">%s</text>`,
		svgPadL, escapeXML(title))

	if len(points) == 0 {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="sans-serif" font-size="14" fill="#888" text-anchor="middle">no history entries</text>`,
			svgWidth/2, svgHeight/2)
		b.WriteString(`</svg>`)
		return []byte(b.String())
	}

	sum := summarizeProgress(points)
	summary := fmt.Sprintf("%.2f%% → %.2f%% (%+.2f pp · %d run%s",
		sum.First.SuccessPct, sum.Last.SuccessPct, sum.DeltaPP, sum.Points, pluralS(sum.Points))
	if sum.Span > 0 {
		summary += " · " + humanDuration(sum.Span)
	}
	summary += ")"
	fmt.Fprintf(&b, `<text x="%d" y="44" font-family="-apple-system, sans-serif" font-size="12" fill="#444">%s</text>`,
		svgPadL, escapeXML(summary))

	plotL := svgPadL
	plotR := svgWidth - svgPadR
	plotT := svgPadT
	plotB := svgHeight - svgPadB
	plotW := plotR - plotL
	plotH := plotB - plotT

	// Axes (just baseline + Y-axis line).
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#bbb"/>`, plotL, plotB, plotR, plotB)
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#bbb"/>`, plotL, plotT, plotL, plotB)

	// Y gridlines + tick labels at 0/25/50/75/100%.
	for _, pct := range []int{0, 25, 50, 75, 100} {
		y := float64(plotB) - float64(pct)/100.0*float64(plotH)
		fmt.Fprintf(&b, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#eee"/>`, plotL, y, plotR, y)
		fmt.Fprintf(&b, `<text x="%d" y="%.1f" font-family="sans-serif" font-size="11" fill="#666" text-anchor="end">%d%%</text>`,
			plotL-6, y+4, pct)
	}

	// Project points to chart coordinates.
	type xy struct{ X, Y float64 }
	coords := make([]xy, len(points))
	for i, p := range points {
		var x float64
		if len(points) == 1 {
			x = float64(plotL) + float64(plotW)/2
		} else {
			x = float64(plotL) + float64(i)/float64(len(points)-1)*float64(plotW)
		}
		y := float64(plotB) - p.SuccessPct/100.0*float64(plotH)
		coords[i] = xy{x, y}
	}

	// Polyline connecting points (skipped on single-point series —
	// a 1-coord polyline is degenerate and some renderers warn).
	if len(coords) >= 2 {
		var poly strings.Builder
		for i, c := range coords {
			if i > 0 {
				poly.WriteString(" ")
			}
			fmt.Fprintf(&poly, "%.1f,%.1f", c.X, c.Y)
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#2471a3" stroke-width="2"/>`, poly.String())
	}

	// Data point dots.
	for _, c := range coords {
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="3.5" fill="#2471a3"/>`, c.X, c.Y)
	}

	// X-axis: timestamp labels at start + end.
	firstLabel := points[0].Timestamp.Format("2006-01-02")
	lastLabel := points[len(points)-1].Timestamp.Format("2006-01-02")
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="sans-serif" font-size="11" fill="#666">%s</text>`,
		plotL, plotB+18, firstLabel)
	if len(points) > 1 {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="sans-serif" font-size="11" fill="#666" text-anchor="end">%s</text>`,
			plotR, plotB+18, lastLabel)
	}

	b.WriteString(`</svg>`)
	return []byte(b.String())
}

// escapeXML escapes the minimal set of characters needed to make a
// string safe in an XML text node. Used for titles and summary
// strings that could (in principle) contain & or < from a label.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// writeProgressChart writes a chart for the given progress series.
// Format is detected from the file extension: .svg renders the line
// chart; .md / .markdown renders the markdown sparkline + table.
// Any other extension (or none) errors out so the user gets a
// clear message rather than a binary blob in a .txt.
func writeProgressChart(path string, points []ProgressPoint, title string) error {
	ext := strings.ToLower(filepath.Ext(path))
	var data []byte
	switch ext {
	case ".svg":
		data = renderProgressSVG(points, title)
	case ".md", ".markdown":
		data = renderProgressMarkdown(points, title)
	default:
		return fmt.Errorf("unsupported chart format %q (want .svg or .md)", ext)
	}
	return os.WriteFile(path, data, 0o644)
}
