package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// postcombat_trigger_value_r60_test.go — pins the R60 round 10+ post-
// combat trigger family added to ChooseAttackers (companion to
// attack_trigger_value_r60_test.go). Two layers:
//   1. postcombatTriggerBonus unit tests pin each categorization branch.
//   2. ChooseAttackers integration confirms a marginal attacker with a
//      postcombat trigger now clears the swing threshold.

// mkPostcombatCard builds an attacker Card with the given oracle text
// (Triggered.Raw → OracleTextLower picks it up) and P/T.
func mkPostcombatCard(name, oracle string, power, toughness int) *gameengine.Card {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{Raw: oracle},
		},
	}
	return &gameengine.Card{
		Name:          name,
		AST:           ast,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
	}
}

// -----------------------------------------------------------------------
// postcombatTriggerBonus categorization
// -----------------------------------------------------------------------

func TestPostcombatTriggerBonus_NilCardIsZero(t *testing.T) {
	if got := postcombatTriggerBonus(nil); got != 0 {
		t.Errorf("nil card should return 0; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_VanillaCreatureIsZero(t *testing.T) {
	c := mkPostcombatCard("Grizzly Bears", "", 2, 2)
	if got := postcombatTriggerBonus(c); got != 0 {
		t.Errorf("vanilla creature should return 0; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_AureliaExtraCombat(t *testing.T) {
	// Aurelia's actual oracle: "Whenever Aurelia, the Warleader attacks
	// for the first time each turn ... after this phase, there is an
	// additional combat phase."
	c := mkPostcombatCard("Aurelia the Warleader",
		"whenever aurelia attacks for the first time each turn, untap each creature you control. after this phase, there is an additional combat phase",
		3, 4)
	if got := postcombatTriggerBonus(c); got != 0.30 {
		t.Errorf("Aurelia (extra combat) should be +0.30; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_ScourgeExtraCombatViaPhrase(t *testing.T) {
	// Scourge of the Throne reads: "Whenever Scourge of the Throne deals
	// combat damage to a player ... after this phase, there is an
	// additional combat phase."
	c := mkPostcombatCard("Scourge of the Throne",
		"dethrone. whenever scourge of the throne deals combat damage to the player with the most life or tied for most life, after this phase, there is an additional combat phase",
		5, 4)
	// Extra-combat wins priority over the combat-damage bucket.
	if got := postcombatTriggerBonus(c); got != 0.30 {
		t.Errorf("Scourge (extra combat) should be +0.30 (extra combat wins priority); got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_ToskiDraw(t *testing.T) {
	c := mkPostcombatCard("Toski Bearer of Secrets",
		"whenever toski bearer of secrets deals combat damage to a player, draw a card",
		1, 1)
	if got := postcombatTriggerBonus(c); got != 0.20 {
		t.Errorf("Toski (combat damage → draw) should be +0.20; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_ColdEyedSelkieDrawScale(t *testing.T) {
	c := mkPostcombatCard("Cold-Eyed Selkie",
		"islandwalk. whenever cold-eyed selkie deals combat damage to a player, you may draw that many cards",
		1, 1)
	if got := postcombatTriggerBonus(c); got != 0.20 {
		t.Errorf("Cold-Eyed Selkie (draw that many) should be +0.20; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_BidentOpponentVariant(t *testing.T) {
	// Bident-attached creature: "Whenever a creature you control deals
	// combat damage to a player, you may draw a card." Phrased as
	// "to a player" — matches the dominant needle.
	c := mkPostcombatCard("Bident Bearer",
		"whenever a creature you control deals combat damage to a player, you may draw a card",
		3, 3)
	if got := postcombatTriggerBonus(c); got != 0.20 {
		t.Errorf("Bident bearer (combat damage to player → draw) should be +0.20; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_GenericDamageNoDrawIsLowerBucket(t *testing.T) {
	// "Whenever X deals combat damage to a player, [non-draw payoff]"
	c := mkPostcombatCard("Tetsuko Shape",
		"whenever a creature you control deals combat damage to an opponent, that player loses 2 life",
		1, 3)
	if got := postcombatTriggerBonus(c); got != 0.15 {
		t.Errorf("combat-damage trigger w/ non-draw payoff should be +0.15; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_EndOfCombatGeneric(t *testing.T) {
	c := mkPostcombatCard("EOC Generic",
		"at the end of combat, remove a counter from this creature",
		2, 2)
	if got := postcombatTriggerBonus(c); got != 0.10 {
		t.Errorf("generic 'at the end of combat' should be +0.10; got %.2f", got)
	}
}

func TestPostcombatTriggerBonus_BecomesBlocked(t *testing.T) {
	c := mkPostcombatCard("Pump on Block",
		"whenever this creature becomes blocked, it gets +2/+2 until end of turn",
		2, 2)
	if got := postcombatTriggerBonus(c); got != 0.10 {
		t.Errorf("becomes-blocked trigger should be +0.10; got %.2f", got)
	}
}

// TestPostcombatTriggerBonus_NonCombatDamageNotMatched pins the false-
// positive guard: a ping trigger like "deals damage to a player" without
// the "combat" qualifier should NOT match — those are zone-of-effect
// triggers (Walking Ballista shape) handled by the activation pipeline.
func TestPostcombatTriggerBonus_NonCombatDamageNotMatched(t *testing.T) {
	c := mkPostcombatCard("Walking Ballista Shape",
		"whenever this creature deals damage to a player, you gain that much life",
		3, 3)
	if got := postcombatTriggerBonus(c); got != 0 {
		t.Errorf("'deals damage to a player' without 'combat' should NOT match (zone ping, not combat); got %.2f", got)
	}
}

// TestPostcombatTriggerBonus_HellriderTextDoesNotDoubleCount confirms
// the disjoint-family property: Hellrider's oracle says "deals 1 damage
// to defending player for each attacking creature" — that's an attack
// trigger (handled by attackTriggerBonus), NOT a postcombat trigger.
// The phrase doesn't contain "deals combat damage to a player" or any
// other postcombat needle, so this helper returns 0.
func TestPostcombatTriggerBonus_HellriderTextDoesNotDoubleCount(t *testing.T) {
	c := mkPostcombatCard("Hellrider",
		"whenever hellrider attacks, hellrider deals 1 damage to defending player for each attacking creature",
		3, 3)
	if got := postcombatTriggerBonus(c); got != 0 {
		t.Errorf("Hellrider (attack trigger only) should return 0 from postcombatTriggerBonus to avoid double-count with attackTriggerBonus; got %.2f",
			got)
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseAttackers tilts toward postcombat-trigger creatures
// -----------------------------------------------------------------------

// TestChooseAttackers_ToskiClearsThresholdWithDrawBonus pins the end-to-
// end fix. A 1/1 Toski (val=0.1 base) would HOLD pre-fix; with the
// +0.20 postcombat-draw bonus it clears the midrange threshold.
func TestChooseAttackers_ToskiClearsThresholdWithDrawBonus(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Turn = 5

	toski := mkPostcombatCard("Toski Bearer of Secrets",
		"whenever toski bearer of secrets deals combat damage to a player, draw a card",
		1, 1)
	atk := &gameengine.Permanent{
		Card:       toski,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atk)
	// Opp with a fat blocker so the open-lane bonus doesn't fire.
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	out := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	if len(out) != 1 || out[0] != atk {
		t.Errorf("Toski with postcombat-draw trigger should attack at midrange threshold; got %v", out)
	}
}

// TestChooseAttackers_AureliaExtraCombatOutscoresVanilla pins the
// strongest postcombat trigger (extra combat): Aurelia-shape attackers
// should attack at least as readily as a vanilla of the same P/T.
// Counterfactual: with no blocker prune blocking the attack, Aurelia
// (val += 0.30) ≥ vanilla score.
func TestChooseAttackers_AureliaExtraCombatOutscoresVanilla(t *testing.T) {
	mkGS := func(card *gameengine.Card) (*gameengine.GameState, *gameengine.Permanent) {
		gs := newTestGame(t, 2)
		gs.Seats[0].Life = 40
		gs.Seats[1].Life = 40
		gs.Turn = 5
		p := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)
		return gs, p
	}

	aurelia := mkPostcombatCard("Aurelia",
		"whenever aurelia attacks for the first time each turn ... after this phase, there is an additional combat phase",
		3, 4)
	vanilla := mkPostcombatCard("Plain 3/4", "", 3, 4)

	gsA, atkA := mkGS(aurelia)
	gsV, atkV := mkGS(vanilla)

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	outA := h.ChooseAttackers(gsA, 0, []*gameengine.Permanent{atkA})
	outV := h.ChooseAttackers(gsV, 0, []*gameengine.Permanent{atkV})

	// Monotonicity: if vanilla attacks, Aurelia must attack; if Aurelia
	// doesn't attack, vanilla must not either. (Aurelia has a strict
	// +0.30 bonus on top of identical base, so its decision can't be
	// strictly worse.)
	vanillaAttacks := len(outV) == 1
	aureliaAttacks := len(outA) == 1
	if vanillaAttacks && !aureliaAttacks {
		t.Errorf("Aurelia (extra combat +0.30) must attack in any scenario the vanilla attacks; vanilla=%v aurelia=%v",
			vanillaAttacks, aureliaAttacks)
	}
}

// TestChooseAttackers_HellriderAndToskiStackBonuses confirms the two
// helpers compose correctly. A hypothetical Hellrider-Toski hybrid
// ("whenever X attacks, deals 1 damage" PLUS "whenever X deals combat
// damage to a player, draw a card") should get BOTH bonuses
// (attackTriggerBonus = +0.20 for damage trigger, postcombatTriggerBonus
// = +0.20 for combat-damage draw) — confirming the helpers are
// independent and accumulate when both apply.
func TestChooseAttackers_HellriderAndToskiHybridStacksBonuses(t *testing.T) {
	c := mkPostcombatCard("Hybrid",
		"whenever hybrid attacks, hybrid deals 1 damage to defending player. whenever hybrid deals combat damage to a player, draw a card",
		2, 2)
	attackBonus := attackTriggerBonus(c)
	postBonus := postcombatTriggerBonus(c)
	if attackBonus != 0.20 {
		t.Errorf("hybrid attack-side should be +0.20 (damage trigger); got %.2f", attackBonus)
	}
	if postBonus != 0.20 {
		t.Errorf("hybrid postcombat-side should be +0.20 (draw on combat damage); got %.2f", postBonus)
	}
	// Sanity: a fictional 2-power Hybrid would carry val = 0.2 (power)
	// + 0.20 (attack damage) + 0.20 (postcombat draw) = 0.60 before
	// any other bonuses. That's well over any reasonable threshold,
	// matching the "double-trigger value-attacker" intuition.
}
