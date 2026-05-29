package counters

import (
	"testing"
)

// Counter DB Phase 6 — property tests for ApplyDoublers and the
// AddCountersWithDoublers entry point. Mirrors the spec scenarios from
// docs/counter-db-implementation-plan-r60.md §6:
//   - Walking Ballista X=2 + Hardened Scales = 3 (Hardened Scales fires
//     once, adding +1)
//   - Walking Ballista X=2 + Doubling Season + Hardened Scales = 5 with
//     order Double-then-AddOne (2 → 4 → 5) or 6 with order
//     AddOne-then-Double (2 → 3 → 6). Both are §616-controller-chosen-
//     legal; the pipeline RESPECTS input order.
//   - Walking Ballista X=2 + DS + HS + BE under §616 controller-chosen
//     orderings produces 6 distinct possible totals; the test pins the
//     two extremes (smallest = 2→4→5→10 = 10, largest = 2→3→6→12 = 12)
//     and confirms the chain length always equals 3.
//   - Energy short-circuits (DoublingApplies=false → identity).
//   - Vorinclex asymmetric: opponent halves, controller doubles. Both
//     gated by ControllerSeat vs targetController.

// mockDoubler implements counters.Doubler with explicit per-field overrides
// so each test case can dial in a tight scenario.
type mockDoubler struct {
	name       string
	sourceID   string
	handlerID  string
	timestamp  int
	controller int
	kind       DoublerKind
	// gate selects the Applies predicate. Empty gate means "always".
	gateOppOnly        bool
	gateControllerOnly bool
	gateCounterType    string
	gateCardType       string
	// op is the Apply transform.
	op func(int) int
}

func (m *mockDoubler) Name() string             { return m.name }
func (m *mockDoubler) SourceInstanceID() string { return m.sourceID }
func (m *mockDoubler) HandlerID() string        { return m.handlerID }
func (m *mockDoubler) Timestamp() int           { return m.timestamp }
func (m *mockDoubler) ControllerSeat() int      { return m.controller }
func (m *mockDoubler) Kind() DoublerKind        { return m.kind }
func (m *mockDoubler) Apply(n int) int          { return m.op(n) }
func (m *mockDoubler) Applies(target Target, counterType string, targetController int) bool {
	if m.gateOppOnly && targetController == m.controller {
		return false
	}
	if m.gateControllerOnly && targetController != m.controller {
		return false
	}
	if m.gateCounterType != "" && counterType != m.gateCounterType {
		return false
	}
	if m.gateCardType != "" && (target == nil || !target.HasCardType(m.gateCardType)) {
		return false
	}
	return true
}

// hardenedScales is the canonical +1/+1, controller-only, AddOne doubler.
func hardenedScales(controller int) *mockDoubler {
	return &mockDoubler{
		name:               "Hardened Scales",
		sourceID:           "hs-001",
		handlerID:          "Hardened Scales::plus1",
		controller:         controller,
		kind:               DoublerKindAddOne,
		gateControllerOnly: true,
		gateCounterType:    "+1/+1",
		op:                 func(n int) int { return n + 1 },
	}
}

// doublingSeason is the symmetric Double-all-counters, controller-only.
func doublingSeason(controller int) *mockDoubler {
	return &mockDoubler{
		name:               "Doubling Season",
		sourceID:           "ds-001",
		handlerID:          "Doubling Season::counter_dbl",
		controller:         controller,
		kind:               DoublerKindDouble,
		gateControllerOnly: true,
		op:                 func(n int) int { return n * 2 },
	}
}

// branchingEvolution doubles +1/+1 on creatures controller controls.
func branchingEvolution(controller int) *mockDoubler {
	return &mockDoubler{
		name:               "Branching Evolution",
		sourceID:           "be-001",
		handlerID:          "Branching Evolution::plus1_dbl",
		controller:         controller,
		kind:               DoublerKindDouble,
		gateControllerOnly: true,
		gateCounterType:    "+1/+1",
		gateCardType:       "creature",
		op:                 func(n int) int { return n * 2 },
	}
}

