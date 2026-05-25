package hat

// R60 round 5 — multi-mode ChooseMode audit + improvements.
//
// Audit finding: `resolveChoice` in internal/gameengine/resolve.go
// (pre-fix) called the Hat's ChooseMode exactly ONCE and resolved
// ONE mode, regardless of the spell's `pick` count. Cryptic Command
// (pick=2 of 4) under-resolved to a single mode whenever the
// controller had a Hat. The "first N in order" fallback only fired
// when no Hat was present.
//
// Fix: engine loops ChooseMode `pick` times, narrowing the modes
// slice by one between calls so the hat can't re-pick. Resolves the
// final list in PRINTED ORDER per CR §608.2g.
//
// Hat-side improvement: when the engine's second ChooseMode call
// arrives with a slice that's the previous one minus our prior
// pick (subset detection), apply `complementBonus` to bias the
// second pick toward a synergistic complement — counter→bounce,
// bounce→draw, destroy→draw, etc. — the canonical multi-mode lines
// experienced players take.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// Engine: multi-mode pick loops ChooseMode
// ---------------------------------------------------------------------

// modeCounter records how many times ChooseMode was called and what
// modes-slice length it saw on each call. Used to verify the engine
// loops correctly for pick > 1.
type modeCounter struct {
	GreedyHat
	callsSeenLen []int
}

func (h *modeCounter) ChooseMode(_ *gameengine.GameState, _ int, modes []gameast.Effect) int {
	h.callsSeenLen = append(h.callsSeenLen, len(modes))
	return 0 // always pick the first available
}

func TestResolveChoice_Pick2CallsChooseModeTwice(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	hat := &modeCounter{}
	gs.Seats[0].Hat = hat

	// Build a 4-option modal effect with pick=2.
	opts := []gameast.Effect{
		gameast.CounterSpell{},
		gameast.Damage{},
		gameast.Draw{},
		gameast.Bounce{},
	}
	choice := &gameast.Choice{
		Options: opts,
		Pick:    gameast.NumberOrRef{IsInt: true, Int: 2},
	}

	src := &gameengine.Permanent{Controller: 0}
	gameengine.ResolveEffect(gs, src, choice)

	if len(hat.callsSeenLen) != 2 {
		t.Errorf("pick=2 must call ChooseMode twice; got %d calls", len(hat.callsSeenLen))
	}
	if len(hat.callsSeenLen) >= 1 && hat.callsSeenLen[0] != 4 {
		t.Errorf("first call must show all 4 options; got %d", hat.callsSeenLen[0])
	}
	if len(hat.callsSeenLen) >= 2 && hat.callsSeenLen[1] != 3 {
		t.Errorf("second call must show 3 options (prior pick stripped); got %d", hat.callsSeenLen[1])
	}
}

func TestResolveChoice_Pick1StillCallsOnce(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	hat := &modeCounter{}
	gs.Seats[0].Hat = hat

	opts := []gameast.Effect{
		gameast.CounterSpell{},
		gameast.Draw{},
	}
	choice := &gameast.Choice{
		Options: opts,
		Pick:    gameast.NumberOrRef{IsInt: true, Int: 1},
	}

	src := &gameengine.Permanent{Controller: 0}
	gameengine.ResolveEffect(gs, src, choice)

	if len(hat.callsSeenLen) != 1 {
		t.Errorf("pick=1 must call ChooseMode once; got %d calls", len(hat.callsSeenLen))
	}
}

// modeRecorder remembers the indices it picked AND the slice contents
// at each call. Used to verify subsequent calls see a slice that
// EXCLUDES the prior pick.
type modeRecorder struct {
	GreedyHat
	calls [][]gameast.Effect
	picks []int
}

func (h *modeRecorder) ChooseMode(_ *gameengine.GameState, _ int, modes []gameast.Effect) int {
	snap := make([]gameast.Effect, len(modes))
	copy(snap, modes)
	h.calls = append(h.calls, snap)
	h.picks = append(h.picks, 0)
	return 0
}

