package gameengine

import (
	"testing"
)

// r63 depth-frontier (loki seed 99 / game 3189, max-turns 120):
// CardIdentity violation "Bruna, the Fading Light appears in both seat 0
// graveyard and seat 0 exile" (106 hits). Root cause: Meld() deposited
// the two component cards into the real EXILE zone (FireZoneChange ->
// moveToZone) instead of leaving them in "merged limbo" represented by
// the melded permanent. When the melded permanent (Brisela) later died,
// UnmergeOnLeavePlay routed the same *Card to the graveyard while the
// stale exile entry remained — the card lived in two zones at once.
//
// CR §712: meld absorbs the components into the single melded permanent;
// they have no independent zone until it leaves play, at which point they
// separate into the destination zone.

func meldComponentInExile(gs *GameState, c *Card) bool {
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, e := range seat.Exile {
			if e == c {
				return true
			}
		}
	}
	return false
}

func meldComponentInGraveyard(gs *GameState, c *Card) bool {
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, g := range seat.Graveyard {
			if g == c {
				return true
			}
		}
	}
	return false
}

func TestMeld_ComponentsLiveInLimboNotExile(t *testing.T) {
	gs := newPhase8GameState(t)
	bruna := makePhase8Creature(gs, 0, "Bruna, the Fading Light", 5, 7)
	gisela := makePhase8Creature(gs, 0, "Gisela, the Broken Blade", 4, 3)
	pBruna := putOnBattlefield(gs, bruna)
	pGisela := putOnBattlefield(gs, gisela)

	melded := Meld(gs, pBruna, pGisela)
	if melded == nil {
		t.Fatalf("Meld returned nil")
	}

	// Components must NOT be deposited loose in the exile zone — they are
	// in merged limbo, tracked by the melded permanent's MergedCardPtrs.
	if meldComponentInExile(gs, bruna) {
		t.Fatalf("Bruna should be in merged limbo, not the exile zone")
	}
	if meldComponentInExile(gs, gisela) {
		t.Fatalf("Gisela should be in merged limbo, not the exile zone")
	}
	if melded.MergedCardPtrs[bruna.InstanceID] != bruna {
		t.Fatalf("Bruna should be tracked in melded.MergedCardPtrs")
	}
	if melded.MergedCardPtrs[gisela.InstanceID] != gisela {
		t.Fatalf("Gisela should be tracked in melded.MergedCardPtrs")
	}
}

func TestMeld_DeathRoutesComponentsToGraveyardWithoutDuplicating(t *testing.T) {
	gs := newPhase8GameState(t)
	bruna := makePhase8Creature(gs, 0, "Bruna, the Fading Light", 5, 7)
	gisela := makePhase8Creature(gs, 0, "Gisela, the Broken Blade", 4, 3)
	pBruna := putOnBattlefield(gs, bruna)
	pGisela := putOnBattlefield(gs, gisela)

	melded := Meld(gs, pBruna, pGisela)
	if melded == nil {
		t.Fatalf("Meld returned nil")
	}

	// Brisela dies — components separate into the graveyard (§712.3), each
	// exactly once, and must NOT also linger in exile.
	DestroyPermanent(gs, melded, nil)

	if meldComponentInExile(gs, bruna) || meldComponentInExile(gs, gisela) {
		t.Fatalf("meld components must not remain in exile after the melded permanent dies (CardIdentity dup)")
	}
	if !meldComponentInGraveyard(gs, bruna) {
		t.Fatalf("Bruna should be in the graveyard after Brisela dies")
	}
	if !meldComponentInGraveyard(gs, gisela) {
		t.Fatalf("Gisela should be in the graveyard after Brisela dies")
	}

	// And no zone holds either card twice.
	if countCardOccurrences(gs, bruna) != 1 {
		t.Fatalf("Bruna present %d times across zones; want exactly 1", countCardOccurrences(gs, bruna))
	}
	if countCardOccurrences(gs, gisela) != 1 {
		t.Fatalf("Gisela present %d times across zones; want exactly 1", countCardOccurrences(gs, gisela))
	}
}

// countCardOccurrences tallies how many of the six real zones (across all
// seats) hold this exact *Card pointer — the CardIdentity invariant trips
// when this exceeds 1.
func countCardOccurrences(gs *GameState, c *Card) int {
	n := 0
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, x := range seat.Library {
			if x == c {
				n++
			}
		}
		for _, x := range seat.Hand {
			if x == c {
				n++
			}
		}
		for _, x := range seat.Graveyard {
			if x == c {
				n++
			}
		}
		for _, x := range seat.Exile {
			if x == c {
				n++
			}
		}
		for _, x := range seat.CommandZone {
			if x == c {
				n++
			}
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Card == c {
				n++
			}
		}
	}
	return n
}
