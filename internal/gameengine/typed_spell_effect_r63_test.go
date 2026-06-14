package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — the production parser emits an instant/sorcery spell body as a Static
// whose Modification.ModKind == "typed_spell_effect" with the typed Effect in
// Args[0]. Before r63 collectSpellEffect / counterSpellEffect only recognized
// the Activated-empty-cost and "spell_effect" shapes, so the ENTIRE
// typed_spell_effect family (Lightning Bolt, Counterspell, Doom Blade, … —
// most real removal and counters) resolved to NOTHING. These corpus-free
// regressions pin the generic extraction so the green-gate protects it even
// without the gitignored AST corpus.

func typedSpellEffectCard(name string, types []string, eff gameast.Effect) *Card {
	return &Card{
		Name:  name,
		Types: types,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "typed_spell_effect",
						Args:    []interface{}{eff},
					},
				},
			},
		},
	}
}

func TestTypedSpellEffect_CollectExtractsDamage(t *testing.T) {
	bolt := typedSpellEffectCard("Bolt",
		[]string{"instant", "cost:1"},
		&gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "any", Quantifier: "any"}})
	eff := collectSpellEffect(bolt)
	if eff == nil {
		t.Fatal("collectSpellEffect must extract the typed_spell_effect body (Damage)")
	}
	if _, ok := eff.(*gameast.Damage); !ok {
		t.Fatalf("expected *Damage, got %T", eff)
	}
}

func TestTypedSpellEffect_CounterRecognized(t *testing.T) {
	counter := typedSpellEffectCard("Counterspell",
		[]string{"instant", "cost:2"},
		&gameast.CounterSpell{Target: gameast.Filter{Base: "spell"}})
	if !CardHasCounterSpell(counter) {
		t.Fatal("CardHasCounterSpell must recognize a typed_spell_effect counter")
	}
	if counterSpellEffect(counter) == nil {
		t.Fatal("counterSpellEffect must extract the typed_spell_effect CounterSpell body")
	}
}

func TestTypedSpellEffect_PermanentSpellStillInert(t *testing.T) {
	// A permanent spell (creature) must NOT have its body returned as a
	// spell-resolution effect (CR §112.1 / §608.3) even if it carries a
	// typed_spell_effect-shaped ability — guards the Cerulean Sphinx class.
	creature := typedSpellEffectCard("Weird Creature",
		[]string{"creature"},
		&gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "any", Quantifier: "any"}})
	if collectSpellEffect(creature) != nil {
		t.Fatal("permanent spells must have no on-resolution spell effect")
	}
}

// End-to-end: a typed_spell_effect burn spell cast through CastSpell now
// actually deals its damage (it dealt zero before r63).
func TestTypedSpellEffect_BoltDealsDamageEndToEnd(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	gs.Phase = "precombat_main"
	gs.Seats[0].ManaPool = 5
	gs.Seats[1].Life = 20
	bolt := typedSpellEffectCard("Bolt",
		[]string{"instant", "cost:1"},
		&gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "any", Quantifier: "any"}})
	bolt.Owner = 0
	gs.Seats[0].Hand = []*Card{bolt}
	if err := CastSpell(gs, 0, bolt, nil); err != nil {
		t.Fatalf("cast failed: %v", err)
	}
	if gs.Seats[1].Life != 17 {
		t.Fatalf("expected seat1 life 17 after a 3-damage bolt, got %d", gs.Seats[1].Life)
	}
}

// The ablation kill-switch genuinely disables the extraction.
func TestTypedSpellEffect_KillSwitch(t *testing.T) {
	bolt := typedSpellEffectCard("Bolt",
		[]string{"instant", "cost:1"},
		&gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "any", Quantifier: "any"}})
	defer func() { typedSpellBodyEnabled = true }()
	typedSpellBodyEnabled = false
	if collectSpellEffect(bolt) != nil {
		t.Fatal("kill-switch must make the typed_spell_effect family inert again")
	}
}