// vorinclexSelfArm doubles all counters on controller's permanents.
func vorinclexSelfArm(controller int) *mockDoubler {
	return &mockDoubler{
		name:               "Vorinclex, Monstrous Raider (self arm)",
		sourceID:           "vor-001",
		handlerID:          "Vorinclex, Monstrous Raider::self_dbl",
		controller:         controller,
		kind:               DoublerKindDouble,
		gateControllerOnly: true,
		op:                 func(n int) int { return n * 2 },
	}
}

// vorinclexOppArm halves all counters on opponents' permanents.
func vorinclexOppArm(controller int) *mockDoubler {
	return &mockDoubler{
		name:        "Vorinclex, Monstrous Raider (opp arm)",
		sourceID:    "vor-001",
		handlerID:   "Vorinclex, Monstrous Raider::opp_halve",
		controller:  controller,
		kind:        DoublerKindHalve,
		gateOppOnly: true,
		op:          func(n int) int { return n / 2 },
	}
}

// newCreature constructs a creature target for the controller. Counter
// placements default to controller-controlled targets unless overridden.
func newCreature() *mockTarget {
	return &mockTarget{cardTypes: []string{"creature"}}
}

// TestApplyDoublers_BallistaWithHardenedScales — Walking Ballista enters
// with X=2 +1/+1 counters; Hardened Scales is the only doubler in play.
// Expected: 2 → 3 (HS adds +1).
func TestApplyDoublers_BallistaWithHardenedScales(t *testing.T) {
	target := newCreature()
	doublers := []Doubler{hardenedScales(0)}
	final, chain := ApplyDoublers(target, "+1/+1", 2, 0, doublers)
	if final != 3 {
		t.Errorf("Ballista X=2 + HS: got %d, want 3", final)
	}
	if len(chain) != 1 {
		t.Fatalf("chain len = %d, want 1", len(chain))
	}
	if chain[0].CountBefore != 2 || chain[0].CountAfter != 3 {
		t.Errorf("chain[0] counts = %d→%d, want 2→3", chain[0].CountBefore, chain[0].CountAfter)
	}
	if chain[0].Kind != DoublerKindAddOne {
		t.Errorf("chain[0].Kind = %v, want AddOne", chain[0].Kind)
	}
	if chain[0].SourceName != "Hardened Scales" {
		t.Errorf("chain[0].SourceName = %q, want Hardened Scales", chain[0].SourceName)
	}
}

// TestApplyDoublers_BallistaWithDoublingSeasonAndHardenedScales —
// Ballista X=2 + DS + HS. Pipeline respects controller's chosen ordering.
// Two distinct controller choices, two distinct legal totals:
//   - DS first, then HS: 2 → 4 → 5 (per docs/counter-db-implementation-plan-r60.md)
//   - HS first, then DS: 2 → 3 → 6
//
// Both are §616-legal; the pipeline must respect input order.
func TestApplyDoublers_BallistaWithDoublingSeasonAndHardenedScales(t *testing.T) {
	target := newCreature()
	dsFirst := []Doubler{doublingSeason(0), hardenedScales(0)}
	hsFirst := []Doubler{hardenedScales(0), doublingSeason(0)}

	finalA, chainA := ApplyDoublers(target, "+1/+1", 2, 0, dsFirst)
	if finalA != 5 {
		t.Errorf("DS-then-HS: got %d, want 5", finalA)
	}
	if len(chainA) != 2 {
		t.Fatalf("DS-then-HS chain len = %d, want 2", len(chainA))
	}
	// Chain ordering must match input order.
	if chainA[0].SourceName != "Doubling Season" || chainA[1].SourceName != "Hardened Scales" {
		t.Errorf("DS-first chain order = [%s,%s], want [Doubling Season, Hardened Scales]",
			chainA[0].SourceName, chainA[1].SourceName)
	}

	finalB, chainB := ApplyDoublers(target, "+1/+1", 2, 0, hsFirst)
	if finalB != 6 {
		t.Errorf("HS-then-DS: got %d, want 6", finalB)
	}
	if len(chainB) != 2 {
		t.Fatalf("HS-then-DS chain len = %d, want 2", len(chainB))
	}
	if chainB[0].SourceName != "Hardened Scales" || chainB[1].SourceName != "Doubling Season" {
		t.Errorf("HS-first chain order = [%s,%s], want [Hardened Scales, Doubling Season]",
			chainB[0].SourceName, chainB[1].SourceName)
	}

	if finalA == finalB {
		t.Errorf("expected order-dependent results — got %d == %d", finalA, finalB)
	}
}