func TestResolveChoice_SecondCallExcludesPriorPick(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	hat := &modeRecorder{}
	gs.Seats[0].Hat = hat

	counter := gameast.CounterSpell{}
	dmg := gameast.Damage{}
	draw := gameast.Draw{}
	bnc := gameast.Bounce{}

	choice := &gameast.Choice{
		Options: []gameast.Effect{counter, dmg, draw, bnc},
		Pick:    gameast.NumberOrRef{IsInt: true, Int: 2},
	}

	gameengine.ResolveEffect(gs, &gameengine.Permanent{Controller: 0}, choice)

	if len(hat.calls) != 2 {
		t.Fatalf("expected 2 calls; got %d", len(hat.calls))
	}
	// First call picked index 0 (CounterSpell). Second call must see
	// a 3-element slice that does NOT contain CounterSpell.
	second := hat.calls[1]
	if len(second) != 3 {
		t.Fatalf("second slice should be length 3; got %d", len(second))
	}
	for _, m := range second {
		if m.Kind() == "counter_spell" {
			t.Error("second ChooseMode call must NOT show the previously-picked CounterSpell")
		}
	}
}

// ---------------------------------------------------------------------
// Hat: complementBonus matrix
// ---------------------------------------------------------------------

func TestComplementBonus_CounterPairsWithBounceOrDraw(t *testing.T) {
	prior := gameast.CounterSpell{}
	if complementBonus(prior, gameast.Bounce{}) <= 0 {
		t.Error("counter→bounce must be positive (Cryptic Command's classic line)")
	}
	if complementBonus(prior, gameast.Draw{}) <= 0 {
		t.Error("counter→draw must be positive (counter and refill)")
	}
}

func TestComplementBonus_BouncePairsWithDrawOrCounter(t *testing.T) {
	prior := gameast.Bounce{}
	if complementBonus(prior, gameast.Draw{}) <= 0 {
		t.Error("bounce→draw must be positive")
	}
	if complementBonus(prior, gameast.CounterSpell{}) <= 0 {
		t.Error("bounce→counter must be positive")
	}
}

func TestComplementBonus_DamagePairsWithDraw(t *testing.T) {
	if complementBonus(gameast.Damage{}, gameast.Draw{}) <= 0 {
		t.Error("damage→draw must be positive (burn + cantrip)")
	}
}

func TestComplementBonus_SameKindIsNegative(t *testing.T) {
	// Picking the same kind twice (e.g., two damage modes) gets a mild
	// penalty for diversity.
	if got := complementBonus(gameast.Damage{}, gameast.Damage{}); got >= 0 {
		t.Errorf("same-kind complement must be negative; got %.3f", got)
	}
	if got := complementBonus(gameast.Draw{}, gameast.Draw{}); got >= 0 {
		t.Errorf("draw→draw must be negative; got %.3f", got)
	}
}

func TestComplementBonus_UnknownPairIsZero(t *testing.T) {
	// scry → sacrifice has no canonical synergy.
	if got := complementBonus(gameast.Scry{}, gameast.Sacrifice{}); got != 0 {
		t.Errorf("unknown pairing must yield 0; got %.3f", got)
	}
}

func TestComplementBonus_NilSafe(t *testing.T) {
	if complementBonus(nil, gameast.Draw{}) != 0 {
		t.Error("nil prior must yield 0")
	}
	if complementBonus(gameast.Draw{}, nil) != 0 {
		t.Error("nil candidate must yield 0")
	}
}

// ---------------------------------------------------------------------
// Hat: follow-up detection
// ---------------------------------------------------------------------

// First call: 4 modes, picks counter. Second call (same turn): 3
// modes = first 4 minus counter. `priorChooseModePickIfFollowup`
// must return the counter Effect from the first pick.
func TestPriorChooseModePickIfFollowup_DetectsStripCall(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	gs.Turn = 5

	counter := gameast.CounterSpell{}
	dmg := gameast.Damage{}
	draw := gameast.Draw{}
	bnc := gameast.Bounce{}
	allFour := []gameast.Effect{counter, dmg, draw, bnc}

	// Simulate first ChooseMode call's bookkeeping side effects.
	h.lastChooseModePick = counter
	h.lastChooseModeSlice = append([]gameast.Effect{}, allFour...)
	h.lastChooseModeTurn = 5

	// Second call: slice is allFour minus counter.
	secondSlice := []gameast.Effect{dmg, draw, bnc}
	prior := h.priorChooseModePickIfFollowup(gs, secondSlice)
	if prior == nil {
		t.Fatal("expected prior to be detected (follow-up); got nil")
	}
	if prior.Kind() != "counter_spell" {
		t.Errorf("prior should be counter_spell; got %s", prior.Kind())
	}
}

