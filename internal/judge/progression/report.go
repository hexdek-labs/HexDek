package progression

import (
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

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

// perCardOwned reports whether a per_card handler owns this card's
// trigger for the engine event family — the checker's out-of-scope
// gate (r63 PROGRESSION residual round). The independence contract
// models EXPECTATIONS from the AST: per_card-owned triggers are
// exactly the cards where the engine resolves the handler INSTEAD of
// the AST (the r63 double-fire gates), and handlers compute REAL
// dynamic values (count zombies you control, half the hand, X = power)
// that a spec-pinned expectation cannot soundly model — the residual
// sweep proved every such comparison a false positive against a
// CORRECT engine (Gyruda's reanimate-back, Lord Xander's true
// half-hand, Krenko's X tokens, Worldspine's self-shuffle…). Skipping
// keeps the score honest; per_card behavioral coverage lives in the
// per_card package's own tests.
func perCardOwned(cardName string, events ...string) bool {
	if gameengine.HasETBHook != nil {
		for _, ev := range events {
			if ev == "etb" && gameengine.HasETBHook(cardName) {
				return true
			}
		}
	}
	return gameengine.PerCardOwnsTrigger(cardName, events...)
}
