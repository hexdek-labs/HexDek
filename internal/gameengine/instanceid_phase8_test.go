package gameengine

import (
	"testing"

	gameast "github.com/hexdek/hexdek/internal/gameast"
)

// newPhase8GameState builds a 2-seat GameState for Phase 8 property
// tests. Mirrors newPhase2GameState — fresh Minter, no deck-load.
func newPhase8GameState(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, nil, nil)
	if gs == nil {
		t.Fatalf("NewGameState returned nil")
	}
	if gs.IIDMinter == nil {
		t.Fatalf("expected IIDMinter to be non-nil")
	}
	return gs
}

// makePhase8Creature builds a minimal *Card representing a Mutate creature
// (or any creature stand-in). Mints an OG InstanceID so MergedCards
// tracking and the unmerge walker have something to record.
func makePhase8Creature(gs *GameState, owner int, name string, power, toughness int) *Card {
	c := &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
		Colors:        []string{"G"},
		CMC:           3,
		AST: &gameast.CardAST{
			Name: name,
		},
	}
	MintOGInstanceID(gs, c)
	return c
}

// putOnBattlefield helper — appends a fresh Permanent wrapping c to the
// owner's battlefield, stamping Owner + Controller and assigning a
// timestamp.
func putOnBattlefield(gs *GameState, c *Card) *Permanent {
	gs.EffectTimestamp++
	p := &Permanent{
		Card:       c,
		Controller: c.Owner,
		Owner:      c.Owner,
		Timestamp:  gs.EffectTimestamp,
	}
	gs.Seats[c.Owner].Battlefield = append(gs.Seats[c.Owner].Battlefield, p)
	return p
}

// ----------------------------------------------------------------------
// Mutate stack lifecycle
// ----------------------------------------------------------------------

// TestPhase8_MutateStackThreeIDs pins that a Brokkos+Vadrok+Snapdax-shape
// stack carries 3 InstanceIDs in MergedCards on a single Permanent, with
// MergeKind=Mutate and TopCard tracking the top of the stack.
func TestPhase8_MutateStackThreeIDs(t *testing.T) {
	gs := newPhase8GameState(t)
	snapdax := makePhase8Creature(gs, 0, "Nethroi", 5, 5)
	vadrok := makePhase8Creature(gs, 0, "Vadrok", 3, 3)
	brokkos := makePhase8Creature(gs, 0, "Brokkos", 6, 6)

	// Snapdax enters first as the bottom of the stack.
	bottom := putOnBattlefield(gs, snapdax)
	if bottom.MergeKind != MergeNone {
		t.Fatalf("fresh permanent should have MergeKind=None, got %v", bottom.MergeKind)
	}

	// Vadrok mutates on top — survives, Snapdax slides under.
	mid := putOnBattlefield(gs, vadrok)
	ApplyMutate(gs, mid, bottom, true)
	// After ApplyMutate, mid is the surviving permanent. Snapdax bottom
	// was removed from the battlefield.
	if mid.MergeKind != MergeMutate {
		t.Fatalf("after first mutate: MergeKind want Mutate, got %v", mid.MergeKind)
	}
	if len(mid.MergedCards) != 2 {
		t.Fatalf("after first mutate: MergedCards want 2, got %d (%v)",
			len(mid.MergedCards), mid.MergedCards)
	}
	if mid.TopCard != vadrok {
		t.Fatalf("after first mutate: TopCard want Vadrok, got %v", topCardName(mid))
	}

	// Brokkos mutates on top — survives, prior stack inherits under.
	top := putOnBattlefield(gs, brokkos)
	ApplyMutate(gs, top, mid, true)
	if top.MergeKind != MergeMutate {
		t.Fatalf("after second mutate: MergeKind want Mutate, got %v", top.MergeKind)
	}
	if len(top.MergedCards) != 3 {
		t.Fatalf("after second mutate: MergedCards want 3, got %d (%v)",
			len(top.MergedCards), top.MergedCards)
	}
	if top.TopCard != brokkos {
		t.Fatalf("after second mutate: TopCard want Brokkos, got %v", topCardName(top))
	}
	// MergedCards should contain all three IDs distinctly.
	seen := map[string]bool{}
	for _, id := range top.MergedCards {
		seen[id] = true
	}
	for _, want := range []string{brokkos.InstanceID, vadrok.InstanceID, snapdax.InstanceID} {
		if !seen[want] {
			t.Fatalf("MergedCards missing %q (have %v)", want, top.MergedCards)
		}
	}
	// And only one Permanent on the battlefield.
	if got := len(gs.Seats[0].Battlefield); got != 1 {
		t.Fatalf("want 1 permanent on battlefield, got %d", got)
	}
}

