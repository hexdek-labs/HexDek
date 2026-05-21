package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R52 stub-batch M ports — 10 enchantment-form / Saga back-face ports.
// Avoids A/B/C/D/E/F/G/H/I/J/K/L picks per the campaign ledger.
//
// Picks:
//   1. Joshua, Phoenix's Dominant   Saga back face chapters I/II/III
//   2. Elesh Norn // Etchings       Saga back face chapters I/II/III
//   3. Terra // Esper Terra         Saga back face chapters I-IV
//   4. Urabrask // The Great Work   Activated {R} sorcery transform
//   5. Sheoldred // True Scriptures Chapter III flip-front-up
//   6. Calix, Guided by Fate        Combat-damage nonlegendary copy
//   7. Necromancy                   LTB sacrifice of bonded creature
//   8. Heliod, Sun-Crowned          UEOT lifelink grant via {1}{W}
//   9. Gisa, Glorious Resurrector   Exile-instead + decayed reanimate
//  10. Archmage Ascension           6+ counter draw → tutor replacement

func bmStampDFC(p *gameengine.Permanent, frontName, backName string) {
	p.FrontFaceAST = &gameast.CardAST{}
	p.BackFaceAST = &gameast.CardAST{}
	p.FrontFaceName = frontName
	p.BackFaceName = backName
}

// ---------------------------------------------------------------------------
// 1. Joshua, Phoenix's Dominant — Saga chapters
// ---------------------------------------------------------------------------

func TestJoshua_SagaChapterIDealsDamage(t *testing.T) {
	gs := newGame(t, 2)
	josh := addPerm(gs, 0, "Joshua, Phoenix's Dominant // Phoenix, Warden of Fire", "creature", "legendary")
	josh.Transformed = true
	startLife := gs.Seats[1].Life

	joshuaPhoenixsSagaChapter(gs, josh, map[string]interface{}{"chapter": 1})

	if gs.Seats[1].Life != startLife-2 {
		t.Errorf("Chapter I should deal 2 damage; life %d → %d", startLife, gs.Seats[1].Life)
	}
}

func TestJoshua_SagaChapterIIIReanimatesAndFlips(t *testing.T) {
	gs := newGame(t, 2)
	josh := addPerm(gs, 0, "Joshua, Phoenix's Dominant // Phoenix, Warden of Fire", "creature", "legendary")
	josh.Transformed = true
	bmStampDFC(josh, "Joshua, Phoenix's Dominant", "Phoenix, Warden of Fire")

	c1 := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature", "cmc:2"}, BasePower: 2, BaseToughness: 2}
	c2 := &gameengine.Card{Name: "Wurm", Owner: 0, Types: []string{"creature", "cmc:5"}, BasePower: 5, BaseToughness: 5}
	bigBomb := &gameengine.Card{Name: "Titan", Owner: 0, Types: []string{"creature", "cmc:9"}, BasePower: 8, BaseToughness: 8}
	gs.Seats[0].Graveyard = []*gameengine.Card{c1, c2, bigBomb}
	bfBefore := len(gs.Seats[0].Battlefield)

	joshuaPhoenixsSagaChapter(gs, josh, map[string]interface{}{"chapter": 3})

	// Bigbomb cmc=9 skipped (>6). c2 cmc=5 fits → total=5. c1 cmc=2 → total=7 (skipped). So just c2.
	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Errorf("Chapter III: expected +1 perm (Wurm 5cmc); got delta=%d", len(gs.Seats[0].Battlefield)-bfBefore)
	}
	if josh.Transformed {
		t.Errorf("Chapter III should flip Joshua back to front face (Transformed=false)")
	}
}

// ---------------------------------------------------------------------------
// 2. Elesh Norn // The Argent Etchings — Saga chapter II buff
// ---------------------------------------------------------------------------

