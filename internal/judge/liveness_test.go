package judge

import (
	"testing"
	"time"
)

func TestCheckLiveness_CleanGame(t *testing.T) {
	v := CheckLiveness(LivenessSnapshot{
		Seed: 60606, GameIdx: 1, Turns: 42, MaxTurns: 60, Ended: true,
		EventCount: 3000, EventBudget: 50000,
		Elapsed: 300 * time.Millisecond, Budget: 60 * time.Second,
	})
	if len(v) != 0 {
		t.Errorf("clean terminated game flagged: %v", v)
	}
}

func TestCheckLiveness_WallClock(t *testing.T) {
	v := CheckLiveness(LivenessSnapshot{
		Seed: 60606, GameIdx: 691, Turns: 48, MaxTurns: 60, Ended: true,
		Elapsed: 32 * time.Minute, Budget: 60 * time.Second,
	})
	if len(v) != 1 || v[0].Name != "wall_clock" {
		t.Fatalf("the Plargg-shape 32-minute game must flag wall_clock, got %v", v)
	}
	if v[0].Dimension != DimensionLiveness {
		t.Errorf("dimension = %q, want liveness", v[0].Dimension)
	}
	// Zero budget disables the check (flag-gated off).
	if v := CheckLiveness(LivenessSnapshot{Elapsed: time.Hour, Ended: true}); len(v) != 0 {
		t.Errorf("zero budget must disable wall_clock, got %v", v)
	}
}

func TestCheckLiveness_TurnOverrun(t *testing.T) {
	v := CheckLiveness(LivenessSnapshot{Turns: 61, MaxTurns: 60, Ended: true})
	if len(v) != 1 || v[0].Name != "turn_overrun" {
		t.Fatalf("turn 61 of 60 must flag turn_overrun, got %v", v)
	}
	// AT the cap is fine (max-turns draws are a legitimate end).
	if v := CheckLiveness(LivenessSnapshot{Turns: 60, MaxTurns: 60, Ended: true}); len(v) != 0 {
		t.Errorf("turn == max-turns must not flag, got %v", v)
	}
}

func TestCheckLiveness_EventFlood(t *testing.T) {
	v := CheckLiveness(LivenessSnapshot{
		Turns: 30, MaxTurns: 60, Ended: false,
		EventCount: 50000, EventBudget: 50000,
	})
	if len(v) != 1 || v[0].Name != "event_flood" {
		t.Fatalf("undecided game at the event cap must flag event_flood, got %v", v)
	}
	// A DECIDED game that filled its log is noisy but alive.
	if v := CheckLiveness(LivenessSnapshot{Ended: true, EventCount: 50000, EventBudget: 50000}); len(v) != 0 {
		t.Errorf("ended game at event cap must not flag, got %v", v)
	}
}

func TestCheckLiveness_CapContract(t *testing.T) {
	// A guard fired AND the game ended — the sba704 depth-guard contract
	// held; clean.
	v := CheckLiveness(LivenessSnapshot{
		Ended: true, CapFires: []string{"percard_inline_depth_cap"},
	})
	if len(v) != 0 {
		t.Errorf("cap fired + game ended is the CORRECT guard behavior, got %v", v)
	}
	// A DRAW-CONTRACT guard fired and the game limped on — broken.
	v = CheckLiveness(LivenessSnapshot{
		Ended: false, CapFires: []string{"trigger_loop_cap"},
	})
	if len(v) != 1 || v[0].Name != "cap_contract" {
		t.Fatalf("draw-contract cap fired without game end must flag cap_contract, got %v", v)
	}
	// A SWALLOW-CONTRACT guard (dispatch cap) firing in a game that
	// runs to the turn limit is BY-DESIGN — the firehose's first wild
	// false-positive shape.
	v = CheckLiveness(LivenessSnapshot{
		Ended: false, CapFires: []string{"percard_dispatch_total_cap", "move_no_progress"},
	})
	if len(v) != 0 {
		t.Errorf("swallow-contract guards without game end must not flag, got %v", v)
	}
}

// TestCheckLiveness_RoutesThroughLogViolation pins the registry
// integration contract: every liveness violation reaches registered
// sinks (whole_test.go asserts this per dimension).
func TestCheckLiveness_RoutesThroughLogViolation(t *testing.T) {
	var got []ValidationViolation
	unregister := RegisterSink(func(v ValidationViolation) {
		if v.Dimension == DimensionLiveness {
			got = append(got, v)
		}
	})
	defer unregister()

	CheckLiveness(LivenessSnapshot{Turns: 99, MaxTurns: 60, Ended: true})
	WatchdogViolation(60606, 691, time.Minute)

	if len(got) != 2 {
		t.Fatalf("expected 2 liveness violations through LogViolation, got %d", len(got))
	}
	if got[1].Name != "wall_clock" || got[1].Context["hung"] != true {
		t.Errorf("watchdog violation malformed: %+v", got[1])
	}
}
