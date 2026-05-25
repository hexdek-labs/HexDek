package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// trick_attacks_r60_test.go — R60 trick-attack signals.
//
// Survey: ChooseAttackers' `canSwingProfitably` prune drops attackers
// that "would die to clean block on every target" unless they're
// flagged strategic (commander / value-engine / combo piece). That
// gate misses a whole class of creatures whose DEATH is the value:
//
//   - Die-trigger creatures (Hangarback Walker, Doomed Traveler,
//     Solemn Simulacrum) — dying creates tokens / draws / ramps.
//   - Persist / Undying / Unearth recursion — Murderous Redcap,
//     Geralf's Messenger come back after the trade with extra value
//     (re-triggered damage, +1/+1 counter restored).
//   - Damage-taken triggers (Stuffy Doll, Boros Reckoner) — incoming
//     damage IS the payoff.
//   - Aristocrats sac-fuel — cheap creatures attacking while a sac
//     outlet (Phyrexian Altar, Goblin Bombardment, Viscera Seer) sits
//     untapped at home; the block becomes a tempo trade where WE
//     choose timing.
//
// Two new signals address these:
//   - `hasDeathPayoffValue(c)`: oracle scan for self-death triggers,
//     damage-taken triggers, and persist/undying/unearth keywords.
//     Adds +0.20 to attack-value and shields from the swing-profit
//     prune.
//   - `hasSacFuelValue(gs, seatIdx, p)`: cheap (≤2 CMC) creature
//     with a sac outlet on the same battlefield. Adds +0.15 and
//     shields from the prune.

// -----------------------------------------------------------------------------
// hasDeathPayoffValue
// -----------------------------------------------------------------------------

func deathPayoffCard(name string, oracle string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"creature"}, 2,
		&gameast.CardAST{
			Name:      name,
			Abilities: []gameast.Ability{&gameast.Static{Raw: oracle}},
		})
}

// TestHasDeathPayoffValue_PositiveCases pins the four oracle-shape
// patterns that should classify as "death is the value":
//   - "when this dies, ..." (Hangarback Walker, Doomed Traveler)
//   - "is dealt damage" trigger (Boros Reckoner, Stuffy Doll)
//   - persist keyword (Murderous Redcap, Kitchen Finks)
//   - undying / unearth keywords
func TestHasDeathPayoffValue_PositiveCases(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Hangarback Walker", "When this dies, create X 1/1 colorless Thopter artifact creature tokens with flying."},
		{"Doomed Traveler", "When this creature dies, create a 1/1 white Spirit creature token with flying."},
		{"Boros Reckoner", "Whenever this is dealt damage, it deals that much damage to any target."},
		{"Murderous Redcap", "Persist (When this creature dies, return it to the battlefield under its owner's control with a -1/-1 counter on it.)"},
		{"Geralf's Messenger", "Undying (When this creature dies, if it had no +1/+1 counters on it, return it under its owner's control with a +1/+1 counter on it.)"},
		{"Crackling Doom", "Unearth {1}{R}"},
	}
	for _, c := range cases {
		card := deathPayoffCard(c.name, c.oracle)
		if !hasDeathPayoffValue(card) {
			t.Errorf("hasDeathPayoffValue(%q) = false; want true (oracle: %q)", c.name, c.oracle)
		}
	}
}

// TestHasDeathPayoffValue_NegativeCases pins that vanilla creatures
// and global death-triggers (Blood Artist) — which need OTHER deaths,
// not their own — don't qualify.
func TestHasDeathPayoffValue_NegativeCases(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
	}{
		{"Grizzly Bears", ""},
		{"Llanowar Elves", "T: Add G."},
		// Blood Artist's trigger is on ANOTHER creature dying — it's a
		// global drain, not a self-death payoff. Attacking with Blood
		// Artist to die-trigger itself yields nothing extra.
		{"Blood Artist", "Whenever a creature dies, target opponent loses 1 life."},
	}
	for _, c := range cases {
		card := deathPayoffCard(c.name, c.oracle)
		if hasDeathPayoffValue(card) {
			t.Errorf("hasDeathPayoffValue(%q) = true; want false (oracle: %q)", c.name, c.oracle)
		}
	}
}

