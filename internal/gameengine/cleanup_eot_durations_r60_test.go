package gameengine

// R60 — "until end of turn" duration cleanup at §514.2 cleanup step.
//
// (Branch name says §704.5n for historical reasons — current CR §704.5n
// is Equipment/Fortification unattach; the "EOT counter cleanup" pattern
// from older sets is actually handled at the §514.2 cleanup step via
// `ScanExpiredDurations`, not as an SBA. This file audits THAT path.)
//
// The engine has three orthogonal "until end of turn" surfaces:
//
//   1. `Permanent.Modifications`              — runtime P/T buffs
//   2. `Permanent.MarkedDamage`               — combat / spell damage
//   3. `GameState.ContinuousEffects`          — layer-system effects
//
// And one notable non-surface: `Permanent.Counters` is INTENTIONALLY
// duration-less per CR (counters persist unless an effect removes them
// — "until end of turn, ~ gets a +1/+1 counter" is not a real MTG
// pattern; cards that look that way actually use a P/T modification or
// a paired remove-at-EOT delayed trigger).
//
// Gaps before this file:
//
//   - No test pins the mixed-duration Modifications case
//     (until_end_of_turn alongside permanent on the same permanent).
//   - No test pins the MarkedDamage clear + `damage_wears_off` event
//     emission at cleanup.
//   - No test asserts that real `Counters` survive cleanup — a future
//     regression that "drained" Counters at cleanup would silently
//     wreck every +1/+1 / loyalty / charge / time accumulation.
//
// Rule citations:
//   §514.2  — cleanup step: damage wears off, "until end of turn"
//             effects end
//   §122.6  — counters persist on permanents until removed
//   §613.6c — duration of continuous effects from spells/abilities

import (
	"testing"
)

// -----------------------------------------------------------------------------
// Modifications: mixed EOT + permanent on the same permanent.
//
// Cleanup must drop the EOT entry, preserve the permanent entry, and
// `Power()` / `Toughness()` must report the post-cleanup buff.
// -----------------------------------------------------------------------------

