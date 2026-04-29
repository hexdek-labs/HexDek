package hat

// PokerHat behavior tests — mode transitions, RAISE cascade, threat
// scoring, archetype detection, and HOLD-mode cast priority.
//
// These mirror the Python test set in tests/test_poker_hat.py so
// Phase 12 parity can diff Go vs Python side-by-side. When the Python
// tests expand, add Go counterparts here.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// Default mode + transitions
// ---------------------------------------------------------------------

func TestPoker_DefaultModeIsCall(t *testing.T) {
	h := NewPokerHat()
	if h.Mode != ModeCall {
		t.Fatalf("PokerHat default mode should be CALL (not HOLD); got %v", h.Mode)
	}
}

// TestPoker_CallToHold_Hysteresis — CALL → HOLD happens at self_threat
// ≤ 8, NOT at 12 (that's the HOLD → CALL threshold). Confirms the
// asymmetric-threshold anti-chatter guard.
func TestPoker_CallToHold_Hysteresis(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	gs.Seats[0].Hat = h
	// Seat 0 has nothing — tiny self-threat score. Need events-seen
	// to exceed cooldown before a transition is allowed.
	for i := 0; i < modeChangeCooldownEvs+2; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})
	}
	if h.Mode != ModeHold {
		t.Fatalf("empty board should drop CALL → HOLD; got %v", h.Mode)
	}
}

// TestPoker_HoldToCall_HighThreat — start in HOLD, build up our board
// past the threshold, verify HOLD → CALL.
func TestPoker_HoldToCall_HighThreat(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeHold)
	gs.Seats[0].Hat = h
	// Fatten seat 0's board: big creatures + draw engine.
	big := newTestCardMinimal("Dragon", []string{"creature"}, 7, nil)
	newTestPermanent(gs.Seats[0], big, 10, 10)
	newTestPermanent(gs.Seats[0], big, 8, 8)
	// Fill hand with cards so hand-size dim contributes.
	for i := 0; i < 7; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			newTestCardMinimal("HandCard", []string{"creature"}, 3, nil))
	}
	// Increment events past cooldown and call LogEvent so reEvaluate fires.
	for i := 0; i < modeChangeCooldownEvs+2; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})
	}
	if h.Mode != ModeCall {
		t.Fatalf("big board should transition HOLD → CALL; got %v", h.Mode)
	}
}

// TestPoker_EmergencyRaise_LowLife — life ≤ 10 forces RAISE.
func TestPoker_EmergencyRaise_LowLife(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	gs.Seats[0].Hat = h
	gs.Seats[0].Life = 8
	for i := 0; i < modeChangeCooldownEvs+2; i++ {
		gs.LogEvent(gameengine.Event{Kind: "damage", Seat: 0})
	}
	if h.Mode != ModeRaise {
		t.Fatalf("low life should trigger emergency RAISE; got %v", h.Mode)
	}
}

// TestPoker_RaiseDecay — RAISE → CALL when life recovers and no
// imminent loss.
//
// Note on event counting: LogEvent broadcasts to every hat, including
// the one that emitted a transition. The player_mode_change event
// emitted by the transition itself BUMPS eventsSeen on the emitting
// hat, which re-enters cooldown. We want to check mode after exactly
// the RAISE→CALL jump fires (event #3 of the cooldown window) and
// BEFORE CALL→HOLD hysteresis can fire later.
func TestPoker_RaiseDecay(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeRaise)
	gs.Seats[0].Hat = h
	gs.Seats[0].Life = 25
	// Give seat 0 some board so CALL→HOLD doesn't immediately chain.
	big := newTestCardMinimal("Dragon", []string{"creature"}, 6, nil)
	newTestPermanent(gs.Seats[0], big, 6, 6)
	newTestPermanent(gs.Seats[0], big, 6, 6)
	// Only the first modeChangeCooldownEvs+1 events are needed to allow
	// the RAISE→CALL jump; extra events can chain into CALL→HOLD once
	// the cooldown re-clears.
	for i := 0; i < modeChangeCooldownEvs; i++ {
		gs.LogEvent(gameengine.Event{Kind: "life_change", Seat: 0})
	}
	if h.Mode != ModeCall {
		t.Fatalf("stable life should decay RAISE → CALL; got %v", h.Mode)
	}
}

