package tournament

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/per_card"
	"github.com/hexdek/hexdek/internal/hat"
)

// turnEventBudget caps the work a single turn may do, measured in
// events logged (gs.EventsLogged) since the turn started. When the
// budget is exceeded the remaining phases fast-forward to cleanup.
//
// This replaces the pre-r62 wall-clock budget (15s of host time): a
// time-based cutoff made seeded games nondeterministic — a slow CI box
// silently changed game outcomes, voiding Loki's `--games N --seed S`
// repro contract, ELO parity, and replay verification. Events logged
// is a deterministic work-volume proxy: a normal Commander turn logs
// tens-to-hundreds of events, a trigger-storm turn a few thousand
// (maxTriggerFiresPerTurn caps trigger fires at 1000/turn), so 20k
// only trips on genuinely pathological loops the inner caps missed.
const turnEventBudget = 20000

// TakeTurn runs a full turn with no phase hook.
func TakeTurn(gs *gameengine.GameState) { takeTurnImpl(gs, nil) }

// TakeTurnWithHook runs a full turn, calling hook after each phase/step
// boundary completes. The showmatch spectator loop uses this for per-phase
// snapshot broadcasts and pacing delays.
func TakeTurnWithHook(gs *gameengine.GameState, hook func(*gameengine.GameState)) {
	takeTurnImpl(gs, hook)
}

// TurnRunnerForRollout returns a TurnRunnerFunc suitable for injection
// into MCTSHat.TurnRunner.
func TurnRunnerForRollout() func(gs *gameengine.GameState) {
	return func(gs *gameengine.GameState) {
		takeTurnImpl(gs, nil)
		gameengine.StateBasedActions(gs)
	}
}

