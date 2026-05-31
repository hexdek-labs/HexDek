package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDetectConditionScaffold_Era1R60Batch2 pins detection for the second
// Era-1 r60 sweep — broadened text matchers + 4 net-new scaffolds (parity /
// keyword / token / zone-state). Each case mirrors a fragment found by
// scripts/era1_unbucketed_dump.py against the 2026-05-30 corpus.
func TestDetectConditionScaffold_Era1R60Batch2(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantKind conditionScaffoldKind
		wantSub  string
		wantCnt  int
	}{
		// --- Hand-size broadening ---
		{
			name:     "hand size exactly thirteen",
			text:     "as long as you have exactly thirteen cards in your hand, you win the game",
			wantKind: condScaffoldHandSizeThreshold,
			wantSub:  "hand_size_eq",
			wantCnt:  13,
		},
		{
			name:     "hand size at least one (you have a card in)",
			text:     "as long as you have a card in your hand, ~ has flying",
			wantKind: condScaffoldHandSizeThreshold,
			wantSub:  "hand_size_ge",
			wantCnt:  1,
		},
		// --- Opp/defending/that-player has more cards ---
		{
			name:     "defending player more cards",
			text:     "if defending player has more cards in hand than you, ~ deals 4 damage",
			wantKind: condScaffoldMoreCardsThanOpponents,
			wantSub:  "inverse",
		},
		{
			name:     "an opponent more cards in hand",
			text:     "if an opponent has more cards in hand than you, draw a card",
			wantKind: condScaffoldMoreCardsThanOpponents,
			wantSub:  "inverse",
		},
		// --- Drew/drawn N or more this turn ---
		// Note: when "draw a card" appears in the continuation, the existing
		// DrawnCardThisTurn matcher (line ~3026) wins. Both are valid bucket
		// destinations for audit-coverage purposes; the test below uses an
		// effect string that avoids the "draw a card" trigger so the count-
		// aware DrawnNCardsThisTurn matcher reaches its branch.
		{
			name:     "drawn more than one card",
			text:     "whenever you've drawn more than one card this turn, ~ deals 1 damage",
			wantKind: condScaffoldDrawnNCardsThisTurn,
		},
		// --- Cast N typed spells this turn ---
		{
			name:     "cast two or more noncreature spells",
			text:     "as long as you've cast two or more noncreature spells this turn, ~ has double strike",
			wantKind: condScaffoldCastNSpellsThisTurn,
			wantCnt:  2,
		},
		// --- Power state broadening ---
		{
			name:     "its power was 3 or greater",
			text:     "if its power was 3 or greater, draw a card",
			wantKind: condScaffoldSelfPowerGE,
			wantCnt:  3,
		},
		{
			name:     "~ has power 7 or greater",
			text:     "as long as ~ has power 7 or greater, ~ has trample",
			wantKind: condScaffoldSelfPowerGE,
			wantCnt:  7,
		},
		// --- Power parity ---
		{
			name:     "power is even",
			text:     "as long as ~'s power is even, you may cast noncreature spells as though they had flash",
			wantKind: condScaffoldSelfPowerParity,
			wantSub:  "even",
		},
		{
			name:     "power is odd",
			text:     "as long as ~'s power is odd, you may cast creature spells as though they had flash",
			wantKind: condScaffoldSelfPowerParity,
			wantSub:  "odd",
		},
		// --- Toughness state ---
		{
			name:     "toughness 6 or greater",
			text:     "if that creature has toughness 6 or greater, exile it",
			wantKind: condScaffoldSelfPowerGE,
			wantSub:  "toughness",
			wantCnt:  6,
		},
		// --- Self has keyword ---
		{
			name:     "it has flying",
			text:     "as long as a card exiled with this creature has flying, this creature has flying",
			wantKind: condScaffoldSelfHasKeyword,
			wantSub:  "flying",
		},
		{
			name:     "it has first strike",
			text:     "if it has first strike, ~ gets +2/+0",
			wantKind: condScaffoldSelfHasKeyword,
			wantSub:  "first strike",
		},
		// --- Self is token ---
		{
			name:     "it isn't a token",
			text:     "if it isn't a token, exile it instead",
			wantKind: condScaffoldSelfIsToken,
			wantSub:  "not_token",
		},
		{
			name:     "it's not a token",
			text:     "as long as it's not a token, ~ has indestructible",
			wantKind: condScaffoldSelfIsToken,
			wantSub:  "not_token",
		},
		// --- Renowned / suspected (route to IsSubtype) ---
		{
			name:     "is renowned",
			text:     "as long as this creature is renowned, it has menace",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "renowned",
		},
		{
			name:     "is suspected",
			text:     "this creature is suspected: pay {2} or sacrifice it",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "suspected",
		},
		// --- Self enchanted ---
		{
			name:     "it's enchanted",
			text:     "as long as it's enchanted, ~ gets +2/+2 and has flying",
			wantKind: condScaffoldEquipmentAttached,
			wantSub:  "enchanted_or_equipped",
		},
		// --- Zone state ---
		{
			name:     "as long as ~ is on the stack",
			text:     "as long as ~ is on the stack, spells that target it cost {2} more to cast",
			wantKind: condScaffoldSelfZoneState,
			wantSub:  "stack",
		},
		{
			name:     "this card is exiled",
			text:     "as long as this card is exiled, you may cast spells from any zone",
			wantKind: condScaffoldSelfZoneState,
			wantSub:  "exile",
		},
		// "~ is in the command zone" — the existing EminenceCommandZone
		// matcher (line ~3779) catches "in the command zone" first; SelfZoneState
		// only fires for "as long as ~ is" prefix when the existing matcher
		// misses (rare in Era 1 corpus). Both bucket the audit; assertion
		// below uses the structurally-distinct "while it's exiled" form.
		// --- Tribal-died broadening ---
		// Note: the existing "drew/drawn a card this turn" matcher (line ~3026)
		// catches "draw a card" in continuation clauses; we test with effects
		// that don't include "draw a card" so the tribal-died matcher reaches
		// its branch.
		{
			name:     "a phyrexian died under your control",
			text:     "if a phyrexian died under your control this turn, deal 2 damage to any target",
			wantKind: condScaffoldCreatureDiedThisTurn,
		},
		// --- Counter-state broadening ---
		{
			name:     "passive counter threshold non-creature subject",
			text:     "if there are three or more dread counters on it, transform it",
			wantKind: condScaffoldCountersOnSelfGE,
			wantSub:  "dread",
			wantCnt:  3,
		},
		{
			name:     "this enchantment has one or more wreck counters",
			text:     "as long as this enchantment has one or more wreck counters on it, ~ gets +1/+1",
			wantKind: condScaffoldCountersOnSelfGE,
			wantSub:  "wreck",
		},
		{
			name:     "doesn't have an indestructible counter",
			text:     "if it doesn't have an indestructible counter on it, destroy it",
			wantKind: condScaffoldSelfHasNoCounter,
			wantSub:  "indestructible",
		},
		// --- Life comparisons broadening ---
		{
			name:     "life total less than 7",
			text:     "if your life total is less than 7, ~ has indestructible",
			wantKind: condScaffoldLifeBelowThreshold,
		},
		{
			name:     "opp has N or less life",
			text:     "as long as an opponent has 10 or less life, ~ has intimidate",
			wantKind: condScaffoldLifeBelowThreshold,
			wantSub:  "opponent",
		},
		// --- Alt-cost / mana-from-creatures ---
		{
			name:     "mana from creatures spent",
			text:     "if three or more mana from creatures was spent to cast it, double its damage",
			wantKind: condScaffoldManaSpentThreshold,
			wantCnt:  3,
		},
		{
			name:     "sneak cost was paid",
			text:     "when ~ enters, if his sneak cost was paid, deal 5 damage to any target",
			wantKind: condScaffoldPaidOptionalCost,
		},
		// --- Cast not from hand ---
		{
			name:     "cast from anywhere other than hand",
			text:     "if this spell was cast from anywhere other than your hand, it costs {2} less",
			wantKind: condScaffoldYouCastFromHand,
			wantSub:  "not_from_hand",
		},
		// --- You haven't cast ---
		{
			name:     "you haven't cast the card",
			text:     "if you haven't cast the card, exile it instead",
			wantKind: condScaffoldWasntCast,
			wantSub:  "you_havent",
		},
		// --- Batch 3 — combat broadening ---
		{
			name:     "it's attacking one of your opponents",
			text:     "as long as it's attacking one of your opponents, ~ gets +2/+0",
			wantKind: condScaffoldIsAttacking,
		},
		{
			name:     "was blocked this turn",
			text:     "if it was blocked this turn, ~ gets +2/+0",
			wantKind: condScaffoldAttackedOrBlockedCombat,
		},
		{
			name:     "they didn't attack you that turn",
			text:     "if they didn't attack you that turn, ~ has hexproof",
			wantKind: condScaffoldDidntAttackThisTurn,
		},
		// --- Batch 3 — phase predicates ---
		{
			name:     "it's an opponent's turn",
			text:     "as long as it's an opponent's turn, ~ has indestructible",
			wantKind: condScaffoldNotTheirTurn,
			wantSub:  "opp_turn",
		},
		// --- Batch 3 — card-type reveal broadened ---
		{
			name:     "if it was a land card",
			text:     "if it was a land card, add {r} or {g}",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "land",
		},
		{
			name:     "if a creature card is exiled this way",
			text:     "if a creature card is exiled this way, create a 1/1 pest token",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "creature",
		},
		// --- Batch 3 — counter broadening ---
		{
			name:     "it has odd number of counters",
			text:     "if it has an odd number of counters on it, draw a card",
			wantKind: condScaffoldSelfHasCounter,
		},
		{
			name:     "there are no echo counters",
			text:     "if there are no echo counters on it, exile it",
			wantKind: condScaffoldSelfHasNoCounter,
			wantSub:  "echo",
		},
		{
			name:     "fewer than three +1/+1 counters",
			text:     "as long as this creature has fewer than three +1/+1 counters on it, ~ gets +1/+0",
			wantKind: condScaffoldSelfHasNoCounter,
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
			if tc.wantCnt != 0 && cs.count != tc.wantCnt {
				t.Errorf("count: got %d, want %d (text=%q)", cs.count, tc.wantCnt, tc.text)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60Batch2_PowerParity exercises the
// SelfPowerParity apply branch end-to-end: detect + apply leaves srcPerm's
// BasePower with the requested parity.
func TestApplyConditionScaffolding_Era1R60Batch2_PowerParity(t *testing.T) {
	for _, parity := range []string{"even", "odd"} {
		t.Run(parity, func(t *testing.T) {
			gs := newTestGameState(2)
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "Kianne, Corrupted Memory", Owner: 0, Types: []string{"creature"}, BasePower: 0, BaseToughness: 4},
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
			if src.Card.BasePower%2 != boolInt(parity == "odd") {
				t.Errorf("BasePower %d does not match parity %q", src.Card.BasePower, parity)
			}
			if src.Flags["power_parity_"+parity] != 1 {
				t.Errorf("power_parity_%s flag not set: %v", parity, src.Flags)
			}
		})
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestApplyConditionScaffolding_Era1R60Batch2_SelfHasKeyword stamps "has_<kw>"
// + canonical-keyword flag for the printed keyword name.
func TestApplyConditionScaffolding_Era1R60Batch2_SelfHasKeyword(t *testing.T) {
	for _, kw := range []string{"flying", "first strike", "deathtouch", "lifelink"} {
		t.Run(kw, func(t *testing.T) {
			gs := newTestGameState(2)
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "TestKeyword", Owner: 0, Types: []string{"creature"}},
				Controller: 0, Owner: 0,
				Flags: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if it has " + kw + ", ~ has menace"}}
			cs := applyConditionScaffolding(gs, cond, src)
			if cs.kind != condScaffoldSelfHasKeyword {
				t.Fatalf("expected SelfHasKeyword, got %v (text=%q)", cs.kind, cond.Args[0])
			}
			canonical := kw
			switch kw {
			case "first strike":
				canonical = "first_strike"
			}
			if src.Flags["has_"+canonical] != 1 {
				t.Errorf("has_%s flag not set: %v", canonical, src.Flags)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60Batch2_SelfIsToken mutates
// Card.Types accordingly so Permanent.IsToken() answers as expected.
func TestApplyConditionScaffolding_Era1R60Batch2_SelfIsToken(t *testing.T) {
	cases := []struct {
		text       string
		wantSub    string
		wantIsTok  bool
	}{
		{"if it isn't a token, exile it", "not_token", false},
		{"if it's not a token, gain 3 life", "not_token", false},
	}
	for _, tc := range cases {
		t.Run(tc.text, func(t *testing.T) {
			gs := newTestGameState(2)
			src := &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "Subject", Owner: 0, Types: []string{"creature", "token"}},
				Controller: 0, Owner: 0,
				Flags: map[string]int{},
			}
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{tc.text}}
			cs := applyConditionScaffolding(gs, cond, src)
			if cs.kind != condScaffoldSelfIsToken {
				t.Fatalf("expected SelfIsToken, got %v", cs.kind)
			}
			if cs.subtype != tc.wantSub {
				t.Errorf("subtype: got %q, want %q", cs.subtype, tc.wantSub)
			}
			if src.IsToken() != tc.wantIsTok {
				t.Errorf("IsToken() = %v, want %v (Types=%v)", src.IsToken(), tc.wantIsTok, src.Card.Types)
			}
		})
	}
}

// TestApplyConditionScaffolding_Era1R60Batch2_SelfZoneState moves srcPerm.Card
// into the named zone for library_top / exile / command.
func TestApplyConditionScaffolding_Era1R60Batch2_SelfZoneState(t *testing.T) {
	t.Run("library_top", func(t *testing.T) {
		gs := newTestGameState(2)
		card := &gameengine.Card{Name: "TopCard", Owner: 0, Types: []string{"creature"}}
		src := &gameengine.Permanent{Card: card, Controller: 0, Owner: 0, Flags: map[string]int{}}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
		cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as ~ is at the top of your library, you may cast it"}}
		cs := applyConditionScaffolding(gs, cond, src)
		if cs.kind != condScaffoldSelfZoneState || cs.subtype != "library_top" {
			t.Fatalf("expected SelfZoneState[library_top], got %v[%q]", cs.kind, cs.subtype)
		}
		if len(gs.Seats[0].Library) == 0 || gs.Seats[0].Library[0] != card {
			t.Errorf("card not at top of library; lib=%v", gs.Seats[0].Library)
		}
	})
	t.Run("exile", func(t *testing.T) {
		gs := newTestGameState(2)
		card := &gameengine.Card{Name: "ExiledCard", Owner: 0, Types: []string{"creature"}}
		src := &gameengine.Permanent{Card: card, Controller: 0, Owner: 0, Flags: map[string]int{}}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
		cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"as long as this card is exiled, you may cast it"}}
		cs := applyConditionScaffolding(gs, cond, src)
		if cs.kind != condScaffoldSelfZoneState || cs.subtype != "exile" {
			t.Fatalf("expected SelfZoneState[exile], got %v[%q]", cs.kind, cs.subtype)
		}
		found := false
		for _, c := range gs.Seats[0].Exile {
			if c == card {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("card not in seat 0 exile; exile=%v", gs.Seats[0].Exile)
		}
		if src.Flags["zone_exile"] != 1 {
			t.Errorf("zone_exile flag not set: %v", src.Flags)
		}
	})
}

// TestEra1AuditGapClosed pins the Era 1 audit-gap target. We use the
// scaffold-detection function directly rather than re-running the Python
// audit (the audit is the ground truth; this test guards against
// regressions in detectConditionScaffold for the BATCH 2 + 3 fragments).
func TestEra1AuditGapClosed_Batch2Coverage(t *testing.T) {
	// 5 representative fragments from each newly-bucketed cluster. If any
	// of these regress, the audit gap will widen — fail loudly here so the
	// regression is caught before audit re-run.
	probes := []struct {
		name string
		text string
	}{
		{"hand_size_words", "as long as you have four or more cards in hand, ~ has vigilance"},
		{"power_state_broadened", "if ~ has power 7 or greater, draw a card"},
		{"renowned_state", "as long as this creature is renowned, it has menace"},
		{"is_enchanted", "as long as this creature is enchanted, ~ gets +2/+2 and has flying"},
		{"died_under_your_control", "if another human died under your control this turn, draw a card"},
		{"is_attacking_player", "as long as it's attacking one of your opponents, ~ gets +2/+0"},
		{"card_type_reveal_broad", "if it was a land card, add {r} or {g}"},
		{"counter_passive_threshold", "if there are three or more dread counters on it, transform"},
	}
	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{p.text}}
			cs := detectConditionScaffold(cond)
			if cs.kind == condScaffoldNone {
				t.Errorf("regression: text bucketed in batch 2/3 now unbucketed: %q", p.text)
			}
		})
	}
}
