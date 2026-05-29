package counters

import "testing"

// keywordCounterPhase2Set enumerates the 12 keyword counters registered
// in Phase 2 (one Phase 1 carryover + 11 new). Pin order is canonical
// names; aliases are exercised separately.
var keywordCounterPhase2Set = []string{
	"lifelink",
	"deathtouch",
	"flying",
	"first strike",
	"double strike",
	"hexproof",
	"indestructible",
	"menace",
	"reach",
	"trample",
	"vigilance",
	"ward",
}

// TestPhase2KeywordCountersRegistered pins that every counter in the
// Phase 2 keyword set is in the registry and shaped correctly: Category
// is KeywordGrant, the GrantedAbility matches the counter name, the
// only ValidTarget is creature, doubling + proliferate are on, and the
// stacking behavior is NoPair (keyword counters don't cancel — that's
// only the §704.5r +1/+1 vs -1/-1 family).
func TestPhase2KeywordCountersRegistered(t *testing.T) {
	for _, name := range keywordCounterPhase2Set {
		def := Lookup(name)
		if def == nil {
			t.Errorf("registry missing Phase 2 keyword counter %q", name)
			continue
		}
		if def.Category != KeywordGrant {
			t.Errorf("%q Category = %v, want KeywordGrant", name, def.Category)
		}
		if def.GrantedAbility == nil || def.GrantedAbility.Keyword != name {
			t.Errorf("%q GrantedAbility = %+v, want Keyword=%q", name, def.GrantedAbility, name)
		}
		if len(def.ValidTargets) != 1 || def.ValidTargets[0] != TargetCreature {
			t.Errorf("%q ValidTargets = %v, want [creature]", name, def.ValidTargets)
		}
		if !def.DoublingApplies {
			t.Errorf("%q DoublingApplies = false, want true", name)
		}
		if !def.Proliferate {
			t.Errorf("%q Proliferate = false, want true", name)
		}
		if def.StackingBehavior.PairsWith != "" {
			t.Errorf("%q StackingBehavior.PairsWith = %q, want empty (NoPair)", name, def.StackingBehavior.PairsWith)
		}
		if def.Notes == "" {
			t.Errorf("%q Notes empty — every keyword-counter entry must cite §122.1c + the keyword's CR §", name)
		}
	}
}

// TestKeywordCounterAliasesResolve pins that the dashed / underscored
// alias forms still find the canonical entry. Engine call sites use
// "first strike" (with a space) but parser-emitted counter names can
// carry either form depending on token source.
func TestKeywordCounterAliasesResolve(t *testing.T) {
	cases := map[string]string{
		"first-strike":  "first strike",
		"first_strike":  "first strike",
		"double-strike": "double strike",
		"double_strike": "double strike",
	}
	for alias, want := range cases {
		def := Lookup(alias)
		if def == nil || def.Name != want {
			t.Errorf("Lookup(%q) = %+v, want canonical %q", alias, def, want)
		}
		if got := CanonicalName(alias); got != want {
			t.Errorf("CanonicalName(%q) = %q, want %q", alias, got, want)
		}
	}
}

// TestHasKeywordCounterEveryKeyword exercises the §122.1c grant
// predicate across every Phase 2 keyword counter. For each: place one
// counter via AddCounters, assert HasKeywordCounter returns true; remove
// the counter, assert it returns false. This is the canonical "the
// counter is the keyword" property — each invocation independently
// confirms the registry → API → predicate path is wired for that
// keyword.
func TestHasKeywordCounterEveryKeyword(t *testing.T) {
	for _, kw := range keywordCounterPhase2Set {
		target := &mockTarget{cardTypes: []string{"creature"}}
		if got := HasKeywordCounter(target, kw); got {
			t.Errorf("%s: HasKeywordCounter on bare creature = true, want false", kw)
		}
		placed, err := AddCounters(target, kw, 1, "h-test-source", 0)
		if err != nil || placed != 1 {
			t.Fatalf("%s: AddCounters returned (%d, %v), want (1, nil)", kw, placed, err)
		}
		if got := HasKeywordCounter(target, kw); !got {
			t.Errorf("%s: HasKeywordCounter after placement = false, want true", kw)
		}
		removed := RemoveCounters(target, kw, 1)
		if removed != 1 {
			t.Fatalf("%s: RemoveCounters returned %d, want 1", kw, removed)
		}
		if got := HasKeywordCounter(target, kw); got {
			t.Errorf("%s: HasKeywordCounter after removal = true, want false", kw)
		}
	}
}

// TestHasKeywordCounterRejectsNonKeywordGrant pins the safety gate that
// keeps HasKeyword chokepoints from false-positiving when stat-modifier
// or resource-marker counters happen to share a name with a keyword.
// "+1/+1" is the canonical risk case — without the KeywordGrant gate, a
// future keyword called "+1/+1" (vanishingly unlikely but cheap to
// defend) would let any creature with a +1/+1 counter satisfy the
// HasKeyword check.
func TestHasKeywordCounterRejectsNonKeywordGrant(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(target, "+1/+1", 3, "h-pump", 0); err != nil {
		t.Fatalf("AddCounters(+1/+1): %v", err)
	}
	if HasKeywordCounter(target, "+1/+1") {
		t.Errorf("HasKeywordCounter(+1/+1) = true, want false (StatModifier, not KeywordGrant)")
	}
	if _, err := AddCounters(target, "stun", 1, "h-stun", 0); err != nil {
		t.Fatalf("AddCounters(stun): %v", err)
	}
	if HasKeywordCounter(target, "stun") {
		t.Errorf("HasKeywordCounter(stun) = true, want false (ResourceMarker, not KeywordGrant)")
	}
}

