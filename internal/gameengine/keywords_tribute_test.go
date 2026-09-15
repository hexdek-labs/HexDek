package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Tribute tests — CR §702.104
// ---------------------------------------------------------------------------

func newTributeGame4P(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(4, rand.New(rand.NewSource(51)), nil)
}

func newTributeGame2P(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(51)), nil)
}

// tributePerm builds a Tribute N creature on the battlefield.
func tributePerm(gs *GameState, owner int, name string, tributeN int) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		TypeLine:      "Creature — Satyr",
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "tribute", Args: []interface{}{float64(tributeN)}},
			},
		},
	}
	perm := &Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[owner].Battlefield = append(gs.Seats[owner].Battlefield, perm)
	return perm
}

// ---------------------------------------------------------------------------
// (d) HasTribute detector
// ---------------------------------------------------------------------------

func TestHasTribute_Detects(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic of Xenagos", 3)
	if !HasTribute(p.Card) {
		t.Fatal("HasTribute should detect tribute keyword")
	}
	if !PermHasTribute(p) {
		t.Fatal("PermHasTribute should be true on a tribute permanent")
	}
	if TributeAmount(p.Card) != 3 {
		t.Fatalf("TributeAmount = %d, want 3", TributeAmount(p.Card))
	}
}

func TestHasTribute_Negative(t *testing.T) {
	gs := newTributeGame4P(t)
	vanilla := &Permanent{
		Card: &Card{
			Name:  "Grizzly Bears",
			Owner: 0,
			Types: []string{"creature"},
			AST:   &gameast.CardAST{Name: "Grizzly Bears"},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	if HasTribute(vanilla.Card) {
		t.Fatal("HasTribute should be false for a vanilla creature")
	}
	if PermHasTribute(vanilla) {
		t.Fatal("PermHasTribute should be false for a vanilla creature")
	}
	if TributeAmount(vanilla.Card) != 0 {
		t.Fatalf("TributeAmount = %d, want 0", TributeAmount(vanilla.Card))
	}
}

// ---------------------------------------------------------------------------
// (a) Opponent accepts = N +1/+1 counters added + WasTributeAccepted=true
// ---------------------------------------------------------------------------

func TestApplyTribute_AcceptedAddsCounters(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic of Xenagos", 3)

	accepted := ApplyTribute(
		gs, p, 0,
		func() int { return 1 },            // controller picks seat 1
		func(opp int) bool { return true }, // opponent accepts
	)
	if !accepted {
		t.Fatal("ApplyTribute should return true on acceptance")
	}
	if !WasTributeAccepted(p) {
		t.Fatal("WasTributeAccepted should be true after acceptance")
	}
	if WasTributeRefused(p) {
		t.Fatal("WasTributeRefused must be false after acceptance")
	}
	if !TributeResolved(p) {
		t.Fatal("TributeResolved should be true after ApplyTribute")
	}
	if got := p.Counters["+1/+1"]; got != 3 {
		t.Fatalf("expected 3 +1/+1 counters, got %d", got)
	}
	if TributeOpponent(p) != 1 {
		t.Fatalf("TributeOpponent = %d, want 1", TributeOpponent(p))
	}
}

// ---------------------------------------------------------------------------
// (b) Opponent declines = no counters + WasTributeAccepted=false
// ---------------------------------------------------------------------------

func TestApplyTribute_RefusedAddsNoCounters(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic of Xenagos", 3)

	accepted := ApplyTribute(
		gs, p, 0,
		func() int { return 2 },
		func(opp int) bool { return false }, // opponent refuses
	)
	if accepted {
		t.Fatal("ApplyTribute should return false on refusal")
	}
	if WasTributeAccepted(p) {
		t.Fatal("WasTributeAccepted should be false after refusal")
	}
	if !WasTributeRefused(p) {
		t.Fatal("WasTributeRefused should be true after refusal — this is the gate for the §702.104b punishment effect")
	}
	if !TributeResolved(p) {
		t.Fatal("TributeResolved should still be true after refusal")
	}
	if got := p.Counters["+1/+1"]; got != 0 {
		t.Fatalf("refused tribute must add 0 counters, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// (c) Controller can pick any opponent (callback respected)
// ---------------------------------------------------------------------------

func TestApplyTribute_ControllerPicksSpecificOpponent(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		func() int { return 3 }, // controller picks seat 3 specifically
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	if decidedBy != 3 {
		t.Fatalf("decide callback should have been called with seat 3, got %d", decidedBy)
	}
	if TributeOpponent(p) != 3 {
		t.Fatalf("TributeOpponent = %d, want 3", TributeOpponent(p))
	}
}

func TestApplyTribute_NilChooseFallsBackToFirstOpponent(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		nil, // no chooseOpp callback → use leftmost opponent
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	// gs.Opponents(0) yields [1,2,3]; leftmost is 1.
	if decidedBy != 1 {
		t.Fatalf("nil chooseOpp should fall back to opponent[0]=1, got %d", decidedBy)
	}
}

func TestApplyTribute_InvalidChoiceFallsBackToFirstOpponent(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		func() int { return 0 }, // controller can't pick themselves
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	if decidedBy != 1 {
		t.Fatalf("controller's self-pick should fall back to opponent[0]=1, got %d", decidedBy)
	}
}

func TestApplyTribute_OutOfRangeChoiceFallsBack(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		func() int { return 99 }, // not a valid seat
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	if decidedBy != 1 {
		t.Fatalf("out-of-range choice should fall back to opponent[0]=1, got %d", decidedBy)
	}
}

func TestApplyTribute_LostOpponentSkippedByFallback(t *testing.T) {
	gs := newTributeGame4P(t)
	gs.Seats[1].Lost = true // seat 1 is out — fallback should skip to seat 2
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		nil,
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	if decidedBy != 2 {
		t.Fatalf("fallback should skip dead seat 1 and pick seat 2, got %d", decidedBy)
	}
}

func TestApplyTribute_PickingLostOpponentFallsBack(t *testing.T) {
	gs := newTributeGame4P(t)
	gs.Seats[3].Lost = true
	p := tributePerm(gs, 0, "Fanatic", 2)

	var decidedBy int = -1
	ApplyTribute(
		gs, p, 0,
		func() int { return 3 }, // controller picks a dead opponent
		func(opp int) bool {
			decidedBy = opp
			return true
		},
	)
	if decidedBy != 1 {
		t.Fatalf("picking a dead seat should fall back to opponent[0]=1, got %d", decidedBy)
	}
}

// ---------------------------------------------------------------------------
// Per-card trigger fan-out — "tribute_resolved"
// ---------------------------------------------------------------------------

func TestApplyTribute_FiresTributeResolvedTriggerHook(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 4)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()

	type fire struct {
		controller int
		opponent   int
		n          int
		accepted   bool
	}
	var observed fire
	saw := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev != "tribute_resolved" {
			return
		}
		saw = true
		observed.controller, _ = ctx["controller_seat"].(int)
		observed.opponent, _ = ctx["opponent_seat"].(int)
		observed.n, _ = ctx["tribute_n"].(int)
		observed.accepted, _ = ctx["accepted"].(bool)
	}

	ApplyTribute(
		gs, p, 0,
		func() int { return 2 },
		func(opp int) bool { return false },
	)
	if !saw {
		t.Fatal("TriggerHook did not observe tribute_resolved")
	}
	if observed != (fire{0, 2, 4, false}) {
		t.Fatalf("ctx = %+v, want {controller=0 opponent=2 n=4 accepted=false}", observed)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestApplyTribute_NoLivingOpponentsRecordsRefused(t *testing.T) {
	gs := newTributeGame2P(t)
	gs.Seats[1].Lost = true
	p := tributePerm(gs, 0, "Fanatic", 3)

	called := false
	accepted := ApplyTribute(
		gs, p, 0,
		nil,
		func(opp int) bool {
			called = true
			return true
		},
	)
	if accepted {
		t.Fatal("no living opponents → tribute can't be paid")
	}
	if called {
		t.Fatal("decide callback must not run when no opponents are eligible")
	}
	if !WasTributeRefused(p) {
		t.Fatal("WasTributeRefused should be true when no opponent could decide (gates the §702.104b punishment)")
	}
	if p.Counters["+1/+1"] != 0 {
		t.Fatal("no counters added when no eligible opponents")
	}
}

func TestApplyTribute_CardWithoutKeywordIsNoOp(t *testing.T) {
	gs := newTributeGame4P(t)
	notATribute := &Permanent{
		Card: &Card{
			Name:  "Grizzly Bears",
			Owner: 0,
			Types: []string{"creature"},
			AST:   &gameast.CardAST{Name: "Grizzly Bears"},
		},
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	called := false
	accepted := ApplyTribute(
		gs, notATribute, 0,
		nil,
		func(opp int) bool {
			called = true
			return true
		},
	)
	if accepted {
		t.Fatal("a card without the tribute keyword should not accept tribute")
	}
	if called {
		t.Fatal("decide callback should not fire when card has no tribute keyword")
	}
	// WasTributeRefused is true because TributeResolved is set + accepted
	// is false. Per-card handlers should be gated by HasTribute first.
	if !TributeResolved(notATribute) {
		t.Fatal("ApplyTribute records resolution even for keywordless cards (so per-card paths short-circuit safely)")
	}
}

func TestApplyTribute_NilDecideCallbackTreatedAsRefuse(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)

	accepted := ApplyTribute(gs, p, 0, nil, nil)
	if accepted {
		t.Fatal("nil decide callback should default to refused")
	}
	if !WasTributeRefused(p) {
		t.Fatal("nil decide should record refused")
	}
}

// ---------------------------------------------------------------------------
// State queries — defaults
// ---------------------------------------------------------------------------

func TestTributeStateAccessors_Defaults(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)
	if WasTributeAccepted(p) {
		t.Fatal("WasTributeAccepted should default to false")
	}
	if WasTributeRefused(p) {
		t.Fatal("WasTributeRefused should default to false (no resolution yet)")
	}
	if TributeResolved(p) {
		t.Fatal("TributeResolved should default to false")
	}
	if TributeOpponent(p) != -1 {
		t.Fatal("TributeOpponent should default to -1")
	}
	// Nil safety on all accessors.
	if WasTributeAccepted(nil) || WasTributeRefused(nil) ||
		TributeResolved(nil) || TributeOpponent(nil) != -1 {
		t.Fatal("accessors must be nil-safe")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestApplyTribute_NilSafe(t *testing.T) {
	gs := newTributeGame4P(t)
	p := tributePerm(gs, 0, "Fanatic", 2)
	if ApplyTribute(nil, p, 0, nil, nil) {
		t.Fatal("nil game should return false")
	}
	if ApplyTribute(gs, nil, 0, nil, nil) {
		t.Fatal("nil perm should return false")
	}
	if ApplyTribute(gs, p, -1, nil, nil) {
		t.Fatal("invalid controller should return false")
	}
	if ApplyTribute(gs, p, 99, nil, nil) {
		t.Fatal("out-of-range controller should return false")
	}
}
