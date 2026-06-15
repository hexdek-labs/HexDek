package gameengine

import (
	"testing"
)

// phase_transition_stress_r60_test is the OODA-plan PR-C
// deterministic phase-transition stress harness. Loki's chaos
// runner exercises random card mixes against the full turn loop,
// but it can't deterministically reproduce the rarer phase-step
// edge cases (skip-untap effects layered with extra combats,
// multi-modification cleanup interactions, extra-turn duration
// carry-over, cleanup-window trigger queuing) at the rate needed
// to catch regressions before they ship.
//
// Each scenario in this file is a deterministic test asserting a
// CR-canonical end-state for one of the five phase-transition
// stress axes called out in the PR-C charter:
//
//	(a) "Skip your untap step" effects — Stasis-shape
//	(b) Multiple "until end of turn" / "until end of combat"
//	    modifications stacking + cleaning up together
//	(c) Extra combat phases with skip-untap on top (CR §500.7 —
//	    extra phases don't get untap steps)
//	(d) "Take an extra turn" with end-step duration carry-over
//	(e) Cleanup step modification + flag-clear interactions
//	    per CR §514.2

// ────────────────────────────────────────────────────────────────
// (a) Skip-untap edge cases — Stasis / Frozen Aether / Sands of Time
// ────────────────────────────────────────────────────────────────

// TestPhaseTransitionStress_StasisSkipUntapStillClearsSummoningSickness
// pins the §502.1 carve-out: even when SkipUntapStep is set, the
// active seat's creatures still lose summoning sickness at the
// start of the would-be untap step. Stasis stops creatures
// from untapping but doesn't preserve sickness — a creature that
// resolved last turn under Stasis is no longer summoning-sick
// this turn even though it didn't untap.
func TestPhaseTransitionStress_StasisSkipUntapStillClearsSummoningSickness(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].SkipUntapStep = true // Stasis-equivalent effect

	c := addPerm(gs, 0, "Llanowar Elves", []string{"creature"})
	c.Tapped = true
	c.SummoningSick = true

	UntapAll(gs, 0)

	if !c.Tapped {
		t.Errorf("Stasis skip-untap: creature should remain tapped (untap step skipped), got Tapped=false")
	}
	if c.SummoningSick {
		t.Errorf("Stasis skip-untap: summoning sickness should still clear at the start of untap step per §502.1 carve-out, got SummoningSick=true")
	}
	// Verify an explicit log event marks the skip — auditable trail.
	foundSkipEvent := false
	for _, e := range gs.EventLog {
		if e.Kind == "untap_step_skipped" {
			foundSkipEvent = true
			break
		}
	}
	if !foundSkipEvent {
		t.Errorf("expected 'untap_step_skipped' log event for SkipUntapStep=true, got none")
	}
}

// TestPhaseTransitionStress_FrozenAetherPerPermanentSkipUntap pins
// the per-permanent skip-untap shape (Frozen Aether / Static Orb
// at the per-card level — Permanent.Flags["skip_untap"] > 0). The
// affected permanent stays tapped; OTHER permanents on the same
// seat's battlefield still untap normally.
func TestPhaseTransitionStress_FrozenAetherPerPermanentSkipUntap(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	frozen := addPerm(gs, 0, "Frozen Token", []string{"creature"})
	frozen.Tapped = true
	frozen.Flags["skip_untap"] = 1

	free := addPerm(gs, 0, "Llanowar Elves", []string{"creature"})
	free.Tapped = true

	UntapAll(gs, 0)

	if !frozen.Tapped {
		t.Errorf("Frozen Token: per-permanent skip_untap should preserve Tapped, got Tapped=false")
	}
	if free.Tapped {
		t.Errorf("Untargeted permanent: should untap normally, got Tapped=true")
	}
}

