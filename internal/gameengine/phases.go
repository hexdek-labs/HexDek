package gameengine

// Phase 12 — phase/step transitions.
//
// This file hosts the cross-phase helpers that the tournament turn loop
// needs to reach rules parity with Python playloop.py:
//
//   - FirePhaseTriggers(gs, phase, step) — CR §603 phase-based triggers
//   - ScanExpiredDurations(gs, phase, step) — CR §514.2 EOT effect removal
//   - FireDelayedTriggers(gs, phase, step) — CR §603.7 delayed triggers
//   - UntapAll(gs, seatIdx)               — CR §502.1 untap step
//   - CleanupHandSize(gs, seatIdx, maxSize) — CR §514.1 discard enforcement
//
// These are used by internal/tournament/turn.go. They live in gameengine
// rather than tournament because they are pure engine mechanics — any
// caller that runs turns (tournament runner, interactive dev CLI, parity
// harness) needs them.
//
// Comp-rules citations throughout refer to data/rules/MagicCompRules-20260227.txt.

import (
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// FirePhaseTriggers walks every permanent on every battlefield and enqueues
// triggered abilities whose Trigger matches the current phase/step. Mirrors
// Python's upkeep_step / end_step collect_*_effects calls.
//
// Matching rules (subset that matters for tournament play):
//
//   - "upkeep"           trigger.Phase == "upkeep"  OR  trigger.Event == "upkeep"
//   - "end_step"         trigger.Phase == "end_step" / "end" OR trigger.Event == "end_step"
//   - "combat_start"     trigger.Phase == "combat_start"
//   - "your_upkeep"      as above, controller-gated to gs.Active
//   - "each_upkeep"      fires for every seat's upkeep (scope: all)
//
// For simplicity the helper checks both trigger.Phase and trigger.Event.
// The parser emits phase-based triggers under .Phase; some hand-authored
// extension tables stash the phase name under .Event.
func FirePhaseTriggers(gs *GameState, phase, step string) {
	if gs == nil {
		return
	}
	step = strings.ToLower(strings.TrimSpace(step))
	phase = strings.ToLower(strings.TrimSpace(phase))
	if step == "" && phase == "" {
		return
	}
	// CR §603.3b: all phase/step triggers across all seats fire from the same
	// boundary event and must be batched, ordered APNAP + controller-choice,
	// then drained. Per-card OnTrigger handlers fired further down also need
	// to land in the same batch.
	defer EndTriggerBatch(gs, BeginTriggerBatch(gs))
	// Collect first so firing doesn't invalidate our iteration when the
	// trigger mutates the battlefield (e.g. a saga advancing its counter).
	type pending struct {
		perm          *Permanent
		effect        gameast.Effect
		interveningIf *gameast.Condition
	}
	var toFire []pending
	for _, seat := range gs.Seats {
		if seat == nil || seat.Lost {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == nil || perm.Card == nil || perm.Card.AST == nil {
				continue
			}
			for _, ab := range perm.Card.AST.Abilities {
				trig, ok := ab.(*gameast.Triggered)
				if !ok || trig.Effect == nil {
					continue
				}
				if !triggerMatchesPhaseStepRaw(&trig.Trigger, phase, step, trig.Raw) {
					continue
				}
				// Controller gating — "your upkeep" fires only for active
				// player; "each upkeep" fires regardless.
				if !triggerControllerMatchesRaw(gs, perm, &trig.Trigger, trig.Raw) {
					continue
				}
				// Judge r63 double-fire gate: per_card owns this card's
				// phase trigger (Phyrexian Arena, Oloro, As Foretold…) —
				// FireCardTrigger below dispatches the handler; pushing
				// the AST effect too resolves the ability twice.
				if name := perm.Card.DisplayName(); (step == "upkeep" && PerCardOwnsTrigger(name, "upkeep")) ||
					((step == "end" || step == "end_step" || step == "end_of_turn") && PerCardOwnsTrigger(name, "end_step")) {
					continue
				}
				// Intervening-if: evaluate the condition now, and again on
				// resolution (both per §603.4). MVP check: defer condition
				// until resolution (resolveConditional handles it).
				toFire = append(toFire, pending{perm: perm, effect: trig.Effect, interveningIf: trig.InterveningIf})
			}
		}
	}
	// §603.3: triggers waiting to be put on the stack are placed in
	// APNAP order, each player chooses theirs. MVP: stable-sort by
	// (seat == active first, then seat index) for determinism.
	sort.SliceStable(toFire, func(i, j int) bool {
		si := toFire[i].perm.Controller
		sj := toFire[j].perm.Controller
		return si < sj
	})
	for _, p := range toFire {
		PushTriggeredAbilityWithIf(gs, p.perm, p.effect, p.interveningIf)
		if gs.CheckEnd() {
			return
		}
	}

	// Also dispatch per-card OnTrigger handlers for the phase/step. AST-
	// parsed triggers above are pushed via PushTriggeredAbility; per-card
	// handlers exist for cards whose phase abilities never made it through
	// the parser (e.g. The One Ring's "at the beginning of your upkeep,
	// you lose 1 life for each burden counter on it"). Without this fan-out
	// the handlers are dead at runtime — they only fire from tests calling
	// FireCardTrigger directly. Event names are the canonical aliases
	// declared in event_aliases.go (NormalizeEventSingle re-maps).
	ctx := map[string]interface{}{
		"phase":       phase,
		"step":        step,
		"active_seat": gs.Active,
	}
	switch step {
	case "upkeep":
		// CR §702.24a — Cumulative upkeep is a triggered ability that
		// fires "at the beginning of your upkeep": put an age counter
		// on this permanent, then may pay [cost] for each age counter
		// or sacrifice. ApplyCumulativeUpkeep was wired but never called
		// from the upkeep step, so every cumulative-upkeep permanent
		// (Tombstone Stairwell, Glacial Chasm, Drought, Phyrexian
		// Marauder, etc.) accumulated zero age counters and was never
		// asked to pay — i.e. the cumulative-upkeep keyword was inert.
		//
		// Iterate only the ACTIVE seat's battlefield: "your upkeep"
		// scopes the trigger to the turn player per CR §702.24a. Snapshot
		// the slice first so a sacrifice path (unpaid upkeep) mid-iteration
		// doesn't invalidate the range.
		if gs.Active >= 0 && gs.Active < len(gs.Seats) && gs.Seats[gs.Active] != nil {
			snapshot := make([]*Permanent, len(gs.Seats[gs.Active].Battlefield))
			copy(snapshot, gs.Seats[gs.Active].Battlefield)
			for _, p := range snapshot {
				if p == nil || p.Flags == nil {
					continue
				}
				if p.Flags["cumulative_upkeep_cost"] <= 0 {
					continue
				}
				// Defensive: skip if the permanent has already left the
				// battlefield (a sibling cumulative-upkeep sacrifice
				// earlier in this pass might have cascaded via SBAs).
				if !permanentOnBattlefield(gs, p) {
					continue
				}
				ApplyCumulativeUpkeep(gs, p)
			}
		}
		FireCardTrigger(gs, "upkeep", ctx)
	case "draw":
		FireCardTrigger(gs, "draw_step", ctx)
	case "end", "end_step", "end_of_turn":
		FireCardTrigger(gs, "end_step", ctx)
	case "untap":
		FireCardTrigger(gs, "untap", ctx)
	}
}

// triggerMatchesPhaseStep returns true if the trigger fires at the given
// (phase, step) boundary. Lenient match: accept either Trigger.Phase or
// Trigger.Event fields carrying the phase/step name.
func triggerMatchesPhaseStep(t *gameast.Trigger, phase, step string) bool {
	if t == nil {
		return false
	}
	tp := strings.ToLower(strings.TrimSpace(t.Phase))
	ev := strings.ToLower(strings.TrimSpace(t.Event))
	// Upkeep (CR §503.1).
	if step == "upkeep" {
		if tp == "upkeep" || ev == "upkeep" || ev == "your_upkeep" || ev == "each_upkeep" {
			return true
		}
	}
	// End step (CR §513.1).
	if step == "end" || step == "end_step" || step == "end_of_turn" {
		if tp == "end_step" || tp == "end" || ev == "end_step" || ev == "end_of_turn" ||
			ev == "at_end_step" || ev == "at_beginning_of_end_step" {
			return true
		}
	}
	// Draw step.
	if step == "draw" {
		if tp == "draw" || ev == "draw_step" {
			return true
		}
	}
	// Combat start (CR §507).
	if step == "beginning_of_combat" || step == "combat_start" {
		if tp == "combat_start" || ev == "combat_start" ||
			ev == "beginning_of_combat" {
			return true
		}
	}
	// Untap — rarely triggered but Python supports it.
	if step == "untap" {
		if tp == "untap" || ev == "untap" {
			return true
		}
	}
	return false
}

// triggerMatchesPhaseStepRaw is the raw-aware variant (mirrors
// triggerControllerMatchesRaw). The parser emits Event
// "beginning_of_ordinal_step" with an EMPTY Phase field for
// "at the beginning of your first/second main phase," triggers — the
// ordinal survives only in the raw clause. The base matcher therefore
// dropped EVERY main-phase trigger (Coalition Relic, Altar of Shadows,
// Abstract Paintmage, Four Knocks, …): their "first main phase" mana / draw
// / counter abilities were silently inert in real games (PROGRESSION r63c
// finding — driven through the chaos runner's
// FirePhaseTriggers(gs,"precombat_main","main") chokepoint). Recover the
// ordinal from the raw and gate it to the matching main-phase boundary.
func triggerMatchesPhaseStepRaw(t *gameast.Trigger, phase, step, raw string) bool {
	if t == nil {
		return false
	}
	if triggerMatchesPhaseStep(t, phase, step) {
		return true
	}
	ev := strings.ToLower(strings.TrimSpace(t.Event))
	if ev != "beginning_of_ordinal_step" {
		return false
	}
	r := strings.ToLower(raw)
	ph := strings.ToLower(strings.TrimSpace(phase))
	st := strings.ToLower(strings.TrimSpace(step))
	// First main phase (CR §505) — fires at the precombat main boundary.
	if strings.Contains(r, "first main phase") {
		return ph == "precombat_main" || (st == "main" && ph != "postcombat_main")
	}
	// Second main phase — fires at the postcombat main boundary.
	if strings.Contains(r, "second main phase") {
		return ph == "postcombat_main"
	}
	return false
}

// triggerControllerMatches gates "your" vs "each" wording.
func triggerControllerMatches(gs *GameState, perm *Permanent, t *gameast.Trigger) bool {
	return triggerControllerMatchesRaw(gs, perm, t, "")
}

// triggerControllerMatchesRaw is the raw-aware variant. r63 PROGRESSION
// finding: the parser emits Controller=None for ALL phase triggers —
// "your upkeep" and "EACH upkeep" alike — and the empty default gated
// everything to the controller's own turn, silently disabling the
// each-player scope for every "at the beginning of each upkeep/end
// step" card (Baleful Force class, 15 corpus shapes flagged by the
// each_scope_fire check). Until the parser carries the scope, the
// wording is recovered from the raw oracle clause.
func triggerControllerMatchesRaw(gs *GameState, perm *Permanent, t *gameast.Trigger, raw string) bool {
	if gs == nil || perm == nil || t == nil {
		return true
	}
	ctrl := strings.ToLower(strings.TrimSpace(t.Controller))
	if ctrl == "" && raw != "" {
		r := strings.ToLower(raw)
		if strings.Contains(r, "each upkeep") || strings.Contains(r, "each player's upkeep") ||
			strings.Contains(r, "each end step") || strings.Contains(r, "each player's end step") ||
			strings.Contains(r, "beginning of each") {
			return true
		}
		if strings.Contains(r, "each opponent's") || strings.Contains(r, "opponent's upkeep") ||
			strings.Contains(r, "opponent's end step") {
			return perm.Controller != gs.Active
		}
	}
	switch ctrl {
	case "", "you":
		// "At the beginning of your upkeep" — only fires on controller's turn.
		return perm.Controller == gs.Active
	case "each", "each_player":
		return true
	case "active_player":
		return perm.Controller == gs.Active
	case "opponent":
		return perm.Controller != gs.Active
	}
	// Default: fire (conservative).
	return true
}

// ScanExpiredDurations clears continuous effects, replacement effects,
// and permanent modifications whose duration has ended at this phase/step
// boundary. Mirrors Python scan_expired_durations.
//
// Fires at:
//   - ending / cleanup (§514.2): "until end of turn" + damage wear-off
//   - ending / end_of_turn: "until next end step"
//   - beginning / untap: "until your next turn" (controller only)
//   - beginning / upkeep: "until next upkeep"
//
// Active-seat-awareness: "your next turn" / "your next end step" expire
// only when the active seat matches the effect's controller (next turn
// semantics mean the SOURCE's controller's next turn).
func ScanExpiredDurations(gs *GameState, phase, step string) {
	if gs == nil {
		return
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	step = strings.ToLower(strings.TrimSpace(step))

	// 0) ZoneCastGrantExpiry backstop: reap any zone-cast grant already past
	// its declared expiry. ExpireZoneCastGrants (below, cleanup step only)
	// misses grants registered after that step or on a turn whose cleanup was
	// skipped; this sweep — run at every scan, including the §502 untap step
	// that opens each turn — guarantees such a grant is gone before any cast
	// decision or invariant check observes it. Uses grantIsLeaked semantics so
	// in-window grants are never reaped early.
	SweepLeakedZoneCastGrants(gs)

	// 1) Continuous effects — gs.ContinuousEffects.
	if len(gs.ContinuousEffects) > 0 {
		kept := gs.ContinuousEffects[:0]
		var expired int
		for _, ce := range gs.ContinuousEffects {
			if ce == nil {
				continue
			}
			if durationExpiresNow(ce.Duration, ce.ControllerSeat, gs.Active, phase, step) {
				expired++
				continue
			}
			kept = append(kept, ce)
		}
		gs.ContinuousEffects = kept
		if expired > 0 {
			gs.InvalidateCharacteristicsCache()
		}
	}

	// 1b) Until-end-of-turn control steals revert at cleanup (§514.2 —
	// r63 shared return-to-owner operation, control_revert.go).
	if step == "cleanup" || (phase == "ending" && step == "cleanup") {
		ExpireTempControlGrants(gs)
	}

	// 2) Permanent.Modifications (until-EOT buffs).
	if step == "cleanup" || (phase == "ending" && step == "cleanup") {
		modsRemoved := false
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p == nil {
					continue
				}
				// §514.2: all damage marked on permanents is removed.
				if p.MarkedDamage > 0 {
					cardName := "<unknown>"
					if p.Card != nil {
						cardName = p.Card.DisplayName()
					}
					gs.LogEvent(Event{
						Kind:   "damage_wears_off",
						Seat:   seat.Idx,
						Source: cardName,
						Amount: p.MarkedDamage,
						Details: map[string]interface{}{
							"rule": "514.2",
						},
					})
				}
				p.MarkedDamage = 0
				// §701.15a — unused regeneration shields wear off at end of
				// turn ("the next time … would be destroyed THIS TURN").
				if p.Flags != nil && p.Flags["regeneration_shield"] != 0 {
					delete(p.Flags, "regeneration_shield")
				}
				// §702.171 — saddled wears off at end of turn.
				if p.Flags != nil && p.Flags["saddled"] != 0 {
					delete(p.Flags, "saddled")
				}
				if len(p.SaddlersThisTurn) > 0 {
					p.SaddlersThisTurn = nil
				}
				if len(p.Modifications) > 0 {
					mods := p.Modifications[:0]
					for _, m := range p.Modifications {
						if m.Duration == "until_end_of_turn" ||
							m.Duration == DurationEndOfTurn {
							modsRemoved = true
							continue
						}
						mods = append(mods, m)
					}
					p.Modifications = mods
				}
				// §514.2 — a manland animated "until end of turn" (Mutavault,
				// the Restless cycle, …) stops being a creature. Strip the
				// card types the animation added; the P/T modification was
				// already removed above.
				if len(p.AnimatedAddedTypes) > 0 && p.Card != nil {
					added := make(map[string]bool, len(p.AnimatedAddedTypes))
					for _, t := range p.AnimatedAddedTypes {
						added[t] = true
					}
					kept := p.Card.Types[:0]
					for _, t := range p.Card.Types {
						if added[t] {
							continue
						}
						kept = append(kept, t)
					}
					p.Card.Types = kept
					p.AnimatedAddedTypes = nil
					modsRemoved = true
				}
				// §514.2: "until end of turn" granted abilities are removed.
				if len(p.GrantedAbilities) > 0 {
					// MVP: we don't track per-grant durations on the slice
					// (the struct carries a flat []string). Python clears
					// all entries at cleanup. We do the same for parity.
					p.GrantedAbilities = p.GrantedAbilities[:0]
					modsRemoved = true
				}
			}
		}
		// Invalidate the characteristics cache after removing modifications
		// so SBAs see the updated P/T values.
		if modsRemoved {
			gs.InvalidateCharacteristicsCache()
		}

		// Clear end-of-turn game flags: fog effects, basilisk grants, etc.
		delete(gs.Flags, "prevent_all_combat_damage")
		for _, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, p := range seat.Battlefield {
				if p == nil || p.Flags == nil {
					continue
				}
				delete(p.Flags, "basilisk_granted")
				delete(p.Flags, "basilisk_combat_hit")
				delete(p.Flags, "basilisk_marked_destroy")
			}
		}

		ExpireZoneCastGrants(gs)
		ExpireEOTGraveyardFlashbackGrants(gs)
		ExpireOrphanedGraveyardFlashbackGrants(gs)
		ExpireZoneCastPoliciesByDuration(gs)
		ExpirePlayFromGraveyardForTurn(gs)
		ClearMayhemDiscards(gs)
		ClearVisitFlags(gs)
		EndStepClearStartYourEngines(gs)

		// Reset the per-card trigger runaway-detection counter. This is a
		// per-turn budget, not a lifetime cap; without the reset a long
		// game accumulates trigger fires until every subsequent dispatch
		// is silently swallowed, breaking TriggerCompleteness on every
		// later death event (Loki r41 cluster, dominantly seen at turn
		// 40+ when the counter exceeds 2000).
		delete(gs.Flags, "trigger_total")

		// Phase E — InstanceID orphan sweep. Runs ONCE per turn at the
		// §514.2 cleanup step, after every "until end of turn" mod has
		// dropped and every until-EOT grant has expired — i.e., the most
		// stable point in the turn cycle. Closes residual TK / OG leak
		// shapes Phase D's chokepoints cannot reach (sideband-zone
		// purges, control-change transients, basic-land *Card drops). See
		// instanceid_orphan_sweep.go for design rationale.
		//
		// Mid-turn placement (e.g., inside StateBasedActions) was tried
		// first but over-ceased: spells transitioning between stack and
		// graveyard are briefly absent from every zone, and SBA fires
		// many times per turn. The cleanup-step placement gives effects
		// a chance to settle, eliminating the false-cease window. The
		// post-turn loki invariant check (cmd/hexdek-loki/main.go:937)
		// runs AFTER cleanup, so the sweep takes effect before any
		// observation.
		SweepOrphanedInstanceIDs(gs)
	}

	// 3) Delayed triggers — we don't expire them here; they consume
	// themselves when they fire via FireDelayedTriggers. But we still drop
	// any "your next turn" etc. that the source-permanent LTB may have
	// stranded — Python does the same cleanup in a follow-up pass.
}

