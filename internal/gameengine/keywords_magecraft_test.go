package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Magecraft tests — ability word, no CR entry per §207.2c
// (was CR §702.137, which is Spectacle; corrected r96)
// ---------------------------------------------------------------------------

func newMagecraftGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(2137))
	return NewGameState(2, rng, nil)
}

// newMagecraftPermCard mints a battlefield-ready creature with the
// magecraft keyword.
func newMagecraftPermCard(name string, owner int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "magecraft"},
			},
		},
	}
}

func newVanillaCreatureCard(name string, owner int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST: &gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{},
		},
	}
}

func newMagecraftInstant(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		CMC:   1,
		AST:   &gameast.CardAST{Name: name, Abilities: []gameast.Ability{}},
	}
}

func newMagecraftSorcery(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		CMC:   3,
		AST:   &gameast.CardAST{Name: name, Abilities: []gameast.Ability{}},
	}
}

func newCreatureSpellCard(name string, owner int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST:           &gameast.CardAST{Name: name, Abilities: []gameast.Ability{}},
	}
}

// putMagecraftPermOnBattlefield places a magecraft-bearing perm on the
// given seat's battlefield.
func putMagecraftPermOnBattlefield(gs *GameState, seat int, card *Card) *Permanent {
	p := &Permanent{Card: card, Controller: seat, Owner: seat}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func countMagecraftEvents(gs *GameState) int {
	n := 0
	for _, e := range gs.EventLog {
		if e.Kind == "magecraft_trigger" {
			n++
		}
	}
	return n
}

// installMagecraftTriggerCapture swaps TriggerHook with a capture stub.
type magecraftTriggerCapture struct {
	source     *Permanent
	spell      *Card
	isCopy     bool
	controller int
}

func installMagecraftTriggerCapture(t *testing.T) (*[]magecraftTriggerCapture, func()) {
	t.Helper()
	prev := TriggerHook
	captured := []magecraftTriggerCapture{}
	TriggerHook = func(gs *GameState, event string, ctx map[string]interface{}) {
		if event != "magecraft" || ctx == nil {
			return
		}
		c := magecraftTriggerCapture{}
		c.source, _ = ctx["source"].(*Permanent)
		c.spell, _ = ctx["spell"].(*Card)
		c.isCopy, _ = ctx["is_copy"].(bool)
		c.controller, _ = ctx["controller"].(int)
		captured = append(captured, c)
	}
	return &captured, func() { TriggerHook = prev }
}

// ---------------------------------------------------------------------------
// HasMagecraft
// ---------------------------------------------------------------------------

func TestHasMagecraft_Positive(t *testing.T) {
	if !HasMagecraft(newMagecraftPermCard("Archmage Emeritus", 0)) {
		t.Fatal("HasMagecraft must be true on a magecraft card")
	}
}

func TestHasMagecraft_Negative(t *testing.T) {
	if HasMagecraft(newVanillaCreatureCard("Plain Bear", 0)) {
		t.Fatal("HasMagecraft must be false on a vanilla creature")
	}
}

func TestHasMagecraft_Nil(t *testing.T) {
	if HasMagecraft(nil) {
		t.Fatal("HasMagecraft(nil) must be false")
	}
}

// ---------------------------------------------------------------------------
// (a) Cast instant triggers magecraft
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_InstantTriggers(t *testing.T) {
	gs := newMagecraftGame(t)
	captured, restore := installMagecraftTriggerCapture(t)
	defer restore()

	source := putMagecraftPermOnBattlefield(gs, 0,
		newMagecraftPermCard("Archmage Emeritus", 0))
	spell := newMagecraftInstant("Lightning Bolt", 0)

	FireMagecraftTriggers(gs, 0, spell, false)

	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count = %d, want 1", got)
	}
	if len(*captured) != 1 {
		t.Fatalf("captured magecraft fan-out count = %d, want 1", len(*captured))
	}
	c := (*captured)[0]
	if c.source != source {
		t.Errorf("ctx[source] mismatch: got %v, want Archmage Emeritus perm", c.source)
	}
	if c.spell != spell {
		t.Errorf("ctx[spell] mismatch: got %v, want Lightning Bolt card", c.spell)
	}
	if c.isCopy {
		t.Errorf("ctx[is_copy] = true, want false for a regular cast")
	}
	if c.controller != 0 {
		t.Errorf("ctx[controller] = %d, want 0", c.controller)
	}
}

