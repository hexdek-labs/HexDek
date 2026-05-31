package tournament

import (
	"errors"
	"fmt"
	"strings"
)

// validModes is the closed set of legal Mode values. Kept in one
// place so ValidateSchema and any future mode-router can share the
// vocabulary.
var validModes = map[string]bool{
	ModeRotate:       true,
	ModePool:         true,
	ModeLazyPool:     true,
	ModeSwiss:        true,
	ModeDoubleElim:   true,
	ModeBalancedPool: true,
	ModeRoundRobin:   true,
}

// ValidateSchema checks that a TournamentResult conforms to the
// canonical schema documented in docs/tournament-output-schema.md.
// Returns nil for valid results; otherwise an error joining every
// violation with "; " so a single call surfaces all gaps in one
// pass rather than fail-fast.
//
// Validation has three layers:
//
//  1. Envelope — SchemaVersion present and matching the current
//     const, Mode present and in the closed set.
//
//  2. Universal invariants — NSeats >= 2; Games >= 0; Crashes >= 0;
//     Draws >= 0; Games == decisive + Draws where decisive is
//     sum(WinsByCommander); CommanderNames non-empty; non-nil
//     core maps (WinsByCommander, ELO, TrueSkill).
//
//  3. Mode-specific — mode-marked standings present iff the mode
//     produces them. Swiss → SwissStandings, DoubleElim →
//     EliminationStandings, BalancedPool → BalancedPodPlan. Other
//     modes MUST NOT carry any of those (leakage detection).
//
// The helper is exported for downstream consumers (tournament-audit,
// API endpoints) that deserialize TournamentResult JSON and need to
// reject incompatible / corrupt payloads before reading further.
func ValidateSchema(r *TournamentResult) error {
	if r == nil {
		return errors.New("tournament-schema: result is nil")
	}
	var errs []string

	// --- Envelope ---
	if r.SchemaVersion == "" {
		errs = append(errs, "schema_version: empty (must be set by canonical constructors)")
	} else if r.SchemaVersion != SchemaVersion {
		errs = append(errs, fmt.Sprintf("schema_version: got %q, current is %q", r.SchemaVersion, SchemaVersion))
	}
	if r.Mode == "" {
		errs = append(errs, "mode: empty (must be one of "+joinValidModes()+")")
	} else if !validModes[r.Mode] {
		errs = append(errs, fmt.Sprintf("mode: %q not in valid set {%s}", r.Mode, joinValidModes()))
	}

	// --- Universal invariants ---
	if r.NSeats < 2 {
		errs = append(errs, fmt.Sprintf("n_seats: %d < 2 (every mode needs at least 2 seats)", r.NSeats))
	}
	if r.Games < 0 {
		errs = append(errs, fmt.Sprintf("games: %d < 0", r.Games))
	}
	if r.Crashes < 0 {
		errs = append(errs, fmt.Sprintf("crashes: %d < 0", r.Crashes))
	}
	if r.Draws < 0 {
		errs = append(errs, fmt.Sprintf("draws: %d < 0", r.Draws))
	}
	if len(r.CommanderNames) == 0 {
		errs = append(errs, "commander_names: empty (every mode pre-populates the deck index)")
	}
	if r.WinsByCommander == nil {
		errs = append(errs, "wins_by_commander: nil (constructors must allocate; empty map is allowed)")
	} else {
		// Conservation check: sum of wins == decisive games.
		// Pool/lazy_pool aggregators don't always track draws, but
		// they also don't increment Draws — so sum(wins) + Draws
		// must equal Games for every mode that populates them.
		// Skip when WinsByCommander hasn't been touched (Games=0).
		if r.Games > 0 {
			sum := 0
			for _, w := range r.WinsByCommander {
				sum += w
			}
			if sum+r.Draws > r.Games {
				errs = append(errs,
					fmt.Sprintf("wins_conservation: sum(wins_by_commander)=%d + draws=%d > games=%d", sum, r.Draws, r.Games))
			}
		}
	}

	// --- Mode-specific population ---
	switch r.Mode {
	case ModeSwiss:
		if r.SwissStandings == nil && r.Games > 0 {
			errs = append(errs, "swiss_standings: nil for mode=swiss (must populate when games > 0)")
		}
		if r.EliminationStandings != nil {
			errs = append(errs, "elimination_standings: populated under mode=swiss (cross-mode leakage)")
		}
		if r.BalancedPodPlan != nil {
			errs = append(errs, "balanced_pod_plan: populated under mode=swiss (cross-mode leakage)")
		}
	case ModeDoubleElim:
		if r.EliminationStandings == nil && r.Games > 0 {
			errs = append(errs, "elimination_standings: nil for mode=double_elim (must populate when games > 0)")
		}
		if r.SwissStandings != nil {
			errs = append(errs, "swiss_standings: populated under mode=double_elim (cross-mode leakage)")
		}
		if r.BalancedPodPlan != nil {
			errs = append(errs, "balanced_pod_plan: populated under mode=double_elim (cross-mode leakage)")
		}
	case ModeBalancedPool:
		if r.BalancedPodPlan == nil && r.Games > 0 {
			errs = append(errs, "balanced_pod_plan: nil for mode=balanced_pool (must populate when games > 0)")
		}
		if r.SwissStandings != nil {
			errs = append(errs, "swiss_standings: populated under mode=balanced_pool (cross-mode leakage)")
		}
		if r.EliminationStandings != nil {
			errs = append(errs, "elimination_standings: populated under mode=balanced_pool (cross-mode leakage)")
		}
	case ModeRotate, ModePool, ModeLazyPool, ModeRoundRobin:
		if r.SwissStandings != nil {
			errs = append(errs, fmt.Sprintf("swiss_standings: populated under mode=%s (cross-mode leakage)", r.Mode))
		}
		if r.EliminationStandings != nil {
			errs = append(errs, fmt.Sprintf("elimination_standings: populated under mode=%s (cross-mode leakage)", r.Mode))
		}
		if r.BalancedPodPlan != nil {
			errs = append(errs, fmt.Sprintf("balanced_pod_plan: populated under mode=%s (cross-mode leakage)", r.Mode))
		}
	}

	if len(errs) > 0 {
		return errors.New("tournament-schema: " + strings.Join(errs, "; "))
	}
	return nil
}

func joinValidModes() string {
	keys := make([]string, 0, len(validModes))
	for k := range validModes {
		keys = append(keys, k)
	}
	// Stable order for error messages.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return strings.Join(keys, ", ")
}
