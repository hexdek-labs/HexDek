package gameengine

import (
	"strings"
	"testing"
)

// cr_613_layer_property_r60_test extends the §400.7c / §108.4 /
// §400.7 / §702.91 property-test moat into CR §613, the continuous-
// effect layer system. §613 defines the 7-layer application order:
//
//	1 copy effects
//	2 control effects
//	3 text-changing effects
//	4 type-changing effects
//	5 color-changing effects
//	6 ability-adding-or-removing effects
//	7 power/toughness effects (sub-layers 7a/7b/7c/7d)
//
// Property: for every per_card handler that emits a continuous
// effect, the registered ContinuousEffect must land at the correct
// layer per §613.1, and the application order must respect the
// layer-ascending + timestamp-tiebreak semantics in §613.7.
//
// Catches layer-ordering bugs like the classic Blood Moon / Urborg
// pair: when Urborg enters before Blood Moon, the application
// order must produce nonbasic Mountains (Blood Moon's later
// timestamp wins at Layer 4); when Blood Moon enters first, Urborg
// applies on top and produces nonbasic Mountains AND Swamps. The
// engine's Layer 4 sort must be timestamp-monotonic — a regression
// where Blood Moon applies BEFORE Urborg regardless of timestamp
// would break the canonical "later effect wins same-layer" rule.

// ────────────────────────────────────────────────────────────────
// (A) Schema check — every registered ContinuousEffect must
// satisfy the structural CR §613 contract.
// ────────────────────────────────────────────────────────────────

// TestCR613_ContinuousEffectSchemaContract verifies that every
// ContinuousEffect registered by the engine's named-card handlers
// satisfies the §613 schema:
//   - Layer ∈ {1, 2, 3, 4, 5, 6, 7}
//   - Layer 7 effects MUST carry a Sublayer in {"a", "b", "c",
//     "d", "e"} (face-down characteristic-defining is "a" with
//     7a semantics — MVP collapses 7a-7e to non-empty sublayer
//     existence)
//   - Layers 1-6 must have empty Sublayer (sublayers are only
//     meaningful at Layer 7 in standard MTG comp rules)
//   - Timestamp > 0 (every registration produces a fresh
//     timestamp via gs.NextTimestamp)
//   - HandlerID non-empty (idempotency key)
func TestCR613_ContinuousEffectSchemaContract(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Register the canonical layer-active named cards. Each call
	// is the live engine path — exercising what the per_card
	// handler does at registration time.
	registerEachNamedHandler(t, gs)

	if len(gs.ContinuousEffects) == 0 {
		t.Fatal("no ContinuousEffects registered — schema check has nothing to test")
	}
	for i, ce := range gs.ContinuousEffects {
		if ce.Layer < 1 || ce.Layer > 7 {
			t.Errorf("effect #%d (source=%q): Layer %d out of CR §613 range [1,7]",
				i, ce.SourceCardName, ce.Layer)
		}
		if ce.Layer == 7 {
			if ce.Sublayer == "" {
				t.Errorf("effect #%d (source=%q): Layer 7 effect missing Sublayer (CR §613.4 requires 7a/7b/7c/7d/7e)",
					i, ce.SourceCardName)
			} else if !validSublayer(ce.Sublayer) {
				t.Errorf("effect #%d (source=%q): Layer 7 Sublayer %q not in {a,b,c,d,e}",
					i, ce.SourceCardName, ce.Sublayer)
			}
		} else if ce.Sublayer != "" {
			t.Errorf("effect #%d (source=%q): Layer %d should not carry a Sublayer (got %q) — sublayers are Layer 7 only in standard MTG",
				i, ce.SourceCardName, ce.Layer, ce.Sublayer)
		}
		if ce.Timestamp <= 0 {
			t.Errorf("effect #%d (source=%q): Timestamp %d <= 0 — every registration must mint a fresh positive timestamp",
				i, ce.SourceCardName, ce.Timestamp)
		}
		if strings.TrimSpace(ce.HandlerID) == "" {
			t.Errorf("effect #%d (source=%q): empty HandlerID — required for idempotent re-registration",
				i, ce.SourceCardName)
		}
	}
}