// durationExpiresNow returns true if a continuous effect whose `duration`
// tag is currently `d` expires at the (phase, step) boundary.
func durationExpiresNow(d string, controllerSeat, activeSeat int, phase, step string) bool {
	switch d {
	case "", DurationPermanent:
		return false
	case DurationEndOfTurn, "until_end_of_turn":
		return step == "cleanup"
	case DurationUntilYourNextTurn:
		return step == "untap" && controllerSeat == activeSeat
	case DurationUntilEndOfYourNextTurn:
		return step == "cleanup" && controllerSeat == activeSeat
	case DurationUntilNextEndStep:
		return step == "end" || step == "end_step"
	case DurationUntilYourNextEndStep:
		return (step == "end" || step == "end_step") && controllerSeat == activeSeat
	case DurationUntilNextUpkeep:
		return step == "upkeep"
	case DurationUntilSourceLeaves:
		// "Until source leaves" durations are primarily managed by
		// UnregisterContinuousEffectsForPermanent on LTB. However, as a
		// safety net, we expire them here if the source permanent is no
		// longer on the battlefield. This catches edge cases where LTB
		// cleanup missed a non-layer effect.
		// NOTE: we return false here -- the LTB unregister path handles
		// it. Returning false means this duration never expires via the
		// phase/step boundary scan, which is correct per CR: the effect
		// lasts as long as the source is on the battlefield.
		return false
	case DurationUntilConditionChanges:
		// "As long as" durations require re-evaluation on state change.
		// The engine handles these via predicate functions on the
		// ContinuousEffect being re-evaluated each layer pass. Return
		// false -- they don't expire on phase/step boundaries.
		return false
	}
	return false
}

