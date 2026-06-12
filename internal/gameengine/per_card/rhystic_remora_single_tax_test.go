package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Round-4 legality sweep regression (sibling of
// TestEsperSentinel_TaxChargedExactlyOnce): Rhystic Study and Mystic
// Remora each had TWO implementations charging the caster on the same
// cast — the per_card handler plus a Wave-1b inline observer in
// cast_counts.go. Rhystic double-taxed visibly (84 over-pay hits across
// the round-4 seeds: per_card charge uncredited + inline charge
// credited); Remora double-taxed INVISIBLY (both sites NoteManaSpend-
// credited in #1024, so the validator read clean while the caster paid
// {8}). The inline arms are deleted; the per_card handlers are the
// sole implementations and both are credited.

func TestRhysticStudy_TaxChargedExactlyOnce(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seed = 555
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	addPerm(gs, 1, "Rhystic Study", "enchantment")
	addLibrary(gs, 1, "R1", "R2")

	spell := &gameengine.Card{
		Name: "Taxed Trinket", Owner: 0,
		Types: []string{"artifact", "cost:2"}, CMC: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 3 // spell {2} + single Rhystic tax {1}
	gameengine.EnsureTypedPool(gs.Seats[0])

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	taxes := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "pay_mana" && ev.Source == "Rhystic Study" {
			taxes++
			if ev.Amount != 1 {
				t.Errorf("Rhystic tax amount = %d, want 1", ev.Amount)
			}
		}
	}
	if taxes != 1 {
		t.Fatalf("Rhystic tax charged %d times, want exactly 1 (double-tax regression)", taxes)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("pool after cast = %d, want 0 (spell 2 + single tax 1)", gs.Seats[0].ManaPool)
	}
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("legitimate Rhystic tax flagged as cost violation: %v", v)
		}
	}
}

func TestMysticRemora_TaxChargedExactlyOnce(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seed = 555
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	addPerm(gs, 1, "Mystic Remora", "enchantment")
	addLibrary(gs, 1, "R1", "R2")

	// Noncreature spell; pool = spell {2} + ONE Remora tax {4}. The old
	// double-charge needed {2}+{8}=10 — with exactly 6 the second charge
	// would have been skipped-by-poverty, so also assert the event count.
	spell := &gameengine.Card{
		Name: "Taxed Scroll", Owner: 0,
		Types: []string{"sorcery", "cost:2"}, CMC: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 6
	gameengine.EnsureTypedPool(gs.Seats[0])

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}

	taxes := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "pay_mana" && ev.Source == "Mystic Remora" {
			taxes++
			if ev.Amount != 4 {
				t.Errorf("Remora tax amount = %d, want 4", ev.Amount)
			}
		}
	}
	if taxes != 1 {
		t.Fatalf("Remora tax charged %d times, want exactly 1 (invisible double-tax regression)", taxes)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("pool after cast = %d, want 0 (spell 2 + single tax 4)", gs.Seats[0].ManaPool)
	}
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("legitimate Remora tax flagged as cost violation: %v", v)
		}
	}
}