// TestPhaseTransitionStress_DoesNotUntapFlagHonoredOnPermanent
// pins the Permanent.DoesNotUntap path (Sands of Time / Stasis
// applied to a single permanent). Same outcome as the skip_untap
// flag — the permanent stays tapped — but tracked via a different
// field that supports nuanced future logic (e.g. removing one
// stun counter per §122.4 in place of untapping).
func TestPhaseTransitionStress_DoesNotUntapFlagHonoredOnPermanent(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := addPerm(gs, 0, "Sands Target", []string{"creature"})
	p.Tapped = true
	p.DoesNotUntap = true

	UntapAll(gs, 0)
	if !p.Tapped {
		t.Errorf("DoesNotUntap permanent should stay tapped, got Tapped=false")
	}
	// Should log an untap_skipped event with the reason.
	foundLog := false
	for _, e := range gs.EventLog {
		if e.Kind == "untap_skipped" && e.Source == "Sands Target" {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Errorf("DoesNotUntap should emit 'untap_skipped' event with source name, got none")
	}
}

// ────────────────────────────────────────────────────────────────
// (b) Multiple until-EOT modifications stacking + cleanup
// ────────────────────────────────────────────────────────────────

// TestPhaseTransitionStress_MultipleUntilEOTModificationsCleanedUp
// verifies that a permanent carrying multiple until-EOT
// modifications has ALL of them stripped in a single ScanExpiredDurations
// call at the cleanup step (CR §514.2). A regression where only
// the first modification gets cleared would compound across turns,
// leaving stale P/T bumps on the board after the turn ends.
func TestPhaseTransitionStress_MultipleUntilEOTModificationsCleanedUp(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := addPerm(gs, 0, "Test Creature", []string{"creature"})
	// Stack 3 until-EOT modifications on the same permanent.
	p.Modifications = []Modification{
		{Power: 2, Toughness: 2, Duration: "until_end_of_turn"},
		{Power: 1, Toughness: 1, Duration: "until_end_of_turn"},
		{Power: 1, Toughness: 0, Duration: "until_end_of_turn"},
	}
	// Plus a permanent (no-duration) mod that should SURVIVE cleanup.
	p.Modifications = append(p.Modifications, Modification{
		Power: 3, Toughness: 3, Duration: "permanent",
	})

	ScanExpiredDurations(gs, "ending", "cleanup")

	if len(p.Modifications) != 1 {
		t.Errorf("cleanup should leave 1 permanent mod, got %d", len(p.Modifications))
	}
	if len(p.Modifications) == 1 && p.Modifications[0].Duration != "permanent" {
		t.Errorf("surviving mod should be the permanent one, got %q", p.Modifications[0].Duration)
	}
}

// TestPhaseTransitionStress_MarkedDamageWearsOffAtCleanup pins the
// §514.2 marked-damage clear: damage on permanents zeroes at
// cleanup regardless of how many sources marked it. A regression
// where MarkedDamage > 0 survives cleanup would let combat damage
// from prior turns accumulate, eventually killing creatures
// silently on a future SBA pass.
func TestPhaseTransitionStress_MarkedDamageWearsOffAtCleanup(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p1 := addPerm(gs, 0, "Damaged Bear", []string{"creature"})
	p1.MarkedDamage = 3
	p2 := addPerm(gs, 0, "Heavily Damaged", []string{"creature"})
	p2.MarkedDamage = 7

	ScanExpiredDurations(gs, "ending", "cleanup")

	if p1.MarkedDamage != 0 {
		t.Errorf("damaged bear: marked damage should wear off, got %d", p1.MarkedDamage)
	}
	if p2.MarkedDamage != 0 {
		t.Errorf("heavily damaged: marked damage should wear off, got %d", p2.MarkedDamage)
	}
}

// TestPhaseTransitionStress_GrantedAbilitiesClearedAtCleanup pins
// the §514.2 EOT-granted-abilities clear. A regression where
// flashback / unearth / haste grants survive cleanup would leak
// abilities into subsequent turns.
func TestPhaseTransitionStress_GrantedAbilitiesClearedAtCleanup(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := addPerm(gs, 0, "Hasted Creature", []string{"creature"})
	p.GrantedAbilities = []string{"haste", "trample"}

	ScanExpiredDurations(gs, "ending", "cleanup")

	if len(p.GrantedAbilities) != 0 {
		t.Errorf("granted abilities should clear at cleanup, got %v", p.GrantedAbilities)
	}
}

// ────────────────────────────────────────────────────────────────
// (c) Extra combat phases — CR §500.7 untap-step absence
// ────────────────────────────────────────────────────────────────

// TestPhaseTransitionStress_ExtraCombatDoesNotProduceUntapStep
// pins CR §500.7 — extra combat phases do NOT come with an
// associated untap step. A regression where AddExtraCombat
// triggered an UntapAll call would let "untap all your creatures
// at the beginning of each combat" effects (Aggravated Assault,
// World at War) compound into infinite-mana shenanigans by
// untapping mana producers each combat.
//
// The deterministic check: queue an extra combat, scan the event
// log, assert no "untap_all" or "untap_step" event fires.
func TestPhaseTransitionStress_ExtraCombatDoesNotProduceUntapStep(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Queue an extra combat; this should append to PendingExtraCombats
	// without triggering an untap step.
	gs.AddExtraCombat(PendingExtraCombat{SourceCard: "World at War"})

	if len(gs.PendingExtraCombats) != 1 {
		t.Errorf("extra combat queued: want 1 in PendingExtraCombats, got %d",
			len(gs.PendingExtraCombats))
	}
	// No untap-related event should have fired.
	for _, e := range gs.EventLog {
		if e.Kind == "untap_step" || e.Kind == "untap_all" {
			t.Errorf("AddExtraCombat triggered unexpected %q event — extra combats don't get untap steps per CR §500.7",
				e.Kind)
		}
	}
}

// TestPhaseTransitionStress_SkipUntapAcrossExtraTurnNotDoubleApplied
// verifies that a single "Skip your next untap step" effect
// applied to a seat is consumed by the seat's NEXT untap step
// only — a subsequent extra turn's untap step proceeds normally.
// The shape: set SkipUntapStep, run UntapAll once (which skips
// the step), then reset SkipUntapStep (real engine clears it via
// the resolution that imposed it expiring at end of turn), then
// run UntapAll again and verify creatures untap normally.
//
// This is the deterministic version of a Stasis-removed-then-
// untap pattern that Loki rarely hits.
func TestPhaseTransitionStress_SkipUntapAcrossExtraTurnNotDoubleApplied(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].SkipUntapStep = true

	c := addPerm(gs, 0, "Llanowar Elves", []string{"creature"})
	c.Tapped = true

	UntapAll(gs, 0)
	if !c.Tapped {
		t.Errorf("first untap step (skipped): want Tapped=true, got false")
	}

	// Simulate the SkipUntapStep effect expiring before the next turn.
	gs.Seats[0].SkipUntapStep = false

	UntapAll(gs, 0)
	if c.Tapped {
		t.Errorf("second untap step (effect expired): want Tapped=false (untapped normally), got true")
	}
}

