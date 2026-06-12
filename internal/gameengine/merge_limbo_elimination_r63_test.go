package gameengine

// merge_limbo_elimination_r63_test.go — CONSERVATION rare-residual class
// (r63): §800.4a vs merge limbo.
//
// Mutate (§702.140) and Meld (§712) absorb constituent *Cards into a
// surviving Permanent's MergedCardPtrs — "merge limbo", a card container
// that is not a zone. The strict-census walk sees limbo cards only
// through battlefield-resident permanents, and HandleSeatElimination's
// battlefield sweep removed merged permanents WITHOUT draining the
// stack. Three shapes, all the same root pattern as the earlier
// ResolvingCards fix (§800.4a must reach every card container, and no
// mover may materialize a card into a departed seat's zones):
//
//  1. Leaver-owned merged permanent swept → constituents stranded in
//     limbo, never ceased → strict-census disappearance.
//  2. Mixed-owner stack (engine-loose; CR 702.140e makes it rare) on a
//     leaver-owned permanent → a SURVIVING player's constituent
//     vanishes from all zone accounting.
//  3. Constituent owner eliminated while the merge survives → a later
//     unmerge (UnmergeOnLeavePlay) appends the card to the dead seat's
//     zone, which the census skips → disappearance.

import (
	"testing"
)

// mutatedStack builds a merged permanent on hostSeat: base card owned by
// hostSeat, rider card owned by riderOwner, rider absorbed into limbo
// (removed from the battlefield, pointer retained in MergedCardPtrs) —
// the state RecordMutateMerge + the mutate handler produce.
func mutatedStack(t *testing.T, gs *GameState, hostSeat, riderOwner int) (*Permanent, *Card) {
	t.Helper()
	base := addBattlefield(gs, hostSeat, "Mutate Base", 2, 2, "creature")
	base.Owner = hostSeat
	MintOGInstanceID(gs, base.Card)

	rider := addBattlefield(gs, hostSeat, "Mutate Rider", 3, 3, "creature")
	rider.Owner = hostSeat
	rider.Card.Owner = riderOwner
	MintOGInstanceID(gs, rider.Card)

	RecordMutateMerge(gs, base, rider, true)

	// The mutate handler removes the dying permanent from the
	// battlefield; its card lives on only in base.MergedCardPtrs.
	kept := gs.Seats[hostSeat].Battlefield[:0]
	for _, p := range gs.Seats[hostSeat].Battlefield {
		if p != rider {
			kept = append(kept, p)
		}
	}
	gs.Seats[hostSeat].Battlefield = kept

	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Fatalf("fixture must start census-clean (rider visible via MergedCardPtrs): ok=%v err=%v", ok, err)
	}
	return base, rider.Card
}

// inAnyZone reports whether c sits in any of seat's card zones.
func inAnyZone(s *Seat, c *Card) bool {
	for _, zone := range [][]*Card{s.Library, s.Hand, s.Graveyard, s.Exile, s.CommandZone} {
		for _, zc := range zone {
			if zc == c {
				return true
			}
		}
	}
	for _, p := range s.Battlefield {
		if p != nil && p.Card == c {
			return true
		}
	}
	return false
}

// Shape 1: the leaver's own merged permanent is swept — its limbo
// constituents leave the game with him (ceased), not stranded.
func TestElimination_MergedPermDrained_LeaverConstituentCeases(t *testing.T) {
	gs := newFixtureGame(t)
	_, riderCard := mutatedStack(t, gs, 1, 1)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[riderCard.InstanceID]; !ceased {
		t.Error("leaver-owned limbo constituent must cease per §800.4a")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after elimination: ok=%v err=%v", ok, err)
	}
}

// Shape 2: a SURVIVOR-owned constituent inside the leaver's merged
// permanent must not vanish — it routes to its owner's exile alongside
// the base-card §800.4c arm, and is NOT ceased.
func TestElimination_MergedPermDrained_SurvivorConstituentExiled(t *testing.T) {
	gs := newFixtureGame(t)
	_, riderCard := mutatedStack(t, gs, 1, 0)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[riderCard.InstanceID]; ceased {
		t.Error("survivor-owned constituent must NOT cease — its owner is still in the game")
	}
	if !inAnyZone(gs.Seats[0], riderCard) {
		t.Error("survivor-owned constituent stranded in merge limbo — must land in its owner's exile")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after elimination: ok=%v err=%v", ok, err)
	}
}

// Shape 3a: the merge SURVIVES on a living seat but a constituent's
// owner is eliminated — the constituent leaves the game at elimination
// time (ceased + stripped from the merge bookkeeping), so no later
// unmerge can materialize it into the dead seat's zones.
func TestElimination_SurvivingMergedPerm_LeaverConstituentStripped(t *testing.T) {
	gs := newFixtureGame(t)
	base, riderCard := mutatedStack(t, gs, 0, 1)

	HandleSeatElimination(gs, 1)

	if _, ceased := gs.CeasedInstanceIDs[riderCard.InstanceID]; !ceased {
		t.Error("leaver-owned constituent inside a surviving merge must cease per §800.4a")
	}
	if _, held := base.MergedCardPtrs[riderCard.InstanceID]; held {
		t.Error("ceased constituent must be stripped from MergedCardPtrs")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after elimination: ok=%v err=%v", ok, err)
	}

	// The later unmerge must not resurrect it.
	DestroyPermanent(gs, base, nil)
	if inAnyZone(gs.Seats[1], riderCard) {
		t.Error("ceased constituent reappeared in the dead seat's zones after unmerge")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after unmerge: ok=%v err=%v", ok, err)
	}
}

// Shape 3b — defense in depth for UnmergeOnLeavePlay itself: even when
// a dead-owner constituent somehow survives in limbo (a path that
// bypassed HandleSeatElimination's sweeps), unmerge must cease it
// rather than appending it to the departed seat's graveyard.
func TestUnmergeOnLeavePlay_DeadOwnerConstituent_CeasesNotDeadZone(t *testing.T) {
	gs := newFixtureGame(t)
	base, riderCard := mutatedStack(t, gs, 0, 1)

	// Mark the seat departed WITHOUT running the elimination sweeps.
	gs.Seats[1].LeftGame = true

	DestroyPermanent(gs, base, nil)

	if inAnyZone(gs.Seats[1], riderCard) {
		t.Error("unmerge materialized a card into a departed seat's zones")
	}
	if _, ceased := gs.CeasedInstanceIDs[riderCard.InstanceID]; !ceased {
		t.Error("dead-owner constituent must cease at unmerge per §800.4a")
	}
	if err, ok := ZoneConservationStrict(gs); !ok || err != nil {
		t.Errorf("strict census must be clean after unmerge: ok=%v err=%v", ok, err)
	}
}