// takeTurn runs a single player's full turn — beginning / main1 / combat
// / main2 / ending — using the Hat on each seat for decisions.
//
// Mirrors scripts/playloop.py :: take_turn exactly:
//
//   beginning_phase:
//     untap_step(active_seat)   — §502: untap permanents, reset per-turn
//                                 flags, ScanExpiredDurations for
//                                 "until your next turn" effects.
//     upkeep_step               — §503: FirePhaseTriggers("upkeep") +
//                                 FireDelayedTriggers (upkeep).
//     draw_step                 — §504: active draws one (turn 1 skip).
//   main_phase_1                — §505: play land + cast loop.
//   combat_phase                — §506-§511 (already ported).
//   extra_combats               — while PendingExtraCombats > 0: another
//                                 combat phase (Aggravated Assault etc.).
//   main_phase_2                — §505: cast loop only.
//   end_step                    — §513: FirePhaseTriggers("end_step") +
//                                 FireDelayedTriggers (end_of_turn).
//   cleanup_step                — §514: ScanExpiredDurations + CleanupHandSize.
//
// Per-seat "played_land_this_turn" state lives on gs.Flags keyed by
// seat index so concurrent games stay isolated.
func takeTurnImpl(gs *gameengine.GameState, hook func(*gameengine.GameState)) {
	if gs == nil {
		return
	}
	active := gs.Active
	if active < 0 || active >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[active]
	if seat == nil || seat.Lost {
		return
	}

	// Conviction concession — let the hat scoop if it sees no path to winning.
	if seat.Hat != nil && seat.Hat.ShouldConcede(gs, active) {
		gameengine.ConcedeGame(gs, active)
		return
	}

	// Per-turn work budget (deterministic): if any single turn logs
	// more than turnEventBudget events, skip remaining phases. Prevents
	// pathological turns (runaway loops the trigger caps missed) from
	// burning the entire game — and, unlike the old wall-clock budget,
	// trips identically on every host for the same seed.
	turnStartEvents := gs.EventsLogged
	budgetEventEmitted := false
	turnOverBudget := func() bool {
		used := gs.EventsLogged - turnStartEvents
		if used <= turnEventBudget {
			return false
		}
		if !budgetEventEmitted {
			budgetEventEmitted = true
			gs.LogEvent(gameengine.Event{
				Kind: "turn_budget_exceeded",
				Seat: active,
				Details: map[string]interface{}{
					"turn":          gs.Turn,
					"phase":         gs.Phase,
					"step":          gs.Step,
					"events_logged": used,
					"budget":        turnEventBudget,
				},
			})
		}
		return true
	}

	gs.LogEvent(gameengine.Event{
		Kind: "turn_start",
		Seat: active,
		Details: map[string]interface{}{
			"turn": gs.Turn,
			"rule": "500.1",
		},
	})

	// Reset per-turn counters.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_trigger_fires_this_turn"] = 0
	for i := range gs.Seats {
		gs.Flags["draws_this_turn_seat_"+strconv.Itoa(i)] = 0
	}

	// turnEndingNow is a local helper that checks + consumes the
	// "turn_ending_now" flag set by Sundial of the Infinite (or any
	// "end the turn" effect per CR §712.5). When true, the turn loop
	// must skip remaining phases and jump directly to cleanup.
	turnEndingNow := func() bool {
		if gs.Flags != nil && gs.Flags["turn_ending_now"] > 0 {
			return true
		}
		return false
	}
	// fastForwardCleanup runs the cleanup step when "end the turn"
	// fires mid-turn. CR §712.5b: "any remaining phases/steps are
	// skipped; the cleanup step happens immediately."
	fastForwardCleanup := func() {
		// Consume the flag so it doesn't fire again.
		delete(gs.Flags, "turn_ending_now")
		gs.LogEvent(gameengine.Event{
			Kind: "turn_ending_fast_forward",
			Seat: active,
			Details: map[string]interface{}{
				"rule": "712.5b",
			},
		})
		// §514.1 discard to hand size.
		gs.Phase, gs.Step = "ending", "cleanup"
		gameengine.CleanupHandSize(gs, active, 7)
		// §514.2 expirations.
		gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
		gs.InvalidateCharacteristicsCache()
		gameengine.StateBasedActions(gs)
		gs.Snapshot()
	}

	// CR §730.2a — day/night transition BEFORE untap.
	gameengine.EvaluateDayNightAtTurnStart(gs)

	// =========================================================
	// BEGINNING PHASE (§500-§504)
	// =========================================================

	{
	// §502 Untap step.
	gs.Phase, gs.Step = "beginning", "untap"
	gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
	gs.InvalidateCharacteristicsCache()
	gameengine.FireDelayedTriggers(gs, gs.Phase, gs.Step)
	gameengine.UntapAll(gs, active)
	// §502 trigger: cards like Rasputin Dreamweaver and Seedborn Muse
	// listen for the untap_step event to snapshot or react to untap state.
	gameengine.FireCardTrigger(gs, "untap_step", map[string]interface{}{
		"active_seat": active,
	})
	// Per-turn bookkeeping: drain mana pool, reset lands-played.
	//
	// Mana drains via the canonical DrainAllPools so the CR §106.4a
	// exemption set (Upwelling "any" / Omnath-style color retention /
	// Cabal Coffers-style etc.) is honored. The previous direct
	// `seat.ManaPool = 0 ; seat.Mana.Clear()` zeroed unconditionally,
	// wiping mana that Upwelling explicitly says should persist across
	// the cleanup → untap boundary. The drain is logically the §106.4
	// firing on the PRIOR turn's cleanup step ending (cleanup is the
	// last step of the ending phase; per CR §514 it runs to completion
	// without a priority window, so no DrainAllPools call lives there —
	// this start-of-untap call is the canonical place to fire it).
	gameengine.DrainAllPools(gs, "ending", "cleanup")
	clearPlayedLand(gs, active)
	gs.PendingExtraCombats = nil
	gs.CurrentCombatRestriction = ""
	// §702.136 Raid — clear the attacked_this_turn flag from previous turn.
	if seat.Flags != nil {
		delete(seat.Flags, "attacked_this_turn")
	}
	// Snapshot life for end-step "life lost this turn" checks (Book of Vile Darkness).
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["life_at_turn_start"] = seat.Life
	// CR §700.4 / §702.40 storm cast-count reset. Global counter wipes
	// at every untap. Active seat's per-seat counter snapshots into
	// SpellsCastLastTurn then zeros. Non-active seats keep accumulating
	// (an instant they cast during an opponent's turn still counts
	// toward their next Storm window until their own next untap).
	gs.SpellsCastThisTurn = 0
	seat.SpellsCastLastTurn = seat.SpellsCastThisTurn
	seat.SpellsCastThisTurn = 0
	if hook != nil { hook(gs) }

	// §503 Upkeep.
	gs.Phase, gs.Step = "beginning", "upkeep"
	gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
	gs.InvalidateCharacteristicsCache()
	gameengine.FireDelayedTriggers(gs, gs.Phase, gs.Step)
	gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
	gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
		"active_seat": active,
	})
	// §503 opponent upkeep: cards like Slicer, Hired Muscle trigger on
	// each opponent's upkeep (i.e. the upkeep of any non-active player).
	for _, opp := range gs.Opponents(active) {
		gameengine.FireCardTrigger(gs, "upkeep_opponent", map[string]interface{}{
			"seat": opp,
		})
	}
	gameengine.StateBasedActions(gs)
	// Drain the stack: resolve any triggered abilities pushed during
	// upkeep (e.g., Mystic Remora, Smothering Tithe, Rhystic Study).
	// Per CR §503.1, players get priority during the upkeep step before
	// moving to the draw step.
	drainStack(gs)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	// §503.1 priority window: after upkeep triggers resolve, the active
	// player may activate instant-speed abilities and cast instants.
	// This is where Braid of Fire mana gets spent, Necropotence draws
	// happen, and flash creatures enter.
	runInstantPriority(gs, active)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}
	if hook != nil { hook(gs) }

	// §504 Draw — first active player does not draw on turn 1.
	gs.Phase, gs.Step = "beginning", "draw"
	if gs.Turn > 1 || active != firstActive(gs) {
		if gameengine.NecropotenceSkipsDraw(gs, active) {
			gs.LogEvent(gameengine.Event{
				Kind: "skip_draw", Seat: active,
				Source: "Necropotence",
				Details: map[string]interface{}{"rule": "504.1"},
			})
		} else {
			drawTop(gs, active)
		}
	}
	gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
	gameengine.FireCardTrigger(gs, "draw_step_controller", map[string]interface{}{
		"active_seat": active,
	})
	gameengine.StateBasedActions(gs)
	// Drain triggers from draw step (e.g., Orcish Bowmasters).
	drainStack(gs)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	// §504.1 priority window: players get priority after the draw and
	// after draw-step triggers resolve. Instant-speed actions before
	// moving to main phase (e.g., Brainstorm in response to draw trigger,
	// flash creatures, Teferi's Protection before main).
	runInstantPriority(gs, active)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}
	if hook != nil { hook(gs) }
	}

	// =========================================================
	// MAIN PHASE 1 (§505)
	// =========================================================
	if turnOverBudget() {
		fastForwardCleanup()
		return
	}
	gs.Phase, gs.Step = "main", "precombat_main"
	// Rad counter trigger fires at the beginning of precombat main phase.
	gameengine.FireRadCounterTriggers(gs)
	// CR §714.2b: "As your precombat main phase begins, you put a lore
	// counter on each Saga you control." Saga chapter abilities fire via
	// the lore_counter_added trigger dispatched inside AdvanceSagaChapter;
	// SBA §704.5s sacrifices the saga once the lore total reaches its
	// final chapter (handled on the next StateBasedActions pass below).
	gameengine.TickSagaChapters(gs, active)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	// Paradigm — cast free copies of paradigm-exiled cards.
	gameengine.ResolveParadigmCopies(gs, active)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	runMainPhase(gs, active, true)
	gameengine.StateBasedActions(gs)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}
	if hook != nil { hook(gs) }

	// =========================================================
	// COMBAT PHASE (§506-§511)
	// =========================================================
	if turnOverBudget() {
		fastForwardCleanup()
		return
	}
	runCombatWithExtras(gs, active)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}

	// Obeka, Splitter of Seconds: extra upkeep steps after combat.
	if gs.Flags != nil {
		if extra := gs.Flags["obeka_extra_upkeeps"]; extra > 0 {
			gs.Flags["obeka_extra_upkeeps"] = 0
			for i := 0; i < extra && !gs.CheckEnd(); i++ {
				gs.Phase, gs.Step = "beginning", "upkeep"
				gs.LogEvent(gameengine.Event{
					Kind: "extra_upkeep", Seat: active,
					Details: map[string]interface{}{
						"source": "Obeka, Splitter of Seconds",
						"index":  i + 1,
						"total":  extra,
					},
				})
				gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
				gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
					"active_seat": active,
				})
				gameengine.StateBasedActions(gs)
				drainStack(gs)
			}
			if gs.CheckEnd() || seat.Lost {
				return
			}
		}
	}
	if hook != nil { hook(gs) }

	// =========================================================
	// MAIN PHASE 2 (§505)
	// =========================================================
	if turnOverBudget() {
		fastForwardCleanup()
		return
	}
	gs.Phase, gs.Step = "main", "postcombat_main"
	gameengine.FireCardTrigger(gs, "postcombat_main_controller", map[string]interface{}{
		"active_seat": active,
	})
	gameengine.StateBasedActions(gs)
	drainStack(gs)
	if gs.CheckEnd() || seat.Lost {
		return
	}
	runMainPhase(gs, active, false)
	gameengine.StateBasedActions(gs)
	if gs.CheckEnd() {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}
	if hook != nil { hook(gs) }

	// Sphinx / Shadow of the Second Sun: extra beginning phase after
	// postcombat main. Untap, upkeep, draw — no extra main phase.
	if per_card.CheckSecondSunExtraPhase(gs, active) {
		gs.LogEvent(gameengine.Event{
			Kind: "extra_beginning_phase",
			Seat: active,
			Details: map[string]interface{}{
				"reason": "sphinx_shadow_second_sun",
			},
		})
		gs.Phase, gs.Step = "beginning", "untap"
		gameengine.UntapAll(gs, active)
		// Mirror the §106.4a-aware drain used by the primary untap path
		// above (Upwelling / Omnath-color-retention / per-color exemption
		// cards). The Sphinx of the Second Sun extra-beginning-phase
		// shape was previously unconditionally zeroing, defeating the
		// same exemption set in the rarer extra-untap window.
		gameengine.DrainAllPools(gs, "ending", "cleanup")
		tapAllManaSources(gs, seat)

		gs.Phase, gs.Step = "beginning", "upkeep"
		gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
		gameengine.FireCardTrigger(gs, "upkeep_controller", map[string]interface{}{
			"active_seat": active,
		})
		gameengine.StateBasedActions(gs)
		drainStack(gs)
		if gs.CheckEnd() || seat.Lost {
			return
		}
		runInstantPriority(gs, active)
		if gs.CheckEnd() || seat.Lost {
			return
		}

		gs.Phase, gs.Step = "beginning", "draw"
		drawTop(gs, active)
		gameengine.StateBasedActions(gs)
		drainStack(gs)
		if gs.CheckEnd() || seat.Lost {
			return
		}
	}

	// =========================================================
	// ENDING PHASE (§513-§514)
	// =========================================================

	// §513 End step.
	gs.Phase, gs.Step = "ending", "end"
	gameengine.FireDelayedTriggers(gs, gs.Phase, gs.Step)
	gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
	gameengine.FireCardTrigger(gs, "end_step", map[string]interface{}{
		"active_seat": active,
	})
	// §721.3 — Monarch draws a card at end step.
	gameengine.FireMonarchEndStep(gs)
	// Drain mana pools (§500.4 / §513 catch-all). Use DrainAllPools
	// instead of raw zeroing so pool_drain events are emitted.
	gameengine.DrainAllPools(gs, gs.Phase, gs.Step)
	gameengine.StateBasedActions(gs)
	// Drain triggered abilities from end step.
	drainStack(gs)
	if gs.CheckEnd() {
		return
	}
	// §513.1 priority window: after end-step triggers resolve, players
	// get priority. Flash creatures, instant-speed removal, Restoration
	// Angel, Teferi's Protection, "at end of turn" plays all happen here.
	runInstantPriority(gs, active)
	if gs.CheckEnd() {
		return
	}
	if turnEndingNow() {
		fastForwardCleanup()
		return
	}
	if hook != nil { hook(gs) }

	// §514 Cleanup step with §514.3a looping.
	// CR §514.3a: "If any state-based actions are performed as a result
	// of a step [514.1-514.2], OR if any triggered abilities are waiting
	// to be put on the stack, players receive priority. Once the stack
	// is empty and all players pass in succession, another cleanup step
	// begins."
	//
	// Both disjuncts matter. The prior implementation only re-looped on
	// sbaChanged, silently skipping the trigger arm — so when a §514.1
	// discard fired a "card_discarded" trigger (Madness, Megrim, Mayhem,
	// Containment Priest-style "you may cast" cleanup-step interactions)
	// the trigger landed on the stack but the cleanup step ended without
	// granting priority. The Madness "may cast for madness cost" choice
	// requires a priority window after the trigger resolves; without it
	// the cast opportunity is silently lost. The invariants.go cleanup
	// comment (§379-383) was already documenting this contract — the
	// engine just wasn't honoring it.
	const maxCleanupLoops = 8 // safety cap
	for cleanupLoop := 0; cleanupLoop < maxCleanupLoops; cleanupLoop++ {
		gs.Phase, gs.Step = "ending", "cleanup"
		preHand := len(seat.Hand)
		// §514.1 discard to hand size.
		gameengine.CleanupHandSize(gs, active, 7)
		// §514.2 expirations — clears until-EOT continuous effects, mods, damage.
		gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
		gs.InvalidateCharacteristicsCache() // ensure SBAs see post-expiry P/T
		sbaChanged := gameengine.StateBasedActions(gs)
		// §514.3a "triggered abilities" arm: we approximate "any trigger
		// fired during the step" by (a) discards actually happened in
		// §514.1 (every common cleanup-step trigger — Madness, Megrim,
		// Mayhem, etc. — keys off card_discarded), and (b) the stack is
		// non-empty (a triggered ability is waiting to resolve). Either
		// condition obligates priority + another cleanup pass per the rule.
		discardsHappened := len(seat.Hand) < preHand
		triggersWaiting := len(gs.Stack) > 0
		if !sbaChanged && !discardsHappened && !triggersWaiting {
			break // no SBAs, no discards, no waiting triggers — cleanup done
		}
		reason := "sba"
		switch {
		case triggersWaiting:
			reason = "triggers_waiting"
		case discardsHappened:
			reason = "discard_triggers"
		}
		gs.LogEvent(gameengine.Event{
			Kind: "cleanup_loop",
			Seat: active,
			Details: map[string]interface{}{
				"iteration": cleanupLoop + 1,
				"rule":      "514.3a",
				"reason":    reason,
			},
		})
		gameengine.PriorityRound(gs)
		if gs.CheckEnd() {
			return
		}
	}
	if hook != nil { hook(gs) }

	// Release Mindslaver control at end of turn. CR §712.6: "The effect
	// of controlling another player's turn expires at the end of that
	// turn."
	if seat.ControlledBy >= 0 {
		gs.LogEvent(gameengine.Event{
			Kind:   "mindslaver_control_end",
			Seat:   seat.ControlledBy,
			Target: active,
			Details: map[string]interface{}{
				"rule": "712.6",
			},
		})
		seat.ControlledBy = -1
	}

	// Emit full game-state snapshot at turn end. Mirrors Python's
	// game.snapshot() call at cleanup.
	gs.Snapshot()
}

