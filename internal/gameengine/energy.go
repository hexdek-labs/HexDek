package gameengine

// Energy counter payment system — CR §122.1b (Kaladesh block, MH3).
//
// Energy is a player resource stored as Seat.Flags["energy_counters"].
// Unlike mana, energy persists across turns and phases. Players gain
// energy from effects (e.g. "you get {E}{E}{E}") and pay energy as
// costs (e.g. "Pay {E}{E}: do something").
//
// 136 cards reference energy counters across Kaladesh block and MH3.

// PayEnergy deducts `amount` energy counters from the given seat.
// Returns true if payment succeeded, false if insufficient energy.
// Does NOT modify state on failure (atomic check-and-deduct).
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

// GainEnergy adds `amount` energy counters to the given seat.
// Initializes the Flags map if nil.
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
