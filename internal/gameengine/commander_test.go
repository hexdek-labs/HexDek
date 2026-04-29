package gameengine

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Phase 9 — Commander format tests (CR §903).
//
// Covers:
//   - §903.6 / §903.7 setup: life 40, commanders in command zone.
//   - §903.8 tax: 0, 2, 4 scaling.
//   - §903.9b replacement: hand/library returns redirect.
//   - §903.9a SBA: graveyard/exile returns (delegates to existing
//     sba704_6d tests for logic; here we just integrate-verify).
//   - §704.6c SBA: 21+ commander damage loss, per-commander independent.
//   - §108.3: ownership survives control swap (Gilded Drake).
//   - Tax persistence across command-zone cycles.
// -----------------------------------------------------------------------------

// newCommanderGame returns a 4-seat game set up with single-commander
// decks. Each commander has Owner set and is in the command zone.
// Seat libraries are empty (tests layer their own).
func newCommanderGame(t *testing.T, n int, commanderNames ...string) *GameState {
	t.Helper()
	gs := newMultiplayerGame(t, n)
	if len(commanderNames) != n {
		t.Fatalf("need %d commander names, got %d", n, len(commanderNames))
	}
	decks := make([]*CommanderDeck, n)
	for i, name := range commanderNames {
		cmdr := &Card{
			Name:          name,
			Owner:         i,
			BasePower:     4,
			BaseToughness: 4,
			Types:         []string{"creature", "legendary"},
		}
		decks[i] = &CommanderDeck{CommanderCards: []*Card{cmdr}}
	}
	SetupCommanderGame(gs, decks)
	return gs
}

// -----------------------------------------------------------------------------
// §903.6 / §903.7 setup
// -----------------------------------------------------------------------------

func TestSetupCommanderGame_StartingLife40(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	for i := 0; i < 4; i++ {
		if gs.Seats[i].Life != 40 {
			t.Fatalf("seat %d life should be 40 (CR §903.7), got %d", i, gs.Seats[i].Life)
		}
		if gs.Seats[i].StartingLife != 40 {
			t.Fatalf("seat %d StartingLife should be 40, got %d", i, gs.Seats[i].StartingLife)
		}
	}
	if !gs.CommanderFormat {
		t.Fatal("CommanderFormat should be true after setup")
	}
}

func TestSetupCommanderGame_CommandersInCommandZone(t *testing.T) {
	gs := newCommanderGame(t, 4, "Atraxa", "Edgar", "Kenrith", "Ur-Dragon")
	for i, name := range []string{"Atraxa", "Edgar", "Kenrith", "Ur-Dragon"} {
		if len(gs.Seats[i].CommandZone) != 1 {
			t.Fatalf("seat %d should have 1 commander in command zone, got %d", i, len(gs.Seats[i].CommandZone))
		}
		if gs.Seats[i].CommandZone[0].DisplayName() != name {
			t.Fatalf("seat %d commander name mismatch: want %s got %s", i, name, gs.Seats[i].CommandZone[0].DisplayName())
		}
		if len(gs.Seats[i].CommanderNames) != 1 || gs.Seats[i].CommanderNames[0] != name {
			t.Fatalf("seat %d CommanderNames wrong: %v", i, gs.Seats[i].CommanderNames)
		}
	}
}

func TestNormalGameStartingLifeIs20(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	for i := 0; i < 4; i++ {
		if gs.Seats[i].Life != 20 {
			t.Fatalf("non-commander life should be 20, got %d", gs.Seats[i].Life)
		}
	}
}

// -----------------------------------------------------------------------------
// §903.8 tax scaling
// -----------------------------------------------------------------------------

func TestCommanderCastCost_NoTax(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	cost := CommanderCastCost(gs.Seats[0], "A", 5)
	if cost != 5 {
		t.Fatalf("first cast should be base CMC 5 (no tax), got %d", cost)
	}
}