// Same turn but the slice ISN'T a strip — fresh modal spell with
// different options. Must NOT report a follow-up.
func TestPriorChooseModePickIfFollowup_DifferentSpellNoFollowup(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	gs.Turn = 5

	h.lastChooseModePick = gameast.CounterSpell{}
	h.lastChooseModeSlice = []gameast.Effect{gameast.CounterSpell{}, gameast.Draw{}, gameast.Bounce{}}
	h.lastChooseModeTurn = 5

	// Different spell — different Effect values.
	fresh := []gameast.Effect{gameast.GainLife{}, gameast.Damage{}}
	if got := h.priorChooseModePickIfFollowup(gs, fresh); got != nil {
		t.Errorf("different spell must not be detected as follow-up; got %v", got)
	}
}

// Cross-turn: prior pick from turn N must not trigger follow-up on
// turn N+1 even if the slice happens to look like a strip.
func TestPriorChooseModePickIfFollowup_TurnInvalidates(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	counter := gameast.CounterSpell{}
	h.lastChooseModePick = counter
	h.lastChooseModeSlice = []gameast.Effect{counter, gameast.Draw{}, gameast.Bounce{}}
	h.lastChooseModeTurn = 5

	gs.Turn = 6 // new turn

	stripSlice := []gameast.Effect{gameast.Draw{}, gameast.Bounce{}}
	if got := h.priorChooseModePickIfFollowup(gs, stripSlice); got != nil {
		t.Errorf("cross-turn follow-up must be invalidated; got %v", got)
	}
}

// game_start clears the follow-up state.
func TestChooseMode_GameStartResetsFollowupState(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	h.lastChooseModePick = gameast.Damage{}
	h.lastChooseModeSlice = []gameast.Effect{gameast.Damage{}, gameast.Draw{}}
	h.lastChooseModeTurn = 5

	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	if h.lastChooseModePick != nil {
		t.Error("game_start must nil lastChooseModePick")
	}
	if h.lastChooseModeSlice != nil {
		t.Error("game_start must nil lastChooseModeSlice")
	}
	if h.lastChooseModeTurn != 0 {
		t.Error("game_start must zero lastChooseModeTurn")
	}
}

// End-to-end: drive ChooseMode twice on a real YggdrasilHat with
// Cryptic-Command-shaped options. The second pick must be a
// complement of the first (not a same-kind repeat).
func TestChooseMode_FollowupPicksComplementaryMode(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	// Deterministic noiseRNG so selectAmongTop's Intn over the
	// confidence-margin band picks reproducibly (NewYggdrasilHat
	// seeds noiseRNG from a non-deterministic global source).
	h.noiseRNG = rand.New(rand.NewSource(1))
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	gs.Seats[0].Hat = h
	gs.Turn = 5

	// Put a hostile spell on the stack so counter_spell scores high.
	hostileItem := &gameengine.StackItem{
		ID: 99, Controller: 1, Kind: "spell",
		Card: &gameengine.Card{Name: "Hostile Spell"},
	}
	gs.Stack = []*gameengine.StackItem{hostileItem}

	// Empty-ish hand → draw scores well.
	gs.Seats[0].Hand = []*gameengine.Card{}

	counter := gameast.CounterSpell{}
	dmg := gameast.Damage{}
	draw := gameast.Draw{}
	bnc := gameast.Bounce{}
	modes := []gameast.Effect{counter, dmg, draw, bnc}

	// First pick — without follow-up state, hat picks based on raw
	// scoring (counter beats the others when a hostile spell is up).
	first := h.ChooseMode(gs, 0, modes)
	if modes[first].Kind() != "counter_spell" {
		t.Fatalf("first pick should be counter_spell when hostile is on stack; got %s",
			modes[first].Kind())
	}

	// Second pick — engine narrows to {dmg, draw, bnc}.
	stripped := []gameast.Effect{dmg, draw, bnc}
	second := h.ChooseMode(gs, 0, stripped)
	// The complement bonus pushes draw / bounce above damage.
	pickedKind := stripped[second].Kind()
	if pickedKind != "draw" && pickedKind != "bounce" {
		t.Errorf("second pick must be a complement of counter (draw or bounce); got %s",
			pickedKind)
	}
}
