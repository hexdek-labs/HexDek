package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// self_trigger_matrix_r60_test.go — full matrix coverage for the self-
// trigger response framework (PRs #410 + #413). Companion to the
// scenario-specific unit tests in self_trigger_response_r60_test.go and
// self_harm_signals_r60_test.go. This file enforces the cross-product
// contract: every (source-kind × harm-type × lethal? × controller)
// cell should produce the documented counter / don't-counter outcome.
//
// Matrix dimensions:
//   - Source kind:    triggered, activated, spell
//   - Trigger shape:  creature ETB, upkeep, landfall (all triggered;
//                     differ only in payload phrasing)
//   - Harm type:      draw-damage (PR #410), mill (PR #413),
//                     life-loss-stated (PR #413), damage-to-self
//                     (life-loss family — "deals N damage to you"),
//                     sacrifice (NOT in framework — confirm no fire),
//                     none (vanilla ETB — confirm no fire)
//   - Lethal?:        yes, no, n/a
//   - Controller:     self, opponent
//
// Many cells collapse: any spell, any activated, any opp-controlled —
// always don't-counter regardless of harm type. The matrix below
// covers each meaningful combination once, plus end-to-end integration
// tests for each lethal harm-type via ChooseResponse.

// -----------------------------------------------------------------------
// Helpers (named with mtx prefix to avoid colliding with other
// test files in the same package).
// -----------------------------------------------------------------------

// mtxBuildSource builds a source permanent with the given oracle text.
// Used for both ETB-shape (creature) and upkeep-shape (enchantment).
func mtxBuildSource(name, typeStr, oracle string) *gameengine.Permanent {
	c := &gameengine.Card{
		Name:  name,
		Types: []string{typeStr},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Triggered{Raw: oracle},
			},
		},
	}
	return &gameengine.Permanent{Card: c, Flags: map[string]int{}}
}

// mtxTriggered constructs a triggered StackItem with given source and
// controller.
func mtxTriggered(src *gameengine.Permanent, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{Kind: "triggered", Controller: controller, Source: src}
}

// mtxActivated constructs an activated StackItem.
func mtxActivated(src *gameengine.Permanent, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{Kind: "activated", Controller: controller, Source: src}
}

// mtxSpell constructs a spell StackItem (Card set, Source nil, Kind "spell").
func mtxSpell(card *gameengine.Card, controller int) *gameengine.StackItem {
	return &gameengine.StackItem{Kind: "spell", Card: card, Controller: controller}
}

// mtxFreshGS returns a 2-seat game with life/library defaults; harm
// scenarios mutate as needed via the setup field.
func mtxFreshGS(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Seats[0].Library = make([]*gameengine.Card, 60)
	gs.Seats[1].Library = make([]*gameengine.Card, 60)
	return gs
}

// -----------------------------------------------------------------------
// The matrix
// -----------------------------------------------------------------------

type mtxCase struct {
	name          string
	build         func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem)
	expectCounter bool
}

func runMatrix(t *testing.T, cases []mtxCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gs, top := tc.build(t)
			h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
			got := h.shouldCounterOwnTrigger(gs, 0, top)
			if got != tc.expectCounter {
				t.Errorf("expected shouldCounterOwnTrigger=%v, got %v", tc.expectCounter, got)
			}
		})
	}
}

// -----------------------------------------------------------------------
// Source-kind axis: triggered vs activated vs spell
// -----------------------------------------------------------------------

func TestSelfTriggerMatrix_SourceKindAxis(t *testing.T) {
	runMatrix(t, []mtxCase{
		{
			name: "spell_self_cast_at_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				spell := &gameengine.Card{Name: "Concentrate", Types: []string{"sorcery"},
					AST: &gameast.CardAST{Name: "Concentrate",
						Abilities: []gameast.Ability{&gameast.Static{Raw: "draw three cards"}}}}
				return gs, mtxSpell(spell, 0)
			},
			expectCounter: false, // spells never self-counter
		},
		{
			name: "activated_self_at_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Bob Source", "creature", "draw three cards")
				return gs, mtxActivated(src, 0)
			},
			expectCounter: false, // activations are intentional
		},
		{
			name: "triggered_self_at_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 2
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true, // triggered self-harm → counter
		},
	})
}

// -----------------------------------------------------------------------
// Controller axis: self vs opponent
// -----------------------------------------------------------------------

func TestSelfTriggerMatrix_ControllerAxis(t *testing.T) {
	runMatrix(t, []mtxCase{
		{
			name: "opp_controlled_lethal_draw_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 2
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Opp Source", "creature",
					"when this enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 1) // controller = 1 (opp)
			},
			expectCounter: false, // opp-controlled triggers are not our problem here
		},
		{
			name: "opp_controlled_lethal_mill_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Library = make([]*gameengine.Card, 1)
				src := mtxBuildSource("Hedron Crab", "creature",
					"whenever a land enters, target player mills 10 cards")
				return gs, mtxTriggered(src, 1)
			},
			expectCounter: false,
		},
	})
}

