package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// investigate_consumers_r60 — 8 handlers wired in
// investigate_consumers_r60.go covering the investigate / Clue family
// (CR §701.15).
// -----------------------------------------------------------------------------

// countClues returns the number of Clue artifacts on seat's battlefield.
func countClues(gs *gameengine.GameState, seatIdx int) int {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	n := 0
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if isClueCard(p.Card) {
			n++
		}
	}
	return n
}

// etbCtx builds the permanent_etb ctx the engine forwards.
func etbCtx(p *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"perm":            p,
		"controller_seat": p.Controller,
		"card":            p.Card,
	}
}

// castCtx builds the spell_cast family ctx.
func castCtx(casterSeat int, c *gameengine.Card) map[string]interface{} {
	return map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  c.DisplayName(),
		"card":        c,
		"is_creature": cardHasType(c, "creature"),
		"cast_zone":   "hand",
	}
}

// sacCtx mirrors zone_change.go's permanent_sacrificed ctx.
func sacCtx(p *gameengine.Permanent) map[string]interface{} {
	return map[string]interface{}{
		"perm":            p,
		"card":            p.Card,
		"controller_seat": p.Controller,
		"card_name":       p.Card.DisplayName(),
		"to_zone":         "graveyard",
	}
}

// -----------------------------------------------------------------------------
// Lonis, Cryptozoologist
// -----------------------------------------------------------------------------

func TestLonis_InvestigatesOnFriendlyCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lonis, Cryptozoologist", "creature", "legendary")
	pre := countClues(gs, 0)

	newcomer := addPerm(gs, 0, "Llanowar Elves", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(newcomer))

	if countClues(gs, 0) != pre+1 {
		t.Errorf("expected 1 new Clue; pre=%d post=%d", pre, countClues(gs, 0))
	}
}

func TestLonis_IgnoresOpponentCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lonis, Cryptozoologist", "creature", "legendary")
	pre := countClues(gs, 0)

	oppCreature := addPerm(gs, 1, "Grizzly Bears", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(oppCreature))

	if countClues(gs, 0) != pre {
		t.Errorf("opp creature ETB must not investigate; pre=%d post=%d", pre, countClues(gs, 0))
	}
}

func TestLonis_IgnoresTokenCreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lonis, Cryptozoologist", "creature", "legendary")
	pre := countClues(gs, 0)

	token := addPerm(gs, 0, "Goblin Token", "creature", "token")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(token))

	if countClues(gs, 0) != pre {
		t.Errorf("token creature must not trigger Lonis ('nontoken'); pre=%d post=%d", pre, countClues(gs, 0))
	}
}

func TestLonis_IgnoresNoncreatureETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Lonis, Cryptozoologist", "creature", "legendary")
	pre := countClues(gs, 0)

	artifact := addPerm(gs, 0, "Sol Ring", "artifact")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(artifact))

	if countClues(gs, 0) != pre {
		t.Errorf("non-creature ETB must not trigger Lonis; pre=%d post=%d", pre, countClues(gs, 0))
	}
}

// -----------------------------------------------------------------------------
// Thraben Inspector / Briarbridge Tracker — self-ETB investigate
// -----------------------------------------------------------------------------

func TestThrabenInspector_InvestigatesOnSelfETB(t *testing.T) {
	gs := newGame(t, 2)
	insp := addPerm(gs, 0, "Thraben Inspector", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(insp))

	if countClues(gs, 0) != 1 {
		t.Errorf("expected 1 Clue from Thraben Inspector ETB, got %d", countClues(gs, 0))
	}
}

func TestThrabenInspector_DoesNotTriggerOnAllyETB(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Thraben Inspector", "creature")
	pre := countClues(gs, 0)
	ally := addPerm(gs, 0, "Llanowar Elves", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(ally))

	if countClues(gs, 0) != pre {
		t.Errorf("Thraben Inspector only triggers on its OWN ETB; got %d clues", countClues(gs, 0))
	}
}

func TestBriarbridgeTracker_InvestigatesOnSelfETB(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Briarbridge Tracker", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(tracker))

	if countClues(gs, 0) != 1 {
		t.Errorf("expected 1 Clue from Briarbridge Tracker ETB, got %d", countClues(gs, 0))
	}
}

// -----------------------------------------------------------------------------
// Bygone Bishop — cast-creature trigger filtered on CMC ≤ 3
// -----------------------------------------------------------------------------

func TestBygoneBishop_InvestigatesOnCheapCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Bygone Bishop", "creature")

	cheapCreature := addCard(gs, 0, "Llanowar Elves", "creature")
	cheapCreature.CMC = 1
	gameengine.FireCardTrigger(gs, "creature_spell_cast", castCtx(0, cheapCreature))

	if countClues(gs, 0) != 1 {
		t.Errorf("Bygone Bishop should investigate on CMC-1 creature cast; clues=%d", countClues(gs, 0))
	}
}