// FireDelayedTriggers walks gs.DelayedTriggers and fires any that match
// the current (phase, step) boundary. Mirrors Python _fire_delayed_triggers.
func FireDelayedTriggers(gs *GameState, phase, step string) int {
	if gs == nil || len(gs.DelayedTriggers) == 0 {
		return 0
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	step = strings.ToLower(strings.TrimSpace(step))
	var toFire []*DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt == nil || dt.Consumed {
			continue
		}
		// CR §800.4a: a delayed trigger controlled by a departed player
		// ceases — never fire it at a phase/step boundary. HandleSeatElimination
		// purges these, but guard here too (mirrors the SeatHasLeftGame
		// chokepoints elsewhere) so an off-turn boundary can't resurrect a
		// dead seat's "next end step / next upkeep" effect.
		if SeatHasLeftGame(gs, dt.ControllerSeat) {
			dt.Consumed = true
			continue
		}
		if delayedTriggerMatches(dt, gs, phase, step) {
			toFire = append(toFire, dt)
		}
	}
	// §603.7: fire in timestamp order.
	sort.SliceStable(toFire, func(i, j int) bool {
		return toFire[i].SourceTimestamp < toFire[j].SourceTimestamp
	})
	fired := 0
	for _, dt := range toFire {
		dt.Consumed = true
		gs.LogEvent(Event{
			Kind:   "delayed_trigger_fires",
			Seat:   dt.ControllerSeat,
			Source: dt.SourceCardName,
			Details: map[string]interface{}{
				"trigger_at": dt.TriggerAt,
				"rule":       "603.7",
			},
		})
		if dt.EffectFn != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						gs.LogEvent(Event{
							Kind:   "delayed_trigger_crashed",
							Source: dt.SourceCardName,
							Details: map[string]interface{}{
								"panic": r,
							},
						})
					}
				}()
				dt.EffectFn(gs)
			}()
		}
		fired++
	}
	if fired > 0 {
		kept := gs.DelayedTriggers[:0]
		for _, dt := range gs.DelayedTriggers {
			if dt != nil && !dt.Consumed {
				kept = append(kept, dt)
			}
		}
		gs.DelayedTriggers = kept
	}
	return fired
}

