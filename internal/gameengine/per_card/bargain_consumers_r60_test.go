package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// bargain_consumers_r60 — 6 handlers wired in bargain_consumers_r60.go
// covering the WOE Bargain keyword family (CR §702.166).
// -----------------------------------------------------------------------------

func bargainedItem(seat int, c *gameengine.Card, bargained bool) *gameengine.StackItem {
	return &gameengine.StackItem{
		Controller: seat,
		Card:       c,
		CostMeta:   map[string]interface{}{"bargained": bargained},
	}
}

// -----------------------------------------------------------------------------
// Torch the Tower
// -----------------------------------------------------------------------------

func TestTorchTheTower_NonBargainedDeals2(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 5

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Torch the Tower", "instant"), false))

	if target.MarkedDamage != 2 {
		t.Errorf("expected 2 damage marked, got %d", target.MarkedDamage)
	}
}

func TestTorchTheTower_BargainedDeals3AndScries(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 5
	addLibrary(gs, 0, "Top")

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Torch the Tower", "instant"), true))

	if target.MarkedDamage != 3 {
		t.Errorf("expected 3 damage marked when bargained, got %d", target.MarkedDamage)
	}
	if hasEvent(gs, "scry") < 1 {
		t.Errorf("expected scry event when bargained; events=%+v", gs.EventLog)
	}
}

func TestTorchTheTower_NoTargetEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Torch the Tower", "instant"), false))

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no target; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Candy Grapple
// -----------------------------------------------------------------------------

func TestCandyGrapple_NonBargainedMinus3(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Hill Giant", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 3, 3

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Candy Grapple", "instant"), false))

	if target.Power() != 0 || target.Toughness() != 0 {
		t.Errorf("expected 0/0 after -3/-3, got %d/%d", target.Power(), target.Toughness())
	}
}

func TestCandyGrapple_BargainedMinus5(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Threat", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 4, 4

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Candy Grapple", "instant"), true))

	if target.Power() != -1 || target.Toughness() != -1 {
		t.Errorf("expected -1/-1 after -5/-5 on a 4/4, got %d/%d", target.Power(), target.Toughness())
	}
}

// -----------------------------------------------------------------------------
// Archon's Glory
// -----------------------------------------------------------------------------

func TestArchonsGlory_NonBargainedPumpsOnly(t *testing.T) {
	gs := newGame(t, 2)
	friend := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend.Card.BasePower, friend.Card.BaseToughness = 1, 1

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Archon's Glory", "instant"), false))

	if friend.Power() != 3 || friend.Toughness() != 3 {
		t.Errorf("expected 3/3 after +2/+2 pump, got %d/%d", friend.Power(), friend.Toughness())
	}
	if friend.Flags["kw:flying"] != 0 {
		t.Errorf("non-bargained must NOT grant flying; got %d", friend.Flags["kw:flying"])
	}
	if friend.Flags["kw:lifelink"] != 0 {
		t.Errorf("non-bargained must NOT grant lifelink; got %d", friend.Flags["kw:lifelink"])
	}
}

func TestArchonsGlory_BargainedAlsoGrantsFlyingLifelink(t *testing.T) {
	gs := newGame(t, 2)
	friend := addPerm(gs, 0, "Llanowar Elves", "creature")
	friend.Card.BasePower, friend.Card.BaseToughness = 1, 1

	gameengine.InvokeResolveHook(gs, bargainedItem(0, addCard(gs, 0, "Archon's Glory", "instant"), true))

	if friend.Power() != 3 || friend.Toughness() != 3 {
		t.Errorf("expected 3/3 after +2/+2, got %d/%d", friend.Power(), friend.Toughness())
	}
	if friend.Flags["kw:flying"] != 1 {
		t.Errorf("bargained must grant flying; got %d", friend.Flags["kw:flying"])
	}
	if friend.Flags["kw:lifelink"] != 1 {
		t.Errorf("bargained must grant lifelink; got %d", friend.Flags["kw:lifelink"])
	}
}

// -----------------------------------------------------------------------------
// Troublemaker Ouphe — cast→ETB bridge
// -----------------------------------------------------------------------------

func TestTroublemakerOuphe_BargainedETBExilesOppEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	oppEnch := addPerm(gs, 1, "Wedding Announcement", "enchantment")

	// Simulate the cast→ETB chain.
	ouphe := addPerm(gs, 0, "Troublemaker Ouphe", "creature")
	ouphe.Card.BasePower, ouphe.Card.BaseToughness = 2, 2
	gameengine.InvokeCastHook(gs, bargainedItem(0, ouphe.Card, true))
	gameengine.InvokeETBHook(gs, ouphe)

	for _, p := range gs.Seats[1].Battlefield {
		if p == oppEnch {
			t.Fatalf("Wedding Announcement should have been exiled by bargained Ouphe")
		}
	}
}

