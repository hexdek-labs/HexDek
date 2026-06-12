package gameengine

// Regression tests for legality-sweep round-4 residual #1: the
// restricted-strand under-pay class (seed 2026 game 293, seed 31337
// game 12 — recurring Ashnod, Flesh Mechanist activations plus creature
// casts with a Powerstone in the pool).
//
// SyncManaAfterSpend drained Any/C/WUBRG after a direct `ManaPool -= N`
// spend but never touched the Restricted strands, so when the scalar
// spend dipped past the unrestricted floor the typed pool released one
// less unit than was paid. The stranded unit then (a) read as an
// under-payment to the §601.2f-h pool-delta check, and (b) was silently
// refunded to the scalar pool by the next AddMana's `ManaPool = Total()`
// mirror — which is why game 293 flagged the same seat at turns 28, 32,
// 44 and 48 with `after=1` every time: one immortal Powerstone unit.
//
// The fix mirrors SpendMana: the sync drains Restricted as last resort
// (unrestricted strands first), then compacts zeroed entries.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// restrictedTotal sums the restricted strands of a pool.
func restrictedTotal(p *ColoredManaPool) int {
	t := 0
	for _, r := range p.Restricted {
		t += r.Amount
	}
	return t
}

// A scalar spend that dips past the unrestricted floor must drain the
// restricted strand so the typed pool tracks the scalar exactly.
func TestSyncManaAfterSpend_DrainsRestrictedAsLastResort(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	AddMana(nil, seat, "C", 3, "Test Rock")
	AddRestrictedMana(nil, seat, 2, "C", "noncreature_or_artifact_activation", "Powerstone Token")
	if seat.ManaPool != 5 {
		t.Fatalf("setup: want pool 5, got %d", seat.ManaPool)
	}

	// Spend 4 the legacy way (the activation.go / stack.go pattern).
	seat.ManaPool -= 4
	SyncManaAfterSpend(seat)

	if got := seat.Mana.Total(); got != seat.ManaPool {
		t.Errorf("typed pool diverged from scalar: typed=%d scalar=%d", got, seat.ManaPool)
	}
	if seat.Mana.C != 0 {
		t.Errorf("unrestricted C should drain first, got %d", seat.Mana.C)
	}
	if got := restrictedTotal(seat.Mana); got != 1 {
		t.Errorf("restricted strand should hold exactly 1, got %d", got)
	}
}

// When unrestricted mana covers the whole spend, restricted strands
// stay untouched (restricted-last ordering, mirroring SpendMana).
func TestSyncManaAfterSpend_RestrictedUntouchedWhenUnrestrictedCovers(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	AddMana(nil, seat, "C", 3, "Test Rock")
	AddRestrictedMana(nil, seat, 2, "C", "noncreature_or_artifact_activation", "Powerstone Token")

	seat.ManaPool -= 3
	SyncManaAfterSpend(seat)

	if got := seat.Mana.Total(); got != seat.ManaPool {
		t.Errorf("typed pool diverged from scalar: typed=%d scalar=%d", got, seat.ManaPool)
	}
	if got := restrictedTotal(seat.Mana); got != 2 {
		t.Errorf("restricted strand should be untouched at 2, got %d", got)
	}
}

// A full drain empties every strand and compacts the zeroed restricted
// entries away.
func TestSyncManaAfterSpend_FullDrainCompactsRestricted(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	AddMana(nil, seat, "C", 3, "Test Rock")
	AddRestrictedMana(nil, seat, 1, "C", "noncreature_or_artifact_activation", "Powerstone Token")
	AddRestrictedMana(nil, seat, 1, "", "creature_spell_only", "Cavern of Souls")

	seat.ManaPool -= 5
	SyncManaAfterSpend(seat)

	if got := seat.Mana.Total(); got != 0 {
		t.Errorf("want empty typed pool, got %d", got)
	}
	if got := len(seat.Mana.Restricted); got != 0 {
		t.Errorf("zeroed restricted entries should compact away, got %d entries", got)
	}
}

// The stranded unit must not resurrect: after a past-the-floor spend, a
// later AddMana's `ManaPool = Total()` mirror used to refund the
// restricted unit to the scalar pool. Post-fix the pools agree, so the
// add credits exactly its own amount.
func TestSyncManaAfterSpend_NoRefundOnNextAddMana(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	AddMana(nil, seat, "C", 3, "Test Rock")
	AddRestrictedMana(nil, seat, 2, "C", "noncreature_or_artifact_activation", "Powerstone Token")

	seat.ManaPool -= 5 // spend everything, including both restricted units
	SyncManaAfterSpend(seat)

	AddMana(nil, seat, "G", 1, "Forest")
	if seat.ManaPool != 1 {
		t.Errorf("adding 1 to an empty pool must yield 1 (refund leak), got %d", seat.ManaPool)
	}
}

// End-to-end mirror of seed 2026 game 293 turn 44: an Ashnod-shaped {5}
// activation paid from a pool of 4 unrestricted + 1 restricted
// (Powerstone) unit. The §601.2f-h pool-delta check must read exactly 5
// spent and the pool must end empty.
func TestActivationPartlyPaidWithRestrictedMana_PoolDeltaMatchesAnnounced(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Phase = "main"
	gs.Active = 0
	gs.RetainEvents = true
	gs.Legality = NewLegalityValidator(2932027)

	dmgEff := &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}}
	perm := makeCreaturePerm(gs, 0, "Ashnod Stand-In", dmgEff, false)
	perm.Card.AST.Abilities[0].(*gameast.Activated).Cost.Mana = &gameast.ManaCost{
		Symbols: []gameast.ManaSymbol{{Generic: 5}},
	}

	seat := gs.Seats[0]
	AddMana(gs, seat, "C", 4, "Test Rock")
	AddRestrictedMana(gs, seat, 1, "C", "noncreature_or_artifact_activation", "Powerstone Token")

	if err := ActivateAbility(gs, 0, perm, 0, nil); err != nil {
		t.Fatalf("activation failed: %v", err)
	}

	if seat.ManaPool != 0 {
		t.Errorf("scalar pool should be empty after paying {5}, got %d", seat.ManaPool)
	}
	if got := seat.Mana.Total(); got != 0 {
		t.Errorf("typed pool should be empty after paying {5}, got %d (restricted strand stranded)", got)
	}
	for _, v := range gs.Legality.Violations {
		t.Errorf("unexpected legality violation: %s %s", v.Rule, v.Detail)
	}
}
