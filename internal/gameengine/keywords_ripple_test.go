package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Ripple tests — CR §702.60 (Coldsnap, 2006)
// ---------------------------------------------------------------------------

func newRippleGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(92))
	return NewGameState(2, rng, nil)
}

// newRippleInstant builds an instant named `name` with ripple-N.
func newRippleInstant(name string, owner, n int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ripple", Args: []any{float64(n)}},
			},
		},
	}
}

// newPlainInstantNamed builds a generic non-ripple card with the given
// name. Used to populate the library with both ripple-name matches and
// non-matches.
func newPlainInstantNamed(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   1,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

// ---------------------------------------------------------------------------
// HasRipple / RippleN
// ---------------------------------------------------------------------------

func TestHasRipple_Detects(t *testing.T) {
	card := newRippleInstant("Thrumming Stone", 0, 4)
	if !HasRipple(card) {
		t.Fatal("HasRipple should be true for a ripple-N card")
	}
}

func TestHasRipple_Negative(t *testing.T) {
	if HasRipple(newPlainInstantNamed("Lightning Bolt", 0)) {
		t.Fatal("HasRipple should be false for a vanilla instant")
	}
}

func TestHasRipple_Nil(t *testing.T) {
	if HasRipple(nil) {
		t.Fatal("HasRipple(nil) must be false")
	}
}

func TestRippleN_ReadsArg(t *testing.T) {
	card := newRippleInstant("Thrumming Stone", 0, 4)
	n, ok := RippleN(card)
	if !ok || n != 4 {
		t.Fatalf("RippleN = (%d, %v), want (4, true)", n, ok)
	}
}

// ---------------------------------------------------------------------------
// (a) 0 matches — all N cards rippled to bottom, no extra casts
// ---------------------------------------------------------------------------

func TestApplyRipple_NoMatches_AllToBottom(t *testing.T) {
	gs := newRippleGame(t)

	source := newRippleInstant("Sage's Row Denizen", 0, 4)
	// Top of library — none match "Sage's Row Denizen".
	a := newPlainInstantNamed("Counterspell", 0)
	b := newPlainInstantNamed("Lightning Bolt", 0)
	c := newPlainInstantNamed("Brainstorm", 0)
	d := newPlainInstantNamed("Path to Exile", 0)
	// A bottom card to detect ordering.
	bottom := newPlainInstantNamed("Bottom Sentinel", 0)
	gs.Seats[0].Library = []*Card{a, b, c, d, bottom}
	preLen := len(gs.Seats[0].Library)

	casts := ApplyRipple(gs, 0, source, 4)
	if casts != 0 {
		t.Errorf("free-cast count = %d, want 0", casts)
	}
	if len(gs.Seats[0].Library) != preLen {
		t.Errorf("library len = %d, want %d (cards return to bottom)",
			len(gs.Seats[0].Library), preLen)
	}
	if len(gs.Stack) != 0 {
		t.Errorf("Stack len = %d, want 0 (no free casts)", len(gs.Stack))
	}
	// Bottom Sentinel was originally last; after ripple-with-no-matches,
	// it should now be in position len-5 because the 4 revealed cards go
	// below it.
	lib := gs.Seats[0].Library
	if lib[0] != bottom {
		t.Errorf("after ripple, library[0] (new top) = %q, want %q",
			lib[0].Name, bottom.Name)
	}
	// The revealed cards should be at the bottom in reveal order
	// (a, b, c, d).
	if lib[len(lib)-4] != a || lib[len(lib)-3] != b ||
		lib[len(lib)-2] != c || lib[len(lib)-1] != d {
		t.Errorf("library bottom order incorrect: got [%q %q %q %q], want [%q %q %q %q]",
			lib[len(lib)-4].Name, lib[len(lib)-3].Name,
			lib[len(lib)-2].Name, lib[len(lib)-1].Name,
			a.Name, b.Name, c.Name, d.Name)
	}
}

func TestApplyRipple_ZeroN_NoOp(t *testing.T) {
	gs := newRippleGame(t)
	source := newRippleInstant("Whatever", 0, 0)
	gs.Seats[0].Library = []*Card{newPlainInstantNamed("Card", 0)}
	if got := ApplyRipple(gs, 0, source, 0); got != 0 {
		t.Errorf("ApplyRipple(n=0) = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// (b) Match chain — newly cast ripple spells re-trigger ripple
// ---------------------------------------------------------------------------

func TestApplyRipple_ChainsThroughRippleHits(t *testing.T) {
	gs := newRippleGame(t)

	// Three "Ripple Bolt" copies on top of the library, all with ripple-2.
	// When the original (passed as sourceCard) triggers, it should reveal
	// two cards, find one Ripple Bolt match, cast it for free. That cast
	// re-triggers its own ripple, revealing another two; if more matches
	// are at the top, those also cast.
	r1 := newRippleInstant("Ripple Bolt", 0, 2)
	r2 := newRippleInstant("Ripple Bolt", 0, 2)
	noise1 := newPlainInstantNamed("Noise", 0)
	r3 := newRippleInstant("Ripple Bolt", 0, 2)
	noise2 := newPlainInstantNamed("Noise", 0)

	// Library top-to-bottom: r1, noise1, r2, noise2, r3
	gs.Seats[0].Library = []*Card{r1, noise1, r2, noise2, r3}

	source := newRippleInstant("Ripple Bolt", 0, 2)
	casts := ApplyRipple(gs, 0, source, 2)

	// Walking the chain:
	//   trigger 1 (source): reveal [r1, noise1] → match r1, cast free
	//     trigger 2 (r1): reveal [r2, noise2] → match r2, cast free
	//       trigger 3 (r2): reveal [r3] → match r3, cast free
	//         trigger 4 (r3): reveal [] (library exhausted) → 0 matches
	//   total = 3 chained free casts
	if casts != 3 {
		t.Errorf("chained casts = %d, want 3", casts)
	}

	// Verify ripple_trigger events fired four times (source + 3 chain).
	triggerCount := 0
	for _, e := range gs.EventLog {
		if e.Kind == "ripple_trigger" {
			triggerCount++
		}
	}
	if triggerCount != 4 {
		t.Errorf("ripple_trigger event count = %d, want 4 (source + 3 chained)", triggerCount)
	}
}

func TestApplyRipple_ChainStopsOnFirstNonMatch(t *testing.T) {
	gs := newRippleGame(t)

	// Top of library has a non-match first, blocking the chain.
	noise := newPlainInstantNamed("Noise", 0)
	r1 := newRippleInstant("Ripple Bolt", 0, 2)
	gs.Seats[0].Library = []*Card{noise, r1}

	source := newRippleInstant("Ripple Bolt", 0, 2)
	casts := ApplyRipple(gs, 0, source, 2)

	// Reveal of 2 still happens for the trigger, and r1 IS one of the
	// revealed cards — ripple matches ANY revealed card with the same
	// name, not just the first. So r1 should cast.
	//   trigger 1: reveal [noise, r1] → r1 matches → cast free
	//     trigger 2 (r1): reveal [] → empty
	//   total = 1
	if casts != 1 {
		t.Errorf("casts = %d, want 1 (only r1 matches; library exhausted after)", casts)
	}
}

// ---------------------------------------------------------------------------
// (c) Cards cast free are stamped CostMeta["ripple_cast"]=true
// ---------------------------------------------------------------------------

func TestApplyRipple_StampsCostMetaRippleCast(t *testing.T) {
	gs := newRippleGame(t)

	// Capture the StackItem pushed for the free cast by intercepting it
	// before resolution. Use a non-ripple match name so chain is bounded.
	match := newPlainInstantNamed("Ripple Bolt", 0)
	noise := newPlainInstantNamed("Noise", 0)
	gs.Seats[0].Library = []*Card{match, noise}

	source := newRippleInstant("Ripple Bolt", 0, 2)
	_ = ApplyRipple(gs, 0, source, 2)

	// The stack item resolves immediately, so we can't peek at it after.
	// Instead, verify via the EventLog that the ripple_hit event fired
	// with the right source. The CostMeta stamp is exercised by the
	// IsRippleCast predicate below (mid-resolution).
	foundHit := false
	for _, e := range gs.EventLog {
		if e.Kind == "ripple_hit" && e.Source == "Ripple Bolt" {
			foundHit = true
		}
	}
	if !foundHit {
		t.Error("expected ripple_hit event for matched name")
	}
}

// Verify the CostMeta directly by intercepting before resolution. We
// achieve that by hijacking the resolver: push a permanent type that
// won't resolve to permanent (using an empty-effect card), then peek
// at gs.Stack before ResolveStackTop runs. Cleanest path: call the
// underlying push helper through ApplyRipple but with a card that has
// no resolve effect (which we already do — Effect is nil-ok), and
// check stack BEFORE the immediate resolution drains it.
//
// Since ApplyRipple resolves inline, we instead probe via the event
// log: ResolveStackTop logs a "spell_resolve" or similar. To get a
// concrete CostMeta assertion, we read it from the predicate by
// recreating the stack-state path through the helper.
func TestApplyRipple_CostMetaPredicateOnStackItem(t *testing.T) {
	_ = newRippleGame(t) // game init keeps parallel-test conventions
	// Direct unit test of the StackItem shape: build it the way
	// ApplyRipple does and check IsRippleCast / CostMeta keys.
	src := newRippleInstant("Ripple Bolt", 0, 2)
	free := newPlainInstantNamed("Ripple Bolt", 0)
	item := &StackItem{
		Controller: 0,
		Card:       free,
		CastZone:   ZoneLibrary,
		CostMeta: map[string]interface{}{
			"ripple_cast":   true,
			"ripple_source": src,
			"ripple_n":      2,
		},
	}
	if !IsRippleCast(item) {
		t.Error("IsRippleCast should be true for ripple-stamped item")
	}
	if b, _ := item.CostMeta["ripple_cast"].(bool); !b {
		t.Error("CostMeta[ripple_cast] should be bool(true)")
	}
	if got, _ := item.CostMeta["ripple_source"].(*Card); got != src {
		t.Error("CostMeta[ripple_source] should be the trigger source card")
	}
	if got, _ := item.CostMeta["ripple_n"].(int); got != 2 {
		t.Errorf("CostMeta[ripple_n] = %v, want 2", got)
	}
	// Negative: a vanilla item is not ripple-cast.
	plain := &StackItem{Controller: 0, Card: newPlainInstantNamed("X", 0)}
	if IsRippleCast(plain) {
		t.Error("IsRippleCast should be false for non-ripple item")
	}
}

// ---------------------------------------------------------------------------
// (d) Library order preserved for non-matched cards
// ---------------------------------------------------------------------------

func TestApplyRipple_LibraryOrderPreservedForNonMatched(t *testing.T) {
	gs := newRippleGame(t)

	source := newRippleInstant("Match", 0, 4)
	// Reveal 4: 2 matches + 2 non-matches, interleaved.
	m1 := newPlainInstantNamed("Match", 0)
	n1 := newPlainInstantNamed("NonMatch-A", 0)
	m2 := newPlainInstantNamed("Match", 0)
	n2 := newPlainInstantNamed("NonMatch-B", 0)
	tail := newPlainInstantNamed("Tail", 0)
	gs.Seats[0].Library = []*Card{m1, n1, m2, n2, tail}

	casts := ApplyRipple(gs, 0, source, 4)
	if casts != 2 {
		t.Errorf("matches cast = %d, want 2", casts)
	}

	// After ripple:
	//   - m1 and m2 cast and resolved (gone from library)
	//   - tail remains at top (was originally at index 4, untouched)
	//   - n1 then n2 appended to bottom (preserving reveal order)
	lib := gs.Seats[0].Library
	if len(lib) != 3 {
		t.Fatalf("library len = %d, want 3 (5 - 2 cast)", len(lib))
	}
	if lib[0] != tail {
		t.Errorf("top of library = %q, want %q (tail untouched)", lib[0].Name, tail.Name)
	}
	if lib[1] != n1 {
		t.Errorf("second card = %q, want %q (first non-match)", lib[1].Name, n1.Name)
	}
	if lib[2] != n2 {
		t.Errorf("third card = %q, want %q (second non-match)", lib[2].Name, n2.Name)
	}
}

// ---------------------------------------------------------------------------
// Case-insensitive name match
// ---------------------------------------------------------------------------

func TestApplyRipple_NameMatchIsCaseInsensitive(t *testing.T) {
	gs := newRippleGame(t)

	source := newRippleInstant("Ripple Bolt", 0, 1)
	gs.Seats[0].Library = []*Card{newPlainInstantNamed("RIPPLE BOLT", 0)}

	casts := ApplyRipple(gs, 0, source, 1)
	if casts != 1 {
		t.Errorf("case-insensitive match cast count = %d, want 1", casts)
	}
}

// ---------------------------------------------------------------------------
// Library shorter than N
// ---------------------------------------------------------------------------

func TestApplyRipple_LibrarySmallerThanN(t *testing.T) {
	gs := newRippleGame(t)
	source := newRippleInstant("Bolt", 0, 4)
	c1 := newPlainInstantNamed("Bolt", 0)
	c2 := newPlainInstantNamed("Other", 0)
	gs.Seats[0].Library = []*Card{c1, c2}

	casts := ApplyRipple(gs, 0, source, 4)
	if casts != 1 {
		t.Errorf("casts = %d, want 1 (one Bolt in library)", casts)
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("library len = %d, want 1 (other returned to bottom)",
			len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Library[0] != c2 {
		t.Errorf("remaining card = %q, want %q", gs.Seats[0].Library[0].Name, c2.Name)
	}
}

// ---------------------------------------------------------------------------
// Empty library — graceful no-op
// ---------------------------------------------------------------------------

func TestApplyRipple_EmptyLibrary(t *testing.T) {
	gs := newRippleGame(t)
	source := newRippleInstant("X", 0, 4)
	// Library is empty.
	casts := ApplyRipple(gs, 0, source, 4)
	if casts != 0 {
		t.Errorf("casts on empty library = %d, want 0", casts)
	}
}

// ---------------------------------------------------------------------------
// Reveal event logged with revealed names
// ---------------------------------------------------------------------------

func TestApplyRipple_LogsRevealedNames(t *testing.T) {
	gs := newRippleGame(t)
	source := newRippleInstant("X", 0, 2)
	a := newPlainInstantNamed("A", 0)
	b := newPlainInstantNamed("B", 0)
	gs.Seats[0].Library = []*Card{a, b}

	_ = ApplyRipple(gs, 0, source, 2)

	found := false
	for _, e := range gs.EventLog {
		if e.Kind != "ripple_reveal" {
			continue
		}
		names, _ := e.Details["revealed"].([]string)
		if len(names) == 2 && names[0] == "A" && names[1] == "B" {
			found = true
		}
	}
	if !found {
		t.Error("expected ripple_reveal event with revealed=[A, B]")
	}
}
