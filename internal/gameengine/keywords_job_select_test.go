package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Job Select tests — CR §702.182 (Aetherdrift)
// ---------------------------------------------------------------------------

func newJobSelectGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(7))
	return NewGameState(2, rng, nil)
}

func newJobSelectCard(name string, jobs ...string) *Card {
	args := make([]any, len(jobs))
	for i, j := range jobs {
		args[i] = j
	}
	return &Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "job select", Args: args},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// HasJobSelect
// ---------------------------------------------------------------------------

func TestHasJobSelect_Detects(t *testing.T) {
	card := newJobSelectCard("Aerial Speedster", "Pilot", "Captain")
	if !HasJobSelect(card) {
		t.Fatal("HasJobSelect should be true for a job-select card")
	}
}

func TestHasJobSelect_Negative(t *testing.T) {
	card := &Card{
		Name: "Vanilla",
		AST:  &gameast.CardAST{Name: "Vanilla", Abilities: []gameast.Ability{}},
	}
	if HasJobSelect(card) {
		t.Fatal("HasJobSelect should be false on a card without the keyword")
	}
}

func TestHasJobSelect_Nil(t *testing.T) {
	if HasJobSelect(nil) {
		t.Fatal("HasJobSelect(nil) must return false")
	}
}

// ---------------------------------------------------------------------------
// JobSelectOptions
// ---------------------------------------------------------------------------

func TestJobSelectOptions_NormalizesCase(t *testing.T) {
	card := newJobSelectCard("Pilotship", "Pilot", "CAPTAIN", " Gunner ")
	opts := JobSelectOptions(card)
	want := []string{"pilot", "captain", "gunner"}
	if len(opts) != len(want) {
		t.Fatalf("options len = %d, want %d (got %v)", len(opts), len(want), opts)
	}
	for i, w := range want {
		if opts[i] != w {
			t.Errorf("opts[%d] = %q, want %q", i, opts[i], w)
		}
	}
}

func TestJobSelectOptions_NoKeyword(t *testing.T) {
	card := &Card{AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	if opts := JobSelectOptions(card); opts != nil {
		t.Fatalf("JobSelectOptions = %v, want nil for non-job-select card", opts)
	}
}

// ---------------------------------------------------------------------------
// SetJobSelectChoice / JobSelectChoice — permanent-tracking
// ---------------------------------------------------------------------------

func TestJobSelectChoice_RoundTrip(t *testing.T) {
	p := &Permanent{Card: newJobSelectCard("Ship", "Pilot", "Captain")}
	if got := JobSelectChoice(p); got != "" {
		t.Fatalf("initial JobSelectChoice = %q, want empty", got)
	}
	SetJobSelectChoice(p, "Pilot")
	if got := JobSelectChoice(p); got != "pilot" {
		t.Fatalf("after SetJobSelectChoice(Pilot): %q, want \"pilot\"", got)
	}
}

func TestJobSelectChoice_ReassignmentClearsPrior(t *testing.T) {
	p := &Permanent{Card: newJobSelectCard("Ship", "Pilot", "Captain")}
	SetJobSelectChoice(p, "Pilot")
	SetJobSelectChoice(p, "Captain")
	if got := JobSelectChoice(p); got != "captain" {
		t.Fatalf("after reassignment: %q, want \"captain\"", got)
	}
	// Verify only one job_select_choice flag remains.
	count := 0
	for k, v := range p.Flags {
		if v == 1 && len(k) > len("job_select_choice:") &&
			k[:len("job_select_choice:")] == "job_select_choice:" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 job_select_choice flag, got %d (flags=%v)", count, p.Flags)
	}
}

func TestSetJobSelectChoice_EmptyClears(t *testing.T) {
	p := &Permanent{Card: newJobSelectCard("Ship", "Pilot", "Captain")}
	SetJobSelectChoice(p, "Pilot")
	SetJobSelectChoice(p, "")
	if got := JobSelectChoice(p); got != "" {
		t.Fatalf("after clear: %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// ChooseJobSelect — cast-time choice (CR §702.183a)
// ---------------------------------------------------------------------------

func TestChooseJobSelect_HappyPath(t *testing.T) {
	gs := newJobSelectGame(t)
	card := newJobSelectCard("Aerial Speedster", "Pilot", "Captain")
	item := &StackItem{Card: card, Controller: 0}
	if err := ChooseJobSelect(gs, item, "Captain"); err != nil {
		t.Fatalf("ChooseJobSelect error: %v", err)
	}
	if got := JobSelectFromStack(item); got != "captain" {
		t.Errorf("JobSelectFromStack = %q, want \"captain\"", got)
	}
	// Event was logged.
	found := false
	for _, e := range gs.EventLog {
		if e.Kind == "job_select_chosen" {
			found = true
		}
	}
	if !found {
		t.Error("job_select_chosen event should be logged")
	}
}

func TestChooseJobSelect_RejectsInvalidJob(t *testing.T) {
	gs := newJobSelectGame(t)
	card := newJobSelectCard("Aerial Speedster", "Pilot", "Captain")
	item := &StackItem{Card: card, Controller: 0}
	if err := ChooseJobSelect(gs, item, "Mechanic"); err == nil {
		t.Fatal("ChooseJobSelect should reject a job not in the printed list")
	}
}

func TestChooseJobSelect_RejectsNonJobSelectCard(t *testing.T) {
	gs := newJobSelectGame(t)
	card := &Card{Name: "Vanilla", AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	item := &StackItem{Card: card, Controller: 0}
	if err := ChooseJobSelect(gs, item, "Pilot"); err == nil {
		t.Fatal("ChooseJobSelect should reject a card without job_select")
	}
}

func TestChooseJobSelect_RejectsEmptyChoice(t *testing.T) {
	gs := newJobSelectGame(t)
	card := newJobSelectCard("Aerial Speedster", "Pilot", "Captain")
	item := &StackItem{Card: card, Controller: 0}
	if err := ChooseJobSelect(gs, item, ""); err == nil {
		t.Fatal("ChooseJobSelect should reject empty job choice")
	}
}

func TestJobSelectFromStack_NoChoice(t *testing.T) {
	item := &StackItem{}
	if got := JobSelectFromStack(item); got != "" {
		t.Fatalf("JobSelectFromStack on empty item = %q, want empty", got)
	}
}
