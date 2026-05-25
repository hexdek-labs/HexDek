package hat

// R60 round 5 — 3rd Eye bluff detection. The existing
// `opponentHasInteraction` signal estimates how loaded an opponent is
// for instant-speed answers based on open mana, interactive colors,
// hand size, tutor + held-mana flags, and revealed cards. It does NOT
// notice when an opponent has SUSTAINED a high apparent-interaction
// signature for several turns without ever firing — the classic
// "bluffing Counterspell" pattern: blue player passes turn with two
// mana up every turn, never casting anything, hoping their opponents
// flinch.
//
// New per-opponent counter `opponentLoadedSilentTurns` increments at
// each upkeep when the opponent's interaction probability was >= 0.4
// but no interactive spell was cast in the previous round. Counter
// resets to 0 on the next observed interaction cast. New surface
// `perceivedInteractionThreat(gs, seat)` dampens the raw probability
// by a `bluffFactor` derived from the streak — `tableInteractionRisk`
// now consumes the dampened signal, so a four-turn bluff routine
// stops paralyzing the hat.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// isInteractionSpellName classifier
// ---------------------------------------------------------------------

func TestIsInteractionSpellName_PositiveCases(t *testing.T) {
	cases := []string{
		"Counterspell",
		"Negate",
		"Swan Song",
		"Force of Will",
		"Pact of Negation",
		"Swords to Plowshares",
		"Path to Exile",
		"Fatal Push",
		"Go for the Throat",
		"Doom Blade",
		"Dispel",
		"Mana Drain",
		"Mana Leak",
		"Spell Pierce",
	}
	for _, name := range cases {
		if !isInteractionSpellName(name) {
			t.Errorf("name %q must classify as an interaction spell", name)
		}
	}
}

func TestIsInteractionSpellName_NegativeCases(t *testing.T) {
	cases := []string{
		"Sol Ring",
		"Lightning Bolt",  // damage, not interaction in this taxonomy
		"Grizzly Bears",
		"Black Lotus",
		"",
		"   ",
	}
	for _, name := range cases {
		if isInteractionSpellName(name) {
			t.Errorf("name %q must NOT classify as an interaction spell", name)
		}
	}
}

func TestIsInteractionSpellName_CaseInsensitive(t *testing.T) {
	if !isInteractionSpellName("COUNTERSPELL") {
		t.Error("case-insensitive match must catch all-caps")
	}
	if !isInteractionSpellName("swords to plowshares") {
		t.Error("case-insensitive match must catch all-lower")
	}
	if !isInteractionSpellName("Force of Negation") {
		t.Error("substring match must catch 'Force of *'")
	}
}

// ---------------------------------------------------------------------
// bluffFactor — streak → multiplier
// ---------------------------------------------------------------------

func TestBluffFactor_NoStreakIsOne(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	h.opponentLoadedSilentTurns = []int{0, 0, 0, 0}
	if got := h.bluffFactor(1); got != 1.0 {
		t.Errorf("zero streak must yield bluffFactor=1.0; got %.3f", got)
	}
}

func TestBluffFactor_GrowsWithStreak(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	h.opponentLoadedSilentTurns = []int{0, 1, 2, 3}
	a := h.bluffFactor(1)
	b := h.bluffFactor(2)
	c := h.bluffFactor(3)
	if !(a > b && b > c) {
		t.Errorf("bluffFactor must DECREASE as streak grows: streak1=%.3f streak2=%.3f streak3=%.3f", a, b, c)
	}
}

func TestBluffFactor_FloorAtMaxStreak(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	h.opponentLoadedSilentTurns = []int{bluffMaxStreak * 10}
	if got := h.bluffFactor(0); got != bluffFloor {
		t.Errorf("very-high streak must clamp to bluffFloor=%.2f; got %.3f", bluffFloor, got)
	}
}

func TestBluffFactor_OutOfRangeIsSafe(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	if h.bluffFactor(-1) != 1.0 {
		t.Error("negative seat must be safe (factor=1.0)")
	}
	if h.bluffFactor(99) != 1.0 {
		t.Error("out-of-range seat must be safe (factor=1.0)")
	}
}

// ---------------------------------------------------------------------
// perceivedInteractionThreat composition
// ---------------------------------------------------------------------

// loadedOpponentSetup gives seat 1 a full hand + 3 untapped Islands +
// known blue color so opponentHasInteraction returns a "loaded" signal.
func loadedOpponentSetup(t *testing.T) (*gameengine.GameState, *YggdrasilHat) {
	t.Helper()
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Full hand.
	for i := 0; i < 7; i++ {
		gs.Seats[1].Hand = append(gs.Seats[1].Hand, &gameengine.Card{Name: "Mystery"})
	}
	// Three untapped Islands so AvailableManaEstimate >= 3.
	for i := 0; i < 3; i++ {
		island := newTestCardMinimal("Island", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[1], island, 0, 0)
	}
	h.opponentColors[1]["U"] = true
	return gs, h
}

func TestPerceivedInteractionThreat_EqualsRawWhenNoStreak(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	raw := h.opponentHasInteraction(gs, 1)
	perceived := h.perceivedInteractionThreat(gs, 1)
	if raw == 0 {
		t.Fatalf("test setup failed — raw interaction prob was 0, can't test composition")
	}
	if raw != perceived {
		t.Errorf("with streak=0, perceived must equal raw; raw=%.3f perceived=%.3f", raw, perceived)
	}
}

