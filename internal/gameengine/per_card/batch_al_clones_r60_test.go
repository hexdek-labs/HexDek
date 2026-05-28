package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Clone
// -----------------------------------------------------------------------------

func TestClone_CopiesTargetCreature(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 0, "Llanowar Elves", "creature")
	target.Card.BasePower = 1
	target.Card.BaseToughness = 1

	clone := addPerm(gs, 0, "Clone", "creature")
	clone.Card.BasePower = 0
	clone.Card.BaseToughness = 0

	cloneETB(gs, clone)

	if clone.Card.DisplayName() != "Llanowar Elves" {
		t.Errorf("Clone should copy Llanowar Elves name, got %q", clone.Card.DisplayName())
	}
	if clone.Card.BasePower != 1 || clone.Card.BaseToughness != 1 {
		t.Errorf("Clone should copy 1/1; got %d/%d", clone.Card.BasePower, clone.Card.BaseToughness)
	}
	if clone.OriginalCard == nil {
		t.Errorf("Clone should retain OriginalCard pointer for unwind")
	}
}

func TestClone_NoTargetFallsThroughGracefully(t *testing.T) {
	gs := newGame(t, 2)
	// No other creatures on any battlefield.
	clone := addPerm(gs, 0, "Clone", "creature")
	cloneETB(gs, clone)

	// Name unchanged — clone enters as itself, a 0/0 destined for SBA.
	if clone.Card.DisplayName() != "Clone" {
		t.Errorf("Clone with no target should keep its name, got %q", clone.Card.DisplayName())
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler emission even on no-target path")
	}
}

