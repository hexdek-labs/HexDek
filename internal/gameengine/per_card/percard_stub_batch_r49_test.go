package per_card

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R49 stub-batch port — 10 engine-piece ports: Wheel/Doubling-Season/
// Cyclonic-Rift-class effects + cost-modifier amplifiers.
//
// Picks:
//   1. The Capitoline Triad         cost {1} less per historic GY card
//   2. Lyse Hext                    noncreature spells cost {1} less
//   3. Magnus the Red               I/S cost {1} less per creature token
//   4. Morophon, the Boundless      chosen-type spells cost 5 less
//   5. The Locust God               {2}{U}{R}: draw + discard activated
//   6. Firesong and Sunspeaker      red I/S lifelink (modeled on noncombat damage)
//   7. Mendicant Core, Guidelight   max-speed artifact spell copy
//   8. Cleopatra, Exiled Pharaoh    self-betrayal trigger
//   9. Sandman, Shifting Scoundrel  CDA P/T = lands
//  10. Old One Eye                  trample anthem

// ---------------------------------------------------------------------------
// 1. The Capitoline Triad cost reduction
// ---------------------------------------------------------------------------

func TestCapitolineTriad_CostReductionPerHistoricGYCard(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "The Capitoline Triad", "artifact", "legendary")
	// Three historic cards in seat 0's graveyard.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
		{Name: "Mox Diamond", Owner: 0, Types: []string{"artifact", "legendary"}},
		{Name: "History of Benalia", Owner: 0, Types: []string{"enchantment", "saga"}},
	}

	// Cast another Triad: expect 3 mana reduction.
	another := &gameengine.Card{Name: "The Capitoline Triad", Owner: 0, Types: []string{"artifact", "legendary"}}
	mods := gameengine.ScanCostModifiers(gs, another, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "The Capitoline Triad" && m.Kind == gameengine.CostModReduction {
			total += m.Amount
		}
	}
	if total != 3 {
		t.Errorf("expected 3 cost reduction from 3 historic GY cards, got %d (mods=%+v)", total, mods)
	}
}

