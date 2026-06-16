package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// constellation_dispatch_r63_test.go — mechanic-probe (CONSTELLATION, CR
// §702.131). Generic-AST constellation payoffs (Eidolon of Blossoms,
// Doomwake Giant, Underworld Coinsmith, …) were INERT: the dedicated
// dispatcher FireConstellationTriggers only LOGGED a constellation_trigger
// event and didn't even detect the Static[ability_word "constellation" →
// Triggered] shape, so no effect ever resolved. Fixed: the dispatcher now
// extracts and resolves the wrapped trigger. These pin each property.

func cnstGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(11)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func cnstDrain(gs *GameState) {
	for i := 0; i < 100 && len(gs.Stack) > 0; i++ {
		ResolveStackTop(gs)
	}
}

// constellation payoff: gain 1 life (to its controller) on this-or-another
// enchantment you control entering — the exact Static[ability_word] shape.
func cnstPayoffCard(name string, owner int) *Card {
	inner := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "etb_or_another"},
		Effect: &gameast.GainLife{
			Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
			Target: gameast.Filter{Base: "controller"},
		},
		Raw: "constellation — whenever this or another enchantment you control enters, you gain 1 life",
	}
	c := &Card{Name: name, Owner: owner, Types: []string{"enchantment"}}
	c.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{
		&gameast.Static{Modification: &gameast.Modification{
			ModKind: "ability_word",
			Args:    []interface{}{"constellation", "triggered", inner},
		}},
	}}
	return c
}

