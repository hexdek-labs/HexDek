package gameengine

// Crime tests. CR §700.13: a player commits a crime as they cast a spell,
// activate an ability, or put a triggered ability on the stack that targets
// at least one opponent; a permanent, spell, or ability an opponent controls;
// and/or a card in an opponent's graveyard.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Crime tests
// ---------------------------------------------------------------------------

func newCrimeGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(55)), nil)
}

// ---------------------------------------------------------------------------
// (e) HasCommitsCrimeTrigger detector
// ---------------------------------------------------------------------------

func TestHasCommitsCrimeTrigger_DetectsCanonicalPhrasings(t *testing.T) {
	cases := []struct {
		name       string
		oracleText string
		want       bool
	}{
		{
			name:       "whenever you commit a crime",
			oracleText: "Whenever you commit a crime, create a tapped Treasure token.",
			want:       true,
		},
		{
			name:       "first crime each turn",
			oracleText: "When you commit your first crime each turn, draw a card.",
			want:       true,
		},
		{
			name:       "if you've committed a crime this turn",
			oracleText: "If you've committed a crime this turn, this creature gets +2/+0.",
			want:       true,
		},
		{
			name:       "vanilla creature no crime",
			oracleText: "Flying.",
			want:       false,
		},
		{
			name:       "talks about crime in flavor but no trigger",
			oracleText: "Sacrifice a creature. (This is not a crime.)",
			want:       false,
		},
		{
			name:       "empty oracle",
			oracleText: "",
			want:       false,
		},
	}
	for _, c := range cases {
		card := &Card{
			Name: c.name,
			AST:  &gameast.CardAST{Name: c.name},
		}
		// Prime the oracle-text cache directly — same package, so we can
		// reach unexported fields without re-running the full AST parser.
		card.OracleTextCache = c.oracleText
		card.oracleTextReady = true

		got := HasCommitsCrimeTrigger(card)
		if got != c.want {
			t.Errorf("%s: HasCommitsCrimeTrigger = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHasCommitsCrimeTrigger_NilSafe(t *testing.T) {
	if HasCommitsCrimeTrigger(nil) {
		t.Fatal("HasCommitsCrimeTrigger(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// (a) Cast spell targeting opponent's perm = crime
// ---------------------------------------------------------------------------

func TestFireCrime_TargetingOpponentPermanentCommitsCrime(t *testing.T) {
	gs := newCrimeGame(t)
	enemy := addBattlefield(gs, 1, "Enemy Beast", 3, 3, "creature")

	target := Target{Kind: TargetKindPermanent, Permanent: enemy}
	if !IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting an opponent's permanent must be a crime")
	}

	count := FireCommitsCrimeTriggers(gs, 0, "Murder", enemy.Card.DisplayName())
	if count != 1 {
		t.Fatalf("expected first crime count=1, got %d", count)
	}
	if !SeatHasCommittedCrimeThisTurn(gs, 0) {
		t.Fatal("seat 0 should register a crime this turn")
	}
	if SeatHasCommittedCrimeThisTurn(gs, 1) {
		t.Fatal("seat 1 must not register a crime — only the actor does")
	}
}

func TestFireCrime_TargetingOpponentSeatCommitsCrime(t *testing.T) {
	gs := newCrimeGame(t)
	target := Target{Kind: TargetKindSeat, Seat: 1}
	if !IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting an opponent (player) must be a crime")
	}
}

func TestFireCrime_TargetingOpponentSpellOnStackCommitsCrime(t *testing.T) {
	gs := newCrimeGame(t)
	opponentSpell := &StackItem{
		Card:       &Card{Name: "Opponent's Spell"},
		Controller: 1,
	}
	target := Target{Kind: TargetKindStackItem, Stack: opponentSpell}
	if !IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting an opponent-controlled spell on the stack must be a crime")
	}
}

// ---------------------------------------------------------------------------
// (b) Targeting own perm = not crime
// ---------------------------------------------------------------------------

func TestFireCrime_TargetingOwnPermanentNotCrime(t *testing.T) {
	gs := newCrimeGame(t)
	own := addBattlefield(gs, 0, "My Beast", 3, 3, "creature")

	target := Target{Kind: TargetKindPermanent, Permanent: own}
	if IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting your own permanent is NOT a crime")
	}

	// Bulk path: should fire zero crimes.
	fired := FireCrimeIfTargetingOpponent(gs, 0, "Giant Growth", []Target{target})
	if fired {
		t.Fatal("FireCrimeIfTargetingOpponent must return false when all targets are self")
	}
	if SeatHasCommittedCrimeThisTurn(gs, 0) {
		t.Fatal("no crime should be recorded for self-targets")
	}
}

func TestFireCrime_TargetingOwnSeatNotCrime(t *testing.T) {
	gs := newCrimeGame(t)
	target := Target{Kind: TargetKindSeat, Seat: 0}
	if IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting yourself is NOT a crime")
	}
}

func TestFireCrime_OwnSpellOnStackNotCrime(t *testing.T) {
	gs := newCrimeGame(t)
	mySpell := &StackItem{Card: &Card{Name: "My Spell"}, Controller: 0}
	target := Target{Kind: TargetKindStackItem, Stack: mySpell}
	if IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting your own spell on the stack is NOT a crime")
	}
}

