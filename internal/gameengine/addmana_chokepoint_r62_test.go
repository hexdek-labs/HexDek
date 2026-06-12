package gameengine

// Regression tests for legality-validator r62 finding #4: the two
// mana-add sites that bypassed the canonical AddMana chokepoint —
// Birgi's cast-trigger observer (cast_counts.go) and resolveAddMana's
// inline mana-ability path (resolve.go) — must route through AddMana so
// every mana addition logs the canonical add_mana event, is visible to
// any future mana-replacement hook, and credits the legality validator's
// in-window observation exactly once (the per-site NoteManaAdd stopgaps
// are gone; a leftover one would double-credit and re-introduce the
// 601.2f-h over/under-pay misreads in the other direction).

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// countAddManaEvents returns how many add_mana events name `source`.
func countAddManaEvents(gs *GameState, source string) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "add_mana" && ev.Source == source {
			n++
		}
	}
	return n
}

// Birgi's cast-trigger mana must flow through AddMana: canonical event
// emitted, red bucket credited, and — with the validator riding along —
// the cast it fires inside of stays violation-free (single in-window
// credit, no double-count from a leftover stopgap).
func TestBirgiCastTriggerMana_RoutesThroughAddMana(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(11)), nil)
	gs.Seed = 11
	gs.Phase = "main"
	gs.Active = 0
	gs.RetainEvents = true
	gs.Legality = NewLegalityValidator(11)
	gs.Seats[0].Hat = &GreedyHatStub{}

	birgi := &Permanent{
		Card: &Card{
			Name: "Birgi, God of Storytelling", Owner: 0,
			Types: []string{"legendary", "creature"},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, birgi)

	gs.Seats[0].ManaPool = 2
	EnsureTypedPool(gs.Seats[0])
	spell := &Card{Name: "Cheap Trick", Owner: 0, Types: []string{"instant", "cost:2"}}
	gs.Seats[0].Hand = []*Card{spell}

	if err := CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	if n := countAddManaEvents(gs, "Birgi, God of Storytelling"); n != 1 {
		t.Errorf("expected exactly 1 canonical add_mana event from Birgi, got %d — chokepoint bypassed or double-fired", n)
	}
	// Paid 2 for the spell, Birgi added {R}: pool must read exactly 1,
	// and it must sit in the RED bucket (AddMana routed the color).
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("pool after cast = %d (want 1: paid 2, Birgi +1)", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana.R != 1 {
		t.Errorf("Birgi's mana should land in the R bucket, got R=%d Any=%d", gs.Seats[0].Mana.R, gs.Seats[0].Mana.Any)
	}
	// No cost misread in either direction: AddMana's single credit must
	// keep the in-window accounting exact.
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("cost check misread with Birgi mana in-window (double/missing credit): %v", v)
		}
	}
}

// resolveAddMana (inline mana abilities) must flow through AddMana:
// canonical event with the source permanent's name, Any-bucket credit
// (bucket-identical to the old SyncManaAfterAdd path), and a clean
// validator observation for the activation.
func TestResolveAddMana_RoutesThroughAddMana(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(11)), nil)
	gs.Seed = 11
	gs.Phase = "main"
	gs.Active = 0
	gs.RetainEvents = true
	gs.Legality = NewLegalityValidator(11)
	gs.Seats[0].Hat = &GreedyHatStub{}

	rock := &Permanent{
		Card: &Card{
			Name: "Test Mana Rock", Owner: 0, Types: []string{"artifact"},
			AST: &gameast.CardAST{Name: "Test Mana Rock", Abilities: []gameast.Ability{
				&gameast.Activated{
					Cost:   gameast.Cost{Tap: true},
					Effect: &gameast.AddMana{AnyColorCount: 2},
				},
			}},
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, rock)

	if err := ActivateAbility(gs, 0, rock, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	if n := countAddManaEvents(gs, "Test Mana Rock"); n != 1 {
		t.Errorf("expected exactly 1 canonical add_mana event from the rock, got %d", n)
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("pool = %d (want 2)", gs.Seats[0].ManaPool)
	}
	if gs.Seats[0].Mana.Any != 2 {
		t.Errorf("inline-ability mana should stay in the Any bucket (bucket parity with the old path), got Any=%d", gs.Seats[0].Mana.Any)
	}
	if n := len(gs.Legality.Violations); n != 0 {
		t.Errorf("mana-ability activation should be violation-free, got: %v", gs.Legality.Violations)
	}
}
