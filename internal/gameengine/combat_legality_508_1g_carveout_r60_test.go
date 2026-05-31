package gameengine

import (
	"math/rand"
	"strings"
	"testing"
)

// TestCombatLegality_508_1g_DefenderCarveOut pins the CR §508.1g
// carve-out introduced in the PR fixing the pathological-gauntlet
// finding (PR #950 docs/loki-pathological-r60.md game 1317): a
// defender creature placed onto the battlefield in an attacking
// state by an effect (Raph & Mikey library-dig in this repro) does
// NOT trip the bare-fact "defender && attacking" check, because
// §508.1g exempts effect-placed attackers from §508.1a-f.
func TestCombatLegality_508_1g_DefenderCarveOut(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	wall := &Permanent{
		Card: &Card{
			Name:          "Wall of Tanglecord",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 6,
		},
		Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{"kw:defender": 1, "kw:reach": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, wall)

	// Pre-fix shape — flagAttacking set, no carve-out tag — must fire.
	wall.Flags[flagAttacking] = 1
	if err := checkCombatLegality(gs); err == nil {
		t.Fatalf("pre-carve-out: expected invariant to fire on bare defender attacking; got nil")
	} else if !strings.Contains(err.Error(), "has defender") {
		t.Fatalf("pre-carve-out: expected defender error message; got %v", err)
	}

	// Post-fix — MarkEnteredAttacking applied — must NOT fire.
	MarkEnteredAttacking(wall)
	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("post-MarkEnteredAttacking: expected clean; got %v", err)
	}
}

// TestCombatLegality_508_1g_SummoningSickCarveOut mirrors the
// carve-out for summoning sickness — same §508.1g rationale.
func TestCombatLegality_508_1g_SummoningSickCarveOut(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	c := &Permanent{
		Card: &Card{
			Name:          "Behemoth of Vault 0",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     5,
			BaseToughness: 5,
		},
		Controller: 0, Owner: 0,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{flagAttacking: 1},
		SummoningSick: true,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, c)

	if err := checkCombatLegality(gs); err == nil {
		t.Fatalf("pre-carve-out: expected invariant to fire on SS attacking; got nil")
	} else if !strings.Contains(err.Error(), "summoning sickness") {
		t.Fatalf("pre-carve-out: expected SS error message; got %v", err)
	}

	MarkEnteredAttacking(c)
	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("post-MarkEnteredAttacking: expected clean; got %v", err)
	}
}

// TestCombatLegality_508_1g_NormalDefenderStillCaught defends the
// carve-out from being over-broad — a normal defender creature that
// somehow ended up with flagAttacking set WITHOUT the §508.1g tag
// must still trip the invariant (catches future regressions where a
// non-§508.1g code path accidentally sets flagAttacking).
func TestCombatLegality_508_1g_NormalDefenderStillCaught(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	gs.Phase = "combat"
	gs.Step = "declare_attackers"

	wall := &Permanent{
		Card: &Card{
			Name:          "Wall of Tanglecord",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     0,
			BaseToughness: 6,
		},
		Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{flagAttacking: 1, "kw:defender": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, wall)

	if err := checkCombatLegality(gs); err == nil {
		t.Fatalf("normal defender attacker (no §508.1g tag) must still trip; got nil")
	}
}

// TestEndOfCombatStep_ClearsFlagEnteredAttacking pins the cleanup:
// the §508.1g tag is torn down at end of combat alongside
// flagAttacking, so a creature surviving combat doesn't carry the
// carve-out into the next combat phase.
func TestEndOfCombatStep_ClearsFlagEnteredAttacking(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	gs.Phase = "combat"
	gs.Step = "end_of_combat"

	c := &Permanent{
		Card: &Card{
			Name:          "Dug Creature",
			Owner:         0,
			Types:         []string{"creature"},
			BasePower:     2,
			BaseToughness: 2,
		},
		Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{},
		Flags:     map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, c)
	MarkEnteredAttacking(c)

	if !permFlag(c, flagEnteredAttacking) || !permFlag(c, flagAttacking) {
		t.Fatalf("setup: MarkEnteredAttacking didn't stamp both flags")
	}

	EndOfCombatStep(gs)

	if permFlag(c, flagEnteredAttacking) {
		t.Errorf("post-EndOfCombatStep: flagEnteredAttacking must be cleared")
	}
	if permFlag(c, flagAttacking) {
		t.Errorf("post-EndOfCombatStep: flagAttacking must be cleared")
	}
}

// TestMarkEnteredAttacking_NilSafe + idempotent guards the helper's
// edge cases.
func TestMarkEnteredAttacking_NilSafeAndIdempotent(t *testing.T) {
	// nil perm must not panic.
	MarkEnteredAttacking(nil)

	p := &Permanent{Flags: nil}
	MarkEnteredAttacking(p)
	if p.Flags == nil {
		t.Fatalf("MarkEnteredAttacking should allocate Flags if nil")
	}
	if p.Flags[flagAttacking] != 1 || p.Flags[flagEnteredAttacking] != 1 {
		t.Fatalf("after first call: got %v", p.Flags)
	}
	// Second call — still 1, no change.
	MarkEnteredAttacking(p)
	if p.Flags[flagAttacking] != 1 || p.Flags[flagEnteredAttacking] != 1 {
		t.Fatalf("after second call: got %v", p.Flags)
	}
}