func TestCapitolineTriad_NoReductionForNonTriadSpells(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "The Capitoline Triad", "artifact", "legendary")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}},
	}

	// Cast a non-Triad spell: no reduction.
	other := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, other, 0)
	for _, m := range mods {
		if m.Source == "The Capitoline Triad" {
			t.Errorf("Triad should not discount non-Triad spells: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Lyse Hext noncreature reduction
// ---------------------------------------------------------------------------

func TestLyseHext_NoncreatureSpellsDiscount1(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lyse Hext", "creature", "legendary")

	instant := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, instant, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Lyse Hext" {
			total += m.Amount
		}
	}
	if total != 1 {
		t.Errorf("expected Lyse Hext to discount noncreature spell by 1, got %d (mods=%+v)", total, mods)
	}
}

func TestLyseHext_CreatureSpellsNotDiscounted(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lyse Hext", "creature", "legendary")

	creature := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	mods := gameengine.ScanCostModifiers(gs, creature, 0)
	for _, m := range mods {
		if m.Source == "Lyse Hext" {
			t.Errorf("Lyse should not discount creature spells: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Magnus the Red Unearthly Power
// ---------------------------------------------------------------------------

func TestMagnusTheRed_DiscountPerCreatureToken(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Magnus the Red", "creature", "legendary")
	// Add 3 creature tokens.
	for i := 0; i < 3; i++ {
		tok := addPerm(gs, 0, "Spawn Token", "creature", "token")
		_ = tok
	}
	// Add a non-token creature (shouldn't count).
	addPerm(gs, 0, "Llanowar Elves", "creature")

	instant := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, instant, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Magnus the Red" {
			total += m.Amount
		}
	}
	if total != 3 {
		t.Errorf("expected Magnus discount=3 from 3 creature tokens, got %d (mods=%+v)", total, mods)
	}
}

// ---------------------------------------------------------------------------
// 4. Morophon, the Boundless chosen-tribe discount
// ---------------------------------------------------------------------------

func TestMorophon_DiscountsChosenTribe(t *testing.T) {
	gs := newGame(t, 2)
	moroph := addPerm(gs, 0, "Morophon, the Boundless", "creature", "legendary", "morophon_tribe:dragon")

	_ = moroph
	dragon := &gameengine.Card{Name: "Lathliss", Owner: 0, Types: []string{"creature", "dragon"}}
	mods := gameengine.ScanCostModifiers(gs, dragon, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Morophon, the Boundless" {
			total += m.Amount
		}
	}
	if total != 5 {
		t.Errorf("expected Morophon to discount chosen-tribe spell by 5, got %d (mods=%+v)", total, mods)
	}
}

func TestMorophon_DoesNotDiscountOffTribe(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Morophon, the Boundless", "creature", "legendary", "morophon_tribe:dragon")

	knight := &gameengine.Card{Name: "Knight", Owner: 0, Types: []string{"creature", "knight"}}
	mods := gameengine.ScanCostModifiers(gs, knight, 0)
	for _, m := range mods {
		if m.Source == "Morophon, the Boundless" {
			t.Errorf("Morophon should not discount off-tribe spell: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// 5. The Locust God {2}{U}{R}: loot
// ---------------------------------------------------------------------------

func TestTheLocustGod_LootDrawsAndDiscards(t *testing.T) {
	gs := newGame(t, 2)
	locust := addPerm(gs, 0, "The Locust God", "creature", "legendary", "god")
	gs.Seats[0].ManaPool = 4
	// Library has one card; hand has two (one land + one nonland).
	addLibrary(gs, 0, "Drawn Card")
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Mountain", Owner: 0, Types: []string{"land", "basic"}},
		{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature"}},
	}

	theLocustGodLootActivate(gs, locust, 0, nil)

	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("expected mana pool emptied after {4} cost, got %d", gs.Seats[0].ManaPool)
	}
	// Drawn Card should be in hand.
	foundDrawn := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Drawn Card" {
			foundDrawn = true
		}
	}
	if !foundDrawn {
		t.Errorf("expected Drawn Card in hand after loot draw; hand=%v", handNames(gs.Seats[0].Hand))
	}
	// Discard should have hit the lowest-CMC nonland (Llanowar Elves, CMC=0).
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected 1 discard in graveyard, got %d", len(gs.Seats[0].Graveyard))
	} else if gs.Seats[0].Graveyard[0].DisplayName() == "Mountain" {
		t.Errorf("loot should prefer discarding nonland; discarded Mountain instead")
	}
}

func TestTheLocustGod_LootFailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	locust := addPerm(gs, 0, "The Locust God", "creature", "legendary")
	gs.Seats[0].ManaPool = 1
	addLibrary(gs, 0, "Drawn Card")

	theLocustGodLootActivate(gs, locust, 0, nil)

	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana pool should stay at 1 when activation fails, got %d", gs.Seats[0].ManaPool)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("hand should remain empty if activation failed; got %v", handNames(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// 6. Firesong and Sunspeaker red I/S lifelink stamp
// ---------------------------------------------------------------------------

func TestFiresong_TracksRedInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	fs := addPerm(gs, 0, "Firesong and Sunspeaker", "creature", "legendary")

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, Colors: []string{"R"}}
	firesongTrackRedCast(gs, fs, map[string]interface{}{
		"caster_seat": 0,
		"card":        bolt,
	})

	if gs.Flags["firesong_red_active_seat"] != 1 {
		t.Errorf("expected firesong_red_active_seat=1 (seat0+1), got %d", gs.Flags["firesong_red_active_seat"])
	}
	if gs.Flags["firesong_red_active_turn"] != gs.Turn {
		t.Errorf("expected turn marker set")
	}
}

