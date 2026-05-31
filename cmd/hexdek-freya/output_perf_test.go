package main

import (
	"io"
	"testing"
)

// output_perf_test.go — benchmarks for the four output renderers.
// Build a realistic-scale FreyaReport (100 cards, populated combos /
// finishers / synergies / win lines / gameplan script) and benchmark
// each rendering format. The goal is to keep each path under 100ms
// per call so a downstream consumer (CI dashboard, hexdek.dev render
// pipeline) doesn't see latency spikes.

func fixtureReportForBench() *FreyaReport {
	// Realistic combo / finisher / synergy counts for a cEDH-shaped
	// deck. Each entry has a description with embedded mana symbols
	// to exercise the RenderMana path.
	mkCombo := func(prefix string, n int, withMana bool) []ComboResult {
		out := make([]ComboResult, n)
		desc := prefix + ": deterministic loop with stable resource gain over time"
		if withMana {
			desc = prefix + ": pay {1}{U} to blink, untap 5 lands, net {3} mana per loop"
		}
		for i := range out {
			out[i] = ComboResult{
				Cards:       []string{"Card A " + prefix, "Card B " + prefix, "Card C " + prefix},
				LoopType:    "infinite",
				Description: desc,
				Confirmed:   i%2 == 0,
			}
		}
		return out
	}

	r := &FreyaReport{
		DeckName:   "bench-deck",
		DeckPath:   "/tmp/bench.txt",
		Commander:  "Kraum, Ludevic's Opus",
		TotalCards: 100,
		TrueInfinites: mkCombo("inf", 8, true),
		Determined:    mkCombo("det", 12, true),
		Finishers:     mkCombo("fin", 6, false),
		Synergies:     mkCombo("syn", 20, false),
		ManaCurve:     [8]int{4, 12, 18, 14, 10, 6, 4, 4},
		AvgCMC:        2.85,
		CurveShape:    "midrange",
		ColorDemand:   map[string]int{"W": 8, "U": 12, "B": 6},
		ColorSupply:   map[string]int{"W": 10, "U": 14, "B": 8},
		LandCount:     34,
		NonlandCount:  66,
		Profile: &DeckProfile{
			Commander:         "Kraum, Ludevic's Opus",
			ColorIdentity:     []string{"U", "R"},
			PrimaryArchetype:  "Combo",
			Bracket:           5,
			BracketLabel:      "cEDH",
			PrimaryWinLine:    "Thassa's Oracle + Demonic Consultation",
			GameplanSummary:   "Combo deck that wins via Thassa's Oracle line. Backed by 8 tutors.",
			Strengths: []string{
				"Heavy ramp package (12 pieces)",
				"Multiple win lines (8 paths)",
				"Deep tutor suite (10 non-land tutors)",
			},
			Weaknesses: []string{
				"Fragile to graveyard hate",
				"Single-point-of-failure on Kraum",
			},
		},
	}

	// Populate gameplan script via the production builder.
	r.Profile.GameplanScript = buildGameplanScript(r.Profile, r)

	// Win lines.
	r.WinLines = &WinLineAnalysis{
		WinLines: []WinLine{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite", Tier: "S", Confidence: 0.95, Desc: "exile lib then ETB win"},
			{Pieces: []string{"Heliod", "Walking Ballista"}, Type: "infinite", Tier: "S", Confidence: 0.88, Desc: "infinite damage"},
			{Pieces: []string{"Aggravated Assault", "Sword of Feast and Famine"}, Type: "determined", Tier: "B", Confidence: 0.7, Desc: "infinite combats"},
		},
	}
	return r
}

// BenchmarkPrintText pins the text-format render latency. Should be
// well under 100ms for a realistic deck.
func BenchmarkPrintText(b *testing.B) {
	r := fixtureReportForBench()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printText(io.Discard, r)
	}
}

// BenchmarkPrintMarkdown pins the markdown-format render latency.
func BenchmarkPrintMarkdown(b *testing.B) {
	r := fixtureReportForBench()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printMarkdown(io.Discard, r)
	}
}

// BenchmarkPrintJSON pins the JSON-format render latency. JSON
// encoding does the most allocation work — typically the slowest of
// the four formats.
func BenchmarkPrintJSON(b *testing.B) {
	r := fixtureReportForBench()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printJSON(io.Discard, r)
	}
}

// BenchmarkPrintHTML pins the HTML-format render latency. Includes
// the inline CSS (~5KB) emitted once per call, plus the SVG mana
// symbol expansion across combo descriptions.
func BenchmarkPrintHTML(b *testing.B) {
	r := fixtureReportForBench()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printHTML(io.Discard, r)
	}
}

// BenchmarkRenderMana_Inline pins the per-call cost of the mana
// renderer on a typical combo description. Called dozens of times
// per render across the combo lists, so it sits on the critical path.
func BenchmarkRenderMana_Inline(b *testing.B) {
	s := "Pay {1}{U} to blink Drake → untap 5 lands → net {3} mana per loop. Repeat with {2}{R}{R} for an extra combat."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderMana(s, ManaText)
	}
}
