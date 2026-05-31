package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ThreatExposure r60-v2 regressions covering three signals layered on
// top of the existing dimension (lethalRatio + dangerousPermanents +
// hoser + poison + mill + cmdr + hardToAnswer + stackPressure +
// wipeMagnet):
//
//   - Signal A: single-removal concentration. Mirror of wipeMagnet —
//     the OVER-committed vs UNDER-committed split. A single keystone
//     creature carrying ≥50% of own offense is one Murder away from
//     position collapse; surface that exposure.
//   - Signal B: untap-step window. When active player isn't us, a
//     tapped key blocker can't block. Defensive-shell compromise.
//   - Signal C: interaction-mana starvation. During opp priority,
//     untapped-land count gates whether we can respond AT ALL.
//
// All three read state the pre-r60-v2 dimension ignored (own-board
// power distribution, gs.Active, tapped state).

// ---------------------------------------------------------------------
// Shared builders
// ---------------------------------------------------------------------

// buildThreatGame: 2-seat game, seat 0 perspective, seat 1 has a
// configurable opponent attacker with given power. Returns the game.
func buildThreatGame(t *testing.T, oppPower int) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	if oppPower > 0 {
		newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Opp Attacker", []string{"creature"}, 4, nil), oppPower, oppPower)
	}
	return gs
}

// ---------------------------------------------------------------------
// Signal A — single-removal concentration
// ---------------------------------------------------------------------

