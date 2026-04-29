package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// newCounterTestGame creates a 2-seat game for counter tests.
func newCounterTestGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	return gs
}

// pushSpell pushes a spell onto the stack with the given card types and
// controller, returning the stack item.
func pushSpell(gs *GameState, controller int, name string, types ...string) *StackItem {
	card := &Card{
		Name:  name,
		Owner: controller,
		Types: types,
	}
	item := &StackItem{
		Controller: controller,
		Card:       card,
	}
	PushStackItem(gs, item)
	return item
}

// pushSpellWithColors pushes a spell with both types and colors.
func pushSpellWithColors(gs *GameState, controller int, name string, colors []string, types ...string) *StackItem {
	card := &Card{
		Name:   name,
		Owner:  controller,
		Types:  types,
		Colors: colors,
	}
	item := &StackItem{
		Controller: controller,
		Card:       card,
	}
	PushStackItem(gs, item)
	return item
}

// pushAbility pushes a triggered/activated ability onto the stack.
func pushAbility(gs *GameState, controller int, name string, kind string) *StackItem {
	card := &Card{Name: name, Owner: controller}
	perm := &Permanent{Card: card, Controller: controller}
	item := &StackItem{
		Controller: controller,
		Source:     perm,
		Kind:       kind,
	}
	PushStackItem(gs, item)
	return item
}

// hasEvent checks if the event log contains an event of the given kind.
func hasEvent(gs *GameState, kind string) bool {
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

// hasEventWithDetail checks for an event with a specific detail key/value.
func hasEventWithDetail(gs *GameState, kind, key, value string) bool {
	for _, ev := range gs.EventLog {
		if ev.Kind != kind {
			continue
		}
		if v, ok := ev.Details[key]; ok {
			if s, ok := v.(string); ok && s == value {
				return true
			}
		}
	}
	return false
}

// ============================================================================
// Test 1: Generic counter — "Counter target spell" (Counterspell)
// ============================================================================

func TestGenericCounter_AnySpell(t *testing.T) {
	gs := newCounterTestGame(t)
	target := pushSpell(gs, 1, "Lightning Bolt", "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("expected counter to succeed")
	}
	if !target.Countered {
		t.Fatal("expected target to be marked countered")
	}
	if !hasEvent(gs, "counter_spell") {
		t.Fatal("expected counter_spell event")
	}
}

// ============================================================================
// Test 2: Noncreature filter — "Counter target noncreature spell" (Negate)
// ============================================================================

func TestGenericCounter_NoncreatureFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a creature spell — should NOT be counterable.
	creature := pushSpell(gs, 1, "Grizzly Bears", "creature")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{
			Base:  "spell",
			Extra: []string{"non-creature"},
		},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("noncreature counter should NOT counter a creature spell")
	}
	if creature.Countered {
		t.Fatal("creature should not be marked countered")
	}

	// Now push a sorcery — should be counterable.
	sorcery := pushSpell(gs, 1, "Wrath of God", "sorcery")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("noncreature counter should counter a sorcery")
	}
	if !sorcery.Countered {
		t.Fatal("sorcery should be marked countered")
	}
}

// ============================================================================
// Test 3: Creature filter — "Counter target creature spell" (Essence Scatter)
// ============================================================================

func TestGenericCounter_CreatureFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a sorcery — should NOT match.
	pushSpell(gs, 1, "Wrath of God", "sorcery")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "creature"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("creature counter should NOT counter a sorcery")
	}

	// Push a creature spell — should match.
	creature := pushSpell(gs, 1, "Grizzly Bears", "creature")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("creature counter should counter a creature spell")
	}
	if !creature.Countered {
		t.Fatal("creature should be marked countered")
	}
}

// ============================================================================
// Test 4: Instant filter — "Counter target instant spell" (Dispel)
// ============================================================================

func TestGenericCounter_InstantFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a sorcery — should NOT match.
	pushSpell(gs, 1, "Thoughtseize", "sorcery")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "instant"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("instant counter should NOT counter a sorcery")
	}

	// Push an instant — should match.
	instant := pushSpell(gs, 1, "Lightning Bolt", "instant")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("instant counter should counter an instant")
	}
	if !instant.Countered {
		t.Fatal("instant should be marked countered")
	}
}

