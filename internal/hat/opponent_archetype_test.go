package hat

// Tests for the OpponentProfile classifier added in
// dev/opponent-archetype. Each test drives recordOpponentPlay
// directly to set up the per-event tallies, then asserts
// classifyOpponent's output (Archetype + Confidence + ThreatLevel),
// or invokes the decision functions to verify integration.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// helper: spin up a YggdrasilHat with the per-opponent slice already
// initialized so recordOpponentPlay / classifyOpponent don't have to
// wait for the first ObserveEvent to allocate.
func primedYggdrasilHat(seats int) *YggdrasilHat {
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	h.seatCount = seats
	h.opponentProfiles = make([]*OpponentProfile, seats)
	for i := 0; i < seats; i++ {
		h.opponentProfiles[i] = &OpponentProfile{Archetype: "unknown"}
	}
	h.opponentHeldMana = make([]int, seats)
	h.opponentTutored = make([]bool, seats)
	h.damageDealtTo = make([]int, seats)
	h.damageReceivedFrom = make([]int, seats)
	h.spellsCastBy = make([]int, seats)
	h.cardsSeen = make([]map[string]int, seats)
	h.opponentColors = make([]map[string]bool, seats)
	h.opponentHandEntropy = make([]float64, seats)
	h.opponentKnownCards = make([]map[string]bool, seats)
	h.lastAttackedUsTurn = make([]int, seats)
	h.poisonReceivedFrom = make([]int, seats)
	h.kingmakerTurn = make([]int, seats)
	h.threatTrajectory = make([][]int, seats)
	h.lastTurnBoardPower = make([]int, seats)
	h.politicalGraph = make([][]int, seats)
	h.linkedExilesByOpponent = make([]int, seats)
	h.perceivedArchetype = make([]string, seats)
	for i := 0; i < seats; i++ {
		h.cardsSeen[i] = make(map[string]int)
		h.opponentColors[i] = make(map[string]bool)
		h.politicalGraph[i] = make([]int, seats)
		h.opponentKnownCards[i] = make(map[string]bool)
	}
	return h
}

// ---------------------------------------------------------------------
// classifyOpponent — archetype detection
// ---------------------------------------------------------------------

func TestClassifyOpponent_AggroByCreatureCount(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 3
	h := primedYggdrasilHat(2)

	// Three creature casts by turn 3.
	for i := 0; i < 3; i++ {
		card := newTestCardMinimal("Goblin Guide", []string{"creature"}, 1, nil)
		h.recordOpponentPlay("cast", card.DisplayName(), 1, card, gs.Turn)
	}

	prof := h.classifyOpponent(gs, 1)
	if prof == nil {
		t.Fatalf("nil profile")
	}
	if prof.Archetype != "aggro" {
		t.Errorf("archetype=%q, want aggro", prof.Archetype)
	}
	if prof.Confidence < 0.55 || prof.Confidence > 0.95 {
		t.Errorf("confidence=%.2f, want in [0.55, 0.95]", prof.Confidence)
	}
}

func TestClassifyOpponent_ComboByTutorAndHeldMana(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	h := primedYggdrasilHat(2)

	// Tutor used + sandbagging mana for 2 turns + light creature board.
	h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, gs.Turn)
	h.opponentHeldMana[1] = 3
	// One creature on the board, no more.
	creature := newTestCardMinimal("Mox Bot", []string{"creature"}, 0, nil)
	h.recordOpponentPlay("cast", creature.DisplayName(), 1, creature, gs.Turn)

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "combo" {
		t.Errorf("archetype=%q, want combo", prof.Archetype)
	}
	if prof.Confidence < 0.65 {
		t.Errorf("confidence=%.2f, want >=0.65", prof.Confidence)
	}
}

func TestClassifyOpponent_ControlByRemovalPattern(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 6
	h := primedYggdrasilHat(2)

	// Two removal spells (oracle text contains "destroy target") + a
	// counterspell + light creature presence.
	swordsAST := newTestCardMinimal("Swords to Plowshares", []string{"instant", "oracle:exile target creature"}, 1, nil)
	swordsAST.Types = append(swordsAST.Types, "oracle:exile target creature")
	// Use a synthetic helper to inject an oracle string the hat can read.
	// gameengine.OracleTextLower walks AST + Types tokens, so attaching
	// an "oracle:..." pseudo-type is a stable way to seed text in tests.
	doomBlade := newTestCardMinimal("Doom Blade", []string{"instant"}, 2, nil)
	doomBlade.Types = append(doomBlade.Types, "oracle:destroy target creature")
	counterspell := newTestCardMinimal("Counterspell", []string{"instant", "counter target spell"}, 2, nil)
	counterspell.Types = append(counterspell.Types, "oracle:counter target spell")

	for _, c := range []*gameengine.Card{swordsAST, doomBlade, counterspell} {
		h.recordOpponentPlay("cast", c.DisplayName(), 1, c, gs.Turn)
	}

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "control" {
		t.Errorf("archetype=%q, want control (removal=%d, counters=%d, creatures=%d)",
			prof.Archetype, prof.RemovalUsed, prof.CountersUsed, prof.CreaturesPlayed)
	}
}

