package gameengine

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r176 — census internals. The transport (real resolve -> rendered
// grinder line) is proven in internal/hexapi; these cover the pieces
// that package cannot reach: the cardinality bound and concurrency.

func r176Probe(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(176)), nil)
	p := &Permanent{Card: &Card{Name: "R176 Probe", BasePower: 1, BaseToughness: 1,
		Types: []string{"creature"}}, Controller: 0, Owner: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	return gs, p
}

// TestR176_EvalConditionCensusBothDirections pins the discriminator at
// the source: evalCondition itself must count the kinds it falls
// through on and ONLY those.
func TestR176_EvalConditionCensusBothDirections(t *testing.T) {
	ResetConditionUnrecognizedForTest()
	t.Cleanup(ResetConditionUnrecognizedForTest)
	gs, p := r176Probe(t)

	// Recognized kinds, exercised through evalCondition directly.
	for _, c := range []*gameast.Condition{
		{Kind: "life_threshold", Args: []interface{}{">=", 1}},
		{Kind: "raw", Args: []interface{}{"text"}},
		{Kind: "self_was_not_cast"},
	} {
		evalCondition(gs, p, c)
	}
	// A nil condition is "no gate at all" and must not be counted
	// either — it returns true before the switch.
	evalCondition(gs, p, nil)

	if got := ConditionUnrecognizedCounts(); len(got) != 0 {
		t.Fatalf("recognized kinds were counted: %v", got)
	}

	evalCondition(gs, p, &gameast.Condition{Kind: "if"})
	got := ConditionUnrecognizedCounts()
	if got["if"] != 1 || len(got) != 1 {
		t.Fatalf("the unrecognized kind was not counted exactly once: %v", got)
	}
}

// TestR176_CensusCardinalityIsBounded — c.Kind comes from the parser
// and nothing structurally stops a future grammar from emitting
// something unbounded, so the map must not grow without limit.
func TestR176_CensusCardinalityIsBounded(t *testing.T) {
	ResetConditionUnrecognizedForTest()
	t.Cleanup(ResetConditionUnrecognizedForTest)

	for i := 0; i < condUnkMax+50; i++ {
		bumpConditionUnrecognized("synthetic_" + strconv.Itoa(i))
	}
	got := ConditionUnrecognizedCounts()
	if len(got) > condUnkMax+1 {
		t.Fatalf("census grew past the bound: %d distinct keys", len(got))
	}
	if got[condUnkOverflow] == 0 {
		t.Fatalf("past the bound, new kinds must land in %q: %v", condUnkOverflow, got)
	}
}

// TestR176_TopIsOrderedAndStable — the rendered field is read by
// humans scanning a log; heaviest first, ties by name, capped at n.
func TestR176_TopIsOrderedAndStable(t *testing.T) {
	ResetConditionUnrecognizedForTest()
	t.Cleanup(ResetConditionUnrecognizedForTest)

	for i := 0; i < 5; i++ {
		bumpConditionUnrecognized("if")
	}
	bumpConditionUnrecognized("bbb")
	bumpConditionUnrecognized("aaa")

	if s := ConditionUnrecognizedTop(2); s != "if:5,aaa:1" {
		t.Fatalf("top-2 wrong: %q", s)
	}
	if s := ConditionUnrecognizedTop(10); s != "if:5,aaa:1,bbb:1" {
		t.Fatalf("full render wrong: %q", s)
	}
	ResetConditionUnrecognizedForTest()
	if s := ConditionUnrecognizedTop(5); s != "" {
		t.Fatalf("an empty census must render nothing, got %q", s)
	}
}

// TestR176_CensusIsRaceFree — the grinder runs N worker goroutines
// evaluating conditions while the ticker goroutine renders the line.
// Reading another worker's state would be a data race; this drives
// both sides at once under -race.
func TestR176_CensusIsRaceFree(t *testing.T) {
	ResetConditionUnrecognizedForTest()
	t.Cleanup(ResetConditionUnrecognizedForTest)

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			gs := NewGameState(2, rand.New(rand.NewSource(int64(w))), nil)
			p := &Permanent{Card: &Card{Name: "R176 Racer", Types: []string{"creature"}},
				Controller: 0, Owner: 0}
			for i := 0; i < 200; i++ {
				evalCondition(gs, p, &gameast.Condition{Kind: "if"})
				evalCondition(gs, p, &gameast.Condition{Kind: "etb_if"})
			}
		}(w)
	}
	// The reader, concurrent with the writers, as the ticker is.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = ConditionUnrecognizedTop(8)
		}
	}()
	wg.Wait()

	if got := ConditionUnrecognizedCounts(); got["if"] != 1600 || got["etb_if"] != 1600 {
		t.Fatalf("concurrent counts lost increments: %v", got)
	}
	if !strings.HasPrefix(ConditionUnrecognizedTop(8), "etb_if:1600,if:1600") {
		t.Fatalf("render after the race: %q", ConditionUnrecognizedTop(8))
	}
}
