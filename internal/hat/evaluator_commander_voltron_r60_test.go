package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_commander_voltron_r60_test.go — pins the voltron-setup signal
// added to scoreCommander in r60. Pre-r60, a vanilla commander on field
// scored identically to a commander stacked with equipment + auras +
// counters — every voltron-shape investment was invisible until the
// commander damage actually landed. The new block adds:
//
//   1. +0.12 per equipment AttachedTo the commander (cap 4)
//   2. +0.10 per aura AttachedTo the commander (cap 4)
//   3. +0.05 per power-point-above-base on the commander (cap 8)
//   4. ×1.5 when Strategy.Archetype == ArchetypeVoltron
//
// All four are pinned below with computed deltas vs a vanilla baseline so
// a tuning change that shifts the per-piece coefficients fails loudly.

// setupVoltronGame builds a 4-seat Commander pod with seat 0's commander
// already on the battlefield. Returns the game + the commander permanent
// so tests can attach equipment/auras to it.
func setupVoltronGame(t *testing.T, basePower int) (*gameengine.GameState, *gameengine.Permanent) {
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	cmdr := newTestCardMinimal("Test Commander", []string{"creature", "legendary"}, 4, nil)
	cmdrPerm := newTestPermanent(gs.Seats[0], cmdr, basePower, basePower)
	gs.Seats[0].CommanderNames = []string{"Test Commander"}
	return gs, cmdrPerm
}

func vanillaCommanderBaseline(t *testing.T) float64 {
	gs, _ := setupVoltronGame(t, 3)
	ev := NewEvaluator(nil)
	return ev.scoreCommander(gs, 0)
}

// -----------------------------------------------------------------------------
// 1. Equipment attached to commander → +0.12/piece (cap 4)
// -----------------------------------------------------------------------------

func TestScoreCommander_EquipmentAttachedAddsVoltronBonus(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	for i := 0; i < 3; i++ {
		eq := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Sword of X and Y", []string{"artifact", "equipment"}, 3, nil), 0, 0)
		eq.AttachedTo = cmdrPerm
	}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)

	// 3 equipment × 0.12 = +0.36 over baseline.
	want := baseline + 3*0.12
	if !floatClose(got, want) {
		t.Errorf("3 equipment on commander: got %.4f, want %.4f (baseline %.4f)", got, want, baseline)
	}
}

func TestScoreCommander_EquipmentSaturatesAtFour(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	// 6 equipment attached — should cap at 4.
	for i := 0; i < 6; i++ {
		eq := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Equipment", []string{"artifact", "equipment"}, 2, nil), 0, 0)
		eq.AttachedTo = cmdrPerm
	}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := baseline + 4*0.12 // capped
	if !floatClose(got, want) {
		t.Errorf("6 equipment (cap 4): got %.4f, want %.4f", got, want)
	}
}

// -----------------------------------------------------------------------------
// 2. Aura attached to commander → +0.10/piece (cap 4)
// -----------------------------------------------------------------------------

func TestScoreCommander_AurasAttachedAddVoltronBonus(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	for i := 0; i < 2; i++ {
		a := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Aura", []string{"enchantment", "aura"}, 2, nil), 0, 0)
		a.AttachedTo = cmdrPerm
	}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := baseline + 2*0.10
	if !floatClose(got, want) {
		t.Errorf("2 auras on commander: got %.4f, want %.4f", got, want)
	}
}

// Aura attached to a NON-commander creature must NOT contribute — the
// voltron signal is commander-specific, not general battlefield buff.
func TestScoreCommander_AuraOnNonCommanderDoesNotFire(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, _ := setupVoltronGame(t, 3)
	other := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	a := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Aura", []string{"enchantment", "aura"}, 2, nil), 0, 0)
	a.AttachedTo = other

	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("aura on non-commander leaked into voltron score: got %.4f, baseline %.4f", got, baseline)
	}
}

// -----------------------------------------------------------------------------
// 3. Power-above-base on commander → +0.05/point (cap 8)
// -----------------------------------------------------------------------------

func TestScoreCommander_CountersOnCommanderAddPowerDelta(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	cmdrPerm.Counters = map[string]int{"+1/+1": 4}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := baseline + 4*0.05
	if !floatClose(got, want) {
		t.Errorf("4 +1/+1 counters: got %.4f, want %.4f", got, want)
	}
}

func TestScoreCommander_ModificationsOnCommanderAddPowerDelta(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	cmdrPerm.Modifications = []gameengine.Modification{
		{Power: 3, Toughness: 3, Duration: "until_end_of_turn"},
	}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := baseline + 3*0.05
	if !floatClose(got, want) {
		t.Errorf("+3 power mod: got %.4f, want %.4f", got, want)
	}
}

