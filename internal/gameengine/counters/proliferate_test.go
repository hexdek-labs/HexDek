package counters

import (
	"errors"
	"testing"
)

// playerMock is a mockTarget shape for player-counter property tests —
// it carries no card type but still satisfies the Target interface.
// Proliferate's primitive intentionally skips the AddCounters
// ValidTargets gate, so this works for poison/experience/rad without
// extending targetMatches.
func newPlayerMock() *mockTarget {
	return &mockTarget{}
}

// seedStack appends a CounterStack directly, bypassing AddCounters'
// validation. Used to seed the player mock with poison/experience/rad
// counters whose ValidTargets exclude non-player TargetTypes — needed
// because AddCounters would reject the placement and skew the test.
func seedStack(m *mockTarget, kind string, count int) {
	m.stacks = append(m.stacks, CounterStack{
		Type:               CanonicalName(kind),
		Count:              count,
		PlacedByInstanceID: "seed",
		PlacedAtTick:       0,
	})
}

// TestProliferate_PoisonOnPlayer pins CR §701.23 against the canonical
// "Atraxa proliferates poison on an opponent" case. Seat with 3 poison
// counters → after proliferate the count must be 4.
func TestProliferate_PoisonOnPlayer(t *testing.T) {
	p := newPlayerMock()
	seedStack(p, "poison", 3)
	choices := []ProliferateChoice{{Target: p, CounterType: "poison"}}
	applied, err := Proliferate(choices, "atraxa-end-step", 42)
	if err != nil {
		t.Fatalf("Proliferate: unexpected error %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if got := CounterCount(p, "poison"); got != 4 {
		t.Errorf("poison after = %d, want 4", got)
	}
}

// TestProliferate_PlusPlusOnCreature pins the +1/+1 path — proliferate
// on a creature already carrying a +1/+1 counter must add one more.
func TestProliferate_PlusPlusOnCreature(t *testing.T) {
	m := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(m, "+1/+1", 2, "seed", 1); err != nil {
		t.Fatalf("seed +1/+1: %v", err)
	}
	applied, err := Proliferate([]ProliferateChoice{{Target: m, CounterType: "+1/+1"}}, "inexorable-tide", 7)
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if got := CounterCount(m, "+1/+1"); got != 3 {
		t.Errorf("+1/+1 after = %d, want 3", got)
	}
}

// TestProliferate_PermanentAndPlayerTogether pins that a single
// proliferate call may target a permanent AND a player simultaneously —
// the §701.23 "any number of permanents and/or players" clause.
func TestProliferate_PermanentAndPlayerTogether(t *testing.T) {
	creature := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(creature, "+1/+1", 1, "seed", 1); err != nil {
		t.Fatalf("seed +1/+1: %v", err)
	}
	player := newPlayerMock()
	seedStack(player, "poison", 5)
	choices := []ProliferateChoice{
		{Target: creature, CounterType: "+1/+1"},
		{Target: player, CounterType: "poison"},
	}
	applied, err := Proliferate(choices, "contagion-engine", 10)
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	if got := CounterCount(creature, "+1/+1"); got != 2 {
		t.Errorf("creature +1/+1 after = %d, want 2", got)
	}
	if got := CounterCount(player, "poison"); got != 6 {
		t.Errorf("player poison after = %d, want 6", got)
	}
}

// TestProliferate_EnergyExcluded pins the §106.11 exception — energy is
// a resource pool, not a §122 counter. Even with energy "counters"
// seeded on a target, Proliferate rejects the choice because the
// registry has no "energy" entry (IsProliferateEligibleType=false).
func TestProliferate_EnergyExcluded(t *testing.T) {
	p := newPlayerMock()
	seedStack(p, "energy", 4) // CanonicalName passthrough; no registry entry
	if IsProliferateEligibleType("energy") {
		t.Fatalf("energy reported proliferate-eligible; want excluded (CR §106.11)")
	}
	_, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "energy"}}, "tezzeret-emblem", 1)
	if !errors.Is(err, ErrUnknownCounterType) {
		t.Errorf("err = %v, want ErrUnknownCounterType (energy not registered as §122 counter)", err)
	}
}