// ============================================================================
// Test 5: Enchantment filter — "Counter target enchantment spell"
// ============================================================================

func TestGenericCounter_EnchantmentFilter(t *testing.T) {
	gs := newCounterTestGame(t)
	pushSpell(gs, 1, "Lightning Bolt", "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "enchantment"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("enchantment counter should NOT counter an instant")
	}

	enchant := pushSpell(gs, 1, "Rhystic Study", "enchantment")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("enchantment counter should counter an enchantment")
	}
	if !enchant.Countered {
		t.Fatal("enchantment should be marked countered")
	}
}

// ============================================================================
// Test 6: Artifact filter — "Counter target artifact spell"
// ============================================================================

func TestGenericCounter_ArtifactFilter(t *testing.T) {
	gs := newCounterTestGame(t)
	pushSpell(gs, 1, "Lightning Bolt", "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "artifact"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("artifact counter should NOT counter an instant")
	}

	artifact := pushSpell(gs, 1, "Sol Ring", "artifact")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("artifact counter should counter an artifact")
	}
	if !artifact.Countered {
		t.Fatal("artifact should be marked countered")
	}
}

// ============================================================================
// Test 7: Ability counter — "Counter target activated ability" (Stifle/Trickbind)
// ============================================================================

func TestGenericCounter_ActivatedAbilityFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a spell — should NOT match activated ability filter.
	pushSpell(gs, 1, "Lightning Bolt", "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "activated"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("activated ability counter should NOT counter a spell")
	}

	// Push an activated ability — should match.
	ability := pushAbility(gs, 1, "Sensei's Divining Top", "activated")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("activated ability counter should counter an activated ability")
	}
	if !ability.Countered {
		t.Fatal("ability should be marked countered")
	}
}

// ============================================================================
// Test 8: "Can't be countered" — Dovin's Veto, Cavern of Souls
// ============================================================================

func TestGenericCounter_CannotBeCountered_CostMeta(t *testing.T) {
	gs := newCounterTestGame(t)

	target := pushSpell(gs, 1, "Dovin's Veto", "instant")
	target.CostMeta = map[string]interface{}{"cannot_be_countered": true}

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("should not counter an uncounterable spell")
	}
	if target.Countered {
		t.Fatal("uncounterable spell should NOT be marked countered")
	}
	if !hasEventWithDetail(gs, "counter_spell_blocked", "reason", "cannot_be_countered") {
		t.Fatal("expected counter_spell_blocked event")
	}
}

func TestGenericCounter_CannotBeCountered_ASTCheck(t *testing.T) {
	gs := newCounterTestGame(t)

	// Create a card with "this spell can't be countered" in its AST.
	card := &Card{
		Name:  "Abrupt Decay",
		Owner: 1,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Name: "Abrupt Decay",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "parsed_tail",
						Args:    []interface{}{"this spell can't be countered"},
					},
				},
			},
		},
	}
	target := &StackItem{Controller: 1, Card: card}
	PushStackItem(gs, target)

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("should not counter a spell with 'can't be countered' in AST")
	}
}

// ============================================================================
// Test 9: Unless-pay — "Counter unless controller pays {3}" (Mana Leak)
// ============================================================================

func TestGenericCounter_UnlessPay_OpponentPays(t *testing.T) {
	gs := newCounterTestGame(t)

	target := pushSpell(gs, 1, "Lightning Bolt", "instant")
	// Give opponent enough mana to pay.
	gs.Seats[1].ManaPool = 5

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
		Unless: &gameast.Cost{
			Mana: &gameast.ManaCost{
				Symbols: []gameast.ManaSymbol{{Generic: 3}},
			},
		},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("opponent should have paid the unless cost")
	}
	if target.Countered {
		t.Fatal("spell should NOT be countered when opponent pays")
	}
	// Check mana was deducted.
	if gs.Seats[1].ManaPool != 2 {
		t.Fatalf("expected opponent mana pool = 2 after paying 3, got %d", gs.Seats[1].ManaPool)
	}
}