// TestHasKeywordCounterRejectsUnknown pins the defensive nil-return for
// unregistered names. Engine HasKeyword fallback must not crash when an
// odd keyword string ("daybound", "intimidate") gets passed in.
func TestHasKeywordCounterRejectsUnknown(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if HasKeywordCounter(target, "daybound") {
		t.Errorf("HasKeywordCounter(daybound) = true, want false (unregistered)")
	}
	if HasKeywordCounter(target, "") {
		t.Errorf("HasKeywordCounter(empty) = true, want false")
	}
	if HasKeywordCounter(nil, "flying") {
		t.Errorf("HasKeywordCounter(nil target) = true, want false")
	}
}

// TestKeywordCounterPersistsThroughTypeStrip pins §122.6 for the
// keyword-counter family specifically. A Humility-style Layer-4 strip
// clears the card type; HasKeywordCounter must still report true
// because the §122.6 rule says counters remain even when the
// permanent's characteristics change.
func TestKeywordCounterPersistsThroughTypeStrip(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(target, "lifelink", 1, "h-bonded", 9); err != nil {
		t.Fatalf("AddCounters(lifelink): %v", err)
	}
	if !HasKeywordCounter(target, "lifelink") {
		t.Fatal("baseline HasKeywordCounter(lifelink) = false")
	}
	target.cardTypes = nil // simulate Humility / Mycosynth Lattice strip
	if !HasKeywordCounter(target, "lifelink") {
		t.Errorf("post-strip HasKeywordCounter(lifelink) = false, want true (§122.6)")
	}
	if CounterCount(target, "lifelink") != 1 {
		t.Errorf("post-strip CounterCount(lifelink) = %d, want 1", CounterCount(target, "lifelink"))
	}
}

// TestAllReturnsPhase2Count pins the registry-size invariant: Phase 1
// (10) + Phase 2 (11 keyword counters; lifelink was Phase 1) + Phase 4
// (3 player-counter types) = 24 baseline entries. Phase 5 added the
// long-tail (~230 types from the Probe F catalog), so the assertion is
// a floor — the spot-check below pins the Phase 2 keyword counters
// remain present after subsequent phases extend the registry.
func TestAllReturnsPhase2Count(t *testing.T) {
	all := All()
	if len(all) < 24 {
		t.Errorf("All() returned %d entries, want at least 24 (10 Phase 1 + 11 Phase 2 keyword counters + 3 Phase 4 player counters)", len(all))
	}
	// Spot-check every Phase 2 addition is present.
	names := map[string]bool{}
	for _, def := range all {
		names[def.Name] = true
	}
	for _, want := range keywordCounterPhase2Set {
		if !names[want] {
			t.Errorf("All() missing Phase 2 keyword counter %q", want)
		}
	}
}

// TestHasKeywordCounterDrainedStack pins the boundary where a stack's
// Count drops to 0 via RemoveCounters but the stack entry briefly
// remains (Phase 1 RemoveCounters drops the entry once Count hits 0; we
// also explicitly defend against a future change that leaves an empty
// stack stub by gating on Count > 0 in the predicate).
func TestHasKeywordCounterDrainedStack(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "flying", 2, "h-a", 0)
	if !HasKeywordCounter(target, "flying") {
		t.Fatal("baseline flying = false")
	}
	// Drain the full stack.
	if removed := RemoveCounters(target, "flying", 2); removed != 2 {
		t.Fatalf("removed=%d, want 2", removed)
	}
	if HasKeywordCounter(target, "flying") {
		t.Errorf("HasKeywordCounter(flying) after full drain = true, want false")
	}
	// Manually inject an empty-count stack to defend the > 0 guard.
	target.SetCounterStacks([]CounterStack{{Type: "flying", Count: 0}})
	if HasKeywordCounter(target, "flying") {
		t.Errorf("HasKeywordCounter(flying) with Count=0 stack = true, want false")
	}
}

// TestHasKeywordCounterMultipleStacksAggregate pins that multiple stacks
// of the same keyword counter (from different placement sources) all
// satisfy the predicate — engine should report true even if no single
// stack has the full count.
func TestHasKeywordCounterMultipleStacksAggregate(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "trample", 1, "h-equipped", 1)
	_, _ = AddCounters(target, "trample", 1, "h-spell", 5)
	stacks := target.CounterStacks()
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks (different sources), got %d", len(stacks))
	}
	if !HasKeywordCounter(target, "trample") {
		t.Errorf("HasKeywordCounter(trample) with two single-count stacks = false, want true")
	}
}
