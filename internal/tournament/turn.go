package tournament

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/per_card"
)

const turnTimeBudget = 15 * time.Second

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

	// Per-turn time budget: if any single turn exceeds the budget,
	// skip remaining phases. Prevents complex boards from burning
	// the entire game timeout on one turn.
	turnStart := time.Now()
	turnOverBudget := func() bool {
		return time.Since(turnStart) > turnTimeBudget
	}

	gs.LogEvent(gameengine.Event{
		Kind: "turn_start",
		Seat: active,
		Details: map[string]interface{}{
			"turn": gs.Turn,
			"rule": "500.1",
		},
	})

	// Reset per-seat draws-this-turn counters (Narset draw suppression).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
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

	// CR §726.3a — day/night transition BEFORE untap.
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
	// Per-turn bookkeeping: drain mana pool (§500.4), reset lands-played.
	seat.ManaPool = 0
	if seat.Mana != nil {
		seat.Mana.Clear()
	}
	clearPlayedLand(gs, active)
	gs.PendingExtraCombats = 0
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
		seat.ManaPool = 0
		if seat.Mana != nil {
			seat.Mana.Clear()
		}
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
	// of a step [514.1-514.2], or if any triggered abilities are waiting
	// to be put on the stack, players receive priority. Once the stack
	// is empty and all players pass in succession, another cleanup step
	// begins."
	const maxCleanupLoops = 8 // safety cap
	for cleanupLoop := 0; cleanupLoop < maxCleanupLoops; cleanupLoop++ {
		gs.Phase, gs.Step = "ending", "cleanup"
		// §514.1 discard to hand size.
		gameengine.CleanupHandSize(gs, active, 7)
		// §514.2 expirations — clears until-EOT continuous effects, mods, damage.
		gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
		gs.InvalidateCharacteristicsCache() // ensure SBAs see post-expiry P/T
		sbaChanged := gameengine.StateBasedActions(gs)
		if !sbaChanged {
			break // no SBAs performed, cleanup is done
		}
		// §514.3a: SBAs happened — players get priority, then loop.
		gs.LogEvent(gameengine.Event{
			Kind: "cleanup_loop",
			Seat: active,
			Details: map[string]interface{}{
				"iteration": cleanupLoop + 1,
				"rule":      "514.3a",
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
	// Extra combats loop.
	for gs.PendingExtraCombats > 0 && !gs.CheckEnd() {
		gs.PendingExtraCombats--
		gs.Phase, gs.Step = "combat", "beginning_of_combat"
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
	}

	// Retry commander cast after the cast loop — ramp spells may have
	// added mana sources that weren't available on the first attempt.
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
		// Find best creature: highest power, prefer non-summoning-sick.
		var bestTarget *gameengine.Permanent
		bestScore := -1
		for _, p := range seat.Battlefield {
			if p == nil || !p.IsCreature() || p.Controller != seatIdx {
				continue
			}
			score := p.Card.BasePower + p.Card.BaseToughness
			if !p.SummoningSick {
				score += 5
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

	// Fire per-card ETB hooks for the land. This is how shocklands,
	// Bojuka Bog, and other lands with ETB effects activate when
	// played from hand (lands don't go through the stack).
	gameengine.InvokeETBHook(gs, perm)

	// Fire generic "permanent_etb" for observer triggers.
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"perm":            perm,
		"controller_seat": seatIdx,
		"card":            chosen,
	})
}

// tryCastCommander asks the Hat if it wants to cast the commander at
// current mana, and if yes invokes CastCommanderFromCommandZone.
func tryCastCommander(gs *gameengine.GameState, seatIdx int) {
	seat := gs.Seats[seatIdx]
	if len(seat.CommandZone) == 0 {
		return
	}
	// Snapshot since CommandZone may be mutated by the cast.
	commanders := append([]*gameengine.Card(nil), seat.CommandZone...)
	for _, cmdr := range commanders {
		if cmdr == nil {
			continue
		}
		name := cmdr.DisplayName()
		baseCMC := gameengine.ManaCostOf(cmdr)
		tax := seat.CommanderTax[name]
		totalCost := baseCMC + 2*tax
		gameengine.EnsureTypedPool(seat)
		cmdrAvailMana := seat.Mana.Total()
		if totalCost > cmdrAvailMana {
			continue
		}
		if seat.Hat == nil {
			continue
		}
		if !seat.Hat.ShouldCastCommander(gs, seatIdx, name, tax) {
			continue
		}
		if err := gameengine.CastCommanderFromCommandZone(gs, seatIdx, name, baseCMC); err != nil {
			continue
		}
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

			// Life cost check.
			if act.Cost.PayLife != nil && *act.Cost.PayLife > 0 {
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

			// Planeswalker loyalty abilities: only one per turn (CR §606.3).
			// Loyalty abilities have a PayLife cost on planeswalkers that
			// represents the loyalty adjustment. We track usage via a
			// per-permanent per-turn flag.
			if perm.IsPlaneswalker() {
				if perm.Flags != nil && perm.Flags["loyalty_used_this_turn"] > 0 {
					continue
				}
				// Check loyalty counter availability for minus abilities.
				if act.Cost.PayLife != nil && *act.Cost.PayLife > 0 {
					loyalty := 0
					if perm.Counters != nil {
						loyalty = perm.Counters["loyalty"]
					}
					if loyalty < *act.Cost.PayLife {
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
	const maxCombatActivations = 2
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