// validSublayer returns true if s is one of the canonical Layer-7
// sublayers per CR §613.4.
func validSublayer(s string) bool {
	switch s {
	case "a", "b", "c", "d", "e":
		return true
	}
	return false
}

// ────────────────────────────────────────────────────────────────
// (B) Layer-classification check — known handlers land at the
// CR-canonical layer for what they DO.
// ────────────────────────────────────────────────────────────────

// expectedLayerForHandler is the curated table mapping a per_card
// handler's effect kind to its canonical CR §613 layer. Entries
// keyed by the SourceCardName the handler stamps on the registered
// effect (case-insensitive lookup).
//
// Layer reference:
//
//	Blood Moon       — Layer 4 (changes land subtype) + Layer 6
//	                   (strips printed land abilities)
//	Magus of the Moon — same as Blood Moon, ability source
//	Urborg           — Layer 4 (adds Swamp subtype to all lands)
//	Humility         — Layer 6 (removes abilities from creatures)
//	                   + Layer 7b (sets P/T to 1/1)
//	Opalescence      — Layer 4 (makes enchantments creatures) +
//	                   Layer 7b (sets P/T = CMC/CMC)
//	Painter's Servant — Layer 5 (colors all cards everywhere)
//	Mycosynth Lattice — Layer 4 (type → artifact) + Layer 5
//	                    (color → all colors)
//	March of the Machines — Layer 4 (noncreature artifacts
//	                        become creatures) + Layer 7b (P/T)
//
// The table lists the SET of layers a handler is expected to
// register at — order doesn't matter, but every layer listed
// must appear among the SourcePerm's registered effects, and no
// unlisted layer should appear.
var expectedLayerForHandler = map[string][]int{
	"blood moon":           {LayerType, LayerAbility},
	"magus of the moon":    {LayerType, LayerAbility},
	"urborg, tomb of yawgmoth": {LayerType},
	"humility":             {LayerAbility, LayerPT},
	"opalescence":          {LayerType, LayerPT},
	// Painter's Servant: Layer 5 (color) is the headline effect.
	// The engine also registers a Layer 3 (text-change) effect to
	// implement the unusual "applies to cards in ALL zones, not
	// just battlefield" CR carve-out — the chosen color text gets
	// added to non-battlefield card characteristics, which is a
	// text-change operation in §613 terms. Both layers are
	// CR-canonical for this card.
	"painter's servant":    {LayerText, LayerColor},
	"mycosynth lattice":    {LayerType, LayerColor},
	"march of the machines": {LayerType, LayerPT},
}

// TestCR613_LayerClassificationByHandler verifies each curated
// handler registers effects at the CR §613-correct layers. A
// Type-changing effect that lands at Layer 6 instead of Layer 4
// would be a layer-misclassification bug that breaks the
// dependency-aware application order.
func TestCR613_LayerClassificationByHandler(t *testing.T) {
	for handlerName, expectLayers := range expectedLayerForHandler {
		t.Run(handlerName, func(t *testing.T) {
			gs := NewGameState(2, nil, nil)
			p := registerOneHandlerByName(t, gs, handlerName)
			if p == nil {
				t.Fatalf("registration of %q did not produce a permanent", handlerName)
			}
			got := layersUsedBySource(gs, p)
			if !sameLayerSet(got, expectLayers) {
				t.Errorf("handler %q: expected layers %v, got %v (CR §613 classification mismatch)",
					handlerName, expectLayers, got)
			}
		})
	}
}

// layersUsedBySource returns the set of distinct Layer values
// across every ContinuousEffect whose SourcePerm == p.
func layersUsedBySource(gs *GameState, p *Permanent) []int {
	seen := map[int]bool{}
	var out []int
	for _, ce := range gs.ContinuousEffects {
		if ce.SourcePerm == p {
			if !seen[ce.Layer] {
				seen[ce.Layer] = true
				out = append(out, ce.Layer)
			}
		}
	}
	return out
}

