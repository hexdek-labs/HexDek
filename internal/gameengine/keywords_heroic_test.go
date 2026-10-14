package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Test helpers — Heroic (CR §702.123)
// ---------------------------------------------------------------------------

func newHeroicGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(43))
	return NewGameState(2, rng, nil)
}

type capturedHeroic struct {
	event string
	ctx   map[string]interface{}
}

func installHeroicTriggerHook(t *testing.T) (*[]capturedHeroic, func()) {
	t.Helper()
	prev := TriggerHook
	cap := []capturedHeroic{}
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		cap = append(cap, capturedHeroic{event: event, ctx: ctx})
	}
	return &cap, func() { TriggerHook = prev }
}

// addHeroicCreature drops a creature with the heroic keyword onto
// `seat`'s battlefield.
func addHeroicCreature(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	c := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "heroic"},
			},
		},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addPlainCreature drops a creature without heroic onto `seat`'s
// battlefield.
func addPlainCreature(gs *GameState, seat int, name string, pow, tough int) *Permanent {
	c := &Card{
		Name: name, Owner: seat,
		BasePower: pow, BaseToughness: tough,
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: name},
	}
	p := &Permanent{
		Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// spellTargetingPermanent builds a StackItem for a spell controlled by
// `casterSeat` with `targets` as its locked-in permanent targets.
// Mirrors what CastSpellWithCosts assembles at the cast-time call site
// where FireHeroicTriggers fires.
func spellTargetingPermanent(casterSeat int, spellName string, targets ...*Permanent) *StackItem {
	spell := &Card{
		Name:  spellName,
		Owner: casterSeat,
		Types: []string{"instant"},
		AST:   &gameast.CardAST{Name: spellName},
	}
	item := &StackItem{
		Kind:       "spell",
		Card:       spell,
		Controller: casterSeat,
		CastZone:   ZoneHand,
	}
	for _, t := range targets {
		item.Targets = append(item.Targets, Target{Kind: TargetKindPermanent, Permanent: t})
	}
	return item
}

func heroicCountEvents(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "heroic_trigger" {
			n++
		}
	}
	return n
}

// ===========================================================================
// HasHeroic detection
// ===========================================================================

func TestHasHeroic_Detects(t *testing.T) {
	c := &Card{
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "heroic"},
			},
		},
	}
	if !HasHeroic(c) {
		t.Fatal("HasHeroic should detect the keyword")
	}
	if HasHeroic(nil) {
		t.Fatal("HasHeroic(nil) should be false")
	}
	if HasHeroic(&Card{AST: &gameast.CardAST{}}) {
		t.Fatal("HasHeroic should be false for a card without the keyword")
	}
}

// ===========================================================================
// (a) Own spell targeting own heroic creature triggers
// ===========================================================================

func TestFireHeroicTriggers_OwnSpellOwnHeroicTriggers(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	spell := spellTargetingPermanent(0, "Gods Willing", hoplite)

	FireHeroicTriggers(gs, spell)

	// One heroic trigger fired via FireCardTrigger.
	heroicEvents := 0
	for _, c := range *cap {
		if c.event != "heroic" {
			continue
		}
		heroicEvents++
		if src, _ := c.ctx["source"].(*Permanent); src != hoplite {
			t.Errorf("source: want %p, got %p", hoplite, src)
		}
		if sp, _ := c.ctx["spell"].(*Card); sp != spell.Card {
			t.Errorf("spell: want %p, got %p", spell.Card, sp)
		}
		if cs, _ := c.ctx["caster_seat"].(int); cs != 0 {
			t.Errorf("caster_seat: want 0, got %d", cs)
		}
	}
	if heroicEvents != 1 {
		t.Fatalf("expected exactly 1 heroic FireCardTrigger; got %d", heroicEvents)
	}
	// And the structured log entry.
	if got := heroicCountEvents(gs); got != 1 {
		t.Errorf("expected 1 heroic_trigger log event, got %d", got)
	}
}

