package hat

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
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
type OracleViolation struct {
	Rule        string // CR section (e.g., "704.5f")
	Description string
	Seat        int
	Severity    string // "critical", "warning", "info"
	Details     map[string]interface{}
}

func (v OracleViolation) String() string {
	return fmt.Sprintf("[%s] %s (seat %d): %s", v.Severity, v.Rule, v.Seat, v.Description)
}

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
		checkZoneAccounting,
		checkExactlyOneWinner,
		checkTurnBounds,
		checkNoNegativeCounters,
		checkPermanentTypes,
	}
	for _, check := range checks {
		check(gs, &result)
		result.Checked++
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
				Rule:        "704.5a",
				Description: fmt.Sprintf("seat %d has %d life but is not marked lost", i, s.Life),
				Seat:        i,
				Severity:    severity,
				Details:     map[string]interface{}{"life": s.Life},
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
					Rule: "704.5f",
					Description: fmt.Sprintf("creature %q has toughness %d on battlefield",
						p.Card.DisplayName(), t),
					Seat:     i,
					Severity: "critical",
					Details:  map[string]interface{}{"card": p.Card.DisplayName(), "toughness": t},
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
				Rule:        "704.5c",
				Description: fmt.Sprintf("seat %d has %d poison but is not lost", i, s.PoisonCounters),
				Seat:        i,
				Severity:    "critical",
				Details:     map[string]interface{}{"poison": s.PoisonCounters},
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
						Rule: "704.5v",
						Description: fmt.Sprintf("seat %d has %d commander damage from %s but is not lost",
							i, dmg, cmdrName),
						Seat:     i,
						Severity: "critical",
						Details:  map[string]interface{}{"commander": cmdrName, "damage": dmg},
					})
				}
			}
		}
	}
}

// Zone accounting: every card should be in exactly one zone.
// Total cards per OWNER should equal starting deck size (99 + commanders).
//
// Owner-reconciled accounting (r61): cards are counted by their owner
// (Card.Owner / Permanent.Owner) across EVERY seat's zones, not by the
// location of the zone they currently sit in. A card owned by seat N but
// physically on another seat's battlefield (stolen via Control Magic,
// Gilded Drake, etc.) still counts toward seat N's tally — eliminating the
// ~1,594 false "card vanishing" warnings where a control-transferred card
// made its owner read "short" and its thief read "long". Permanent.Owner
// survives control change (resolve.go keeps Owner), so this is the correct
// conservation key.
func checkZoneAccounting(gs *gameengine.GameState, r *OracleResult) {
	// r63 — the InstanceID strict census is the single production
	// authority for zone conservation. The owner-count heuristic below
	// false-warned on nearly every game with a CR §800.4 elimination
	// (production flood, -5..-20 "cards-light" on games a strict-census
	// run proved 499/500 CLEAN by identity): counts cannot model the
	// §800.4 flows (eliminated owners' cards ceasing across every zone,
	// survivor-owned cards in the leaver's zones, exile-outside-game),
	// while the census subtracts ceased IDs by construction. The
	// heuristic remains ONLY as a fallback for states with no minted
	// InstanceIDs (legacy fixtures/replays).
	if err, authoritative := gameengine.ZoneConservationStrict(gs); authoritative {
		if err != nil {
			r.Violations = append(r.Violations, OracleViolation{
				Rule:        "zone_conservation",
				Description: err.Error(),
				Seat:        -1,
				Severity:    "critical",
				Details: map[string]interface{}{
					"check": "instanceid_strict_census",
				},
			})
		}
		return
	}
	checkZoneAccountingCountHeuristic(gs, r)
}