func TestTroublemakerOuphe_NonBargainedETBNoOps(t *testing.T) {
	gs := newGame(t, 2)
	oppEnch := addPerm(gs, 1, "Wedding Announcement", "enchantment")
	pre := len(gs.Seats[1].Battlefield)

	ouphe := addPerm(gs, 0, "Troublemaker Ouphe", "creature")
	ouphe.Card.BasePower, ouphe.Card.BaseToughness = 2, 2
	gameengine.InvokeCastHook(gs, bargainedItem(0, ouphe.Card, false))
	gameengine.InvokeETBHook(gs, ouphe)

	if len(gs.Seats[1].Battlefield) != pre {
		t.Errorf("non-bargained Ouphe must not exile anything; opp pre=%d post=%d",
			pre, len(gs.Seats[1].Battlefield))
	}
	// Sanity: Wedding Announcement still present.
	stillThere := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppEnch {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Errorf("opp enchantment should still be present")
	}
}

func TestTroublemakerOuphe_BargainCounterConsumedOncePerETB(t *testing.T) {
	gs := newGame(t, 2)
	// Two opp enchantments — exiling only one verifies the counter
	// drains to 0 after the first ETB and a SECOND ETB no-ops.
	addPerm(gs, 1, "Wedding Announcement", "enchantment")
	addPerm(gs, 1, "Banishing Light", "enchantment")
	prePerms := len(gs.Seats[1].Battlefield)

	ouphe1 := addPerm(gs, 0, "Troublemaker Ouphe", "creature")
	ouphe1.Card.BasePower, ouphe1.Card.BaseToughness = 2, 2
	gameengine.InvokeCastHook(gs, bargainedItem(0, ouphe1.Card, true))
	gameengine.InvokeETBHook(gs, ouphe1)

	if len(gs.Seats[1].Battlefield) != prePerms-1 {
		t.Errorf("first bargained ETB should exile one opp enchantment; post=%d", len(gs.Seats[1].Battlefield))
	}

	// Second Ouphe ETBs WITHOUT a paired bargained cast — must no-op.
	ouphe2 := addPerm(gs, 0, "Troublemaker Ouphe", "creature")
	ouphe2.Card.BasePower, ouphe2.Card.BaseToughness = 2, 2
	gameengine.InvokeETBHook(gs, ouphe2)

	if len(gs.Seats[1].Battlefield) != prePerms-1 {
		t.Errorf("second ETB without paired bargained cast must no-op; post=%d", len(gs.Seats[1].Battlefield))
	}
}

// -----------------------------------------------------------------------------
// High Fae Negotiator
// -----------------------------------------------------------------------------

func TestHighFaeNegotiator_BargainedDrains3PerOpp(t *testing.T) {
	gs := newGame(t, 4)
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 20
	}
	fae := addPerm(gs, 0, "High Fae Negotiator", "creature")
	fae.Card.BasePower, fae.Card.BaseToughness = 3, 3

	gameengine.InvokeCastHook(gs, bargainedItem(0, fae.Card, true))
	gameengine.InvokeETBHook(gs, fae)

	if gs.Seats[0].Life != 23 {
		t.Errorf("expected +3 life to controller, got %d", gs.Seats[0].Life)
	}
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 17 {
			t.Errorf("expected opp %d at 17, got %d", i, gs.Seats[i].Life)
		}
	}
}

