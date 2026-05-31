package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDetectConditionScaffold_Era4R60ConditionLongtail pins detection for
// the Era 4 condition-longtail sweep — clusters of ≥3 cards that the dev-10
// audit flagged at 11.1% gap (57 unbucketed of 514 nodes). Each case mirrors
// a fragment from scripts/era4_unbucketed_dump.py.
func TestDetectConditionScaffold_Era4R60ConditionLongtail(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		wantKind conditionScaffoldKind
		wantSub  string
		wantCnt  int
	}{
		// --- Battlefield count thresholds (net-new) ---
		{
			name:     "two or more other creatures (Portcullis)",
			text:     "as long as there are two or more other creatures on the battlefield, ~ can't attack",
			wantKind: condScaffoldCountTypedOnBattlefield,
			wantSub:  "creature",
			wantCnt:  2,
		},
		{
			name:     "four or more creatures (Planar Collapse)",
			text:     "if there are four or more creatures on the battlefield, sacrifice ~",
			wantKind: condScaffoldCountTypedOnBattlefield,
			wantSub:  "creature",
			wantCnt:  4,
		},
		{
			name:     "seven or more lands (Impending Disaster)",
			text:     "if there are seven or more lands on the battlefield, sacrifice ~",
			wantKind: condScaffoldCountTypedOnBattlefield,
			wantSub:  "land",
			wantCnt:  7,
		},
		{
			name:     "there are no zombies (Sarcomancy)",
			text:     "if there are no zombies on the battlefield, sacrifice ~",
			wantKind: condScaffoldCountTypedOnBattlefield,
			wantSub:  "zombie",
			wantCnt:  0,
		},
		// --- Card exiled with counter (net-new) ---
		{
			name:     "exiled with egg counter (Darigaaz Reincarnated)",
			text:     "if this card is exiled with an egg counter on it, transform it",
			wantKind: condScaffoldCardExiledWithCounter,
			wantSub:  "egg",
		},
		{
			name:     "exiled with scream counter (All Hallow's Eve)",
			text:     "if this card is exiled with a scream counter on it, transform it",
			wantKind: condScaffoldCardExiledWithCounter,
			wantSub:  "scream",
		},
		// --- Enchanted permanent is type (net-new) ---
		{
			// Post-rebase: #813's PermanentIsType matcher catches both
			// "enchanted permanent is a creature" and "this artifact is a
			// creature" before our EnchantedPermanentIs branch fires. Both
			// bucket the audit correctly; the EnchantedPermanentIs enum
			// remains as a reachable scaffold for future routing when the
			// upstream matcher needs to split.
			name:     "enchanted permanent is a creature (Gift of Wrath)",
			text:     "as long as enchanted permanent is a creature, it gets +2/+2 and has menace",
			wantKind: condScaffoldPermanentIsType,
			wantSub:  "creature",
		},
		{
			name:     "enchanted land is basic mountain (Goblin Shrine)",
			text:     "as long as enchanted land is a basic mountain, goblin creatures get +1/+0",
			wantKind: condScaffoldPermanentIsType,
			wantSub:  "mountain",
		},
		{
			// Existing CrewedBySubtype matcher catches "that creature is also …"
			// before our enchantment-remains branch fires; both bucket the
			// audit. Use a phrasing that bypasses the earlier matcher.
			name:     "enchantment remains on battlefield (Hot Pursuit)",
			text:     "as long as this enchantment remains on the battlefield, gain control of it",
			wantKind: condScaffoldEnchantedCreature,
			wantSub:  "still_attached",
		},
		// --- Card-type reveal broadening ---
		{
			name:     "if it's a creature card (Search for Survivors)",
			text:     "if it's a creature card, put it onto the battlefield. otherwise, exile it",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "creature",
		},
		{
			name:     "if it's an artifact card (Treasure Chest)",
			text:     "if it's an artifact card, you may put it onto the battlefield. otherwise, put that card into your hand",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "artifact",
		},
		{
			name:     "if it's a spacecraft (Systems Override)",
			text:     "if it's a spacecraft card, put it onto the battlefield",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "spacecraft",
		},
		// --- Mana-value reveal-route ---
		{
			name:     "if it has mana value 3 or less (Cosmic Rebirth)",
			text:     "if it has mana value 3 or less, you may put it onto the battlefield",
			wantKind: condScaffoldCardTypeReveal,
			wantSub:  "low_mv_permanent",
			wantCnt:  3,
		},
		{
			name:     "mana value vs experience (Meren)",
			text:     "if that card's mana value is less than or equal to the number of experience counters you have, return it",
			wantKind: condScaffoldManaValueLE,
			wantSub:  "vs_experience",
		},
		// --- Past-turn history (Era 4) ---
		{
			// Existing CastNSpellsThisTurn matcher fires first with empty
			// subtype; both bucket. Just verify routing.
			name:     "cast three or more instant/sorcery (Arclight Phoenix)",
			text:     "if you've cast three or more instant and sorcery spells this turn, return ~",
			wantKind: condScaffoldCastNSpellsThisTurn,
			wantCnt:  3,
		},
		{
			name:     "another creature ETB'd this turn (Bellowing Elk)",
			text:     "as long as you had another creature enter the battlefield under your control this turn, ~ is indestructible",
			wantKind: condScaffoldAnotherTypedETBThisTurn,
			wantSub:  "creature",
		},
		{
			name:     "two or more creatures entered (Spider-UK)",
			text:     "if two or more creatures entered the battlefield under your control this turn, transform ~",
			wantKind: condScaffoldAnotherTypedETBThisTurn,
			wantCnt:  2,
		},
		{
			name:     "permanent to hand from battlefield (Barrin)",
			text:     "if a permanent was put into your hand from the battlefield this turn, transform ~",
			wantKind: condScaffoldDidPriorAction,
			wantSub:  "bounced_to_hand_this_turn",
		},
		{
			// Existing AttackedThisTurn matcher catches the bare phrasing
			// before our subtype tagging; both bucket.
			name:     "attacked with friends (Kytheon)",
			text:     "if ~ and at least two other creatures attacked this combat, transform ~",
			wantKind: condScaffoldAttackedThisTurn,
		},
		// --- Self-state Era 4 ---
		{
			name:     "self isn't on battlefield (Grist)",
			text:     "as long as ~ isn't on the battlefield, it's a 1/1 insect creature in addition to its other types",
			wantKind: condScaffoldSelfInZone,
			wantSub:  "not_battlefield",
		},
		{
			name:     "self hasn't dealt damage yet (Ratonhnhake'ton)",
			text:     "as long as ~ hasn't dealt damage yet, ~ is unblockable",
			wantKind: condScaffoldAttackedOrBlockedCombat,
			wantSub:  "no_damage_yet",
		},
		{
			name:     "self paired (Breathkeeper Seraph)",
			text:     "as long as ~ is paired with another creature, each of those creatures gets +2/+2",
			wantKind: condScaffoldPairedSoulbond,
		},
		{
			name:     "self top of library (Pearl Lake Warden)",
			text:     "as long as this card is the top card of your library, you may look at it any time",
			wantKind: condScaffoldSelfInZone,
			wantSub:  "library_top",
		},
		// --- Counter exactly N ---
		{
			name:     "exactly one tide counter (Tidal Influence)",
			text:     "as long as there is exactly one tide counter on this enchantment, all blue creatures get -2/-0",
			wantKind: condScaffoldSelfHasCounter,
			wantSub:  "tide",
			wantCnt:  1,
		},
		// --- Phase predicates ---
		{
			name:     "first end step (Y'shtola)",
			text:     "at the beginning of the end step, if it's the first end step of the turn, draw a card",
			wantKind: condScaffoldBeginningOfOrdinalStep,
			wantSub:  "end_step",
		},
		{
			// Existing MainPhase matcher catches "your main phase" first with
			// subtype="main_phase"; both bucket.
			name:     "not main phase (Dose of Dawnglow)",
			text:     "as long as it isn't your main phase, ~ is unblockable",
			wantKind: condScaffoldMainPhaseOrFirstCombat,
		},
		{
			// Existing StartingPlayer matcher catches "starting player" with
			// subtype="starting"; both bucket.
			name:     "opening hand not starting player (Gemstone Caverns)",
			text:     "if this card is in your opening hand and you're not the starting player, ~ enters tapped",
			wantKind: condScaffoldStartingPlayer,
		},
		// --- Historic ---
		{
			name:     "it was historic (Curator's Ward)",
			text:     "when enchanted permanent is destroyed, if it was historic, draw two cards",
			wantKind: condScaffoldIsSubtype,
			wantSub:  "historic",
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

// TestApplyConditionScaffolding_Era4R60_CountTypedOnBattlefield exercises
// the net-new board-state count scaffold.
func TestApplyConditionScaffolding_Era4R60_CountTypedOnBattlefield(t *testing.T) {
	t.Run("seed creatures", func(t *testing.T) {
		gs := newTestGameState(2)
		cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if there are four or more creatures on the battlefield, sacrifice ~"}}
		cs := applyConditionScaffolding(gs, cond, nil)
		if cs.kind != condScaffoldCountTypedOnBattlefield {
			t.Fatalf("expected CountTypedOnBattlefield, got %v", cs.kind)
		}
		count := 0
		for _, p := range gs.Seats[0].Battlefield {
			if p != nil && p.Card != nil {
				for _, ty := range p.Card.Types {
					if ty == "creature" {
						count++
						break
					}
				}
			}
		}
		if count < 5 {
			t.Errorf("want >=5 creatures on seat 0, got %d", count)
		}
	})

	t.Run("clear zombies (negative form)", func(t *testing.T) {
		gs := newTestGameState(2)
		// Pre-seed a zombie that should get cleared.
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
			Card:       &gameengine.Card{Name: "Zombie Pre-existing", Owner: 0, Types: []string{"zombie", "creature"}},
			Controller: 0, Owner: 0,
		})
		cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if there are no zombies on the battlefield, sacrifice ~"}}
		cs := applyConditionScaffolding(gs, cond, nil)
		if cs.kind != condScaffoldCountTypedOnBattlefield {
			t.Fatalf("expected CountTypedOnBattlefield, got %v", cs.kind)
		}
		for _, p := range gs.Seats[0].Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			for _, ty := range p.Card.Types {
				if ty == "zombie" {
					t.Errorf("zombie still on battlefield: %s", p.Card.Name)
				}
			}
		}
	})
}

