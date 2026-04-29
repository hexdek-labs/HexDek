package gameengine

// Tests for the 5 keywords flagged by 7174n1c:
//
//   1. Proliferate — CR §701.27
//   2. Populate    — CR §701.30
//   3. Evolve      — CR §702.100
//   4. Extort      — CR §702.101
//   5. Bargain     — CR §702.166

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ===========================================================================
// Test helpers (scoped to this file)
// ===========================================================================

func newFlaggedGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(2, rng, nil)
}

func newFlaggedGame4P(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(4, rng, nil)
}

func addFlaggedPerm(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         append([]string{}, types...),
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

func addFlaggedPermWithKeyword(gs *GameState, seat int, name string, pow, tough int, keyword string, types ...string) *Permanent {
	p := addFlaggedPerm(gs, seat, name, pow, tough, types...)
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: keyword},
		},
	}
	return p
}

func flaggedCountEvents(gs *GameState, kind string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

func flaggedCountTokens(gs *GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsToken() {
			n++
		}
	}
	return n
}

// ===========================================================================
// 1. PROLIFERATE — CR §701.27
// ===========================================================================

func TestProliferate_AddsCountersToOwnPermanents(t *testing.T) {
	gs := newFlaggedGame(t)

	// Creature with 2 +1/+1 counters.
	creature := addFlaggedPerm(gs, 0, "Managorger Hydra", 1, 1, "creature")
	creature.AddCounter("+1/+1", 2)

	// Planeswalker with 3 loyalty counters.
	pw := addFlaggedPerm(gs, 0, "Jace Beleren", 0, 0, "planeswalker")
	pw.AddCounter("loyalty", 3)

	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// +1/+1 counters should go from 2 -> 3.
	if creature.Counters["+1/+1"] != 3 {
		t.Errorf("expected 3 +1/+1 counters, got %d", creature.Counters["+1/+1"])
	}
	// Loyalty should go from 3 -> 4.
	if pw.Counters["loyalty"] != 4 {
		t.Errorf("expected 4 loyalty counters, got %d", pw.Counters["loyalty"])
	}
	if flaggedCountEvents(gs, "proliferate") != 1 {
		t.Error("expected 1 proliferate event")
	}
}

func TestProliferate_SkipsOpponentPlusCounters(t *testing.T) {
	gs := newFlaggedGame(t)

	// Opponent's creature with +1/+1 counters — should NOT be proliferated.
	oppCreature := addFlaggedPerm(gs, 1, "Opponent Bear", 2, 2, "creature")
	oppCreature.AddCounter("+1/+1", 2)

	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// Opponent's +1/+1 counters should stay at 2.
	if oppCreature.Counters["+1/+1"] != 2 {
		t.Errorf("opponent's +1/+1 counters should remain 2, got %d", oppCreature.Counters["+1/+1"])
	}
}

func TestProliferate_AddsOpponentPoisonCounters(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[1].PoisonCounters = 5

	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// Opponent should go from 5 -> 6 poison counters.
	if gs.Seats[1].PoisonCounters != 6 {
		t.Errorf("expected 6 poison counters on opponent, got %d", gs.Seats[1].PoisonCounters)
	}
}

func TestProliferate_DoesNotAddSelfPoison(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[0].PoisonCounters = 3

	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// Self poison counters should stay at 3.
	if gs.Seats[0].PoisonCounters != 3 {
		t.Errorf("self poison counters should remain 3, got %d", gs.Seats[0].PoisonCounters)
	}
}

