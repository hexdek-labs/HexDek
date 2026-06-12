package hat

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

// Feynman Oracle — provably-correct invariant checker.
//
// Runs after each completed game and verifies that the engine's final
// state satisfies fundamental MTG rules invariants. Violations indicate
// engine bugs (SBA missed, combat damage wrong, zone tracking broken).
//
// Named after Feynman's "checking from a different direction" principle:
// the main engine plays forward fast; the oracle checks backward slow.

// OracleViolation describes a single rules invariant that was broken.
// OracleViolation is the canonical violation type (consolidation step 4
// collapsed the Feynman-local struct into validation's one vocabulary;
// field mapping was Rule→Name, Description→Message, Details→Context.
// String() now renders the canonical "[sev] surface/name [seat N]: msg"
// shape).
type OracleViolation = judge.ValidationViolation

// OracleResult is the output of a Feynman check on one completed game.
type OracleResult struct {
	Violations []OracleViolation
	GameTurns  int
	Checked    int // number of invariants checked
}

func (r OracleResult) Clean() bool { return len(r.Violations) == 0 }

// CheckGame runs all Feynman invariants on a completed game state.
// Call after the game loop exits but before cleanup.
func CheckGame(gs *gameengine.GameState) OracleResult {
	result := OracleResult{GameTurns: gs.Turn}
	checks := []func(*gameengine.GameState, *OracleResult){
		checkLifeSBA,
		checkToughnessSBA,
		checkPoisonSBA,
		checkCommanderDamageSBA,
		judgeConservation,
		checkExactlyOneWinner,
		checkTurnBounds,
		checkNoNegativeCounters,
		checkPermanentTypes,
	}
	for _, check := range checks {
		check(gs, &result)
		result.Checked++
	}
	// Consolidation step 4: stamp the surface tag and route every
	// violation through the unified LogViolation sink (the Hex Judge
	// observation seam). Emitters in this file don't set Surface
	// individually; this is the single normalization point.
	for i := range result.Violations {
		if result.Violations[i].Surface == "" {
			result.Violations[i].Surface = judge.SurfaceFeynman
		}
		judge.LogViolation(result.Violations[i])
	}
	return result
}

// §704.5a — A player with 0 or less life loses the game.
// Exception: "can't lose the game" effects (Platinum Angel, Lich's Mastery)
// prevent the loss via FireLoseGameEvent. The SBA fires but the replacement
// cancels it, so the player legitimately has ≤0 life without being Lost.
// We detect this by checking SBA704_5a_emitted: if false and life ≤0, the
// SBA checked but something prevented the loss → downgrade to info.
func checkLifeSBA(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Life <= 0 && !s.Lost {
			severity := "critical"
			if !s.SBA704_5a_emitted && hasCantLoseEffect(gs, i) {
				severity = "info"
			}
			r.Violations = append(r.Violations, OracleViolation{
				Name:     "704.5a",
				Message:  fmt.Sprintf("seat %d has %d life but is not marked lost", i, s.Life),
				Seat:     i,
				Severity: severity,
				Context:  map[string]interface{}{"life": s.Life},
			})
		}
	}
}

// hasCantLoseEffect checks if any permanent on the battlefield has a
// "would_lose_game" replacement registered for the given seat.
func hasCantLoseEffect(gs *gameengine.GameState, seat int) bool {
	if gs == nil {
		return false
	}
	for _, repl := range gs.Replacements {
		if repl != nil && repl.EventType == "would_lose_game" && repl.ControllerSeat == seat {
			return true
		}
	}
	return false
}

// §704.5f — A creature with toughness 0 or less is put into its
// owner's graveyard. (Check no living creatures with ≤0 toughness.)
func checkToughnessSBA(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			t := gs.ToughnessOf(p)
			if t <= 0 {
				r.Violations = append(r.Violations, OracleViolation{
					Name: "704.5f",
					Message: fmt.Sprintf("creature %q has toughness %d on battlefield",
						p.Card.DisplayName(), t),
					Seat:     i,
					Severity: "critical",
					Context:  map[string]interface{}{"card": p.Card.DisplayName(), "toughness": t},
				})
			}
		}
	}
}

