package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDetectConditionScaffold_Era1R60 pins detection for the 19 new
// pre-Modern scaffolds added in the R60 era-1 sweep. Each case mirrors a
// real card phrase from scripts/era1_scaffold_audit.py's unbucketed
// fragments table; subtype/count expectations match what the detector
// should lift from the raw text.
func TestDetectConditionScaffold_Era1R60(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		text     string
		wantKind conditionScaffoldKind
		wantSub  string
		wantCnt  int
	}{
		{
			name:     "tribute not paid raw",
			kind:     "raw",
			text:     "when ~ enters the battlefield, if tribute wasn't paid, draw a card",
			wantKind: condScaffoldTributeNotPaid,
		},
		{
			name:     "tribute structured kind",
			kind:     "tribute_not_paid",
			text:     "",
			wantKind: condScaffoldTributeNotPaid,
		},
		{
			name:     "bargained raw",
			kind:     "raw",
			text:     "when ~ enters, if it was bargained, deal 3 damage",
			wantKind: condScaffoldBargained,
		},
		{
			name:     "shares creature type raw",
			kind:     "raw",
			text:     "if it shares a creature type with this creature, you may put it onto the battlefield",
			wantKind: condScaffoldSharesCreatureType,
		},
		{
			name:     "not their turn raw",
			kind:     "raw",
			text:     "this triggers only if it's not their turn",
			wantKind: condScaffoldNotTheirTurn,
		},
		{
			name:     "more life than opponent raw",
			kind:     "raw",
			text:     "if you have more life than an opponent, draw a card",
			wantKind: condScaffoldMoreLifeThanOpponent,
		},
		{
			name:     "opponent more life raw",
			kind:     "raw",
			text:     "if an opponent has more life than you, gain 5 life",
			wantKind: condScaffoldOpponentMoreLife,
		},
		{
			name:     "time counter on self raw",
			kind:     "raw",
			text:     "if it had no time counters on it, exile it",
			wantKind: condScaffoldTimeCounterOnSelf,
		},
		{
			name:     "self is suspended raw",
			kind:     "raw",
			text:     "as long as this card is suspended, it has flying",
			wantKind: condScaffoldSelfIsSuspended,
		},
		{
			name:     "wasnt cast raw",
			kind:     "raw",
			text:     "if it wasn't cast, return it to its owner's hand",
			wantKind: condScaffoldWasntCast,
		},
		{
			name:     "wasnt blocking raw",
			kind:     "raw",
			text:     "if it wasn't blocking, it deals 2 damage",
			wantKind: condScaffoldWasntBlocking,
		},
		{
			name:     "surge cost paid raw",
			kind:     "raw",
			text:     "if its surge cost was paid, draw two cards",
			wantKind: condScaffoldSurgeCostPaid,
		},
		{
			name:     "library empty raw",
			kind:     "raw",
			text:     "if your library has no cards in it, you win the game",
			wantKind: condScaffoldLibraryEmpty,
		},
		{
			name:     "colored mana spent RR raw",
			kind:     "raw",
			text:     "if {r}{r} was spent to cast it, deals 4 damage instead",
			wantKind: condScaffoldColoredManaSpent,
			wantSub:  "RR",
			wantCnt:  2,
		},
		{
			name:     "colored mana spent GG raw",
			kind:     "raw",
			text:     "if {g}{g} was spent to cast it, put two +1/+1 counters",
			wantKind: condScaffoldColoredManaSpent,
			wantSub:  "GG",
			wantCnt:  2,
		},
		{
			name:     "colored mana spent single R raw",
			kind:     "raw",
			text:     "if {r} was spent to cast it, it gets +1/+0",
			wantKind: condScaffoldColoredManaSpent,
			wantSub:  "R",
			wantCnt:  1,
		},
		{
			name:     "mana from treasure spent raw",
			kind:     "raw",
			text:     "if mana from a treasure was spent to cast it, gain 2 life",
			wantKind: condScaffoldColoredManaSpent,
			wantSub:  "treasure",
		},
		{
			name:     "lost life last turn raw",
			kind:     "raw",
			text:     "if you lost life last turn, draw a card",
			wantKind: condScaffoldLostLifeLastTurn,
		},
		{
			name:     "damage dealt to self this turn raw",
			kind:     "raw",
			text:     "if 4 or more damage was dealt to it this turn, it deals 4 damage to its controller",
			wantKind: condScaffoldDamageDealtToSelfTurn,
		},
		{
			name:     "more cards in hand than each opponent raw",
			kind:     "raw",
			text:     "if you have more cards in hand than each opponent, draw two cards",
			wantKind: condScaffoldMoreCardsThanOpponents,
		},
		{
			name:     "self is untapped raw",
			kind:     "raw",
			text:     "as long as this creature is untapped, prevent all damage",
			wantKind: condScaffoldSelfIsUntapped,
		},
		{
			name:     "self has no oil counter raw",
			kind:     "raw",
			text:     "as long as it has no oil counters on it, this creature has trample",
			wantKind: condScaffoldSelfHasNoCounter,
			wantSub:  "oil",
		},
		{
			name:     "life above starting raw",
			kind:     "raw",
			text:     "as long as your life total is greater than your starting life total, this creature has flying",
			wantKind: condScaffoldLifeAboveStarting,
		},

		// Era 1 r60 era1-scaffold-sweep — detection cases for the 19 new
		// raw-text patterns landed in this sweep.
		{
			name: "past-tense self power threshold",
			kind: "raw",
			text: "if its power was 3 or greater, draw a card",
			wantKind: condScaffoldSelfPowerGE,
			wantCnt: 3,
		},
		{
			name: "toughness threshold",
			kind: "raw",
			text: "if that creature has toughness 6 or greater, destroy it",
			wantKind: condScaffoldSelfPowerGE,
			wantSub: "toughness",
			wantCnt: 6,
		},
		{
			name: "typed dread counter threshold",
			kind: "raw",
			text: "if there are three or more dread counters on it, sacrifice it",
			wantKind: condScaffoldCountersOnSelfGE,
			wantSub:  "dread",
			wantCnt:  3,
		},
		{
			name: "counter parity odd",
			kind: "raw",
			text: "as long as ~ has an odd number of counters, draw a card",
			wantKind: condScaffoldSelfHasCounter,
			wantSub:  "any_parity",
			wantCnt:  1,
		},
		{
			name: "counter negation indestructible",
			kind: "raw",
			text: "if it doesn't have an indestructible counter on it, sacrifice it",
			wantKind: condScaffoldSelfHasNoCounter,
			wantSub:  "indestructible",
		},
		{
			name: "no arrowhead counters on artifact",
			kind: "raw",
			text: "if there are no arrowhead counters on this artifact, sacrifice it",
			wantKind: condScaffoldSelfHasNoCounter,
			wantSub:  "arrowhead",
		},
		{
			name: "past-tense typed counter -1/-1",
			kind: "raw",
			text: "when this creature dies, if it had a -1/-1 counter on it, return it",
			wantKind: condScaffoldHadCountersOnIt,
			wantSub:  "-1/-1",
		},
		{
			name: "life less than N",
			kind: "raw",
			text: "as long as your life total is less than 7, prevent all damage",
			wantKind: condScaffoldLifeBelowThreshold,
		},
		{
			name: "opponent has 10 or less life",
			kind: "raw",
			text: "as long as an opponent has 10 or less life, this creature gets +2/+1",
			wantKind: condScaffoldLifeBelowThreshold,
			wantSub:  "opponent",
		},
		{
			name: "defending player more cards",
			kind: "raw",
			text: "if defending player has more cards in hand than you, this creature gets +2/+0",
			wantKind: condScaffoldMoreCardsThanOpponents,
			wantSub:  "opponent_more",
		},
		{
			name: "creature has madness",
			kind: "raw",
			text: "as long as it has madness, this creature has haste",
			wantKind: condScaffoldCreatureHasKeyword,
			wantSub:  "madness",
		},
		{
			name: "creature has first strike",
			kind: "raw",
			text: "if it has first strike, draw a card",
			wantKind: condScaffoldCreatureHasKeyword,
			wantSub:  "first_strike",
		},
		{
			name: "isn't a token",
			kind: "raw",
			text: "when ~ dies, if it isn't a token, return it to its owner's hand",
			wantKind: condScaffoldIsToken,
			wantSub:  "non_token",
		},
		{
			name: "this card is exiled",
			kind: "raw",
			text: "as long as this card is exiled, you may play it",
			wantKind: condScaffoldSelfInZone,
			wantSub:  "exile",
		},
		{
			name: "at the top of your library",
			kind: "raw",
			text: "as long as ~ is at the top of your library, you may look at it",
			wantKind: condScaffoldSelfInZone,
			wantSub:  "library_top",
		},
		{
			name: "self is enchanted krond",
			kind: "raw",
			text: "whenever ~ attacks, if it's enchanted, destroy target nonland permanent",
			wantKind: condScaffoldSelfIsEnchanted,
		},
		{
			name: "self is suspected",
			kind: "raw",
			text: "as long as this creature is suspected, deal 1 damage to each opponent",
			wantKind: condScaffoldSelfIsSuspected,
		},
		{
			name: "no land played this turn",
			kind: "raw",
			text: "if you didn't play a land this turn, you may search your library",
			wantKind: condScaffoldNoLandPlayedThisTurn,
		},
		{
			name: "typed creature died this turn",
			kind: "raw",
			text: "if another human died under your control this turn, this creature gets +1/+1",
			wantKind: condScaffoldCreatureDiedThisTurn,
			wantSub:  "typed",
		},
		{
			name: "sneak cost paid",
			kind: "raw",
			text: "if his sneak cost was paid, ~ can't be blocked this turn",
			wantKind: condScaffoldPaidOptionalCost,
			wantCnt:  1,
		},
		{
			name: "mana from creatures spent",
			kind: "raw",
			text: "if three or more mana from creatures was spent to cast it, draw three cards",
			wantKind: condScaffoldManaSpentThreshold,
			wantSub:  "from_creatures",
			wantCnt:  3,
		},
		{
			name: "noncreature spell count this turn",
			kind: "raw",
			text: "as long as you've cast two or more noncreature spells this turn, ~ has double strike",
			wantKind: condScaffoldCastNSpellsThisTurn,
			wantSub:  "noncreature",
			wantCnt:  2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond := &gameast.Condition{Kind: tc.kind, Args: []interface{}{tc.text}}
			got := detectConditionScaffold(cond)
			if got.kind != tc.wantKind {
				t.Fatalf("kind: want %v, got %v (raw=%q)", tc.wantKind, got.kind, got.rawText)
			}
			if tc.wantSub != "" && got.subtype != tc.wantSub {
				t.Errorf("subtype: want %q, got %q", tc.wantSub, got.subtype)
			}
			if tc.wantCnt != 0 && got.count != tc.wantCnt {
				t.Errorf("count: want %d, got %d", tc.wantCnt, got.count)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60 verifies that each new apply
// implementation mutates engine state in the expected direction so a
// downstream condition reader can resolve the predicate to true.

func TestApplyConditionScaffolding_Era1R60_TributeNotPaid(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Pharagax Giant", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if tribute wasn't paid, deal 5 damage"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldTributeNotPaid {
		t.Fatalf("expected TributeNotPaid, got %v", cs.kind)
	}
	if src.Flags["tribute_declined"] != 1 {
		t.Errorf("tribute_declined want 1, got %d", src.Flags["tribute_declined"])
	}
	if src.Flags["tribute_paid"] != 0 {
		t.Errorf("tribute_paid want 0, got %d", src.Flags["tribute_paid"])
	}
}

func TestApplyConditionScaffolding_Era1R60_Bargained(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Realm-Scorcher Hellkite", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it was bargained, deal 3 damage to any target"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldBargained {
		t.Fatalf("expected Bargained, got %v", cs.kind)
	}
	if src.Flags["bargained"] != 1 || src.Flags["paid_optional_cost"] != 1 {
		t.Errorf("bargained/paid_optional_cost flags not set: %v", src.Flags)
	}
}

func TestApplyConditionScaffolding_Era1R60_NotTheirTurn(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"this triggers only if it's not their turn"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldNotTheirTurn {
		t.Fatalf("expected NotTheirTurn, got %v", cs.kind)
	}
	if gs.Active != 1 {
		t.Errorf("gs.Active want 1, got %d", gs.Active)
	}
	if gs.Flags["off_turn"] != 1 {
		t.Errorf("off_turn flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_MoreLifeThanOpponent(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you have more life than an opponent"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldMoreLifeThanOpponent {
		t.Fatalf("expected MoreLifeThanOpponent, got %v", cs.kind)
	}
	if gs.Seats[0].Life <= gs.Seats[1].Life {
		t.Errorf("seat 0 life (%d) must exceed seat 1 life (%d)", gs.Seats[0].Life, gs.Seats[1].Life)
	}
}

func TestApplyConditionScaffolding_Era1R60_OpponentMoreLife(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if an opponent has more life than you, gain 5 life"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldOpponentMoreLife {
		t.Fatalf("expected OpponentMoreLife, got %v", cs.kind)
	}
	if gs.Seats[1].Life <= gs.Seats[0].Life {
		t.Errorf("seat 1 life (%d) must exceed seat 0 life (%d)", gs.Seats[1].Life, gs.Seats[0].Life)
	}
}

func TestApplyConditionScaffolding_Era1R60_LibraryEmpty(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Library = []*gameengine.Card{
		{Name: "filler 1", Owner: 0, Types: []string{"creature"}},
		{Name: "filler 2", Owner: 0, Types: []string{"creature"}},
	}
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if your library has no cards in it, you win the game"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldLibraryEmpty {
		t.Fatalf("expected LibraryEmpty, got %v", cs.kind)
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("seat 0 library should be empty, got %d cards", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Flags["library_empty"] != 1 {
		t.Errorf("library_empty flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_ColoredManaSpent_RR(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Vibrance", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if {r}{r} was spent to cast it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldColoredManaSpent {
		t.Fatalf("expected ColoredManaSpent, got %v", cs.kind)
	}
	if src.Flags["mana_spent_RR"] != 2 {
		t.Errorf("mana_spent_RR want 2, got %d", src.Flags["mana_spent_RR"])
	}
	if src.Flags["mana_spent"] < 2 {
		t.Errorf("generic mana_spent want >=2, got %d", src.Flags["mana_spent"])
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfIsUntapped(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Sphinx Sovereign", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Tapped:     true,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as this creature is untapped"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfIsUntapped {
		t.Fatalf("expected SelfIsUntapped, got %v", cs.kind)
	}
	if src.Tapped {
		t.Errorf("srcPerm should be untapped")
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfHasNoCounter(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Evolved Spinoderm", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{"oil": 3},
		Flags:    map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as it has no oil counters on it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfHasNoCounter {
		t.Fatalf("expected SelfHasNoCounter, got %v", cs.kind)
	}
	if src.Counters["oil"] != 0 {
		t.Errorf("oil counters should be cleared, got %d", src.Counters["oil"])
	}
	if src.Flags["no_oil_counters"] != 1 {
		t.Errorf("no_oil_counters flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_LifeAboveStarting(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as your life total is greater than your starting life total"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldLifeAboveStarting {
		t.Fatalf("expected LifeAboveStarting, got %v", cs.kind)
	}
	if gs.Seats[0].Life < 41 {
		t.Errorf("seat 0 life want >=41 (above Commander starting), got %d", gs.Seats[0].Life)
	}
}

func TestApplyConditionScaffolding_Era1R60_DamageDealtToSelfTurn(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Burning-Eye Zubera", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if 4 or more damage was dealt to it this turn"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldDamageDealtToSelfTurn {
		t.Fatalf("expected DamageDealtToSelfTurn, got %v", cs.kind)
	}
	if src.MarkedDamage < 4 {
		t.Errorf("MarkedDamage want >=4, got %d", src.MarkedDamage)
	}
}

func TestApplyConditionScaffolding_Era1R60_MoreCardsThanOpponents(t *testing.T) {
	gs := newTestGameState(3)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you have more cards in hand than each opponent"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldMoreCardsThanOpponents {
		t.Fatalf("expected MoreCardsThanOpponents, got %v", cs.kind)
	}
	if len(gs.Seats[0].Hand) <= len(gs.Seats[1].Hand) {
		t.Errorf("seat 0 hand (%d) must exceed seat 1 hand (%d)",
			len(gs.Seats[0].Hand), len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[0].Hand) <= len(gs.Seats[2].Hand) {
		t.Errorf("seat 0 hand (%d) must exceed seat 2 hand (%d)",
			len(gs.Seats[0].Hand), len(gs.Seats[2].Hand))
	}
}

func TestApplyConditionScaffolding_Era1R60_SurgeCostPaid(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Reckless Bushwhacker", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if its surge cost was paid"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSurgeCostPaid {
		t.Fatalf("expected SurgeCostPaid, got %v", cs.kind)
	}
	if src.Flags["surge_paid"] != 1 || src.Flags["paid_optional_cost"] != 1 {
		t.Errorf("surge_paid + paid_optional_cost not set: %v", src.Flags)
	}
}

func TestApplyConditionScaffolding_Era1R60_TimeCounterOnSelf(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Chronozoa", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Counters: map[string]int{"time": 1},
		Flags:    map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it had no time counters on it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldTimeCounterOnSelf {
		t.Fatalf("expected TimeCounterOnSelf, got %v", cs.kind)
	}
	if src.Counters["time"] != 0 {
		t.Errorf("time counters should be cleared, got %d", src.Counters["time"])
	}
}

func TestApplyConditionScaffolding_Era1R60_WasntCast(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Rapid Augmenter", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{"cast_from_hand": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it wasn't cast, return it to your hand"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldWasntCast {
		t.Fatalf("expected WasntCast, got %v", cs.kind)
	}
	if src.Flags["wasnt_cast"] != 1 {
		t.Errorf("wasnt_cast flag not set")
	}
	if _, ok := src.Flags["cast_from_hand"]; ok {
		t.Errorf("cast_from_hand should be cleared")
	}
}

func TestApplyConditionScaffolding_Era1R60_WasntBlocking(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Guildsworn Prowler", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{"blocking": 1, "is_blocking": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it wasn't blocking, deal 2 damage"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldWasntBlocking {
		t.Fatalf("expected WasntBlocking, got %v", cs.kind)
	}
	if _, ok := src.Flags["blocking"]; ok {
		t.Errorf("blocking flag should be cleared")
	}
	if src.Flags["wasnt_blocking"] != 1 {
		t.Errorf("wasnt_blocking flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfIsSuspended(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Nihilith", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags:    map[string]int{},
		Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as this card is suspended"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfIsSuspended {
		t.Fatalf("expected SelfIsSuspended, got %v", cs.kind)
	}
	if src.Flags["suspended"] != 1 {
		t.Errorf("suspended flag not set")
	}
	if src.Counters["time"] < 1 {
		t.Errorf("time counters not stamped, got %d", src.Counters["time"])
	}
	if len(gs.Seats[0].Exile) == 0 {
		t.Errorf("seat 0 exile should contain a suspended card placeholder")
	}
}

func TestApplyConditionScaffolding_Era1R60_LostLifeLastTurn(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you lost life last turn"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldLostLifeLastTurn {
		t.Fatalf("expected LostLifeLastTurn, got %v", cs.kind)
	}
	if gs.Seats[0].Flags["lost_life_last_turn"] == 0 {
		t.Errorf("lost_life_last_turn flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_SharesCreatureType(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Leaf-Crowned Elder", Owner: 0,
			Types: []string{"creature", "treefolk"},
		},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it shares a creature type with this creature, you may put it onto the battlefield"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSharesCreatureType {
		t.Fatalf("expected SharesCreatureType, got %v", cs.kind)
	}
	if len(gs.Seats[0].Library) == 0 {
		t.Fatalf("seat 0 library should have a placed match card")
	}
	top := gs.Seats[0].Library[0]
	hasCreature := false
	for _, ty := range top.Types {
		if ty == "creature" {
			hasCreature = true
			break
		}
	}
	if !hasCreature {
		t.Errorf("top-of-library reveal should be a creature, got %v", top.Types)
	}
}

// Era 1 r60 era1-scaffold-sweep — apply tests for the 6 new scaffold kinds.
// Each pins the engine-state mutation the scaffold makes so a downstream
// predicate reader can resolve true at scaffold time.

func TestApplyConditionScaffolding_Era1R60_CreatureHasKeyword(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Anje Falkenrath", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it has madness, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldCreatureHasKeyword {
		t.Fatalf("expected CreatureHasKeyword, got %v", cs.kind)
	}
	if src.Flags["madness"] != 1 {
		t.Errorf("madness flag not set on srcPerm")
	}
	found := false
	for _, g := range src.GrantedAbilities {
		if g == "madness" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("madness not present in GrantedAbilities, got %v", src.GrantedAbilities)
	}
}

func TestApplyConditionScaffolding_Era1R60_IsToken_NonToken(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Gruff Triplets", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{"token": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"when ~ dies, if it isn't a token, return it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldIsToken {
		t.Fatalf("expected IsToken, got %v", cs.kind)
	}
	if _, ok := src.Flags["token"]; ok {
		t.Errorf("token flag should be cleared for non_token subtype")
	}
	if src.Flags["non_token"] != 1 {
		t.Errorf("non_token flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfInZone_Exile(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Uvilda, Dean of Perfection", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as this card is exiled, you may cast it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfInZone {
		t.Fatalf("expected SelfInZone, got %v", cs.kind)
	}
	if cs.subtype != "exile" {
		t.Errorf("expected subtype=exile, got %q", cs.subtype)
	}
	if src.Flags["in_exile_zone"] != 1 {
		t.Errorf("in_exile_zone flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfIsEnchanted(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Krond the Dawn-Clad", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"whenever ~ attacks, if it's enchanted, destroy target nonland permanent"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfIsEnchanted {
		t.Fatalf("expected SelfIsEnchanted, got %v", cs.kind)
	}
	if src.Flags["enchanted"] != 1 {
		t.Errorf("enchanted flag not set")
	}
	foundAura := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.AttachedTo == src {
			foundAura = true
			break
		}
	}
	if !foundAura {
		t.Errorf("expected an Aura permanent attached to srcPerm")
	}
}

func TestApplyConditionScaffolding_Era1R60_SelfIsSuspected(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Repeat Offender", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as this creature is suspected, opponents lose 1 life"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldSelfIsSuspected {
		t.Fatalf("expected SelfIsSuspected, got %v", cs.kind)
	}
	if src.Flags["suspected"] != 1 {
		t.Errorf("suspected flag not set")
	}
}

func TestApplyConditionScaffolding_Era1R60_NoLandPlayedThisTurn(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Turn.LandsPlayed = 1
	gs.Seats[0].Flags["landfall_this_turn"] = 1
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you didn't play a land this turn, you may search your library"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldNoLandPlayedThisTurn {
		t.Fatalf("expected NoLandPlayedThisTurn, got %v", cs.kind)
	}
	if gs.Seats[0].Turn.LandsPlayed != 0 {
		t.Errorf("LandsPlayed should be 0, got %d", gs.Seats[0].Turn.LandsPlayed)
	}
	if gs.Seats[0].Flags["no_land_played"] != 1 {
		t.Errorf("no_land_played flag not set")
	}
	if _, ok := gs.Seats[0].Flags["landfall_this_turn"]; ok {
		t.Errorf("landfall_this_turn should be cleared")
	}
}
