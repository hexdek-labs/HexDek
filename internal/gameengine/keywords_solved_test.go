package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Solved tests — CR §702.169
// ---------------------------------------------------------------------------

func newSolvedGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(44))
	return NewGameState(2, rng, nil)
}

func newCaseEnchantment(name string, owner int) *Permanent {
	card := &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"enchantment", "case"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
	return &Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Flags:      map[string]int{},
	}
}

func TestIsSolved_DefaultsFalse(t *testing.T) {
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)
	if IsSolved(perm) {
		t.Fatal("a freshly created case should not be solved")
	}
}

func TestIsSolved_NilSafe(t *testing.T) {
	if IsSolved(nil) {
		t.Fatal("IsSolved(nil) should be false")
	}
}

func TestMarkSolved_SetsFlag(t *testing.T) {
	gs := newSolvedGame(t)
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)
	MarkSolved(gs, perm)
	if !IsSolved(perm) {
		t.Fatal("permanent should be solved after MarkSolved")
	}
	if perm.Flags["solved"] != 1 {
		t.Fatalf("perm.Flags[\"solved\"] = %d, want 1", perm.Flags["solved"])
	}
}

func TestMarkSolved_EmitsEvent(t *testing.T) {
	gs := newSolvedGame(t)
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)
	before := len(gs.EventLog)
	MarkSolved(gs, perm)
	var found *Event
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "became_solved" {
			found = &gs.EventLog[i]
			break
		}
	}
	if found == nil {
		t.Fatal("MarkSolved should emit a became_solved event")
	}
	if found.Source != "Case of the Pilfered Proof" {
		t.Fatalf("event Source = %q, want \"Case of the Pilfered Proof\"", found.Source)
	}
	if found.Seat != 0 {
		t.Fatalf("event Seat = %d, want 0 (perm.Controller)", found.Seat)
	}
}

func TestMarkSolved_Idempotent(t *testing.T) {
	gs := newSolvedGame(t)
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)
	MarkSolved(gs, perm)
	before := len(gs.EventLog)
	MarkSolved(gs, perm)
	// A second call should not emit a second became_solved event.
	for i := before; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Kind == "became_solved" {
			t.Fatal("MarkSolved should not re-emit became_solved on an already-solved perm")
		}
	}
	if !IsSolved(perm) {
		t.Fatal("perm should remain solved")
	}
}

func TestMarkSolved_FiresTriggerHook(t *testing.T) {
	gs := newSolvedGame(t)
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var observed string
	var observedPerm *Permanent
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "became_solved" {
			observed = ev
			if p, ok := ctx["perm"].(*Permanent); ok {
				observedPerm = p
			}
		}
	}

	MarkSolved(gs, perm)
	if observed != "became_solved" {
		t.Fatalf("TriggerHook did not see became_solved event (got %q)", observed)
	}
	if observedPerm != perm {
		t.Fatal("TriggerHook context did not carry the solved perm pointer")
	}
}

func TestMarkSolved_NilSafe(t *testing.T) {
	gs := newSolvedGame(t)
	MarkSolved(gs, nil) // must not panic
	MarkSolved(nil, newCaseEnchantment("Case", 0))
}

func TestClearSolved_RemovesFlag(t *testing.T) {
	gs := newSolvedGame(t)
	perm := newCaseEnchantment("Case of the Pilfered Proof", 0)
	MarkSolved(gs, perm)
	ClearSolved(perm)
	if IsSolved(perm) {
		t.Fatal("ClearSolved should remove the solved designation")
	}
	if _, ok := perm.Flags["solved"]; ok {
		t.Fatal("perm.Flags[\"solved\"] should be deleted, not zeroed")
	}
}

func TestHasSolveAbility_KeywordRouted(t *testing.T) {
	card := &Card{
		Name:  "Case of the Pilfered Proof",
		Owner: 0,
		Types: []string{"enchantment", "case"},
		AST: &gameast.CardAST{
			Name: "Case of the Pilfered Proof",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "solved"},
			},
		},
	}
	if !HasSolveAbility(card) {
		t.Fatal("HasSolveAbility should return true for a card with the solved keyword")
	}
	if HasSolveAbility(&Card{AST: &gameast.CardAST{}}) {
		t.Fatal("HasSolveAbility should return false for a card without the solved keyword")
	}
}
