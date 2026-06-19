package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// suspect_target_r63_test.go — Suspect (CR §701.62) WHICH-creature wiring.
//
// The core designation (menace + can't-block + persistence) was already wired
// for self-suspect. These pin the target-selection fixes: "suspect enchanted
// creature" hit the Aura (no-op), "suspect_it" fell through inert, and
// suspect_typed ("suspect target creature") wrongly suspected the source.

func doSuspectMod(gs *GameState, src *Permanent, kind string, args ...interface{}) {
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: kind, Args: args})
}

// (1) Self-suspect ("suspect" / "pronoun") suspects the source and grants the
// persistent menace + can't-block designation.
func TestSuspect_SelfPronoun(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Person of Interest", 2, 2, "creature")
	doSuspectMod(gs, c, "suspect", "pronoun")

	if !IsSuspected(c) {
		t.Fatal("source should be suspected")
	}
	if !c.HasKeyword("menace") {
		t.Fatal("suspected creature should have menace")
	}
}

// (2) "suspect enchanted creature" suspects the Aura's attached creature, NOT
// the Aura itself (the old handler no-op'd on the non-creature Aura).
func TestSuspect_EnchantedCreature(t *testing.T) {
	gs := newCombatGame(t)
	creature := addBattlefield(gs, 0, "Enchanted Bear", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Convenient Target", 0, 0, "enchantment", "aura")
	aura.AttachedTo = creature

	doSuspectMod(gs, aura, "suspect", "enchanted_creature")

	if IsSuspected(aura) {
		t.Fatal("the Aura itself must not be suspected")
	}
	if !IsSuspected(creature) {
		t.Fatal("the enchanted creature should be suspected")
	}
	if !creature.HasKeyword("menace") {
		t.Fatal("enchanted creature should gain menace")
	}
}

// (3) suspect_it (parser synonym, empty args) suspects the source — was inert.
func TestSuspect_SuspectIt(t *testing.T) {
	gs := newCombatGame(t)
	c := addBattlefield(gs, 0, "Barbed Servitor", 3, 3, "creature")
	doSuspectMod(gs, c, "suspect_it")

	if !IsSuspected(c) {
		t.Fatal("suspect_it should suspect the source (was inert before the fix)")
	}
}

// (4) suspect_typed ("suspect target creature") suspects an OPPONENT'S creature,
// not the source (the old _typed alias suspected the source).
func TestSuspect_TypedTargetsOpponent(t *testing.T) {
	gs := newCombatGame(t)
	nelly := addBattlefield(gs, 0, "Nelly Borca, Impulsive Accuser", 3, 3, "creature")
	victim := addBattlefield(gs, 1, "Opp Blocker", 2, 2, "creature")

	doSuspectMod(gs, nelly, "suspect_typed")

	if IsSuspected(nelly) {
		t.Fatal("suspect_typed must NOT suspect the source")
	}
	if !IsSuspected(victim) {
		t.Fatal("suspect_typed should suspect an opponent's creature")
	}
}

// (5) Behavioral integration: a suspected creature can't block (CR §701.62c).
func TestSuspect_CantBlock(t *testing.T) {
	gs := newCombatGame(t)
	attacker := addBattlefield(gs, 0, "Attacker", 3, 3, "creature")
	blocker := addBattlefield(gs, 1, "Suspect Blocker", 4, 4, "creature")

	if !canBlockGS(gs, attacker, blocker) {
		t.Fatal("precondition: blocker should be able to block before being suspected")
	}
	doSuspectMod(gs, blocker, "suspect", "pronoun")
	if canBlockGS(gs, attacker, blocker) {
		t.Fatal("a suspected creature must not be able to block")
	}
}

// (6) Unsuspect clears the designation, menace, and the can't-block restriction.
func TestSuspect_UnsuspectClears(t *testing.T) {
	gs := newCombatGame(t)
	attacker := addBattlefield(gs, 0, "Attacker", 3, 3, "creature")
	c := addBattlefield(gs, 1, "Investigated", 2, 2, "creature")
	doSuspectMod(gs, c, "suspect", "pronoun")

	UnsuspectCreature(gs, c)
	if IsSuspected(c) {
		t.Fatal("unsuspect should clear the designation")
	}
	if c.HasKeyword("menace") {
		t.Fatal("unsuspect should clear the suspect-granted menace")
	}
	if !canBlockGS(gs, attacker, c) {
		t.Fatal("an un-suspected creature should be able to block again")
	}
}