func TestFiresong_NoncombatDamageGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	fs := addPerm(gs, 0, "Firesong and Sunspeaker", "creature", "legendary")
	startLife := gs.Seats[0].Life

	// Track a red I/S cast.
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, Colors: []string{"R"}}
	firesongTrackRedCast(gs, fs, map[string]interface{}{
		"caster_seat": 0,
		"card":        bolt,
	})

	// Damage event fires.
	firesongLifelinkOnPlayer(gs, fs, map[string]interface{}{
		"seat":   1,
		"amount": 3,
		"source": "Lightning Bolt",
	})

	if gs.Seats[0].Life != startLife+3 {
		t.Errorf("expected lifelink to gain 3 life, before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

func TestFiresong_WhiteInstantNotTrackedRed(t *testing.T) {
	gs := newGame(t, 2)
	fs := addPerm(gs, 0, "Firesong and Sunspeaker", "creature", "legendary")

	wrath := &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery"}, Colors: []string{"W"}}
	firesongTrackRedCast(gs, fs, map[string]interface{}{
		"caster_seat": 0,
		"card":        wrath,
	})

	if gs.Flags["firesong_red_active_seat"] != 0 {
		t.Errorf("white I/S should NOT set red-active flag")
	}
}

// ---------------------------------------------------------------------------
// 7. Mendicant Core, Guidelight — max-speed copy
// ---------------------------------------------------------------------------

func TestMendicantCore_MaxSpeedCopiesArtifactSpell(t *testing.T) {
	gs := newGame(t, 2)
	mend := addPerm(gs, 0, "Mendicant Core, Guidelight", "creature", "artifact", "legendary")
	mend.Flags["speed"] = 4
	gs.Seats[0].ManaPool = 5

	// Put an artifact spell on the stack.
	artCard := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	stackItem := &gameengine.StackItem{Controller: 0, Card: artCard, Kind: "spell"}
	gs.Stack = append(gs.Stack, stackItem)

	preStack := len(gs.Stack)
	mendicantMaxSpeedCopy(gs, mend, map[string]interface{}{
		"caster_seat": 0,
		"card":        artCard,
	})

	if len(gs.Stack) != preStack+1 {
		t.Errorf("expected one copy pushed to stack, before=%d after=%d", preStack, len(gs.Stack))
	}
	top := gs.Stack[len(gs.Stack)-1]
	if !top.IsCopy {
		t.Errorf("top stack item should be marked IsCopy")
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("expected {1} paid for copy, mana pool should be 4 got %d", gs.Seats[0].ManaPool)
	}
}

func TestMendicantCore_NoCopyBelowMaxSpeed(t *testing.T) {
	gs := newGame(t, 2)
	mend := addPerm(gs, 0, "Mendicant Core, Guidelight", "creature", "artifact", "legendary")
	mend.Flags["speed"] = 3 // not max
	gs.Seats[0].ManaPool = 5

	artCard := &gameengine.Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	stackItem := &gameengine.StackItem{Controller: 0, Card: artCard, Kind: "spell"}
	gs.Stack = append(gs.Stack, stackItem)
	preStack := len(gs.Stack)

	mendicantMaxSpeedCopy(gs, mend, map[string]interface{}{
		"caster_seat": 0,
		"card":        artCard,
	})

	if len(gs.Stack) != preStack {
		t.Errorf("should not copy below max speed; stack changed %d→%d", preStack, len(gs.Stack))
	}
}

func TestMendicantCore_NonArtifactIgnored(t *testing.T) {
	gs := newGame(t, 2)
	mend := addPerm(gs, 0, "Mendicant Core, Guidelight", "creature", "artifact", "legendary")
	mend.Flags["speed"] = 4
	gs.Seats[0].ManaPool = 5

	instCard := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	stackItem := &gameengine.StackItem{Controller: 0, Card: instCard, Kind: "spell"}
	gs.Stack = append(gs.Stack, stackItem)
	preStack := len(gs.Stack)

	mendicantMaxSpeedCopy(gs, mend, map[string]interface{}{
		"caster_seat": 0,
		"card":        instCard,
	})

	if len(gs.Stack) != preStack {
		t.Errorf("non-artifact spell should not be copied; stack changed %d→%d", preStack, len(gs.Stack))
	}
}

// ---------------------------------------------------------------------------
// 8. Cleopatra self-betrayal
// ---------------------------------------------------------------------------

func TestCleopatra_SelfBetrayalDrawsAndLoses2Life(t *testing.T) {
	gs := newGame(t, 2)
	cleo := addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary")
	cleo.Counters["+1/+1"] = 4
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	startLife := gs.Seats[0].Life

	// Self-dies trigger.
	cleopatraBetrayalDies(gs, cleo, map[string]interface{}{
		"perm":            cleo,
		"card":            cleo.Card,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 4 {
		t.Errorf("expected 4 cards drawn from 4 counters on dying Cleo, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != startLife-2 {
		t.Errorf("expected -2 life on betrayal, before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// 9. Sandman CDA
// ---------------------------------------------------------------------------

func TestSandman_PowerToughnessEqualsLandCount(t *testing.T) {
	gs := newGame(t, 2)
	sand := addPerm(gs, 0, "Sandman, Shifting Scoundrel", "creature", "legendary")
	// Add 5 lands.
	for i := 0; i < 5; i++ {
		addPerm(gs, 0, "Mountain", "land", "basic")
	}

	// R56: Sandman's CDA is a Layer 7b continuous effect registered at
	// ETB. Effective characteristics (not Card.BasePower) carry the
	// runtime value. Pre-R56 this test read Card.BasePower directly.
	sandmanRefreshPTOnETB(gs, sand)

	chars := gameengine.GetEffectiveCharacteristics(gs, sand)
	if chars.Power != 5 {
		t.Errorf("expected Sandman effective power 5 with 5 lands, got %d", chars.Power)
	}
	if chars.Toughness != 5 {
		t.Errorf("expected Sandman effective toughness 5 with 5 lands, got %d", chars.Toughness)
	}
}

func TestSandman_RefreshOnPermanentETB(t *testing.T) {
	gs := newGame(t, 2)
	sand := addPerm(gs, 0, "Sandman, Shifting Scoundrel", "creature", "legendary")
	addPerm(gs, 0, "Island", "land", "basic")
	sandmanRefreshPTOnETB(gs, sand)
	chars := gameengine.GetEffectiveCharacteristics(gs, sand)
	if chars.Power != 1 {
		t.Fatalf("baseline mismatch: want effective power 1, got %d", chars.Power)
	}

	// Second land enters — the CDA re-evaluates on the next layer pass.
	addPerm(gs, 0, "Forest", "land", "basic")
	gs.InvalidateCharacteristicsCache()
	chars = gameengine.GetEffectiveCharacteristics(gs, sand)
	if chars.Power != 2 {
		t.Errorf("expected effective power to refresh to 2 after second land, got %d", chars.Power)
	}
}

// ---------------------------------------------------------------------------
// 10. Old One Eye trample anthem
// ---------------------------------------------------------------------------

func TestOldOneEye_StampsTrampleOnOtherCreatures(t *testing.T) {
	gs := newGame(t, 2)
	one := addPerm(gs, 0, "Old One Eye", "creature", "legendary")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	knight := addPerm(gs, 0, "White Knight", "creature")
	// An artifact (non-creature) should NOT be stamped.
	ring := addPerm(gs, 0, "Sol Ring", "artifact")
	// An opponent creature should NOT be stamped.
	oppCreature := addPerm(gs, 1, "Goblin", "creature")

	oldOneEyeApplyAnthemOnETB(gs, one)

	if bear.Flags["kw:trample"] != 1 {
		t.Errorf("expected trample stamped on Grizzly Bears; flags=%v", bear.Flags)
	}
	if knight.Flags["kw:trample"] != 1 {
		t.Errorf("expected trample stamped on White Knight; flags=%v", knight.Flags)
	}
	if ring.Flags["kw:trample"] == 1 {
		t.Errorf("trample should NOT be stamped on non-creature (Sol Ring)")
	}
	if oppCreature.Flags["kw:trample"] == 1 {
		t.Errorf("trample should NOT be stamped on opponent's creature")
	}
}

func TestOldOneEye_DoesNotStampSelf(t *testing.T) {
	gs := newGame(t, 2)
	one := addPerm(gs, 0, "Old One Eye", "creature", "legendary")

	oldOneEyeApplyAnthemOnETB(gs, one)

	if one.Flags["kw:trample_from_old_one_eye"] == 1 {
		t.Errorf("Old One Eye should not stamp the from-marker on itself")
	}
}

// ---------------------------------------------------------------------------
// Registration smoke test
// ---------------------------------------------------------------------------

func TestBatchDR49_AllHandlersRegistered(t *testing.T) {
	r := Global()
	// Cost-modifier cards don't need per_card registration; they just need
	// gen_*.go ETB-side wiring that's already present.
	// The 5 custom handlers DO register events.
	customs := []struct {
		card  string
		event string
	}{
		{"The Locust God", "activated"},
		{"Firesong and Sunspeaker", "trigger:instant_or_sorcery_cast"},
		{"Mendicant Core, Guidelight", "trigger:spell_cast"},
		{"Sandman, Shifting Scoundrel", "trigger:permanent_etb"},
		{"Old One Eye", "trigger:permanent_etb"},
	}
	_ = r
	for _, c := range customs {
		var ok bool
		switch {
		case c.event == "activated":
			ok = HasActivated(c.card)
		case strings.HasPrefix(c.event, "trigger:"):
			ok = HasTrigger(c.card, strings.TrimPrefix(c.event, "trigger:"))
		}
		if !ok {
			t.Errorf("batchD card %q: missing registration for %q", c.card, c.event)
		}
	}
}
