package gameengine

import (
	"testing"
)

// sba_stub_return_capture_r60_test.go — r60 SBA loop completeness audit.
//
// CR §704.3: "Whenever a player would get priority, the game first performs
// all applicable state-based actions as a single event, then repeats this
// process until no state-based actions are performed."
//
// The StateBasedActions outer loop relied on each helper's bool return to
// keep the loop alive. Five helpers (§704.5h deathtouch tracking, §704.5t
// dungeon completion, §704.5w battle protector assignment, §704.5x siege
// protector reset, §704.5z Start Your Engines speed init) were marked as
// "stubs" at their call sites — comments dated from when the helpers
// really were no-ops — but the helpers were later wired to real
// implementations. The call sites still dropped the bool return, so a
// pass in which ONLY one of those helpers mutated state would terminate
// the SBA loop early, violating CR §704.3.
//
// Two regressions here:
//
// 1. TestSBA_LoopAlive_DeathtouchAloneReportsChange — solo §704.5h fire
//    must report changed=true to the outer loop. Without the fix the
//    creature still gets destroyed (the helper itself works), but
//    StateBasedActions returns false and the caller has no way to know
//    state mutated.
//
// 2. TestSBA_LoopAlive_BattleSacrificeCascadesToAuraIllegalAttach — §704.5w
//    sacrifices a battle whose Aura then orphans. §704.5w runs AFTER
//    §704.5m in the pass list, so the Aura cleanup requires a SECOND
//    SBA pass. Without the fix, the outer loop exits after pass 1
//    because §704.5w's return was dropped; the orphan Aura sits on the
//    battlefield with AttachedTo=nil, violating §704.5m.

func TestSBA_LoopAlive_DeathtouchAloneReportsChange(t *testing.T) {
	gs := newFixtureGame(t)
	// Create a creature with deathtouch_damaged flag set and nonzero
	// MarkedDamage. Toughness > 0 keeps §704.5f from picking it up;
	// MarkedDamage < toughness keeps §704.5g out of it. Only §704.5h
	// can destroy this creature in this pass.
	p := addBattlefield(gs, 0, "Touched Creature", 2, 3, "creature")
	p.Flags["deathtouch_damaged"] = 1
	p.MarkedDamage = 1

	changed := StateBasedActions(gs)
	if !changed {
		t.Fatal("StateBasedActions must report changed=true when §704.5h destroys a creature; " +
			"a dropped bool from sba704_5h hides the mutation from the outer loop and from callers")
	}
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("§704.5h should have destroyed the deathtouch-damaged creature; %d still on battlefield",
			len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Graveyard) == 0 {
		t.Fatal("destroyed creature should land in graveyard")
	}
	// Confirm via event that the destroy was attributed to §704.5h, not
	// §704.5f or §704.5g — defends against the test passing for the
	// wrong reason if e.g. someone widens §704.5f to catch this case.
	found := false
	for _, e := range gs.EventLog {
		if e.Kind == "sba_704_5h" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sba_704_5h event in log; deathtouch SBA should have been the attribution")
	}
}

func TestSBA_LoopAlive_BattleSacrificeCascadesToAuraIllegalAttach(t *testing.T) {
	gs := newFixtureGame(t)
	// Seat 1 is the only opponent in a 2-seat fixture; mark it lost so
	// §704.5w cannot find a living opponent for the battle, forcing the
	// "no valid protector → sacrifice" branch (sba.go:1451-1455).
	gs.Seats[1].Lost = true

	// Place a battle on seat 0 with no current protector. §704.5w tries
	// to assign a protector from gs.Opponents(0); the only opponent is
	// Lost, so the battle gets sacrificed.
	//
	// Give the battle defense=3 so §704.5v ("battle with 0 defense →
	// graveyard") doesn't pre-empt §704.5w. That ordering matters: with
	// defense=0 the battle dies via §704.5v earlier in the same pass and
	// §704.5w never gets a chance to fire, defeating the test's purpose.
	battle := addBattlefield(gs, 0, "Test Battle", 0, 0, "battle")
	battle.Counters["defense"] = 3
	battle.Flags["has_protector"] = 0

	// Attach an Aura to the battle. After §704.5w sacrifices the battle,
	// destroyPermSBA → detachAll clears the Aura's AttachedTo, which
	// §704.5m would catch — but §704.5m runs BEFORE §704.5w in the same
	// pass (see sba.go ordering: 5m at line 93, 5w at line 120), so the
	// Aura's orphan state needs a SECOND SBA pass.
	aura := addBattlefield(gs, 0, "Test Aura", 0, 0, "enchantment", "aura")
	aura.AttachedTo = battle

	changed := StateBasedActions(gs)
	if !changed {
		t.Fatal("StateBasedActions must report changed=true after §704.5w sacrifice")
	}

	// Battle should be sacrificed.
	for _, p := range gs.Seats[0].Battlefield {
		if p == battle {
			t.Fatal("battle should have been sacrificed by §704.5w (no valid protector)")
		}
	}

	// CRITICAL: the orphaned Aura must also have been destroyed by
	// §704.5m on a second SBA pass. Before the fix, the outer loop
	// exited after pass 1 because sba704_5w's return was dropped, so
	// §704.5m never got a second pass to see the orphan Aura.
	for _, p := range gs.Seats[0].Battlefield {
		if p == aura {
			t.Fatal("orphaned Aura should have been destroyed by §704.5m on the second SBA pass; " +
				"the outer loop exited early because §704.5w's return was dropped")
		}
	}

	// Both should be in graveyard.
	gvCards := 0
	for _, c := range gs.Seats[0].Graveyard {
		if c == battle.Card || c == aura.Card {
			gvCards++
		}
	}
	if gvCards != 2 {
		t.Errorf("expected both battle and aura in graveyard, got %d cards matching", gvCards)
	}
}
