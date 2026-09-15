package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Disturb cast helper tests — CR §702.146
// ---------------------------------------------------------------------------

func newDisturbGame(t *testing.T) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(43))
	gs := NewGameState(2, rng, nil)
	gs.Active = 0
	gs.Step = "precombat_main"
	return gs
}

// newDisturbCard builds a transforming DFC with disturb on the
// front face and an enchantment-creature back face (mirrors real
// printed Disturb cards like Lunarch Veteran // Luminous Phantom).
//
//   - Front face: creature with the printed disturb keyword.
//   - Back face: enchantment-creature (typically Spirit). Stored in
//     BackFaceAST + BackFaceName + BackFaceTypes per the existing
//     DFC plumbing in state.go.
func newDisturbCard(name, backName string, owner int, disturbCost string,
	backIsInstantSpeed bool) *Card {
	frontAST := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "disturb", Args: []any{disturbCost}},
		},
	}
	backAST := &gameast.CardAST{
		Name:      backName,
		Abilities: []gameast.Ability{},
	}
	backTypes := []string{"creature", "enchantment", "spirit"}
	if backIsInstantSpeed {
		// Not realistic for printed Disturb (all back-faces are
		// enchantment-creature permanents in MID/VOW), but parameterised
		// for the "(b) instant speed" task requirement.
		backTypes = []string{"instant"}
	}
	return &Card{
		Name:          name,
		Owner:         owner,
		Types:         []string{"creature"},
		CMC:           3,
		BasePower:     1,
		BaseToughness: 1,
		AST:           frontAST,
		BackFaceName:  backName,
		BackFaceTypes: backTypes,
		// Forward the back-face AST to CastWithDisturb via the
		// Card.Meta side channel — the cast helper copies it into
		// CostMeta so the stack.go disturb-resolve hook can wire
		// perm.FrontFaceAST / perm.BackFaceAST before
		// ApplyDisturbETB runs. Corpus-loaded cards arrange the
		// same wiring via the loader's per-face cache.
		Meta: map[string]any{
			"disturb_back_face_ast": backAST,
		},
	}
}

func newDisturbPlainSorcery(name string, owner int) *Card {
	return &Card{
		Name:  name,
		Owner: owner,
		Types: []string{"sorcery"},
		AST:   &gameast.CardAST{Name: name},
	}
}

// ---------------------------------------------------------------------------
// (c) HasDisturb detector
// ---------------------------------------------------------------------------

func TestHasDisturb_Detects(t *testing.T) {
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	if !HasDisturb(card) {
		t.Fatal("HasDisturb should be true for a disturb card")
	}
}

func TestHasDisturb_Negative(t *testing.T) {
	if HasDisturb(nil) {
		t.Fatal("HasDisturb(nil) should be false")
	}
	plain := newDisturbPlainSorcery("Plain Sorcery", 0)
	if HasDisturb(plain) {
		t.Fatal("HasDisturb should be false for a card without the keyword")
	}
}

func TestDisturbCost_ParsesManaString(t *testing.T) {
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	if got := DisturbCost(card); got != 2 {
		t.Fatalf("DisturbCost = %d, want 2 (for {1}{W})", got)
	}
}

// ---------------------------------------------------------------------------
// (a) Cast from grave succeeds for disturb back-face card
// ---------------------------------------------------------------------------

func TestCastWithDisturb_SucceedsFromGraveyard(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithDisturb(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithDisturb returned error: %v", err)
	}
	if len(gs.Stack) != 1 {
		t.Fatalf("expected 1 stack item, got %d", len(gs.Stack))
	}
	// Card removed from graveyard.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("disturb-cast spell should not still be in graveyard")
		}
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("mana left = %d, want 3 (paid 2 of 5)", gs.Seats[0].ManaPool)
	}
	if !SpellDisturbedThisTurn(gs, 0) {
		t.Fatal("SpellDisturbedThisTurn should be true after a successful disturb cast")
	}
}

// ---------------------------------------------------------------------------
// (b) Cast at instant speed allowed (CastWithDisturb doesn't gate by phase)
// ---------------------------------------------------------------------------

