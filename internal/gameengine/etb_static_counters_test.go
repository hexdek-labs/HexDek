package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Loki r60 final residual: seed 43 game 1003 turn 53 — District Mascot
// surfaced as a 0/0 creature with no +1/+1 counter and no mods, despite
// SBA 704.5f being correctly wired. Root cause: the AST classifier emits
// District Mascot's "this creature enters with a +1/+1 counter on it"
// as a Static ability with Modification.ModKind == "etb_with_counters",
// but the engine only ran the etb_with_counters handler in
// resolveModificationEffect (i.e. when the modification appears as a
// resolving spell EFFECT). Static abilities don't resolve through the
// stack, so District Mascot's counter was never applied — and a 0/0
// creature reachable by the loki invariant before the next SBA pass
// surfaced as a 704.5f-miss false positive.

// districtMascotAST returns a minimal AST mirroring the real District
// Mascot oracle: a single Static ability with the
// `etb_with_counters` modification (count=1, kind="+1/+1"). Other
// abilities are omitted — the test only exercises the ETB path.
func districtMascotAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "District Mascot",
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "etb_with_counters",
					Args:    []interface{}{1, "+1/+1"},
				},
				Raw: "this creature enters with a +1/+1 counter on it",
			},
		},
	}
}

func districtMascotCard() *Card {
	return &Card{
		Name:          "District Mascot",
		TypeLine:      "Creature — Dog Mount",
		Types:         []string{"creature"},
		BasePower:     0,
		BaseToughness: 0,
		AST:           districtMascotAST(),
	}
}

// TestApplyStaticETBCounters_DistrictMascotEntersWithCounter is the
// minimal regression: insert a District-Mascot-shape perm with the
// AST present, run the primitive, and assert one +1/+1 counter is
// applied.
func TestApplyStaticETBCounters_DistrictMascotEntersWithCounter(t *testing.T) {
	gs := newTestGameState(2)
	perm := &Permanent{
		Card:       districtMascotCard(),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	ApplyStaticETBCounters(gs, perm)

	if got := perm.Counters["+1/+1"]; got != 1 {
		t.Fatalf("District Mascot should enter with one +1/+1 counter, got %d", got)
	}
}

// TestApplyStaticETBCounters_SBA704_5fDoesNotKillAfterApply confirms
// the end-to-end intent: with the counter applied, the resulting
// creature is 1/1 and SBA 704.5f leaves it alone. (This was the loki
// invariant trip: the seed-43 game 1003 violation was a 704.5f miss
// because the counter never got applied at all.)
func TestApplyStaticETBCounters_SBA704_5fDoesNotKillAfterApply(t *testing.T) {
	gs := newTestGameState(2)
	perm := &Permanent{
		Card:       districtMascotCard(),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	ApplyStaticETBCounters(gs, perm)

	// Run SBA 704.5f directly.
	_ = sba704_5f(gs)

	// District Mascot should still be on the battlefield.
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("District Mascot should survive SBA 704.5f at 1/1; battlefield=%d",
			len(gs.Seats[0].Battlefield))
	}
}

// TestApplyStaticETBCounters_SBA704_5fKillsWithoutPrimitive is the
// counterfactual: without the counter, a 0/0 creature must die to
// SBA 704.5f. Pins the SBA path as correct — confirming the loki
// residual was the missing primitive, not a broken SBA rule.
func TestApplyStaticETBCounters_SBA704_5fKillsWithoutPrimitive(t *testing.T) {
	gs := newTestGameState(2)
	perm := &Permanent{
		Card:       districtMascotCard(),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	// Deliberately DO NOT call ApplyStaticETBCounters — simulate the
	// pre-fix state where District Mascot entered at 0/0 with no
	// counter.

	_ = sba704_5f(gs)

	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatalf("0/0 District Mascot without counter must die to SBA 704.5f; battlefield=%d",
			len(gs.Seats[0].Battlefield))
	}
}

// TestApplyStaticETBCounters_HonorsArgs1ForCustomCounterKind verifies
// the second arg picks the counter kind. Covers e.g. "enters with N
// loyalty counters" / "enters with N +1/+1 counters" parity — the
// resolveModificationEffect side already handles both, so the new
// Static-side primitive must match its arg shape.
func TestApplyStaticETBCounters_HonorsArgs1ForCustomCounterKind(t *testing.T) {
	gs := newTestGameState(2)
	perm := &Permanent{
		Card: &Card{
			Name:     "Synthetic",
			TypeLine: "Creature",
			Types:    []string{"creature"},
			AST: &gameast.CardAST{
				Name: "Synthetic",
				Abilities: []gameast.Ability{
					&gameast.Static{
						Modification: &gameast.Modification{
							ModKind: "etb_with_counters",
							Args:    []interface{}{3, "charge"},
						},
					},
				},
			},
		},
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	ApplyStaticETBCounters(gs, perm)

	if got := perm.Counters["charge"]; got != 3 {
		t.Fatalf("expected 3 charge counters, got %d", got)
	}
	if got := perm.Counters["+1/+1"]; got != 0 {
		t.Fatalf("expected no +1/+1 counters, got %d", got)
	}
}

// TestApplyStaticETBCounters_SkipsFaceDown — face-down permanents have
// no abilities per CR §708.4, including the etb_with_counters Static.
func TestApplyStaticETBCounters_SkipsFaceDown(t *testing.T) {
	gs := newTestGameState(2)
	perm := &Permanent{
		Card:       districtMascotCard(),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{"face_down": 1},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	ApplyStaticETBCounters(gs, perm)

	if got := perm.Counters["+1/+1"]; got != 0 {
		t.Fatalf("face-down District Mascot must not gain ETB counter, got %d", got)
	}
}

// TestFirePermanentETBTriggers_AppliesStaticEtbCounters confirms the
// blink/reanimate/token-mint entry path (FirePermanentETBTriggers)
// also runs the primitive — not just the stack-cast path. This is the
// reanimate/flicker family ETB integration test.
func TestFirePermanentETBTriggers_AppliesStaticEtbCounters(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	perm := &Permanent{
		Card:       districtMascotCard(),
		Controller: 0,
		Owner:      0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	FirePermanentETBTriggers(gs, perm)

	if got := perm.Counters["+1/+1"]; got != 1 {
		t.Fatalf("blink/reanimate path should also apply etb_with_counters; got %d", got)
	}
}