func TestGenericCounter_UnlessPay_OpponentCantPay(t *testing.T) {
	gs := newCounterTestGame(t)

	target := pushSpell(gs, 1, "Lightning Bolt", "instant")
	// Opponent doesn't have enough mana.
	gs.Seats[1].ManaPool = 1

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
		Unless: &gameast.Cost{
			Mana: &gameast.ManaCost{
				Symbols: []gameast.ManaSymbol{{Generic: 3}},
			},
		},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("counter should succeed when opponent can't pay")
	}
	if !target.Countered {
		t.Fatal("spell should be countered when opponent can't pay")
	}
}

// ============================================================================
// Test 10: Color-based filter — "Counter target black spell" (Lifeforce)
// ============================================================================

func TestGenericCounter_ColorFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a red spell — should NOT match "black" filter.
	pushSpellWithColors(gs, 1, "Lightning Bolt", []string{"R"}, "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "black"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("black counter should NOT counter a red spell")
	}

	// Push a black spell — should match.
	blackSpell := pushSpellWithColors(gs, 1, "Dark Ritual", []string{"B"}, "instant")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("black counter should counter a black spell")
	}
	if !blackSpell.Countered {
		t.Fatal("black spell should be marked countered")
	}
}

// ============================================================================
// Test 11: Own spell not counterable by self
// ============================================================================

func TestGenericCounter_WontCounterOwnSpell(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a spell controlled by the caster (seat 0).
	pushSpell(gs, 0, "Lightning Bolt", "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("should not counter your own spell")
	}
}

// ============================================================================
// Test 12: Empty stack fizzles
// ============================================================================

func TestGenericCounter_EmptyStackFizzles(t *testing.T) {
	gs := newCounterTestGame(t)

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("should fizzle on empty stack")
	}
	if !hasEvent(gs, "counter_spell_fizzle") {
		t.Fatal("expected counter_spell_fizzle event")
	}
}

// ============================================================================
// Test 13: Colorless filter — "Counter target colorless spell" (Ceremonious Rejection)
// ============================================================================

func TestGenericCounter_ColorlessFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a colored spell — should NOT match.
	pushSpellWithColors(gs, 1, "Lightning Bolt", []string{"R"}, "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{
			Base:  "spell",
			Extra: []string{"colorless"},
		},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("colorless counter should NOT counter a red spell")
	}

	// Push a colorless spell — should match.
	colorless := pushSpellWithColors(gs, 1, "Sol Ring", []string{}, "artifact")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("colorless counter should counter a colorless spell")
	}
	if !colorless.Countered {
		t.Fatal("colorless spell should be marked countered")
	}
}

// ============================================================================
// Test 14: CounterCanTarget — Hat filter matching
// ============================================================================

func TestCounterCanTarget_FilterAwareness(t *testing.T) {
	// Negate-style: noncreature only.
	negateEffect := &gameast.CounterSpell{
		Target: gameast.Filter{
			Base:  "spell",
			Extra: []string{"non-creature"},
		},
	}

	creatureItem := &StackItem{
		Controller: 1,
		Card:       &Card{Name: "Grizzly Bears", Types: []string{"creature"}},
	}
	instantItem := &StackItem{
		Controller: 1,
		Card:       &Card{Name: "Lightning Bolt", Types: []string{"instant"}},
	}

	if CounterCanTarget(negateEffect, creatureItem) {
		t.Fatal("Negate should NOT be able to target a creature spell")
	}
	if !CounterCanTarget(negateEffect, instantItem) {
		t.Fatal("Negate should be able to target an instant")
	}

	// Dispel-style: instant only.
	dispelEffect := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "instant"},
	}
	sorceryItem := &StackItem{
		Controller: 1,
		Card:       &Card{Name: "Wrath of God", Types: []string{"sorcery"}},
	}

	if CounterCanTarget(dispelEffect, sorceryItem) {
		t.Fatal("Dispel should NOT be able to target a sorcery")
	}
	if !CounterCanTarget(dispelEffect, instantItem) {
		t.Fatal("Dispel should be able to target an instant")
	}
}