func TestCastWithDisturb_AllowsCastDuringNonMainPhase(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	gs.Step = "combat_declare_attackers" // not a main phase
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	// CastWithDisturb itself doesn't impose sorcery-speed — the back
	// face's card type drives timing (instant back-face → instant
	// speed; sorcery back-face → sorcery speed). The upstream cast
	// pipeline is responsible for the type-based check; the helper
	// here cleanly allows the cast regardless of phase.
	if _, err := CastWithDisturb(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithDisturb during combat phase failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// (d) CostMeta stamped correctly
// ---------------------------------------------------------------------------

func TestCastWithDisturb_StampsCostMeta(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithDisturb(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithDisturb failed: %v", err)
	}
	item := gs.Stack[0]
	if item.Card != card {
		t.Fatal("stack item should reference the disturb-cast card")
	}
	if item.CastZone != ZoneGraveyard {
		t.Fatalf("CastZone = %q, want %q", item.CastZone, ZoneGraveyard)
	}
	if v, ok := item.CostMeta["disturb_cast"]; !ok || v != true {
		t.Fatalf("CostMeta[\"disturb_cast\"] = %v, want true", item.CostMeta["disturb_cast"])
	}
	if v, ok := item.CostMeta["disturb_cost"]; !ok || v != 2 {
		t.Fatalf("CostMeta[\"disturb_cost\"] = %v, want 2", item.CostMeta["disturb_cost"])
	}
	if v, ok := item.CostMeta["zone_cast_keyword"]; !ok || v != "disturb" {
		t.Fatalf("CostMeta[\"zone_cast_keyword\"] = %v, want \"disturb\"", item.CostMeta["zone_cast_keyword"])
	}
	if !IsDisturbCast(item) {
		t.Fatal("IsDisturbCast should be true for a disturb-cast stack item")
	}
	// Disturb does NOT use exile_on_resolve — back face is a
	// permanent that goes to the battlefield. The §702.146b "exile
	// instead of graveyard" replacement fires later when the
	// disturbed permanent would die.
	if ShouldExileOnResolve(item) {
		t.Fatal("disturb cast must not stamp exile_on_resolve (back face goes to battlefield, not exile)")
	}
}

// ---------------------------------------------------------------------------
// (e) On resolve: back face transforms onto battlefield with dies→exile
//     replacement registered (per actual §702.146b).
// ---------------------------------------------------------------------------
//
// Note: the round-35 task spec asked for "on resolve goes to exile" —
// that's not what §702.146 actually says. The back face is typically
// an enchantment-creature (Spirit) permanent; it resolves onto the
// battlefield transformed. The exile happens LATER when that
// disturbed permanent would die. Verified here.

func TestCastWithDisturb_ResolveEntersBattlefieldTransformed(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithDisturb(gs, 0, card, 2); err != nil {
		t.Fatalf("CastWithDisturb failed: %v", err)
	}
	ResolveStackTop(gs)

	// Spell off graveyard, on battlefield, transformed.
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			t.Fatal("disturb-cast spell should be on the battlefield, not graveyard")
		}
	}
	var resolvedPerm *Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card == card {
			resolvedPerm = p
			break
		}
	}
	if resolvedPerm == nil {
		t.Fatal("disturb-cast spell should resolve as a permanent on the battlefield")
	}
	if !resolvedPerm.Transformed {
		t.Fatal("disturb-cast permanent must enter transformed (back face up)")
	}
	if resolvedPerm.Flags["disturbed"] != 1 {
		t.Fatal("disturb-cast permanent should carry the disturbed flag")
	}
	// Dies→exile replacement registered (CR §702.146b).
	foundReplacement := false
	for _, r := range gs.Replacements {
		if r != nil && r.SourcePerm == resolvedPerm &&
			r.EventType == "would_change_zone" {
			foundReplacement = true
			break
		}
	}
	if !foundReplacement {
		t.Fatal("disturb-cast permanent should have a §702.146b dies→exile replacement registered")
	}
}

// ---------------------------------------------------------------------------
// Failure paths
// ---------------------------------------------------------------------------

func TestCastWithDisturb_NoKeyword(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	plain := newDisturbPlainSorcery("Plain Sorcery", 0)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, plain)
	_, err := CastWithDisturb(gs, 0, plain, 2)
	if err == nil {
		t.Fatal("CastWithDisturb should fail for a card without disturb")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "no_disturb_keyword" {
		t.Fatalf("expected CastError no_disturb_keyword, got %v", err)
	}
}

func TestCastWithDisturb_NoBackFaceRejected(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	// Has the keyword but no back face configured (parser typo / corpus
	// mistake). Must reject.
	weird := &Card{
		Name:  "Headless",
		Owner: 0,
		Types: []string{"creature"},
		AST: &gameast.CardAST{
			Name: "Headless",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "disturb", Args: []any{"{W}"}},
			},
		},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, weird)
	_, err := CastWithDisturb(gs, 0, weird, 1)
	if err == nil {
		t.Fatal("CastWithDisturb should reject a disturb card without a back face")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "no_back_face" {
		t.Fatalf("expected CastError no_back_face, got %v", err)
	}
}

func TestCastWithDisturb_NotInGraveyard(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	// Card NOT in graveyard.
	_, err := CastWithDisturb(gs, 0, card, 2)
	if err == nil {
		t.Fatal("CastWithDisturb should fail when card isn't in graveyard")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "not_in_graveyard" {
		t.Fatalf("expected CastError not_in_graveyard, got %v", err)
	}
}

func TestCastWithDisturb_InsufficientMana(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 0
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	_, err := CastWithDisturb(gs, 0, card, 2)
	if err == nil {
		t.Fatal("CastWithDisturb should fail with insufficient mana")
	}
	ce, ok := err.(*CastError)
	if !ok || ce.Reason != "insufficient_mana" {
		t.Fatalf("expected CastError insufficient_mana, got %v", err)
	}
	// Card still in graveyard.
	found := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("card should remain in graveyard after failed disturb cast")
	}
}

func TestCastWithDisturb_NegativeCostUsesPrinted(t *testing.T) {
	gs := newDisturbGame(t)
	gs.Seats[0].ManaPool = 5
	card := newDisturbCard("Lunarch Veteran", "Luminous Phantom", 0, "{1}{W}", false)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	if _, err := CastWithDisturb(gs, 0, card, -1); err != nil {
		t.Fatalf("CastWithDisturb with -1 sentinel failed: %v", err)
	}
	// Printed cost is {1}{W} = 2 CMC.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("mana left = %d, want 3 (printed DisturbCost 2 of 5)", gs.Seats[0].ManaPool)
	}
}
