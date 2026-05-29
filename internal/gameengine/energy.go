package gameengine

// Energy counter payment system — CR §106.11 (Kaladesh block, MH3).
//
// Per CR §106.11, energy is a RESOURCE POOL, not a §122 counter. The
// distinction is load-bearing for the Phase 8 carveout:
//
//  - Energy is NOT a §122 counter, so proliferate (CR §701.27) does
//    NOT add {E} — the counters package registry intentionally has no
//    "energy" entry; IsProliferateEligibleType("energy") = false.
//  - Doubling Season (CR §122.1g) does NOT double energy gain because
//    "If a player would get a number of counters" references §122
//    counters; energy is a resource. See ApplyDoublers' short-circuit
//    for the unregistered-type case (returns identity).
//
// Storage lives at Seat.Flags["energy_counters"] (legacy, hot-path
// reads) and is mirrored to Seat.EnergyCounters (typed field for the
// activation registry and per_card readers). Both helpers below keep
// the two in sync on every mutation. 136+ cards reference {E} across
// Kaladesh block and MH3.

// PayEnergy deducts `amount` energy counters from the given seat.
// Returns true if payment succeeded, false if insufficient energy.
// Does NOT modify state on failure (atomic check-and-deduct). Keeps
// Seat.EnergyCounters in sync with Seat.Flags["energy_counters"].
func PayEnergy(gs *GameState, seat, amount int) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) || amount <= 0 {
		return false
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil {
		return false
	}
	if s.Flags["energy_counters"] < amount {
		return false
	}
	s.Flags["energy_counters"] -= amount
	s.EnergyCounters = s.Flags["energy_counters"]
	gs.LogEvent(Event{
		Kind:   "energy_paid",
		Seat:   seat,
		Amount: amount,
		Details: map[string]interface{}{
			"remaining": s.Flags["energy_counters"],
		},
	})
	return true
}

// GainEnergy adds `amount` energy counters to the given seat. Per CR
// §106.11 this bypasses the §122 counter pipeline — no doubling,
// no proliferate, no §122.1g replacement-effect walk. Initializes
// the Flags map if nil and keeps the typed Seat.EnergyCounters
// mirror in sync.
func GainEnergy(gs *GameState, seat, amount int) {
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
	s.Flags["energy_counters"] += amount
	s.EnergyCounters = s.Flags["energy_counters"]
	gs.LogEvent(Event{
		Kind:   "energy_gained",
		Seat:   seat,
		Amount: amount,
		Details: map[string]interface{}{
			"total": s.Flags["energy_counters"],
		},
	})
}

// GetEnergy returns the current energy counter count for a seat.
func GetEnergy(gs *GameState, seat int) int {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return 0
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil {
		return 0
	}
	return s.Flags["energy_counters"]
}
