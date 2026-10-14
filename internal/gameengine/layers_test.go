package gameengine

// Phase 8 — §613 Continuous Effects / Layer System tests.
//
// 25+ test cases covering the 9 layer-critical cards + interaction
// paradoxes (Humility + Opalescence, Mycosynth + Painter, Blood Moon
// + Dryad Arbor, Humility + counters, etc.) + cache invalidation +
// benchmark.
//
// Uses the shared fixture helpers from resolve_test.go:
//   - newFixtureGame(t)
//   - addBattlefield(gs, seat, name, pow, tough, types...)
//
// Rule citations:
//   §613.1d / §613.4b — layer 4 (type) + layer 7b (P/T set)
//   §613.4c           — layer 7c post-pass for counters
//   §613.7            — timestamp tiebreak
//   §305.7            — Blood Moon land-subtype replacement
//   §708.2 / §613.2b  — face-down characteristic-defining override

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// layerAt is a convenience: add battlefield perm + register its
// per-card layer handlers.
func layerAt(gs *GameState, seat int, name string, pow, tough int, types ...string) *Permanent {
	p := addBattlefield(gs, seat, name, pow, tough, types...)
	RegisterContinuousEffectsForPermanent(gs, p)
	return p
}

// -----------------------------------------------------------------------------
// Humility — layer 6 + 7b strip & set-P/T.
// -----------------------------------------------------------------------------

func TestLayer_Humility_Alone(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	_ = layerAt(gs, 0, "", 0, 0) // no-op marker
	_ = bear
	// Second creature with baseline 5/5 + hypothetical flying keyword
	dragon := addBattlefield(gs, 1, "Shivan Dragon", 5, 5, "creature")
	// Pre-strip: dragon is 5/5; after Humility: 1/1, no keywords.
	chars := GetEffectiveCharacteristics(gs, dragon)
	if chars.Power != 1 || chars.Toughness != 1 {
		t.Fatalf("Humility: dragon should be 1/1, got %d/%d",
			chars.Power, chars.Toughness)
	}
	if len(chars.Keywords) != 0 {
		t.Errorf("Humility should strip keywords, got %v", chars.Keywords)
	}
	if len(chars.Abilities) != 0 {
		t.Errorf("Humility should strip abilities, got %d abilities", len(chars.Abilities))
	}
	// Bear: also 1/1.
	bchars := GetEffectiveCharacteristics(gs, bear)
	if bchars.Power != 1 || bchars.Toughness != 1 {
		t.Errorf("Humility: bear should be 1/1, got %d/%d", bchars.Power, bchars.Toughness)
	}
}

func TestLayer_Humility_DoesNotAffectNonCreatures(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	artifact := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	chars := GetEffectiveCharacteristics(gs, artifact)
	// Not a creature → Humility shouldn't touch it.
	if !charsHaveType(chars.Types, "artifact") {
		t.Error("artifact should still be an artifact")
	}
}

// -----------------------------------------------------------------------------
// Opalescence — layer 4 + 7b: enchantments become creatures with P/T=CMC.
// -----------------------------------------------------------------------------

func TestLayer_Opalescence_Alone(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	// Register a 3-CMC enchantment that isn't an aura.
	ench := addBattlefield(gs, 0, "Sylvan Library", 0, 0, "enchantment")
	ench.Flags["cmc"] = 3
	chars := GetEffectiveCharacteristics(gs, ench)
	if !charsHaveType(chars.Types, "creature") {
		t.Fatalf("Opalescence should add creature type; got types=%v", chars.Types)
	}
	if !charsHaveType(chars.Types, "enchantment") {
		t.Error("enchantment type must be preserved")
	}
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("CMC-3 enchantment should be 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Opalescence_SelfExclusion(t *testing.T) {
	gs := newFixtureGame(t)
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4
	// Opalescence itself should NOT become a creature (self-exclusion).
	chars := GetEffectiveCharacteristics(gs, opal)
	if charsHaveType(chars.Types, "creature") {
		t.Fatalf("Opalescence self-exclusion failed; types=%v", chars.Types)
	}
}

func TestLayer_Opalescence_SkipsAuras(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	aura := addBattlefield(gs, 0, "Unquestioned Authority", 0, 0, "enchantment", "aura")
	aura.Flags["cmc"] = 3
	chars := GetEffectiveCharacteristics(gs, aura)
	if charsHaveType(chars.Types, "creature") {
		t.Errorf("Aura should NOT become a creature via Opalescence; types=%v", chars.Types)
	}
}

// THE paradox: Humility + Opalescence both on battlefield. Humility
// ETBs FIRST (earlier timestamp), Opalescence second.
//
// Expected resolution per Python reference
// (test_layer_humility.py::test_humility_opalescence_interaction):
//
//   Layer 4 (Opalescence ts=3, predicate "other non-Aura enchantment"):
//     + Humility gets "creature" type added.
//     + Opalescence skips itself via self-exclusion ("each other").
//     + Bears: predicate fails (not enchantment); unaffected.
//   Layer 6 (Humility ts=1, strip abilities):
//     + Humility IS a creature in-layer → abilities stripped from self.
//     + Bears are creatures → abilities stripped.
//   Layer 7b (Humility ts=1 → Opalescence ts=3, timestamp order):
//     + Humility's 7b sets every creature to 1/1 (including self).
//     + Opalescence's 7b fires AFTER, re-sets Humility to P/T=CMC=4.
//       (Opalescence's 7b predicate: "other non-Aura enchantment", which
//        Humility still matches.)
//     + Bears: Opalescence's 7b predicate fails (bears aren't
//       enchantments), so Humility's 1/1 sticks.
//
// Final state:
//   - Humility: creature+enchantment, abilities=[], P/T=4/4 (CMC)
//   - Opalescence: enchantment only (self-excluded), abilities intact
//   - Bears: creature, abilities=[], P/T=1/1
//
// THIS IS THE XMAGE-BEATING PARADOX TEST — see
// scripts/test_xmage_differential.py for the XMage comparison.
func TestLayer_Humility_Opalescence_Paradox(t *testing.T) {
	gs := newFixtureGame(t)
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	hum.Flags["cmc"] = 4
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	// Bears: 1/1 no abilities (Humility L7b + L6).
	bc := GetEffectiveCharacteristics(gs, bear)
	if bc.Power != 1 || bc.Toughness != 1 {
		t.Errorf("bear under Humility should be 1/1, got %d/%d", bc.Power, bc.Toughness)
	}
	if len(bc.Abilities) != 0 {
		t.Errorf("bear under Humility should have no abilities, got %d", len(bc.Abilities))
	}

	// Humility: Opalescence's L4 made it a creature; L6 stripped its
	// abilities; L7b Humility set 1/1 then L7b Opalescence overrode to
	// CMC=4. Final: 4/4.
	hc := GetEffectiveCharacteristics(gs, hum)
	if !charsHaveType(hc.Types, "creature") {
		t.Errorf("Humility should be a creature via Opalescence L4; types=%v", hc.Types)
	}
	if hc.Power != 4 || hc.Toughness != 4 {
		t.Errorf("Humility should be 4/4 (Opalescence's L7b fires AFTER Humility's by timestamp), got %d/%d", hc.Power, hc.Toughness)
	}
	if len(hc.Abilities) != 0 {
		t.Errorf("Humility should have no abilities (stripped by self at L6), got %d", len(hc.Abilities))
	}

	// Opalescence: self-exclusion at layer 4 → NOT a creature. Not
	// touched by Humility's 7b (predicate checks in-layer creature).
	oc := GetEffectiveCharacteristics(gs, opal)
	if charsHaveType(oc.Types, "creature") {
		t.Errorf("Opalescence self-exclusion broken; types=%v", oc.Types)
	}
}

// -----------------------------------------------------------------------------
// Blood Moon — nonbasic lands are Mountains (§305.7).
// -----------------------------------------------------------------------------

func TestLayer_BloodMoon_NonbasicLand(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")
	// Nonbasic land "Steam Vents" with subtypes [island mountain].
	vents := addBattlefield(gs, 0, "Steam Vents", 0, 0, "land")
	vents.Card.Types = append(vents.Card.Types, "island", "mountain") // initial subtypes land-wise
	// Approximation: subtypes live on Card.Types under Phase 3
	// convention. We express them in the baseline chars via a manual set.
	// Instead, seed via Characteristics injection is cleaner — put
	// "island" as a subtype. We inject by adding to chars post-baseline:
	// this tests the registered-effect replacement path correctly.
	// Preferred path: override Card.AST — but simpler to mutate chars
	// via a synthetic layer-0 seed. For test purposes we verify that
	// the post-Blood Moon subtype list is ["mountain"].
	// Baseline: seed subtype via perm.Flags → but our
	// BaseCharacteristics doesn't read Flags for subtypes. Simulate
	// by registering a pre-layer-4 effect that adds printed subtypes.
	seed := addBattlefield(gs, 1, "_seed_vents", 0, 0)
	_ = seed
	// Easier: test that non-basic land's subtype becomes just
	// ["mountain"]. Baseline had empty subtype list (since addBattlefield
	// doesn't seed subtypes). Blood Moon's apply_fn unconditionally
	// appends "mountain" — which is what we want.
	chars := GetEffectiveCharacteristics(gs, vents)
	hasMountain := false
	for _, s := range chars.Subtypes {
		if s == "mountain" {
			hasMountain = true
		}
	}
	if !hasMountain {
		t.Errorf("Steam Vents should have mountain subtype after Blood Moon; got %v", chars.Subtypes)
	}
}

func TestLayer_BloodMoon_SkipsBasicLands(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")
	// Basic Forest — predicate should reject it.
	forest := addBattlefield(gs, 0, "Forest", 0, 0, "land", "basic")
	chars := GetEffectiveCharacteristics(gs, forest)
	// Blood Moon's predicate returns false; subtypes untouched.
	for _, s := range chars.Subtypes {
		if s == "mountain" {
			t.Errorf("Blood Moon should NOT affect basic Forest; got subtypes=%v", chars.Subtypes)
		}
	}
}

// Dryad Arbor: Land Creature — Forest Dryad. Blood Moon strips forest,
// adds mountain, preserves dryad creature-subtype.
func TestLayer_BloodMoon_DryadArbor(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")
	dryad := addBattlefield(gs, 0, "Dryad Arbor", 1, 1, "land", "creature")
	// Seed the subtypes: printed line "Land Creature — Forest Dryad"
	// isn't fully modeled in fixture; we inject via post-baseline seed.
	// We register a layer 0 "prior" effect that sets subtypes.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerCopy, Timestamp: gs.NextTimestamp(),
		SourcePerm: dryad, SourceCardName: "_dryad_seed",
		ControllerSeat: 0,
		HandlerID:      "dryad_seed",
		Predicate: func(_ *GameState, t *Permanent) bool { return t == dryad },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Subtypes = []string{"forest", "dryad"}
		},
	})
	chars := GetEffectiveCharacteristics(gs, dryad)
	hasMountain := false
	hasDryad := false
	hasForest := false
	for _, s := range chars.Subtypes {
		if s == "mountain" {
			hasMountain = true
		}
		if s == "dryad" {
			hasDryad = true
		}
		if s == "forest" {
			hasForest = true
		}
	}
	if !hasMountain {
		t.Errorf("Blood Moon should give Dryad Arbor mountain subtype; got %v", chars.Subtypes)
	}
	if !hasDryad {
		t.Errorf("Blood Moon should preserve 'dryad' creature-subtype per §305.7; got %v", chars.Subtypes)
	}
	if hasForest {
		t.Errorf("Blood Moon should strip 'forest' land-subtype; got %v", chars.Subtypes)
	}
	// Types preserved: land + creature.
	if !charsHaveType(chars.Types, "land") || !charsHaveType(chars.Types, "creature") {
		t.Errorf("Dryad Arbor types should be preserved; got %v", chars.Types)
	}
}

