package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Shared test helpers for the stubs-tail mechanics.
// ---------------------------------------------------------------------------

func newStubsTailGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(1)), nil)
}

func newKeywordInstant(name string, owner, cmc int, kw string, arg any) *Card {
	abilities := []gameast.Ability{}
	if arg == nil {
		abilities = append(abilities, &gameast.Keyword{Name: kw})
	} else {
		abilities = append(abilities, &gameast.Keyword{Name: kw, Args: []any{arg}})
	}
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   cmc,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: abilities,
		},
	}
}

// ---------------------------------------------------------------------------
// §702.183 — Tiered
// ---------------------------------------------------------------------------

func TestTieredCost_ManaStringAndNumeric(t *testing.T) {
	mana := newKeywordInstant("Tier Bolt", 0, 1, "tiered", "{2}")
	if got := TieredCost(mana); got != 2 {
		t.Fatalf("TieredCost mana string = %d, want 2", got)
	}
	numeric := newKeywordInstant("Tier Bolt II", 0, 1, "tiered", 4)
	if got := TieredCost(numeric); got != 4 {
		t.Fatalf("TieredCost numeric = %d, want 4", got)
	}
	if got := TieredCost(nil); got != 0 {
		t.Fatalf("TieredCost(nil) = %d, want 0", got)
	}
	plain := newPlainInstant("Bolt", 0, 1)
	if got := TieredCost(plain); got != 0 {
		t.Fatalf("TieredCost on non-tiered = %d, want 0", got)
	}
}

