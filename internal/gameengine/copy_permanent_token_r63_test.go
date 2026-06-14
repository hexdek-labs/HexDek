package gameengine

import (
	"strings"
	"testing"
	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — generic "a copy of a PERMANENT spell becomes a token" path + the
// Storm-to-Aura seam behind Amphibian Downpour.

func amphibAura(name string) *Card {
	return &Card{
		Name: name, Owner: 0, Types: []string{"enchantment", "aura"},
		TypeLine: "Enchantment — Aura",
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "storm"},
				&gameast.Keyword{Name: "enchant creature"},
				&gameast.Static{Modification: &gameast.Modification{ModKind: "aura_loses",
					Args: []interface{}{"all abilities and is a blue frog creature with base power and toughness 1/1"}}},
			},
		},
	}
}

func isFrog(gs *GameState, p *Permanent) bool {
	c := GetEffectiveCharacteristics(gs, p)
	hasFrog := false
	for _, s := range c.Subtypes {
		if s == "Frog" {
			hasFrog = true
		}
	}
	return hasFrog && c.Power == 1 && c.Toughness == 1 && c.LostAllAbilities
}

// (a)+(c) a COPY of an aura permanent spell resolves into a TOKEN aura (does NOT
// cease under §707.10), attaches, and applies its base-layer transform.
func TestCopyPerm_AuraCopyBecomesTokenAndTransforms(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	card := amphibAura("Amphibian Downpour")
	card.IsCopy = true
	item := &StackItem{Card: card, Controller: 0, IsCopy: true}
	item.ID = nextStackID(gs)
	perm := resolvePermanentSpellETB(gs, item)
	if perm == nil || !perm.IsToken() || !perm.IsAura() {
		t.Fatalf("aura copy must become a TOKEN aura permanent, got %+v", perm)
	}
	if perm.AttachedTo != bear {
		t.Fatalf("token aura must attach to the legal creature")
	}
	StateBasedActions(gs)
	if !isFrog(gs, bear) {
		t.Fatalf("enchanted creature must become a blue Frog 1/1 with no abilities; P/T=%d/%d",
			gs.PowerOf(bear), gs.ToughnessOf(bear))
	}
}

// (d) when the token aura ceases (SBA token cleanup), the creature REVERTS.
func TestCopyPerm_RevertsWhenTokenAuraCeases(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	card := amphibAura("Amphibian Downpour")
	card.IsCopy = true
	item := &StackItem{Card: card, Controller: 0, IsCopy: true}
	item.ID = nextStackID(gs)
	perm := resolvePermanentSpellETB(gs, item)
	StateBasedActions(gs)
	if !isFrog(gs, bear) {
		t.Fatal("setup: bear should be a Frog")
	}
	// Remove the token aura (sacrifice/destroy → graveyard → token ceases).
	SacrificePermanent(gs, perm, "test_remove")
	StateBasedActions(gs)
	if gs.PowerOf(bear) != 2 || gs.ToughnessOf(bear) != 2 {
		t.Fatalf("creature must REVERT to 2/2 after the token aura ceases; got %d/%d",
			gs.PowerOf(bear), gs.ToughnessOf(bear))
	}
	if GetEffectiveCharacteristics(gs, bear).LostAllAbilities {
		t.Fatal("creature must regain its abilities after reverting")
	}
}

// (b) a token aura with NO legal target does not stay on the battlefield.
func TestCopyPerm_NoLegalTargetCeases(t *testing.T) {
	gs := newFixtureGame(t)
	// No creatures anywhere — "enchant creature" has no legal object.
	card := amphibAura("Amphibian Downpour")
	card.IsCopy = true
	item := &StackItem{Card: card, Controller: 0, IsCopy: true}
	item.ID = nextStackID(gs)
	perm := resolvePermanentSpellETB(gs, item)
	StateBasedActions(gs)
	for si := range gs.Seats {
		for _, p := range gs.Seats[si].Battlefield {
			if p == perm {
				t.Fatal("a token aura with no legal target must not remain on the battlefield")
			}
		}
	}
}

// CAPSTONE (a–e): Amphibian Downpour end-to-end via STORM. With N prior spells
// this turn, storm makes N token-Aura copies; each becomes a Frog attached to a
// distinct creature; the storm count (all-players, minus self) holds; each
// reverts when removed.
func TestCopyPerm_AmphibianDownpourStormCapstone(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Active = 0
	// 3 opponent creatures to frog.
	c1 := addBattlefield(gs, 1, "Bear A", 3, 3, "creature")
	c2 := addBattlefield(gs, 1, "Bear B", 4, 4, "creature")
	c3 := addBattlefield(gs, 1, "Bear C", 5, 5, "creature")

	// Two prior spells cast this turn (any players) + the Downpour itself.
	IncrementCastCount(gs, 0)
	IncrementCastCount(gs, 1)
	IncrementCastCount(gs, 0) // the storm spell

	card := amphibAura("Amphibian Downpour")
	original := &StackItem{Card: card, Controller: 0}
	original.ID = nextStackID(gs)
	gs.Stack = append(gs.Stack, original)

	// Storm: copies become tokens (the rider). StormCount = 2 (other spells).
	if n := ApplyStormCopies(gs, original, 0); n != 2 {
		t.Fatalf("storm must make 2 copies (2 prior spells), got %d", n)
	}
	// Drain the stack — original + 2 token-aura copies resolve.
	DrainStack(gs)
	StateBasedActions(gs)

	// 3 Amphibian Downpour permanents on the battlefield (1 real + 2 tokens),
	// 2 of which are tokens.
	auras, tokenAuras := 0, 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && strings.HasPrefix(p.Card.DisplayName(), "Amphibian Downpour") {
			auras++
			if p.IsToken() {
				tokenAuras++
			}
		}
	}
	if auras != 3 || tokenAuras != 2 {
		t.Fatalf("want 3 Downpour auras (2 token copies + 1 original); got %d auras, %d tokens", auras, tokenAuras)
	}
	// All three creatures are now blue Frog 1/1 (spread across distinct targets).
	frogs := 0
	for _, c := range []*Permanent{c1, c2, c3} {
		if isFrog(gs, c) {
			frogs++
		}
	}
	if frogs != 3 {
		t.Fatalf("all 3 enchanted creatures must be Frog 1/1; got %d frogs", frogs)
	}
	// Remove every Downpour aura → all creatures revert.
	for _, p := range append([]*Permanent(nil), gs.Seats[0].Battlefield...) {
		if p.Card != nil && strings.HasPrefix(p.Card.DisplayName(), "Amphibian Downpour") {
			SacrificePermanent(gs, p, "capstone_cleanup")
		}
	}
	StateBasedActions(gs)
	if gs.PowerOf(c1) != 3 || gs.PowerOf(c2) != 4 || gs.PowerOf(c3) != 5 {
		t.Fatalf("creatures must revert to 3/4/5 power after auras gone; got %d/%d/%d",
			gs.PowerOf(c1), gs.PowerOf(c2), gs.PowerOf(c3))
	}
}
