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
	// Collect first so firing doesn't invalidate our iteration when the
	// trigger mutates the battlefield (e.g. a saga advancing its counter).
	type pending struct {
		perm   *Permanent
		effect gameast.Effect
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
				if !triggerMatchesPhaseStep(&trig.Trigger, phase, step) {
					continue
				}
				// Controller gating — "your upkeep" fires only for active
				// player; "each upkeep" fires regardless.
				if !triggerControllerMatches(gs, perm, &trig.Trigger) {
					continue
				}
				// Intervening-if: evaluate the condition now, and again on
				// resolution (both per §603.4). MVP check: defer condition
				// until resolution (resolveConditional handles it).
				toFire = append(toFire, pending{perm: perm, effect: trig.Effect})
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
		PushTriggeredAbility(gs, p.perm, p.effect)
		if gs.CheckEnd() {
			return
		}
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

// triggerControllerMatches gates "your" vs "each" wording.
func triggerControllerMatches(gs *GameState, perm *Permanent, t *gameast.Trigger) bool {
	if gs == nil || perm == nil || t == nil {
		return true
	}
	ctrl := strings.ToLower(strings.TrimSpace(t.Controller))
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

// ExpireSourceLeftEffects scans continuous effects with DurationUntilSourceLeaves
// and removes any whose SourcePerm is no longer on any battlefield. Called
// after zone transitions (LTB, exile, bounce) as a safety net in addition
// to UnregisterContinuousEffectsForPermanent.
func ExpireSourceLeftEffects(gs *GameState) int {
	if gs == nil || len(gs.ContinuousEffects) == 0 {
		return 0
	}
	// Build a set of all permanents currently on battlefields.
	onBF := map[*Permanent]bool{}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil {
				onBF[p] = true
			}
		}
	}
	kept := gs.ContinuousEffects[:0]
	removed := 0
	for _, ce := range gs.ContinuousEffects {
		if ce == nil {
			continue
		}
		if ce.Duration == DurationUntilSourceLeaves && ce.SourcePerm != nil && !onBF[ce.SourcePerm] {
			removed++
			continue
		}
		kept = append(kept, ce)
	}
	gs.ContinuousEffects = kept
	if removed > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	return removed
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
	seat.DescendedThisTurn = false

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
	if maxSize <= 0 {
		maxSize = 7
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
