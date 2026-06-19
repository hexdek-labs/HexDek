package gameengine

import "testing"

// r63 — manifest real-card overlay migration (morph redesign piece A).
//
// These pin the bug the migration killed: under the old synthetic-wrapper
// model a manifested face-down that left the battlefield put the throwaway
// "Face-Down Creature" / "Manifested Creature" wrapper into the next zone
// while the REAL card (stashed on OriginalCard / orphaned for dread) was
// lost — a zone-conservation leak. In the real-card model the real card is
// the permanent of record, so it is the card that moves.

func TestManifest_DeadCreaturePutsRealCardInGraveyard(t *testing.T) {
	gs := newManifestGame(t)
	real := libraryCreature("Shivan Dragon", 5, 5, 6)
	real.Owner = 0
	gs.Seats[0].Library = append(gs.Seats[0].Library, real)

	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]
	if perm.Card != real {
		t.Fatal("real card should be the permanent of record")
	}

	if !DestroyPermanent(gs, perm, nil) {
		t.Fatal("DestroyPermanent returned false")
	}

	gy := gs.Seats[0].Graveyard
	if len(gy) != 1 {
		t.Fatalf("expected exactly the real card in graveyard, got %d", len(gy))
	}
	if gy[0] != real {
		t.Fatalf("graveyard holds %q, want the REAL card (not a wrapper)", gy[0].DisplayName())
	}
	if gy[0].Name != "Shivan Dragon" {
		t.Fatalf("graveyard card name = %q, want Shivan Dragon", gy[0].Name)
	}
	// CR §707.2 — the card reverts FACE UP in the new zone.
	if gy[0].FaceDown {
		t.Error("dead manifested card must enter graveyard FACE UP")
	}
}

func TestManifestDread_DeadCreaturePutsRealCardInGraveyard(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Top A", true) // creature
	a.Owner = 0
	b := md_makeCard("Second B", false)
	gs.Seats[0].Library = []*Card{a, b}

	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int { return 0 })
	if perm == nil || perm.Card != a {
		t.Fatal("chosen real card should be the permanent of record")
	}

	if !DestroyPermanent(gs, perm, nil) {
		t.Fatal("DestroyPermanent returned false")
	}
	gy := gs.Seats[0].Graveyard
	foundReal := false
	for _, c := range gy {
		if c == a {
			foundReal = true
		}
		if c.Name == "Manifested Creature" {
			t.Fatal("a synthetic wrapper must not reach the graveyard")
		}
	}
	if !foundReal {
		t.Fatal("the chosen real card must be in the graveyard (was orphaned in the wrapper model)")
	}
	if a.FaceDown {
		t.Error("dead manifest-dread card must enter graveyard FACE UP")
	}
}

// The hidden card's own ETB self-trigger must NOT fire while manifested
// (CR §708.4 — a face-down permanent has no abilities). The generic
// permanent_etb observer event still fires (a creature did enter).
func TestManifest_HiddenCardSelfETBDoesNotFire(t *testing.T) {
	gs := newManifestGame(t)
	// A creature whose AST carries an ETB self-trigger.
	real := libraryCreature("Trigger Bear", 3, 3, 3)
	real.Owner = 0
	gs.Seats[0].Library = append(gs.Seats[0].Library, real)

	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	sawPermanentETB := false
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "permanent_etb" {
			sawPermanentETB = true
		}
	}

	ApplyManifestTop(gs, 0, 1)
	perm := gs.Seats[0].Battlefield[0]
	// Generic creature-entered observer event still fires.
	if !sawPermanentETB {
		t.Error("a manifested creature entering should still fire permanent_etb")
	}
	// While manifested the layered characteristics expose NO abilities.
	chars := GetEffectiveCharacteristics(gs, perm)
	if len(chars.Abilities) != 0 || len(chars.Keywords) != 0 {
		t.Errorf("face-down manifest should expose no abilities/keywords, got %v / %v",
			chars.Abilities, chars.Keywords)
	}
}

// Manifest-dread creatures can now be turned face up via the manifest
// right — the old model couldn't (it set neither OriginalCard nor a flip
// path for dread).
func TestManifestDread_CanFlipFaceUp(t *testing.T) {
	gs := md_makeGame(t)
	a := md_makeCard("Flippable", true)
	a.Owner = 0
	a.CMC = 2
	b := md_makeCard("Other", false)
	gs.Seats[0].Library = []*Card{a, b}
	gs.Seats[0].ManaPool = 5

	perm := ApplyManifestDread(gs, 0, func(top2 [2]*Card) int { return 0 })
	if perm == nil {
		t.Fatal("nil perm")
	}
	if err := ManifestedFaceUp(gs, perm, 2); err != nil {
		t.Fatalf("manifest-dread creature should flip face up: %v", err)
	}
	if perm.Card.FaceDown || IsManifested(perm) {
		t.Error("flip should clear face-down + manifested state")
	}
	if perm.Card.Name != "Flippable" {
		t.Errorf("flipped name = %q, want Flippable", perm.Card.Name)
	}
}
