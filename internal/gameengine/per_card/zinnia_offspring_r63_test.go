package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestZinnia_OffspringRoutesThroughCanonicalCopy pins that Zinnia's
// offspring trigger mints a FAITHFUL 1/1 token copy via the canonical
// gameengine.CreateOffspringToken path — replacing the old hand-rolled
// name/types/colors-only token that dropped copiable values (CMC → 0) and
// bypassed token doublers + the token_created trigger.
func TestZinnia_OffspringRoutesThroughCanonicalCopy(t *testing.T) {
	gs := newTestGS(2)

	zinnia := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Zinnia, Valley's Voice", Owner: 0, Types: []string{"creature"}},
		Controller: 0, Owner: 0, Timestamp: 10,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	enteringCard := &gameengine.Card{
		Name:          "Skyfin Drake",
		Owner:         0,
		BasePower:     4,
		BaseToughness: 4,
		CMC:           5,
		Colors:        []string{"U"},
		Types:         []string{"creature", "drake"},
	}
	entering := &gameengine.Permanent{
		Card: enteringCard, Controller: 0, Owner: 0, Timestamp: 11,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{zinnia, entering}

	zinniaValleysVoiceOffspring(gs, zinnia, map[string]interface{}{
		"perm":            entering,
		"controller_seat": 0,
	})

	// Find the minted token (the *Card that isn't the entering creature's).
	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p == zinnia || p == entering || p.Card == nil {
			continue
		}
		if p.IsToken() {
			tok = p
		}
	}
	if tok == nil {
		t.Fatal("Zinnia offspring should mint a token copy of the entering creature")
	}
	// Faithful 1/1 copy with copiable values carried.
	if tok.Card.BasePower != 1 || tok.Card.BaseToughness != 1 {
		t.Fatalf("offspring token must be 1/1, got %d/%d", tok.Card.BasePower, tok.Card.BaseToughness)
	}
	if tok.Card.CMC != 5 {
		t.Fatalf("offspring token should copy CMC 5 (old hand-rolled token dropped it), got %d", tok.Card.CMC)
	}
	if len(tok.Card.Colors) != 1 || tok.Card.Colors[0] != "U" {
		t.Fatalf("offspring token should copy colors [U], got %v", tok.Card.Colors)
	}
	if tok.Card == enteringCard {
		t.Fatal("offspring token must be a fresh *Card, not alias the entering creature's")
	}

	// No recursion: a token entering must NOT spawn another offspring token
	// (Zinnia's IsToken guard). Exactly one token copy on the battlefield.
	tokenCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p != entering && p.Card != nil && p.IsToken() {
			tokenCount++
		}
	}
	if tokenCount != 1 {
		t.Fatalf("exactly one offspring token expected (no recursive minting), got %d", tokenCount)
	}
}
