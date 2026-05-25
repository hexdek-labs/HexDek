package hat

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_into_wipe_signals_r60.go — R60 round 11+ ChooseAttackers
// counter-balance to the existing wrath-aware hold-back at line 5191.
//
// Existing behavior: when wrath is suspected and the attacker is a value
// engine, val -= 0.15 to preserve the strategic asset for post-wipe
// rebuild. That correctly disincentivizes sending Sword of Feast and
// Famine into a Wrath of God.
//
// Audit: nothing currently INCENTIVIZES the inverse play — deliberately
// attacking into a likely wipe with EXPENDABLE creatures. Two scenarios
// where attacking into a wipe is correct:
//
//   1. Wipe-bait expendable. The wipe takes the creature anyway. Chip
//      damage on the way to the graveyard is free value. A 2/2 vanilla
//      that swings for 2 then dies to Wrath nets +2 damage vs holding
//      and losing the creature anyway.
//
//   2. Use-it-or-lose-it trigger. An Edric / Toski / Hellrider that
//      gets ITS TRIGGER VALUE before the wipe lands has captured the
//      whole payoff — the trigger payoff is permanent (cards drawn,
//      damage dealt) but the body is dying regardless. Held back,
//      the trigger never fires.
//
// Both signals gated on wrathSuspected and a non-desperate position
// (relPos > -0.2, mirroring the existing wrath check). Strategic
// creatures (commanders, finishers, value engines, combo pieces) are
// excluded — they should still hold per the existing logic. Sized
// small (+0.10 each, max +0.20 stack) so they nudge marginal attackers
// without overriding the profitability prune.

// wipeBaitExpendableBonus returns +0.10 when the hat should treat the
// attacker as expendable bait under wrath pressure. Returns 0 when:
//   - wrath isn't suspected (no pressure)
//   - the hat is desperate (relPos ≤ -0.2 — already losing, just
//     swing everything per existing logic)
//   - the card is strategically valuable (commander, value engine,
//     combo piece, finisher) — those should hold back, mirroring the
//     existing -0.15 on value engines
//
// Signal A — pairs symmetrically with the existing line 5191 penalty:
// VE under wrath → -0.15 (hold back the engine); non-VE under wrath →
// +0.10 (push the chaff in for chip damage).
func (h *YggdrasilHat) wipeBaitExpendableBonus(gs *gameengine.GameState, seatIdx int, p *gameengine.Permanent, wrathSuspected bool, relPos float64) float64 {
	if !wrathSuspected || relPos <= -0.2 {
		return 0
	}
	if p == nil || p.Card == nil {
		return 0
	}
	// Strategic-asset exclusions. Same set the rest of the wrath /
	// blocker-pool / discard guards use as "don't expose this".
	if isCommanderCard(gs, seatIdx, p.Card) {
		return 0
	}
	if h.isValueEngineKey(p.Card) || h.isComboRelevant(p.Card) || h.isFinisher(p.Card) {
		return 0
	}
	return 0.10
}

// useItOrLoseItTriggerBonus returns +0.10 when the attacker has an
// attack-declaration or postcombat trigger AND a wipe is likely. The
// trigger payoff (Toski draw, Hellrider damage, Cosima card) sticks
// even if the body dies to the wipe, so deferring the swing surrenders
// the whole payoff. Stacks on top of attackTriggerBonus +
// postcombatTriggerBonus to push triggered creatures over the swing
// threshold one more step under wrath pressure.
//
// Same desperation gate as wipeBaitExpendableBonus: doesn't fire when
// relPos ≤ -0.2 (the all-in / desperation arm already swings
// everything).
//
// Signal B — does NOT exclude strategic creatures by intent. An Edric
// or Hellrider that's also a value-engine SHOULD swing into the wipe:
// the existing line 5191 -0.15 penalty plus the +0.20 trigger bonus
// plus this +0.10 nets to +0.15 — still positive. Effectively the
// strategic-asset penalty is partially refunded for the
// trigger-bearing case, which matches Magic's "value-engine creatures
// with attack triggers want to keep attacking under wrath threat"
// intuition.
func useItOrLoseItTriggerBonus(p *gameengine.Permanent, wrathSuspected bool, relPos float64) float64 {
	if !wrathSuspected || relPos <= -0.2 {
		return 0
	}
	if p == nil || p.Card == nil {
		return 0
	}
	if attackTriggerBonus(p.Card) > 0 || postcombatTriggerBonus(p.Card) > 0 {
		return 0.10
	}
	return 0
}
