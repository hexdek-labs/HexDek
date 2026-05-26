package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerToxrillTheCorrosiveCustom wires Toxrill's slime-counter end-
// step trigger and the {U}{B}, remove-a-slime-counter draw activation.
// The auto-generated stub gen_toxrill_the_corrosive.go was deleted in
// R60 Phase 2I (PR #498 Class C follow-up) — the stub's "trigger 1"
// drew a card on every end_step and minted a "1/1 Token Token" with
// types ["token","creature","token"], while the custom handler does
// the correct slime-counter + destroy work. Both handlers fired on
// end_step (overlap on the same trigger event), so the broken stub
// activated alongside the correct one in production runs.
//
// Oracle text (Innistrad: Crimson Vow, {4}{U}{B}{B}):
//
//	Deathtouch
//	At the beginning of each opponent's end step, put a slime counter
//	on each creature they control. Then destroy each creature with a
//	slime counter on it that they control.
//	{U}{B}, Remove a slime counter from a creature: Draw a card.
//	Activate only as a sorcery.
//
// Implementation:
//   - "end_step" trigger gated on active_seat being a LIVING opponent
//     of Toxrill's controller. Walk the active player's creatures, add
//     one slime counter to each, then DestroyPermanent every one with
//     a slime counter (which will be every creature they control by
//     this point — the trigger applies to all "their" creatures).
//   - OnActivated(0): {U}{B}, Remove a slime counter — find any creature
//     with a slime counter (any controller) and decrement; draw 1.
//   - Deathtouch is granted by the AST keyword pipeline.
func registerToxrillTheCorrosiveCustom(r *Registry) {
	r.OnTrigger("Toxrill, the Corrosive", "end_step", toxrillEndStepSlime)
	r.OnActivated("Toxrill, the Corrosive", toxrillRemoveSlimeDraw)
}

func toxrillEndStepSlime(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toxrill_the_corrosive_end_step_slime"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat == perm.Controller {
		// Only opponents' end steps.
		return
	}
	if activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	target := gs.Seats[activeSeat]
	if target == nil || target.Lost {
		return
	}

	// Step 1: place one slime counter on each creature they control.
	var slimed []*gameengine.Permanent
	for _, p := range target.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.AddCounter("slime", 1)
		slimed = append(slimed, p)
	}
	gs.InvalidateCharacteristicsCache()

	// Step 2: destroy each creature they control with a slime counter on it.
	destroyed := 0
	// Iterate slimed list (avoids issues if Battlefield is mutated by triggers).
	for _, p := range slimed {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Counters == nil || p.Counters["slime"] <= 0 {
			continue
		}
		// Indestructible / shield counter / regenerate shields are checked
		// inside DestroyPermanent.
		if gameengine.DestroyPermanent(gs, p, perm) {
			destroyed++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"opponent_seat": activeSeat,
		"slimed":        len(slimed),
		"destroyed":     destroyed,
	})
}

// toxrillRemoveSlimeDraw — "{U}{B}, Remove a slime counter from a
// creature: Draw a card. Activate only as a sorcery." Mana cost is paid
// at activation time; we just enforce the slime-counter cost and the
// sorcery-speed restriction.
func toxrillRemoveSlimeDraw(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "toxrill_the_corrosive_remove_slime_draw"
	if gs == nil || src == nil {
		return
	}
	if abilityIdx != 0 {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}

	// Sorcery speed check: must be the active player's main phase with an
	// empty stack. Use gs.IsSorcerySpeed when available, fall back to
	// active-seat + empty-stack heuristic.
	if !toxrillIsSorcerySpeed(gs, seatIdx) {
		emitFail(gs, slug, src.Card.DisplayName(), "not_sorcery_speed", nil)
		return
	}

	// Find any creature (any seat) with a slime counter.
	var target *gameengine.Permanent
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if p.Counters != nil && p.Counters["slime"] > 0 {
				target = p
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_creature_with_slime_counter", nil)
		return
	}

	target.Counters["slime"]--
	if target.Counters["slime"] <= 0 {
		delete(target.Counters, "slime")
	}
	gs.InvalidateCharacteristicsCache()

	drew := drawOne(gs, seatIdx, src.Card.DisplayName())
	drewName := ""
	if drew != nil {
		drewName = drew.DisplayName()
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"slime_target": target.Card.DisplayName(),
		"drew":         drewName,
	})
}

// toxrillIsSorcerySpeed returns true when the controller can take a
// sorcery-speed action: they're the active player, the stack is empty,
// and the phase is a main phase.
func toxrillIsSorcerySpeed(gs *gameengine.GameState, seatIdx int) bool {
	if gs == nil {
		return false
	}
	if gs.Active != seatIdx {
		return false
	}
	if len(gs.Stack) > 0 {
		return false
	}
	switch gs.Phase {
	case "precombat_main", "postcombat_main", "main", "main1", "main2":
		return true
	}
	return false
}
