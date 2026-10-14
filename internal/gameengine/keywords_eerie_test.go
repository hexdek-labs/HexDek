package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Eerie (CR §702.182)
// ---------------------------------------------------------------------------

func newEerieGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(37))
	return NewGameState(2, rng, nil)
}

// installEerieTriggerHook captures FireCardTrigger calls so tests can
// assert which eerie triggers fired and with what context. Mirrors the
// installCapturingTriggerHook pattern in keywords_battalion_test.go but
// kept local to avoid coupling test files.
func installEerieTriggerHook(t *testing.T) (*[]capturedEerie, func()) {
	t.Helper()
	prev := TriggerHook
	cap := []capturedEerie{}
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		cap = append(cap, capturedEerie{event: event, ctx: ctx})
	}
	return &cap, func() { TriggerHook = prev }
}

type capturedEerie struct {
	event string
	ctx   map[string]interface{}
}

// addEnchantmentEerie drops an enchantment carrying the eerie keyword
// onto `seat`'s battlefield. The eerie effect text is irrelevant for
// the unit tests — we only assert the trigger fires.
func addEerieEnchantment(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"enchantment"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "eerie"},
			},
		},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addPlainEnchantmentToBF drops an enchantment WITHOUT the eerie
// keyword onto `seat`'s battlefield (used to verify ETB-driven eerie
// triggers fire only for the *carriers*, and to construct the
// triggering enchantment in non-ETB tests).
func addPlainEnchantmentToBF(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"enchantment"},
		AST: &gameast.CardAST{
			Name: name,
		},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// newPlainEnchantment builds an enchantment Permanent WITHOUT placing
// it on a battlefield — caller uses it as the `enteredPerm` argument
// to OnEnchantmentETB to simulate a fresh ETB.
func newPlainEnchantment(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name: name, Owner: seat,
		Types: []string{"enchantment"},
		AST:   &gameast.CardAST{Name: name},
	}
	return &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
}

// addRoomToBF drops a Room (enchantment with subtype "room") onto
// `seat`'s battlefield. By default neither half is unlocked; the
// caller sets Flags["room_unlocked_a"] / ["room_unlocked_b"] to
// simulate progressive unlock events.
func addRoomToBF(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: seat,
		Types: []string{"enchantment", "room"},
		AST: &gameast.CardAST{
			Name: name,
		},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// ===========================================================================
// HasEerie + IsRoomFullyUnlocked detection
// ===========================================================================

func TestHasEerie_Detects(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "eerie"},
			},
		},
	}
	if !HasEerie(c) {
		t.Fatal("HasEerie should detect the keyword")
	}
	if HasEerie(nil) {
		t.Fatal("HasEerie(nil) should be false")
	}
	if HasEerie(&Card{AST: &gameast.CardAST{}}) {
		t.Fatal("HasEerie should be false for a card without the keyword")
	}
}

func TestIsRoomFullyUnlocked(t *testing.T) {
	gs := newEerieGame(t)
	room := addRoomToBF(gs, 0, "Bottomless Pool / Locker Room")

	if IsRoomFullyUnlocked(room) {
		t.Fatal("a Room with neither half unlocked should not be fully unlocked")
	}
	room.Flags["room_unlocked_a"] = 1
	if IsRoomFullyUnlocked(room) {
		t.Fatal("a Room with only one half unlocked should not be fully unlocked")
	}
	room.Flags["room_unlocked_b"] = 1
	if !IsRoomFullyUnlocked(room) {
		t.Fatal("a Room with both halves unlocked should be fully unlocked")
	}

	// Non-Room with both flags set should NOT count.
	other := addPlainEnchantmentToBF(gs, 0, "Plain Enchant")
	other.Flags["room_unlocked_a"] = 1
	other.Flags["room_unlocked_b"] = 1
	if IsRoomFullyUnlocked(other) {
		t.Fatal("a non-Room with the flags set should not count as a fully-unlocked Room")
	}

	if IsRoomFullyUnlocked(nil) {
		t.Fatal("IsRoomFullyUnlocked(nil) should be false")
	}
}

// ===========================================================================
// (a) Enchantment ETB triggers eerie on the same controller
// ===========================================================================

func TestOnEnchantmentETB_FiresForSameControllerEerie(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	src := addEerieEnchantment(gs, 0, "Unsettling Twins")
	entered := newPlainEnchantment(gs, 0, "Vanilla Aura")

	OnEnchantmentETB(gs, entered)

	eerieEvents := 0
	for _, c := range *captured {
		if c.event == "eerie" {
			eerieEvents++
			if v, _ := c.ctx["eerie_source"].(*Permanent); v != src {
				t.Errorf("eerie_source: want %p, got %p", src, v)
			}
			if v, _ := c.ctx["trigger_perm"].(*Permanent); v != entered {
				t.Errorf("trigger_perm: want %p, got %p", entered, v)
			}
			if v, _ := c.ctx["controller_seat"].(int); v != 0 {
				t.Errorf("controller_seat: want 0, got %d", v)
			}
			if v, _ := c.ctx["cause"].(string); v != "enchantment_etb" {
				t.Errorf("cause: want \"enchantment_etb\", got %q", v)
			}
		}
	}
	if eerieEvents != 1 {
		t.Fatalf("expected exactly 1 eerie trigger, got %d", eerieEvents)
	}

	// Eerie log event too.
	sawLog := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "eerie_trigger" && ev.Seat == 0 {
			if rule, _ := ev.Details["rule"].(string); rule == "702.182a" {
				if cause, _ := ev.Details["cause"].(string); cause == "enchantment_etb" {
					sawLog = true
					break
				}
			}
		}
	}
	if !sawLog {
		t.Error("expected an eerie_trigger event with rule 702.182a, cause enchantment_etb")
	}
}