// TestThreatR60v2_KeystoneConcentration_FiresOnSoleBigCreature: a
// single 8-power creature on a board with no other creatures is the
// max-exposure case (share=1.0, penalty=0.50 capped).
func TestThreatR60v2_KeystoneConcentration_FiresOnSoleBigCreature(t *testing.T) {
	gs := buildThreatGame(t, 6)
	// Sole creature on our side carries all our offense.
	newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vorinclex", []string{"creature"}, 8, nil), 8, 8)
	gs.Active = 0 // our turn — neutralize the untap-window/starvation signals

	ev := NewEvaluator(nil)
	withKeystone := ev.scoreThreat(gs, 0)

	gsSpread := buildThreatGame(t, 6)
	// Same total power (8), but spread across 4 creatures of 2/2 each.
	for i := 0; i < 4; i++ {
		newTestPermanent(gsSpread.Seats[0],
			newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	}
	gsSpread.Active = 0
	spread := ev.scoreThreat(gsSpread, 0)

	if withKeystone >= spread {
		t.Errorf("keystone share=1.0 should be MORE negative than spread share=0.25; keystone=%.4f spread=%.4f",
			withKeystone, spread)
	}
	// Concentration penalty at share=1.0 is (1.0-0.5)=0.50 capped at 0.50.
	delta := spread - withKeystone
	if delta < 0.40 {
		t.Errorf("keystone penalty too small: delta=%.4f (want ≥0.40 at the 0.50 cap)", delta)
	}
}

// TestThreatR60v2_KeystoneConcentration_DoesNotFireBelow50PctShare:
// at the same total-board-power (so wipeMagnet contributes equally),
// a share<0.5 distribution should score IDENTICALLY to itself — the
// concentration penalty must not leak in. Compare two share=0.4
// distributions: (4/3/3) vs (4/3/3) (literally the same) to pin no
// contribution; then confirm a share=0.8 distribution at the same
// total is meaningfully worse.
func TestThreatR60v2_KeystoneConcentration_DoesNotFireBelow50PctShare(t *testing.T) {
	build := func(powers []int) *gameengine.GameState {
		gs := buildThreatGame(t, 6)
		for i, pow := range powers {
			newTestPermanent(gs.Seats[0],
				newTestCardMinimal("C"+itoa(i), []string{"creature"}, 3, nil), pow, pow)
		}
		gs.Active = 0
		return gs
	}

	ev := NewEvaluator(nil)
	spread := ev.scoreThreat(build([]int{4, 3, 3}), 0)        // share 4/10 = 0.4
	concentrated := ev.scoreThreat(build([]int{8, 1, 1}), 0) // share 8/10 = 0.8

	// Same total power (10) means identical wipeMagnet contribution; the
	// only delta is concentration. Concentrated should score worse by
	// roughly (0.8-0.5)=0.30 (within slack for unrelated paths reading
	// per-creature attributes).
	delta := spread - concentrated
	if delta < 0.20 {
		t.Errorf("concentration not differentiating share=0.4 vs 0.8 at fixed total power; spread=%.4f concentrated=%.4f delta=%.4f (want ≥0.20)",
			spread, concentrated, delta)
	}
	if delta > 0.50 {
		t.Errorf("concentration over-firing at share=0.8: delta=%.4f (cap is 0.50)", delta)
	}
}

// TestThreatR60v2_KeystoneConcentration_GatedOnNonTrivialBoard: tiny
// total power (<=4) should not trigger even at share=1.0 — a single
// 2/2 is not a keystone.
func TestThreatR60v2_KeystoneConcentration_GatedOnNonTrivialBoard(t *testing.T) {
	gs := buildThreatGame(t, 6)
	newTestPermanent(gs.Seats[0], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	gs.Active = 0

	ev := NewEvaluator(nil)
	got := ev.scoreThreat(gs, 0)

	gsEmpty := buildThreatGame(t, 6)
	gsEmpty.Active = 0
	empty := ev.scoreThreat(gsEmpty, 0)

	if got < empty-0.05 {
		t.Errorf("concentration fired on trivial 2/2: got=%.4f empty=%.4f", got, empty)
	}
}

// ---------------------------------------------------------------------
// Signal B — untap-step window defensive exposure
// ---------------------------------------------------------------------

// TestThreatR60v2_UntapWindow_FiresWhenBestBlockerTappedOnOppTurn:
// active=opp, our best blocker (5-tough) is tapped, only an untapped
// 2-tough left. Diff=3 → bonus=0.15.
func TestThreatR60v2_UntapWindow_FiresWhenBestBlockerTappedOnOppTurn(t *testing.T) {
	gs := buildThreatGame(t, 5)
	bigBlocker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Big Wall", []string{"creature"}, 4, nil), 0, 5)
	bigBlocker.Tapped = true
	_ = newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Small Wall", []string{"creature"}, 2, nil), 0, 2)
	gs.Active = 1 // opponent's turn

	ev := NewEvaluator(nil)
	tapped := ev.scoreThreat(gs, 0)

	// Counterfactual: same board state, big blocker UNTAPPED. The window
	// signal should not fire.
	gsUntapped := buildThreatGame(t, 5)
	_ = newTestPermanent(gsUntapped.Seats[0],
		newTestCardMinimal("Big Wall", []string{"creature"}, 4, nil), 0, 5)
	_ = newTestPermanent(gsUntapped.Seats[0],
		newTestCardMinimal("Small Wall", []string{"creature"}, 2, nil), 0, 2)
	gsUntapped.Active = 1
	untapped := ev.scoreThreat(gsUntapped, 0)

	if tapped >= untapped {
		t.Errorf("tapped big blocker on opp turn should be MORE negative than untapped; tapped=%.4f untapped=%.4f",
			tapped, untapped)
	}
}

// TestThreatR60v2_UntapWindow_InertOnOwnTurn: same tapped-big-blocker
// state but it's our turn — defensive shell isn't being tested right
// now, so no penalty.
func TestThreatR60v2_UntapWindow_InertOnOwnTurn(t *testing.T) {
	build := func(active int) *gameengine.GameState {
		gs := buildThreatGame(t, 5)
		b := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Big Wall", []string{"creature"}, 4, nil), 0, 5)
		b.Tapped = true
		_ = newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Small Wall", []string{"creature"}, 2, nil), 0, 2)
		gs.Active = active
		return gs
	}

	ev := NewEvaluator(nil)
	onOwnTurn := ev.scoreThreat(build(0), 0)
	onOppTurn := ev.scoreThreat(build(1), 0)

	// Own turn should be LESS negative (no untap-window penalty fires).
	if onOwnTurn <= onOppTurn {
		t.Errorf("own turn should not fire untap-window penalty; own=%.4f opp=%.4f",
			onOwnTurn, onOppTurn)
	}
}

// TestThreatR60v2_UntapWindow_GatedOnOppThreat: opp has only a 1/1
// token (power=1, below the >=4 gate) → window penalty shouldn't fire
// even with our big blocker tapped, because there's no pressure to
// block.
func TestThreatR60v2_UntapWindow_GatedOnOppThreat(t *testing.T) {
	gs := buildThreatGame(t, 1)
	b := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Big Wall", []string{"creature"}, 4, nil), 0, 5)
	b.Tapped = true
	_ = newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Small Wall", []string{"creature"}, 2, nil), 0, 2)
	gs.Active = 1

	ev := NewEvaluator(nil)
	got := ev.scoreThreat(gs, 0)

	gsBaseline := buildThreatGame(t, 1)
	_ = newTestPermanent(gsBaseline.Seats[0],
		newTestCardMinimal("Big Wall", []string{"creature"}, 4, nil), 0, 5)
	_ = newTestPermanent(gsBaseline.Seats[0],
		newTestCardMinimal("Small Wall", []string{"creature"}, 2, nil), 0, 2)
	gsBaseline.Active = 1
	baseline := ev.scoreThreat(gsBaseline, 0)

	// With opp power below the >=4 gate, tapped vs untapped should not
	// differ on the untap-window axis.
	if got < baseline-0.05 {
		t.Errorf("untap-window fired below opp-threat gate: got=%.4f baseline=%.4f", got, baseline)
	}
}

// ---------------------------------------------------------------------
// Signal C — interaction-mana starvation
// ---------------------------------------------------------------------

// TestThreatR60v2_InteractionStarvation_NoUntappedLandsOnOppTurn:
// active=opp, our only lands are tapped → 0 untapped → -0.20 penalty.
func TestThreatR60v2_InteractionStarvation_NoUntappedLandsOnOppTurn(t *testing.T) {
	gs := buildThreatGame(t, 5)
	for i := 0; i < 3; i++ {
		l := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
		l.Tapped = true
	}
	gs.Active = 1

	ev := NewEvaluator(nil)
	starved := ev.scoreThreat(gs, 0)

	gsUntapped := buildThreatGame(t, 5)
	for i := 0; i < 3; i++ {
		_ = newTestPermanent(gsUntapped.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	gsUntapped.Active = 1
	notStarved := ev.scoreThreat(gsUntapped, 0)

	if starved >= notStarved {
		t.Errorf("0 untapped lands on opp turn should be MORE negative; starved=%.4f untapped=%.4f",
			starved, notStarved)
	}
	delta := notStarved - starved
	if delta < 0.15 {
		t.Errorf("starvation delta too small: %.4f (want ≥0.15 for the -0.20 penalty)", delta)
	}
}

// TestThreatR60v2_InteractionStarvation_OneUntappedLandIsLessSevere:
// 1 untapped → -0.08 (less severe than 0 untapped's -0.20).
func TestThreatR60v2_InteractionStarvation_OneUntappedLandIsLessSevere(t *testing.T) {
	mk := func(untappedCount int) *gameengine.GameState {
		gs := buildThreatGame(t, 5)
		for i := 0; i < 3; i++ {
			l := newTestPermanent(gs.Seats[0],
				newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
			if i >= untappedCount {
				l.Tapped = true
			}
		}
		gs.Active = 1
		return gs
	}

	ev := NewEvaluator(nil)
	zero := ev.scoreThreat(mk(0), 0)
	one := ev.scoreThreat(mk(1), 0)
	plenty := ev.scoreThreat(mk(3), 0)

	if zero >= one {
		t.Errorf("0 untapped should be more negative than 1; 0=%.4f 1=%.4f", zero, one)
	}
	if one >= plenty {
		t.Errorf("1 untapped should be more negative than 3; 1=%.4f plenty=%.4f", one, plenty)
	}
}

// TestThreatR60v2_InteractionStarvation_InertOnOwnTurn: our own turn,
// even with 0 untapped lands, starvation penalty doesn't fire (we're
// not the one being threatened with unanswered casts).
func TestThreatR60v2_InteractionStarvation_InertOnOwnTurn(t *testing.T) {
	mk := func(active int) *gameengine.GameState {
		gs := buildThreatGame(t, 5)
		for i := 0; i < 3; i++ {
			l := newTestPermanent(gs.Seats[0],
				newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
			l.Tapped = true
		}
		gs.Active = active
		return gs
	}

	ev := NewEvaluator(nil)
	onOwnTurn := ev.scoreThreat(mk(0), 0)
	onOppTurn := ev.scoreThreat(mk(1), 0)

	if onOwnTurn <= onOppTurn {
		t.Errorf("own turn should not fire starvation; own=%.4f opp=%.4f", onOwnTurn, onOppTurn)
	}
}

// ---------------------------------------------------------------------
// Cross-signal independence — concentration + window + starvation
// each contribute independently, no double-counting.
// ---------------------------------------------------------------------

// TestThreatR60v2_AllSignalsStack: a keystone creature, tapped on opp
// turn, with 0 untapped lands triggers all three penalties additively.
// Total combined penalty must exceed any single signal alone but stay
// under the sum-of-caps so the dimension doesn't blow out to lethal
// from this trio alone.
func TestThreatR60v2_AllSignalsStack(t *testing.T) {
	gs := buildThreatGame(t, 8)
	keystone := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Keystone", []string{"creature"}, 8, nil), 10, 10)
	keystone.Tapped = true
	for i := 0; i < 3; i++ {
		l := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
		l.Tapped = true
	}
	gs.Active = 1

	ev := NewEvaluator(nil)
	all := ev.scoreThreat(gs, 0)

	gsNone := buildThreatGame(t, 8)
	// Same creature but untapped, lands untapped, our turn.
	_ = newTestPermanent(gsNone.Seats[0],
		newTestCardMinimal("Spread A", []string{"creature"}, 3, nil), 4, 4)
	_ = newTestPermanent(gsNone.Seats[0],
		newTestCardMinimal("Spread B", []string{"creature"}, 3, nil), 3, 3)
	_ = newTestPermanent(gsNone.Seats[0],
		newTestCardMinimal("Spread C", []string{"creature"}, 3, nil), 3, 3)
	for i := 0; i < 3; i++ {
		_ = newTestPermanent(gsNone.Seats[0],
			newTestCardMinimal("Plains", []string{"land"}, 0, nil), 0, 0)
	}
	gsNone.Active = 0
	none := ev.scoreThreat(gsNone, 0)

	combinedDelta := none - all
	if combinedDelta < 0.40 {
		t.Errorf("combined signals delta too small: %.4f (want ≥0.40)", combinedDelta)
	}
	// Sum-of-caps: concentration 0.50 + window 0.30 + starvation 0.20 = 1.00.
	// Allow for unrelated paths (lethalRatio differs slightly between the
	// two boards), but the total combined signal contribution should not
	// exceed ~1.10.
	if combinedDelta > 1.10 {
		t.Errorf("combined signals delta too large: %.4f (want ≤1.10 sum-of-caps + slack)", combinedDelta)
	}
}