func TestCommanderCastCost_TaxScales(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	gs.Seats[0].CommanderTax["A"] = 2
	cost := CommanderCastCost(gs.Seats[0], "A", 5)
	// 5 + 2*2 = 9
	if cost != 9 {
		t.Fatalf("cost with tax=2 should be 9, got %d", cost)
	}
}

func TestCastCommanderFromCommandZone_FirstCastZeroTax(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	gs.Seats[0].ManaPool = 10
	err := CastCommanderFromCommandZone(gs, 0, "A", 5)
	if err != nil {
		t.Fatalf("first cast should succeed: %v", err)
	}
	// Tax should now be 1 (was 0 at cast time).
	if gs.Seats[0].CommanderTax["A"] != 1 {
		t.Fatalf("tax after first cast should be 1, got %d", gs.Seats[0].CommanderTax["A"])
	}
	// Mana deducted = base 5 + 2*0 = 5.
	if gs.Seats[0].ManaPool != 5 {
		t.Fatalf("mana pool should be 5 after paying 5, got %d", gs.Seats[0].ManaPool)
	}
	// Card should be on stack.
	if len(gs.Stack) != 1 {
		t.Fatalf("stack should have 1 item, got %d", len(gs.Stack))
	}
	// Card should be out of command zone.
	if len(gs.Seats[0].CommandZone) != 0 {
		t.Fatal("commander should have left command zone")
	}
}

func TestCastCommanderFromCommandZone_SecondCastCostsPlusTwo(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	gs.Seats[0].CommanderTax["A"] = 1 // as if already cast once
	// Put the commander back so we can cast again.
	gs.Seats[0].ManaPool = 10
	err := CastCommanderFromCommandZone(gs, 0, "A", 5)
	if err != nil {
		t.Fatalf("second cast should succeed: %v", err)
	}
	// Paid = 5 + 2*1 = 7.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("mana pool should be 3 (paid 7), got %d", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].CommanderTax["A"] != 2 {
		t.Fatalf("tax after second cast should be 2, got %d", gs.Seats[0].CommanderTax["A"])
	}
}

func TestCastCommanderFromCommandZone_InsufficientMana(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	gs.Seats[0].ManaPool = 3
	err := CastCommanderFromCommandZone(gs, 0, "A", 5)
	if err == nil {
		t.Fatal("cast should fail with insufficient mana")
	}
	if gs.Seats[0].CommanderTax["A"] != 0 {
		t.Fatal("tax should not increment on failed cast")
	}
	if len(gs.Seats[0].CommandZone) != 1 {
		t.Fatal("commander should still be in command zone")
	}
}

// Hand-cast of a commander (e.g. via flicker → hand then normal cast)
// must NOT increment tax. Only command-zone casts pay tax per §903.8.
func TestHandCastCommander_NoTaxIncrement(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Simulate: commander returned to hand via some weird interaction.
	handCard := &Card{Name: "A", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, handCard)
	// A hand cast does NOT invoke CastCommanderFromCommandZone.
	// The tax stays put.
	if gs.Seats[0].CommanderTax["A"] != 0 {
		t.Fatal("tax should remain 0 before any cast")
	}
	// Normal cast path wouldn't touch CommanderTax.
	// Verify CastCommanderFromCommandZone fails for a card not in CZ.
	err := CastCommanderFromCommandZone(gs, 0, "NonExistent", 0)
	if err == nil {
		t.Fatal("cast of non-CZ card should fail")
	}
	if gs.Seats[0].CommanderTax["A"] != 0 {
		t.Fatal("tax should not change after failed cast")
	}
}

// -----------------------------------------------------------------------------
// §903.9b replacement: hand/library return → command zone
// -----------------------------------------------------------------------------

