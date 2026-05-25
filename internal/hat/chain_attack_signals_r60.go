package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// chain_attack_signals_r60.go — R60 round 12+ ChooseAttackers chain-
// attack awareness signals. Audit:
//
//   1. The hat doesn't read gs.PendingExtraCombats anywhere. The engine
//      already tracks queued extra combats (Aurelia, Najeela, Aggravated
//      Assault, Moraug, Seize the Day) in a FIFO at state.go:174, but
//      ChooseAttackers ignores it. Result: each combat is scored as if
//      it's the last — no commitment escalation when more swings are
//      queued.
//
//   2. postcombatTriggerBonus (PR #357) gives +0.30 to the Aurelia /
//      Najeela / Scourge attacker themselves, but OTHER attackers in
//      the same combat phase don't know an extra combat is about to
//      fire. Their commit value should also escalate.
//
// Two signals address both gaps without overcomplicating the per-
// attacker scoring loop. Both sized small (+0.05 per increment, capped
// at +0.15) so chip-damage commitment compounds appropriately across
// the chain without overriding the profitability prune downstream.

// chainAttackPendingBonus returns a small per-attacker bonus based on
// how many extra combats are already queued behind the current one.
// Bonus = 0.05 × queue depth, capped at 0.15.
//
// Why per-attacker (not per-call): the bonus encourages committing
// MORE attackers this combat because every one of them generates chip
// damage / triggers / death-payoffs that compound across the queue.
// Reserving a creature for "the next combat" is the wrong frame —
// once you're in a chain, the goal is to fire as much as possible
// in each swing.
func chainAttackPendingBonus(gs *gameengine.GameState) float64 {
	if gs == nil {
		return 0
	}
	n := len(gs.PendingExtraCombats)
	if n <= 0 {
		return 0
	}
	bonus := 0.05 * float64(n)
	if bonus > 0.15 {
		bonus = 0.15
	}
	return bonus
}

// chainAttackAnticipationBonus returns +0.05 when our legal-attacker
// pool contains a creature with "additional combat" trigger text. The
// trigger hasn't fired yet — but we're ABOUT to attack with the
// trigger source, so the chain will start this combat. Other attackers
// in the same combat get the same commitment escalation as if the
// pending queue already had an entry.
//
// Conservative on detection: scans only the legal pool we're given,
// not the full battlefield (a sleeping Aurelia with summoning sickness
// or a tapped Aurelia from a previous combat won't fire). The trigger
// source itself ALSO receives this bonus, which is intentional —
// double-counts marginally with postcombatTriggerBonus's +0.30, but
// the extra +0.05 reflects "swinging the trigger-bearer is even more
// valuable because the chain it kicks off lifts every other attacker."
//
// Returns 0 when no trigger source is in the legal pool.
func chainAttackAnticipationBonus(legal []*gameengine.Permanent) float64 {
	for _, p := range legal {
		if p == nil || p.Card == nil {
			continue
		}
		ot := gameengine.OracleTextLower(p.Card)
		if strings.Contains(ot, "additional combat phase") ||
			strings.Contains(ot, "additional combat") {
			return 0.05
		}
	}
	return 0
}
