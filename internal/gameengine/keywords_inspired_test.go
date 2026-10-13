package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Inspired tests — CR §702.124
// ---------------------------------------------------------------------------

func newInspiredCreature(name string, owner, power, toughness int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
		CMC:           2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "inspired"},
			},
		},
	}
}

func newPlainCreature(name string, owner, power, toughness int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
		CMC:           2,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newInspiredGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(17))
	gs := NewGameState(2, rng, nil)
	gs.Active = 0
	gs.Step = "untap"
	gs.Turn = 1
	return gs
}

// putBattlefield drops a card onto seat's battlefield as a tapped creature.
func putBattlefield(gs *GameState, seat int, card *Card, tapped bool) *Permanent {
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Tapped:     tapped,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

// inspiredFires returns how many "inspired" events were captured, and
// the list of source permanents that fired.
func inspiredFires(captured []capturedTrigger) (int, []*Permanent) {
	var srcs []*Permanent
	n := 0
	for _, c := range captured {
		if c.event != "inspired" {
			continue
		}
		n++
		if src, _ := c.ctx["source"].(*Permanent); src != nil {
			srcs = append(srcs, src)
		}
	}
	return n, srcs
}

// ---------------------------------------------------------------------------
// HasInspired / HasInspiredPerm
// ---------------------------------------------------------------------------

func TestHasInspired_Detects(t *testing.T) {
	card := newInspiredCreature("Siren of the Silent Song", 0, 2, 2)
	if !HasInspired(card) {
		t.Fatal("HasInspired returned false for an inspired creature")
	}
}

func TestHasInspired_Negative(t *testing.T) {
	if HasInspired(newPlainCreature("Grizzly Bears", 0, 2, 2)) {
		t.Fatal("HasInspired should be false for a creature without the keyword")
	}
	if HasInspired(nil) {
		t.Fatal("HasInspired(nil) should be false")
	}
}

func TestHasInspiredPerm_GrantedAbilityPath(t *testing.T) {
	gs := newInspiredGame(t)
	card := newPlainCreature("Hill Giant", 0, 3, 3)
	perm := putBattlefield(gs, 0, card, false)
	perm.GrantedAbilities = []string{"inspired"}
	if !HasInspiredPerm(gs, perm) {
		t.Fatal("HasInspiredPerm should pick up a granted inspired ability")
	}
}

// ---------------------------------------------------------------------------
// (a) Inspired trigger fires on the untap step
// ---------------------------------------------------------------------------

func TestInspired_FiresOnUntapStep(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)

	UntapAll(gs, 0)

	if perm.Tapped {
		t.Fatal("UntapAll should have untapped the inspired permanent")
	}
	n, srcs := inspiredFires(*captured)
	if n != 1 {
		t.Fatalf("expected 1 inspired trigger, got %d", n)
	}
	if len(srcs) != 1 || srcs[0] != perm {
		t.Fatalf("inspired source = %v, want %v", srcs, perm)
	}
}

// ---------------------------------------------------------------------------
// (b) Inspired trigger fires on UntapPermanent (Bear Umbra-style mid-turn)
// ---------------------------------------------------------------------------

func TestInspired_FiresOnUntapPermanent_MidTurn(t *testing.T) {
	gs := newInspiredGame(t)
	gs.Step = "postcombat_main"
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)

	if !UntapPermanent(gs, perm, "bear_umbra") {
		t.Fatal("UntapPermanent should report a transition for a tapped inspired permanent")
	}
	if perm.Tapped {
		t.Fatal("UntapPermanent should have flipped Tapped to false")
	}
	n, _ := inspiredFires(*captured)
	if n != 1 {
		t.Fatalf("expected 1 inspired trigger on mid-turn untap, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// (c) No trigger if already untapped
// ---------------------------------------------------------------------------

func TestInspired_NoTriggerIfAlreadyUntapped_UntapStep(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	// Permanent is already untapped (Tapped=false) — UntapAll should
	// skip it; no inspired trigger should fire.
	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), false)

	UntapAll(gs, 0)

	if perm.Tapped {
		t.Fatal("permanent should still be untapped")
	}
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("expected 0 inspired triggers for an already-untapped permanent, got %d", n)
	}
}

