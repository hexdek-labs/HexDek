package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Devotion (CR §700.5)
// ---------------------------------------------------------------------------

func newDevotionGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(163))
	return NewGameState(2, rng, nil)
}

// addCostedPermanent drops a creature with a printed mana cost
// string onto `seat`'s battlefield. Devotion uses ManaCostString to
// count pips per §700.5; the Colors slice is left empty so we exercise
// the ManaCostString path (not the legacy Colors fallback).
func addCostedPermanent(gs *GameState, seat int, name, manaCost string) *Permanent {
	c := &Card{
		Name:           name,
		Owner:          seat,
		ManaCostString: manaCost,
		Types:          []string{"creature"},
		BasePower:      1, BaseToughness: 1,
		AST: &gameast.CardAST{Name: name},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// ===========================================================================
// (a) Pip-parsing accuracy: {B}{B}, {1}{B}, hybrid {B/R}
// ===========================================================================

func TestDevotionPipsFromManaCost_SingleAndDouble(t *testing.T) {
	cases := []struct {
		cost  string
		color string
		want  int
	}{
		{"{B}{B}", "B", 2},
		{"{1}{B}", "B", 1},
		{"{1}{B}", "R", 0},
		{"{B}{B}{B}", "B", 3},
		{"{3}", "B", 0},
		{"{W}{W}{W}", "W", 3},
		{"{U}{B}", "U", 1},
		{"{U}{B}", "B", 1},
		{"{U}{B}", "R", 0},
		{"", "B", 0},
		{"{R}{R}", "", 0},
	}
	for _, tc := range cases {
		if got := DevotionPipsFromManaCost(tc.cost, tc.color); got != tc.want {
			t.Errorf("DevotionPipsFromManaCost(%q, %q) = %d, want %d",
				tc.cost, tc.color, got, tc.want)
		}
	}
}

func TestDevotionPipsFromManaCost_HybridCountsBoth(t *testing.T) {
	// CR §700.5: hybrid pip counts as devotion to BOTH halves' colors.
	cost := "{B/R}{B/R}{1}"
	if got := DevotionPipsFromManaCost(cost, "B"); got != 2 {
		t.Errorf("hybrid {B/R}{B/R} devotion to B: want 2, got %d", got)
	}
	if got := DevotionPipsFromManaCost(cost, "R"); got != 2 {
		t.Errorf("hybrid {B/R}{B/R} devotion to R: want 2, got %d", got)
	}
	if got := DevotionPipsFromManaCost(cost, "U"); got != 0 {
		t.Errorf("hybrid {B/R} devotion to U: want 0, got %d", got)
	}
	// Mixed: one pure black + one hybrid.
	cost2 := "{B}{B/R}"
	if got := DevotionPipsFromManaCost(cost2, "B"); got != 2 {
		t.Errorf("{B}{B/R} devotion to B: want 2, got %d", got)
	}
	if got := DevotionPipsFromManaCost(cost2, "R"); got != 1 {
		t.Errorf("{B}{B/R} devotion to R: want 1, got %d", got)
	}
}

func TestDevotionPipsFromManaCost_TwobridAndPhyrexian(t *testing.T) {
	// {2/B}: twobrid — counts +1 toward B per §700.5.
	if got := DevotionPipsFromManaCost("{2/B}{2/B}", "B"); got != 2 {
		t.Errorf("twobrid {2/B}{2/B} devotion to B: want 2, got %d", got)
	}
	// {B/P}: Phyrexian — counts +1 toward B.
	if got := DevotionPipsFromManaCost("{B/P}{B}", "B"); got != 2 {
		t.Errorf("Phyrexian {B/P} + {B} devotion to B: want 2, got %d", got)
	}
}

func TestDevotionPipsFromManaCost_MalformedTolerant(t *testing.T) {
	// Unclosed brace stops parsing without panicking.
	if got := DevotionPipsFromManaCost("{B}{B}{R", "B"); got != 2 {
		t.Errorf("malformed cost: parser should stop at the unclosed brace; want 2, got %d", got)
	}
	if got := DevotionPipsFromManaCost("garbage", "B"); got != 0 {
		t.Errorf("no braces should yield 0, got %d", got)
	}
}

func TestCountDevotion_HybridAcrossBoardManaCost(t *testing.T) {
	// End-to-end: a 4-permanent board exercising single-color, generic,
	// and hybrid contributions across multiple devotion colors.
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "Black Knight", "{B}{B}")
	addCostedPermanent(gs, 0, "Bear", "{1}{B}")
	addCostedPermanent(gs, 0, "Rakdos Cackler", "{B/R}")
	addCostedPermanent(gs, 0, "Hill Giant", "{3}{R}")

	// Devotion to B: {B}{B}=2 + {1}{B}=1 + {B/R}=1 = 4
	if got := CountDevotion(gs, 0, "B"); got != 4 {
		t.Errorf("devotion to B: want 4, got %d", got)
	}
	// Devotion to R: {B/R}=1 + {R}=1 = 2
	if got := CountDevotion(gs, 0, "R"); got != 2 {
		t.Errorf("devotion to R: want 2, got %d", got)
	}
	// Devotion to U: 0
	if got := CountDevotion(gs, 0, "U"); got != 0 {
		t.Errorf("devotion to U: want 0, got %d", got)
	}
}

// ===========================================================================
// Legacy Colors fallback (no ManaCostString)
// ===========================================================================

func TestCountDevotion_FallsBackToColorsWhenNoManaCostString(t *testing.T) {
	// Permanent with Colors=[B] but no ManaCostString uses the legacy
	// "1 per color-matching permanent" heuristic. Preserves the
	// keywords_misc_test.go fixture behavior.
	gs := newDevotionGame(t)
	p := addCostedPermanent(gs, 0, "Legacy", "")
	p.Card.Colors = []string{"B"}

	if got := CountDevotion(gs, 0, "B"); got != 1 {
		t.Errorf("legacy fallback: 1 per Colors match; want 1, got %d", got)
	}
}

// ===========================================================================
// (b) HasDevotionRider detects oracle text + keyword
// ===========================================================================

func TestHasDevotionRider_DetectsKeyword(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "devotion"},
			},
		},
	}
	// Keyword path is color-agnostic — any color query returns true.
	for _, col := range []string{"W", "U", "B", "R", "G"} {
		if !HasDevotionRider(c, col) {
			t.Errorf("keyword-tagged devotion should match color %q", col)
		}
	}
}