// runCombatWithExtras runs combat_phase, then repeats for any pending
// extra combats queued by resolving spells/abilities (Aggravated Assault,
// Seize the Day, Moraug). CR §500.5.
func runCombatWithExtras(gs *gameengine.GameState, active int) {
	gs.Phase, gs.Step = "combat", "beginning_of_combat"
	gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
	// FIX 3: Offer combat-timing activated abilities (pump, sacrifice
	// outlets, etc.) before the full combat phase resolves.
	runCombatActivations(gs, active)
	if gs.CheckEnd() {
		return
	}
	gameengine.CombatPhase(gs)
	gameengine.StateBasedActions(gs)
	// Fire any end-of-combat delayed triggers registered during combat.
	gs.Phase, gs.Step = "combat", "end_of_combat"
	gameengine.FireDelayedTriggers(gs, gs.Phase, gs.Step)
	// Drain triggered abilities from end-of-combat step.
	drainStack(gs)
	// Extra combats loop. Pop the front of the queue, apply that
	// combat's restriction + OnBegin hook, then run a normal combat
	// phase. Entries may be added to the queue MID-LOOP (e.g. Moraug
	// landfall during the extra combat's main-equivalent steps creates
	// more entries), and the append-tail / pop-head pattern handles
	// that naturally — new entries get drained in subsequent iterations.
	for len(gs.PendingExtraCombats) > 0 && !gs.CheckEnd() {
		current := gs.PendingExtraCombats[0]
		gs.PendingExtraCombats = gs.PendingExtraCombats[1:]
		gs.CurrentCombatRestriction = current.Restriction
		gs.Phase, gs.Step = "combat", "beginning_of_combat"
		// Per-combat OnBegin hook (e.g. Moraug's "untap all creatures"
		// or Bumi Unleashed's "untap all lands") fires here, BEFORE
		// the beginning_of_combat trigger fires so attackers can be
		// untapped before declare-attackers reads their tap state.
		if current.OnBegin != nil {
			current.OnBegin(gs)
		}
		gameengine.FirePhaseTriggers(gs, gs.Phase, gs.Step)
		runCombatActivations(gs, active)
		if gs.CheckEnd() {
			return
		}
		gameengine.CombatPhase(gs)
		gameengine.StateBasedActions(gs)
		gs.Phase, gs.Step = "combat", "end_of_combat"
		gameengine.FireDelayedTriggers(gs, gs.Phase, gs.Step)
		drainStack(gs)
		gs.CurrentCombatRestriction = ""
	}
}

// firstActive returns the seat that was active at game start.
func firstActive(gs *gameengine.GameState) int {
	for _, ev := range gs.EventLog {
		if ev.Kind == "game_start" {
			return ev.Seat
		}
	}
	return -1
}

// RunLondonMulligan implements the London mulligan procedure (CR §103.5)
// for a single seat. Call before the first turn begins.
//
// Procedure:
//  1. Draw 7 cards.
//  2. Hat decides keep or mulligan via ChooseMulligan.
//  3. If mulligan: shuffle hand into library, draw 7 again, increment
//     mulligan count.
//  4. Repeat until keep or hand size = 0.
//  5. On keep: put N cards from hand on bottom of library (N = number
//     of mulligans taken). Hat picks which N cards via ChooseBottomCards.
func RunLondonMulligan(gs *gameengine.GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}

	mulligansTaken := 0
	const maxMulligans = 7 // can't mulligan more times than cards

	for mulligansTaken < maxMulligans {
		// Draw 7.
		drawN(gs, seatIdx, 7)

		if len(seat.Hand) == 0 {
			break
		}

		// Hat decides.
		keep := true
		if seat.Hat != nil {
			keep = !seat.Hat.ChooseMulligan(gs, seatIdx, seat.Hand)
		}

		if keep {
			break
		}

		// Mulligan: shuffle hand into library, draw 7 again.
		mulligansTaken++
		gs.LogEvent(gameengine.Event{
			Kind:   "mulligan",
			Seat:   seatIdx,
			Amount: mulligansTaken,
			Details: map[string]interface{}{
				"rule":      "103.5",
				"hand_size": len(seat.Hand),
			},
		})

		// Put hand back into library.
		seat.Library = append(seat.Library, seat.Hand...)
		seat.Hand = seat.Hand[:0]

		// Shuffle library.
		if gs.Rng != nil {
			gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
				seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
			})
		}
	}

	// §103.5: put N cards on bottom (N = mulligansTaken).
	if mulligansTaken > 0 && len(seat.Hand) > 0 {
		bottomCount := mulligansTaken
		if bottomCount > len(seat.Hand) {
			bottomCount = len(seat.Hand)
		}

		var toBottom []*gameengine.Card
		if seat.Hat != nil {
			toBottom = seat.Hat.ChooseBottomCards(gs, seatIdx, seat.Hand, bottomCount)
		}
		if len(toBottom) != bottomCount {
			// Fallback: bottom the last N cards.
			if bottomCount <= len(seat.Hand) {
				toBottom = make([]*gameengine.Card, bottomCount)
				copy(toBottom, seat.Hand[len(seat.Hand)-bottomCount:])
			}
		}

		// Remove chosen cards from hand and put on bottom.
		for _, c := range toBottom {
			for i, h := range seat.Hand {
				if h == c {
					seat.Hand = append(seat.Hand[:i], seat.Hand[i+1:]...)
					break
				}
			}
			seat.Library = append(seat.Library, c)
		}

		gs.LogEvent(gameengine.Event{
			Kind:   "mulligan_bottom",
			Seat:   seatIdx,
			Amount: bottomCount,
			Details: map[string]interface{}{
				"rule":            "103.5",
				"mulligans_taken": mulligansTaken,
				"final_hand_size": len(seat.Hand),
			},
		})
	}

	recordMulliganHistory(gs, seatIdx, mulligansTaken)
}

// recordMulliganHistory stamps the final post-bottom opening hand and
// mulligan count onto gs.MulliganHistory[seatIdx]. Sizes the slice
// lazily to len(gs.Seats) on first write so out-of-order seat
// resolution (rare but valid) doesn't drop earlier seats.
func recordMulliganHistory(gs *gameengine.GameState, seatIdx, mulligansTaken int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if len(gs.MulliganHistory) < len(gs.Seats) {
		grown := make([]gameengine.SeatMulliganStats, len(gs.Seats))
		copy(grown, gs.MulliganHistory)
		gs.MulliganHistory = grown
	}
	seat := gs.Seats[seatIdx]
	hand := make([]gameengine.MulliganHandEntry, 0, len(seat.Hand))
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		entry := gameengine.MulliganHandEntry{
			Name: c.DisplayName(),
		}
		if len(c.Types) > 0 {
			entry.Types = make([]string, len(c.Types))
			copy(entry.Types, c.Types)
		}
		hand = append(hand, entry)
	}
	gs.MulliganHistory[seatIdx] = gameengine.SeatMulliganStats{
		MulligansTaken: mulligansTaken,
		OpeningHand:    hand,
	}
}

// drawN draws N cards from the top of the library into hand.
func drawN(gs *gameengine.GameState, seatIdx int, n int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	for i := 0; i < n; i++ {
		if len(seat.Library) == 0 {
			break
		}
		c := seat.Library[0]
		seat.Library = seat.Library[1:]
		seat.Hand = append(seat.Hand, c)
	}
}