// TestProliferate_RadCountersEligible pins that the Phase 4 rad
// registration makes rad counters proliferate-eligible (March of the
// Machine mechanic).
func TestProliferate_RadCountersEligible(t *testing.T) {
	if !IsProliferateEligibleType("rad") {
		t.Fatalf("rad reported NOT proliferate-eligible; want eligible per Phase 4 registration")
	}
	p := newPlayerMock()
	seedStack(p, "rad", 2)
	applied, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "rad"}}, "fallout", 3)
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if got := CounterCount(p, "rad"); got != 3 {
		t.Errorf("rad after = %d, want 3", got)
	}
}

// TestProliferate_ExperienceEligible mirrors the rad case for the
// experience-counter family (Daretti, Mizzix, Ezuri-style trackers).
func TestProliferate_ExperienceEligible(t *testing.T) {
	if !IsProliferateEligibleType("experience") {
		t.Fatalf("experience reported NOT proliferate-eligible")
	}
	p := newPlayerMock()
	seedStack(p, "experience", 1)
	applied, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "experience"}}, "mizzix-of-the-izmagnus", 11)
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1", applied)
	}
	if got := CounterCount(p, "experience"); got != 2 {
		t.Errorf("experience after = %d, want 2", got)
	}
}

// TestProliferate_ChoiceConstraintRejectsAbsentKind pins the §701.23
// "of a kind already there" rule — choosing a counter type the target
// does not currently carry returns ErrCounterTypeNotPresent. Defends
// against per_card handlers that build a choice list from stale state
// (e.g. a +1/+1 stack drained by §704.5q in the same priority pass).
func TestProliferate_ChoiceConstraintRejectsAbsentKind(t *testing.T) {
	m := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(m, "+1/+1", 1, "seed", 1); err != nil {
		t.Fatalf("seed +1/+1: %v", err)
	}
	// Caller asks for charge — never present on this target.
	_, err := Proliferate([]ProliferateChoice{{Target: m, CounterType: "charge"}}, "src", 1)
	if !errors.Is(err, ErrCounterTypeNotPresent) {
		t.Errorf("err = %v, want ErrCounterTypeNotPresent", err)
	}
	if got := CounterCount(m, "charge"); got != 0 {
		t.Errorf("charge after rejected proliferate = %d, want 0", got)
	}
}

// TestProliferate_EmptyChoicesNoOp pins that an empty choices slice
// returns (0, nil) cleanly — CR §701.23 permits "any number" including
// zero (controller chose to proliferate nothing).
func TestProliferate_EmptyChoicesNoOp(t *testing.T) {
	applied, err := Proliferate(nil, "src", 0)
	if err != nil || applied != 0 {
		t.Errorf("Proliferate(nil) = (%d, %v), want (0, nil)", applied, err)
	}
}

// TestProliferate_NilTargetRejected pins the defensive nil-target gate.
// A wrapper that fails to filter nil seats / dead permanents should hit
// a loud error rather than a silent skip.
func TestProliferate_NilTargetRejected(t *testing.T) {
	_, err := Proliferate([]ProliferateChoice{{Target: nil, CounterType: "+1/+1"}}, "src", 1)
	if !errors.Is(err, ErrNilTarget) {
		t.Errorf("err = %v, want ErrNilTarget", err)
	}
}

// TestProliferate_LineageStamped pins that PlacedByInstanceID + tick
// flow through to the resulting CounterStack — the InstanceID Phase 3
// linkage continues through proliferate-driven placements so Loki
// replay can trace each counter to its source ability.
func TestProliferate_LineageStamped(t *testing.T) {
	p := newPlayerMock()
	seedStack(p, "poison", 1)
	_, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "poison"}}, "atraxa-end-step-iid", 99)
	if err != nil {
		t.Fatalf("Proliferate: %v", err)
	}
	// Find the new stack — appendOrMerge will fold into the seed if the
	// instance ids match (they don't here: seed vs atraxa-end-step-iid),
	// so we expect a fresh stack with the source iid.
	var found bool
	for _, st := range p.CounterStacks() {
		if st.PlacedByInstanceID == "atraxa-end-step-iid" && st.PlacedAtTick == 99 && st.Type == "poison" && st.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("no poison stack with lineage atraxa-end-step-iid@99; stacks = %+v", p.CounterStacks())
	}
}
