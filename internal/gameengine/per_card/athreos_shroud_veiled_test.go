package per_card

import (
	"testing"
)

// r54 regression — Athreos, Shroud-Veiled's "creature with a coin
// counter dies → return under your control" trigger must not leave
// the same *Card wrapped by two Permanents on two seats' battlefields.
// Loki r53 surfaced this as the game-3107 "Eager Cadet appears in
// both seat 0 battlefield and seat 3 battlefield" CardIdentity
// violation — 206 hits in 5K (became the top-1 CardIdentity lead
// after the Mondrak r51 + Gerrard r53 fixes landed).

func TestAthreosShroudDies_StealsCardUnderControllerWithoutDualBfEntry(t *testing.T) {
	gs := newGame(t, 4)
	// Athreos on seat 3 (mirrors the game-3107 layout — seat 3's
	// commander, attempting to steal from seat 0).
	athreos := addPerm(gs, 3, "Athreos, Shroud-Veiled", "creature", "legendary")

	// Eager Cadet on seat 0's battlefield, owned by seat 0, has a coin
	// counter from a prior Athreos end-step trigger. About to die.
	cadet := addPerm(gs, 0, "Eager Cadet", "creature")
	cadet.AddCounter("coin", 1)
	cadetCard := cadet.Card

	// Simulate the death: SBA removes the Permanent from seat 0's
	// battlefield, the Card lands in seat 0's graveyard. Then the
	// "creature_dies" trigger fires the per-card hook.
	for i, p := range gs.Seats[0].Battlefield {
		if p == cadet {
			gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield[:i], gs.Seats[0].Battlefield[i+1:]...)
			break
		}
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, cadetCard)

	athreosShroudDies(gs, athreos, map[string]interface{}{
		"perm": cadet,
		"card": cadetCard,
	})

	// Cadet's *Card must be wrapped by exactly ONE Permanent total —
	// on seat 3 (Athreos's controller), not seat 0.
	totalWraps := 0
	for seatIdx, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card == cadetCard {
				totalWraps++
				if seatIdx != 3 {
					t.Errorf("REGRESSION: Cadet wrapped by Permanent on seat %d; should only be on seat 3 (Athreos's controller)", seatIdx)
				}
			}
		}
	}
	if totalWraps != 1 {
		t.Fatalf("expected exactly 1 Permanent wrapping Cadet's *Card; got %d (r53 game-3107 CardIdentity leak)", totalWraps)
	}

	// Card should no longer be in seat 0's graveyard (createPermanent
	// sweeps the owner's private zones).
	for _, c := range gs.Seats[0].Graveyard {
		if c == cadetCard {
			t.Errorf("Cadet's *Card still in seat 0 graveyard after steal — owner-sweep failed")
		}
	}
}

func TestAthreosShroudDies_SkipsWhenNoCoinCounter(t *testing.T) {
	gs := newGame(t, 2)
	athreos := addPerm(gs, 0, "Athreos, Shroud-Veiled", "creature", "legendary")
	// Dying creature with NO coin counter — should be skipped.
	cadet := addPerm(gs, 1, "Eager Cadet", "creature")
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, cadet.Card)

	preBF := len(gs.Seats[0].Battlefield)
	athreosShroudDies(gs, athreos, map[string]interface{}{
		"perm": cadet,
		"card": cadet.Card,
	})

	if len(gs.Seats[0].Battlefield) != preBF {
		t.Errorf("no coin counter → no steal; got bf delta = %d", len(gs.Seats[0].Battlefield)-preBF)
	}
}

func TestAthreosShroudDies_SkipsTokens(t *testing.T) {
	gs := newGame(t, 2)
	athreos := addPerm(gs, 0, "Athreos, Shroud-Veiled", "creature", "legendary")
	tok := addPerm(gs, 1, "Soldier Token", "creature", "token", "soldier")
	tok.AddCounter("coin", 1)
	// IsToken checks Types for "token".
	preBF := len(gs.Seats[0].Battlefield)

	athreosShroudDies(gs, athreos, map[string]interface{}{
		"perm": tok,
		"card": tok.Card,
	})

	if len(gs.Seats[0].Battlefield) != preBF {
		t.Errorf("tokens cease to exist; no steal; got bf delta = %d", len(gs.Seats[0].Battlefield)-preBF)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed: token_ceases_to_exist")
	}
}
