package hat

// Repro for the LIVE grinder §509.1 finding that survived the first
// no-double-commit pass: "blocker committed twice (block:Noxious Gearhulk)" /
// "(block:Ikra Shidiqi)". The earlier regression only exercised the
// single-survivor menace case, where `chosen` holds exactly one blocker and
// the menace second-blocker is picked from `extras` gated on `b != chosen[0]`.
//
// YggdrasilHat has a SECOND path the GreedyHat lacks: the gang-block branch
// sets `chosen` to a MULTI-creature gang (2-3 blockers) against a dangerous
// must-block attacker. When that attacker also has menace, the menace append
// excluded only chosen[0] — so it could re-append a creature already in the
// gang, committing it twice against the same attacker. The grinder server runs
// YggdrasilHat (not GreedyHat), so the finding is real, not a stale binary.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// A dangerous menace attacker that no single blocker can kill, against a
// low-life defender, forces YggdrasilHat down the gang-block path AND the
// menace second-blocker path simultaneously — the exact double-commit trap.
func TestYggdrasil_AssignBlockers_NoDoubleCommit_GangPlusMenace(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 4 // low → willDie → gang path engages

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Noxious Gearhulk", []string{"creature"}, 6, nil), 5, 4)
	grantLayerMenace(gs, atk)

	// Three modest blockers: no single one survives or kills the 5/4, so the
	// hat wants to gang up; the gang is 2-3 creatures and menace needs 2+.
	for i := 0; i < 3; i++ {
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Skirk Prospector", []string{"creature"}, 1, nil), 3, 3)
	}

	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h
	assertNoDuplicateBlocker(t, "YggdrasilHat/gang+menace", h, gs, 1, []*gameengine.Permanent{atk})
}

// The dedup must REJECT the duplicate without suppressing a legitimate menace
// pairing: a menace attacker still needs 2+ DISTINCT blockers (§702.110b), and
// the gang path already provides them. Same board as above — the fix must
// leave the assignment with at least two distinct creatures (so menace is
// satisfied) and zero duplicates. (Engine-level multi-block-grant preservation
// — a "block any number" creature across distinct attackers — is pinned
// separately in gameengine/block_additional_cap_r63_test.go.)
func TestYggdrasil_AssignBlockers_MenacePairingStillTwoDistinct(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[1].Life = 4

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Noxious Gearhulk", []string{"creature"}, 6, nil), 5, 4)
	grantLayerMenace(gs, atk)
	for i := 0; i < 3; i++ {
		newTestPermanent(gs.Seats[1], newTestCardMinimal("Skirk Prospector", []string{"creature"}, 1, nil), 3, 3)
	}

	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	gs.Seats[1].Hat = h

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	blockers := out[atk]
	distinct := map[*gameengine.Permanent]bool{}
	for _, b := range blockers {
		if b == nil {
			continue
		}
		if distinct[b] {
			t.Fatalf("menace block double-committed %q (§509.1)", b.Card.DisplayName())
		}
		distinct[b] = true
	}
	// If the hat blocks a menace attacker at all, §702.110b requires 2+
	// distinct creatures; a single-blocker menace declaration is illegal.
	if len(blockers) == 1 {
		t.Fatalf("menace attacker blocked by exactly one creature (§702.110b)")
	}
	if len(blockers) >= 2 && len(distinct) < 2 {
		t.Fatalf("menace pairing must be 2+ DISTINCT blockers, got %d distinct of %d", len(distinct), len(blockers))
	}
}
