package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Investigate tests — CR §701.16 (keyword surface)
// ---------------------------------------------------------------------------

func newInvestigateGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(701))
	return NewGameState(2, rng, nil)
}

// newCardWithKeyword builds a card carrying a single keyword AST node.
func newCardWithKeyword(name, kw string) *Card {
	return &Card{
		Name:  name,
		Types: []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: kw},
			},
		},
	}
}

// newCardWithStaticText builds a card whose AST has a Static ability
// carrying `text` as its Raw — this is what HasInvestigate's oracle
// scan reads.
func newCardWithStaticText(name, text string) *Card {
	return &Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: text},
			},
		},
	}
}

func newCardWithTriggeredText(name, text string) *Card {
	return &Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Triggered{Raw: text},
			},
		},
	}
}

func newPlainCardI(name string) *Card {
	return &Card{
		Name:  name,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

// putInvestigatePerm wraps a card in a Permanent on seat's battlefield.
func putInvestigatePerm(gs *GameState, seat int, card *Card) *Permanent {
	p := &Permanent{Card: card, Controller: seat, Owner: seat}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func countClueTokens(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == "clue" {
				n++
				break
			}
		}
	}
	return n
}

func countInvestigateEvents(gs *GameState, kind string) int {
	n := 0
	for _, e := range gs.EventLog {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// (a) HasInvestigate true for oracle text containing "investigate"
// ---------------------------------------------------------------------------

func TestHasInvestigate_OracleStaticText(t *testing.T) {
	card := newCardWithStaticText("Tireless Tracker",
		"Whenever you draw a card, this creature gets a +1/+1 counter. {2}, sacrifice a Clue: investigate.")
	if !HasInvestigate(card) {
		t.Fatal("HasInvestigate should be true when oracle text contains \"investigate\"")
	}
}

func TestHasInvestigate_OracleTriggeredText(t *testing.T) {
	card := newCardWithTriggeredText("Bygone Bishop",
		"Whenever you cast a creature spell with mana value 3 or less, investigate.")
	if !HasInvestigate(card) {
		t.Fatal("HasInvestigate should detect investigate in Triggered.Raw text")
	}
}

func TestHasInvestigate_CaseInsensitive(t *testing.T) {
	card := newCardWithStaticText("Caps Lock", "INVESTIGATE this turn.")
	if !HasInvestigate(card) {
		t.Fatal("HasInvestigate should be case-insensitive")
	}
}

func TestHasInvestigate_DirectKeyword(t *testing.T) {
	card := newCardWithKeyword("Direct Keyword Card", "investigate")
	if !HasInvestigate(card) {
		t.Fatal("HasInvestigate should detect a direct \"investigate\" Keyword node")
	}
}

func TestHasInvestigate_Negative(t *testing.T) {
	if HasInvestigate(newPlainCardI("Plain Bear")) {
		t.Fatal("HasInvestigate must be false on a vanilla card")
	}
	// A card mentioning clues but not "investigate" should not register.
	other := newCardWithStaticText("Clue Lover", "Sacrifice a Clue: draw a card.")
	if HasInvestigate(other) {
		t.Fatal("HasInvestigate must be false when oracle mentions clue but not investigate")
	}
}

func TestHasInvestigate_Nil(t *testing.T) {
	if HasInvestigate(nil) {
		t.Fatal("HasInvestigate(nil) must be false")
	}
	if HasInvestigate(&Card{Name: "No-AST"}) {
		t.Fatal("HasInvestigate on AST-less card must be false")
	}
}

// ---------------------------------------------------------------------------
// (b) ApplyInvestigateEffect mints N Clue tokens via CreateClueToken
// ---------------------------------------------------------------------------

func TestApplyInvestigateEffect_MintsOneClue(t *testing.T) {
	gs := newInvestigateGame(t)
	preClues := countClueTokens(gs, 0)
	got := ApplyInvestigateEffect(gs, 0, 1)
	if got != 1 {
		t.Errorf("ApplyInvestigateEffect returned %d, want 1", got)
	}
	if countClueTokens(gs, 0)-preClues != 1 {
		t.Errorf("Clue tokens on battlefield delta = %d, want 1",
			countClueTokens(gs, 0)-preClues)
	}
}

func TestApplyInvestigateEffect_MintsNClues(t *testing.T) {
	gs := newInvestigateGame(t)
	preClues := countClueTokens(gs, 0)
	const n = 5
	got := ApplyInvestigateEffect(gs, 0, n)
	if got != n {
		t.Errorf("ApplyInvestigateEffect returned %d, want %d", got, n)
	}
	if delta := countClueTokens(gs, 0) - preClues; delta != n {
		t.Errorf("Clue tokens delta = %d, want %d", delta, n)
	}
	// An "investigate" summary event should record the count.
	foundEvent := false
	for _, e := range gs.EventLog {
		if e.Kind != "investigate" || e.Seat != 0 {
			continue
		}
		if e.Amount == n {
			foundEvent = true
		}
	}
	if !foundEvent {
		t.Errorf("expected investigate event with Amount=%d", n)
	}
}

func TestApplyInvestigateEffect_ZeroOrNegativeNoOp(t *testing.T) {
	gs := newInvestigateGame(t)
	if got := ApplyInvestigateEffect(gs, 0, 0); got != 0 {
		t.Errorf("ApplyInvestigateEffect(n=0) = %d, want 0", got)
	}
	if got := ApplyInvestigateEffect(gs, 0, -3); got != 0 {
		t.Errorf("ApplyInvestigateEffect(n=-3) = %d, want 0", got)
	}
	if countClueTokens(gs, 0) != 0 {
		t.Errorf("unexpected clue tokens after n<=0 calls: %d", countClueTokens(gs, 0))
	}
}

func TestApplyInvestigateEffect_NilSafe(t *testing.T) {
	if got := ApplyInvestigateEffect(nil, 0, 3); got != 0 {
		t.Errorf("nil gs ApplyInvestigateEffect = %d, want 0", got)
	}
	gs := newInvestigateGame(t)
	if got := ApplyInvestigateEffect(gs, 99, 3); got != 0 {
		t.Errorf("invalid seat ApplyInvestigateEffect = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// (c) FireInvestigateTriggers fires for source
// ---------------------------------------------------------------------------

func TestFireInvestigateTriggers_FiresWithSource(t *testing.T) {
	gs := newInvestigateGame(t)

	source := putInvestigatePerm(gs, 0,
		newCardWithTriggeredText("Tireless Tracker",
			"Whenever you draw a card, this creature gets a +1/+1 counter."))

	// Capture the trigger.
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var captured map[string]interface{}
	var capturedEvent string
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if event == "investigate" {
			capturedEvent = event
			captured = ctx
		}
	}

	FireInvestigateTriggers(gs, 0, source, nil)

	if capturedEvent != "investigate" {
		t.Errorf("captured event = %q, want \"investigate\"", capturedEvent)
	}
	if captured == nil {
		t.Fatal("captured ctx is nil")
	}
	if got, _ := captured["source"].(*Permanent); got != source {
		t.Errorf("ctx[source] = %v, want %v", got, source)
	}
	if got, _ := captured["seat"].(int); got != 0 {
		t.Errorf("ctx[seat] = %d, want 0", got)
	}
	if countInvestigateEvents(gs, "investigate_trigger") != 1 {
		t.Errorf("investigate_trigger event count = %d, want 1",
			countInvestigateEvents(gs, "investigate_trigger"))
	}
}

func TestFireInvestigateTriggers_MergesExtraCtx(t *testing.T) {
	gs := newInvestigateGame(t)
	source := putInvestigatePerm(gs, 0, newPlainCardI("Source"))

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var captured map[string]interface{}
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if event == "investigate" {
			captured = ctx
		}
	}

	FireInvestigateTriggers(gs, 0, source, map[string]interface{}{
		"clues_minted":  2,
		"trigger_chain": "etb",
	})
	if got, _ := captured["clues_minted"].(int); got != 2 {
		t.Errorf("ctx[clues_minted] = %v, want 2", captured["clues_minted"])
	}
	if got, _ := captured["trigger_chain"].(string); got != "etb" {
		t.Errorf("ctx[trigger_chain] = %v, want \"etb\"", captured["trigger_chain"])
	}
}

func TestFireInvestigateTriggers_NilSafe(t *testing.T) {
	FireInvestigateTriggers(nil, 0, nil, nil)
	gs := newInvestigateGame(t)
	FireInvestigateTriggers(gs, 99, nil, nil)
	FireInvestigateTriggers(gs, -1, nil, nil)
	if countInvestigateEvents(gs, "investigate_trigger") != 0 {
		t.Errorf("expected 0 events on nil/invalid inputs, got %d",
			countInvestigateEvents(gs, "investigate_trigger"))
	}
}

func TestFireInvestigateTriggers_NilSourceStillFires(t *testing.T) {
	gs := newInvestigateGame(t)
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := false
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if event == "investigate" {
			fired = true
		}
	}
	// Investigate can be sourced from a spell that no longer has a
	// permanent (instant/sorcery resolution). nil source must still fire.
	FireInvestigateTriggers(gs, 0, nil, nil)
	if !fired {
		t.Error("FireInvestigateTriggers should fire even when source is nil (spell-driven investigate)")
	}
}

// ---------------------------------------------------------------------------
// (d) Integration: Sacrifice Clue, Draw a Card
// ---------------------------------------------------------------------------

// findFirstClue returns the first Clue token permanent on seatIdx's
// battlefield, or nil if none.
func findFirstClue(gs *GameState, seatIdx int) *Permanent {
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, t := range p.Card.Types {
			if t == "clue" {
				return p
			}
		}
	}
	return nil
}

// removePermFromBattlefield is a thin test helper that mirrors what a
// sacrifice cost would do at the engine level: pull the permanent off
// the battlefield slice (we're testing the surface, not the full sac
// pipeline).
func removePermFromBattlefield(gs *GameState, p *Permanent) bool {
	if gs == nil || p == nil {
		return false
	}
	seat := gs.Seats[p.Controller]
	if seat == nil {
		return false
	}
	for i, q := range seat.Battlefield {
		if q == p {
			seat.Battlefield = append(seat.Battlefield[:i], seat.Battlefield[i+1:]...)
			return true
		}
	}
	return false
}

func TestInvestigate_SacrificeClueDrawCardIntegration(t *testing.T) {
	gs := newInvestigateGame(t)
	// Stack the library so we have a card to draw.
	bolt := newPlainCardI("Lightning Bolt")
	gs.Seats[0].Library = []*Card{bolt}

	// Step 1: investigate, minting a Clue.
	if got := ApplyInvestigateEffect(gs, 0, 1); got != 1 {
		t.Fatalf("expected 1 clue minted, got %d", got)
	}
	clue := findFirstClue(gs, 0)
	if clue == nil {
		t.Fatal("expected a Clue token on the battlefield after investigate")
	}

	// Step 2: sacrifice the Clue.
	if !removePermFromBattlefield(gs, clue) {
		t.Fatal("failed to remove clue token from battlefield (sac)")
	}
	if findFirstClue(gs, 0) != nil {
		t.Fatal("Clue token should be gone after sacrifice")
	}

	// Step 3: draw a card (the clue's printed effect).
	preHand := len(gs.Seats[0].Hand)
	if _, ok := gs.drawOne(0); !ok {
		t.Fatal("drawOne failed despite library having 1 card")
	}
	if len(gs.Seats[0].Hand) != preHand+1 {
		t.Errorf("hand size after draw = %d, want %d", len(gs.Seats[0].Hand), preHand+1)
	}
	// The drawn card should be Lightning Bolt.
	if gs.Seats[0].Hand[len(gs.Seats[0].Hand)-1] != bolt {
		t.Error("drew the wrong card from the top of the library")
	}
}

func TestInvestigate_MultipleClueSacrificesIndependent(t *testing.T) {
	gs := newInvestigateGame(t)
	gs.Seats[0].Library = []*Card{
		newPlainCardI("Card-A"), newPlainCardI("Card-B"), newPlainCardI("Card-C"),
	}

	// Investigate 3.
	if got := ApplyInvestigateEffect(gs, 0, 3); got != 3 {
		t.Fatalf("expected 3 clues minted, got %d", got)
	}
	if countClueTokens(gs, 0) != 3 {
		t.Fatalf("expected 3 clues on battlefield, got %d", countClueTokens(gs, 0))
	}

	// Sac all three and draw three.
	for i := 0; i < 3; i++ {
		c := findFirstClue(gs, 0)
		if c == nil {
			t.Fatalf("missing clue on iteration %d", i)
		}
		if !removePermFromBattlefield(gs, c) {
			t.Fatalf("failed to remove clue on iteration %d", i)
		}
		if _, ok := gs.drawOne(0); !ok {
			t.Fatalf("drawOne failed on iteration %d", i)
		}
	}
	if countClueTokens(gs, 0) != 0 {
		t.Errorf("expected 0 clues after sacrificing all, got %d", countClueTokens(gs, 0))
	}
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("hand size = %d, want 3 after 3 clue draws", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// End-to-end: FireInvestigateTriggers + ApplyInvestigateEffect compose
// cleanly. The natural per_card pattern is: fire the trigger and mint.
// ---------------------------------------------------------------------------

func TestInvestigate_TriggerThenMintComposition(t *testing.T) {
	gs := newInvestigateGame(t)
	source := putInvestigatePerm(gs, 0, newCardWithStaticText(
		"Spelltower Spy", "When this creature enters, investigate."))

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	triggerCount := 0
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if event == "investigate" {
			triggerCount++
		}
	}

	FireInvestigateTriggers(gs, 0, source, nil)
	ApplyInvestigateEffect(gs, 0, 1)

	if triggerCount != 1 {
		t.Errorf("trigger fan-out count = %d, want 1", triggerCount)
	}
	if countClueTokens(gs, 0) != 1 {
		t.Errorf("clue count = %d, want 1", countClueTokens(gs, 0))
	}
}