// ---------------------------------------------------------------------------
// (c) Targeting card in opponent's graveyard = crime
// ---------------------------------------------------------------------------

func TestFireCrime_TargetingCardInOpponentGraveyardCommitsCrime(t *testing.T) {
	gs := newCrimeGame(t)
	// A card sitting in seat 1's graveyard.
	gravesite := &Card{Name: "Yawgmoth's Will"}
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, gravesite)

	target := Target{
		Kind: TargetKindCard,
		Card: gravesite,
		Seat: 1, // owner of the zone
	}
	if !IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting a card in an opponent's graveyard must be a crime")
	}

	fired := FireCrimeIfTargetingOpponent(gs, 0, "Surgical Extraction", []Target{target})
	if !fired {
		t.Fatal("FireCrimeIfTargetingOpponent must fire for opp graveyard card")
	}
	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatalf("expected 1 crime, got %d", SeatCrimeCountThisTurn(gs, 0))
	}
}

func TestFireCrime_CardInOwnGraveyardNotCrime(t *testing.T) {
	gs := newCrimeGame(t)
	own := &Card{Name: "Reanimate-target"}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, own)

	target := Target{Kind: TargetKindCard, Card: own, Seat: 0}
	if IsCrimeTarget(gs, 0, target) {
		t.Fatal("§700.13: targeting your own graveyard is NOT a crime")
	}
}

// ---------------------------------------------------------------------------
// (d) "First crime each turn" fires once per turn
// ---------------------------------------------------------------------------

func TestFireCrime_FirstCrimeTriggerFiresOnce(t *testing.T) {
	gs := newCrimeGame(t)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	firstCount := 0
	crimeCount := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		switch ev {
		case "first_crime":
			firstCount++
		case "crime":
			crimeCount++
		}
	}

	// Commit three crimes on the same turn.
	FireCommitsCrimeTriggers(gs, 0, "Spell A", "target")
	FireCommitsCrimeTriggers(gs, 0, "Spell B", "target")
	FireCommitsCrimeTriggers(gs, 0, "Spell C", "target")

	if crimeCount != 3 {
		t.Fatalf("crime trigger should fire on every crime; got %d, want 3", crimeCount)
	}
	if firstCount != 1 {
		t.Fatalf("first_crime trigger should fire exactly once per turn; got %d, want 1", firstCount)
	}
	if SeatCrimeCountThisTurn(gs, 0) != 3 {
		t.Fatalf("crime count = %d, want 3", SeatCrimeCountThisTurn(gs, 0))
	}
}

func TestFireCrime_FirstCrimeResetsOnTurnStart(t *testing.T) {
	gs := newCrimeGame(t)
	FireCommitsCrimeTriggers(gs, 0, "Spell A", "tgt")
	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatal("setup: expected 1 crime")
	}

	// Turn-start reset clears Turn counters.
	UntapAll(gs, 0)

	if SeatCrimeCountThisTurn(gs, 0) != 0 {
		t.Fatalf("UntapAll must clear CommittedCrimes; got %d", SeatCrimeCountThisTurn(gs, 0))
	}
	if SeatHasCommittedCrimeThisTurn(gs, 0) {
		t.Fatal("SeatHasCommittedCrimeThisTurn must be false after turn-start reset")
	}

	// On the new turn, a crime should fire the "first_crime" trigger again.
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	firstCount := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "first_crime" {
			firstCount++
		}
	}
	FireCommitsCrimeTriggers(gs, 0, "Spell B", "tgt")
	if firstCount != 1 {
		t.Fatalf("first_crime should re-arm on the new turn; got %d, want 1", firstCount)
	}
}

// ---------------------------------------------------------------------------
// Per-seat isolation
// ---------------------------------------------------------------------------

func TestFireCrime_PerSeatIsolation(t *testing.T) {
	gs := newCrimeGame(t)
	FireCommitsCrimeTriggers(gs, 0, "S0 spell", "tgt")
	FireCommitsCrimeTriggers(gs, 0, "S0 spell", "tgt")
	FireCommitsCrimeTriggers(gs, 1, "S1 spell", "tgt")

	if SeatCrimeCountThisTurn(gs, 0) != 2 {
		t.Fatalf("seat 0 crime count = %d, want 2", SeatCrimeCountThisTurn(gs, 0))
	}
	if SeatCrimeCountThisTurn(gs, 1) != 1 {
		t.Fatalf("seat 1 crime count = %d, want 1", SeatCrimeCountThisTurn(gs, 1))
	}
}

