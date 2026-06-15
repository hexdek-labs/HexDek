package gameengine

import "testing"

// CR §702.21d / §118.4 — Ward—Pay N life (Charging War Boar, Sheltered by
// Ghosts). Regression for the r63 ward probe: payWardByLife decremented
// caster.Life directly, bypassing LoseLife — so the payment fired no
// "whenever you lose life" triggers, set none of the per-turn life-loss
// bookkeeping, and skipped the §104 post-game-end guard. The sibling ward
// variants already route through canonical helpers (SacrificePermanent /
// DiscardCard / AddCounter).

func TestWardPayLife_RoutesThroughLoseLife(t *testing.T) {
	gs := newWardAltGame(t)
	// Ward—Pay 3 life (WardCostLife amount 3).
	boar := wardedPerm(gs, "Charging War Boar", int(WardCostLife), 3, "creature")

	gs.Seats[1].Life = 20
	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should have been paid (caster has plenty of life)")
	}
	if gs.Seats[1].Life != 17 {
		t.Errorf("caster should have paid 3 life (20 -> 17), got %d", gs.Seats[1].Life)
	}
	// The decisive check: LoseLife sets the per-turn life-loss bookkeeping
	// and fires life_lost/life_change triggers; a raw `caster.Life -= n`
	// does neither. Pre-fix this flag stayed 0.
	if gs.Seats[1].Flags["lost_life_this_turn"] != 3 {
		t.Errorf("ward-pay-life must route through LoseLife (lost_life_this_turn=3); got %d",
			gs.Seats[1].Flags["lost_life_this_turn"])
	}
	if gs.Seats[1].Turn.LifeLost != 3 {
		t.Errorf("Turn.LifeLost should be 3 after a ward-life payment, got %d", gs.Seats[1].Turn.LifeLost)
	}
}

// A "whenever you lose life" observer must see the ward-life payment — the
// life_change trigger fires (it previously did not).
func TestWardPayLife_FiresLifeChangeTrigger(t *testing.T) {
	gs := newWardAltGame(t)
	boar := wardedPerm(gs, "Charging War Boar", int(WardCostLife), 2, "creature")
	gs.Seats[1].Life = 20
	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should have been paid")
	}
	// LoseLife fires FireCardTrigger("life_change", …); pre-fix the raw
	// decrement bypassed it. Assert the per-turn flag both triggers set.
	if gs.Seats[1].Flags["life_lost_this_turn"] != 2 {
		t.Errorf("life_lost_this_turn should be 2 (LoseLife path), got %d",
			gs.Seats[1].Flags["life_lost_this_turn"])
	}
}

// When the caster can't afford the life cost, the spell is countered (the
// AI declines / can't pay) — CR §702.21c.
func TestWardPayLife_CountersWhenInsufficientLife(t *testing.T) {
	gs := newWardAltGame(t)
	boar := wardedPerm(gs, "Charging War Boar", int(WardCostLife), 3, "creature")
	gs.Seats[1].Life = 2 // can't pay 3
	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Error("ward-pay-life must counter the spell when the caster can't pay the life")
	}
	if gs.Seats[1].Life != 2 {
		t.Errorf("no life should be lost when the payment is declined, got %d", gs.Seats[1].Life)
	}
}

// (e) The warded permanent's OWN controller targeting it does not trigger
// ward (no life paid, not countered).
func TestWardPayLife_OwnSpellDoesNotTrigger(t *testing.T) {
	gs := newWardAltGame(t)
	boar := wardedPerm(gs, "Charging War Boar", int(WardCostLife), 3, "creature")
	gs.Seats[0].Life = 20
	// Seat 0 (the boar's controller) targets its own creature.
	item := &StackItem{
		Kind: "spell", Controller: 0,
		Card:    &Card{Name: "Giant Growth", Owner: 0, Types: []string{"instant"}},
		Targets: []Target{{Kind: TargetKindPermanent, Permanent: boar}},
	}
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Error("ward must not trigger off the controller's own spell (§702.21b 'an opponent')")
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("no ward life paid for own spell, got %d", gs.Seats[0].Life)
	}
}
