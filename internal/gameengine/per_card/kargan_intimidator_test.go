package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func karganGame(t *testing.T) (*gameengine.GameState, *gameengine.Permanent) {
	gs := newGame(t, 2)
	gs.Active = 0
	k := addCreature(gs, 0, "Kargan Intimidator", 2, 2, "human", "warrior")
	gameengine.InvokeETBHook(gs, k)
	return gs, k
}

// Mode 1 (pump): +1/+1 until end of turn, and it's then unavailable until
// next turn; resets on the new turn.
func TestKargan_PumpMode_OncePerTurn(t *testing.T) {
	gs, k := karganGame(t)
	base := gs.PowerOf(k)

	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{"kargan_mode": "pump"})
	if got := gs.PowerOf(k); got != base+1 {
		t.Fatalf("pump should give +1/+1, power %d→%d", base, got)
	}
	// Second activation of the SAME mode this turn must be unavailable (no-op).
	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{"kargan_mode": "pump"})
	if got := gs.PowerOf(k); got != base+1 {
		t.Errorf("pump must fire at most once per turn; power moved to %d", got)
	}
	if karganModeAvailable(k, "pump", gs.Turn) {
		t.Error("pump must be marked unavailable after use this turn")
	}

	// New turn → mode resets.
	gs.Turn++
	if !karganModeAvailable(k, "pump", gs.Turn) {
		t.Error("pump must be available again next turn")
	}
}

// Mode 2 (coward): target creature becomes a Coward until end of turn; once
// per turn.
func TestKargan_CowardMode_AppliesAndOncePerTurn(t *testing.T) {
	gs, k := karganGame(t)
	victim := addCreature(gs, 1, "Grizzly Bears", 2, 2)

	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{
		"kargan_mode": "coward", "target_perm": victim,
	})
	if !gs.HasTypeOf(victim, "coward") {
		t.Error("coward mode must make the target a Coward")
	}
	if karganModeAvailable(k, "coward", gs.Turn) {
		t.Error("coward mode must be unavailable after use this turn")
	}
}

// The static "Cowards can't block Warriors" actually prevents the block.
func TestKargan_CowardsCantBlockWarriors(t *testing.T) {
	gs, k := karganGame(t)
	warrior := addCreature(gs, 0, "Some Warrior", 3, 3, "warrior")
	blocker := addCreature(gs, 1, "Enemy Wall", 0, 4)

	// Baseline: a normal creature CAN block the Warrior.
	if !gameengine.CanBlockGS(gs, warrior, blocker) {
		t.Fatalf("fixture: a non-Coward must be able to block")
	}
	// Make the blocker a Coward (Kargan mode 2).
	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{
		"kargan_mode": "coward", "target_perm": blocker,
	})
	if !gs.HasTypeOf(blocker, "coward") {
		t.Fatalf("fixture: blocker should be a Coward")
	}
	if gameengine.CanBlockGS(gs, warrior, blocker) {
		t.Error("a Coward must NOT be able to block a Warrior while Kargan is on the battlefield")
	}
	// A non-Warrior attacker is still blockable by the Coward.
	bear := addCreature(gs, 0, "Plain Bear", 2, 2)
	if !gameengine.CanBlockGS(gs, bear, blocker) {
		t.Error("a Coward may still block a non-Warrior")
	}
}

// Mode 3 (trample): target Warrior gains trample until end of turn; once per turn.
func TestKargan_TrampleMode_GrantsTrample(t *testing.T) {
	gs, k := karganGame(t)
	warrior := addCreature(gs, 0, "Ally Warrior", 3, 3, "warrior")
	if gs.HasKeywordOf(warrior, "trample") {
		t.Fatalf("fixture: warrior should not start with trample")
	}
	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{
		"kargan_mode": "trample", "target_perm": warrior,
	})
	if !gs.HasKeywordOf(warrior, "trample") {
		t.Error("trample mode must grant trample to the target Warrior")
	}
	if karganModeAvailable(k, "trample", gs.Turn) {
		t.Error("trample mode must be unavailable after use this turn")
	}
}

// The "until end of turn" durations actually wear off at cleanup.
func TestKargan_ModesExpireAtEndOfTurn(t *testing.T) {
	gs, k := karganGame(t)
	basePow := gs.PowerOf(k)
	victim := addCreature(gs, 1, "Foe", 2, 2)
	warrior := addCreature(gs, 0, "Pal", 2, 2, "warrior")

	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{"kargan_mode": "pump"})
	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{"kargan_mode": "coward", "target_perm": victim})
	karganIntimidatorActivate(gs, k, 0, map[string]interface{}{"kargan_mode": "trample", "target_perm": warrior})

	// Run the cleanup-step duration sweep.
	gameengine.ScanExpiredDurations(gs, "cleanup", "cleanup")
	// Modifications (pump) expire in the phases.go cleanup pass; sweep them too.
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil {
			continue
		}
		kept := p.Modifications[:0]
		for _, m := range p.Modifications {
			if m.Duration != "until_end_of_turn" && m.Duration != gameengine.DurationEndOfTurn {
				kept = append(kept, m)
			}
		}
		p.Modifications = kept
	}
	gs.InvalidateCharacteristicsCache()

	if got := gs.PowerOf(k); got != basePow {
		t.Errorf("pump should wear off at end of turn, power %d (base %d)", got, basePow)
	}
	if gs.HasTypeOf(victim, "coward") {
		t.Error("becomes-a-Coward should wear off at end of turn")
	}
	if gs.HasKeywordOf(warrior, "trample") {
		t.Error("trample grant should wear off at end of turn")
	}
}

// All three modes are independently once-per-turn: each fires exactly once in
// a turn; a 4th activation finds nothing available; all reset next turn.
func TestKargan_AllModesIndependentlyOncePerTurn(t *testing.T) {
	gs, k := karganGame(t)
	addCreature(gs, 1, "Foe", 2, 2)        // coward target
	addCreature(gs, 0, "Pal", 2, 2, "warrior") // trample target

	fired := map[string]bool{}
	for i := 0; i < 3; i++ {
		before := map[string]bool{}
		for _, m := range karganModeOrder {
			before[m] = karganModeAvailable(k, m, gs.Turn)
		}
		karganIntimidatorActivate(gs, k, 0, nil) // auto-pick first available
		for _, m := range karganModeOrder {
			if before[m] && !karganModeAvailable(k, m, gs.Turn) {
				fired[m] = true
			}
		}
	}
	if len(fired) != 3 {
		t.Errorf("auto-pick should have fired all 3 modes once each, fired=%v", fired)
	}
	// 4th activation: nothing left this turn.
	for _, m := range karganModeOrder {
		if karganModeAvailable(k, m, gs.Turn) {
			t.Errorf("mode %s should be exhausted this turn", m)
		}
	}
	// Next turn → all reset.
	gs.Turn++
	for _, m := range karganModeOrder {
		if !karganModeAvailable(k, m, gs.Turn) {
			t.Errorf("mode %s must reset next turn", m)
		}
	}
}
