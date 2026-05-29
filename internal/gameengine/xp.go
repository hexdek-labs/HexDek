package gameengine

// Experience counter pool — Phase 8 carveout (per Probe F findings).
//
// Per CR §122 experience counters technically satisfy the player-counter
// shape (Daxos / Mizzix / Daretti / Ezuri / Vish Kal / Meren style
// cumulative trackers), but HexDek treats them as a Seat-resource analog
// of energy (CR §106.11):
//
//  - Proliferate (CR §701.27) does NOT add experience — the counters
//    package registry intentionally has no "experience" entry as of
//    Counter DB Phase 8; IsProliferateEligibleType("experience") = false.
//  - Doubling Season (CR §122.1g) does NOT double XP gain because the
//    Seat-resource framing routes XP through this helper rather than
//    AddCountersWithDoublers; the replacement-effect walk is skipped.
//
// Rationale: keeping the §122 player-counter registry to exactly the
// proliferate/§122.1g-eligible set (poison + rad) means AI target
// pickers and per_card handlers' registry probes are load-bearing for
// legality rather than a filter that has to special-case experience.
//
// Storage lives at Seat.Flags["experience_counters"] (legacy, hot-path
// reads) and is mirrored to Seat.XPCounters (typed field). Both helpers
// below keep the two in sync on every mutation.

// GainXP adds `amount` experience counters to the given seat. Bypasses
// the §122 counter pipeline — no doubling, no proliferate, no §122.1g
// replacement-effect walk. Initializes the Flags map if nil and keeps
// the typed Seat.XPCounters mirror in sync.
func GainXP(gs *GameState, seat, amount int) {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || amount <= 0 {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}
	s.Flags["experience_counters"] += amount
	s.XPCounters = s.Flags["experience_counters"]
	gs.LogEvent(Event{
		Kind:   "experience_gained",
		Seat:   seat,
		Amount: amount,
		Details: map[string]interface{}{
			"total": s.Flags["experience_counters"],
		},
	})
}

// GetXP returns the current experience counter count for a seat.
func GetXP(gs *GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil {
		return 0
	}
	return s.Flags["experience_counters"]
}

// SyncSeatResourcePools refreshes the typed Seat.EnergyCounters and
// Seat.XPCounters mirrors from the canonical Flags storage. Per_card
// handlers that mutate Flags directly (legacy pattern from Phases 1–7,
// pre-Phase-8) can call this to ensure read-side consumers see the
// current value. The canonical helpers (GainEnergy/PayEnergy/GainXP)
// keep the mirrors in sync inline, so this is only needed at the
// boundary between legacy direct-Flags writes and typed-field reads.
func SyncSeatResourcePools(gs *GameState) {
	if gs == nil {
		return
	}
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if s.Flags == nil {
			s.EnergyCounters = 0
			s.XPCounters = 0
			continue
		}
		s.EnergyCounters = s.Flags["energy_counters"]
		s.XPCounters = s.Flags["experience_counters"]
	}
}