// drawTop pulls one card from the top of seat's library into its hand.
func drawTop(gs *gameengine.GameState, seatIdx int) {
	// Narset: opponents can't draw more than one card each turn.
	if gameengine.NarsetBlocksDraw(gs, seatIdx) {
		return
	}
	s := gs.Seats[seatIdx]
	if len(s.Library) == 0 {
		s.AttemptedEmptyDraw = true
		return
	}
	c := s.Library[0]
	s.Library = s.Library[1:]
	s.Hand = append(s.Hand, c)
	gameengine.IncrementDrawCount(gs, seatIdx)
	gs.LogEvent(gameengine.Event{
		Kind:   "draw",
		Seat:   seatIdx,
		Source: c.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"rule":      "504.1",
			"hand_size": len(s.Hand),
		},
	})
	// Fire draw-trigger observers (Smothering Tithe, Orcish Bowmasters).
	// Set the suppress-first-draw-step flag so Bowmasters skips the
	// normal draw-step draw (CR §614.6).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["_suppress_first_draw_trigger_seat"] = seatIdx + 1
	gameengine.FireDrawTriggerObservers(gs, seatIdx, 1, false)
}

// runMainPhase plays a land (pre-combat only) + adds mana + casts via
// the Hat.
func runMainPhase(gs *gameengine.GameState, seatIdx int, precombat bool) {
	seat := gs.Seats[seatIdx]

	if precombat && !playedLandThisTurn(gs, seatIdx) {
		tryPlayLand(gs, seatIdx)
	}

	// Tap all mana sources (lands + artifacts) for mana.
	tapAllManaSources(gs, seat)

	if gs.CommanderFormat && seat.Hat != nil {
		tryCastCommander(gs, seatIdx)
	}

	// Cast loop, bounded to avoid infinite loops on pathological hats.
	// After each successful cast, re-tap any NEW mana sources that may
	// have entered the battlefield (e.g. a ramp spell that fetches a land
	// or an ETB artifact like Sol Ring from a tutor).
	//
	// When a cast attempt fails, we track the failed card and rebuild
	// the castable list rather than aborting the loop -- a failed Sol
	// Ring (wrong timing) shouldn't prevent casting a creature.
	var lastFailed *gameengine.Card
	for attempt := 0; attempt < 20; attempt++ {
		castable := buildCastableList(gs, seatIdx)
		if len(castable) == 0 {
			break
		}
		var chosen *gameengine.Card
		if seat.Hat != nil {
			chosen = seat.Hat.ChooseCastFromHand(gs, seatIdx, castable)
		}
		if chosen == nil {
			break
		}
		// Infinite-loop guard: if the hat keeps choosing the same failing
		// card, break out.
		if chosen == lastFailed {
			break
		}
		before := len(seat.Hand)
		err := gameengine.CastSpell(gs, seatIdx, chosen, nil)
		if err != nil || len(seat.Hand) == before {
			lastFailed = chosen
			continue // try again with a different card
		}
		lastFailed = nil // reset on success
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			return
		}
		// Re-tap any new mana sources that ETB'd from the spell.
		tapAllManaSources(gs, seat)

		// High-urgency commander bias: for decks where the commander IS the
		// gameplan (Voltron, combo commanders, tribal lords), retry the
		// commander after every successful in-loop cast. A ramp spell may
		// have just added the missing mana, and waiting until the post-loop
		// retry means we burn the rest of the cast budget on filler the
		// deck doesn't need until the commander is on the battlefield.
		if gs.CommanderFormat && len(seat.CommandZone) > 0 {
			if highUrgencyCommanderInZone(seat) {
				tryCastCommander(gs, seatIdx)
				if gs.CheckEnd() {
					return
				}
			}
		}
	}

	// Retry commander cast after the cast loop — ramp spells may have
	// added mana sources that weren't available on the first attempt.
	// Catches the low-urgency case (single post-loop retry) and serves as
	// a backstop for high-urgency commanders if the in-loop retries above
	// all failed (e.g. spells were cast but no ramp was among them).
	if gs.CommanderFormat && seat.Hat != nil && len(seat.CommandZone) > 0 {
		tapAllManaSources(gs, seat)
		tryCastCommander(gs, seatIdx)
	}

	// --- Activated ability loop (FIX 1) ---
	// After casting, offer activated abilities to the Hat. Capped to
	// prevent infinite loops. Mana abilities are excluded (they resolve
	// inline via tapAllManaSources). Sacrifice-heavy boards get higher
	// caps since aristocrat strategies need multiple activations per turn.
	maxMainPhaseActivations := 5
	maxActivationsPerPerm := 2
	creatureCount := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			creatureCount++
		}
	}
	if creatureCount >= 3 {
		maxMainPhaseActivations = creatureCount + 2
		if maxMainPhaseActivations > 12 {
			maxMainPhaseActivations = 12
		}
		maxActivationsPerPerm = 3
	}
	permActCount := map[*gameengine.Permanent]int{}
	for actCount := 0; actCount < maxMainPhaseActivations; actCount++ {
		options := buildActivationOptions(gs, seatIdx, "main")
		// Filter out permanents that hit their per-turn cap.
		filtered := options[:0]
		for _, o := range options {
			if permActCount[o.Permanent] < maxActivationsPerPerm {
				filtered = append(filtered, o)
			}
		}
		if len(filtered) == 0 {
			break
		}
		chosen := seat.Hat.ChooseActivation(gs, seatIdx, filtered)
		if chosen == nil {
			break
		}
		err := gameengine.ActivateAbility(gs, seatIdx, chosen.Permanent, chosen.Ability, nil)
		if err != nil {
			break
		}
		permActCount[chosen.Permanent]++
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			return
		}
		// Re-tap new mana sources that may have appeared from ability resolution.
		tapAllManaSources(gs, seat)
	}

	// --- Equipment equip loop ---
	// Attach unequipped equipment to the best creature on the battlefield.
	tryEquipAll(gs, seatIdx)
}

// tryEquipAll scans the seat's battlefield for unattached equipment and
// equips each to the best creature available, paying the equip cost.
func tryEquipAll(gs *gameengine.GameState, seatIdx int) {
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	for _, equip := range seat.Battlefield {
		if equip == nil || !equip.IsEquipment() || equip.AttachedTo != nil {
			continue
		}
		cost := gameengine.EquipCost(equip.Card)
		gameengine.EnsureTypedPool(seat)
		if seat.Mana.Total() < cost {
			continue
		}
		var bestTarget *gameengine.Permanent
		bestScore := -1
		equipOT := ""
		if equip.Card != nil {
			equipOT = gameengine.OracleTextLower(equip.Card)
		}
		hasDeathTrigger := strings.Contains(equipOT, "equipped creature dies") ||
			strings.Contains(equipOT, "whenever equipped creature dies")
		hasConnectTrigger := strings.Contains(equipOT, "deals combat damage")
		hasIndestructible := strings.Contains(equipOT, "indestructible")

		for _, p := range seat.Battlefield {
			if p == nil || p == equip || !p.IsCreature() || p.Controller != seatIdx {
				continue
			}
			score := gs.PowerOf(p)*2 + gs.ToughnessOf(p)

			isCommander := gs.CommanderFormat && gameengine.IsCommanderCard(gs, seatIdx, p.Card)
			if isCommander {
				score += 15
				if hasIndestructible {
					score += 10
				}
				if hasDeathTrigger {
					score += 15
				}
			} else if hasDeathTrigger {
				score += 5
			}

			hasEvasion := p.HasKeyword("flying") || p.HasKeyword("trample") ||
				p.HasKeyword("menace") || p.HasKeyword("fear") ||
				p.HasKeyword("intimidate") || p.HasKeyword("shadow") ||
				p.HasKeyword("skulk") || p.HasKeyword("horsemanship")
			if p.Card != nil {
				pot := gameengine.OracleTextLower(p.Card)
				if strings.Contains(pot, "can't be blocked") {
					hasEvasion = true
				}
			}
			if hasEvasion {
				score += 8
				if hasConnectTrigger {
					score += 12
				}
			}

			attachedCount := 0
			for _, bp := range seat.Battlefield {
				if bp != nil && bp.IsEquipment() && bp.AttachedTo == p && bp != equip {
					attachedCount++
				}
			}
			if attachedCount > 0 {
				score += attachedCount * 4
			}

			if !p.SummoningSick {
				score += 3
			}
			if score > bestScore {
				bestScore = score
				bestTarget = p
			}
		}
		if bestTarget != nil {
			gameengine.ActivateEquip(gs, seatIdx, equip, bestTarget)
		}
	}
}

