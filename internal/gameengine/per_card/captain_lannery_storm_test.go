package per_card

import (
	"testing"
)

func TestLannery_AttackSpawnsTreasureToken(t *testing.T) {
	gs := newGame(t, 2)
	lannery := addPerm(gs, 0, "Captain Lannery Storm", "creature", "pirate")
	preCount := len(gs.Seats[0].Battlefield)
	lanneryAttack(gs, lannery, map[string]interface{}{"attacker_perm": lannery})
	if len(gs.Seats[0].Battlefield) != preCount+1 {
		t.Errorf("expected 1 new Treasure token on battlefield, got %d new",
			len(gs.Seats[0].Battlefield)-preCount)
	}
}

func TestLannery_NoFireWhenOtherCreatureAttacks(t *testing.T) {
	gs := newGame(t, 2)
	lannery := addPerm(gs, 0, "Captain Lannery Storm", "creature")
	other := addPerm(gs, 0, "Other", "creature")
	preCount := len(gs.Seats[0].Battlefield)
	lanneryAttack(gs, lannery, map[string]interface{}{"attacker_perm": other})
	if len(gs.Seats[0].Battlefield) != preCount {
		t.Errorf("Lannery must not fire when a different attacker is declared")
	}
}

func TestLannery_TreasureSacPumpsLannery(t *testing.T) {
	gs := newGame(t, 2)
	lannery := addPerm(gs, 0, "Captain Lannery Storm", "creature")
	prePower := lannery.Power()
	treasureCard := newCardWithTypes("Treasure Token", 0, "artifact", "treasure", "token")
	lanneryArtifactSac(gs, lannery, map[string]interface{}{
		"controller_seat": 0,
		"card":            treasureCard,
	})
	if lannery.Power() != prePower+1 {
		t.Errorf("expected Lannery +1 power after treasure sac, got %d → %d", prePower, lannery.Power())
	}
}

func TestLannery_NonTreasureArtifactSacDoesNotPump(t *testing.T) {
	gs := newGame(t, 2)
	lannery := addPerm(gs, 0, "Captain Lannery Storm", "creature")
	prePower := lannery.Power()
	nonTreasureArtifact := newCardWithTypes("Sol Ring", 0, "artifact")
	lanneryArtifactSac(gs, lannery, map[string]interface{}{
		"controller_seat": 0,
		"card":            nonTreasureArtifact,
	})
	if lannery.Power() != prePower {
		t.Errorf("non-Treasure artifact sac must not pump Lannery")
	}
}

func TestLannery_OpponentTreasureSacDoesNotPump(t *testing.T) {
	gs := newGame(t, 2)
	lannery := addPerm(gs, 0, "Captain Lannery Storm", "creature")
	prePower := lannery.Power()
	oppTreasure := newCardWithTypes("Opp Treasure", 1, "artifact", "treasure", "token")
	lanneryArtifactSac(gs, lannery, map[string]interface{}{
		"controller_seat": 1,
		"card":            oppTreasure,
	})
	if lannery.Power() != prePower {
		t.Errorf("opp treasure sac must not pump (oracle says \"you sacrifice\")")
	}
}
