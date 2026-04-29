package gameengine

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// Fixture helpers — no external corpus required for Phase 3 verification.
// -----------------------------------------------------------------------------

// newFixtureGame spins up a deterministic two-seat game with empty libraries.
// Tests layer concrete cards onto Seats as needed.
func newFixtureGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	return gs
}

// addBattlefield puts a card on seat's battlefield as a Permanent and returns
// the new Permanent. Used by handlers that need a source or a target.
func addBattlefield(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string{}, types...),
	}
	p := &Permanent{
		Card:          card,
		Controller:    seat,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
		SummoningSick: false,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// addLibrary appends synthetic cards to a seat's library in deck order
// (top first). Useful for Draw / Mill / Tutor tests.
func addLibrary(gs *GameState, seat int, names ...string) {
	for _, n := range names {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &Card{
			Name:  n,
			Owner: seat,
			Types: []string{"creature"},
		})
	}
}

// addHand appends synthetic cards to a seat's hand.
func addHand(gs *GameState, seat int, names ...string) {
	for _, n := range names {
		gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, &Card{
			Name:  n,
			Owner: seat,
			Types: []string{"creature"},
		})
	}
}

// countEvents returns the number of events with the given Kind.
func countEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// lastEventOfKind returns the last event with the given Kind (or zero value).
func lastEventOfKind(gs *GameState, kind string) Event {
	for i := len(gs.EventLog) - 1; i >= 0; i-- {
		if gs.EventLog[i].Kind == kind {
			return gs.EventLog[i]
		}
	}
	return Event{}
}

// -----------------------------------------------------------------------------
// Tests — leaf effects
// -----------------------------------------------------------------------------

func TestDamageToPlayer(t *testing.T) {
	gs := newFixtureGame(t)
	// src: seat 0's "Lightning Bolt" instance on the battlefield (proxy).
	src := addBattlefield(gs, 0, "Lightning Bolt", 0, 0, "instant")

	e := &gameast.Damage{
		Amount: *gameast.NumInt(3),
		Target: gameast.TargetPlayer(),
	}
	e.Target.Base = "opponent"
	e.Target.Targeted = true

	ResolveEffect(gs, src, e)

	if gs.Seats[1].Life != 17 {
		t.Errorf("expected seat 1 life 17, got %d", gs.Seats[1].Life)
	}
	if countEvents(gs, "damage") != 1 {
		t.Errorf("expected 1 damage event, got %d", countEvents(gs, "damage"))
	}
	ev := lastEventOfKind(gs, "damage")
	if ev.Amount != 3 || ev.Target != 1 {
		t.Errorf("damage event wrong: %+v", ev)
	}
}

func TestDamageAnyTarget(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Bolter", 0, 0, "instant")

	// "any_target" filter — resolver picks opponent seat in MVP.
	e := &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.TargetAny(),
	}
	ResolveEffect(gs, src, e)
	if gs.Seats[1].Life != 18 {
		t.Errorf("expected seat 1 life 18, got %d", gs.Seats[1].Life)
	}
}

func TestDamageToCreature(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Volley", 0, 0, "instant")
	grizzly := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	e := &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)
	if grizzly.MarkedDamage != 2 {
		t.Errorf("expected grizzly marked damage 2, got %d", grizzly.MarkedDamage)
	}
}

func TestDrawCards(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Ancestral Recall", 0, 0, "instant")
	addLibrary(gs, 0, "A", "B", "C", "D")

	e := &gameast.Draw{
		Count:  *gameast.NumInt(3),
		Target: gameast.Filter{Base: "controller"},
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected hand size 3, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected library size 1, got %d", len(gs.Seats[0].Library))
	}
	if ev := lastEventOfKind(gs, "draw"); ev.Amount != 3 {
		t.Errorf("expected draw event Amount=3, got %+v", ev)
	}
}

