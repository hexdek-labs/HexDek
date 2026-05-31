package main

import (
	"io"
	"testing"
)

// output_perf_scale_test.go — pathological-scale benchmarks. The
// realistic-fixture benchmarks (output_perf_test.go) cover a 100-card
// deck with ~46 combos; this file scales each combo bucket to 10x to
// surface latency on a worst-case deck (Spellbook-merged cEDH with
// every two-card combo line populated).
//
// Goal: every render path stays under 100ms even at this scale. The
// user's optimization gate is "if any step is >100ms, optimize" —
// these benchmarks establish whether the bar is currently met OR
// whether a future combo-database expansion would push past it.

func fixtureReportXXL() *FreyaReport {
	r := fixtureReportForBench()
	// 10x combo populations to simulate a Spellbook-fully-merged deck.
	multiply := func(src []ComboResult, factor int) []ComboResult {
		out := make([]ComboResult, 0, len(src)*factor)
		for f := 0; f < factor; f++ {
			out = append(out, src...)
		}
		return out
	}
	r.TrueInfinites = multiply(r.TrueInfinites, 10)
	r.Determined = multiply(r.Determined, 10)
	r.Finishers = multiply(r.Finishers, 10)
	r.Synergies = multiply(r.Synergies, 10)
	return r
}

func BenchmarkPrintText_XXL(b *testing.B) {
	r := fixtureReportXXL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printText(io.Discard, r)
	}
}

func BenchmarkPrintMarkdown_XXL(b *testing.B) {
	r := fixtureReportXXL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printMarkdown(io.Discard, r)
	}
}

func BenchmarkPrintJSON_XXL(b *testing.B) {
	r := fixtureReportXXL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printJSON(io.Discard, r)
	}
}

func BenchmarkPrintHTML_XXL(b *testing.B) {
	r := fixtureReportXXL()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printHTML(io.Discard, r)
	}
}

// TestOutputRenderBudget asserts the 100ms-per-render contract
// against the realistic fixture. Runs each renderer once and fails
// if any exceeds the budget. The benchmarks above provide finer
// per-call latency; this test is the build-time tripwire.
func TestOutputRenderBudget(t *testing.T) {
	const budgetNS = 100 * 1_000_000 // 100ms in nanoseconds
	cases := []struct {
		name string
		fn   func(*FreyaReport)
	}{
		{"text", func(r *FreyaReport) { printText(io.Discard, r) }},
		{"markdown", func(r *FreyaReport) { printMarkdown(io.Discard, r) }},
		{"json", func(r *FreyaReport) { printJSON(io.Discard, r) }},
		{"html", func(r *FreyaReport) { printHTML(io.Discard, r) }},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name+"_realistic", func(t *testing.T) {
			r := fixtureReportForBench()
			res := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					c.fn(r)
				}
			})
			if res.NsPerOp() > budgetNS {
				t.Errorf("%s render exceeded 100ms budget: %.2fms/op",
					c.name, float64(res.NsPerOp())/1_000_000)
			} else {
				t.Logf("%s OK: %.2fms/op", c.name, float64(res.NsPerOp())/1_000_000)
			}
		})
		t.Run(c.name+"_xxl", func(t *testing.T) {
			r := fixtureReportXXL()
			res := testing.Benchmark(func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					c.fn(r)
				}
			})
			if res.NsPerOp() > budgetNS {
				t.Errorf("%s render exceeded 100ms budget at XXL scale: %.2fms/op",
					c.name, float64(res.NsPerOp())/1_000_000)
			} else {
				t.Logf("%s OK at XXL: %.2fms/op", c.name, float64(res.NsPerOp())/1_000_000)
			}
		})
	}
}
