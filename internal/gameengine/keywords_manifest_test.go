package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// Manifest tests — CR §701.34
// ---------------------------------------------------------------------------

func newManifestGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(58)), nil)
}

// libraryCreature builds a "real" creature card to seed a library.
func libraryCreature(name string, power, tough, cmc int) *Card {
	return &Card{
		Name:          name,
		BasePower:     power,
		BaseToughness: tough,
		CMC:           cmc,
		Types:         []string{"creature"},
		TypeLine:      "Creature — Beast",
		AST:           &gameast.CardAST{Name: name},
	}
}

func libraryNonCreature(name string, cmc int) *Card {
	return &Card{
		Name:     name,
		CMC:      cmc,
		Types:    []string{"sorcery"},
		TypeLine: "Sorcery",
		AST:      &gameast.CardAST{Name: name},
	}
}

// ---------------------------------------------------------------------------
// (b) HasManifest detector
// ---------------------------------------------------------------------------

func TestHasManifest_DetectsCanonicalPhrasings(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Manifest the top card of your library.", true},
		{"Manifest the top three cards of your library.", true},
		{"Manifest dread.", true},
		{"Flying.", false},
		{"This creature can attack.", false},
		{"", false},
	}
	for _, c := range cases {
		card := &Card{Name: "x", AST: &gameast.CardAST{Name: "x"}}
		card.OracleTextCache = c.text
		card.oracleTextReady = true
		if got := HasManifest(card); got != c.want {
			t.Errorf("HasManifest(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestHasManifest_NilSafe(t *testing.T) {
	if HasManifest(nil) {
		t.Fatal("HasManifest(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// (a) ApplyManifestTop creates 2/2 face-down with hidden underlying card
// ---------------------------------------------------------------------------

func TestApplyManifestTop_CreatesFaceDown2x2(t *testing.T) {
	gs := newManifestGame(t)
	underlying := libraryCreature("Shivan Dragon", 5, 5, 6)
	gs.Seats[0].Library = append(gs.Seats[0].Library, underlying)

	count := ApplyManifestTop(gs, 0, 1)
	if count != 1 {
		t.Fatalf("ApplyManifestTop = %d, want 1", count)
	}
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatalf("expected 1 manifested perm on battlefield, got %d",
			len(gs.Seats[0].Battlefield))
	}
	perm := gs.Seats[0].Battlefield[0]
	if !perm.Card.FaceDown {
		t.Fatal("manifested perm should be face down")
	}
	if !IsManifested(perm) {
		t.Fatal("perm.Flags[manifested] should be 1")
	}
	// Real-card overlay model: the real card IS the permanent of record,
	// but its effective characteristics are the §707.2 2/2 vanilla shape.
	if perm.Card != underlying {
		t.Fatal("real card should be the permanent of record (perm.Card)")
	}
	if perm.OriginalCard != nil {
		t.Fatal("no synthetic wrapper / OriginalCard orphan in the real-card model")
	}
	if perm.Power() != 2 || perm.Toughness() != 2 {
		t.Fatalf("effective face-down stats = %d/%d, want 2/2",
			perm.Power(), perm.Toughness())
	}
	// The underlying identity is hidden in the layered characteristics.
	if chars := GetEffectiveCharacteristics(gs, perm); chars.Name != "" {
		t.Fatalf("face-down should have no visible name, got %q", chars.Name)
	}
	// Library shrank.
	if len(gs.Seats[0].Library) != 0 {
		t.Fatalf("library not consumed: %d remaining", len(gs.Seats[0].Library))
	}
}

func TestApplyManifestTop_BatchManifestThreeCards(t *testing.T) {
	gs := newManifestGame(t)
	for _, name := range []string{"A", "B", "C", "D", "E"} {
		gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature(name, 3, 3, 2))
	}

	count := ApplyManifestTop(gs, 0, 3)
	if count != 3 {
		t.Fatalf("ApplyManifestTop(3) = %d, want 3", count)
	}
	if len(gs.Seats[0].Battlefield) != 3 {
		t.Fatalf("expected 3 perms on battlefield, got %d", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Fatalf("expected 2 library cards left, got %d", len(gs.Seats[0].Library))
	}
	// Order: top of library (A) is the first manifested. The real card is
	// the permanent of record (perm.Card).
	names := []string{
		gs.Seats[0].Battlefield[0].Card.Name,
		gs.Seats[0].Battlefield[1].Card.Name,
		gs.Seats[0].Battlefield[2].Card.Name,
	}
	if names[0] != "A" || names[1] != "B" || names[2] != "C" {
		t.Fatalf("manifest order wrong: got %v, want [A B C]", names)
	}
}

func TestApplyManifestTop_FiresETBTriggers(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("Manifested", 2, 2, 2))

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawEvent := ""
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "permanent_etb" {
			sawEvent = ev
		}
	}

	ApplyManifestTop(gs, 0, 1)
	if sawEvent != "permanent_etb" {
		t.Fatal("manifested perm should fire permanent_etb")
	}
}

// ---------------------------------------------------------------------------
// (e) Library empty = no-op
// ---------------------------------------------------------------------------

func TestApplyManifestTop_EmptyLibraryIsNoOp(t *testing.T) {
	gs := newManifestGame(t)
	// Library is empty.
	count := ApplyManifestTop(gs, 0, 3)
	if count != 0 {
		t.Fatalf("manifest from empty library = %d, want 0", count)
	}
	if len(gs.Seats[0].Battlefield) != 0 {
		t.Fatal("no perms should be created from an empty library")
	}
}

func TestApplyManifestTop_PartialFillWhenLibraryRunsDry(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("Solo", 2, 2, 2))

	// Ask for 3 but only 1 available.
	count := ApplyManifestTop(gs, 0, 3)
	if count != 1 {
		t.Fatalf("partial manifest = %d, want 1", count)
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Fatal("library should be empty after consuming the one card")
	}
}

func TestApplyManifestTop_ZeroNIsNoOp(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("X", 2, 2, 2))
	if count := ApplyManifestTop(gs, 0, 0); count != 0 {
		t.Fatalf("ApplyManifestTop(0) = %d, want 0", count)
	}
	if count := ApplyManifestTop(gs, 0, -5); count != 0 {
		t.Fatalf("ApplyManifestTop(-5) = %d, want 0", count)
	}
	// Library should be untouched.
	if len(gs.Seats[0].Library) != 1 {
		t.Fatal("library should not be consumed by zero/negative n")
	}
}

