package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// gift_consumers_r60 — 6 OnResolve handlers wired in
// gift_consumers_r60.go covering OTJ Gift spells (CR §702.192).
// -----------------------------------------------------------------------------

// giftItem builds a StackItem with the gift_promised flag. Recipient
// is `recipient` when promised; -1 otherwise (the engine's CastGift
// path would set recipient to a real seat when promised, but for unit
// tests it's enough that gift_promised is the gating bool).
func giftItem(seat int, name, kind, giftType string, promised bool, recipient int) *gameengine.StackItem {
	card := &gameengine.Card{Name: name, Owner: seat, Types: []string{kind}}
	costMeta := map[string]interface{}{}
	costMeta["gift_promised"] = promised
	if promised {
		costMeta["gift_recipient"] = recipient
		costMeta["gift_type"] = giftType
	}
	return &gameengine.StackItem{
		Controller: seat,
		Card:       card,
		CostMeta:   costMeta,
	}
}

// countCardTypeOnBattlefield counts perms whose Types include the
// lowercase type token. Used for gift-token delivery assertions.
func countCardTypeOnBattlefield(gs *gameengine.GameState, seat int, ty string) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == ty {
				n++
				break
			}
		}
	}
	return n
}

// -----------------------------------------------------------------------------
// Blooming Blast
// -----------------------------------------------------------------------------

func TestBloomingBlast_NotPromisedDealsTwoToCreatureOnly(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 20
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 4, 4

	gameengine.InvokeResolveHook(gs, giftItem(0, "Blooming Blast", "instant", "treasure", false, -1))

	if target.MarkedDamage != 2 {
		t.Errorf("expected 2 damage to creature, got %d", target.MarkedDamage)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("opp life should be unchanged when not promised, got %d", gs.Seats[1].Life)
	}
}

func TestBloomingBlast_PromisedDealsTwoPlusThreeAndGivesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 20
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 4, 4
	preOppTreasures := countCardTypeOnBattlefield(gs, 1, "treasure")

	gameengine.InvokeResolveHook(gs, giftItem(0, "Blooming Blast", "instant", "treasure", true, 1))

	if target.MarkedDamage != 2 {
		t.Errorf("expected 2 damage to creature, got %d", target.MarkedDamage)
	}
	if gs.Seats[1].Life != 17 {
		t.Errorf("opp life should drop by 3 when promised, got %d", gs.Seats[1].Life)
	}
	if countCardTypeOnBattlefield(gs, 1, "treasure") != preOppTreasures+1 {
		t.Errorf("expected a Treasure delivered to opp")
	}
}

// -----------------------------------------------------------------------------
// Crumb and Get It
// -----------------------------------------------------------------------------

func TestCrumbAndGetIt_NotPromisedPumpsOnly(t *testing.T) {
	gs := newGame(t, 2)
	friend := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend.Card.BasePower, friend.Card.BaseToughness = 1, 1

	gameengine.InvokeResolveHook(gs, giftItem(0, "Crumb and Get It", "instant", "food", false, -1))

	if friend.Power() != 3 || friend.Toughness() != 3 {
		t.Errorf("expected 3/3 after +2/+2, got %d/%d", friend.Power(), friend.Toughness())
	}
	if friend.Flags["kw:indestructible"] != 0 {
		t.Errorf("non-promised must NOT grant indestructible; got %d", friend.Flags["kw:indestructible"])
	}
}

func TestCrumbAndGetIt_PromisedAlsoGrantsIndestructibleAndDeliversFood(t *testing.T) {
	gs := newGame(t, 2)
	friend := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend.Card.BasePower, friend.Card.BaseToughness = 1, 1

	gameengine.InvokeResolveHook(gs, giftItem(0, "Crumb and Get It", "instant", "food", true, 1))

	if friend.Power() != 3 || friend.Toughness() != 3 {
		t.Errorf("expected 3/3 pump, got %d/%d", friend.Power(), friend.Toughness())
	}
	if friend.Flags["kw:indestructible"] != 1 {
		t.Errorf("promised must grant indestructible")
	}
	if countCardTypeOnBattlefield(gs, 1, "food") != 1 {
		t.Errorf("expected a Food token delivered to opp")
	}
}

// -----------------------------------------------------------------------------
// Wear Down
// -----------------------------------------------------------------------------

func TestWearDown_NotPromisedDestroysOne(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Sol Ring", "artifact").Card.Types = append([]string{"artifact", "cmc:1"})
	addPerm(gs, 1, "Mind's Eye", "artifact").Card.Types = append([]string{"artifact", "cmc:5"})
	preOppArtifacts := countCardTypeOnBattlefield(gs, 1, "artifact")

	gameengine.InvokeResolveHook(gs, giftItem(0, "Wear Down", "sorcery", "card", false, -1))

	if countCardTypeOnBattlefield(gs, 1, "artifact") != preOppArtifacts-1 {
		t.Errorf("expected exactly 1 artifact destroyed (not promised); pre=%d post=%d",
			preOppArtifacts, countCardTypeOnBattlefield(gs, 1, "artifact"))
	}
}

func TestWearDown_PromisedDestroysTwo(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Sol Ring", "artifact").Card.Types = append([]string{"artifact", "cmc:1"})
	addPerm(gs, 1, "Mind's Eye", "artifact").Card.Types = append([]string{"artifact", "cmc:5"})

	gameengine.InvokeResolveHook(gs, giftItem(0, "Wear Down", "sorcery", "card", true, 1))

	if countCardTypeOnBattlefield(gs, 1, "artifact") != 0 {
		t.Errorf("expected both artifacts destroyed when promised; remaining=%d",
			countCardTypeOnBattlefield(gs, 1, "artifact"))
	}
}