func TestClone_PrefersOwnCreatureOverOpponents(t *testing.T) {
	gs := newGame(t, 2)
	// Seat 1 has a Dragon; seat 0 has a Mulldrifter we'd rather clone.
	opp := addPerm(gs, 1, "Shivan Dragon", "creature")
	opp.Card.BasePower = 5
	mine := addPerm(gs, 0, "Mulldrifter", "creature")
	mine.Card.BasePower = 2

	clone := addPerm(gs, 0, "Clone", "creature")
	cloneETB(gs, clone)

	if clone.Card.DisplayName() != "Mulldrifter" {
		t.Errorf("Clone should prefer own-controller target (Mulldrifter), got %q", clone.Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Phyrexian Metamorph
// -----------------------------------------------------------------------------

func TestPhyrexianMetamorph_CopiesArtifact(t *testing.T) {
	gs := newGame(t, 2)
	sol := addPerm(gs, 0, "Sol Ring", "artifact")

	meta := addPerm(gs, 0, "Phyrexian Metamorph", "artifact", "creature")
	phyrexianMetamorphETB(gs, meta)

	if meta.Card.DisplayName() != "Sol Ring" {
		t.Errorf("Metamorph should copy Sol Ring, got %q", meta.Card.DisplayName())
	}
	if meta.Flags["phy_metamorph_artifact_addendum"] != 1 {
		t.Errorf("Metamorph should stamp artifact-addendum flag")
	}
	_ = sol
}

func TestPhyrexianMetamorph_CopiesCreature(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Llanowar Elves", "creature")

	meta := addPerm(gs, 0, "Phyrexian Metamorph", "artifact", "creature")
	phyrexianMetamorphETB(gs, meta)

	if meta.Card.DisplayName() != "Llanowar Elves" {
		t.Errorf("Metamorph should copy creature when no artifact, got %q", meta.Card.DisplayName())
	}
	// Artifact addendum still applied — copied creature gains artifact type.
	if meta.Flags["phy_metamorph_artifact_addendum"] != 1 {
		t.Errorf("Metamorph artifact rider must apply even when copying a non-artifact creature")
	}
}

func TestPhyrexianMetamorph_IgnoresNoncreatureNonartifact(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Ghostly Prison", "enchantment")

	meta := addPerm(gs, 0, "Phyrexian Metamorph", "artifact", "creature")
	phyrexianMetamorphETB(gs, meta)

	if meta.Card.DisplayName() != "Phyrexian Metamorph" {
		t.Errorf("Metamorph should not copy an enchantment, got %q", meta.Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Copy Enchantment
// -----------------------------------------------------------------------------

func TestCopyEnchantment_CopiesEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Rhystic Study", "enchantment")

	copy := addPerm(gs, 0, "Copy Enchantment", "enchantment")
	copyEnchantmentETB(gs, copy)

	if copy.Card.DisplayName() != "Rhystic Study" {
		t.Errorf("Copy Enchantment should copy Rhystic Study, got %q", copy.Card.DisplayName())
	}
}

func TestCopyEnchantment_AuraStampsAttachFlag(t *testing.T) {
	gs := newGame(t, 2)
	aura := addPerm(gs, 0, "Pacifism", "enchantment", "aura")
	_ = aura

	copy := addPerm(gs, 0, "Copy Enchantment", "enchantment")
	copyEnchantmentETB(gs, copy)

	if copy.Card.DisplayName() != "Pacifism" {
		t.Errorf("Copy Enchantment should copy Pacifism, got %q", copy.Card.DisplayName())
	}
	if copy.Flags["copy_enchantment_needs_aura_attach"] != 1 {
		t.Errorf("Aura copy must stamp attach-needed flag for engine pickup")
	}
}

func TestCopyEnchantment_NoEnchantmentFallsThrough(t *testing.T) {
	gs := newGame(t, 2)
	// Only creatures on battlefield.
	addPerm(gs, 0, "Llanowar Elves", "creature")

	copy := addPerm(gs, 0, "Copy Enchantment", "enchantment")
	copyEnchantmentETB(gs, copy)

	if copy.Card.DisplayName() != "Copy Enchantment" {
		t.Errorf("Copy Enchantment with no enchantment target should keep its name, got %q",
			copy.Card.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// Spitting Image
// -----------------------------------------------------------------------------

func TestSpittingImage_CreatesTokenCopyOfTarget(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Avenger of Zendikar", "creature")
	target.Card.BasePower = 3
	target.Card.BaseToughness = 3

	bfBefore := len(gs.Seats[0].Battlefield)
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Spitting Image", Owner: 0},
		Targets: []gameengine.Target{
			{Kind: gameengine.TargetKindPermanent, Permanent: target},
		},
	}
	spittingImageResolve(gs, item)

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Fatalf("Spitting Image should mint 1 token; bf grew by %d",
			len(gs.Seats[0].Battlefield)-bfBefore)
	}
	token := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if token.Card.DisplayName() != "Avenger of Zendikar" {
		t.Errorf("token should be a copy of Avenger, got %q", token.Card.DisplayName())
	}
	// Token marker on the Card types.
	foundToken := false
	for _, ty := range token.Card.Types {
		if ty == "token" {
			foundToken = true
			break
		}
	}
	if !foundToken {
		t.Errorf("Spitting Image token should carry 'token' type tag, got types %v", token.Card.Types)
	}
	if !token.SummoningSick {
		t.Errorf("Spitting Image token should enter with summoning sickness")
	}
	if token.Controller != 0 {
		t.Errorf("Spitting Image token should be controlled by caster (seat 0), got %d", token.Controller)
	}
}

func TestSpittingImage_NoTargetEmitsButDoesNotPanic(t *testing.T) {
	gs := newGame(t, 2)
	bfBefore := len(gs.Seats[0].Battlefield)
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Spitting Image", Owner: 0},
	}
	spittingImageResolve(gs, item)

	if len(gs.Seats[0].Battlefield) != bfBefore {
		t.Errorf("Spitting Image with no target should mint nothing; bf grew by %d",
			len(gs.Seats[0].Battlefield)-bfBefore)
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler emission for no-target path")
	}
}

// -----------------------------------------------------------------------------
// Sakashima upgrade — full DeepCopy + legend suppression
// -----------------------------------------------------------------------------

func TestSakashima_FullDeepCopyAfterUpgrade(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 0, "Avenger of Zendikar", "creature")
	target.Card.BasePower = 5
	target.Card.BaseToughness = 5
	// Subtypes live in Types per the engine's flat type model.
	target.Card.Types = append(target.Card.Types, "elemental")

	saka := addPerm(gs, 0, "Sakashima of a Thousand Faces", "creature")
	saka.Card.BasePower = 3
	saka.Card.BaseToughness = 1

	sakashimaCopyETB(gs, saka)

	// After upgrade Sakashima should reflect the full DeepCopy: P/T,
	// subtypes, and the legend-rule-suppression rider all set.
	if saka.Card.DisplayName() != "Avenger of Zendikar" {
		t.Errorf("Sakashima upgrade should copy target's Card name, got %q", saka.Card.DisplayName())
	}
	if saka.Card.BasePower != 5 || saka.Card.BaseToughness != 5 {
		t.Errorf("Sakashima upgrade should copy 5/5; got %d/%d",
			saka.Card.BasePower, saka.Card.BaseToughness)
	}
	foundElemental := false
	for _, ty := range saka.Card.Types {
		if ty == "elemental" {
			foundElemental = true
			break
		}
	}
	if !foundElemental {
		t.Errorf("Sakashima upgrade should copy subtypes (looking for 'elemental' in Types); got %v", saka.Card.Types)
	}
	if saka.Flags["legend_rule_suppressed"] != 1 {
		t.Errorf("Sakashima upgrade should stamp legend_rule_suppressed; flags=%v", saka.Flags)
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test
// -----------------------------------------------------------------------------

func TestRegistry_BatchALClonesRegistered(t *testing.T) {
	etbCards := []string{
		"Clone",
		"Phyrexian Metamorph",
		"Copy Enchantment",
	}
	for _, n := range etbCards {
		if !HasETB(n) {
			t.Errorf("expected ETB handler for %s", n)
		}
	}
	if !HasResolve("Spitting Image") {
		t.Errorf("expected Resolve handler for Spitting Image")
	}
}