func TestDrawEmptyLibrary(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Brainstorm", 0, 0, "instant")
	// No library cards — attempted draw flags should be set.

	e := &gameast.Draw{
		Count:  *gameast.NumInt(1),
		Target: gameast.Filter{Base: "controller"},
	}
	ResolveEffect(gs, src, e)

	if !gs.Seats[0].AttemptedEmptyDraw {
		t.Errorf("expected AttemptedEmptyDraw=true on empty library")
	}
}

func TestDiscardFromHand(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Duress", 0, 0, "instant")
	addHand(gs, 1, "Card A", "Card B")

	e := &gameast.Discard{
		Count:  *gameast.NumInt(1),
		Target: gameast.TargetOpponent(),
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("expected hand size 1 after discard, got %d", len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[1].Graveyard) != 1 {
		t.Errorf("expected graveyard size 1, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestMillCards(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Mind Funeral", 0, 0, "instant")
	addLibrary(gs, 1, "X", "Y", "Z", "W")

	e := &gameast.Mill{
		Count:  *gameast.NumInt(3),
		Target: gameast.TargetOpponent(),
	}
	ResolveEffect(gs, src, e)
	if len(gs.Seats[1].Graveyard) != 3 {
		t.Errorf("expected 3 milled cards, got %d", len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[1].Library) != 1 {
		t.Errorf("expected 1 card left in library, got %d", len(gs.Seats[1].Library))
	}
}

func TestGainLife(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Healing Salve", 0, 0, "instant")
	e := &gameast.GainLife{
		Amount: *gameast.NumInt(5),
		Target: gameast.Filter{Base: "controller"},
	}
	ResolveEffect(gs, src, e)
	if gs.Seats[0].Life != 25 {
		t.Errorf("expected life 25, got %d", gs.Seats[0].Life)
	}
}

func TestLoseLife(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Exsanguinate", 0, 0, "instant")
	e := &gameast.LoseLife{
		Amount: *gameast.NumInt(4),
		Target: gameast.TargetOpponent(),
	}
	ResolveEffect(gs, src, e)
	if gs.Seats[1].Life != 16 {
		t.Errorf("expected opponent life 16, got %d", gs.Seats[1].Life)
	}
}

func TestDestroyCreature(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Doom Blade", 0, 0, "instant")
	_ = addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	e := &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("expected opponent battlefield empty, got %d", len(gs.Seats[1].Battlefield))
	}
	if len(gs.Seats[1].Graveyard) != 1 {
		t.Errorf("expected 1 card in opponent graveyard, got %d", len(gs.Seats[1].Graveyard))
	}
}

func TestExileCreature(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Swords to Plowshares", 0, 0, "instant")
	_ = addBattlefield(gs, 1, "Primeval Titan", 6, 6, "creature")
	e := &gameast.Exile{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)
	if len(gs.Seats[1].Exile) != 1 {
		t.Errorf("expected 1 exiled card, got %d", len(gs.Seats[1].Exile))
	}
}

func TestBounce(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Unsummon", 0, 0, "instant")
	_ = addBattlefield(gs, 1, "Llanowar Elves", 1, 1, "creature")
	e := &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
		To:     "owners_hand",
	}
	ResolveEffect(gs, src, e)
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("expected 1 in hand, got %d", len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("expected no creatures on battlefield, got %d", len(gs.Seats[1].Battlefield))
	}
}

func TestTapAndUntap(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Twiddle", 0, 0, "instant")
	target := addBattlefield(gs, 1, "Mox Pearl", 0, 0, "artifact")

	// Tap
	ResolveEffect(gs, src, &gameast.TapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})
	if !target.Tapped {
		t.Errorf("expected target tapped")
	}
	// Untap
	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "permanent", OpponentControls: true, Targeted: true},
	})
	if target.Tapped {
		t.Errorf("expected target untapped")
	}
}

