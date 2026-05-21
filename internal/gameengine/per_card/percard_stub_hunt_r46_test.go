package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R46 stub-hunt batch — tests for the nine ports listed in
// docs/stub-hunt-percard-r46.md. Each test exercises the new behavior
// in isolation so a regression nukes one card, not the batch.
//
// Ports under test:
//   - Eruth, Tormented Prophet           replacement: would_draw → exile 2
//   - Kudo, King Among Bears             layer 7b base 2/2 + layer 4 add bear
//   - Clara Oswald                       Doctor ETB-trigger doubler
//   - Katara, the Fearless               Ally ETB-trigger doubler
//   - The Twelfth Doctor                 +1/+1 counter on spell-copy trigger
//   - Toph, the First Metalbender        end_step earthbend on a land
//   - Aloy, Savior of Meridian           promotion: ApplyDiscover wired
//   - Ivy, Gleeful Spellthief            promotion: push copy retargeting Ivy
//   - Bello, Bard of the Brambles        layered statics + combat draw

// ---------------------------------------------------------------------------
// Eruth, Tormented Prophet
// ---------------------------------------------------------------------------

func TestEruth_WouldDrawExilesTopTwoAndCancels(t *testing.T) {
	gs := newGame(t, 2)
	eruth := stampCreaturePT(addPerm(gs, 0, "Eruth, Tormented Prophet", "creature", "legendary"), 2, 3)
	addLibrary(gs, 0, "Top1", "Top2", "Top3")

	eruthRegisterDrawReplacement(gs, eruth)

	_, cancelled := gameengine.FireDrawEvent(gs, 0, eruth)
	if !cancelled {
		t.Fatalf("expected draw to be cancelled by Eruth replacement")
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected library reduced by 2 to size 1, got %d", len(gs.Seats[0].Library))
	}
	if len(gs.Seats[0].Exile) != 2 {
		t.Errorf("expected 2 cards in exile, got %d", len(gs.Seats[0].Exile))
	}
	// Both exiled cards should have a ZoneCastGrant registered.
	grants := 0
	for _, c := range gs.Seats[0].Exile {
		if g := gameengine.GetZoneCastGrant(gs, c); g != nil && g.SourceName == "Eruth, Tormented Prophet" {
			grants++
		}
	}
	if grants != 2 {
		t.Errorf("expected 2 Eruth-sourced zone cast grants, got %d", grants)
	}
}

func TestEruth_ReplacementUnregistersOnLTB(t *testing.T) {
	gs := newGame(t, 2)
	eruth := stampCreaturePT(addPerm(gs, 0, "Eruth, Tormented Prophet", "creature", "legendary"), 2, 3)
	eruthRegisterDrawReplacement(gs, eruth)

	pre := len(gs.Replacements)
	if pre == 0 {
		t.Fatalf("expected replacement registered, got 0")
	}
	gs.UnregisterReplacementsForPermanent(eruth)
	if got := len(gs.Replacements); got != pre-1 {
		t.Errorf("expected replacement to unregister; before=%d after=%d", pre, got)
	}
}

// ---------------------------------------------------------------------------
// Kudo, King Among Bears
// ---------------------------------------------------------------------------

