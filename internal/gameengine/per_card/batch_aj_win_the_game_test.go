package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// batch_aj_win_the_game_test.go — pins the r60 batch AJ handlers
// (Approach of the Second Sun / Felidar Sovereign / Test of Endurance /
// Mayael's Aria / Triskaidekaphobia).
//
// Each handler is exercised in isolation:
//   - happy path: the win/loss condition fires and Seat.Won / Seat.Lost
//     is set
//   - boundary: condition misses by one
//   - control gating: trigger only on perm.Controller's upkeep
//
// The §704.5c master-check itself (CheckEnd's §104.2c aggregator) is
// covered by the engine-side seat_loss_conditions_r60_test.go from #687;
// here we pin that the per_card handler correctly stamps Seat.Won so
// CheckEnd has something to aggregate.

// =============================================================================
// Felidar Sovereign
// =============================================================================

func TestFelidarSovereign_Upkeep_LifeAtForty_Wins(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 40
	felidar := addPerm(gs, 0, "Felidar Sovereign", "creature", "cat", "beast")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        felidar,
	})

	if !gs.Seats[0].Won {
		t.Errorf("Seat 0 must be marked Won when life=40 at controller upkeep")
	}
}

func TestFelidarSovereign_Upkeep_LifeAt39_DoesNotWin(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 39
	felidar := addPerm(gs, 0, "Felidar Sovereign", "creature", "cat", "beast")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        felidar,
	})

	if gs.Seats[0].Won {
		t.Error("Seat 0 must NOT be marked Won at life 39 (one short of 40)")
	}
}

func TestFelidarSovereign_OpponentUpkeep_DoesNotTrigger(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 100
	felidar := addPerm(gs, 0, "Felidar Sovereign", "creature", "cat", "beast")

	// Opponent's upkeep — trigger must be gated off.
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 1,
		"perm":        felidar,
	})

	if gs.Seats[0].Won {
		t.Error("Felidar's win check must NOT fire on opponent's upkeep")
	}
}

// =============================================================================
// Test of Endurance
// =============================================================================

func TestTestOfEndurance_Upkeep_LifeAtFifty_Wins(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 50
	test := addPerm(gs, 0, "Test of Endurance", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        test,
	})

	if !gs.Seats[0].Won {
		t.Errorf("Seat 0 must be marked Won when life=50 at upkeep")
	}
}

func TestTestOfEndurance_Upkeep_LifeAt49_DoesNotWin(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 49
	test := addPerm(gs, 0, "Test of Endurance", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        test,
	})

	if gs.Seats[0].Won {
		t.Error("Seat 0 must NOT be marked Won at life 49")
	}
}

// =============================================================================
// Mayael's Aria
// =============================================================================

func TestMayaelsAria_Upkeep_PowerTen_Wins(t *testing.T) {
	gs := newGame(t, 2)
	aria := addPerm(gs, 0, "Mayael's Aria", "enchantment")
	bigBeast := addPerm(gs, 0, "Marath, Will of the Wild", "creature", "elemental", "beast")
	bigBeast.Card.BasePower = 10
	bigBeast.Card.BaseToughness = 10

	startLife := gs.Seats[0].Life
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        aria,
	})

	if !gs.Seats[0].Won {
		t.Errorf("Seat 0 must Won — chosen creature post-counter power ≥ 10")
	}
	if gs.Seats[0].Life != startLife+4 {
		t.Errorf("life = %d, want %d (gained 4)", gs.Seats[0].Life, startLife+4)
	}
	if bigBeast.Counters["+1/+1"] != 1 {
		t.Errorf("counter on chosen creature = %d, want 1", bigBeast.Counters["+1/+1"])
	}
}