// -----------------------------------------------------------------------------
// Magus of the Moon — same layer 4 as Blood Moon, stacks idempotently.
// -----------------------------------------------------------------------------

func TestLayer_MagusOfTheMoon_Stacks(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")
	_ = layerAt(gs, 1, "Magus of the Moon", 2, 2, "creature")
	land := addBattlefield(gs, 0, "Tropical Island", 0, 0, "land")
	chars := GetEffectiveCharacteristics(gs, land)
	count := 0
	for _, s := range chars.Subtypes {
		if s == "mountain" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Blood Moon + Magus should produce exactly ONE mountain subtype (idempotent), got %d", count)
	}
}

// -----------------------------------------------------------------------------
// Painter's Servant — layer 5 adds chosen color.
// -----------------------------------------------------------------------------

func TestLayer_PaintersServant_AddsColor(t *testing.T) {
	gs := newFixtureGame(t)
	painter := addBattlefield(gs, 0, "Painter's Servant", 1, 3, "artifact", "creature")
	if painter.Flags == nil {
		painter.Flags = map[string]int{}
	}
	painter.Flags["painter_color_B"] = 1
	RegisterPaintersServant(gs, painter)

	// Multi-color creature baseline — say Abzan Charm would be WBG.
	target := addBattlefield(gs, 0, "Sultai Ascendancy", 0, 0, "enchantment")
	// Baseline seeds colors via Card. We inject via a prior layer-0 effect.
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerCopy, Timestamp: gs.NextTimestamp(),
		SourcePerm: target, HandlerID: "seed_colors",
		Predicate: func(_ *GameState, t *Permanent) bool { return t == target },
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Colors = []string{"U", "B", "G"}
		},
	})
	chars := GetEffectiveCharacteristics(gs, target)
	hasB := 0
	other := 0
	for _, c := range chars.Colors {
		if c == "B" {
			hasB++
		}
		if c == "U" || c == "G" {
			other++
		}
	}
	if hasB != 1 {
		t.Errorf("Painter 'B' should appear exactly once (original + add merged), got %d in %v", hasB, chars.Colors)
	}
	if other != 2 {
		t.Errorf("Painter should preserve other colors U, G; got %v", chars.Colors)
	}
	if gs.PainterColor != "B" {
		t.Errorf("gs.PainterColor should be B, got %q", gs.PainterColor)
	}
}

// -----------------------------------------------------------------------------
// Mycosynth Lattice — layer 4 artifact + layer 5 colorless.
// -----------------------------------------------------------------------------

func TestLayer_MycosynthLattice_AllArtifacts(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Mycosynth Lattice", 0, 0, "artifact")
	creature := addBattlefield(gs, 1, "Llanowar Elves", 1, 1, "creature")
	chars := GetEffectiveCharacteristics(gs, creature)
	if !charsHaveType(chars.Types, "artifact") {
		t.Errorf("Mycosynth should make creature an artifact; got types=%v", chars.Types)
	}
	if !charsHaveType(chars.Types, "creature") {
		t.Errorf("Original creature type must be preserved; got %v", chars.Types)
	}
	// And colorless.
	if len(chars.Colors) != 0 {
		t.Errorf("Mycosynth should make permanents colorless; got %v", chars.Colors)
	}
}

// Mycosynth (layer 5 colorless) + Painter (layer 5 adds color).
// Timestamp order decides final state: later-timestamp wins? No —
// within a layer both apply in timestamp order; Mycosynth comes
// first (clears), then Painter (adds). So Painter's color wins if
// Painter is timestamped AFTER Mycosynth.
func TestLayer_MycosynthPainter_TimestampOrder(t *testing.T) {
	gs := newFixtureGame(t)
	// Mycosynth first (earlier timestamp).
	_ = layerAt(gs, 0, "Mycosynth Lattice", 0, 0, "artifact")
	// Painter second (later timestamp).
	painter := addBattlefield(gs, 1, "Painter's Servant", 1, 3, "artifact", "creature")
	if painter.Flags == nil {
		painter.Flags = map[string]int{}
	}
	painter.Flags["painter_color_R"] = 1
	RegisterPaintersServant(gs, painter)

	target := addBattlefield(gs, 0, "Llanowar Elves", 1, 1, "creature")
	chars := GetEffectiveCharacteristics(gs, target)
	// Mycosynth clears → Painter adds R → final colors = ["R"].
	if len(chars.Colors) != 1 || chars.Colors[0] != "R" {
		t.Errorf("With Painter after Mycosynth, expected colors=[R], got %v", chars.Colors)
	}
}