func TestProliferate_MultipleCounterTypes(t *testing.T) {
	gs := newFlaggedGame(t)

	// Permanent with both charge and +1/+1 counters.
	perm := addFlaggedPerm(gs, 0, "Astral Cornucopia", 0, 0, "artifact")
	perm.AddCounter("charge", 3)
	perm.AddCounter("+1/+1", 1)

	src := addFlaggedPerm(gs, 0, "Atraxa", 4, 4, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// Both counter types should increase by 1.
	if perm.Counters["charge"] != 4 {
		t.Errorf("expected 4 charge counters, got %d", perm.Counters["charge"])
	}
	if perm.Counters["+1/+1"] != 2 {
		t.Errorf("expected 2 +1/+1 counters, got %d", perm.Counters["+1/+1"])
	}
}

func TestProliferate_NothingToProliferate(t *testing.T) {
	gs := newFlaggedGame(t)

	// No permanents with counters, no poison.
	addFlaggedPerm(gs, 0, "Bear", 2, 2, "creature")
	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// Should still log the event with amount=0.
	if flaggedCountEvents(gs, "proliferate") != 1 {
		t.Error("expected 1 proliferate event even with nothing to proliferate")
	}
}

func TestProliferate_OpponentMinusCountersProliferated(t *testing.T) {
	gs := newFlaggedGame(t)

	// Opponent's creature with -1/-1 counters — SHOULD be proliferated
	// (hurts opponent).
	oppCreature := addFlaggedPerm(gs, 1, "Withered Bear", 2, 2, "creature")
	oppCreature.AddCounter("-1/-1", 1)

	src := addFlaggedPerm(gs, 0, "Thrummingbird", 1, 1, "creature")

	e := &gameast.ModificationEffect{ModKind: "proliferate"}
	resolveModificationEffect(gs, src, e)

	// -1/-1 should go from 1 -> 2.
	if oppCreature.Counters["-1/-1"] != 2 {
		t.Errorf("opponent -1/-1 counters should be 2, got %d", oppCreature.Counters["-1/-1"])
	}
}

// ===========================================================================
// 2. POPULATE — CR §701.30
// ===========================================================================

func TestPopulate_CopiesCreatureToken(t *testing.T) {
	gs := newFlaggedGame(t)

	// Create a creature token on the battlefield.
	tokenCard := &Card{
		Name:          "Beast Token",
		Owner:         0,
		BasePower:     3,
		BaseToughness: 3,
		Types:         []string{"token", "creature"},
		Colors:        []string{"G"},
	}
	tokenPerm := &Permanent{
		Card:          tokenCard,
		Controller:    0,
		Owner:         0,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, tokenPerm)

	src := addFlaggedPerm(gs, 0, "Rootborn Defenses", 0, 0, "instant")

	initialTokens := flaggedCountTokens(gs, 0)

	e := &gameast.ModificationEffect{ModKind: "populate"}
	resolveModificationEffect(gs, src, e)

	// Should have one more token.
	newTokens := flaggedCountTokens(gs, 0)
	if newTokens != initialTokens+1 {
		t.Errorf("expected %d tokens after populate, got %d", initialTokens+1, newTokens)
	}

	if flaggedCountEvents(gs, "populate") != 1 {
		t.Error("expected 1 populate event")
	}
	if flaggedCountEvents(gs, "create_token") != 1 {
		t.Error("expected 1 create_token event from populate")
	}
}

func TestPopulate_NoCreatureToken(t *testing.T) {
	gs := newFlaggedGame(t)

	// Only have a non-token creature — populate should do nothing.
	addFlaggedPerm(gs, 0, "Bear", 2, 2, "creature")
	src := addFlaggedPerm(gs, 0, "Rootborn Defenses", 0, 0, "instant")

	e := &gameast.ModificationEffect{ModKind: "populate"}
	resolveModificationEffect(gs, src, e)

	if flaggedCountEvents(gs, "create_token") != 0 {
		t.Error("should not create a token when no creature token exists")
	}
	if flaggedCountEvents(gs, "populate") != 1 {
		t.Error("populate event should still be logged")
	}
}

func TestPopulate_PicksStrongestToken(t *testing.T) {
	gs := newFlaggedGame(t)

	// 1/1 token.
	smallToken := &Card{
		Name: "Soldier Token", Owner: 0,
		BasePower: 1, BaseToughness: 1,
		Types: []string{"token", "creature"},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: smallToken, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	})

	// 5/5 token.
	bigToken := &Card{
		Name: "Wurm Token", Owner: 0,
		BasePower: 5, BaseToughness: 5,
		Types: []string{"token", "creature"},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: bigToken, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	})

	src := addFlaggedPerm(gs, 0, "Trostani", 2, 5, "creature")

	e := &gameast.ModificationEffect{ModKind: "populate"}
	resolveModificationEffect(gs, src, e)

	// The new token should be a copy of the 5/5 Wurm (strongest).
	found := false
	wurmCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card.Name == "Wurm Token" {
			wurmCount++
		}
	}
	if wurmCount == 2 {
		found = true
	}
	if !found {
		t.Error("populate should copy the strongest creature token (Wurm Token)")
	}
}

