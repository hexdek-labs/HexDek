package gameengine

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestAddManaBailoutCorpusCoverage_r109 drives the REAL resolver over the
// REAL arg texts of every corpus card that lands in the variable-quantity
// bailout, rather than reasoning about which shapes are covered.
//
// testdata_r109_bailout_cards.json is a verbatim dump of the add_mana_effect
// args[0] of all 25 such entries in ~/hexdek/data/rules/ast_dataset.jsonl
// on .206 (89 add_mana_effect nodes total, 25 bailing).
//
// Reports only — this is a coverage ledger, not a contract. Its job is to
// make the next round's starting point a measurement instead of a memory.
func TestAddManaBailoutCorpusCoverage_r109(t *testing.T) {
	raw, err := os.ReadFile("testdata_r109_bailout_cards.json")
	if err != nil {
		t.Skipf("corpus dump absent: %v", err)
	}
	var cards []struct{ Name, Text string }
	if err := json.Unmarshal(raw, &cards); err != nil {
		t.Fatalf("dump unparseable: %v", err)
	}
	if len(cards) == 0 {
		t.Fatal("dump is empty — the ledger would read 0/0 and mean nothing")
	}

	covered := 0
	for _, c := range cards {
		gs := newFixtureGame(t)
		// Power 3 and three creatures controlled, so that a power- or
		// creature-count-based resolver has a non-trivial basis and a
		// correct result can't be confused with the fallback's 1.
		p := addCreature(gs, 0, c.Name, 3, 3)
		addCreature(gs, 0, c.Name+" friend a", 1, 1)
		addCreature(gs, 0, c.Name+" friend b", 1, 1)
		p.Card.AST = &gameast.CardAST{Name: c.Name, Abilities: []gameast.Ability{
			&gameast.Activated{Cost: gameast.Cost{},
				Effect: &gameast.ModificationEffect{ModKind: "add_mana_effect", Args: []any{c.Text}}},
		}}
		gs.Active = 0
		gs.Seats[0].ManaPool = 0
		gs.Seats[0].Mana = &ColoredManaPool{}

		if err := ActivateAbility(gs, 0, p, 0, nil); err != nil {
			t.Logf("%-38s ACTIVATE ERR %v", c.Name, err)
			continue
		}
		s := gs.Seats[0]
		EnsureTypedPool(s)
		// Covered = produced typed colored mana rather than the legacy
		// one-generic Any.
		fixed := s.Mana.Any == 0 && s.Mana.Total() > 0
		mark := "fallback"
		if fixed {
			mark = "RESOLVED"
			covered++
		}
		t.Logf("%-8s %-34s %s", mark, c.Name, manaSummary(s))
	}
	t.Logf("r109 coverage: %d of %d bailout cards now produce typed mana", covered, len(cards))
}