// sameLayerSet returns true if `a` and `b` contain the same set
// of integers (order-independent).
func sameLayerSet(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int]bool{}
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		if !seen[v] {
			return false
		}
	}
	return true
}

// ────────────────────────────────────────────────────────────────
// (C) Application-order check — the classic Blood Moon / Urborg
// regression case.
// ────────────────────────────────────────────────────────────────

// TestCR613_BloodMoonUrborgTimestampOrdering verifies the classic
// layer-ordering case: when Urborg enters BEFORE Blood Moon, all
// lands become Swamps first (Urborg ts=1, Layer 4), then nonbasic
// Swamps become Mountains (Blood Moon ts=2, Layer 4 — later
// timestamp at the same layer wins per §613.7). When Blood Moon
// enters first, nonbasics become Mountains first, then Urborg
// adds Swamp on top of every land. Both orders must produce the
// CR-correct final state.
//
// This is the canonical regression case for §613.7 timestamp-
// monotonic application: a bug where Blood Moon applies before
// Urborg regardless of timestamp would produce Swamps on nonbasics
// in BOTH orderings, masking the correct "Mountain only" outcome
// when Blood Moon's later timestamp should win.
func TestCR613_BloodMoonUrborgTimestampOrdering(t *testing.T) {
	t.Run("urborg_then_blood_moon", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		// Urborg first (lower timestamp)
		urborg := addPerm(gs, 0, "Urborg, Tomb of Yawgmoth", []string{"land"})
		RegisterUrborg(gs, urborg)
		// Blood Moon second (higher timestamp)
		bm := addPerm(gs, 0, "Blood Moon", []string{"enchantment"})
		RegisterBloodMoon(gs, bm)
		// A nonbasic land target
		mishras := addPerm(gs, 0, "Mishra's Factory", []string{"land"})
		chars := GetEffectiveCharacteristics(gs, mishras)
		// Expected: Mountain ONLY (Blood Moon's later Layer 4
		// timestamp strips Urborg's Swamp from nonbasics).
		if !hasSubtype(chars.Subtypes, "mountain") {
			t.Errorf("Mishra's Factory: expected 'mountain' subtype after Urborg→BloodMoon, got %v", chars.Subtypes)
		}
		if hasSubtype(chars.Subtypes, "swamp") {
			t.Errorf("Mishra's Factory: 'swamp' subtype should NOT remain on nonbasic after Blood Moon overrides Urborg, got %v", chars.Subtypes)
		}
	})

	t.Run("blood_moon_then_urborg", func(t *testing.T) {
		gs := NewGameState(2, nil, nil)
		// Blood Moon first
		bm := addPerm(gs, 0, "Blood Moon", []string{"enchantment"})
		RegisterBloodMoon(gs, bm)
		// Urborg second
		urborg := addPerm(gs, 0, "Urborg, Tomb of Yawgmoth", []string{"land"})
		RegisterUrborg(gs, urborg)
		// Nonbasic target
		mishras := addPerm(gs, 0, "Mishra's Factory", []string{"land"})
		chars := GetEffectiveCharacteristics(gs, mishras)
		// Expected: Mountain AND Swamp (Urborg's later timestamp
		// adds Swamp on top of Blood Moon's Mountain — both layer
		// 4 effects apply additively, later wins on ordering but
		// Urborg ADDS rather than REPLACES like Blood Moon does).
		if !hasSubtype(chars.Subtypes, "mountain") {
			t.Errorf("Mishra's Factory: expected 'mountain' from Blood Moon under BloodMoon→Urborg, got %v", chars.Subtypes)
		}
		if !hasSubtype(chars.Subtypes, "swamp") {
			t.Errorf("Mishra's Factory: expected 'swamp' from Urborg under BloodMoon→Urborg, got %v", chars.Subtypes)
		}
	})
}

