package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// copy_triggers_sweep_r63_test.go — regressions for the r63 consolidated
// copy-trigger sweep. Every spell-copy maker must route through the canonical
// FireSpellCopyTriggers chokepoint so a magecraft permanent (CR §702.137a:
// "cast OR COPY an instant or sorcery") and "whenever you copy a spell"
// payoffs see the copy. Each test puts a magecraft creature on the copier's
// battlefield and asserts a magecraft_trigger with is_copy=true fires — the
// canonical observable proving FireSpellCopyTriggers ran (it fires magecraft +
// spell_copied together). countMagecraftCopyTriggers lives in
// casualty_behavior_r63_test.go.

// sorcerySpellCard builds a vanilla sorcery carrying the given keyword AST.
func sorcerySpellCard(name string, baseCost int, kws ...gameast.Ability) *Card {
	return &Card{
		Name:  name,
		Owner: 0,
		Types: []string{"sorcery", "cost:" + itoa(baseCost)},
		AST:   &gameast.CardAST{Name: name, Abilities: kws},
	}
}

func TestCopyTriggers_Conspire(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 3
	c1 := addKWCombatBattlefield(gs, 0, "Goblin 1", 2, 2, "creature")
	c1.Card.Colors = []string{"R"}
	c2 := addKWCombatBattlefield(gs, 0, "Goblin 2", 2, 2, "creature")
	c2.Card.Colors = []string{"R"}
	mc := addKWCombatBattlefieldWithKeyword(gs, 0, "Storm-Kiln Artist", 1, 2, "magecraft", "creature")
	mc.Card.Colors = []string{"R"}

	card := instantSpellCard("Conspired Bolt", 3, []string{"R"}, &gameast.Keyword{Name: "conspire"})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("conspire copy must fire magecraft is_copy=true; got %d", got)
	}
}

func TestCopyTriggers_Replicate(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].Hat = &payHat{}
	gs.Seats[0].ManaPool = 6
	addKWCombatBattlefieldWithKeyword(gs, 0, "Archmage Emeritus", 1, 2, "magecraft", "creature")

	card := instantSpellCard("Replicated Bolt", 1, nil,
		&gameast.Keyword{Name: "replicate", Args: []interface{}{float64(1)}})
	gs.Seats[0].Hand = []*Card{card}
	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("replicate copies must fire magecraft is_copy=true; got %d", got)
	}
}

func TestCopyTriggers_Storm(t *testing.T) {
	gs := newKWCombatGame(t)
	addKWCombatBattlefieldWithKeyword(gs, 0, "Storm-Kiln Artist", 1, 2, "magecraft", "creature")
	card := instantSpellCard("Storm Bolt", 1, []string{"R"})
	item := stormStackItem(card, 0)
	if n := ApplyStormCopy(gs, item, 2); n != 2 {
		t.Fatalf("ApplyStormCopy made %d copies, want 2", n)
	}
	if got := countMagecraftCopyTriggers(gs); got < 2 {
		t.Errorf("each storm copy must fire magecraft is_copy=true; got %d, want >=2", got)
	}
}

func TestCopyTriggers_Demonstrate(t *testing.T) {
	gs := newKWCombatGame4P(t)
	addKWCombatBattlefieldWithKeyword(gs, 0, "Symmetry Sage", 1, 2, "magecraft", "creature")
	card := instantSpellCard("Demonstrated Spell", 2, []string{"U"})
	item := &StackItem{Kind: "spell", Card: card, Controller: 0, CastZone: ZoneHand}
	optIn := func() bool { return true }
	oppChoice := func() int { return 1 }
	if n := ApplyDemonstrate(gs, item, 0, optIn, oppChoice); n < 1 {
		t.Fatalf("ApplyDemonstrate made %d copies, want >=1", n)
	}
	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("demonstrate controller copy must fire magecraft is_copy=true; got %d", got)
	}
}

func TestCopyTriggers_Tiered(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Seats[0].ManaPool = 10
	addKWCombatBattlefieldWithKeyword(gs, 0, "Archmage Emeritus", 1, 2, "magecraft", "creature")
	card := instantSpellCard("Tiered Spell", 1, nil,
		&gameast.Keyword{Name: "tiered", Args: []interface{}{float64(1)}})
	item := &StackItem{Kind: "spell", Card: card, Controller: 0, CastZone: ZoneHand}
	item.ID = nextStackID(gs)
	if n := ApplyTiered(gs, item, []int{0}, 1); n < 1 {
		t.Fatalf("ApplyTiered made %d copies, want >=1", n)
	}
	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("tiered copy must fire magecraft is_copy=true; got %d", got)
	}
}

func TestCopyTriggers_Epic(t *testing.T) {
	gs := newKWCombatGame(t)
	gs.Active = 0
	addKWCombatBattlefieldWithKeyword(gs, 0, "Archmage Emeritus", 1, 2, "magecraft", "creature")
	// Epic spells are sorceries (Enduring Ideal etc.) → magecraft applies.
	card := sorcerySpellCard("Enduring Ideal", 6, &gameast.Keyword{Name: "epic"})
	item := &StackItem{Kind: "spell", Card: card, Controller: 0, CastZone: ZoneHand}
	ApplyEpic(gs, 0, item)
	// Fire the registered upkeep delayed trigger directly (the copy maker).
	fired := false
	for _, dt := range gs.DelayedTriggers {
		if dt != nil && dt.TriggerAt == "upkeep" && dt.EffectFn != nil {
			dt.EffectFn(gs)
			fired = true
		}
	}
	if !fired {
		t.Fatal("epic should register an upkeep copy delayed trigger")
	}
	if got := countMagecraftCopyTriggers(gs); got < 1 {
		t.Errorf("epic upkeep copy must fire magecraft is_copy=true; got %d", got)
	}
}