// TestPhase8_MutateLeavePlayUnmergeToGraveyard pins §702.139d: when the
// merged permanent dies, each MergedCards constituent moves to graveyard
// individually with its InstanceID preserved.
func TestPhase8_MutateLeavePlayUnmergeToGraveyard(t *testing.T) {
	gs := newPhase8GameState(t)
	bottom := makePhase8Creature(gs, 0, "Snapdax", 4, 4)
	mid := makePhase8Creature(gs, 0, "Vadrok", 3, 3)
	top := makePhase8Creature(gs, 0, "Brokkos", 6, 6)

	pBottom := putOnBattlefield(gs, bottom)
	pMid := putOnBattlefield(gs, mid)
	ApplyMutate(gs, pMid, pBottom, true)
	pTop := putOnBattlefield(gs, top)
	ApplyMutate(gs, pTop, pMid, true)

	if len(pTop.MergedCards) != 3 {
		t.Fatalf("pre-death sanity: want 3 merged IDs, got %d", len(pTop.MergedCards))
	}

	// Kill the surviving permanent.
	DestroyPermanent(gs, pTop, nil)

	gy := gs.Seats[0].Graveyard
	if len(gy) < 3 {
		t.Fatalf("after destroy: want 3 cards in graveyard, got %d (%v)",
			len(gy), graveyardNames(gy))
	}
	// Every original *Card must show up, identified by InstanceID.
	gyIDs := map[string]bool{}
	for _, c := range gy {
		gyIDs[c.InstanceID] = true
	}
	for _, want := range []string{bottom.InstanceID, mid.InstanceID, top.InstanceID} {
		if !gyIDs[want] {
			t.Fatalf("graveyard missing InstanceID %q (have %v)", want, gyIDs)
		}
	}
	// The merged permanent itself is gone.
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("want 0 permanents post-destroy, got %d", len(gs.Seats[0].Battlefield))
	}
}

// ----------------------------------------------------------------------
// Meld unified shape
// ----------------------------------------------------------------------

// TestPhase8_MeldBrunaGiselaCreatesBriselaWithTwoIDs pins that the
// canonical Bruna+Gisela meld produces a Permanent with MergeKind=Meld
// and exactly 2 InstanceIDs in MergedCards.
func TestPhase8_MeldBrunaGiselaCreatesBriselaWithTwoIDs(t *testing.T) {
	gs := newPhase8GameState(t)
	bruna := makePhase8Creature(gs, 0, "Bruna, the Fading Light", 5, 7)
	gisela := makePhase8Creature(gs, 0, "Gisela, the Broken Blade", 4, 3)
	pBruna := putOnBattlefield(gs, bruna)
	pGisela := putOnBattlefield(gs, gisela)

	melded := Meld(gs, pBruna, pGisela)
	if melded == nil {
		t.Fatalf("Meld returned nil")
	}
	if melded.MergeKind != MergeMeld {
		t.Fatalf("melded.MergeKind want Meld, got %v", melded.MergeKind)
	}
	if len(melded.MergedCards) != 2 {
		t.Fatalf("melded.MergedCards want 2, got %d (%v)", len(melded.MergedCards), melded.MergedCards)
	}
	// Both component IDs present.
	want := map[string]bool{bruna.InstanceID: true, gisela.InstanceID: true}
	for _, id := range melded.MergedCards {
		if !want[id] {
			t.Fatalf("MergedCards has unexpected ID %q (want subset of %v)", id, want)
		}
	}
}

