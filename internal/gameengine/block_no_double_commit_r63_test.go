package gameengine

import "testing"

// block_no_double_commit_r63_test.go — defense-in-depth for the §509.1
// "blocker committed twice in one assignment" watchdog finding. The AI
// policies no longer produce a double-commit (pinned in internal/hat); these
// pin the ENGINE backstop so that even if a policy ever regresses, the engine
// never executes a double-block and the legality watchdog still flags it.

// The engine sanitizer drops a blocker committed twice against one attacker.
func TestSanitizeDeclaredBlockers_DropsDoubleCommit(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Ardbert", 3, 3)
	b := addCreature(gs, 1, "Bothersome Quasit", 4, 4)

	kept := sanitizeDeclaredBlockers(gs, 1, atk, []*Permanent{b, b}) // same blocker twice
	if len(kept) != 1 || kept[0] != b {
		names := make([]string, len(kept))
		for i, k := range kept {
			names[i] = k.Card.DisplayName()
		}
		t.Fatalf("sanitizer must collapse a double-committed blocker to ONE, got %d %v", len(kept), names)
	}
}

// The §509.1 legality check FLAGS a raw declaration that commits a blocker
// twice (this is what the live-grinder watchdog reports).
func TestCheckLegalityBlockDecl_FlagsDoubleCommit(t *testing.T) {
	gs := newCombatGame(t)
	atk := addCreature(gs, 0, "Vishgraz", 4, 4)
	b := addCreature(gs, 1, "Surgespanner", 2, 2)

	obs := &LegalityObservation{
		Kind:                 "block",
		Seat:                 1,
		Attacker:             atk,
		Blockers:             []*Permanent{b, b},
		BlockersWereBlocking: []bool{false, false},
	}
	viols := checkLegalityBlockDecl(gs, obs)
	found := false
	for _, v := range viols {
		if v.Rule == "509.1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("checkLegalityBlockDecl must flag a double-committed blocker as §509.1, got %+v", viols)
	}
}
