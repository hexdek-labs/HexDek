package outcome

import "github.com/hexdek/hexdek/internal/judge"

// report.go — OUTCOME dimension registration through the Judge router
// (phase 3 final): every finding the harness produces is emitted as a
// canonical ValidationViolation through judge.LogViolation at origin
// (RunEffect), tagged Surface=outcome / Dimension=outcome. Drivers
// (corpus audits, loki) consume the returned Finding for their reports;
// the violation stream is the Judge-facing record.

// Canonical maps a Finding onto the canonical violation vocabulary.
func (f *Finding) Canonical() judge.ValidationViolation {
	return judge.ValidationViolation{
		Surface:   judge.SurfaceOutcome,
		Dimension: judge.DimensionOutcome,
		Name:      f.Kind,
		Severity:  judge.SeverityCritical,
		Message:   f.CardName + ": expected " + f.Expected + ", actual " + f.Actual,
		Seat:      -1, // scenario boards: divergence is per-effect, not per-seat
		Context: map[string]interface{}{
			"card":     f.CardName,
			"expected": f.Expected,
			"actual":   f.Actual,
			"raw":      f.Raw,
		},
	}
}

// EmitFinding routes one finding through the Judge router. Exported so
// drivers that construct findings outside RunEffect (widened harnesses)
// route through the same origin.
func EmitFinding(f *Finding) {
	if f == nil {
		return
	}
	judge.LogViolation(f.Canonical())
}
