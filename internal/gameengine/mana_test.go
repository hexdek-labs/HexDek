package gameengine

// CR §106 typed mana pool — Go-side tests.
//
// Mirrors scripts/test_mana_pool.py. Covers:
//   - Sol Ring taps for {C}{C}.
//   - Treasure token taps + sacrifices for any color.
//   - Powerstone token produces restricted colorless.
//   - Food Chain-style restricted mana can't pay non-creature spells.
//   - Omnath retains green across phase boundaries.
//   - Upwelling retains all colors across phase boundaries.
//   - Pool drains at every phase/step transition absent exemption.
//   - Legacy ManaPool int API remains compatible.

import (
	"testing"
)

func makeSeat() *Seat {
	s := newSeat(0)
	return s
}

func TestTypedPool_AddAndTotal(t *testing.T) {
	s := makeSeat()
	AddMana(nil, s, "W", 3, "test")
	AddMana(nil, s, "R", 2, "test")
	if s.Mana.Total() != 5 {
		t.Fatalf("expected total 5, got %d", s.Mana.Total())
	}
	if s.ManaPool != 5 {
		t.Fatalf("ManaPool mirror expected 5, got %d", s.ManaPool)
	}
}

func TestTypedPool_LegacyIntCompatibility(t *testing.T) {
	s := makeSeat()
	// Legacy code writes seat.ManaPool = 10 directly.
	s.ManaPool = 10
	// PayGenericCost should drain via the fallback branch.
	if !PayGenericCost(nil, s, 5, "generic", "test", "Counterspell") {
		t.Fatalf("legacy ManaPool should pay 5 via fallback")
	}
	if s.ManaPool != 5 {
		t.Fatalf("expected ManaPool 5 after 5 paid, got %d", s.ManaPool)
	}
}

