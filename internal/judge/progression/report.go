package progression

import "github.com/hexdek/hexdek/internal/judge"

// report.go — PROGRESSION dimension registration through the Judge
// router (phase 3 final): every finding the trigger-correctness checks
// produce is emitted as a canonical ValidationViolation through
// judge.LogViolation at origin (CheckTrigger / CheckPhaseTrigger /
// CheckLTBTrigger), tagged Surface=progression / Dimension=progression.

// Canonical maps a Finding onto the canonical violation vocabulary.
func (f *Finding) Canonical() judge.ValidationViolation {
	return judge.ValidationViolation{
		Surface:   judge.SurfaceProgression,
		Dimension: judge.DimensionProgression,
		Name:      f.Event + "/" + f.Check,
		Severity:  judge.SeverityCritical,
		Message:   f.CardName + ": expected " + f.Expected + ", actual " + f.Actual,
		Seat:      -1, // scenario boards: divergence is per-trigger, not per-seat
		Context: map[string]interface{}{
			"card":     f.CardName,
			"event":    f.Event,
			"check":    f.Check,
			"expected": f.Expected,
			"actual":   f.Actual,
			"raw":      f.Raw,
		},
	}
}

// EmitFinding routes one finding through the Judge router.
func EmitFinding(f *Finding) {
	if f == nil {
		return
	}
	judge.LogViolation(f.Canonical())
}

// emitAll routes a batch at origin — the Check* functions call this on
// their findings before returning them to the driver.
func emitAll(fs []*Finding) {
	for _, f := range fs {
		EmitFinding(f)
	}
}