func TestApplyTiered_RecordsModesAndPushesCopies(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 6

	card := newKeywordInstant("Trifold Strike", 0, 2, "tiered", 2)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	// Tiers=2 → 2 extra copies, total stack should be 3.
	got := ApplyTiered(gs, item, []int{2, 0, 0, 1}, 2)
	if got != 2 {
		t.Fatalf("ApplyTiered returned %d, want 2", got)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("ManaPool = %d, want 2 (paid 2 tiers × 2)", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 3 {
		t.Fatalf("stack size = %d, want 3 (original + 2 copies)", len(gs.Stack))
	}
	modes := TieredModes(item)
	if len(modes) != 3 || modes[0] != 0 || modes[1] != 1 || modes[2] != 2 {
		t.Fatalf("TieredModes = %v, want [0 1 2] (sorted/dedup)", modes)
	}
	if TieredTiers(item) != 2 {
		t.Fatalf("TieredTiers = %d, want 2", TieredTiers(item))
	}
	if !IsTiered(item) {
		t.Fatal("IsTiered should be true")
	}
	for i, c := range gs.Stack[1:] {
		if !c.IsCopy {
			t.Fatalf("copy %d: IsCopy should be true", i)
		}
		cm := TieredModes(c)
		if len(cm) != 3 {
			t.Fatalf("copy %d: modes = %v, want 3 entries", i, cm)
		}
	}
}

func TestApplyTiered_InsufficientManaFailsAtomically(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 1 // only enough for 0 tiers

	card := newKeywordInstant("Trifold Strike", 0, 2, "tiered", 2)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	got := ApplyTiered(gs, item, []int{0}, 2) // wants 4 mana, has 1
	if got != 0 {
		t.Fatalf("ApplyTiered returned %d, want 0 (atomic fail)", got)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("ManaPool should be unchanged, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("stack should still hold only the original, got %d", len(gs.Stack))
	}
	if TieredTiers(item) != 0 {
		t.Fatalf("TieredTiers = %d, want 0 on fail", TieredTiers(item))
	}
	// Modes were still recorded — mode choice is independent of tier payment.
	if got := TieredModes(item); len(got) != 1 || got[0] != 0 {
		t.Fatalf("modes after fail = %v, want [0]", got)
	}
}

func TestApplyTiered_ZeroTiersStillRecordsModes(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0

	card := newKeywordInstant("Trifold Strike", 0, 2, "tiered", 2)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	got := ApplyTiered(gs, item, []int{1, 1, 2}, 0)
	if got != 0 {
		t.Fatalf("ApplyTiered tiers=0 returned %d, want 0", got)
	}
	modes := TieredModes(item)
	if len(modes) != 2 || modes[0] != 1 || modes[1] != 2 {
		t.Fatalf("modes = %v, want [1 2] (deduped)", modes)
	}
}

// ---------------------------------------------------------------------------
// §702.190 — Infinity
// ---------------------------------------------------------------------------

func TestInfinityCost_ParsesArgs(t *testing.T) {
	card := newKeywordInstant("Boundless Bolt", 0, 1, "infinity", "{1}{r}")
	if got := InfinityCost(card); got != 2 {
		t.Fatalf("InfinityCost = %d, want 2", got)
	}
	numeric := newKeywordInstant("Eternal Hex", 0, 2, "infinity", 3)
	if got := InfinityCost(numeric); got != 3 {
		t.Fatalf("InfinityCost numeric = %d, want 3", got)
	}
	if got := InfinityCost(newPlainInstant("Bolt", 0, 1)); got != 0 {
		t.Fatalf("InfinityCost on non-infinity = %d, want 0", got)
	}
}

func TestApplyInfinity_PaysStacksAndRecordsCount(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := newKeywordInstant("Boundless Bolt", 0, 1, "infinity", 1)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	got := ApplyInfinity(gs, item, 4)
	if got != 4 {
		t.Fatalf("ApplyInfinity returned %d, want 4", got)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("ManaPool = %d, want 1 (paid 4×1)", gs.Seats[0].ManaPool)
	}
	if InfinityCount(item) != 4 {
		t.Fatalf("InfinityCount = %d, want 4", InfinityCount(item))
	}
	if !IsInfinityCast(item) {
		t.Fatal("IsInfinityCast should be true after pay")
	}
}

func TestApplyInfinity_ZeroStacksExplicitlyRecorded(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5

	card := newKeywordInstant("Boundless Bolt", 0, 1, "infinity", 1)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	if got := ApplyInfinity(gs, item, 0); got != 0 {
		t.Fatalf("ApplyInfinity stacks=0 returned %d, want 0", got)
	}
	if !IsInfinityCast(item) {
		t.Fatal("IsInfinityCast should still be true — explicit no-pay decision")
	}
	if InfinityCount(item) != 0 {
		t.Fatalf("InfinityCount = %d, want 0", InfinityCount(item))
	}
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("ManaPool changed on zero-stack cast: %d", gs.Seats[0].ManaPool)
	}
}

func TestApplyInfinity_InsufficientManaAtomicFail(t *testing.T) {
	gs := newStubsTailGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 2

	card := newKeywordInstant("Boundless Bolt", 0, 1, "infinity", 1)
	item := &StackItem{Card: card, Controller: 0}
	PushStackItem(gs, item)

	if got := ApplyInfinity(gs, item, 5); got != 0 {
		t.Fatalf("ApplyInfinity returned %d, want 0 on atomic fail", got)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("ManaPool changed on fail: %d", gs.Seats[0].ManaPool)
	}
	if InfinityCount(item) != 0 {
		t.Fatalf("InfinityCount = %d, want 0 on fail", InfinityCount(item))
	}
}

// ---------------------------------------------------------------------------
// §702.158 — Space Sculptor
// ---------------------------------------------------------------------------

func makeSculptorPerm(seat int, name string) *Permanent {
	c := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1}
	return &Permanent{Card: c, Controller: seat, Owner: seat, Flags: map[string]int{}}
}

func TestAssignSector_PlacesAndClearsPrior(t *testing.T) {
	gs := newStubsTailGame(t)
	p := makeSculptorPerm(0, "Astro Cat")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	if !AssignSector(gs, p, "alpha") {
		t.Fatal("AssignSector(alpha) returned false")
	}
	if got := PermanentSector(p); got != "alpha" {
		t.Fatalf("PermanentSector after assign = %q, want alpha", got)
	}
	if !AssignSector(gs, p, "gamma") {
		t.Fatal("AssignSector(gamma) returned false")
	}
	if got := PermanentSector(p); got != "gamma" {
		t.Fatalf("PermanentSector after reassign = %q, want gamma", got)
	}
	if p.Flags["sculptor_sector_alpha"] == 1 {
		t.Fatal("old sector flag should be cleared on reassign")
	}
}

func TestAssignSector_RejectsInvalidName(t *testing.T) {
	gs := newStubsTailGame(t)
	p := makeSculptorPerm(0, "X")
	if AssignSector(gs, p, "epsilon") {
		t.Fatal("AssignSector should reject unknown sector")
	}
	if PermanentSector(p) != "" {
		t.Fatal("perm should remain unassigned after reject")
	}
}

func TestClaimSector_AndControllerLookup(t *testing.T) {
	gs := newStubsTailGame(t)
	if got := SectorController(gs, "beta"); got != -1 {
		t.Fatalf("SectorController unset = %d, want -1", got)
	}
	if !ClaimSector(gs, 1, "beta") {
		t.Fatal("ClaimSector failed")
	}
	if got := SectorController(gs, "beta"); got != 1 {
		t.Fatalf("SectorController = %d, want 1", got)
	}
	if !ClaimSector(gs, 0, "beta") {
		t.Fatal("ClaimSector overwrite failed")
	}
	if got := SectorController(gs, "beta"); got != 0 {
		t.Fatalf("SectorController after overwrite = %d, want 0", got)
	}
	if !ReleaseSector(gs, "beta") {
		t.Fatal("ReleaseSector failed")
	}
	if got := SectorController(gs, "beta"); got != -1 {
		t.Fatalf("SectorController after release = %d, want -1", got)
	}
}

func TestControlsPermanentViaSculptor(t *testing.T) {
	gs := newStubsTailGame(t)
	p := makeSculptorPerm(0, "Astro Cat")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	AssignSector(gs, p, "gamma")
	ClaimSector(gs, 1, "gamma")

	if !ControlsPermanentViaSculptor(gs, 1, p) {
		t.Fatal("seat 1 should have sculptor zone-control over perm in gamma")
	}
	if ControlsPermanentViaSculptor(gs, 0, p) {
		t.Fatal("seat 0 should NOT have sculptor zone-control (perm.Controller==0 but sector controlled by 1)")
	}
}

func TestPermanentsInSector_AcrossSeats(t *testing.T) {
	gs := newStubsTailGame(t)
	a := makeSculptorPerm(0, "A")
	b := makeSculptorPerm(0, "B")
	c := makeSculptorPerm(1, "C")
	d := makeSculptorPerm(1, "D")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, a, b)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, c, d)

	AssignSector(gs, a, "alpha")
	AssignSector(gs, c, "alpha")
	AssignSector(gs, b, "beta")
	AssignSector(gs, d, "gamma")

	got := PermanentsInSector(gs, "alpha")
	if len(got) != 2 {
		t.Fatalf("PermanentsInSector(alpha) = %d perms, want 2", len(got))
	}
	for _, p := range got {
		if PermanentSector(p) != "alpha" {
			t.Fatalf("returned perm in wrong sector: %q", PermanentSector(p))
		}
	}
	if PermanentsInSector(gs, "no-such") != nil {
		t.Fatal("invalid sector should return nil")
	}
}