func TestHasDevotionRider_DetectsOracleText(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Each opponent loses life equal to your devotion to black."},
			},
		},
	}
	if !HasDevotionRider(c, "B") {
		t.Fatal("HasDevotionRider should detect 'devotion to black' in oracle text")
	}
	if HasDevotionRider(c, "R") {
		t.Fatal("HasDevotionRider should NOT match a different color")
	}
}

func TestHasDevotionRider_AllColors(t *testing.T) {
	type tc struct {
		text  string
		color string
	}
	for _, x := range []tc{
		{"X is your devotion to white.", "W"},
		{"X is your devotion to blue.", "U"},
		{"X is your devotion to black.", "B"},
		{"X is your devotion to red.", "R"},
		{"X is your devotion to green.", "G"},
	} {
		c := &Card{
			AST: &gameast.CardAST{
				Abilities: []gameast.Ability{&gameast.Static{Raw: x.text}},
			},
		}
		if !HasDevotionRider(c, x.color) {
			t.Errorf("HasDevotionRider(%q, %q) = false; want true", x.text, x.color)
		}
	}
}

func TestHasDevotionRider_NegativeCases(t *testing.T) {
	if HasDevotionRider(nil, "B") {
		t.Fatal("HasDevotionRider(nil, ...) should be false")
	}
	if HasDevotionRider(&Card{}, "") {
		t.Fatal("HasDevotionRider with empty color should be false")
	}
	if HasDevotionRider(&Card{}, "Z") {
		t.Fatal("HasDevotionRider with unrecognized color code should be false")
	}
	// Card with text mentioning "devotion" but not the keyword phrase
	// pattern — should still match the substring "devotion to black"
	// if present; should NOT match for an unrelated color.
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Reduce by your devotion to black."},
			},
		},
	}
	if HasDevotionRider(c, "W") {
		t.Fatal("HasDevotionRider for the wrong color should not match")
	}
}

// ===========================================================================
// (c) DevotionTo color wrappers
// ===========================================================================

func TestDevotionToColor_Wrappers(t *testing.T) {
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "WW", "{W}{W}")
	addCostedPermanent(gs, 0, "UU", "{U}")
	addCostedPermanent(gs, 0, "BBB", "{B}{B}{B}")
	addCostedPermanent(gs, 0, "RR", "{R}{R}")
	addCostedPermanent(gs, 0, "G", "{G}")

	if got := DevotionToWhite(gs, 0); got != 2 {
		t.Errorf("DevotionToWhite: want 2, got %d", got)
	}
	if got := DevotionToBlue(gs, 0); got != 1 {
		t.Errorf("DevotionToBlue: want 1, got %d", got)
	}
	if got := DevotionToBlack(gs, 0); got != 3 {
		t.Errorf("DevotionToBlack: want 3, got %d", got)
	}
	if got := DevotionToRed(gs, 0); got != 2 {
		t.Errorf("DevotionToRed: want 2, got %d", got)
	}
	if got := DevotionToGreen(gs, 0); got != 1 {
		t.Errorf("DevotionToGreen: want 1, got %d", got)
	}
}