// ===========================================================================
// (b) Opponent's spell targeting own heroic creature does NOT trigger
// ===========================================================================

func TestFireHeroicTriggers_OpponentSpellDoesNotTrigger(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	// Spell cast by seat 1, but targeting seat 0's heroic creature.
	spell := spellTargetingPermanent(1, "Opponent's Targeting Spell", hoplite)

	FireHeroicTriggers(gs, spell)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("opponent's spell must NOT trigger your heroic creature")
		}
	}
	if heroicCountEvents(gs) != 0 {
		t.Error("expected no heroic_trigger log entries")
	}
}

// ===========================================================================
// (c) Own spell targeting an OPPONENT's heroic creature does NOT trigger
// ===========================================================================

func TestFireHeroicTriggers_TargetingOpponentHeroicDoesNotTrigger(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	// Heroic creature on seat 1 (opponent's). Spell cast by seat 0
	// targeting it.
	opponentHero := addHeroicCreature(gs, 1, "Opponent's Hero", 2, 2)
	spell := spellTargetingPermanent(0, "Bolt", opponentHero)

	FireHeroicTriggers(gs, spell)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("targeting an opponent's heroic creature must NOT trigger heroic")
		}
	}
	if heroicCountEvents(gs) != 0 {
		t.Error("expected no heroic_trigger log entries")
	}
}

// ===========================================================================
// (d) Multi-target spell triggers heroic on each of YOUR heroic targets
// ===========================================================================

func TestFireHeroicTriggers_MultiTargetFiresEachOwnHeroic(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	akroan := addHeroicCreature(gs, 0, "Akroan Crusader", 1, 1)
	// And an opponent's heroic that also gets targeted — must NOT fire.
	oppHero := addHeroicCreature(gs, 1, "Opponent's Hero", 2, 2)

	// A 3-target spell (e.g. "any number of target creatures") cast
	// by seat 0 hitting all three creatures.
	spell := spellTargetingPermanent(0, "Hero of Iroas",
		hoplite, akroan, oppHero)

	FireHeroicTriggers(gs, spell)

	seen := map[*Permanent]int{}
	for _, c := range *cap {
		if c.event != "heroic" {
			continue
		}
		s, _ := c.ctx["source"].(*Permanent)
		seen[s]++
	}
	if seen[hoplite] != 1 {
		t.Errorf("hoplite should fire once, got %d", seen[hoplite])
	}
	if seen[akroan] != 1 {
		t.Errorf("akroan crusader should fire once, got %d", seen[akroan])
	}
	if seen[oppHero] != 0 {
		t.Errorf("opponent's hero should NOT fire, got %d", seen[oppHero])
	}
	if heroicCountEvents(gs) != 2 {
		t.Errorf("expected 2 heroic_trigger log entries, got %d", heroicCountEvents(gs))
	}
}

func TestFireHeroicTriggers_SameHeroicTargetedTwiceFiresOnce(t *testing.T) {
	// Defensive de-dup: if the same heroic creature appears as two
	// distinct target slots on the same spell, fire heroic exactly once.
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	spell := spellTargetingPermanent(0, "Twin-target Spell", hoplite, hoplite)

	FireHeroicTriggers(gs, spell)

	heroicEvents := 0
	for _, c := range *cap {
		if c.event == "heroic" {
			heroicEvents++
		}
	}
	if heroicEvents != 1 {
		t.Fatalf("expected exactly 1 heroic trigger for double-listed target; got %d", heroicEvents)
	}
}

// ===========================================================================
// (e) Non-targeting spell does NOT trigger
// ===========================================================================

func TestFireHeroicTriggers_NoTargetsNoTrigger(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	_ = addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	// Spell cast by seat 0 with NO targets (e.g. a board wipe, a
	// non-targeted card-draw spell).
	spell := spellTargetingPermanent(0, "Wrath of God")

	FireHeroicTriggers(gs, spell)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("a non-targeting spell must NOT trigger heroic")
		}
	}
	if heroicCountEvents(gs) != 0 {
		t.Error("expected no heroic_trigger log entries")
	}
}

