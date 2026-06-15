package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §707.2 — a face-down permanent has mana value 0. Target-filter mana-value
// constraints must use 0 for a face-down creature, not the (hidden) real card's
// CMC. Before r63 the face-down branch of matchesPermanent returned true for any
// creature filter, ignoring the mana-value constraint entirely.

func mvFilter(op string, v int) gameast.Filter {
	return gameast.Filter{Base: "creature", Targeted: true, ManaValueOp: op, ManaValue: &v}
}

func TestFaceDownManaValue_IsZeroForTargetFilter(t *testing.T) {
	// Face-down creature whose REAL card CMC is 6.
	fd := &Permanent{
		Card:       &Card{Name: "Hidden", Types: []string{"creature"}, CMC: 6, FaceDown: true},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}

	// MV 2 or less → face-down MV 0 qualifies.
	if !matchesPermanent(mvFilter("<=", 2), fd) {
		t.Fatal("face-down (MV 0) should match 'mana value 2 or less'")
	}
	// MV 1 or greater → face-down MV 0 does NOT qualify (the bug: real CMC 6 would).
	if matchesPermanent(mvFilter(">=", 1), fd) {
		t.Fatal("face-down (MV 0) must NOT match 'mana value 1 or greater' (real CMC 6 must not leak through)")
	}
	// MV 5 or greater → no.
	if matchesPermanent(mvFilter(">=", 5), fd) {
		t.Fatal("face-down (MV 0) must NOT match 'mana value 5 or greater'")
	}
	// MV exactly 0 → yes.
	if !matchesPermanent(mvFilter("==", 0), fd) {
		t.Fatal("face-down (MV 0) should match 'mana value 0'")
	}
}

func TestFaceUpManaValue_UsesRealCMC(t *testing.T) {
	// Control: the SAME card face up uses its real CMC of 6.
	fu := &Permanent{
		Card:       &Card{Name: "Hidden", Types: []string{"creature"}, CMC: 6, FaceDown: false},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	if matchesPermanent(mvFilter("<=", 2), fu) {
		t.Fatal("face-up CMC-6 creature must NOT match 'mana value 2 or less'")
	}
	if !matchesPermanent(mvFilter(">=", 5), fu) {
		t.Fatal("face-up CMC-6 creature should match 'mana value 5 or greater'")
	}
}

// A plain creature filter (no mana-value constraint) still matches a face-down.
func TestFaceDown_PlainCreatureFilterStillMatches(t *testing.T) {
	fd := &Permanent{
		Card:       &Card{Name: "Hidden", Types: []string{"creature"}, CMC: 6, FaceDown: true},
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	if !matchesPermanent(gameast.Filter{Base: "creature", Targeted: true}, fd) {
		t.Fatal("face-down creature should match a plain 'target creature' filter")
	}
}
