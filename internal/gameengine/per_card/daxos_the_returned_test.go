package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestDaxos_GainsExperienceOnEnchantmentCast(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos the Returned", "creature")
	enchant := newCardWithTypes("Sphere of Safety", 0, "enchantment")
	daxosEnchantmentCast(gs, daxos, map[string]interface{}{
		"caster_seat": 0,
		"card":        enchant,
	})
	if gs.Seats[0].Flags["experience_counters"] != 1 {
		t.Errorf("expected 1 XP after own enchantment cast, got %d", gs.Seats[0].Flags["experience_counters"])
	}
}

func TestDaxos_DoesNotFireOnNonEnchantmentCast(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos the Returned", "creature")
	creature := newCardWithTypes("Llanowar Elves", 0, "creature")
	daxosEnchantmentCast(gs, daxos, map[string]interface{}{
		"caster_seat": 0,
		"card":        creature,
	})
	if gs.Seats[0].Flags["experience_counters"] != 0 {
		t.Errorf("XP must not increment on non-enchantment cast")
	}
}

func TestDaxos_DoesNotFireOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos the Returned", "creature")
	enchant := newCardWithTypes("Sphere of Safety", 1, "enchantment")
	daxosEnchantmentCast(gs, daxos, map[string]interface{}{
		"caster_seat": 1,
		"card":        enchant,
	})
	if gs.Seats[0].Flags["experience_counters"] != 0 {
		t.Errorf("Daxos's XP must scope to controller's own casts")
	}
}

func TestDaxos_SpiritTokenScalesWithExperience(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos the Returned", "creature")
	gs.Seats[0].Flags = map[string]int{"experience_counters": 5}

	daxosActivate(gs, daxos, 0, nil)

	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Spirit Token" {
			token = p
		}
	}
	if token == nil {
		t.Fatalf("expected Spirit Token on battlefield")
	}
	if token.Power() != 5 || token.Toughness() != 5 {
		t.Errorf("expected Spirit Token at 5/5 (XP=5), got %d/%d", token.Power(), token.Toughness())
	}
}

func TestDaxos_SpiritTokenWithZeroXPIsZeroSlashZero(t *testing.T) {
	gs := newGame(t, 2)
	daxos := addPerm(gs, 0, "Daxos the Returned", "creature")
	daxosActivate(gs, daxos, 0, nil)
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Spirit Token" {
			token = p
		}
	}
	if token == nil {
		t.Fatal("expected Spirit Token")
	}
	if token.Power() != 0 || token.Toughness() != 0 {
		t.Errorf("Spirit Token at 0 XP must be 0/0, got %d/%d", token.Power(), token.Toughness())
	}
}
