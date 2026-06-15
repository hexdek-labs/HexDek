package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// undying_persist_r63_test.go — behavioral regressions for Undying (CR §702.93)
// and Persist (CR §702.92), which were entirely unwired before r63 (the
// Check* helpers had no callers; nothing returned these creatures on death).
// Deaths are driven through the real path (SacrificePermanent → the dies
// chokepoint) so the new FireZoneChangeTriggers wiring is exercised end-to-end.

func newUPGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(91)), nil)
	gs.Active = 0
	return gs
}

func upCreature(gs *GameState, seat int, name string, pt int, keywords ...string) *Permanent {
	abilities := make([]gameast.Ability, len(keywords))
	for i, kw := range keywords {
		abilities[i] = &gameast.Keyword{Name: kw}
	}
	card := &Card{
		Name:          name,
		Owner:         seat,
		Types:         []string{"creature"},
		BasePower:     pt,
		BaseToughness: pt,
		AST:           &gameast.CardAST{Name: name, Abilities: abilities},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func creaturesNamed(gs *GameState, seat int, name string) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			out = append(out, p)
		}
	}
	return out
}

func upCardInGraveyard(gs *GameState, seat int, name string) bool {
	for _, c := range gs.Seats[seat].Graveyard {
		if c != nil && c.Name == name {
			return true
		}
	}
	return false
}

// (a)+(d) Undying: dies with no +1/+1 → returns with one +1/+1, as a NEW,
// summoning-sick object; the card left the graveyard.
func TestUndying_ReturnsWithCounter_NewSickObject(t *testing.T) {
	gs := newUPGame(t)
	wolf := upCreature(gs, 0, "Young Wolf", 1, "undying")

	SacrificePermanent(gs, wolf, "test")

	got := creaturesNamed(gs, 0, "Young Wolf")
	if len(got) != 1 {
		t.Fatalf("(a) undying should return exactly one Young Wolf; got %d", len(got))
	}
	r := got[0]
	if r == wolf {
		t.Error("(d) the returned creature must be a NEW object")
	}
	if r.Counters["+1/+1"] != 1 {
		t.Errorf("(a) returned creature should have one +1/+1 counter; got %d", r.Counters["+1/+1"])
	}
	if !r.SummoningSick {
		t.Error("(d) the returned creature must be summoning-sick")
	}
	if upCardInGraveyard(gs, 0, "Young Wolf") {
		t.Error("(d) the card must leave the graveyard when it returns")
	}
}

// (b) Persist: dies with no -1/-1 → returns with one -1/-1.
func TestPersist_ReturnsWithMinusCounter(t *testing.T) {
	gs := newUPGame(t)
	redcap := upCreature(gs, 0, "Murderous Redcap", 3, "persist") // 3/3 so -1/-1 leaves it alive

	SacrificePermanent(gs, redcap, "test")

	got := creaturesNamed(gs, 0, "Murderous Redcap")
	if len(got) != 1 {
		t.Fatalf("(b) persist should return exactly one creature; got %d", len(got))
	}
	if got[0].Counters["-1/-1"] != 1 {
		t.Errorf("(b) returned creature should have one -1/-1 counter; got %d", got[0].Counters["-1/-1"])
	}
}

// (c) LKI: a creature that died WITH the relevant counter does NOT return.
func TestUndyingPersist_LKICounterBlocksReturn(t *testing.T) {
	// Undying with a +1/+1 already on it → no return.
	gs := newUPGame(t)
	wolf := upCreature(gs, 0, "Scavenging Ooze", 2, "undying")
	wolf.Counters["+1/+1"] = 1
	SacrificePermanent(gs, wolf, "test")
	if n := len(creaturesNamed(gs, 0, "Scavenging Ooze")); n != 0 {
		t.Errorf("(c) undying creature that died WITH a +1/+1 must not return; on bf=%d", n)
	}
	if !upCardInGraveyard(gs, 0, "Scavenging Ooze") {
		t.Error("(c) the non-returning undying creature should stay in the graveyard")
	}

	// Persist with a -1/-1 already on it → no return (combo terminator).
	gs2 := newUPGame(t)
	rc := upCreature(gs2, 0, "Kitchen Finks", 3, "persist")
	rc.Counters["-1/-1"] = 1
	SacrificePermanent(gs2, rc, "test")
	if n := len(creaturesNamed(gs2, 0, "Kitchen Finks")); n != 0 {
		t.Errorf("(c) persist creature that died WITH a -1/-1 must not return; on bf=%d", n)
	}
}

