package per_card

import (
	"testing"
)

func TestMikaeusLunarch_ETBWithXCountersFromFlag(t *testing.T) {
	gs := newGame(t, 2)
	mikaeus := addPerm(gs, 0, "Mikaeus, the Lunarch", "creature")
	mikaeus.Flags["x_paid"] = 4
	mikaeusLunarchETB(gs, mikaeus)
	if mikaeus.Counters["+1/+1"] != 4 {
		t.Errorf("expected 4 +1/+1 counters from x_paid=4, got %d", mikaeus.Counters["+1/+1"])
	}
}

func TestMikaeusLunarch_ETBNoXPaidEmitsPartial(t *testing.T) {
	gs := newGame(t, 2)
	mikaeus := addPerm(gs, 0, "Mikaeus, the Lunarch", "creature")
	// x_paid not set — defensive zero.
	mikaeusLunarchETB(gs, mikaeus)
	if mikaeus.Counters["+1/+1"] != 0 {
		t.Errorf("expected 0 counters when x_paid missing")
	}
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "mikaeus_lunarch_etb_x_counters" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial for x_paid threading gap")
	}
}

func TestMikaeusLunarch_ActivatedSelfPump(t *testing.T) {
	gs := newGame(t, 2)
	mikaeus := addPerm(gs, 0, "Mikaeus, the Lunarch", "creature")
	mikaeusLunarchActivate(gs, mikaeus, 0, nil)
	if mikaeus.Counters["+1/+1"] != 1 {
		t.Errorf("expected 1 counter after self-pump, got %d", mikaeus.Counters["+1/+1"])
	}
}

func TestMikaeusLunarch_SpreadCountersToOtherCreatures(t *testing.T) {
	gs := newGame(t, 2)
	mikaeus := addPerm(gs, 0, "Mikaeus, the Lunarch", "creature")
	mikaeus.Counters["+1/+1"] = 3
	other1 := addPerm(gs, 0, "Llanowar Elves", "creature")
	other2 := addPerm(gs, 0, "Birds of Paradise", "creature")
	notACreature := addPerm(gs, 0, "Sol Ring", "artifact")

	mikaeusLunarchActivate(gs, mikaeus, 1, nil)

	if mikaeus.Counters["+1/+1"] != 2 {
		t.Errorf("expected Mikaeus down to 2 counters (paid 1), got %d", mikaeus.Counters["+1/+1"])
	}
	if other1.Counters["+1/+1"] != 1 || other2.Counters["+1/+1"] != 1 {
		t.Errorf("expected each other creature to gain 1 counter, got %d and %d",
			other1.Counters["+1/+1"], other2.Counters["+1/+1"])
	}
	if notACreature.Counters["+1/+1"] != 0 {
		t.Errorf("non-creature must not gain a counter, got %d", notACreature.Counters["+1/+1"])
	}
}

func TestMikaeusLunarch_SpreadCountersRefusedWithNoCounters(t *testing.T) {
	gs := newGame(t, 2)
	mikaeus := addPerm(gs, 0, "Mikaeus, the Lunarch", "creature")
	other := addPerm(gs, 0, "Llanowar Elves", "creature")
	mikaeusLunarchActivate(gs, mikaeus, 1, nil)
	if other.Counters["+1/+1"] != 0 {
		t.Errorf("activation must fail when Mikaeus has no counter to remove")
	}
}
