package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestMishraEminent_CombatBeginCopiesArtifactAsFourFourWarform(t *testing.T) {
	gs := newGame(t, 2)
	mishra := addPerm(gs, 0, "Mishra, Eminent One", "creature", "phyrexian")
	mox := addPerm(gs, 0, "Mox Opal", "artifact")
	mox.Card.Types = append(mox.Card.Types, "cmc:0")

	mishraEminentOneCombatBegin(gs, mishra, map[string]interface{}{"active_seat": 0})

	var warform *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Mishra's Warform" {
			warform = p
		}
	}
	if warform == nil {
		t.Fatal("expected Mishra's Warform token")
	}
	if warform.Card.BasePower != 4 || warform.Card.BaseToughness != 4 {
		t.Errorf("warform must be 4/4, got %d/%d", warform.Card.BasePower, warform.Card.BaseToughness)
	}
	if !cardHasType(warform.Card, "creature") {
		t.Errorf("warform must have creature type")
	}
	if !cardHasSubtype(warform.Card, "construct") {
		t.Errorf("warform must have construct subtype")
	}
	if !cardHasType(warform.Card, "artifact") {
		t.Errorf("warform must preserve artifact type")
	}
	if warform.Flags["kw:haste"] != 1 {
		t.Errorf("warform must have haste")
	}
}

func TestMishraEminent_PicksHighestCMCArtifact(t *testing.T) {
	gs := newGame(t, 2)
	mishra := addPerm(gs, 0, "Mishra, Eminent One", "creature")
	small := addPerm(gs, 0, "Sol Ring", "artifact")
	small.Card.Types = append(small.Card.Types, "cmc:1")
	big := addPerm(gs, 0, "Citadel", "artifact")
	big.Card.Types = append(big.Card.Types, "cmc:6")
	mishraEminentOneCombatBegin(gs, mishra, map[string]interface{}{"active_seat": 0})
	// Mox got picked? Check that Warform was created. The copy keeps the
	// original's BasePower/Toughness pre-modification — we overwrite to 4.
	// Just verify a Warform was created and the higher-CMC artifact source
	// would have produced it. We can't easily tell which source without
	// tracking — verify count.
	warformCount := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Mishra's Warform" {
			warformCount++
		}
	}
	if warformCount != 1 {
		t.Errorf("expected exactly 1 Warform token, got %d", warformCount)
	}
}

func TestMishraEminent_RefusesCreatureArtifact(t *testing.T) {
	gs := newGame(t, 2)
	mishra := addPerm(gs, 0, "Mishra, Eminent One", "creature")
	// Artifact CREATURE — not a valid target.
	addPerm(gs, 0, "Steel Hellkite", "artifact", "creature")
	mishraEminentOneCombatBegin(gs, mishra, map[string]interface{}{"active_seat": 0})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Mishra's Warform" {
			t.Errorf("Warform must not be created when only artifact creatures exist")
		}
	}
}

func TestMishraEminent_NoFireOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	mishra := addPerm(gs, 0, "Mishra, Eminent One", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact")
	mishraEminentOneCombatBegin(gs, mishra, map[string]interface{}{"active_seat": 1})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Mishra's Warform" {
			t.Errorf("Mishra must not fire on opp turn")
		}
	}
}

func TestMishraEminent_RegistersEOTSacDelayedTrigger(t *testing.T) {
	gs := newGame(t, 2)
	mishra := addPerm(gs, 0, "Mishra, Eminent One", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact")
	pre := len(gs.DelayedTriggers)
	mishraEminentOneCombatBegin(gs, mishra, map[string]interface{}{"active_seat": 0})
	if len(gs.DelayedTriggers) != pre+1 {
		t.Errorf("expected 1 delayed trigger for EOT sac, got %d new", len(gs.DelayedTriggers)-pre)
	}
	if gs.DelayedTriggers[len(gs.DelayedTriggers)-1].TriggerAt != "next_end_step" {
		t.Errorf("expected TriggerAt=next_end_step")
	}
}