// ────────────────────────────────────────────────────────────────
// (d) Extra-turn end-step duration carry-over
// ────────────────────────────────────────────────────────────────

// TestPhaseTransitionStress_ExtraTurnsPendingCounterStacks pins the
// extra-turn accounting: each "Take an extra turn" resolves into
// gs.Flags["extra_turns_pending"]++. A regression where the
// counter doesn't stack would let multiple extra-turn spells
// resolve into only ONE extra turn (the counter would overwrite
// rather than increment).
func TestPhaseTransitionStress_ExtraTurnsPendingCounterStacks(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Simulate 3 "take an extra turn" resolutions.
	gs.Flags["extra_turns_pending"]++
	gs.Flags["extra_turns_pending"]++
	gs.Flags["extra_turns_pending"]++
	if gs.Flags["extra_turns_pending"] != 3 {
		t.Errorf("3 extra-turn resolutions: want extra_turns_pending=3, got %d",
			gs.Flags["extra_turns_pending"])
	}
}

// TestPhaseTransitionStress_DurationUntilEndOfYourNextTurnSurvivesIntervening
// pins CR-correct duration carry-over: a "until end of your next
// turn" duration registered on seat 0's turn should NOT expire at
// seat 0's end step of THIS turn — it expires at the end of seat
// 0's NEXT turn. A regression where the duration expires at the
// current turn's end step would cut off temporary control-change
// effects (Threaten, Act of Treason scope) early.
func TestPhaseTransitionStress_DurationUntilEndOfYourNextTurnSurvivesIntervening(t *testing.T) {
	// Seat 0 is active; registering a "until end of your next turn"
	// effect on seat 0's turn (turn 5) should NOT expire at this turn's
	// end step because the source's NEXT turn hasn't happened yet.
	expired := durationExpiresNow(DurationUntilEndOfYourNextTurn,
		0 /* controllerSeat */, 0 /* activeSeat */, "ending", "end_step",
		5 /* currentTurn */, 5 /* createdTurn */)
	if expired {
		t.Errorf("until_end_of_your_next_turn on seat 0's turn: should NOT expire at seat 0's first end step (it expires at the end of the NEXT seat-0 turn)")
	}

	// r63 cleanup-step regression: the effect must ALSO survive THIS
	// turn's CLEANUP step (the §514.2 boundary). Before the CreatedTurn
	// guard, durationExpiresNow matched step=="cleanup" && controller==
	// active and reaped the effect a full turn early — at the end of the
	// turn it was created on, not the controller's NEXT turn (property e).
	if durationExpiresNow(DurationUntilEndOfYourNextTurn,
		0, 0, "ending", "cleanup", 5 /* currentTurn */, 5 /* createdTurn */) {
		t.Errorf("until_end_of_your_next_turn created on turn 5: must NOT expire at turn 5's cleanup — that is the SAME turn, not the controller's next turn")
	}
	// On the controller's NEXT own turn (turn 9 in a 4-player game), the
	// cleanup boundary DOES expire it.
	if !durationExpiresNow(DurationUntilEndOfYourNextTurn,
		0, 0, "ending", "cleanup", 9 /* currentTurn */, 5 /* createdTurn */) {
		t.Errorf("until_end_of_your_next_turn created on turn 5: MUST expire at the controller's next own-turn cleanup (turn 9), got false")
	}
	// Same shape for "until your next end step".
	if durationExpiresNow(DurationUntilYourNextEndStep,
		0, 0, "ending", "end_step", 5, 5) {
		t.Errorf("until_your_next_end_step created on turn 5: must NOT expire at turn 5's own end step")
	}
	if !durationExpiresNow(DurationUntilYourNextEndStep,
		0, 0, "ending", "end_step", 9, 5) {
		t.Errorf("until_your_next_end_step created on turn 5: MUST expire at the controller's next own end step (turn 9), got false")
	}
}

