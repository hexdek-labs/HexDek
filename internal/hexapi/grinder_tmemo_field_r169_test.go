package hexapi

import (
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameengine/per_card"
)

// r169 — the grinder-line wiring for tmemo=, tested from the CALLER's
// package (r163's lesson: a field is wired by its caller, and a test
// that lives only in the producing package cannot see the caller drop
// it).
//
// r167 shipped a field that compiled, had five green tests, and printed
// nothing through two real capped games, because every one of those
// tests hand-fed the value at the destination. The two tests here split
// that failure in half: one proves the NUMBER is really produced by
// real dispatch, the other proves the LOG LINE really asks for it.

// TestR169_TriggerMemoCountersMoveUnderRealDispatch drives the
// production entry point on a real board and asserts the counters the
// grinder line reads actually move. No hand-fed value: if the memo were
// never consulted, both counters would stay at zero and the field would
// print nothing forever — exactly r167's failure.
func TestR169_TriggerMemoCountersMoveUnderRealDispatch(t *testing.T) {
	per_card.ResetTriggerMemoStatsForTest()

	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(42)), nil)
	card := &gameengine.Card{Name: "R169 Hexapi Bystander", BasePower: 2, BaseToughness: 2,
		Types: []string{"creature"}}
	perm := &gameengine.Permanent{Card: card, Controller: 0, Owner: 0}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0, "perm": perm,
	})
	h1, m1 := per_card.TriggerMemoStats()
	if h1+m1 == 0 {
		t.Fatalf("real dispatch must consult the memo; hits=%d misses=%d", h1, m1)
	}
	if m1 == 0 {
		t.Errorf("the first look at a fresh card must be a miss; misses=%d", m1)
	}

	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": 0, "perm": perm,
	})
	h2, _ := per_card.TriggerMemoStats()
	if h2 <= h1 {
		t.Errorf("the second look at the same card must be a hit; hits went %d -> %d", h1, h2)
	}
}

// TestR169_GrinderLineWiresTheTriggerMemoField asserts on SOURCE, which
// is cruder than driving the code and is admitted as such. Its only job
// is to fail if someone deletes the field from the Printf while leaving
// the counters in place — the precise shape of r167's defect, which no
// amount of testing in per_card could have caught.
func TestR169_GrinderLineWiresTheTriggerMemoField(t *testing.T) {
	// r176 split the rendering into grinder_line_r176.go; tmemo is
	// still computed in showmatch.go (it is a window rate owned by the
	// ticker loop) and passed through, so this checks both halves.
	for _, f := range []struct {
		file    string
		needles []string
	}{
		{"showmatch.go", []string{
			"per_card.TriggerMemoStats()",
			`" tmemo=%.1f%% (%d/%d)"`,
			"tmemo:          tmemo,",
		}},
		{"grinder_line_r176.go", []string{
			"guards, deadlines, w.tmemo, guardTop, tdepth, tpField, condUnk,",
		}},
	} {
		b, err := os.ReadFile(f.file)
		if err != nil {
			t.Fatalf("read %s: %v", f.file, err)
		}
		src := string(b)
		for _, needle := range f.needles {
			if !strings.Contains(src, needle) {
				t.Errorf("%s no longer contains %s — the tmemo field is not wired", f.file, needle)
			}
		}
	}
}
