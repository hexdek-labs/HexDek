package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// makeCreatureManaDork builds a creature with one tap-for-mana activated
// ability (a mana dork) plus a non-mana activated ability at index 1.
func makeCreatureManaDork(gs *GameState, seat int, name string) *Permanent {
	card := &Card{
		Name: name, Owner: seat,
		Types:         []string{"creature"},
		BasePower:     1,
		BaseToughness: 1,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: &gameast.AddMana{AnyColorCount: 1}},
				&gameast.Activated{Cost: gameast.Cost{Tap: true}, Effect: &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}},
			},
		},
	}
	perm := &Permanent{
		Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

// (d) loses-all-abilities removes ACTIVATED and MANA abilities: a stripped
// mana dork can neither tap for mana (idx 0) nor activate its non-mana
// ability (idx 1). Without a strip both are activatable.
func TestProbe_StripSuppressesActivatedAndManaAbilities(t *testing.T) {
	gs := newFixtureGame(t)
	dork := makeCreatureManaDork(gs, 0, "Llanowar-like Dork")

	// Control: no strip → neither ability is suppressed.
	if StaxCheck(gs, 0, dork, 0).Suppressed {
		t.Fatal("mana ability should be activatable with no strip active")
	}
	if StaxCheck(gs, 0, dork, 1).Suppressed {
		t.Fatal("non-mana ability should be activatable with no strip active")
	}

	// Humility enters → strips all abilities.
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	gs.InvalidateCharacteristicsCache()

	mana := StaxCheck(gs, 0, dork, 0)
	if !mana.Suppressed || mana.Reason != "abilities_removed" {
		t.Errorf("(d) stripped creature's MANA ability must be suppressed; got %+v", mana)
	}
	nonMana := StaxCheck(gs, 0, dork, 1)
	if !nonMana.Suppressed || nonMana.Reason != "abilities_removed" {
		t.Errorf("(d) stripped creature's activated ability must be suppressed; got %+v", nonMana)
	}

	// End-to-end: ActivateAbility rejects it.
	gs.Seats[0].ManaPool = 10
	if err := ActivateAbility(gs, 0, dork, 0, nil); err == nil {
		t.Errorf("(d) ActivateAbility must reject a stripped creature's mana ability")
	}
}

// Probe harness for CR §613 multi-transform layer composition.
// Registers raw continuous effects so we exercise the engine pipeline
// directly (independent of which cards have handlers).

func registerStripAbilities(gs *GameState, target *Permanent, srcName string) *ContinuousEffect {
	return gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerAbility, Timestamp: gs.NextTimestamp(),
		SourcePerm: target, SourceCardName: srcName,
		ControllerSeat: target.Controller,
		HandlerID:      layerHandlerKey("probe_strip_"+srcName, target),
		Predicate:      func(_ *GameState, p *Permanent) bool { return p == target },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Abilities = nil
			chars.Keywords = nil
			chars.LostAllAbilities = true
		},
	})
}

// (b) A separate layer-7b base-P/T-set must survive a layer-6 strip.
func TestProbe_StripDoesNotWipeBasePTSet(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.GrantedAbilities = []string{"flying"}
	RegisterSetPT(gs, bear, 3, 3, "permanent", "ProbeSet", "probe")
	registerStripAbilities(gs, bear, "ProbeStrip")
	gs.InvalidateCharacteristicsCache()
	c := GetEffectiveCharacteristics(gs, bear)
	if c.Power != 3 || c.Toughness != 3 {
		t.Errorf("base-PT-set 3/3 must survive ability strip; got %d/%d", c.Power, c.Toughness)
	}
	if gs.HasKeywordOf(bear, "flying") {
		t.Errorf("strip should remove granted flying")
	}
	if !charsHaveType(c.Types, "creature") {
		t.Errorf("(d) strip must NOT remove the creature card type")
	}
}

// (b) An ability granted by a LATER-timestamp effect than the strip survives.
func TestProbe_LaterGrantSurvivesEarlierStrip(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	registerStripAbilities(gs, bear, "EarlyStrip") // ts = T1
	RegisterGrantKeyword(gs, bear, "trample", "permanent", "LateGrant", "probe") // ts = T2 > T1
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(bear, "trample") {
		t.Errorf("(b) a grant with a later timestamp than the strip must survive (§613.7 timestamp order)")
	}
}

