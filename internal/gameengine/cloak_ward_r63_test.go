package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// cloak_ward_r63_test.go — Cloak (CR §702.171a). Cloak puts the top library
// card face down as a 2/2 creature WITH ward {2}. The engine put the 2/2 down
// but granted no ward — the defining cloak-vs-manifest difference was missing.

func cloakGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(13)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

// (1) Cloak → face-down 2/2 with ward {2} and summoning sickness.
func TestCloak_FaceDownTwoTwoWithWardTwo(t *testing.T) {
	gs := cloakGame(t)
	card := &Card{Name: "Hidden Horror", Owner: 0, Types: []string{"creature"}, BasePower: 5, BaseToughness: 5}
	perm := PerformCloak(gs, 0, card)
	if perm == nil {
		t.Fatal("PerformCloak returned nil")
	}
	if !IsFaceDown(perm) {
		t.Fatal("cloaked perm should be face down")
	}
	chars := GetEffectiveCharacteristics(gs, perm)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Fatalf("cloaked creature should be 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	if !perm.HasKeyword("ward") {
		t.Fatal("cloaked creature must have ward (the key cloak-vs-manifest difference)")
	}
	if perm.Flags["ward_cost"] != CloakFaceDownWardCost {
		t.Fatalf("ward cost should be %d, got %d", CloakFaceDownWardCost, perm.Flags["ward_cost"])
	}
	if !perm.SummoningSick {
		t.Fatal("a freshly cloaked creature should be summoning sick")
	}
}

// (2) Ward {2} is enforced: an opponent who can't pay {2} is countered.
func TestCloak_WardCountersWhenUnpaid(t *testing.T) {
	gs := cloakGame(t)
	perm := PerformCloak(gs, 0, &Card{Name: "X", Owner: 0, Types: []string{"creature"}})
	gs.Seats[1].ManaPool = 1
	item := &StackItem{
		Kind: "spell", Controller: 1,
		Card:    &Card{Name: "Doom Blade", Owner: 1, Types: []string{"instant"}},
		Targets: []Target{{Kind: TargetKindPermanent, Permanent: perm}},
	}
	CheckWardOnTargeting(gs, item)
	if !item.Countered {
		t.Fatal("opponent spell targeting a cloaked creature should be countered when ward {2} unpaid")
	}
}

// (3) Ward {2} is payable: opponent with enough mana pays 2 and resolves.
func TestCloak_WardPaidWhenAffordable(t *testing.T) {
	gs := cloakGame(t)
	perm := PerformCloak(gs, 0, &Card{Name: "X", Owner: 0, Types: []string{"creature"}})
	gs.Seats[1].ManaPool = 5
	item := &StackItem{
		Kind: "spell", Controller: 1,
		Card:    &Card{Name: "Murder", Owner: 1, Types: []string{"instant"}},
		Targets: []Target{{Kind: TargetKindPermanent, Permanent: perm}},
	}
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Fatal("spell should not be countered when ward {2} is paid")
	}
	if gs.Seats[1].ManaPool != 3 {
		t.Fatalf("ward {2} should be paid: want 3 mana left, got %d", gs.Seats[1].ManaPool)
	}
}

// (4) Contrast: a manifested creature has NO ward — pins the cloak-only ward.
func TestCloak_ManifestHasNoWard(t *testing.T) {
	gs := cloakGame(t)
	addLibrary(gs, 0, "Manifest Me")
	if !manifestTopOfLibrary(gs, 0) {
		t.Fatal("manifestTopOfLibrary failed")
	}
	perm := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if perm.HasKeyword("ward") {
		t.Fatal("a manifested creature must NOT have ward (only cloak grants ward {2})")
	}
}

// (5) End-to-end: the "cloak" ModificationEffect pulls the top library card
// and cloaks it face down with ward {2}.
func TestCloak_ResolvePathPullsTopAndWards(t *testing.T) {
	gs := cloakGame(t)
	addLibrary(gs, 0, "Top Card")
	src := &Permanent{Card: &Card{Name: "Cloaker", Owner: 0}, Controller: 0, Owner: 0, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	before := len(gs.Seats[0].Battlefield)

	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "cloak"})

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Fatal("cloak resolve should add a face-down creature to the battlefield")
	}
	cloaked := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if !IsFaceDown(cloaked) || !cloaked.HasKeyword("ward") {
		t.Fatal("resolve-path cloak should produce a face-down creature with ward")
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Fatalf("cloak should have removed the top library card, %d left", len(gs.Seats[0].Library))
	}
}
