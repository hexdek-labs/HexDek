package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// LifeResource r60-v2 regressions covering three signals layered on
// the existing dimension (convex danger curve, life-payoff dampener,
// lifegain engines, painland tax, opponent pressure):
//
//   - Signal A: broader life-as-resource detection. The pre-r60
//     matcher required "pay" + "life" + (draw|cast|search), missing
//     K'rrik ("cost X life more"), Bolas's Citadel ("rather than pay
//     its mana cost"), Yawgmoth ("pay 2 life: put -1/-1 counter"),
//     and Ad Nauseam ("lose life equal to" + reveal).
//   - Signal B: effective-clock awareness via commander damage. Pre-
//     r60 a seat at life=12 with 14 commander damage from one source
//     scored identically to life=12 with 0 commander damage; the
//     practical clock was masked.
//   - Signal C: inevitability amplifier. Low life vs an over-board
//     opponent is much more dangerous than low life vs a stalled
//     table. Pre-r60 the dimension ignored opponent board power.

// ---------------------------------------------------------------------
// Signal A — broader life-as-resource detection
// ---------------------------------------------------------------------

// lifeCardWithOracle builds a Card with the given oracle text snippet
// attached as a Static ability.
func lifeCardWithOracle(name string, types []string, cmc int, oracleSnippet string) *gameengine.Card {
	return gyCardWithOracle(name, types, cmc, oracleSnippet)
}

func TestIsLifeAsResourcePayoff_KnownShapes(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
		want   bool
	}{
		{"Necropotence", "skip your draw step. pay 1 life: exile the top card of your library face down.", true},
		{"Bolas's Citadel", "you may play lands and cast spells from the top of your library. if you cast a spell this way, pay life equal to its mana value rather than pay its mana cost.", true},
		{"K'rrik", "black spells you cast cost 2 life more to cast.", true},
		{"Greed", "{b}, pay 2 life: draw a card.", true},
		{"Yawgmoth Thran Physician", "{b}, pay 2 life, discard a card: put a -1/-1 counter on target non-human creature.", true},
		{"Sylvan Library", "at the beginning of your draw step, you may draw two additional cards. if you do, pay 4 life or put the card on top of your library.", true},
		{"Ad Nauseam", "reveal the top card of your library and put that card into your hand. you lose life equal to its mana value.", true},
		{"Dark Confidant", "at the beginning of your upkeep, reveal the top card of your library and put that card into your hand. you lose life equal to its mana value.", true},
		{"Plain creature", "flying, vigilance.", false},
		{"Pay life for OPPONENT loss only", "pay 2 life: target player loses 2 life.", false}, // self-drain attack, not engine
		{"Lifegain card", "whenever you gain life, draw a card.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLifeAsResourcePayoff(tc.oracle)
			if got != tc.want {
				t.Errorf("%s: isLifeAsResourcePayoff = %v want %v\n  oracle: %q", tc.name, got, tc.want, tc.oracle)
			}
		})
	}
}

