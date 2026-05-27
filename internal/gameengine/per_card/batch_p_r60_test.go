package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch P (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Cryptic Command
// -----------------------------------------------------------------------------

func TestCrypticCommand_BounceAndDrawWithOppCreature(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D")
	// One big opp creature: bounce score 10+power outranks tap score 2*1.
	opc1 := addPerm(gs, 1, "Big Beater", "creature")
	opc1.Card.BasePower = 5
	opc1.Card.BaseToughness = 5

	card := addCard(gs, 0, "Cryptic Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Picker scores bounce=15, draw=5, tap=2, counter=0 → bounce + draw.
	bounced := true
	for _, p := range gs.Seats[1].Battlefield {
		if p == opc1 {
			bounced = false
		}
	}
	if !bounced {
		t.Errorf("expected Big Beater to be bounced")
	}
	if len(gs.Seats[0].Hand) < 1 {
		t.Errorf("expected at least 1 card drawn, got %d", len(gs.Seats[0].Hand))
	}
}

func TestCrypticCommand_TapAndDrawWhenManyCreatures(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	// Many low-power creatures: tap score = 2*N > bounce (10 + max power).
	// 7 creatures @ power 1: tap=14, bounce=11.
	var creatures []*gameengine.Permanent
	for i := 0; i < 7; i++ {
		c := addPerm(gs, 1, "Token", "creature")
		c.Card.BasePower = 1
		c.Card.BaseToughness = 1
		creatures = append(creatures, c)
	}

	card := addCard(gs, 0, "Cryptic Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	tappedCount := 0
	for _, p := range creatures {
		if p.Tapped {
			tappedCount++
		}
	}
	if tappedCount < 6 {
		t.Errorf("expected most creatures tapped when tap mode wins scoring, got %d/7",
			tappedCount)
	}
}

func TestCrypticCommand_NoTargetsAllDraw(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "A", "B", "C")
	// No opp creatures, no stack.

	card := addCard(gs, 0, "Cryptic Command", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Tap mode scores 0 (no untapped opp creatures), counter 0,
	// bounce 0 → only draw scores positive. Picker picks 2 modes
	// distinct; tap-with-zero is still a valid pick but resolves to
	// no effect (tapping 0 creatures), so we should see 1 draw and
	// 1 "tap" no-op (or 2 draws if tap is excluded).
	if len(gs.Seats[0].Hand) < 1 {
		t.Errorf("expected at least 1 card drawn fallback, got %d", len(gs.Seats[0].Hand))
	}
}

// -----------------------------------------------------------------------------
// Dig Through Time
// -----------------------------------------------------------------------------

func TestDigThroughTime_LooksSevenKeepsTwo(t *testing.T) {
	gs := newGame(t, 2)
	// 8 cards in library — top 7 are looked at.
	bomb := &gameengine.Card{Name: "Bomb", Owner: 0, Types: []string{"sorcery", "cmc:5"}}
	land1 := &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"basic", "land"}}
	land2 := &gameengine.Card{Name: "Mountain", Owner: 0, Types: []string{"basic", "land"}}
	mid := &gameengine.Card{Name: "Tutor", Owner: 0, Types: []string{"sorcery", "cmc:3"}}
	cantrip := &gameengine.Card{Name: "Ponder", Owner: 0, Types: []string{"sorcery", "cmc:1"}}
	bomb2 := &gameengine.Card{Name: "Bomb2", Owner: 0, Types: []string{"creature", "cmc:5"}}
	junk := &gameengine.Card{Name: "Junk", Owner: 0, Types: []string{"land"}}
	deep := &gameengine.Card{Name: "Deep", Owner: 0, Types: []string{"creature"}} // 8th, untouched
	gs.Seats[0].Library = []*gameengine.Card{bomb, land1, land2, mid, cantrip, bomb2, junk, deep}

	card := addCard(gs, 0, "Dig Through Time", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Two highest-keep-score cards should be in hand. Bomb (4) and Bomb2 (4)
	// are the top scorers.
	bombInHand := false
	bomb2InHand := false
	for _, c := range gs.Seats[0].Hand {
		if c == bomb {
			bombInHand = true
		}
		if c == bomb2 {
			bomb2InHand = true
		}
	}
	if !bombInHand || !bomb2InHand {
		t.Errorf("expected both bombs in hand, bomb=%v bomb2=%v",
			bombInHand, bomb2InHand)
	}
	// Library after: 8 - 2 (kept) = 6 cards. The 8th card (Deep) is still
	// near the top — the 7 dug cards were rearranged at the bottom.
	if len(gs.Seats[0].Library) != 6 {
		t.Errorf("expected 6 library cards remaining, got %d", len(gs.Seats[0].Library))
	}
}

func TestDigThroughTime_EmptyLibraryFails(t *testing.T) {
	gs := newGame(t, 2)
	// Library is empty.

	card := addCard(gs, 0, "Dig Through Time", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty library")
	}
}

// -----------------------------------------------------------------------------
// Demonic Pact
// -----------------------------------------------------------------------------

func TestDemonicPact_FirstUpkeepDealsDamageAndGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	pact := addPerm(gs, 0, "Demonic Pact", "enchantment")
	preLife0 := gs.Seats[0].Life
	preLife1 := gs.Seats[1].Life

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[1].Life != preLife1-4 {
		t.Errorf("expected opp to take 4 damage, life=%d (was %d)",
			gs.Seats[1].Life, preLife1)
	}
	if gs.Seats[0].Life != preLife0+4 {
		t.Errorf("expected controller +4 life, life=%d (was %d)",
			gs.Seats[0].Life, preLife0)
	}
	if pact.Flags["pact_mode_damage_chosen"] != 1 {
		t.Errorf("expected damage mode flag set")
	}
}

func TestDemonicPact_FourthUpkeepLosesGame(t *testing.T) {
	gs := newGame(t, 2)
	pact := addPerm(gs, 0, "Demonic Pact", "enchantment")
	pact.Flags = map[string]int{
		"pact_mode_damage_chosen":  1,
		"pact_mode_discard_chosen": 1,
		"pact_mode_draw_chosen":    1,
	}

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 0,
	})

	if !gs.Seats[0].Lost {
		t.Errorf("Demonic Pact's 4th mode should mark controller Lost")
	}
}