func TestHighFaeNegotiator_NonBargainedDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	fae := addPerm(gs, 0, "High Fae Negotiator", "creature")
	fae.Card.BasePower, fae.Card.BaseToughness = 3, 3
	gameengine.InvokeCastHook(gs, bargainedItem(0, fae.Card, false))
	gameengine.InvokeETBHook(gs, fae)

	if gs.Seats[0].Life != 20 || gs.Seats[1].Life != 20 {
		t.Errorf("non-bargained must not drain; seat0=%d seat1=%d", gs.Seats[0].Life, gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Tenacious Tomeseeker
// -----------------------------------------------------------------------------

func TestTenaciousTomeseeker_BargainedReturnsBestInstantSorcery(t *testing.T) {
	gs := newGame(t, 2)
	// Seed graveyard with a creature card (should be skipped) and an
	// instant + sorcery (should compete on CMC).
	// pickHighestCMCInstantOrSorcery uses cardCMC, which reads the
	// "cmc:N" type-line marker (the test convention) rather than
	// Card.CMC. Use the marker so the picker sorts correctly.
	cheapBolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant", "cmc:1"}, CMC: 1}
	bigSorcery := &gameengine.Card{Name: "Decree of Pain", Owner: 0, Types: []string{"sorcery", "cmc:8"}, CMC: 8}
	noise := &gameengine.Card{Name: "Random Creature", Owner: 0, Types: []string{"creature"}}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cheapBolt, bigSorcery, noise)

	seeker := addPerm(gs, 0, "Tenacious Tomeseeker", "creature")
	seeker.Card.BasePower, seeker.Card.BaseToughness = 2, 4
	gameengine.InvokeCastHook(gs, bargainedItem(0, seeker.Card, true))
	gameengine.InvokeETBHook(gs, seeker)

	// Highest-CMC i/s (Decree at 8) returns to hand.
	foundDecree := false
	for _, c := range gs.Seats[0].Hand {
		if c.DisplayName() == "Decree of Pain" {
			foundDecree = true
			break
		}
	}
	if !foundDecree {
		t.Errorf("expected Decree of Pain in hand after bargained ETB; hand=%v", handNames(gs.Seats[0].Hand))
	}
	// Creature stays in graveyard.
	stillInGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == noise {
			stillInGY = true
			break
		}
	}
	if !stillInGY {
		t.Errorf("non-i/s should stay in graveyard")
	}
}

func TestTenaciousTomeseeker_NonBargainedNoOps(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		&gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1},
	)

	seeker := addPerm(gs, 0, "Tenacious Tomeseeker", "creature")
	seeker.Card.BasePower, seeker.Card.BaseToughness = 2, 4
	gameengine.InvokeCastHook(gs, bargainedItem(0, seeker.Card, false))
	gameengine.InvokeETBHook(gs, seeker)

	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-bargained must not return; hand=%d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("instant should stay in graveyard; gy=%d", len(gs.Seats[0].Graveyard))
	}
}

// -----------------------------------------------------------------------------
// Bridge primitive — flag-counter sanity
// -----------------------------------------------------------------------------

func TestBargainBridge_CounterStacksAndDrains(t *testing.T) {
	gs := newGame(t, 2)
	bridge := bargainCastBridge("test_card")

	// Two bargained casts.
	bridge(gs, bargainedItem(0, addCard(gs, 0, "Test Card", "creature"), true))
	bridge(gs, bargainedItem(0, addCard(gs, 0, "Test Card", "creature"), true))
	if gs.Seats[0].Flags["bargained_pending:test_card"] != 2 {
		t.Errorf("expected counter 2, got %d", gs.Seats[0].Flags["bargained_pending:test_card"])
	}

	// Consume twice → 0 (deleted).
	if !consumeBargainFlag(gs, 0, "test_card") {
		t.Errorf("expected first consume to succeed")
	}
	if !consumeBargainFlag(gs, 0, "test_card") {
		t.Errorf("expected second consume to succeed")
	}
	if consumeBargainFlag(gs, 0, "test_card") {
		t.Errorf("third consume should fail")
	}
	if _, exists := gs.Seats[0].Flags["bargained_pending:test_card"]; exists {
		t.Errorf("flag should be deleted after counter hits 0")
	}
}

func TestBargainBridge_NonBargainedCastNoOps(t *testing.T) {
	gs := newGame(t, 2)
	bridge := bargainCastBridge("test_card")
	bridge(gs, bargainedItem(0, addCard(gs, 0, "Test Card", "creature"), false))

	if gs.Seats[0].Flags["bargained_pending:test_card"] != 0 {
		t.Errorf("non-bargained cast must not increment counter; got %d",
			gs.Seats[0].Flags["bargained_pending:test_card"])
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable post-registerDefaults.
// -----------------------------------------------------------------------------

func TestBargainConsumers_AllRegistered(t *testing.T) {
	resolveCards := []string{"Torch the Tower", "Candy Grapple", "Archon's Glory"}
	etbCards := []string{"Troublemaker Ouphe", "High Fae Negotiator", "Tenacious Tomeseeker"}
	for _, name := range resolveCards {
		if !HasResolve(name) {
			t.Errorf("expected OnResolve handler for %s", name)
		}
	}
	for _, name := range etbCards {
		if !HasETB(name) {
			t.Errorf("expected OnETB handler for %s", name)
		}
	}
}
