package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R51 stub-batch J ports — 4 cheap (CMC 1-2) ports to real handlers.
// Initial scope was 10 but the candidate pool was thinned by prior
// batches A-G covering most low-CMC commanders; the four below are
// the tractable ports remaining.

// ---------------------------------------------------------------------------
// 1. Akiri, Line-Slinger — promote artifact-buff to real layer-7c CE
// ---------------------------------------------------------------------------

func TestAkiri_R51_LayerCEAppliesPowerBuffPerArtifact(t *testing.T) {
	gs := newGame(t, 2)
	akiri := stampCreaturePT(addPerm(gs, 0, "Akiri, Line-Slinger", "creature"), 2, 3)
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Mana Crypt", "artifact")
	addPerm(gs, 0, "Lotus Petal", "artifact")

	akiriLineSlingerRegister(gs, akiri)

	chars := gameengine.GetEffectiveCharacteristics(gs, akiri)
	// Base 2 power + 3 artifacts → 5 effective power.
	if chars.Power != 5 {
		t.Fatalf("expected Akiri power 5 (base 2 + 3 artifacts), got %d", chars.Power)
	}
}

func TestAkiri_R51_NoArtifactsNoBuff(t *testing.T) {
	gs := newGame(t, 2)
	akiri := stampCreaturePT(addPerm(gs, 0, "Akiri, Line-Slinger", "creature"), 2, 3)
	akiriLineSlingerRegister(gs, akiri)
	chars := gameengine.GetEffectiveCharacteristics(gs, akiri)
	if chars.Power != 2 {
		t.Errorf("expected Akiri base power 2 with no artifacts, got %d", chars.Power)
	}
}

// ---------------------------------------------------------------------------
// 2. Aziza, Mage Tower Captain — spell-copy when 3 untapped friendlies tap
// ---------------------------------------------------------------------------

func TestAziza_R51_CopiesSpellOnInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	aziza := stampCreaturePT(addPerm(gs, 0, "Aziza, Mage Tower Captain", "creature"), 3, 3)
	// Three untapped friendly creatures (cost-payable).
	c1 := stampCreaturePT(addPerm(gs, 0, "Soldier 1", "creature"), 1, 1)
	c2 := stampCreaturePT(addPerm(gs, 0, "Soldier 2", "creature"), 1, 1)
	c3 := stampCreaturePT(addPerm(gs, 0, "Soldier 3", "creature"), 1, 1)

	castCard := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	origItem := &gameengine.StackItem{Controller: 0, Card: castCard, CostMeta: map[string]interface{}{}}
	gs.Stack = append(gs.Stack, origItem)

	preStack := len(gs.Stack)
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        castCard,
		"spell_name":  "Lightning Bolt",
	})

	if len(gs.Stack) != preStack+1 {
		t.Fatalf("expected one copy pushed, stack went %d → %d", preStack, len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if !top.IsCopy {
		t.Errorf("expected copy StackItem to have IsCopy=true")
	}
	if top.Card != castCard {
		t.Errorf("expected copy to reference the same Card pointer")
	}
	// All three cost-creatures should be tapped.
	if !c1.Tapped || !c2.Tapped || !c3.Tapped {
		t.Errorf("expected all 3 cost creatures tapped; got %v %v %v",
			c1.Tapped, c2.Tapped, c3.Tapped)
	}
}

func TestAziza_R51_NoCopyWithoutThreeUntapped(t *testing.T) {
	gs := newGame(t, 2)
	aziza := stampCreaturePT(addPerm(gs, 0, "Aziza, Mage Tower Captain", "creature"), 3, 3)
	// Only two untapped friendlies — cannot pay cost.
	addPerm(gs, 0, "Lonely Soldier", "creature")
	addPerm(gs, 0, "Lone Wolf", "creature")

	castCard := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"}}
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Controller: 0, Card: castCard, CostMeta: map[string]interface{}{}})

	preStack := len(gs.Stack)
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 0,
		"card":        castCard,
		"spell_name":  "Counterspell",
	})

	if len(gs.Stack) != preStack {
		t.Errorf("expected no copy pushed with only 2 untapped friendlies, stack %d → %d",
			preStack, len(gs.Stack))
	}
}

func TestAziza_R51_DoesNotFireForOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	aziza := stampCreaturePT(addPerm(gs, 0, "Aziza, Mage Tower Captain", "creature"), 3, 3)
	for i := 0; i < 3; i++ {
		addPerm(gs, 0, "Buddy", "creature")
	}
	castCard := &gameengine.Card{Name: "Counterspell", Owner: 1, Types: []string{"instant"}}
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Controller: 1, Card: castCard, CostMeta: map[string]interface{}{}})

	preStack := len(gs.Stack)
	azizaSpellCopy(gs, aziza, map[string]interface{}{
		"caster_seat": 1,
		"card":        castCard,
		"spell_name":  "Counterspell",
	})

	if len(gs.Stack) != preStack {
		t.Errorf("expected no copy on opponent's cast")
	}
}

