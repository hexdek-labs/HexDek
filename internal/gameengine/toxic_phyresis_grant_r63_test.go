package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestPhyresisCounter_GrantsInfectInCombat pins CR §122.1c / §702.91: a
// phyresis counter grants infect, so a creature carrying one deals its
// combat damage to a player as poison counters instead of life loss.
//
// The counter is placed into the legacy Permanent.Counters map — the store
// the generic AST "put a <kind> counter" resolution path (AddCounter)
// writes to. Before the r63 fix, HasKeyword("infect") matched the counter
// only by literal name ("infect"), so a "phyresis" counter never resolved
// to the infect grant and the damage was dealt as ordinary life loss.
func TestPhyresisCounter_GrantsInfectInCombat(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := addTestPerm(gs, 0, "Compleated Creature", "creature")
	attacker.Card.BasePower = 3
	attacker.Card.BaseToughness = 3
	// Phyresis counter as the generic counter path would store it.
	attacker.AddCounter("phyresis", 1)

	if !HasInfect(attacker) {
		t.Fatal("a phyresis counter must grant infect (CR §122.1c / §702.91)")
	}

	startingLife := gs.Seats[1].Life
	applyCombatDamageToPlayer(gs, attacker, 3, 1)

	if gs.Seats[1].Life != startingLife {
		t.Errorf("infect (via phyresis) must not reduce life; expected %d, got %d",
			startingLife, gs.Seats[1].Life)
	}
	if gs.Seats[1].PoisonCounters != 3 {
		t.Errorf("expected 3 poison counters from phyresis-granted infect, got %d",
			gs.Seats[1].PoisonCounters)
	}
}

// TestInfectPlusToxic_ComposeNoDoubleCount pins the infect+toxic interaction
// (CR §702.90 infect replaces the damage with poison; §702.180 toxic adds N
// poison on top). A creature with both, dealing 3 combat damage with toxic
// 2, must give the player 3 (infect) + 2 (toxic) = 5 poison and lose no
// life — the two effects compose additively, neither double-counts.
func TestInfectPlusToxic_ComposeNoDoubleCount(t *testing.T) {
	gs := newTestGame(t, 2)
	attacker := addTestPerm(gs, 0, "Festering Stalker", "creature")
	attacker.Card.BasePower = 3
	attacker.Card.BaseToughness = 3
	attacker.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "infect"},
			&gameast.Keyword{Name: "toxic", Args: []interface{}{float64(2)}},
		},
	}

	startingLife := gs.Seats[1].Life
	applyCombatDamageToPlayer(gs, attacker, 3, 1)

	if gs.Seats[1].Life != startingLife {
		t.Errorf("infect must not reduce life; expected %d, got %d",
			startingLife, gs.Seats[1].Life)
	}
	if gs.Seats[1].PoisonCounters != 5 {
		t.Errorf("expected 5 poison (3 infect + 2 toxic), got %d",
			gs.Seats[1].PoisonCounters)
	}
}

// TestMultipleToxicSources_StackPoison pins that two separate toxic
// attackers each contribute their N poison to the same player (CR §702.180c
// fires per source on combat damage). 7174n1c probe item (1): "stacks with
// multiple toxic sources."
func TestMultipleToxicSources_StackPoison(t *testing.T) {
	gs := newTestGame(t, 2)
	mk := func(name string, n int) *Permanent {
		p := addTestPerm(gs, 0, name, "creature")
		p.Card.BasePower, p.Card.BaseToughness = 1, 1
		p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "toxic", Args: []interface{}{float64(n)}},
		}}
		return p
	}
	a := mk("Toxic A", 1)
	b := mk("Toxic B", 3)

	applyCombatDamageToPlayer(gs, a, 1, 1)
	applyCombatDamageToPlayer(gs, b, 1, 1)

	if gs.Seats[1].PoisonCounters != 4 {
		t.Errorf("expected 4 poison (1 + 3) from two toxic sources, got %d",
			gs.Seats[1].PoisonCounters)
	}
}
