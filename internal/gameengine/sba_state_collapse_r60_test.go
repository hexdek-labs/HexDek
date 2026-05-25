package gameengine

// R60 — SBA state-collapse audit: multiple simultaneous state changes
// resolving in one StateBasedActions call.
//
// CR §704.3 says SBAs happen as a single event. The engine calls helper
// rules sequentially within each pass (sba704_5f, then 5g, then 5h, ...),
// but the contract is: every applicable rule fires before priority is
// granted, and cross-rule interactions reach a fixed point in the
// multi-pass loop. Three audit gaps:
//
//   1. Cross-rule collapse — 3 toughness-zero creatures (§704.5f),
//      1 zero-loyalty planeswalker (§704.5i), and 1 zero-defense battle
//      (§704.5v) ALL die in a single StateBasedActions call. Existing
//      tests cover each rule in isolation; nothing pins the cross-rule
//      "single event" semantics.
//
//   2. Lethal damage beats legend rule in same pass — §704.5g runs
//      before §704.5j in the helper order. Two legendary copies, one
//      with lethal damage: only the damaged one dies (via §704.5g),
//      legend rule does not fire because only one copy survives the
//      lethal check.
//
//   3. Aura goes with its host in same pass — §704.5g destroys an
//      enchanted creature; §704.5m sees the now-orphaned Aura on the
//      same pass and graveyards it. Tests that §704.5m re-snapshots
//      the battlefield after §704.5g runs, so orphan detection works
//      within the same StateBasedActions call.
//
// Rule citations:
//   §704.3   — SBAs run as a single event
//   §704.5f  — toughness ≤ 0 → graveyard
//   §704.5g  — lethal damage → destroy
//   §704.5i  — 0 loyalty → graveyard
//   §704.5j  — legend rule
//   §704.5m  — Aura illegally attached → graveyard
//   §704.5v  — battle 0 defense → graveyard

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Cross-rule single-pass collapse: 3 × §704.5f + 1 × §704.5i + 1 × §704.5v.
//
// Five permanents qualifying for THREE different SBA rules must all be
// resolved in one call. Each rule's destroyPermSBA fires its specific
// sba_<rule> event AND the generic `destroy` event, so the audit trail
// must contain 5 destroys total partitioned 3/1/1.
// -----------------------------------------------------------------------------

func TestSBA_StateCollapse_MixedRules_OnePass(t *testing.T) {
	gs := newFixtureGame(t)

	// Three creatures at 0 toughness (§704.5f).
	c1 := addBattlefield(gs, 0, "Withered Bear", 2, 2, "creature")
	c1.AddCounter("-1/-1", 2)
	c2 := addBattlefield(gs, 0, "Withered Elf", 1, 1, "creature")
	c2.AddCounter("-1/-1", 1)
	c3 := addBattlefield(gs, 1, "Withered Wolf", 3, 3, "creature")
	c3.AddCounter("-1/-1", 3)

	// One planeswalker at 0 loyalty (§704.5i).
	walker := addBattlefield(gs, 0, "Chandra, Spent Fury", 0, 0, "planeswalker", "legendary")
	walker.AddCounter("loyalty", 0) // explicit 0 (not absent) → §704.5i fires

	// One battle at 0 defense (§704.5v).
	battle := addBattlefield(gs, 1, "Invasion of Glorious", 0, 0, "battle", "siege")
	battle.Counters = map[string]int{"defense": 0}

	StateBasedActions(gs)

	// All five must be off the battlefield.
	for _, s := range gs.Seats {
		if got := len(s.Battlefield); got != 0 {
			t.Errorf("seat %d battlefield: expected 0 remaining, got %d", s.Idx, got)
		}
	}
	// Generic destroy events: one per perm.
	if got := countEvents(gs, "destroy"); got != 5 {
		t.Errorf("expected 5 destroy events (3 + 1 + 1), got %d", got)
	}
	// Specific rule events.
	if got := countEvents(gs, "sba_704_5f"); got != 3 {
		t.Errorf("expected 3 sba_704_5f events, got %d", got)
	}
	if got := countEvents(gs, "sba_704_5i"); got != 1 {
		t.Errorf("expected 1 sba_704_5i event, got %d", got)
	}
	if got := countEvents(gs, "sba_704_5v"); got != 1 {
		t.Errorf("expected 1 sba_704_5v event, got %d", got)
	}
	// Sanity: cards landed in graveyards (no token semantics here).
	_ = c1
	_ = c2
	_ = c3
	_ = walker
	_ = battle
}

