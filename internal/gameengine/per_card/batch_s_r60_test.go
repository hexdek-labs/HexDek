package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch S (R60) — tests for 5 new handlers
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Beast Within
// -----------------------------------------------------------------------------

func TestBeastWithin_DestroysPlaneswalkerOverBigCreature(t *testing.T) {
	gs := newGame(t, 2)
	// Opp has a 5/5 creature AND a planeswalker. Tier ordering puts
	// planeswalker (5) above big creature (4) — picker takes the walker.
	bigCreature := addPerm(gs, 1, "Big Beast", "creature")
	bigCreature.Card.BasePower = 5
	bigCreature.Card.BaseToughness = 5
	pw := addPerm(gs, 1, "Tezzeret", "planeswalker")
	pw.Card.BaseToughness = 4

	card := addCard(gs, 0, "Beast Within", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Planeswalker should be gone from opp's battlefield.
	for _, p := range gs.Seats[1].Battlefield {
		if p == pw {
			t.Errorf("expected Tezzeret to be destroyed before the big creature")
		}
	}
	// Big creature should still be on the battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == bigCreature {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Big Beast should still be on opp battlefield (planeswalker took priority)")
	}
}

func TestBeastWithin_CreatesBeastTokenForOpp(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Sol Ring", "artifact")
	_ = target

	card := addCard(gs, 0, "Beast Within", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Opp should own a new 3/3 Beast token.
	foundBeast := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Beast" && p.Card.BasePower == 3 && p.Card.BaseToughness == 3 {
			foundBeast = true
		}
	}
	if !foundBeast {
		t.Errorf("expected a 3/3 Beast token on opp battlefield after Beast Within")
	}
}

func TestBeastWithin_NoTargetNoOp(t *testing.T) {
	gs := newGame(t, 2)
	// Opp has no permanents.

	card := addCard(gs, 0, "Beast Within", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on no targets")
	}
}

// -----------------------------------------------------------------------------
// Time Warp
// -----------------------------------------------------------------------------

func TestTimeWarp_IncrementsExtraTurnsPending(t *testing.T) {
	gs := newGame(t, 2)
	pre := gs.Flags["extra_turns_pending"]

	card := addCard(gs, 0, "Time Warp", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if got := gs.Flags["extra_turns_pending"]; got != pre+1 {
		t.Errorf("expected extra_turns_pending to bump by 1; pre=%d post=%d", pre, got)
	}
	if hasEvent(gs, "extra_turn") < 1 {
		t.Errorf("expected an extra_turn event")
	}
}

func TestTimeWarp_LogsTargetSelf(t *testing.T) {
	gs := newGame(t, 2)
	card := addCard(gs, 0, "Time Warp", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	for _, ev := range gs.EventLog {
		if ev.Kind == "extra_turn" {
			if ev.Seat != 0 {
				t.Errorf("expected extra_turn seat == caster (0), got %d", ev.Seat)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Karmic Guide
// -----------------------------------------------------------------------------

func TestKarmicGuide_ReturnsReveillarkFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	reveillark := &gameengine.Card{Name: "Reveillark", Owner: 0, Types: []string{"creature"}}
	otherCreature := &gameengine.Card{Name: "Mulldrifter", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = []*gameengine.Card{otherCreature, reveillark}

	guide := addPerm(gs, 0, "Karmic Guide", "creature")
	gameengine.InvokeETBHook(gs, guide)

	// Reveillark should be on the battlefield (priority over Mulldrifter).
	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == reveillark {
			foundOnBF = true
		}
	}
	if !foundOnBF {
		t.Errorf("expected Reveillark to be returned to battlefield")
	}
	// Mulldrifter should still be in graveyard.
	stillInGy := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == otherCreature {
			stillInGy = true
		}
	}
	if !stillInGy {
		t.Errorf("Mulldrifter should still be in graveyard (Reveillark won the pick)")
	}
}

func TestKarmicGuide_EmptyGraveyardNoOp(t *testing.T) {
	gs := newGame(t, 2)
	guide := addPerm(gs, 0, "Karmic Guide", "creature")
	gameengine.InvokeETBHook(gs, guide)

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on empty graveyard")
	}
}

func TestKarmicGuide_SkipsNonCreatureCards(t *testing.T) {
	gs := newGame(t, 2)
	// Graveyard has only an instant — no creature target.
	instant := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Graveyard = []*gameengine.Card{instant}

	guide := addPerm(gs, 0, "Karmic Guide", "creature")
	gameengine.InvokeETBHook(gs, guide)

	// Instant should still be in graveyard.
	stillInGy := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == instant {
			stillInGy = true
		}
	}
	if !stillInGy {
		t.Errorf("instant should not be a valid Karmic Guide target")
	}
}

// -----------------------------------------------------------------------------
// Fellwar Stone
// -----------------------------------------------------------------------------

func TestFellwarStone_AddsManaFromOppIslandColor(t *testing.T) {
	gs := newGame(t, 2)
	stone := addPerm(gs, 0, "Fellwar Stone", "artifact")
	// Opp has an Island only.
	addPerm(gs, 1, "Island", "land", "basic", "island")
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, stone, 0, nil)

	if !stone.Tapped {
		t.Errorf("expected Fellwar Stone to be tapped after activation")
	}
	if gs.Seats[0].ManaPool != preMana+1 {
		t.Errorf("expected +1 mana, got pool=%d (was %d)", gs.Seats[0].ManaPool, preMana)
	}
}

func TestFellwarStone_NoOppColorSourcesFails(t *testing.T) {
	gs := newGame(t, 2)
	stone := addPerm(gs, 0, "Fellwar Stone", "artifact")
	// Opp has only a Wastes / Cabal Coffers analog with no basic subtype.
	addPerm(gs, 1, "Cabal Coffers", "land")
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, stone, 0, nil)

	if gs.Seats[0].ManaPool != preMana {
		t.Errorf("expected no mana add, got pool=%d", gs.Seats[0].ManaPool)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no opp color source available")
	}
}