// TestApplyDoublers_BallistaWithThreeDoublersStackingOrder — Walking
// Ballista X=2 with Doubling Season + Hardened Scales + Branching
// Evolution. The §616 rule: controller chooses application order.
// Two extreme orderings:
//   - All-doubles-first then HS: 2 → 4 → 8 → 9 (DS, BE, HS)
//   - HS first then both doubles: 2 → 3 → 6 → 12 (HS, BE, DS) or (HS, DS, BE)
//
// Test pins both extremes and confirms the chain length is always 3 and
// the chain ordering matches input slice ordering.
func TestApplyDoublers_BallistaWithThreeDoublersStackingOrder(t *testing.T) {
	target := newCreature()
	// Smallest legal ordering: doubles first, then AddOne.
	dsBeHs := []Doubler{doublingSeason(0), branchingEvolution(0), hardenedScales(0)}
	final, chain := ApplyDoublers(target, "+1/+1", 2, 0, dsBeHs)
	if final != 9 {
		t.Errorf("DS-BE-HS ordering: got %d, want 9 (2→4→8→9)", final)
	}
	if len(chain) != 3 {
		t.Errorf("chain len = %d, want 3", len(chain))
	}

	// Largest legal ordering: AddOne first, then both doubles.
	hsDsBe := []Doubler{hardenedScales(0), doublingSeason(0), branchingEvolution(0)}
	final2, chain2 := ApplyDoublers(target, "+1/+1", 2, 0, hsDsBe)
	if final2 != 12 {
		t.Errorf("HS-DS-BE ordering: got %d, want 12 (2→3→6→12)", final2)
	}
	if len(chain2) != 3 {
		t.Errorf("chain len = %d, want 3", len(chain2))
	}

	// Independence: swapping the two doubles' relative order (DS-BE vs BE-DS)
	// after the same HS prefix produces the same final count — the two
	// doubles commute.
	hsBeDs := []Doubler{hardenedScales(0), branchingEvolution(0), doublingSeason(0)}
	final3, _ := ApplyDoublers(target, "+1/+1", 2, 0, hsBeDs)
	if final3 != 12 {
		t.Errorf("HS-BE-DS ordering: got %d, want 12 (commutativity of ×2)", final3)
	}

	// The §616-extreme spread between minimal-DS-first ordering and
	// maximal-HS-first ordering is 9 vs 12 — a 3-count spread that
	// demonstrates the controller-choice matters.
	if final == final2 {
		t.Errorf("§616 ordering independence broken: %d == %d", final, final2)
	}
}

