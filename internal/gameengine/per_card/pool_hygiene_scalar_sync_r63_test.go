package per_card

// r63 — pool-hygiene regression for the "Knights" memorabilia 601.2f-h
// residual (seed 42 game 482: cast announced 0, pool delta 1).
//
// Mechanism: three per_card payment sites decremented the legacy scalar
// (`ManaPool--`) WITHOUT SyncManaAfterSpend, leaving the typed pool one
// unit higher than the scalar. The stray typed unit is invisible until
// the seat's next cast: CastSpell's `ManaPool -= cost` +
// SyncManaAfterSpend drains the leftover INSIDE that cast's legality
// window, so a 0-cost card (game 482's "Knights", a memorabilia insert
// that builds as a bare-bones CMC-0 card) reads "announced 0, spent 1".
//
// Sites fixed: rhystic_study.go (tax), card_cycled_consumers_r60.go
// payOneMana (Lightning Rift / Faith of the Devoted / Drake Haven),
// urabrask_the_great_work.go (transform {R} — no AST ability exists for
// the DFC, so the handler payment is the only charge).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func assertPoolsInSync(t *testing.T, gs *gameengine.GameState, seat int, label string) {
	t.Helper()
	s := gs.Seats[seat]
	typed := 0
	if s.Mana != nil {
		typed = s.Mana.Total()
	}
	if typed != s.ManaPool {
		t.Errorf("%s: typed pool (%d) diverged from scalar (%d) — stray mana will poison the next cast's 601.2f-h window", label, typed, s.ManaPool)
	}
}

// The exact game-482 shape end-to-end: a Rhystic-taxed cast leaves the
// pools in sync, so a follow-up 0-cost bare-bones card (the memorabilia
// "Knights" build) casts with a clean pool delta of 0.
func TestRhysticTaxThenZeroCostCast_NoStrayManaNo601Flag(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seed = 4820043
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	addPerm(gs, 1, "Rhystic Study", "enchantment")
	addLibrary(gs, 1, "R1", "R2")

	spell := &gameengine.Card{
		Name: "Taxed Trinket", Owner: 0,
		Types: []string{"artifact", "cost:2"}, CMC: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gameengine.AddMana(gs, gs.Seats[0], "C", 4, "Test Rock") // {2} spell + {1} tax + {1} spare

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("taxed cast failed: %v", err)
	}
	assertPoolsInSync(t, gs, 0, "after Rhystic-taxed cast")
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("want 1 spare after {2}+{1} from 4, got %d", gs.Seats[0].ManaPool)
	}

	// The memorabilia-insert shape: bare-bones, CMC 0, no AST, typeless.
	// Note the second cast is ALSO Rhystic-taxed (noncreature, opponent
	// of the Study's controller) — the spare {1} legitimately pays that
	// second tax, so the pool ends at 0. What must NOT happen: a stray
	// typed unit (pre-fix the first tax left typed=2/scalar=1) or any
	// 601.2f-h flag on either cast.
	knights := &gameengine.Card{Name: "Knights", Owner: 0}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, knights)
	if err := gameengine.CastSpell(gs, 0, knights, nil); err != nil {
		t.Fatalf("zero-cost cast failed: %v", err)
	}
	assertPoolsInSync(t, gs, 0, "after zero-cost cast")
	taxes := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "pay_mana" && ev.Source == "Rhystic Study" {
			taxes++
		}
	}
	if taxes != 2 {
		t.Errorf("want 2 Rhystic taxes (one per cast), got %d", taxes)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("want pool 0 after {2}+{1} cast and two {1} taxes from 4, got %d", gs.Seats[0].ManaPool)
	}
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("stray-mana 601.2f-h flag on the innocent cast (game-482 shape): %s", v.Detail)
		}
	}
}

// payOneMana (cycling unless-pay consumers) must keep the typed pool in
// step with the scalar it debits.
func TestCyclingPayOneMana_KeepsPoolsInSync(t *testing.T) {
	gs := newGame(t, 2)
	gameengine.AddMana(gs, gs.Seats[0], "C", 2, "Test Rock")

	if !payOneMana(gs, 0) {
		t.Fatal("payOneMana should succeed with 2 in pool")
	}
	assertPoolsInSync(t, gs, 0, "after payOneMana")
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("want scalar 1 after paying 1 from 2, got %d", gs.Seats[0].ManaPool)
	}
}

// The actual game-482 mechanism (seed 42, "cast:Knights: announced 0,
// delta 1"): the caster DIES inside their own cast window. Seat 0 casts
// a 0-cost noncreature with 1 floating mana; the opponent's Ruric Thar
// punisher trigger deals 6 (lethal) inside fireCastTriggers; the SBA
// marks seat 0 lost and markSeatLost clears the pool per CR §106.4.
// The cleared float is NOT a payment — the cost check must not flag a
// seat that left the game mid-window.
func TestCasterDiesMidCast_EliminationPoolClearNot601Flagged(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Seed = 4820043
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	addPerm(gs, 1, "Ruric Thar, the Unbowed", "legendary", "creature")
	gs.Seats[0].Life = 5 // Ruric's 6 is lethal

	knights := &gameengine.Card{Name: "Knights", Owner: 0} // memorabilia shape: typeless, CMC 0, no AST
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, knights)
	gameengine.AddMana(gs, gs.Seats[0], "C", 1, "Test Rock") // the floating mana the elimination clears

	_ = gameengine.CastSpell(gs, 0, knights, nil)

	if !gs.Seats[0].Lost {
		t.Fatal("setup failed: Ruric Thar's punisher should have eliminated seat 0 mid-cast")
	}
	if gs.Seats[0].ManaPool != 0 || gs.Seats[0].Mana.Total() != 0 {
		t.Fatalf("eliminated seat must hold no mana (CR §106.4): scalar=%d typed=%d",
			gs.Seats[0].ManaPool, gs.Seats[0].Mana.Total())
	}
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("elimination pool-clear misread as payment: %s", v.Detail)
		}
	}
}

// Urabrask's {R} transform payment (per_card-only — the DFC has no AST
// abilities, so the handler is the sole charge site) must stay synced.
func TestUrabraskTransformPayment_KeepsPoolsInSync(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	ura := addPerm(gs, 0, "Urabrask", "creature")
	gs.Seats[0].Turn.SpellsCast = 3
	gameengine.AddMana(gs, gs.Seats[0], "R", 2, "Test Rock")

	urabraskActivate(gs, ura, 0, nil)

	assertPoolsInSync(t, gs, 0, "after Urabrask transform payment")
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("want scalar 1 after paying {R} from 2, got %d", gs.Seats[0].ManaPool)
	}
}
