package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R53 stub-batch port O. The per_card stub pool is heavily thinned
// after batches A–N and the R46 stub hunt — only 16 emitPartial-bearing
// gen_*.go handlers remain in scope after exclusion, and the partials
// in most are legitimate engine-deep gaps (cost_modifiers, AST keyword
// pipeline, while_source_on_bf grants) that correctly belong on the
// engine side. Batch O ships three real fixes rather than padding to
// 10 with low-value churn, matching the batchJ/batchL precedent.
//
// Picks:
//
// (A) DFC face-name dispatch gap — REAL BUG FIX:
//   - Cecil, Dark Knight //         Author registered the three damage
//     Cecil, Redeemed Paladin        triggers (combat / noncombat to
//                                     player / noncombat to creature)
//                                     under only the DFC composite
//                                     name, but registered the attack
//                                     trigger under both face names
//                                     too. lookupCandidates strips
//                                     composite → front but does NOT
//                                     re-compose front-only → composite,
//                                     so a Cecil whose Card.Name
//                                     resolves to "Cecil, Dark Knight"
//                                     or "Cecil, Redeemed Paladin"
//                                     alone (e.g., after transform)
//                                     silently skipped the Darkness
//                                     self-drain. Added 6 new face-
//                                     name aliases for the damage
//                                     triggers (3 triggers × 2 face
//                                     names).
//
// (B) Flicker-bypass on once-per-turn gate — REAL BUG FIX:
//   - The Reaper, King No More      Oracle: "Do this only once each
//                                     turn." The old gate stored
//                                     perm.Flags["reaper_stole_turn"]
//                                     = gs.Turn on the source. A
//                                     flickered (Cloudstone Curio /
//                                     Ephemerate / Restoration Angel)
//                                     Reaper got a fresh empty Flags
//                                     map on re-entry and could steal
//                                     a SECOND dying counter-bearing
//                                     creature on the same turn,
//                                     violating the printed cap.
//                                     Moved the gate to controller's
//                                     Seat.Flags so the "once per
//                                     controller's turn" cap survives
//                                     flicker. Value is gs.Turn+1 to
//                                     avoid the 0-vs-unset collision
//                                     on turn 0.
//
// (C) Misleading parser-gap signal — audit channel cleanup:
//   - The Jolly Balloon Man         Dropped the OnETB hook. Its only
//                                     body emitted a per_card_partial
//                                     / parser_gap event saying "haste
//                                     static handled by AST engine;
//                                     per_card stub for registration
//                                     tracking" — but haste IS handled
//                                     by the AST keyword pipeline (no
//                                     per_card gap exists) and
//                                     "registration tracking" is
//                                     already covered by the OnActivated
//                                     binding. Every Balloon Man ETB
//                                     was polluting the parser_gap
//                                     channel with a false signal.

// ---------------------------------------------------------------------------
// Cecil DFC face-name aliases
// ---------------------------------------------------------------------------

func TestCecil_DamageTriggersRegisteredUnderBothFaceNames(t *testing.T) {
	Reset()
	// All three damage triggers should be findable under each face
	// name independently of the DFC composite registration, because
	// the lookup walks face-only first.
	for _, face := range []string{"Cecil, Dark Knight", "Cecil, Redeemed Paladin"} {
		if !hasTriggerForCard(face, "combat_damage_to_player") {
			t.Errorf("%q: combat_damage_to_player handler not registered", face)
		}
		if !hasTriggerForCard(face, "noncombat_damage_to_player") {
			t.Errorf("%q: noncombat_damage_to_player handler not registered", face)
		}
		if !hasTriggerForCard(face, "noncombat_damage_to_creature") {
			t.Errorf("%q: noncombat_damage_to_creature handler not registered", face)
		}
		if !hasTriggerForCard(face, "creature_attacks") {
			t.Errorf("%q: creature_attacks handler not registered (pre-existing alias)", face)
		}
	}
}

