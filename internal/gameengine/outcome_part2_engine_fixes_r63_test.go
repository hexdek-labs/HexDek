package gameengine

// r63 OUTCOME-dimension part-2 engine regressions — the widened
// interpreter's corpus catches:
//
//	#2 harmful single-target tie-break: own and opponent LANDS both
//	   scored 0, so "destroy target land" destroyed the caster's own
//	   land (74 corpus shapes: Avalanche Riders, Ark of Blight...).
//	#3 tap-state-blind tap/untap picks: untap effects landed on
//	   already-untapped permanents (Aphetto Alchemist class, 22), tap
//	   effects on already-tapped ones (Chandra's Revolution class, 7),
//	   and the multi-pick path spent untaps on OPPONENT permanents
//	   (Tezzeret the Seeker +1).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestHarmfulSingleTarget_TieBreakNeverPicksOwn(t *testing.T) {
	gs := newFixtureGame(t)
	own := addBattlefield(gs, 0, "Own Wastes", 0, 0, "land", "basic")
	opp := addBattlefield(gs, 1, "Opp Wastes", 0, 0, "land", "basic")
	src := addBattlefield(gs, 0, "Avalanche Riders", 2, 2, "creature")

	ResolveEffect(gs, src, &gameast.Destroy{
		Target: gameast.Filter{Base: "land", Quantifier: "one", Targeted: true},
	})

	stillOwn := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == own {
			stillOwn = true
		}
	}
	if !stillOwn {
		t.Fatal("destroy target land destroyed the caster's OWN land (score-tie bug)")
	}
	for _, p := range gs.Seats[1].Battlefield {
		if p == opp {
			t.Fatal("opponent's land should have been the pick")
		}
	}
}

func TestUntapEffect_PrefersOwnTappedCandidate(t *testing.T) {
	gs := newFixtureGame(t)
	ownTapped := addBattlefield(gs, 0, "Own Relic", 0, 0, "artifact")
	ownTapped.Tapped = true
	ownUntapped := addBattlefield(gs, 0, "Own Bear", 2, 2, "creature")
	src := addBattlefield(gs, 0, "Aphetto Alchemist", 1, 2, "creature")

	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "artifact or creature", Quantifier: "one", Targeted: true},
	})

	if ownTapped.Tapped {
		t.Fatal("untap effect must reach the TAPPED candidate, not no-op on an untapped pick")
	}
	_ = ownUntapped
}

func TestUntapEffect_NeverSpendsOnOpponent(t *testing.T) {
	gs := newFixtureGame(t)
	oppTapped := addBattlefield(gs, 1, "Opp Relic", 0, 0, "artifact")
	oppTapped.Tapped = true
	src := addBattlefield(gs, 0, "Tezzeret Source", 1, 1, "artifact")

	// "untap up to two target artifacts" — only tapped artifact on the
	// board belongs to the opponent: the greedy play is to decline.
	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{
			Base: "artifact", Quantifier: "up_to_n",
			Count: &gameast.NumberOrRef{IsInt: true, Int: 2}, Targeted: true,
		},
	})
	if !oppTapped.Tapped {
		t.Fatal("optional untap must not be spent on the opponent's permanent")
	}

	// Mandatory symmetric fan-out still reaches everyone (Blinkmoth
	// Infusion: "untap all artifacts").
	ResolveEffect(gs, src, &gameast.UntapEffect{
		Target: gameast.Filter{Base: "artifact", Quantifier: "all"},
	})
	if oppTapped.Tapped {
		t.Fatal("mandatory 'untap all' must untap the opponent's artifact too")
	}
}

func TestTapEffect_PrefersUntappedCandidate(t *testing.T) {
	gs := newFixtureGame(t)
	oppTapped := addBattlefield(gs, 1, "Opp Wastes A", 0, 0, "land", "basic")
	oppTapped.Tapped = true
	oppUntapped := addBattlefield(gs, 1, "Opp Wastes B", 0, 0, "land", "basic")
	src := addBattlefield(gs, 0, "Revolution Source", 1, 1, "creature")

	ResolveEffect(gs, src, &gameast.TapEffect{
		Target: gameast.Filter{Base: "land", Quantifier: "one", Targeted: true},
	})

	if !oppUntapped.Tapped {
		t.Fatal("tap effect must reach the UNTAPPED candidate, not no-op on a tapped pick")
	}
}
