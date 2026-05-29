package counters

import (
	"errors"
	"testing"
)

// Counter DB Phase 8 — Energy + Experience Seat-resource carveout.
//
// Per CR §106.11, energy is a resource pool (not a §122 counter).
// Per Phase 8 (Probe F), experience counters are treated as a Seat
// resource analog of energy: Daxos / Mizzix / Daretti / Ezuri / Meren
// trackers live at Seat.XPCounters (mirror of Seat.Flags["experience_
// counters"]) and bypass both the proliferate pipeline and the §122.1g
// doubling-replacement walk.
//
// The carveout is enforced structurally — both "energy" and
// "experience" are intentionally absent from the §122 registry, so:
//   - IsProliferateEligibleType returns false for both.
//   - AddCountersWithDoublers returns ErrUnknownCounterType (which the
//     gameengine bridge interprets as "skip this kind").
//   - ApplyDoublers short-circuits to identity even with a Doubling
//     Season-equivalent Doubler in the chain.
//
// These properties are the load-bearing safety net for the per_card
// energy/XP handlers' direct-Flags writes; if a future registry change
// accidentally re-introduces "energy" or "experience", every test below
// fails noisily.

// TestPhase8EnergyAbsentFromRegistry pins the §106.11 exclusion. The
// registry must NOT carry an "energy" entry.
func TestPhase8EnergyAbsentFromRegistry(t *testing.T) {
	if def := Lookup("energy"); def != nil {
		t.Errorf("energy registered: %+v (CR §106.11 — energy is a resource pool, not a §122 counter)", def)
	}
}

// TestPhase8ExperienceAbsentFromRegistry pins the Phase 8 carveout.
// The registry must NOT carry an "experience" entry; per_card handlers
// route XP through Seat.XPCounters instead.
func TestPhase8ExperienceAbsentFromRegistry(t *testing.T) {
	if def := Lookup("experience"); def != nil {
		t.Errorf("experience registered: %+v (Phase 8 — XP is a Seat resource, not a §122 counter)", def)
	}
}

// TestPhase8EnergyNotProliferateEligible pins that proliferate cannot
// add energy. Even with energy "counters" present on a target, the
// registry probe rejects the choice.
func TestPhase8EnergyNotProliferateEligible(t *testing.T) {
	if IsProliferateEligibleType("energy") {
		t.Fatal("energy reported proliferate-eligible; want excluded (CR §106.11)")
	}
	p := newPlayerMock()
	seedStack(p, "energy", 4)
	_, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "energy"}}, "tezzeret-emblem", 1)
	if !errors.Is(err, ErrUnknownCounterType) {
		t.Errorf("err = %v, want ErrUnknownCounterType", err)
	}
}

// TestPhase8ExperienceNotProliferateEligible pins that proliferate
// cannot add experience. Inexorable Tide on a Mizzix board cannot
// substitute for casting spells to gain XP.
func TestPhase8ExperienceNotProliferateEligible(t *testing.T) {
	if IsProliferateEligibleType("experience") {
		t.Fatal("experience reported proliferate-eligible; want excluded (Phase 8 Seat-resource carveout)")
	}
	p := newPlayerMock()
	seedStack(p, "experience", 1)
	_, err := Proliferate([]ProliferateChoice{{Target: p, CounterType: "experience"}}, "inexorable-tide", 1)
	if !errors.Is(err, ErrUnknownCounterType) {
		t.Errorf("err = %v, want ErrUnknownCounterType", err)
	}
}

// TestPhase8PoisonAndRadStillProliferateEligible is a regression test
// for the Phase 4 player-counter family. Phase 8 narrowed the player-
// counter set to exactly {poison, rad}; both MUST stay proliferate-
// eligible per CR §701.27. Defends against an over-broad future sweep
// that mistakenly removes poison/rad alongside experience.
func TestPhase8PoisonAndRadStillProliferateEligible(t *testing.T) {
	for _, name := range []string{"poison", "rad"} {
		if !IsProliferateEligibleType(name) {
			t.Errorf("%q reported NOT proliferate-eligible; Phase 4 ruling requires eligibility per CR §701.27", name)
		}
		def := Lookup(name)
		if def == nil {
			t.Errorf("%q not registered", name)
			continue
		}
		if def.DoublingApplies {
			t.Errorf("%q DoublingApplies = true — player counters must NOT double per engine ruling", name)
		}
	}
}

