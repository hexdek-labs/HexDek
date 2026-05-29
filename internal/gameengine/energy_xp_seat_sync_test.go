package gameengine

import "testing"

// Counter DB Phase 8 — engine-side seat-resource sync tests.
//
// Energy ({E}) and Experience are stored at Seat.Flags["energy_counters"]
// and Seat.Flags["experience_counters"] respectively, mirrored to the
// typed Seat.EnergyCounters and Seat.XPCounters fields for activation-
// registry consumers. The canonical helpers (GainEnergy/PayEnergy/GainXP)
// MUST keep the mirrors in sync inline so per_card readers that hit
// either path see the same value.

// TestPhase8GainEnergySyncsTypedField pins that {E} production through
// the canonical helper updates BOTH the legacy Flags storage and the
// typed Seat.EnergyCounters mirror.
func TestPhase8GainEnergySyncsTypedField(t *testing.T) {
	gs := newTestGame(t, 2)
	GainEnergy(gs, 0, 3)
	if got := gs.Seats[0].Flags["energy_counters"]; got != 3 {
		t.Errorf("Flags[energy_counters] = %d, want 3", got)
	}
	if got := gs.Seats[0].EnergyCounters; got != 3 {
		t.Errorf("Seat.EnergyCounters = %d, want 3 (typed mirror must sync inline)", got)
	}
}

// TestPhase8PayEnergySyncsTypedField pins that spending energy via the
// canonical helper keeps the typed mirror in sync on the debit side.
func TestPhase8PayEnergySyncsTypedField(t *testing.T) {
	gs := newTestGame(t, 2)
	GainEnergy(gs, 0, 5)
	if !PayEnergy(gs, 0, 3) {
		t.Fatal("PayEnergy(3) failed unexpectedly")
	}
	if got := gs.Seats[0].Flags["energy_counters"]; got != 2 {
		t.Errorf("Flags[energy_counters] = %d, want 2", got)
	}
	if got := gs.Seats[0].EnergyCounters; got != 2 {
		t.Errorf("Seat.EnergyCounters = %d, want 2 (typed mirror must sync on debit)", got)
	}
}

// TestPhase8PayEnergyInsufficientNoDrift pins that a failed PayEnergy
// (insufficient pool) leaves BOTH the Flags storage and the typed
// mirror unchanged — no atomicity drift between the two storages.
func TestPhase8PayEnergyInsufficientNoDrift(t *testing.T) {
	gs := newTestGame(t, 2)
	GainEnergy(gs, 0, 2)
	if PayEnergy(gs, 0, 5) {
		t.Fatal("PayEnergy(5) succeeded with only 2 available")
	}
	if got := gs.Seats[0].Flags["energy_counters"]; got != 2 {
		t.Errorf("Flags[energy_counters] drifted to %d after failed pay, want 2", got)
	}
	if got := gs.Seats[0].EnergyCounters; got != 2 {
		t.Errorf("Seat.EnergyCounters drifted to %d after failed pay, want 2", got)
	}
}

// TestPhase8GainXPSyncsTypedField pins that the XP gain helper updates
// BOTH the legacy Flags storage and the typed Seat.XPCounters mirror.
// Mirrors the energy helper contract.
func TestPhase8GainXPSyncsTypedField(t *testing.T) {
	gs := newTestGame(t, 2)
	GainXP(gs, 0, 1)
	GainXP(gs, 0, 1)
	if got := gs.Seats[0].Flags["experience_counters"]; got != 2 {
		t.Errorf("Flags[experience_counters] = %d, want 2", got)
	}
	if got := gs.Seats[0].XPCounters; got != 2 {
		t.Errorf("Seat.XPCounters = %d, want 2 (typed mirror must sync inline)", got)
	}
}

// TestPhase8SyncSeatResourcePoolsBackfills pins the
// legacy-direct-Flags-write reconciliation path: per_card handlers
// that mutate Seat.Flags["energy_counters"]/["experience_counters"]
// directly (pre-Phase-8 pattern) can call SyncSeatResourcePools to
// refresh the typed fields without needing to switch to the canonical
// helpers wholesale.
func TestPhase8SyncSeatResourcePoolsBackfills(t *testing.T) {
	gs := newTestGame(t, 2)
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["energy_counters"] = 7
	gs.Seats[0].Flags["experience_counters"] = 4
	// Typed fields stale at this point.
	if gs.Seats[0].EnergyCounters != 0 || gs.Seats[0].XPCounters != 0 {
		t.Fatalf("expected typed fields untouched pre-sync; got EnergyCounters=%d XPCounters=%d",
			gs.Seats[0].EnergyCounters, gs.Seats[0].XPCounters)
	}
	SyncSeatResourcePools(gs)
	if got := gs.Seats[0].EnergyCounters; got != 7 {
		t.Errorf("after sync: Seat.EnergyCounters = %d, want 7", got)
	}
	if got := gs.Seats[0].XPCounters; got != 4 {
		t.Errorf("after sync: Seat.XPCounters = %d, want 4", got)
	}
}

// TestPhase8GetXPReadsFlags pins the read accessor — GetXP returns the
// Flags storage value (canonical) so a per_card handler that wrote
// directly to Flags pre-helper-call still gets a correct read.
func TestPhase8GetXPReadsFlags(t *testing.T) {
	gs := newTestGame(t, 2)
	if gs.Seats[0].Flags == nil {
		gs.Seats[0].Flags = map[string]int{}
	}
	gs.Seats[0].Flags["experience_counters"] = 5
	if got := GetXP(gs, 0); got != 5 {
		t.Errorf("GetXP = %d, want 5 (read from canonical Flags)", got)
	}
}