func TestScoreCommander_PowerDeltaSaturatesAtEight(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	cmdrPerm.Counters = map[string]int{"+1/+1": 15} // way above cap
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	want := baseline + 8*0.05 // capped at 8
	if !floatClose(got, want) {
		t.Errorf("15 counters (cap 8): got %.4f, want %.4f", got, want)
	}
}

// Negative power delta (e.g. -1/-1 counters dragging commander below base)
// must NOT subtract from the voltron signal — the signal credits INVESTMENT,
// not net stat ranking. A debuffed commander is a separate concern handled
// by other dimensions.
func TestScoreCommander_NegativePowerDeltaIsClamped(t *testing.T) {
	baseline := vanillaCommanderBaseline(t)

	gs, cmdrPerm := setupVoltronGame(t, 3)
	cmdrPerm.Counters = map[string]int{"-1/-1": 2}
	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("negative power delta leaked into voltron: got %.4f, baseline %.4f", got, baseline)
	}
}

// -----------------------------------------------------------------------------
// 4. Voltron archetype gets 1.5x multiplier on the voltron block
// -----------------------------------------------------------------------------

func TestScoreCommander_VoltronArchetypeAmplifies(t *testing.T) {
	gs, cmdrPerm := setupVoltronGame(t, 3)
	eq := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Sword", []string{"artifact", "equipment"}, 3, nil), 0, 0)
	eq.AttachedTo = cmdrPerm

	evGeneric := NewEvaluator(nil)
	genericScore := evGeneric.scoreCommander(gs, 0)

	evVoltron := NewEvaluator(&StrategyProfile{Archetype: ArchetypeVoltron})
	voltronScore := evVoltron.scoreCommander(gs, 0)

	// With 1 equipment: generic baseline + 0.12; voltron baseline + 0.18.
	// Both share the same on-field synergy bonus baseline, so the delta
	// is 0.06 (1.5x - 1.0x times 0.12).
	delta := voltronScore - genericScore
	want := 0.12 * 0.5
	if !floatClose(delta, want) {
		t.Errorf("voltron amplification delta: got %.4f, want %.4f (generic=%.4f voltron=%.4f)",
			delta, want, genericScore, voltronScore)
	}
}

// -----------------------------------------------------------------------------
// Combined: equipment + aura + power delta + voltron amp all stack
// -----------------------------------------------------------------------------

func TestScoreCommander_FullVoltronStack(t *testing.T) {
	gs, cmdrPerm := setupVoltronGame(t, 3)
	// 3 equipment, 2 auras, 4 +1/+1 counters → equipment 0.36 + aura 0.20
	// + power 0.20 = 0.76, then ×1.5 voltron = 1.14.
	for i := 0; i < 3; i++ {
		eq := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Equipment", []string{"artifact", "equipment"}, 2, nil), 0, 0)
		eq.AttachedTo = cmdrPerm
	}
	for i := 0; i < 2; i++ {
		a := newTestPermanent(gs.Seats[0],
			newTestCardMinimal("Aura", []string{"enchantment", "aura"}, 2, nil), 0, 0)
		a.AttachedTo = cmdrPerm
	}
	cmdrPerm.Counters = map[string]int{"+1/+1": 4}

	evVoltron := NewEvaluator(&StrategyProfile{Archetype: ArchetypeVoltron})
	got := evVoltron.scoreCommander(gs, 0)

	// Baseline (commander on field, no command zone, no damage): same
	// synergy bonus the empty-board test uses.
	gsBase, _ := setupVoltronGame(t, 3)
	baseline := evVoltron.scoreCommander(gsBase, 0)
	voltron := (3*0.12 + 2*0.10 + 4*0.05) * 1.5
	want := baseline + voltron
	if !floatClose(got, want) {
		t.Errorf("full voltron stack: got %.4f, want %.4f (baseline %.4f, voltron %.4f)",
			got, want, baseline, voltron)
	}
}

// -----------------------------------------------------------------------------
// Negative-of-the-fix: commander NOT on field → no voltron signal.
// -----------------------------------------------------------------------------

func TestScoreCommander_NoVoltronWhenCommanderOffField(t *testing.T) {
	// Use the existing helper from evaluator_commander_progress_r60_test.go
	// which puts the commander in command zone.
	gs := setupCommanderGame(t, 4, 0, 5)
	// Even if some random equipment exists on board (unattached or attached
	// to other creatures), no voltron credit should fire because the
	// commander isn't on the battlefield.
	other := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	eq := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Sword", []string{"artifact", "equipment"}, 3, nil), 0, 0)
	eq.AttachedTo = other

	ev := NewEvaluator(nil)
	got := ev.scoreCommander(gs, 0)
	// With no commander on field, no tax (CommanderCastCounts=0), no damage,
	// score should be 0 — no voltron leak.
	if !floatClose(got, 0) {
		t.Errorf("commander off field should yield no voltron credit: got %.4f", got)
	}
}