func TestDemonicPact_OpponentUpkeepDoesNotFire(t *testing.T) {
	gs := newGame(t, 2)
	pact := addPerm(gs, 0, "Demonic Pact", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 1, // opp's upkeep
	})

	if pact.Flags["pact_mode_damage_chosen"] != 0 {
		t.Errorf("Demonic Pact should not fire on opponent's upkeep")
	}
}

// -----------------------------------------------------------------------------
// Carpet of Flowers
// -----------------------------------------------------------------------------

func TestCarpetOfFlowers_AddsManaPerOppIsland(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	carpet := addPerm(gs, 0, "Carpet of Flowers", "enchantment")
	_ = carpet
	// Opp controls 3 Islands.
	addPerm(gs, 1, "Island1", "land", "island")
	addPerm(gs, 1, "Island2", "land", "island")
	addPerm(gs, 1, "Island3", "land", "island")
	preMana := gs.Seats[0].ManaPool

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{
		"active_seat": 0,
	})

	if gs.Seats[0].ManaPool != preMana+3 {
		t.Errorf("expected +3 mana from 3 Islands, pool=%d (was %d)",
			gs.Seats[0].ManaPool, preMana)
	}
}

func TestCarpetOfFlowers_OncePerTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	addPerm(gs, 0, "Carpet of Flowers", "enchantment")
	addPerm(gs, 1, "Island1", "land", "island")
	preMana := gs.Seats[0].ManaPool

	// Fire twice — should only add mana once.
	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{"active_seat": 0})
	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{"active_seat": 0})

	if gs.Seats[0].ManaPool != preMana+1 {
		t.Errorf("expected +1 mana (once-per-turn gate), got %d (was %d)",
			gs.Seats[0].ManaPool, preMana)
	}
}

func TestCarpetOfFlowers_NoOppIslandsNoOp(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Carpet of Flowers", "enchantment")
	// No opp Islands.
	addPerm(gs, 1, "Forest", "land", "basic")
	preMana := gs.Seats[0].ManaPool

	gameengine.FireCardTrigger(gs, "upkeep", map[string]interface{}{"active_seat": 0})

	if gs.Seats[0].ManaPool != preMana {
		t.Errorf("should not add mana with no opp Islands, pool=%d", gs.Seats[0].ManaPool)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on no-islands")
	}
}

// -----------------------------------------------------------------------------
// Praetor's Grasp
// -----------------------------------------------------------------------------

func TestPraetorsGrasp_ExilesOppWinconAndGrantsCast(t *testing.T) {
	gs := newGame(t, 2)
	// Opp library has a Thassa's Oracle and some junk.
	junk := &gameengine.Card{Name: "Junk", Owner: 1, Types: []string{"land"}}
	oracle := &gameengine.Card{Name: "Thassa's Oracle", Owner: 1, Types: []string{"creature"}}
	junk2 := &gameengine.Card{Name: "Junk2", Owner: 1, Types: []string{"creature"}}
	gs.Seats[1].Library = []*gameengine.Card{junk, oracle, junk2}
	gs.Seats[1].Hand = []*gameengine.Card{{Name: "h", Owner: 1}}

	card := addCard(gs, 0, "Praetor's Grasp", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Oracle should be in owner's (opp's) exile — Praetor's Grasp
	// moves the card to exile while preserving owner, with a
	// ZoneCastGrant giving the controller the cast permission.
	foundInExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == oracle {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("Thassa's Oracle should be in opp owner's exile")
	}
	// Opp library should be missing the Oracle.
	for _, c := range gs.Seats[1].Library {
		if c == oracle {
			t.Errorf("Oracle should be removed from opp library")
		}
	}
	// ZoneCastGrant should be registered.
	grant := gameengine.GetZoneCastGrant(gs, oracle)
	if grant == nil {
		t.Errorf("expected ZoneCastGrant on exiled card")
	}
}

func TestPraetorsGrasp_EmptyOppLibraryFails(t *testing.T) {
	gs := newGame(t, 2)
	// Opp library empty.

	card := addCard(gs, 0, "Praetor's Grasp", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty opp library")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke
// -----------------------------------------------------------------------------

func TestBatchPR60_AllRegistered(t *testing.T) {
	if !HasResolve("Cryptic Command") {
		t.Errorf("Cryptic Command not registered")
	}
	if !HasResolve("Dig Through Time") {
		t.Errorf("Dig Through Time not registered")
	}
	if !HasTrigger("Demonic Pact", "upkeep") {
		t.Errorf("Demonic Pact upkeep trigger not registered")
	}
	if !HasTrigger("Carpet of Flowers", "upkeep") {
		t.Errorf("Carpet of Flowers upkeep trigger not registered")
	}
	if !HasResolve("Praetor's Grasp") {
		t.Errorf("Praetor's Grasp not registered")
	}
}
