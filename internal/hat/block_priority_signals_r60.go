package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// block_priority_signals_r60.go — R60 round 8+ AssignBlockers combat-trick
// awareness signals. The pre-fix AssignBlockers survivor math only reads
// raw P/T + evergreen keywords; it doesn't anticipate combat tricks on
// either side of the table.
//
// Two complementary signals shape the survivor pool:
//
//   Signal A — defensive-trick rescue (true → otherwise-dead blockers
//     are added to the survivor pool). When the hat holds an affordable
//     instant that rescues blockers (regenerate, until-EOT
//     indestructibility, hexproof grant, +N/+N toughness pump), the
//     AssignBlockers loop trusts it'll cast the trick during the
//     declare-blockers / damage step, so a blocker that would die in
//     raw combat math becomes a survivor candidate. Skipped against
//     deathtouch attackers — toughness pumps don't escape DT, and
//     regen-vs-DT is a narrower interaction than the heuristic warrants.
//
//   Signal B — opp combat-trick mana buffer (true → tighten survivor
//     threshold by +1 toughness). When any opp has ≥ 2 untapped mana
//     AND we're past the early game, marginal survivors (those that
//     live by exactly 1 toughness) become unreliable — opp can pump or
//     regen-strip with a +1/-1 trick. Require a 2-toughness buffer so
//     the survivor pool only contains blockers that survive even a
//     small opp combat trick.
//
// Both signals only fire when their evidence is present. With neither
// signal active, the survivor math is unchanged from the pre-R60 r8
// shape, so existing AssignBlockers regressions hold.

// defensiveTrickNeedles are oracle-text substrings that signal an
// instant can rescue a blocker. Conservative set — false negatives
// (missed tricks) just lose the bonus; false positives would commit
// unrescuable blockers and bleed cards.
var defensiveTrickNeedles = []string{
	"regenerate",         // classic regen — Reanimate effect, blocker stays
	"gains indestructible", // Heroic Intervention / Boros Charm shape
	"gain indestructible",  // alternative phrasing
	"gains hexproof",       // post-block protection from removal
	"gain hexproof",        // alternative phrasing
	"gains protection",     // Apostle's Blessing-style protection grant
	"gain protection",
	"gets +0/+",  // pure-toughness pump
	"+0/+",       // bare-form +0/+N pump
	"prevent all combat damage", // Holy Day / Ethereal Haze / Settle
	"prevent the next",          // Prevention shield
}

// hasAffordableDefensiveTrick reports whether the hat holds a defensive
// instant (oracle-text-matched against defensiveTrickNeedles) that the
// hat's seat can pay for from current untapped mana + pool. A heuristic:
// we don't fully simulate cost+target resolution. Conservative on the
// affordability check via Total() — sufficient since defensive tricks
// are almost universally 1-2 mana.
func (h *YggdrasilHat) hasAffordableDefensiveTrick(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	est := gameengine.AvailableColoredManaEstimate(gs, seat)
	avail := est.Total()
	if avail == 0 {
		return false
	}
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		// Defensive tricks are instants; sorcery-speed pumps can't react
		// to a block being declared.
		if !typeLineContains(c, "instant") {
			continue
		}
		if gameengine.ManaCostOf(c) > avail {
			continue
		}
		ot := gameengine.OracleTextLower(c)
		for _, needle := range defensiveTrickNeedles {
			if strings.Contains(ot, needle) {
				return true
			}
		}
	}
	return false
}

// oppHasCombatTrickMana reports whether ANY non-self opponent has ≥ 2
// untapped mana available AND the game is past turn 3 (so the mana is
// realistic combat-trick fuel, not a T2 ramp deck's open Sol Ring).
// The hat uses this to tighten the survivor buffer: a blocker that
// survives by exactly 1 toughness is unreliable when opp can spend 2
// to pump the attacker or hand it deathtouch.
func (h *YggdrasilHat) oppHasCombatTrickMana(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || gs.Turn < 4 {
		return false
	}
	for i, opp := range gs.Seats {
		if i == seatIdx || opp == nil || opp.Lost || opp.LeftGame {
			continue
		}
		est := gameengine.AvailableColoredManaEstimate(gs, opp)
		if est.Total() >= 2 {
			return true
		}
	}
	return false
}
