package gameengine

// Extra-turn / skip-turn sequencing primitives (CR §720 extra turns,
// §500.7 / §725 end-the-turn interaction, and "skip your next turn").
//
// Producers (Time Warp, Time Stretch, Temporal Manipulation/AST, Eon
// Frolicker, Lighthouse Chronologist, …) call GrantExtraTurn to push the
// target seat onto a LIFO stack. The turn-advance loop in each driver
// (tournament runner, paritycheck, hexapi showmatch) calls AdvanceActiveSeat
// after a turn completes; that helper pops the stack (CR §720.2 — the most
// recently created extra turn is taken first) and, when no extra turn is
// owed, rotates to the next living seat while honoring "skip your next turn".
//
// The driver continues to own gs.Turn (incremented once per loop iteration)
// and round bookkeeping, so an extra turn naturally receives the next
// monotonic turn number — preserving the globally-monotonic gs.Turn invariant
// that ScanExpiredDurations relies on (the cleanup-r63 "until your next turn"
// fix).

// GrantExtraTurn records that `seat` is owed an extra turn (CR §720.1).
// Pushes onto the LIFO stack and keeps the legacy
// gs.Flags["extra_turns_pending"] counter and the per-seat
// "extra_turn_queued" marker in sync for code/tests that read them.
func GrantExtraTurn(gs *GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gs.ExtraTurns = append(gs.ExtraTurns, seat)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["extra_turns_pending"] = len(gs.ExtraTurns)
	if s := gs.Seats[seat]; s != nil {
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["extra_turn_queued"]++
	}
}

// ConsumeExtraTurn pops the most-recently-granted extra turn (LIFO, CR
// §720.2) and returns its owning seat. ok is false when no extra turn is
// pending. Keeps the legacy counter + per-seat marker in sync.
func ConsumeExtraTurn(gs *GameState) (seat int, ok bool) {
	if gs == nil || len(gs.ExtraTurns) == 0 {
		return -1, false
	}
	n := len(gs.ExtraTurns) - 1
	seat = gs.ExtraTurns[n]
	gs.ExtraTurns = gs.ExtraTurns[:n]
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["extra_turns_pending"] = len(gs.ExtraTurns)
	if seat >= 0 && seat < len(gs.Seats) {
		if s := gs.Seats[seat]; s != nil && s.Flags != nil && s.Flags["extra_turn_queued"] > 0 {
			s.Flags["extra_turn_queued"]--
		}
	}
	return seat, true
}

// GrantSkipNextTurn flags `seat` to skip its next turn (CR — "that player
// skips their next turn"). Stacks: a seat told to skip twice skips its next
// two turns.
func GrantSkipNextTurn(gs *GameState, seat int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["skip_next_turn"]++
}

// ConsumeSkipTurn reports whether `seat` owes a skipped turn and, if so,
// decrements the counter. Returns true when the turn should be skipped.
func ConsumeSkipTurn(gs *GameState, seat int) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil || s.Flags["skip_next_turn"] <= 0 {
		return false
	}
	s.Flags["skip_next_turn"]--
	return true
}

// seatIsLive reports whether a seat can take a turn (exists, not lost, not
// having left the game).
func seatIsLive(gs *GameState, seat int) bool {
	if seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
	return s != nil && !s.Lost && !s.LeftGame
}

// nextLiveSeatAfter returns the next live seat strictly after `from` in
// seat order, wrapping around. Returns `from` if no other live seat exists.
func nextLiveSeatAfter(gs *GameState, from int) int {
	n := len(gs.Seats)
	if n == 0 {
		return from
	}
	for k := 1; k <= n; k++ {
		cand := (from + k) % n
		if seatIsLive(gs, cand) {
			return cand
		}
	}
	return from
}

// AdvanceActiveSeat moves gs.Active to whoever takes the next turn and
// reports whether that next turn is an EXTRA turn for the same player
// (sameSeatExtraTurn == true) versus a normal rotation to the next seat.
//
// Order of operations (CR §720.2 + skip-turn):
//  1. Pop the LIFO extra-turn stack. If the owed seat is still live, it takes
//     the extra turn immediately (gs.Active := that seat). A dead/left seat's
//     extra turn evaporates; keep popping.
//  2. Otherwise rotate to the next live seat, skipping (and consuming) any
//     seat that owes a skipped turn.
//
// The caller owns gs.Turn and round counting. For an extra turn the caller
// should NOT advance the round (it is the same player taking another turn).
func AdvanceActiveSeat(gs *GameState) (sameSeatExtraTurn bool) {
	if gs == nil || len(gs.Seats) == 0 {
		return false
	}
	// 1) Extra turns (LIFO). Discard extra turns owed to seats that can no
	//    longer take them.
	for {
		seat, ok := ConsumeExtraTurn(gs)
		if !ok {
			break
		}
		if seatIsLive(gs, seat) {
			prev := gs.Active
			gs.Active = seat
			gs.LogEvent(Event{
				Kind: "extra_turn_begin",
				Seat: seat,
				Details: map[string]interface{}{
					"rule":      "720.1",
					"from_seat": prev,
					"remaining": len(gs.ExtraTurns),
				},
			})
			return true
		}
		// owed to a dead/left seat — drop it and try the next stacked entry
	}

	// 2) Normal rotation, honoring "skip your next turn".
	from := gs.Active
	// Bound skips to one full rotation per skipped seat to avoid an
	// unbounded loop if every live seat owes a skip.
	maxSkips := len(gs.Seats)
	for skips := 0; ; skips++ {
		next := nextLiveSeatAfter(gs, from)
		if next == from {
			// No other live seat (or single-seat game): no rotation possible.
			gs.Active = next
			return false
		}
		if skips < maxSkips && ConsumeSkipTurn(gs, next) {
			gs.LogEvent(Event{
				Kind: "turn_skipped",
				Seat: next,
				Details: map[string]interface{}{
					"rule": "skip_next_turn",
				},
			})
			from = next
			continue
		}
		gs.Active = next
		return false
	}
}
