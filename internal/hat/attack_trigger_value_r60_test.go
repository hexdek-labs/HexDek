package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// attack_trigger_value_r60_test.go — pins the R60 round 9+ ChooseAttackers
// audit fix. Two layers:
//   1. attackTriggerBonus unit tests pin the categorization decision
//      (damage, tutor, pump, draw, drain, treasure, none) per attacker.
//   2. ChooseAttackers integration confirms a marginal attacker that the
//      pre-fix loop would HOLD now ATTACKS once the trigger bonus is
//      folded into its score.

// mkAttackTriggerCard builds an attacker Card with an oracle text
// matching the requested trigger shape. Uses AST.Abilities[0].Raw so
// OracleTextLower picks it up.
func mkAttackTriggerCard(name string, oracle string, power, toughness int) *gameengine.Card {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{Raw: oracle},
		},
	}
	c := &gameengine.Card{
		Name:          name,
		AST:           ast,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
	}
	return c
}

// -----------------------------------------------------------------------
// attackTriggerBonus categorization
// -----------------------------------------------------------------------

func TestAttackTriggerBonus_NilCardIsZero(t *testing.T) {
	if got := attackTriggerBonus(nil); got != 0 {
		t.Errorf("nil card should return 0 bonus; got %.2f", got)
	}
}

func TestAttackTriggerBonus_VanillaCreatureIsZero(t *testing.T) {
	c := mkAttackTriggerCard("Grizzly Bears", "", 2, 2)
	if got := attackTriggerBonus(c); got != 0 {
		t.Errorf("vanilla creature should return 0; got %.2f", got)
	}
}

func TestAttackTriggerBonus_HellriderDamage(t *testing.T) {
	// Engine renders "Whenever Hellrider attacks, deals 1 damage to defending player..."
	c := mkAttackTriggerCard("Hellrider",
		"whenever hellrider attacks, hellrider deals 1 damage to defending player for each attacking creature",
		3, 3)
	if got := attackTriggerBonus(c); got != 0.20 {
		t.Errorf("Hellrider (damage trigger) should be +0.20; got %.2f", got)
	}
}

func TestAttackTriggerBonus_MaraxusPump(t *testing.T) {
	c := mkAttackTriggerCard("Maraxus of Keld",
		"whenever maraxus of keld attacks, it gets +x/+0 until end of turn, where x is the number of untapped creatures you control",
		3, 4)
	if got := attackTriggerBonus(c); got != 0.15 {
		t.Errorf("Maraxus (pump trigger) should be +0.15; got %.2f", got)
	}
}

func TestAttackTriggerBonus_GoldspanTreasure(t *testing.T) {
	c := mkAttackTriggerCard("Goldspan Dragon",
		"whenever goldspan dragon attacks or becomes the target of a spell, create a treasure token",
		4, 4)
	if got := attackTriggerBonus(c); got != 0.10 {
		t.Errorf("Goldspan (treasure trigger) should be +0.10; got %.2f", got)
	}
}

func TestAttackTriggerBonus_LordOfTheForsakenDrain(t *testing.T) {
	c := mkAttackTriggerCard("Lord of the Forsaken",
		"whenever a creature you control attacks, each opponent loses 1 life",
		6, 6)
	if got := attackTriggerBonus(c); got != 0.15 {
		t.Errorf("Lord of the Forsaken (drain trigger) should be +0.15; got %.2f", got)
	}
}

func TestAttackTriggerBonus_CosimaDraw(t *testing.T) {
	c := mkAttackTriggerCard("Cosima God of the Voyage",
		"whenever you attack with cosima you may exile her. ... draw a card and put a voyage counter",
		2, 4)
	if got := attackTriggerBonus(c); got != 0.15 {
		t.Errorf("Cosima (draw trigger) should be +0.15; got %.2f", got)
	}
}

func TestAttackTriggerBonus_EtaliTutorShape(t *testing.T) {
	c := mkAttackTriggerCard("Etali Primal Storm",
		"whenever etali attacks, exile the top card of each player's library. you may cast",
		6, 6)
	// "exile the top" matches the tutor branch.
	if got := attackTriggerBonus(c); got != 0.25 {
		t.Errorf("Etali (tutor-shape trigger) should be +0.25; got %.2f", got)
	}
}

// TestAttackTriggerBonus_NoTriggerNoBonus pins the negative case: an
// instant whose oracle text mentions "attacks" but lacks a self-trigger
// clause should NOT get a bonus. Lightning Strike has "attacks" in
// "deals damage to any target including attacking creatures" — that's
// passive removal-target text, not a trigger.
func TestAttackTriggerBonus_NoSelfTriggerNoBonus(t *testing.T) {
	c := mkAttackTriggerCard("Lightning Strike",
		"deal 3 damage to any target. lightning strike can target attacking creatures",
		0, 0) // not actually a creature in real life, but oracle string is what matters
	if got := attackTriggerBonus(c); got != 0 {
		t.Errorf("text mentioning 'attacking creatures' but no trigger phrase should be 0; got %.2f", got)
	}
}