func TestEleshNornEtchings_ChapterIIBuffsCreatures(t *testing.T) {
	gs := newGame(t, 2)
	etchings := addPerm(gs, 0, "Elesh Norn // The Argent Etchings", "enchantment", "saga")
	etchings.Transformed = true
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	eleshNornEtchingsSagaChapter(gs, etchings, map[string]interface{}{"chapter": 2})

	if bear.Power() != 3 {
		t.Errorf("Chapter II should buff bear to 3/3; got %d", bear.Power())
	}
	if bear.Flags["kw:double_strike"] != 1 {
		t.Errorf("Chapter II should grant double strike; flags=%v", bear.Flags)
	}
}

func TestEleshNornEtchings_ChapterISpawnsFiveTokens(t *testing.T) {
	gs := newGame(t, 2)
	etchings := addPerm(gs, 0, "Elesh Norn // The Argent Etchings", "enchantment", "saga")
	etchings.Transformed = true
	bfBefore := len(gs.Seats[0].Battlefield)

	eleshNornEtchingsSagaChapter(gs, etchings, map[string]interface{}{"chapter": 1})

	if len(gs.Seats[0].Battlefield)-bfBefore != 5 {
		t.Errorf("Chapter I should spawn 5 tokens; got delta=%d", len(gs.Seats[0].Battlefield)-bfBefore)
	}
}

// ---------------------------------------------------------------------------
// 3. Terra // Esper Terra — Saga chapter IV mana add
// ---------------------------------------------------------------------------

func TestTerra_ChapterIVAddsMana(t *testing.T) {
	gs := newGame(t, 2)
	terra := addPerm(gs, 0, "Terra, Magical Adept // Esper Terra", "enchantment", "saga", "legendary")
	terra.Transformed = true
	bmStampDFC(terra, "Terra, Magical Adept", "Esper Terra")
	startMana := gs.Seats[0].ManaPool

	terraSagaChapter(gs, terra, map[string]interface{}{"chapter": 4})

	if gs.Seats[0].ManaPool-startMana != 10 {
		t.Errorf("Chapter IV should add 10 mana; got delta=%d", gs.Seats[0].ManaPool-startMana)
	}
	if terra.Transformed {
		t.Errorf("Chapter IV should flip Terra back to front face")
	}
}

func TestTerra_ChapterICopiesNonlegendaryEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	terra := addPerm(gs, 0, "Terra, Magical Adept // Esper Terra", "enchantment", "saga", "legendary")
	terra.Transformed = true
	ench := addPerm(gs, 0, "Sylvan Library", "enchantment")
	ench.Card.CMC = 3
	_ = ench
	bfBefore := len(gs.Seats[0].Battlefield)

	terraSagaChapter(gs, terra, map[string]interface{}{"chapter": 1})

	if len(gs.Seats[0].Battlefield)-bfBefore != 1 {
		t.Errorf("Chapter I should spawn a copy token; got delta=%d", len(gs.Seats[0].Battlefield)-bfBefore)
	}
}

// ---------------------------------------------------------------------------
// 4. Urabrask // The Great Work — activated transform
// ---------------------------------------------------------------------------

func TestUrabrask_ActivateTransformsAtThreeSpells(t *testing.T) {
	gs := newGame(t, 2)
	ura := addPerm(gs, 0, "Urabrask // The Great Work", "creature", "legendary")
	bmStampDFC(ura, "Urabrask", "The Great Work")
	gs.Seats[0].ManaPool = 1
	gs.Seats[0].Turn.SpellsCast = 3

	urabraskActivate(gs, ura, 0, nil)

	if !ura.Transformed {
		t.Errorf("Should have transformed Urabrask with 3 spells cast and {R} available")
	}
}

func TestUrabrask_ActivateFailsBelowThreshold(t *testing.T) {
	gs := newGame(t, 2)
	ura := addPerm(gs, 0, "Urabrask // The Great Work", "creature", "legendary")
	bmStampDFC(ura, "Urabrask", "The Great Work")
	gs.Seats[0].ManaPool = 1
	gs.Seats[0].Turn.SpellsCast = 2

	urabraskActivate(gs, ura, 0, nil)

	if ura.Transformed {
		t.Errorf("Should NOT transform with only 2 spells cast")
	}
}

// ---------------------------------------------------------------------------
// 5. Sheoldred — chapter III flip already covered by lore_counter_added.
//     Just verify the chapter III flip path doesn't break (smoke).
// ---------------------------------------------------------------------------

