package hat

// Regression for the live-grinder §509.1 watchdog finding "blocker committed
// twice in one block assignment" (e.g. Bothersome Quasit twice vs Ardbert,
// Surgespanner twice vs Vishgraz). The watchdog log was from a STALE grinder
// binary; on current main the AI block-assignment policies (GreedyHat /
// YggdrasilHat — the grinder uses GreedyHat) never commit a creature twice:
// the menace second-blocker is picked from `extras` gated on `b != chosen[0]`,
// and a per-policy `used` map keeps a blocker out of any other attacker's list.
// 2000+ legality-instrumented games reproduced 0 hits.
//
// These tests pin that invariant so a future policy edit can't re-introduce a
// double-commit (which the §509.1 legality watchdog flags on the RAW policy
// output, before the engine sanitizer drops it).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// assertNoDuplicateBlocker runs a Hat over a board and asserts that no
// attacker's blocker list contains the same creature twice, and that no
// creature is committed to more than one attacker.
func assertNoDuplicateBlocker(t *testing.T, label string, h gameengine.Hat, gs *gameengine.GameState, defenderSeat int, attackers []*gameengine.Permanent) {
	t.Helper()
	out := h.AssignBlockers(gs, defenderSeat, attackers)
	committed := map[*gameengine.Permanent]*gameengine.Permanent{} // blocker -> attacker it blocks
	for _, atk := range attackers {
		seen := map[*gameengine.Permanent]bool{}
		for _, b := range out[atk] {
			if b == nil {
				continue
			}
			if seen[b] {
				t.Errorf("%s: blocker %q committed TWICE against the same attacker %q (§509.1)",
					label, b.Card.DisplayName(), atk.Card.DisplayName())
			}
			seen[b] = true
			if prev, ok := committed[b]; ok && prev != atk {
				t.Errorf("%s: blocker %q committed to TWO attackers (%q and %q) (§509.1, no multi-block)",
					label, b.Card.DisplayName(), prev.Card.DisplayName(), atk.Card.DisplayName())
			}
			committed[b] = atk
		}
	}
}

// A granted-menace attacker with exactly TWO legal survivors is the tightest
// double-commit trap: the policy needs two blockers but must not reuse the
// first as the second.
func TestAssignBlockers_NoDoubleCommit_MenaceTwoBlockers(t *testing.T) {
	build := func() (*gameengine.GameState, *gameengine.Permanent, []*gameengine.Permanent) {
		gs := newTestGame(t, 2)
		gs.Seats[1].Life = 40
		atk := newTestPermanent(gs.Seats[0], newTestCardMinimal("Ardbert", []string{"creature"}, 3, nil), 3, 3)
		grantLayerMenace(gs, atk)
		var blockers []*gameengine.Permanent
		for i := 0; i < 2; i++ {
			b := newTestPermanent(gs.Seats[1], newTestCardMinimal("Bothersome Quasit", []string{"creature"}, 3, nil), 4, 4)
			blockers = append(blockers, b)
		}
		return gs, atk, blockers
	}

	for _, tc := range []struct {
		name string
		hat  func() gameengine.Hat
	}{
		{"GreedyHat", func() gameengine.Hat { return &GreedyHat{} }},
		{"YggdrasilHat", func() gameengine.Hat { h := NewYggdrasilHat(nil, 0); h.Noise = 0; return h }},
	} {
		gs, atk, _ := build()
		gs.Seats[1].Hat = tc.hat()
		assertNoDuplicateBlocker(t, tc.name, tc.hat(), gs, 1, []*gameengine.Permanent{atk})
	}
}

// Multiple attackers competing for a small blocker pool must not let one
// creature be committed to two attackers.
func TestAssignBlockers_NoDoubleCommit_MultipleAttackers(t *testing.T) {
	for _, tc := range []struct {
		name string
		hat  func() gameengine.Hat
	}{
		{"GreedyHat", func() gameengine.Hat { return &GreedyHat{} }},
		{"YggdrasilHat", func() gameengine.Hat { h := NewYggdrasilHat(nil, 0); h.Noise = 0; return h }},
	} {
		gs := newTestGame(t, 2)
		gs.Seats[1].Life = 6 // low life → wants to block everything
		var attackers []*gameengine.Permanent
		for i := 0; i < 3; i++ {
			attackers = append(attackers, newTestPermanent(gs.Seats[0], newTestCardMinimal("Vishgraz", []string{"creature"}, 4, nil), 4, 4))
		}
		// Only two blockers for three attackers — forces contention.
		for i := 0; i < 2; i++ {
			newTestPermanent(gs.Seats[1], newTestCardMinimal("Surgespanner", []string{"creature"}, 2, nil), 2, 2)
		}
		gs.Seats[1].Hat = tc.hat()
		assertNoDuplicateBlocker(t, tc.name, tc.hat(), gs, 1, attackers)
	}
}
