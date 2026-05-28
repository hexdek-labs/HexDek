package gameengine

// Seat-scope ward (r60, 2026-05-27) — extends the unified WardCost
// primitive from ward_alt_payment.go to anthem-style "all your
// creatures have ward N" continuous effects (Hexing Squelcher,
// Indomitable Might-class printings).
//
// Architecture:
//   - One SeatWardEntry stored in gs.SeatWardEffects per source
//     permanent granting the anthem ward. NOT one per affected
//     creature — the per-target instance is computed at evaluation
//     time inside CheckWardOnTargeting.
//   - Source perm's CURRENT controller is the seat the effect applies
//     to (CR §613-style — the effect tracks the source, and a control
//     change on the source naturally moves which seat benefits).
//   - On Source LTB, RemoveSeatWardCostsForSource drops the entry.
//
// Stacking semantics (CR §702.21e — each ward instance is a separate
// triggered ability): when a target permanent has BOTH a printed ward
// AND inherits one from a seat-scope effect, each fires as a separate
// payment. Mana costs sum naturally because each is a separate spend;
// alt-payment wards each demand their own payment. CheckWardOnTargeting
// orchestrates the sequence — if any one ward can't be paid, the spell
// is countered.

// SeatWardEntry — one anthem-style continuous ward effect.
type SeatWardEntry struct {
	// Source is the permanent generating the effect. The seat that
	// benefits is always Source.Controller at lookup time, so a control
	// change on Source automatically moves the effect to the new owner.
	// When Source LTBs the entry is removed by
	// RemoveSeatWardCostsForSource.
	Source *Permanent

	// Cost is the WardCost dispatched per matching target.
	Cost WardCost

	// FilterFn narrows which permanents inherit this ward. nil defaults
	// to "all creatures controlled by Source.Controller" — the canonical
	// Hexing Squelcher / Indomitable Might shape.
	FilterFn func(perm *Permanent) bool
}

// AddSeatWardCost registers a seat-scope ward effect on gs. Returns
// the registered entry for callers that want to remove it later via
// pointer identity (although RemoveSeatWardCostsForSource by Source
// permanent is the canonical cleanup path).
//
// Filter nil ⇒ default to "creatures controlled by Source.Controller".
func AddSeatWardCost(gs *GameState, source *Permanent, cost WardCost, filter func(perm *Permanent) bool) *SeatWardEntry {
	if gs == nil || source == nil {
		return nil
	}
	entry := &SeatWardEntry{
		Source:   source,
		Cost:     cost,
		FilterFn: filter,
	}
	gs.SeatWardEffects = append(gs.SeatWardEffects, entry)
	return entry
}

// RemoveSeatWardCostsForSource drops every seat-scope ward entry whose
// Source == source. Called from the LTB cleanup path alongside
// UnregisterContinuousEffectsForPermanent / UnregisterReplacementsForPermanent.
func RemoveSeatWardCostsForSource(gs *GameState, source *Permanent) int {
	if gs == nil || source == nil || len(gs.SeatWardEffects) == 0 {
		return 0
	}
	before := len(gs.SeatWardEffects)
	kept := gs.SeatWardEffects[:0]
	for _, e := range gs.SeatWardEffects {
		if e == nil || e.Source == source {
			continue
		}
		kept = append(kept, e)
	}
	gs.SeatWardEffects = kept
	return before - len(gs.SeatWardEffects)
}

// SeatWardCostsFor returns the list of WardCost entries that apply to
// targetPerm via seat-scope continuous effects. Each entry is yielded
// individually so CheckWardOnTargeting can fire them as separate ward
// triggers per CR §702.21e.
//
// Filter logic:
//   - Source's CURRENT controller must equal target's controller
//     (anthem grants apply only to creatures the source's controller
//     owns at lookup time — handles control-change semantics naturally).
//   - SeatWardEntry.FilterFn (when non-nil) further restricts targets.
//     nil defaults to "isCreature".
func SeatWardCostsFor(gs *GameState, targetPerm *Permanent) []SeatWardEntry {
	if gs == nil || targetPerm == nil || len(gs.SeatWardEffects) == 0 {
		return nil
	}
	out := make([]SeatWardEntry, 0, len(gs.SeatWardEffects))
	for _, e := range gs.SeatWardEffects {
		if e == nil || e.Source == nil {
			continue
		}
		// The grant flows from Source.Controller — only their creatures
		// inherit. If Source has been stolen, this naturally switches
		// the beneficiary seat.
		if e.Source.Controller != targetPerm.Controller {
			continue
		}
		if e.FilterFn != nil {
			if !e.FilterFn(targetPerm) {
				continue
			}
		} else if !targetPerm.IsCreature() {
			// Default filter: creatures only (Hexing Squelcher shape).
			continue
		}
		out = append(out, *e)
	}
	return out
}
