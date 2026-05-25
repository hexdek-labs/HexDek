package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDetectConditionScaffold_Era4R60 pins detection for the 19 new
// Era 4 (2023+) scaffolds added in the R60 era-4 sweep. Each case mirrors a
// real card phrase from scripts/era4_scaffold_audit.py's unbucketed
// fragments table; subtype/count expectations match what the detector
// should lift from the raw text.
func TestDetectConditionScaffold_Era4R60(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		text     string
		wantKind conditionScaffoldKind
		wantSub  string
		wantCnt  int
	}{
		{
			name:     "opponent more creatures raw",
			kind:     "raw",
			text:     "if an opponent controls more creatures than you, draw a card",
			wantKind: condScaffoldOpponentMoreCreatures,
		},
		{
			name:     "opponent more creatures structured",
			kind:     "opponent_more_creatures",
			text:     "",
			wantKind: condScaffoldOpponentMoreCreatures,
		},
		{
			name:     "wasnt creature type raw",
			kind:     "raw",
			text:     "when ~ enters, if it wasn't a demon, sacrifice it",
			wantKind: condScaffoldWasntCreatureType,
			wantSub:  "demon",
		},
		{
			name:     "wasnt creature type structured",
			kind:     "wasnt_creature_type",
			text:     "zombie",
			wantKind: condScaffoldWasntCreatureType,
			wantSub:  "zombie",
		},
		{
			name:     "all lands type raw",
			kind:     "raw",
			text:     "as long as all lands on the battlefield are islands, this creature has flying",
			wantKind: condScaffoldAllLandsType,
			wantSub:  "island",
		},
		{
			name:     "was attacking raw",
			kind:     "raw",
			text:     "if it was attacking, exile it instead",
			wantKind: condScaffoldWasAttacking,
		},
		{
			name:     "permanent mv le raw",
			kind:     "raw",
			text:     "if it's a permanent card with mana value 3 or less, you may cast it",
			wantKind: condScaffoldPermanentMVLE,
			wantCnt:  3,
		},
		{
			name:     "was cast raw",
			kind:     "raw",
			text:     "when ~ enters, if it was cast, draw a card",
			wantKind: condScaffoldWasCast,
		},
		{
			name:     "didnt die raw",
			kind:     "raw",
			text:     "at the next end step, if it didn't die, exile it",
			wantKind: condScaffoldDidntDie,
		},
		{
			name:     "self didnt etb this turn raw",
			kind:     "raw",
			text:     "as long as this creature didn't enter the battlefield this turn, it has indestructible",
			wantKind: condScaffoldSelfDidntETBThisTurn,
		},
		{
			name:     "no named tokens raw",
			kind:     "raw",
			text:     "at the beginning of your upkeep, if there are no reflection tokens on the battlefield, create one",
			wantKind: condScaffoldNoNamedTokens,
			wantSub:  "reflection",
		},
		{
			name:     "wasnt sacrificed raw",
			kind:     "raw",
			text:     "when ~ leaves the battlefield, if it wasn't sacrificed, return it to your hand",
			wantKind: condScaffoldWasntSacrificed,
		},
		{
			name:     "counters on self ge raw",
			kind:     "raw",
			text:     "at the end of your turn, if it has three or more +1/+1 counters on it, draw a card",
			wantKind: condScaffoldCountersOnSelfGE,
			wantSub:  "+1/+1",
			wantCnt:  3,
		},
		{
			name:     "counters on self ge bloodline raw",
			kind:     "raw",
			text:     "if there are three or more bloodline counters on it, sacrifice it",
			wantKind: condScaffoldCountersOnSelfGE,
		},
		{
			name:     "no damage since last turn raw",
			kind:     "raw",
			text:     "if you haven't been dealt combat damage since your last turn, draw a card",
			wantKind: condScaffoldNoDamageSinceLastTurn,
		},
		{
			name:     "first time resolved this turn raw",
			kind:     "raw",
			text:     "if this is the first time this ability has resolved this turn, draw a card",
			wantKind: condScaffoldFirstTimeResolvedThisTurn,
		},
		{
			name:     "player no creatures raw",
			kind:     "raw",
			text:     "if a player controls no creatures, deal 5 damage",
			wantKind: condScaffoldPlayerNoCreatures,
		},
		{
			name:     "another typed etb this turn raw",
			kind:     "raw",
			text:     "if another human entered the battlefield under your control this turn, draw a card",
			wantKind: condScaffoldAnotherTypedETBThisTurn,
			wantSub:  "human",
		},
		{
			name:     "creature etb last turn raw",
			kind:     "raw",
			text:     "if you had another creature enter the battlefield under your control last turn, draw a card",
			wantKind: condScaffoldCreatureETBLastTurn,
		},
		{
			name:     "exiled with count ge raw",
			kind:     "raw",
			text:     "if three or more cards have been exiled with this artifact, sacrifice it",
			wantKind: condScaffoldExiledWithCountGE,
			wantCnt:  3,
		},
		{
			name:     "exiled this turn raw",
			kind:     "raw",
			text:     "at the beginning of your end step, if one or more cards were put into exile this turn, draw a card",
			wantKind: condScaffoldExiledThisTurn,
		},
		{
			name:     "damaged creature died raw",
			kind:     "raw",
			text:     "at end of combat, if a creature dealt damage by this creature this turn died, draw a card",
			wantKind: condScaffoldDamagedCreatureDied,
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

// TestApplyConditionScaffolding_Era4R60_* verifies that each new apply
// implementation mutates engine state in the expected direction so a
// downstream condition reader can resolve the predicate to true.

func TestApplyConditionScaffolding_Era4R60_OpponentMoreCreatures(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if an opponent controls more creatures than you, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldOpponentMoreCreatures {
		t.Fatalf("expected OpponentMoreCreatures, got %v", cs.kind)
	}
	if len(gs.Seats[1].Battlefield) <= len(gs.Seats[0].Battlefield) {
		t.Errorf("seat 1 should control more creatures than seat 0 (1=%d, 0=%d)",
			len(gs.Seats[1].Battlefield), len(gs.Seats[0].Battlefield))
	}
}

func TestApplyConditionScaffolding_Era4R60_WasntCreatureType(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Infernal Vessel", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it wasn't a demon, sacrifice it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldWasntCreatureType {
		t.Fatalf("expected WasntCreatureType, got %v", cs.kind)
	}
	if cs.subtype != "demon" {
		t.Errorf("subtype: want demon, got %q", cs.subtype)
	}
	if src.Flags["wasnt_demon"] != 1 {
		t.Errorf("wasnt_demon flag not set")
	}
}

func TestApplyConditionScaffolding_Era4R60_AllLandsType(t *testing.T) {
	gs := newTestGameState(2)
	// Seed a non-island land on seat 0 to verify conversion.
	existing := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Forest Setup",
			Owner: 0,
			Types: []string{"land", "forest"},
		},
		Controller: 0, Owner: 0,
		Flags:    map[string]int{},
		Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, existing)

	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as all lands on the battlefield are islands, this creature has flying"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldAllLandsType {
		t.Fatalf("expected AllLandsType, got %v", cs.kind)
	}
	if cs.subtype != "island" {
		t.Errorf("subtype: want island, got %q", cs.subtype)
	}
	// Existing land should be converted to island.
	foundIsland := false
	for _, t2 := range existing.Card.Types {
		if t2 == "island" {
			foundIsland = true
			break
		}
	}
	if !foundIsland {
		t.Errorf("existing land not converted to island, types=%v", existing.Card.Types)
	}
}