func TestFireMagecraftTriggers_SorceryAlsoTriggers(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Symmetry Sage", 0))
	spell := newMagecraftSorcery("Strategic Planning", 0)

	FireMagecraftTriggers(gs, 0, spell, false)

	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count = %d, want 1 (sorcery should trigger)", got)
	}
}

// ---------------------------------------------------------------------------
// (b) Cast creature does NOT trigger
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_CreatureSpellDoesNotTrigger(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	creatureSpell := newCreatureSpellCard("Bear Cub", 0)

	FireMagecraftTriggers(gs, 0, creatureSpell, false)

	if got := countMagecraftEvents(gs); got != 0 {
		t.Errorf("magecraft_trigger count = %d, want 0 (creature spell must not trigger)", got)
	}
}

func TestFireMagecraftTriggers_ArtifactSpellDoesNotTrigger(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	artifactSpell := &Card{
		Name:  "Sol Ring",
		Owner: 0,
		Types: []string{"artifact"},
		AST:   &gameast.CardAST{Name: "Sol Ring", Abilities: []gameast.Ability{}},
	}

	FireMagecraftTriggers(gs, 0, artifactSpell, false)
	if got := countMagecraftEvents(gs); got != 0 {
		t.Errorf("magecraft_trigger count = %d, want 0 (artifact must not trigger)", got)
	}
}

// ---------------------------------------------------------------------------
// (c) Copy of an instant also triggers (per §702.137a)
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_CopyOfInstantTriggers(t *testing.T) {
	gs := newMagecraftGame(t)
	captured, restore := installMagecraftTriggerCapture(t)
	defer restore()

	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	spell := newMagecraftInstant("Lightning Bolt", 0)

	FireMagecraftTriggers(gs, 0, spell, true)

	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count on copy = %d, want 1", got)
	}
	if len(*captured) != 1 {
		t.Fatalf("captured fan-out count = %d, want 1", len(*captured))
	}
	if !(*captured)[0].isCopy {
		t.Error("ctx[is_copy] must be true when FireMagecraftTriggers is called with isCopy=true")
	}
	// The event-log Details should carry is_copy too.
	for _, e := range gs.EventLog {
		if e.Kind != "magecraft_trigger" {
			continue
		}
		got, _ := e.Details["is_copy"].(bool)
		if !got {
			t.Error("magecraft_trigger event Details[is_copy] should be true")
		}
	}
}

// ---------------------------------------------------------------------------
// (d) Opponent's spell does NOT trigger
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_OpponentCastDoesNotTriggerMine(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	// My magecraft perm on seat 0.
	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	// Opponent's spell — fire with casterSeat=1.
	spell := newMagecraftInstant("Counterspell", 1)
	FireMagecraftTriggers(gs, 1, spell, false)

	// Opponent has no magecraft perms, so no triggers fire — and crucially,
	// my (seat 0's) Archmage Emeritus does NOT fire because the cast wasn't
	// by me.
	if got := countMagecraftEvents(gs); got != 0 {
		t.Errorf("magecraft_trigger count = %d, want 0 (opponent cast must not trigger my magecraft)", got)
	}
}

func TestFireMagecraftTriggers_OnlyOwnPermsScanned(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	// Both seats have a magecraft perm. Seat 0 casts an instant.
	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("My Mage", 0))
	putMagecraftPermOnBattlefield(gs, 1, newMagecraftPermCard("Opponent's Mage", 1))

	spell := newMagecraftInstant("Lightning Bolt", 0)
	FireMagecraftTriggers(gs, 0, spell, false)

	// Only my mage triggers; opponent's mage does not.
	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count = %d, want 1 (only caster's own magecraft perms fire)", got)
	}
	for _, e := range gs.EventLog {
		if e.Kind != "magecraft_trigger" {
			continue
		}
		if e.Source == "Opponent's Mage" {
			t.Error("opponent's magecraft perm fired on my cast — should not")
		}
	}
}