// ===========================================================================
// (d) Devotion flips as permanents enter/leave
// ===========================================================================

func TestCountDevotion_FlipsAsPermanentsEnterLeave(t *testing.T) {
	gs := newDevotionGame(t)
	// Start empty.
	if got := CountDevotion(gs, 0, "B"); got != 0 {
		t.Errorf("empty board: devotion to B should be 0, got %d", got)
	}

	p1 := addCostedPermanent(gs, 0, "BB", "{B}{B}")
	if got := CountDevotion(gs, 0, "B"); got != 2 {
		t.Errorf("after BB ETB: devotion to B want 2, got %d", got)
	}

	addCostedPermanent(gs, 0, "B", "{B}")
	if got := CountDevotion(gs, 0, "B"); got != 3 {
		t.Errorf("after 2nd ETB: devotion to B want 3, got %d", got)
	}

	// Remove p1 from the battlefield (simulating LTB).
	gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[1:]
	if got := CountDevotion(gs, 0, "B"); got != 1 {
		t.Errorf("after BB leaves: devotion to B want 1, got %d", got)
	}

	// Wipe.
	gs.Seats[0].Battlefield = nil
	if got := CountDevotion(gs, 0, "B"); got != 0 {
		t.Errorf("after board wipe: devotion to B want 0, got %d", got)
	}
	_ = p1
}

// ===========================================================================
// (e) Per-seat isolation
// ===========================================================================

func TestCountDevotion_PerSeatIsolation(t *testing.T) {
	gs := newDevotionGame(t)
	// Seat 0: 3 black pips.
	addCostedPermanent(gs, 0, "BB", "{B}{B}")
	addCostedPermanent(gs, 0, "B", "{B}")
	// Seat 1: 1 blue pip.
	addCostedPermanent(gs, 1, "U", "{U}")

	if got := CountDevotion(gs, 0, "B"); got != 3 {
		t.Errorf("seat 0 B: want 3, got %d", got)
	}
	if got := CountDevotion(gs, 1, "B"); got != 0 {
		t.Fatalf("seat 1's devotion to B must NOT see seat 0's black pips; got %d", got)
	}
	if got := CountDevotion(gs, 0, "U"); got != 0 {
		t.Fatalf("seat 0's devotion to U must NOT see seat 1's blue pips; got %d", got)
	}
	if got := CountDevotion(gs, 1, "U"); got != 1 {
		t.Errorf("seat 1 U: want 1, got %d", got)
	}
}

// ===========================================================================
// ApplyDevotionRider integration
// ===========================================================================

// devotionRiderCard builds a card with a tagged devotion-rider
// Activated ability whose payload is observable (a Draw effect).
func devotionRiderCard(name, oracleText string, payload gameast.Effect) *Card {
	return &Card{
		Name: name,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: oracleText},
				&gameast.Activated{
					Cost:   gameast.Cost{Extra: []string{"devotion_rider"}},
					Effect: payload,
					Raw:    "Devotion — payload",
				},
			},
		},
	}
}

func TestApplyDevotionRider_FiresAndRunsPayload(t *testing.T) {
	gs := newDevotionGame(t)
	// Seat 0: 4 devotion to black (Gray Merchant-ish).
	addCostedPermanent(gs, 0, "BB1", "{B}{B}")
	addCostedPermanent(gs, 0, "BB2", "{B}{B}")
	// Set up a library so the rider payload (draw) is observable.
	gs.Seats[0].Library = []*Card{{Name: "T1", Owner: 0}}

	card := devotionRiderCard("Gray Merchant",
		"Each opponent loses life equal to your devotion to black.",
		&gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		})
	src := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}

	handBefore := len(gs.Seats[0].Hand)
	fired := ApplyDevotionRider(gs, src, "B")
	handAfter := len(gs.Seats[0].Hand)

	if !fired {
		t.Fatal("ApplyDevotionRider should have fired for a card with a B devotion rider")
	}
	if handAfter-handBefore != 1 {
		t.Errorf("payload (draw 1) should run; drew %d", handAfter-handBefore)
	}

	// Event records the count and color.
	sawEvent := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "devotion_rider" && ev.Amount == 4 {
			if col, _ := ev.Details["color"].(string); col == "B" {
				if rule, _ := ev.Details["rule"].(string); rule == "700.5" {
					sawEvent = true
					break
				}
			}
		}
	}
	if !sawEvent {
		t.Error("expected a devotion_rider event with color=B amount=4 rule=700.5")
	}
}

