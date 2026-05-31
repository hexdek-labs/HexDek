package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Wave 5 (final) ctx-key audit (R60)
//
// Continues PRs #815 / #824 / #841 / #867 (which together closed 17
// silent-inert handler bugs across permanent_etb / combat_damage_player
// / creature_attacks / card_exiled / permanent_phased_out).
//
// This wave covers the remaining keyword-event surface: scry, roll_dice,
// artifact_tapped_for_mana, card_discarded, declare_attackers, and a
// second pass on combat_damage_player. The hypothetical events
// mana_paid / counter_added / card_revealed / library_searched /
// spell_resolved / ability_activated don't exist as engine fire sites
// and have no handlers registered for them — empty surface.
//
// 6 handlers across 7 bugs:
//
//   - Elrond, Master of Healing (scry)            — reads "count" instead
//     of "amount"; X always defaults to 1 regardless of scry size.
//   - Vrondiss, Rage of Ancients (roll_dice)      — reads "controller_seat"
//     instead of "seat"; only fires for seat-0 controllers.
//   - Roxanne, Starfall Savant (artifact_tapped)  — reads "seat" instead
//     of "controller_seat" + "mana_type" (not written) instead of "pips";
//     only fires for seat-0 controllers.
//   - Craig Boone, Novac Guard (declare_attackers) — reads "seat" + "count"
//     (neither written for the attacks event). Fully inert. Now counts
//     attackers via battlefield scan + per-turn gate.
//   - Captain Howler, Sea Scourge (card_discarded) — reads "seat" instead
//     of "discarder_seat"; only fires for seat-0 controllers.
//   - Captain Howler, Sea Scourge (combat_damage_player) — reads
//     "source_perm" (not written); falls back to source_card name match.
//   - Rielle, the Everwise (card_discarded)        — reads "seat" instead
//     of "discarder_seat"; only fires for seat-0 controllers.
//
// Each handler now reads the canonical engine key first with the legacy
// key as fallback for Hat/tests that thread the old shape explicitly.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Elrond, Master of Healing — scry pumps N creatures by X
// ---------------------------------------------------------------------------

func TestElrond_BuffsAmountCreaturesOnCanonicalScryCtx(t *testing.T) {
	gs := newGame(t, 2)
	elrond := addPerm(gs, 0, "Elrond, Master of Healing", "creature", "legendary", "elf")
	elrond.Card.BasePower = 3
	elrond.Card.BaseToughness = 3

	a := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	a.Card.BasePower = 1
	a.Card.BaseToughness = 1
	b := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	b.Card.BasePower = 2
	b.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "scry", map[string]interface{}{
		"seat":   0,
		"amount": 3, // engine-canonical key
	})

	// Elrond itself is in seat[0].Battlefield too, so the cap of 3
	// buffs are: elrond (+1) + a (+1) + b (+1).
	totalBuffed := 0
	if elrond.Counters["+1/+1"] > 0 {
		totalBuffed += elrond.Counters["+1/+1"]
	}
	if a.Counters["+1/+1"] > 0 {
		totalBuffed += a.Counters["+1/+1"]
	}
	if b.Counters["+1/+1"] > 0 {
		totalBuffed += b.Counters["+1/+1"]
	}
	if totalBuffed != 3 {
		t.Errorf("expected scry-3 → 3 creatures buffed by +1/+1 each (total counters = 3), got %d", totalBuffed)
	}
}

// ---------------------------------------------------------------------------
// Vrondiss, Rage of Ancients — roll_dice damages self
// ---------------------------------------------------------------------------

func TestVrondiss_SelfDamagesOnRollFromNonSeatZeroController(t *testing.T) {
	gs := newGame(t, 2)
	// Put Vrondiss on seat 1 — pre-fix the controller_seat read defaulted
	// to 0, so rolls by a non-seat-0 controller never fired the trigger.
	vr := addPerm(gs, 1, "Vrondiss, Rage of Ancients", "creature", "legendary", "elf", "berserker")
	vr.Card.BasePower = 5
	vr.Card.BaseToughness = 4

	gameengine.FireCardTrigger(gs, "roll_dice", map[string]interface{}{
		"seat":   1, // engine-canonical key
		"result": 12,
		"sides":  20,
	})

	if vr.MarkedDamage != 1 {
		t.Errorf("expected Vrondiss self-damage=1 after seat-1 roll, got %d", vr.MarkedDamage)
	}
}

// ---------------------------------------------------------------------------
// Roxanne, Starfall Savant — artifact-token-tap mana doubler
// ---------------------------------------------------------------------------

func TestRoxanne_FiresDoubledManaEmitOnNonSeatZeroController(t *testing.T) {
	gs := newGame(t, 2)
	rox := addPerm(gs, 1, "Roxanne, Starfall Savant", "creature", "legendary", "cat", "druid")
	rox.Card.BasePower = 3
	rox.Card.BaseToughness = 4

	// An artifact token on seat 1 that taps for mana.
	tok := addPerm(gs, 1, "Meteorite", "token", "artifact")

	gameengine.FireCardTrigger(gs, "artifact_tapped_for_mana", map[string]interface{}{
		"controller_seat": 1, // engine-canonical key
		"perm":            tok,
		"card":            tok.Card,
		"pips":            1,
		"artifact_name":   "Meteorite",
	})

	// Verify the doubling emit fired (slug present in event log).
	saw := false
	for _, ev := range gs.EventLog {
		if d, ok := ev.Details["slug"].(string); ok && d == "roxanne_artifact_token_double_mana" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected roxanne_artifact_token_double_mana emit on canonical ctx for non-seat-0 controller")
	}
}