func TestMayaelsAria_Upkeep_PowerNine_NoWin_StillAppliesCounterAndLife(t *testing.T) {
	gs := newGame(t, 2)
	aria := addPerm(gs, 0, "Mayael's Aria", "enchantment")
	mid := addPerm(gs, 0, "Steel Leaf Champion", "creature", "elf")
	mid.Card.BasePower = 8
	mid.Card.BaseToughness = 8
	// 8 + 1 counter = 9; below the 10 threshold → no win, but +1/+1
	// and the 4 life still apply.

	startLife := gs.Seats[0].Life
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        aria,
	})

	if gs.Seats[0].Won {
		t.Error("Seat 0 must NOT win — chosen creature post-counter power 9 < 10")
	}
	if gs.Seats[0].Life != startLife+4 {
		t.Errorf("life gain must still apply, got %d", gs.Seats[0].Life)
	}
	if mid.Counters["+1/+1"] != 1 {
		t.Errorf("counter must still apply, got %d", mid.Counters["+1/+1"])
	}
}

func TestMayaelsAria_Upkeep_NoCreatures_NoFire(t *testing.T) {
	gs := newGame(t, 2)
	aria := addPerm(gs, 0, "Mayael's Aria", "enchantment")
	startLife := gs.Seats[0].Life

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        aria,
	})

	if gs.Seats[0].Won {
		t.Error("no creature to pick — must not win")
	}
	if gs.Seats[0].Life != startLife {
		t.Errorf("no creature picked — life must be unchanged, got %d", gs.Seats[0].Life)
	}
}

// =============================================================================
// Approach of the Second Sun
// =============================================================================

func TestApproachOfTheSecondSun_FirstCast_LibraryPlaceLifeAndCount(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest")
	startLife := gs.Seats[0].Life
	startLibSize := len(gs.Seats[0].Library)

	approach := &gameengine.Card{
		Name:  "Approach of the Second Sun",
		Owner: 0,
		Types: []string{"sorcery"},
	}
	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       approach,
		CastZone:   "hand",
	}
	approachOfTheSecondSunResolve(gs, item)

	if gs.Seats[0].Won {
		t.Error("first cast must NOT win")
	}
	if gs.Seats[0].Life != startLife+7 {
		t.Errorf("life = %d, want %d (gained 7)", gs.Seats[0].Life, startLife+7)
	}
	if len(gs.Seats[0].Library) != startLibSize+1 {
		t.Errorf("library size = %d, want %d (+1 from library insert)", len(gs.Seats[0].Library), startLibSize+1)
	}
	if gs.Flags["approach_cast_count:0"] != 1 {
		t.Errorf("cast count = %d, want 1", gs.Flags["approach_cast_count:0"])
	}
	// Inserted at index 6 — the 7th-from-top slot.
	if gs.Seats[0].Library[6].Name != "Approach of the Second Sun" {
		t.Errorf("library[6] = %q, want Approach", gs.Seats[0].Library[6].Name)
	}
}

func TestApproachOfTheSecondSun_SecondCastFromHand_Wins(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Forest", "Forest")
	gs.Flags = map[string]int{"approach_cast_count:0": 1} // already cast once

	approach := &gameengine.Card{
		Name:  "Approach of the Second Sun",
		Owner: 0,
		Types: []string{"sorcery"},
	}
	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       approach,
		CastZone:   "hand",
	}
	approachOfTheSecondSunResolve(gs, item)

	if !gs.Seats[0].Won {
		t.Error("second cast from hand must Win")
	}
	if gs.Flags["approach_cast_count:0"] != 2 {
		t.Errorf("cast count must increment even on the winning cast, got %d", gs.Flags["approach_cast_count:0"])
	}
}

func TestApproachOfTheSecondSun_SecondCastFromGraveyard_DoesNotWin(t *testing.T) {
	// Per oracle: "If this spell was CAST FROM YOUR HAND..." — flashback
	// or other-zone casts must NOT trigger the win even when the count
	// is sufficient.
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Forest", "Forest")
	gs.Flags = map[string]int{"approach_cast_count:0": 1}

	approach := &gameengine.Card{
		Name:  "Approach of the Second Sun",
		Owner: 0,
		Types: []string{"sorcery"},
	}
	item := &gameengine.StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       approach,
		CastZone:   "graveyard",
	}
	approachOfTheSecondSunResolve(gs, item)

	if gs.Seats[0].Won {
		t.Error("non-hand cast must NOT win even with prior cast count ≥ 1")
	}
}