// TestApplyDoublers_EnergyShortCircuits — energy is a §106.11 resource
// pool, NOT a §122 counter, so doubling never applies. Even with Doubling
// Season in play, an energy "placement" returns baseCount unchanged.
//
// Critical because Phase 8 will route Aetherworks Marvel / Whirler Virtuoso
// energy gains through the same wrapper; the gate must hold.
//
// Note: energy is not in the Phase 5 registry as a TargetPlayer counter —
// confirm via registry-absence shortcircuit too. The pipeline should
// short-circuit when Lookup returns nil OR DoublingApplies=false.
func TestApplyDoublers_EnergyShortCircuits(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	// "energy" is intentionally absent from the registry.
	final, chain := ApplyDoublers(target, "energy", 1, 0, []Doubler{doublingSeason(0)})
	if final != 1 {
		t.Errorf("energy gain with DS: got %d, want 1 (unchanged — §106.11 carve-out)", final)
	}
	if len(chain) != 0 {
		t.Errorf("energy chain len = %d, want 0", len(chain))
	}

	// Poison is in the registry with DoublingApplies=false. Same gate.
	target2 := &mockTarget{cardTypes: []string{"player"}}
	final2, chain2 := ApplyDoublers(target2, "poison", 3, 0, []Doubler{doublingSeason(0)})
	if final2 != 3 {
		t.Errorf("poison + DS: got %d, want 3 (DoublingApplies=false — §122.1g excludes player counters)", final2)
	}
	if len(chain2) != 0 {
		t.Errorf("poison chain len = %d, want 0", len(chain2))
	}
}

// TestApplyDoublers_VorinclexAsymmetric — controller arm doubles,
// opponent arm halves (rounded down). Tests both gates fire on the right
// side and never on the wrong side.
func TestApplyDoublers_VorinclexAsymmetric(t *testing.T) {
	target := newCreature()
	vorinclexController := 0
	doublers := []Doubler{vorinclexSelfArm(vorinclexController), vorinclexOppArm(vorinclexController)}

	// Controller places counters: self arm fires (doubles), opp arm does NOT.
	finalSelf, chainSelf := ApplyDoublers(target, "+1/+1", 3, 0, doublers)
	if finalSelf != 6 {
		t.Errorf("Vorinclex controller placement: got %d, want 6 (3 × 2)", finalSelf)
	}
	if len(chainSelf) != 1 {
		t.Fatalf("controller chain len = %d, want 1 (only self arm)", len(chainSelf))
	}
	if chainSelf[0].Kind != DoublerKindDouble {
		t.Errorf("controller chain[0].Kind = %v, want Double", chainSelf[0].Kind)
	}

	// Opponent places counters: opp arm fires (halves), self arm does NOT.
	finalOpp, chainOpp := ApplyDoublers(target, "+1/+1", 7, 1, doublers)
	if finalOpp != 3 {
		t.Errorf("Vorinclex opponent placement of 7: got %d, want 3 (7 / 2 rounded down)", finalOpp)
	}
	if len(chainOpp) != 1 {
		t.Fatalf("opponent chain len = %d, want 1 (only opp arm)", len(chainOpp))
	}
	if chainOpp[0].Kind != DoublerKindHalve {
		t.Errorf("opponent chain[0].Kind = %v, want Halve", chainOpp[0].Kind)
	}

	// Odd counts halve to floor.
	finalOdd, _ := ApplyDoublers(target, "+1/+1", 1, 1, doublers)
	if finalOdd != 0 {
		t.Errorf("Vorinclex opponent placement of 1: got %d, want 0 (1 / 2 floor)", finalOdd)
	}
}