func TestBygoneBishop_NoInvestigateOnExpensiveCreature(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Bygone Bishop", "creature")

	expensiveCreature := addCard(gs, 0, "Craterhoof Behemoth", "creature")
	expensiveCreature.CMC = 8
	gameengine.FireCardTrigger(gs, "creature_spell_cast", castCtx(0, expensiveCreature))

	if countClues(gs, 0) != 0 {
		t.Errorf("CMC > 3 must not trigger Bygone Bishop; clues=%d", countClues(gs, 0))
	}
}

func TestBygoneBishop_IgnoresOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Bygone Bishop", "creature")

	oppCreature := addCard(gs, 1, "Llanowar Elves", "creature")
	oppCreature.CMC = 1
	gameengine.FireCardTrigger(gs, "creature_spell_cast", castCtx(1, oppCreature))

	if countClues(gs, 0) != 0 {
		t.Errorf("opp casts must not trigger Bygone Bishop; clues=%d", countClues(gs, 0))
	}
}

// -----------------------------------------------------------------------------
// Trail of Evidence — cast i/s investigate
// -----------------------------------------------------------------------------

func TestTrailOfEvidence_InvestigatesOnInstantCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trail of Evidence", "enchantment")

	bolt := addCard(gs, 0, "Lightning Bolt", "instant")
	bolt.CMC = 1
	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", castCtx(0, bolt))

	if countClues(gs, 0) != 1 {
		t.Errorf("Trail of Evidence should investigate on i/s cast; clues=%d", countClues(gs, 0))
	}
}

func TestTrailOfEvidence_IgnoresOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Trail of Evidence", "enchantment")

	bolt := addCard(gs, 1, "Lightning Bolt", "instant")
	bolt.CMC = 1
	gameengine.FireCardTrigger(gs, "instant_or_sorcery_cast", castCtx(1, bolt))

	if countClues(gs, 0) != 0 {
		t.Errorf("opp casts must not trigger Trail of Evidence; clues=%d", countClues(gs, 0))
	}
}

// -----------------------------------------------------------------------------
// Tireless Tracker — landfall investigate + sac-clue counter
// -----------------------------------------------------------------------------

func TestTirelessTracker_InvestigatesOnLandfall(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Tireless Tracker", "creature")

	land := addPerm(gs, 0, "Forest", "land", "basic")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(land))

	if countClues(gs, 0) != 1 {
		t.Errorf("Tireless Tracker should investigate on landfall; clues=%d", countClues(gs, 0))
	}
}

func TestTirelessTracker_NoLandfallOnNonLand(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Tireless Tracker", "creature")

	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	gameengine.FireCardTrigger(gs, "permanent_etb", etbCtx(bear))

	if countClues(gs, 0) != 0 {
		t.Errorf("non-land ETB must not trigger Tracker landfall; clues=%d", countClues(gs, 0))
	}
}

func TestTirelessTracker_CounterOnClueSac(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Tireless Tracker", "creature")
	tracker.Card.BasePower, tracker.Card.BaseToughness = 3, 2
	clue := addPerm(gs, 0, "Clue Token", "artifact", "clue", "token")

	gameengine.FireCardTrigger(gs, "permanent_sacrificed", sacCtx(clue))

	if tracker.Counters["+1/+1"] != 1 {
		t.Errorf("expected +1/+1 counter on Tracker after sac-clue; got %d", tracker.Counters["+1/+1"])
	}
	if tracker.Power() != 4 || tracker.Toughness() != 3 {
		t.Errorf("expected 4/3 after counter; got %d/%d", tracker.Power(), tracker.Toughness())
	}
}

func TestTirelessTracker_NonClueSacDoesNotCounter(t *testing.T) {
	gs := newGame(t, 2)
	tracker := addPerm(gs, 0, "Tireless Tracker", "creature")
	tracker.Card.BasePower, tracker.Card.BaseToughness = 3, 2

	// Sac a non-Clue artifact (Sol Ring) — must not trigger.
	solRing := addPerm(gs, 0, "Sol Ring", "artifact")
	gameengine.FireCardTrigger(gs, "permanent_sacrificed", sacCtx(solRing))

	if tracker.Counters["+1/+1"] != 0 {
		t.Errorf("non-Clue sac must not add counter; got %d", tracker.Counters["+1/+1"])
	}
}

// -----------------------------------------------------------------------------
// Ongoing Investigation
// -----------------------------------------------------------------------------

func TestOngoingInvestigation_InvestigatesOnCombatDamageToPlayer(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ongoing Investigation", "enchantment")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Llanowar Elves",
		"defender_seat": 1,
		"amount":        2,
	})

	if countClues(gs, 0) != 1 {
		t.Errorf("expected 1 Clue from Ongoing Investigation; got %d", countClues(gs, 0))
	}
}