// tapAllManaSources taps every untapped land and mana artifact on the
// seat's battlefield, crediting the appropriate mana pools.
//
// Lands: recognized basic land subtypes (Plains/Island/Swamp/Mountain/
// Forest) use the typed ColoredManaPool via AddMana. Utility/colorless
// lands without a recognized subtype also use AddMana with "any" color.
//
// Artifacts: delegates to ApplyArtifactMana for each untapped, non-
// destructive-cost artifact (skips Lion's Eye Diamond). This ensures
// Sol Ring, Mana Crypt, Arcane Signet, Signets, Talismans, and all
// other mana rocks contribute to the available pool.
func tapAllManaSources(gs *gameengine.GameState, seat *gameengine.Seat) {
	// Pass 1: Tap lands.
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsLand() || p.Tapped {
			continue
		}
		p.Tapped = true
		color := landSubtypeColor(p.Card)
		if color != "" {
			gameengine.AddMana(gs, seat, color, 1, p.Card.DisplayName())
		} else {
			// Utility/colorless lands: use AddMana with "any" so the mana
			// flows through the typed pool (avoids legacy ManaPool drift).
			gameengine.AddMana(gs, seat, "any", 1, p.Card.DisplayName())
		}
	}
	// Pass 2: Tap mana artifacts. Work on a snapshot of the battlefield
	// slice to tolerate sacrifice-as-cost artifacts (Treasure, Lotus
	// Petal) that remove themselves during ApplyArtifactMana.
	snapshot := make([]*gameengine.Permanent, len(seat.Battlefield))
	copy(snapshot, seat.Battlefield)
	for _, p := range snapshot {
		if p == nil || p.Tapped {
			continue
		}
		if !gameengine.IsArtifactOnly(p) {
			continue
		}
		if gameengine.ArtifactHasDestructiveCost(p) {
			continue
		}
		gameengine.ApplyArtifactMana(gs, seat, p)
	}
}

// buildCastableList returns the subset of seat's hand that's affordable
// at current mana AND has legal targets (if the spell requires them).
// Filters to non-land cards only. Uses CalculateTotalCost to account for
// battlefield cost modifiers (Thalia, Trinisphere, etc.).
func buildCastableList(gs *gameengine.GameState, seatIdx int) []*gameengine.Card {
	seat := gs.Seats[seatIdx]
	if len(seat.Hand) == 0 {
		return nil
	}
	// Ensure the typed pool is initialized and bridges any legacy
	// ManaPool integer mana. After this call seat.Mana.Total() is
	// authoritative (it includes any legacy ManaPool delta). We use
	// Mana.Total() as the single source of truth to avoid the double-
	// counting bug (seat.ManaPool + seat.Mana.Total() would count
	// typed mana twice because AddMana already syncs ManaPool to Total).
	gameengine.EnsureTypedPool(seat)
	availableMana := seat.Mana.Total()

	out := make([]*gameengine.Card, 0, len(seat.Hand))
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if isLand(c) {
			continue
		}
		// CR §202.1a: cards with no mana cost cannot be cast from hand
		// via normal casting. Suspend-only cards (Profane Tutor, Ancestral
		// Vision) have CMC=0 but should only be cast via their suspend
		// triggered ability, not from hand.
		if hasNoManaCost(c) {
			// MDFC: front face may be unccastable (no mana cost / suspend)
			// but back face is a normal spell. Try back face.
			if c.IsMDFC() && c.BackFaceCMC > 0 {
				c.CastingBackFace = true
				backCost := gameengine.CalculateTotalCost(gs, c, seatIdx)
				c.CastingBackFace = false
				if backCost <= availableMana {
					c.CastingBackFace = true
					out = append(out, c)
					continue
				}
			}
			continue
		}
		c.CastingBackFace = false
		cost := gameengine.CalculateTotalCost(gs, c, seatIdx)
		if cost > availableMana {
			// Front face too expensive — try back face if MDFC.
			if c.IsMDFC() && c.BackFaceCMC > 0 {
				c.CastingBackFace = true
				backCost := gameengine.CalculateTotalCost(gs, c, seatIdx)
				c.CastingBackFace = false
				if backCost <= availableMana {
					c.CastingBackFace = true
					out = append(out, c)
					continue
				}
			}
			continue
		}
		// Front face is affordable. For MDFCs, also check if back face is
		// affordable AND strategically preferable (non-creature back faces
		// like enchantments/sorceries are typically the "real" spell).
		if c.IsMDFC() && c.BackFaceCMC > 0 && c.BackFaceCMC <= availableMana {
			if mdfcPreferBackFace(c) {
				c.CastingBackFace = true
			}
		}
		// Target legality gate: counterspells require a spell on the stack
		// controlled by an opponent. During main phase (stack empty), they
		// can't be cast.
		if gameengine.CardHasCounterSpell(c) {
			if !hasCounterableTarget(gs, seatIdx) {
				continue
			}
		}
		// Targeted removal: if the spell has a Destroy/Exile/Bounce effect
		// targeting a creature/permanent, verify at least one legal target
		// exists on an opponent's battlefield.
		if needsTargetCreature(c) && !hasTargetCreature(gs, seatIdx) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// hasCounterableTarget returns true if the stack contains a spell
// controlled by an opponent of seatIdx.
func hasCounterableTarget(gs *gameengine.GameState, seatIdx int) bool {
	for _, si := range gs.Stack {
		if si != nil && !si.Countered && si.Controller != seatIdx {
			return true
		}
	}
	return false
}

// needsTargetCreature returns true if the card has a targeted
// Destroy/Exile effect that requires a creature on an opponent's
// battlefield. We detect this via the per_card registry (cards with
// OnResolve handlers that target creatures) or via AST spell effects.
func needsTargetCreature(c *gameengine.Card) bool {
	if c == nil || c.AST == nil {
		return false
	}
	for _, ab := range c.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a.Effect == nil {
			continue
		}
		if needsCreatureTarget(a.Effect) {
			return true
		}
	}
	return false
}

// needsCreatureTarget walks an effect tree looking for Destroy/Exile/Fight
// effects that target creatures specifically.
func needsCreatureTarget(e gameast.Effect) bool {
	if e == nil {
		return false
	}
	switch eff := e.(type) {
	case *gameast.Destroy:
		return filterTargetsCreature(eff.Target)
	case *gameast.Exile:
		return filterTargetsCreature(eff.Target)
	case *gameast.Fight:
		return true
	case *gameast.Sequence:
		for _, sub := range eff.Items {
			if needsCreatureTarget(sub) {
				return true
			}
		}
	}
	return false
}

// filterTargetsCreature returns true if a filter specifies "creature"
// as its base type (targeted creature removal).
func filterTargetsCreature(f gameast.Filter) bool {
	if f.Base == "" {
		return false
	}
	base := f.Base
	return base == "creature" || base == "target_creature"
}

// hasTargetCreature returns true if any opponent controls at least one
// creature on the battlefield.
func hasTargetCreature(gs *gameengine.GameState, seatIdx int) bool {
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.IsCreature() {
				return true
			}
		}
	}
	return false
}

// tryPlayLand asks the Hat for a land; if it provides one that's in
// hand, moves it to the battlefield.
func tryPlayLand(gs *gameengine.GameState, seatIdx int) {
	seat := gs.Seats[seatIdx]
	lands := make([]*gameengine.Card, 0)
	for _, c := range seat.Hand {
		if c != nil && isLand(c) {
			lands = append(lands, c)
		}
	}
	if len(lands) == 0 {
		return
	}
	var chosen *gameengine.Card
	if seat.Hat != nil {
		chosen = seat.Hat.ChooseLandToPlay(gs, seatIdx, lands)
	}
	if chosen == nil {
		return
	}
	if !removeCard(&seat.Hand, chosen) {
		return
	}

	// MDFC / split-type face cleanup at battlefield entry. Two leak shapes
	// the deckparser's combined-type_line parse produces:
	//   - Forward MDFC ("Fell the Profane // Fell Mire", spell/land):
	//     Types=["sorcery","//","land","swamp"]. Need to swap to back-face
	//     land identity (Name/Types/CMC/TypeLine) so the front-face
	//     instant/sorcery doesn't ride onto the battlefield.
	//   - Reverse MDFC ("Midgar, City of Mako // Reactor Raid", land/spell;
	//     the FF land cycle): Types=["land","//","sorcery"]. Front-face
	//     land identity is correct as-is, but the leaked "//"+"sorcery"
	//     tokens still need to be stripped.
	// EnsureBattlefieldFrontFace composes the forward swap +
	// reverse-strip + vanilla no-op in the right order. Pre-fix, this
	// site only handled the forward case (gated by MDFCBackFaceIsLand)
	// and 339 reverse-MDFC violations leaked through to Feynman's
	// permanent_types invariant.
	gameengine.EnsureBattlefieldFrontFace(chosen)

	if !containsType(chosen.Types, "land") {
		chosen.Types = append(chosen.Types, "land")
	}
	perm := &gameengine.Permanent{
		Card:       chosen,
		Controller: seatIdx,
		Owner:      seatIdx,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}

	// Check for "enters the battlefield tapped" — three detection paths:
	//
	//   1. "etb_tapped" type tag (set by AST/extensions if present).
	//   2. Oracle text contains "enters tapped" / "enters the battlefield
	//      tapped" — covers the 584 lands with unconditional ETB-tapped
	//      text that don't have per-card handlers (guildgates, tap-duals,
	//      refuges, tri-lands, etc.).
	//   3. Per-card ETB handlers (shocklands, Bojuka Bog) fire below and
	//      may set Tapped=true conditionally.
	//
	// Path 2 intentionally does NOT handle conditional enters-tapped
	// ("unless you control a Plains or Island") — those need per-card
	// handlers that inspect the battlefield. The substring match is safe
	// here because unconditional ETB-tapped text always starts with
	// "~ enters tapped" or "~ enters the battlefield tapped" as its own
	// sentence, and conditional variants contain "unless" or "pay" which
	// bypass this gate.
	if containsType(chosen.Types, "etb_tapped") {
		perm.Tapped = true
	} else if oracleIndicatesETBTapped(chosen) {
		perm.Tapped = true
	}

	seat.Battlefield = append(seat.Battlefield, perm)
	setPlayedLand(gs, seatIdx)
	gs.LogEvent(gameengine.Event{
		Kind:   "play_land",
		Seat:   seatIdx,
		Source: chosen.DisplayName(),
		Details: map[string]interface{}{
			"rule":   "305.1",
			"tapped": perm.Tapped,
		},
	})

	// Route the land through the SAME battlefield-entry pattern every
	// other entry path uses (stack-cast, reanimate, token-mint, blink,
	// tutor-to-battlefield — see resolve.go placeTutoredCard): register
	// the land's replacement effects, then fire the full ETB dispatcher
	// cascade. Pre-r62 this site hand-rolled a ~15% subset of the
	// cascade (bare InvokeETBHook + an unbatched permanent_etb), so a
	// land played from hand never reached
	// RegisterContinuousEffectsForPermanent / ApplyStaticETBCounters /
	// self-AST ETB triggers / fireObserverETBTriggers — an Urborg,
	// Tomb of Yawgmoth (named layer dispatch, layers.go) was silently
	// inert on the most common land-entry path in every simulated game,
	// the exact bug class PR #999 closed for the non-cast entry paths.
	// The dispatcher subsumes both removed calls: it invokes the
	// per-card ETB hook and fires "permanent_etb" itself, inside a
	// §603.3b trigger batch.
	gameengine.RegisterReplacementsForPermanent(gs, perm)
	gameengine.FirePermanentETBTriggers(gs, perm)
}