// ---------------------------------------------------------------------------
// Craig Boone, Novac Guard — quest counters when attacking with 2+
// ---------------------------------------------------------------------------

func TestCraigBoone_AddsQuestCountersWhenAttackingWithTwoOrMore(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	cb := addPerm(gs, 0, "Craig Boone, Novac Guard", "creature", "legendary", "human", "soldier")
	cb.Card.BasePower = 3
	cb.Card.BaseToughness = 3
	gameengine.SetAttackerDefender(cb, 1)
	cb.Flags["attacking"] = 1

	// A second attacker so the "two or more" gate passes.
	ally := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	ally.Card.BasePower = 1
	ally.Card.BaseToughness = 1
	gameengine.SetAttackerDefender(ally, 1)
	ally.Flags["attacking"] = 1

	// Give the opponent a creature so the damage clause has a target.
	opp := addPerm(gs, 1, "Birds of Paradise", "creature")
	opp.Card.BasePower = 0
	opp.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": cb,
		"attacker_seat": 0,
		"attacker_card": cb.Card,
	})

	if cb.Counters["quest"] != 2 {
		t.Errorf("expected Craig Boone +2 quest counters, got %v", cb.Counters)
	}
}

func TestCraigBoone_DoesNotFireWithOnlyOneAttacker(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	cb := addPerm(gs, 0, "Craig Boone, Novac Guard", "creature", "legendary", "human", "soldier")
	cb.Card.BasePower = 3
	cb.Card.BaseToughness = 3
	gameengine.SetAttackerDefender(cb, 1)
	cb.Flags["attacking"] = 1

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": cb,
		"attacker_seat": 0,
		"attacker_card": cb.Card,
	})

	if cb.Counters["quest"] != 0 {
		t.Errorf("Craig Boone should NOT fire with only 1 attacker, got %v", cb.Counters)
	}
}

// ---------------------------------------------------------------------------
// Captain Howler, Sea Scourge — discard-pump + combat-damage draw
// ---------------------------------------------------------------------------

func TestCaptainHowler_PumpsBiggestCreatureOnNonSeatZeroDiscard(t *testing.T) {
	gs := newGame(t, 2)
	howler := addPerm(gs, 1, "Captain Howler, Sea Scourge", "creature", "legendary", "shark", "pirate")
	howler.Card.BasePower = 5
	howler.Card.BaseToughness = 4

	target := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	target.Card.BasePower = 2
	target.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"discarder_seat": 1, // engine-canonical key
		"card":           &gameengine.Card{Name: "Lightning Bolt", Owner: 1},
		"card_name":      "Lightning Bolt",
	})

	// Howler picks the biggest creature (itself at power 5 vs Bears at 2).
	// Either could be pumped depending on best-power tie-break; verify
	// some +2 mod landed.
	pumped := false
	for _, p := range gs.Seats[1].Battlefield {
		for _, m := range p.Modifications {
			if m.Power == 2 {
				pumped = true
			}
		}
	}
	if !pumped {
		t.Errorf("expected one of Howler's creatures to receive +2/+0 UEOT")
	}
}

// ---------------------------------------------------------------------------
// Rielle, the Everwise — first discard each turn draws
// ---------------------------------------------------------------------------

func TestRielle_DrawsOnFirstDiscardFromNonSeatZeroController(t *testing.T) {
	gs := newGame(t, 2)
	rl := addPerm(gs, 1, "Rielle, the Everwise", "creature", "legendary", "human", "wizard")
	rl.Card.BasePower = 3
	rl.Card.BaseToughness = 3
	addLibrary(gs, 1, "A", "B", "C")
	startHand := len(gs.Seats[1].Hand)

	gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
		"discarder_seat": 1, // engine-canonical key
		"card":           &gameengine.Card{Name: "Lightning Bolt", Owner: 1},
		"card_name":      "Lightning Bolt",
	})

	if len(gs.Seats[1].Hand) != startHand+1 {
		t.Errorf("expected Rielle draw +1 on first non-seat-0 discard, hand %d → %d",
			startHand, len(gs.Seats[1].Hand))
	}
}

func TestRielle_OnlyFiresOncePerTurn(t *testing.T) {
	gs := newGame(t, 2)
	rl := addPerm(gs, 0, "Rielle, the Everwise", "creature", "legendary", "human", "wizard")
	rl.Card.BasePower = 3
	rl.Card.BaseToughness = 3
	addLibrary(gs, 0, "A", "B", "C", "D")
	startHand := len(gs.Seats[0].Hand)

	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "card_discarded", map[string]interface{}{
			"discarder_seat": 0,
			"card":           &gameengine.Card{Name: "Lightning Bolt", Owner: 0},
			"card_name":      "Lightning Bolt",
		})
	}

	// Once-per-turn gate caps at +1.
	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Errorf("expected Rielle to fire only once per turn, hand %d → %d",
			startHand, len(gs.Seats[0].Hand))
	}
}
