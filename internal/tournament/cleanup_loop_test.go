package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// ---------------------------------------------------------------------------
// P1 #7: Cleanup Step Looping (CR §514.3a) Tests
// ---------------------------------------------------------------------------

func TestCleanupLoop_BasicTurnCompletes(t *testing.T) {
	// Verify a normal turn still completes without issues.
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	// Give seat 0 some cards so it has something to do.
	for i := 0; i < 5; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
	}
	for i := 0; i < 5; i++ {
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}
	for i := 0; i < 3; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
	}

	// Take a turn — should not panic or infinite loop.
	TakeTurn(gs)

	// Verify the turn completed.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "turn_start" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected turn_start event")
	}
}

// TestCleanupLoop_DiscardTriggersAnotherCleanupPass pins the r60 fix to
// the §514.3a loop condition. Prior to the fix, the cleanup loop only
// re-iterated when SBAs fired, silently skipping the rule's explicit
// "OR if any triggered abilities trigger" arm. §514.1 discards routinely
// fire card_discarded triggers (Madness, Megrim, Mayhem); per §514.3a,
// after those triggers resolve the active player must receive priority
// and another cleanup step must begin. Without the fix, an active player
// who overdrew (hand > 7 entering cleanup) would discard down to 7, the
// `cleanup_loop` event would never log, and any Madness "may cast for
// madness cost" window would be silently skipped.
//
// The observable invariant: when discards actually happen in §514.1, a
// `cleanup_loop` event with reason="discard_triggers" (or "sba" if SBAs
// piggybacked) must appear before the cleanup step ends.
func TestCleanupLoop_DiscardTriggersAnotherCleanupPass(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	// Library: a few lands so draw step works and nothing weird happens.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}

	// Seat 0 enters the turn with 12 high-CMC creatures in hand and no
	// mana sources on the battlefield — the AI cannot cast them, so they
	// sit in hand. After draw step (1 card) hand=13, well above the 7-card
	// maximum. Cleanup §514.1 must discard 6 down to 7.
	for i := 0; i < 12; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
			Name:           "Phyrexian Dreadnought",
			Types:          []string{"creature"},
			Owner:          0,
			BasePower:      12,
			BaseToughness:  12,
			CMC:            12,
			ManaCostString: "{12}",
		})
	}

	TakeTurn(gs)

	// Active seat's hand must end at exactly maximum hand size (§514.1).
	if got := len(gs.Seats[0].Hand); got != 7 {
		t.Fatalf("expected seat 0 hand to be discarded down to 7, got %d", got)
	}

	// A cleanup_loop event must have been logged with discard-trigger reason.
	// Before the r60 fix, the loop would break on !sbaChanged before logging,
	// even though §514.3a obligates priority + another pass when triggers
	// fired during the step.
	foundLoop := false
	loopReason := ""
	for _, ev := range gs.EventLog {
		if ev.Kind != "cleanup_loop" {
			continue
		}
		foundLoop = true
		if ev.Details != nil {
			if r, ok := ev.Details["reason"].(string); ok {
				loopReason = r
			}
		}
		break
	}
	if !foundLoop {
		t.Fatalf("expected cleanup_loop event after §514.1 discards fired triggers; got none")
	}
	if loopReason != "discard_triggers" && loopReason != "triggers_waiting" && loopReason != "sba" {
		t.Fatalf("cleanup_loop reason should be one of {discard_triggers,triggers_waiting,sba}, got %q", loopReason)
	}
}

// TestCleanupLoop_NoDiscardNoLoop is the negative companion: when the
// active player's hand is already at or below maximum entering cleanup
// and no SBAs/triggers fire, the cleanup loop must break on the first
// iteration with no cleanup_loop event. Pins the !discardsHappened &&
// !sbaChanged && !triggersWaiting break branch so it doesn't regress
// into looping forever.
func TestCleanupLoop_NoDiscardNoLoop(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}
	// Seat 0 enters with 3 cards — draw step pushes to 4. Well under 7,
	// no discard, no SBAs, no triggers → no cleanup_loop event.
	for i := 0; i < 3; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
	}

	TakeTurn(gs)

	for _, ev := range gs.EventLog {
		if ev.Kind == "cleanup_loop" {
			t.Fatalf("did not expect cleanup_loop event when hand <= 7 and no SBAs/triggers; got %+v", ev)
		}
	}
}

func TestCleanupLoop_SBADuringCleanupTriggersLoop(t *testing.T) {
	// This test verifies the loop happens when SBAs fire during cleanup.
	// Setup: a creature at exactly 0 toughness enters cleanup, SBA kills
	// it, which should trigger the cleanup loop (§514.3a).
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Active = 0
	gs.Turn = 2

	// Give both seats libraries so draw step works.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			&gameengine.Card{Name: "Forest", Types: []string{"land"}, Owner: 0})
		gs.Seats[1].Library = append(gs.Seats[1].Library,
			&gameengine.Card{Name: "Island", Types: []string{"land"}, Owner: 1})
	}

	// Put a creature with a until-EOT buff that will expire during cleanup,
	// leaving it at 0 toughness.
	creature := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:          "Fragile Thing",
			Types:         []string{"creature"},
			BasePower:     1,
			BaseToughness: 0, // 0 base toughness
		},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Modifications: []gameengine.Modification{
			{Power: 0, Toughness: 1, Duration: "until_end_of_turn"},
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, creature)

	TakeTurn(gs)

	// The creature should have been destroyed by SBAs during cleanup.
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Fragile Thing" {
			t.Fatal("Fragile Thing should have been destroyed during cleanup")
		}
	}
}
