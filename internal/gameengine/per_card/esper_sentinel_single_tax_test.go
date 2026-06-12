package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestEsperSentinel_TaxChargedExactlyOnce pins the r62 double-tax fix
// (judge legality sweep round 3, seed 777 game 663: Mind Slash announced
// 1, pool delta 3 — the caster paid the Sentinel tax TWICE, once from
// the per_card handler and once from a duplicate inline observer in
// cast_counts.go that also computed X wrongly, as the Sentinel
// controller's creature COUNT instead of Sentinel's POWER). The inline
// arm is deleted; this per_card handler is the sole, oracle-correct
// implementation, and its payment is NoteManaSpend-credited so the
// legality validator doesn't read the tax as the spell over-paying.
func TestEsperSentinel_TaxChargedExactlyOnce(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seed = 99
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	// Seat 1 controls Esper Sentinel (1/1 — X = power = 1). Give seat 1
	// extra creatures so the OLD wrong formula (X = creature count = 3)
	// would visibly diverge from the correct X = 1.
	sentinel := addPerm(gs, 1, "Esper Sentinel", "artifact", "creature", "human", "soldier")
	sentinel.Card.BasePower, sentinel.Card.BaseToughness = 1, 1
	bystander1 := addPerm(gs, 1, "Bystander A", "creature")
	bystander1.Card.BasePower, bystander1.Card.BaseToughness = 2, 2
	bystander2 := addPerm(gs, 1, "Bystander B", "creature")
	bystander2.Card.BasePower, bystander2.Card.BaseToughness = 2, 2
	addLibrary(gs, 1, "X1", "X2", "X3")

	// Seat 0 casts a 2-mana noncreature spell with exactly 2 + 1 mana:
	// spell {2} + the single correct Sentinel tax {1}.
	spell := &gameengine.Card{
		Name: "Tithed Trinket", Owner: 0,
		Types: []string{"artifact", "cost:2"}, CMC: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 3
	gameengine.EnsureTypedPool(gs.Seats[0])

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	// Exactly one Sentinel tax payment, amount 1 (power, not creature count).
	taxEvents := 0
	for _, ev := range gs.EventLog {
		if ev.Kind != "pay_mana" || ev.Source != "Esper Sentinel" {
			continue
		}
		taxEvents++
		if ev.Amount != 1 {
			t.Errorf("Sentinel tax amount = %d, want 1 (X = Sentinel's power, not controller's creature count)", ev.Amount)
		}
	}
	if taxEvents != 1 {
		t.Fatalf("Sentinel tax charged %d times, want exactly 1 (double-tax regression)", taxEvents)
	}

	// Pool: 3 - spell 2 - tax 1 = 0.
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("pool after cast = %d, want 0 (spell 2 + single tax 1)", gs.Seats[0].ManaPool)
	}

	// The legitimate tax must not read as the spell over-paying.
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("Sentinel tax flagged as cost violation: %v", v)
		}
	}
}