// -----------------------------------------------------------------------
// Harm-type axis × lethality axis
// -----------------------------------------------------------------------

func TestSelfTriggerMatrix_HarmTypeAxis(t *testing.T) {
	runMatrix(t, []mtxCase{
		// --- creature ETB / no harm: vanilla creature with no trigger ---
		{
			name: "creature_etb_vanilla_no_harm_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				src := mtxBuildSource("Vanilla Bear", "creature", "")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false, // no harm projection → no fire
		},
		// --- creature ETB / draw-damage / lethal ---
		{
			name: "creature_etb_draw_damage_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 2
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// --- creature ETB / draw-damage / non-lethal ---
		{
			name: "creature_etb_draw_damage_non_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 40
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
		// --- landfall / mill / lethal (decks us out) ---
		{
			name: "landfall_self_mill_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Library = make([]*gameengine.Card, 2)
				src := mtxBuildSource("Hedron Crab", "creature",
					"landfall — whenever a land you control enters, target player mills 3 cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// --- landfall / mill / non-lethal (library abundant) ---
		{
			name: "landfall_self_mill_non_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Library = make([]*gameengine.Card, 50)
				src := mtxBuildSource("Hedron Crab", "creature",
					"whenever a land enters, target player mills 3 cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
		// --- upkeep / stated life-loss / lethal ---
		{
			name: "upkeep_life_loss_stated_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				src := mtxBuildSource("Phyrexian Arena", "enchantment",
					"at the beginning of your upkeep, you draw a card and you lose 2 life")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// --- upkeep / stated life-loss / non-lethal ---
		{
			name: "upkeep_life_loss_stated_non_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 40
				src := mtxBuildSource("Phyrexian Arena", "enchantment",
					"at the beginning of your upkeep, you lose 1 life")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
		// --- damage-to-self / lethal (same family as life-loss but via
		//     "deals N damage to you" phrasing) ---
		{
			name: "creature_etb_damage_to_self_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 3
				src := mtxBuildSource("Mistmeadow Skulk Knockoff", "creature",
					"when this creature enters, it deals 5 damage to you")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// --- damage-to-self / non-lethal ---
		{
			name: "creature_etb_damage_to_self_non_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 40
				src := mtxBuildSource("Self Dinger", "creature",
					"when this creature enters, it deals 2 damage to you")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
		// --- sacrifice / not-in-framework: hat picks what to sac so the
		//     trigger itself is not lethal projection material ---
		{
			name: "sacrifice_trigger_self_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				// Sacrifice triggers don't match any harm-projection helper.
				// We CHOOSE what to sacrifice — the trigger itself isn't
				// projecting lethal damage / mill / life-loss.
				src := mtxBuildSource("Smallpox Shape", "enchantment",
					"at the beginning of your upkeep, each player sacrifices a creature and a land")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false, // out of framework scope — explicit non-fire pin
		},
		// --- discard / not-in-framework (same shape as sacrifice — choice-based) ---
		{
			name: "discard_trigger_self_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				src := mtxBuildSource("Discard Shape", "enchantment",
					"at the beginning of your upkeep, you discard a card")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
	})
}

// -----------------------------------------------------------------------
// Edge cases: stacking, variable amounts, boundary lethality
// -----------------------------------------------------------------------

func TestSelfTriggerMatrix_EdgeCases(t *testing.T) {
	runMatrix(t, []mtxCase{
		// Damage-on-draw punishers stack: 2 punishers × 2 cards drawn =
		// 4 damage. At 3 life → lethal.
		{
			name: "stacked_punishers_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 3
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				mkPunisher(gs, 0, "Curse of Wizardry",
					"whenever a player draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// Boundary: exact equality (damage == life) is lethal per
		// the helper's >= contract.
		{
			name: "exact_lethal_boundary_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 2
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// Just-under-lethal: 2 dmg at 3 life is non-lethal.
		{
			name: "just_under_lethal_skips",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 3
				mkPunisher(gs, 0, "Underworld Dreams",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false,
		},
		// Variable life-loss (Dark Confidant's "lose life equal to its
		// CMC") intentionally NOT matched even at 1 life — we can't
		// project without per-card state.
		{
			name: "variable_life_loss_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				src := mtxBuildSource("Dark Confidant", "creature",
					"at the beginning of your upkeep, reveal the top card. you lose life equal to its converted mana cost")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false, // helper intentionally doesn't match variable amounts
		},
		// Library exactly equal to mill count — lethal (would deck on
		// next draw per the >= contract in triggerCausesLethalMill).
		{
			name: "library_equals_mill_count_lethal_fires",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Library = make([]*gameengine.Card, 5)
				src := mtxBuildSource("Big Self-Mill", "creature",
					"when this creature enters, you mill 5 cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: true,
		},
		// Damage-on-opp-draw-only punisher (no symmetric phrasing, no
		// canonical name) doesn't fire even at lethal life because the
		// punisher only hits opps when THEY draw — not us.
		{
			name: "opp_only_punisher_does_not_fire",
			build: func(t *testing.T) (*gameengine.GameState, *gameengine.StackItem) {
				gs := mtxFreshGS(t)
				gs.Seats[0].Life = 1
				mkPunisher(gs, 0, "Asymmetric Custom Punisher",
					"whenever an opponent draws a card, that player loses 1 life")
				src := mtxBuildSource("Mulldrifter", "creature",
					"when mulldrifter enters the battlefield, draw two cards")
				return gs, mtxTriggered(src, 0)
			},
			expectCounter: false, // asymmetric (opp-only) punisher doesn't hit us
		},
	})
}

// -----------------------------------------------------------------------
// End-to-end ChooseResponse integration for each lethal scenario
// -----------------------------------------------------------------------

// mtxGiveCounterAndMana stages a Counterspell + 2 Islands on seat 0
// so ChooseResponse has a payable counter available.
func mtxGiveCounterAndMana(gs *gameengine.GameState) {
	for i := 0; i < 2; i++ {
		island := &gameengine.Card{Name: "Island", Types: []string{"land"}, TypeLine: "Basic Land — Island"}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield,
			&gameengine.Permanent{Card: island, Controller: 0, Owner: 0, Flags: map[string]int{}})
	}
	counter := &gameengine.Card{
		Name: "Counterspell", Types: []string{"instant", "cost:2"}, ManaCostString: "{U}{U}",
		AST: &gameast.CardAST{Name: "Counterspell",
			Abilities: []gameast.Ability{
				&gameast.Activated{Cost: gameast.Cost{}, Effect: &gameast.CounterSpell{}},
			}},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, counter)
}

func TestSelfTriggerMatrix_E2E_DrawDamageLethalCountered(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Mulldrifter", "creature",
		"when mulldrifter enters the battlefield, draw two cards")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil || resp.Card == nil || resp.Card.Name != "Counterspell" {
		t.Errorf("expected Counterspell response; got %v", resp)
	}
}

func TestSelfTriggerMatrix_E2E_MillLethalCountered(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Library = make([]*gameengine.Card, 2)
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Hedron Crab", "creature",
		"whenever a land enters, target player mills 5 cards")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil || resp.Card == nil || resp.Card.Name != "Counterspell" {
		t.Errorf("expected Counterspell response on self-mill deck-out; got %v", resp)
	}
}

func TestSelfTriggerMatrix_E2E_LifeLossLethalCountered(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 1
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Phyrexian Arena", "enchantment",
		"at the beginning of your upkeep, you lose 2 life")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil || resp.Card == nil || resp.Card.Name != "Counterspell" {
		t.Errorf("expected Counterspell response on self-life-loss lethal; got %v", resp)
	}
}

func TestSelfTriggerMatrix_E2E_DamageToSelfLethalCountered(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 2
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Self Dinger", "creature",
		"when this creature enters, it deals 5 damage to you")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp == nil || resp.Card == nil || resp.Card.Name != "Counterspell" {
		t.Errorf("expected Counterspell response on self-damage lethal; got %v", resp)
	}
}

func TestSelfTriggerMatrix_E2E_SacrificeNotCountered(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 1
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Smallpox Shape", "enchantment",
		"at the beginning of your upkeep, each player sacrifices a creature and a land")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	resp := h.ChooseResponse(gs, 0, top)
	if resp != nil {
		t.Errorf("sacrifice trigger should NOT be self-countered (we choose what to sac); got %v", resp.Card)
	}
}

func TestSelfTriggerMatrix_E2E_NoLethalNoCounter(t *testing.T) {
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 40 // comfortable
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	mtxGiveCounterAndMana(gs)
	src := mtxBuildSource("Mulldrifter", "creature",
		"when mulldrifter enters the battlefield, draw two cards")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if resp := h.ChooseResponse(gs, 0, top); resp != nil {
		t.Errorf("comfortable life should NOT trigger self-counter; got %v", resp.Card)
	}
}

func TestSelfTriggerMatrix_E2E_NoCounterInHand(t *testing.T) {
	// Lethal self-trigger but no counterspell — ChooseResponse should
	// return nil (can't counter what we don't have).
	gs := mtxFreshGS(t)
	gs.Seats[0].Life = 2
	mkPunisher(gs, 0, "Underworld Dreams",
		"whenever an opponent draws a card, that player loses 1 life")
	// No counterspell or mana.
	src := mtxBuildSource("Mulldrifter", "creature",
		"when mulldrifter enters the battlefield, draw two cards")
	top := mtxTriggered(src, 0)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	if resp := h.ChooseResponse(gs, 0, top); resp != nil {
		t.Errorf("lethal self-trigger with no counter in hand should return nil; got %v", resp.Card)
	}
}