func TestCreateToken(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Raise the Alarm", 0, 0, "instant")
	pt := [2]int{1, 1}
	e := &gameast.CreateToken{
		Count: *gameast.NumInt(2),
		PT:    &pt,
		Types: []string{"creature", "soldier"},
	}
	ResolveEffect(gs, src, e)

	// Expect 2 new permanents (plus the source = 3 total on seat 0).
	if len(gs.Seats[0].Battlefield) != 3 {
		t.Errorf("expected 3 permanents on seat 0, got %d", len(gs.Seats[0].Battlefield))
	}
	// Token power/toughness
	tok := gs.Seats[0].Battlefield[1]
	if tok.Power() != 1 || tok.Toughness() != 1 {
		t.Errorf("expected 1/1 token, got %d/%d", tok.Power(), tok.Toughness())
	}
	if !tok.IsCreature() {
		t.Errorf("expected token IsCreature")
	}
}

func TestBuff(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Giant Growth", 0, 0, "instant")
	creature := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	e := &gameast.Buff{
		Power:     3,
		Toughness: 3,
		Target:    gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		Duration:  "until_end_of_turn",
	}
	ResolveEffect(gs, src, e)
	if creature.Power() != 5 || creature.Toughness() != 5 {
		t.Errorf("expected 5/5 after buff, got %d/%d", creature.Power(), creature.Toughness())
	}
}

func TestCounterModPutP1P1(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Oran-Rief", 0, 0, "land")
	creature := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")

	e := &gameast.CounterMod{
		Op:          "put",
		Count:       *gameast.NumInt(2),
		CounterKind: "+1/+1",
		Target:      gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)
	if creature.Power() != 3 || creature.Toughness() != 3 {
		t.Errorf("expected 3/3 after counters, got %d/%d", creature.Power(), creature.Toughness())
	}
}

func TestAddMana(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")

	e := &gameast.AddMana{
		Pool: []gameast.ManaSymbol{
			{Raw: "{C}"},
			{Raw: "{C}"},
		},
	}
	ResolveEffect(gs, src, e)
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana pool 2, got %d", gs.Seats[0].ManaPool)
	}
}

func TestShuffleDeterministic(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	before := make([]string, 0, 5)
	for _, c := range gs.Seats[0].Library {
		before = append(before, c.Name)
	}
	src := addBattlefield(gs, 0, "Shuffler", 0, 0, "instant")

	e := &gameast.Shuffle{Target: gameast.Filter{Base: "you"}}
	ResolveEffect(gs, src, e)

	// With seed 42, the order WILL change (all 5 cards still present, but
	// not in original order for a run of 5). At minimum, verify no cards
	// were lost.
	if len(gs.Seats[0].Library) != len(before) {
		t.Errorf("expected library size %d after shuffle, got %d", len(before), len(gs.Seats[0].Library))
	}
}

func TestTutorToHand(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Demonic Tutor", 0, 0, "sorcery")
	// Seed library with a mix.
	addLibrary(gs, 0, "Shock", "Grizzly Bears", "Island")

	// Tutor for a creature.
	e := &gameast.Tutor{
		Query: gameast.Filter{Base: "creature", Targeted: false},
		Count: *gameast.NumInt(1),
	}
	ResolveEffect(gs, src, e)

	// Our addLibrary adds type="creature" to every card, so the first match
	// is "Shock" — still a creature in our fixture.
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected 1 card tutored to hand, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Errorf("expected library size 2, got %d", len(gs.Seats[0].Library))
	}
}

func TestReanimate(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Reanimate", 0, 0, "sorcery")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &Card{
		Name:          "Griselbrand",
		Owner:         0,
		BasePower:     7,
		BaseToughness: 7,
		Types:         []string{"creature"},
	})

	e := &gameast.Reanimate{
		Query:       gameast.Filter{Base: "creature"},
		FromZone:    "your_graveyard",
		Destination: "battlefield",
	}
	ResolveEffect(gs, src, e)

	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected graveyard empty, got %d", len(gs.Seats[0].Graveyard))
	}
	// src + reanimated = 2 permanents
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Errorf("expected 2 permanents, got %d", len(gs.Seats[0].Battlefield))
	}
}