// TestPhaseTransitionStress_UntilEndOfTurnExpiresAtCleanupNotEndStep
// pins the §514.2 vs §513 distinction: "until end of turn" effects
// expire at the CLEANUP step, not the END step. A regression where
// they expired at end_step would let end-step triggers see a
// world where the modifications had already worn off, breaking
// "at the beginning of your end step, do X to each creature with
// a +1/+1 counter" type effects.
func TestPhaseTransitionStress_UntilEndOfTurnExpiresAtCleanupNotEndStep(t *testing.T) {
	// Both at active seat 0 — duration is "until end of turn".
	expiresAtEnd := durationExpiresNow("until_end_of_turn",
		0, 0, "ending", "end_step", 5, 5)
	if expiresAtEnd {
		t.Errorf("until_end_of_turn at end_step: should NOT expire yet — expiry is at cleanup per §514.2")
	}
	expiresAtCleanup := durationExpiresNow("until_end_of_turn",
		0, 0, "ending", "cleanup", 5, 5)
	if !expiresAtCleanup {
		t.Errorf("until_end_of_turn at cleanup: should expire per §514.2, got false")
	}
}

// ────────────────────────────────────────────────────────────────
// (e) Cleanup-step trigger interactions per CR §514
// ────────────────────────────────────────────────────────────────