func TestSpaceSculptorSectors_StableOrder(t *testing.T) {
	got := SpaceSculptorSectors()
	// CR §702.158b: "The sector designations are alpha sector, beta sector,
	// and gamma sector." Three. This test previously asserted a fourth,
	// "delta", codifying a sector that exists in no edition of the rules.
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("sector list len = %d, want %d", len(got), len(want))
	}
	for i, s := range want {
		if got[i] != s {
			t.Fatalf("sector[%d] = %q, want %q", i, got[i], s)
		}
	}
}

// TestSpaceSculptorSectors_NoFourthSector pins the sector vocabulary to the
// Comprehensive Rules. CR §702.158b, verbatim:
//
//	"A sector designation is a designation a permanent can have. The sector
//	 designations are alpha sector, beta sector, and gamma sector."
//
// The engine previously defined a fourth, SectorDelta = "delta", and its own
// test asserted it. Nothing in the rules has ever had a delta sector; it was
// pattern-completion (alpha, beta, gamma... delta) rather than a reading of
// the rule. Space sculptor has no production callers today, so the invented
// sector was inert — but it would have produced an illegal board state the
// moment the subsystem was wired to a card.
func TestSpaceSculptorSectors_NoFourthSector(t *testing.T) {
	for _, bad := range []string{"delta", "epsilon", "omega"} {
		if canon, ok := validSculptorSector(bad); ok {
			t.Errorf("validSculptorSector(%q) accepted it as %q; CR §702.158b lists only alpha, beta and gamma", bad, canon)
		}
	}
	if got := len(SpaceSculptorSectors()); got != 3 {
		t.Errorf("SpaceSculptorSectors() has %d sectors, want 3 per CR §702.158b", got)
	}
	gs := newStubsTailGame(t)
	p := makeSculptorPerm(0, "Astro Cat")
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	if AssignSector(gs, p, "delta") {
		t.Error("AssignSector accepted a delta sector; no such designation exists in the CR")
	}
}
