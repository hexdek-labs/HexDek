package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R45 stub-batch ports — five gen_*.go pure-stub / partial-ETB handlers
// from the alphabetical SECOND half (n-z), avoiding the
// R36/R37/R41/R42/R42b/R43/R44 sets.
//
// Picks:
//   - Urabrask, Heretic Praetor     own-upkeep impulse + cast-from-exile grant
//   - The Master, Multiplied         token legend-rule exception via SBA tweak
//   - Phylath, World Sculptor        Plant ETB swarm + landfall +1/+1 counters
//   - Neriv, Crackling Vanguard      Goblin ETB + attack-exile impulse
//   - Vishgraz, the Doomhive         Phyrexian Mite ETB + opp-poison P/T buff

// ---------------------------------------------------------------------------
// Urabrask, Heretic Praetor
// ---------------------------------------------------------------------------

func TestUrabrask_OwnUpkeepExilesTopAndGrantsCast(t *testing.T) {
	gs := newGame(t, 2)
	urabrask := stampCreaturePT(addPerm(gs, 0, "Urabrask, Heretic Praetor", "creature", "legendary"), 4, 1)
	bolt := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, bolt)

	urabraskUpkeepImpulse(gs, urabrask, map[string]interface{}{
		"active_seat": 0,
	})

	// Bolt should be exiled.
	foundExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == bolt {
			foundExile = true
			break
		}
	}
	if !foundExile {
		t.Fatal("top card should have been exiled")
	}
	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[bolt] == nil {
		t.Fatal("exiled card should receive a cast-from-exile grant")
	}
	g := gs.ZoneCastGrants[bolt]
	if g.Zone != gameengine.ZoneExile {
		t.Errorf("grant Zone = %q, want exile", g.Zone)
	}
	if g.Duration != "until_end_of_turn" {
		t.Errorf("grant Duration = %q, want until_end_of_turn", g.Duration)
	}
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected end-of-turn cleanup delayed trigger")
	}
}

func TestUrabrask_OpponentUpkeepEmitsPartialOnly(t *testing.T) {
	gs := newGame(t, 2)
	urabrask := stampCreaturePT(addPerm(gs, 0, "Urabrask, Heretic Praetor", "creature", "legendary"), 4, 1)
	bolt := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, bolt)

	urabraskUpkeepImpulse(gs, urabrask, map[string]interface{}{
		"active_seat": 1, // opponent upkeep
	})

	// Library should be untouched.
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("opponent upkeep should not exile Urabrask controller's library; exile=%d", len(gs.Seats[0].Exile))
	}
}

// ---------------------------------------------------------------------------
// The Master, Multiplied — token legend-rule exception via SBA
// ---------------------------------------------------------------------------

func TestTheMaster_TokenLegendRuleExceptionPreservesTokens(t *testing.T) {
	gs := newGame(t, 2)
	master := stampCreaturePT(addPerm(gs, 0, "The Master, Multiplied", "creature", "legendary"), 4, 4)
	_ = master

	// Two creature-token copies of the same legendary token name —
	// without the exception, SBA would graveyard one. With The Master
	// active, both survive.
	for i := 0; i < 2; i++ {
		tokCard := &gameengine.Card{
			Name:          "Master Copy",
			Owner:         0,
			Types:         []string{"token", "creature", "legendary"},
			BasePower:     4,
			BaseToughness: 4,
		}
		tokPerm := &gameengine.Permanent{
			Card:       tokCard,
			Controller: 0,
			Owner:      0,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokPerm)
	}

	gameengine.StateBasedActions(gs)

	// Count surviving "Master Copy" tokens.
	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Master Copy" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("with The Master active, both legendary token copies should survive; count=%d", count)
	}
}

func TestTheMaster_LegendRuleStillAppliesWithoutMaster(t *testing.T) {
	gs := newGame(t, 2)
	// No Master on the battlefield. Two legendary creature tokens with
	// the same name → SBA should kill one.
	for i := 0; i < 2; i++ {
		tokCard := &gameengine.Card{
			Name:          "Master Copy",
			Owner:         0,
			Types:         []string{"token", "creature", "legendary"},
			BasePower:     4,
			BaseToughness: 4,
		}
		tokPerm := &gameengine.Permanent{
			Card:       tokCard,
			Controller: 0,
			Owner:      0,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokPerm)
	}

	gameengine.StateBasedActions(gs)

	count := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Master Copy" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("without The Master, legend rule should leave 1 survivor; count=%d", count)
	}
}

// ---------------------------------------------------------------------------
// Phylath, World Sculptor
// ---------------------------------------------------------------------------