func TestSolRing_Produces_CC(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	sol := &Permanent{
		Card: &Card{
			Name:     "Sol Ring",
			TypeLine: "Legendary Artifact",
			Types:    []string{"artifact", "legendary"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, sol)
	pips, ok := ApplyArtifactMana(gs, seat, sol)
	if !ok {
		t.Fatalf("Sol Ring should tap for mana")
	}
	if pips != 2 {
		t.Fatalf("Sol Ring should produce 2 pips, got %d", pips)
	}
	if seat.Mana.C != 2 {
		t.Fatalf("Sol Ring should add C=2, got %d", seat.Mana.C)
	}
	if !sol.Tapped {
		t.Fatalf("Sol Ring should be tapped")
	}
}

func TestManaCrypt_Produces_CC(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	crypt := &Permanent{
		Card: &Card{
			Name: "Mana Crypt", TypeLine: "Artifact",
			Types: []string{"artifact"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, crypt)
	_, ok := ApplyArtifactMana(gs, seat, crypt)
	if !ok {
		t.Fatalf("Mana Crypt should tap")
	}
	if seat.Mana.C != 2 {
		t.Fatalf("Mana Crypt should add C=2")
	}
}

func TestTreasureToken_TapSacForAny(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	CreateTreasureToken(gs, 0)
	if len(seat.Battlefield) != 1 {
		t.Fatalf("Treasure should appear on battlefield")
	}
	treasure := seat.Battlefield[0]
	_, ok := ApplyArtifactMana(gs, seat, treasure)
	if !ok {
		t.Fatalf("Treasure should tap-sac for mana")
	}
	if seat.Mana.Any != 1 {
		t.Fatalf("Treasure should produce 1 any-color, got Any=%d", seat.Mana.Any)
	}
	if len(seat.Battlefield) != 0 {
		t.Fatalf("Treasure should be sacrificed; battlefield=%d", len(seat.Battlefield))
	}
}

func TestLotusPetal_TapSacForAny(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	petal := &Permanent{
		Card: &Card{
			Name: "Lotus Petal", TypeLine: "Artifact",
			Types: []string{"artifact"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, petal)
	_, ok := ApplyArtifactMana(gs, seat, petal)
	if !ok {
		t.Fatalf("Lotus Petal should tap-sac")
	}
	if seat.Mana.Any != 1 {
		t.Fatalf("Lotus Petal should produce 1 any-color")
	}
	if len(seat.Battlefield) != 0 {
		t.Fatalf("Lotus Petal should be sacrificed")
	}
}

func TestPowerstoneToken_RestrictedColorless(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	CreatePowerstoneToken(gs, 0)
	stone := seat.Battlefield[0]
	_, ok := ApplyArtifactMana(gs, seat, stone)
	if !ok {
		t.Fatalf("Powerstone should tap")
	}
	if len(seat.Mana.Restricted) != 1 {
		t.Fatalf("Powerstone should add 1 restricted pip, got %+v",
			seat.Mana.Restricted)
	}
	r := seat.Mana.Restricted[0]
	if r.Color != "C" {
		t.Fatalf("Powerstone color should be C, got %q", r.Color)
	}
	if r.Restriction != "noncreature_or_artifact_activation" {
		t.Fatalf("Powerstone restriction wrong: %q", r.Restriction)
	}
	// Must not pay a creature spell (generic amount 1, spell_type creature).
	if seat.Mana.CanPayGeneric(1, "creature") {
		t.Fatalf("Powerstone mana must NOT pay a creature cost")
	}
	// Should pay an instant (noncreature).
	if !seat.Mana.CanPayGeneric(1, "instant") {
		t.Fatalf("Powerstone mana should pay an instant cost")
	}
}

func TestFoodChainRestriction(t *testing.T) {
	s := makeSeat()
	AddRestrictedMana(nil, s, 5, "", "creature_spell_only", "Food Chain")
	// Creature spell: 3-cost should be payable.
	if !PayGenericCost(nil, s, 3, "creature", "cast", "Test Creature") {
		t.Fatalf("Food Chain mana should pay a creature cost")
	}
	// Recharge.
	s.Mana = nil
	s.ManaPool = 0
	AddRestrictedMana(nil, s, 5, "", "creature_spell_only", "Food Chain")
	if PayGenericCost(nil, s, 3, "sorcery", "cast", "Test Sorcery") {
		t.Fatalf("Food Chain mana should NOT pay a sorcery")
	}
}

func TestOmnathRetainsGreen(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	omnath := &Permanent{
		Card: &Card{
			Name: "Omnath, Locus of Mana", TypeLine: "Legendary Creature",
			Types: []string{"creature", "legendary", "elemental"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, omnath)
	AddMana(gs, seat, "G", 5, "test")
	AddMana(gs, seat, "R", 3, "test")
	AddMana(gs, seat, "any", 2, "test")
	DrainAllPools(gs, "precombat_main", "")
	if seat.Mana.G != 5 {
		t.Fatalf("Omnath should retain G=5; got %d", seat.Mana.G)
	}
	if seat.Mana.R != 0 {
		t.Fatalf("R should drain; got %d", seat.Mana.R)
	}
	if seat.Mana.Any != 0 {
		t.Fatalf("any should drain; got %d", seat.Mana.Any)
	}
}

func TestUpwellingRetainsAll(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	up := &Permanent{
		Card: &Card{
			Name: "Upwelling", TypeLine: "Enchantment",
			Types: []string{"enchantment"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, up)
	AddMana(gs, seat, "W", 2, "test")
	AddMana(gs, seat, "R", 2, "test")
	AddMana(gs, seat, "any", 2, "test")
	before := seat.Mana.Total()
	DrainAllPools(gs, "combat", "end_of_combat")
	if seat.Mana.Total() != before {
		t.Fatalf("Upwelling should retain pool; before=%d after=%d",
			before, seat.Mana.Total())
	}
}

func TestPoolDrainsWithoutExemption(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	AddMana(gs, seat, "R", 4, "test")
	AddMana(gs, seat, "any", 2, "test")
	DrainAllPools(gs, "precombat_main", "")
	if seat.Mana.Total() != 0 {
		t.Fatalf("Pool should empty, got total=%d", seat.Mana.Total())
	}
	if seat.ManaPool != 0 {
		t.Fatalf("Legacy ManaPool should mirror 0, got %d", seat.ManaPool)
	}
}

func TestColoredPip_UsesMatchingBucket(t *testing.T) {
	s := makeSeat()
	p := EnsureTypedPool(s)
	p.W = 2
	p.U = 2
	p.R = 2
	bucket := PayColoredPip(p, "W", "instant")
	if bucket != "W" {
		t.Fatalf("expected W bucket, got %q", bucket)
	}
	if p.W != 1 {
		t.Fatalf("W should decrement to 1, got %d", p.W)
	}
}

func TestSignetIsRecognized(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	// Signets cost {1} to activate — seat needs at least 1 mana.
	seat.ManaPool = 3
	signet := &Permanent{
		Card: &Card{
			Name: "Izzet Signet", TypeLine: "Artifact",
			Types: []string{"artifact"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, signet)
	pips, ok := ApplyArtifactMana(gs, seat, signet)
	if !ok {
		t.Fatalf("Izzet Signet should tap")
	}
	if pips != 2 {
		t.Fatalf("Signet should produce 2 pips, got %d", pips)
	}
	// Net mana: started with 3, paid 1, added 2 => 4
	if seat.ManaPool != 4 {
		t.Fatalf("Signet net mana: started 3, paid 1, added 2 => want 4, got %d", seat.ManaPool)
	}
}

func TestSignetCantActivateWithoutMana(t *testing.T) {
	gs := newGameStateForMana(1)
	seat := gs.Seats[0]
	// Seat has 0 mana — signet activation should fail.
	signet := &Permanent{
		Card: &Card{
			Name: "Dimir Signet", TypeLine: "Artifact",
			Types: []string{"artifact"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, signet)
	_, ok := ApplyArtifactMana(gs, seat, signet)
	if ok {
		t.Fatalf("Dimir Signet should NOT activate with 0 mana pool")
	}
}

func TestArtifactHasDestructiveCost_LED(t *testing.T) {
	led := &Permanent{Card: &Card{Name: "Lion's Eye Diamond"}}
	if !ArtifactHasDestructiveCost(led) {
		t.Fatalf("LED should be flagged destructive")
	}
	sol := &Permanent{Card: &Card{Name: "Sol Ring"}}
	if ArtifactHasDestructiveCost(sol) {
		t.Fatalf("Sol Ring should NOT be flagged destructive")
	}
}

// --- Test helper -----------------------------------------------------------

func newGameStateForMana(seatCount int) *GameState {
	seats := make([]*Seat, seatCount)
	for i := 0; i < seatCount; i++ {
		seats[i] = newSeat(i)
	}
	return &GameState{
		Seats:    seats,
		Turn:     1,
		Phase:    "beginning",
		Step:     "untap",
		Active:   0,
		Flags:    map[string]int{},
		EventLog: make([]Event, 0, 64),
	}
}
