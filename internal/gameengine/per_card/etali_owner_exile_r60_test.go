package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// etali_owner_exile_r60_test.go — regression pin for the Etali clusters
// surfaced by the Loki r60 25K-game sweep (PR #682). Two related
// failure modes, both rooted in the same pre-PR-#683 bug:
//
//   ZoneConservation Cluster (280 / 11 games): Etali handler routed
//     each opponent's exiled card to Etali-controller's exile pile.
//     The per-seat card census then reads "N extra real cards
//     appeared" because Etali-controller is holding cards owned by
//     other seats.
//
//   CardIdentity Cluster (548 / 5 games): cards exiled by Etali
//     (and physically sitting in Etali-controller's exile) could
//     later surface in their OWNER's graveyard/library through
//     subsequent zone-change paths (refill shuffles, return-to-
//     owner cleanups), leaving the same *Card pointer in two zones
//     simultaneously.
//
// Root cause: the pre-PR-#683 handler did
//   moveCardBetweenZones(gs, seatIdx, top, "library", "library_remove", "etali_exile")
//   gs.Seats[perm.Controller].Exile = append(gs.Seats[perm.Controller].Exile, top)
// The "library_remove" string isn't a real zone, so MoveCard's
// destination branch returned "" and skipped every cleanup hook
// (card_exiled trigger, zone_change trigger, etc.). The manual
// append then cross-seat-routed every exiled card to Etali-
// controller's exile, violating CR §400.7c ("an effect that puts
// a card into a zone moves it to the corresponding zone owned by
// that player").
//
// Fix: route to each owner's exile via canonical MoveCard("library"
// → "exile"). The grant remains keyed on the *Card pointer and
// RequireController = Etali-controller seat, so the cast-from-exile
// permission still works correctly regardless of which seat's exile
// pile the card physically lives in.

// -----------------------------------------------------------------------------
// Cluster fix pin — ZoneConservation: cards land in owner's exile
// -----------------------------------------------------------------------------

func TestEtali_R60_ClusterFix_CardsRouteToOwnersExile(t *testing.T) {
	// 4-seat pod (the Loki violation pods were 4-seat). Etali at seat 1
	// attacks; each opponent's top card lands in THAT opponent's exile
	// per CR §400.7c. Etali-controller's exile receives only their own
	// exiled card. The pre-PR-#683 bug would put all 4 cards in seat 1's
	// exile and 0 in seats 0/2/3 — this test pins the correct routing.
	gs := newGame(t, 4)
	etali := addPerm(gs, 1, "Etali, Primal Storm", "creature", "legendary")
	cards := []*gameengine.Card{}
	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		c := &gameengine.Card{
			Name:  "Hostile Realm Owner" + string(rune('0'+seatIdx)),
			Owner: seatIdx,
			Types: []string{"land"},
		}
		cards = append(cards, c)
		gs.Seats[seatIdx].Library = []*gameengine.Card{c}
	}

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 1,
	})

	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		// Each seat's library is now empty.
		if len(gs.Seats[seatIdx].Library) != 0 {
			t.Errorf("seat %d library should be empty, got %d", seatIdx, len(gs.Seats[seatIdx].Library))
		}
		// Each seat's exile holds exactly its own card.
		ex := gs.Seats[seatIdx].Exile
		if len(ex) != 1 {
			t.Errorf("seat %d exile size: got %d, want 1 (owner-routed)", seatIdx, len(ex))
			continue
		}
		if ex[0] != cards[seatIdx] {
			t.Errorf("seat %d exile[0] owner mismatch: got %q owner=%d, want %q owner=%d",
				seatIdx, ex[0].DisplayName(), ex[0].Owner,
				cards[seatIdx].DisplayName(), cards[seatIdx].Owner)
		}
	}
}

// -----------------------------------------------------------------------------
// Cluster fix pin — CardIdentity: no same-*Card-in-two-zones residue
// -----------------------------------------------------------------------------

