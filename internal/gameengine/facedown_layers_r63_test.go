package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — face-down template-driven base + grant-retention fix (layers.go).

// The core grant-retention regression: a keyword GRANTED to a face-down
// creature (recorded on Permanent.GrantedAbilities) must survive the
// §707.2 override. Before the fix the face-down branch nulled Keywords and
// returned early, wiping the grant.
func TestFaceDown_GrantedAbilitiesSurvive(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.GrantedAbilities = []string{"flying", "lifelink"}
	bear.Card.FaceDown = true
	gs.InvalidateCharacteristicsCache()

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("face-down base should be 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	if !containsFold(chars.Keywords, "flying") {
		t.Errorf("granted flying wiped from face-down characteristics: %v", chars.Keywords)
	}
	if !containsFold(chars.Keywords, "lifelink") {
		t.Errorf("granted lifelink wiped from face-down characteristics: %v", chars.Keywords)
	}
	if !gs.HasKeywordOf(bear, "flying") {
		t.Errorf("HasKeywordOf should report granted flying on the face-down creature")
	}
	// The card's PRINTED abilities are still gone (no Abilities slice).
	if len(chars.Abilities) != 0 {
		t.Errorf("printed abilities should still be stripped while face down")
	}
}

// An anthem (continuous effect, layer 7c) must still buff a face-down
// creature — the template is the base layer, downstream layers apply on
// top.
func TestFaceDown_AnthemAppliesOnTop(t *testing.T) {
	gs := newFixtureGame(t)
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
	bear.Card.FaceDown = true
	gs.InvalidateCharacteristicsCache()

	chars := GetEffectiveCharacteristics(gs, bear)
	// 2/2 face-down base + 1/1 anthem = 3/3.
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Errorf("face-down + anthem should be 3/3, got %d/%d", chars.Power, chars.Toughness)
	}
}

// The cyber template produces a 2/2 colorless Cyberman ARTIFACT creature
// with mana value 0.
func TestFaceDown_CyberTemplate(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.Card.CMC = 6
	dragon.Card.FaceDown = true
	dragon.FaceDownTemplate = "cyber"
	gs.InvalidateCharacteristicsCache()

	chars := GetEffectiveCharacteristics(gs, dragon)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("cyber face-down should be 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	hasArtifact, hasCreature := false, false
	for _, ty := range chars.Types {
		if ty == "artifact" {
			hasArtifact = true
		}
		if ty == "creature" {
			hasCreature = true
		}
	}
	if !hasArtifact || !hasCreature {
		t.Errorf("cyber face-down should be an artifact creature, types=%v", chars.Types)
	}
	if !containsFold(chars.Subtypes, "cyberman") {
		t.Errorf("cyber face-down should be a Cyberman, subtypes=%v", chars.Subtypes)
	}
	if len(chars.Colors) != 0 {
		t.Errorf("cyber face-down should be colorless, got %v", chars.Colors)
	}
	if chars.CMC != 0 {
		t.Errorf("face-down mana value should be 0 (§707.2), got %d", chars.CMC)
	}
}

// The disguise template carries an intrinsic ward keyword into the
// face-down characteristics (CR §702.166a).
func TestFaceDown_DisguiseTemplateHasWard(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Card.FaceDown = true
	bear.FaceDownTemplate = "disguise"
	gs.InvalidateCharacteristicsCache()

	chars := GetEffectiveCharacteristics(gs, bear)
	if !containsFold(chars.Keywords, "ward") {
		t.Errorf("disguise face-down should have intrinsic ward, keywords=%v", chars.Keywords)
	}
	// A plain morph template has NO ward.
	bear.FaceDownTemplate = "morph"
	gs.InvalidateCharacteristicsCache()
	chars = GetEffectiveCharacteristics(gs, bear)
	if containsFold(chars.Keywords, "ward") {
		t.Errorf("morph face-down should NOT have ward, keywords=%v", chars.Keywords)
	}
}
