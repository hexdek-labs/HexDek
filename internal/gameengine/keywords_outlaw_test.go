package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Outlaw type-group tests — CR §205.3m
// ---------------------------------------------------------------------------

func newOutlawGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(56)), nil)
}

func subtypeCreature(name string, owner int, subtypes ...string) *Card {
	types := append([]string{"creature"}, subtypes...)
	typeLine := "Creature — " + name + " (" + subtypesJoin(subtypes) + ")"
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         types,
		TypeLine:      typeLine,
		BasePower:     2,
		BaseToughness: 2,
		AST:           &gameast.CardAST{Name: name},
	}
}

func subtypesJoin(subs []string) string {
	out := ""
	for i, s := range subs {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out
}

func subtypePerm(gs *GameState, owner int, name string, subtypes ...string) *Permanent {
	card := subtypeCreature(name, owner, subtypes...)
	perm := &Permanent{
		Card:       card,
		Controller: owner,
		Owner:      owner,
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[owner].Battlefield = append(gs.Seats[owner].Battlefield, perm)
	return perm
}

// ---------------------------------------------------------------------------
// (a) IsOutlaw true for the five canonical subtypes
// ---------------------------------------------------------------------------

func TestIsOutlaw_AcceptsAllFiveSubtypes(t *testing.T) {
	cases := []string{"assassin", "mercenary", "pirate", "rogue", "warlock"}
	for _, sub := range cases {
		c := subtypeCreature("Outlaw "+sub, 0, sub)
		if !IsOutlaw(c) {
			t.Errorf("IsOutlaw should accept %s creature", sub)
		}
	}
}

func TestIsOutlaw_CaseInsensitive(t *testing.T) {
	c := subtypeCreature("Capital", 0, "ASSASSIN")
	if !IsOutlaw(c) {
		t.Fatal("IsOutlaw should be case-insensitive on the subtype")
	}
}

func TestIsOutlaw_MultiSubtypeCountsAsOne(t *testing.T) {
	// CR §205.3m: an Assassin Rogue is still ONE outlaw.
	c := subtypeCreature("Assassin Rogue", 0, "assassin", "rogue")
	if !IsOutlaw(c) {
		t.Fatal("IsOutlaw should be true for multi-Outlaw-subtype creatures")
	}
}

func TestIsOutlaw_TypeLineFallback(t *testing.T) {
	// Types[] doesn't carry the subtype, but TypeLine does.
	c := &Card{
		Name:     "TypeLine-Only Rogue",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Goblin Rogue",
	}
	if !IsOutlaw(c) {
		t.Fatal("IsOutlaw should consult TypeLine when Types[] lacks the subtype")
	}
}

func TestIsOutlaw_TypeLineWholeWordOnly(t *testing.T) {
	// "Spirate" contains "irate" but no whole-word "pirate" — must not
	// false-positive. Use a contrived type line.
	c := &Card{
		Name:     "Spirate",
		Owner:    0,
		Types:    []string{"creature"},
		TypeLine: "Creature — Spirate",
	}
	if IsOutlaw(c) {
		t.Fatal("IsOutlaw must not substring-match 'pirate' inside 'spirate'")
	}
}

// ---------------------------------------------------------------------------
// (b) IsOutlaw false for non-outlaw subtypes
// ---------------------------------------------------------------------------

func TestIsOutlaw_FalseForBearAndGoblin(t *testing.T) {
	bear := subtypeCreature("Grizzly Bears", 0, "bear")
	if IsOutlaw(bear) {
		t.Fatal("a Bear is not an Outlaw")
	}
	goblin := subtypeCreature("Goblin Brigand", 0, "goblin")
	if IsOutlaw(goblin) {
		t.Fatal("a Goblin is not an Outlaw")
	}
}

func TestIsOutlaw_FalseForTypelessAndNonCreatures(t *testing.T) {
	c := &Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	if IsOutlaw(c) {
		t.Fatal("artifact must not be an outlaw")
	}
	if IsOutlaw(nil) {
		t.Fatal("IsOutlaw(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// (c) CountOutlawsControlled accurate
// ---------------------------------------------------------------------------

func TestCountOutlawsControlled_TalliesAcrossSubtypes(t *testing.T) {
	gs := newOutlawGame(t)
	subtypePerm(gs, 0, "Assassin", "assassin")
	subtypePerm(gs, 0, "Pirate", "pirate")
	subtypePerm(gs, 0, "Rogue", "rogue")
	subtypePerm(gs, 0, "Bear", "bear") // not counted
	subtypePerm(gs, 0, "Warlock", "warlock")
	subtypePerm(gs, 0, "Mercenary", "mercenary")

	got := CountOutlawsControlled(gs, 0)
	if got != 5 {
		t.Fatalf("CountOutlawsControlled = %d, want 5", got)
	}
}

func TestCountOutlawsControlled_PerSeat(t *testing.T) {
	gs := newOutlawGame(t)
	subtypePerm(gs, 0, "A", "assassin")
	subtypePerm(gs, 0, "B", "rogue")
	subtypePerm(gs, 1, "C", "pirate")

	if CountOutlawsControlled(gs, 0) != 2 {
		t.Fatalf("seat 0 outlaws = %d, want 2", CountOutlawsControlled(gs, 0))
	}
	if CountOutlawsControlled(gs, 1) != 1 {
		t.Fatalf("seat 1 outlaws = %d, want 1", CountOutlawsControlled(gs, 1))
	}
}

func TestCountOutlawsControlled_MultiSubtypeOutlawCountsOnce(t *testing.T) {
	gs := newOutlawGame(t)
	// CR §205.3m: an Assassin Rogue is still ONE outlaw.
	subtypePerm(gs, 0, "Assassin Rogue", "assassin", "rogue")
	if CountOutlawsControlled(gs, 0) != 1 {
		t.Fatalf("multi-Outlaw-subtype perm should count once, got %d",
			CountOutlawsControlled(gs, 0))
	}
}

func TestCountOutlawsControlled_NilSafe(t *testing.T) {
	if CountOutlawsControlled(nil, 0) != 0 {
		t.Fatal("nil game must return 0")
	}
	gs := newOutlawGame(t)
	if CountOutlawsControlled(gs, -1) != 0 {
		t.Fatal("invalid seat must return 0")
	}
	if CountOutlawsControlled(gs, 99) != 0 {
		t.Fatal("out-of-range seat must return 0")
	}
}

// ---------------------------------------------------------------------------
// (d) Outlaw-watching trigger fires on outlaw ETB
// ---------------------------------------------------------------------------

func TestHasOutlawTrigger_DetectsCanonicalPhrasings(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Whenever an Outlaw enters the battlefield under your control, do Y.", true},
		{"When an Outlaw enters the battlefield, draw a card.", true},
		{"Whenever another Outlaw enters the battlefield, gain 1 life.", true},
		{"Whenever you cast an Outlaw spell, scry 1.", true},
		{"Flying.", false},
		{"", false},
	}
	for _, c := range cases {
		card := &Card{Name: "x", AST: &gameast.CardAST{Name: "x"}}
		card.OracleTextCache = c.text
		card.oracleTextAST = card.AST
		if got := HasOutlawTrigger(card); got != c.want {
			t.Errorf("HasOutlawTrigger(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestFireOutlawETBTriggers_FiresOnOutlawETB(t *testing.T) {
	gs := newOutlawGame(t)

	// Watcher card with the trigger preamble.
	watcher := subtypePerm(gs, 0, "Bonny Pall, Clearcutter", "demon")
	watcher.Card.OracleTextCache = "whenever an outlaw enters the battlefield under your control, draw a card"
	watcher.Card.oracleTextAST = watcher.Card.AST

	// Newly ETB'ing outlaw.
	rogue := subtypePerm(gs, 0, "Stealthy Rogue", "rogue")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	var sawCtx map[string]interface{}
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "outlaw_etb" {
			sawCtx = ctx
		}
	}

	FireOutlawETBTriggers(gs, rogue)

	if sawCtx == nil {
		t.Fatal("FireOutlawETBTriggers should fire outlaw_etb when the entering perm is an outlaw")
	}
	if got, _ := sawCtx["perm"].(*Permanent); got != rogue {
		t.Fatal("ctx[perm] should reference the entering outlaw")
	}
	if w, _ := sawCtx["watcher"].(*Permanent); w != watcher {
		t.Fatal("ctx[watcher] should reference the watching permanent")
	}
}

func TestFireOutlawETBTriggers_PipelineIntegration(t *testing.T) {
	gs := newOutlawGame(t)
	watcher := subtypePerm(gs, 0, "Outlaw Hunter", "human")
	watcher.Card.OracleTextCache = "whenever an outlaw enters the battlefield, draw a card"
	watcher.Card.oracleTextAST = watcher.Card.AST

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawEvent := ""
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "outlaw_etb" {
			sawEvent = ev
		}
	}

	// Go through the full FirePermanentETBTriggers pipeline.
	pirate := subtypePerm(gs, 0, "Pirate", "pirate")
	FirePermanentETBTriggers(gs, pirate)

	if sawEvent != "outlaw_etb" {
		t.Fatal("FirePermanentETBTriggers should run the Outlaw ETB fan-out")
	}
}

// ---------------------------------------------------------------------------
// (e) Does NOT fire on non-outlaw ETB
// ---------------------------------------------------------------------------

func TestFireOutlawETBTriggers_NoopOnNonOutlaw(t *testing.T) {
	gs := newOutlawGame(t)
	watcher := subtypePerm(gs, 0, "Watcher", "human")
	watcher.Card.OracleTextCache = "whenever an outlaw enters the battlefield, gain 1 life"
	watcher.Card.oracleTextAST = watcher.Card.AST

	bear := subtypePerm(gs, 0, "Bear", "bear")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "outlaw_etb" {
			fired = true
		}
	}

	FireOutlawETBTriggers(gs, bear)
	if fired {
		t.Fatal("non-outlaw ETB must not fire outlaw_etb")
	}
}

func TestFireOutlawETBTriggers_NoWatchersIsCheap(t *testing.T) {
	gs := newOutlawGame(t)
	// Battlefield has no watchers — the helper should just early-return.
	subtypePerm(gs, 0, "Vanilla", "human") // not a watcher
	rogue := subtypePerm(gs, 0, "Rogue", "rogue")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "outlaw_etb" {
			fired = true
		}
	}

	FireOutlawETBTriggers(gs, rogue)
	if fired {
		t.Fatal("outlaw_etb must not fire when no permanent watches for outlaws")
	}
}

func TestFireOutlawETBTriggers_FiresAcrossSeats(t *testing.T) {
	gs := newOutlawGame(t)
	// Watcher on seat 1's battlefield (some cards watch the global outlaw
	// ETB, not just under-your-control — the engine fires for every
	// watcher and per-card handlers gate "your control" themselves).
	watcher := subtypePerm(gs, 1, "Global Watcher", "spirit")
	watcher.Card.OracleTextCache = "whenever an outlaw enters the battlefield, do Y"
	watcher.Card.oracleTextAST = watcher.Card.AST

	pirate := subtypePerm(gs, 0, "Pirate", "pirate")

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	watcherSeen := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev != "outlaw_etb" {
			return
		}
		if w, ok := ctx["watcher"].(*Permanent); ok && w == watcher {
			watcherSeen = true
		}
	}

	FireOutlawETBTriggers(gs, pirate)
	if !watcherSeen {
		t.Fatal("FireOutlawETBTriggers must fan out to watchers on opponents' battlefields too")
	}
}

// ---------------------------------------------------------------------------
// Nil safety
// ---------------------------------------------------------------------------

func TestFireOutlawETBTriggers_NilSafe(t *testing.T) {
	gs := newOutlawGame(t)
	FireOutlawETBTriggers(nil, &Permanent{}) // must not panic
	FireOutlawETBTriggers(gs, nil)
}

func TestPermIsOutlaw_NilSafe(t *testing.T) {
	if PermIsOutlaw(nil) {
		t.Fatal("PermIsOutlaw(nil) should be false")
	}
}

func TestHasOutlawTrigger_NilSafe(t *testing.T) {
	if HasOutlawTrigger(nil) {
		t.Fatal("HasOutlawTrigger(nil) should be false")
	}
}

func TestOutlawSubtypes_ReturnsFiveCanonical(t *testing.T) {
	got := OutlawSubtypes()
	if len(got) != 5 {
		t.Fatalf("OutlawSubtypes len = %d, want 5", len(got))
	}
	want := map[string]bool{"assassin": true, "mercenary": true, "pirate": true, "rogue": true, "warlock": true}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected outlaw subtype: %q", s)
		}
	}
}