// highUrgencyCommanderInZone returns true if the seat's hat is a
// YggdrasilHat AND any commander currently in the command zone scores
// ≥0.7 on commander urgency. Used to gate the in-loop commander retry
// — for low-urgency commanders we keep the original behavior (one
// pre-loop attempt + one post-loop retry).
func highUrgencyCommanderInZone(seat *gameengine.Seat) bool {
	if seat == nil || seat.Hat == nil || len(seat.CommanderNames) == 0 {
		return false
	}
	yh, ok := seat.Hat.(*hat.YggdrasilHat)
	if !ok {
		return false
	}
	for _, name := range seat.CommanderNames {
		if yh.CommanderUrgency(name) >= 0.7 {
			return true
		}
	}
	return false
}

// tryCastCommander asks the Hat if it wants to cast the commander at
// current mana, and if yes invokes CastCommanderFromCommandZone.
//
// Retry flow:
//   - runMainPhase calls this once BEFORE the hand-cast loop (line ~707).
//   - For high-urgency commanders (Voltron, combo, tribal — see
//     hat.YggdrasilHat.CommanderUrgency), runMainPhase ALSO calls this
//     after every successful in-loop cast so a ramp spell that just
//     resolved can immediately fund the commander.
//   - Finally, runMainPhase calls this once AFTER the loop (line ~775)
//     as a backstop for the low-urgency case and for high-urgency decks
//     that didn't ramp this turn.
//
// For MDFC commanders whose back face is a non-creature spell (Esika /
// The Prismatic Bridge, Jadzi / Journey to the Oracle), prefer the back
// face when affordable — those decks are built around the back face,
// and the front-face creature is the budget option. Without this, the
// AI always cast the front face and Bridge never deployed, dropping
// Esika win rate to ~9%.
//
// Iteration is keyed off seat.CommanderNames (the canonical full-DFC
// oracle name set by SetupCommanderGame) rather than card.DisplayName(),
// because a back-face cast mutates Card.Name in place. After the cast
// resolves and the commander dies, the Card returns to the command
// zone with its name still set to the back face — looking up by
// DisplayName() would then miss the original commander.
func tryCastCommander(gs *gameengine.GameState, seatIdx int) {
	seat := gs.Seats[seatIdx]
	if len(seat.CommandZone) == 0 {
		return
	}
	// Snapshot CommanderNames + CommandZone since the cast can mutate both.
	cmdrNames := append([]string(nil), seat.CommanderNames...)
	for _, name := range cmdrNames {
		var cmdr *gameengine.Card
		for _, c := range seat.CommandZone {
			if c == nil {
				continue
			}
			if c.DisplayName() == name || gameengine.DFCCardMatchesName(c, name) {
				cmdr = c
				break
			}
		}
		if cmdr == nil {
			continue
		}
		// Front-face cost is the default. Determine whether the back face
		// would be the strategically better cast — and only flip if it's
		// also affordable, so we don't refuse to cast Esika just because
		// Bridge's 6-mana cost isn't met yet.
		frontCMC := gameengine.ManaCostOf(cmdr)
		tax := seat.CommanderTax[name]
		gameengine.EnsureTypedPool(seat)
		cmdrAvailMana := seat.Mana.Total()

		castBackFace := false
		baseCMC := frontCMC
		if cmdr.IsMDFC() && cmdr.BackFaceCMC > 0 && mdfcPreferBackFace(cmdr) {
			backTotal := cmdr.BackFaceCMC + 2*tax
			if backTotal <= cmdrAvailMana {
				castBackFace = true
				baseCMC = cmdr.BackFaceCMC
			}
		}
		totalCost := baseCMC + 2*tax
		if totalCost > cmdrAvailMana {
			continue
		}
		if seat.Hat == nil {
			continue
		}
		if !seat.Hat.ShouldCastCommander(gs, seatIdx, name, tax) {
			continue
		}
		// Stamp the transient back-face flag the resolve path reads in
		// stack.go (resolvePermanentSpellETB) — it swaps Name/Types/CMC
		// to the back face there. Cleared in either branch on failure
		// so a stale flip can't leak.
		cmdr.CastingBackFace = castBackFace
		if err := gameengine.CastCommanderFromCommandZone(gs, seatIdx, name, baseCMC); err != nil {
			cmdr.CastingBackFace = false
			continue
		}
		// CastCommanderFromCommandZone PUSHES the commander spell and
		// returns — its documented contract is "the caller drives the
		// stack resolution via PriorityRound + ResolveStackTop"
		// (commander.go). Pre-r62 this site only ran SBAs, so the
		// commander spell squatted unresolved on the stack while the
		// main-phase loop kept casting: every later cast that turn
		// announced with an item already on the stack (CR §117.1a/§307.1
		// sequencing violations — the legality validator's 302-hit
		// mid-stack cluster, see legality-validator-r62 report), later
		// spells LIFO-resolved BEFORE the commander, and the commander
		// only resolved as a bystander of the next cast's drain. Mirror
		// CastSpell's own tail: open the §117.3c response window, then
		// drain.
		gameengine.PriorityRound(gs)
		gameengine.DrainStack(gs)
		gameengine.StateBasedActions(gs)
	}
}

// containsType is a lowercase-aware membership test.
func containsType(types []string, t string) bool {
	for _, x := range types {
		if x == t {
			return true
		}
	}
	return false
}

// isLand checks the card's Types for "land". We no longer use the AST
// heuristic (pure {T}: AddMana ability) because it misclassifies mana
// artifacts like Sol Ring, Mana Crypt, and Arcane Signet as "lands,"
// causing them to be filtered out of the castable list and never cast.
// Lands are identified ONLY by having "land" in their type line.
func isLand(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return containsType(c.Types, "land")
}

// hasNoManaCost detects cards with no printed mana cost that cannot be
// cast from hand via normal casting rules (CR §202.1a). These are cards
// like Profane Tutor (suspend-only), Ancestral Vision, Evermind, etc.
// A card with a cost of {0} (Ornithopter, Memnite) HAS a mana cost
// and can be cast normally. We detect "no mana cost" as: CMC=0, no
// cost:N type tag, not an artifact/creature with 0-cost (those are
// legitimate free spells).
// mdfcPreferBackFace returns true when an MDFC's back face is the
// strategically better cast. Heuristic: if the front face is a creature
// and the back face is an enchantment, sorcery, or artifact, prefer the
// back face (Bridge, Journey to the Oracle, etc.). For creature//creature
// MDFCs, prefer front face (already the default).
func mdfcPreferBackFace(c *gameengine.Card) bool {
	if c == nil || !c.IsMDFC() {
		return false
	}
	frontIsCreature := containsType(c.Types, "creature")
	backIsNonCreature := !containsType(c.BackFaceTypes, "creature")
	return frontIsCreature && backIsNonCreature
}

func hasNoManaCost(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if c.CMC > 0 {
		return false
	}
	for _, t := range c.Types {
		if strings.HasPrefix(t, "cost:") {
			return false
		}
	}
	// Zero-CMC permanents (artifacts, creatures) are legitimate free
	// spells: Ornithopter, Memnite, Mox Amber, etc.
	if containsType(c.Types, "artifact") || containsType(c.Types, "creature") ||
		containsType(c.Types, "enchantment") || containsType(c.Types, "planeswalker") {
		return false
	}
	// Instants/sorceries with no mana cost and no cost tag are
	// suspend-only or similar — cannot be cast from hand.
	if containsType(c.Types, "instant") || containsType(c.Types, "sorcery") {
		return true
	}
	return false
}