// hasTriggerForCard is a thin wrapper that probes the global registry
// for a card-name + event-name pair without exposing the internal
// map. We use the same lookup the dispatcher uses (HasTrigger if
// available, else direct map probe via card lookup).
func hasTriggerForCard(cardName, event string) bool {
	r := Global()
	r.mu.RLock()
	defer r.mu.RUnlock()
	canonical := gameengine.NormalizeEventSingle(event)
	for _, k := range lookupCandidates(cardName) {
		if hs, ok := r.onTrigger[k]; ok {
			if _, has := hs[canonical]; has {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Reaper King — once-per-turn gate flicker-bypass closure
// ---------------------------------------------------------------------------

func TestReaperKing_OncePerTurnGateOnSeatNotPerm(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	reaper := addPerm(gs, 0, "The Reaper, King No More", "creature", "legendary")

	// Stage a dying opponent creature with a -1/-1 counter.
	target := addPerm(gs, 1, "Bear", "creature")
	target.Counters["-1/-1"] = 1
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target.Card)

	// Fire the dies trigger — should steal.
	theReaperDies(gs, reaper, map[string]interface{}{
		"perm":            target,
		"card":            target.Card,
		"controller_seat": 1,
	})

	if gs.Seats[0].Flags["reaper_stole_turn"] != gs.Turn+1 {
		t.Errorf("first steal should stamp seat flag = gs.Turn+1 (=%d), got %d", gs.Turn+1, gs.Seats[0].Flags["reaper_stole_turn"])
	}
}

func TestReaperKing_FlickerCannotBypassOncePerTurnCap(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	reaper := addPerm(gs, 0, "The Reaper, King No More", "creature", "legendary")

	// Pre-stamp the seat flag as if a prior Reaper already stole this
	// turn. A flickered (fresh) Reaper with empty perm.Flags would
	// previously bypass the cap; the seat-level gate closes that hole.
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["reaper_stole_turn"] = gs.Turn + 1

	// Stage a fresh dying target.
	target := addPerm(gs, 1, "Bear", "creature")
	target.Counters["-1/-1"] = 1
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target.Card)
	preGY := len(gs.Seats[1].Graveyard)

	theReaperDies(gs, reaper, map[string]interface{}{
		"perm":            target,
		"card":            target.Card,
		"controller_seat": 1,
	})

	// The card should still be in the opponent's graveyard — Reaper's
	// cap-gate should have blocked the steal.
	if len(gs.Seats[1].Graveyard) != preGY {
		t.Errorf("seat-level cap should have blocked the second steal; opp graveyard size changed %d → %d",
			preGY, len(gs.Seats[1].Graveyard))
	}
}

func TestReaperKing_NewTurnReleasesGate(t *testing.T) {
	gs := newGame(t, 2)
	gs.Turn = 5
	reaper := addPerm(gs, 0, "The Reaper, King No More", "creature", "legendary")

	// Stamp the previous turn's cap.
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["reaper_stole_turn"] = (gs.Turn - 1) + 1 // value matches "turn 4"

	target := addPerm(gs, 1, "Bear", "creature")
	target.Counters["-1/-1"] = 1
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, target.Card)

	theReaperDies(gs, reaper, map[string]interface{}{
		"perm":            target,
		"card":            target.Card,
		"controller_seat": 1,
	})

	// Should have stolen — stale gate from prior turn doesn't block.
	if gs.Seats[0].Flags["reaper_stole_turn"] != gs.Turn+1 {
		t.Errorf("new-turn steal should overwrite stale gate; want %d, got %d",
			gs.Turn+1, gs.Seats[0].Flags["reaper_stole_turn"])
	}
}

// ---------------------------------------------------------------------------
// Jolly Balloon Man — ETB hook dropped (no more false partial signal)
// ---------------------------------------------------------------------------

func TestJollyBalloonMan_ETBHookNoLongerRegistered(t *testing.T) {
	Reset()
	// The only ETB body emitted a misleading parser_gap event. After
	// dropping the hook, no per_card ETB handler should be registered
	// for the card — the Activated handler is the sole binding.
	if HasETB("The Jolly Balloon Man") {
		t.Errorf("The Jolly Balloon Man should no longer have an ETB handler; the previous body emitted a false parser_gap signal")
	}
	if !HasActivated("The Jolly Balloon Man") {
		t.Errorf("Activated handler should remain registered")
	}
}

func TestJollyBalloonMan_NoFalseParserGapOnETB(t *testing.T) {
	Reset()
	gs := newGame(t, 2)
	balloon := addPerm(gs, 0, "The Jolly Balloon Man", "creature", "legendary")

	preGapCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "parser_gap" {
			preGapCount++
		}
	}

	// Invoking the ETB hook should be a no-op now — no per_card_partial
	// or parser_gap events fired.
	gameengine.InvokeETBHook(gs, balloon)

	postGapCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "parser_gap" {
			postGapCount++
		}
	}
	if postGapCount != preGapCount {
		t.Errorf("Balloon Man ETB should not emit any parser_gap events; %d → %d",
			preGapCount, postGapCount)
	}
}
