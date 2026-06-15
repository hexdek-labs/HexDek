package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 bestow probe — CR §702.103. While a bestow permanent is attached as
// an Aura it is an enchantment — Aura and is NOT a creature; when the
// enchanted creature leaves the battlefield the bestow permanent becomes a
// creature (it stays on the battlefield, unattached) rather than dying.

func bf_bestowCard(gs *GameState, seat int, name string, bestowCost, pow, tough int) *Card {
	c := &Card{
		Name: name, Owner: seat, Types: []string{"enchantment", "creature"},
		BasePower: pow, BaseToughness: tough, CMC: 3,
		AST: &gameast.CardAST{Name: name,
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "bestow", Args: []interface{}{float64(bestowCost)}}}},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

func bf_findPerm(gs *GameState, seat int, card *Card) *Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p.Card == card {
			return p
		}
	}
	return nil
}

// (a)/(c): cast for the bestow cost → attached Aura that is NOT a creature,
// grants +P/+T to the host.
func TestBestow_AttachedIsAuraNotCreature(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 10
	card := bf_bestowCard(gs, 0, "Boon Satyr", 5, 4, 2)
	target := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")

	if err := CastWithBestow(gs, 0, card, target); err != nil {
		t.Fatalf("CastWithBestow failed: %v", err)
	}
	perm := bf_findPerm(gs, 0, card)
	if perm == nil {
		t.Fatal("bestow permanent should be on the battlefield")
	}
	// (c) — it is ONLY an Aura while attached.
	if perm.IsCreature() {
		t.Error("a bestowed permanent must NOT be a creature (CR §702.103a)")
	}
	if !perm.IsAura() {
		t.Error("a bestowed permanent must be an Aura while attached")
	}
	if !perm.IsEnchantment() {
		t.Error("a bestowed permanent must be an enchantment while attached")
	}
	// (a) — the host got +4/+2.
	if target.Power() != 6 || target.Toughness() != 4 {
		t.Errorf("host should be 6/4 after bestow (+4/+2), got %d/%d", target.Power(), target.Toughness())
	}
}

// (d): host leaves → bestow permanent BECOMES a creature (stays on the
// battlefield, unattached), it does NOT go to the graveyard.
func TestBestow_HostLeaves_BecomesCreatureNotGraveyard(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 10
	card := bf_bestowCard(gs, 0, "Boon Satyr", 5, 4, 2)
	target := addP0Battlefield(gs, 0, "Bear", 2, 2, "creature")
	if err := CastWithBestow(gs, 0, card, target); err != nil {
		t.Fatalf("CastWithBestow failed: %v", err)
	}

	// The host creature leaves the battlefield.
	DestroyPermanent(gs, target, nil)
	StateBasedActions(gs)

	perm := bf_findPerm(gs, 0, card)
	if perm == nil {
		t.Fatal("bestow permanent must remain on the battlefield as a creature (CR §702.103e), not die")
	}
	// In the graveyard? That's the pre-fix bug.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("bestow permanent must NOT be put into the graveyard when its host leaves")
		}
	}
	// It is now an ordinary creature, unattached.
	if !perm.IsCreature() {
		t.Error("after falloff the bestow permanent must be a creature again")
	}
	if perm.IsAura() {
		t.Error("after falloff the bestow permanent must no longer be an Aura")
	}
	if perm.AttachedTo != nil {
		t.Error("after falloff AttachedTo must be cleared")
	}
}

// (b): cast for its NORMAL mana cost (NOT via CastWithBestow) → it is an
// ordinary enchantment creature, not an Aura.
func TestBestow_NormalCast_IsCreatureNotAura(t *testing.T) {
	gs := newP0Game(t)
	// Simulate a normal-cast bestow creature already on the battlefield
	// (no bestowed flag).
	p := addP0Battlefield(gs, 0, "Boon Satyr", 4, 2, "enchantment", "creature")
	if !p.IsCreature() {
		t.Error("a normally-cast bestow card is a creature")
	}
	if p.IsAura() {
		t.Error("a normally-cast bestow card is NOT an Aura")
	}
}