func delayedTriggerMatches(dt *DelayedTrigger, gs *GameState, phase, step string) bool {
	switch dt.TriggerAt {
	case "end_of_turn", "next_end_step":
		return step == "end" || step == "end_step"
	case "your_next_end_step":
		return (step == "end" || step == "end_step") &&
			gs.Active == dt.ControllerSeat &&
			gs.Turn > dt.CreatedTurn
	case "next_upkeep":
		return step == "upkeep" &&
			(gs.Turn > dt.CreatedTurn || gs.Active != dt.ControllerSeat)
	case "your_next_upkeep":
		return step == "upkeep" &&
			gs.Active == dt.ControllerSeat &&
			gs.Turn > dt.CreatedTurn
	case "end_of_combat":
		return phase == "combat" && (step == "end_of_combat" || step == "combat_end")
	case "your_next_turn":
		return step == "untap" &&
			gs.Active == dt.ControllerSeat &&
			gs.Turn > dt.CreatedTurn
	}
	return false
}

// ---------------------------------------------------------------------------
// Phasing — CR §702.26.
// ---------------------------------------------------------------------------

// PhaseOut sets the PhasedOut flag on a permanent. Phased-out permanents
// are treated as though they don't exist (§702.26a). They can't be
// targeted, don't trigger, and aren't counted by SBAs. Auras, Equipment,
// and Fortifications attached to a phasing permanent phase out alongside
// it (§702.26d — "indirect phasing").
func PhaseOut(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || p.PhasedOut {
		return
	}
	p.PhasedOut = true
	cardName := "<unknown>"
	if p.Card != nil {
		cardName = p.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "phase_out",
		Seat:   p.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "702.26",
		},
	})
	// Fire permanent_phased_out for The War Doctor, etc.
	FireCardTrigger(gs, "permanent_phased_out", map[string]interface{}{
		"seat": p.Controller,
		"card": cardName,
	})
	// §702.26d — indirectly phase out attached permanents.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, att := range s.Battlefield {
			if att.AttachedTo == p && !att.PhasedOut {
				att.PhasedOut = true
				attName := "<unknown>"
				if att.Card != nil {
					attName = att.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "phase_out",
					Seat:   att.Controller,
					Source: attName,
					Details: map[string]interface{}{
						"rule":   "702.26d",
						"reason": "indirect_phase_out",
					},
				})
			}
		}
	}
}