// removeCard removes c from slice by pointer identity.
func removeCard(slice *[]*gameengine.Card, c *gameengine.Card) bool {
	s := *slice
	for i, x := range s {
		if x == c {
			*slice = append(s[:i], s[i+1:]...)
			return true
		}
	}
	return false
}

// played_land_this_turn flag lives in gs.Flags so concurrent games
// are isolated (each game has its own GameState).
func playedLandKey(seatIdx int) string {
	return fmt.Sprintf("played_land_s%d", seatIdx)
}

func playedLandThisTurn(gs *gameengine.GameState, seatIdx int) bool {
	if gs.Flags == nil {
		return false
	}
	return gs.Flags[playedLandKey(seatIdx)] > 0
}

func setPlayedLand(gs *gameengine.GameState, seatIdx int) {
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags[playedLandKey(seatIdx)] = 1
}

func clearPlayedLand(gs *gameengine.GameState, seatIdx int) {
	if gs.Flags != nil {
		delete(gs.Flags, playedLandKey(seatIdx))
	}
}

// runInstantPriority gives the active player a chance to activate instant-
// speed abilities and cast instants/flash spells during any step where
// players receive priority (upkeep, draw, end step, etc.). Uses the
// current gs.Phase/gs.Step to determine timing legality — sorcery-speed
// abilities are automatically excluded by buildActivationOptions.
func runInstantPriority(gs *gameengine.GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost || seat.Hat == nil {
		return
	}

	// Lazy mana-tap: only tap the whole mana base if the hat plausibly has
	// an instant-speed play. Without this gate, tapAllManaSources floats all
	// the seat's mana at every priority window (upkeep/draw/end-step/etc.);
	// when the hat has nothing to do the lands stay tapped through every
	// opponent's turn, so the AI can never hold up instant-speed interaction.
	// A false "yes" merely taps as before (status quo); we lean toward true
	// when uncertain, so we never skip a window the hat actually wanted.
	if !hasAffordableInstantPlay(gs, seatIdx) {
		return
	}

	tapAllManaSources(gs, seat)

	const maxUpkeepActions = 3
	for i := 0; i < maxUpkeepActions; i++ {
		if gs.CheckEnd() || seat.Lost {
			return
		}

		acted := false

		// Instant-speed activated abilities (non-sorcery, non-mana).
		options := buildActivationOptions(gs, seatIdx, gs.Phase)
		if len(options) > 0 {
			chosen := seat.Hat.ChooseActivation(gs, seatIdx, options)
			if chosen != nil {
				err := gameengine.ActivateAbility(gs, seatIdx, chosen.Permanent, chosen.Ability, nil)
				if err == nil {
					acted = true
					gameengine.StateBasedActions(gs)
					drainStack(gs)
					tapAllManaSources(gs, seat)
				}
			}
		}

		// Instant-speed spells (instants + flash creatures/permanents).
		if !acted {
			castable := buildInstantCastableList(gs, seatIdx)
			if len(castable) > 0 {
				chosen := seat.Hat.ChooseCastFromHand(gs, seatIdx, castable)
				if chosen != nil {
					before := len(seat.Hand)
					err := gameengine.CastSpell(gs, seatIdx, chosen, nil)
					if err == nil && len(seat.Hand) < before {
						acted = true
						gameengine.StateBasedActions(gs)
						drainStack(gs)
						tapAllManaSources(gs, seat)
					}
				}
			}
		}

		if !acted {
			break
		}
	}
}

// hasAffordableInstantPlay reports whether the seat plausibly has an
// instant-speed play available right now, computed from POTENTIAL (untapped)
// mana via AvailableManaEstimate — which counts untapped lands + mana rocks
// WITHOUT tapping anything. It returns true if the seat could afford at least
// one instant-speed spell in hand from that potential, OR has at least one
// legal instant-speed activated ability.
//
// This is the gate that keeps runInstantPriority from eagerly tapping out the
// whole mana base when the hat has nothing to do. It is intentionally
// optimistic: a false positive just taps as before (status quo, harmless),
// while a false negative would skip a window the hat wanted — so when uncertain
// we lean toward returning true.
func hasAffordableInstantPlay(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Hat == nil {
		return false
	}

	// Potential (untapped) mana, computed WITHOUT tapping anything.
	potential := gameengine.AvailableManaEstimate(gs, seat)

	// 1) Any instant-speed spell in hand affordable from potential mana?
	for _, c := range seat.Hand {
		if c == nil || isLand(c) {
			continue
		}
		if hasNoManaCost(c) {
			continue
		}
		if !isInstantSpeed(c) {
			continue
		}
		if gameengine.CalculateTotalCost(gs, c, seatIdx) <= potential {
			return true
		}
	}

	// 2) Any legal instant-speed activated ability? buildActivationOptions
	// already filters by timing legality (sorcery-speed excluded) and by
	// affordability — but it gates affordability off seat.Mana.Total()
	// (already-floated mana), which is empty before the tap. To avoid a
	// false negative for an ability that's only payable once we tap, float
	// the potential mana into a throwaway copy of the seat's pool first,
	// probe, then restore. We never mutate the real battlefield tap state.
	if probeActivationOptionsWithPotential(gs, seat, seatIdx) {
		return true
	}

	return false
}

// probeActivationOptionsWithPotential reports whether the seat has at least
// one legal instant-speed activated ability once its POTENTIAL mana is taken
// into account. It temporarily inflates the seat's typed mana pool by the
// untapped-source potential, runs buildActivationOptions, then restores the
// pool exactly. No battlefield permanent is tapped and no real mana is spent.
func probeActivationOptionsWithPotential(gs *gameengine.GameState, seat *gameengine.Seat, seatIdx int) bool {
	gameengine.EnsureTypedPool(seat)
	floated := seat.Mana.Total()
	potential := gameengine.AvailableManaEstimate(gs, seat)
	// AvailableManaEstimate already includes the floated pool; the extra
	// untapped-source mana is the difference.
	extra := potential - floated
	if seat.Mana == nil {
		return len(buildActivationOptions(gs, seatIdx, gs.Phase)) > 0
	}
	saved := seat.Mana.Any
	if extra > 0 {
		seat.Mana.Any += extra
	}
	has := len(buildActivationOptions(gs, seatIdx, gs.Phase)) > 0
	seat.Mana.Any = saved
	return has
}

// buildInstantCastableList returns cards from hand that can be cast at
// instant speed: actual instants and permanents with flash.
func buildInstantCastableList(gs *gameengine.GameState, seatIdx int) []*gameengine.Card {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || len(seat.Hand) == 0 {
		return nil
	}
	gameengine.EnsureTypedPool(seat)
	availableMana := seat.Mana.Total()

	var out []*gameengine.Card
	for _, c := range seat.Hand {
		if c == nil || isLand(c) {
			continue
		}
		if hasNoManaCost(c) {
			continue
		}
		if !isInstantSpeed(c) {
			continue
		}
		cost := gameengine.CalculateTotalCost(gs, c, seatIdx)
		if cost > availableMana {
			continue
		}
		out = append(out, c)
	}
	return out
}

// isInstantSpeed returns true if the card can be cast at instant speed.
func isInstantSpeed(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	if containsType(c.Types, "instant") {
		return true
	}
	// Check Types for "flash" (some tokens/copies mark it there).
	for _, t := range c.Types {
		if strings.EqualFold(t, "flash") {
			return true
		}
	}
	// Check AST abilities for the Flash keyword.
	if c.AST != nil {
		for _, ab := range c.AST.Abilities {
			if kw, ok := ab.(*gameast.Keyword); ok {
				if strings.EqualFold(kw.Name, "flash") {
					return true
				}
			}
		}
	}
	return false
}

// drainStack resolves all items on the stack (triggered abilities and
// spells) until the stack is empty. This is called at phase/step
// boundaries where the rules require the stack to be empty before moving
// to the next phase. The loop alternates ResolveStackTop with SBA checks
// and priority rounds, mirroring the comp rules priority loop (CR §117).
//
// Safety cap prevents infinite loops from malformed triggered abilities.
func drainStack(gs *gameengine.GameState) {
	const maxIterations = 64
	for i := 0; i < maxIterations; i++ {
		if len(gs.Stack) == 0 {
			return
		}
		if gs.CheckEnd() {
			return
		}
		// Give players priority to respond before resolving.
		gameengine.PriorityRound(gs)
		if len(gs.Stack) == 0 {
			return
		}
		// Resolve the top item.
		gameengine.ResolveStackTop(gs)
		// Check SBAs after resolution — may push more triggers.
		gameengine.StateBasedActions(gs)
	}
}