// checkZoneAccountingCountHeuristic is the legacy owner-reconciled count
// check, demoted (r63) to fallback-only for game states without
// InstanceID minting. Do not extend — the strict census above is the
// authority; mint-coverage gaps are the thing to fix instead.
func checkZoneAccountingCountHeuristic(gs *gameengine.GameState, r *OracleResult) {
	n := len(gs.Seats)
	if n == 0 {
		return
	}

	owned := make([]int, n)
	tokensBySeat := make([]int, n)

	// bump attributes a card to its owner; out-of-range / zero-value owners
	// fall back to seat 0 to preserve legacy behavior.
	bump := func(c *gameengine.Card) {
		if c == nil {
			return
		}
		o := c.Owner
		if o < 0 || o >= n {
			o = 0
		}
		owned[o]++
	}

	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, c := range s.Hand {
			bump(c)
		}
		for _, c := range s.Library {
			bump(c)
		}
		for _, c := range s.Graveyard {
			bump(c)
		}
		for _, c := range s.Exile {
			bump(c)
		}
		for _, c := range s.CommandZone {
			bump(c)
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			if p.IsToken() {
				o := p.Owner
				if o < 0 || o >= n {
					o = 0
				}
				tokensBySeat[o]++
				continue
			}
			bump(p.Card)
		}
	}

	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		total := owned[i]

		// Expected: 99-card deck + 1-2 commanders = 100-101.
		// Allow some tolerance for edge cases (partner commanders, companion).
		expected := 100
		if len(s.CommanderNames) > 1 {
			expected = 99 + len(s.CommanderNames)
		}

		// §800.4a: when a player leaves the game, objects they own on the
		// battlefield/stack cease to exist. These cards are not in any zone.
		if s.Flags != nil {
			expected -= s.Flags["cards_left_game"]
		}

		diff := total - expected
		// Asymmetric tolerance: negative diffs (missing cards) are real bugs,
		// but positive diffs are almost always copy/clone effects (Clone, Spark
		// Double, Sakashima, Phyrexian Metamorph, etc.) creating non-token card
		// objects that weren't in the original deck. These copies ARE cards (not
		// tokens) per §706, so they inflate the count. A copy-heavy deck can
		// easily produce +15 or more extra cards.
		if diff < -3 || diff > 20 {
			r.Violations = append(r.Violations, OracleViolation{
				Rule: "zone_accounting",
				Description: fmt.Sprintf("seat %d owns %d cards (owner-reconciled; expected ~%d, diff=%d) [tokens owned=%d]",
					i, total, expected, diff, tokensBySeat[i]),
				Seat:     i,
				Severity: "warning",
				Details: map[string]interface{}{
					"owned_cards":  total,
					"owned_tokens": tokensBySeat[i],
					"expected":     expected,
					"diff":         diff,
				},
			})
		}
	}
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
			Rule:        "game_end",
			Description: fmt.Sprintf("last-seat-standing win but %d of %d seats lost (expected %d)", lost, len(gs.Seats), expected),
			Seat:        -1,
			Severity:    "critical",
			Details:     map[string]interface{}{"lost": lost, "total": len(gs.Seats)},
		})
	}
}

// Sanity: games shouldn't run more than ~200 turns. Flag runaways.
func checkTurnBounds(gs *gameengine.GameState, r *OracleResult) {
	if gs.Turn > 200 {
		r.Violations = append(r.Violations, OracleViolation{
			Rule:        "turn_bound",
			Description: fmt.Sprintf("game ran %d turns (possible infinite loop)", gs.Turn),
			Seat:        -1,
			Severity:    "warning",
			Details:     map[string]interface{}{"turns": gs.Turn},
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
						Rule: "counter_negative",
						Description: fmt.Sprintf("%q has %d %s counters",
							p.Card.DisplayName(), count, kind),
						Seat:     i,
						Severity: "warning",
						Details: map[string]interface{}{
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
						Rule: "permanent_types",
						Description: fmt.Sprintf("permanent %q has type %q (instants and sorceries cannot be permanents)",
							p.Card.DisplayName(), t),
						Seat:     i,
						Severity: "critical",
						Details: map[string]interface{}{
							"card": p.Card.DisplayName(), "type": t,
						},
					})
				}
			}
			if tl := p.Card.TypeLine; tl != "" && !typeLineHasPermanentType(tl) {
				r.Violations = append(r.Violations, OracleViolation{
					Rule: "permanent_types",
					Description: fmt.Sprintf("permanent %q has printed type line %q which has no permanent type",
						p.Card.DisplayName(), tl),
					Seat:     i,
					Severity: "critical",
					Details: map[string]interface{}{
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
