package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Live-grinder §614.1c firespot: "etb Moonlit Lamenter — enters-with-counters
// self-replacement not applied: want >=1 -1/-1 counter, got fewer".
//
// Moonlit Lamenter is a PLAIN unconditional "this creature enters with a -1/-1
// counter on it". The engine DOES place that counter (verified by
// TestMoonlit_PlainEntersWithMinusCounter below). The §614.1c check, however,
// asserted the counter still PERSISTS at the ETB observation — but CR §614.1d /
// §122.1g make the self-replacement about the act of ENTERING; once placed, a
// counter can be legitimately removed before the observation closes. The most
// common case is the §704.5q +1/+1 / -1/-1 annihilation SBA, which per_card ETB
// handlers run INLINE mid-cascade (so it can fire before ObserveETB). The check
// then false-flagged a rules-correct annihilation as a missing self-replacement.
//
// This is the §704.5q analogue of the #1075 §616 doubler/halver FP fix. The
// guard now skips when ApplyStaticETBCounters recorded a full-count placement
// (perm.Flags["_etb_placed:<kind>"] >= want), regardless of later removal.

func moonlitLamenterAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Moonlit Lamenter",
		Abilities: []gameast.Ability{
			&gameast.Static{
				Modification: &gameast.Modification{
					ModKind: "etb_with_counters",
					Args:    []interface{}{1, "-1/-1"},
				},
				Raw: "this creature enters with a -1/-1 counter on it",
			},
		},
	}
}

func moonlitPerm(gs *GameState, seat int, preCounters map[string]int) *Permanent {
	c := map[string]int{}
	for k, v := range preCounters {
		c[k] = v
	}
	p := &Permanent{
		Card: &Card{
			Name:          "Moonlit Lamenter",
			TypeLine:      "Creature — Treefolk Cleric",
			Types:         []string{"creature", "treefolk", "cleric"},
			BasePower:     2,
			BaseToughness: 5,
			AST:           moonlitLamenterAST(),
		},
		Controller: seat, Owner: seat,
		Counters:  c,
		Flags:     map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// The engine places the -1/-1 counter (the self-replacement works); plain ETB
// produces no §614.1c violation.
func TestMoonlit_PlainEntersWithMinusCounter(t *testing.T) {
	gs := newTestGameState(2)
	gs.Legality = NewLegalityValidator(1)
	p := moonlitPerm(gs, 0, nil)

	ApplyStaticETBCounters(gs, p)
	if got := p.Counters["-1/-1"]; got != 1 {
		t.Fatalf("Moonlit should enter with 1 -1/-1 counter, got %d", got)
	}
	gs.Legality.ObserveETB(gs, p)
	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "614.1c" {
			t.Fatalf("plain Moonlit ETB false-flagged 614.1c: %s", lv.Detail)
		}
	}
}

// The firespot: the -1/-1 enters-with counter is placed, then a +1/+1 counter
// present on the permanent annihilates it via §704.5q before the ETB obs. The
// self-replacement WAS applied, so 614.1c must NOT fire.
func TestMoonlit_MinusCounterAnnihilated_No614FalsePositive(t *testing.T) {
	gs := newTestGameState(2)
	gs.Legality = NewLegalityValidator(1)
	// A +1/+1 already on the permanent (an inline ETB-counter granter placed it
	// earlier in the cascade) — §704.5q will pair it off against the -1/-1.
	p := moonlitPerm(gs, 0, map[string]int{"+1/+1": 1})

	ApplyStaticETBCounters(gs, p) // places the -1/-1 (records _etb_placed)
	if p.Counters["+1/+1"] != 1 || p.Counters["-1/-1"] != 1 {
		t.Fatalf("setup: want +1/+1=1 -1/-1=1, got +1/+1=%d -1/-1=%d", p.Counters["+1/+1"], p.Counters["-1/-1"])
	}
	StateBasedActions(gs) // §704.5q annihilation → both removed
	if p.Counters["+1/+1"] != 0 || p.Counters["-1/-1"] != 0 {
		t.Fatalf("§704.5q annihilation didn't fire: +1/+1=%d -1/-1=%d", p.Counters["+1/+1"], p.Counters["-1/-1"])
	}

	gs.Legality.ObserveETB(gs, p)
	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "614.1c" {
			t.Fatalf("614.1c false-positive on §704.5q-annihilated enters-with counter: %s", lv.Detail)
		}
	}
}

// Guard: the District-Mascot bug the check exists for still fires — an
// "enters with N counters" static whose counters were NEVER placed (no
// _etb_placed record, no replacement in play) is a real §614.1c miss.
func TestMoonlit_GenuinelyMissingCounterStillFlags(t *testing.T) {
	gs := newTestGameState(2)
	gs.Legality = NewLegalityValidator(1)
	// Build the perm but DO NOT call ApplyStaticETBCounters — the counter was
	// never applied and nothing modified the placement.
	p := moonlitPerm(gs, 0, nil)

	gs.Legality.ObserveETB(gs, p)
	found := false
	for _, lv := range gs.Legality.Violations {
		if lv.Rule == "614.1c" {
			found = true
		}
	}
	if !found {
		t.Fatal("614.1c must still fire when the enters-with counter was genuinely never applied")
	}
}