func TestHasDeathPayoffValue_NilSafe(t *testing.T) {
	if hasDeathPayoffValue(nil) {
		t.Errorf("hasDeathPayoffValue(nil) must return false")
	}
}

// -----------------------------------------------------------------------------
// hasSacFuelValue
// -----------------------------------------------------------------------------

func TestHasSacFuelValue_FiresWithOutletPresent(t *testing.T) {
	gs := newTestGame(t, 2)
	// A 1-mana 1/1 attacker.
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Memnite", []string{"creature"}, 1, nil), 1, 1)
	// A sac outlet on the same battlefield.
	outlet := newTestCardMinimal("Phyrexian Altar", []string{"artifact"}, 3,
		&gameast.CardAST{
			Name:      "Phyrexian Altar",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Sacrifice a creature: Add one mana of any color."}},
		})
	newTestPermanent(gs.Seats[0], outlet, 0, 0)

	if !hasSacFuelValue(gs, 0, attacker) {
		t.Errorf("hasSacFuelValue should fire: cheap attacker + sac outlet on board")
	}
}

func TestHasSacFuelValue_NoFireWithoutOutlet(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Memnite", []string{"creature"}, 1, nil), 1, 1)
	if hasSacFuelValue(gs, 0, attacker) {
		t.Errorf("hasSacFuelValue must not fire without a sac outlet on the battlefield")
	}
}

func TestHasSacFuelValue_NoFireOnExpensiveAttacker(t *testing.T) {
	gs := newTestGame(t, 2)
	// 5-mana attacker — too expensive to be sac-fuel even with an outlet.
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Big Threat", []string{"creature"}, 5, nil), 5, 5)
	outlet := newTestCardMinimal("Phyrexian Altar", []string{"artifact"}, 3,
		&gameast.CardAST{
			Name:      "Phyrexian Altar",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Sacrifice a creature: Add one mana of any color."}},
		})
	newTestPermanent(gs.Seats[0], outlet, 0, 0)

	if hasSacFuelValue(gs, 0, attacker) {
		t.Errorf("hasSacFuelValue must not fire on a 5-mana creature — not chump fuel")
	}
}

// TestHasSacFuelValue_IgnoresSpellSacCostsNotOutlets pins that one-shot
// spells with sacrifice in their cost ("Sacrifice a creature: target ...
// gets +X/+X") are NOT on the battlefield as standing outlets, so the
// signal must not over-fire on, say, a Diabolic Edict pattern that
// happens to mention "sacrifice a creature" outside an activated cost.
func TestHasSacFuelValue_IgnoresNonOutletSacrificeText(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Memnite", []string{"creature"}, 1, nil), 1, 1)
	// Note: the text says "sacrifice a creature" but without a colon,
	// so it's not an activated-ability outlet shape.
	notAnOutlet := newTestCardMinimal("Edict in a Can", []string{"artifact"}, 3,
		&gameast.CardAST{
			Name:      "Edict in a Can",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "When this enters the battlefield, target player must sacrifice a creature."}},
		})
	newTestPermanent(gs.Seats[0], notAnOutlet, 0, 0)

	if hasSacFuelValue(gs, 0, attacker) {
		t.Errorf("hasSacFuelValue must not fire on edict-pattern text (no activated-cost colon)")
	}
}

func TestHasSacFuelValue_NilSafe(t *testing.T) {
	gs := newTestGame(t, 2)
	if hasSacFuelValue(nil, 0, nil) {
		t.Errorf("hasSacFuelValue(nil, *, nil) must return false")
	}
	if hasSacFuelValue(gs, -1, nil) {
		t.Errorf("hasSacFuelValue with nil permanent must return false")
	}
}