// (e) The entering counter respects counter doublers (Doubling Season).
func TestUndyingPersist_CounterRespectsDoublers(t *testing.T) {
	// Undying + Doubling Season → 2 +1/+1.
	gs := newUPGame(t)
	ds := upCreature(gs, 0, "Doubling Season", 0)
	RegisterDoublingSeason(gs, ds)
	wolf := upCreature(gs, 0, "Young Wolf", 1, "undying")
	SacrificePermanent(gs, wolf, "test")
	got := creaturesNamed(gs, 0, "Young Wolf")
	if len(got) != 1 || got[0].Counters["+1/+1"] != 2 {
		t.Errorf("(e) undying counter must double under Doubling Season → 2; got %v", got)
	}

	// Persist + Doubling Season → 2 -1/-1.
	gs2 := newUPGame(t)
	ds2 := upCreature(gs2, 0, "Doubling Season", 0)
	RegisterDoublingSeason(gs2, ds2)
	rc := upCreature(gs2, 0, "Murderous Redcap", 4, "persist")
	SacrificePermanent(gs2, rc, "test")
	got2 := creaturesNamed(gs2, 0, "Murderous Redcap")
	if len(got2) != 1 || got2[0].Counters["-1/-1"] != 2 {
		t.Errorf("(e) persist counter must double under Doubling Season → 2; got %v", got2)
	}
}

// (f) Persist iteration: returns once with a -1/-1; the SECOND death (now WITH
// the -1/-1) does NOT return — the combo terminates without an external
// +1/+1 source.
func TestPersist_SecondDeathWithCounterDoesNotReturn(t *testing.T) {
	gs := newUPGame(t)
	rc := upCreature(gs, 0, "Murderous Redcap", 3, "persist")
	SacrificePermanent(gs, rc, "first")

	got := creaturesNamed(gs, 0, "Murderous Redcap")
	if len(got) != 1 || got[0].Counters["-1/-1"] != 1 {
		t.Fatalf("setup: expected one returned redcap with a -1/-1; got %v", got)
	}
	// Kill the returned one — it now has a -1/-1, so persist must NOT fire.
	SacrificePermanent(gs, got[0], "second")
	if n := len(creaturesNamed(gs, 0, "Murderous Redcap")); n != 0 {
		t.Errorf("(f) the -1/-1 must block the second persist return; on bf=%d", n)
	}
}

// (g) A creature with BOTH undying and persist returns exactly ONCE (the object
// can only be returned once, CR §702.93b).
func TestUndyingAndPersist_ReturnsExactlyOnce(t *testing.T) {
	gs := newUPGame(t)
	both := upCreature(gs, 0, "Twice-Risen", 3, "undying", "persist")

	SacrificePermanent(gs, both, "test")

	got := creaturesNamed(gs, 0, "Twice-Risen")
	if len(got) != 1 {
		t.Fatalf("(g) undying+persist must return exactly ONE creature; got %d", len(got))
	}
	// Undying is checked first → it comes back with a +1/+1, not a -1/-1.
	if got[0].Counters["+1/+1"] != 1 || got[0].Counters["-1/-1"] != 0 {
		t.Errorf("(g) expected the single return to carry undying's +1/+1; got +1/+1=%d -1/-1=%d",
			got[0].Counters["+1/+1"], got[0].Counters["-1/-1"])
	}
}
