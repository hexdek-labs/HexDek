package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestTriggerDrainLoopGuard_CopySpellCascade pins the LIVENESS fix for the
// live-grinder game-216 stall: an unbounded spell-COPY cascade inside
// drainPendingTriggers' batch/stack drain. Root cause was Veyran, Voice of
// Duality doubling magecraft/copy triggers so every copy spawned more copies;
// resolveCopySpell mints+pushes a copy each time, and that work is NOT a
// counted "trigger fire", so the per-turn trigger_loop_cap never trips and the
// drain loop spins forever (the live game ran past the 10-minute liveness
// budget and was abandoned by the watchdog).
//
// This reproduces the same shape with a synthetic self-copying spell: a
// copyable instant whose effect is "copy a spell" sits on the stack as the
// persistent copy target, and a triggered copy-spell ability drains into it —
// each resolution copies the instant and pushes a fresh copy-spell, forever.
// The maxTriggerDrainIterations guard must bound the drain and end the game as
// a draw (CR §104.4b), rather than hang.
func TestTriggerDrainLoopGuard_CopySpellCascade(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	src := &Permanent{
		Card:       &Card{Name: "Loop Source", Types: []string{"enchantment"}, Owner: 0},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	// Persistent copy TARGET: a copyable instant whose effect copies a spell.
	// It sits at the stack baseline and is never resolved (the inner drain
	// loop stops above it), so every copy finds it again.
	echo := &Card{Name: "Echo", Types: []string{"instant"}, Owner: 0}
	gs.Stack = append(gs.Stack, &StackItem{
		Controller: 0, Card: echo, Kind: "spell",
		Effect: &gameast.CopySpell{},
	})

	// Seed the cascade: a triggered "copy a spell" ability. Draining it copies
	// Echo → pushes a copy-spell → resolving that copies Echo → … unbounded.
	gs.pendingTriggers = append(gs.pendingTriggers, &StackItem{
		Controller: 0, Source: src, Kind: "triggered",
		Effect: &gameast.CopySpell{},
	})

	// Must TERMINATE (the package test -timeout would otherwise kill the suite)
	// and end the game as a draw via the trigger-drain loop guard.
	drainPendingTriggers(gs)

	if gs.Flags["ended"] != 1 {
		t.Fatal("drain loop did not terminate the game; the trigger-drain guard failed to fire")
	}
	if gs.Flags["game_draw"] != 1 {
		t.Error("unbreakable copy loop must end the game as a draw (CR §104.4b)")
	}
	sawGuard := false
	for _, ev := range gs.EventLog {
		if ev.Kind == EventLoopGuardFired {
			if g, _ := ev.Details["guard"].(string); g == "trigger_drain_cap" {
				sawGuard = true
				break
			}
		}
	}
	if !sawGuard {
		t.Error("expected a loop_guard_fired event with guard=trigger_drain_cap")
	}
}
