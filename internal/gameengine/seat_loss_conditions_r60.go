package gameengine

// R60 — two-level loss/win architecture (port of internal/game/ #680 to the
// real rules engine).
//
// Level 1 (per-seat, this file): Seat.CheckLossConditions classifies a seat
// against its own CR §704.5 / §903.10a clauses as a PURE read-only method.
// Mirror of Player.CheckLossConditions in the MVP layer.
//
// Level 2 (engine, multiplayer.go CheckEnd): existing master aggregator
// applies CR §104.2a (last-seat-wins) + §104.3b (simultaneous-elim-draw)
// AND, as of this same R60 wave, §104.2c "you win the game" effects via
// the Seat.Won flag (Approach of the Second Sun's second cast, Felidar
// Sovereign upkeep, Maze's End at 10 gates, Lab Maniac empty-library
// alt-win, etc.). §104.3a — "loss beats win when simultaneous on the
// same player" — is enforced by gating the Won-scan on `!s.Lost` (an
// SBA pass that marked the seat Lost in the same window suppresses the
// otherwise-pending win).
//
// The per-clause SBA functions in sba.go (sba704_5a/b/c/6c) keep their
// existing canonical role — they own the replacement-event firing
// (FireLoseGameEvent for Platinum Angel cancellation), the log-spam
// suppression (SBA704_5a_emitted), and the markSeatLost state mutation
// that this classifier deliberately does NOT perform. CheckLossConditions
// is the read-only view used by:
//
//   - downstream analytics (Heimdall, Loki invariants) classifying a
//     seat's status without firing SBA;
//   - AI evaluators (hat) asking "would this seat be lost right now?";
//   - the symmetric public API surface with internal/game/.

// LossCause identifies which CR §704.5 / §903.10a clause currently
// applies to a seat. Stable string values — they're referenced in the
// LossReason text written by the SBA pipeline and surfaced in
// Heimdall post-game summaries. New clauses get a new constant; existing
// values must not change.
type LossCause string

const (
	// LossNone — the seat meets no §704.5 / §903.10a loss condition.
	LossNone LossCause = ""
	// LossLife — CR §704.5a: life total of 0 or less.
	LossLife LossCause = "life_zero_or_less"
	// LossEmptyLibrary — CR §704.5b: attempted to draw from an empty
	// library since the last SBA check.
	LossEmptyLibrary LossCause = "attempted_empty_library_draw"
	// LossPoison — CR §704.5c: 10 or more poison counters.
	LossPoison LossCause = "ten_or_more_poison_counters"
	// LossCommanderDamage — CR §704.6c / §903.10a: 21+ combat damage
	// from a single commander. Only meaningful when
	// GameState.CommanderFormat == true.
	LossCommanderDamage LossCause = "twenty_one_commander_damage"
)

// CheckLossConditions evaluates this seat against its own §704.5 /
// §903.10a clauses in isolation. Returns the FIRST matched cause —
// when a seat satisfies multiple clauses simultaneously (life=-5 AND
// poison=12), only one cause is reported. The SBA pipeline still fires
// each clause's own §704.5X function independently so all applicable
// loss-events get logged; this method is the consolidated read-only
// view, not the canonical SBA driver.
//
// Pure: never mutates Seat or GameState, never fires events, never
// invokes replacement effects. A seat that would normally be saved by
// a would_lose_game replacement (Platinum Angel) STILL reports lost
// here — the classifier doesn't model replacement effects; callers who
// need the "after Platinum Angel" view should ask the SBA pipeline.
//
// The GameState argument is consumed only for the format gate on
// commander-damage (CR §704.6c only applies in Commander). Passing nil
// is safe — the commander-damage clause is silently skipped.
//
// Clause ordering (first match wins): Life → EmptyLibrary → Poison →
// CommanderDamage. Stable — referenced by tests pinning multi-clause
// behavior.
func (s *Seat) CheckLossConditions(gs *GameState) (LossCause, bool) {
	if s == nil {
		return LossNone, false
	}
	// §704.5a — life total of 0 or less.
	if s.Life <= 0 {
		return LossLife, true
	}
	// §704.5b — attempted to draw from an empty library.
	if s.AttemptedEmptyDraw {
		return LossEmptyLibrary, true
	}
	// §704.5c — ten or more poison counters.
	if s.PoisonCounters >= 10 {
		return LossPoison, true
	}
	// §704.6c / §903.10a — commander damage 21+ (format-gated).
	if gs != nil && gs.CommanderFormat {
		for _, byName := range s.CommanderDamage {
			for _, dmg := range byName {
				if dmg >= 21 {
					return LossCommanderDamage, true
				}
			}
		}
	}
	return LossNone, false
}
