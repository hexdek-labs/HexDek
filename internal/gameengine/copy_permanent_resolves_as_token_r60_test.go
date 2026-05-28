package gameengine

import (
	"math/rand"
	"testing"
)

// copy_permanent_resolves_as_token_r60_test.go — pins the r60 fix for
// the Naru Meha + Panharmonicon copy-cascade ZoneConservation cluster
// surfaced by the #692 post-fix verification report (1 game / 2
// violations in the 25K seed-42 sweep).
//
// Root cause: resolvePermanentSpellETB created a Permanent for an
// IsCopy=true stack item without stamping "token" onto the resolving
// Card.Types per CR §707.10f ("If a copy of a permanent spell
// resolves, it becomes a token; it's still a copy of the spell it was
// a copy of"). Every cascade-copy that resolved as a permanent landed
// on the battlefield with the original Card.Types unmodified — and
// the ZoneConservation invariant's countRealCards/cardIsTokenForInv
// only checks Types for "token", so each copy was counted as a real
// card. A Naru Meha + Panharmonicon spell-copy cascade can resolve
// hundreds of permanent copies before the §704.3 mandatory-loop SBA
// cap closes the game, drifting the per-seat census by hundreds.
//
// Fix: in resolvePermanentSpellETB, stamp "token" onto card.Types
// when item.IsCopy && the card doesn't already carry the token type.
// Done at resolve-time (not at copy creation in resolveCopySpell)
// because §707.10f only kicks in on resolution — the copy is a real
// spell while it's on the stack.

func TestCopyPermanent_ResolvesAsToken(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(42)), nil)
	// Pre-test: lock in the zone-conservation baseline so the invariant
	// has something to compare against.
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("baseline ZoneConservation seed failed: %v", err)
	}
	baseline := gs.Flags["_zone_conservation_total"]

	// Synthesize a permanent spell (creature card) copy resolving as
	// the cascade does in the Naru Meha case.
	naruMeha := &Card{
		Name:          "Naru Meha, Master Wizard",
		Owner:         0,
		Types:         []string{"legendary", "creature", "human", "wizard"},
		BasePower:     3,
		BaseToughness: 3,
	}
	copyItem := &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       naruMeha,
		IsCopy:     true,
	}
	perm := resolvePermanentSpellETB(gs, copyItem)
	if perm == nil {
		t.Fatal("resolvePermanentSpellETB returned nil for a creature copy")
	}
	if !perm.IsToken() {
		t.Errorf("copy-resolved permanent must be a token per CR §707.10f, Types=%v", perm.Card.Types)
	}
	// Idempotency: a second resolution of an already-tokened copy must
	// not append a duplicate "token" type.
	naruMeha2 := &Card{
		Name:          "Naru Meha, Master Wizard",
		Owner:         0,
		Types:         []string{"legendary", "creature", "human", "wizard", "token"},
		BasePower:     3,
		BaseToughness: 3,
	}
	copyItem2 := &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       naruMeha2,
		IsCopy:     true,
	}
	perm2 := resolvePermanentSpellETB(gs, copyItem2)
	if perm2 == nil {
		t.Fatal("resolvePermanentSpellETB returned nil for already-tokened copy")
	}
	tokenCount := 0
	for _, ty := range perm2.Card.Types {
		if ty == "token" {
			tokenCount++
		}
	}
	if tokenCount != 1 {
		t.Errorf("already-tokened copy must not double-stamp token, Types=%v", perm2.Card.Types)
	}

	// Zone conservation must NOT fire after the copy resolves — both
	// permanents are tokens, so the per-seat census shouldn't grow.
	if err := checkZoneConservation(gs); err != nil {
		t.Errorf("ZoneConservation must stay clean after copy-perm resolves as token: %v", err)
	}
	if gs.Flags["_zone_conservation_total"] != baseline {
		t.Errorf("baseline drift: was %d, now %d", baseline, gs.Flags["_zone_conservation_total"])
	}
}

func TestCopyPermanent_NonCopyStillReal(t *testing.T) {
	// Counterfactual: a non-copy permanent spell that resolves must
	// NOT be flagged as a token (the fix targets ONLY IsCopy items).
	gs := NewGameState(2, rand.New(rand.NewSource(42)), nil)
	bears := &Card{
		Name:          "Grizzly Bears",
		Owner:         0,
		Types:         []string{"creature", "bear"},
		BasePower:     2,
		BaseToughness: 2,
	}
	realCast := &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       bears,
		IsCopy:     false,
	}
	perm := resolvePermanentSpellETB(gs, realCast)
	if perm == nil {
		t.Fatal("nil return on a real (non-copy) creature cast")
	}
	if perm.IsToken() {
		t.Error("real (non-copy) cast must NOT be flagged as a token — only IsCopy=true gets the §707.10f tag")
	}
	if perm.Flags["was_cast"] != 1 {
		t.Errorf("real cast must keep the was_cast flag, got %v", perm.Flags)
	}
}

// TestCopyPermanentCascade_CensusDoesNotDrift simulates the cascade
// shape from game 14620: many copy permanents resolving in sequence.
// Pre-fix this drifts the ZoneConservation total by N (one per copy);
// post-fix every copy is a token and the total stays put.
func TestCopyPermanentCascade_CensusDoesNotDrift(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(42)), nil)
	if err := checkZoneConservation(gs); err != nil {
		t.Fatalf("baseline seed failed: %v", err)
	}
	baseline := gs.Flags["_zone_conservation_total"]

	// 50 cascade copies — well above the invariant's "delta > 10"
	// suspicious threshold.
	for i := 0; i < 50; i++ {
		card := &Card{
			Name:          "Naru Meha, Master Wizard",
			Owner:         0,
			Types:         []string{"legendary", "creature", "human", "wizard"},
			BasePower:     3,
			BaseToughness: 3,
		}
		item := &StackItem{
			Kind:       "spell",
			Controller: 0,
			Card:       card,
			IsCopy:     true,
		}
		resolvePermanentSpellETB(gs, item)
	}

	// All 50 must be on the battlefield as tokens.
	tokens, nonTokens := 0, 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil {
			continue
		}
		if p.IsToken() {
			tokens++
		} else {
			nonTokens++
		}
	}
	if tokens != 50 {
		t.Errorf("expected 50 token permanents on battlefield, got %d (non-tokens: %d)", tokens, nonTokens)
	}

	// Census must NOT have drifted.
	if err := checkZoneConservation(gs); err != nil {
		t.Errorf("ZoneConservation drifted after 50-copy cascade: %v", err)
	}
	if gs.Flags["_zone_conservation_total"] != baseline {
		t.Errorf("baseline drift: was %d, now %d (50 copies leaked into the census)",
			baseline, gs.Flags["_zone_conservation_total"])
	}
}