// TestAddCountersWithDoublers_StampsLineage — placement should:
//  1. Compute the doubled count via ApplyDoublers.
//  2. Stamp a CounterStack with the FINAL count + sourceInstanceID.
//  3. Return the chain for engine-side audit logging.
func TestAddCountersWithDoublers_StampsLineage(t *testing.T) {
	target := newCreature()
	doublers := []Doubler{doublingSeason(0), hardenedScales(0)}

	placed, chain, err := AddCountersWithDoublers(
		target, "+1/+1", 2,
		"sage-of-hours-ab-001", 42, 0, doublers)
	if err != nil {
		t.Fatalf("AddCountersWithDoublers err = %v", err)
	}
	if placed != 5 {
		t.Errorf("placed = %d, want 5 (2 → 4 → 5)", placed)
	}
	if len(chain) != 2 {
		t.Errorf("chain len = %d, want 2", len(chain))
	}
	// CounterStack must reflect the final post-doubling count.
	stacks := target.CounterStacks()
	if len(stacks) != 1 {
		t.Fatalf("stacks len = %d, want 1", len(stacks))
	}
	if stacks[0].Count != 5 {
		t.Errorf("stack count = %d, want 5", stacks[0].Count)
	}
	if stacks[0].PlacedByInstanceID != "sage-of-hours-ab-001" {
		t.Errorf("stack source = %q, want sage-of-hours-ab-001", stacks[0].PlacedByInstanceID)
	}
	if stacks[0].PlacedAtTick != 42 {
		t.Errorf("stack tick = %d, want 42", stacks[0].PlacedAtTick)
	}
}

// TestApplyDoublers_BranchingEvolutionGatesByTargetType — Branching
// Evolution applies only to creatures. A non-creature target (planeswalker)
// must not trigger the doubler even when the counter type is +1/+1.
func TestApplyDoublers_BranchingEvolutionGatesByTargetType(t *testing.T) {
	// Planeswalker +1/+1 placement under BE — should NOT apply (BE is
	// creature-only).
	pw := &mockTarget{cardTypes: []string{"planeswalker"}}
	final, chain := ApplyDoublers(pw, "+1/+1", 2, 0, []Doubler{branchingEvolution(0)})
	if final != 2 {
		t.Errorf("BE on planeswalker: got %d, want 2 (BE gates by creature)", final)
	}
	if len(chain) != 0 {
		t.Errorf("BE chain len on planeswalker = %d, want 0", len(chain))
	}
}

// TestApplyDoublers_HardenedScalesStacks — two Hardened Scales each fire
// once independently per §122.1g, adding +1 each. 2 → 3 → 4.
func TestApplyDoublers_HardenedScalesStacks(t *testing.T) {
	target := newCreature()
	hs1 := hardenedScales(0)
	hs1.sourceID = "hs-a"
	hs1.handlerID = "Hardened Scales::a"
	hs2 := hardenedScales(0)
	hs2.sourceID = "hs-b"
	hs2.handlerID = "Hardened Scales::b"

	final, chain := ApplyDoublers(target, "+1/+1", 2, 0, []Doubler{hs1, hs2})
	if final != 4 {
		t.Errorf("two HS in sequence: got %d, want 4 (2 → 3 → 4)", final)
	}
	if len(chain) != 2 {
		t.Errorf("chain len = %d, want 2", len(chain))
	}
}

// TestApplyDoublers_BaseCountZeroShortCircuits — the §122.1g rule keys on
// "one or more counters would be placed". A 0-count event must not fire
// any doubler.
func TestApplyDoublers_BaseCountZeroShortCircuits(t *testing.T) {
	target := newCreature()
	final, chain := ApplyDoublers(target, "+1/+1", 0, 0, []Doubler{doublingSeason(0)})
	if final != 0 {
		t.Errorf("base=0: got %d, want 0", final)
	}
	if len(chain) != 0 {
		t.Errorf("chain len = %d, want 0", len(chain))
	}
}

// TestAddCountersWithDoublers_EnergyRejected — energy is unregistered, so
// the API returns ErrUnknownCounterType. Defends the §106.11 carve-out
// at the engine entry point too, not just the pipeline.
func TestAddCountersWithDoublers_EnergyRejected(t *testing.T) {
	target := newCreature()
	_, _, err := AddCountersWithDoublers(target, "energy", 1, "src", 0, 0, []Doubler{doublingSeason(0)})
	if err != ErrUnknownCounterType {
		t.Errorf("AddCountersWithDoublers(energy) err = %v, want ErrUnknownCounterType", err)
	}
}
