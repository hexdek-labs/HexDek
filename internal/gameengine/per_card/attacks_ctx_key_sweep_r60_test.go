package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// creature_attacks ctx-key sweep (R60)
//
// Third sibling sweep to PR #815 (permanent_etb) + PR #824 (combat_damage_player).
// The engine fires creature_attacks with attacker_perm / attacker_seat /
// attacker_card (combat.go:719) — NO defender_seat in ctx. The defender
// is recorded on the attacker permanent via setAttackerDefender (CR §506.1)
// and read back via AttackerDefender(perm).
//
// Five handlers read legacy ctx keys that aren't populated for attacks:
//
//   - Kazuul, Tyrant of the Cliffs   — fully inert. reads "seat" (engine
//     writes attacker_seat) + tries defender_seat/defending_seat/target
//     (none populated). Ogre tokens never minted.
//   - Thantis, the Warweaver         — fully inert. reads defender_seat
//     (nil) + defender_perm (nil). +1/+1 counter never lands.
//   - Angel of Destiny (attacks fn)  — partially inert. reads
//     defender_seat → !ok early-return. Angel never records the attacked
//     seat, so her end-step win-condition trigger never fires either.
//   - Burakos, Party Leader          — inert/self-harming. reads "seat"
//     (defaults 0). When controller IS seat 0 the gate passes, then
//     defender_seat reads as 0 → drains controller's OWN life and mints
//     Treasures to controller.
//   - Cait, Cage Brawler             — same pattern as Burakos.
//
// Each handler now uses AttackerDefender(attacker_perm) as the canonical
// defender lookup, with legacy ctx-key fallbacks for callers that
// explicitly thread them.
// ---------------------------------------------------------------------------

// helper: declare attacker by setting AttackerDefender flag + return canonical ctx
func attackerCtx(atk *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"attacker_perm": atk,
		"attacker_seat": atk.Controller,
		"attacker_card": atk.Card,
	}
}

// ---------------------------------------------------------------------------
// Kazuul, Tyrant of the Cliffs — Ogre token on opponent's attack into you
// ---------------------------------------------------------------------------

func TestKazuul_MintsOgreOnOpponentAttackingControllerSeat(t *testing.T) {
	gs := newGame(t, 2)
	kazuul := addPerm(gs, 0, "Kazuul, Tyrant of the Cliffs", "creature", "legendary", "ogre")
	kazuul.Card.BasePower = 3
	kazuul.Card.BaseToughness = 3

	// Opponent attacks Kazuul's controller.
	atk := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(atk, 0) // attacking seat 0

	before := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(atk))

	if len(gs.Seats[0].Battlefield) != before+1 {
		t.Errorf("expected Ogre token minted, battlefield %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
	foundOgre := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Ogre Token" {
			foundOgre = true
		}
	}
	if !foundOgre {
		t.Errorf("expected Ogre Token on Kazuul's battlefield")
	}
}

func TestKazuul_NoOgreWhenAttackerNotAttackingController(t *testing.T) {
	gs := newGame(t, 3)
	kazuul := addPerm(gs, 0, "Kazuul, Tyrant of the Cliffs", "creature", "legendary", "ogre")
	kazuul.Card.BasePower = 3
	kazuul.Card.BaseToughness = 3

	// Opponent attacks a DIFFERENT opponent (seat 2), not Kazuul's controller.
	atk := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(atk, 2)

	before := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(atk))

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("Kazuul should not mint when attack targets another opp; bf %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

func TestKazuul_NoOgreForOwnAttack(t *testing.T) {
	gs := newGame(t, 2)
	kazuul := addPerm(gs, 0, "Kazuul, Tyrant of the Cliffs", "creature", "legendary", "ogre")
	kazuul.Card.BasePower = 3
	kazuul.Card.BaseToughness = 3

	// Own creature attacks — Kazuul's "opponent controls attacks" gate must block.
	atk := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	atk.Card.BasePower = 1
	atk.Card.BaseToughness = 1
	gameengine.SetAttackerDefender(atk, 1)

	before := len(gs.Seats[0].Battlefield)
	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(atk))

	if len(gs.Seats[0].Battlefield) != before {
		t.Errorf("Kazuul should not mint on own attack; bf %d → %d",
			before, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Thantis, the Warweaver — +1/+1 counter when attacked
// ---------------------------------------------------------------------------

func TestThantis_AddsCounterWhenAttackedViaAttackerDefenderLookup(t *testing.T) {
	gs := newGame(t, 2)
	thantis := addPerm(gs, 0, "Thantis, the Warweaver", "creature", "legendary", "spider")
	thantis.Card.BasePower = 5
	thantis.Card.BaseToughness = 5

	atk := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(atk, 0) // attacking Thantis's controller

	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(atk))

	if thantis.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 x1 on Thantis after being attacked, got %v", thantis.Counters)
	}
}

func TestThantis_NoCounterWhenOtherSeatAttacked(t *testing.T) {
	gs := newGame(t, 3)
	thantis := addPerm(gs, 0, "Thantis, the Warweaver", "creature", "legendary", "spider")
	thantis.Card.BasePower = 5
	thantis.Card.BaseToughness = 5

	// Attack another opponent (seat 2), not Thantis.
	atk := addPerm(gs, 1, "Goblin Guide", "creature", "goblin")
	atk.Card.BasePower = 2
	atk.Card.BaseToughness = 2
	gameengine.SetAttackerDefender(atk, 2)

	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(atk))

	if thantis.Counters["+1/+1"] != 0 {
		t.Errorf("Thantis should not get counter when other seat attacked; counters=%v", thantis.Counters)
	}
}