func TestInspired_NoTriggerIfAlreadyUntapped_UntapPermanent(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), false)

	if UntapPermanent(gs, perm, "manual") {
		t.Fatal("UntapPermanent should report false for an already-untapped permanent")
	}
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("expected 0 inspired triggers for already-untapped permanent, got %d", n)
	}
}

func TestInspired_StunCounterVetoesUntapAndTrigger(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)
	perm.Counters = map[string]int{"stun": 1}

	if UntapPermanent(gs, perm, "manual") {
		t.Fatal("UntapPermanent should report false when a stun counter vetoes the untap (§122.4)")
	}
	if !perm.Tapped {
		t.Fatal("stun-vetoed permanent should remain tapped")
	}
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("expected 0 inspired triggers when stun vetoed the untap, got %d", n)
	}
	if perm.Counters["stun"] != 0 {
		t.Fatalf("stun counter should be decremented to 0, got %d", perm.Counters["stun"])
	}
}

// ---------------------------------------------------------------------------
// (d) Multiple inspired creatures all fire
// ---------------------------------------------------------------------------

func TestInspired_MultipleCreatures_AllFire(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	a := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)
	b := putBattlefield(gs, 0, newInspiredCreature("Felhide Spiritbinder", 0, 3, 3), true)
	c := putBattlefield(gs, 0, newInspiredCreature("Ephara's Enlightenment", 0, 1, 1), true)
	// One plain creature that should not fire inspired even though it
	// gets untapped at the same time.
	plain := putBattlefield(gs, 0, newPlainCreature("Hill Giant", 0, 3, 3), true)

	UntapAll(gs, 0)

	for _, p := range []*Permanent{a, b, c, plain} {
		if p.Tapped {
			t.Fatalf("permanent %q should be untapped after UntapAll", p.Card.DisplayName())
		}
	}
	n, srcs := inspiredFires(*captured)
	if n != 3 {
		t.Fatalf("expected 3 inspired triggers (one per inspired creature), got %d", n)
	}
	gotPerms := map[*Permanent]bool{}
	for _, s := range srcs {
		gotPerms[s] = true
	}
	for _, want := range []*Permanent{a, b, c} {
		if !gotPerms[want] {
			t.Fatalf("inspired trigger missing for %q", want.Card.DisplayName())
		}
	}
	if gotPerms[plain] {
		t.Fatal("plain creature should NOT have fired inspired")
	}
}

func TestInspired_RepeatTransitionsFireRepeatedly(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()

	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)

	UntapPermanent(gs, perm, "untap_1")
	perm.Tapped = true
	UntapPermanent(gs, perm, "untap_2")
	perm.Tapped = true
	UntapPermanent(gs, perm, "untap_3")

	if n, _ := inspiredFires(*captured); n != 3 {
		t.Fatalf("expected 3 inspired triggers across 3 transitions, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// FireInspiredTriggers — defensive surface tests
// ---------------------------------------------------------------------------

func TestFireInspiredTriggers_NilSafe(t *testing.T) {
	captured, restore := installCapturingTriggerHook(t)
	defer restore()
	FireInspiredTriggers(nil, nil)
	gs := newInspiredGame(t)
	FireInspiredTriggers(gs, nil)
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("nil inputs must not fire triggers, got %d", n)
	}
}

func TestFireInspiredTriggers_NoKeyword_NoFire(t *testing.T) {
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()
	perm := putBattlefield(gs, 0, newPlainCreature("Hill Giant", 0, 3, 3), false)
	FireInspiredTriggers(gs, perm)
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("non-inspired creature must not fire trigger, got %d", n)
	}
}

func TestFireInspiredTriggers_StillTapped_NoFire(t *testing.T) {
	// Defense-in-depth: even if a caller mistakenly invokes
	// FireInspiredTriggers before flipping Tapped, the transition
	// guard suppresses the event.
	gs := newInspiredGame(t)
	captured, restore := installCapturingTriggerHook(t)
	defer restore()
	perm := putBattlefield(gs, 0, newInspiredCreature("Siren of the Silent Song", 0, 2, 2), true)
	FireInspiredTriggers(gs, perm)
	if n, _ := inspiredFires(*captured); n != 0 {
		t.Fatalf("FireInspiredTriggers on still-tapped permanent must no-op, got %d fires", n)
	}
}
