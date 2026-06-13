package hat

// Regression for the deep loki r63b sweep finding: the block-assignment
// menace check used the non-layer-aware p.HasKeyword("menace"), so a
// LAYER-GRANTED menace (anthem / continuous keyword grant) was invisible
// to the Hat. The Hat declared a SINGLE blocker against such an attacker;
// the engine's block sanitizer (layer-aware keywordActive) then dropped
// the illegal block (§702.110b), leaving the menace creature unblocked —
// 54 §702.110b hits across the 4000-game max-turns-90 sweep, every one a
// granted-menace attacker. The fix makes the Hat's menace check
// layer-aware (gs.HasKeywordOf) to match the engine.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// grantLayerMenace registers a §613 layer-6 continuous effect that grants
// "menace" to exactly `target`, so gs.HasKeywordOf(target,"menace") is
// true while target.HasKeyword("menace") stays false (no printed / Flags
// keyword) — exactly the granted-menace shape the sweep surfaced.
func grantLayerMenace(gs *gameengine.GameState, target *gameengine.Permanent) {
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:      gameengine.LayerAbility,
		Timestamp:  1,
		SourcePerm: target,
		HandlerID:  "test_grant_menace",
		Predicate:  func(_ *gameengine.GameState, p *gameengine.Permanent) bool { return p == target },
		ApplyFn: func(_ *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			chars.Keywords = append(chars.Keywords, "menace")
		},
	})
	gs.InvalidateCharacteristicsCache()
}

// assertNeverSingleBlockOnGrantedMenace runs one Hat and checks it never
// assigns EXACTLY ONE blocker to a granted-menace attacker (the illegal
// state the engine would have to sanitize). With three 4/4 survivors
// against a 3/3, the menace-aware Hat should pair two blockers.
func assertNeverSingleBlockOnGrantedMenace(t *testing.T, label string, h gameengine.Hat) {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[1].Hat = h
	gs.Seats[1].Life = 40

	atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Sneaky Attacker", []string{"creature"}, 3, nil), 3, 3)
	// Three survivors (4/4 outlive a 3/3) so the defender genuinely wants
	// to block — the menace pairing must then take two of them.
	for i := 0; i < 3; i++ {
		b := newTestPermanent(gs.Seats[1], newTestCardMinimal("Wall", []string{"creature"}, 3, nil), 4, 4)
		b.Tapped = false
	}

	grantLayerMenace(gs, atk)
	if atk.HasKeyword("menace") {
		t.Fatalf("%s: precondition — attacker must NOT have a printed/Flags menace", label)
	}
	if !gs.HasKeywordOf(atk, "menace") {
		t.Fatalf("%s: precondition — attacker must have LAYER-granted menace", label)
	}

	out := h.AssignBlockers(gs, 1, []*gameengine.Permanent{atk})
	n := len(out[atk])
	if n == 1 {
		t.Fatalf("%s: assigned exactly ONE blocker to a granted-menace attacker — illegal (§702.110b), engine would drop it leaving the creature unblocked", label)
	}
	// With three survivors available it should pair two (the correct play),
	// not bail.
	if n != 2 {
		t.Errorf("%s: expected a 2-creature menace block, got %d", label, n)
	}
}

func TestGreedyHat_GrantedMenace_NoSingleBlock(t *testing.T) {
	assertNeverSingleBlockOnGrantedMenace(t, "GreedyHat", &GreedyHat{})
}

func TestYggdrasilHat_GrantedMenace_NoSingleBlock(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	h.Noise = 0
	assertNeverSingleBlockOnGrantedMenace(t, "YggdrasilHat", h)
}
