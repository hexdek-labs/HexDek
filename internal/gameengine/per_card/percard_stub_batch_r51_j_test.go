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
	// Post-Phase-G the copy MUST be a distinct *Card (via MintSpellCopy)
	// so §707.10 cease on resolve retires the copy's CP-provenance ID,
	// not the source's OG ID. Aliasing the source pointer is the leak
	// shape that drove the 34-hit Lash Out fabrication in Loki seed-42
	// game 2762.
	if top.Card == castCard {
		t.Errorf("expected copy to be a DISTINCT *Card pointer (Phase G fix); got source alias")
	}
	if top.Card == nil {
		t.Fatalf("MintSpellCopy returned nil; spell-copy chokepoint regression")
	}
	if top.Card.Name != castCard.Name {
		t.Errorf("expected copy to preserve Name %q; got %q", castCard.Name, top.Card.Name)
	}
	if !top.Card.IsCopy {
		t.Errorf("expected copy.IsCopy=true (MintSpellCopy contract)")
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
// 3. Arguel's Blood Fast — wire low-life transform
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