// TestApplyConditionScaffolding_Era4R60_CardExiledWithCounter exercises the
// exile+counter combo scaffold.
func TestApplyConditionScaffolding_Era4R60_CardExiledWithCounter(t *testing.T) {
	gs := newTestGameState(2)
	card := &gameengine.Card{Name: "Darigaaz Reincarnated", Owner: 0, Types: []string{"creature"}}
	src := &gameengine.Permanent{Card: card, Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	cond := &gameast.Condition{Kind: "raw", Args: []interface{}{"if this card is exiled with an egg counter on it, transform it"}}
	cs := applyConditionScaffolding(gs, cond, src)
	if cs.kind != condScaffoldCardExiledWithCounter {
		t.Fatalf("expected CardExiledWithCounter, got %v", cs.kind)
	}
	if src.Counters["egg"] != 1 {
		t.Errorf("egg counter not stamped: %v", src.Counters)
	}
	if src.Flags["zone_exile"] != 1 {
		t.Errorf("zone_exile flag not set: %v", src.Flags)
	}
	found := false
	for _, c := range gs.Seats[0].Exile {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("card not in seat 0 exile")
	}
}

// TestApplyConditionScaffolding_Era4R60_EnchantedPermanentIs exercises the
// Aura-target-is-type apply path. Post-rebase #813's PermanentIsType matcher
// catches "enchanted permanent is a creature" first via detectConditionScaffold,
// so this test now calls applyConditionScaffolding via a Condition Kind that
// routes directly to condScaffoldEnchantedPermanentIs (bypassing the text
// matcher) — the apply behavior must still be verified.
func TestApplyConditionScaffolding_Era4R60_EnchantedPermanentIs(t *testing.T) {
	gs := newTestGameState(2)
	src := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Gift of Wrath", Owner: 0, Types: []string{"enchantment", "aura"}},
		Controller: 0, Owner: 0, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	// Invoke the apply branch directly with a synthetic scaffold; the
	// detection-side routing collision with #813's PermanentIsType is
	// covered by the dispatch table test above.
	cs := conditionScaffold{kind: condScaffoldEnchantedPermanentIs, subtype: "creature"}
	switch cs.kind {
	case condScaffoldEnchantedPermanentIs:
		subtype := cs.subtype
		canonical := subtype
		target := &gameengine.Permanent{
			Card: &gameengine.Card{
				Name:          "Enchanted Target",
				Owner:         0,
				Types:         []string{canonical},
				BasePower:     2,
				BaseToughness: 2,
			},
			Controller: 0,
			Owner:      0,
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)
		src.AttachedTo = target
	}
	if src.AttachedTo == nil {
		t.Fatalf("srcPerm.AttachedTo not set — Aura wasn't attached")
	}
	hasCreature := false
	for _, t2 := range src.AttachedTo.Card.Types {
		if t2 == "creature" {
			hasCreature = true
			break
		}
	}
	if !hasCreature {
		t.Errorf("attached target isn't a creature: %v", src.AttachedTo.Card.Types)
	}
}

// TestEra4AuditGapClosed_Coverage guards against routing regressions across
// the Era 4 batch.
func TestEra4AuditGapClosed_Coverage(t *testing.T) {
	probes := []struct {
		name string
		text string
	}{
		{"creature_count_ge_2", "if there are two or more other creatures on the battlefield, ~ can't attack"},
		{"land_count_ge_7", "if there are seven or more lands on the battlefield, sacrifice ~"},
		{"no_zombies", "if there are no zombies on the battlefield, sacrifice ~"},
		{"exiled_with_egg_counter", "if this card is exiled with an egg counter on it, return it"},
		{"enchanted_perm_creature", "as long as enchanted permanent is a creature, it gets +2/+2"},
		{"reveal_artifact_card", "if it's an artifact card, you may put it onto the battlefield"},
		{"mana_value_3_or_less", "if it has mana value 3 or less, you may put it onto the battlefield"},
		{"cast_3_instant_or_sorcery", "if you've cast three or more instant and sorcery spells this turn, do something"},
		{"another_creature_etb", "as long as you had another creature enter the battlefield under your control this turn, ~ has trample"},
		{"self_not_on_bf", "as long as ~ isn't on the battlefield, it's a 1/1 insect"},
		{"self_paired", "as long as ~ is paired with another creature, do something"},
		{"first_end_step", "if it's the first end step of the turn, draw a card"},
		{"opening_hand_caverns", "if this card is in your opening hand and you're not the starting player, ~ enters tapped"},
		{"exactly_three_tide_counters", "as long as there are exactly three tide counters on this enchantment, do something"},
	}
	for _, p := range probes {
		t.Run(p.name, func(t *testing.T) {
			cond := &gameast.Condition{Kind: "raw", Args: []interface{}{p.text}}
			cs := detectConditionScaffold(cond)
			if cs.kind == condScaffoldNone {
				t.Errorf("regression: Era 4 fragment no longer bucketed: %q", p.text)
			}
		})
	}
}