// PhaseIn clears the PhasedOut flag on a permanent so it re-enters
// the game. Indirectly-phased permanents also phase in (§702.26d).
func PhaseIn(gs *GameState, p *Permanent) {
	if gs == nil || p == nil || !p.PhasedOut {
		return
	}
	p.PhasedOut = false
	cardName := "<unknown>"
	if p.Card != nil {
		cardName = p.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "phase_in",
		Seat:   p.Controller,
		Source: cardName,
		Details: map[string]interface{}{
			"rule": "702.26",
		},
	})
	// §702.26d — indirectly phased attachments phase in too.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, att := range s.Battlefield {
			if att.AttachedTo == p && att.PhasedOut {
				att.PhasedOut = false
				attName := "<unknown>"
				if att.Card != nil {
					attName = att.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "phase_in",
					Seat:   att.Controller,
					Source: attName,
					Details: map[string]interface{}{
						"rule":   "702.26d",
						"reason": "indirect_phase_in",
					},
				})
			}
		}
	}
}

// PhaseInAll phases in all phased-out permanents controlled by seatIdx.
// CR §502.1: "As the untap step begins, all phased-in permanents with
// phasing that the active player controls 'phase out,' and all phased-out
// permanents that the active player controlled when they phased out
// 'phase in.'" This function handles the phase-in half.
func PhaseInAll(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// Phase in directly-phased permanents. Indirectly-phased ones come
	// along automatically via PhaseIn's indirect handling.
	for _, p := range seat.Battlefield {
		if p == nil || !p.PhasedOut {
			continue
		}
		PhaseIn(gs, p)
	}
}