// TestPhase8_MeldCrossTypeLandInputHandled pins that Probe E's
// land-input meld surprise (e.g. Urza's land-based meld) routes through
// the same MergedCards/MergeKind plumbing. Two land components → meld
// result with MergedCards = [land1_id, land2_id].
func TestPhase8_MeldCrossTypeLandInputHandled(t *testing.T) {
	gs := newPhase8GameState(t)
	mountain := &Card{
		Name:  "Argoth, Sanctum of Nature",
		Owner: 0,
		Types: []string{"land"},
		Colors: nil,
		CMC:    0,
		AST:    &gameast.CardAST{Name: "Argoth"},
	}
	MintOGInstanceID(gs, mountain)
	other := &Card{
		Name:  "Titania, Voice of Gaea",
		Owner: 0,
		Types: []string{"creature"},
		Colors: []string{"G"},
		CMC:    4,
		BasePower:    3,
		BaseToughness: 4,
		AST: &gameast.CardAST{Name: "Titania"},
	}
	MintOGInstanceID(gs, other)
	pLand := putOnBattlefield(gs, mountain)
	pCreature := putOnBattlefield(gs, other)

	melded := Meld(gs, pLand, pCreature)
	if melded == nil {
		t.Fatalf("Meld returned nil for cross-type input")
	}
	if melded.MergeKind != MergeMeld {
		t.Fatalf("melded.MergeKind want Meld, got %v", melded.MergeKind)
	}
	if len(melded.MergedCards) != 2 {
		t.Fatalf("melded.MergedCards want 2 (cross-type), got %d", len(melded.MergedCards))
	}
}

// ----------------------------------------------------------------------
// Delayed-trigger pool
// ----------------------------------------------------------------------

// TestPhase8_DelayedAbilityPactStyleFiresOnUpkeep simulates a Pact of
// Negation owe-trigger: a one-shot delayed ability scheduled on
// "upkeep_begin" filtered by the controller's seat. Fires when an
// upkeep_begin event with matching seat is walked.
func TestPhase8_DelayedAbilityPactStyleFiresOnUpkeep(t *testing.T) {
	gs := newPhase8GameState(t)
	fired := 0
	ab := &AbilityInstance{
		InstanceID:       "ab-test-001",
		SourceInstanceID: "src-test",
		Controller:       0,
		DelayedUntil: &DelayedCondition{
			EventType: "upkeep_begin",
			EventFilter: func(ev *Event) bool {
				return ev != nil && ev.Seat == 0
			},
			OneShot: true,
		},
		TriggerMetadata: map[string]any{
			"effect_fn": func(gs *GameState, ab *AbilityInstance, ev *Event) {
				fired++
			},
		},
	}
	RegisterDelayedAbility(gs, ab)
	if len(gs.DelayedAbilityInstances) != 1 {
		t.Fatalf("pool size want 1, got %d", len(gs.DelayedAbilityInstances))
	}

	// Wrong seat — should not fire.
	FireDelayedAbilityPool(gs, &Event{Kind: "upkeep_begin", Seat: 1})
	if fired != 0 {
		t.Fatalf("delayed ability fired on wrong seat: %d", fired)
	}
	if len(gs.DelayedAbilityInstances) != 1 {
		t.Fatalf("pool drained on non-match")
	}

	// Right seat — fires and one-shot removes.
	FireDelayedAbilityPool(gs, &Event{Kind: "upkeep_begin", Seat: 0})
	if fired != 1 {
		t.Fatalf("delayed ability did not fire: %d", fired)
	}
	if len(gs.DelayedAbilityInstances) != 0 {
		t.Fatalf("one-shot pool entry not removed: %d remaining", len(gs.DelayedAbilityInstances))
	}
}

// TestPhase8_DelayedAbilitySourceDeathIndependence pins §112.7a: a pool
// entry's effect still fires after its source permanent is destroyed.
func TestPhase8_DelayedAbilitySourceDeathIndependence(t *testing.T) {
	gs := newPhase8GameState(t)
	src := makePhase8Creature(gs, 0, "Source Creature", 1, 1)
	pSrc := putOnBattlefield(gs, src)

	fired := 0
	ab := &AbilityInstance{
		InstanceID:       "ab-test-indep",
		SourceInstanceID: src.InstanceID,
		Controller:       0,
		DelayedUntil: &DelayedCondition{
			EventType: "end_step",
			OneShot:   true,
		},
		TriggerMetadata: map[string]any{
			"effect_fn": func(gs *GameState, ab *AbilityInstance, ev *Event) {
				fired++
			},
		},
	}
	RegisterDelayedAbility(gs, ab)

	// Kill the source.
	DestroyPermanent(gs, pSrc, nil)

	// Fire end_step — delayed ability should still fire.
	FireDelayedAbilityPool(gs, &Event{Kind: "end_step", Seat: 0})
	if fired != 1 {
		t.Fatalf("delayed ability did not survive source death: fired=%d", fired)
	}
}