// =============================================================================
// Triskaidekaphobia
// =============================================================================

func TestTriskaidekaphobia_Upkeep_OppositeMode_PicksLose1WhenOppAt14(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 14
	tris := addPerm(gs, 0, "Triskaidekaphobia", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        tris,
	})

	// Mode picked = 1 (lose 1) since opp at 14 → after lose, opp at 13 → loses.
	if !gs.Seats[1].Lost {
		t.Errorf("seat 1 must be marked Lost — life went from 14 to 13 after lose-1-life mode")
	}
	if gs.Seats[1].LossReason != "card_effect: Triskaidekaphobia — exactly 13 life" {
		t.Errorf("LossReason = %q, want canonical card_effect-prefixed Triskaidekaphobia reason (via MarkSeatLostByEffect)", gs.Seats[1].LossReason)
	}
	if !gs.Seats[1].LostByEffect {
		t.Error("LostByEffect flag not set — Triskaidekaphobia should route through MarkSeatLostByEffect")
	}
}

// TestTriskaidekaphobia_PlatinumAngelCancelsLoss pins the structural-wave1
// PR #5 fix: routing through MarkSeatLostByEffect means a seat under
// Platinum Angel does NOT lose to the §704 / Triskaidekaphobia state check
// at exactly 13 life. Pre-fix the direct seat.Lost = true write bypassed
// §614 and Platinum Angel was inert here.
func TestTriskaidekaphobia_PlatinumAngelCancelsLoss(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 14
	tris := addPerm(gs, 0, "Triskaidekaphobia", "enchantment")
	// Drop a Platinum Angel on seat 1 + register its §614 cancel handler.
	pa := addPerm(gs, 1, "Platinum Angel", "creature")
	gameengine.RegisterPlatinumAngel(gs, pa)

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        tris,
	})

	if gs.Seats[1].Lost {
		t.Error("seat 1 must NOT be Lost — Platinum Angel cancels the would_lose_game via §614")
	}
	if gs.Seats[1].LossReason != "" {
		t.Errorf("LossReason must stay empty when §614 cancels, got %q", gs.Seats[1].LossReason)
	}
}

func TestTriskaidekaphobia_Upkeep_OppAt12_PicksGain1ModeToPushToThirteen(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 12
	tris := addPerm(gs, 0, "Triskaidekaphobia", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        tris,
	})

	if !gs.Seats[1].Lost {
		t.Errorf("seat 1 must be Lost after gain-1 pushes from 12 → 13")
	}
}

func TestTriskaidekaphobia_Upkeep_NoOppAtBoundary_NoLoss(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 18 // far from 13
	tris := addPerm(gs, 0, "Triskaidekaphobia", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        tris,
	})

	if gs.Seats[1].Lost {
		t.Error("no seat at 13 after life-shift — no loss should fire")
	}
}

func TestTriskaidekaphobia_Upkeep_ControllerAt14_LosesSelfWhenMode1Picks(t *testing.T) {
	// Mode picks lose-1 (because opp at 14 → 13 wins). Controller at 14
	// will ALSO go to 13 and lose. Symmetric.
	gs := newGame(t, 2)
	gs.Seats[0].Life = 14
	gs.Seats[1].Life = 14
	tris := addPerm(gs, 0, "Triskaidekaphobia", "enchantment")

	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": 0,
		"perm":        tris,
	})

	if !gs.Seats[0].Lost {
		t.Error("controller at 14 + lose-1 mode → controller also at 13 → must lose (symmetric)")
	}
	if !gs.Seats[1].Lost {
		t.Error("opponent at 14 + lose-1 mode → opp also at 13 → must lose")
	}
}
