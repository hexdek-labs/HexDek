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
//     pipeline sees haste. Reassign Controller=caster and move the
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
	priorSick      bool
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
				priorSick:      p.SummoningSick,
				priorHaste:     priorHaste,
			})
			// Untap.
			p.Tapped = false
			p.SummoningSick = false
			// Grant haste via flag (the same shape baral_and_kari_zev.go
			// uses for First Mate Ragavan's haste-on-token grant).
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["kw:haste"] = 1
			// Control change: remove from original seat's battlefield
			// (unless it's already the caster's), append to caster's,
			// reassign Controller. The direct-mutation pattern matches
			// roil_elemental.go:78.
			if p.Controller != seat {
				old := gs.Seats[p.Controller]
				if old != nil {
					for j, q := range old.Battlefield {
						if q == p {
							old.Battlefield = append(old.Battlefield[:j], old.Battlefield[j+1:]...)
							break
						}
					}
				}
				p.Controller = seat
				caster.Battlefield = append(caster.Battlefield, p)
			}
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
		TriggerAt:      "end_of_turn",
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
				casterBF := gs.Seats[seat]
				if casterBF == nil {
					continue
				}
				stillThere := false
				for j, q := range casterBF.Battlefield {
					if q == p {
						casterBF.Battlefield = append(casterBF.Battlefield[:j], casterBF.Battlefield[j+1:]...)
						stillThere = true
						break
					}
				}
				if !stillThere {
					continue
				}
				p.Controller = cap.priorController
				if orig := gs.Seats[cap.priorController]; orig != nil {
					orig.Battlefield = append(orig.Battlefield, p)
				}
				// Clear the haste flag we added. Don't restore the
				// prior Tapped/Sick — combat happened with this
				// creature untapped, those states are spent.
				if p.Flags != nil {
					if cap.priorHaste == 0 {
						delete(p.Flags, "kw:haste")
					}
				}
			}
		},
	})

	emitPartial(gs, slug, "Insurrection",
		"control_change_duration_via_eot_delayed_trigger_not_layered_613")
}