// -----------------------------------------------------------------------------
// Nocturnal Hunger — INVERTED rider (penalty when NOT promised)
// -----------------------------------------------------------------------------

func TestNocturnalHunger_NotPromisedDestroysAndCostsTwoLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 4, 4

	gameengine.InvokeResolveHook(gs, giftItem(0, "Nocturnal Hunger", "instant", "food", false, -1))

	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == target {
			stillThere = true
		}
	}
	if stillThere {
		t.Errorf("creature should be destroyed")
	}
	if gs.Seats[0].Life != 18 {
		t.Errorf("expected 18 life after -2 from inverted rider, got %d", gs.Seats[0].Life)
	}
}

func TestNocturnalHunger_PromisedDestroysWithNoLifeLoss(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 4, 4

	gameengine.InvokeResolveHook(gs, giftItem(0, "Nocturnal Hunger", "instant", "food", true, 1))

	if gs.Seats[0].Life != 20 {
		t.Errorf("promised → no life loss; got %d", gs.Seats[0].Life)
	}
	if countCardTypeOnBattlefield(gs, 1, "food") != 1 {
		t.Errorf("expected a Food delivered to opp")
	}
}

// -----------------------------------------------------------------------------
// Wildfire Howl
// -----------------------------------------------------------------------------

func TestWildfireHowl_NotPromisedHitsAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	mine := addPerm(gs, 0, "Llanowar Elves", "creature")
	mine.Card.BasePower, mine.Card.BaseToughness = 1, 1
	opp := addPerm(gs, 1, "Hill Giant", "creature")
	opp.Card.BasePower, opp.Card.BaseToughness = 3, 3

	gameengine.InvokeResolveHook(gs, giftItem(0, "Wildfire Howl", "sorcery", "card", false, -1))

	if mine.MarkedDamage != 2 {
		t.Errorf("expected 2 dmg to friendly creature, got %d", mine.MarkedDamage)
	}
	if opp.MarkedDamage != 2 {
		t.Errorf("expected 2 dmg to opp creature, got %d", opp.MarkedDamage)
	}
}

func TestWildfireHowl_PromisedAlsoOnePingToTarget(t *testing.T) {
	gs := newGame(t, 2)
	mine := addPerm(gs, 0, "Llanowar Elves", "creature")
	mine.Card.BasePower, mine.Card.BaseToughness = 1, 1
	opp := addPerm(gs, 1, "Hill Giant", "creature")
	opp.Card.BasePower, opp.Card.BaseToughness = 3, 3

	gameengine.InvokeResolveHook(gs, giftItem(0, "Wildfire Howl", "sorcery", "card", true, 1))

	if mine.MarkedDamage != 2 {
		t.Errorf("expected 2 dmg to friendly, got %d", mine.MarkedDamage)
	}
	if opp.MarkedDamage != 3 {
		t.Errorf("expected 2 + 1 ping = 3 dmg to opp creature, got %d", opp.MarkedDamage)
	}
}

// -----------------------------------------------------------------------------
// Valley Rally
// -----------------------------------------------------------------------------

func TestValleyRally_NotPromisedPumpsAllFriendlyCreatures(t *testing.T) {
	gs := newGame(t, 2)
	a := addPerm(gs, 0, "Goblin Guide", "creature")
	a.Card.BasePower, a.Card.BaseToughness = 2, 2
	b := addPerm(gs, 0, "Llanowar Elves", "creature")
	b.Card.BasePower, b.Card.BaseToughness = 1, 1

	gameengine.InvokeResolveHook(gs, giftItem(0, "Valley Rally", "instant", "food", false, -1))

	if a.Power() != 4 || b.Power() != 3 {
		t.Errorf("expected both pumped +2/+0; got a.Power=%d b.Power=%d", a.Power(), b.Power())
	}
	if a.Flags["kw:first_strike"] != 0 && b.Flags["kw:first_strike"] != 0 {
		t.Errorf("non-promised must NOT grant first strike")
	}
}

func TestValleyRally_PromisedGrantsFirstStrikeToBestFriendly(t *testing.T) {
	gs := newGame(t, 2)
	weak := addPerm(gs, 0, "Llanowar Elves", "creature")
	weak.Card.BasePower, weak.Card.BaseToughness = 1, 1
	big := addPerm(gs, 0, "Craterhoof", "creature")
	big.Card.BasePower, big.Card.BaseToughness = 5, 5

	gameengine.InvokeResolveHook(gs, giftItem(0, "Valley Rally", "instant", "food", true, 1))

	if weak.Power() != 3 || big.Power() != 7 {
		t.Errorf("expected both pumped; got weak=%d big=%d", weak.Power(), big.Power())
	}
	if big.Flags["kw:first_strike"] != 1 {
		t.Errorf("strongest friendly should have first strike when promised")
	}
	if weak.Flags["kw:first_strike"] != 0 {
		t.Errorf("only the targeted (strongest) friendly should have first strike")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestGiftConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Blooming Blast",
		"Crumb and Get It",
		"Wear Down",
		"Nocturnal Hunger",
		"Wildfire Howl",
		"Valley Rally",
	}
	for _, name := range cards {
		if !HasResolve(name) {
			t.Errorf("expected OnResolve handler for %s", name)
		}
	}
}
