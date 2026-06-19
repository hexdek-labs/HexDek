package gameengine

import "testing"

// r63 — unified face-down overlay registry + makeFaceDown primitive.

func TestFaceDownTemplateFor_KnownKeys(t *testing.T) {
	cases := []struct {
		key       string
		power     int
		ward      int
		hidden    bool
		turnUp    string
		artifact  bool
		cyberman  bool
	}{
		{"morph", 2, 0, true, "morph", false, false},
		{"disguise", 2, DisguiseFaceDownWardCost, true, "disguise", false, false},
		{"cloak", 2, CloakFaceDownWardCost, true, "mana", false, false},
		{"manifest", 2, 0, true, "mana", false, false},
		{"cyber", 2, 0, false, "none", true, true},
	}
	for _, tc := range cases {
		tmpl := FaceDownTemplateFor(tc.key)
		if tmpl.Key != tc.key {
			t.Errorf("%s: key=%q", tc.key, tmpl.Key)
		}
		if tmpl.Power != tc.power || tmpl.Toughness != tc.power {
			t.Errorf("%s: P/T=%d/%d want %d/%d", tc.key, tmpl.Power, tmpl.Toughness, tc.power, tc.power)
		}
		if tmpl.Ward != tc.ward {
			t.Errorf("%s: ward=%d want %d", tc.key, tmpl.Ward, tc.ward)
		}
		if tmpl.Hidden != tc.hidden {
			t.Errorf("%s: hidden=%v want %v", tc.key, tmpl.Hidden, tc.hidden)
		}
		if tmpl.TurnUp != tc.turnUp {
			t.Errorf("%s: turnUp=%q want %q", tc.key, tmpl.TurnUp, tc.turnUp)
		}
		hasArtifact := false
		for _, ty := range tmpl.Types {
			if ty == "artifact" {
				hasArtifact = true
			}
		}
		if hasArtifact != tc.artifact {
			t.Errorf("%s: artifact=%v want %v (types=%v)", tc.key, hasArtifact, tc.artifact, tmpl.Types)
		}
		hasCyberman := false
		for _, st := range tmpl.Subtypes {
			if st == "cyberman" {
				hasCyberman = true
			}
		}
		if hasCyberman != tc.cyberman {
			t.Errorf("%s: cyberman=%v want %v", tc.key, hasCyberman, tc.cyberman)
		}
	}
}

// An unknown / empty key falls back to the morph template — the §707.2
// default that keeps legacy face-down permanents (FaceDownTemplate == "")
// computing the right base.
func TestFaceDownTemplateFor_FallsBackToMorph(t *testing.T) {
	for _, key := range []string{"", "bogus", "Morph"} {
		tmpl := FaceDownTemplateFor(key)
		if tmpl.Key != "morph" {
			t.Errorf("key %q: fell back to %q, want morph", key, tmpl.Key)
		}
	}
}

func TestMakeFaceDown_StampsTemplateAndFlags(t *testing.T) {
	gs := newDisguiseGame(t)
	card := &Card{Name: "Hooded Hydra", Owner: 0}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}

	makeFaceDown(gs, perm, "disguise", faceDownOpts{
		Markers: []string{"face_down", "morph_creature", "disguise_face_down"},
	})

	if !card.FaceDown {
		t.Fatal("Card.FaceDown not set")
	}
	if perm.FaceDownTemplate != "disguise" {
		t.Fatalf("FaceDownTemplate=%q want disguise", perm.FaceDownTemplate)
	}
	// Ward {2} derived from the template, not from a marker.
	if perm.Flags["kw:ward"] != 1 || perm.Flags["ward_cost"] != DisguiseFaceDownWardCost {
		t.Errorf("ward flags wrong: kw:ward=%d ward_cost=%d", perm.Flags["kw:ward"], perm.Flags["ward_cost"])
	}
	for _, m := range []string{"face_down", "morph_creature", "disguise_face_down"} {
		if perm.Flags[m] != 1 {
			t.Errorf("marker %q not set", m)
		}
	}
}

// A no-ward template (manifest) must NOT raise the ward flags.
func TestMakeFaceDown_ManifestNoWard(t *testing.T) {
	gs := newManifestGame(t)
	perm := &Permanent{Card: &Card{Name: "X", Owner: 0}, Controller: 0, Owner: 0}
	makeFaceDown(gs, perm, "manifest", faceDownOpts{Markers: []string{"manifested"}})
	if perm.Flags["kw:ward"] != 0 || perm.Flags["ward_cost"] != 0 {
		t.Errorf("manifest should have no ward: kw:ward=%d ward_cost=%d", perm.Flags["kw:ward"], perm.Flags["ward_cost"])
	}
	if perm.FaceDownTemplate != "manifest" {
		t.Errorf("FaceDownTemplate=%q want manifest", perm.FaceDownTemplate)
	}
}

func TestMakeFaceDown_NilSafe(t *testing.T) {
	gs := newManifestGame(t)
	makeFaceDown(gs, nil, "morph", faceDownOpts{})          // nil perm
	makeFaceDown(gs, &Permanent{}, "morph", faceDownOpts{}) // nil card
}

// The mint paths now record the template on the live permanent.
func TestMintPaths_RecordTemplate(t *testing.T) {
	t.Run("disguise", func(t *testing.T) {
		gs := newDisguiseGame(t)
		gs.Seats[0].ManaPool = DisguiseFaceDownCost
		card := disguiseHandCard(gs, 0, "Hooded Hydra", 4)
		perm, err := CastDisguiseFaceDown(gs, 0, card)
		if err != nil {
			t.Fatalf("CastDisguiseFaceDown: %v", err)
		}
		if perm.FaceDownTemplate != "disguise" {
			t.Errorf("disguise template=%q", perm.FaceDownTemplate)
		}
	})
	t.Run("manifest", func(t *testing.T) {
		gs := newManifestGame(t)
		gs.Seats[0].Library = []*Card{libraryCreature("Grizzly Bears", 2, 2, 2)}
		ApplyManifestTop(gs, 0, 1)
		bf := gs.Seats[0].Battlefield
		if len(bf) == 0 {
			t.Fatal("nothing manifested")
		}
		if bf[len(bf)-1].FaceDownTemplate != "manifest" {
			t.Errorf("manifest template=%q", bf[len(bf)-1].FaceDownTemplate)
		}
	})
}
