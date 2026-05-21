package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R58 regression — Lich's Mastery "exile a permanent" no longer
// duplicates the exiled *Card across exile + battlefield.
//
// Loki r57 lead 1: Mire's Grasp (ptr 0xc009d8dc20, game-3744) appeared
// in both seat 0 exile AND seat 0 battlefield. Root cause: the
// lichsMasteryLifeLost handler called MoveCard(card, seat, "battlefield",
// "exile", ...). MoveCard's removeCardFromZone("battlefield") is a no-op
// (zone_move.go:239 — battlefield source removal is the caller's
// responsibility). The Permanent was never unwound from the battlefield,
// so the *Card ended up in both exile (from FireZoneChange) and as a
// live battlefield Permanent. Same anti-pattern family as the Krark r54
// bounce-back leak.
//
// Fix: use ExilePermanent (zone_change.go:163) which does the full
// §406.3 lifecycle including removePermanent + detachAll.

func TestLichsMastery_R58_ExilesPermanentWithoutDuplication(t *testing.T) {
	gs := newGame(t, 2)
	mastery := stampCreaturePT(addPerm(gs, 0, "Lich's Mastery", "enchantment"), 0, 0)
	target := stampCreaturePT(addPerm(gs, 0, "Mire's Grasp", "enchantment", "aura"), 0, 0)
	preBfLen := len(gs.Seats[0].Battlefield)
	preExileLen := len(gs.Seats[0].Exile)

	lichsMasteryLifeLost(gs, mastery, map[string]interface{}{
		"seat":   0,
		"amount": 1,
	})

	// The target *Card must be in exile exactly once.
	exileCount := 0
	for _, c := range gs.Seats[0].Exile {
		if c == target.Card {
			exileCount++
		}
	}
	if exileCount != 1 {
		t.Errorf("expected Mire's Grasp in exile exactly once, got %d", exileCount)
	}
	// The Permanent must be removed from battlefield.
	for _, p := range gs.Seats[0].Battlefield {
		if p == target {
			t.Errorf("CardIdentity leak: target Permanent still on battlefield after Lich's Mastery exile")
		}
	}
	if got := len(gs.Seats[0].Exile); got != preExileLen+1 {
		t.Errorf("expected exile to grow by 1; before=%d after=%d", preExileLen, got)
	}
	if got := len(gs.Seats[0].Battlefield); got != preBfLen-1 {
		t.Errorf("expected battlefield to shrink by 1; before=%d after=%d", preBfLen, got)
	}
}

func TestLichsMastery_R58_FallsBackToHandWhenBattlefieldEmpty(t *testing.T) {
	gs := newGame(t, 2)
	mastery := stampCreaturePT(addPerm(gs, 0, "Lich's Mastery", "enchantment"), 0, 0)
	// Remove Lich's Mastery from battlefield so the only battlefield permanent
	// path is empty (we want to exercise the hand fallback).
	gs.Seats[0].Battlefield = nil
	handCard := &gameengine.Card{Name: "Some Card", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, handCard)
	preHandLen := len(gs.Seats[0].Hand)
	preExileLen := len(gs.Seats[0].Exile)

	lichsMasteryLifeLost(gs, mastery, map[string]interface{}{
		"seat":   0,
		"amount": 1,
	})

	// Hand should have shrunk by 1; exile grown by 1.
	if got := len(gs.Seats[0].Hand); got != preHandLen-1 {
		t.Errorf("expected hand to shrink by 1; before=%d after=%d", preHandLen, got)
	}
	if got := len(gs.Seats[0].Exile); got != preExileLen+1 {
		t.Errorf("expected exile to grow by 1; before=%d after=%d", preExileLen, got)
	}
}

// r58 follow-up — Lich's Mastery must not exile itself. The "last
// permanent in the slice" picker auto-self-exiles when Lich is the
// only (or last-positioned) battlefield permanent, triggering its own
// "leaves the battlefield → you lose the game" SBA and defeating the
// engine's whole purpose. Pin that the handler skips the source and
// falls back to hand/graveyard.
func TestLichsMastery_R58_DoesNotExileSelfWhenLastOnBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	mastery := stampCreaturePT(addPerm(gs, 0, "Lich's Mastery", "enchantment"), 0, 0)
	// Lich's Mastery is the SOLE battlefield permanent. Provide a hand
	// fallback so the handler has something to exile that isn't Lich.
	handCard := &gameengine.Card{
		Name: "Fallback Card", Owner: 0, Types: []string{"instant"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, handCard)

	lichsMasteryLifeLost(gs, mastery, map[string]interface{}{
		"seat":   0,
		"amount": 1,
	})

	// Lich's Mastery must still be on the battlefield.
	stillOnBf := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == mastery {
			stillOnBf = true
		}
	}
	if !stillOnBf {
		t.Fatalf("REGRESSION: Lich's Mastery exiled itself — would trigger LTB lose-the-game SBA")
	}
	// And the hand card should have been exiled instead.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected hand emptied via fallback; got %d remaining", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("expected 1 card in exile (the hand fallback); got %d", len(gs.Seats[0].Exile))
	}
	for _, c := range gs.Seats[0].Exile {
		if c == mastery.Card {
			t.Errorf("REGRESSION: Lich's Mastery's *Card appended to exile (self-target leak)")
		}
	}
}

// r58 follow-up — pin amount>1 with mixed seat 0 battlefield: 1 Lich +
// N other permanents. The handler must exile N-1 others (NOT Lich)
// without ever entering an infinite loop or self-target.
func TestLichsMastery_R58_MultiAmountSkipsSelfAndExilesOthers(t *testing.T) {
	gs := newGame(t, 2)
	mastery := stampCreaturePT(addPerm(gs, 0, "Lich's Mastery", "enchantment"), 0, 0)
	addPerm(gs, 0, "Mire's Grasp", "enchantment", "aura")
	addPerm(gs, 0, "Mire's Grasp", "enchantment", "aura")
	addPerm(gs, 0, "Mire's Grasp", "enchantment", "aura")

	lichsMasteryLifeLost(gs, mastery, map[string]interface{}{
		"seat":   0,
		"amount": 3,
	})

	// All three Mires exiled, Lich stays.
	stillOnBf := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == mastery {
			stillOnBf = true
		}
	}
	if !stillOnBf {
		t.Fatalf("Lich's Mastery self-exiled (regression)")
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Errorf("expected only Lich's Mastery left on bf; got %d permanents", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Exile) != 3 {
		t.Errorf("expected 3 cards in exile; got %d", len(gs.Seats[0].Exile))
	}
}

func TestLichsMastery_R58_DoesNotFireForOpponentLifeLoss(t *testing.T) {
	gs := newGame(t, 2)
	mastery := stampCreaturePT(addPerm(gs, 0, "Lich's Mastery", "enchantment"), 0, 0)
	target := stampCreaturePT(addPerm(gs, 0, "Bystander", "creature"), 2, 2)
	preBfLen := len(gs.Seats[0].Battlefield)

	// Opponent (seat 1) lost life — Lich's Mastery's controller (seat 0)
	// shouldn't exile any of their own permanents.
	lichsMasteryLifeLost(gs, mastery, map[string]interface{}{
		"seat":   1,
		"amount": 3,
	})

	if got := len(gs.Seats[0].Battlefield); got != preBfLen {
		t.Errorf("Lich's Mastery should not fire on opponent's life loss; battlefield went %d → %d",
			preBfLen, got)
	}
	if got := len(gs.Seats[0].Exile); got != 0 {
		t.Errorf("expected no exile from opponent life loss; got %d", got)
	}
	_ = target
}
