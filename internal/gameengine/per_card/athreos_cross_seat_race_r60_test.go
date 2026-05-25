package per_card

import (
	"testing"
)

// Loki r60 deep-stress / seed 2024 game 2798 turn 36 cleanup surfaced:
//
//   CardIdentity: card "Woodland Liege" (ptr 0xc006695440) appears in
//   both seat 2 battlefield and seat 3 battlefield  (24 hits)
//
// Event trail showed Athreos, Shroud-Veiled triggers firing TWICE in
// succession from seats 2 and 3 on the same creature_dies event —
// both controllers had placed a coin counter on the dying creature,
// so both Athreoses triggered. The first handler claimed the *Card
// from owner's graveyard and placed a Permanent on its battlefield;
// the second handler raced — the *Card was no longer in graveyard,
// but createPermanent's dedup only checks the TARGET seat's
// battlefield, so the race-loser still wrapped the already-stolen
// *Card on its own battlefield. Result: same *Card pointer on two
// seats' battlefields = CardIdentity violation.
//
// Same race surface as the Adric / Oketra / Gisa / The Reaper §704.6d
// fixes. Fix: athreosShroudDies validates the dying card is still in
// owner's graveyard before delegating to enterBattlefieldWithETB. If
// absent, a sibling Athreos won the race — no-op.

// TestAthreosCrossSeat_LoserNoOpsWhenCardAlreadyStolen pins the
// game-2798 shape: two Athreos handlers fire in sequence on the same
// dying creature. The second must no-op because the first already
// claimed the card.
func TestAthreosCrossSeat_LoserNoOpsWhenCardAlreadyStolen(t *testing.T) {
	gs := newGame(t, 4)

	// Two Athreos, Shroud-Veiled on seats 2 and 3 (matches the
	// game-2798 seat layout).
	athreos2 := addPerm(gs, 2, "Athreos, Shroud-Veiled", "creature", "legendary")
	athreos3 := addPerm(gs, 3, "Athreos, Shroud-Veiled", "creature", "legendary")

	// Woodland Liege on seat 0's battlefield (matches game-2798 — the
	// liege dies between the two Athreos triggers). Has a coin counter
	// from a prior Athreos end-step trigger; about to die.
	liege := addPerm(gs, 0, "Woodland Liege", "creature")
	liege.AddCounter("coin", 1)
	liegeCard := liege.Card

	// Simulate death: SBA removes the Permanent and adds the *Card to
	// seat 0's graveyard.
	for i, p := range gs.Seats[0].Battlefield {
		if p == liege {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, liegeCard)

	// First Athreos (seat 2) fires — should successfully steal.
	athreosShroudDies(gs, athreos2, map[string]interface{}{
		"perm": liege,
		"card": liegeCard,
	})

	// Verify the liege is now on seat 2's battlefield and removed
	// from seat 0's graveyard.
	on2 := false
	for _, p := range gs.Seats[2].Battlefield {
		if p != nil && p.Card == liegeCard {
			on2 = true
			break
		}
	}
	if !on2 {
		t.Fatal("first Athreos must successfully steal the dying Liege onto seat 2")
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == liegeCard {
			t.Fatal("dying Liege must be removed from owner's graveyard after first steal")
		}
	}

	// SECOND Athreos (seat 3) fires on the same event — must NO-OP
	// because the card is no longer in graveyard.
	athreosShroudDies(gs, athreos3, map[string]interface{}{
		"perm": liege,
		"card": liegeCard,
	})

	// Pin the invariant: exactly ONE Permanent total wraps the *Card,
	// on seat 2 (the race winner), NOT seat 3.
	totalWraps := 0
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == liegeCard {
				totalWraps++
				if seatIdx != 2 {
					t.Errorf("REGRESSION: Liege wrapped by Permanent on seat %d; should only be on seat 2 (race winner)", seatIdx)
				}
			}
		}
	}
	if totalWraps != 1 {
		t.Fatalf("expected exactly 1 Permanent wrapping Liege *Card; got %d (Athreos cross-seat race leak)", totalWraps)
	}
}

// TestAthreosCrossSeat_BothHandlersFireInOrderEitherWayClean swaps
// the order — Athreos on seat 3 wins, Athreos on seat 2 loses. Same
// invariant: exactly one Permanent on the winner's seat.
func TestAthreosCrossSeat_BothHandlersFireInOrderEitherWayClean(t *testing.T) {
	gs := newGame(t, 4)
	athreos2 := addPerm(gs, 2, "Athreos, Shroud-Veiled", "creature", "legendary")
	athreos3 := addPerm(gs, 3, "Athreos, Shroud-Veiled", "creature", "legendary")

	liege := addPerm(gs, 0, "Woodland Liege", "creature")
	liege.AddCounter("coin", 1)
	liegeCard := liege.Card
	for i, p := range gs.Seats[0].Battlefield {
		if p == liege {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, liegeCard)

	// Reverse order: seat 3 first.
	athreosShroudDies(gs, athreos3, map[string]interface{}{"perm": liege, "card": liegeCard})
	athreosShroudDies(gs, athreos2, map[string]interface{}{"perm": liege, "card": liegeCard})

	totalWraps := 0
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == liegeCard {
				totalWraps++
				if seatIdx != 3 {
					t.Errorf("REGRESSION: Liege wrapped on seat %d; should be seat 3 (race winner)", seatIdx)
				}
			}
		}
	}
	if totalWraps != 1 {
		t.Fatalf("expected exactly 1 Permanent wrapping Liege *Card; got %d", totalWraps)
	}
}

// TestAthreosCrossSeat_SingleAthreosUnaffected pins that the new
// "still in graveyard" gate doesn't regress the single-Athreos case
// (the original r54 fix surface). One Athreos, one dying creature
// with a coin counter, no race — must still successfully steal.
func TestAthreosCrossSeat_SingleAthreosUnaffected(t *testing.T) {
	gs := newGame(t, 4)
	athreos := addPerm(gs, 3, "Athreos, Shroud-Veiled", "creature", "legendary")

	cadet := addPerm(gs, 0, "Eager Cadet", "creature")
	cadet.AddCounter("coin", 1)
	cadetCard := cadet.Card
	for i, p := range gs.Seats[0].Battlefield {
		if p == cadet {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cadetCard)

	athreosShroudDies(gs, athreos, map[string]interface{}{"perm": cadet, "card": cadetCard})

	on3 := false
	for _, p := range gs.Seats[3].Battlefield {
		if p != nil && p.Card == cadetCard {
			on3 = true
			break
		}
	}
	if !on3 {
		t.Fatal("single-Athreos path must still steal the dying card")
	}
}
