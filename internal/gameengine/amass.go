package gameengine

import "strings"

// Amass — CR §701.43.
//
// "Amass N [subtype]" means: if you control no Army, create a 0/0 black
// [subtype] Army creature token first; then put N +1/+1 counters on an Army you
// control. This is the single canonical primitive — every amass site (the AST
// `amass` mod case, the keyword-action residual path, and per-card handlers like
// Sauron's Orcs / Nazgûl Wraiths) routes through it so the behavior is uniform:
//
//   - The Army token is BLACK (CR §701.43c) with the correct "[Subtype] Army"
//     type line.
//   - amass grows the SAME Army rather than spreading — the N counters all go
//     on one Army the player controls (existing one if any, by the "Army"
//     creature type).
//   - The counters are placed through the §616 would_put_counter chain, so
//     +1/+1 counter doublers (Doubling Season, Hardened Scales, Branching
//     Evolution, …) apply. Raw AddCounter at the call sites silently bypassed
//     them.
//
// Amass does NOT emit the "amass" log/trigger event — callers keep their own
// event logging (and "whenever you amass" wiring, if any) so this helper stays
// a pure mechanic. Returns the Army that received the counters, or nil on an
// invalid seat / count.
func Amass(gs *GameState, seatIdx, n int, subtype string) *Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n <= 0 {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}
	if subtype == "" {
		subtype = "zombie"
	}
	subtype = strings.ToLower(subtype)

	// Find an existing Army the seat controls (CR detection: the "Army"
	// creature type). amass puts ALL N counters on a single Army.
	var army *Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p.PhasedOut {
			continue
		}
		for _, tp := range p.Card.Types {
			if strings.EqualFold(tp, "army") {
				army = p
				break
			}
		}
		if army != nil {
			break
		}
	}

	if army == nil {
		// §701.43c — create one 0/0 BLACK [subtype] Army creature token.
		army = CreateCreatureToken(gs, seatIdx,
			capitalize(subtype)+" Army",
			[]string{"creature", subtype, "army"}, 0, 0)
		if army != nil && army.Card != nil {
			army.Card.Colors = []string{"B"}
		}
	}
	if army == nil {
		return nil
	}

	// §616 would_put_counter chain — +1/+1 doublers apply to the amassed
	// counters, mirroring the canonical CounterMod "put" path in resolve.go.
	modified, cancelled := FirePutCounterEvent(gs, army, "+1/+1", n, nil)
	if !cancelled && modified > 0 {
		army.AddCounter("+1/+1", modified)
		gs.InvalidateCharacteristicsCache()
	}
	return army
}