// ---------------------------------------------------------------------------
// Bulk-target: one crime per resolution
// ---------------------------------------------------------------------------

func TestFireCrime_MultiTargetSpellFiresOnceCrime(t *testing.T) {
	gs := newCrimeGame(t)
	e1 := addBattlefield(gs, 1, "Enemy 1", 2, 2, "creature")
	e2 := addBattlefield(gs, 1, "Enemy 2", 2, 2, "creature")
	mine := addBattlefield(gs, 0, "Mine", 2, 2, "creature")

	targets := []Target{
		{Kind: TargetKindPermanent, Permanent: e1},
		{Kind: TargetKindPermanent, Permanent: e2}, // also opp
		{Kind: TargetKindPermanent, Permanent: mine},
	}
	fired := FireCrimeIfTargetingOpponent(gs, 0, "Hurricane", targets)
	if !fired {
		t.Fatal("bulk targeting should fire a crime when ANY target is opp-controlled")
	}
	// At most one crime per resolution regardless of target count.
	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatalf("multi-target spell must commit one crime; got %d", SeatCrimeCountThisTurn(gs, 0))
	}
}

func TestFireCrime_BulkAllSelfTargetsNoCrime(t *testing.T) {
	gs := newCrimeGame(t)
	a := addBattlefield(gs, 0, "Mine A", 2, 2, "creature")
	b := addBattlefield(gs, 0, "Mine B", 2, 2, "creature")
	targets := []Target{
		{Kind: TargetKindPermanent, Permanent: a},
		{Kind: TargetKindPermanent, Permanent: b},
	}
	if FireCrimeIfTargetingOpponent(gs, 0, "Self-buff", targets) {
		t.Fatal("self-only targeting must not fire a crime")
	}
	if SeatCrimeCountThisTurn(gs, 0) != 0 {
		t.Fatal("no crime should be recorded for self-only targeting")
	}
}

// ---------------------------------------------------------------------------
// Event log inspection
// ---------------------------------------------------------------------------

func TestFireCrime_EmitsStructuredEvent(t *testing.T) {
	gs := newCrimeGame(t)
	before := len(gs.EventLog)
	FireCommitsCrimeTriggers(gs, 0, "Murder", "Enemy Beast")
	var found *Event
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "commit_crime" {
			found = &gs.EventLog[i]
			break
		}
	}
	if found == nil {
		t.Fatal("FireCommitsCrimeTriggers should emit a commit_crime event")
	}
	if found.Seat != 0 {
		t.Fatalf("event.Seat = %d, want 0", found.Seat)
	}
	if found.Source != "Murder" {
		t.Fatalf("event.Source = %q, want \"Murder\"", found.Source)
	}
	if found.Amount != 1 {
		t.Fatalf("event.Amount = %d, want 1 (first crime)", found.Amount)
	}
	if tgt, ok := found.Details["target"].(string); !ok || tgt != "Enemy Beast" {
		t.Fatalf("event.Details[target] = %v, want \"Enemy Beast\"", found.Details["target"])
	}
}

// ---------------------------------------------------------------------------
// Integration: maybeFireCrime (resolve.go) routes through the new surface
// ---------------------------------------------------------------------------

func TestMaybeFireCrime_DelegatesToCanonicalSurface(t *testing.T) {
	gs := newCrimeGame(t)
	caster := addBattlefield(gs, 0, "Caster", 0, 0, "creature")
	enemy := addBattlefield(gs, 1, "Enemy", 2, 2, "creature")

	targets := []Target{{Kind: TargetKindPermanent, Permanent: enemy}}
	maybeFireCrime(gs, caster, targets)

	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatal("maybeFireCrime should bump the canonical CommittedCrimes counter")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestFireCrime_NilSafe(t *testing.T) {
	gs := newCrimeGame(t)
	if FireCommitsCrimeTriggers(nil, 0, "x", "y") != 0 {
		t.Fatal("nil game should return 0")
	}
	if FireCommitsCrimeTriggers(gs, -1, "x", "y") != 0 {
		t.Fatal("invalid seat should return 0")
	}
	if FireCommitsCrimeTriggers(gs, 99, "x", "y") != 0 {
		t.Fatal("out-of-range seat should return 0")
	}
	if SeatCrimeCountThisTurn(nil, 0) != 0 {
		t.Fatal("nil game accessor must be safe")
	}
	if SeatHasCommittedCrimeThisTurn(nil, 0) {
		t.Fatal("nil game accessor must be safe")
	}
	if IsCrimeTarget(nil, 0, Target{}) {
		t.Fatal("nil game IsCrimeTarget must be false")
	}
	if FireCrimeIfTargetingOpponent(nil, 0, "x", nil) {
		t.Fatal("nil game bulk must be safe")
	}
	if FireCrimeIfTargetingOpponent(gs, 0, "x", nil) {
		t.Fatal("empty targets must not fire a crime")
	}
}
