package gameengine

// r63 OUTCOME-dimension finding (the judge's first corpus catch):
// "destroy up to three target creatures" filled the optional count with
// the controller's OWN creatures — including the source itself — once
// opponent candidates ran out (Armaggon, Future Shark ETB; Finale of
// Eternity; Liliana, the Necromancer -7; Sorin, Lord of Innistrad -6).
// Optional harmful picks now stop at the opponent boundary; mandatory-N
// picks keep own permanents as a last resort for targeting legality.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestPickNHarmful_UpToNeverSelfTargets(t *testing.T) {
	gs := newFixtureGame(t)
	oppBear := addBattlefield(gs, 1, "Opp Bear", 4, 4, "creature")
	ownBear := addBattlefield(gs, 0, "Own Bear", 4, 4, "creature")
	src := addBattlefield(gs, 0, "Armaggon, Future Shark", 5, 5, "creature")

	upTo3 := gameast.Filter{
		Base: "creature", Quantifier: "up_to_n",
		Count: &gameast.NumberOrRef{IsInt: true, Int: 3}, Targeted: true,
	}
	ts := PickTargetHarmful(gs, src, upTo3)
	if len(ts) != 1 {
		t.Fatalf("harmful up-to-3 with 1 opponent creature must pick exactly 1, got %d", len(ts))
	}
	if ts[0].Permanent != oppBear {
		t.Errorf("must pick the opponent's creature, got %q", ts[0].Permanent.Card.DisplayName())
	}
	_ = ownBear

	// End-to-end: the destroy resolves against only the opponent.
	ResolveEffect(gs, src, &gameast.Destroy{Target: upTo3})
	if len(gs.Seats[0].Battlefield) != 2 {
		t.Errorf("controller's board must be untouched (own bear + source), got %d permanents", len(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Errorf("opponent's creature should be destroyed, got %d remaining", len(gs.Seats[1].Battlefield))
	}
}

// Beneficial up-to picks (untap up to N) still include own permanents —
// the boundary only applies to harmful intent.
func TestPickN_BeneficialUpToStillIncludesOwn(t *testing.T) {
	gs := newFixtureGame(t)
	own := addBattlefield(gs, 0, "Own Land", 0, 0, "land", "basic")
	own.Tapped = true
	src := addBattlefield(gs, 0, "Whale Source", 4, 4, "creature")

	upTo2 := gameast.Filter{
		Base: "land", Quantifier: "up_to_n",
		Count: &gameast.NumberOrRef{IsInt: true, Int: 2},
	}
	ts := PickTarget(gs, src, upTo2) // neutral intent
	found := false
	for _, tgt := range ts {
		if tgt.Permanent == own {
			found = true
		}
	}
	if !found {
		t.Error("neutral/beneficial up-to pick must still reach own permanents")
	}
}

// Mandatory-N harmful picks may still fall back to own permanents for
// legality when opponents can't supply enough targets.
func TestPickNHarmful_MandatoryNKeepsOwnAsLastResort(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 1, "Opp Bear", 4, 4, "creature")
	addBattlefield(gs, 0, "Own Bear", 4, 4, "creature")
	src := addBattlefield(gs, 0, "Two-Target Source", 1, 1, "artifact")

	exactly2 := gameast.Filter{
		Base: "creature", Quantifier: "n",
		Count: &gameast.NumberOrRef{IsInt: true, Int: 2}, Targeted: true,
	}
	ts := PickTargetHarmful(gs, src, exactly2)
	if len(ts) != 2 {
		t.Fatalf("mandatory choose-two with only 1 opponent creature must fall back to own: got %d", len(ts))
	}
}