func TestSheoldred_ChapterIIIFlipsBackToFront(t *testing.T) {
	gs := newGame(t, 2)
	sheol := addPerm(gs, 0, "Sheoldred // The True Scriptures", "enchantment", "saga", "legendary")
	sheol.Transformed = true
	bmStampDFC(sheol, "Sheoldred", "The True Scriptures")

	sheoldredTSLore(gs, sheol, map[string]interface{}{"chapter": 3})

	if sheol.Transformed {
		t.Errorf("Chapter III should flip Sheoldred back to front face")
	}
}

// ---------------------------------------------------------------------------
// 6. Calix, Guided by Fate — combat copy
// ---------------------------------------------------------------------------

func TestCalix_CombatDamageCopiesNonlegendaryEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	calix := addPerm(gs, 0, "Calix, Guided by Fate", "creature", "legendary")
	ench := addPerm(gs, 0, "Sylvan Library", "enchantment")
	ench.Card.CMC = 3
	_ = ench
	bfBefore := len(gs.Seats[0].Battlefield)

	calixCombatCopy(gs, calix, map[string]interface{}{
		"source_perm": calix,
	})

	if len(gs.Seats[0].Battlefield)-bfBefore != 1 {
		t.Errorf("Calix combat copy should spawn a token; got delta=%d", len(gs.Seats[0].Battlefield)-bfBefore)
	}
}

func TestCalix_OncePerTurnGate(t *testing.T) {
	gs := newGame(t, 2)
	calix := addPerm(gs, 0, "Calix, Guided by Fate", "creature", "legendary")
	ench := addPerm(gs, 0, "Sylvan Library", "enchantment")
	ench.Card.CMC = 3
	_ = ench

	calixCombatCopy(gs, calix, map[string]interface{}{"source_perm": calix})
	bfAfterFirst := len(gs.Seats[0].Battlefield)
	calixCombatCopy(gs, calix, map[string]interface{}{"source_perm": calix})

	if len(gs.Seats[0].Battlefield) != bfAfterFirst {
		t.Errorf("Calix should be once-per-turn; second call added more perms")
	}
}

// ---------------------------------------------------------------------------
// 7. Necromancy — LTB sacrifices bonded creature
// ---------------------------------------------------------------------------

func TestNecromancy_LTBSacsBondedCreature(t *testing.T) {
	gs := newGame(t, 2)
	necro := addPerm(gs, 0, "Necromancy", "enchantment", "aura")
	bonded := addPerm(gs, 0, "Bear", "creature")
	necromancyTargets.Store(necro, bonded)

	necromancyLTBSacBondedCreature(gs, necro, map[string]interface{}{
		"perm": necro,
	})

	// Bonded should have been removed from battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == bonded {
			t.Errorf("bonded creature should have been sacrificed off battlefield")
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Heliod, Sun-Crowned — UEOT lifelink grant
// ---------------------------------------------------------------------------

func TestHeliod_ActivateGrantsLifelink(t *testing.T) {
	gs := newGame(t, 2)
	heliod := addPerm(gs, 0, "Heliod, Sun-Crowned", "creature", "legendary", "enchantment")
	heliod.Card.BasePower = 5
	heliod.Card.BaseToughness = 5
	bear := addPerm(gs, 0, "Big Bear", "creature")
	bear.Card.BasePower = 5
	bear.Card.BaseToughness = 5
	gs.Seats[0].ManaPool = 2

	heliodSunCrownedActivate(gs, heliod, 0, nil)

	if bear.Flags["kw:lifelink"] != 1 {
		t.Errorf("Bear should have kw:lifelink after Heliod activate; flags=%v", bear.Flags)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("Mana pool should be emptied to 0 after {1}{W} pay; got %d", gs.Seats[0].ManaPool)
	}
}

func TestHeliod_ActivateFailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	heliod := addPerm(gs, 0, "Heliod, Sun-Crowned", "creature", "legendary", "enchantment")
	bear := addPerm(gs, 0, "Big Bear", "creature")
	bear.Card.BasePower = 5
	bear.Card.BaseToughness = 5
	gs.Seats[0].ManaPool = 0

	heliodSunCrownedActivate(gs, heliod, 0, nil)

	if bear.Flags["kw:lifelink"] == 1 {
		t.Errorf("Should not grant lifelink without mana")
	}
}