func TestPopulate_NonCreatureTokenIgnored(t *testing.T) {
	gs := newFlaggedGame(t)

	// Artifact token (Treasure) — not a creature token.
	CreateTreasureToken(gs, 0)
	src := addFlaggedPerm(gs, 0, "Rootborn Defenses", 0, 0, "instant")

	initialTokens := flaggedCountTokens(gs, 0)

	e := &gameast.ModificationEffect{ModKind: "populate"}
	resolveModificationEffect(gs, src, e)

	// Should not create any new tokens — Treasure isn't a creature token.
	newTokens := flaggedCountTokens(gs, 0)
	if newTokens != initialTokens {
		t.Errorf("should not populate non-creature tokens, expected %d, got %d",
			initialTokens, newTokens)
	}
}

// ===========================================================================
// 3. EVOLVE — CR §702.100
// ===========================================================================

func TestEvolve_TriggersByPower(t *testing.T) {
	gs := newFlaggedGame(t)

	// 1/1 evolve creature.
	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 1, 1,
		"evolve", "creature")

	// 3/1 creature enters — higher power triggers evolve.
	newCreature := addFlaggedPerm(gs, 0, "Lightning Mauler", 3, 1, "creature")

	FireEvolveTriggers(gs, 0, newCreature)

	if evolveCreature.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter from evolve, got %d",
			evolveCreature.Counters["+1/+1"])
	}
	if flaggedCountEvents(gs, "evolve_trigger") != 1 {
		t.Error("expected 1 evolve_trigger event")
	}
}

func TestEvolve_TriggersByToughness(t *testing.T) {
	gs := newFlaggedGame(t)

	// 2/2 evolve creature.
	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 2, 2,
		"evolve", "creature")

	// 1/4 creature enters — lower power but higher toughness triggers evolve.
	newCreature := addFlaggedPerm(gs, 0, "Wall of Omens", 1, 4, "creature")

	FireEvolveTriggers(gs, 0, newCreature)

	if evolveCreature.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 +1/+1 counter from evolve by toughness, got %d",
			evolveCreature.Counters["+1/+1"])
	}
}

func TestEvolve_DoesNotTriggerIfSmaller(t *testing.T) {
	gs := newFlaggedGame(t)

	// 3/3 evolve creature.
	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 3, 3,
		"evolve", "creature")

	// 2/2 creature enters — both power and toughness are smaller.
	newCreature := addFlaggedPerm(gs, 0, "Bear", 2, 2, "creature")

	FireEvolveTriggers(gs, 0, newCreature)

	if evolveCreature.Counters["+1/+1"] != 0 {
		t.Errorf("evolve should not trigger for smaller creature, got %d counters",
			evolveCreature.Counters["+1/+1"])
	}
	if flaggedCountEvents(gs, "evolve_trigger") != 0 {
		t.Error("no evolve_trigger event should fire for smaller creature")
	}
}

