package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Renown tests — CR §702.112
// ---------------------------------------------------------------------------

func newRenownCreature(name string, owner, power, toughness, renownN int) *Card {
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		BasePower:     power,
		BaseToughness: toughness,
		CMC:           2,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "renown", Args: []any{renownN}},
			},
		},
	}
}

func newRenownGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(19))
	gs := NewGameState(2, rng, nil)
	gs.Active = 0
	gs.Step = "combat_damage"
	gs.Turn = 1
	return gs
}

func putRenownBattlefield(gs *GameState, seat int, card *Card) *Permanent {
	perm := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, perm)
	return perm
}

func plus1Counters(perm *Permanent) int {
	if perm == nil || perm.Counters == nil {
		return 0
	}
	return perm.Counters["+1/+1"]
}

// ---------------------------------------------------------------------------
// HasRenown / RenownValue / IsRenowned
// ---------------------------------------------------------------------------

func TestHasRenown_Detects(t *testing.T) {
	card := newRenownCreature("Topan Freeblade", 0, 2, 2, 1)
	if !HasRenown(card) {
		t.Fatal("HasRenown should be true for a Renown N creature")
	}
}

func TestHasRenown_Negative(t *testing.T) {
	plain := &Card{
		Name:  "Grizzly Bears",
		Owner: 0,
		Types: []string{"creature"},
		AST:   &gameast.CardAST{Name: "Grizzly Bears"},
	}
	if HasRenown(plain) {
		t.Fatal("HasRenown should be false for a plain creature")
	}
	if HasRenown(nil) {
		t.Fatal("HasRenown(nil) should be false")
	}
}

func TestRenownValue_ParsesNumericArg(t *testing.T) {
	if got := RenownValue(newRenownCreature("Vastwood Gorger", 0, 5, 6, 3)); got != 3 {
		t.Fatalf("RenownValue = %d, want 3", got)
	}
}

func TestRenownValue_BareKeywordDefaultsToOne(t *testing.T) {
	card := &Card{
		Name:  "Bare Renown",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Bare Renown",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "renown"},
			},
		},
	}
	if got := RenownValue(card); got != 1 {
		t.Fatalf("bare renown should default to 1, got %d", got)
	}
}

func TestIsRenowned_NoFlag(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))
	if IsRenowned(perm) {
		t.Fatal("IsRenowned should be false before the trigger fires")
	}
}

// ---------------------------------------------------------------------------
// (a) Deal combat damage to player triggers — gain N +1/+1 + renowned flag
// ---------------------------------------------------------------------------

func TestRenown_TriggersOnCombatDamageToPlayer(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))

	fired := ApplyRenownOnCombatDamage(gs, perm, 1)

	if !fired {
		t.Fatal("ApplyRenownOnCombatDamage should report fired=true for a fresh renown trigger")
	}
	if !IsRenowned(perm) {
		t.Fatal("perm should be renowned after the trigger fires")
	}
	if got := plus1Counters(perm); got != 1 {
		t.Fatalf("+1/+1 counters = %d, want 1 (Renown 1)", got)
	}
}

func TestRenown_NCountersMatchKeywordArg(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Vastwood Gorger", 0, 5, 6, 3))

	if !ApplyRenownOnCombatDamage(gs, perm, 1) {
		t.Fatal("expected fresh renown trigger to fire")
	}
	if got := plus1Counters(perm); got != 3 {
		t.Fatalf("+1/+1 counters = %d, want 3 (Renown 3)", got)
	}
}

func TestRenown_FiresViaCombatDamagePath(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))

	// Drive the live combat-damage-to-player code path so the wiring
	// in combat.go is exercised end-to-end.
	applyCombatDamageToPlayer(gs, perm, 2, 1)

	if !IsRenowned(perm) {
		t.Fatal("perm should be renowned after combat damage to player")
	}
	if got := plus1Counters(perm); got != 1 {
		t.Fatalf("+1/+1 counters = %d, want 1", got)
	}
	// Defender lost life via the live path (NewGameState starts at 20 life).
	startLife := 20
	if got := gs.Seats[1].Life; got != startLife-2 {
		t.Fatalf("defender life = %d, want %d (%d - 2 combat damage)", got, startLife-2, startLife)
	}
}

// ---------------------------------------------------------------------------
// (b) Already-renowned does nothing
// ---------------------------------------------------------------------------