func TestOnEnchantmentETB_FiresViaFirePermanentETBTriggers(t *testing.T) {
	// End-to-end: when an enchantment ETBs through the regular
	// FirePermanentETBTriggers pipeline, eerie should automatically
	// fire. This confirms the wiring in etb_dispatch.go.
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	_ = addEerieEnchantment(gs, 0, "Unsettling Twins")
	entered := newPlainEnchantment(gs, 0, "Vanilla Aura")
	// Simulate the ETB: add the permanent to the battlefield, then
	// fire the dispatch hook (this is what MoveCard / cast resolution
	// does for permanent spells).
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, entered)
	FirePermanentETBTriggers(gs, entered)

	eerieCount := 0
	for _, c := range *captured {
		if c.event == "eerie" {
			eerieCount++
		}
	}
	if eerieCount != 1 {
		t.Fatalf("expected 1 eerie trigger via FirePermanentETBTriggers, got %d", eerieCount)
	}
}

// ===========================================================================
// (b) Opponent's enchantment ETB does NOT trigger
// ===========================================================================

func TestOnEnchantmentETB_OpponentControlledDoesNotTrigger(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	// Eerie carrier on seat 0; entering enchantment controlled by seat 1.
	_ = addEerieEnchantment(gs, 0, "Unsettling Twins")
	opponentEntered := newPlainEnchantment(gs, 1, "Opponent's Curse")

	OnEnchantmentETB(gs, opponentEntered)

	for _, c := range *captured {
		if c.event == "eerie" {
			t.Fatal("opponent-controlled enchantment ETB must not trigger your eerie")
		}
	}
}

func TestOnEnchantmentETB_NonEnchantmentETBNoop(t *testing.T) {
	// A non-enchantment ETB shouldn't trigger eerie at all. The hook
	// is called liberally from FirePermanentETBTriggers and must
	// early-return cleanly.
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	_ = addEerieEnchantment(gs, 0, "Unsettling Twins")
	// A plain creature, not an enchantment.
	creatureCard := &Card{
		Name: "Bear", Owner: 0,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST:           &gameast.CardAST{Name: "Bear"},
	}
	creature := &Permanent{
		Card: creatureCard, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}

	OnEnchantmentETB(gs, creature)

	for _, c := range *captured {
		if c.event == "eerie" {
			t.Fatal("a creature ETB must not trigger eerie")
		}
	}
}

func TestOnEnchantmentETB_FaceDownEerieSourceSkipped(t *testing.T) {
	// CR §708.2: face-down permanents have no abilities. An eerie
	// carrier turned face-down (manifested, morphed, or disguised in
	// the engine model) must NOT fire.
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	src := addEerieEnchantment(gs, 0, "Hooded Eerie")
	src.Flags["face_down"] = 1
	entered := newPlainEnchantment(gs, 0, "Vanilla Aura")

	OnEnchantmentETB(gs, entered)

	for _, c := range *captured {
		if c.event == "eerie" {
			t.Fatal("face-down eerie source must not fire")
		}
	}
}

// ===========================================================================
// (c) Room fully unlocked triggers eerie (mocked unlock state)
// ===========================================================================

func TestOnRoomFullyUnlocked_FiresForSameControllerEerie(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	src := addEerieEnchantment(gs, 0, "Unsettling Twins")
	room := addRoomToBF(gs, 0, "Bottomless Pool / Locker Room")
	// Mock the unlock: both halves set.
	room.Flags["room_unlocked_a"] = 1
	room.Flags["room_unlocked_b"] = 1

	OnRoomFullyUnlocked(gs, room)

	eerieEvents := 0
	for _, c := range *captured {
		if c.event != "eerie" {
			continue
		}
		eerieEvents++
		if v, _ := c.ctx["eerie_source"].(*Permanent); v != src {
			t.Errorf("eerie_source: want %p, got %p", src, v)
		}
		if v, _ := c.ctx["trigger_perm"].(*Permanent); v != room {
			t.Errorf("trigger_perm: want room %p, got %p", room, v)
		}
		if v, _ := c.ctx["cause"].(string); v != "room_unlocked" {
			t.Errorf("cause: want \"room_unlocked\", got %q", v)
		}
	}
	if eerieEvents != 1 {
		t.Fatalf("expected exactly 1 eerie trigger from room unlock, got %d", eerieEvents)
	}

	sawLog := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "eerie_trigger" {
			if cause, _ := ev.Details["cause"].(string); cause == "room_unlocked" {
				sawLog = true
				break
			}
		}
	}
	if !sawLog {
		t.Error("expected an eerie_trigger event with cause=room_unlocked")
	}
}