// (b) inverse: a grant EARLIER than the strip is wiped by it.
func TestProbe_EarlierGrantWipedByLaterStrip(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	RegisterGrantKeyword(gs, bear, "trample", "permanent", "EarlyGrant", "probe") // ts = T1
	registerStripAbilities(gs, bear, "LateStrip")                                  // ts = T2 > T1
	gs.InvalidateCharacteristicsCache()
	if gs.HasKeywordOf(bear, "trample") {
		t.Errorf("a grant earlier than the strip must be wiped by the later strip")
	}
}

// (c) becomes-1/1 with a -1/-1 counter is 0/0.
func TestProbe_SetOneOne_MinusCounter_IsZeroZero(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Serra Angel", 4, 4, "creature")
	RegisterSetPT(gs, bear, 1, 1, "permanent", "ProbeSet", "probe")
	bear.AddCounter("-1/-1", 1)
	gs.InvalidateCharacteristicsCache()
	c := GetEffectiveCharacteristics(gs, bear)
	if c.Power != 0 || c.Toughness != 0 {
		t.Errorf("(c) base 1/1 + -1/-1 counter should be 0/0, got %d/%d", c.Power, c.Toughness)
	}
}

// (c) becomes-1/1 with a +1/+1 counter is 2/2.
func TestProbe_SetOneOne_PlusCounter_IsTwoTwo(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Serra Angel", 4, 4, "creature")
	RegisterSetPT(gs, bear, 1, 1, "permanent", "ProbeSet", "probe")
	bear.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	c := GetEffectiveCharacteristics(gs, bear)
	if c.Power != 2 || c.Toughness != 2 {
		t.Errorf("(c) base 1/1 + +1/+1 counter should be 2/2, got %d/%d", c.Power, c.Toughness)
	}
}

// (d) strip preserves counters (they live on the Permanent, not chars).
func TestProbe_StripPreservesCounters(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.AddCounter("+1/+1", 2)
	registerStripAbilities(gs, bear, "ProbeStrip")
	gs.InvalidateCharacteristicsCache()
	if bear.Counters["+1/+1"] != 2 {
		t.Errorf("(d) strip must not remove counters; got %d", bear.Counters["+1/+1"])
	}
	c := GetEffectiveCharacteristics(gs, bear)
	// 2/2 base + 2 counters = 4/4 (abilities stripped, P/T untouched by strip).
	if c.Power != 4 || c.Toughness != 4 {
		t.Errorf("counters still apply after strip; got %d/%d", c.Power, c.Toughness)
	}
}

// (a) additive type-change keeps the existing types (Curious Colossus
// "becomes a Coward IN ADDITION to its other types").
func TestProbe_AdditiveTypeChangeKeepsExisting(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	RegisterAddTypes(gs, bear, []string{"coward"}, "permanent", "ProbeAdd", "probe")
	gs.InvalidateCharacteristicsCache()
	c := GetEffectiveCharacteristics(gs, bear)
	hasCreature := charsHaveType(c.Types, "creature")
	hasCoward := charsHaveType(c.Types, "coward") || charsHaveType(c.Subtypes, "coward")
	if !hasCreature {
		t.Errorf("(a) additive type-change must KEEP existing creature type")
	}
	if !hasCoward {
		t.Errorf("(a) additive type-change must add coward; types=%v subtypes=%v", c.Types, c.Subtypes)
	}
}

// (e) Darksteel-Mutation shape: one effect strips all OTHER abilities and
// grants indestructible. Correct registration order (grant after strip,
// same source) keeps indestructible. Plus base 0/1.
func TestProbe_DarksteelMutationShape(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.GrantedAbilities = []string{"flying"}
	// One conceptual effect: strip (registered first), then grant
	// indestructible (registered after), then base 0/1.
	registerStripAbilities(gs, bear, "DarksteelStrip")
	RegisterGrantKeyword(gs, bear, "indestructible", "permanent", "DarksteelGrant", "probe")
	RegisterSetPT(gs, bear, 0, 1, "permanent", "DarksteelPT", "probe")
	gs.InvalidateCharacteristicsCache()
	c := GetEffectiveCharacteristics(gs, bear)
	if c.Power != 0 || c.Toughness != 1 {
		t.Errorf("(e) Darksteel base P/T 0/1; got %d/%d", c.Power, c.Toughness)
	}
	if gs.HasKeywordOf(bear, "flying") {
		t.Errorf("(e) original flying must be stripped")
	}
	if !gs.HasKeywordOf(bear, "indestructible") {
		t.Errorf("(e) granted indestructible must survive the same-effect strip")
	}
}