func TestPerceivedInteractionThreat_DampensWithStreak(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	rawBefore := h.opponentHasInteraction(gs, 1)
	if rawBefore < bluffLoadedThreshold {
		t.Fatalf("test setup must produce a loaded-grade signal (>= %.2f); got %.3f", bluffLoadedThreshold, rawBefore)
	}

	h.opponentLoadedSilentTurns[1] = 3
	perceivedAfter := h.perceivedInteractionThreat(gs, 1)

	if perceivedAfter >= rawBefore {
		t.Errorf("streak=3 must dampen perceived below raw: raw=%.3f perceived=%.3f", rawBefore, perceivedAfter)
	}
}

// ---------------------------------------------------------------------
// End-to-end signal lifecycle
// ---------------------------------------------------------------------

// TestBluff_LoadedSilentTurnsAccumulateAtUpkeep: simulate two consecutive
// upkeep events where opponent 1 has mana up + interactive colors but
// hasn't cast anything. Counter should be 2.
func TestBluff_LoadedSilentTurnsAccumulateAtUpkeep(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})

	if got := h.opponentLoadedSilentTurns[1]; got != 2 {
		t.Errorf("two loaded-silent upkeeps must yield streak=2; got %d", got)
	}
}

// TestBluff_InteractionCastResetsStreak: build a 3-turn streak, then
// observe an interaction cast — streak must drop to 0.
func TestBluff_InteractionCastResetsStreak(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	h.opponentLoadedSilentTurns[1] = 3

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "cast", Seat: 1, Source: "Counterspell",
	})

	if got := h.opponentLoadedSilentTurns[1]; got != 0 {
		t.Errorf("interaction cast must reset streak to 0; got %d", got)
	}
	if !h.opponentFiredInteractionThisRound[1] {
		t.Error("interaction cast must mark the per-round fired flag")
	}
}

// TestBluff_NonInteractionCastDoesNotResetStreak: an opponent casting
// Llanowar Elves while sitting on Counterspell mana doesn't tell us
// they have nothing — we shouldn't reset the streak.
func TestBluff_NonInteractionCastDoesNotResetStreak(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	h.opponentLoadedSilentTurns[1] = 2

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "cast", Seat: 1, Source: "Llanowar Elves",
	})

	if got := h.opponentLoadedSilentTurns[1]; got != 2 {
		t.Errorf("non-interaction cast must not reset streak; got %d (want 2)", got)
	}
}

// TestBluff_StreakDoesNotGrowWhenInteractionFired: opponent fires
// interaction during a round; the upkeep evaluation that follows
// must NOT bump the streak (the flag was set, fired-this-round=true).
func TestBluff_StreakDoesNotGrowWhenInteractionFired(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	// They fire interaction this round.
	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "cast", Seat: 1, Source: "Counterspell",
	})
	// Then upkeep evaluates — must not bump streak.
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})

	if got := h.opponentLoadedSilentTurns[1]; got != 0 {
		t.Errorf("streak must stay 0 when opponent fired interaction in the round; got %d", got)
	}
}

// TestBluff_StreakCappedAtMax: bumping past the cap must clamp.
func TestBluff_StreakCappedAtMax(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	for i := 0; i < bluffMaxStreak+10; i++ {
		h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})
	}

	if got := h.opponentLoadedSilentTurns[1]; got != bluffMaxStreak {
		t.Errorf("streak must clamp at bluffMaxStreak=%d; got %d", bluffMaxStreak, got)
	}
}

// TestBluff_GameStartResetsStreak: per-game state must reset cleanly.
func TestBluff_GameStartResetsStreak(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	h.opponentLoadedSilentTurns[1] = 3
	h.opponentFiredInteractionThisRound[1] = true

	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	if got := h.opponentLoadedSilentTurns[1]; got != 0 {
		t.Errorf("game_start must reset loaded-silent streak; got %d", got)
	}
	if h.opponentFiredInteractionThisRound[1] {
		t.Error("game_start must reset fired-this-round flag")
	}
}

// TestBluff_TableInteractionRiskConsumesDampening: with a stale 4-turn
// bluff streak on a single opponent, the table-wide risk surface must
// reflect the dampening (drops vs the same setup with streak=0).
func TestBluff_TableInteractionRiskConsumesDampening(t *testing.T) {
	gs, h := loadedOpponentSetup(t)

	riskFresh := h.tableInteractionRisk(gs, 0)
	if riskFresh == 0 {
		t.Fatal("test setup must produce a non-zero risk")
	}

	h.opponentLoadedSilentTurns[1] = bluffMaxStreak
	riskStale := h.tableInteractionRisk(gs, 0)

	if riskStale >= riskFresh {
		t.Errorf("stale bluff streak must lower table-wide interaction risk; fresh=%.3f stale=%.3f",
			riskFresh, riskStale)
	}
	if riskStale < riskFresh*bluffFloor-0.01 {
		t.Errorf("dampening must not fall below bluffFloor=%.2f * raw; fresh=%.3f stale=%.3f",
			bluffFloor, riskFresh, riskStale)
	}
}
