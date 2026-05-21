package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R49 stub-batch-E ports — 10 defensive utility cards (protection,
// removal, counter denial). Each test pins the primary defensive
// behavior introduced by the port and at least one negative-case
// guard against regressions.

// ---------------------------------------------------------------------------
// 1. Maha, Its Feathers Night — opponent base toughness 1 (real CE)
// ---------------------------------------------------------------------------

func TestMaha_R49_RegistersRealLayer7bContinuousEffect(t *testing.T) {
	gs := newGame(t, 2)
	maha := stampCreaturePT(addPerm(gs, 0, "Maha, Its Feathers Night", "creature"), 6, 6)
	oppCreature := stampCreaturePT(addPerm(gs, 1, "Big Threat", "creature"), 5, 5)

	mahaETBRegisterBaseTough(gs, maha)

	// Force a layer recompute and read effective characteristics.
	chars := gameengine.GetEffectiveCharacteristics(gs, oppCreature)
	if chars.BaseToughness != 1 {
		t.Fatalf("expected opponent base toughness 1 post-Maha, got %d (full chars: %+v)",
			chars.BaseToughness, chars)
	}
	if chars.Toughness != 1 {
		t.Errorf("expected effective toughness 1, got %d", chars.Toughness)
	}
}

func TestMaha_R49_DoesNotAffectOwnCreatures(t *testing.T) {
	gs := newGame(t, 2)
	maha := stampCreaturePT(addPerm(gs, 0, "Maha, Its Feathers Night", "creature"), 6, 6)
	myCreature := stampCreaturePT(addPerm(gs, 0, "My Buddy", "creature"), 4, 4)
	mahaETBRegisterBaseTough(gs, maha)

	chars := gameengine.GetEffectiveCharacteristics(gs, myCreature)
	if chars.BaseToughness == 1 {
		t.Errorf("own creature should NOT be set to base toughness 1, got %d", chars.BaseToughness)
	}
}

// ---------------------------------------------------------------------------
// 2. Sokrates, Athenian Teacher — hexproof while untapped (sync triggers)
// ---------------------------------------------------------------------------

func TestSokrates_R49_StampsHexproofOnETBWhenUntapped(t *testing.T) {
	gs := newGame(t, 2)
	sokrates := stampCreaturePT(addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature"), 1, 4)
	sokrates.Tapped = false
	sokratesETBStampHexproof(gs, sokrates)
	if sokrates.Flags["kw:hexproof"] != 1 {
		t.Fatalf("expected hexproof on ETB when untapped, got %d", sokrates.Flags["kw:hexproof"])
	}
}

func TestSokrates_R49_TappedClearsHexproof(t *testing.T) {
	gs := newGame(t, 2)
	sokrates := stampCreaturePT(addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature"), 1, 4)
	sokratesETBStampHexproof(gs, sokrates)
	if sokrates.Flags["kw:hexproof"] != 1 {
		t.Fatalf("expected hexproof on ETB, got %d", sokrates.Flags["kw:hexproof"])
	}
	sokrates.Tapped = true
	sokratesTappedClearHexproof(gs, sokrates, map[string]interface{}{"target_perm": sokrates})
	if sokrates.Flags["kw:hexproof"] != 0 {
		t.Errorf("expected hexproof cleared when tapped, got %d", sokrates.Flags["kw:hexproof"])
	}
}

func TestSokrates_R49_UpkeepRestampsWhenUntapped(t *testing.T) {
	gs := newGame(t, 2)
	sokrates := stampCreaturePT(addPerm(gs, 0, "Sokrates, Athenian Teacher", "creature"), 1, 4)
	sokrates.Tapped = true
	sokratesETBStampHexproof(gs, sokrates) // does NOT stamp since tapped
	if sokrates.Flags["kw:hexproof"] == 1 {
		t.Fatalf("hexproof should NOT stamp when entering tapped")
	}
	sokrates.Tapped = false
	sokratesUpkeepRestampHexproof(gs, sokrates, nil)
	if sokrates.Flags["kw:hexproof"] != 1 {
		t.Errorf("upkeep should re-stamp hexproof on untapped Sokrates")
	}
}

// ---------------------------------------------------------------------------
// 3. Thrun, Breaker of Silence — active-turn indestructible
// ---------------------------------------------------------------------------

func TestThrun_R49_IndestructibleOnOwnTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	thrun := stampCreaturePT(addPerm(gs, 0, "Thrun, Breaker of Silence", "creature"), 5, 5)
	thrunETBStampIndestructible(gs, thrun)
	if thrun.Flags["kw:indestructible"] != 1 {
		t.Fatalf("expected indestructible on own turn, got %d", thrun.Flags["kw:indestructible"])
	}
}

func TestThrun_R49_NotIndestructibleOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1
	thrun := stampCreaturePT(addPerm(gs, 0, "Thrun, Breaker of Silence", "creature"), 5, 5)
	thrunETBStampIndestructible(gs, thrun)
	if thrun.Flags["kw:indestructible"] == 1 {
		t.Errorf("Thrun should NOT have indestructible on opponent's turn")
	}
}

func TestThrun_R49_UpkeepTogglesIndestructible(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	thrun := stampCreaturePT(addPerm(gs, 0, "Thrun, Breaker of Silence", "creature"), 5, 5)
	thrunETBStampIndestructible(gs, thrun)
	if thrun.Flags["kw:indestructible"] != 1 {
		t.Fatalf("indestructible should be set on own turn after ETB")
	}
	// Pass turn to opponent; upkeep tick should clear.
	gs.Active = 1
	thrunUpkeepRefreshIndestructible(gs, thrun, map[string]interface{}{"active_seat": 1})
	if thrun.Flags["kw:indestructible"] == 1 {
		t.Errorf("upkeep on opponent's turn should clear indestructible")
	}
}

// ---------------------------------------------------------------------------
// 4. Ozai, the Phoenix King — ≥6 mana flying+indestructible
// ---------------------------------------------------------------------------

func TestOzai_R49_GrantsFlyingAndIndestructibleAtSixMana(t *testing.T) {
	gs := newGame(t, 2)
	ozai := stampCreaturePT(addPerm(gs, 0, "Ozai, the Phoenix King", "creature"), 5, 5)
	gs.Seats[0].ManaPool = 7
	ozaiETBSetFlagsAndConditionalKW(gs, ozai)
	if ozai.Flags["kw:flying"] != 1 || ozai.Flags["kw:indestructible"] != 1 {
		t.Errorf("Ozai at 7 mana: expected flying+indestructible; got flying=%d indestructible=%d",
			ozai.Flags["kw:flying"], ozai.Flags["kw:indestructible"])
	}
}

func TestOzai_R49_ClearsGrantsBelowSixMana(t *testing.T) {
	gs := newGame(t, 2)
	ozai := stampCreaturePT(addPerm(gs, 0, "Ozai, the Phoenix King", "creature"), 5, 5)
	gs.Seats[0].ManaPool = 6
	ozaiETBSetFlagsAndConditionalKW(gs, ozai)
	if ozai.Flags["kw:flying"] != 1 {
		t.Fatalf("Ozai at exactly 6 mana should have flying, got %d", ozai.Flags["kw:flying"])
	}
	gs.Seats[0].ManaPool = 3
	ozaiPhaseRecheck(gs, ozai, nil)
	if ozai.Flags["kw:flying"] == 1 || ozai.Flags["kw:indestructible"] == 1 {
		t.Errorf("Ozai below 6 mana: grants should clear; got flying=%d indestructible=%d",
			ozai.Flags["kw:flying"], ozai.Flags["kw:indestructible"])
	}
}

// ---------------------------------------------------------------------------
// 5. Progenitus — graveyard-shuffle replacement
// ---------------------------------------------------------------------------

func TestProgenitus_R49_ReplacementRedirectsToLibrary(t *testing.T) {
	gs := newGame(t, 2)
	prog := stampCreaturePT(addPerm(gs, 0, "Progenitus", "creature"), 10, 10)
	progenitusETBSetProtectionAndRegisterReplacement(gs, prog)

	// Simulate the would_be_put_into_graveyard event.
	ev := gameengine.NewReplEvent("would_be_put_into_graveyard")
	ev.TargetSeat = 0
	ev.TargetPerm = prog
	ev.Source = prog
	ev.Payload["card_name"] = "Progenitus"
	ev.Payload["to_zone"] = "graveyard"
	gameengine.FireEvent(gs, ev)

	if !ev.Cancelled {
		t.Errorf("Progenitus replacement should cancel the event after manual library insert")
	}
	if len(gs.Seats[0].Library) != 1 || gs.Seats[0].Library[0] != prog.Card {
		t.Errorf("Progenitus Card should be in library; library=%+v", gs.Seats[0].Library)
	}
}

func TestProgenitus_R49_ProtectionFlagSet(t *testing.T) {
	gs := newGame(t, 2)
	prog := stampCreaturePT(addPerm(gs, 0, "Progenitus", "creature"), 10, 10)
	progenitusETBSetProtectionAndRegisterReplacement(gs, prog)
	if prog.Flags["prot:*"] != 1 {
		t.Errorf("expected Progenitus universal-protection sentinel set, got %d", prog.Flags["prot:*"])
	}
}

// ---------------------------------------------------------------------------
// 6. Wilson, Refined Grizzly — this spell can't be countered (OnCast hook)
// ---------------------------------------------------------------------------

func TestWilson_R49_CastStampsUncounterableMeta(t *testing.T) {
	gs := newGame(t, 2)
	wilsonCard := &gameengine.Card{Name: "Wilson, Refined Grizzly", Owner: 0, Types: []string{"creature"}}
	item := &gameengine.StackItem{Controller: 0, Card: wilsonCard, CostMeta: map[string]interface{}{}}
	wilsonRefinedGrizzlyCastUncounterable(gs, item)
	if v, ok := item.CostMeta["cannot_be_countered"]; !ok || v != true {
		t.Errorf("expected cannot_be_countered=true on Wilson cast, got %v", item.CostMeta)
	}
}

// ---------------------------------------------------------------------------
// 7. Erebos, God of the Dead — opponents can't gain life
// ---------------------------------------------------------------------------

func TestErebos_R49_OpponentLifegainPrevented(t *testing.T) {
	gs := newGame(t, 2)
	erebos := stampCreaturePT(addPerm(gs, 0, "Erebos, God of the Dead", "creature"), 5, 7)
	erebosETBRegisterLifegainDenial(gs, erebos)

	// Fire the would_gain_life replacement chain (the AST-resolution
	// pathway in resolve.go) — opponent's count should be zeroed.
	modified, cancelled := gameengine.FireGainLifeEvent(gs, 1, 5, nil)
	if cancelled {
		t.Errorf("event should not be cancelled outright; count should just go to 0")
	}
	if modified != 0 {
		t.Errorf("Erebos: opponent lifegain should be zeroed; got %d", modified)
	}
}

func TestErebos_R49_ControllerLifegainAllowed(t *testing.T) {
	gs := newGame(t, 2)
	erebos := stampCreaturePT(addPerm(gs, 0, "Erebos, God of the Dead", "creature"), 5, 7)
	erebosETBRegisterLifegainDenial(gs, erebos)

	modified, cancelled := gameengine.FireGainLifeEvent(gs, 0, 5, nil)
	if cancelled {
		t.Errorf("controller lifegain should not be cancelled")
	}
	if modified != 5 {
		t.Errorf("Erebos: own lifegain should be unaffected; got %d", modified)
	}
}

// ---------------------------------------------------------------------------
// 8. Lier, Disciple of the Drowned — spells can't be countered
// ---------------------------------------------------------------------------

func TestLier_R49_OwnSpellGetsUncounterableMeta(t *testing.T) {
	gs := newGame(t, 2)
	lier := stampCreaturePT(addPerm(gs, 0, "Lier, Disciple of the Drowned", "creature"), 3, 1)
	_ = lier

	// Simulate seat 0 casting a spell after Lier resolves: the engine
	// pushes the StackItem first, THEN fires the spell_cast trigger.
	mySpellCard := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"}}
	item := &gameengine.StackItem{Controller: 0, Card: mySpellCard, CostMeta: map[string]interface{}{}}
	gs.Stack = append(gs.Stack, item)

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 0,
		"card":        mySpellCard,
		"spell_name":  "Counterspell",
	})

	if v, ok := item.CostMeta["cannot_be_countered"]; !ok || v != true {
		t.Errorf("Lier: own spell should be flagged cannot_be_countered; got %+v", item.CostMeta)
	}
}

func TestLier_R49_OpponentSpellNotFlagged(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lier, Disciple of the Drowned", "creature")

	oppSpellCard := &gameengine.Card{Name: "Counterspell", Owner: 1, Types: []string{"instant"}}
	item := &gameengine.StackItem{Controller: 1, Card: oppSpellCard, CostMeta: map[string]interface{}{}}
	gs.Stack = append(gs.Stack, item)

	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": 1,
		"card":        oppSpellCard,
		"spell_name":  "Counterspell",
	})

	if _, ok := item.CostMeta["cannot_be_countered"]; ok {
		t.Errorf("Lier should NOT flag opponent's spell; got %+v", item.CostMeta)
	}
}

