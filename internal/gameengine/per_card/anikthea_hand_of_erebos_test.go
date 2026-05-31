package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func makeEnchantment(name string, owner, cmc int, subtypes ...string) *gameengine.Card {
	types := append([]string{"enchantment"}, subtypes...)
	c := newCardWithTypes(name, owner, types...)
	c.Types = append(c.Types, "cmc:"+intToStr(cmc))
	return c
}

func TestAnikthea_ETBCopiesEnchantmentAsThreeThreeZombie(t *testing.T) {
	gs := newGame(t, 2)
	anikthea := addPerm(gs, 0, "Anikthea, Hand of Erebos", "creature", "enchantment", "legendary")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		makeEnchantment("Doomwake Giant", 0, 5),
	}
	aniktheaETB(gs, anikthea)
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Doomwake Giant" && p.Card.IsCopy {
			token = p
		}
	}
	if token == nil {
		t.Fatalf("expected a Doomwake Giant copy on battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
	if token.Card.BasePower != 3 || token.Card.BaseToughness != 3 {
		t.Errorf("token must be 3/3, got %d/%d", token.Card.BasePower, token.Card.BaseToughness)
	}
	if !cardHasSubtype(token.Card, "zombie") {
		t.Errorf("token must have zombie type, got %v", token.Card.Types)
	}
	if !cardHasType(token.Card, "creature") {
		t.Errorf("token must have creature type, got %v", token.Card.Types)
	}
	if !cardHasType(token.Card, "enchantment") {
		t.Errorf("token preserves original enchantment type \"in addition to\", got %v", token.Card.Types)
	}
}

func TestAnikthea_AttackTriggerCopiesEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	anikthea := addPerm(gs, 0, "Anikthea, Hand of Erebos", "creature", "enchantment")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		makeEnchantment("Eidolon of Blossoms", 0, 4),
	}
	aniktheaAttack(gs, anikthea, map[string]interface{}{"attacker_perm": anikthea})
	foundCopy := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Eidolon of Blossoms" && p.Card.IsCopy {
			foundCopy = true
		}
	}
	if !foundCopy {
		t.Errorf("expected token copy of Eidolon of Blossoms after attack trigger")
	}
}

func TestAnikthea_AttackNoFireOnNonSelfAttacker(t *testing.T) {
	gs := newGame(t, 2)
	anikthea := addPerm(gs, 0, "Anikthea, Hand of Erebos", "creature", "enchantment")
	other := addPerm(gs, 0, "Other Attacker", "creature")
	gs.Seats[0].Graveyard = []*gameengine.Card{makeEnchantment("X", 0, 1)}
	aniktheaAttack(gs, anikthea, map[string]interface{}{"attacker_perm": other})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.IsCopy {
			t.Errorf("Anikthea must not fire when another creature attacks")
		}
	}
}

func TestAnikthea_PicksHighestCMCNonAuraEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	anikthea := addPerm(gs, 0, "Anikthea, Hand of Erebos", "creature", "enchantment")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		makeEnchantment("Small", 0, 1),
		makeEnchantment("Big", 0, 6),
		makeEnchantment("Aura Token", 0, 4, "aura"),
	}
	aniktheaETB(gs, anikthea)
	foundBig := false
	foundSmallOrAura := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil || !p.Card.IsCopy {
			continue
		}
		switch p.Card.DisplayName() {
		case "Big":
			foundBig = true
		case "Small", "Aura Token":
			foundSmallOrAura = true
		}
	}
	if !foundBig {
		t.Errorf("expected highest-CMC non-Aura enchantment (Big, CMC 6) picked")
	}
	if foundSmallOrAura {
		t.Errorf("Anikthea must not have picked Small or the Aura")
	}
}

func TestAnikthea_NoopOnEmptyGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	anikthea := addPerm(gs, 0, "Anikthea, Hand of Erebos", "creature", "enchantment")
	preCount := len(gs.Seats[0].Battlefield)
	aniktheaETB(gs, anikthea)
	if len(gs.Seats[0].Battlefield) != preCount {
		t.Errorf("expected no battlefield change with empty graveyard")
	}
}