// §704.5c — A player with 10+ poison counters loses the game.
func checkPoisonSBA(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.PoisonCounters >= 10 && !s.Lost {
			r.Violations = append(r.Violations, OracleViolation{
				Name:     "704.5c",
				Message:  fmt.Sprintf("seat %d has %d poison but is not lost", i, s.PoisonCounters),
				Seat:     i,
				Severity: "critical",
				Context:  map[string]interface{}{"poison": s.PoisonCounters},
			})
		}
	}
}

// §704.5v — Commander damage: a player who has been dealt 21+ combat
// damage by a single commander loses the game.
func checkCommanderDamageSBA(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil || s.CommanderDamage == nil {
			continue
		}
		for _, cmdrMap := range s.CommanderDamage {
			for cmdrName, dmg := range cmdrMap {
				if dmg >= 21 && !s.Lost {
					r.Violations = append(r.Violations, OracleViolation{
						Name: "704.5v",
						Message: fmt.Sprintf("seat %d has %d commander damage from %s but is not lost",
							i, dmg, cmdrName),
						Seat:     i,
						Severity: "critical",
						Context:  map[string]interface{}{"commander": cmdrName, "damage": dmg},
					})
				}
			}
		}
	}
}

// judgeConservation is the CONSERVATION-dimension Judge check at the
// post-game (Feynman) hook point: the InstanceID strict census
// (gameengine.ZoneConservationStrict — set-equality by identity,
// disappearance half enabled). The owner-count heuristic that used to
// live here is DELETED (r63 Judge fold): a strict-census run proved
// 499/500 of its warnings were false positives (counts cannot model CR
// §800.4 departures), and the legacy unminted-state fallback went with
// it — unminted means struct-literal fixture, nothing to conserve.
func judgeConservation(gs *gameengine.GameState, r *OracleResult) {
	err, authoritative := gameengine.ZoneConservationStrict(gs)
	if !authoritative || err == nil {
		return
	}
	r.Violations = append(r.Violations, OracleViolation{
		Surface:   judge.SurfaceInvariants,
		Dimension: judge.DimensionConservation,
		Name:      "zone_conservation",
		Message:   err.Error(),
		Seat:      -1,
		Severity:  judge.SeverityCritical,
		Context: map[string]interface{}{
			"check": "instanceid_strict_census",
		},
	})
}

// Exactly one winner: at game end, exactly N-1 seats should be lost.
func checkExactlyOneWinner(gs *gameengine.GameState, r *OracleResult) {
	lost, alive := 0, 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Lost {
			lost++
		} else {
			alive++
		}
	}
	// The "exactly N-1 seats Lost" invariant ONLY applies to a CR §104.2a
	// last-seat-standing win: the game ended (Flags["ended"]==1), a winner was
	// set (Flags["winner"]), and that win was by ELIMINATION — exactly one seat
	// still alive. It does NOT hold for the other legitimate terminal states,
	// which the old unconditional check flagged as ~8,764 false positives across
	// the fishtank:
	//   - turn-cap-leader finish: the showmatch loop hits showmatchMaxTurn and
	//     exits WITHOUT CheckEnd flipping Flags["ended"]; the leader wins on life
	//     but the other seats are alive, not Lost.
	//   - §104.2c "you win the game" effects (Thassa's Oracle / Approach of the
	//     Second Sun / Laboratory Maniac): a winner is set without every opponent
	//     dying, so alive > 1.
	//   - §104.3b/§104.4a simultaneous-death draws: no winner flag; all/none Lost.
	// Real loser-marking bugs (a seat at <=0 life not marked Lost) are caught by
	// checkLifeSBA, so narrowing this check loses no genuine coverage.
	ended := gs.Flags != nil && gs.Flags["ended"] == 1
	_, hasWinner := gs.Flags["winner"]
	if !ended || !hasWinner || alive != 1 {
		return
	}
	// ended + winner + exactly one seat alive => CR §104.2a. N-1 Lost must hold;
	// anything else is a genuine loser-marking bug.
	expected := len(gs.Seats) - 1
	if lost != expected {
		r.Violations = append(r.Violations, OracleViolation{
			Name:     "game_end",
			Message:  fmt.Sprintf("last-seat-standing win but %d of %d seats lost (expected %d)", lost, len(gs.Seats), expected),
			Seat:     -1,
			Severity: "critical",
			Context:  map[string]interface{}{"lost": lost, "total": len(gs.Seats)},
		})
	}
}

