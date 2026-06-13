package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mr2_cover_r60_test.go — regression pins for shard M-R batches 3 (more
// counterspells) and 4 (destroy-two removal).

func mr2Opp(gs *gameengine.GameState, name string, types ...string) *gameengine.StackItem {
	si := &gameengine.StackItem{
		Controller: 1,
		Card:       &gameengine.Card{Name: name, Owner: 1, Types: append([]string{}, types...)},
	}
	gs.Stack = append(gs.Stack, si)
	return si
}

func mr2Cast(gs *gameengine.GameState, name string) {
	card := addCard(gs, 0, name, "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
}

func TestNeutralize_CountersAny(t *testing.T) {
	gs := newGame(t, 2)
	opp := mr2Opp(gs, "Cultivate", "sorcery")
	mr2Cast(gs, "Neutralize")
	if !opp.Countered {
		t.Errorf("Neutralize should counter any spell")
	}
}

func TestMiscast_OnlyInstantSorcery(t *testing.T) {
	gs := newGame(t, 2)
	cre := mr2Opp(gs, "Bear", "creature")
	mr2Cast(gs, "Miscast")
	if cre.Countered {
		t.Errorf("Miscast must not counter a creature spell")
	}
	gs2 := newGame(t, 2)
	bolt := mr2Opp(gs2, "Lightning Bolt", "instant")
	mr2Cast(gs2, "Miscast")
	if !bolt.Countered {
		t.Errorf("Miscast should counter an instant")
	}
}

func TestNeutralizingBlast_OnlyMulticolored(t *testing.T) {
	gs := newGame(t, 2)
	mono := mr2Opp(gs, "Mono Spell", "instant")
	mono.Card.Colors = []string{"U"}
	mr2Cast(gs, "Neutralizing Blast")
	if mono.Countered {
		t.Errorf("Neutralizing Blast must not counter a mono-colored spell")
	}
	gs2 := newGame(t, 2)
	multi := mr2Opp(gs2, "Gold Spell", "instant")
	multi.Card.Colors = []string{"U", "R"}
	mr2Cast(gs2, "Neutralizing Blast")
	if !multi.Countered {
		t.Errorf("Neutralizing Blast should counter a multicolored spell")
	}
}

func TestRevolutionaryRebuff_NonArtifactOnly(t *testing.T) {
	gs := newGame(t, 2)
	art := mr2Opp(gs, "Sol Ring", "artifact")
	mr2Cast(gs, "Revolutionary Rebuff")
	if art.Countered {
		t.Errorf("Revolutionary Rebuff must not counter an artifact spell")
	}
	gs2 := newGame(t, 2)
	nonart := mr2Opp(gs2, "Bear", "creature")
	mr2Cast(gs2, "Revolutionary Rebuff")
	if !nonart.Countered {
		t.Errorf("Revolutionary Rebuff should counter a nonartifact spell")
	}
}

func TestRethinkOverride_CounterUnlessPay(t *testing.T) {
	for _, name := range []string{"Rethink", "Override", "Oppressive Will", "Rakshasa's Disdain", "Mindstatic", "Miscalculation"} {
		gs := newGame(t, 2)
		opp := mr2Opp(gs, "Some Spell", "sorcery")
		mr2Cast(gs, name)
		if !opp.Countered {
			t.Errorf("%s should counter (engine no-pay convention)", name)
		}
	}
}

// --- destroy-two removal ---

func mr2Perm(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	p := addPerm(gs, seat, name, types...)
	p.Card.BaseToughness = 3 // survive SBA
	return p
}

func TestPeaceAndQuiet_DestroysTwoEnchantments(t *testing.T) {
	gs := newGame(t, 2)
	mr2Perm(gs, 1, "Enchantment A", "enchantment")
	mr2Perm(gs, 1, "Enchantment B", "enchantment")
	mr2Perm(gs, 1, "Enchantment C", "enchantment")
	mr2Perm(gs, 1, "Bear", "creature") // not targeted
	mr2Cast(gs, "Peace and Quiet")
	ench := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsEnchantment() {
			ench++
		}
	}
	if ench != 1 {
		t.Errorf("enchantments left = %d, want 1 (2 destroyed)", ench)
	}
}

func TestRackAndRuin_DestroysTwoArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	mr2Perm(gs, 1, "Sol Ring", "artifact")
	mr2Perm(gs, 1, "Mox", "artifact")
	mr2Cast(gs, "Rack and Ruin")
	arts := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsArtifact() {
			arts++
		}
	}
	if arts != 0 {
		t.Errorf("artifacts left = %d, want 0", arts)
	}
}

func TestRainOfSalt_DestroysTwoLands(t *testing.T) {
	gs := newGame(t, 2)
	mr2Perm(gs, 1, "Forest", "land")
	mr2Perm(gs, 1, "Island", "land")
	mr2Perm(gs, 1, "Mountain", "land")
	mr2Cast(gs, "Rain of Salt")
	lands := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsLand() {
			lands++
		}
	}
	if lands != 1 {
		t.Errorf("lands left = %d, want 1 (2 destroyed)", lands)
	}
}
