package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// suspend_time_counters_test.go — regressions for the §702.62 (suspend),
// §702.63 (vanishing) and §702.32 (fading) upkeep countdowns and the
// §702.62g free-cast. Before suspend_time_counters.go these counters were
// placed but never read; ~107 cards were inert past entry.

// --- Vanishing / Fading (TickTimeFadeCounters) ----------------------------

func mkTimeFadePerm(gs *GameState, seat int, name, kw string, n int) *Permanent {
	counterKind := "time"
	if kw == "fading" {
		counterKind = "fade"
	}
	card := &Card{
		Name: name, Owner: seat, Types: []string{"creature"},
		BasePower: 2, BaseToughness: 2,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: kw, Raw: kw, Args: []interface{}{n}},
		}},
	}
	p := &Permanent{
		Card: card, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{counterKind: n},
		Flags:     map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Vanishing N: a time counter is removed each upkeep; when the LAST is
// removed (count hits 0), the permanent is sacrificed (§702.63c). So
// vanishing-3 is sacrificed on the 3rd tick.
func TestVanishing_CountdownSacrificesWhenLastRemoved(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTimeFadePerm(gs, 0, "Errant Ephemeron", "vanishing", 3)

	// Tick 1: 3 -> 2, survives.
	TickTimeFadeCounters(gs, 0)
	if p.Counters["time"] != 2 {
		t.Fatalf("after tick 1 want 2 time counters, got %d", p.Counters["time"])
	}
	if !onBattlefield(gs, p) {
		t.Fatalf("vanishing perm must survive tick 1")
	}
	// Tick 2: 2 -> 1, survives.
	TickTimeFadeCounters(gs, 0)
	if p.Counters["time"] != 1 {
		t.Fatalf("after tick 2 want 1 time counter, got %d", p.Counters["time"])
	}
	if !onBattlefield(gs, p) {
		t.Fatalf("vanishing perm must survive tick 2")
	}
	// Tick 3: 1 -> 0, last counter removed -> sacrifice.
	TickTimeFadeCounters(gs, 0)
	if onBattlefield(gs, p) {
		t.Fatalf("vanishing perm must be sacrificed on the tick that removes its last time counter")
	}
	if !inGraveyard(gs, 0, "Errant Ephemeron") {
		t.Fatalf("sacrificed vanishing perm must be in its owner's graveyard")
	}
}

// Fading N: a fade counter is removed each upkeep; if there is none to
// remove, the permanent is sacrificed (§702.32b). So fading-2 survives 2
// removal ticks and is sacrificed on the 3rd ("can't remove").
func TestFading_CountdownSacrificesWhenCannotRemove(t *testing.T) {
	gs := newTestGame(t, 2)
	p := mkTimeFadePerm(gs, 0, "Blastoderm", "fading", 2)

	TickTimeFadeCounters(gs, 0) // 2 -> 1
	if p.Counters["fade"] != 1 || !onBattlefield(gs, p) {
		t.Fatalf("fading: after tick 1 want 1 fade counter and alive, got %d alive=%v", p.Counters["fade"], onBattlefield(gs, p))
	}
	TickTimeFadeCounters(gs, 0) // 1 -> 0, still alive (had a counter to remove)
	if p.Counters["fade"] != 0 || !onBattlefield(gs, p) {
		t.Fatalf("fading: after tick 2 want 0 fade counters and alive, got %d alive=%v", p.Counters["fade"], onBattlefield(gs, p))
	}
	TickTimeFadeCounters(gs, 0) // can't remove -> sacrifice
	if onBattlefield(gs, p) {
		t.Fatalf("fading perm must be sacrificed on the upkeep it cannot remove a fade counter")
	}
	if !inGraveyard(gs, 0, "Blastoderm") {
		t.Fatalf("sacrificed fading perm must be in its owner's graveyard")
	}
}

// Only the active seat's permanents tick down (the upkeep belongs to one
// player). Ticking seat 0 must not touch seat 1's vanishing perm.
func TestTimeFade_OnlyActiveSeatTicks(t *testing.T) {
	gs := newTestGame(t, 2)
	mine := mkTimeFadePerm(gs, 0, "Mine VF", "vanishing", 2)
	theirs := mkTimeFadePerm(gs, 1, "Their VF", "vanishing", 2)

	TickTimeFadeCounters(gs, 0)
	if mine.Counters["time"] != 1 {
		t.Fatalf("active seat's perm must tick, got %d", mine.Counters["time"])
	}
	if theirs.Counters["time"] != 2 {
		t.Fatalf("non-active seat's perm must NOT tick, got %d", theirs.Counters["time"])
	}
}

// --- Suspend (TickSuspendCounters + CastSuspendedCard) --------------------

// A suspended creature counts down each upkeep and is cast for free with
// haste when the last time counter is removed (§702.62f/g). The card must
// move exile -> battlefield exactly once (no ZoneConservation leak).
func TestSuspend_TicksThenCastsCreatureWithHaste(t *testing.T) {
	gs := newTestGame(t, 2)
	card := &Card{
		Name: "Aeon Chronicler", Owner: 0, Types: []string{"creature"},
		BasePower: 4, BaseToughness: 4,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	SuspendCard(gs, 0, card, 2)

	if len(gs.Seats[0].Exile) != 1 || gs.Seats[0].Exile[0] != card {
		t.Fatalf("SuspendCard must place the card in exile")
	}
	if susp, _ := card.Meta["suspended"].(bool); !susp {
		t.Fatalf("SuspendCard must mark the card suspended in Meta")
	}

	// Tick 1: 2 -> 1, still suspended in exile.
	TickSuspendCounters(gs, 0)
	if n, _ := card.Meta["suspend_counters"].(int); n != 1 {
		t.Fatalf("after tick 1 want 1 counter, got %d", n)
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Fatalf("card must still be in exile after tick 1")
	}

	// Tick 2: 1 -> 0, cast for free.
	TickSuspendCounters(gs, 0)

	// No longer in exile (cast removed it; no duplication).
	if len(gs.Seats[0].Exile) != 0 {
		t.Fatalf("suspended card must leave exile when cast, exile=%d", len(gs.Seats[0].Exile))
	}
	// On the battlefield exactly once, as a creature, with haste.
	var found *Permanent
	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			found = p
			count++
		}
	}
	if count != 1 {
		t.Fatalf("suspended creature must resolve onto the battlefield exactly once, got %d", count)
	}
	if found.Flags["kw:haste"] != 1 {
		t.Fatalf("a suspended creature cast for free must gain haste (§702.62g)")
	}
}

