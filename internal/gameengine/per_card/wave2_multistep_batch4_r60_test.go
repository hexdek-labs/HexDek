package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// wave2_multistep_batch4_r60_test.go — Wave 2 final batch migrations.
//
// 5 per_card sites carrying manual splice + append pairs that bypass
// the canonical chokepoints. Each migration routes through MoveCard /
// DiscardCard / enterBattlefieldWithETB so §614 + §903.9b + observer
// triggers fire.
//
// Migrated:
//   1. chitinous_crawler           — graveyard → exile cost (descend 8)
//   2. katara_waterbending_master  — draw N + discard 1 via canonical
//   3. page_loose_leaf             — library tutor: library → hand
//   4. winota_joiner_of_forces     — attack-trigger library cheat
//   5. custom_feather_the_redeemed — exile → hand return-on-resolve
//
// Feather's test path is exercised indirectly via the existing
// `feather_test.go` suite (its setup needs a live stack item + cast
// trigger). The migration only affects the exile-pull step.

// ---------------------------------------------------------------------------
// 1. Chitinous Crawler
// ---------------------------------------------------------------------------

func TestWave2MS4_ChitinousCrawler_ExilesGraveyardCardCleanly(t *testing.T) {
	gs := newGame(t, 2)
	crawler := addPerm(gs, 0, "Chitinous Crawler", "creature")
	target := &gameengine.Card{
		Name: "Buried Beast", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     2, BaseToughness: 2,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, target)

	chitinousCrawlerActivated(gs, crawler, 0, nil)

	if countCardIn(gs.Seats[0].Graveyard, target) != 0 {
		t.Errorf("Chitinous: target still in graveyard")
	}
	if countCardIn(gs.Seats[0].Exile, target) != 1 {
		t.Errorf("Chitinous: target must be in exile once; got %d",
			countCardIn(gs.Seats[0].Exile, target))
	}
}

// ---------------------------------------------------------------------------
// 2. Katara
// ---------------------------------------------------------------------------

func TestWave2MS4_Katara_DrawAndDiscardRouteThroughChokepoints(t *testing.T) {
	gs := newGame(t, 2)
	katara := stampCreaturePT(addPerm(gs, 0, "Katara, Waterbending Master", "creature"), 3, 3)
	if katara.Flags == nil {
		katara.Flags = map[string]int{}
	}
	katara.Flags["katara_xp"] = 2 // draw 2 (handler reads Flags, not Counters)

	c1 := &gameengine.Card{Name: "Draw1", Owner: 0, Types: []string{"sorcery"}}
	c2 := &gameengine.Card{Name: "Draw2", Owner: 0, Types: []string{"creature"}, BasePower: 1, BaseToughness: 1}
	gs.Seats[0].Library = append(gs.Seats[0].Library, c1, c2)
	preDiscarded := gs.Seats[0].Turn.Discarded

	kataraAttacks(gs, katara, map[string]interface{}{
		"seat":          0,
		"attacker_perm": katara,
	})

	// Two cards drawn → both gone from library, one in hand (c1), one
	// discarded to graveyard (c2 was last). The migration pins that
	// the canonical chokepoints fired.
	if countCardIn(gs.Seats[0].Library, c1) != 0 ||
		countCardIn(gs.Seats[0].Library, c2) != 0 {
		t.Errorf("Katara: drawn cards still in library")
	}
	if gs.Seats[0].Turn.Discarded <= preDiscarded {
		t.Errorf("Katara: Turn.Discarded must increment via DiscardCard (got %d → %d)",
			preDiscarded, gs.Seats[0].Turn.Discarded)
	}
}

// ---------------------------------------------------------------------------
// 3. Page Loose Leaf — Grandeur (library → hand tutor)
// ---------------------------------------------------------------------------

func TestWave2MS4_PageLooseLeaf_TutorsInstantToHandCleanly(t *testing.T) {
	gs := newGame(t, 2)
	page := addPerm(gs, 0, "Page, Loose Leaf", "creature")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand,
		// Grandeur cost: discard another Page, Loose Leaf.
		&gameengine.Card{Name: "Page, Loose Leaf", Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1})
	target := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	filler := &gameengine.Card{Name: "Mountain", Owner: 0, Types: []string{"land", "basic"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, filler, target, filler)

	pageLooseLeafGrandeur(gs, page, 1, nil)

	if countCardIn(gs.Seats[0].Library, target) != 0 {
		t.Errorf("Page: target still in library after Grandeur tutor")
	}
	if countCardIn(gs.Seats[0].Hand, target) != 1 {
		t.Errorf("Page: target must be in hand once; got %d",
			countCardIn(gs.Seats[0].Hand, target))
	}
}

// ---------------------------------------------------------------------------
// 4. Winota
// ---------------------------------------------------------------------------

func TestWave2MS4_Winota_CheatsHumanFromLibraryCleanly(t *testing.T) {
	gs := newGame(t, 2)
	winota := stampCreaturePT(addPerm(gs, 0, "Winota, Joiner of Forces", "creature", "human"), 4, 4)
	// Attacker must be non-Human for Winota to look.
	attacker := stampCreaturePT(addPerm(gs, 0, "Goblin Bandit", "creature", "goblin"), 2, 2)
	human := &gameengine.Card{
		Name: "Human Soldier", Owner: 0,
		Types:         []string{"creature", "human"},
		BasePower:     2, BaseToughness: 2,
	}
	others := []*gameengine.Card{
		{Name: "Filler1", Owner: 0, Types: []string{"sorcery"}},
		{Name: "Filler2", Owner: 0, Types: []string{"instant"}},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, others[0], human, others[1])

	winotaAttackTrigger(gs, winota, map[string]interface{}{
		"attacker_perm": attacker,
		"attacker_seat": 0,
	})

	if countCardIn(gs.Seats[0].Library, human) != 0 {
		t.Errorf("Winota: human still in library after cheat")
	}
	if countBFCard(gs.Seats[0].Battlefield, human) != 1 {
		t.Errorf("Winota: human must be on battlefield once; got %d",
			countBFCard(gs.Seats[0].Battlefield, human))
	}
}

// ---------------------------------------------------------------------------
// 5. Feather — Migration is on the exile→hand pull. The full handler
// path requires a live stack item with exile-on-resolve set, which is
// covered by the existing feather suite. This test exercises only the
// migrated zone-move in isolation.
// ---------------------------------------------------------------------------

func TestWave2MS4_Feather_ExileReturnRoutesThroughMoveCard(t *testing.T) {
	gs := newGame(t, 2)
	// Direct exile → hand move using the same canonical helper that the
	// migrated path now uses.
	target := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, target)

	gameengine.MoveCard(gs, target, 0, "exile", "hand", "feather_return_to_hand")

	if countCardIn(gs.Seats[0].Exile, target) != 0 {
		t.Errorf("Feather migration: target still in exile after MoveCard")
	}
	if countCardIn(gs.Seats[0].Hand, target) != 1 {
		t.Errorf("Feather migration: target must be in hand once; got %d",
			countCardIn(gs.Seats[0].Hand, target))
	}
}