// TestPhase8_DelayedAbilityExpiresWithoutMatch pins the expiry path:
// an entry whose DelayedExpiresAt has passed is dropped without firing.
func TestPhase8_DelayedAbilityExpiresWithoutMatch(t *testing.T) {
	gs := newPhase8GameState(t)
	gs.EffectTimestamp = 5
	fired := 0
	ab := &AbilityInstance{
		InstanceID: "ab-test-exp",
		Controller: 0,
		DelayedUntil: &DelayedCondition{
			EventType: "never_happens",
			OneShot:   true,
		},
		DelayedExpiresAt: 10,
		TriggerMetadata: map[string]any{
			"effect_fn": func(gs *GameState, ab *AbilityInstance, ev *Event) { fired++ },
		},
	}
	RegisterDelayedAbility(gs, ab)
	gs.EffectTimestamp = 11
	dropped := CleanupExpiredDelayedAbilities(gs)
	if dropped != 1 {
		t.Fatalf("expected 1 expired drop, got %d", dropped)
	}
	if fired != 0 {
		t.Fatalf("expired entry should not have fired, got %d", fired)
	}
	if len(gs.DelayedAbilityInstances) != 0 {
		t.Fatalf("expired entry still in pool")
	}
}

// ----------------------------------------------------------------------
// Suspend as delayed-trigger composition
// ----------------------------------------------------------------------

// TestPhase8_SuspendTicksAndCastsForFree drives the §702.61 composition
// end-to-end: registering Suspend on a 3-counter card produces (1) a
// recurring upkeep-tick that decrements counters and (2) a one-shot
// cast-when-zero. Three upkeep events tick down, after which the
// suspend_cast_for_free path emits.
func TestPhase8_SuspendTicksAndCastsForFree(t *testing.T) {
	gs := newPhase8GameState(t)
	c := makePhase8Creature(gs, 0, "Errant Ephemeron", 4, 4)
	// Put the card in exile (suspend cast already happened off-stage).
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, c)

	RegisterSuspendComposition(gs, c, 0, 3)
	if len(gs.DelayedAbilityInstances) != 2 {
		t.Fatalf("expected 2 pool entries (tick + cast), got %d", len(gs.DelayedAbilityInstances))
	}

	// Three upkeep ticks.
	for i := 0; i < 3; i++ {
		FireDelayedAbilityPool(gs, &Event{Kind: "upkeep_begin", Seat: 0})
	}
	// Counters depleted.
	if n, _ := c.Meta["suspend_counters"].(int); n != 0 {
		t.Fatalf("counters want 0 after 3 ticks, got %d", n)
	}
	// suspended flag flipped off by the cast effect.
	if suspended, _ := c.Meta["suspended"].(bool); suspended {
		t.Fatalf("card should no longer be suspended after free cast")
	}
}

// TestPhase8_SuspendDoesNotFireOnOpponentUpkeep pins the seat filter on
// the tick: only the controller's upkeep advances the counters.
func TestPhase8_SuspendDoesNotFireOnOpponentUpkeep(t *testing.T) {
	gs := newPhase8GameState(t)
	c := makePhase8Creature(gs, 0, "Rift Bolt", 0, 0)
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, c)
	RegisterSuspendComposition(gs, c, 0, 2)

	// Opponent's upkeep — should NOT decrement.
	FireDelayedAbilityPool(gs, &Event{Kind: "upkeep_begin", Seat: 1})
	if n, _ := c.Meta["suspend_counters"].(int); n != 2 {
		t.Fatalf("opponent upkeep should not decrement, got %d", n)
	}
}

// ----------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------

func graveyardNames(gy []*Card) []string {
	names := make([]string, 0, len(gy))
	for _, c := range gy {
		names = append(names, c.DisplayName())
	}
	return names
}
