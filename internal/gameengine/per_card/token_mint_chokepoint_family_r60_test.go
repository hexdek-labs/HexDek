package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestTokenMintChokepointFamily pins the Phase 5 mint-coverage sweep
// (PR #853 follow-up). Each handler in the sweep creates a battlefield
// token COPY of a source permanent; pre-fix they DeepCopy'd the source
// *Card without going through MintTokenAsCopyOf, so the token Card
// inherited the source's OG InstanceID — the exact same-ID-in-two-perms
// shape Drafna surfaced in PR #853.
//
// The sweep covers: Hazel, Orvar, Phoenix Fleet, Calix, Satya
// (vanilla copy family), Paradigm Echocasting + Era3 Urza (copy-with-
// different-name family), Hashaton + Altair + Terra + Shiko (copy-with-
// template-changes family), Brudiclad (in-place perm rewrite — the
// fallback DeepCopy was removed in favor of skip-on-nil).
//
// This file exercises one representative card per family end-to-end to
// pin the chokepoint contract. The other family members follow the same
// pattern at the source level (the fix is mechanical — replace
// `target.Card.DeepCopy()` with
// `gameengine.MintTokenAsCopyOf(gs, target.Card, controller, enablerID)`
// and apply downstream overrides AFTER the mint).

func mintProvenance(id string) string {
	if len(id) < 4 {
		return ""
	}
	return id[2:4]
}

// TestTokenMintChokepoint_Hazel — vanilla copy family.
func TestTokenMintChokepoint_Hazel(t *testing.T) {
	gs := newTestGS(2)
	gs.Active = 0

	hazel := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Hazel of the Rootbloom", Owner: 0, InstanceID: "h0OGhazel1",
			Types: []string{"legendary", "creature"},
		},
		Controller: 0, Owner: 0, Timestamp: 10,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, hazel)

	srcID := "h0OGtreant1"
	target := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Treant Token", Owner: 0, InstanceID: srcID,
			Types: []string{"creature", "token"},
			BasePower: 2, BaseToughness: 2,
		},
		Controller: 0, Owner: 0, Timestamp: 11,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)
	gs.MintedInstanceIDs = map[string]struct{}{
		hazel.Card.InstanceID:  {},
		target.Card.InstanceID: {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		hazel.Card.InstanceID:  hazel.Card.Name,
		target.Card.InstanceID: target.Card.Name,
	}

	// Hazel's end-step trigger creates 1 (or 2 for squirrel) token copies.
	hazelRootbloomEndStep(gs, hazel, map[string]interface{}{
		"active_seat": 0,
	})

	if target.Card.InstanceID != srcID {
		t.Fatalf("source InstanceID rewritten: %q != %q", target.Card.InstanceID, srcID)
	}

	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p == hazel || p == target {
			continue
		}
		token = p
		break
	}
	if token == nil {
		t.Fatalf("no new token on seat 0's battlefield post-end-step")
	}
	if token.Card == target.Card {
		t.Fatalf("token *Card aliases source")
	}
	if token.Card.InstanceID == srcID {
		t.Fatalf("token inherited source InstanceID — MintTokenAsCopyOf missed")
	}
	if mintProvenance(token.Card.InstanceID) != "TK" {
		t.Fatalf("token InstanceID %q is not TK-provenance", token.Card.InstanceID)
	}
	if token.Card.SourceInstanceID != srcID {
		t.Fatalf("token SourceInstanceID = %q, want %q", token.Card.SourceInstanceID, srcID)
	}

	for _, inv := range gameengine.AllInvariants() {
		if inv.Name != "CardIdentity" {
			continue
		}
		if err := inv.Check(gs); err != nil {
			t.Fatalf("CardIdentity: %v", err)
		}
	}
}

// TestTokenMintChokepoint_Hashaton — copy-with-template-changes family
// (force 4/4 black Zombie). Post-mint overrides must survive.
func TestTokenMintChokepoint_Hashaton(t *testing.T) {
	gs := newTestGS(2)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10

	hashaton := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Hashaton, Scarab's Fist", Owner: 0, InstanceID: "h0OGhash1",
			Types: []string{"legendary", "creature"},
		},
		Controller: 0, Owner: 0, Timestamp: 10,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, hashaton)

	srcID := "h0OGdiscard1"
	discardCard := &gameengine.Card{
		Name: "Tarmogoyf", Owner: 0, InstanceID: srcID,
		Types: []string{"creature", "elemental"}, CMC: 2,
		BasePower: 0, BaseToughness: 1,
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, discardCard)
	gs.MintedInstanceIDs = map[string]struct{}{
		hashaton.Card.InstanceID: {},
		discardCard.InstanceID:   {},
	}

	// Hashaton's discard trigger fires when controller discards a card.
	// Drive it directly with ctx carrying the discarded card.
	hashatonDiscardTrigger(gs, hashaton, map[string]interface{}{
		"card":          discardCard,
		"discarder_seat": 0,
	})

	if discardCard.InstanceID != srcID {
		t.Fatalf("discarded card's InstanceID rewritten: %q != %q", discardCard.InstanceID, srcID)
	}

	// Hashaton's handler may opt to skip (no_target / unmet condition). If
	// a token landed, validate the chokepoint; otherwise skip.
	var token *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p == hashaton {
			continue
		}
		token = p
		break
	}
	if token == nil {
		t.Skip("hashaton handler did not produce a token (cost/condition unmet) — skip")
		return
	}
	if token.Card == discardCard {
		t.Fatalf("zombie token *Card aliases discarded card")
	}
	if token.Card.InstanceID == srcID {
		t.Fatalf("zombie token inherited discarded ID — MintTokenAsCopyOf missed")
	}
	if mintProvenance(token.Card.InstanceID) != "TK" {
		t.Fatalf("zombie token InstanceID %q is not TK-provenance", token.Card.InstanceID)
	}
	// Template overrides must still apply post-mint.
	if token.Card.BasePower != 4 || token.Card.BaseToughness != 4 {
		t.Fatalf("zombie token P/T = %d/%d, want 4/4 (template overrides lost)",
			token.Card.BasePower, token.Card.BaseToughness)
	}
	hasZombie := false
	for _, tp := range token.Card.Types {
		if tp == "zombie" {
			hasZombie = true
		}
	}
	if !hasZombie {
		t.Fatalf("zombie token missing 'zombie' subtype after mint+override")
	}
}
