package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Judge legality sweep round 5: the payManaFromPool "defensive cost
// gate" had two composed bugs.
//
//  1. Double-pay: ActivateAbility already charges Cost.Mana when the
//     AST Activated node carries one; four handlers (Mayael, Commander
//     Mustard, Bristly Bill, Ezrim) then defensively paid the same
//     amount AGAIN — and at exact budgets the second charge failed,
//     silently no-opping an ability the player had already paid for.
//  2. Hidden desync: payManaFromPool deducted the scalar ManaPool
//     without SyncManaAfterSpend, leaving the typed pool rich; the NEXT
//     payment's sync flushed the drift, and the validator flagged that
//     innocent action (the Mageta the Lion announced-4/measured-8 hit:
//     the extra 4 was four accumulated unsynced Rhystic-tax payments —
//     see TestPerCardTaxPayment_KeepsPoolsInSync; the same hole existed
//     in payManaFromPool itself and 20 other per_card sites).

// mayaelASTCard builds Mayael with her real ability shape: one
// Activated node costing {3}{R}{G}{W} (CMC 6) + tap.
func mayaelASTCard() *gameengine.Card {
	sym := func(g int, c ...string) gameast.ManaSymbol {
		return gameast.ManaSymbol{Generic: g, Color: c}
	}
	return &gameengine.Card{
		Name: "Mayael the Anima", Owner: 0,
		Types: []string{"legendary", "creature"},
		AST: &gameast.CardAST{Name: "Mayael the Anima", Abilities: []gameast.Ability{
			&gameast.Activated{
				Cost: gameast.Cost{
					Mana: &gameast.ManaCost{Symbols: []gameast.ManaSymbol{
						sym(3), sym(0, "R"), sym(0, "G"), sym(0, "W"),
					}},
					Tap: true,
				},
				Raw: "{3}{R}{G}{W}, {T}: look at the top five cards...",
			},
		}},
	}
}

// TestMayael_ExactBudget_SingleChargeAndEffectFires drives the REAL
// ActivateAbility with a pool of exactly 6. Pre-fix: dispatcher paid 6,
// the handler's defensive gate then failed on the empty pool and the
// ability no-opped (insufficient_mana emitFail) despite full payment.
func TestMayael_ExactBudget_SingleChargeAndEffectFires(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	gs.Step = "precombat_main"
	gs.Seed = 555
	gs.Legality = gameengine.NewLegalityValidator(gs.Seed)

	mayael := &gameengine.Permanent{
		Card: mayaelASTCard(), Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, mayael)
	// Top-5 includes a power-5+ creature for the effect to put in.
	big := &gameengine.Card{Name: "Big Beast", Owner: 0, Types: []string{"creature"}, BasePower: 6, BaseToughness: 6, CMC: 7}
	gs.Seats[0].Library = append(gs.Seats[0].Library, big)
	addLibrary(gs, 0, "F1", "F2", "F3", "F4")

	gs.Seats[0].ManaPool = 6 // exact budget
	gameengine.EnsureTypedPool(gs.Seats[0])

	if err := gameengine.ActivateAbility(gs, 0, mayael, 0, nil); err != nil {
		t.Fatalf("ActivateAbility failed: %v", err)
	}

	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("pool after activation = %d, want 0 (single charge of 6)", gs.Seats[0].ManaPool)
	}
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_fail" || (ev.Details != nil && ev.Details["reason"] == "insufficient_mana") {
			if ev.Source == "Mayael the Anima" {
				t.Errorf("defensive gate rejected an already-paid activation: %v", ev.Details)
			}
		}
	}
	// The effect must actually have run: the big creature left the library
	// (Mayael puts it onto the battlefield).
	for _, c := range gs.Seats[0].Library {
		if c == big {
			t.Error("Mayael effect did not run — Big Beast still in library (silent no-op regression)")
		}
	}
	for _, v := range gs.Legality.Violations {
		if v.Rule == "601.2f-h" {
			t.Errorf("single-charged activation flagged: %v", v)
		}
	}
}

// TestPayManaFromPool_SyncsTypedPool pins bug 2: a defensive payment
// must keep the typed pool in lockstep with the scalar so the drift
// can't be blamed on the next payer.
func TestPayManaFromPool_SyncsTypedPool(t *testing.T) {
	gs := newGame(t, 2)
	seat := gs.Seats[0]
	seat.ManaPool = 5
	gameengine.EnsureTypedPool(seat)

	if !payManaFromPool(seat, 3) {
		t.Fatal("payment should succeed with pool 5")
	}
	if seat.ManaPool != 2 {
		t.Errorf("scalar pool = %d, want 2", seat.ManaPool)
	}
	if got := seat.Mana.Total(); got != 2 {
		t.Errorf("typed pool total = %d, want 2 (desync: next payer takes the blame)", got)
	}
}

// TestPayDefensiveManaCost_PaysWhenNoASTCost pins the per_card-only
// shape the defensive gate exists for (Jolly Balloon Man: no AST mana
// cost, the handler's payment is the only one): it must still pay.
func TestPayDefensiveManaCost_PaysWhenNoASTCost(t *testing.T) {
	gs := newGame(t, 2)
	perm := addPerm(gs, 0, "Gateless Snowflake", "creature") // no AST
	seat := gs.Seats[0]
	seat.ManaPool = 2
	gameengine.EnsureTypedPool(seat)

	if !payDefensiveManaCost(perm, seat, 0, 1) {
		t.Fatal("defensive payment should succeed when no AST cost exists")
	}
	if seat.ManaPool != 1 {
		t.Errorf("pool = %d, want 1 (defensive gate must pay for per_card-only abilities)", seat.ManaPool)
	}

	// And with an AST mana cost present, it must NOT pay (dispatcher did).
	astPerm := &gameengine.Permanent{
		Card: mayaelASTCard(), Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	if !payDefensiveManaCost(astPerm, seat, 0, 6) {
		t.Fatal("should report success without paying when dispatcher charged")
	}
	if seat.ManaPool != 1 {
		t.Errorf("pool = %d, want 1 (no second charge when AST cost exists)", seat.ManaPool)
	}
}

// TestPerCardTaxPayment_KeepsPoolsInSync pins the drift class that the
// round-4 Rhystic swap reintroduced: the per_card tax handler decremented
// the scalar ManaPool without SyncManaAfterSpend, leaving the typed pool
// +1 rich per tax. The drift then surfaced on the NEXT payment — the
// seed-555 game-691 "Mageta announced 4 / measured 8" hit was four
// accumulated Rhystic-tax drifts flushed by Mageta's payment, with the
// innocent activation taking the validator blame. All 21 unsynced
// per_card scalar decrements now sync; this pins the proven offender.
func TestPerCardTaxPayment_KeepsPoolsInSync(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	gs.Phase = "main"
	addPerm(gs, 1, "Rhystic Study", "enchantment")
	addLibrary(gs, 1, "R1")

	spell := &gameengine.Card{
		Name: "Synced Trinket", Owner: 0,
		Types: []string{"artifact", "cost:2"}, CMC: 2,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spell)
	gs.Seats[0].ManaPool = 3
	gameengine.EnsureTypedPool(gs.Seats[0])

	if err := gameengine.CastSpell(gs, 0, spell, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("scalar pool = %d, want 0", gs.Seats[0].ManaPool)
	}
	if got := gs.Seats[0].Mana.Total(); got != gs.Seats[0].ManaPool {
		t.Errorf("typed pool (%d) != scalar pool (%d) after tax — drift will be blamed on the next payer",
			got, gs.Seats[0].ManaPool)
	}
}
