package counters

import (
	"strings"
	"testing"
)

// mockTarget is the test stand-in for a *gameengine.Permanent. Phase 1
// keeps the counters package gameengine-independent — wiring through a
// real Permanent lands in Phase 2.
type mockTarget struct {
	cardTypes []string
	subtypes  []string
	stacks    []CounterStack
}

func (m *mockTarget) HasCardType(t string) bool {
	for _, ct := range m.cardTypes {
		if strings.EqualFold(ct, t) {
			return true
		}
	}
	return false
}

func (m *mockTarget) HasSubtype(t string) bool {
	for _, st := range m.subtypes {
		if strings.EqualFold(st, t) {
			return true
		}
	}
	return false
}

func (m *mockTarget) CounterStacks() []CounterStack { return m.stacks }
func (m *mockTarget) SetCounterStacks(s []CounterStack) {
	m.stacks = s
}

// TestRegistryLookupByName pins the 10 Phase 1 types are present and
// retrievable by their canonical names.
func TestRegistryLookupByName(t *testing.T) {
	phase1Types := []string{
		"+1/+1", "-1/-1", "lifelink", "charge", "loyalty",
		"lore", "time", "oil", "stun", "shield",
	}
	for _, name := range phase1Types {
		def := Lookup(name)
		if def == nil {
			t.Errorf("registry missing Phase 1 type %q", name)
			continue
		}
		if def.Name != name {
			t.Errorf("Lookup(%q).Name = %q, want %q", name, def.Name, name)
		}
		if def.Notes == "" {
			t.Errorf("Lookup(%q) has empty Notes — every registry entry must carry CR §-citation", name)
		}
		if len(def.ValidTargets) == 0 {
			t.Errorf("Lookup(%q) has no ValidTargets", name)
		}
	}
}

