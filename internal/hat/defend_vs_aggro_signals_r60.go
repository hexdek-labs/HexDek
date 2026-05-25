package hat

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// defend_vs_aggro_signals_r60.go — R60 round 7+ defensive-pressure signals
// for cardHeuristic. The pre-fix heuristic shapes plays by phase, archetype,
// curse DNA, and commander themes — but does NOT react to opponent board
// pressure. The hat will happily slam a 7-mana value engine on T6 even
// while an aggro opponent has 12 power on the board threatening lethal in
// 2 turns. Both signals here gate on the same defensive-urgency
// computation so they fire together:
//
//   Signal A — defensive-card bonus under pressure (+0.20 × urgency)
//     when the max projected single-turn opponent damage exceeds 33% of
//     current life, creature removal / sweepers / blockers with toughness
//     ≥ 3 or evergreen defensive keywords (reach, flying, deathtouch,
//     defender, lifelink) bubble up in the cast pool.
//
//   Signal B — expensive non-defensive penalty under pressure
//     (−0.15 × urgency) — CMC ≥ 5 cards that AREN'T defensive get
//     penalized. The hat needs to be willing to skip its "fatty turn" to
//     answer the threat instead of racing into lethal.
//
// Both signals only fire when pressure crosses the 33%-life threshold;
// the baseline curve play and archetype identity are untouched at lower
// pressure. The urgency scaling means even at the threshold the bonus is
// small (~0.066), growing to 0.20 at lethal-this-turn.

// defenseUrgencyVsAggro returns a pressure score in [0, 1] indicating
// how urgently the hat needs to defend. 0 = no untapped opp board, 1 =
// projected lethal next turn. Computed as the maximum (per-opponent)
// of (projected single-turn damage / current life), capped at 1.0.
//
// Projection per attacker: power × 2 for double-strike, × 1 otherwise.
// Tapped creatures count at full power (they untap next turn). Power < 0
// creatures (e.g. -1/-1-counter casualties before SBA) are ignored — they
// don't project damage. The computation deliberately ignores blockers
// the hat already controls — Signal A wants to BOOST removal even when
// we have blockers, because trading a blocker into the threat costs us
// a card too. The blocker math is the AssignBlockers loop's job.
func (h *YggdrasilHat) defenseUrgencyVsAggro(gs *gameengine.GameState, seatIdx int) float64 {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	me := gs.Seats[seatIdx]
	if me == nil || me.Life <= 0 {
		return 0
	}
	worst := 0
	for i, opp := range gs.Seats {
		if i == seatIdx || opp == nil || opp.Lost || opp.LeftGame {
			continue
		}
		oppDamage := 0
		for _, p := range opp.Battlefield {
			if p == nil || !p.IsCreature() || p.PhasedOut {
				continue
			}
			pow := gs.PowerOf(p)
			if pow <= 0 {
				continue
			}
			mul := 1
			if p.HasKeyword("double strike") || p.HasKeyword("double_strike") {
				mul = 2
			}
			oppDamage += pow * mul
		}
		if oppDamage > worst {
			worst = oppDamage
		}
	}
	if worst == 0 {
		return 0
	}
	urgency := float64(worst) / float64(me.Life)
	if urgency > 1.0 {
		urgency = 1.0
	}
	return urgency
}

// defensiveKeywords are the evergreens that make a creature good at
// stopping aggro damage, regardless of P/T:
//   - reach + flying: block flyers
//   - deathtouch: trade up
//   - defender: pure wall, untargetable for attacks but blocks
//   - lifelink: blocking races life back
var defensiveKeywords = []string{
	"reach", "flying", "deathtouch", "defender", "lifelink",
}

// isDefensiveCard returns true for cards that materially help defend
// against incoming aggro damage:
//   - creature removal (CatRemoval — covers Doom Blade, Lightning Bolt,
//     Path to Exile, Wrath of God, etc.; categorizeWithFreya routes both
//     spot removal and sweepers here)
//   - creatures with toughness ≥ 3 (solid blocker for most aggro hits)
//   - creatures with any keyword in defensiveKeywords
//
// Conservative on misses: an instant-speed life-gain spell isn't flagged
// (no removal effect, no creature toughness); the bonus is meant to bias
// the cast pool toward cards that physically stop the damage, not just
// race the life total.
func (h *YggdrasilHat) isDefensiveCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if h.categorizeWithFreya(c) == CatRemoval {
		return true
	}
	if !typeLineContains(c, "creature") {
		return false
	}
	// Toughness threshold for "real blocker". BaseToughness is the raw
	// printed value before mods; sufficient for cast-pool heuristics
	// (we don't know what counters / equipment will land on it post-ETB).
	if c.BaseToughness >= 3 {
		return true
	}
	ot := gameengine.OracleTextLower(c)
	for _, kw := range defensiveKeywords {
		if strings.Contains(ot, kw) {
			return true
		}
	}
	return false
}