// TestPoker_CooldownPreventsChatter — cooldown must suppress transitions
// within modeChangeCooldownEvs events of the last change.
func TestPoker_CooldownPreventsChatter(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeHold)
	gs.Seats[0].Hat = h
	gs.Seats[0].Life = 8 // would force RAISE

	// Just one event — cooldown blocks the jump.
	gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 0})
	// Cooldown: we need (eventsSeen - lastModeChangeSeq) >= 3. Hat
	// started with both 0 — so first call sees 1 - 0 = 1 < 3, no
	// transition allowed yet.
	if h.Mode != ModeHold {
		t.Fatalf("cooldown should block the first transition; got %v", h.Mode)
	}
}

// ---------------------------------------------------------------------
// RAISE cascade
// ---------------------------------------------------------------------

// TestPoker_RaiseCascade — when ANOTHER seat RAISEs and we have combo
// pieces / board power / imminent loss, we match.
func TestPoker_RaiseCascade_Matches(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeCall)
	gs.Seats[0].Hat = h
	// Fatten our board so cascade threshold fires.
	big := newTestCardMinimal("Dragon", []string{"creature"}, 6, nil)
	newTestPermanent(gs.Seats[0], big, 12, 12)

	// Seed events to consume cooldown.
	for i := 0; i < modeChangeCooldownEvs+1; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 1})
	}
	// Now simulate OTHER seat transitioning to RAISE.
	gs.LogEvent(gameengine.Event{
		Kind: "player_mode_change", Seat: 1,
		Details: map[string]interface{}{
			"from_mode": "call", "to_mode": "raise", "reason": "test",
		},
	})
	if h.Mode != ModeRaise {
		t.Fatalf("cascade should match when board≥10; got %v", h.Mode)
	}
}

// TestPoker_RaiseCascade_NoMatch — empty-hand, empty-board seat should
// NOT cascade (don't chase an empty bluff).
func TestPoker_RaiseCascade_NoMatch(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeCall)
	gs.Seats[0].Hat = h
	gs.Seats[0].Life = 20

	// Seed events to consume cooldown.
	for i := 0; i < modeChangeCooldownEvs+1; i++ {
		gs.LogEvent(gameengine.Event{Kind: "turn_start", Seat: 1})
	}
	gs.LogEvent(gameengine.Event{
		Kind: "player_mode_change", Seat: 1,
		Details: map[string]interface{}{"to_mode": "raise"},
	})
	if h.Mode == ModeRaise {
		t.Fatalf("empty-board seat should not cascade; got %v", h.Mode)
	}
}

// ---------------------------------------------------------------------
// HOLD-mode cast priority (tutor/draw/recursion over threats)
// ---------------------------------------------------------------------

// TestPoker_HoldChooseCast_PrefersTutor — HOLD picks tutors ahead of
// creatures.
func TestPoker_HoldChooseCast_PrefersTutor(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeHold)
	tutor := newTestCardMinimal("DemonicTutor", []string{"sorcery"}, 2,
		&gameast.CardAST{
			Name: "DemonicTutor",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Effect: &gameast.Tutor{Query: gameast.Filter{Base: "card"}, Destination: "hand"},
				},
			},
		})
	bigCreature := newTestCardMinimal("Wurm", []string{"creature"}, 6, nil)
	got := h.ChooseCastFromHand(gs, 0, []*gameengine.Card{bigCreature, tutor})
	if got == nil || got.DisplayName() != "DemonicTutor" {
		t.Fatalf("HOLD should prefer Tutor over big creature; got %v", got)
	}
}