// IsEffectivelyOnBattlefield returns true if the permanent is on the
// battlefield AND not phased out. Use this instead of checking the
// battlefield slice directly when phasing matters.
func IsEffectivelyOnBattlefield(gs *GameState, p *Permanent) bool {
	if p == nil || p.PhasedOut {
		return false
	}
	if p.Controller < 0 || p.Controller >= len(gs.Seats) {
		return false
	}
	for _, q := range gs.Seats[p.Controller].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

// UntapAll mirrors Python untap_step's core loop. Untaps every permanent
// the given seat controls and clears summoning sickness. Does NOT touch
// per-turn flags (the caller owns those). Events: one `untap` per
// permanent that actually changes state.
//
// §502.1: phased-out permanents phase in BEFORE untapping. PhaseInAll
// is called first.
//
// Handles:
//   - §502.2: "doesn't untap during your untap step" (DoesNotUntap flag)
//   - §122.4: stun counters — if a permanent with a stun counter would
//     untap, remove one stun counter instead of untapping.
//   - Seat.SkipUntapStep — if true, the entire untap is skipped (Stasis).
func UntapAll(gs *GameState, seatIdx int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// Turn-start resets for per-turn seat flags. Placed at the top of
	// UntapAll because the untap step is the first step of the turn
	// (§502.1) — even when SkipUntapStep is set, this code path still
	// runs, so the reset lands correctly regardless of Stasis-like
	// effects. DescendedThisTurn is written by MoveCard in zone_move.go.
	seat.Turn.Reset()
	seat.DescendedThisTurn = false
	// CR §702.94a — "the first card you've drawn this turn." Miracle's
	// first-draw window is game-turn-scoped for EVERY player, not just the
	// active one: a player who draws their first card of the current turn
	// at instant speed during an opponent's turn still qualifies. The
	// per-seat TurnCounters.Reset above only resets the active seat, so the
	// dedicated miracle draw counter is zeroed for ALL seats here at the
	// canonical turn-start hook (untap step, §502.1).
	for _, sx := range gs.Seats {
		if sx == nil {
			continue
		}
		if sx.Flags == nil {
			sx.Flags = map[string]int{}
		}
		sx.Flags["miracle_draws_this_turn"] = 0
	}
	// CR §701.4 — close the "beheld this turn" window. Behold registry
	// is game-turn-scoped, not per-seat-scoped, so the active seat's
	// untap step is the canonical reset point.
	ClearBeholdRegistry(gs)
	// Snapshot life total at turn start for Vecna-trilogy end-step checks
	// and similar "life lost this turn" computations.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["life_at_turn_start"] = seat.Life

	// §502.1: phase in phased-out permanents before untapping.
	PhaseInAll(gs, seatIdx)

	// "Skip your untap step" (Stasis, Brine Elemental, etc.)
	if seat.SkipUntapStep {
		gs.LogEvent(Event{
			Kind: "untap_step_skipped",
			Seat: seatIdx,
			Details: map[string]interface{}{
				"reason": "skip_untap_step",
				"rule":   "502.1",
			},
		})
		// Still clear summoning sickness even when untap is skipped —
		// creatures that entered last turn are no longer summoning-sick.
		for _, p := range seat.Battlefield {
			if p != nil {
				p.SummoningSick = false
			}
		}
		return
	}

	for _, p := range seat.Battlefield {
		if p == nil || p.PhasedOut {
			continue
		}
		// §302.1: summoning sickness wears off at the untap step.
		p.SummoningSick = false

		// §606.3: clear per-turn planeswalker loyalty activation flag.
		if p.Flags != nil {
			delete(p.Flags, "loyalty_used_this_turn")
		}

		// "Doesn't untap during your untap step" — skip this permanent.
		if p.DoesNotUntap {
			if p.Tapped {
				cardName := "<unknown>"
				if p.Card != nil {
					cardName = p.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "untap_skipped",
					Seat:   seatIdx,
					Source: cardName,
					Details: map[string]interface{}{
						"reason": "does_not_untap",
						"rule":   "502.2",
					},
				})
			}
			continue
		}

		// Also check Flags-based "skip_untap" for legacy compat.
		if p.Flags != nil && p.Flags["skip_untap"] > 0 {
			continue
		}

		// r63 scaffold-kind: aura_no_untap — "enchanted permanent doesn't
		// untap during its controller's untap step" (Waterknot, Shackles,
		// Capture Sphere, …). Dynamic check, no stale flag. See
		// scaffold_aura_no_untap_r63.go.
		if auraHoldsDownUntap(gs, p) {
			if p.Tapped {
				cardName := "<unknown>"
				if p.Card != nil {
					cardName = p.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "untap_skipped",
					Seat:   seatIdx,
					Source: cardName,
					Details: map[string]interface{}{
						"reason": "aura_no_untap",
						"rule":   "502.2",
					},
				})
			}
			continue
		}

		if p.Tapped {
			// §122.4: stun counters — if a permanent with a stun counter
			// would untap, remove one stun counter instead.
			stunCount := 0
			if p.Counters != nil {
				stunCount = p.Counters["stun"]
			}
			if p.Flags != nil && p.Flags["stun"] > 0 && stunCount == 0 {
				// Legacy flag-based stun (from resolve_helpers stun_target_next_untap).
				stunCount = p.Flags["stun"]
			}
			if stunCount > 0 {
				// Remove one stun counter instead of untapping.
				if p.Counters != nil && p.Counters["stun"] > 0 {
					p.Counters["stun"]--
					if p.Counters["stun"] <= 0 {
						delete(p.Counters, "stun")
					}
				} else if p.Flags != nil && p.Flags["stun"] > 0 {
					p.Flags["stun"]--
					if p.Flags["stun"] <= 0 {
						delete(p.Flags, "stun")
					}
				}
				cardName := "<unknown>"
				if p.Card != nil {
					cardName = p.Card.DisplayName()
				}
				gs.LogEvent(Event{
					Kind:   "stun_counter_removed",
					Seat:   seatIdx,
					Source: cardName,
					Details: map[string]interface{}{
						"reason": "would_untap",
						"rule":   "122.4",
					},
				})
				continue // stays tapped
			}

			p.Tapped = false
			cardName := "<unknown>"
			if p.Card != nil {
				cardName = p.Card.DisplayName()
			}
			gs.LogEvent(Event{
				Kind:   "untap_done",
				Seat:   seatIdx,
				Source: cardName,
				Details: map[string]interface{}{
					"reason": "untap_step",
					"rule":   "500.2",
				},
			})
			// CR §702.124 — Inspired triggers on the tapped→untapped
			// transition. FireInspiredTriggers is a no-op for
			// permanents without the keyword, so the per-permanent
			// dispatch cost is a single keyword lookup.
			FireInspiredTriggers(gs, p)
		}
	}
}