func TestApplyConditionScaffolding_Era4R60_WasAttacking(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Zurgo Stormrender", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it was attacking, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldWasAttacking {
		t.Fatalf("expected WasAttacking, got %v", cs.kind)
	}
	if src.Flags["attacking"] != 1 || src.Flags["was_attacking"] != 1 {
		t.Errorf("attacking flags not set: %v", src.Flags)
	}
}

func TestApplyConditionScaffolding_Era4R60_CountersOnSelfGE(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Ordeal of Nylea", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0,
		Flags:    map[string]int{},
		Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it has three or more +1/+1 counters on it, sacrifice it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldCountersOnSelfGE {
		t.Fatalf("expected CountersOnSelfGE, got %v", cs.kind)
	}
	if src.Counters["+1/+1"] < 3 {
		t.Errorf("+1/+1 counters: want >=3, got %d", src.Counters["+1/+1"])
	}
}

func TestApplyConditionScaffolding_Era4R60_DidntDie(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:         &gameengine.Card{Name: "Taeko, the Patient Avalanche", Owner: 0, Types: []string{"creature"}},
		Controller:   0,
		Owner:        0,
		Flags:        map[string]int{},
		MarkedDamage: 5,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it didn't die, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldDidntDie {
		t.Fatalf("expected DidntDie, got %v", cs.kind)
	}
	if src.Flags["didnt_die"] != 1 {
		t.Errorf("didnt_die flag not set")
	}
	if src.MarkedDamage != 0 {
		t.Errorf("MarkedDamage should be cleared, got %d", src.MarkedDamage)
	}
}