// TestPoker_HoldChooseCast_SkipsHaymakers — HOLD skips uncategorized
// big cards.
func TestPoker_HoldChooseCast_SkipsHaymakers(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeHold)
	haymaker := newTestCardMinimal("Plague Wind", []string{"sorcery"}, 9, nil)
	got := h.ChooseCastFromHand(gs, 0, []*gameengine.Card{haymaker})
	if got != nil {
		t.Fatalf("HOLD should skip haymakers; got %v", got)
	}
}

// TestPoker_CallChooseCast_Greedy — CALL mode falls through to
// GreedyHat (biggest-affordable-first).
func TestPoker_CallChooseCast_Greedy(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeCall)
	cheap := newTestCardMinimal("Bolt", []string{"instant"}, 1, nil)
	big := newTestCardMinimal("Wurm", []string{"creature"}, 6, nil)
	got := h.ChooseCastFromHand(gs, 0, []*gameengine.Card{cheap, big})
	if got == nil || got.DisplayName() != "Wurm" {
		t.Fatalf("CALL should defer to greedy (biggest first); got %v", got)
	}
}

// ---------------------------------------------------------------------
// 7-dim threat score
// ---------------------------------------------------------------------

// TestPoker_ThreatScore_BoardPower — basic sanity: bigger board means
// higher score than empty board.
func TestPoker_ThreatScore_BoardPower(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	// Target: seat 1 has a big board.
	newTestPermanent(gs.Seats[1], newTestCardMinimal("X", []string{"creature"}, 3, nil), 6, 6)
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Y", []string{"creature"}, 3, nil), 5, 5)
	bigBoard := h.threatBreakdown(gs, 0, gs.Seats[1])

	// Now clear the board.
	gs.Seats[1].Battlefield = nil
	empty := h.threatBreakdown(gs, 0, gs.Seats[1])

	if bigBoard.Score <= empty.Score {
		t.Fatalf("big board should outscore empty; %+v vs %+v", bigBoard, empty)
	}
	if bigBoard.Board == 0 {
		t.Fatalf("board dim should be non-zero for stacked board; got %+v", bigBoard)
	}
}

// TestPoker_ThreatScore_GraveyardValue — a fat graveyard adds threat
// (combo / reanimator signal).
func TestPoker_ThreatScore_GraveyardValue(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	for i := 0; i < 12; i++ {
		gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard,
			newTestCardMinimal("Fatty", []string{"creature"}, 5, nil))
	}
	bd := h.threatBreakdown(gs, 0, gs.Seats[1])
	if bd.Graveyard <= 0 {
		t.Fatalf("graveyard dim should contribute > 0 for 12-fatty yard; got %+v", bd)
	}
}

// TestPoker_ThreatScore_LowLifeLeverage — life ≤ 10 boosts threat.
func TestPoker_ThreatScore_LowLifeLeverage(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	gs.Seats[1].Life = 5
	low := h.threatBreakdown(gs, 0, gs.Seats[1])
	gs.Seats[1].Life = 25
	high := h.threatBreakdown(gs, 0, gs.Seats[1])
	if low.Score <= high.Score {
		t.Fatalf("low life should outscore high life; low=%+v high=%+v", low, high)
	}
}

// ---------------------------------------------------------------------
// Archetype detection
// ---------------------------------------------------------------------

func TestPoker_Archetype_Control(t *testing.T) {
	o := &observation{cardsCastTotal: 10, countersCast: 6}
	if got := classifyArchetype(o, 5); got != ArchetypeControl {
		t.Fatalf("6+ counters -> control; got %s", got)
	}
}

func TestPoker_Archetype_Ramp(t *testing.T) {
	o := &observation{cardsCastTotal: 5, rampSpent: 9}
	if got := classifyArchetype(o, 4); got != ArchetypeRamp {
		t.Fatalf("9 ramp by T4 -> ramp; got %s", got)
	}
}

