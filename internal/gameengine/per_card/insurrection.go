package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInsurrection wires Insurrection.
//
// Oracle text (verified via data/rules/ast_dataset.jsonl, 2026-05-28):
//
//	{5}{R}{R}{R} Sorcery
//	Untap all creatures and gain control of them until end of turn.
//	They gain haste until end of turn.
//
// The flagship "I attack with the entire table" red mass-Threaten. Cast
// the turn before someone is about to combo so the board damage gets
// redirected at the cEDH winner; even outside Commander it's an
// 8-mana Overrun-equivalent that ends most games on resolution.
//
// Implementation:
//   - OnResolve: walk every seat's battlefield, collect creature
//     permanents (own + opps; the oracle says "all creatures").
//   - For each: clear Tapped, clear SummoningSick, stamp the haste
//     flag (Flags["kw:haste"]=1) so the combat-legality + attack
//     pipeline sees haste. r102: the reparent is now the canonical
//     gameengine.ApplyControlChange and runs BEFORE the haste clear.
//     Historically this was a hand-rolled move of the
//     permanent from the original seat's Battlefield slice to the
//     caster's. This is the direct-battlefield-mutation pattern from
//     roil_elemental.go's landfall steal.
//   - Schedule a DelayedTrigger at end_of_turn that reverses both
//     halves: return each captured perm to its original controller's
//     battlefield AND clear the haste flag.
//
// emitPartial breadcrumbs two gaps consistent with the existing
// control-change family (Edea Possessed Sorceress / Roil Elemental):
//   1. Layered "until end of turn" duration is hand-rolled via the EOT
//      delayed trigger rather than the §613 layer system (the engine
//      doesn't track per-permanent control-change durations as §613
//      layers yet — Roil Elemental's "for as long as you control" gap
//      acknowledges the same limitation).
//   2. The captured perm losing all abilities until ETOT is NOT in the
//      oracle text — Insurrection doesn't include the standard
//      "creatures lose all abilities" clause that Threaten has (only
//      Conquer and Mass Mutiny do). So that's an intentional skip,
//      not a gap.
func registerInsurrection(r *Registry) {
	r.OnResolve("Insurrection", insurrectionResolve)
}

// insurrectionCapture tracks one stolen-creature-state pair so the EOT
// closure can reverse the control change.
type insurrectionCapture struct {
	perm           *gameengine.Permanent
	priorController int
	priorTapped    bool
	priorHaste     int
}

func insurrectionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "insurrection_mass_threaten"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	caster := gs.Seats[seat]
	if caster == nil {
		return
	}

	captures := make([]insurrectionCapture, 0, 16)
	for i := range gs.Seats {
		s := gs.Seats[i]
		if s == nil {
			continue
		}
		// Snapshot the battlefield slice first — we mutate it in the
		// inner loop. Iterating a slice we're about to splice from is
		// the well-known Go bug; capture-then-walk is the canonical
		// roil_elemental.go pattern.
		bf := make([]*gameengine.Permanent, len(s.Battlefield))
		copy(bf, s.Battlefield)
		for _, p := range bf {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			priorHaste := 0
			if p.Flags != nil {
				priorHaste = p.Flags["kw:haste"]
			}
			captures = append(captures, insurrectionCapture{
				perm:           p,
				priorController: p.Controller,
				priorTapped:    p.Tapped,
				priorHaste:     priorHaste,
			})
			// r102 — the canonical reparent runs FIRST, because it
			// stamps CR §302.6 sickness on the new controller and
			// Insurrection's printed "they gain haste until end of
			// turn" is what clears it. Clearing before the move would
			// be silently undone by the stamp.
			gameengine.ApplyControlChange(gs, p, seat)
			// "Untap them, and they gain haste until end of turn."
			p.Tapped = false
			p.SummoningSick = false
			// Grant haste via flag (the same shape baral_and_kari_zev.go
			// uses for First Mate Ragavan's haste-on-token grant).
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			gameengine.SetKeywordFlags(gs, p, "haste")
		}
	}

	emit(gs, slug, "Insurrection", map[string]interface{}{
		"seat":            seat,
		"creatures_taken": len(captures),
	})

	if len(captures) == 0 {
		// No creatures on any battlefield — nothing to revert at EOT.
		return
	}

	// Schedule the EOT reversal. Mirrors sen_triplets's
	// RegisterDelayedTrigger end-of-turn cleanup shape.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      gameengine.CleanupStepTrigger,
		ControllerSeat: seat,
		SourceCardName: "Insurrection",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			for _, cap := range captures {
				p := cap.perm
				if p == nil {
					continue
				}
				// Restore controller — but only if the perm is still
				// on the caster's battlefield (it may have died,
				// exiled, bounced, or been control-changed again in
				// the meantime — same defensive pattern as
				// zidane_tantalus_thief.go's EOT closure).
				if p.Controller != seat {
					continue
				}
				// r102 — canonical reparent home. Returns false if the
				// permanent is no longer where we left it, which is the
				// defensive check the hand-rolled splice was doing.
				if !gameengine.ApplyControlChange(gs, p, cap.priorController) {
					continue
				}
				// Clear the haste flag we added. The prior Tapped state
				// is NOT restored — combat happened with this creature
				// untapped and that is spent. Nor is the prior
				// SummoningSick: CR §302.6 asks whether the controller
				// has had it CONTINUOUSLY since their most recent turn
				// began, and this one's control was interrupted, so the
				// reparent's fresh stamp is the right answer and a
				// snapshot of the pre-theft value would be the wrong
				// one. That is why priorSick is gone from the capture.
				if p.Flags != nil {
					if cap.priorHaste == 0 {
						gameengine.ClearKeywordFlags(gs, p, "haste")
					}
				}
			}
		},
	})

	emitPartial(gs, slug, "Insurrection",
		"control_change_duration_via_eot_delayed_trigger_not_layered_613")
}