// -----------------------------------------------------------------------------
// Integration — die-trigger creature is NOT pruned from ChooseAttackers
// -----------------------------------------------------------------------------

// TestChooseAttackers_DieTriggerCreatureSwingsIntoCleanBlock pins the
// end-to-end fix: a Doomed Traveler (1/1 for {W}, dies-makes-token)
// attacking into a 3/3 blocker would have been pruned by
// canSwingProfitably pre-fix. The death-payoff shield must keep it
// in the attackers list.
func TestChooseAttackers_DieTriggerCreatureSwingsIntoCleanBlock(t *testing.T) {
	gs := newTestGame(t, 2)
	// Our die-trigger attacker.
	attacker := newTestPermanent(gs.Seats[0],
		deathPayoffCard("Doomed Traveler",
			"When this creature dies, create a 1/1 white Spirit creature token with flying."),
		1, 1)
	// Opponent's clean blocker — 3/3 untapped will kill the 1/1.
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Hill Giant", []string{"creature"}, 3, nil), 3, 3)

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{attacker})

	found := false
	for _, p := range got {
		if p == attacker {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("die-trigger Doomed Traveler should attack into a clean block; got attackers=%v", got)
	}
}

// TestChooseAttackers_PersistCreatureSwingsIntoCleanBlock — Murderous
// Redcap (2/2 persist) attacking into a 3/3 dies, comes back as 1/1
// with persist counter, and re-triggers its "deals 2 damage when this
// enters" ETB (since persist re-ETBs). The trade is net-positive — the
// shield must keep it.
func TestChooseAttackers_PersistCreatureSwingsIntoCleanBlock(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := newTestPermanent(gs.Seats[0],
		deathPayoffCard("Murderous Redcap",
			"When this enters, it deals 2 damage to any target. Persist (when this dies, return with a -1/-1 counter on it.)"),
		2, 2)
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Hill Giant", []string{"creature"}, 3, nil), 3, 3)

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{attacker})

	found := false
	for _, p := range got {
		if p == attacker {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("persist creature should attack into a clean block; got %d attackers", len(got))
	}
}

// TestChooseAttackers_SacFuelSwingsWithOutletOnBoard — a 1-mana 1/1
// token attacking into a 3/3 with Phyrexian Altar on the board. Pre-
// fix, pruned. The sac-fuel shield must keep it.
func TestChooseAttackers_SacFuelSwingsWithOutletOnBoard(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Goblin Token", []string{"creature"}, 1, nil), 1, 1)
	outlet := newTestCardMinimal("Phyrexian Altar", []string{"artifact"}, 3,
		&gameast.CardAST{
			Name:      "Phyrexian Altar",
			Abilities: []gameast.Ability{&gameast.Static{Raw: "Sacrifice a creature: Add one mana of any color."}},
		})
	newTestPermanent(gs.Seats[0], outlet, 0, 0)
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Hill Giant", []string{"creature"}, 3, nil), 3, 3)

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{attacker})

	found := false
	for _, p := range got {
		if p == attacker {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("sac-fuel attacker with outlet on board should swing into clean block; got %d attackers", len(got))
	}
}

// TestChooseAttackers_VanillaCreatureStillPrunedOnCleanBlock pins the
// negative side: without a death-payoff signal, a vanilla 1/1 into a
// 3/3 still gets pruned. The two new signals must not blanket-disable
// the canSwingProfitably gate.
func TestChooseAttackers_VanillaCreatureStillPrunedOnCleanBlock(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Vanilla 1/1", []string{"creature"}, 1, nil), 1, 1)
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Hill Giant", []string{"creature"}, 3, nil), 3, 3)

	h := NewYggdrasilHatWithNoise(&StrategyProfile{Archetype: ArchetypeMidrange}, 0, 0)
	got := h.ChooseAttackers(gs, 0, []*gameengine.Permanent{attacker})

	for _, p := range got {
		if p == attacker {
			t.Errorf("vanilla 1/1 should still be pruned vs a 3/3 clean block (no death-payoff signal)")
		}
	}
}
