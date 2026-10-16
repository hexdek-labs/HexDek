package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// onAnySeatBattlefield reports whether target is currently on some seat's
// battlefield, mirroring fireTrigger's own scan exactly — same nil-seat
// skip, same pointer identity, because a Permanent has no ID field and the
// pointer IS the object identity.
//
// r148: this replaces a per-event `map[*Permanent]bool` that fireTrigger
// filled with every permanent in the game purely so the leaving-perm
// collect could avoid double-handling a permanent that had already been
// scanned. A permanent is "already scanned" exactly when it is on a
// battlefield, so one O(n) question at the end replaces n map inserts at
// the start. Called once per leaving-perm event, not once per permanent.
func onAnySeatBattlefield(gs *gameengine.GameState, target *gameengine.Permanent) bool {
	if gs == nil || target == nil {
		return false
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, perm := range seat.Battlefield {
			if perm == target {
				return true
			}
		}
	}
	return false
}