// ---------------------------------------------------------------------------
// 9. Gisa, Glorious Resurrector — exile + decayed reanimate
// ---------------------------------------------------------------------------

func TestGisaResurrector_ExilesOpponentCreatureOnDeath(t *testing.T) {
	gs := newGame(t, 2)
	gisa := addPerm(gs, 0, "Gisa, Glorious Resurrector", "creature", "legendary")
	dying := &gameengine.Card{Name: "Opp Goblin", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Graveyard = []*gameengine.Card{dying}

	gisaResurrectorDies(gs, gisa, map[string]interface{}{
		"card":            dying,
		"controller_seat": 1,
	})

	// Should be lifted from seat 1's graveyard into seat 0's exile.
	for _, c := range gs.Seats[1].Graveyard {
		if c == dying {
			t.Errorf("dying card should have left opponent's graveyard")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == dying {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("dying card should be in Gisa's exile pile")
	}
}

func TestGisaResurrector_UpkeepReturnsWithDecayed(t *testing.T) {
	gs := newGame(t, 2)
	gisa := addPerm(gs, 0, "Gisa, Glorious Resurrector", "creature", "legendary")
	stolen := &gameengine.Card{Name: "Opp Goblin", Owner: 1, Types: []string{"creature"}}
	gs.Seats[0].Exile = []*gameengine.Card{stolen}
	gisaExiledPile.Store(gisa, []*gameengine.Card{stolen})

	gisaResurrectorUpkeep(gs, gisa, map[string]interface{}{"active_seat": 0})

	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card == stolen {
			found = true
			if p.Flags["kw:decayed"] != 1 {
				t.Errorf("returned creature should have kw:decayed; flags=%v", p.Flags)
			}
		}
	}
	if !found {
		t.Errorf("stolen creature should be on Gisa's battlefield after upkeep")
	}
}

// ---------------------------------------------------------------------------
// 10. Archmage Ascension — 6+ counter draw → tutor
// ---------------------------------------------------------------------------

func TestArchmageAscension_TutorsAtSixCounters(t *testing.T) {
	gs := newGame(t, 2)
	asc := addPerm(gs, 0, "Archmage Ascension", "enchantment")
	asc.AddCounter("quest", 6)
	bigCard := &gameengine.Card{Name: "Sorcery Big", Owner: 0, Types: []string{"sorcery", "cmc:6"}, CMC: 6}
	smallCard := &gameengine.Card{Name: "Sorcery Small", Owner: 0, Types: []string{"sorcery", "cmc:1"}, CMC: 1}
	gs.Seats[0].Library = []*gameengine.Card{smallCard, bigCard}

	ctx := map[string]interface{}{
		"draw_seat": 0,
	}
	archmageAscensionTutorOnDraw(gs, asc, ctx)

	// Should have moved bigCard to hand.
	foundInHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == bigCard {
			foundInHand = true
		}
	}
	if !foundInHand {
		t.Errorf("Archmage tutor should have moved big card to hand; hand=%+v", gs.Seats[0].Hand)
	}
	if ctx["draw_replaced"] != true {
		t.Errorf("ctx draw_replaced should be set to true")
	}
}

func TestArchmageAscension_NoTutorBelowSix(t *testing.T) {
	gs := newGame(t, 2)
	asc := addPerm(gs, 0, "Archmage Ascension", "enchantment")
	asc.AddCounter("quest", 5)
	c := &gameengine.Card{Name: "Tome", Owner: 0, Types: []string{"sorcery"}}
	gs.Seats[0].Library = []*gameengine.Card{c}

	ctx := map[string]interface{}{"draw_seat": 0}
	archmageAscensionTutorOnDraw(gs, asc, ctx)

	if ctx["draw_replaced"] == true {
		t.Errorf("Should not replace draw below 6 counters")
	}
	if len(gs.Seats[0].Hand) > 0 {
		t.Errorf("Should not have tutored at 5 counters")
	}
}