func TestPhylath_ETBSpawnsPlantPerBasic(t *testing.T) {
	gs := newGame(t, 2)
	phylath := stampCreaturePT(addPerm(gs, 0, "Phylath, World Sculptor", "creature", "legendary"), 5, 5)

	// Two basics + one nonbasic land.
	addPerm(gs, 0, "Forest 1", "land", "basic", "forest")
	addPerm(gs, 0, "Forest 2", "land", "basic", "forest")
	addPerm(gs, 0, "Spirebluff Canal", "land")

	preBF := len(gs.Seats[0].Battlefield)
	phylathWorldSculptorETB(gs, phylath)

	plants := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Plant" {
			plants++
		}
	}
	if plants != 2 {
		t.Errorf("expected 2 Plant tokens (one per basic), got %d", plants)
	}
	if got := len(gs.Seats[0].Battlefield); got != preBF+2 {
		t.Errorf("battlefield delta = %d, want +2", got-preBF)
	}
}

func TestPhylath_LandfallStampsFourCountersOnBestPlant(t *testing.T) {
	gs := newGame(t, 2)
	phylath := stampCreaturePT(addPerm(gs, 0, "Phylath, World Sculptor", "creature", "legendary"), 5, 5)

	// Two Plants: one small, one big.
	smallPlant := stampCreaturePT(addPerm(gs, 0, "Small Plant", "creature", "plant"), 1, 1)
	bigPlant := stampCreaturePT(addPerm(gs, 0, "Big Plant", "creature", "plant"), 3, 5)

	enteringLand := addPerm(gs, 0, "Forest", "land", "basic", "forest")
	phylathLandfall(gs, phylath, map[string]interface{}{
		"perm":            enteringLand,
		"controller_seat": 0,
	})

	if bigPlant.Counters["+1/+1"] != 4 {
		t.Errorf("expected 4 +1/+1 counters on highest-toughness Plant; got %d", bigPlant.Counters["+1/+1"])
	}
	if smallPlant.Counters["+1/+1"] != 0 {
		t.Errorf("smaller Plant should not have been chosen; got %d", smallPlant.Counters["+1/+1"])
	}
}

func TestPhylath_LandfallIgnoresOpponentLands(t *testing.T) {
	gs := newGame(t, 2)
	phylath := stampCreaturePT(addPerm(gs, 0, "Phylath, World Sculptor", "creature", "legendary"), 5, 5)
	plant := stampCreaturePT(addPerm(gs, 0, "Plant", "creature", "plant"), 0, 1)

	oppLand := addPerm(gs, 1, "Forest", "land", "basic", "forest")
	phylathLandfall(gs, phylath, map[string]interface{}{
		"perm":            oppLand,
		"controller_seat": 1,
	})
	if plant.Counters["+1/+1"] != 0 {
		t.Errorf("opponent land entering should NOT trigger; got %d counters", plant.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Neriv, Crackling Vanguard
// ---------------------------------------------------------------------------

func TestNeriv_ETBSpawnsTwoGoblins(t *testing.T) {
	gs := newGame(t, 2)
	neriv := stampCreaturePT(addPerm(gs, 0, "Neriv, Crackling Vanguard", "creature", "legendary"), 3, 3)
	preBF := len(gs.Seats[0].Battlefield)

	nerivCracklingVanguardETB(gs, neriv)

	goblins := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Goblin" {
			goblins++
		}
	}
	if goblins != 2 {
		t.Errorf("expected 2 Goblin tokens, got %d", goblins)
	}
	if got := len(gs.Seats[0].Battlefield); got != preBF+2 {
		t.Errorf("battlefield delta = %d, want +2", got-preBF)
	}
}

func TestNeriv_AttackExilesCardsEqualToDifferentNamedTokens(t *testing.T) {
	gs := newGame(t, 2)
	neriv := stampCreaturePT(addPerm(gs, 0, "Neriv, Crackling Vanguard", "creature", "legendary"), 3, 3)

	// Mint 3 distinct token names by hand.
	for _, name := range []string{"Goblin", "Treasure", "Spirit"} {
		tokCard := &gameengine.Card{
			Name:  name,
			Owner: 0,
			Types: []string{"token", "creature"},
		}
		tokPerm := &gameengine.Permanent{
			Card:       tokCard,
			Controller: 0,
			Owner:      0,
			Timestamp:  gs.NextTimestamp(),
			Counters:   map[string]int{},
			Flags:      map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokPerm)
	}
	// Also add a duplicate-name Goblin: should NOT count as a new name.
	tokCard := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"token", "creature"}}
	tokPerm := &gameengine.Permanent{Card: tokCard, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokPerm)

	// Library: 5 cards.
	c1 := &gameengine.Card{Name: "C1", Owner: 0, Types: []string{"instant"}}
	c2 := &gameengine.Card{Name: "C2", Owner: 0, Types: []string{"sorcery"}}
	c3 := &gameengine.Card{Name: "C3", Owner: 0, Types: []string{"creature"}}
	c4 := &gameengine.Card{Name: "C4", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Library = []*gameengine.Card{c1, c2, c3, c4}

	nerivAttackImpulse(gs, neriv, map[string]interface{}{
		"attacker_perm": neriv,
		"attacker_seat": 0,
	})

	// 3 distinct token names → exile top 3.
	if len(gs.Seats[0].Exile) != 3 {
		t.Errorf("expected 3 exiled cards, got %d", len(gs.Seats[0].Exile))
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("expected 1 card left in library, got %d", len(gs.Seats[0].Library))
	}
	// Each exiled card should have a grant.
	for _, c := range []*gameengine.Card{c1, c2, c3} {
		if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[c] == nil {
			t.Errorf("exiled card %q should have a cast grant", c.DisplayName())
		}
	}
}