func TestApplyConditionScaffolding_Era4R60_PlayerNoCreatures(t *testing.T) {
	gs := newTestGameState(2)
	// Seed seat 1 with a creature that should be removed.
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Opponent Creature", Owner: 1, Types: []string{"creature"}},
		Controller: 1, Owner: 1,
		Flags: map[string]int{},
	})
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if a player controls no creatures, deal 5 damage"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldPlayerNoCreatures {
		t.Fatalf("expected PlayerNoCreatures, got %v", cs.kind)
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil {
			for _, ty := range p.Card.Types {
				if ty == "creature" {
					t.Errorf("seat 1 should have no creatures, found %q", p.Card.Name)
				}
			}
		}
	}
	if gs.Flags["a_player_controls_no_creatures"] != 1 {
		t.Errorf("a_player_controls_no_creatures flag not set")
	}
}

func TestApplyConditionScaffolding_Era4R60_AnotherTypedETBThisTurn(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if another human entered the battlefield under your control this turn, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldAnotherTypedETBThisTurn {
		t.Fatalf("expected AnotherTypedETBThisTurn, got %v", cs.kind)
	}
	// Find the seeded human creature on seat 0.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		for _, ty := range p.Card.Types {
			if ty == "human" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a human creature to be seeded on seat 0")
	}
	if gs.Seats[0].Flags["another_human_etb_this_turn"] != 1 {
		t.Errorf("another_human_etb_this_turn flag not set")
	}
}

func TestApplyConditionScaffolding_Era4R60_ExiledWithCountGE(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Colfenor's Urn", Owner: 0, Types: []string{"artifact"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if three or more cards have been exiled with this artifact, sacrifice it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldExiledWithCountGE {
		t.Fatalf("expected ExiledWithCountGE, got %v", cs.kind)
	}
	if len(gs.Seats[0].Exile) < 3 {
		t.Errorf("seat 0 exile: want >=3, got %d", len(gs.Seats[0].Exile))
	}
	if src.Flags["exiled_with_count"] < 3 {
		t.Errorf("exiled_with_count: want >=3, got %d", src.Flags["exiled_with_count"])
	}
}

func TestApplyConditionScaffolding_Era4R60_NoNamedTokens(t *testing.T) {
	gs := newTestGameState(2)
	// Seed a reflection token on seat 0 that should be removed.
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Reflection",
			Owner: 0,
			Types: []string{"creature", "token", "reflection"},
		},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	})
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if there are no reflection tokens on the battlefield, create one"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldNoNamedTokens {
		t.Fatalf("expected NoNamedTokens, got %v", cs.kind)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Reflection" {
			t.Errorf("reflection token should have been removed")
		}
	}
	if gs.Flags["no_reflection_tokens"] != 1 {
		t.Errorf("no_reflection_tokens flag not set")
	}
}

func TestApplyConditionScaffolding_Era4R60_NoDamageSinceLastTurn(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you haven't been dealt combat damage since your last turn, draw a card"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldNoDamageSinceLastTurn {
		t.Fatalf("expected NoDamageSinceLastTurn, got %v", cs.kind)
	}
	if gs.Seats[0].Flags["no_damage_since_last_turn"] != 1 {
		t.Errorf("no_damage_since_last_turn flag not set")
	}
}