// landSubtypeColor returns the mana color for a land's basic land
// subtype, or "" if no recognized subtype is present. When a land has
// MULTIPLE basic land subtypes (original duals, shocklands, triomes),
// we return "any" — the engine's AddMana "any" bucket lets the cost
// checker spend it on any color, which is the correct MVP behavior
// (a Watery Grave can tap for U or B; "any" is a safe superset).
//
// CR SS305.6: Plains={W}, Island={U}, Swamp={B}, Mountain={R}, Forest={G}.
func landSubtypeColor(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	found := 0
	var color string
	for _, t := range c.Types {
		switch t {
		case "plains":
			found++
			color = "W"
		case "island":
			found++
			color = "U"
		case "swamp":
			found++
			color = "B"
		case "mountain":
			found++
			color = "R"
		case "forest":
			found++
			color = "G"
		}
	}
	if found == 0 {
		return ""
	}
	if found == 1 {
		return color
	}
	// Multi-subtype land (dual, triome) — use "any" as a safe superset.
	return "any"
}

// oracleIndicatesETBTapped detects "enters tapped" or "enters the
// battlefield tapped" from the card's oracle text (stored in AST).
// This covers the ~584 lands with unconditional ETB-tapped oracle text
// that lack per-card handlers.
//
// IMPORTANT: we skip cards whose oracle text also contains "unless" or
// "pay" near the "enters tapped" clause, because those are CONDITIONAL
// enters-tapped (check lands, fast lands, shock lands) that need per-card
// handlers to evaluate the condition. Those per-card ETB handlers fire
// separately after this function is called.
func oracleIndicatesETBTapped(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	// OracleTextLower reconstructs the oracle text from AST ability raws.
	// For lands with "this land enters tapped" as a Static ability, the
	// raw text is included in the reconstruction.
	oracle := gameengine.OracleTextLower(c)
	if oracle == "" {
		return false
	}
	// Look for unconditional "enters tapped" / "enters the battlefield tapped".
	idx := strings.Index(oracle, "enters tapped")
	if idx < 0 {
		idx = strings.Index(oracle, "enters the battlefield tapped")
	}
	if idx < 0 {
		return false
	}
	// Extract the sentence containing the ETB clause. We look at the text
	// from the start of the current sentence (last period or start of text)
	// to the next period or end of text.
	sentStart := strings.LastIndex(oracle[:idx], ".") + 1
	sentEnd := strings.Index(oracle[idx:], ".")
	if sentEnd < 0 {
		sentEnd = len(oracle) - idx
	}
	sentence := oracle[sentStart : idx+sentEnd]
	// If the sentence contains "unless" or "pay", it's conditional — skip.
	// Per-card ETB handlers (shocklands, check lands, fast lands) handle
	// these cards with proper battlefield inspection.
	if strings.Contains(sentence, "unless") || strings.Contains(sentence, "pay") {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Activated-ability option builder (FIX 1)
// ---------------------------------------------------------------------------

// buildActivationOptions walks all permanents controlled by seatIdx and
// returns legal, non-mana activated ability options. Each option has passed
// stax checks, timing restrictions, tap/mana cost gating, and summoning
// sickness validation.
//
// `phase` is "main" or "combat" — used for sorcery-speed timing restrictions.
//
// Mana abilities are excluded (they resolve inline via tapAllManaSources).
// Exhaust abilities that have already been used are excluded.
// Summoning-sick creatures with tap-cost abilities are excluded.
func buildActivationOptions(gs *gameengine.GameState, seatIdx int, phase string) []gameengine.Activation {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Hat == nil {
		return nil
	}

	gameengine.EnsureTypedPool(seat)
	availableMana := seat.Mana.Total()

	var options []gameengine.Activation

	for _, perm := range seat.Battlefield {
		if perm == nil || perm.Card == nil || perm.Card.AST == nil {
			continue
		}
		if perm.Controller != seatIdx {
			continue
		}

		for idx, ab := range perm.Card.AST.Abilities {
			act, ok := ab.(*gameast.Activated)
			if !ok {
				continue
			}

			// Skip mana abilities — they resolve inline, not through Hat.
			if gameengine.IsManaAbility(perm, idx) {
				continue
			}

			// Stax check (Null Rod, Cursed Totem, Grand Abolisher, split second).
			supp := gameengine.StaxCheck(gs, seatIdx, perm, idx)
			if supp.Suppressed {
				continue
			}

			// Exhaust check — already used this game.
			if gameengine.IsExhaustAbility(perm, idx) && gameengine.IsExhausted(perm, idx) {
				continue
			}

			// Tap-cost check: can't tap an already-tapped permanent.
			if act.Cost.Tap && perm.Tapped {
				continue
			}

			// Summoning sickness: creatures can't use tap-symbol abilities
			// on the turn they entered (CR §302.6).
			if act.Cost.Tap && perm.SummoningSick && perm.IsCreature() {
				continue
			}

			// Mana cost check.
			if act.Cost.Mana != nil {
				if act.Cost.Mana.CMC() > availableMana {
					continue
				}
			}

			// Life cost check. Not applied to planeswalkers: a PayLife
			// there is a legacy encoding of the loyalty adjustment (the
			// planeswalker block below gates it against loyalty counters,
			// not the player's life).
			if act.Cost.PayLife != nil && *act.Cost.PayLife > 0 && !perm.IsPlaneswalker() {
				if seat.Life <= *act.Cost.PayLife {
					continue
				}
			}

			// Sacrifice cost check: must have a valid target to sacrifice.
			if act.Cost.Sacrifice != nil {
				if gameengine.FindSacrificeTarget(gs, seatIdx, perm, act.Cost.Sacrifice) == nil {
					continue
				}
			}

			// Discard cost check: must have enough cards in hand.
			if act.Cost.Discard != nil && *act.Cost.Discard > 0 {
				if len(seat.Hand) < *act.Cost.Discard {
					continue
				}
			}

			// Channel-style abilities have costs in Extra that include
			// "discard this card" — these are hand-activated, not battlefield.
			// Skip them since the engine doesn't model hand activations.
			if hasChannelCost(act) {
				continue
			}

			// Sorcery-speed timing restriction: only allowed during main
			// phases when the stack is empty.
			if act.TimingRestriction == "sorcery" {
				if phase != "main" || len(gs.Stack) > 0 {
					continue
				}
			}

			// Planeswalker loyalty abilities (CR §606): only one per turn
			// (§606.3), sorcery speed, and a minus ability needs at least
			// that many loyalty counters (§606.5). Thor encodes the
			// loyalty cost as a Cost.Extra string ("+1" / "−3" / "0" —
			// gameengine.LoyaltyCost); legacy datasets parsed minus costs
			// into PayLife, kept as a fallback.
			if perm.IsPlaneswalker() {
				if perm.Flags != nil && perm.Flags["loyalty_used_this_turn"] > 0 {
					continue
				}
				// Loyalty abilities are main-phase, empty-stack only —
				// same gate as TimingRestriction == "sorcery" above.
				if phase != "main" || len(gs.Stack) > 0 {
					continue
				}
				need := 0
				if d, ok := gameengine.LoyaltyCost(act); ok {
					if d < 0 {
						need = -d
					}
				} else if act.Cost.PayLife != nil && *act.Cost.PayLife > 0 {
					need = *act.Cost.PayLife
				}
				if need > 0 {
					loyalty := 0
					if perm.Counters != nil {
						loyalty = perm.Counters["loyalty"]
					}
					if loyalty < need {
						continue
					}
				}
			}

			options = append(options, gameengine.Activation{
				Permanent: perm,
				Ability:   idx,
			})
		}
	}

	return options
}

// runCombatActivations offers activated ability activations to the attacking
// player after blockers are declared but before damage. This is the window
// for ninjutsu-like abilities, pump effects, sacrifice outlets, etc.
// Capped at maxCombatActivations per player.
//
// Note: Ninjutsu itself is handled by CheckNinjutsuRefactored in combat.go;
// this function covers OTHER activated abilities during combat (equip won't
// fire here since it's sorcery-speed, but sacrifice outlets and pump will).
func runCombatActivations(gs *gameengine.GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Hat == nil {
		return
	}
	maxCombatActivations := 2
	for _, p := range seat.Battlefield {
		if p != nil && p.IsCreature() {
			maxCombatActivations++
		}
	}
	if maxCombatActivations > 8 {
		maxCombatActivations = 8
	}
	for actCount := 0; actCount < maxCombatActivations; actCount++ {
		options := buildActivationOptions(gs, seatIdx, "combat")
		if len(options) == 0 {
			break
		}
		chosen := seat.Hat.ChooseActivation(gs, seatIdx, options)
		if chosen == nil {
			break
		}
		err := gameengine.ActivateAbility(gs, seatIdx, chosen.Permanent, chosen.Ability, nil)
		if err != nil {
			break
		}
		gameengine.StateBasedActions(gs)
		if gs.CheckEnd() {
			return
		}
	}
}

// hasChannelCost returns true if an activated ability has Channel-style
// costs in its Extra field (e.g. "discard this card", "channel - {3}{R}").
// These abilities are designed to be activated from hand, not battlefield.
func hasChannelCost(act *gameast.Activated) bool {
	for _, extra := range act.Cost.Extra {
		lower := strings.ToLower(extra)
		if strings.Contains(lower, "discard this card") || strings.Contains(lower, "channel") {
			return true
		}
	}
	return false
}

