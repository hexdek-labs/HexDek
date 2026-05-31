package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// Combat-damage ctx-key sweep (R60)
//
// Sibling to the permanent_etb ctx["permanent"] vs ctx["perm"] sweep. The
// engine fires combat_damage_player with ctx keys source_seat /
// source_card / defender_seat / amount (combat.go:1921). Four per_card
// handlers read the wrong keys (damager_seat / damager_perm / target_seat)
// and were silently inert before this PR:
//
//   - Archpriest of Shadows  (always early-returns on `nil != perm`)
//   - April Reporter         (per_card_batch_k_r60.go — both gates miss)
//   - Quilled Greatwurm      (early-returns on missing damager_perm)
//   - Angel of Destiny       (partial — works only for seat 0)
//
// Each handler now reads the canonical engine keys with a fallback to the
// legacy keys (so tests/callers that still thread damager_perm/damager_seat
// keep working). These tests fire the engine-canonical shape.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Archpriest of Shadows — reanimate highest-CMC creature on combat damage
// ---------------------------------------------------------------------------

func TestArchpriest_ReanimatesOnCanonicalCombatDamageCtx(t *testing.T) {
	gs := newGame(t, 2)
	ap := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	ap.Card.BasePower = 3
	ap.Card.BaseToughness = 4

	// Two creatures in graveyard with different CMCs — Archpriest picks the
	// highest-CMC one to reanimate.
	low := addCardToGraveyard(gs, 0, "Llanowar Elves", "creature", "elf", "druid", "cost:1")
	low.BasePower = 1
	low.BaseToughness = 1
	high := addCardToGraveyard(gs, 0, "Grave Titan", "creature", "giant", "cost:6")
	high.BasePower = 6
	high.BaseToughness = 5
	_ = low

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Archpriest of Shadows",
		"defender_seat": 1,
		"amount":        3,
	})

	foundHigh := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == high {
			foundHigh = true
		}
	}
	if !foundHigh {
		t.Errorf("expected Grave Titan reanimated by Archpriest on canonical ctx, battlefield=%d", len(gs.Seats[0].Battlefield))
	}
}

func TestArchpriest_DoesNotFireWhenOtherCreatureDealsDamage(t *testing.T) {
	gs := newGame(t, 2)
	ap := addPerm(gs, 0, "Archpriest of Shadows", "creature", "human", "warlock")
	ap.Card.BasePower = 3
	ap.Card.BaseToughness = 4

	body := addCardToGraveyard(gs, 0, "Bone Splinters", "creature", "cmc:1")
	body.BasePower = 1
	body.BaseToughness = 1

	// A non-Archpriest source — Archpriest's "this creature deals" gate must block.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Grizzly Bears",
		"defender_seat": 1,
		"amount":        2,
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == body {
			t.Errorf("Archpriest should not have reanimated when a different creature dealt damage")
		}
	}
}

// ---------------------------------------------------------------------------
// Angel of Destiny — symmetric lifegain on combat damage
// ---------------------------------------------------------------------------

func TestAngelOfDestiny_LifegainOnCanonicalCombatDamageCtxNonSeatZero(t *testing.T) {
	gs := newGame(t, 3)
	// Put Angel on seat 1 (not seat 0) — pre-fix the handler only worked
	// when dmgSeat (defaulting to 0 from missing key) happened to equal
	// the controller's seat. Move the controller off seat 0 to surface
	// the source_seat fix.
	angel := addPerm(gs, 1, "Angel of Destiny", "creature", "angel")
	angel.Card.BasePower = 2
	angel.Card.BaseToughness = 4
	_ = angel

	startCtrl := gs.Seats[1].Life
	startTgt := gs.Seats[2].Life

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1,
		"source_card":   "Angel of Destiny",
		"defender_seat": 2,
		"amount":        4,
	})

	if gs.Seats[1].Life != startCtrl+4 {
		t.Errorf("expected Angel controller (seat 1) gain 4 life, %d → %d", startCtrl, gs.Seats[1].Life)
	}
	if gs.Seats[2].Life != startTgt+4 {
		t.Errorf("expected defender (seat 2) gain 4 life (symmetric), %d → %d", startTgt, gs.Seats[2].Life)
	}
}