func TestCommanderReturnToHand_RedirectsToCommandZone(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Pretend commander was on battlefield — craft a Permanent.
	cmdr := gs.Seats[0].CommandZone[0]
	gs.Seats[0].CommandZone = gs.Seats[0].CommandZone[:0]
	perm := &Permanent{Card: cmdr, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	// Now route a "would go to hand" zone change through FireZoneChange.
	gs.removePermanent(perm)
	dest := FireZoneChange(gs, perm, cmdr, 0, "battlefield", "hand")
	if dest != "command_zone" {
		t.Fatalf("expected redirect to command_zone, got %s", dest)
	}
	if len(gs.Seats[0].CommandZone) != 1 {
		t.Fatal("commander should be back in command zone")
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Fatal("commander should NOT be in hand")
	}
}

func TestCommanderReturnToLibrary_RedirectsToCommandZone(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	cmdr := gs.Seats[0].CommandZone[0]
	gs.Seats[0].CommandZone = gs.Seats[0].CommandZone[:0]
	dest := FireZoneChange(gs, nil, cmdr, 0, "battlefield", "library_top")
	if dest != "command_zone" {
		t.Fatalf("expected redirect to command_zone, got %s", dest)
	}
	if len(gs.Seats[0].CommandZone) != 1 {
		t.Fatal("commander should be in command zone")
	}
}

func TestNonCommanderCard_NoRedirect(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	rando := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	dest := FireZoneChange(gs, nil, rando, 0, "battlefield", "hand")
	if dest != "hand" {
		t.Fatalf("non-commander should go to hand, got %s", dest)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatal("Grizzly Bears should be in hand")
	}
}

// §903.9a — commander in graveyard/exile returns to command zone as SBA.
func TestCommanderFromGraveyard_SBAReturns(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Put commander in graveyard.
	cmdr := gs.Seats[0].CommandZone[0]
	gs.Seats[0].CommandZone = gs.Seats[0].CommandZone[:0]
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cmdr)
	StateBasedActions(gs)
	if len(gs.Seats[0].CommandZone) != 1 {
		t.Fatal("commander should have returned to command zone via §704.6d SBA")
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Fatal("graveyard should be empty")
	}
}

func TestCommanderFromExile_SBAReturns(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	cmdr := gs.Seats[1].CommandZone[0]
	gs.Seats[1].CommandZone = gs.Seats[1].CommandZone[:0]
	gs.Seats[1].Exile = append(gs.Seats[1].Exile, cmdr)
	StateBasedActions(gs)
	if len(gs.Seats[1].CommandZone) != 1 {
		t.Fatal("commander should have returned from exile")
	}
}

// -----------------------------------------------------------------------------
// §704.6c / §903.10a — commander damage
// -----------------------------------------------------------------------------

func TestCommanderDamage_AccumulateTo21_Loses(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Seat 1 commander "B" does 10 damage to seat 0, then 11 more = 21.
	AccumulateCommanderDamage(gs, 0, 1, "B", 10)
	StateBasedActions(gs)
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 should not lose at 10 commander damage")
	}
	AccumulateCommanderDamage(gs, 0, 1, "B", 11)
	StateBasedActions(gs)
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should lose at 21 commander damage (CR §704.6c)")
	}
}

// Two different commanders each dealing 20 damage: neither triggers loss.
func TestCommanderDamage_DifferentCommandersTrackedSeparately(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Seat 0 takes 20 from seat 1's "B" and 20 from seat 2's "C".
	AccumulateCommanderDamage(gs, 0, 1, "B", 20)
	AccumulateCommanderDamage(gs, 0, 2, "C", 20)
	StateBasedActions(gs)
	if gs.Seats[0].Lost {
		t.Fatal("seat 0 should survive two separate 20-damage commanders")
	}
}

