package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r110 — (17c). Vivi Ornitier's OTHER two clauses, which r108 left as an
// explicitly-labelled suspicion rather than a measurement.
//
// Both AST nodes below are transcribed from the REAL corpus entry on .206
// (`ast_dataset.jsonl`, `Vivi Ornitier`, `fully_parsed: true`), not invented:
//
//	Activated  cost {0}  effect Modification add_mana_effect
//	           timing_restriction: null            <-- note
//	Static     Modification timing_restriction, args ["activate only during"]
//	           raw "activate only during your turn and only once each turn"
//	Triggered  event cast_filtered, actor null, target_filter null
//	           raw "whenever you cast a noncreature spell, …"
//
// The card has no per_card handler (r98 removed the one that implemented a
// different card), so the raw-fallback dispatch is the live path.

func r110Game(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(110)), nil)
}

// viviTriggerAST is Vivi's real cast_filtered node: nil Actor, nil
// target_filter, the noncreature filter surviving only in Raw.
func viviTriggerAST() *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "cast_filtered"},
		Effect: &gameast.Sequence{Items: []gameast.Effect{
			&gameast.CounterMod{
				Op:          "put",
				Count:       gameast.NumberOrRef{IsInt: true, Int: 1},
				CounterKind: "+1/+1",
				Target:      gameast.Filter{Base: "thing", Quantifier: "one"},
			},
			&gameast.Damage{
				Amount: gameast.NumberOrRef{IsInt: true, Int: 1},
				Target: gameast.Filter{Base: "opponent", Quantifier: "each"},
			},
		}},
		Raw: "whenever you cast a noncreature spell, put a +1/+1 counter on ~ and it deals 1 damage to each opponent",
	}
}

// TestViviCastFilter_r110 answers r108's suspicion that the cast_filtered
// trigger fires on EVERY spell because its target_filter is null.
//
// It does not. fireObserverCastTriggers falls through to
// observerCastMatchesByRaw when Actor is nil, and that matcher parses
// "a noncreature spell," out of the Raw text. The suspicion is UNFOUNDED
// and is retracted here.
func TestViviCastFilter_r110(t *testing.T) {
	mk := func() (*GameState, *Permanent) {
		gs := r110Game(t)
		card := &Card{Name: "Vivi Ornitier", Owner: 0,
			Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
		card.AST = &gameast.CardAST{Name: "Vivi Ornitier",
			Abilities: []gameast.Ability{viviTriggerAST()}}
		p := &Permanent{Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{}}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		return gs, p
	}

	cases := []struct {
		label     string
		spell     *Card
		caster    int
		wantFires bool
	}{
		// POSITIVE CONTROL. If this one doesn't fire the probe is dead and
		// every "did not fire" below means nothing.
		{"CONTROL you cast an instant (noncreature)", &Card{Name: "r110 Bolt", Types: []string{"instant"}}, 0, true},
		{"you cast a sorcery (noncreature)", &Card{Name: "r110 Rite", Types: []string{"sorcery"}}, 0, true},
		{"you cast a CREATURE spell", &Card{Name: "r110 Bear", Types: []string{"creature"}}, 0, false},
		{"OPPONENT casts an instant", &Card{Name: "r110 Bolt", Types: []string{"instant"}}, 1, false},
	}

	for _, c := range cases {
		gs, vivi := mk()
		oppLife := gs.Seats[1].Life
		fireCastTriggers(gs, c.caster, c.spell)
		counters := vivi.Counters["+1/+1"]
		dmg := oppLife - gs.Seats[1].Life
		fired := counters > 0 || dmg > 0
		t.Logf("%-42s counters=%d  opp_life_delta=%d  fired=%v (want %v)",
			c.label, counters, dmg, fired, c.wantFires)
		if fired != c.wantFires {
			t.Errorf("%s: fired=%v, want %v (counters=%d dmg=%d)",
				c.label, fired, c.wantFires, counters, dmg)
		}
	}
}

// TestViviTimingRestriction_r110 measures whether "Activate only during your
// turn and only once each turn" is enforced anywhere.
//
// Characterization: asserts only the positive control (that the ability is
// activatable at all), and LOGS the restriction outcomes, so it records
// today's behaviour without freezing it. See the write-up for the verdict.
func TestViviTimingRestriction_r110(t *testing.T) {
	mk := func() (*GameState, *Permanent) {
		gs := newFixtureGame(t)
		p := addCreature(gs, 0, "Vivi Ornitier", 3, 3)
		p.Card.AST = &gameast.CardAST{
			Name: "Vivi Ornitier",
			Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost: gameast.Cost{},
					Effect: &gameast.ModificationEffect{
						ModKind: "add_mana_effect",
						Args:    []any{"add x mana in any combination of {u} and/or {r}, where x is ~'s power"},
					},
					// The real AST leaves this NULL — the restriction is a
					// separate Static node, transcribed below.
					TimingRestriction: "",
				},
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "timing_restriction",
						Args:    []any{"activate only during"},
					},
					Raw: "activate only during your turn and only once each turn",
				},
			},
		}
		gs.Seats[0].ManaPool = 0
		gs.Seats[0].Mana = &ColoredManaPool{}
		return gs, p
	}

	// POSITIVE CONTROL: on your own turn, the first activation must work.
	gs, p := mk()
	gs.Active = 0
	if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
		t.Fatalf("control: first activation on own turn failed: %v — probe is dead", err)
	}
	EnsureTypedPool(gs.Seats[0])
	first := gs.Seats[0].Mana.Total()
	t.Logf("CONTROL  own turn, 1st activation    ok, mana=%s", manaSummary(gs.Seats[0]))
	if first == 0 {
		t.Fatal("control: first activation produced no mana — probe is dead")
	}

	// ONCE EACH TURN: a second activation on the same turn.
	err2 := ActivateAbility(gs, 0, p, 0, nil)
	EnsureTypedPool(gs.Seats[0])
	t.Logf("PROBE    own turn, 2nd activation    err=%v  mana now=%s  (printed: \"only once each turn\")",
		err2, manaSummary(gs.Seats[0]))

	// YOUR TURN ONLY: activate while the opponent is the active player.
	gs2, p2 := mk()
	gs2.Active = 1
	err3 := ActivateAbility(gs2, 0, p2, 0, nil)
	EnsureTypedPool(gs2.Seats[0])
	t.Logf("PROBE    OPPONENT's turn, activation err=%v  mana=%s  (printed: \"only during your turn\")",
		err3, manaSummary(gs2.Seats[0]))

	switch {
	case err2 == nil && err3 == nil:
		t.Log("VERDICT: NEITHER half of the timing restriction is enforced.")
	case err2 == nil:
		t.Log("VERDICT: once-per-turn is NOT enforced; your-turn-only IS.")
	case err3 == nil:
		t.Log("VERDICT: your-turn-only is NOT enforced; once-per-turn IS.")
	default:
		t.Log("VERDICT: both halves enforced.")
	}
}