func TestAngelOfDestiny_NoLifegainOnOpponentDamage(t *testing.T) {
	gs := newGame(t, 2)
	angel := addPerm(gs, 0, "Angel of Destiny", "creature", "angel")
	angel.Card.BasePower = 2
	angel.Card.BaseToughness = 4

	startCtrl := gs.Seats[0].Life
	startTgt := gs.Seats[1].Life

	// Opponent deals damage to me — Angel's "you deal" gate must block.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1,
		"source_card":   "Goblin Guide",
		"defender_seat": 0,
		"amount":        2,
	})

	if gs.Seats[0].Life != startCtrl {
		t.Errorf("Angel should not gain life from opponent's damage, %d → %d", startCtrl, gs.Seats[0].Life)
	}
	if gs.Seats[1].Life != startTgt {
		t.Errorf("opponent should not gain life from their own damage either, %d → %d", startTgt, gs.Seats[1].Life)
	}
}

// ---------------------------------------------------------------------------
// April, Reporter of the Weird — draw N, discard 1 on her combat damage
// ---------------------------------------------------------------------------

func TestApril_DrawsAndDiscardsOnCanonicalCombatDamageCtx(t *testing.T) {
	gs := newGame(t, 2)
	april := addPerm(gs, 0, "April, Reporter of the Weird", "creature", "legendary", "human", "advisor")
	april.Card.BasePower = 2
	april.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C", "D")
	startHand := len(gs.Seats[0].Hand)
	startYard := len(gs.Seats[0].Graveyard)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "April, Reporter of the Weird",
		"defender_seat": 1,
		"amount":        3,
	})

	// April draws 3, then discards 1 — net hand +2.
	if len(gs.Seats[0].Hand) != startHand+2 {
		t.Errorf("expected hand +2 after draw 3 / discard 1, %d → %d", startHand, len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != startYard+1 {
		t.Errorf("expected graveyard +1 (discard), %d → %d", startYard, len(gs.Seats[0].Graveyard))
	}
}

func TestApril_DoesNotFireWhenOtherCreatureDealsDamage(t *testing.T) {
	gs := newGame(t, 2)
	april := addPerm(gs, 0, "April, Reporter of the Weird", "creature", "legendary", "human", "advisor")
	april.Card.BasePower = 2
	april.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B", "C")
	startHand := len(gs.Seats[0].Hand)

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Grizzly Bears",
		"defender_seat": 1,
		"amount":        2,
	})

	if len(gs.Seats[0].Hand) != startHand {
		t.Errorf("April should only fire on her own damage; hand %d → %d", startHand, len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Quilled Greatwurm — +1/+1 counter to attacker dealing combat damage on
// your turn
// ---------------------------------------------------------------------------

func TestQuilledGreatwurm_AddsCountersOnOwnTurnCanonicalCtx(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	qg := addPerm(gs, 0, "Quilled Greatwurm", "creature", "wurm")
	qg.Card.BasePower = 8
	qg.Card.BaseToughness = 8

	// Friendly attacker on the battlefield — Quilled looks it up by source_card name.
	atkr := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	atkr.Card.BasePower = 1
	atkr.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Llanowar Elves",
		"defender_seat": 1,
		"amount":        4,
	})

	if atkr.Counters["+1/+1"] != 4 {
		t.Errorf("expected +1/+1 x4 on Llanowar Elves, got %v", atkr.Counters)
	}
}

func TestQuilledGreatwurm_DoesNotFireOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 1 // opponent's turn
	qg := addPerm(gs, 0, "Quilled Greatwurm", "creature", "wurm")
	qg.Card.BasePower = 8
	qg.Card.BaseToughness = 8

	atkr := addPerm(gs, 0, "Llanowar Elves", "creature", "elf", "druid")
	atkr.Card.BasePower = 1
	atkr.Card.BaseToughness = 2

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Llanowar Elves",
		"defender_seat": 1,
		"amount":        4,
	})

	if atkr.Counters["+1/+1"] != 0 {
		t.Errorf("Quilled should only fire on its controller's turn; counters=%v", atkr.Counters)
	}
}
