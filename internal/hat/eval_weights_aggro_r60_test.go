package hat

// R60 round 5 — Aggro archetype retune off the generic creature-deck
// profile. Pins four properties:
//   1. BoardPresence anchored at 2.0 (signature dimension, matches other
//      custom-profile anchors).
//   2. LifeResource bumped above midrange — aggro values the opponent-
//      life win clock.
//   3. ComboProximity stays low — aggro doesn't off-line into engine
//      pieces.
//   4. ManaAdvantage is neutral (>= midrange) — aggro shouldn't snub
//      cheap ramp / on-curve utility mana.
// Also pins the `scoreLife` opponent-pressure component that makes the
// LifeResource weight actually translate into "press the weakest opp's
// life to zero" behavior.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestAggro_R60_WeightAnchors(t *testing.T) {
	a := DefaultWeightsForArchetype(ArchetypeAggro)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)

	if a.BoardPresence < 2.0 {
		t.Errorf("Aggro BoardPresence must anchor at 2.0 (signature dimension); got %.2f", a.BoardPresence)
	}
	if a.LifeResource <= m.LifeResource {
		t.Errorf("Aggro LifeResource (%.2f) must exceed midrange (%.2f) — opponent-life clock is the win plan",
			a.LifeResource, m.LifeResource)
	}
	if a.LifeResource < 1.5 {
		t.Errorf("Aggro LifeResource must be substantially above midrange (>=1.5); got %.2f", a.LifeResource)
	}
	if a.ComboProximity >= m.ComboProximity {
		t.Errorf("Aggro ComboProximity (%.2f) must stay below midrange (%.2f) — no off-line combo distraction",
			a.ComboProximity, m.ComboProximity)
	}
	if a.ManaAdvantage < m.ManaAdvantage-0.01 {
		t.Errorf("Aggro ManaAdvantage (%.2f) must be neutral (>= midrange %.2f); pre-r60 was 0.3 which snubbed cheap ramp",
			a.ManaAdvantage, m.ManaAdvantage)
	}
}

func TestAggro_R60_OffGeneric(t *testing.T) {
	// "Off generic" — the Aggro profile must not collapse onto midrange
	// across all four directive dimensions simultaneously.
	a := DefaultWeightsForArchetype(ArchetypeAggro)
	m := DefaultWeightsForArchetype(ArchetypeMidrange)
	if a == m {
		t.Fatal("Aggro profile is identical to midrange — retune did not land")
	}
	differs := 0
	if a.BoardPresence != m.BoardPresence {
		differs++
	}
	if a.LifeResource != m.LifeResource {
		differs++
	}
	if a.ComboProximity != m.ComboProximity {
		differs++
	}
	// ManaAdvantage may equal midrange (that's the "neutral" instruction).
	if differs < 3 {
		t.Errorf("expected Aggro to diverge from midrange on >=3 of (BoardPresence, LifeResource, ComboProximity); got %d differences", differs)
	}
}

func TestScoreLife_OpponentPressureLowersEnemyLifeRaisesScore(t *testing.T) {
	// Holding own life constant, lowering an opponent's life should
	// INCREASE LifeResource — that's the new "win clock" signal.
	ev := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})

	gsHealthy := newTestGame(t, 4)
	for i := range gsHealthy.Seats {
		gsHealthy.Seats[i].Life = 40
		gsHealthy.Seats[i].StartingLife = 40
	}

	gsPressured := newTestGame(t, 4)
	for i := range gsPressured.Seats {
		gsPressured.Seats[i].Life = 40
		gsPressured.Seats[i].StartingLife = 40
	}
	gsPressured.Seats[1].Life = 12 // strongest threat opp on a clock

	healthy := ev.scoreLife(gsHealthy, 0)
	pressured := ev.scoreLife(gsPressured, 0)

	if pressured <= healthy {
		t.Errorf("scoreLife should rise as a weak opp's life drops: healthy=%.3f pressured=%.3f",
			healthy, pressured)
	}
}

