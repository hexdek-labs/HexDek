package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Batch V (R60) — tests for 5 new protection handlers
// -----------------------------------------------------------------------------

// helper: assert a GrantedAbilities slice contains all of `want`.
func hasGrants(p *gameengine.Permanent, want ...string) bool {
	if p == nil {
		return false
	}
	has := map[string]bool{}
	for _, g := range p.GrantedAbilities {
		has[g] = true
	}
	for _, w := range want {
		if !has[w] {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// Heroic Intervention
// -----------------------------------------------------------------------------

func TestHeroicIntervention_GrantsHexproofAndIndestructibleToAllOwnPerms(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "Creature A", "creature")
	c2 := addPerm(gs, 0, "Creature B", "creature")
	rock := addPerm(gs, 0, "Sol Ring", "artifact")
	oppPerm := addPerm(gs, 1, "Opp Creature", "creature")

	card := addCard(gs, 0, "Heroic Intervention", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !hasGrants(c1, "hexproof", "indestructible") {
		t.Errorf("c1 should have hexproof + indestructible, got %v", c1.GrantedAbilities)
	}
	if !hasGrants(c2, "hexproof", "indestructible") {
		t.Errorf("c2 should have hexproof + indestructible, got %v", c2.GrantedAbilities)
	}
	if !hasGrants(rock, "hexproof", "indestructible") {
		t.Errorf("Sol Ring should also get grants (non-creature perms included), got %v", rock.GrantedAbilities)
	}
	if hasGrants(oppPerm, "hexproof") {
		t.Errorf("opp creature should NOT have hexproof grant")
	}
}

// -----------------------------------------------------------------------------
// Mother of Runes
// -----------------------------------------------------------------------------

func TestMotherOfRunes_GrantsProtectionFromMostCommonOppColor(t *testing.T) {
	gs := newGame(t, 2)
	mom := addPerm(gs, 0, "Mother of Runes", "creature")
	mom.SummoningSick = false
	mom.Card.BasePower = 1
	mom.Card.BaseToughness = 1
	target := addPerm(gs, 0, "Big Creature", "creature")
	target.Card.BasePower = 5

	// Opp has 2 blue creatures.
	c1 := addPerm(gs, 1, "Snapcaster", "creature", "pip:U")
	c2 := addPerm(gs, 1, "Counterspell Goblin", "creature", "pip:U")
	_ = c1
	_ = c2

	gameengine.InvokeActivatedHook(gs, mom, 0, nil)

	if !mom.Tapped {
		t.Errorf("Mom should be tapped")
	}
	if !hasGrants(target, "protection") {
		t.Errorf("target should have 'protection' grant, got %v", target.GrantedAbilities)
	}
	if target.Flags["protection_from_U"] != 1 {
		t.Errorf("target should have protection_from_U set (opp board is mono-blue), flags=%v", target.Flags)
	}
}

func TestMotherOfRunes_SummoningSickFails(t *testing.T) {
	gs := newGame(t, 2)
	mom := addPerm(gs, 0, "Mother of Runes", "creature")
	mom.SummoningSick = true
	addPerm(gs, 0, "Target", "creature")

	gameengine.InvokeActivatedHook(gs, mom, 0, nil)

	if mom.Tapped {
		t.Errorf("summoning-sick Mom should not be tappable")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed on summoning sick")
	}
}

// -----------------------------------------------------------------------------
// Selfless Spirit
// -----------------------------------------------------------------------------

func TestSelflessSpirit_SacrificesAndGrantsIndestructibleToCreatures(t *testing.T) {
	gs := newGame(t, 2)
	spirit := addPerm(gs, 0, "Selfless Spirit", "creature")
	c1 := addPerm(gs, 0, "Friend A", "creature")
	c2 := addPerm(gs, 0, "Friend B", "creature")
	rock := addPerm(gs, 0, "Sol Ring", "artifact")

	gameengine.InvokeActivatedHook(gs, spirit, 0, nil)

	// Spirit should be in graveyard.
	foundInGy := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == spirit.Card {
			foundInGy = true
		}
	}
	if !foundInGy {
		t.Errorf("Selfless Spirit should be in graveyard after sac")
	}
	if !hasGrants(c1, "indestructible") {
		t.Errorf("c1 should have indestructible, got %v", c1.GrantedAbilities)
	}
	if !hasGrants(c2, "indestructible") {
		t.Errorf("c2 should have indestructible, got %v", c2.GrantedAbilities)
	}
	if hasGrants(rock, "indestructible") {
		t.Errorf("Sol Ring (non-creature) should NOT get indestructible from Selfless Spirit")
	}
}

// -----------------------------------------------------------------------------
// Tamiyo's Safekeeping
// -----------------------------------------------------------------------------

func TestTamiyosSafekeeping_GrantsAndGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	target := addPerm(gs, 0, "Walking Ballista", "artifact", "creature")
	target.Card.BasePower = 5

	card := addCard(gs, 0, "Tamiyo's Safekeeping", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !hasGrants(target, "hexproof", "indestructible") {
		t.Errorf("target should have hexproof + indestructible, got %v", target.GrantedAbilities)
	}
	if gs.Seats[0].Life != 22 {
		t.Errorf("expected +2 life, got %d", gs.Seats[0].Life)
	}
}

func TestTamiyosSafekeeping_LifeGainEvenOnNoTarget(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	// No own permanents.

	card := addCard(gs, 0, "Tamiyo's Safekeeping", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[0].Life != 22 {
		t.Errorf("life clause should apply unconditionally, got %d", gs.Seats[0].Life)
	}
}

// -----------------------------------------------------------------------------
// Akroma's Will
// -----------------------------------------------------------------------------

func TestAkromasWill_GrantsKeywordStackAndColorProtection(t *testing.T) {
	gs := newGame(t, 2)
	c1 := addPerm(gs, 0, "Creature A", "creature")
	c2 := addPerm(gs, 0, "Creature B", "creature")
	rock := addPerm(gs, 0, "Sol Ring", "artifact")

	card := addCard(gs, 0, "Akroma's Will", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	wantKeywords := []string{"flying", "vigilance", "double_strike", "lifelink", "indestructible"}
	if !hasGrants(c1, wantKeywords...) {
		t.Errorf("c1 should have all 5 keywords, got %v", c1.GrantedAbilities)
	}
	if !hasGrants(c2, wantKeywords...) {
		t.Errorf("c2 should have all 5 keywords, got %v", c2.GrantedAbilities)
	}
	if hasGrants(rock, "flying") {
		t.Errorf("non-creature should NOT get Akroma's Will keywords")
	}
	if c1.Flags["protection_from_black"] != 1 {
		t.Errorf("c1 should have protection_from_black, got flags=%v", c1.Flags)
	}
	if c1.Flags["protection_from_red"] != 1 {
		t.Errorf("c1 should have protection_from_red, got flags=%v", c1.Flags)
	}
}

func TestAkromasWill_EmptyBoardNoOp(t *testing.T) {
	gs := newGame(t, 2)
	// No creatures.

	card := addCard(gs, 0, "Akroma's Will", "sorcery")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	// Should not panic. The handler emits a per_card_handler event
	// with granted=0.
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler event even on empty-board resolve")
	}
}