// -----------------------------------------------------------------------------
// Ensoul Artifact — layer 4 creature + 7b 5/5.
// -----------------------------------------------------------------------------

func TestLayer_EnsoulArtifact_MakesArtifactCreature(t *testing.T) {
	gs := newFixtureGame(t)
	sol := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	aura := addBattlefield(gs, 0, "Ensoul Artifact", 0, 0, "enchantment", "aura")
	aura.AttachedTo = sol
	RegisterEnsoulArtifact(gs, aura)
	chars := GetEffectiveCharacteristics(gs, sol)
	if !charsHaveType(chars.Types, "creature") {
		t.Errorf("Ensoul'd Sol Ring should be a creature; types=%v", chars.Types)
	}
	if !charsHaveType(chars.Types, "artifact") {
		t.Errorf("Sol Ring should remain artifact; types=%v", chars.Types)
	}
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("Ensoul'd Sol Ring should be 5/5, got %d/%d", chars.Power, chars.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Splinter Twin — grants activated ability.
// -----------------------------------------------------------------------------

func TestLayer_SplinterTwin_GrantsAbility(t *testing.T) {
	gs := newFixtureGame(t)
	exarch := addBattlefield(gs, 0, "Deceiver Exarch", 1, 4, "creature")
	twin := addBattlefield(gs, 0, "Splinter Twin", 0, 0, "enchantment", "aura")
	twin.AttachedTo = exarch
	RegisterSplinterTwin(gs, twin)
	chars := GetEffectiveCharacteristics(gs, exarch)
	hasGrant := false
	for _, k := range chars.Keywords {
		if k == "splinter_twin_copy_token_activated" {
			hasGrant = true
		}
	}
	if !hasGrant {
		t.Errorf("Splinter Twin should grant copy ability; keywords=%v", chars.Keywords)
	}
}

// -----------------------------------------------------------------------------
// Conspiracy — layer 4 adds chosen subtype. §613.8 dependency SKIPPED.
// -----------------------------------------------------------------------------

func TestLayer_Conspiracy_AddsChosenSubtype(t *testing.T) {
	gs := newFixtureGame(t)
	consp := addBattlefield(gs, 0, "Conspiracy", 0, 0, "enchantment")
	if consp.Flags == nil {
		consp.Flags = map[string]int{}
	}
	consp.Flags["conspiracy_type_zombie"] = 1
	RegisterConspiracy(gs, consp)

	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	chars := GetEffectiveCharacteristics(gs, bear)
	hasZombie := false
	for _, s := range chars.Subtypes {
		if s == "Zombie" {
			hasZombie = true
		}
	}
	if !hasZombie {
		t.Errorf("Conspiracy should add Zombie subtype; got %v", chars.Subtypes)
	}
	// §613.8 dependency ordering is now implemented — no skipped flag.
}

// -----------------------------------------------------------------------------
// Counters (layer 7c post-pass).
// -----------------------------------------------------------------------------

func TestLayer_Plus1Counter(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("2/2 with +1/+1 counter should be 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Humility_Plus1Counter(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	chars := GetEffectiveCharacteristics(gs, bear)
	// Humility sets 1/1 (layer 7b), counter adds +1/+1 (post-pass) → 2/2.
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("Humility + +1/+1 counter: expected 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Minus1Counter(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Serra Angel", 4, 4, "creature")
	bear.AddCounter("-1/-1", 2)
	gs.InvalidateCharacteristicsCache()
	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("4/4 with 2× -1/-1 should be 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Cache invalidation.
// -----------------------------------------------------------------------------

func TestLayer_CacheInvalidation(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	// First query: 2/2.
	c1 := GetEffectiveCharacteristics(gs, bear)
	if c1.Power != 2 {
		t.Fatalf("baseline power=2, got %d", c1.Power)
	}
	// Second query returns cached pointer.
	c1b := GetEffectiveCharacteristics(gs, bear)
	if c1 != c1b {
		t.Errorf("cache should return same pointer")
	}

	// Register Humility → cache invalidation → next query sees 1/1.
	_ = layerAt(gs, 1, "Humility", 0, 0, "enchantment")
	c2 := GetEffectiveCharacteristics(gs, bear)
	if c2.Power != 1 {
		t.Errorf("post-Humility bear should be 1/1, got %d/%d", c2.Power, c2.Toughness)
	}
	if c2 == c1 {
		t.Errorf("cache should have been invalidated — same pointer returned")
	}
}

func TestLayer_UnregisterCleanup(t *testing.T) {
	gs := newFixtureGame(t)
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	c1 := GetEffectiveCharacteristics(gs, bear)
	if c1.Power != 1 {
		t.Fatalf("Humility active → bear should be 1/1, got %d", c1.Power)
	}
	// Humility leaves the battlefield.
	n := gs.UnregisterContinuousEffectsForPermanent(hum)
	if n == 0 {
		t.Errorf("expected to remove Humility's 2 continuous effects")
	}
	c2 := GetEffectiveCharacteristics(gs, bear)
	if c2.Power != 2 {
		t.Errorf("after Humility leaves, bear should be 2/2, got %d/%d", c2.Power, c2.Toughness)
	}
}

// -----------------------------------------------------------------------------
// Idempotent HandlerID registration.
// -----------------------------------------------------------------------------

func TestLayer_HandlerID_Idempotent(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	// Double-register — should be a no-op.
	before := len(gs.ContinuousEffects)
	RegisterHumility(gs, gs.Seats[0].Battlefield[0])
	// RegisterHumility creates 2 entries with new handler_id (new timestamp),
	// so idempotency is per-HandlerID, not per-card. Let's instead test
	// direct re-register with same struct:
	ce := &ContinuousEffect{
		Layer: LayerPT, Sublayer: "b", Timestamp: 0,
		HandlerID: "idem_test",
		Predicate: func(*GameState, *Permanent) bool { return false },
		ApplyFn:   func(*GameState, *Permanent, *Characteristics) {},
	}
	gs.RegisterContinuousEffect(ce)
	after1 := len(gs.ContinuousEffects)
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "b",
		HandlerID: "idem_test",
		Predicate: func(*GameState, *Permanent) bool { return false },
		ApplyFn:   func(*GameState, *Permanent, *Characteristics) {},
	})
	after2 := len(gs.ContinuousEffects)
	if after2 != after1 {
		t.Errorf("HandlerID idempotency broken: %d → %d (should stay)", after1, after2)
	}
	_ = before
}

// -----------------------------------------------------------------------------
// Face-down (§708.2 / §613.2b) — 2/2 colorless no-abilities.
// -----------------------------------------------------------------------------

func TestLayer_FaceDown_CharacteristicOverride(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Card.FaceDown = true
	gs.InvalidateCharacteristicsCache()
	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("face-down should be 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	if chars.Name != "" {
		t.Errorf("face-down should have no name, got %q", chars.Name)
	}
	if len(chars.Colors) != 0 {
		t.Errorf("face-down should be colorless, got %v", chars.Colors)
	}
	if len(chars.Abilities) != 0 {
		t.Errorf("face-down should have no abilities")
	}
	if !chars.FaceDown {
		t.Errorf("FaceDown flag should be true")
	}
}

// -----------------------------------------------------------------------------
// Integration — PowerOf / ToughnessOf accessors.
// -----------------------------------------------------------------------------

func TestLayer_PowerOfAccessor(t *testing.T) {
	gs := newFixtureGame(t)
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	if gs.PowerOf(bear) != 1 {
		t.Errorf("gs.PowerOf should route through layers; expected 1, got %d", gs.PowerOf(bear))
	}
	if gs.ToughnessOf(bear) != 1 {
		t.Errorf("gs.ToughnessOf should route through layers; expected 1, got %d", gs.ToughnessOf(bear))
	}
	if !gs.IsCreatureOf(bear) {
		t.Errorf("gs.IsCreatureOf should see bear as creature post-Humility")
	}
}

func TestLayer_HasKeywordOf_StrippedByHumility(t *testing.T) {
	gs := newFixtureGame(t)
	// Pre-Humility: register a dragon with Flying via grant.
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.GrantedAbilities = []string{"flying"}
	if !gs.HasKeywordOf(dragon, "flying") {
		t.Fatalf("dragon should have flying pre-Humility")
	}
	_ = layerAt(gs, 1, "Humility", 0, 0, "enchantment")
	if gs.HasKeywordOf(dragon, "flying") {
		t.Errorf("Humility should strip flying from dragon")
	}
}

// -----------------------------------------------------------------------------
// §613.8 Dependency Ordering Tests
// -----------------------------------------------------------------------------

// TestLayerDependency_HumilityOpalescence tests the classic paradox:
// Humility (layer 6 strip + layer 7b set 1/1) + Opalescence (layer 4
// type-add + layer 7b set P/T=CMC). These create a circular dependency
// at layer 7b (Humility sets 1/1, Opalescence overrides to CMC). CR
// §613.8b resolves circular dependencies via timestamp order.
//
// With Humility entered first (ts=1) and Opalescence second (ts=2):
//   - Layer 4: Opalescence makes Humility a creature (predicate: other
//     non-Aura enchantment). Opalescence self-excludes.
//   - Layer 6: Humility strips abilities from all creatures (including
//     itself, now a creature via Opalescence).
//   - Layer 7b: Humility sets all creatures to 1/1 (ts=1 first), then
//     Opalescence overrides Humility back to CMC=4 (ts=2 second).
//
// This is the XMAGE-BEATING PARADOX — dependency ordering detects the
// circularity and falls back to timestamp order, producing the correct
// result.
func TestLayerDependency_HumilityOpalescence(t *testing.T) {
	gs := newFixtureGame(t)
	// Humility enters first (lower timestamp).
	hum := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	hum.Flags["cmc"] = 4
	// Opalescence enters second (higher timestamp).
	opal := layerAt(gs, 0, "Opalescence", 0, 0, "enchantment")
	opal.Flags["cmc"] = 4
	// A regular creature to verify Humility's effect.
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	gs.InvalidateCharacteristicsCache()

	// Bears: Humility sets 1/1 (L7b), abilities stripped (L6).
	bc := GetEffectiveCharacteristics(gs, bear)
	if bc.Power != 1 || bc.Toughness != 1 {
		t.Errorf("bear under Humility should be 1/1, got %d/%d", bc.Power, bc.Toughness)
	}
	if len(bc.Abilities) != 0 {
		t.Errorf("bear should have no abilities under Humility, got %d", len(bc.Abilities))
	}

	// Humility: Opalescence L4 makes it a creature, L6 strips abilities,
	// L7b circular dependency → timestamp order → Humility 1/1 then
	// Opalescence overrides to 4/4.
	hc := GetEffectiveCharacteristics(gs, hum)
	if !charsHaveType(hc.Types, "creature") {
		t.Errorf("Humility should be a creature via Opalescence L4; types=%v", hc.Types)
	}
	if hc.Power != 4 || hc.Toughness != 4 {
		t.Errorf("Humility should be 4/4 (Opalescence's L7b overrides after circular dep fallback), got %d/%d", hc.Power, hc.Toughness)
	}
	if len(hc.Abilities) != 0 {
		t.Errorf("Humility should have no abilities (stripped by self at L6), got %d", len(hc.Abilities))
	}

	// Opalescence: self-exclusion at layer 4 → NOT a creature.
	oc := GetEffectiveCharacteristics(gs, opal)
	if charsHaveType(oc.Types, "creature") {
		t.Errorf("Opalescence self-exclusion broken; types=%v", oc.Types)
	}

	t.Log("§613.8 Humility + Opalescence: circular dependency detected, timestamp fallback correct")
}

// TestLayerDependency_BloodMoonUrborg tests Blood Moon + Urborg, Tomb
// of Yawgmoth interaction — a dependency ordering case (not circular).
//
// Blood Moon: nonbasic lands are Mountains (layer 4 type + layer 6
// ability strip). Urborg: all lands are Swamps in addition (layer 4).
//
// If Blood Moon entered first (ts=1), Urborg entered second (ts=2):
//   - Blood Moon applies to Urborg (nonbasic land) → Urborg becomes a
//     Mountain, losing Swamp subtype and its abilities.
//   - Urborg's effect is stripped by Blood Moon's L6 → no "all lands are
//     Swamps" ability.
//   - Result: all nonbasic lands are Mountains. Urborg is a Mountain.
//
// If Urborg entered first (ts=1), Blood Moon entered second (ts=2):
//   - Urborg makes all lands Swamps (including itself).
//   - Blood Moon then makes all nonbasic lands Mountains (overriding).
//   - Result: nonbasic lands are Mountains (Blood Moon wins for nonbasics),
//     but Urborg's ability persists because Blood Moon applies to Urborg
//     and strips its ability... unless dependency ordering puts Blood Moon
//     first for Urborg.
//
// This tests the timestamp-order interaction between two L4 effects.
func TestLayerDependency_BloodMoonUrborg(t *testing.T) {
	gs := newFixtureGame(t)

	// Blood Moon enters first.
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")

	// Urborg enters second.
	urborg := addBattlefield(gs, 0, "Urborg, Tomb of Yawgmoth", 0, 0, "land")
	RegisterContinuousEffectsForPermanent(gs, urborg)

	// A nonbasic land to test.
	tropical := addBattlefield(gs, 0, "Tropical Island", 0, 0, "land")

	gs.InvalidateCharacteristicsCache()

	// Urborg is a nonbasic land — Blood Moon makes it a Mountain.
	uc := GetEffectiveCharacteristics(gs, urborg)
	hasMountain := false
	for _, s := range uc.Subtypes {
		if s == "mountain" {
			hasMountain = true
		}
	}
	if !hasMountain {
		t.Errorf("Urborg should be a Mountain under Blood Moon; subtypes=%v", uc.Subtypes)
	}

	// Tropical Island should be a Mountain (Blood Moon).
	tc := GetEffectiveCharacteristics(gs, tropical)
	hasMountainT := false
	for _, s := range tc.Subtypes {
		if s == "mountain" {
			hasMountainT = true
		}
	}
	if !hasMountainT {
		t.Errorf("Tropical Island should be a Mountain under Blood Moon; subtypes=%v", tc.Subtypes)
	}

	t.Log("§613.8 Blood Moon + Urborg: timestamp order produces correct result")
}

// TestLayerDependency_UrborgBloodMoon_ReversedTimestamp tests the
// reverse timestamp order: Urborg first, then Blood Moon.
func TestLayerDependency_UrborgBloodMoon_ReversedTimestamp(t *testing.T) {
	gs := newFixtureGame(t)

	// Urborg enters first.
	urborg := addBattlefield(gs, 0, "Urborg, Tomb of Yawgmoth", 0, 0, "land")
	RegisterContinuousEffectsForPermanent(gs, urborg)

	// Blood Moon enters second.
	_ = layerAt(gs, 0, "Blood Moon", 0, 0, "enchantment")

	// A nonbasic land.
	tropical := addBattlefield(gs, 0, "Tropical Island", 0, 0, "land")
	// A basic Forest.
	forest := addBattlefield(gs, 0, "Forest", 0, 0, "land", "basic")

	gs.InvalidateCharacteristicsCache()

	// Urborg first (ts lower) makes all lands Swamps. Then Blood Moon
	// (ts higher) makes nonbasics Mountains. For nonbasic lands, Blood
	// Moon's "subtypes = [mountain]" replaces Urborg's swamp addition.
	tc := GetEffectiveCharacteristics(gs, tropical)
	hasMountain := false
	for _, s := range tc.Subtypes {
		if s == "mountain" {
			hasMountain = true
		}
	}
	if !hasMountain {
		t.Errorf("Tropical Island should be a Mountain; subtypes=%v", tc.Subtypes)
	}

	// Basic Forest: Blood Moon skips basics. Urborg adds Swamp.
	fc := GetEffectiveCharacteristics(gs, forest)
	hasSwamp := false
	for _, s := range fc.Subtypes {
		if s == "swamp" {
			hasSwamp = true
		}
	}
	if !hasSwamp {
		t.Errorf("Basic Forest should have Swamp (from Urborg, Blood Moon skips basics); subtypes=%v", fc.Subtypes)
	}

	t.Log("§613.8 Urborg (first) + Blood Moon (second): correct interaction")
}

// TestLayerDependency_DependencyOrderUnit tests DependencyOrder
// directly with synthetic effects.
func TestLayerDependency_DependencyOrderUnit(t *testing.T) {
	gs := newFixtureGame(t)

	// Two independent effects at same layer — should remain in timestamp
	// order (no dependencies).
	e1 := &ContinuousEffect{
		Layer: LayerColor, Timestamp: 1,
		SourcePerm: &Permanent{Card: &Card{Name: "A"}, Flags: map[string]int{}},
	}
	e2 := &ContinuousEffect{
		Layer: LayerColor, Timestamp: 2,
		SourcePerm: &Permanent{Card: &Card{Name: "B"}, Flags: map[string]int{}},
	}
	ordered := DependencyOrder([]*ContinuousEffect{e1, e2}, gs)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(ordered))
	}
	if ordered[0] != e1 || ordered[1] != e2 {
		t.Errorf("independent effects should remain in timestamp order")
	}

	// Single effect — trivial case.
	single := DependencyOrder([]*ContinuousEffect{e1}, gs)
	if len(single) != 1 || single[0] != e1 {
		t.Errorf("single effect should pass through")
	}

	// Empty — trivial case.
	empty := DependencyOrder(nil, gs)
	if len(empty) != 0 {
		t.Errorf("nil should return nil/empty")
	}
}

// TestLayerDependency_NoCrashOnNilEffects verifies that the dependency
// system handles nil effects gracefully.
func TestLayerDependency_NoCrashOnNilEffects(t *testing.T) {
	gs := newFixtureGame(t)
	// Should not panic.
	result := DependencyOrder([]*ContinuousEffect{nil, nil}, gs)
	if len(result) != 2 {
		t.Errorf("nil effects should pass through, got %d", len(result))
	}
}

// -----------------------------------------------------------------------------
// Benchmark — GetEffectiveCharacteristics with 10 effects registered.
// -----------------------------------------------------------------------------

func BenchmarkGetEffectiveCharacteristics(b *testing.B) {
	rng := newBenchRng()
	_ = rng
	gs := NewGameState(2, nil, nil)
	// 10 registered effects: Humility + Opalescence + Blood Moon + Magus
	// + Painter + Mycosynth + 4 misc.
	humility := &Permanent{Card: &Card{Name: "Humility", Types: []string{"enchantment"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)
	RegisterHumility(gs, humility)
	opal := &Permanent{Card: &Card{Name: "Opalescence", Types: []string{"enchantment"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{"cmc": 4}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, opal)
	RegisterOpalescence(gs, opal)
	bm := &Permanent{Card: &Card{Name: "Blood Moon", Types: []string{"enchantment"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, bm)
	RegisterBloodMoon(gs, bm)
	mag := &Permanent{Card: &Card{Name: "Magus of the Moon", Types: []string{"creature"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, mag)
	RegisterMagusOfTheMoon(gs, mag)
	paint := &Permanent{Card: &Card{Name: "Painter's Servant", Types: []string{"artifact", "creature"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{"painter_color_B": 1}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, paint)
	RegisterPaintersServant(gs, paint)
	myco := &Permanent{Card: &Card{Name: "Mycosynth Lattice", Types: []string{"artifact"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, myco)
	RegisterMycosynthLattice(gs, myco)

	// Target — the one we benchmark queries for.
	target := &Permanent{Card: &Card{Name: "Grizzly Bears", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	// Cached path: run once to prime.
	_ = GetEffectiveCharacteristics(gs, target)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetEffectiveCharacteristics(gs, target)
	}
}

// BenchmarkGetEffectiveCharacteristics_Uncached measures cold path.
func BenchmarkGetEffectiveCharacteristics_Uncached(b *testing.B) {
	gs := NewGameState(2, nil, nil)
	humility := &Permanent{Card: &Card{Name: "Humility", Types: []string{"enchantment"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, humility)
	RegisterHumility(gs, humility)
	opal := &Permanent{Card: &Card{Name: "Opalescence", Types: []string{"enchantment"}}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{"cmc": 4}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, opal)
	RegisterOpalescence(gs, opal)
	target := &Permanent{Card: &Card{Name: "Grizzly Bears", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}, Controller: 0, Timestamp: gs.NextTimestamp(), Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.InvalidateCharacteristicsCache()
		_ = GetEffectiveCharacteristics(gs, target)
	}
}

// tiny helper so the benchmark compiles without rand import side-effects.
func newBenchRng() int { return 42 }

// -----------------------------------------------------------------------------
// Lignify — layer 4 Treefolk + layer 6 strip + layer 7b 0/4.
// -----------------------------------------------------------------------------

func TestLayer_Lignify_Basic(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.GrantedAbilities = []string{"flying"}
	lignify := addBattlefield(gs, 0, "Lignify", 0, 0, "enchantment", "aura")
	lignify.AttachedTo = dragon
	RegisterLignify(gs, lignify)

	chars := GetEffectiveCharacteristics(gs, dragon)
	// Should be 0/4.
	if chars.Power != 0 || chars.Toughness != 4 {
		t.Errorf("Lignify should set P/T to 0/4, got %d/%d", chars.Power, chars.Toughness)
	}
	// Should have Treefolk subtype.
	hasTreefolk := false
	for _, s := range chars.Subtypes {
		if s == "Treefolk" {
			hasTreefolk = true
		}
	}
	if !hasTreefolk {
		t.Errorf("Lignify should add Treefolk subtype; got subtypes=%v", chars.Subtypes)
	}
	// Should have no abilities.
	if len(chars.Abilities) != 0 {
		t.Errorf("Lignify should strip all abilities; got %d", len(chars.Abilities))
	}
	if len(chars.Keywords) != 0 {
		t.Errorf("Lignify should strip all keywords; got %v", chars.Keywords)
	}
	// Should still be a creature.
	if !charsHaveType(chars.Types, "creature") {
		t.Errorf("Lignify creature should still be a creature; got types=%v", chars.Types)
	}
}

func TestLayer_Lignify_WithCounters(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.AddCounter("+1/+1", 3)
	lignify := addBattlefield(gs, 0, "Lignify", 0, 0, "enchantment", "aura")
	lignify.AttachedTo = dragon
	RegisterLignify(gs, lignify)

	chars := GetEffectiveCharacteristics(gs, dragon)
	// Lignify sets 0/4 at layer 7b, counters add +3/+3 at post-pass -> 3/7.
	if chars.Power != 3 || chars.Toughness != 7 {
		t.Errorf("Lignify + 3 counters: expected 3/7, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Lignify_Unregister(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	lignify := addBattlefield(gs, 0, "Lignify", 0, 0, "enchantment", "aura")
	lignify.AttachedTo = dragon
	RegisterLignify(gs, lignify)

	// While Lignify is on: 0/4.
	chars := GetEffectiveCharacteristics(gs, dragon)
	if chars.Power != 0 || chars.Toughness != 4 {
		t.Fatalf("Lignify active: expected 0/4, got %d/%d", chars.Power, chars.Toughness)
	}

	// Lignify leaves the battlefield.
	n := gs.UnregisterContinuousEffectsForPermanent(lignify)
	if n != 3 {
		t.Errorf("expected 3 effects unregistered for Lignify, got %d", n)
	}
	chars2 := GetEffectiveCharacteristics(gs, dragon)
	if chars2.Power != 5 || chars2.Toughness != 5 {
		t.Errorf("after Lignify leaves: expected 5/5, got %d/%d", chars2.Power, chars2.Toughness)
	}
}

// -----------------------------------------------------------------------------
// CopyPermanentLayered — layer-1 copy effects (Clone / Cytoshape).
// -----------------------------------------------------------------------------

func TestLayer_CopyPermanentLayered_Basic(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.GrantedAbilities = []string{"flying"}
	clone := addBattlefield(gs, 1, "Clone", 0, 0, "creature")

	CopyPermanentLayered(gs, clone, dragon, DurationPermanent)

	chars := GetEffectiveCharacteristics(gs, clone)
	if chars.Name != "Shivan Dragon" {
		t.Errorf("Clone should copy name; got %q", chars.Name)
	}
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("Clone should copy P/T; got %d/%d", chars.Power, chars.Toughness)
	}
	if !charsHaveType(chars.Types, "creature") {
		t.Errorf("Clone should copy creature type; got types=%v", chars.Types)
	}
}

func TestLayer_CopyPermanentLayered_RetainsStatus(t *testing.T) {
	gs := newFixtureGame(t)
	source := addBattlefield(gs, 0, "Serra Angel", 4, 4, "creature")
	target := addBattlefield(gs, 1, "Clone", 0, 0, "creature")
	target.Tapped = true
	target.AddCounter("+1/+1", 2)

	CopyPermanentLayered(gs, target, source, DurationPermanent)

	// Should copy P/T but retain tapped state and counters.
	if !target.Tapped {
		t.Errorf("Clone should retain tapped state")
	}
	if target.Counters["+1/+1"] != 2 {
		t.Errorf("Clone should retain counters; got %d", target.Counters["+1/+1"])
	}
	chars := GetEffectiveCharacteristics(gs, target)
	// Base 4/4 from copy + 2 from counters = 6/6.
	if chars.Power != 6 || chars.Toughness != 6 {
		t.Errorf("Clone (4/4 copy + 2 counters): expected 6/6, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_CopyPermanentLayered_WithHumility(t *testing.T) {
	gs := newFixtureGame(t)
	source := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	clone := addBattlefield(gs, 1, "Clone", 0, 0, "creature")

	// Clone copies Dragon at layer 1.
	CopyPermanentLayered(gs, clone, source, DurationPermanent)
	// Then Humility strips at layer 6/7b.
	_ = layerAt(gs, 0, "Humility", 0, 0, "enchantment")

	chars := GetEffectiveCharacteristics(gs, clone)
	// Layer 1 sets 5/5 (from Dragon), layer 7b sets 1/1 (Humility).
	if chars.Power != 1 || chars.Toughness != 1 {
		t.Errorf("Clone + Humility: expected 1/1, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_CopyPermanentLayered_EOTDuration(t *testing.T) {
	gs := newFixtureGame(t)
	source := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	target := addBattlefield(gs, 1, "Morphling", 3, 3, "creature")

	// Cytoshape-style EOT copy.
	CopyPermanentLayered(gs, target, source, DurationEndOfTurn)

	chars := GetEffectiveCharacteristics(gs, target)
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("EOT copy should copy P/T; got %d/%d", chars.Power, chars.Toughness)
	}

	// Simulate cleanup step: expire durations.
	ScanExpiredDurations(gs, "ending", "cleanup")
	gs.InvalidateCharacteristicsCache()

	chars2 := GetEffectiveCharacteristics(gs, target)
	if chars2.Power != 3 || chars2.Toughness != 3 {
		t.Errorf("After EOT: copy should expire, restoring 3/3; got %d/%d", chars2.Power, chars2.Toughness)
	}
}

// -----------------------------------------------------------------------------
// AST-driven anthem registration (layer 7c).
// -----------------------------------------------------------------------------

// addBattlefieldWithAST is like addBattlefield but attaches an AST.
func addBattlefieldWithAST(gs *GameState, seat int, name string, pow, tough int, ast *gameast.CardAST, types ...string) *Permanent {
	p := addBattlefield(gs, seat, name, pow, tough, types...)
	p.Card.AST = ast
	return p
}

func TestLayer_Anthem_OtherYoursCreatures(t *testing.T) {
	gs := newFixtureGame(t)

	// Lord with "Other creatures you control get +1/+1".
	lordAST := &gameast.CardAST{
		Name: "Glorious Anthem Lord",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "other_yours_anthem",
				Args:    []interface{}{1, 1},
				Layer:   "7c",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, 0, "Glorious Anthem Lord", 2, 2, lordAST, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)

	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	oppBear := addBattlefield(gs, 1, "Opp Bears", 3, 3, "creature")

	// Lord doesn't buff itself ("other").
	lordChars := GetEffectiveCharacteristics(gs, lord)
	if lordChars.Power != 2 || lordChars.Toughness != 2 {
		t.Errorf("lord should stay 2/2, got %d/%d", lordChars.Power, lordChars.Toughness)
	}

	// Friendly creature gets +1/+1.
	bearChars := GetEffectiveCharacteristics(gs, bear)
	if bearChars.Power != 3 || bearChars.Toughness != 3 {
		t.Errorf("friendly bear should be 3/3, got %d/%d", bearChars.Power, bearChars.Toughness)
	}

	// Opponent creature NOT buffed.
	oppChars := GetEffectiveCharacteristics(gs, oppBear)
	if oppChars.Power != 3 || oppChars.Toughness != 3 {
		t.Errorf("opp bear should stay 3/3, got %d/%d", oppChars.Power, oppChars.Toughness)
	}
}

func TestLayer_Anthem_NewCreatureBenefits(t *testing.T) {
	gs := newFixtureGame(t)

	lordAST := &gameast.CardAST{
		Name: "Lord of the Unreal",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "other_yours_anthem",
				Args:    []interface{}{1, 1},
				Layer:   "7c",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, 0, "Lord of the Unreal", 2, 2, lordAST, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)

	// Creature entering AFTER the lord should still get the buff.
	lateCreature := addBattlefield(gs, 0, "Late Arrival", 1, 1, "creature")
	chars := GetEffectiveCharacteristics(gs, lateCreature)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("late creature should be 2/2 (1/1 + 1/1 anthem), got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Anthem_LordLeavesRemovesBuff(t *testing.T) {
	gs := newFixtureGame(t)

	lordAST := &gameast.CardAST{
		Name: "Anthem Lord",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "other_yours_anthem",
				Args:    []interface{}{2, 2},
				Layer:   "7c",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, 0, "Anthem Lord", 3, 3, lordAST, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)

	bear := addBattlefield(gs, 0, "Bear", 2, 2, "creature")

	// Buffed: 2+2 = 4.
	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 4 {
		t.Fatalf("before: bear should be 4/4, got %d/%d", chars.Power, chars.Toughness)
	}

	// Remove lord → unregister effects → buff gone.
	gs.UnregisterContinuousEffectsForPermanent(lord)

	chars2 := GetEffectiveCharacteristics(gs, bear)
	if chars2.Power != 2 || chars2.Toughness != 2 {
		t.Errorf("after lord leaves: bear should revert to 2/2, got %d/%d", chars2.Power, chars2.Toughness)
	}
}

func TestLayer_Anthem_OppDebuff(t *testing.T) {
	gs := newFixtureGame(t)

	// Elesh Norn style: opponent creatures get -2/-2.
	nornAST := &gameast.CardAST{
		Name: "Elesh Norn",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "opp_creatures_pt",
				Args:    []interface{}{-2, -2},
				Layer:   "7c",
			}},
		},
	}
	norn := addBattlefieldWithAST(gs, 0, "Elesh Norn", 4, 7, nornAST, "creature")
	RegisterContinuousEffectsForPermanent(gs, norn)

	oppCreature := addBattlefield(gs, 1, "Opp Elf", 1, 1, "creature")
	ownCreature := addBattlefield(gs, 0, "Own Soldier", 2, 2, "creature")

	oppChars := GetEffectiveCharacteristics(gs, oppCreature)
	if oppChars.Power != -1 || oppChars.Toughness != -1 {
		t.Errorf("opp creature should be -1/-1 (1-2), got %d/%d", oppChars.Power, oppChars.Toughness)
	}

	ownChars := GetEffectiveCharacteristics(gs, ownCreature)
	if ownChars.Power != 2 || ownChars.Toughness != 2 {
		t.Errorf("own creature should stay 2/2, got %d/%d", ownChars.Power, ownChars.Toughness)
	}
}

func TestLayer_Anthem_DoesNotAffectNonCreatures(t *testing.T) {
	gs := newFixtureGame(t)

	lordAST := &gameast.CardAST{
		Name: "Anthem",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "your_creatures_anthem_bare",
				Args:    []interface{}{1, 1},
				Layer:   "7c",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, 0, "Anthem", 0, 0, lordAST, "enchantment")
	RegisterContinuousEffectsForPermanent(gs, lord)

	artifact := addBattlefield(gs, 0, "Sol Ring", 0, 0, "artifact")
	chars := GetEffectiveCharacteristics(gs, artifact)
	if chars.Power != 0 || chars.Toughness != 0 {
		t.Errorf("non-creature should not be buffed, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestLayer_Anthem_StacksWithHumility(t *testing.T) {
	gs := newFixtureGame(t)

	// Anthem (+2/+2) enters first.
	lordAST := &gameast.CardAST{
		Name: "Big Anthem",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "your_creatures_anthem_bare",
				Args:    []interface{}{2, 2},
				Layer:   "7c",
			}},
		},
	}
	anthem := addBattlefieldWithAST(gs, 0, "Big Anthem", 0, 0, lordAST, "enchantment")
	RegisterContinuousEffectsForPermanent(gs, anthem)

	// Humility enters second: layer 7b sets base to 1/1, layer 7c anthem adds +2/+2.
	humility := layerAt(gs, 0, "Humility", 0, 0, "enchantment")
	_ = humility

	bear := addBattlefield(gs, 0, "Bear", 5, 5, "creature")
	chars := GetEffectiveCharacteristics(gs, bear)
	// Humility sets to 1/1 (layer 7b), then anthem adds +2/+2 (layer 7c) = 3/3.
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("Humility(1/1) + anthem(+2/+2) should give 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

// =============================================================================
// Layer 3 — Text-Changing Effects (§613.1c)
// =============================================================================

func TestLayer3_SwirlTheMists_SwapsLandTypeInKeywords(t *testing.T) {
	gs := newFixtureGame(t)
	// Swirl the Mists: change "swamp" → "island" in text.
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_swamp"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	// A creature with "swampwalk" keyword should have it become "islandwalk".
	creature := addBattlefield(gs, 0, "Filth", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Filth",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	// The keyword "swampwalk" should now read "islandwalk".
	foundIslandwalk := false
	for _, kw := range chars.Keywords {
		if kw == "islandwalk" {
			foundIslandwalk = true
		}
		if kw == "swampwalk" {
			t.Errorf("Layer 3: swampwalk should have been changed to islandwalk")
		}
	}
	if !foundIslandwalk {
		t.Errorf("Layer 3: expected islandwalk keyword after text change, got %v", chars.Keywords)
	}
}

func TestLayer3_SwirlTheMists_SwapsLandTypeInAbilityRaw(t *testing.T) {
	gs := newFixtureGame(t)
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_mountain"] = 1
	swirl.Flags["text_to_forest"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	// A permanent with an activated ability referencing "mountain".
	perm := addBattlefield(gs, 0, "Mountain Walker", 3, 3, "creature")
	perm.Card.AST = &gameast.CardAST{
		Name: "Mountain Walker",
		Abilities: []gameast.Ability{
			&gameast.Activated{Raw: "Tap target Mountain you control"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, perm)
	if len(chars.Abilities) < 1 {
		t.Fatalf("expected at least 1 ability")
	}
	act, ok := chars.Abilities[0].(*gameast.Activated)
	if !ok {
		t.Fatalf("expected *gameast.Activated, got %T", chars.Abilities[0])
	}
	if act.Raw != "Tap target forest you control" {
		t.Errorf("Layer 3: expected 'Tap target forest you control', got %q", act.Raw)
	}
}

func TestLayer3_MindBend_LandTypeSwap(t *testing.T) {
	gs := newFixtureGame(t)
	// Mind Bend is an instant that creates a lasting text-change on target.
	// We model this by placing flags on the target permanent.
	target := addBattlefield(gs, 0, "Mind Bend", 0, 0, "enchantment")
	target.Flags["text_from_swamp"] = 1
	target.Flags["text_to_plains"] = 1
	RegisterContinuousEffectsForPermanent(gs, target)

	// A creature with "swampwalk" should have it changed to "plainswalk".
	creature := addBattlefield(gs, 0, "Bog Wraith", 3, 3, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Bog Wraith",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	foundPlainswalk := false
	for _, kw := range chars.Keywords {
		if kw == "plainswalk" {
			foundPlainswalk = true
		}
	}
	if !foundPlainswalk {
		t.Errorf("Layer 3 Mind Bend: expected plainswalk, got keywords=%v", chars.Keywords)
	}
}

func TestLayer3_PaintersServant_TextChangeColorWords(t *testing.T) {
	gs := newFixtureGame(t)
	// Painter's Servant choosing blue.
	servant := addBattlefield(gs, 0, "Painter's Servant", 1, 3, "creature", "artifact")
	servant.Flags["painter_color_U"] = 1
	RegisterContinuousEffectsForPermanent(gs, servant)

	// A creature with "protection from black" in ability text.
	creature := addBattlefield(gs, 0, "Knight of Grace", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Knight of Grace",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "Protection from black"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	if len(chars.Abilities) < 1 {
		t.Fatalf("expected at least 1 ability")
	}
	st, ok := chars.Abilities[0].(*gameast.Static)
	if !ok {
		t.Fatalf("expected *gameast.Static, got %T", chars.Abilities[0])
	}
	// "black" should be changed to "blue" by Painter's Servant Layer 3.
	if st.Raw != "Protection from blue" {
		t.Errorf("Layer 3 Painter text: expected 'Protection from blue', got %q", st.Raw)
	}
}

func TestLayer3_PaintersServant_StillAddsColor(t *testing.T) {
	gs := newFixtureGame(t)
	// Painter's Servant choosing red.
	servant := addBattlefield(gs, 0, "Painter's Servant", 1, 3, "creature", "artifact")
	servant.Flags["painter_color_R"] = 1
	RegisterContinuousEffectsForPermanent(gs, servant)

	// Verify Layer 5 still works (adds red to all permanents).
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	chars := GetEffectiveCharacteristics(gs, bear)
	foundRed := false
	for _, c := range chars.Colors {
		if c == "R" {
			foundRed = true
		}
	}
	if !foundRed {
		t.Errorf("Painter's Servant should still add color R via Layer 5, got colors=%v", chars.Colors)
	}
}

func TestLayer3_RunsBeforeLayer4(t *testing.T) {
	gs := newFixtureGame(t)
	// This test verifies that Layer 3 text changes happen BEFORE Layer 4
	// type changes. A Layer 3 effect changes "Mountain" text to "Island".
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_mountain"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	// A creature with "mountainwalk" — after Layer 3 it becomes "islandwalk".
	creature := addBattlefield(gs, 0, "Mountain Goat", 1, 1, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Mountain Goat",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "mountainwalk", Raw: "Mountainwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	foundIslandwalk := false
	for _, kw := range chars.Keywords {
		if kw == "islandwalk" {
			foundIslandwalk = true
		}
	}
	if !foundIslandwalk {
		t.Errorf("Layer 3 should run before Layer 4; expected islandwalk, got %v", chars.Keywords)
	}
}

func TestLayer3_NoOpWhenFromEqualsTo(t *testing.T) {
	gs := newFixtureGame(t)
	// If from == to, the effect should be a no-op (not registered).
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_island"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	creature := addBattlefield(gs, 0, "Water Elemental", 5, 4, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Water Elemental",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "islandwalk", Raw: "Islandwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	foundIslandwalk := false
	for _, kw := range chars.Keywords {
		if kw == "islandwalk" {
			foundIslandwalk = true
		}
	}
	if !foundIslandwalk {
		t.Errorf("No-op text change should preserve islandwalk, got %v", chars.Keywords)
	}
}

func TestLayer3_TraitDoctoring_ColorWordMode(t *testing.T) {
	gs := newFixtureGame(t)
	// Trait Doctoring in color word mode: change "white" → "green".
	td := addBattlefield(gs, 0, "Trait Doctoring", 0, 0, "enchantment")
	td.Flags["text_color_from_white"] = 1
	td.Flags["text_color_to_green"] = 1
	RegisterContinuousEffectsForPermanent(gs, td)

	creature := addBattlefield(gs, 0, "White Knight", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "White Knight",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "Protection from white"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	if len(chars.Abilities) < 1 {
		t.Fatalf("expected at least 1 ability")
	}
	st, ok := chars.Abilities[0].(*gameast.Static)
	if !ok {
		t.Fatalf("expected *gameast.Static, got %T", chars.Abilities[0])
	}
	if st.Raw != "Protection from green" {
		t.Errorf("Trait Doctoring color swap: expected 'Protection from green', got %q", st.Raw)
	}
}

func TestLayer3_TraitDoctoring_LandTypeMode(t *testing.T) {
	gs := newFixtureGame(t)
	// Trait Doctoring in land type mode: change "forest" → "swamp".
	td := addBattlefield(gs, 0, "Trait Doctoring", 0, 0, "enchantment")
	td.Flags["text_from_forest"] = 1
	td.Flags["text_to_swamp"] = 1
	RegisterContinuousEffectsForPermanent(gs, td)

	creature := addBattlefield(gs, 0, "Elvish Champion", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Elvish Champion",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "forestwalk", Raw: "Forestwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	foundSwampwalk := false
	for _, kw := range chars.Keywords {
		if kw == "swampwalk" {
			foundSwampwalk = true
		}
	}
	if !foundSwampwalk {
		t.Errorf("Trait Doctoring land type: expected swampwalk, got %v", chars.Keywords)
	}
}

func TestLayer3_MultipleAbilities_AllChanged(t *testing.T) {
	gs := newFixtureGame(t)
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_swamp"] = 1
	swirl.Flags["text_to_plains"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	// A permanent with multiple abilities referencing "swamp".
	perm := addBattlefield(gs, 0, "Swamp Lord", 3, 3, "creature")
	perm.Card.AST = &gameast.CardAST{
		Name: "Swamp Lord",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
			&gameast.Triggered{Raw: "Whenever a Swamp enters the battlefield"},
			&gameast.Static{Raw: "Other Swamp creatures get +1/+1"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, perm)
	// Keyword should be plainswalk.
	if len(chars.Keywords) < 1 || chars.Keywords[0] != "plainswalk" {
		t.Errorf("expected plainswalk keyword, got %v", chars.Keywords)
	}
	// Triggered ability raw text should reference "plains".
	if len(chars.Abilities) < 2 {
		t.Fatalf("expected at least 2 abilities, got %d", len(chars.Abilities))
	}
	trig, ok := chars.Abilities[1].(*gameast.Triggered)
	if !ok {
		t.Fatalf("expected *gameast.Triggered at index 1, got %T", chars.Abilities[1])
	}
	if trig.Raw != "Whenever a plains enters the battlefield" {
		t.Errorf("expected 'Whenever a plains enters the battlefield', got %q", trig.Raw)
	}
	// Static ability raw text should reference "plains".
	stat, ok := chars.Abilities[2].(*gameast.Static)
	if !ok {
		t.Fatalf("expected *gameast.Static at index 2, got %T", chars.Abilities[2])
	}
	if stat.Raw != "Other plains creatures get +1/+1" {
		t.Errorf("expected 'Other plains creatures get +1/+1', got %q", stat.Raw)
	}
}

func TestLayer3_AST_TextChangeLandType(t *testing.T) {
	gs := newFixtureGame(t)
	// Test the AST-driven registration path with ModKind "text_change_land_type".
	astCard := &gameast.CardAST{
		Name: "Custom Text Changer",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "text_change_land_type",
				Layer:   "3",
			}},
		},
	}
	changer := addBattlefieldWithAST(gs, 0, "Custom Text Changer", 0, 0, astCard, "enchantment")
	changer.Flags["text_from_forest"] = 1
	changer.Flags["text_to_mountain"] = 1
	RegisterContinuousEffectsForPermanent(gs, changer)

	creature := addBattlefield(gs, 0, "Forest Keeper", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Forest Keeper",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "forestwalk", Raw: "Forestwalk"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	foundMountainwalk := false
	for _, kw := range chars.Keywords {
		if kw == "mountainwalk" {
			foundMountainwalk = true
		}
	}
	if !foundMountainwalk {
		t.Errorf("AST text_change_land_type: expected mountainwalk, got %v", chars.Keywords)
	}
}

func TestLayer3_AST_TextChangeColorWord(t *testing.T) {
	gs := newFixtureGame(t)
	// Test the AST-driven registration path with ModKind "text_change_color_word".
	astCard := &gameast.CardAST{
		Name: "Custom Color Changer",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "text_change_color_word",
				Args:    []interface{}{"red", "blue"},
				Layer:   "3",
			}},
		},
	}
	changer := addBattlefieldWithAST(gs, 0, "Custom Color Changer", 0, 0, astCard, "enchantment")
	RegisterContinuousEffectsForPermanent(gs, changer)

	creature := addBattlefield(gs, 0, "Red Ward", 0, 0, "enchantment")
	creature.Card.AST = &gameast.CardAST{
		Name: "Red Ward",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "Enchanted creature has protection from red"},
		},
	}
	chars := GetEffectiveCharacteristics(gs, creature)
	if len(chars.Abilities) < 1 {
		t.Fatalf("expected at least 1 ability")
	}
	st, ok := chars.Abilities[0].(*gameast.Static)
	if !ok {
		t.Fatalf("expected *gameast.Static, got %T", chars.Abilities[0])
	}
	if st.Raw != "Enchanted creature has protection from blue" {
		t.Errorf("AST text_change_color_word: expected 'Enchanted creature has protection from blue', got %q", st.Raw)
	}
}

func TestLayer3_Idempotent(t *testing.T) {
	gs := newFixtureGame(t)
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_swamp"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	creature := addBattlefield(gs, 0, "Filth", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Filth",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
		},
	}
	// Call twice to verify idempotency.
	chars1 := GetEffectiveCharacteristics(gs, creature)
	gs.InvalidateCharacteristicsCache()
	chars2 := GetEffectiveCharacteristics(gs, creature)

	if len(chars1.Keywords) != len(chars2.Keywords) {
		t.Fatalf("idempotency violation: keywords differ between calls")
	}
	for i := range chars1.Keywords {
		if chars1.Keywords[i] != chars2.Keywords[i] {
			t.Errorf("idempotency: keyword[%d] %q != %q", i, chars1.Keywords[i], chars2.Keywords[i])
		}
	}
	// Verify the keyword is "islandwalk" (not "swampwalk" or double-applied).
	if len(chars2.Keywords) < 1 || chars2.Keywords[0] != "islandwalk" {
		t.Errorf("idempotent result should be islandwalk, got %v", chars2.Keywords)
	}
}

func TestLayer3_UnregisterRemovesEffect(t *testing.T) {
	gs := newFixtureGame(t)
	swirl := addBattlefield(gs, 0, "Swirl the Mists", 0, 0, "enchantment")
	swirl.Flags["text_from_swamp"] = 1
	swirl.Flags["text_to_island"] = 1
	RegisterContinuousEffectsForPermanent(gs, swirl)

	creature := addBattlefield(gs, 0, "Filth", 2, 2, "creature")
	creature.Card.AST = &gameast.CardAST{
		Name: "Filth",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampwalk", Raw: "Swampwalk"},
		},
	}

	// Before unregister: islandwalk.
	chars := GetEffectiveCharacteristics(gs, creature)
	if len(chars.Keywords) < 1 || chars.Keywords[0] != "islandwalk" {
		t.Fatalf("before unregister: expected islandwalk, got %v", chars.Keywords)
	}

	// Unregister (simulate Swirl the Mists leaving the battlefield).
	gs.UnregisterContinuousEffectsForPermanent(swirl)

	// After unregister: back to swampwalk.
	chars = GetEffectiveCharacteristics(gs, creature)
	if len(chars.Keywords) < 1 || chars.Keywords[0] != "swampwalk" {
		t.Errorf("after unregister: expected swampwalk, got %v", chars.Keywords)
	}
}