// TestRegistryAliasLookup pins +1/+1 resolves through its aliases.
func TestRegistryAliasLookup(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"plus-one", "+1/+1"},
		{"+1+1", "+1/+1"},
		{"minus-one", "-1/-1"},
	}
	for _, c := range cases {
		def := Lookup(c.input)
		if def == nil {
			t.Errorf("alias %q did not resolve", c.input)
			continue
		}
		if def.Name != c.want {
			t.Errorf("alias %q resolved to %q, want %q", c.input, def.Name, c.want)
		}
		if got := CanonicalName(c.input); got != c.want {
			t.Errorf("CanonicalName(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestRegistryUnknownTypeReturnsNil pins the defensive nil-return — the
// engine degrades gracefully on uncatalogued types rather than panicking.
func TestRegistryUnknownTypeReturnsNil(t *testing.T) {
	if def := Lookup("madeup-counter-type"); def != nil {
		t.Errorf("Lookup of unregistered type returned %+v, want nil", def)
	}
	if name := CanonicalName("madeup-counter-type"); name != "madeup-counter-type" {
		t.Errorf("CanonicalName passthrough: got %q, want input unchanged", name)
	}
}

// TestPlacementValidityPerValidTargets exercises the AcceptsTarget +
// AddCounters target-type gate. Lifelink on a creature is legal; on a
// land it is not.
func TestPlacementValidityPerValidTargets(t *testing.T) {
	creature := &mockTarget{cardTypes: []string{"creature"}}
	land := &mockTarget{cardTypes: []string{"land"}}

	if got, err := AddCounters(creature, "lifelink", 1, "hABC", 0); err != nil || got != 1 {
		t.Errorf("lifelink on creature: got (%d, %v), want (1, nil)", got, err)
	}
	if got, err := AddCounters(land, "lifelink", 1, "hABC", 0); err != ErrInvalidTarget || got != 0 {
		t.Errorf("lifelink on land: got (%d, %v), want (0, ErrInvalidTarget)", got, err)
	}

	// Saga lore counters require subtype match, not card-type match.
	saga := &mockTarget{cardTypes: []string{"enchantment"}, subtypes: []string{"saga"}}
	enchantment := &mockTarget{cardTypes: []string{"enchantment"}}
	if got, err := AddCounters(saga, "lore", 1, "hABC", 0); err != nil || got != 1 {
		t.Errorf("lore on saga: got (%d, %v), want (1, nil)", got, err)
	}
	if _, err := AddCounters(enchantment, "lore", 1, "hABC", 0); err != ErrInvalidTarget {
		t.Errorf("lore on non-saga enchantment: got err %v, want ErrInvalidTarget", err)
	}
}

// TestUnknownTypeErrorsCleanly pins AddCounters surfaces the registry
// miss as an error.
func TestUnknownTypeErrorsCleanly(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if got, err := AddCounters(target, "no-such-type", 1, "hABC", 0); err != ErrUnknownCounterType || got != 0 {
		t.Errorf("unknown type: got (%d, %v), want (0, ErrUnknownCounterType)", got, err)
	}
}

// TestZeroOrNegativeCountIsNoOp matches the engine's convention.
func TestZeroOrNegativeCountIsNoOp(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if got, err := AddCounters(target, "+1/+1", 0, "hABC", 0); got != 0 || err != nil {
		t.Errorf("count=0: got (%d, %v), want (0, nil)", got, err)
	}
	if got, err := AddCounters(target, "+1/+1", -5, "hABC", 0); got != 0 || err != nil {
		t.Errorf("count=-5: got (%d, %v), want (0, nil)", got, err)
	}
	if len(target.stacks) != 0 {
		t.Errorf("no-op AddCounters left %d stacks, want 0", len(target.stacks))
	}
	if removed := RemoveCounters(target, "+1/+1", 0); removed != 0 {
		t.Errorf("RemoveCounters count=0: got %d, want 0", removed)
	}
}

// TestPlacedByInstanceIDLineageRecorded pins that the source InstanceID
// is captured on every placement — the forensic chain backing future
// Loki invariants.
func TestPlacedByInstanceIDLineageRecorded(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(target, "+1/+1", 3, "h1AB-source-id", 42); err != nil {
		t.Fatalf("AddCounters: %v", err)
	}
	stacks := target.CounterStacks()
	if len(stacks) != 1 {
		t.Fatalf("got %d stacks, want 1", len(stacks))
	}
	if stacks[0].PlacedByInstanceID != "h1AB-source-id" {
		t.Errorf("PlacedByInstanceID = %q, want %q", stacks[0].PlacedByInstanceID, "h1AB-source-id")
	}
	if stacks[0].PlacedAtTick != 42 {
		t.Errorf("PlacedAtTick = %d, want 42", stacks[0].PlacedAtTick)
	}
	if stacks[0].Count != 3 {
		t.Errorf("Count = %d, want 3", stacks[0].Count)
	}
}

// TestSameSourceAggregates pins the Phase 1 merge policy: two placements
// from the same source on the same type fold into a single stack.
func TestSameSourceAggregates(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "+1/+1", 2, "hSRC", 1)
	_, _ = AddCounters(target, "+1/+1", 3, "hSRC", 5)
	stacks := target.CounterStacks()
	if len(stacks) != 1 {
		t.Fatalf("got %d stacks, want 1 (merged)", len(stacks))
	}
	if stacks[0].Count != 5 {
		t.Errorf("merged Count = %d, want 5", stacks[0].Count)
	}
	if stacks[0].PlacedAtTick != 5 {
		t.Errorf("merged PlacedAtTick = %d, want 5 (most recent)", stacks[0].PlacedAtTick)
	}
}

// TestDifferentSourcesKeepSeparateStacks pins the lineage-preservation
// arm: counters from different abilities stay in separate stacks.
func TestDifferentSourcesKeepSeparateStacks(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "+1/+1", 2, "hSAGE", 1)
	_, _ = AddCounters(target, "+1/+1", 3, "hOZOLITH", 5)
	stacks := target.CounterStacks()
	if len(stacks) != 2 {
		t.Fatalf("got %d stacks, want 2 (per-source)", len(stacks))
	}
	if CounterCount(target, "+1/+1") != 5 {
		t.Errorf("aggregated CounterCount = %d, want 5", CounterCount(target, "+1/+1"))
	}
}

// TestRemoveCountersOldestFirst pins §122.3 — removal drains the oldest
// insertion-order stack first.
func TestRemoveCountersOldestFirst(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "+1/+1", 2, "hOLD", 1)
	_, _ = AddCounters(target, "+1/+1", 3, "hNEW", 5)

	if removed := RemoveCounters(target, "+1/+1", 1); removed != 1 {
		t.Errorf("removed=%d, want 1", removed)
	}
	stacks := target.CounterStacks()
	if len(stacks) != 2 {
		t.Fatalf("partial drain: got %d stacks, want 2", len(stacks))
	}
	if stacks[0].PlacedByInstanceID != "hOLD" || stacks[0].Count != 1 {
		t.Errorf("oldest stack post-drain: %+v, want hOLD/1", stacks[0])
	}
	if stacks[1].PlacedByInstanceID != "hNEW" || stacks[1].Count != 3 {
		t.Errorf("newest stack post-drain: %+v, want hNEW/3", stacks[1])
	}

	// Drain past the oldest stack entirely.
	if removed := RemoveCounters(target, "+1/+1", 2); removed != 2 {
		t.Errorf("second drain removed=%d, want 2", removed)
	}
	stacks = target.CounterStacks()
	if len(stacks) != 1 || stacks[0].PlacedByInstanceID != "hNEW" || stacks[0].Count != 2 {
		t.Errorf("post-second-drain stacks: %+v, want [hNEW/2]", stacks)
	}

	// Drain more than remaining: returns actual amount.
	if removed := RemoveCounters(target, "+1/+1", 99); removed != 2 {
		t.Errorf("over-drain: got %d, want 2", removed)
	}
	if len(target.CounterStacks()) != 0 {
		t.Errorf("post-over-drain stacks: %d, want 0", len(target.CounterStacks()))
	}
}