// ---------------------------------------------------------------------------
// 3. Kolodin, Triumph Caster — promote haste static to real layer-6 CE
// ---------------------------------------------------------------------------

func TestKolodin_R51_GrantsHasteToMountsAndVehicles(t *testing.T) {
	gs := newGame(t, 2)
	kolodin := stampCreaturePT(addPerm(gs, 0, "Kolodin, Triumph Caster", "creature"), 2, 4)
	mount := stampCreaturePT(addPerm(gs, 0, "Smiling Spelltender", "creature", "mount"), 2, 3)
	vehicle := stampCreaturePT(addPerm(gs, 0, "Skysovereign", "artifact", "vehicle"), 6, 5)
	other := stampCreaturePT(addPerm(gs, 0, "Plain Soldier", "creature"), 2, 2)

	kolodinTriumphCasterETB(gs, kolodin)

	chars := gameengine.GetEffectiveCharacteristics(gs, mount)
	if !containsKeyword(chars.Keywords, "haste") {
		t.Errorf("expected mount to gain haste; keywords=%+v", chars.Keywords)
	}
	chars = gameengine.GetEffectiveCharacteristics(gs, vehicle)
	if !containsKeyword(chars.Keywords, "haste") {
		t.Errorf("expected vehicle to gain haste; keywords=%+v", chars.Keywords)
	}
	chars = gameengine.GetEffectiveCharacteristics(gs, other)
	if containsKeyword(chars.Keywords, "haste") {
		t.Errorf("non-mount/vehicle should NOT gain haste from Kolodin")
	}
}

func TestKolodin_R51_DoesNotGrantHasteToOpponentMounts(t *testing.T) {
	gs := newGame(t, 2)
	kolodin := stampCreaturePT(addPerm(gs, 0, "Kolodin, Triumph Caster", "creature"), 2, 4)
	oppMount := stampCreaturePT(addPerm(gs, 1, "Wolly Mammoth", "creature", "mount"), 3, 3)
	kolodinTriumphCasterETB(gs, kolodin)
	chars := gameengine.GetEffectiveCharacteristics(gs, oppMount)
	if containsKeyword(chars.Keywords, "haste") {
		t.Errorf("opponent's mount should not gain haste from Kolodin")
	}
}

func containsKeyword(kws []string, want string) bool {
	for _, k := range kws {
		if k == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 4. Arguel's Blood Fast — wire low-life transform
// ---------------------------------------------------------------------------

func TestArguelsBloodFast_R51_TransformsAtFiveOrLess(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	arguel := addPerm(gs, 0, "Arguel's Blood Fast", "enchantment")
	// Set up the DFC face data so TransformPermanent flips faces rather
	// than no-op'ing on missing AST.
	arguel.FrontFaceAST = &gameast.CardAST{}
	arguel.BackFaceAST = &gameast.CardAST{}
	arguel.FrontFaceName = "Arguel's Blood Fast"
	arguel.BackFaceName = "Temple of Aclazotz"
	gs.Seats[0].Life = 5

	arguelsBloodFastUpkeep(gs, arguel, map[string]interface{}{"active_seat": 0})

	if !arguel.Transformed {
		t.Errorf("expected Arguel's Blood Fast transformed at 5 life; transformed=%v", arguel.Transformed)
	}
}

func TestArguelsBloodFast_R51_DoesNotTransformAboveThreshold(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	arguel := addPerm(gs, 0, "Arguel's Blood Fast", "enchantment")
	gs.Seats[0].Life = 10

	arguelsBloodFastUpkeep(gs, arguel, map[string]interface{}{"active_seat": 0})

	if arguel.Transformed {
		t.Errorf("expected NO transform at 10 life; transformed=%v", arguel.Transformed)
	}
}

func TestArguelsBloodFast_R51_DoesNotTransformOnOpponentUpkeep(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1
	arguel := addPerm(gs, 0, "Arguel's Blood Fast", "enchantment")
	gs.Seats[0].Life = 5

	arguelsBloodFastUpkeep(gs, arguel, map[string]interface{}{"active_seat": 1})

	if arguel.Transformed {
		t.Errorf("expected NO transform on opponent's upkeep; transformed=%v", arguel.Transformed)
	}
}