func TestCleanup_EOT_ModificationsMixed_OnlyEOTExpires(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")

	// Two stacked buffs: Giant Growth (+3/+3 EOT) and Glorious Anthem (+1/+1 permanent).
	bear.Modifications = append(bear.Modifications,
		Modification{Power: 3, Toughness: 3, Duration: "until_end_of_turn", Timestamp: gs.NextTimestamp()},
		Modification{Power: 1, Toughness: 1, Duration: "permanent", Timestamp: gs.NextTimestamp()},
	)
	if got := bear.Power(); got != 6 { // 2 + 3 + 1
		t.Fatalf("pre-cleanup Power: expected 6, got %d", got)
	}
	if got := bear.Toughness(); got != 6 {
		t.Fatalf("pre-cleanup Toughness: expected 6, got %d", got)
	}

	ScanExpiredDurations(gs, "ending", "cleanup")

	// Exactly one Modification should remain — the permanent one.
	if got := len(bear.Modifications); got != 1 {
		t.Fatalf("expected 1 surviving Modification (permanent), got %d", got)
	}
	if got := bear.Modifications[0].Duration; got != "permanent" {
		t.Errorf("the surviving Modification should be the permanent one, got Duration=%q", got)
	}
	if got := bear.Power(); got != 3 { // 2 base + 1 permanent
		t.Errorf("post-cleanup Power: expected 3, got %d", got)
	}
	if got := bear.Toughness(); got != 3 {
		t.Errorf("post-cleanup Toughness: expected 3, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// MarkedDamage wears off + emits `damage_wears_off` audit event (§514.2).
//
// Damage marked on creatures must zero at cleanup, and the audit trail
// must carry one `damage_wears_off` per creature that had damage. A
// creature with zero MarkedDamage must NOT emit the event.
// -----------------------------------------------------------------------------

func TestCleanup_EOT_MarkedDamageClearsAndEmitsEvent(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	hill := addBattlefield(gs, 0, "Hill Giant", 3, 3, "creature")
	wall := addBattlefield(gs, 0, "Wall of Stone", 0, 8, "creature")

	bear.MarkedDamage = 1 // soaked a Shock but survived because of a +1 buff (immaterial)
	hill.MarkedDamage = 2
	// wall takes no damage this turn.

	ScanExpiredDurations(gs, "ending", "cleanup")

	if bear.MarkedDamage != 0 {
		t.Errorf("Bear: MarkedDamage should clear at cleanup, got %d", bear.MarkedDamage)
	}
	if hill.MarkedDamage != 0 {
		t.Errorf("Hill Giant: MarkedDamage should clear at cleanup, got %d", hill.MarkedDamage)
	}
	if wall.MarkedDamage != 0 {
		t.Errorf("Wall: MarkedDamage should remain 0, got %d", wall.MarkedDamage)
	}
	// Two damage_wears_off events — one per creature with damage. The wall
	// MUST NOT emit the event (no damage to wear off).
	if got := countEvents(gs, "damage_wears_off"); got != 2 {
		t.Fatalf("expected exactly 2 damage_wears_off events (bear+hill), got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Counters intentionally survive cleanup — they have no duration.
//
// Even if a creature got a +1/+1 counter "earlier this turn" (Slith
// shape), MTG counters are persistent (§122.6) — cleanup must NOT touch
// them. This is the audit the user's "EOT counter durations" question
// implicitly asks: the engine should NOT model EOT-removed counters
// because no such thing exists in modern rules.
//
// Loyalty, charge, time, and the +1/+1 / -1/-1 family all survive.
// A future regression that drained `Counters` at cleanup would silently
// wreck planeswalker loyalty across turns, undo every Hardened-Scales
// stack, and reset Glissa Sunslayer's poison/oil counters.
// -----------------------------------------------------------------------------

func TestCleanup_EOT_CountersPersistAcrossCleanup(t *testing.T) {
	gs := newFixtureGame(t)
	walker := addBattlefield(gs, 0, "Karn, the Great Creator", 0, 0, "planeswalker", "legendary")
	walker.AddCounter("loyalty", 5)
	bear := addBattlefield(gs, 0, "Servant of the Scale", 1, 1, "creature")
	bear.AddCounter("+1/+1", 3)
	chalice := addBattlefield(gs, 0, "Chalice of the Void", 0, 0, "artifact")
	chalice.AddCounter("charge", 2)
	relic := addBattlefield(gs, 0, "Suspended Relic", 0, 0, "artifact")
	relic.AddCounter("time", 4)
	// A creature with a -1/-1 counter too, to pin both signs.
	pup := addBattlefield(gs, 0, "Wither Pup", 2, 2, "creature")
	pup.AddCounter("-1/-1", 1)

	ScanExpiredDurations(gs, "ending", "cleanup")

	if got := walker.Counters["loyalty"]; got != 5 {
		t.Errorf("loyalty must persist through cleanup; got %d", got)
	}
	if got := bear.Counters["+1/+1"]; got != 3 {
		t.Errorf("+1/+1 must persist through cleanup; got %d", got)
	}
	if got := chalice.Counters["charge"]; got != 2 {
		t.Errorf("charge must persist through cleanup; got %d", got)
	}
	if got := relic.Counters["time"]; got != 4 {
		t.Errorf("time must persist through cleanup; got %d", got)
	}
	if got := pup.Counters["-1/-1"]; got != 1 {
		t.Errorf("-1/-1 must persist through cleanup; got %d", got)
	}
	// And derived P/T should still reflect those counters post-cleanup.
	if got := bear.Power(); got != 4 { // 1 base + 3 +1/+1
		t.Errorf("bear Power after cleanup: counters must still apply; got %d", got)
	}
}