// ---------------------------------------------------------------------------
// (c) Flip face-up reveals stats/abilities
// ---------------------------------------------------------------------------

func TestManifestedFaceUp_RevealsUnderlyingCard(t *testing.T) {
	gs := newManifestGame(t)
	underlying := libraryCreature("Eternal Dragon", 7, 7, 7)
	gs.Seats[0].Library = append(gs.Seats[0].Library, underlying)
	gs.Seats[0].ManaPool = 7

	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]

	if !perm.Card.FaceDown || perm.Card != underlying {
		t.Fatal("setup: real card should be the permanent of record, face down")
	}

	if err := ManifestedFaceUp(gs, perm, 7); err != nil {
		t.Fatalf("ManifestedFaceUp returned error: %v", err)
	}
	if perm.Card != underlying {
		t.Fatal("perm.Card should now reference the underlying card")
	}
	if perm.Card.FaceDown {
		t.Fatal("underlying card should be face up")
	}
	if perm.Card.Name != "Eternal Dragon" {
		t.Fatalf("flipped name = %q, want \"Eternal Dragon\"", perm.Card.Name)
	}
	if perm.Card.BasePower != 7 || perm.Card.BaseToughness != 7 {
		t.Fatalf("flipped P/T = %d/%d, want 7/7",
			perm.Card.BasePower, perm.Card.BaseToughness)
	}
	if IsManifested(perm) {
		t.Fatal("manifested flag should be cleared after flip")
	}
	if perm.OriginalCard != nil {
		t.Fatal("OriginalCard should be cleared after flip")
	}
	// Mana paid.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("mana pool = %d, want 0", gs.Seats[0].ManaPool)
	}
}

func TestManifestedFaceUp_FiresFlipTrigger(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("X", 4, 4, 3))
	gs.Seats[0].ManaPool = 5
	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawEvent := ""
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "manifest_flipped" {
			sawEvent = ev
		}
	}
	if err := ManifestedFaceUp(gs, perm, 3); err != nil {
		t.Fatalf("flip failed: %v", err)
	}
	if sawEvent != "manifest_flipped" {
		t.Fatal("manifest_flipped trigger should fire on flip")
	}
}

func TestManifestedFaceUp_RejectsNonManifested(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].ManaPool = 5
	// A normal creature on the battlefield, not manifested.
	normal := addBattlefield(gs, 0, "Normal Bear", 2, 2, "creature")
	if err := ManifestedFaceUp(gs, normal, 0); err == nil {
		t.Fatal("ManifestedFaceUp must reject a non-manifested perm")
	}
}