func TestScoreLife_OpponentPressureUsesWeakestOpp(t *testing.T) {
	// The pressure signal must track the WEAKEST opponent, not the
	// average — that's where the win clock points.
	ev := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})

	gsOneLow := newTestGame(t, 4)
	for i := range gsOneLow.Seats {
		gsOneLow.Seats[i].Life = 40
		gsOneLow.Seats[i].StartingLife = 40
	}
	gsOneLow.Seats[1].Life = 5 // very weak opp

	gsAllMid := newTestGame(t, 4)
	for i := range gsAllMid.Seats {
		gsAllMid.Seats[i].Life = 25 // all mid-range, no clock
		gsAllMid.Seats[i].StartingLife = 40
	}
	gsAllMid.Seats[0].Life = 40

	oneLow := ev.scoreLife(gsOneLow, 0)
	allMid := ev.scoreLife(gsAllMid, 0)

	if oneLow <= allMid {
		t.Errorf("scoreLife should weight a single weak opp (life=5) above three mid opps (life=25): oneLow=%.3f allMid=%.3f",
			oneLow, allMid)
	}
}

func TestScoreLife_PressureSkipsLostOpponents(t *testing.T) {
	// A lost / left-game opponent must not contribute to the pressure
	// signal (they can't be attacked).
	ev := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})

	gs := newTestGame(t, 4)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Seats[1].Life = 0
	gs.Seats[1].Lost = true

	got := ev.scoreLife(gs, 0)
	// Own life=40, ratio=1.0, base=(1.0-0.5)*0.5=0.25; no live opps below
	// starting life → pressure=0. Total ~0.25.
	if got > 0.30 {
		t.Errorf("lost opponent must not generate pressure signal; got scoreLife=%.3f, expected ~0.25",
			got)
	}
}

func TestScoreLife_PressureBoundedAtHalf(t *testing.T) {
	// Even with an opp at 0 life (not yet flagged Lost), pressure is
	// capped at +0.5 so it can't overwhelm the own-life term.
	ev := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})

	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 0
	gs.Seats[1].StartingLife = 40

	got := ev.scoreLife(gs, 0)
	// Own life=40 → base=0.25; pressure capped at +0.5 → total <= 0.75.
	if got > 0.76 {
		t.Errorf("scoreLife pressure component must be capped at +0.5; got %.3f", got)
	}
}

// Sanity: with the new weights and pressure signal, an Aggro hat
// evaluating a board where it's pressuring a weak opp should weight the
// LifeResource contribution higher than a Control hat would on the same
// state.
func TestEvaluator_AggroValuesPressureMoreThanControl(t *testing.T) {
	gs := newTestGame(t, 4)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	gs.Seats[1].Life = 8 // weakest opp on a clock

	aggro := NewEvaluator(&StrategyProfile{Archetype: ArchetypeAggro})
	control := NewEvaluator(&StrategyProfile{Archetype: ArchetypeControl})

	aggroDetail := aggro.EvaluateDetailed(gs, 0)
	controlDetail := control.EvaluateDetailed(gs, 0)

	aggroContrib := aggroDetail.LifeResource * DefaultWeightsForArchetype(ArchetypeAggro).LifeResource
	controlContrib := controlDetail.LifeResource * DefaultWeightsForArchetype(ArchetypeControl).LifeResource

	if aggroContrib <= controlContrib {
		t.Errorf("Aggro LifeResource contribution (%.3f) must exceed Control (%.3f) when pressuring a weak opp",
			aggroContrib, controlContrib)
	}
	// And the raw per-seat life score must be positive (own healthy + opp
	// on the clock) — not the previous "own life as a survival resource"
	// near-zero output.
	if aggroDetail.LifeResource <= 0 {
		t.Errorf("scoreLife should be positive when healthy AND pressuring a weak opp; got %.3f",
			aggroDetail.LifeResource)
	}
	_ = gameengine.GameState{}
}