// A suspended instant/sorcery resolves its spell and is put into the
// graveyard — it must not linger in exile or on the battlefield.
func TestSuspend_InstantResolvesToGraveyard(t *testing.T) {
	gs := newTestGame(t, 2)
	card := &Card{
		Name: "Rift Bolt", Owner: 0, Types: []string{"sorcery"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}},
	}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, card)
	card.Meta = map[string]any{
		"suspended": true, "suspend_counters": 1, "suspend_controller": 0,
	}

	TickSuspendCounters(gs, 0)

	if len(gs.Seats[0].Exile) != 0 {
		t.Fatalf("cast suspended sorcery must leave exile, exile=%d", len(gs.Seats[0].Exile))
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			t.Fatalf("a sorcery must not stay on the battlefield")
		}
	}
	if !inGraveyard(gs, 0, "Rift Bolt") {
		t.Fatalf("a resolved suspended sorcery must be in the graveyard")
	}
}

// The suspend tick belongs to the controller's upkeep only — ticking seat 1
// must not advance seat 0's suspended card.
func TestSuspend_OnlyControllerUpkeepTicks(t *testing.T) {
	gs := newTestGame(t, 2)
	card := &Card{
		Name: "Errant Ephemeron", Owner: 0, Types: []string{"creature"},
		BasePower: 6, BaseToughness: 4,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}},
	}
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, card)
	card.Meta = map[string]any{
		"suspended": true, "suspend_counters": 3, "suspend_controller": 0,
	}

	// Opponent's upkeep: tick seat 1 — seat 0's card must be untouched.
	TickSuspendCounters(gs, 1)
	if n, _ := card.Meta["suspend_counters"].(int); n != 3 {
		t.Fatalf("opponent upkeep must not decrement controller's suspend counters, got %d", n)
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Fatalf("card must remain in exile after an opponent upkeep")
	}
}

// CastSuspendedCard returns false (and is a no-op) when the card is not in
// the seat's exile — defends the cast path against a stale/duplicate call.
func TestCastSuspendedCard_NoopWhenNotInExile(t *testing.T) {
	gs := newTestGame(t, 2)
	card := &Card{Name: "Ghostly Flicker", Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	if CastSuspendedCard(gs, 0, card) {
		t.Fatalf("CastSuspendedCard must return false when the card is not in exile")
	}
	if len(gs.Stack) != 0 {
		t.Fatalf("no-op cast must not push anything onto the stack")
	}
}
