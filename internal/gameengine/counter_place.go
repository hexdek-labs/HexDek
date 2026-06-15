package gameengine

// counter_place.go — the canonical +1/+1 (and any) counter-placement primitive.
//
// A recurring two-part bug (proliferate, amass, populate, monstrosity, adapt,
// …) was a counter-placer that (1) called perm.AddCounter directly, bypassing
// the §616 would_put_counter doubler chain (Doubling Season / Hardened Scales /
// Branching Evolution / Vorinclex), and (2) never fired the counter_placed
// trigger, so payoffs (Abzan Falconer, Hapatra, Conclave Mentor, …) didn't see
// the counters. PutCountersTriggered folds both into one chokepoint, mirroring
// the resolve.go CounterMod "put" path. Every keyword counter-placer routes
// through it so the bug can't recur per-site.
//
// Returns the number of counters actually placed (post-doubling, post-cancel).
func PutCountersTriggered(gs *GameState, perm *Permanent, kind string, n int, src *Permanent) int {
	if gs == nil || perm == nil || n <= 0 {
		return 0
	}
	// §616 would_put_counter chain — doublers apply (each self-filters on the
	// counter kind / controller).
	modified, cancelled := FirePutCounterEvent(gs, perm, kind, n, src)
	if cancelled || modified <= 0 {
		return 0
	}
	perm.AddCounter(kind, modified)
	gs.InvalidateCharacteristicsCache()

	srcSeat := perm.Controller
	if src != nil {
		srcSeat = controllerSeat(src)
	}
	FireCardTrigger(gs, "counter_placed", map[string]interface{}{
		"target_perm":  perm,
		"target_seat":  perm.Controller,
		"counter_kind": kind,
		"amount":       modified,
		"source_card":  sourceName(src),
		"source_seat":  srcSeat,
	})
	return modified
}