func TestCommanderDamage_TrackedAcrossOpps(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Seat 1's commander "B" hits seat 0 for 15, seat 2 for 15.
	AccumulateCommanderDamage(gs, 0, 1, "B", 15)
	AccumulateCommanderDamage(gs, 2, 1, "B", 15)
	StateBasedActions(gs)
	if gs.Seats[0].Lost || gs.Seats[2].Lost {
		t.Fatal("neither seat should lose at 15 damage")
	}
	// Now seat 0 takes 6 more from B → 21 → loss. Seat 2 still on 15.
	AccumulateCommanderDamage(gs, 0, 1, "B", 6)
	StateBasedActions(gs)
	if !gs.Seats[0].Lost {
		t.Fatal("seat 0 should lose at 21 damage from B")
	}
	if gs.Seats[2].Lost {
		t.Fatal("seat 2 at 15 damage from B should survive")
	}
}

// -----------------------------------------------------------------------------
// Gilded Drake ownership (§108.3)
// -----------------------------------------------------------------------------

// Gilded Drake swaps CONTROL; OWNERSHIP stays. When the commander eventually
// changes zone (e.g. dies → 903.9b fires), it redirects to the original
// OWNER's command zone, not the current controller's.
func TestGildedDrake_OwnershipSurvivesControlSwap(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	// Seat 0's commander "A" lands on battlefield, then seat 1 steals it.
	cmdr := gs.Seats[0].CommandZone[0]
	gs.Seats[0].CommandZone = gs.Seats[0].CommandZone[:0]
	perm := &Permanent{Card: cmdr, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	// Gilded Drake control swap: Controller moves to seat 1, Owner stays.
	gs.removePermanent(perm)
	perm.Controller = 1
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, perm)
	// Verify the invariant.
	if perm.Owner != 0 {
		t.Fatalf("ownership should stay with seat 0, got owner=%d", perm.Owner)
	}
	// Now commander "returns to hand" — should redirect to OWNER's command zone.
	gs.removePermanent(perm)
	dest := FireZoneChange(gs, perm, cmdr, perm.Owner, "battlefield", "hand")
	if dest != "command_zone" {
		t.Fatalf("expected command_zone redirect, got %s", dest)
	}
	// Must be in seat 0's command zone (owner), not seat 1's (controller).
	if len(gs.Seats[0].CommandZone) != 1 {
		t.Fatal("owner (seat 0) should get commander back in command zone")
	}
	if gs.Seats[0].CommandZone[0] != cmdr {
		t.Fatal("owner's command zone should contain the redirected commander")
	}
	// Seat 1 should still only have its own commander "B" in its command
	// zone — no "A" card. Verify by name.
	for _, c := range gs.Seats[1].CommandZone {
		if c == cmdr {
			t.Fatal("controller (seat 1) should NOT receive the stolen commander")
		}
	}
}

// -----------------------------------------------------------------------------
// Tax persistence across command-zone → battlefield → command-zone cycles
// -----------------------------------------------------------------------------

func TestCommanderTax_PersistsAcrossZoneCycles(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	gs.Seats[0].ManaPool = 100
	// First cast: pay 5, tax → 1.
	if err := CastCommanderFromCommandZone(gs, 0, "A", 5); err != nil {
		t.Fatalf("first cast failed: %v", err)
	}
	if gs.Seats[0].CommanderTax["A"] != 1 {
		t.Fatalf("tax after 1st: want 1, got %d", gs.Seats[0].CommanderTax["A"])
	}
	// Simulate commander going to battlefield then back to command zone.
	// The card went to stack via PushStackItem; pop it and put back in CZ.
	cmdr := gs.Stack[len(gs.Stack)-1].Card
	gs.Stack = gs.Stack[:len(gs.Stack)-1]
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, cmdr)
	// Second cast: should cost 5 + 2*1 = 7; tax → 2.
	if err := CastCommanderFromCommandZone(gs, 0, "A", 5); err != nil {
		t.Fatalf("second cast failed: %v", err)
	}
	if gs.Seats[0].CommanderTax["A"] != 2 {
		t.Fatalf("tax after 2nd: want 2, got %d", gs.Seats[0].CommanderTax["A"])
	}
	// Mana: 100 - 5 - 7 = 88.
	if gs.Seats[0].ManaPool != 88 {
		t.Fatalf("mana after two casts: want 88, got %d", gs.Seats[0].ManaPool)
	}
	// Third cast: 5 + 2*2 = 9.
	cmdr = gs.Stack[len(gs.Stack)-1].Card
	gs.Stack = gs.Stack[:len(gs.Stack)-1]
	gs.Seats[0].CommandZone = append(gs.Seats[0].CommandZone, cmdr)
	if err := CastCommanderFromCommandZone(gs, 0, "A", 5); err != nil {
		t.Fatalf("third cast failed: %v", err)
	}
	if gs.Seats[0].CommanderTax["A"] != 3 {
		t.Fatalf("tax after 3rd: want 3, got %d", gs.Seats[0].CommanderTax["A"])
	}
	// Mana: 88 - 9 = 79.
	if gs.Seats[0].ManaPool != 79 {
		t.Fatalf("mana after three casts: want 79, got %d", gs.Seats[0].ManaPool)
	}
}

