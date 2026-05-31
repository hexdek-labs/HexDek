package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestDrafna_TokenCopyClearsInheritedInstanceID pins the Loki r60
// CardIdentity violation surfaced by game 4635 (seed 42 turn 51,
// "Spikeshell Harrier h1OGVU500020 appears in both seat 1 battlefield
// and seat 1 battlefield"). Drafna's activated ability creates a
// token copy of a target nontoken artifact. The pre-fix path
// DeepCopy'd the source Card and prepended "token" to Types without
// clearing the inherited InstanceID — so the token Permanent wrapped
// a Card whose InstanceID still equaled the original Spikeshell
// Harrier's OG ID, producing the same-ID-in-two-perms duplication
// shape the CardIdentity invariant correctly flags.
//
// Fix: route through gameengine.MintTokenAsCopyOf, the Phase 5
// chokepoint (instanceid_phase5.go:300) — DeepCopys + clears the
// inherited ID + mints a fresh TK-provenance ID. Same fix pattern
// resolveCreateTokenCopy uses at resolve.go:2054.
func TestDrafna_TokenCopyClearsInheritedInstanceID(t *testing.T) {
	gs := newTestGS(2)
	gs.Active = 0

	drafnaCard := &gameengine.Card{
		Name:       "Drafna, Founder of Lat-Nam",
		Owner:      0,
		InstanceID: "h0OGdrafna1",
		Types:      []string{"legendary", "creature"},
		Colors:     []string{"U"},
	}
	drafna := &gameengine.Permanent{
		Card: drafnaCard, Controller: 0, Owner: 0, Timestamp: 10,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, drafna)

	// Source artifact whose ID we must NOT see on the token.
	sourceID := "h0OGspike1"
	sourceCard := &gameengine.Card{
		Name:       "Spikeshell Harrier",
		Owner:      0,
		InstanceID: sourceID,
		Types:      []string{"artifact", "creature"},
		BasePower:  4, BaseToughness: 4, CMC: 5,
	}
	source := &gameengine.Permanent{
		Card: sourceCard, Controller: 0, Owner: 0, Timestamp: 11,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, source)
	gs.MintedInstanceIDs = map[string]struct{}{
		drafnaCard.InstanceID: {},
		sourceCard.InstanceID: {},
	}
	gs.MintedInstanceIDNames = map[string]string{
		drafnaCard.InstanceID: drafnaCard.Name,
		sourceCard.InstanceID: sourceCard.Name,
	}

	preCount := len(gs.Seats[0].Battlefield)
	drafnaActivated(gs, drafna, 0, nil)
	if len(gs.Seats[0].Battlefield) != preCount+1 {
		t.Fatalf("expected one new token on seat 0's battlefield, got %d → %d",
			preCount, len(gs.Seats[0].Battlefield))
	}

	token := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if token == nil || token.Card == nil {
		t.Fatalf("token Permanent malformed: %v", token)
	}
	if token.Card == sourceCard {
		t.Fatalf("token's *Card aliases the source (DeepCopy bypassed)")
	}
	if token.Card.InstanceID == sourceID {
		t.Fatalf("token inherited the source's InstanceID %q — MintTokenAsCopyOf missed the clear-then-mint step",
			token.Card.InstanceID)
	}
	if token.Card.InstanceID == "" {
		t.Fatalf("token has no InstanceID — MintTokenAsCopyOf returned an unstamped Card")
	}
	if len(token.Card.InstanceID) < 4 || token.Card.InstanceID[2:4] != "TK" {
		t.Fatalf("token InstanceID %q is not TK-provenance (positions 2-3 must be 'TK')",
			token.Card.InstanceID)
	}
	if token.Card.SourceInstanceID != sourceID {
		t.Fatalf("token SourceInstanceID = %q, want %q (lineage tag missing)",
			token.Card.SourceInstanceID, sourceID)
	}

	hasToken := false
	for _, tp := range token.Card.Types {
		if tp == "token" {
			hasToken = true
			break
		}
	}
	if !hasToken {
		t.Fatalf("token's Types missing 'token' tag — MintTokenAsCopyOf should ensure it")
	}

	// CardIdentity invariant must be clean (no same-ID-in-two-perms).
	for _, inv := range gameengine.AllInvariants() {
		if inv.Name != "CardIdentity" {
			continue
		}
		if err := inv.Check(gs); err != nil {
			t.Fatalf("CardIdentity post-fix: %v", err)
		}
	}
}
