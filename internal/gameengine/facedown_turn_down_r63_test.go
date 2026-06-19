package gameengine

import "testing"

// r63 — TurnPermanentFaceDown: turn an EXISTING battlefield creature face
// down under a template (the Cyber Conversion / Ixidron entry point).

func TestTurnPermanentFaceDown_CyberOverlay(t *testing.T) {
	gs := newFixtureGame(t)
	dragon := addBattlefield(gs, 0, "Shivan Dragon", 5, 5, "creature")
	dragon.Card.CMC = 6
	dragon.GrantedAbilities = []string{"flying"}
	realCard := dragon.Card

	if !TurnPermanentFaceDown(gs, dragon, "cyber") {
		t.Fatal("TurnPermanentFaceDown returned false")
	}
	// The REAL card stays the permanent of record.
	if dragon.Card != realCard {
		t.Error("real card must remain perm.Card (permanent of record)")
	}
	if !dragon.Card.FaceDown {
		t.Error("Card.FaceDown not set")
	}
	if dragon.FaceDownTemplate != "cyber" {
		t.Errorf("FaceDownTemplate=%q want cyber", dragon.FaceDownTemplate)
	}
	chars := GetEffectiveCharacteristics(gs, dragon)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("should be a 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
	hasArtifact := false
	for _, ty := range chars.Types {
		if ty == "artifact" {
			hasArtifact = true
		}
	}
	if !hasArtifact {
		t.Errorf("cyber face-down should be an artifact creature, types=%v", chars.Types)
	}
	if !containsFold(chars.Subtypes, "cyberman") {
		t.Errorf("should be a Cyberman, subtypes=%v", chars.Subtypes)
	}
	// The granted flying survives (grant-retention).
	if !containsFold(chars.Keywords, "flying") {
		t.Errorf("granted flying should survive the face-down overlay, keywords=%v", chars.Keywords)
	}
}

func TestTurnPermanentFaceDown_AlreadyFaceDownIsNoOp(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.Card.FaceDown = true
	if TurnPermanentFaceDown(gs, bear, "cyber") {
		t.Error("turning an already-face-down permanent face down should return false")
	}
}

func TestTurnPermanentFaceDown_PreservesCounters(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	bear.AddCounter("+1/+1", 3)
	if !TurnPermanentFaceDown(gs, bear, "cyber") {
		t.Fatal("returned false")
	}
	if bear.Counters["+1/+1"] != 3 {
		t.Errorf("counters must be preserved across face-down (§707.2), got %d", bear.Counters["+1/+1"])
	}
	// 2/2 base + three +1/+1 counters = 5/5.
	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("2/2 + 3 counters should be 5/5, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestTurnPermanentFaceDown_NilSafe(t *testing.T) {
	gs := newFixtureGame(t)
	if TurnPermanentFaceDown(gs, nil, "cyber") {
		t.Error("nil perm should return false")
	}
	if TurnPermanentFaceDown(gs, &Permanent{}, "cyber") {
		t.Error("cardless perm should return false")
	}
}