func TestCounterSpellMarksStack(t *testing.T) {
	gs := newFixtureGame(t)
	// Put a fake spell on the stack.
	gs.Stack = append(gs.Stack, &StackItem{
		ID:         1,
		Controller: 1,
	})
	src := addBattlefield(gs, 0, "Counterspell", 0, 0, "instant")
	e := &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell", Targeted: true},
	}
	ResolveEffect(gs, src, e)
	if !gs.Stack[0].Countered {
		t.Errorf("expected stack item 0 to be countered")
	}
}

func TestFight(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Prey Upon", 0, 0, "sorcery")
	a := addBattlefield(gs, 0, "Rhino", 4, 4, "creature")
	b := addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	e := &gameast.Fight{
		A: gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		B: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)

	if a.MarkedDamage != 2 {
		t.Errorf("expected rhino marked damage 2, got %d", a.MarkedDamage)
	}
	if b.MarkedDamage != 4 {
		t.Errorf("expected bear marked damage 4, got %d", b.MarkedDamage)
	}
}

func TestFight_Deathtouch(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Prey Upon", 0, 0, "sorcery")
	a := addBattlefield(gs, 0, "Deathtouch Snake", 1, 1, "creature")
	a.Flags = map[string]int{"kw:deathtouch": 1}
	b := addBattlefield(gs, 1, "Dragon", 6, 6, "creature")

	e := &gameast.Fight{
		A: gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		B: gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
	}
	ResolveEffect(gs, src, e)

	// Dragon should have deathtouch_damaged flag set.
	if b.Flags == nil || b.Flags["deathtouch_damaged"] != 1 {
		t.Fatal("fight with deathtouch source should set deathtouch_damaged on target")
	}
	// Snake took 6 damage from dragon (mutual fight).
	if a.MarkedDamage != 6 {
		t.Errorf("snake should take 6 damage, got %d", a.MarkedDamage)
	}
	// Dragon took 1 damage from snake.
	if b.MarkedDamage != 1 {
		t.Errorf("dragon should take 1 damage, got %d", b.MarkedDamage)
	}
}

func TestBite_OneSided(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Ram Through", 0, 0, "instant")
	a := addBattlefield(gs, 0, "Rhino", 4, 4, "creature")
	b := addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	e := &gameast.Fight{
		A:        gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		B:        gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
		OneSided: true,
	}
	ResolveEffect(gs, src, e)

	// Bite: only A deals damage to B. B does NOT deal damage to A.
	if a.MarkedDamage != 0 {
		t.Errorf("bite: rhino should take 0 damage (one-sided), got %d", a.MarkedDamage)
	}
	if b.MarkedDamage != 4 {
		t.Errorf("bite: bear should take 4 damage, got %d", b.MarkedDamage)
	}

	// Verify event is "bite" not "fight".
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "bite" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'bite' event for one-sided fight")
	}
}

func TestBite_DeathtouchOneSided(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Ram Through", 0, 0, "instant")
	a := addBattlefield(gs, 0, "Vampire", 1, 1, "creature")
	a.Flags = map[string]int{"kw:deathtouch": 1}
	b := addBattlefield(gs, 1, "Titan", 8, 8, "creature")

	e := &gameast.Fight{
		A:        gameast.Filter{Base: "creature", YouControl: true, Targeted: true},
		B:        gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true},
		OneSided: true,
	}
	ResolveEffect(gs, src, e)

	// Titan gets deathtouch flag, vampire takes no damage.
	if b.Flags == nil || b.Flags["deathtouch_damaged"] != 1 {
		t.Fatal("bite with deathtouch should set deathtouch_damaged on target")
	}
	if a.MarkedDamage != 0 {
		t.Error("bite: vampire should take 0 damage (one-sided)")
	}
}