// CleanupHandSize mirrors Python cleanup_step's hand-size enforcement.
// Active seat discards down to maxSize; the Hat (if any) picks which cards;
// the fallback is highest-CMC first. Emits one `discard` event per card.
//
// maxSize of 0 is treated as 7 (CR §402.2 default).
func CleanupHandSize(gs *GameState, seatIdx, maxSize int) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	// CR §800.4a — objects of a player who has left the game cease to
	// exist; the §514.1 hand-size discard does not apply. Skipping here
	// closes the Phase F §400.7c fabrication class: without it, a
	// leaving-seat hand still gets force-discarded at the same turn's
	// cleanup step, the discard fires Madness / Mayhem / "card_discarded"
	// observers, and those observers register fresh state (e.g.
	// MadnessExile + ZoneCastGrant) keyed on a *Card whose InstanceID was
	// already ceased by HandleSeatElimination. The next invariant tick
	// then walks gs.ZoneCastGrants (sideband zone in the census present
	// set) sees the *Card, expected (Minted - Ceased) lacks the ID, and
	// flags it as fabricated. Loki r60 seed-42 game 411: 46 of 52
	// fabrications were Distemper of the Blood owned by seat 1, ceased
	// at seat-1 elim, re-registered via this exact path.
	if seat.LeftGame {
		return
	}
	if maxSize <= 0 {
		maxSize = 7
	}
	// Honor per-seat maximum-hand-size modifiers set by static abilities.
	// Callers pass the CR §402.2 default of 7; the flags below are stamped
	// at ETB by the relevant per_card handlers but were previously never
	// consulted here, leaving these effects inert. Resolution order:
	//   1. "no maximum hand size" (unlimited) wins — Kruphix, God of
	//      Horizons; Reliquary Tower; Thought Vessel; The Second Doctor.
	//   2. An explicit numeric override (Cecily = 11; Winter, Misanthropic
	//      Guide's per-opponent cap) replaces the default 7.
	if seatHasNoMaxHandSize(gs, seat) {
		return
	}
	if ov, ok := seatMaxHandSizeOverride(seat); ok {
		maxSize = ov
	}
	if len(seat.Hand) <= maxSize {
		return
	}
	overflow := len(seat.Hand) - maxSize
	var toDiscard []*Card
	if seat.Hat != nil {
		toDiscard = seat.Hat.ChooseDiscard(gs, seatIdx, seat.Hand, overflow)
	}
	if len(toDiscard) == 0 {
		// Fallback: highest-CMC first.
		cp := append([]*Card(nil), seat.Hand...)
		sort.SliceStable(cp, func(i, j int) bool {
			return ManaCostOf(cp[i]) > ManaCostOf(cp[j])
		})
		if overflow > len(cp) {
			overflow = len(cp)
		}
		toDiscard = cp[:overflow]
	}
	for _, c := range toDiscard {
		DiscardCard(gs, c, seatIdx)
		gs.LogEvent(Event{
			Kind:   "discard",
			Seat:   seatIdx,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"reason": "cleanup_hand_size",
				"rule":   "514.1",
			},
		})
	}
}

