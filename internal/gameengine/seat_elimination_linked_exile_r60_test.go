package gameengine

import (
	"math/rand"
	"testing"
)

// Loki r60 fresh-main (2026-05-30, PR #781 + post-#781 dispatch fix
// f374f26b) surfaced 26 ExileLinkageIntegrity hits on Leonardo, Sewer
// Samurai // Leonardo, Sewer Samurai in seat 0 exile, linked to a
// source timestamp no longer on any battlefield. Same shape on
// Prison Barricade (42 hits, game 2164). After the trigger-dispatch
// fix added ctx["perm"] fallback for permanent_ltb (closing the
// DestroyPermanent / ExilePermanent / etc. paths), the residual signal
// trace to HandleSeatElimination — which sweeps every permanent
// owned/controlled by the leaving seat in a single pass and bypasses
// the canonical zone_change.go LTB pipeline that would otherwise fire
// permanent_ltb and run the Banisher-Priest-family ltbReturnLinkedExile
// handler.
//
// Pre-fix: §800.4a elimination called MarkInstanceIDCeased,
// UnregisterReplacementsForPermanent, UnregisterContinuousEffectsForPermanent,
// ExpireSourceGrants — but never fired permanent_ltb and never released
// LinkedExile bookkeeping. The exiled card kept ExiledByTimestamp
// pointing at the now-gone source perm; the ExileLinkageIntegrity
// invariant flagged it.
//
// Fix: HandleSeatElimination now calls ReleaseSourceLinkedExiles on
// every detached perm. The exiled cards stay in opp's exile (the
// rules-correct §406.7 return is a separate engine-correctness TODO
// because routing MoveCard into a battlefield mid-sweep would race
// with the in-progress seat removal), but their dangling
// ExiledByTimestamp pointers are cleared so the invariant runs clean.