func TestNeriv_AttackIgnoresOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	neriv := stampCreaturePT(addPerm(gs, 0, "Neriv, Crackling Vanguard", "creature", "legendary"), 3, 3)
	other := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)

	tokCard := &gameengine.Card{Name: "Goblin", Owner: 0, Types: []string{"token", "creature"}}
	tokPerm := &gameengine.Permanent{Card: tokCard, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokPerm)
	addLibrary(gs, 0, "X")

	nerivAttackImpulse(gs, neriv, map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})
	if len(gs.Seats[0].Exile) != 0 {
		t.Errorf("non-Neriv attacker should NOT trigger; exile=%d", len(gs.Seats[0].Exile))
	}
}

// ---------------------------------------------------------------------------
// Vishgraz, the Doomhive
// ---------------------------------------------------------------------------

func TestVishgraz_ETBSpawnsThreeMites(t *testing.T) {
	gs := newGame(t, 2)
	vishgraz := stampCreaturePT(addPerm(gs, 0, "Vishgraz, the Doomhive", "creature", "legendary"), 4, 4)
	preBF := len(gs.Seats[0].Battlefield)

	vishgrazTheDoomhiveETB(gs, vishgraz)

	mites := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Phyrexian Mite" {
			mites++
			if p.Flags["kw:toxic"] != 1 {
				t.Errorf("Mite should have kw:toxic flag")
			}
			if p.Flags["cant_block"] != 1 {
				t.Errorf("Mite should have cant_block flag")
			}
		}
	}
	if mites != 3 {
		t.Errorf("expected 3 Phyrexian Mites, got %d", mites)
	}
	if got := len(gs.Seats[0].Battlefield); got != preBF+3 {
		t.Errorf("battlefield delta = %d, want +3", got-preBF)
	}
}

func TestVishgraz_BuffScalesWithOpponentPoison(t *testing.T) {
	gs := newGame(t, 3)
	vishgraz := stampCreaturePT(addPerm(gs, 0, "Vishgraz, the Doomhive", "creature", "legendary"), 4, 4)

	gs.Seats[1].PoisonCounters = 3
	gs.Seats[2].PoisonCounters = 2

	vishgrazTheDoomhiveETB(gs, vishgraz)

	// Base 4/4 + 5/5 buff = 9/9.
	if got := vishgraz.Power(); got != 9 {
		t.Errorf("expected power 9 (4 + Σ opp poison 5), got %d", got)
	}
	if got := vishgraz.Toughness(); got != 9 {
		t.Errorf("expected toughness 9, got %d", got)
	}
}

func TestVishgraz_BuffIgnoresOwnPoison(t *testing.T) {
	gs := newGame(t, 2)
	vishgraz := stampCreaturePT(addPerm(gs, 0, "Vishgraz, the Doomhive", "creature", "legendary"), 4, 4)
	gs.Seats[0].PoisonCounters = 7 // own poison should not buff
	gs.Seats[1].PoisonCounters = 0

	vishgrazTheDoomhiveETB(gs, vishgraz)

	if got := vishgraz.Power(); got != 4 {
		t.Errorf("expected base 4 (own poison ignored), got %d", got)
	}
}

func TestVishgraz_RecomputeReplacesBuff(t *testing.T) {
	gs := newGame(t, 2)
	vishgraz := stampCreaturePT(addPerm(gs, 0, "Vishgraz, the Doomhive", "creature", "legendary"), 4, 4)

	gs.Seats[1].PoisonCounters = 2
	vishgrazTheDoomhiveETB(gs, vishgraz)
	if vishgraz.Power() != 6 {
		t.Fatalf("initial buff missing; power=%d", vishgraz.Power())
	}

	// Opponent poison drops to 0; recompute should drop the buff.
	gs.Seats[1].PoisonCounters = 0
	vishgrazRecomputeBuff(gs, vishgraz, map[string]interface{}{})

	if got := vishgraz.Power(); got != 4 {
		t.Errorf("after recompute with 0 opp poison, power should be 4, got %d", got)
	}
	for _, m := range vishgraz.Modifications {
		if m.Duration == vishgrazPoisonBuffTag {
			t.Errorf("residual poison-buff Modification after recompute: %+v", m)
		}
	}
}