// TestRemoveCountersIgnoresOtherTypes pins type-scoped removal.
func TestRemoveCountersIgnoresOtherTypes(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	_, _ = AddCounters(target, "+1/+1", 3, "hP", 1)
	_, _ = AddCounters(target, "-1/-1", 2, "hM", 2)

	if removed := RemoveCounters(target, "+1/+1", 5); removed != 3 {
		t.Errorf("plus-drain returned %d, want 3", removed)
	}
	if CounterCount(target, "-1/-1") != 2 {
		t.Errorf("minus untouched: got %d, want 2", CounterCount(target, "-1/-1"))
	}
}

// TestCountersPersistThroughTypeStrip pins the §122.6 invariant: a
// Layer-4 type-strip event MUST NOT alter the CounterStacks slice. The
// mock target's type-strip simulation clears cardTypes; we verify the
// slice is unchanged and that HasKeywordCounter-style readback still
// works once the Phase 2 helper lands.
func TestCountersPersistThroughTypeStrip(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if _, err := AddCounters(target, "lifelink", 1, "hCounter", 7); err != nil {
		t.Fatalf("AddCounters: %v", err)
	}
	pre := append([]CounterStack(nil), target.CounterStacks()...)

	// Layer-4 type-strip simulation — Humility / Mycosynth Lattice clears
	// the type line. The counter slice must be left alone.
	target.cardTypes = nil

	post := target.CounterStacks()
	if len(post) != len(pre) {
		t.Fatalf("type-strip changed slice length: pre=%d post=%d", len(pre), len(post))
	}
	for i := range pre {
		if pre[i] != post[i] {
			t.Errorf("type-strip mutated stack[%d]: pre=%+v post=%+v", i, pre[i], post[i])
		}
	}
	if CounterCount(target, "lifelink") != 1 {
		t.Errorf("post-type-strip CounterCount(lifelink) = %d, want 1", CounterCount(target, "lifelink"))
	}
}

// TestApplyDoublingPipelinePlaceholder pins the Phase 1 identity behavior.
// Phase 6 will replace this implementation with the real replacement-effect
// walk; the test should fail then and get updated to assert real doubling.
func TestApplyDoublingPipelinePlaceholder(t *testing.T) {
	target := &mockTarget{cardTypes: []string{"creature"}}
	if got := ApplyDoublingPipeline(target, "+1/+1", 3); got != 3 {
		t.Errorf("Phase 1 placeholder: got %d, want 3 (identity)", got)
	}
	// Non-doubling types short-circuit regardless of pipeline state.
	if got := ApplyDoublingPipeline(target, "lore", 3); got != 3 {
		t.Errorf("lore (DoublingApplies=false): got %d, want 3", got)
	}
	if got := ApplyDoublingPipeline(target, "unknown-type", 3); got != 3 {
		t.Errorf("unknown type: got %d, want 3", got)
	}
}