// ============================================================================
// Test 15: Multicolored filter — "Counter target multicolored spell"
// ============================================================================

func TestGenericCounter_MulticoloredFilter(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a mono-colored spell — should NOT match.
	pushSpellWithColors(gs, 1, "Lightning Bolt", []string{"R"}, "instant")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{
			Base:  "spell",
			Extra: []string{"multicolored"},
		},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if ok {
		t.Fatal("multicolored counter should NOT counter a mono-colored spell")
	}

	// Push a multicolored spell — should match.
	multi := pushSpellWithColors(gs, 1, "Azorius Charm", []string{"W", "U"}, "instant")
	ok = ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("multicolored counter should counter a multicolored spell")
	}
	if !multi.Countered {
		t.Fatal("multicolored spell should be marked countered")
	}
}

// ============================================================================
// Test 16: ExtractCounterSpellNode — walks nested sequences
// ============================================================================

func TestExtractCounterSpellNode_Sequence(t *testing.T) {
	inner := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}
	seq := &gameast.Sequence{
		Items: []gameast.Effect{
			&gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}},
			inner,
		},
	}

	result := ExtractCounterSpellNode(seq)
	if result == nil {
		t.Fatal("should find CounterSpell inside a Sequence")
	}
	if result != inner {
		t.Fatal("should return the exact CounterSpell pointer")
	}
}

func TestExtractCounterSpellNode_Nil(t *testing.T) {
	result := ExtractCounterSpellNode(nil)
	if result != nil {
		t.Fatal("nil effect should return nil")
	}
}

// ============================================================================
// Test 17: counterSpellEffect — Static/spell_effect layout detection
// ============================================================================

func TestCounterSpellEffect_StaticLayout(t *testing.T) {
	// Simulate a real corpus card: Static ability with spell_effect modification.
	innerCS := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}
	card := &Card{
		Name: "Counterspell",
		AST: &gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "spell_effect",
						Args:    []interface{}{gameast.Effect(innerCS)},
					},
				},
			},
		},
	}

	eff := counterSpellEffect(card)
	if eff == nil {
		t.Fatal("counterSpellEffect should detect Static/spell_effect layout")
	}
	if _, ok := eff.(*gameast.CounterSpell); !ok {
		t.Fatal("returned effect should be a *CounterSpell")
	}
}

func TestCounterSpellEffect_ActivatedLayout(t *testing.T) {
	// Legacy test layout: Activated ability with CounterSpell effect.
	innerCS := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}
	card := &Card{
		Name: "Counterspell",
		AST: &gameast.CardAST{
			Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: innerCS,
				},
			},
		},
	}

	eff := counterSpellEffect(card)
	if eff == nil {
		t.Fatal("counterSpellEffect should detect Activated layout")
	}
}

// ============================================================================
// Test 18: Triggered ability counter — "Counter all abilities" (Kadena's Silencer)
// ============================================================================

func TestGenericCounter_AllAbilities(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push a triggered ability.
	triggered := pushAbility(gs, 1, "Rhystic Study", "triggered")

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "abilities", Quantifier: "all"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("abilities counter should counter a triggered ability")
	}
	if !triggered.Countered {
		t.Fatal("triggered ability should be marked countered")
	}
}

// ============================================================================
// Test 19: Skips already-countered items on stack
// ============================================================================

func TestGenericCounter_SkipsAlreadyCountered(t *testing.T) {
	gs := newCounterTestGame(t)

	// Push two spells, counter the top one.
	bottom := pushSpell(gs, 1, "Dark Ritual", "instant")
	top := pushSpell(gs, 1, "Lightning Bolt", "instant")
	top.Countered = true

	cs := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell"},
	}

	ok := ResolveCounterSpellGeneric(gs, 0, cs)
	if !ok {
		t.Fatal("should find the uncountered bottom spell")
	}
	if !bottom.Countered {
		t.Fatal("bottom spell should be marked countered")
	}
}