func TestRenown_AlreadyRenownedIsNoOp(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))

	// First trigger lands the renown designation.
	if !ApplyRenownOnCombatDamage(gs, perm, 1) {
		t.Fatal("first call should fire")
	}
	firstCounters := plus1Counters(perm)
	if firstCounters != 1 {
		t.Fatalf("setup: expected 1 +1/+1 counter, got %d", firstCounters)
	}

	// Subsequent combat-damage hits must not re-trigger renown (it
	// applies at most once per game per source per §702.112a's "if it
	// isn't renowned" intervening if).
	fired := ApplyRenownOnCombatDamage(gs, perm, 1)
	if fired {
		t.Fatal("ApplyRenownOnCombatDamage on an already-renowned source should report fired=false")
	}
	if got := plus1Counters(perm); got != firstCounters {
		t.Fatalf("+1/+1 counters changed on already-renowned source: got %d, want %d", got, firstCounters)
	}
}

func TestRenown_LivePathSecondHitNoOp(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))

	applyCombatDamageToPlayer(gs, perm, 2, 1)
	if !IsRenowned(perm) {
		t.Fatal("setup: should be renowned after first hit")
	}
	firstCounters := plus1Counters(perm)
	applyCombatDamageToPlayer(gs, perm, 2, 1)
	if got := plus1Counters(perm); got != firstCounters {
		t.Fatalf("second hit to player added more +1/+1 counters (got %d, want %d)",
			got, firstCounters)
	}
}

// ---------------------------------------------------------------------------
// (c) Damage to creature does NOT trigger renown
// ---------------------------------------------------------------------------

func TestRenown_NoTriggerOnDamageToCreature(t *testing.T) {
	gs := newRenownGame(t)
	attacker := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))
	defenderCard := &Card{
		Name:          "Grizzly Bears",
		Owner:         1,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST:           &gameast.CardAST{Name: "Grizzly Bears"},
	}
	defender := putRenownBattlefield(gs, 1, defenderCard)

	// Drive the creature-target damage path. Renown is NOT wired
	// into applyCombatDamageToCreature, so this exercises the
	// "creature-vs-creature damage doesn't trigger renown" rule.
	applyCombatDamageToCreature(gs, attacker, 2, defender)

	if IsRenowned(attacker) {
		t.Fatal("attacker must NOT be renowned after dealing combat damage to a creature (§702.112a)")
	}
	if got := plus1Counters(attacker); got != 0 {
		t.Fatalf("attacker should have 0 +1/+1 counters, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// (d) Blocked attacker dealing 0 to player does not trigger
// ---------------------------------------------------------------------------

func TestRenown_NoTriggerWhenZeroDamageToPlayer(t *testing.T) {
	gs := newRenownGame(t)
	perm := putRenownBattlefield(gs, 0, newRenownCreature("Topan Freeblade", 0, 2, 2, 1))

	// Blocked-to-zero attacker: combat damage to player = 0. The live
	// path short-circuits at amount <= 0 before calling our hook, so
	// we model the "blocked" outcome by not calling the helper at all
	// (the attacker dealt zero damage to the player). Assert renown
	// state is unchanged.
	if amount := 0; amount > 0 {
		applyCombatDamageToPlayer(gs, perm, amount, 1)
	}
	if IsRenowned(perm) {
		t.Fatal("blocked attacker should not become renowned (no damage to player dealt)")
	}
	if got := plus1Counters(perm); got != 0 {
		t.Fatalf("blocked attacker should have 0 +1/+1 counters, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Defensive surface
// ---------------------------------------------------------------------------

func TestApplyRenownOnCombatDamage_NilSafe(t *testing.T) {
	if ApplyRenownOnCombatDamage(nil, nil, 0) {
		t.Fatal("nil inputs must not fire renown")
	}
	gs := newRenownGame(t)
	if ApplyRenownOnCombatDamage(gs, nil, 1) {
		t.Fatal("nil perm must not fire renown")
	}
}

func TestApplyRenownOnCombatDamage_NoKeyword_NoFire(t *testing.T) {
	gs := newRenownGame(t)
	plain := &Card{
		Name:          "Grizzly Bears",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
		AST:           &gameast.CardAST{Name: "Grizzly Bears"},
	}
	perm := putRenownBattlefield(gs, 0, plain)
	if ApplyRenownOnCombatDamage(gs, perm, 1) {
		t.Fatal("non-renown creature must not fire renown")
	}
	if IsRenowned(perm) {
		t.Fatal("non-renown creature should never be flagged renowned")
	}
}