func TestKudo_OtherCreaturesGetBase2_2AndBear(t *testing.T) {
	gs := newGame(t, 2)
	kudo := stampCreaturePT(addPerm(gs, 0, "Kudo, King Among Bears", "creature", "legendary"), 3, 3)
	// Add a vanilla 5/5 — should become base 2/2 with bear subtype.
	other := stampCreaturePT(addPerm(gs, 0, "Giant", "creature"), 5, 5)

	kudoKingAmongBearsRegister(gs, kudo)
	gs.InvalidateCharacteristicsCache()

	chars := gameengine.GetEffectiveCharacteristics(gs,other)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("expected other creature 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	hasBear := false
	for _, s := range chars.Subtypes {
		if s == "bear" {
			hasBear = true
		}
	}
	if !hasBear {
		t.Errorf("expected bear subtype on other, got Subtypes=%v", chars.Subtypes)
	}

	// Kudo herself should NOT be affected ("other" predicate).
	selfChars := gameengine.GetEffectiveCharacteristics(gs,kudo)
	if selfChars.Power == 2 && selfChars.Toughness == 2 {
		t.Errorf("Kudo herself should not be set to 2/2 (printed %d/%d)", kudo.Card.BasePower, kudo.Card.BaseToughness)
	}
}

// ---------------------------------------------------------------------------
// Clara Oswald — Doctor ETB-trigger doubler
// ---------------------------------------------------------------------------

func TestClara_DoublesDoctorETBTrigger(t *testing.T) {
	gs := newGame(t, 2)
	clara := stampCreaturePT(addPerm(gs, 0, "Clara Oswald", "creature", "legendary"), 1, 2)
	doctor := stampCreaturePT(addPerm(gs, 0, "Twelfth Doctor", "creature", "legendary", "doctor"), 3, 3)

	claraOswaldRegisterDoctorDoubler(gs, clara)

	count, cancelled := gameengine.FireETBTriggerEvent(gs, doctor)
	if cancelled {
		t.Fatalf("Clara should not cancel ETB triggers")
	}
	if count != 2 {
		t.Errorf("expected Doctor ETB trigger doubled to 2, got %d", count)
	}
}

func TestClara_DoesNotDoubleNonDoctor(t *testing.T) {
	gs := newGame(t, 2)
	clara := stampCreaturePT(addPerm(gs, 0, "Clara Oswald", "creature", "legendary"), 1, 2)
	wizard := stampCreaturePT(addPerm(gs, 0, "Goblin Mage", "creature", "wizard"), 2, 2)

	claraOswaldRegisterDoctorDoubler(gs, clara)

	count, _ := gameengine.FireETBTriggerEvent(gs, wizard)
	if count != 1 {
		t.Errorf("expected non-Doctor ETB trigger unchanged (1), got %d", count)
	}
}

func TestClara_IgnoresOpponentDoctor(t *testing.T) {
	gs := newGame(t, 2)
	clara := stampCreaturePT(addPerm(gs, 0, "Clara Oswald", "creature", "legendary"), 1, 2)
	oppDoctor := stampCreaturePT(addPerm(gs, 1, "The Doctor's Companion", "creature", "doctor"), 2, 2)

	claraOswaldRegisterDoctorDoubler(gs, clara)

	count, _ := gameengine.FireETBTriggerEvent(gs, oppDoctor)
	if count != 1 {
		t.Errorf("Clara should not double opponent's Doctor; got count=%d", count)
	}
}

// ---------------------------------------------------------------------------
// Katara, the Fearless — Ally ETB-trigger doubler
// ---------------------------------------------------------------------------

func TestKatara_DoublesAllyETBTrigger(t *testing.T) {
	gs := newGame(t, 2)
	katara := stampCreaturePT(addPerm(gs, 0, "Katara, the Fearless", "creature", "legendary"), 2, 2)
	ally := stampCreaturePT(addPerm(gs, 0, "Highland Berserker", "creature", "ally"), 2, 2)

	kataraTheFearlessRegisterAllyDoubler(gs, katara)

	count, _ := gameengine.FireETBTriggerEvent(gs, ally)
	if count != 2 {
		t.Errorf("expected Ally ETB trigger doubled to 2, got %d", count)
	}
}

func TestKatara_DoesNotDoubleNonAlly(t *testing.T) {
	gs := newGame(t, 2)
	katara := stampCreaturePT(addPerm(gs, 0, "Katara, the Fearless", "creature", "legendary"), 2, 2)
	beast := stampCreaturePT(addPerm(gs, 0, "Grizzly Bears", "creature", "bear"), 2, 2)

	kataraTheFearlessRegisterAllyDoubler(gs, katara)

	count, _ := gameengine.FireETBTriggerEvent(gs, beast)
	if count != 1 {
		t.Errorf("non-Ally should not double; got %d", count)
	}
}

// ---------------------------------------------------------------------------
// The Twelfth Doctor — +1/+1 counter on spell-copy trigger
// ---------------------------------------------------------------------------

func TestTwelfthDoctor_AddsCounterOnSpellCopy(t *testing.T) {
	gs := newGame(t, 2)
	doc := stampCreaturePT(addPerm(gs, 0, "The Twelfth Doctor", "creature", "legendary", "doctor"), 3, 4)

	gameengine.FireCardTrigger(gs, "spell_copied", map[string]interface{}{
		"caster_seat": 0,
	})

	if got := doc.Counters["+1/+1"]; got != 1 {
		t.Errorf("expected +1/+1 counter, got %d (counters=%v)", got, doc.Counters)
	}
}

func TestTwelfthDoctor_OpponentCopyDoesNotCount(t *testing.T) {
	gs := newGame(t, 2)
	doc := stampCreaturePT(addPerm(gs, 0, "The Twelfth Doctor", "creature", "legendary", "doctor"), 3, 4)

	gameengine.FireCardTrigger(gs, "spell_copied", map[string]interface{}{
		"caster_seat": 1,
	})

	if got := doc.Counters["+1/+1"]; got != 0 {
		t.Errorf("opponent copy should not add counter; got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Toph, the First Metalbender — earthbend 2 at end step
// ---------------------------------------------------------------------------

func TestToph1stMB_EndStepStampsLand(t *testing.T) {
	gs := newGame(t, 2)
	toph := stampCreaturePT(addPerm(gs, 0, "Toph, the First Metalbender", "creature", "legendary"), 3, 3)
	land := addPerm(gs, 0, "Forest", "land", "basic")
	gs.Active = 0

	tophFirstMetalbenderEndStep(gs, toph, map[string]interface{}{"active_seat": 0})

	if land.Flags["earthbent"] != 1 {
		t.Errorf("expected land earthbent flag, got %d", land.Flags["earthbent"])
	}
	// R54: haste now comes from a Layer 6 continuous effect, not the
	// transient temp_haste perm flag. Read effective characteristics.
	if !gs.HasKeywordOf(land, "haste") {
		t.Errorf("expected layer-6 haste keyword grant, keywords=%v",
			gameengine.GetEffectiveCharacteristics(gs, land).Keywords)
	}
	if got := land.Counters["+1/+1"]; got != 2 {
		t.Errorf("expected 2 +1/+1 counters on land, got %d", got)
	}
	if hasEvent(gs, "earthbend") == 0 {
		t.Errorf("expected earthbend event in log")
	}
}

func TestToph1stMB_DoesNotFireOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	toph := stampCreaturePT(addPerm(gs, 0, "Toph, the First Metalbender", "creature", "legendary"), 3, 3)
	land := addPerm(gs, 0, "Forest", "land", "basic")

	tophFirstMetalbenderEndStep(gs, toph, map[string]interface{}{"active_seat": 1})

	if land.Flags["earthbent"] == 1 {
		t.Errorf("should not earthbend on opponent's end step")
	}
}

// ---------------------------------------------------------------------------
// Aloy, Savior of Meridian — discover wire-up
// ---------------------------------------------------------------------------

func TestAloy_PromotedHandlerCallsApplyDiscover(t *testing.T) {
	gs := newGame(t, 2)
	aloy := stampCreaturePT(addPerm(gs, 0, "Aloy, Savior of Meridian", "creature", "legendary", "artifact"), 3, 3)
	// Attacker has to be an artifact creature controlled by Aloy's seat.
	atk := stampCreaturePT(addPerm(gs, 0, "Bronze Sable", "creature", "artifact"), 4, 4)
	atk.Flags = map[string]int{"attacking": 1}
	// Seed library so ApplyDiscover has something to exile.
	addLibrary(gs, 0, "Tin Land", "Steel Cog", "Bronze Spear")
	for _, c := range gs.Seats[0].Library {
		c.Types = []string{"artifact"}
		c.CMC = 2
		c.BasePower = 1
		c.BaseToughness = 1
	}

	preLib := len(gs.Seats[0].Library)
	aloyAttacks(gs, aloy, map[string]interface{}{"attacker_perm": atk})

	// ApplyDiscover should have peeled at least one card off the top.
	if len(gs.Seats[0].Library) >= preLib {
		t.Errorf("ApplyDiscover should have removed cards from library; preLib=%d post=%d", preLib, len(gs.Seats[0].Library))
	}
	if hasEvent(gs, "per_card_handler") == 0 {
		t.Errorf("expected aloy per_card_handler emit")
	}
}

// ---------------------------------------------------------------------------
// Ivy, Gleeful Spellthief — push spell copy retargeting Ivy
// ---------------------------------------------------------------------------

func TestIvy_PushesCopyRetargetingIvy(t *testing.T) {
	gs := newGame(t, 2)
	ivy := stampCreaturePT(addPerm(gs, 0, "Ivy, Gleeful Spellthief", "creature", "legendary"), 2, 2)
	otherCreature := stampCreaturePT(addPerm(gs, 0, "Buddy", "creature"), 3, 3)
	// Originating spell on the stack: pretend a Lightning Bolt targeting Buddy
	spellCard := &gameengine.Card{Name: "Lightning Bolt", Owner: 1, Types: []string{"instant"}}
	origin := &gameengine.StackItem{
		Controller: 1,
		Card:       spellCard,
		Kind:       "spell",
		Targets:    []gameengine.Target{{Kind: gameengine.TargetKindPermanent, Permanent: otherCreature}},
	}
	gameengine.PushStackItem(gs, origin)
	preStack := len(gs.Stack)

	ivySpellCast(gs, ivy, map[string]interface{}{
		"caster_seat": 1,
		"card":        spellCard,
		"target_perm": otherCreature,
	})

	if len(gs.Stack) != preStack+1 {
		t.Fatalf("expected one new stack item; pre=%d post=%d", preStack, len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if !top.IsCopy {
		t.Errorf("new top stack item should be a copy")
	}
	if top.Controller != 0 {
		t.Errorf("copy controller should be Ivy's controller (0), got %d", top.Controller)
	}
	if len(top.Targets) != 1 || top.Targets[0].Permanent != ivy {
		t.Errorf("copy should retarget Ivy; targets=%+v", top.Targets)
	}
}

func TestIvy_DoesNotFireOnNonCreatureTarget(t *testing.T) {
	gs := newGame(t, 2)
	ivy := stampCreaturePT(addPerm(gs, 0, "Ivy, Gleeful Spellthief", "creature", "legendary"), 2, 2)
	land := addPerm(gs, 0, "Forest", "land")
	preStack := len(gs.Stack)

	ivySpellCast(gs, ivy, map[string]interface{}{
		"caster_seat": 1,
		"target_perm": land,
	})

	if len(gs.Stack) != preStack {
		t.Errorf("non-creature target should not push a copy; stack delta=%d", len(gs.Stack)-preStack)
	}
}

func TestIvy_DoesNotFireOnIvyHerself(t *testing.T) {
	gs := newGame(t, 2)
	ivy := stampCreaturePT(addPerm(gs, 0, "Ivy, Gleeful Spellthief", "creature", "legendary"), 2, 2)
	preStack := len(gs.Stack)

	ivySpellCast(gs, ivy, map[string]interface{}{
		"caster_seat": 1,
		"target_perm": ivy,
	})

	if len(gs.Stack) != preStack {
		t.Errorf("self-target should not fire; stack delta=%d", len(gs.Stack)-preStack)
	}
}

// ---------------------------------------------------------------------------
// Bello, Bard of the Brambles — layered statics + combat draw
// ---------------------------------------------------------------------------

func TestBello_4PlusArtifactBecomes4_4ElementalDuringOwnerTurn(t *testing.T) {
	gs := newGame(t, 2)
	bello := stampCreaturePT(addPerm(gs, 0, "Bello, Bard of the Brambles", "creature", "legendary"), 3, 3)
	art := addPerm(gs, 0, "Crystal Ball", "artifact")
	art.Card.CMC = 4
	gs.Active = 0

	belloRegister(gs, bello)
	gs.InvalidateCharacteristicsCache()

	chars := gameengine.GetEffectiveCharacteristics(gs,art)
	hasCreature := false
	for _, t := range chars.Types {
		if t == "creature" {
			hasCreature = true
		}
	}
	if !hasCreature {
		t.Errorf("4+ MV artifact should be creature; types=%v", chars.Types)
	}
	hasElem := false
	for _, s := range chars.Subtypes {
		if s == "elemental" {
			hasElem = true
		}
	}
	if !hasElem {
		t.Errorf("expected elemental subtype; got %v", chars.Subtypes)
	}
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Errorf("expected 4/4 base; got %d/%d", chars.Power, chars.Toughness)
	}
	hasInd, hasHaste := false, false
	for _, kw := range chars.Keywords {
		if kw == "indestructible" {
			hasInd = true
		}
		if kw == "haste" {
			hasHaste = true
		}
	}
	if !hasInd || !hasHaste {
		t.Errorf("expected indestructible + haste; got %v", chars.Keywords)
	}
}

func TestBello_DoesNotApplyOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	bello := stampCreaturePT(addPerm(gs, 0, "Bello, Bard of the Brambles", "creature", "legendary"), 3, 3)
	art := addPerm(gs, 0, "Crystal Ball", "artifact")
	art.Card.CMC = 4
	gs.Active = 1 // opponent's turn

	belloRegister(gs, bello)
	gs.InvalidateCharacteristicsCache()

	chars := gameengine.GetEffectiveCharacteristics(gs, art)
	if containsType(chars.Types, "creature") {
		t.Errorf("artifact should NOT be a creature on opponent's turn; types=%v", chars.Types)
	}
}

func TestBello_SubMV4ArtifactNotAffected(t *testing.T) {
	gs := newGame(t, 2)
	bello := stampCreaturePT(addPerm(gs, 0, "Bello, Bard of the Brambles", "creature", "legendary"), 3, 3)
	art := addPerm(gs, 0, "Wayfarer's Bauble", "artifact")
	art.Card.CMC = 1
	gs.Active = 0

	belloRegister(gs, bello)
	gs.InvalidateCharacteristicsCache()

	chars := gameengine.GetEffectiveCharacteristics(gs, art)
	if containsType(chars.Types, "creature") {
		t.Errorf("MV<4 artifact should not become creature; types=%v", chars.Types)
	}
}

func TestBello_EquipmentExcluded(t *testing.T) {
	gs := newGame(t, 2)
	bello := stampCreaturePT(addPerm(gs, 0, "Bello, Bard of the Brambles", "creature", "legendary"), 3, 3)
	eq := addPerm(gs, 0, "Argentum Armor", "artifact", "equipment")
	eq.Card.CMC = 6
	gs.Active = 0

	belloRegister(gs, bello)
	gs.InvalidateCharacteristicsCache()

	chars := gameengine.GetEffectiveCharacteristics(gs, eq)
	if containsType(chars.Types, "creature") {
		t.Errorf("Equipment artifact should be excluded; types=%v", chars.Types)
	}
}

func containsType(ts []string, want string) bool {
	for _, t := range ts {
		if t == want {
			return true
		}
	}
	return false
}