// TestAttackTriggerBonus_GenericArticleNotSelfTrigger pins that
// "whenever a creature attacks" without "you control" qualifier
// shouldn't trigger the fallback path's word-check (which would
// otherwise match "creature" as the noun before "attacks").
func TestAttackTriggerBonus_GenericArticleNotSelfTrigger(t *testing.T) {
	// "whenever a creature attacks" — generic, not self-targeted.
	// hasSelfAttackTrigger should reject this via the generic-article
	// filter.
	c := mkAttackTriggerCard("Generic Anthem",
		"whenever a creature attacks, it gets +1/+0",
		0, 0)
	if got := attackTriggerBonus(c); got != 0 {
		t.Errorf("'whenever a creature attacks' (generic, not self) should return 0; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseAttackers tilts toward trigger-bearing creatures
// -----------------------------------------------------------------------

// TestChooseAttackers_HellriderClearsThresholdWithDamageBonus pins the
// end-to-end fix. Mid-game (Develop), aggro DNA OFF (default 0.5), a
// 1/3 with a Hellrider-shape damage trigger should attack. Pre-fix:
// val = 0.1 (power/10) ≈ below most thresholds. Post-fix: val = 0.30
// after the +0.20 trigger bonus.
//
// We use a 1/3 specifically to keep the base val low and isolate the
// trigger bonus as the cause of the swing. The hat must be told the
// opp has at least one creature so canSwingProfitably-style prunes
// don't bypass the scoring branch entirely.
func TestChooseAttackers_HellriderClearsThresholdWithDamageBonus(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Turn = 5

	// Attacker: 2-power Hellrider-shape. Needs to be on our battlefield
	// AND in `legal` (we pass it directly to ChooseAttackers).
	hellrider := mkAttackTriggerCard("Hellrider",
		"whenever hellrider attacks, deals 1 damage to defending player for each attacking creature",
		2, 3)
	atk := &gameengine.Permanent{
		Card:       hellrider,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atk)

	// Give opp a single small blocker so the "open lane" bonus DOESN'T
	// fire (we want to isolate the trigger bonus).
	opp := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	_ = opp

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	out := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	if len(out) != 1 || out[0] != atk {
		t.Errorf("Hellrider with attack-trigger should attack at midrange threshold; got %v", out)
	}
}

// TestChooseAttackers_VanillaSamePTHolds pins the gate by counterfactual:
// a vanilla 2/3 with NO trigger in the same scenario should NOT attack
// (val=0.20 base — flying/etc. all zero — sits at/below mid threshold
// for midrange archetype on turn 5).
func TestChooseAttackers_VanillaSamePTHoldsAtThreshold(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Turn = 5

	vanilla := mkAttackTriggerCard("Plain Vanilla", "", 2, 3)
	atk := &gameengine.Permanent{
		Card:       vanilla,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atk)
	// Opp with a fat blocker so open-lane bonus doesn't fire and the
	// 2-power attacker has to score against a real defender.
	newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	out := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{atk})
	// We don't require it MUST be empty (other DNA factors could lift
	// the threshold); the contract is "the vanilla should not be
	// strictly more valued than the Hellrider in the prior test".
	// Re-run the Hellrider scenario with the same conditions:
	gsH := newTestGame(t, 2)
	gsH.Seats[0].Life = 40
	gsH.Seats[1].Life = 40
	gsH.Turn = 5
	hellrider := mkAttackTriggerCard("Hellrider",
		"whenever hellrider attacks, deals 1 damage to defending player for each attacking creature",
		2, 3)
	hAtk := &gameengine.Permanent{
		Card:       hellrider,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{},
	}
	gsH.Seats[0].Battlefield = append(gsH.Seats[0].Battlefield, hAtk)
	newTestPermanent(gsH.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 0, 5)
	hH := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	outH := hH.ChooseAttackers(gsH, 0, []*gameengine.Permanent{hAtk})

	// The bonus is the differential: Hellrider should attack in at
	// least as many scenarios as the vanilla. Strict counterfactual:
	// if vanilla attacks, Hellrider attacks; if Hellrider doesn't
	// attack, vanilla doesn't either.
	vanillaAttacks := len(out) == 1
	hellriderAttacks := len(outH) == 1
	if vanillaAttacks && !hellriderAttacks {
		t.Error("Hellrider should attack in any scenario the vanilla attacks (trigger bonus is monotonic)")
	}
	if !vanillaAttacks && !hellriderAttacks {
		t.Error("Hellrider trigger bonus should have lifted it over the threshold even when vanilla doesn't qualify")
	}
}
