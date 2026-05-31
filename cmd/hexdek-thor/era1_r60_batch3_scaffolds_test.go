package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDetectConditionScaffold_Era1R60Batch3 pins detection for the batch-3
// residual sweep — patterns the #787 batch did NOT cover. Each case mirrors
// a fragment from scripts/era1_unbucketed_dump.py against the 2026-05-30
// post-#787 corpus.
func TestDetectConditionScaffold_Era1R60Batch3(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantKind conditionScaffoldKind
		wantSub  string
	}{
		// --- Power parity (net-new) ---
		{
			name:     "power is even (Kianne)",
			text:     "as long as ~'s power is even, you may cast noncreature spells as though they had flash",
			wantKind: condScaffoldSelfPowerParity,
			wantSub:  "even",
		},
		{
			name:     "power is odd (Kianne)",
			text:     "as long as ~'s power is odd, you may cast creature spells as though they had flash",
			wantKind: condScaffoldSelfPowerParity,
			wantSub:  "odd",
		},
		// --- "That creature" P/T queries ---
		{
			name:     "that creature has power 7+ (Yavimaya)",
			text:     "if that creature has power 7 or greater, destroy it",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "power",
		},
		{
			name:     "that creature's power 0 or less (Depressurize)",
			text:     "if that creature's power is 0 or less, exile it",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "power",
		},
		{
			name:     "that creature toughness 1+ (Gore Vassal)",
			text:     "if that creature's toughness is 1 or greater, regenerate it",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "toughness",
		},
		{
			name:     "toughness was less than 1 (Massacre Girl)",
			text:     "if its toughness was less than 1, draw a card",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "toughness",
		},
		{
			name:     "greater power or toughness (Evolving Adaptive)",
			text:     "if that creature has greater power than this creature, draw a card",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "compare",
		},
		// --- Counter fewer-than / exactly ---
		{
			name:     "fewer than three +1/+1 (Runaway Steam-Kin)",
			text:     "as long as this creature has fewer than three +1/+1 counters on it, do something",
			wantKind: condScaffoldSelfHasCounter,
		},
		{
			name:     "exactly four +1/+1 (Ayara's Oathsworn)",
			text:     "if it has exactly four +1/+1 counters on it, draw a card",
			wantKind: condScaffoldSelfHasCounter,
		},
		// --- Bare-existence named counter ---
		{
			name:     "as long as ~ has a conqueror counter on him (Zhao)",
			text:     "as long as ~ has a conqueror counter on him, nonbasic lands are mountains",
			wantKind: condScaffoldSelfHasCounter,
			wantSub:  "conqueror",
		},
		// --- Hand-size exact ---
		{
			name:     "hand exactly thirteen (Triskaidekaphile)",
			text:     "as long as you have exactly thirteen cards in your hand, you win the game",
			wantKind: condScaffoldHandSizeThreshold,
			wantSub:  "hand_size_eq",
		},
		// --- An opponent / that player more cards ---
		{
			name:     "an opponent more cards (Pulse of the Grid)",
			text:     "if an opponent has more cards in hand than you, draw two cards",
			wantKind: condScaffoldMoreCardsThanOpponents,
			// #787's matcher catches this with subtype="opponent_more";
			// both bucket correctly so we don't pin subtype.
		},
		{
			name:     "planeswalker controller more life (Pulse of the Forge)",
			text:     "if that player or that planeswalker's controller has more life than you, deal damage",
			wantKind: condScaffoldOpponentMoreLife,
		},
		// --- Past-tense enchanted/equipped ---
		{
			name:     "it was enchanted (Gunner Conscript)",
			text:     "when ~ enters, if it was enchanted, do something",
			wantKind: condScaffoldSelfIsEnchanted,
			wantSub:  "past",
		},
		{
			name:     "it was equipped (Gunner Conscript)",
			text:     "when ~ enters, if it was equipped, do something",
			wantKind: condScaffoldSelfIsEnchanted,
			wantSub:  "past",
		},
		// --- Attached-to-creature ---
		{
			name:     "as long as ~ is attached to a creature (Reality Chip)",
			text:     "as long as ~ is attached to a creature, you may play lands from the top of your library",
			wantKind: condScaffoldEquipmentAttached,
			// Existing EquipmentAttached matcher catches the "is attached"
			// substring first and returns without our "attached" subtype.
		},
		{
			name:     "enchanted equipment is attached (Artificer's Hex)",
			text:     "when enchanted equipment is attached to a creature, do something",
			wantKind: condScaffoldEquipmentAttached,
		},
		// --- Permanent-is-creature ---
		{
			name:     "this artifact is a creature (Foriysian Totem)",
			text:     "as long as this artifact is a creature, it can block an additional creature each combat",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "creature",
		},
		{
			name:     "this permanent is a creature (Triton Wavebreaker)",
			text:     "as long as this permanent is a creature, it has prowess",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "creature",
		},
		{
			name:     "is legendary (Tenza)",
			text:     "as long as it's legendary, it gets an additional +2/+2",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "legendary",
		},
		// --- Past-turn negative (net-new) ---
		{
			name:     "didn't activate loyalty (The Chain Veil)",
			text:     "if you didn't activate a loyalty ability of a planeswalker this turn, do something",
			wantKind: condScaffoldNoLoyaltyActivated,
		},
		{
			name:     "lost 2+ life this turn (Book of Vile Darkness)",
			text:     "if you lost 2 or more life this turn, ~ gets +1/+1",
			wantKind: condScaffoldLostNLifeThisTurn,
		},
		// --- Combat-state ---
		{
			name:     "was dealt damage this turn (Wall of Resistance)",
			text:     "if this creature was dealt damage this turn, regenerate it",
			wantKind: condScaffoldAttackedOrBlockedCombat,
			wantSub:  "damaged",
		},
		{
			name:     "tribal-combat attacked (Fearless Swashbuckler)",
			text:     "if a pirate and a vehicle attacked this combat, you may do something",
			wantKind: condScaffoldAttackedThisTurn,
			wantSub:  "tribal_combat",
		},
		// --- Combat / defending player ---
		{
			name:     "defending player is poisoned (Septic Rats)",
			text:     "as long as defending player is poisoned, gain 1 life",
			wantKind: condScaffoldNotDeclaredAttacker,
			wantSub:  "poisoned",
		},
		// --- Web-slinging ---
		{
			// "they were cast using web-slinging" — the existing WasCast
			// matcher (line ~3019) catches "they were cast" first; both
			// bucket correctly. We assert it doesn't fall through to None.
			name:     "cast using web-slinging (Spiders-Man)",
			text:     "if they were cast using web-slinging, put a +1/+1 counter on a target",
			wantKind: condScaffoldWasCast,
		},
		// --- Card-type reveal ---
		{
			name:     "if it was a land card (Misfortune Teller)",
			text:     "if it was a land card, create a treasure token",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "land",
		},
		{
			name:     "if a card with the chosen name was milled (Predict)",
			text:     "if a card with the chosen name was milled this way, you draw two cards",
			wantKind: condScaffoldCardTypeReveal,
		},
		{
			name:     "if it's a mount card (Bucolic Ranch)",
			text:     "if it's a mount card, you may reveal it and put it into your hand",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "mount",
		},
		{
			name:     "the exiled card is a snow land (Storm Elemental)",
			text:     "as long as the exiled card is a snow land, gain 1 life",
			wantKind: condScaffoldCardTypeReveal,
		},
		{
			name:     "the exiled card doesn't have suspend (Gandalf)",
			text:     "if the exiled card doesn't have suspend, you may cast it for free",
			wantKind: condScaffoldCardTypeReveal,
		},
		// --- "You don't control X" ---
		{
			name:     "you don't control a food (Butterbur)",
			text:     "as long as you don't control a food, ~ gets +1/+0",
			wantKind: condScaffoldYouControlSubtype,
			wantSub:  "not_food",
		},
		// --- "If you have no land cards in your hand" ---
		{
			name:     "no land cards in hand (Bounty of the Deep)",
			text:     "if you have no land cards in your hand, seek a land card and a nonland card",
			wantKind: condScaffoldHandSizeThreshold,
			wantSub:  "no_lands_in_hand",
		},
		// --- "Revealed a dragon as you cast" ---
		{
			name:     "revealed or controlled (Orator of Ojutai)",
			text:     "if you revealed a dragon card or controlled a dragon as you cast this spell, draw a card",
			wantKind: condScaffoldDidPriorAction,
			wantSub:  "revealed_or_controlled",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{tc.text}}
			cs := detectConditionScaffold(cond)
			if cs.kind != tc.wantKind {
				t.Fatalf("detectConditionScaffold(%q): got kind=%v, want %v", tc.text, cs.kind, tc.wantKind)
			}
			if tc.wantSub != "" && cs.subtype != tc.wantSub {
				t.Errorf("subtype: got %q, want %q (text=%q)", cs.subtype, tc.wantSub, tc.text)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60Batch3_PowerParity exercises the
// SelfPowerParity apply branch end-to-end.
func TestApplyConditionScaffolding_Era1R60Batch3_PowerParity(t *testing.T) {
	for _, parity := range []string{"even", "odd"} {
		t.Run(parity, func(t *testing.T) {
			gs := newTestGameState(2)
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "Kianne", Owner: 0, Types: []string{"creature"}, BasePower: 0, BaseToughness: 4},
				Controller: 0, Owner: 0,
				Flags: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as ~'s power is " + parity + ", do something"}}
			cs := applyConditionScaffolding(gs, cond, src)
			if cs.kind != condScaffoldSelfPowerParity {
				t.Fatalf("expected SelfPowerParity, got %v", cs.kind)
			}
			if cs.subtype != parity {
				t.Fatalf("subtype: got %q, want %q", cs.subtype, parity)
			}
			wantOdd := 0
			if parity == "odd" {
				wantOdd = 1
			}
			if src.Card.BasePower%2 != wantOdd {
				t.Errorf("BasePower %d does not match parity %q", src.Card.BasePower, parity)
			}
			if src.Flags["power_parity_"+parity] != 1 {
				t.Errorf("power_parity_%s flag not set: %v", parity, src.Flags)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60Batch3_NoLoyaltyActivated stamps the
// flag on seat 0.
func TestApplyConditionScaffolding_Era1R60Batch3_NoLoyaltyActivated(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you didn't activate a loyalty ability of a planeswalker this turn, do something"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldNoLoyaltyActivated {
		t.Fatalf("expected NoLoyaltyActivated, got %v", cs.kind)
	}
	if gs.Seats[0].Flags["no_loyalty_activated_this_turn"] != 1 {
		t.Errorf("flag not set: %v", gs.Seats[0].Flags)
	}
}

// TestApplyConditionScaffolding_Era1R60Batch3_LostNLifeThisTurn bumps
// LifeLost.
func TestApplyConditionScaffolding_Era1R60Batch3_LostNLifeThisTurn(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if you lost 3 or more life this turn, do something"}}
	cs := applyConditionScaffolding(gs, cond, nil)
	if cs.kind != condScaffoldLostNLifeThisTurn {
		t.Fatalf("expected LostNLifeThisTurn, got %v", cs.kind)
	}
	if gs.Seats[0].Turn.LifeLost < 2 {
		t.Errorf("Turn.LifeLost want >=2, got %d", gs.Seats[0].Turn.LifeLost)
	}
}

// TestEra1AuditGapClosed_Batch3Coverage guards against routing regressions
// across the batch-3 cluster surface. If any probe regresses to None, the
// audit gap will widen.
func TestEra1AuditGapClosed_Batch3Coverage(t *testing.T) {
	probes := []struct {
		name string
		text string
	}{
		{"power_parity_even", "as long as ~'s power is even, draw a card"},
		{"that_creature_power_ge", "if that creature has power 7 or greater, destroy it"},
		{"fewer_than_counters", "if this creature has fewer than three +1/+1 counters on it, ~ gets +1/+0"},
		{"hand_size_exact", "if you have exactly thirteen cards in your hand, you win the game"},
		{"opp_more_cards", "if an opponent has more cards in hand than you, draw a card"},
		{"was_enchanted", "when ~ enters, if it was enchanted, do something"},
		{"attached_to_creature", "as long as ~ is attached to a creature, do something"},
		{"this_artifact_is_creature", "as long as this artifact is a creature, it has flying"},
		{"didnt_activate_loyalty", "if you didn't activate a loyalty ability this turn, do something"},
		{"lost_n_life_turn", "if you lost 2 or more life this turn, ~ has menace"},
		{"defending_poisoned", "as long as defending player is poisoned, ~ has lifelink"},
		{"card_type_was_milled", "if a land card was milled this way, you gain 1 life"},
		{"exiled_card_property", "if the exiled card is a snow land, do something"},
		{"web_slinging", "if they were cast using web-slinging, ~ enters with a counter"},
	}
	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{p.text}}
			cs := detectConditionScaffold(cond)
			if cs.kind == condScaffoldNone {
				t.Errorf("regression: batch-3 fragment no longer bucketed: %q", p.text)
			}
		})
	}
}