// -----------------------------------------------------------------------------
// IsCommanderCard
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Benchmarks — Phase 9 perf baseline
// -----------------------------------------------------------------------------

// BenchmarkSetupCommanderGame measures the full 4-seat setup (§903.6 +
// §903.7 + 4× §903.9b replacement registrations). Target: <100µs so the
// gauntlet's 1000-game 4p EDH suite starts in <0.1s of setup overhead.
func BenchmarkSetupCommanderGame4Seats(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gs := NewGameState(4, nil, nil)
		decks := make([]*CommanderDeck, 4)
		for j := 0; j < 4; j++ {
			cmdr := &Card{
				Name:          "Commander" + string(rune('A'+j)),
				Owner:         j,
				BasePower:     4,
				BaseToughness: 4,
				Types:         []string{"creature", "legendary"},
			}
			decks[j] = &CommanderDeck{CommanderCards: []*Card{cmdr}}
		}
		SetupCommanderGame(gs, decks)
	}
}

// BenchmarkFourSeat100TurnCombat drives 100 CombatPhase calls on a
// 4-seat game with a single-creature attacker per seat. Measures the
// steady-state cost of target-selection + damage assignment across
// opponents, which dominates gauntlet wallclock.
func BenchmarkFourSeat100TurnCombat(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		gs := NewGameState(4, nil, nil)
		// Seed each seat with a 2/2 haste creature.
		for s := 0; s < 4; s++ {
			c := &Card{
				Name:          "Goblin",
				Owner:         s,
				BasePower:     2,
				BaseToughness: 2,
				Types:         []string{"creature"},
			}
			p := &Permanent{
				Card: c, Controller: s, Owner: s,
				Timestamp: gs.NextTimestamp(),
				Counters:  map[string]int{}, Flags: map[string]int{},
			}
			gs.Seats[s].Battlefield = append(gs.Seats[s].Battlefield, p)
		}
		for turn := 0; turn < 100; turn++ {
			gs.Active = turn % 4
			// Untap pre-combat attackers so they can attack next turn.
			for _, p := range gs.Seats[gs.Active].Battlefield {
				p.Tapped = false
				p.SummoningSick = false
			}
			CombatPhase(gs)
			if gs.CheckEnd() {
				break
			}
		}
	}
}

func TestIsCommanderCard(t *testing.T) {
	gs := newCommanderGame(t, 4, "A", "B", "C", "D")
	cmdr := gs.Seats[1].CommandZone[0]
	if !IsCommanderCard(gs, 1, cmdr) {
		t.Fatal("seat 1's commander should report true")
	}
	if IsCommanderCard(gs, 0, cmdr) {
		t.Fatal("seat 0 should not recognize seat 1's commander as theirs")
	}
	rando := &Card{Name: "Forest", Owner: 0}
	if IsCommanderCard(gs, 0, rando) {
		t.Fatal("non-commander should report false")
	}
}
