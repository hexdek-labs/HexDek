package per_card

import (
	"testing"
)

// keyword_counter_grant_r92_test.go — Indominus Rex, Alpha stamped a raw
// Flags["kw:<name>"] beside each keyword counter it placed.
//
// Found while auditing the other clone-adjacent re-entry paths after
// r90: the audit itself came back clean (every as-enters replacement
// reachable through fireETBOnCopy pays its cost exactly once), but the
// permanent's flag map after a Clone-of-Indominus showed a keyword flag
// sitting next to a counter of the same name, which is this.
//
// The flag is redundant on the way in and permanent on the way out. CR
// 122.1b: "A keyword counter on a permanent ... causes that object to
// gain that keyword."

// TestKeywordCounterR92_CounterAloneGrantsOnBothAuthorities is the
// "redundant on the way in" half, and the reason the flag was never
// needed. It must pass before and after — an over-suppression guard.
func TestKeywordCounterR92_CounterAloneGrantsOnBothAuthorities(t *testing.T) {
	gs := newGame(t, 2)
	p := r89Creature(gs, 0, "R92 Beast", "creature")

	if p.HasKeyword("flying") {
		t.Fatalf("fixture wrong: already has flying")
	}
	p.AddCounter("flying", 1)
	gs.InvalidateCharacteristicsCache()

	if !p.HasKeyword("flying") {
		t.Errorf("raw authority does not see a flying counter (CR 122.1b)")
	}
	if !gs.HasKeywordOf(p, "flying") {
		t.Errorf("layer-aware authority does not see a flying counter (CR 122.1b)")
	}
	if _, ok := p.Flags["kw:flying"]; ok {
		t.Errorf("AddCounter should not need a mirrored keyword flag (flags: %+v)", p.Flags)
	}
}

// TestKeywordCounterR92_IndominusKeywordDiesWithItsCounter is the
// "permanent on the way out" half — the defect. Pre-fix the keyword
// survives removal of the counter that granted it, on BOTH authorities,
// for the rest of the game.
func TestKeywordCounterR92_IndominusKeywordDiesWithItsCounter(t *testing.T) {
	gs := newGame(t, 2)
	rex := r89Creature(gs, 0, "Indominus Rex, Alpha", "creature")
	r92HandFlyer(gs, 0)

	indominusRexAlphaETB(gs, rex)

	if rex.Counters["flying"] != 1 {
		t.Fatalf("fixture inert: Indominus Rex placed no flying counter (counters: %+v)", rex.Counters)
	}
	if !rex.HasKeyword("flying") {
		t.Fatalf("fixture inert: the flying counter granted nothing")
	}

	// A counter-remover takes the counter off (Vampire Hexmage, Hex
	// Parasite, Thief of Blood). The keyword must go with it.
	delete(rex.Counters, "flying")
	gs.InvalidateCharacteristicsCache()

	if rex.HasKeyword("flying") {
		t.Errorf("flying outlived the counter that granted it (flags: %+v, counters: %+v)", rex.Flags, rex.Counters)
	}
	if gs.HasKeywordOf(rex, "flying") {
		t.Errorf("flying outlived its counter on the layer-aware authority too (flags: %+v)", rex.Flags)
	}
}

// TestKeywordCounterR92_TayamCounterReachesTheLayerAuthority is the
// invalidation half, and it took two attempts to make it mean anything.
//
// A keyword counter is a layer-6 grant (CR 122.1b). AddCounter is the
// raw primitive: it takes no gs and cannot invalidate, and every raw
// keyword-counter site in the tree had skipped the invalidation the
// canonical placer (counter_place.go) performs. Priming the cache with
// a layer-aware read first is what makes the gap observable.
//
// Tayam is the entry point used rather than Indominus Rex, because
// Indominus discards before it places counters and DiscardCard
// invalidates as a side effect — so the same assertion against that
// card passes with or without the fix. A test can be made of the right
// shape and still be measuring someone else's invalidation.
func TestKeywordCounterR92_TayamCounterReachesTheLayerAuthority(t *testing.T) {
	gs := newGame(t, 2)
	tayam := r89Creature(gs, 0, "Tayam, Luminous Enigma", "creature")
	entering := r89Creature(gs, 0, "R92 Entrant", "creature")

	// Prime the characteristics cache for the entering creature.
	if gs.HasKeywordOf(entering, "vigilance") {
		t.Fatalf("fixture wrong: already has vigilance")
	}

	tayamETBCounter(gs, tayam, map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	if entering.Counters["vigilance"] != 1 {
		t.Fatalf("fixture inert: Tayam placed no vigilance counter (counters: %+v)", entering.Counters)
	}
	if !gs.HasKeywordOf(entering, "vigilance") {
		t.Errorf("the layer-aware authority is serving a stale cache entry: the counter is on the permanent but nothing invalidated (counters: %+v)", entering.Counters)
	}
}

// TestKeywordCounterR92_IndestructibleCounterIsCacheFreeForTheSBA pins
// the BOUND on the harm above, and must pass both ways.
//
// Four of the seven raw sites are the Dominus cycle granting
// indestructible, which looks alarming until you read what consults it:
// Permanent.IsIndestructible checks Flags, GrantedAbilities and the two
// counter stores directly and never goes through
// GetEffectiveCharacteristics. So a stale cache could never have let a
// Dominus die to destruction. The damage was confined to readers built
// on the layer query — which is where the r70-r91 migration moved
// combat, and the reason this was worth fixing rather than noting.
func TestKeywordCounterR92_IndestructibleCounterIsCacheFreeForTheSBA(t *testing.T) {
	gs := newGame(t, 2)
	p := r89Creature(gs, 0, "R92 Dominus", "creature")

	_ = gs.HasKeywordOf(p, "indestructible") // prime, deliberately not invalidated after
	p.AddCounter("indestructible", 1)

	if !p.IsIndestructible() {
		t.Errorf("the destruction path must see an indestructible counter with no invalidation at all (counters: %+v)", p.Counters)
	}
}

// TestKeywordCounterR92_CloneOfIndominusPaysItsCostOnce is the audit
// result the above came out of, kept as a regression. A Clone entering
// as a copy of Indominus Rex reaches indominusRexAlphaETB through
// fireETBOnCopy; its "as it enters, discard" must be paid exactly once,
// which is what r90's claim and the single fireETBOnCopy per entry buy.
func TestKeywordCounterR92_CloneOfIndominusPaysItsCostOnce(t *testing.T) {
	gs := newGame(t, 2)
	_ = r90Clone(gs, 1, "Indominus Rex, Alpha", "creature")
	clone := r90Clone(gs, 0, "Clone", "creature")
	r92HandFlyer(gs, 0)
	r92HandFlyer(gs, 0)
	before := len(gs.Seats[0].Hand)

	cloneETB(gs, clone)

	if got := clone.Card.DisplayName(); got != "Indominus Rex, Alpha" {
		t.Fatalf("fixture inert: clone did not become Indominus Rex, got %q", got)
	}
	// Both hand cards are legal discards for the as-enters choice, so
	// "paid once" is one application of the choice, not one card.
	if n := before - len(gs.Seats[0].Hand); n != 2 {
		t.Errorf("as-enters discard paid %d cards from a hand of %d, want the single application that takes both", n, before)
	}
	if n := clone.Counters["flying"]; n != 1 {
		t.Errorf("flying counter count = %d, want exactly 1 — a second application would stack another", n)
	}
}