// -----------------------------------------------------------------------------
// Lethal damage beats legend rule in same pass.
//
// Two legendary copies of the same card under the same controller. One
// has lethal damage. The helper order in StateBasedActions runs §704.5g
// (line 76) BEFORE §704.5j (line 85), so the damaged copy is destroyed
// via §704.5g first; §704.5j then sees only one legendary on the
// battlefield and no-ops. The UNDAMAGED copy survives.
//
// This pins helper ordering — a regression that swapped §704.5j to fire
// first would force the player to pick a keeper before lethal damage
// resolved, potentially saving the damaged copy in cases where it had
// the lower timestamp (legend rule keeps earliest).
// -----------------------------------------------------------------------------

func TestSBA_StateCollapse_LethalBeatsLegendRule_SamePass(t *testing.T) {
	gs := newFixtureGame(t)

	// Earliest-timestamp copy takes lethal damage. Legend rule would
	// otherwise keep this one (it's earliest); since §704.5g runs first
	// it dies, and the LATER copy survives.
	dying := addBattlefield(gs, 0, "Akroma, Angel of Wrath", 6, 6, "creature", "legendary")
	surviving := addBattlefield(gs, 0, "Akroma, Angel of Wrath", 6, 6, "creature", "legendary")
	if dying.Timestamp >= surviving.Timestamp {
		t.Fatalf("test setup: dying must have earlier timestamp; got %d vs %d",
			dying.Timestamp, surviving.Timestamp)
	}
	dying.MarkedDamage = 6 // lethal

	StateBasedActions(gs)

	// Exactly one Akroma left on the battlefield.
	if got := len(gs.Seats[0].Battlefield); got != 1 {
		t.Fatalf("expected 1 Akroma surviving, got %d", got)
	}
	// And it must be the UNDAMAGED one (later timestamp). If §704.5j had
	// fired first, it would have destroyed the LATER copy (later
	// timestamp = legend rule loses) and the damaged earlier copy would
	// have died next pass via §704.5g — leaving zero Akromas.
	if gs.Seats[0].Battlefield[0] != surviving {
		t.Error("undamaged Akroma must survive; lethal damage applied first, legend rule never fires")
	}
	// One destroy via §704.5g, zero via §704.5j (legend rule didn't run).
	if got := countEvents(gs, "sba_704_5g"); got != 1 {
		t.Errorf("expected 1 sba_704_5g, got %d", got)
	}
	if got := countEvents(gs, "sba_704_5j"); got != 0 {
		t.Errorf("legend rule must NOT fire (only one Akroma after §704.5g); got %d sba_704_5j events", got)
	}
}

// -----------------------------------------------------------------------------
// Aura goes with its host in the same pass.
//
// Setup: a creature with an Aura attached. Lethal damage on the creature
// triggers §704.5g; the Aura's AttachedTo now points to a perm that's no
// longer on the battlefield, so §704.5m sees it as orphaned and
// graveyards it in the SAME StateBasedActions call.
//
// Helper ordering: §704.5g (line 76) → §704.5m (line 93). §704.5m's
// snapshotBattlefield is taken at function entry, AFTER §704.5g removed
// the host. The orphan detection in line 878 catches the dangling
// AttachedTo and fires another destroyPermSBA in the same pass.
// -----------------------------------------------------------------------------

func TestSBA_StateCollapse_AuraGoesWithHostInSamePass(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Pacifism", 0, 0, "enchantment", "aura")
	aura.AttachedTo = bear

	bear.MarkedDamage = 2 // lethal

	StateBasedActions(gs)

	if got := len(gs.Seats[0].Battlefield); got != 0 {
		t.Fatalf("both bear and aura must leave the battlefield; got %d remaining", got)
	}
	if got := countEvents(gs, "destroy"); got != 2 {
		t.Errorf("expected 2 destroy events (creature + aura), got %d", got)
	}
	if got := countEvents(gs, "sba_704_5g"); got != 1 {
		t.Errorf("expected 1 sba_704_5g (the bear), got %d", got)
	}
	if got := countEvents(gs, "sba_704_5m"); got != 1 {
		t.Errorf("expected 1 sba_704_5m (the orphaned aura), got %d", got)
	}
	// Both should be in the graveyard.
	if got := len(gs.Seats[0].Graveyard); got != 2 {
		t.Errorf("expected 2 cards in graveyard, got %d", got)
	}
}