func TestEvolve_DoesNotTriggerForSelf(t *testing.T) {
	gs := newFlaggedGame(t)

	// Evolve creature enters — should not trigger on itself.
	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 1, 1,
		"evolve", "creature")

	FireEvolveTriggers(gs, 0, evolveCreature)

	if evolveCreature.Counters["+1/+1"] != 0 {
		t.Errorf("evolve should not trigger for itself, got %d counters",
			evolveCreature.Counters["+1/+1"])
	}
}

func TestEvolve_MultipleEvolveCreatures(t *testing.T) {
	gs := newFlaggedGame(t)

	// Two evolve creatures with different stats.
	evolve1 := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 1, 1,
		"evolve", "creature")
	evolve2 := addFlaggedPermWithKeyword(gs, 0, "Cloudfin Raptor", 0, 1,
		"evolve", "creature")

	// 2/3 creature enters — should trigger both.
	newCreature := addFlaggedPerm(gs, 0, "Loxodon Smiter", 2, 3, "creature")

	FireEvolveTriggers(gs, 0, newCreature)

	if evolve1.Counters["+1/+1"] != 1 {
		t.Errorf("evolve1 should have 1 counter, got %d", evolve1.Counters["+1/+1"])
	}
	if evolve2.Counters["+1/+1"] != 1 {
		t.Errorf("evolve2 should have 1 counter, got %d", evolve2.Counters["+1/+1"])
	}
	if flaggedCountEvents(gs, "evolve_trigger") != 2 {
		t.Errorf("expected 2 evolve_trigger events, got %d",
			flaggedCountEvents(gs, "evolve_trigger"))
	}
}

func TestEvolve_DoesNotTriggerForOpponentCreature(t *testing.T) {
	gs := newFlaggedGame(t)

	// Evolve creature on seat 0.
	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 1, 1,
		"evolve", "creature")

	// Creature enters on seat 1 (opponent) — evolve should NOT trigger.
	oppCreature := addFlaggedPerm(gs, 1, "Enormous Baloth", 7, 7, "creature")

	FireEvolveTriggers(gs, 1, oppCreature)

	if evolveCreature.Counters["+1/+1"] != 0 {
		t.Errorf("evolve should not trigger for opponent's creature, got %d",
			evolveCreature.Counters["+1/+1"])
	}
}

func TestEvolve_NonCreatureDoesNotTrigger(t *testing.T) {
	gs := newFlaggedGame(t)

	evolveCreature := addFlaggedPermWithKeyword(gs, 0, "Experiment One", 1, 1,
		"evolve", "creature")

	// Non-creature enters — evolve should not trigger.
	nonCreature := addFlaggedPerm(gs, 0, "Sol Ring", 0, 0, "artifact")

	FireEvolveTriggers(gs, 0, nonCreature)

	if evolveCreature.Counters["+1/+1"] != 0 {
		t.Errorf("evolve should not trigger for non-creature, got %d",
			evolveCreature.Counters["+1/+1"])
	}
}

// ===========================================================================
// 4. EXTORT — CR §702.101
// ===========================================================================