func addCnstPayoff(gs *GameState, seat int, name string) *Permanent {
	p := &Permanent{Card: cnstPayoffCard(name, seat), Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// enterEnchantment puts an enchantment (with optional extra types) onto
// seat's battlefield and fires the full ETB cascade (→ OnConstellationETB),
// then drains the stack. Returns the new permanent.
func enterEnchantment(gs *GameState, seat int, name string, extraTypes ...string) *Permanent {
	types := append([]string{"enchantment"}, extraTypes...)
	c := &Card{Name: name, Owner: seat, Types: types}
	p := &Permanent{Card: c, Controller: seat, Owner: seat,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	cnstDrain(gs)
	return p
}

// (a): once per enchantment — two enchantments entering = two triggers.
func TestConstellation_OncePerEnchantment(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Underworld Coinsmith")
	life := gs.Seats[0].Life

	enterEnchantment(gs, 0, "Bottomless Pit")
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("(a) constellation fired %d times for 1 enchantment, want 1 (was INERT pre-fix)", got)
	}
	enterEnchantment(gs, 0, "Oblivion Ring")
	if got := gs.Seats[0].Life - life; got != 2 {
		t.Fatalf("(a) constellation fired %d times total for 2 enchantments, want 2", got)
	}
}

// (b): the SOURCE enchantment entering triggers its OWN constellation, AND
// other constellation enchantments already in play.
func TestConstellation_SourceTriggersItself(t *testing.T) {
	gs := cnstGame(t)
	life := gs.Seats[0].Life
	// A constellation card ENTERS with nothing else in play — it must see
	// itself enter and fire (constellation counts itself).
	p := &Permanent{Card: cnstPayoffCard("Eidolon of Blossoms", 0), Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	cnstDrain(gs)
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("(b) entering constellation card must trigger its OWN constellation; got +%d want +1", got)
	}
}

func TestConstellation_SourceAndOthersBothFire(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Doomwake Giant") // already in play
	life := gs.Seats[0].Life
	// A second constellation enchantment enters: BOTH fire (the newcomer
	// sees itself; the incumbent sees the newcomer).
	p := &Permanent{Card: cnstPayoffCard("Eidolon of Blossoms", 0), Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	FirePermanentETBTriggers(gs, p)
	cnstDrain(gs)
	if got := gs.Seats[0].Life - life; got != 2 {
		t.Fatalf("(b/f) source + incumbent constellation must both fire; got +%d want +2", got)
	}
}

// (c): any enchantment TYPE entering triggers constellation — enchantment
// creature (God), Saga, aura, enchantment artifact.
func TestConstellation_AnyEnchantmentType(t *testing.T) {
	for _, tc := range []struct {
		name   string
		extras []string
	}{
		{"God (ench creature)", []string{"creature", "god"}},
		{"A Saga", []string{"saga"}},
		{"An Aura", []string{"aura"}},
		{"Ench Artifact", []string{"artifact"}},
	} {
		gs := cnstGame(t)
		addCnstPayoff(gs, 0, "Underworld Coinsmith")
		life := gs.Seats[0].Life
		enterEnchantment(gs, 0, tc.name, tc.extras...)
		if got := gs.Seats[0].Life - life; got != 1 {
			t.Fatalf("(c) %s entering must trigger constellation; got +%d want +1", tc.name, got)
		}
	}
}

// (d): your-control-only — an opponent's enchantment must NOT fire my
// constellation.
func TestConstellation_OpponentEnchantmentDoesNotFire(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Eidolon of Blossoms")
	life := gs.Seats[0].Life
	enterEnchantment(gs, 1, "Opp Enchantment") // opponent's
	if gs.Seats[0].Life != life {
		t.Fatalf("(d) opponent's enchantment fired my constellation: +%d", gs.Seats[0].Life-life)
	}
}

// (e): tokens that are enchantments count.
func TestConstellation_TokenEnchantmentCounts(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Underworld Coinsmith")
	life := gs.Seats[0].Life
	enterEnchantment(gs, 0, "Enchant Token", "token")
	if got := gs.Seats[0].Life - life; got != 1 {
		t.Fatalf("(e) token enchantment must trigger constellation; got +%d want +1", got)
	}
}

// (f): multiple constellation permanents each trigger per enchantment ETB.
func TestConstellation_MultipleSourcesEachFire(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Eidolon of Blossoms")
	addCnstPayoff(gs, 0, "Doomwake Giant")
	addCnstPayoff(gs, 0, "Underworld Coinsmith")
	life := gs.Seats[0].Life
	enterEnchantment(gs, 0, "Bottomless Pit") // one enchantment ETB
	if got := gs.Seats[0].Life - life; got != 3 {
		t.Fatalf("(f) 3 constellation sources must each fire on one enchantment ETB; got +%d want +3", got)
	}
}

// nonland/noncreature non-enchantment entering must NOT fire constellation.
func TestConstellation_NonEnchantmentDoesNotFire(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Eidolon of Blossoms")
	life := gs.Seats[0].Life
	// a plain creature (no enchantment type) enters under seat 0
	cr := &Permanent{Card: &Card{Name: "Bear", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, cr)
	FirePermanentETBTriggers(gs, cr)
	cnstDrain(gs)
	if gs.Seats[0].Life != life {
		t.Fatalf("constellation fired on a non-enchantment ETB: +%d", gs.Seats[0].Life-life)
	}
}

// double-fire guard: when a per_card handler owns the card's enchantment-ETB
// (Calix registers OnTrigger("permanent_etb")), the dispatcher must NOT also
// resolve the wrapped effect.
func TestConstellation_PerCardGuardNoDoubleFire(t *testing.T) {
	gs := cnstGame(t)
	addCnstPayoff(gs, 0, "Calix, Guided by Fate")

	old := HasTriggerHook
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Calix, Guided by Fate" && event == "permanent_etb"
	}
	defer func() { HasTriggerHook = old }()

	life := gs.Seats[0].Life
	enterEnchantment(gs, 0, "Bottomless Pit")
	if gs.Seats[0].Life != life {
		t.Fatalf("dispatcher resolved a constellation owned by a per_card permanent_etb handler (double-fire): +%d", gs.Seats[0].Life-life)
	}
}