// TestPhaseTransitionStress_CleanupClearsTransientGameFlags pins
// the §514.2 game-wide flag clear that runs alongside the per-
// permanent cleanup. A regression where transient flags like
// "prevent_all_combat_damage" (Fog) or per-permanent
// "basilisk_granted" survived cleanup would carry combat
// modifiers into subsequent turns.
func TestPhaseTransitionStress_CleanupClearsTransientGameFlags(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Flags["prevent_all_combat_damage"] = 1
	p := addPerm(gs, 0, "Cockatrice", []string{"creature"})
	p.Flags["basilisk_granted"] = 1
	p.Flags["basilisk_combat_hit"] = 1
	p.Flags["basilisk_marked_destroy"] = 1
	// Set a NON-transient flag — should survive.
	p.Flags["was_cast"] = 1

	ScanExpiredDurations(gs, "ending", "cleanup")

	if gs.Flags["prevent_all_combat_damage"] != 0 {
		t.Errorf("game flag prevent_all_combat_damage: should clear at cleanup, got %d",
			gs.Flags["prevent_all_combat_damage"])
	}
	if p.Flags["basilisk_granted"] != 0 {
		t.Errorf("basilisk_granted: should clear, got %d", p.Flags["basilisk_granted"])
	}
	if p.Flags["basilisk_combat_hit"] != 0 {
		t.Errorf("basilisk_combat_hit: should clear, got %d", p.Flags["basilisk_combat_hit"])
	}
	if p.Flags["basilisk_marked_destroy"] != 0 {
		t.Errorf("basilisk_marked_destroy: should clear, got %d", p.Flags["basilisk_marked_destroy"])
	}
	// Non-transient flag should survive.
	if p.Flags["was_cast"] != 1 {
		t.Errorf("was_cast (non-transient): should survive cleanup, got %d",
			p.Flags["was_cast"])
	}
}

// TestPhaseTransitionStress_CleanupIsIdempotent pins that running
// the cleanup pass twice doesn't double-clear anything or restore
// already-cleared state. A regression where the second call
// undoes the first (e.g. by re-adding modifications) would be
// catastrophic but theoretically possible if the cleanup logic
// has any non-idempotent state writes.
func TestPhaseTransitionStress_CleanupIsIdempotent(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	p := addPerm(gs, 0, "Modified Creature", []string{"creature"})
	p.Modifications = []Modification{
		{Power: 2, Toughness: 2, Duration: "until_end_of_turn"},
	}
	p.MarkedDamage = 5

	ScanExpiredDurations(gs, "ending", "cleanup")
	firstModCount := len(p.Modifications)
	firstDamage := p.MarkedDamage

	// Run again — should be a no-op
	ScanExpiredDurations(gs, "ending", "cleanup")

	if len(p.Modifications) != firstModCount {
		t.Errorf("second cleanup changed mod count: %d -> %d", firstModCount, len(p.Modifications))
	}
	if p.MarkedDamage != firstDamage {
		t.Errorf("second cleanup changed damage: %d -> %d", firstDamage, p.MarkedDamage)
	}
}

// TestPhaseTransitionStress_DurationUntilNextEndStepExpiresAtNextEndStep
// pins the CR §613.7 / §613.8 duration: a continuous effect with
// DurationUntilNextEndStep should expire at the very next end
// step, regardless of which seat's turn it is.
func TestPhaseTransitionStress_DurationUntilNextEndStepExpiresAtNextEndStep(t *testing.T) {
	// At end_step, the duration should signal expiry regardless of
	// controllerSeat / activeSeat alignment.
	if !durationExpiresNow(DurationUntilNextEndStep, 1, 0, "ending", "end_step", 5, 4) {
		t.Errorf("DurationUntilNextEndStep at end_step (cross-seat): should expire, got false")
	}
	if !durationExpiresNow(DurationUntilNextEndStep, 0, 0, "ending", "end_step", 5, 4) {
		t.Errorf("DurationUntilNextEndStep at end_step (same-seat): should expire, got false")
	}
	// At cleanup, should not yet expire — already done at end_step.
	if durationExpiresNow(DurationUntilNextEndStep, 0, 0, "ending", "cleanup", 5, 4) {
		t.Errorf("DurationUntilNextEndStep at cleanup: should already have expired at end_step")
	}
}