// Sanity: games shouldn't run more than ~200 turns. Flag runaways.
func checkTurnBounds(gs *gameengine.GameState, r *OracleResult) {
	if gs.Turn > 200 {
		r.Violations = append(r.Violations, OracleViolation{
			Name:     "turn_bound",
			Message:  fmt.Sprintf("game ran %d turns (possible infinite loop)", gs.Turn),
			Seat:     -1,
			Severity: "warning",
			Context:  map[string]interface{}{"turns": gs.Turn},
		})
	}
}

// No permanent should have negative counter values.
func checkNoNegativeCounters(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Counters == nil {
				continue
			}
			for kind, count := range p.Counters {
				if count < 0 {
					r.Violations = append(r.Violations, OracleViolation{
						Name: "counter_negative",
						Message: fmt.Sprintf("%q has %d %s counters",
							p.Card.DisplayName(), count, kind),
						Seat:     i,
						Severity: "warning",
						Context: map[string]interface{}{
							"card":    p.Card.DisplayName(),
							"counter": kind, "count": count,
						},
					})
				}
			}
		}
	}
}

// §205 — Type-line consistency. Every permanent on the battlefield must
// have at least one permanent type (artifact, creature, enchantment,
// planeswalker, land, battle) and must NOT have a non-permanent type
// (instant, sorcery). Runtime Card.Types tracks type-changing effects
// (Blood Moon adds "mountain", Humility / type-stripping continuous
// effects). We only flag states that are impossible under any effect:
// a card whose runtime types include "instant"/"sorcery", or a card whose
// printed Scryfall type line names no permanent type yet sits on the
// battlefield. Tokens are skipped — they have engine-assigned types and
// no Scryfall record.
func checkPermanentTypes(gs *gameengine.GameState, r *OracleResult) {
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsToken() {
				continue
			}
			for _, t := range p.Card.Types {
				lower := strings.ToLower(t)
				if lower == "instant" || lower == "sorcery" {
					r.Violations = append(r.Violations, OracleViolation{
						Name: "permanent_types",
						Message: fmt.Sprintf("permanent %q has type %q (instants and sorceries cannot be permanents)",
							p.Card.DisplayName(), t),
						Seat:     i,
						Severity: "critical",
						Context: map[string]interface{}{
							"card": p.Card.DisplayName(), "type": t,
						},
					})
				}
			}
			if tl := p.Card.TypeLine; tl != "" && !typeLineHasPermanentType(tl) {
				r.Violations = append(r.Violations, OracleViolation{
					Name: "permanent_types",
					Message: fmt.Sprintf("permanent %q has printed type line %q which has no permanent type",
						p.Card.DisplayName(), tl),
					Seat:     i,
					Severity: "critical",
					Context: map[string]interface{}{
						"card": p.Card.DisplayName(), "type_line": tl,
					},
				})
			}
		}
	}
}

// typeLineHasPermanentType reports whether a Scryfall-style printed type
// line contains at least one permanent type. Only the portion before the
// em dash is examined; subtypes after the dash are subtype-only tokens
// (e.g. "— Bear") and don't determine permanent-ness.
func typeLineHasPermanentType(typeLine string) bool {
	head := strings.ToLower(typeLine)
	if i := strings.Index(head, "—"); i >= 0 {
		head = head[:i]
	} else if i := strings.Index(head, "-"); i >= 0 {
		head = head[:i]
	}
	for _, t := range []string{"artifact", "creature", "enchantment", "planeswalker", "land", "battle"} {
		if strings.Contains(head, t) {
			return true
		}
	}
	return false
}

// FormatViolations returns a human-readable summary of all violations.
func FormatViolations(violations []OracleViolation) string {
	if len(violations) == 0 {
		return "Feynman Oracle: all invariants satisfied"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Feynman Oracle: %d violation(s)\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(&b, "  %s\n", v)
	}
	return b.String()
}
