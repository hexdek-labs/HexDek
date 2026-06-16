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
		judgeStateIntegrity, // §704.5a/c/6c + §104.2a (judge.CheckStateIntegrity)
		checkToughnessSBA,
		judgeConservation,
		checkTurnBounds,
		checkNoNegativeCounters,
		checkPermanentTypes,
	}
	for _, check := range checks {
		check(gs, &result)
		result.Checked++
	}
	// Route the feynman-local residual checks through the unified sink.
	// Convention (log-at-origin): an empty Surface marks a violation
	// not yet routed; Judge checks (state-integrity, conservation)
	// arrive WITH Surface set and were routed at origin — re-logging
	// them here would double-count.
	for i := range result.Violations {
		if result.Violations[i].Surface == "" {
			result.Violations[i].Surface = judge.SurfaceFeynman
			result.Violations[i].Dimension = judge.DimensionStateIntegrity
			judge.LogViolation(result.Violations[i])
		}
	}
	return result
}

// judgeStateIntegrity is the STATE-INTEGRITY dimension's end-of-game
// hook: builds the neutral GameSnapshot (resolving the engine-specific
// can't-lose shields) and forwards to judge.CheckStateIntegrity. The
// four standalone check bodies that used to live here (§704.5a life,
// §704.5c poison, §704.6c commander damage, §104.2a exactly-one-winner)
// are DELETED — promoted verbatim into internal/judge/stateintegrity.go.
func judgeStateIntegrity(gs *gameengine.GameState, r *OracleResult) {
	snap := judge.GameSnapshot{
		TotalSeats: len(gs.Seats),
		Ended:      gs.Flags != nil && gs.Flags["ended"] == 1,
	}
	if gs.Flags != nil {
		_, snap.HasWinner = gs.Flags["winner"]
	}
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		ss := judge.SeatSnapshot{
			Seat:           i,
			Life:           s.Life,
			Lost:           s.Lost,
			PoisonCounters: s.PoisonCounters,
			CantLoseShield: hasCantLoseEffect(gs, i),
			SBALossEmitted: s.SBA704_5a_emitted,
		}
		if s.CommanderDamage != nil {
			ss.CommanderDamage = map[string]int{}
			for _, cmdrMap := range s.CommanderDamage {
				for name, dmg := range cmdrMap {
					if dmg > ss.CommanderDamage[name] {
						ss.CommanderDamage[name] = dmg
					}
				}
			}
		}
		snap.Seats = append(snap.Seats, ss)
	}
	r.Violations = append(r.Violations, judge.CheckStateIntegrity(snap)...)
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
	// Once the game has ended (§104.2 / §104.3a), StateBasedActions
	// returns early without running any §704.5 checks (see sba.go:43). A
	// creature whose toughness reached ≤ 0 in the same window the game
	// resolved — e.g. an "until end of turn" −X/−X falling off, an anthem
	// source leaving, or a counter-fueled 0/0 (Fertilid) losing its last
	// counter as the final blow landed — legitimately remains on the
	// battlefield in the ended snapshot: the final SBA pass that would have
	// binned it never runs. This is the SAME post-ended carve-out the
	// in-game twin gameengine.checkSBACompleteness already documents and
	// applies; the post-game Feynman oracle samples exactly that ended
	// snapshot, so it must skip the toughness SBA too or it flags states
	// the engine is correct to leave alone (the recurring live-grinder
	// surface=feynman 704.5f false positive). When the game did NOT end
	// (e.g. a max-turns cap with ended unset) the check still runs, so a
	// genuine in-game §704.5f miss stays visible.
	if gs.Flags != nil && gs.Flags["ended"] == 1 {
		return
	}
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		for _, p := range s.Battlefield {
			// Layer-aware creature predicate — MUST match sba704_5f's
			// gs.IsCreatureOf, not the base printed-type p.IsCreature.
			// A continuous effect that strips the creature type (Song of
			// the Dryads → Forest land, Imprisoned in the Moon, etc.)
			// leaves a printed-creature reading toughness 0 (a land has
			// no P/T); §704.5f does not apply and the SBA correctly skips
			// it. Reading the printed type here produced a post-game FP
			// (the live-fishtank r63 finding). See feynman_704_5f_typestrip_r63_test.
			if p == nil || !gs.IsCreatureOf(p) {
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