func TestFellwarStone_AlreadyTappedNoOp(t *testing.T) {
	gs := newGame(t, 2)
	stone := addPerm(gs, 0, "Fellwar Stone", "artifact")
	stone.Tapped = true
	addPerm(gs, 1, "Island", "land", "basic", "island")
	preMana := gs.Seats[0].ManaPool

	gameengine.InvokeActivatedHook(gs, stone, 0, nil)

	if gs.Seats[0].ManaPool != preMana {
		t.Errorf("tapped stone should not produce mana")
	}
}

// -----------------------------------------------------------------------------
// Anguished Unmaking
// -----------------------------------------------------------------------------

func TestAnguishedUnmaking_ExilesPlaneswalkerAndCostsLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	pw := addPerm(gs, 1, "Tezzeret", "planeswalker")

	card := addCard(gs, 0, "Anguished Unmaking", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Planeswalker should be in opp exile.
	foundInExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == pw.Card {
			foundInExile = true
		}
	}
	if !foundInExile {
		t.Errorf("expected Tezzeret in opp exile")
	}
	// Controller paid 3 life.
	if gs.Seats[0].Life != 17 {
		t.Errorf("expected life 17 after 3-life cost, got %d", gs.Seats[0].Life)
	}
}

func TestAnguishedUnmaking_CannotTargetLand(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	land := addPerm(gs, 1, "Island", "land", "basic", "island")

	card := addCard(gs, 0, "Anguished Unmaking", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Land should still be on the battlefield.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == land {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("Anguished Unmaking should NOT exile a land")
	}
	// No life cost on no-op (target picker bailed before resolve).
	if gs.Seats[0].Life != 20 {
		t.Errorf("expected life unchanged on no-target no-op, got %d", gs.Seats[0].Life)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when only target is a land")
	}
}

func TestAnguishedUnmaking_PrefersHighEVOverEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	// Opp has a junk enchantment AND a big artifact. Artifact tier (3)
	// outranks enchantment (2) — picker takes the artifact.
	enchant := addPerm(gs, 1, "Junk Enchantment", "enchantment")
	rock := addPerm(gs, 1, "Sol Ring", "artifact")

	card := addCard(gs, 0, "Anguished Unmaking", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Sol Ring should be exiled.
	foundRockExiled := false
	for _, c := range gs.Seats[1].Exile {
		if c == rock.Card {
			foundRockExiled = true
		}
	}
	if !foundRockExiled {
		t.Errorf("expected Sol Ring in exile (higher tier than enchantment)")
	}
	// Enchantment still in play.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == enchant {
			stillThere = true
		}
	}
	if !stillThere {
		t.Errorf("enchantment should still be on opp battlefield")
	}
}
