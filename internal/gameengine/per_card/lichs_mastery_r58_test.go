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
