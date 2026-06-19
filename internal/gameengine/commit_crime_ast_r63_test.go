package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// commit_crime_ast_r63_test.go — Commit a crime (CR §701.71) generic AST
// trigger dispatch. Crime DETECTION + the per-turn counter were already
// wired, but "whenever you commit a crime" AST triggers with no bespoke
// per_card handler (the ~18-card dominant group) never resolved their
// effect — FireCommitsCrimeTriggers reached the per_card registry alone.

func crimeGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(11)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func crimeDrain(gs *GameState) {
	for i := 0; i < 40 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// crimeWatcherAST: "whenever you commit a crime, you gain 3 life" (a clean,
// observable stand-in for Hardbristle Bandit / Vadmir / Marauding Sphinx).
func crimeWatcherAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Crime Watcher",
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "commit_crime"},
				Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 3}, Target: gameast.Filter{Base: "controller"}},
				Raw:     "whenever you commit a crime, you gain 3 life",
			},
		},
	}
}

// (1) A committed crime fires the controller's "whenever you commit a crime"
// AST trigger (was inert — only the counter bumped).
func TestCommitCrime_ASTTriggerFires(t *testing.T) {
	gs := crimeGame(t)
	addBattlefieldWithAST(gs, 0, "Crime Watcher", 2, 2, crimeWatcherAST(), "creature")
	life := gs.Seats[0].Life

	// Simulate a damage/removal spell that targeted an opponent's creature.
	victim := addBattlefield(gs, 1, "Opp Creature", 2, 2, "creature")
	FireCrimeIfTargetingOpponent(gs, 0, "Murder", []Target{{Kind: TargetKindPermanent, Permanent: victim}})
	crimeDrain(gs)

	if got := gs.Seats[0].Life - life; got != 3 {
		t.Fatalf("commit-crime AST trigger should have gained 3 life, lifeΔ=%d", got)
	}
	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatalf("expected 1 crime this turn, got %d", SeatCrimeCountThisTurn(gs, 0))
	}
}

// (2) Targeting only your OWN stuff is not a crime — no trigger, no count.
func TestCommitCrime_NoCrimeOnSelfTarget(t *testing.T) {
	gs := crimeGame(t)
	addBattlefieldWithAST(gs, 0, "Crime Watcher", 2, 2, crimeWatcherAST(), "creature")
	life := gs.Seats[0].Life

	own := addBattlefield(gs, 0, "Own Creature", 2, 2, "creature")
	fired := FireCrimeIfTargetingOpponent(gs, 0, "Friendly Buff", []Target{{Kind: TargetKindPermanent, Permanent: own}})
	crimeDrain(gs)

	if fired {
		t.Fatal("targeting your own permanent must not commit a crime")
	}
	if gs.Seats[0].Life != life {
		t.Fatalf("no crime should mean no life gain, lifeΔ=%d", gs.Seats[0].Life-life)
	}
	if SeatCrimeCountThisTurn(gs, 0) != 0 {
		t.Fatal("self-target must not bump the crime counter")
	}
}

// (3) IsCrimeTarget classifies all four §701.71a opponent-object kinds, and
// rejects self-controlled objects.
func TestCommitCrime_IsCrimeTargetClassification(t *testing.T) {
	gs := crimeGame(t)
	oppPerm := addBattlefield(gs, 1, "Opp Perm", 1, 1, "creature")
	myPerm := addBattlefield(gs, 0, "My Perm", 1, 1, "creature")

	cases := []struct {
		name string
		t    Target
		want bool
	}{
		{"opponent seat", Target{Kind: TargetKindSeat, Seat: 1}, true},
		{"own seat", Target{Kind: TargetKindSeat, Seat: 0}, false},
		{"opponent permanent", Target{Kind: TargetKindPermanent, Permanent: oppPerm}, true},
		{"own permanent", Target{Kind: TargetKindPermanent, Permanent: myPerm}, false},
		{"opponent stack item", Target{Kind: TargetKindStackItem, Stack: &StackItem{Controller: 1}}, true},
		{"opponent graveyard card", Target{Kind: TargetKindCard, Seat: 1, Card: &Card{Name: "X"}}, true},
		{"own graveyard card", Target{Kind: TargetKindCard, Seat: 0, Card: &Card{Name: "X"}}, false},
	}
	for _, c := range cases {
		if got := IsCrimeTarget(gs, 0, c.t); got != c.want {
			t.Errorf("%s: IsCrimeTarget = %v, want %v", c.name, got, c.want)
		}
	}
}

// (4) CR §701.71b — one crime per spell even if it targets several opponent
// objects: the trigger fires ONCE (3 life, not 6) and the counter is 1.
func TestCommitCrime_OncePerResolution(t *testing.T) {
	gs := crimeGame(t)
	addBattlefieldWithAST(gs, 0, "Crime Watcher", 2, 2, crimeWatcherAST(), "creature")
	life := gs.Seats[0].Life

	a := addBattlefield(gs, 1, "Opp A", 2, 2, "creature")
	b := addBattlefield(gs, 1, "Opp B", 2, 2, "creature")
	FireCrimeIfTargetingOpponent(gs, 0, "Multi Target", []Target{
		{Kind: TargetKindPermanent, Permanent: a},
		{Kind: TargetKindPermanent, Permanent: b},
	})
	crimeDrain(gs)

	if got := gs.Seats[0].Life - life; got != 3 {
		t.Fatalf("multi-opponent-target spell should commit ONE crime (3 life), lifeΔ=%d", got)
	}
	if SeatCrimeCountThisTurn(gs, 0) != 1 {
		t.Fatalf("two opponent targets is still one crime, got count %d", SeatCrimeCountThisTurn(gs, 0))
	}
}
