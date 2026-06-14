package gameengine

// r63 — proliferate mechanic probe (CR §701.34).
//
// Two bugs fixed:
//   (f) proliferate ignored §122.1g counter-doublers: the wrapper placed the
//       base count via perm.AddCounter and the counters-package
//       ApplyDoublingPipeline was an identity stub, so Doubling Season /
//       Hardened Scales never amplified a proliferated counter. Now the
//       legacy-map write runs through the §616 would_put_counter chain.
//   greedy cherry-pick: BuildGreedyProliferateTargets skipped an opponent's
//       +1/+1 while keeping its other kinds — an illegal partial proliferate
//       under §701.34a ("one more counter of EACH kind"). Now a permanent is
//       chosen all-kinds or not at all.

import "testing"

// (f) Doubling Season amplifies a proliferated +1/+1: base 1 → 2, so a
// creature with one +1/+1 ends at three.
func TestProliferate_DoublingSeasonAmplifies(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	cre := addCreature(gs, 0, "Walking Ballista", 0, 0)
	cre.Counters["+1/+1"] = 1

	applied, err := Proliferate(gs, 0, []ProliferateTarget{
		{Permanent: cre, CounterType: "+1/+1"},
	})
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1 (one choice)", applied)
	}
	if got := cre.Counters["+1/+1"]; got != 3 {
		t.Errorf("+1/+1 after proliferate under Doubling Season = %d, want 3 (1 + doubled 2)", got)
	}
}

// (f) Hardened Scales adds +1 to the proliferated +1/+1: base 1 → 2, so 1 → 3.
func TestProliferate_HardenedScalesAmplifies(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Hardened Scales", RegisterHardenedScales)
	cre := addCreature(gs, 0, "Walking Ballista", 0, 0)
	cre.Counters["+1/+1"] = 1

	if _, err := Proliferate(gs, 0, []ProliferateTarget{
		{Permanent: cre, CounterType: "+1/+1"},
	}); err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if got := cre.Counters["+1/+1"]; got != 3 {
		t.Errorf("+1/+1 after proliferate under Hardened Scales = %d, want 3 (1 + (1+1))", got)
	}
}

// (e) Without any doubler, proliferate adds EXACTLY one — not the full amount.
func TestProliferate_NoDoublerAddsExactlyOne(t *testing.T) {
	gs := newCombatGame(t)
	cre := addCreature(gs, 0, "Big Creature", 0, 0)
	cre.Counters["+1/+1"] = 5

	if _, err := Proliferate(gs, 0, []ProliferateTarget{
		{Permanent: cre, CounterType: "+1/+1"},
	}); err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if got := cre.Counters["+1/+1"]; got != 6 {
		t.Errorf("+1/+1 after proliferate = %d, want 6 (added exactly one, not doubled-to-10)", got)
	}
}

// greedy cherry-pick: an opponent permanent carrying a +1/+1 must be skipped
// ENTIRELY (you can't add its other kinds without also growing the +1/+1),
// while an opponent permanent with only bad-for-them counters IS chosen.
func TestBuildGreedyProliferateTargets_AtomicPerTarget(t *testing.T) {
	gs := newCombatGame(t)

	// Opponent creature with BOTH +1/+1 and -1/-1 — must be skipped whole.
	oppMixed := addCreature(gs, 1, "Opp Mixed", 2, 2)
	oppMixed.Counters["+1/+1"] = 1
	oppMixed.Counters["-1/-1"] = 1

	// Opponent creature with only a -1/-1 (bad for them) — proliferating it
	// helps us, so it IS chosen.
	oppBad := addCreature(gs, 1, "Opp Stunned", 2, 2)
	oppBad.Counters["-1/-1"] = 1

	// Our creature with +1/+1 — chosen (all kinds).
	mine := addCreature(gs, 0, "My Creature", 2, 2)
	mine.Counters["+1/+1"] = 1

	targets := BuildGreedyProliferateTargets(gs, 0)

	sawMixed, sawOppBad, sawMine := false, false, false
	for _, tg := range targets {
		if tg.Permanent == oppMixed {
			sawMixed = true
		}
		if tg.Permanent == oppBad {
			sawOppBad = true
		}
		if tg.Permanent == mine {
			sawMine = true
		}
	}
	if sawMixed {
		t.Errorf("opponent permanent with a +1/+1 must NOT be chosen (illegal to add its -1/-1 while omitting +1/+1 per §701.34a)")
	}
	if !sawOppBad {
		t.Errorf("opponent permanent with only a -1/-1 SHOULD be chosen")
	}
	if !sawMine {
		t.Errorf("our own +1/+1 creature should be chosen")
	}
}

// (a) zero is a legal choice — empty target list is a clean no-op.
func TestProliferate_ZeroChoicesNoOp(t *testing.T) {
	gs := newCombatGame(t)
	applied, err := Proliferate(gs, 0, nil)
	if err != nil || applied != 0 {
		t.Errorf("Proliferate(nil) = (%d, %v), want (0, nil)", applied, err)
	}
}

// (c)/(d) player poison and planeswalker loyalty both proliferate (+1 each).
func TestProliferate_PlayerPoisonAndLoyalty(t *testing.T) {
	gs := newCombatGame(t)
	gs.Seats[1].PoisonCounters = 3
	pw := addBattlefield(gs, 0, "Some Planeswalker", 0, 0, "planeswalker")
	pw.Counters["loyalty"] = 4

	_, err := Proliferate(gs, 0, []ProliferateTarget{
		{Player: gs.Seats[1], CounterType: "poison"},
		{Permanent: pw, CounterType: "loyalty"},
	})
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if gs.Seats[1].PoisonCounters != 4 {
		t.Errorf("opponent poison after proliferate = %d, want 4", gs.Seats[1].PoisonCounters)
	}
	if got := pw.Counters["loyalty"]; got != 5 {
		t.Errorf("planeswalker loyalty after proliferate = %d, want 5", got)
	}
}