// ---------------------------------------------------------------------------
// (e) Multiple magecraft perms each fire
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_MultiplePermsEachFire(t *testing.T) {
	gs := newMagecraftGame(t)
	captured, restore := installMagecraftTriggerCapture(t)
	defer restore()

	p1 := putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	p2 := putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Symmetry Sage", 0))
	p3 := putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Ardent Dustspeaker", 0))

	spell := newMagecraftInstant("Strategic Planning", 0)
	FireMagecraftTriggers(gs, 0, spell, false)

	if got := countMagecraftEvents(gs); got != 3 {
		t.Errorf("magecraft_trigger count = %d, want 3 (one per magecraft perm)", got)
	}
	if len(*captured) != 3 {
		t.Errorf("fan-out count = %d, want 3", len(*captured))
	}
	// Each capture should reference a distinct source.
	saw := map[*Permanent]bool{}
	for _, c := range *captured {
		saw[c.source] = true
	}
	for _, p := range []*Permanent{p1, p2, p3} {
		if !saw[p] {
			t.Errorf("expected fan-out to fire for %q", p.Card.DisplayName())
		}
	}
}

// ---------------------------------------------------------------------------
// Granted-magecraft (kw:magecraft flag) is detected
// ---------------------------------------------------------------------------

func TestPermanentHasMagecraft_FromFlag(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	// Vanilla creature with kw:magecraft flag — same fan-out should fire.
	p := &Permanent{
		Card:       newVanillaCreatureCard("Granted Mage", 0),
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{"kw:magecraft": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	FireMagecraftTriggers(gs, 0, newMagecraftInstant("Bolt", 0), false)
	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count = %d, want 1 (flag-granted magecraft)", got)
	}
}

func TestPermanentHasMagecraft_FromGrantedAbility(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	p := &Permanent{
		Card:             newVanillaCreatureCard("Granted Mage", 0),
		Controller:       0,
		Owner:            0,
		GrantedAbilities: []string{"magecraft"},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)

	FireMagecraftTriggers(gs, 0, newMagecraftInstant("Bolt", 0), false)
	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("magecraft_trigger count = %d, want 1 (GrantedAbilities magecraft)", got)
	}
}

// ---------------------------------------------------------------------------
// Nil-safety
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_NilSafe(t *testing.T) {
	FireMagecraftTriggers(nil, 0, nil, false)
	gs := newMagecraftGame(t)
	FireMagecraftTriggers(gs, 0, nil, false)
	FireMagecraftTriggers(gs, 99, newMagecraftInstant("Bolt", 0), false)
	FireMagecraftTriggers(gs, -1, newMagecraftInstant("Bolt", 0), false)
	if countMagecraftEvents(gs) != 0 {
		t.Errorf("expected 0 events on nil/invalid inputs, got %d", countMagecraftEvents(gs))
	}
}

// ---------------------------------------------------------------------------
// End-to-end: real cast through stack.go's fireCastTriggersFromZone
// fires magecraft (isCopy=false)
// ---------------------------------------------------------------------------

func TestFireMagecraftTriggers_E2E_RealCastWiresThrough(t *testing.T) {
	gs := newMagecraftGame(t)
	_, restore := installMagecraftTriggerCapture(t)
	defer restore()

	putMagecraftPermOnBattlefield(gs, 0, newMagecraftPermCard("Archmage Emeritus", 0))
	spell := newMagecraftInstant("Lightning Bolt", 0)

	// fireCastTriggers is the package-internal cast hook; calling it
	// directly tests the wiring without needing to push/resolve a real
	// stack item end-to-end.
	fireCastTriggers(gs, 0, spell)

	if got := countMagecraftEvents(gs); got != 1 {
		t.Errorf("E2E cast magecraft count = %d, want 1", got)
	}
}
