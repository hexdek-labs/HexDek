package judge

import "fmt"

// stateintegrity.go — the STATE-INTEGRITY dimension's end-of-game
// checks (Hex Judge phase 3 fold; pattern per 44e84b1e / legality.go).
//
// Three groups make up the STATE-INTEGRITY dimension:
//
//  1. END-OF-GAME SBA COMPLIANCE (this file): the §704.5 "should have
//     lost" checks — life (§704.5a), poison (§704.5c), commander damage
//     (§704.6c, historically tagged 704.5v here) — plus the CR §104.2a
//     exactly-one-winner check. Promoted verbatim from hat/feynman.go
//     (whose standalone check bodies are deleted); Feynman's CheckGame
//     is now a driver that builds a GameSnapshot and forwards. Checks
//     assert on a neutral snapshot shape — no engine dependency; the
//     caller resolves engine-specific facts (can't-lose shields) before
//     calling.
//
//  2. INLINE INVARIANTS (gameengine/invariants.go): StackIntegrity,
//     TurnStructure, LayerIdempotency, AttachmentConsistency,
//     ExileLinkageIntegrity, ReplacementCompleteness et al — registered
//     on the engine's Invariant table with explicit Dimension tags;
//     RunAllInvariants stamps and routes.
//
//  3. SEAT-OUTCOME SELF-CHECKER (gameengine/seat_outcome.go): win/loss
//     truth + leave-game cleanup; its Canonical() stamps this dimension
//     and routes at record time.
//
// Every violation found here is routed through LogViolation (with
// Surface set, so drivers' normalization loops know it is already
// routed) AND returned for the driver's report.

// ---------------------------------------------------------------------------
// Input shape
// ---------------------------------------------------------------------------

// SeatSnapshot is one seat's end-of-game state, resolved by the caller.
type SeatSnapshot struct {
	// Seat index.
	Seat int
	// Life total.
	Life int
	// Lost marks the seat eliminated.
	Lost bool
	// PoisonCounters on the seat.
	PoisonCounters int
	// CommanderDamage is the per-commander combat damage this seat has
	// RECEIVED (commander name → total). Callers flatten their richer
	// per-source maps to the max per name.
	CommanderDamage map[string]int
	// CantLoseShield is true when a "would lose the game" replacement
	// (Platinum Angel class) is registered for this seat — resolved by
	// the caller since it requires engine replacement-registry access.
	CantLoseShield bool
	// SBALossEmitted is true when the engine's §704.5a SBA recorded a
	// loss-or-prevention for this seat's current life drop.
	SBALossEmitted bool
}

// GameSnapshot is the Judge's neutral end-of-game state-integrity input.
type GameSnapshot struct {
	// Seats holds one snapshot per occupied seat.
	Seats []SeatSnapshot
	// TotalSeats is the table size including vacated/nil seats — the
	// CR §104.2a expected-losers count is TotalSeats-1.
	TotalSeats int
	// Ended is true when the engine flipped its game-ended flag.
	Ended bool
	// HasWinner is true when a winner was recorded.
	HasWinner bool
}

// ---------------------------------------------------------------------------
// CheckStateIntegrity — the STATE-INTEGRITY end-of-game checks.
// ---------------------------------------------------------------------------

// CheckStateIntegrity runs the §704.5 SBA-compliance and §104.2a
// winner-truth checks on a completed game's snapshot. Violations are
// routed through LogViolation and returned.
func CheckStateIntegrity(g GameSnapshot) []ValidationViolation {
	var out []ValidationViolation
	emit := func(v ValidationViolation) {
		v.Surface = SurfaceFeynman
		v.Dimension = DimensionStateIntegrity
		LogViolation(v)
		out = append(out, v)
	}

	lost, alive := 0, 0
	for _, s := range g.Seats {
		if s.Lost {
			lost++
		} else {
			alive++
		}

		// §704.5a — life 0 or less means the seat should be Lost.
		// Exception: "can't lose the game" effects (Platinum Angel,
		// Lich's Mastery) cancel the loss via the would_lose_game
		// replacement; when the SBA never recorded the drop AND a
		// shield is registered, downgrade to info.
		if s.Life <= 0 && !s.Lost {
			severity := SeverityCritical
			if !s.SBALossEmitted && s.CantLoseShield {
				severity = SeverityInfo
			}
			emit(ValidationViolation{
				Name:     "704.5a",
				Message:  fmt.Sprintf("seat %d has %d life but is not marked lost", s.Seat, s.Life),
				Seat:     s.Seat,
				Severity: severity,
				Context:  map[string]interface{}{"life": s.Life},
			})
		}

		// §704.5c — 10+ poison counters.
		if s.PoisonCounters >= 10 && !s.Lost {
			emit(ValidationViolation{
				Name:     "704.5c",
				Message:  fmt.Sprintf("seat %d has %d poison but is not lost", s.Seat, s.PoisonCounters),
				Seat:     s.Seat,
				Severity: SeverityCritical,
				Context:  map[string]interface{}{"poison": s.PoisonCounters},
			})
		}

		// §704.6c — 21+ combat damage from a single commander
		// (historically reported as 704.5v by the Feynman original;
		// the Name is kept for report continuity).
		if !s.Lost {
			for cmdrName, dmg := range s.CommanderDamage {
				if dmg >= 21 {
					emit(ValidationViolation{
						Name: "704.5v",
						Message: fmt.Sprintf("seat %d has %d commander damage from %s but is not lost",
							s.Seat, dmg, cmdrName),
						Seat:     s.Seat,
						Severity: SeverityCritical,
						Context:  map[string]interface{}{"commander": cmdrName, "damage": dmg},
					})
				}
			}
		}
	}

	// CR §104.2a exactly-one-winner. ONLY a last-seat-standing win
	// (ended + winner + exactly one alive) implies N-1 Lost; turn-cap
	// leader finishes, §104.2c win effects, and §104.3b/§104.4a draws
	// legitimately violate the naive form (the unconditional version
	// produced ~8,764 fishtank false positives — keep the gating).
	if g.Ended && g.HasWinner && alive == 1 {
		expected := g.TotalSeats - 1
		if lost != expected {
			emit(ValidationViolation{
				Name:     "game_end",
				Message:  fmt.Sprintf("last-seat-standing win but %d of %d seats lost (expected %d)", lost, g.TotalSeats, expected),
				Seat:     -1,
				Severity: SeverityCritical,
				Context:  map[string]interface{}{"lost": lost, "total": g.TotalSeats},
			})
		}
	}

	return out
}