func TestHexproofFromColor_BlocksTargeting(t *testing.T) {
	gs := newFixtureGame(t)
	// Creature with hexproof from black.
	perm := addBattlefield(gs, 1, "Sigarda", 5, 5, "creature")
	perm.Flags = map[string]int{"hexproof_from:B": 1}

	// Source is a black spell trying to target.
	srcCard := &Card{Name: "Doom Blade", Colors: []string{"B"}}
	src := &Permanent{Card: srcCard, Controller: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	f := gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true}
	targets := PickTarget(gs, src, f)

	// Should not target Sigarda — hexproof from black blocks it.
	for _, tgt := range targets {
		if tgt.Permanent == perm {
			t.Fatal("hexproof from black should prevent black spell from targeting")
		}
	}
}

func TestHexproofFromColor_AllowsOtherColor(t *testing.T) {
	gs := newFixtureGame(t)
	// Creature with hexproof from black.
	perm := addBattlefield(gs, 1, "Sigarda", 5, 5, "creature")
	perm.Flags = map[string]int{"hexproof_from:B": 1}

	// Source is a RED spell — should be allowed.
	srcCard := &Card{Name: "Lightning Bolt", Colors: []string{"R"}}
	src := &Permanent{Card: srcCard, Controller: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	f := gameast.Filter{Base: "creature", OpponentControls: true, Targeted: true}
	targets := PickTarget(gs, src, f)

	found := false
	for _, tgt := range targets {
		if tgt.Permanent == perm {
			found = true
		}
	}
	if !found {
		t.Fatal("hexproof from black should allow red spell to target")
	}
}

func TestWard_OnActivatedAbility(t *testing.T) {
	gs := newFixtureGame(t)
	// Seat 1 has a ward creature.
	wardCreature := &Permanent{
		Card:       &Card{Name: "Ward Beast", Types: []string{"creature"}},
		Controller: 1,
		Owner:      1,
		Flags:      map[string]int{"kw:ward": 1, "ward_cost": 3},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, wardCreature)

	// Seat 0 tries to activate an ability targeting the ward creature.
	activator := &Permanent{
		Card:       &Card{Name: "Activator", Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, activator)

	// Seat 0 has insufficient mana to pay ward.
	gs.Seats[0].ManaPool = 1

	item := &StackItem{
		Kind:       "activated",
		Controller: 0,
		Source:     activator,
		Card:       activator.Card,
		Targets: []Target{
			{Kind: TargetKindPermanent, Permanent: wardCreature},
		},
	}

	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("activated ability targeting ward creature should be countered when can't pay")
	}
}

// -----------------------------------------------------------------------------
// Tests — control flow
// -----------------------------------------------------------------------------

func TestSequenceInOrder(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Kor Spiritdancer", 0, 0, "creature")
	addLibrary(gs, 0, "A")

	// Sequence(Damage 2 to opponent; Draw 1)
	seq := &gameast.Sequence{
		Items: []gameast.Effect{
			&gameast.Damage{
				Amount: *gameast.NumInt(2),
				Target: gameast.TargetOpponent(),
			},
			&gameast.Draw{
				Count:  *gameast.NumInt(1),
				Target: gameast.Filter{Base: "controller"},
			},
		},
	}
	ResolveEffect(gs, src, seq)

	if gs.Seats[1].Life != 18 {
		t.Errorf("expected opponent life 18, got %d", gs.Seats[1].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected hand size 1, got %d", len(gs.Seats[0].Hand))
	}
	// Events must contain damage, life_change, and draw in order.
	// MoveCard may inject zone_change events between them.
	foundDamage, foundLifeChange, foundDraw := false, false, false
	for _, ev := range gs.EventLog {
		switch ev.Kind {
		case "damage":
			foundDamage = true
		case "life_change":
			if foundDamage {
				foundLifeChange = true
			}
		case "draw":
			if foundLifeChange {
				foundDraw = true
			}
		}
	}
	if !foundDamage {
		t.Errorf("expected damage event in log")
	}
	if !foundLifeChange {
		t.Errorf("expected life_change event after damage")
	}
	if !foundDraw {
		t.Errorf("expected draw event after life_change")
	}
}

func TestConditionalLifeThresholdTrueBranch(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Death's Shadow", 0, 0, "creature")
	// Seat 0 life 20; condition is life <= 10 → false → take else branch (draw 1).
	addLibrary(gs, 0, "X")

	cond := &gameast.Conditional{
		Condition: &gameast.Condition{
			Kind: "life_threshold",
			Args: []interface{}{"<=", 10},
		},
		Body: &gameast.Damage{
			Amount: *gameast.NumInt(5),
			Target: gameast.TargetOpponent(),
		},
		ElseBody: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	ResolveEffect(gs, src, cond)
	if gs.Seats[1].Life != 20 {
		t.Errorf("expected opponent life unchanged (20), got %d", gs.Seats[1].Life)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected draw 1 branch (hand size 1), got %d", len(gs.Seats[0].Hand))
	}
}

func TestConditionalLifeThresholdFalseBranch(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Death's Shadow", 0, 0, "creature")
	gs.Seats[0].Life = 5
	addLibrary(gs, 0, "X")

	cond := &gameast.Conditional{
		Condition: &gameast.Condition{
			Kind: "life_threshold",
			Args: []interface{}{"<=", 10},
		},
		Body: &gameast.Damage{
			Amount: *gameast.NumInt(5),
			Target: gameast.TargetOpponent(),
		},
		ElseBody: &gameast.Draw{
			Count:  *gameast.NumInt(1),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	ResolveEffect(gs, src, cond)
	if gs.Seats[1].Life != 15 {
		t.Errorf("expected opponent life 15, got %d", gs.Seats[1].Life)
	}
}

func TestOptionalAlwaysResolves(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Doom Whisperer", 0, 0, "creature")
	opt := &gameast.Optional_{
		Body: &gameast.GainLife{
			Amount: *gameast.NumInt(2),
			Target: gameast.Filter{Base: "controller"},
		},
	}
	ResolveEffect(gs, src, opt)
	if gs.Seats[0].Life != 22 {
		t.Errorf("expected life 22 from optional body, got %d", gs.Seats[0].Life)
	}
}

func TestChoicePicksFirst(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Kolaghan's Command", 0, 0, "instant")
	choice := &gameast.Choice{
		Pick: *gameast.NumInt(1),
		Options: []gameast.Effect{
			&gameast.Damage{
				Amount: *gameast.NumInt(2),
				Target: gameast.TargetOpponent(),
			},
			&gameast.GainLife{
				Amount: *gameast.NumInt(5),
				Target: gameast.Filter{Base: "controller"},
			},
		},
	}
	ResolveEffect(gs, src, choice)
	if gs.Seats[1].Life != 18 {
		t.Errorf("expected opponent life 18 (first option taken), got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("expected seat 0 life unchanged, got %d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Tests — meta
// -----------------------------------------------------------------------------

func TestExtraTurnFlag(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Time Walk", 0, 0, "sorcery")
	e := &gameast.ExtraTurn{Target: gameast.Filter{Base: "controller"}}
	ResolveEffect(gs, src, e)
	if gs.Flags["extra_turns_pending"] != 1 {
		t.Errorf("expected extra_turns_pending=1, got %d", gs.Flags["extra_turns_pending"])
	}
}

func TestWinGame(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Coalition Victory", 0, 0, "sorcery")
	e := &gameast.WinGame{Target: gameast.Filter{Base: "controller"}}
	ResolveEffect(gs, src, e)
	if !gs.Seats[0].Won {
		t.Errorf("expected seat 0 to have Won=true")
	}
}

func TestUnknownEffectDoesNotPanic(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Bizarre Card", 0, 0, "instant")
	ResolveEffect(gs, src, &gameast.UnknownEffect{RawText: "does something weird"})
	if countEvents(gs, "unknown_effect") != 1 {
		t.Errorf("expected 1 unknown_effect event, got %d", countEvents(gs, "unknown_effect"))
	}
}

func TestNilEffectNoPanic(t *testing.T) {
	gs := newFixtureGame(t)
	// Must not panic, must not mutate.
	ResolveEffect(gs, nil, nil)
	if len(gs.EventLog) != 0 {
		t.Errorf("expected no events on nil effect, got %d", len(gs.EventLog))
	}
}

// -----------------------------------------------------------------------------
// Corpus-driven integration test (skipped if corpus absent)
// -----------------------------------------------------------------------------

// corpusPath walks up from the package dir looking for data/rules/ast_dataset.jsonl.
func corpusPath(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "data", "rules", "ast_dataset.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// TestWithCorpus_LightningBolt is an integration test — loads the real AST
// corpus, finds Lightning Bolt, digs out its Damage effect through the
// Static/spell_effect wrapper, and resolves it against a fixture game.
func TestWithCorpus_LightningBolt(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: skipping corpus-driven test")
	}
	path := corpusPath(t)
	if path == "" {
		t.Skip("corpus not found; skipping integration test")
	}
	corpus, err := astload.Load(path)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	bolt, ok := corpus.Get("Lightning Bolt")
	if !ok {
		t.Fatalf("Lightning Bolt not in corpus")
	}
	// Reach through Static/parsed_tail → look for a Damage effect in args.
	var dmg *gameast.Damage
	for _, ab := range bolt.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok {
			continue
		}
		if st.Modification == nil {
			continue
		}
		for _, arg := range st.Modification.Args {
			if d, ok := arg.(*gameast.Damage); ok {
				dmg = d
				break
			}
		}
	}
	if dmg == nil {
		// The current parser wraps Bolt as parsed_tail; skip gracefully.
		t.Skip("Lightning Bolt Damage not structured in corpus (parsed_tail); skipping")
	}
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Lightning Bolt", 0, 0, "instant")
	ResolveEffect(gs, src, dmg)
	if gs.Seats[1].Life >= 20 {
		t.Errorf("expected opponent life < 20 after bolt, got %d", gs.Seats[1].Life)
	}
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

func BenchmarkResolveDamage(b *testing.B) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	src := addBattlefield(gs, 0, "Shock", 0, 0, "instant")
	e := &gameast.Damage{
		Amount: *gameast.NumInt(1),
		Target: gameast.TargetOpponent(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveEffect(gs, src, e)
	}
}

func BenchmarkResolveDraw(b *testing.B) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	src := addBattlefield(gs, 0, "Opt", 0, 0, "instant")
	// Pre-populate a very large library so draws don't go empty.
	for i := 0; i < b.N+8; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "X"})
	}
	e := &gameast.Draw{
		Count:  *gameast.NumInt(1),
		Target: gameast.Filter{Base: "controller"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveEffect(gs, src, e)
	}
}

func BenchmarkResolveCreateToken(b *testing.B) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	src := addBattlefield(gs, 0, "Raise the Alarm", 0, 0, "instant")
	pt := [2]int{1, 1}
	e := &gameast.CreateToken{
		Count: *gameast.NumInt(1),
		PT:    &pt,
		Types: []string{"creature", "soldier"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveEffect(gs, src, e)
	}
}

func BenchmarkResolveSequenceDmgDraw(b *testing.B) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	src := addBattlefield(gs, 0, "K-Command", 0, 0, "instant")
	for i := 0; i < b.N+16; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "X"})
	}
	seq := &gameast.Sequence{
		Items: []gameast.Effect{
			&gameast.Damage{Amount: *gameast.NumInt(1), Target: gameast.TargetOpponent()},
			&gameast.Draw{Count: *gameast.NumInt(1), Target: gameast.Filter{Base: "controller"}},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveEffect(gs, src, seq)
	}
}