func TestEtali_R60_ClusterFix_NoSameCardInTwoZones(t *testing.T) {
	// The CardIdentity violation in game 1944 manifested as a single
	// *Card pointer ("Hostile Realm" owned by seat 0) appearing in BOTH
	// seat 0's graveyard AND seat 1's (Etali-controller's) exile after
	// multiple Etali attacks + subsequent zone-change cleanup. Post
	// PR #683 the card lives only in its OWNER's exile, so no
	// downstream cleanup path can produce the dupe.
	gs := newGame(t, 4)
	etali := addPerm(gs, 1, "Etali, Primal Storm", "creature", "legendary")
	hostileRealm := &gameengine.Card{
		Name: "Hostile Realm", Owner: 0, Types: []string{"land"},
	}
	gs.Seats[0].Library = []*gameengine.Card{hostileRealm}
	// Other seats get filler so Etali's trigger doesn't no-op on
	// empty libraries.
	for seatIdx := 1; seatIdx < 4; seatIdx++ {
		gs.Seats[seatIdx].Library = []*gameengine.Card{
			{Name: "Filler", Owner: seatIdx, Types: []string{"creature"}},
		}
	}

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 1,
	})

	// Hostile Realm must appear in exactly ONE zone: seat 0's exile.
	zonesHoldingHostileRealm := []string{}
	for seatIdx, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, c := range seat.Library {
			if c == hostileRealm {
				zonesHoldingHostileRealm = append(zonesHoldingHostileRealm, "seat"+string(rune('0'+seatIdx))+".library")
			}
		}
		for _, c := range seat.Hand {
			if c == hostileRealm {
				zonesHoldingHostileRealm = append(zonesHoldingHostileRealm, "seat"+string(rune('0'+seatIdx))+".hand")
			}
		}
		for _, c := range seat.Graveyard {
			if c == hostileRealm {
				zonesHoldingHostileRealm = append(zonesHoldingHostileRealm, "seat"+string(rune('0'+seatIdx))+".graveyard")
			}
		}
		for _, c := range seat.Exile {
			if c == hostileRealm {
				zonesHoldingHostileRealm = append(zonesHoldingHostileRealm, "seat"+string(rune('0'+seatIdx))+".exile")
			}
		}
	}
	if len(zonesHoldingHostileRealm) != 1 {
		t.Errorf("Hostile Realm *Card must appear in exactly 1 zone, got %d: %v",
			len(zonesHoldingHostileRealm), zonesHoldingHostileRealm)
	}
	if len(zonesHoldingHostileRealm) == 1 && zonesHoldingHostileRealm[0] != "seat0.exile" {
		t.Errorf("Hostile Realm should be in seat 0 exile (owner-routed), got %q",
			zonesHoldingHostileRealm[0])
	}
}

// -----------------------------------------------------------------------------
// Per-seat census stability — defends against ZoneConservation regression
// -----------------------------------------------------------------------------