// seatHasNoMaxHandSize reports whether seat currently has a "no maximum
// hand size" static effect — set on ETB by Kruphix (gs.Flags), Reliquary
// Tower / Thought Vessel (gs.Flags), or The Second Doctor (seat.Flags).
// Both flag conventions are checked so any registering card is honored.
func seatHasNoMaxHandSize(gs *GameState, seat *Seat) bool {
	if seat == nil {
		return false
	}
	if seat.Flags != nil && seat.Flags["no_max_hand_size"] > 0 {
		return true
	}
	if gs != nil && gs.Flags != nil &&
		gs.Flags["no_max_hand_size_seat_"+itoa(seat.Idx)] > 0 {
		return true
	}
	return false
}

// seatMaxHandSizeOverride returns an explicit numeric maximum-hand-size
// override for seat (Cecily, Haunted Mage = 11; Winter, Misanthropic
// Guide's per-opponent cap), or (0,false) if none is set. The "no
// maximum hand size" case is handled separately by seatHasNoMaxHandSize.
func seatMaxHandSizeOverride(seat *Seat) (int, bool) {
	if seat == nil || seat.Flags == nil {
		return 0, false
	}
	if v, ok := seat.Flags["max_hand_size"]; ok {
		return v, true
	}
	if v, ok := seat.Flags["max_hand_size_override"]; ok {
		return v, true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// Paradigm — Secrets of Strixhaven keyword action
// ---------------------------------------------------------------------------

// ResolveParadigmCopies fires at the beginning of the active player's first
// main phase. For each card in gs.ParadigmExile[active], it creates a copy
// and resolves it without paying its mana cost. The original stays in exile.
//
// Per reminder text: "Then exile this spell. After you first resolve a spell
// with this name, you may cast a copy of it from exile without paying its
// mana cost at the beginning of each of your first main phases."
func ResolveParadigmCopies(gs *GameState, active int) {
	if gs == nil || gs.ParadigmExile == nil {
		return
	}
	cards := gs.ParadigmExile[active]
	if len(cards) == 0 {
		return
	}
	for _, card := range cards {
		if card == nil {
			continue
		}
		inExile := false
		if active >= 0 && active < len(gs.Seats) && gs.Seats[active] != nil {
			for _, c := range gs.Seats[active].Exile {
				if c == card {
					inExile = true
					break
				}
			}
		}
		if !inExile {
			continue
		}

		// Route through MintSpellCopy chokepoint so SourceInstanceID +
		// EnablerInstanceID are also cleared (the inline pattern here
		// only zeroed InstanceID + EnablerHistory pre-Phase G).
		copyCard := MintSpellCopy(gs, card)
		eff := collectSpellEffect(copyCard)
		item := &StackItem{
			Controller: active,
			Card:       copyCard,
			// CR §707.10: a paradigm copy is a transient game object. The
			// StackItem.IsCopy flag is what ResolveStackTop checks to make
			// the spell cease to exist on resolution instead of routing
			// the Card to the graveyard. Without it, every paradigm tick
			// added a "real" Card to the graveyard, inflating the zone-
			// conservation total by 1 per cast (824 violations in r41
			// game 181 / Decorum Dissertation / Loki seed 41).
			IsCopy: true,
			Effect: eff,
			CostMeta: map[string]interface{}{
				"paradigm_copy": true,
			},
		}
		PushStackItem(gs, item)
		gs.LogEvent(Event{
			Kind:   "paradigm_copy_cast",
			Seat:   active,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"rule": "paradigm_keyword",
			},
		})
		IncrementCastCount(gs, active)
		RecordCast(gs, active, copyCard, 0)
		FireCastTriggerObservers(gs, copyCard, active, false)
		PriorityRound(gs)
		DrainStack(gs)
	}
}

// RegisterParadigmExile adds a card to the paradigm exile tracking for the
// given seat. Called when a paradigm spell resolves.
func RegisterParadigmExile(gs *GameState, seatIdx int, card *Card) {
	if gs == nil || card == nil {
		return
	}
	if gs.ParadigmExile == nil {
		gs.ParadigmExile = map[int][]*Card{}
	}
	gs.ParadigmExile[seatIdx] = append(gs.ParadigmExile[seatIdx], card)
	gs.LogEvent(Event{
		Kind:   "paradigm_exile_created",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"pool_size": len(gs.ParadigmExile[seatIdx]),
		},
	})
}

// Unprepare sets a permanent's Prepared state to false and clears the
// legacy flag. Called after the prepared creature's spell copy resolves.
func Unprepare(perm *Permanent) {
	if perm == nil {
		return
	}
	perm.Prepared = false
	if perm.Flags != nil {
		perm.Flags["prepared"] = 0
	}
}
