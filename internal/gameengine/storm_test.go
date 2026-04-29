package gameengine

// Unit tests for Storm keyword + cast-count infrastructure.
// Mirrors scripts/test_storm.py (Python reference).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// addHandCardFull adds a card to a seat's hand with full metadata:
//   - Name, Types (with cost:N for mana), AST Effect
//   - Colors, TypeLine (for cast-trigger observers)
func addHandCardFull(gs *GameState, seat int, name string, cost int,
	eff gameast.Effect, typeLine string, types []string, colors []string) *Card {
	t := append([]string(nil), types...)
	if cost > 0 {
		t = append(t, "cost:"+itoa(cost))
	}
	ast := &gameast.CardAST{Name: name}
	if eff != nil {
		ast.Abilities = []gameast.Ability{&gameast.Activated{Effect: eff}}
	}
	c := &Card{
		Name:     name,
		Owner:    seat,
		Types:    t,
		Colors:   colors,
		TypeLine: typeLine,
		AST:      ast,
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// makeFiller is a CMC-1 sorcery with no effect. Used to pump the cast
// counter.
func makeFiller(gs *GameState, seat int, name string) *Card {
	return addHandCardFull(gs, seat, name, 1, nil, "Sorcery",
		[]string{"sorcery"}, nil)
}

// makeRedFiller is a CMC-1 red sorcery.
func makeRedFiller(gs *GameState, seat int, name string) *Card {
	return addHandCardFull(gs, seat, name, 1, nil, "Sorcery",
		[]string{"sorcery"}, []string{"R"})
}

// makeGrapeshot — Grapeshot-shaped spell: "deals 1 damage to any target",
// Storm. We mark the card name as "Grapeshot" so the storm lookup matches.
func makeGrapeshot(gs *GameState, seat int) *Card {
	eff := &gameast.Damage{
		Amount: *gameast.NumInt(1),
		Target: gameast.TargetOpponent(),
	}
	return addHandCardFull(gs, seat, "Grapeshot", 2, eff, "Sorcery",
		[]string{"sorcery"}, []string{"R"})
}

// addObserverPermanent drops an observer permanent (Storm-Kiln Artist,
// Young Pyromancer, Birgi, …) onto the battlefield for `seat`. Creature
// P/T are irrelevant for cast-trigger testing.
func addObserverPermanent(gs *GameState, seat int, name string, colors []string) *Permanent {
	return addBattlefieldFull(gs, seat, name, 1, 1,
		[]string{"creature"}, colors)
}

// addBattlefieldFull is like addBattlefield but populates Colors too.
func addBattlefieldFull(gs *GameState, seat int, name string,
	pow, tough int, types []string, colors []string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string(nil), types...),
		Colors:        append([]string(nil), colors...),
	}
	p := &Permanent{
		Card:          card,
		Controller:    seat,
		Owner:         seat,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
		SummoningSick: false,
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// ---------------------------------------------------------------------------
// Test 1: cast-count increment
// ---------------------------------------------------------------------------

func TestStorm_CastCountIncrement(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10

	if gs.SpellsCastThisTurn != 0 {
		t.Fatalf("baseline: SpellsCastThisTurn=%d, want 0",
			gs.SpellsCastThisTurn)
	}
	if gs.Seats[0].SpellsCastThisTurn != 0 {
		t.Fatalf("baseline: Seat[0].SpellsCastThisTurn=%d, want 0",
			gs.Seats[0].SpellsCastThisTurn)
	}
	f := makeFiller(gs, 0, "Filler")
	if err := CastSpell(gs, 0, f, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if gs.SpellsCastThisTurn != 1 {
		t.Errorf("after 1 cast: SpellsCastThisTurn=%d, want 1",
			gs.SpellsCastThisTurn)
	}
	if gs.Seats[0].SpellsCastThisTurn != 1 {
		t.Errorf("after 1 cast: Seat[0].SpellsCastThisTurn=%d, want 1",
			gs.Seats[0].SpellsCastThisTurn)
	}
	if gs.Seats[1].SpellsCastThisTurn != 0 {
		t.Errorf("Seat[1].SpellsCastThisTurn=%d, want 0",
			gs.Seats[1].SpellsCastThisTurn)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Storm copies Grapeshot for 3 total damage
// ---------------------------------------------------------------------------

func TestStorm_GrapeshotCopies(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100

	// Pre-cast 2 fillers
	for i := 0; i < 2; i++ {
		f := makeFiller(gs, 0, "Filler")
		if err := CastSpell(gs, 0, f, nil); err != nil {
			t.Fatalf("filler %d: %v", i, err)
		}
	}
	if gs.SpellsCastThisTurn != 2 {
		t.Fatalf("after 2 fillers: SpellsCastThisTurn=%d, want 2",
			gs.SpellsCastThisTurn)
	}
	lifeBefore := gs.Seats[1].Life
	grape := makeGrapeshot(gs, 0)
	if err := CastSpell(gs, 0, grape, nil); err != nil {
		t.Fatalf("grapeshot: %v", err)
	}
	if gs.SpellsCastThisTurn != 3 {
		t.Errorf("after grapeshot: SpellsCastThisTurn=%d, want 3",
			gs.SpellsCastThisTurn)
	}
	damage := lifeBefore - gs.Seats[1].Life
	if damage != 3 {
		t.Errorf("Grapeshot (with 2 prior casts) dealt %d damage, want 3 "+
			"(1 original + 2 storm copies)", damage)
	}
	// Verify storm_trigger event fired once
	stormEvents := countEvents(gs, "storm_trigger")
	if stormEvents != 1 {
		t.Errorf("storm_trigger events = %d, want 1", stormEvents)
	}
	storm := lastEventOfKind(gs, "storm_trigger")
	if storm.Amount != 2 {
		t.Errorf("storm_trigger.Amount = %d, want 2", storm.Amount)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Runaway Steam-Kin counter accumulation
// ---------------------------------------------------------------------------

func TestStorm_RunawaySteamKin(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100
	perm := addObserverPermanent(gs, 0, "Runaway Steam-Kin", []string{"R"})

	if perm.Counters["+1/+1"] != 0 {
		t.Fatalf("baseline counters: %d, want 0", perm.Counters["+1/+1"])
	}
	for i := 0; i < 3; i++ {
		r := makeRedFiller(gs, 0, "Red")
		if err := CastSpell(gs, 0, r, nil); err != nil {
			t.Fatalf("red cast %d: %v", i, err)
		}
	}
	if got := perm.Counters["+1/+1"]; got != 3 {
		t.Errorf("after 3 red casts: counters=%d, want 3", got)
	}
	// Fourth red cast must not push past 3.
	r4 := makeRedFiller(gs, 0, "Red4")
	if err := CastSpell(gs, 0, r4, nil); err != nil {
		t.Fatalf("red 4: %v", err)
	}
	if got := perm.Counters["+1/+1"]; got != 3 {
		t.Errorf("after 4th red cast: counters=%d, want 3 (capped)", got)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Birgi mana gen
// ---------------------------------------------------------------------------

func TestStorm_BirgiManaGen(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100
	addObserverPermanent(gs, 0, "Birgi, God of Storytelling", []string{"R"})

	poolBefore := gs.Seats[0].ManaPool
	for i := 0; i < 5; i++ {
		f := makeFiller(gs, 0, "Filler")
		if err := CastSpell(gs, 0, f, nil); err != nil {
			t.Fatalf("cast %d: %v", i, err)
		}
	}
	poolAfter := gs.Seats[0].ManaPool
	// Each cast: -1 for cost, +1 from Birgi — net 0.
	if poolAfter != poolBefore {
		t.Errorf("mana pool %d -> %d (delta %d), want 0 "+
			"(5 casts cost 5, Birgi refunds 5)",
			poolBefore, poolAfter, poolAfter-poolBefore)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Young Pyromancer tokens
// ---------------------------------------------------------------------------

func TestStorm_YoungPyromancerTokens(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100
	addObserverPermanent(gs, 0, "Young Pyromancer", []string{"R"})

	before := len(gs.Seats[0].Battlefield)
	for i := 0; i < 3; i++ {
		f := makeFiller(gs, 0, "Filler")
		if err := CastSpell(gs, 0, f, nil); err != nil {
			t.Fatalf("cast %d: %v", i, err)
		}
	}
	after := len(gs.Seats[0].Battlefield)
	if after-before != 3 {
		t.Errorf("battlefield: %d -> %d (delta %d), want 3 Elemental tokens",
			before, after, after-before)
	}
	// Validate the new tokens are Elementals
	elementalCount := 0
	for _, p := range gs.Seats[0].Battlefield[before:] {
		if p.Card.DisplayName() == "Elemental Token" {
			elementalCount++
		}
	}
	if elementalCount != 3 {
		t.Errorf("elemental tokens = %d, want 3", elementalCount)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Storm copies don't re-trigger Storm
// ---------------------------------------------------------------------------

func TestStorm_CopiesNoRecursion(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100

	// 1 filler + Grapeshot. Grapeshot is the 2nd cast; storm makes 1 copy.
	// That copy must not trigger storm again.
	f := makeFiller(gs, 0, "Filler")
	if err := CastSpell(gs, 0, f, nil); err != nil {
		t.Fatalf("filler: %v", err)
	}
	grape := makeGrapeshot(gs, 0)
	if err := CastSpell(gs, 0, grape, nil); err != nil {
		t.Fatalf("grapeshot: %v", err)
	}
	if got := countEvents(gs, "storm_trigger"); got != 1 {
		t.Errorf("storm_trigger events = %d, want 1", got)
	}
	if gs.SpellsCastThisTurn != 2 {
		t.Errorf("SpellsCastThisTurn=%d, want 2 (copies don't increment)",
			gs.SpellsCastThisTurn)
	}
}

// ---------------------------------------------------------------------------
// Test 7: Storm copies don't fire cast-trigger observers
// ---------------------------------------------------------------------------

func TestStorm_CopiesNoObserverTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100
	addObserverPermanent(gs, 0, "Young Pyromancer", []string{"R"})

	before := len(gs.Seats[0].Battlefield)

	f := makeFiller(gs, 0, "Filler")
	if err := CastSpell(gs, 0, f, nil); err != nil {
		t.Fatalf("filler: %v", err)
	}
	grape := makeGrapeshot(gs, 0)
	if err := CastSpell(gs, 0, grape, nil); err != nil {
		t.Fatalf("grapeshot: %v", err)
	}
	after := len(gs.Seats[0].Battlefield)
	// Real casts: filler + Grapeshot = 2 casts = 2 tokens. Storm copies
	// don't produce tokens.
	got := after - before
	if got != 2 {
		t.Errorf("elemental tokens = %d, want 2 (2 real casts, copies don't trigger)", got)
	}
}

// ---------------------------------------------------------------------------
// Test 8: HasStormKeyword lookup correctness
// ---------------------------------------------------------------------------

func TestStorm_HasStormKeyword(t *testing.T) {
	tests := []struct {
		name     string
		cardName string
		want     bool
	}{
		{"grapeshot", "Grapeshot", true},
		{"tendrils", "Tendrils of Agony", true},
		{"brain_freeze", "Brain Freeze", true},
		{"mind's_desire", "Mind's Desire", true},
		{"not_storm", "Lightning Bolt", false},
		{"nil_card", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cardName == "" {
				if HasStormKeyword(nil) {
					t.Error("HasStormKeyword(nil) = true, want false")
				}
				return
			}
			c := &Card{Name: tt.cardName}
			if got := HasStormKeyword(c); got != tt.want {
				t.Errorf("HasStormKeyword(%q) = %v, want %v",
					tt.cardName, got, tt.want)
			}
		})
	}
}