func TestApplyDevotionRider_NoRiderForColorNoFire(t *testing.T) {
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "B", "{B}")
	// Card has a devotion-to-BLUE rider, not black.
	card := devotionRiderCard("Master of Waves",
		"Power equal to your devotion to blue.", nil)
	src := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}

	if ApplyDevotionRider(gs, src, "B") {
		t.Fatal("ApplyDevotionRider for color B must not fire on a U-only card")
	}
}

func TestApplyDevotionRider_PendingEventWhenNoTaggedPayload(t *testing.T) {
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "B", "{B}")
	// HasDevotionRider true (oracle text), but no tagged payload.
	card := &Card{
		Name: "Gray Merchant",
		AST: &gameast.CardAST{
			Name: "Gray Merchant",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Lose life equal to your devotion to black."},
			},
		},
	}
	src := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}

	if !ApplyDevotionRider(gs, src, "B") {
		t.Fatal("ApplyDevotionRider should fire even without tagged payload (logs pending)")
	}
	saw := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "devotion_rider_pending" {
			saw = true
			break
		}
	}
	if !saw {
		t.Error("expected a devotion_rider_pending event when AST lacks tagged payload")
	}
}

func TestApplyDevotionRider_NilSafe(t *testing.T) {
	if ApplyDevotionRider(nil, nil, "B") {
		t.Fatal("ApplyDevotionRider(nil, nil, ...) should be false")
	}
	gs := newDevotionGame(t)
	if ApplyDevotionRider(gs, nil, "B") {
		t.Fatal("ApplyDevotionRider(gs, nil, ...) should be false")
	}
	if ApplyDevotionRider(gs, &Permanent{}, "B") {
		t.Fatal("ApplyDevotionRider with nil-Card perm should be false")
	}
}

// ===========================================================================
// ApplyDevotionRidersAllColors fan-out
// ===========================================================================

func TestApplyDevotionRidersAllColors_FiresOnePerColorPresent(t *testing.T) {
	// Card with riders for two colors (B and R via separate oracle
	// lines). All-colors fan-out fires once per color whose rider is
	// present.
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "BR", "{B}{R}")
	card := &Card{
		Name: "Dual Devotion",
		AST: &gameast.CardAST{
			Name: "Dual Devotion",
			Abilities: []gameast.Ability{
				&gameast.Static{Raw: "Effect A is your devotion to black. Effect B is your devotion to red."},
			},
		},
	}
	src := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}

	ApplyDevotionRidersAllColors(gs, src)

	colors := map[string]int{}
	for _, ev := range gs.EventLog {
		if ev.Kind != "devotion_rider" {
			continue
		}
		col, _ := ev.Details["color"].(string)
		colors[col]++
	}
	if colors["B"] != 1 {
		t.Errorf("expected 1 devotion_rider event for B, got %d", colors["B"])
	}
	if colors["R"] != 1 {
		t.Errorf("expected 1 devotion_rider event for R, got %d", colors["R"])
	}
	if colors["W"] != 0 || colors["U"] != 0 || colors["G"] != 0 {
		t.Errorf("expected no events for unmentioned colors; got W=%d U=%d G=%d",
			colors["W"], colors["U"], colors["G"])
	}
}

// ===========================================================================
// resolveSequence wiring — outer-only firing for nested sequences
// ===========================================================================

func TestResolveSequence_FiresDevotionRiderOnceForNestedSequences(t *testing.T) {
	gs := newDevotionGame(t)
	addCostedPermanent(gs, 0, "B", "{B}{B}")

	card := devotionRiderCard("Nested Test",
		"Effect equal to your devotion to black.",
		&gameast.Draw{Count: *gameast.NumInt(1), Target: gameast.Filter{Base: "controller"}})
	src := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Library = []*Card{{Name: "L1", Owner: 0}, {Name: "L2", Owner: 0}, {Name: "L3", Owner: 0}}

	inner := &gameast.Sequence{Items: []gameast.Effect{}}
	outer := &gameast.Sequence{Items: []gameast.Effect{inner}}
	resolveSequence(gs, src, outer)

	riderCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "devotion_rider" {
			riderCount++
		}
	}
	if riderCount != 1 {
		t.Errorf("devotion rider should fire exactly once for nested-sequence resolution; got %d", riderCount)
	}
}