// TestRegistryStackingBehaviorPairs pins the §704.5r pair-removal config
// metadata for Phase 3 to read.
func TestRegistryStackingBehaviorPairs(t *testing.T) {
	plus := Lookup("+1/+1")
	if plus.StackingBehavior.PairsWith != "-1/-1" {
		t.Errorf("+1/+1 StackingBehavior.PairsWith = %q, want -1/-1", plus.StackingBehavior.PairsWith)
	}
	minus := Lookup("-1/-1")
	if minus.StackingBehavior.PairsWith != "+1/+1" {
		t.Errorf("-1/-1 StackingBehavior.PairsWith = %q, want +1/+1", minus.StackingBehavior.PairsWith)
	}
	if lifelink := Lookup("lifelink"); lifelink.StackingBehavior.PairsWith != "" {
		t.Errorf("lifelink StackingBehavior.PairsWith = %q, want empty (NoPair)", lifelink.StackingBehavior.PairsWith)
	}
}

// TestRegistryKeywordCounterGrantsAbility pins §122.1c metadata.
func TestRegistryKeywordCounterGrantsAbility(t *testing.T) {
	lifelink := Lookup("lifelink")
	if lifelink.GrantedAbility == nil || lifelink.GrantedAbility.Keyword != "lifelink" {
		t.Errorf("lifelink GrantedAbility = %+v, want Keyword=lifelink", lifelink.GrantedAbility)
	}
	if plus := Lookup("+1/+1"); plus.GrantedAbility != nil {
		t.Errorf("+1/+1 GrantedAbility = %+v, want nil (StatModifier, not KeywordGrant)", plus.GrantedAbility)
	}
}

// TestPlacementRuleAllows pins the bitmask helper.
func TestPlacementRuleAllows(t *testing.T) {
	plus := Lookup("+1/+1")
	if !plus.Placement.Allows(PlaceProliferateOnly) {
		t.Errorf("+1/+1 should allow proliferate placement")
	}
	if !plus.Placement.Allows(PlaceEnterCondition) {
		t.Errorf("+1/+1 should allow ETB placement")
	}
	lore := Lookup("lore")
	if !lore.Placement.Allows(PlaceAutoUpkeep) {
		t.Errorf("lore should allow auto-upkeep placement")
	}
	if lore.Placement.Allows(PlaceEnterCondition) {
		t.Errorf("lore should not allow ETB placement (placed by Saga rules at main phase)")
	}
}

// TestCategoryStringEnumeration pins the serialized identifier surface.
// Heimdall replay UI keys off these strings.
func TestCategoryStringEnumeration(t *testing.T) {
	cases := map[CounterCategory]string{
		StatModifier:    "stat_modifier",
		KeywordGrant:    "keyword_grant",
		ResourceMarker:  "resource_marker",
		LoreCounter:     "lore_counter",
		LoyaltyCounter:  "loyalty_counter",
		TimeCounter:     "time_counter",
		OtherTracker:    "other_tracker",
		CategoryUnknown: "unknown",
	}
	for cat, want := range cases {
		if got := cat.String(); got != want {
			t.Errorf("Category(%d).String() = %q, want %q", cat, got, want)
		}
	}
}

// TestAllReturnsAllRegistered pins the iteration surface used by future
// registry validators (Loki invariant: every counter type referenced by
// an oracle text has a registry entry). The Phase 1 set is asserted by
// name; Phase 2+ extensions are tracked in their own per-phase tests
// (see keyword_grants_test.go's TestAllReturnsPhase2Count).
func TestAllReturnsAllRegistered(t *testing.T) {
	all := All()
	names := map[string]bool{}
	for _, def := range all {
		names[def.Name] = true
	}
	for _, want := range []string{"+1/+1", "-1/-1", "lifelink", "charge", "loyalty", "lore", "time", "oil", "stun", "shield"} {
		if !names[want] {
			t.Errorf("All() missing Phase 1 type %q", want)
		}
	}
}