// TestSeatElimination_ReleasesLinkedExileOnLeavingSource pins the
// bit-stable signature: seat 0 controls a Banisher-Priest-style
// permanent that exiled an opp's (seat 1) creature; seat 0 is
// eliminated; the exiled card's ExiledByTimestamp must be cleared
// and the source's LinkedExile/ExiledByMe must be empty, so
// ExileLinkageIntegrity runs clean.
func TestSeatElimination_ReleasesLinkedExileOnLeavingSource(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Faceless Butcher equivalent on seat 0's battlefield.
	srcCard := &Card{
		Name:       "Faceless Butcher",
		Owner:      0,
		InstanceID: "h0OGVB300010",
		Types:      []string{"creature"},
	}
	src := &Permanent{
		Card:       srcCard,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	// Leonardo (opp's creature) — exiled by the butcher, sitting in
	// seat 1's exile linked to src.Timestamp.
	preyCard := &Card{
		Name:       "Leonardo, Sewer Samurai",
		Owner:      1,
		InstanceID: "h1OGVW400017",
		Types:      []string{"creature", "legendary"},
	}
	gs.MintedInstanceIDs = map[string]struct{}{
		srcCard.InstanceID:  {},
		preyCard.InstanceID: {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		srcCard.InstanceID:  srcCard.Name,
		preyCard.InstanceID: preyCard.Name,
	}
	// Mirror the post-ExileLinked state without re-running the full
	// ETB flow: prey in opp's exile, ExiledByTimestamp pointing at
	// the butcher's timestamp; butcher's LinkedExile + ExiledByMe
	// populated; LinkageKind tagged LTBReturn.
	preyCard.ExiledByTimestamp = src.Timestamp
	src.LinkedExile = []*Card{preyCard}
	src.ExiledByMe = []string{preyCard.InstanceID}
	src.LinkageKind = LTBReturn
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, preyCard)

	// Sanity: pre-elim, prey's timestamp resolves to a live source.
	if !permanentWithTimestampExists(gs, src.Timestamp) {
		t.Fatalf("pre-elim: source perm not findable by timestamp %d", src.Timestamp)
	}

	// Eliminate seat 0.
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// Post-elim assertions:
	// 1. Source's LinkedExile and ExiledByMe must be empty.
	if len(src.LinkedExile) != 0 {
		t.Fatalf("post-elim: src.LinkedExile=%v, want empty", src.LinkedExile)
	}
	if len(src.ExiledByMe) != 0 {
		t.Fatalf("post-elim: src.ExiledByMe=%v, want empty", src.ExiledByMe)
	}
	// 2. Prey's ExiledByTimestamp must be cleared.
	if preyCard.ExiledByTimestamp != 0 {
		t.Fatalf("post-elim: prey.ExiledByTimestamp=%d, want 0", preyCard.ExiledByTimestamp)
	}
	// 3. Prey stays in seat 1's exile (engine-correctness TODO is to
	//    route back to battlefield per §406.7 via a deferred queue;
	//    for now we just clear the linkage).
	found := false
	for _, c := range gs.Seats[1].Exile {
		if c == preyCard {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("post-elim: prey expected to remain in seat 1 exile (release-only fix), but it's gone")
	}
	// 4. ExileLinkageIntegrity invariant runs clean.
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("ExileLinkageIntegrity must pass post-elimination; got: %v", err)
	}
}

// TestSeatElimination_PreservesLinkedExileOnSurvivingSources confirms
// the release is scoped to leaving perms only — Banisher-Priest-family
// sources on SURVIVING seats keep their LinkedExile + ExiledByMe
// intact across the elimination event.
func TestSeatElimination_PreservesLinkedExileOnSurvivingSources(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Survivor Banisher-style perm on seat 2.
	survCard := &Card{
		Name:       "Banisher Priest",
		Owner:      2,
		InstanceID: "h2OGVW200005",
		Types:      []string{"creature"},
	}
	surv := &Permanent{
		Card:       survCard,
		Controller: 2,
		Owner:      2,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[2].Battlefield = append(gs.Seats[2].Battlefield, surv)

	// Surv has exiled seat 3's creature.
	preyCard := &Card{
		Name:       "Goblin Guide",
		Owner:      3,
		InstanceID: "h3OGVR100008",
		Types:      []string{"creature"},
	}
	gs.MintedInstanceIDs = map[string]struct{}{
		survCard.InstanceID: {},
		preyCard.InstanceID: {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		survCard.InstanceID: survCard.Name,
		preyCard.InstanceID: preyCard.Name,
	}
	preyCard.ExiledByTimestamp = surv.Timestamp
	surv.LinkedExile = []*Card{preyCard}
	surv.ExiledByMe = []string{preyCard.InstanceID}
	surv.LinkageKind = LTBReturn
	gs.Seats[3].Exile = append(gs.Seats[3].Exile, preyCard)

	// Eliminate seat 0 (totally unrelated).
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// Survivor's linkage must be untouched.
	if len(surv.LinkedExile) != 1 || surv.LinkedExile[0] != preyCard {
		t.Fatalf("survivor's LinkedExile must be untouched; got %v", surv.LinkedExile)
	}
	if len(surv.ExiledByMe) != 1 || surv.ExiledByMe[0] != preyCard.InstanceID {
		t.Fatalf("survivor's ExiledByMe must be untouched; got %v", surv.ExiledByMe)
	}
	if preyCard.ExiledByTimestamp != surv.Timestamp {
		t.Fatalf("prey's ExiledByTimestamp must be untouched; got %d", preyCard.ExiledByTimestamp)
	}
}

// TestSeatElimination_NoOpWhenLeavingPermHasNoLinkedExile pins that
// the new release call is a no-op when the leaving perm has no exile
// linkage — defends against the helper accidentally mutating
// unrelated state.
func TestSeatElimination_NoOpWhenLeavingPermHasNoLinkedExile(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Plain vanilla creature on seat 0 — no LinkedExile, no ExiledByMe.
	srcCard := &Card{
		Name:       "Bears",
		Owner:      0,
		InstanceID: "h0OGVG200001",
		Types:      []string{"creature"},
	}
	src := &Permanent{
		Card:       srcCard,
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	gs.MintedInstanceIDs = map[string]struct{}{srcCard.InstanceID: {}}
	gs.MintedInstanceIDNames = map[string]string{srcCard.InstanceID: srcCard.Name}

	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// Nothing to assert beyond "didn't panic"; this test exists to
	// guarantee the new ReleaseSourceLinkedExiles call short-circuits
	// cleanly on the common case (most leaving perms have no exile
	// linkage at all).
	if err := checkExileLinkageIntegrity(gs); err != nil {
		t.Fatalf("ExileLinkageIntegrity must pass post-elimination; got: %v", err)
	}
}