func TestOngoingInvestigation_DedupesAcrossMultipleCreaturesSameDefender(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ongoing Investigation", "enchantment")

	// Two of your creatures both hit the same opponent this combat. Oracle
	// says "one or more creatures ... deal combat damage to a player" — fires
	// ONCE per defender per combat, not per attacker.
	for i := 0; i < 3; i++ {
		gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
			"source_seat":   0,
			"source_card":   "Attacker",
			"defender_seat": 1,
			"amount":        2,
		})
	}

	if countClues(gs, 0) != 1 {
		t.Errorf("expected exactly 1 Clue per defender per turn; got %d", countClues(gs, 0))
	}
}

func TestOngoingInvestigation_IgnoresOpponentDamage(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ongoing Investigation", "enchantment")

	// Opponent attacks us — source_seat is THEM. Ongoing Investigation
	// triggers only on YOUR creatures' combat damage.
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":   1,
		"source_card":   "Opp Creature",
		"defender_seat": 0,
		"amount":        2,
	})

	if countClues(gs, 0) != 0 {
		t.Errorf("opp damage must not trigger; got %d clues", countClues(gs, 0))
	}
}

// -----------------------------------------------------------------------------
// Erdwal Illuminator — first investigate per turn → investigate again
// -----------------------------------------------------------------------------

func TestErdwalIlluminator_DoublesFirstInvestigateOfTurn(t *testing.T) {
	gs := newGame(t, 2)
	source := addPerm(gs, 0, "Thraben Inspector", "creature")
	source.Card.BasePower, source.Card.BaseToughness = 1, 2
	erdwal := addPerm(gs, 0, "Erdwal Illuminator", "creature")
	erdwal.Card.BasePower, erdwal.Card.BaseToughness = 1, 3

	// Manually invoke the investigate via FireInvestigateTriggers — same
	// path the engine takes from any Inspector-like causer.
	gameengine.ApplyInvestigateEffect(gs, 0, 1)
	gameengine.FireInvestigateTriggers(gs, 0, source, nil)

	// Expected: 1 from the original investigate + 1 from Erdwal's
	// additional investigate = 2 clues.
	if countClues(gs, 0) != 2 {
		t.Errorf("expected 2 clues (orig + Erdwal double); got %d", countClues(gs, 0))
	}
}

func TestErdwalIlluminator_SecondInvestigateSameTurnNoOps(t *testing.T) {
	gs := newGame(t, 2)
	source := addPerm(gs, 0, "Thraben Inspector", "creature")
	source.Card.BasePower, source.Card.BaseToughness = 1, 2
	erdwal := addPerm(gs, 0, "Erdwal Illuminator", "creature")
	erdwal.Card.BasePower, erdwal.Card.BaseToughness = 1, 3

	// First investigate — doubles to 2.
	gameengine.ApplyInvestigateEffect(gs, 0, 1)
	gameengine.FireInvestigateTriggers(gs, 0, source, nil)
	pre := countClues(gs, 0)

	// Second investigate same turn — Erdwal must NOT double again.
	gameengine.ApplyInvestigateEffect(gs, 0, 1)
	gameengine.FireInvestigateTriggers(gs, 0, source, nil)
	// One real clue added but no Erdwal doubling → exactly +1.
	if countClues(gs, 0) != pre+1 {
		t.Errorf("second investigate of turn should add only the base clue (no Erdwal); pre=%d post=%d",
			pre, countClues(gs, 0))
	}
}

func TestErdwalIlluminator_IgnoresOpponentInvestigate(t *testing.T) {
	gs := newGame(t, 2)
	source := addPerm(gs, 1, "Thraben Inspector", "creature")
	addPerm(gs, 0, "Erdwal Illuminator", "creature")

	gameengine.ApplyInvestigateEffect(gs, 1, 1)
	gameengine.FireInvestigateTriggers(gs, 1, source, nil)

	if countClues(gs, 0) != 0 {
		t.Errorf("Erdwal must not double opp investigates; seat-0 clues=%d", countClues(gs, 0))
	}
	if countClues(gs, 1) != 1 {
		t.Errorf("opp should have 1 clue from their own investigate (no Erdwal doubling); clues=%d", countClues(gs, 1))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — every handler reachable.
// -----------------------------------------------------------------------------

func TestInvestigateConsumers_AllRegistered(t *testing.T) {
	cards := []string{
		"Lonis, Cryptozoologist",
		"Thraben Inspector",
		"Briarbridge Tracker",
		"Bygone Bishop",
		"Trail of Evidence",
		"Tireless Tracker",
		"Ongoing Investigation",
		"Erdwal Illuminator",
	}
	reg := Global()
	for _, name := range cards {
		_, hasTrigger := reg.HasCastAndTrigger(normalizeName(name))
		if !hasTrigger {
			t.Errorf("expected OnTrigger handler for %s", name)
		}
	}
}