// TestPhase8EnergyAddCountersRejected pins the §122.1g pipeline gate:
// AddCountersWithDoublers refuses energy, so Doubling Season cannot
// double an energy "placement" even if a caller tries to wrap one.
func TestPhase8EnergyAddCountersRejected(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _, err := AddCountersWithDoublers(target, "energy", 3, "harnessed-lightning", 0, 0, []Doubler{doublingSeason(0)})
	if !errors.Is(err, ErrUnknownCounterType) {
		t.Errorf("err = %v, want ErrUnknownCounterType (energy excluded from §122 pipeline)", err)
	}
}

// TestPhase8ExperienceAddCountersRejected mirrors the energy test —
// Doubling Season cannot wrap an XP gain even if a caller routes it
// through the §122.1g pipeline.
func TestPhase8ExperienceAddCountersRejected(t *testing.T) {
	p := newPlayerMock()
	_, _, err := AddCountersWithDoublers(p, "experience", 1, "daxos-the-returned", 0, 0, []Doubler{doublingSeason(0)})
	if !errors.Is(err, ErrUnknownCounterType) {
		t.Errorf("err = %v, want ErrUnknownCounterType (experience excluded from §122 pipeline in Phase 8)", err)
	}
}

// TestPhase8EnergyApplyDoublersIdentity pins the explicit short-circuit:
// ApplyDoublers on an unregistered type (energy) returns baseCount
// unchanged regardless of how many doublers are in the chain. This is
// the same gate that protects the legacy direct-Flags writes used by
// the energy.go GainEnergy helper.
func TestPhase8EnergyApplyDoublersIdentity(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	final, chain := ApplyDoublers(target, "energy", 2, 0, []Doubler{doublingSeason(0), doublingSeason(1)})
	if final != 2 {
		t.Errorf("energy gain with 2x DS: got %d, want 2 (unchanged — §106.11 carve-out)", final)
	}
	if len(chain) != 0 {
		t.Errorf("energy chain len = %d, want 0", len(chain))
	}
}

// TestPhase8ExperienceApplyDoublersIdentity is the analogue for XP.
// Doubling Season on the battlefield does NOT double Daxos's
// "you get an experience counter" trigger — the Phase 8 carveout
// keeps XP gain at the printed rate.
func TestPhase8ExperienceApplyDoublersIdentity(t *testing.T) {
	p := newPlayerMock()
	final, chain := ApplyDoublers(p, "experience", 1, 0, []Doubler{doublingSeason(0)})
	if final != 1 {
		t.Errorf("XP gain with DS: got %d, want 1 (unchanged — Phase 8 carve-out)", final)
	}
	if len(chain) != 0 {
		t.Errorf("XP chain len = %d, want 0", len(chain))
	}
}

// TestPhase8GenericPlayerCounterFamilyIsPoisonAndRad pins the narrowed
// Phase 8 §122-generic player-counter family: exactly poison + rad.
// Restricted to (Category=OtherTracker AND ValidTargets=[TargetPlayer])
// — Phase 5 per-card replacement player-counters (incarnation, echo)
// live in Category=ResourceMarker and are intentionally not part of
// the generic family. Defends against silent regression if a future
// commit re-adds experience as an OtherTracker §122 entry.
func TestPhase8GenericPlayerCounterFamilyIsPoisonAndRad(t *testing.T) {
	got := map[string]bool{}
	for _, def := range All() {
		if def.Category != OtherTracker {
			continue
		}
		if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetPlayer {
			continue
		}
		got[def.Name] = true
	}
	want := map[string]bool{"poison": true, "rad": true}
	if len(got) != len(want) {
		t.Errorf("generic §122 player-counter family size = %d, want 2 (poison + rad); got = %v", len(got), got)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing generic §122 player counter %q", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("unexpected generic §122 player counter %q (Phase 8 carveout requires Seat-resource routing instead)", name)
		}
	}
}