func TestManifestedFaceUp_RejectsNonCreatureUnderlying(t *testing.T) {
	gs := newManifestGame(t)
	// A sorcery on top of the library — manifests as a 2/2 but can't
	// be flipped face up by the manifest right (§701.34b creature-only).
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryNonCreature("Wrath", 4))
	gs.Seats[0].ManaPool = 10
	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]
	if err := ManifestedFaceUp(gs, perm, 4); err == nil {
		t.Fatal("manifested non-creature must not be flipped face up via the manifest right")
	}
	if !perm.Card.FaceDown {
		t.Fatal("rejected flip must leave the perm face-down")
	}
}

func TestManifestedFaceUp_RejectsInsufficientMana(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("Big", 8, 8, 8))
	gs.Seats[0].ManaPool = 3
	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]
	if err := ManifestedFaceUp(gs, perm, 8); err == nil {
		t.Fatal("flip must reject when controller can't pay the cost")
	}
	// Atomic: no mana spent, perm stays face-down.
	if gs.Seats[0].ManaPool != 3 {
		t.Fatal("rejected flip must not pay mana")
	}
	if IsManifested(perm) != true {
		t.Fatal("rejected flip must leave the manifested flag set")
	}
}

// ---------------------------------------------------------------------------
// (d) Manifested cards can attack/block as 2/2
// ---------------------------------------------------------------------------

func TestApplyManifestTop_ManifestedActsAs2x2Creature(t *testing.T) {
	gs := newManifestGame(t)
	// Underlying is a 5/5, but face-down it should act as a 2/2.
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("Big Dragon", 5, 5, 6))
	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]

	if !perm.IsCreature() {
		t.Fatal("manifested perm should be a creature (§701.34a — 2/2 creature)")
	}
	if got := perm.Power(); got != 2 {
		t.Fatalf("face-down Power() = %d, want 2", got)
	}
	if got := perm.Toughness(); got != 2 {
		t.Fatalf("face-down Toughness() = %d, want 2", got)
	}
}

func TestApplyManifestTop_ManifestedCanBlock(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("Hidden", 4, 4, 4))
	ApplyManifestTop(gs, 0, 1)
	// Untap so the manifested creature is eligible to block.
	manifested := gs.Seats[0].Battlefield[0]
	manifested.Tapped = false
	// SummoningSick is set by manifest — for the can-block test, clear
	// it (summoning sickness affects attacking, not blocking).
	manifested.SummoningSick = false

	atk := addBattlefield(gs, 1, "Attacker", 3, 3, "creature")
	atk.Flags["attacking"] = 1

	if !canBlockGS(gs, atk, manifested) {
		t.Fatal("a manifested face-down 2/2 creature must be a legal blocker")
	}
}

// ---------------------------------------------------------------------------
// IsManifested
// ---------------------------------------------------------------------------

func TestIsManifested_ChecksFlag(t *testing.T) {
	gs := newManifestGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library, libraryCreature("X", 2, 2, 2))
	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]
	if !IsManifested(perm) {
		t.Fatal("IsManifested should be true on a freshly manifested perm")
	}
	gs.Seats[0].ManaPool = 5
	if err := ManifestedFaceUp(gs, perm, 2); err != nil {
		t.Fatalf("flip failed: %v", err)
	}
	if IsManifested(perm) {
		t.Fatal("IsManifested should be false after flip")
	}
}

func TestIsManifested_NilSafe(t *testing.T) {
	if IsManifested(nil) {
		t.Fatal("IsManifested(nil) should be false")
	}
}

// ---------------------------------------------------------------------------
// Nil / invalid-input safety
// ---------------------------------------------------------------------------

func TestApplyManifestTop_NilSafe(t *testing.T) {
	if ApplyManifestTop(nil, 0, 1) != 0 {
		t.Fatal("nil game ApplyManifestTop must return 0")
	}
	gs := newManifestGame(t)
	if ApplyManifestTop(gs, -1, 1) != 0 {
		t.Fatal("invalid seat ApplyManifestTop must return 0")
	}
	if ApplyManifestTop(gs, 99, 1) != 0 {
		t.Fatal("out-of-range seat ApplyManifestTop must return 0")
	}
}

func TestManifestedFaceUp_NilSafe(t *testing.T) {
	gs := newManifestGame(t)
	if err := ManifestedFaceUp(nil, nil, 0); err == nil {
		t.Fatal("nil game must return error")
	}
	if err := ManifestedFaceUp(gs, nil, 0); err == nil {
		t.Fatal("nil perm must return error")
	}
}