func TestExtort_DrainsOpponentGainsLife(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Seats[0].ManaPool = 5

	// Permanent with extort keyword on seat 0.
	addFlaggedPermWithKeyword(gs, 0, "Blind Obedience", 0, 0,
		"extort", "enchantment")

	// Cast a spell.
	spell := &Card{Name: "Lightning Bolt", Types: []string{"instant"}}

	FireCastTriggerObservers(gs, spell, 0, false)

	// Extort should drain 1 from opponent, gain 1 for controller.
	if gs.Seats[1].Life != 19 {
		t.Errorf("opponent should lose 1 life from extort, got life=%d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 21 {
		t.Errorf("controller should gain 1 life from extort, got life=%d", gs.Seats[0].Life)
	}
	// Should cost 1 mana.
	if gs.Seats[0].ManaPool != 4 {
		t.Errorf("expected 4 mana remaining after extort payment, got %d", gs.Seats[0].ManaPool)
	}
	if flaggedCountEvents(gs, "extort_trigger") != 1 {
		t.Error("expected 1 extort_trigger event")
	}
}

func TestExtort_NoPayIfNoMana(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Seats[0].ManaPool = 0 // No mana to pay

	addFlaggedPermWithKeyword(gs, 0, "Blind Obedience", 0, 0,
		"extort", "enchantment")

	spell := &Card{Name: "Lightning Bolt", Types: []string{"instant"}}

	FireCastTriggerObservers(gs, spell, 0, false)

	// No mana to pay — extort should not trigger.
	if gs.Seats[1].Life != 20 {
		t.Errorf("opponent life should remain 20 with no mana for extort, got %d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 20 {
		t.Errorf("controller life should remain 20, got %d", gs.Seats[0].Life)
	}
	if flaggedCountEvents(gs, "extort_trigger") != 0 {
		t.Error("no extort_trigger event should fire when no mana available")
	}
}

func TestExtort_MultiplePermanentsTriggerSeparately(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Seats[0].ManaPool = 5

	// Two permanents with extort.
	addFlaggedPermWithKeyword(gs, 0, "Blind Obedience", 0, 0,
		"extort", "enchantment")
	addFlaggedPermWithKeyword(gs, 0, "Crypt Ghast", 2, 2,
		"extort", "creature")

	spell := &Card{Name: "Doom Blade", Types: []string{"instant"}}

	FireCastTriggerObservers(gs, spell, 0, false)

	// 2 extort triggers, each drains 1 from opponent.
	if gs.Seats[1].Life != 18 {
		t.Errorf("opponent should lose 2 life from 2 extort triggers, got life=%d", gs.Seats[1].Life)
	}
	if gs.Seats[0].Life != 22 {
		t.Errorf("controller should gain 2 life from 2 extort triggers, got life=%d", gs.Seats[0].Life)
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Errorf("expected 3 mana remaining after 2 extort payments, got %d", gs.Seats[0].ManaPool)
	}
	if flaggedCountEvents(gs, "extort_trigger") != 2 {
		t.Errorf("expected 2 extort_trigger events, got %d",
			flaggedCountEvents(gs, "extort_trigger"))
	}
}

func TestExtort_4PlayerDrainsAllOpponents(t *testing.T) {
	gs := newFlaggedGame4P(t)
	for i := 0; i < 4; i++ {
		gs.Seats[i].Life = 40
	}
	gs.Seats[0].ManaPool = 5

	addFlaggedPermWithKeyword(gs, 0, "Blind Obedience", 0, 0,
		"extort", "enchantment")

	spell := &Card{Name: "Path to Exile", Types: []string{"instant"}}

	FireCastTriggerObservers(gs, spell, 0, false)

	// Each of 3 opponents loses 1 life. Controller gains 3.
	for i := 1; i < 4; i++ {
		if gs.Seats[i].Life != 39 {
			t.Errorf("seat %d should lose 1 life from extort, got %d", i, gs.Seats[i].Life)
		}
	}
	if gs.Seats[0].Life != 43 {
		t.Errorf("controller should gain 3 life in 4-player extort, got %d", gs.Seats[0].Life)
	}
}

func TestExtort_DoesNotFireFromCopies(t *testing.T) {
	gs := newFlaggedGame(t)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20
	gs.Seats[0].ManaPool = 5

	addFlaggedPermWithKeyword(gs, 0, "Blind Obedience", 0, 0,
		"extort", "enchantment")

	spell := &Card{Name: "Lightning Bolt", Types: []string{"instant"}}

	// Copies should not trigger cast observers (fromCopy=true).
	FireCastTriggerObservers(gs, spell, 0, true)

	if gs.Seats[1].Life != 20 {
		t.Error("copies should not trigger extort")
	}
}

// ===========================================================================
// 5. BARGAIN — CR §702.166
// ===========================================================================

func TestBargain_FindsCandidateArtifact(t *testing.T) {
	gs := newFlaggedGame(t)

	addFlaggedPerm(gs, 0, "Sol Ring", 0, 0, "artifact")

	candidate := findBargainCandidate(gs, gs.Seats[0])
	if candidate == nil {
		t.Fatal("expected to find an artifact as bargain candidate")
	}
	if candidate.Card.Name != "Sol Ring" {
		t.Errorf("expected Sol Ring, got %s", candidate.Card.Name)
	}
}

func TestBargain_FindsCandidateEnchantment(t *testing.T) {
	gs := newFlaggedGame(t)

	addFlaggedPerm(gs, 0, "Rhystic Study", 0, 0, "enchantment")

	candidate := findBargainCandidate(gs, gs.Seats[0])
	if candidate == nil {
		t.Fatal("expected to find an enchantment as bargain candidate")
	}
}

func TestBargain_FindsCandidateToken(t *testing.T) {
	gs := newFlaggedGame(t)

	// Only a creature token — should be eligible.
	createSimpleCreatureToken(gs, 0, "Soldier Token", 1, 1, []string{"W"})

	candidate := findBargainCandidate(gs, gs.Seats[0])
	if candidate == nil {
		t.Fatal("expected to find a token as bargain candidate")
	}
}

func TestBargain_NoCandidateOnlyCreature(t *testing.T) {
	gs := newFlaggedGame(t)

	// Only a non-token creature — not eligible for bargain.
	addFlaggedPerm(gs, 0, "Bear", 2, 2, "creature")

	candidate := findBargainCandidate(gs, gs.Seats[0])
	if candidate != nil {
		t.Error("plain creature should not be a bargain candidate")
	}
}

func TestBargain_PicksCheapest(t *testing.T) {
	gs := newFlaggedGame(t)

	expensive := addFlaggedPerm(gs, 0, "Expensive Artifact", 0, 0, "artifact")
	expensive.Card.CMC = 5

	cheap := addFlaggedPerm(gs, 0, "Cheap Artifact", 0, 0, "artifact")
	cheap.Card.CMC = 1

	candidate := findBargainCandidate(gs, gs.Seats[0])
	if candidate == nil {
		t.Fatal("expected a bargain candidate")
	}
	if candidate.Card.Name != "Cheap Artifact" {
		t.Errorf("GreedyHat should pick cheapest: expected Cheap Artifact, got %s",
			candidate.Card.Name)
	}
}

func TestBargain_AdditionalCostPayment(t *testing.T) {
	gs := newFlaggedGame(t)

	addFlaggedPerm(gs, 0, "Treasure Token", 0, 0, "token", "artifact")

	bargain := BargainAdditionalCost()
	card := &Card{Name: "Beseech the Mirror", Owner: 0, CMC: 4, Types: []string{"sorcery"}}

	if !CanPayAdditionalCost(gs, 0, bargain) {
		t.Fatal("should be able to pay bargain with a token artifact")
	}

	result := PayAdditionalCost(gs, 0, card, bargain)
	if result == nil {
		t.Fatal("bargain payment should succeed")
	}
	if len(result.SacrificedPermanents) != 1 {
		t.Errorf("expected 1 sacrificed permanent, got %d", len(result.SacrificedPermanents))
	}
	if flaggedCountEvents(gs, "bargain_paid") != 1 {
		t.Error("expected 1 bargain_paid event")
	}
}

func TestBargain_CannotPayWithNoCandidates(t *testing.T) {
	gs := newFlaggedGame(t)

	// Only a non-token creature — no eligible bargain targets.
	addFlaggedPerm(gs, 0, "Bear", 2, 2, "creature")

	bargain := BargainAdditionalCost()

	if CanPayAdditionalCost(gs, 0, bargain) {
		t.Error("should not be able to pay bargain with only a plain creature")
	}
}