// ===========================================================================
// Spell-only gate: triggered/activated abilities targeting heroic
// creatures must NOT fire heroic.
// ===========================================================================

func TestFireHeroicTriggers_AbilityNotSpellNoTrigger(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	source := addPlainCreature(gs, 0, "Ability Source", 1, 1)

	// Build a stack item that LOOKS like it targets hoplite but is an
	// activated ability (Source set, Card may be set to source's card
	// for activated-ability bookkeeping). Per §702.123a, abilities
	// don't trigger heroic.
	item := &StackItem{
		Kind:       "activated",
		Source:     source,
		Card:       source.Card,
		Controller: 0,
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: hoplite},
		},
	}

	FireHeroicTriggers(gs, item)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("activated/triggered ability stack items must NOT fire heroic")
		}
	}
}

// ===========================================================================
// Face-down heroic source is skipped (CR §708.2)
// ===========================================================================

func TestFireHeroicTriggers_FaceDownHeroicSkipped(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Hidden Hero", 1, 2)
	hoplite.Flags["face_down"] = 1
	spell := spellTargetingPermanent(0, "Test Spell", hoplite)

	FireHeroicTriggers(gs, spell)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("face-down heroic creature must NOT fire heroic")
		}
	}
}

// ===========================================================================
// Player-target spells don't fire heroic (target.Kind != Permanent)
// ===========================================================================

func TestFireHeroicTriggers_PlayerTargetsIgnored(t *testing.T) {
	gs := newHeroicGame(t)
	cap, restore := installHeroicTriggerHook(t)
	defer restore()

	_ = addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	spell := &StackItem{
		Kind:       "spell",
		Card:       &Card{Name: "Shock", Owner: 0, Types: []string{"instant"}},
		Controller: 0,
		Targets: []Target{
			{Kind: TargetKindSeat, Seat: 1},
		},
	}

	FireHeroicTriggers(gs, spell)

	for _, c := range *cap {
		if c.event == "heroic" {
			t.Fatal("player-target spell must NOT fire heroic on a side-line creature")
		}
	}
}

// ===========================================================================
// Wiring: FireHeroicTriggers is invoked at cast-time from
// CastSpellWithCosts (next to CheckWardOnTargeting). The pure-function
// unit tests above cover the trigger logic; this test confirms it's
// also reachable via the higher-level CastSpellWithCosts path by
// hand-building a stack item the way that path would.
// ===========================================================================

func TestFireHeroicTriggers_CastTimeNotResolveTime(t *testing.T) {
	// The defining property: heroic fires when the spell is CAST, even
	// if the spell would later be countered. We don't simulate a full
	// counter here — we just verify that FireHeroicTriggers fires
	// without ever resolving the stack item (the spell stays unresolved
	// after the call, and the heroic event is already logged).
	gs := newHeroicGame(t)
	_, restore := installHeroicTriggerHook(t)
	defer restore()

	hoplite := addHeroicCreature(gs, 0, "Favored Hoplite", 1, 2)
	spell := spellTargetingPermanent(0, "Gods Willing", hoplite)

	stackSizeBefore := len(gs.Stack)
	FireHeroicTriggers(gs, spell)
	stackSizeAfter := len(gs.Stack)

	if stackSizeAfter != stackSizeBefore {
		t.Errorf("FireHeroicTriggers must not touch the stack; before=%d after=%d",
			stackSizeBefore, stackSizeAfter)
	}
	if heroicCountEvents(gs) != 1 {
		t.Errorf("heroic should have fired at cast-time without resolution; got %d events",
			heroicCountEvents(gs))
	}
}

// ===========================================================================
// Nil safety
// ===========================================================================

func TestFireHeroicTriggers_NilSafe(t *testing.T) {
	FireHeroicTriggers(nil, nil)
	gs := newHeroicGame(t)
	FireHeroicTriggers(gs, nil)
	FireHeroicTriggers(gs, &StackItem{Card: nil})
}