func TestPoker_Archetype_Aggro(t *testing.T) {
	o := &observation{cardsCastTotal: 5, creaturesCast: 3}
	if got := classifyArchetype(o, 5); got != ArchetypeAggro {
		t.Fatalf("60%% creatures -> aggro; got %s", got)
	}
}

func TestPoker_Archetype_Combo_BigGraveyard(t *testing.T) {
	o := &observation{cardsCastTotal: 5, graveyardHighWater: 11}
	if got := classifyArchetype(o, 6); got != ArchetypeCombo {
		t.Fatalf(">10 yard by T6 -> combo; got %s", got)
	}
}

func TestPoker_Archetype_Unknown_NotEnoughData(t *testing.T) {
	o := &observation{cardsCastTotal: 2}
	if got := classifyArchetype(o, 3); got != ArchetypeUnknown {
		t.Fatalf("<3 casts -> unknown; got %s", got)
	}
}

// ---------------------------------------------------------------------
// Blocker behavior in RAISE mode
// ---------------------------------------------------------------------

// TestPoker_AssignBlockers_RaiseDeclinesIfSafe — RAISE declines to block
// when incoming isn't lethal.
func TestPoker_AssignBlockers_RaiseDeclinesIfSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeRaise)
	gs.Seats[1].Life = 20
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("A", []string{"creature"}, 1, nil), 2, 2)
	_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("B", []string{"creature"}, 1, nil), 1, 1)
	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	if len(out[atk]) != 0 {
		t.Fatalf("RAISE should decline block when safe; got %v", out[atk])
	}
}

// TestPoker_AssignBlockers_RaiseBlocksIfLethal — RAISE still blocks
// when death is on the line.
func TestPoker_AssignBlockers_RaiseBlocksIfLethal(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHatWithMode(ModeRaise)
	gs.Seats[1].Life = 3
	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("A", []string{"creature"}, 5, nil), 5, 5)
	_ = newTestPermanent(gs.Seats[1], newTestCardMinimal("B", []string{"creature"}, 1, nil), 1, 1)
	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	// RAISE falls through to GreedyHat which chumps when lethal is on the way.
	if len(out[atk]) == 0 {
		t.Fatalf("RAISE should still block lethal; got no blockers")
	}
}

// ---------------------------------------------------------------------
// Attack-target open-target preference
// ---------------------------------------------------------------------

// TestPoker_ChooseAttackTarget_PrefersOpen — attacker picks open seat
// (no untapped blockers) over a tougher-defended seat with lower life.
func TestPoker_ChooseAttackTarget_PrefersOpen(t *testing.T) {
	gs := newTestGame(t, 3)
	h := NewPokerHat()
	// Seat 1: low life but has a big untapped blocker.
	gs.Seats[1].Life = 5
	bigBlk := newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 5, 5)
	bigBlk.Tapped = false
	// Seat 2: high life but no blockers (open).
	gs.Seats[2].Life = 30

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 1)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 2 {
		t.Fatalf("want seat 2 (open target); got seat %d", got)
	}
}

// ---------------------------------------------------------------------
// String / introspection
// ---------------------------------------------------------------------

func TestPoker_String(t *testing.T) {
	h := NewPokerHatWithMode(ModeRaise)
	if got := h.String(); got != "PokerHat(mode=raise)" {
		t.Fatalf("wrong repr: %q", got)
	}
}

func TestPoker_ThreatBreakdownFor_Exposed(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewPokerHat()
	newTestPermanent(gs.Seats[1], newTestCardMinimal("X", []string{"creature"}, 3, nil), 4, 4)
	bd := h.ThreatBreakdownFor(gs, 0, gs.Seats[1])
	if bd.Score == 0 {
		t.Fatalf("exposed breakdown should produce non-zero score; got %+v", bd)
	}
}