func TestOnRoomFullyUnlocked_NoopWhenOnlyOneHalfUnlocked(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	_ = addEerieEnchantment(gs, 0, "Unsettling Twins")
	room := addRoomToBF(gs, 0, "Bottomless Pool / Locker Room")
	room.Flags["room_unlocked_a"] = 1 // only one half

	OnRoomFullyUnlocked(gs, room)

	for _, c := range *captured {
		if c.event == "eerie" {
			t.Fatal("OnRoomFullyUnlocked must not fire when only one half is unlocked")
		}
	}
}

func TestOnRoomFullyUnlocked_OpponentRoomDoesNotTriggerYourEerie(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	_ = addEerieEnchantment(gs, 0, "Unsettling Twins")
	oppRoom := addRoomToBF(gs, 1, "Opponent's Twisted Hall")
	oppRoom.Flags["room_unlocked_a"] = 1
	oppRoom.Flags["room_unlocked_b"] = 1

	OnRoomFullyUnlocked(gs, oppRoom)

	for _, c := range *captured {
		if c.event == "eerie" {
			t.Fatal("an opponent's Room unlock must not trigger your eerie")
		}
	}
}

// ===========================================================================
// (d) Multiple eerie permanents all fire
// ===========================================================================

func TestOnEnchantmentETB_MultipleEerieAllFire(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	src1 := addEerieEnchantment(gs, 0, "Unsettling Twins")
	src2 := addEerieEnchantment(gs, 0, "Haunted Library")
	src3 := addEerieEnchantment(gs, 0, "Cosmic Window")
	entered := newPlainEnchantment(gs, 0, "Vanilla Aura")

	OnEnchantmentETB(gs, entered)

	seen := map[*Permanent]int{}
	for _, c := range *captured {
		if c.event != "eerie" {
			continue
		}
		s, _ := c.ctx["eerie_source"].(*Permanent)
		seen[s]++
	}
	if seen[src1] != 1 || seen[src2] != 1 || seen[src3] != 1 {
		t.Fatalf("expected exactly one trigger per eerie source; got src1=%d src2=%d src3=%d",
			seen[src1], seen[src2], seen[src3])
	}
	totalEerie := 0
	for _, c := range *captured {
		if c.event == "eerie" {
			totalEerie++
		}
	}
	if totalEerie != 3 {
		t.Fatalf("expected 3 eerie triggers total, got %d", totalEerie)
	}
}

func TestOnRoomFullyUnlocked_MultipleEerieAllFire(t *testing.T) {
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	src1 := addEerieEnchantment(gs, 0, "Unsettling Twins")
	src2 := addEerieEnchantment(gs, 0, "Haunted Library")
	room := addRoomToBF(gs, 0, "Bottomless Pool / Locker Room")
	room.Flags["room_unlocked_a"] = 1
	room.Flags["room_unlocked_b"] = 1

	OnRoomFullyUnlocked(gs, room)

	seen := map[*Permanent]int{}
	for _, c := range *captured {
		if c.event != "eerie" {
			continue
		}
		s, _ := c.ctx["eerie_source"].(*Permanent)
		seen[s]++
	}
	if seen[src1] != 1 || seen[src2] != 1 {
		t.Fatalf("expected one trigger per eerie source; got src1=%d src2=%d", seen[src1], seen[src2])
	}
}

// ===========================================================================
// Self-triggering: an eerie enchantment ETB'ing itself counts
// ===========================================================================

func TestOnEnchantmentETB_EerieEnchantmentTriggersItselfOnEntry(t *testing.T) {
	// An enchantment that prints "Eerie — ..." should trigger its own
	// eerie on its own ETB (since it IS an enchantment-you-control
	// entering, and it IS a controller-side eerie carrier the moment
	// it's on the battlefield).
	gs := newEerieGame(t)
	captured, restore := installEerieTriggerHook(t)
	defer restore()

	// The eerie carrier IS the entering permanent.
	src := addEerieEnchantment(gs, 0, "Self-Eerie Enchantment")

	OnEnchantmentETB(gs, src)

	count := 0
	for _, c := range *captured {
		if c.event == "eerie" {
			count++
			if s, _ := c.ctx["eerie_source"].(*Permanent); s != src {
				t.Errorf("eerie_source: want %p, got %p", src, s)
			}
		}
	}
	if count != 1 {
		t.Fatalf("a self-eerie enchantment should trigger itself exactly once on ETB; got %d", count)
	}
}

// ===========================================================================
// Nil safety
// ===========================================================================

func TestOnEnchantmentETB_NilSafe(t *testing.T) {
	OnEnchantmentETB(nil, nil)
	gs := newEerieGame(t)
	OnEnchantmentETB(gs, nil)
}

func TestOnRoomFullyUnlocked_NilSafe(t *testing.T) {
	OnRoomFullyUnlocked(nil, nil)
	gs := newEerieGame(t)
	OnRoomFullyUnlocked(gs, nil)
}