func TestClassifyOpponent_ConfidenceIncreasesOverTime(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 3
	h := primedYggdrasilHat(2)

	for i := 0; i < 3; i++ {
		card := newTestCardMinimal("Goblin Guide", []string{"creature"}, 1, nil)
		h.recordOpponentPlay("cast", card.DisplayName(), 1, card, gs.Turn)
	}

	prof1 := h.classifyOpponent(gs, 1)
	if prof1.Archetype != "aggro" {
		t.Fatalf("setup: want aggro, got %q", prof1.Archetype)
	}
	conf1 := prof1.Confidence

	// Same archetype another turn — confidence should ratchet upward.
	gs.Turn = 4
	// Another creature reinforces.
	card := newTestCardMinimal("Lightning Bolt-Hands", []string{"creature"}, 1, nil)
	h.recordOpponentPlay("cast", card.DisplayName(), 1, card, gs.Turn)
	prof2 := h.classifyOpponent(gs, 1)
	conf2 := prof2.Confidence

	if conf2 <= conf1 {
		t.Errorf("confidence should grow turn-over-turn: turn3=%.2f turn4=%.2f", conf1, conf2)
	}
	if conf2 > 0.95 {
		t.Errorf("confidence cap broken: %.2f", conf2)
	}
}

func TestClassifyOpponent_UnknownByDefault(t *testing.T) {
	gs := newTestGame(t, 2)
	h := primedYggdrasilHat(2)

	prof := h.classifyOpponent(gs, 1)
	if prof.Archetype != "unknown" {
		t.Errorf("default archetype=%q, want unknown", prof.Archetype)
	}
	if prof.Confidence != 0 {
		t.Errorf("default confidence=%.2f, want 0", prof.Confidence)
	}
}

// ---------------------------------------------------------------------
// ThreatLevel
// ---------------------------------------------------------------------

func TestClassifyOpponent_ComboThreatHigherThanIdle(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Turn = 5
	gs.Seats[1].Life = 40

	h := primedYggdrasilHat(2)
	h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, gs.Turn)
	h.recordOpponentPlay("tutor", "Vampiric Tutor", 1, nil, gs.Turn)
	h.opponentHeldMana[1] = 3
	creature := newTestCardMinimal("Mox Bot", []string{"creature"}, 0, nil)
	h.recordOpponentPlay("cast", creature.DisplayName(), 1, creature, gs.Turn)
	combo := h.classifyOpponent(gs, 1)

	// Reset for an "idle" opponent (no plays).
	h2 := primedYggdrasilHat(2)
	idle := h2.classifyOpponent(gs, 1)

	if combo.ThreatLevel <= idle.ThreatLevel {
		t.Errorf("combo threat (%.2f) should exceed idle (%.2f)",
			combo.ThreatLevel, idle.ThreatLevel)
	}
}

// ---------------------------------------------------------------------
// Decision-function integration
// ---------------------------------------------------------------------

// bestTarget should bias toward a confident-combo opponent over a
// confident-control opponent when life / board state are otherwise
// equal.
func TestBestTarget_PrefersComboOpponent(t *testing.T) {
	gs := newTestGame(t, 3)
	gs.Turn = 5
	h := primedYggdrasilHat(3)

	// Match life and board so politics doesn't dominate.
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 30

	// Seat 1 = confident combo (tutor + held mana).
	h.recordOpponentPlay("tutor", "Demonic Tutor", 1, nil, gs.Turn)
	h.recordOpponentPlay("tutor", "Vampiric Tutor", 1, nil, gs.Turn)
	h.opponentHeldMana[1] = 3

	// Seat 2 = confident control (3 removal).
	for _, name := range []string{"Doom Blade", "Swords to Plowshares", "Counterspell"} {
		c := newTestCardMinimal(name, []string{"instant"}, 2, nil)
		c.Types = append(c.Types, "oracle:destroy target creature")
		h.recordOpponentPlay("cast", c.DisplayName(), 2, c, gs.Turn)
	}

	// Warm the classifier (it caches by seat).
	combo := h.classifyOpponent(gs, 1)
	control := h.classifyOpponent(gs, 2)
	if combo.Archetype != "combo" || control.Archetype != "control" {
		t.Fatalf("setup: combo=%q control=%q", combo.Archetype, control.Archetype)
	}

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 2, 2)

	got := h.ChooseAttackTarget(gs, 0, atk, []int{1, 2})
	if got != 1 {
		t.Fatalf("bestTarget should prefer combo seat 1 over control seat 2; got seat %d", got)
	}
}