// TestScoreLifeR60v2_PayoffDampensLowLifeDanger_NewShapes: a seat at
// life=6 with K'rrik on board should score noticeably better than
// the same seat without K'rrik (the engine signals intentional
// life-spending, dampening the shock-band penalty).
func TestScoreLifeR60v2_PayoffDampensLowLifeDanger_NewShapes(t *testing.T) {
	build := func(includePayoff bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 6
		if includePayoff {
			krrik := lifeCardWithOracle("K'rrik", []string{"creature", "legendary"}, 5,
				"black spells you cast cost 2 life more to cast.")
			newTestPermanent(gs.Seats[0], krrik, 4, 4)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	withPayoff := ev.scoreLife(build(true), 0)
	noPayoff := ev.scoreLife(build(false), 0)

	if withPayoff <= noPayoff {
		t.Errorf("K'rrik should dampen low-life penalty; with=%.4f no=%.4f", withPayoff, noPayoff)
	}
}

// ---------------------------------------------------------------------
// Signal B — effective-clock awareness via commander damage
// ---------------------------------------------------------------------

// TestScoreLifeR60v2_CommanderDamageNarrowsClock: a seat at life=20
// taking 18 commander damage from one source has effective clock = 3
// (lethal in one swing from that commander). Dimension should score
// MUCH worse than the same life=20 with 0 commander damage.
func TestScoreLifeR60v2_CommanderDamageNarrowsClock(t *testing.T) {
	build := func(cmdrDmg int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 20
		gs.Seats[0].CommanderNames = []string{"Me"}
		gs.Seats[1].CommanderNames = []string{"Opp Cmdr"}
		if cmdrDmg > 0 {
			gs.Seats[0].CommanderDamage = map[int]map[string]int{
				1: {"Opp Cmdr": cmdrDmg},
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	noCmdrDmg := ev.scoreLife(build(0), 0)
	heavyCmdrDmg := ev.scoreLife(build(18), 0) // clock = 21-18 = 3 (shock band)

	if heavyCmdrDmg >= noCmdrDmg {
		t.Errorf("18 commander damage should narrow clock and worsen score; heavy=%.4f none=%.4f",
			heavyCmdrDmg, noCmdrDmg)
	}
	// effectiveLife=3 enters the shock band; raw life=20 was in the calm
	// mid-band. The delta should be substantial.
	delta := noCmdrDmg - heavyCmdrDmg
	if delta < 0.30 {
		t.Errorf("commander-clock narrowing delta too small: %.4f (want ≥0.30)", delta)
	}
}

// TestScoreLifeR60v2_CommanderDamageBelowThresholdDoesNotNarrow: 5
// commander damage from a source is below the >=14 threshold —
// dimension reads raw seat.Life only.
func TestScoreLifeR60v2_CommanderDamageBelowThresholdDoesNotNarrow(t *testing.T) {
	build := func(cmdrDmg int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 20
		gs.Seats[0].CommanderNames = []string{"Me"}
		gs.Seats[1].CommanderNames = []string{"Opp Cmdr"}
		if cmdrDmg > 0 {
			gs.Seats[0].CommanderDamage = map[int]map[string]int{
				1: {"Opp Cmdr": cmdrDmg},
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	noCmdrDmg := ev.scoreLife(build(0), 0)
	mildCmdrDmg := ev.scoreLife(build(5), 0) // below the >=14 threshold

	if noCmdrDmg != mildCmdrDmg {
		t.Errorf("5 commander damage should be below clock-narrowing threshold; got %.4f vs %.4f",
			mildCmdrDmg, noCmdrDmg)
	}
}

// TestScoreLifeR60v2_CommanderDamageOnlyInCommanderFormat: non-EDH
// games ignore commander damage entirely; the dimension reads raw
// life regardless.
func TestScoreLifeR60v2_CommanderDamageOnlyInCommanderFormat(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.CommanderFormat = false
	gs.Seats[0].Life = 20
	gs.Seats[0].StartingLife = 20
	gs.Seats[1].Life = 20
	gs.Seats[1].StartingLife = 20
	gs.Seats[0].CommanderDamage = map[int]map[string]int{
		1: {"Phantom Cmdr": 18},
	}

	ev := NewEvaluator(nil)
	got := ev.scoreLife(gs, 0)

	// Counterfactual: same state, no commander damage.
	gsNo := newTestGame(t, 2)
	gsNo.CommanderFormat = false
	gsNo.Seats[0].Life = 20
	gsNo.Seats[0].StartingLife = 20
	gsNo.Seats[1].Life = 20
	gsNo.Seats[1].StartingLife = 20
	no := ev.scoreLife(gsNo, 0)

	if got != no {
		t.Errorf("non-EDH should ignore commander damage; got=%.4f no=%.4f", got, no)
	}
}

// TestScoreLifeR60v2_CommanderDamage_MaxAcrossSources: max-per-source
// (not sum across sources) drives the clock. 12 from opp 1 + 12 from
// opp 2 = sum 24 but max 12 — below the >=14 threshold, no narrowing.
// 14 from opp 1 alone DOES narrow.
func TestScoreLifeR60v2_CommanderDamage_MaxAcrossSources(t *testing.T) {
	mk := func(damages map[int]int) *gameengine.GameState {
		gs := newTestGame(t, 4)
		gs.CommanderFormat = true
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 20
		gs.Seats[0].CommanderNames = []string{"Me"}
		gs.Seats[0].CommanderDamage = map[int]map[string]int{}
		for oppIdx, dmg := range damages {
			gs.Seats[oppIdx].CommanderNames = []string{"Opp" + itoa(oppIdx)}
			gs.Seats[0].CommanderDamage[oppIdx] = map[string]int{"Opp" + itoa(oppIdx): dmg}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	splitSum := ev.scoreLife(mk(map[int]int{1: 12, 2: 12}), 0)         // max 12, no fire
	singleHeavy := ev.scoreLife(mk(map[int]int{1: 14}), 0)              // max 14, fires
	splitNoFire := ev.scoreLife(mk(map[int]int{1: 13, 2: 13, 3: 13}), 0) // max 13, no fire

	if singleHeavy >= splitSum {
		t.Errorf("single 14-source should worsen score over 12+12 split; single=%.4f split=%.4f",
			singleHeavy, splitSum)
	}
	if splitSum != splitNoFire {
		// Both have max < 14 so neither narrows the clock — scores should be
		// identical on the commander-clock axis. (Other dimensions don't
		// fire here.)
		t.Errorf("12+12 vs 13+13+13 should both fail the >=14 gate; 12+12=%.4f 13+13+13=%.4f",
			splitSum, splitNoFire)
	}
}

// ---------------------------------------------------------------------
// Signal C — inevitability amplifier
// ---------------------------------------------------------------------

// TestScoreLifeR60v2_Inevitability_LowLifeVsBigBoard: at life=4, an
// opponent with a 20-power board should be MUCH worse than the same
// life=4 vs a 2-power opponent.
func TestScoreLifeR60v2_Inevitability_LowLifeVsBigBoard(t *testing.T) {
	build := func(oppPow int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 4
		if oppPow > 0 {
			newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Big", []string{"creature"}, 4, nil), oppPow, oppPow)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	vsSmall := ev.scoreLife(build(2), 0)
	vsBig := ev.scoreLife(build(20), 0)

	if vsBig >= vsSmall {
		t.Errorf("low life vs 20-power opp should be MORE negative than vs 2-power; big=%.4f small=%.4f",
			vsBig, vsSmall)
	}
}

// TestScoreLifeR60v2_Inevitability_InertOutsideShockBand: at life=25
// (calm band), opp board size shouldn't trigger the inevitability
// amplifier — it's gated on effectiveLife <= 10.
func TestScoreLifeR60v2_Inevitability_InertOutsideShockBand(t *testing.T) {
	build := func(oppPow int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 25
		if oppPow > 0 {
			newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Big", []string{"creature"}, 4, nil), oppPow, oppPow)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	vsSmall := ev.scoreLife(build(2), 0)
	vsBig := ev.scoreLife(build(20), 0)

	// At life=25 the inevitability gate doesn't fire. Other dimensions
	// might cause minor drift (e.g., pressure changes if opp life changes)
	// but here opp life is 40 in both cases. Allow a small epsilon.
	delta := vsSmall - vsBig
	if delta > 0.05 {
		t.Errorf("inevitability fired outside shock band: vsSmall=%.4f vsBig=%.4f delta=%.4f",
			vsSmall, vsBig, delta)
	}
}

// TestScoreLifeR60v2_Inevitability_SuppressedByLifePayoff: low life
// with a life-payoff engine on board (Bolas's Citadel) is intentional;
// the inevitability amplifier should NOT fire because hasLifePayoff
// signals "we WANT to be here."
func TestScoreLifeR60v2_Inevitability_SuppressedByLifePayoff(t *testing.T) {
	build := func(includePayoff bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		for i := range gs.Seats {
			gs.Seats[i].Life = 40
			gs.Seats[i].StartingLife = 40
		}
		gs.Seats[0].Life = 4
		newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Big", []string{"creature"}, 4, nil), 20, 20)
		if includePayoff {
			citadel := lifeCardWithOracle("Bolas's Citadel", []string{"artifact", "legendary"}, 6,
				"if you cast a spell this way, pay life equal to its mana value rather than pay its mana cost.")
			newTestPermanent(gs.Seats[0], citadel, 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	withPayoff := ev.scoreLife(build(true), 0)
	noPayoff := ev.scoreLife(build(false), 0)

	// With payoff, the inevitability arm is suppressed AND the existing
	// hasLifePayoff dampener halves the base shock penalty. Both push
	// withPayoff > noPayoff (less negative).
	if withPayoff <= noPayoff {
		t.Errorf("life-payoff engine should suppress inevitability + dampen shock penalty; with=%.4f no=%.4f",
			withPayoff, noPayoff)
	}
}