// hasSubtype is a case-insensitive lookup on the chars.Subtypes
// slice (subtype storage isn't lowercased internally; this lets
// the assertion stay readable).
func hasSubtype(subtypes []string, want string) bool {
	for _, s := range subtypes {
		if strings.EqualFold(s, want) {
			return true
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────────
// (D) Per-source idempotency check.
// ────────────────────────────────────────────────────────────────

// TestCR613_HandlerIDsAreUniquePerEffect verifies that every
// ContinuousEffect carries a HandlerID and that two re-
// registrations of the SAME source permanent at the SAME layer
// don't produce duplicate effects. The HandlerID is the
// idempotency key per layers.go RegisterContinuousEffect — a
// regression where re-registration appends rather than dedupes
// would inflate gs.ContinuousEffects on every cache invalidation
// pass.
func TestCR613_HandlerIDsAreUniquePerEffect(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	bm := addPerm(gs, 0, "Blood Moon", []string{"enchantment"})
	RegisterBloodMoon(gs, bm)
	beforeCount := len(gs.ContinuousEffects)
	// Re-register against the same permanent — should be a no-op
	// because the HandlerID is keyed on (cardName, permanent).
	RegisterBloodMoon(gs, bm)
	afterCount := len(gs.ContinuousEffects)
	if afterCount != beforeCount {
		t.Errorf("RegisterBloodMoon re-registration: count %d -> %d, expected idempotent no-op (HandlerID dedup)",
			beforeCount, afterCount)
	}
}

// ────────────────────────────────────────────────────────────────
// Test fixture helpers
// ────────────────────────────────────────────────────────────────

// registerEachNamedHandler registers every curated layer-active
// per_card handler against `gs`, returning nothing — callers
// inspect gs.ContinuousEffects after this returns.
func registerEachNamedHandler(t *testing.T, gs *GameState) {
	t.Helper()
	for handlerName := range expectedLayerForHandler {
		registerOneHandlerByName(t, gs, handlerName)
	}
}

// registerOneHandlerByName dispatches to the named handler's
// registration function and returns the permanent it registered.
// Unknown names skip silently (defends against future additions
// to expectedLayerForHandler that haven't grown dispatch entries
// yet — the schema test still runs against everything that DID
// register).
func registerOneHandlerByName(t *testing.T, gs *GameState, name string) *Permanent {
	t.Helper()
	switch strings.ToLower(name) {
	case "blood moon":
		p := addPerm(gs, 0, "Blood Moon", []string{"enchantment"})
		RegisterBloodMoon(gs, p)
		return p
	case "magus of the moon":
		p := addPerm(gs, 0, "Magus of the Moon", []string{"creature"})
		RegisterMagusOfTheMoon(gs, p)
		return p
	case "urborg, tomb of yawgmoth":
		p := addPerm(gs, 0, "Urborg, Tomb of Yawgmoth", []string{"land"})
		RegisterUrborg(gs, p)
		return p
	case "humility":
		p := addPerm(gs, 0, "Humility", []string{"enchantment"})
		RegisterHumility(gs, p)
		return p
	case "opalescence":
		p := addPerm(gs, 0, "Opalescence", []string{"enchantment"})
		p.Flags["cmc"] = 4
		RegisterOpalescence(gs, p)
		return p
	case "painter's servant":
		p := addPerm(gs, 0, "Painter's Servant", []string{"artifact", "creature"})
		p.Flags["painter_color_B"] = 1
		RegisterPaintersServant(gs, p)
		return p
	case "mycosynth lattice":
		p := addPerm(gs, 0, "Mycosynth Lattice", []string{"artifact"})
		RegisterMycosynthLattice(gs, p)
		return p
	case "march of the machines":
		p := addPerm(gs, 0, "March of the Machines", []string{"enchantment"})
		RegisterMarchOfTheMachines(gs, p)
		return p
	}
	return nil
}

// addPerm creates + battlefields a Permanent with a Card backing
// it. Mirrors the bench-test pattern at layers_test.go:891-908 —
// minimal allocation, freshly minted timestamp.
func addPerm(gs *GameState, seat int, name string, types []string) *Permanent {
	p := &Permanent{
		Card:       &Card{Name: name, Types: types},
		Controller: seat,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}
