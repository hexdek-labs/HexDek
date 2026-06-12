package gameengine

// Regression tests for r62 follow-up #1 (phase-3 report): the §605
// mana-ability check's per_card blind spot. Previously, inline
// activations with no AST Activated node (obs.Ability == nil) were
// blanket-EXEMPT from the 605.1a/605.3a discipline because their call
// sites didn't declare why they resolved inline. Now:
//
//   - every engine inline-mana site (ApplyArtifactMana, the turn
//     runner's land taps) declares NoStackReason="mana_ability";
//   - the exemption is REMOVED: an undeclared no-stack-item completion
//     flags 605.3a regardless of AST presence;
//   - AST-less mana claims are verified BEHAVIORALLY: a window that
//     claimed mana_ability but produced no mana flags 605.1a;
//   - AST-less inline-mana windows skip the cost check (a Signet
//     legitimately pays {1} inside its window; announced cost is
//     un-derivable without an AST).

import (
	"math/rand"
	"testing"
)

func manaFixture(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(31)), nil)
	gs.Seed = 31
	gs.Phase = "main"
	gs.Active = 0
	gs.Legality = NewLegalityValidator(31)
	return gs
}

func artifactPerm(gs *GameState, seat int, name string, typeLine string) *Permanent {
	p := &Permanent{
		Card: &Card{
			Name: name, Owner: seat,
			Types:    []string{"artifact"},
			TypeLine: typeLine,
		},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// The exemption is gone: an inline completion with NO declared reason
// and NO AST flags 605.3a (this was silent before this change).
func TestManaAbilityInline_UndeclaredCompletion_Flagged(t *testing.T) {
	gs := manaFixture(t)
	rogue := artifactPerm(gs, 0, "Rogue Engine", "artifact")

	// Simulate a rogue inline site: window opened, resolution did
	// arbitrary things, finished with NO reason declared.
	obs := gs.Legality.BeginActivation(gs, 0, rogue, -1, nil)
	gs.Legality.FinishActivation(gs, obs, nil)

	found := false
	for _, v := range gs.Legality.Violations {
		if v.Rule == "605.3a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("undeclared AST-less inline completion not flagged (the removed exemption is back); violations=%v", gs.Legality.Violations)
	}
}

// Behavioral verification: an AST-less window CLAIMING mana_ability that
// produced no mana flags 605.1a — the exact slip the task describes (a
// per_card handler resolving a non-mana ability inline as if it were a
// mana ability, denying opponents responses).
func TestManaAbilityInline_ClaimedManaNoProduction_Flagged(t *testing.T) {
	gs := manaFixture(t)
	faker := artifactPerm(gs, 0, "Response Denier", "artifact")

	obs := gs.Legality.BeginActivation(gs, 0, faker, -1, nil)
	// ...inline resolution that does something non-mana (e.g. draws)...
	obs.SetNoStackReason("mana_ability")
	gs.Legality.FinishActivation(gs, obs, nil)

	found := false
	for _, v := range gs.Legality.Violations {
		if v.Rule == "605.1a" {
			found = true
		}
	}
	if !found {
		t.Fatalf("claimed-mana-produced-none inline completion not flagged; violations=%v", gs.Legality.Violations)
	}
}

// The real engine sites are clean end-to-end: Sol Ring through
// ApplyArtifactMana produces mana inside its declared window — zero
// violations, no leaked active frames.
func TestManaAbilityInline_SolRing_Clean(t *testing.T) {
	gs := manaFixture(t)
	ring := artifactPerm(gs, 0, "Sol Ring", "artifact")

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], ring)
	if !ok || pips != 2 {
		t.Fatalf("Sol Ring tap failed: pips=%d ok=%v", pips, ok)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("Sol Ring inline mana flagged (false positive): %v", gs.Legality.Violations)
	}
	if n := len(gs.Legality.ActiveObservations()); n != 0 {
		t.Errorf("leaked %d active observation frames after ApplyArtifactMana", n)
	}
}

// A Signet pays {1} inside its window (net +1): the cost check must not
// false-positive on AST-less inline-mana windows.
func TestManaAbilityInline_Signet_CostCheckSkipped(t *testing.T) {
	gs := manaFixture(t)
	signet := artifactPerm(gs, 0, "Azorius Signet", "artifact")
	gs.Seats[0].ManaPool = 1
	EnsureTypedPool(gs.Seats[0])

	pips, ok := ApplyArtifactMana(gs, gs.Seats[0], signet)
	if !ok || pips != 2 {
		t.Fatalf("Signet tap failed: pips=%d ok=%v", pips, ok)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("Signet net pool = %d (want 2: paid 1, added 2)", gs.Seats[0].ManaPool)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("Signet inline mana flagged (cost-check skip missing): %v", gs.Legality.Violations)
	}
}

// ok=false abandons the window without running checks and without
// leaking the frame (the artifact was never tapped — nothing happened).
func TestManaAbilityInline_FailedTap_Abandoned(t *testing.T) {
	gs := manaFixture(t)
	signet := artifactPerm(gs, 0, "Azorius Signet", "artifact")
	// No mana to pay the Signet's {1} → applyArtifactManaImpl ok=false.
	if _, ok := ApplyArtifactMana(gs, gs.Seats[0], signet); ok {
		t.Fatal("expected Signet tap to fail with an empty pool")
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("failed tap produced violations: %v", gs.Legality.Violations)
	}
	if n := len(gs.Legality.ActiveObservations()); n != 0 {
		t.Errorf("leaked %d active observation frames after failed tap", n)
	}
}
