package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestReaperKing_AnotherScarecrowETBDestroysOpponentCreature(t *testing.T) {
	gs := newGame(t, 2)
	reaper := addPerm(gs, 0, "Reaper King", "creature", "scarecrow")
	scarecrow := addPerm(gs, 0, "Reaper Drone", "creature", "scarecrow")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature", "bear")
	reaperKingETB(gs, reaper, map[string]interface{}{
		"controller_seat": 0,
		"perm":            scarecrow,
	})
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			t.Errorf("expected victim destroyed, still on battlefield")
		}
	}
	if len(gs.Seats[1].Graveyard) == 0 || gs.Seats[1].Graveyard[0].DisplayName() != "Grizzly Bears" {
		t.Errorf("expected victim in opponent's graveyard, got %v",
			func() []string {
				out := []string{}
				for _, c := range gs.Seats[1].Graveyard {
					if c != nil {
						out = append(out, c.DisplayName())
					}
				}
				return out
			}())
	}
}

func TestReaperKing_DoesNotFireOnSelfETB(t *testing.T) {
	gs := newGame(t, 2)
	reaper := addPerm(gs, 0, "Reaper King", "creature", "scarecrow")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature")
	reaperKingETB(gs, reaper, map[string]interface{}{
		"controller_seat": 0,
		"perm":            reaper,
	})
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			return // still alive — correct
		}
	}
	t.Errorf("Reaper must not fire on his own ETB (\"another Scarecrow\")")
}

func TestReaperKing_DoesNotFireOnNonScarecrowETB(t *testing.T) {
	gs := newGame(t, 2)
	reaper := addPerm(gs, 0, "Reaper King", "creature", "scarecrow")
	bear := addPerm(gs, 0, "Llanowar Elves", "creature", "elf")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature")
	reaperKingETB(gs, reaper, map[string]interface{}{
		"controller_seat": 0,
		"perm":            bear,
	})
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			return
		}
	}
	t.Errorf("Reaper must not fire on non-Scarecrow ETB")
}

func TestReaperKing_DoesNotFireOnOpponentScarecrowETB(t *testing.T) {
	gs := newGame(t, 2)
	reaper := addPerm(gs, 0, "Reaper King", "creature", "scarecrow")
	oppScarecrow := addPerm(gs, 1, "Other Scarecrow", "creature", "scarecrow")
	victim := addPerm(gs, 1, "Grizzly Bears", "creature")
	reaperKingETB(gs, reaper, map[string]interface{}{
		"controller_seat": 1, // opponent's Scarecrow
		"perm":            oppScarecrow,
	})
	for _, p := range gs.Seats[1].Battlefield {
		if p == victim {
			return
		}
	}
	t.Errorf("Reaper must not fire on opponent's Scarecrow ETB")
}

func TestReaperKing_PrefersHighestPowerCreature(t *testing.T) {
	gs := newGame(t, 2)
	reaper := addPerm(gs, 0, "Reaper King", "creature", "scarecrow")
	scarecrow := addPerm(gs, 0, "Reaper Drone", "creature", "scarecrow")
	small := addPerm(gs, 1, "Goblin", "creature")
	small.Card.BasePower = 1
	big := addPerm(gs, 1, "Big Threat", "creature")
	big.Card.BasePower = 7
	_ = small
	reaperKingETB(gs, reaper, map[string]interface{}{
		"controller_seat": 0,
		"perm":            scarecrow,
	})
	// Big should be in graveyard.
	foundBig := false
	for _, c := range gs.Seats[1].Graveyard {
		if c != nil && c.DisplayName() == "Big Threat" {
			foundBig = true
		}
	}
	if !foundBig {
		t.Errorf("expected highest-power creature destroyed, big not in graveyard")
	}
}

func _silence(_ *gameengine.Permanent) {}
