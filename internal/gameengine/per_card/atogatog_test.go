package per_card

import (
	"testing"
)

func TestAtogatog_SacAtogPumpsByPower(t *testing.T) {
	gs := newGame(t, 2)
	atogatog := addPerm(gs, 0, "Atogatog", "creature", "atog")
	atog := addPerm(gs, 0, "Auriok Survivors Atog", "creature", "atog")
	atog.Card.BasePower = 4
	atog.Card.BaseToughness = 4
	prePower := atogatog.Power()

	atogatogActivate(gs, atogatog, 0, map[string]interface{}{"target_perm": atog})

	// atog is sacrificed
	for _, p := range gs.Seats[0].Battlefield {
		if p == atog {
			t.Errorf("expected Atog sacrificed off battlefield")
		}
	}
	if atogatog.Power() != prePower+4 {
		t.Errorf("expected Atogatog +4/+4 (target Power=4), got %d → %d", prePower, atogatog.Power())
	}
}

func TestAtogatog_RefusesNonAtogTarget(t *testing.T) {
	gs := newGame(t, 2)
	atogatog := addPerm(gs, 0, "Atogatog", "creature", "atog")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	prePower := atogatog.Power()
	atogatogActivate(gs, atogatog, 0, map[string]interface{}{"target_perm": bear})
	if atogatog.Power() != prePower {
		t.Errorf("Atogatog must only eat Atogs")
	}
}

func TestAtogatog_RefusesSelfTarget(t *testing.T) {
	gs := newGame(t, 2)
	atogatog := addPerm(gs, 0, "Atogatog", "creature", "atog")
	prePower := atogatog.Power()
	atogatogActivate(gs, atogatog, 0, map[string]interface{}{"target_perm": atogatog})
	if atogatog.Power() != prePower {
		t.Errorf("self-target should be rejected (pointless)")
	}
}

func TestAtogatog_RefusesOpponentAtog(t *testing.T) {
	gs := newGame(t, 2)
	atogatog := addPerm(gs, 0, "Atogatog", "creature", "atog")
	oppAtog := addPerm(gs, 1, "Opp Atog", "creature", "atog")
	prePower := atogatog.Power()
	atogatogActivate(gs, atogatog, 0, map[string]interface{}{"target_perm": oppAtog})
	if atogatog.Power() != prePower {
		t.Errorf("Atogatog can only sacrifice his controller's Atogs")
	}
}

func TestAtogatog_ZeroPowerTargetNoOps(t *testing.T) {
	gs := newGame(t, 2)
	atogatog := addPerm(gs, 0, "Atogatog", "creature", "atog")
	zero := addPerm(gs, 0, "Defender Atog", "creature", "atog")
	zero.Card.BasePower = 0
	zero.Card.BaseToughness = 5
	prePower := atogatog.Power()
	atogatogActivate(gs, atogatog, 0, map[string]interface{}{"target_perm": zero})
	if atogatog.Power() != prePower {
		t.Errorf("0-power Atog sac should not pump (X=0)")
	}
}