func TestEtali_R60_ClusterFix_PerSeatCensusStable(t *testing.T) {
	// Pre-PR-#683 bug: every Etali attack ADDED foreign cards to
	// seat 1's (Etali-controller's) exile pile while leaving the
	// owner seats' exiles empty. The per-seat card census would
	// then show seat 1 holding (own cards + N foreign cards),
	// triggering the ZoneConservation "N extra real cards appeared"
	// invariant in the 25K Loki sweep. This test pins that after
	// the fix, the per-seat card-count delta from a single Etali
	// attack is exactly +1 per seat (each seat's own card moved
	// library → exile), NOT +N in Etali-controller and +0
	// elsewhere.
	gs := newGame(t, 4)
	etali := addPerm(gs, 1, "Etali, Primal Storm", "creature", "legendary")
	// Pre-PR-#683 the buggy handler bypassed MoveCard's full machinery
	// (the "library_remove" string fell through moveToZone's default
	// case and silently routed to graveyard) so SBA never ran. The
	// fixed handler routes through canonical MoveCard("library" →
	// "exile"), which fires FireZoneChangeTriggers + SBA-eligible
	// cascades — that means Etali's stub 0/0 P/T (from addPerm) would
	// die to SBA §704.5f during the trigger's PriorityRound. Give Etali
	// real Base P/T so it survives long enough for the test to inspect
	// post-state censuses.
	etali.Card.BasePower = 6
	etali.Card.BaseToughness = 6

	// Pre-attack: seed each seat with 5 library cards. Total cards
	// per seat = 5 (library only; ignore Etali for the census since
	// it's a battlefield perm).
	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		gs.Seats[seatIdx].Library = nil
		for i := 0; i < 5; i++ {
			gs.Seats[seatIdx].Library = append(gs.Seats[seatIdx].Library,
				&gameengine.Card{Name: "L" + string(rune('0'+seatIdx)) + string(rune('0'+i)),
					Owner: seatIdx, Types: []string{"creature"}})
		}
	}

	preCensus := [4]int{}
	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		s := gs.Seats[seatIdx]
		preCensus[seatIdx] = len(s.Library) + len(s.Hand) + len(s.Graveyard) + len(s.Exile)
	}

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 1,
	})

	// Post-attack: each seat moves 1 card library → its own exile.
	// Net per-seat census delta = 0 for every seat. The ZoneConservation
	// invariant in Loki measures EXACTLY this — if Etali-controller
	// ends up with foreign cards in exile, that seat's census shows
	// +N while owner seats show -N.
	for seatIdx := 0; seatIdx < 4; seatIdx++ {
		s := gs.Seats[seatIdx]
		post := len(s.Library) + len(s.Hand) + len(s.Graveyard) + len(s.Exile)
		if post != preCensus[seatIdx] {
			t.Errorf("seat %d census drift: pre=%d post=%d (Δ=%+d) — ZoneConservation regression risk",
				seatIdx, preCensus[seatIdx], post, post-preCensus[seatIdx])
		}
		// Specifically: library lost 1, exile gained 1, others unchanged.
		if len(s.Library) != 4 {
			t.Errorf("seat %d library: got %d, want 4 (lost 1 to exile)", seatIdx, len(s.Library))
		}
		if len(s.Exile) != 1 {
			t.Errorf("seat %d exile: got %d, want 1 (gained its own top library card)", seatIdx, len(s.Exile))
		}
	}
}

// -----------------------------------------------------------------------------
// Grant lookup independence — RequireController works regardless of pile
// -----------------------------------------------------------------------------

func TestEtali_R60_ClusterFix_GrantUsableFromOwnersExile(t *testing.T) {
	// The pre-PR-#683 comment in the handler claimed the cross-seat
	// routing was needed because "the exile zone is per-seat... we
	// need them on Etali's controller's pile so RequireController
	// matches." That was wrong — the grant is keyed on the *Card
	// pointer (gs.ZoneCastGrants[*Card]), and RequireController is
	// a seat field on the grant, not a zone-residence requirement.
	// This test pins that a grant created with RequireController =
	// Etali-controller continues to work even when the card lives
	// in the owner's exile (different seat).
	gs := newGame(t, 2)
	etali := addPerm(gs, 0, "Etali, Primal Storm", "creature", "legendary")

	// Seat 1's top is a nonland — gets exiled to SEAT 1's exile
	// (owner-routed) with a grant pointing at seat 0 as RequireController.
	nonland := &gameengine.Card{Name: "Lightning Bolt", Owner: 1, Types: []string{"instant"}}
	gs.Seats[1].Library = append(gs.Seats[1].Library, nonland)

	gameengine.FireCardTrigger(gs, "creature_attacks", map[string]interface{}{
		"attacker_perm": etali,
		"attacker_seat": 0,
	})

	// Card landed in seat 1 (owner) exile.
	if len(gs.Seats[1].Exile) != 1 || gs.Seats[1].Exile[0] != nonland {
		t.Fatal("Lightning Bolt should be in seat 1 (owner) exile")
	}
	// Grant keyed on the *Card pointer, RequireController = 0 (Etali).
	grant, ok := gs.ZoneCastGrants[nonland]
	if !ok || grant == nil {
		t.Fatal("expected grant on Lightning Bolt, got nil")
	}
	if grant.RequireController != 0 {
		t.Errorf("RequireController = %d, want 0 (Etali controller, regardless of physical exile pile)",
			grant.RequireController)
	}
}