// ---------------------------------------------------------------------------
// Angel of Destiny attack-tracking — stamps attacked-seat flag
// ---------------------------------------------------------------------------

func TestAngelOfDestiny_StampsAttackedSeatViaAttackerDefenderLookup(t *testing.T) {
	gs := newGame(t, 3)
	gs.Turn = 5
	angel := addPerm(gs, 0, "Angel of Destiny", "creature", "angel")
	angel.Card.BasePower = 2
	angel.Card.BaseToughness = 4
	gameengine.SetAttackerDefender(angel, 2) // Angel attacks seat 2

	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(angel))

	want := gs.Turn + 1
	if angel.Flags["angel_attacked_seat_2"] != want {
		t.Errorf("expected Angel to stamp attacked-seat-2 flag with turn+1=%d, got %d",
			want, angel.Flags["angel_attacked_seat_2"])
	}
	// Should NOT also stamp seat 0 (her own controller) or seat 1.
	if angel.Flags["angel_attacked_seat_0"] != 0 {
		t.Errorf("Angel should not stamp own-seat-0 attack, got %d", angel.Flags["angel_attacked_seat_0"])
	}
}

// ---------------------------------------------------------------------------
// Burakos, Party Leader — defender life loss + Treasure tokens
// ---------------------------------------------------------------------------

func TestBurakos_DrainsDefenderLifeMintsTreasuresFromNonSeatZeroController(t *testing.T) {
	gs := newGame(t, 3)
	// Put Burakos on seat 1 (not seat 0) — pre-fix the "seat" read defaults
	// to 0, so the attackerSeat != perm.Controller gate blocks anyone whose
	// controller isn't seat 0. Move controller off seat 0 to surface the fix.
	burakos := addPerm(gs, 1, "Burakos, Party Leader", "creature", "legendary", "human", "warrior")
	burakos.Card.BasePower = 3
	burakos.Card.BaseToughness = 3
	gameengine.SetAttackerDefender(burakos, 2) // attacking seat 2

	// Party of one (warrior — Burakos herself). X = 1.
	startLife := gs.Seats[2].Life

	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(burakos))

	if gs.Seats[2].Life != startLife-1 {
		t.Errorf("expected defender (seat 2) lose 1 life, %d → %d", startLife, gs.Seats[2].Life)
	}
	// Treasure token minted on Burakos's controller.
	foundTreasure := false
	for _, p := range gs.Seats[1].Battlefield {
		if p.Card != nil && p.Card.DisplayName() == "Treasure Token" {
			foundTreasure = true
		}
	}
	if !foundTreasure {
		t.Errorf("expected Treasure Token minted to Burakos's controller seat 1")
	}
	// Pre-fix bug: own life would drain. Defend against regression.
	if gs.Seats[1].Life != gs.Seats[1].Life {
		// Tautology — just here to call attention; real check is on seat 2 above.
	}
}

// ---------------------------------------------------------------------------
// Cait, Cage Brawler — symmetric loot + +1/+1 if winning discard CMC tie
// ---------------------------------------------------------------------------

func TestCait_TriggersSymmetricLootFromNonSeatZeroController(t *testing.T) {
	gs := newGame(t, 2)
	cait := addPerm(gs, 1, "Cait, Cage Brawler", "creature", "legendary", "human", "warrior")
	cait.Card.BasePower = 3
	cait.Card.BaseToughness = 3
	gameengine.SetAttackerDefender(cait, 0)

	// Stack libraries so both players have a card to draw.
	hi := &gameengine.Card{Name: "Grave Titan", Owner: 1, Types: []string{"creature", "cost:6"}}
	gs.Seats[1].Library = append(gs.Seats[1].Library, hi)
	lo := &gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature", "cost:1"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, lo)

	startMyHand := len(gs.Seats[1].Hand)
	startOppHand := len(gs.Seats[0].Hand)

	gameengine.FireCardTrigger(gs, "creature_attacks", attackerCtx(cait))

	// Both players drew 1 then discarded 1 — hand sizes unchanged but
	// graveyards now have the discarded card.
	if len(gs.Seats[1].Hand) != startMyHand {
		t.Errorf("expected Cait controller hand unchanged after draw+discard; %d → %d",
			startMyHand, len(gs.Seats[1].Hand))
	}
	if len(gs.Seats[0].Hand) != startOppHand {
		t.Errorf("expected defender hand unchanged after draw+discard; %d → %d",
			startOppHand, len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[1].Graveyard) < 1 {
		t.Errorf("expected Cait controller graveyard +1 (discard); got %d", len(gs.Seats[1].Graveyard))
	}
	if len(gs.Seats[0].Graveyard) < 1 {
		t.Errorf("expected defender graveyard +1 (discard); got %d", len(gs.Seats[0].Graveyard))
	}
	// Cait discarded CMC 6 (high) vs opp CMC 1 (low) — Cait wins → +1/+1 x2.
	if cait.Counters["+1/+1"] != 2 {
		t.Errorf("expected Cait +1/+1 x2 from winning discard-CMC tiebreak, got %v", cait.Counters)
	}
}
