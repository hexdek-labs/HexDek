package gameengine

import (
	"testing"
)

// CR §702.94 — GRANTED miracle + {0} cost edge (follow-up to the printed-
// miracle wiring). A permanent granting all hand cards "miracle {0}" must
// NOT let a player cast their whole hand for free: §702.94a still gates it,
// so only the ONE card that is the first card drawn this turn opens a {0}
// miracle window. Cards already in hand were never "drawn this turn".

// registerAllHandMiracleZero registers a grant that gives every card in
// seat's hand miracle {0}, sourced from a permanent on the battlefield —
// modeling the flagged card.
func registerAllHandMiracleZero(gs *GameState, seat int) *Permanent {
	src := addKWCombatBattlefield(gs, seat, "Miracle Engine", 0, 0, "enchantment")
	gs.RegisterMiracleGrant(&MiracleGrant{
		SourcePerm:     src,
		HandlerID:      "test_all_hand_miracle_zero",
		ControllerSeat: seat,
		Cost:           0,
		Predicate:      nil, // all cards
	})
	return src
}

// (a)+(d): first card drawn this turn is castable for the granted {0} cost,
// paying NO mana (free cast).
func TestGrantedMiracle_FirstDrawCastableForZero(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[0].ManaPool = 0 // prove the cast needs no mana
	registerAllHandMiracleZero(gs, 0)

	// A plain (no native miracle) card — miracle is purely granted.
	card := plainSpell("Granted Bolt")
	gs.Seats[0].Library = []*Card{card}

	drawn, ok := gs.drawOne(0)
	if !ok || drawn != card {
		t.Fatalf("expected to draw the card, ok=%v", ok)
	}
	if !MiracleWindowOpen(gs, card) {
		t.Fatal("granted miracle should open a window on the first card drawn this turn")
	}
	if !CanCastMiracle(gs, 0, card) {
		t.Fatal("first-drawn card under an all-hand grant should be miracle-castable")
	}
	if err := CastWithMiracle(gs, 0, card, nil); err != nil {
		t.Fatalf("granted {0} miracle cast should succeed: %v", err)
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("granted {0} miracle must be FREE; mana pool=%d (want 0)", gs.Seats[0].ManaPool)
	}
	for _, c := range gs.Seats[0].Hand {
		if c == card {
			t.Error("cast card should have left hand")
		}
	}
}

// (b) THE GATE: the rest of the hand (cards NOT drawn this turn) is NOT
// free-castable, even though the grant covers every card in hand.
func TestGrantedMiracle_RestOfHandNotFreeCastable(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[0].ManaPool = 0
	registerAllHandMiracleZero(gs, 0)

	// Five cards already in hand (drawn on prior turns / opening hand).
	preHand := []*Card{
		plainSpell("h1"), plainSpell("h2"), plainSpell("h3"),
		plainSpell("h4"), plainSpell("h5"),
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, preHand...)

	// Draw one card this turn — only THIS one may open a window.
	drawnCard := plainSpell("topdeck")
	gs.Seats[0].Library = []*Card{drawnCard}
	gs.drawOne(0)

	if !CanCastMiracle(gs, 0, drawnCard) {
		t.Fatal("the one card drawn this turn should be miracle-eligible")
	}
	eligible := 0
	for _, c := range gs.Seats[0].Hand {
		if CanCastMiracle(gs, 0, c) {
			eligible++
		}
	}
	if eligible != 1 {
		t.Fatalf("exactly ONE card (the first draw) may be miracle-cast; got %d eligible — free-hand-dump bug", eligible)
	}
	// Every pre-existing hand card must be rejected for a miracle cast.
	for _, c := range preHand {
		if CanCastMiracle(gs, 0, c) {
			t.Errorf("pre-existing hand card %q must NOT be miracle-castable", c.Name)
		}
		if err := CastWithMiracle(gs, 0, c, nil); err == nil {
			t.Errorf("CastWithMiracle on non-drawn hand card %q must fail", c.Name)
		}
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Errorf("no free casts of the rest of hand should have occurred; mana pool=%d", gs.Seats[0].ManaPool)
	}
}

// (c) second+ draws this turn are not eligible, even under an all-hand grant.
func TestGrantedMiracle_SecondDrawNotEligible(t *testing.T) {
	gs := newMiracleGame(t, 2)
	gs.Seats[0].ManaPool = 0
	registerAllHandMiracleZero(gs, 0)

	first := plainSpell("first")
	second := plainSpell("second")
	gs.Seats[0].Library = []*Card{first, second}

	gs.drawOne(0) // first draw → window
	gs.drawOne(0) // second draw → no window

	if !CanCastMiracle(gs, 0, first) {
		t.Error("first draw should be eligible")
	}
	if MiracleWindowOpen(gs, second) {
		t.Error("second draw must NOT open a window under a grant")
	}
	if CanCastMiracle(gs, 0, second) {
		t.Error("second-drawn card must not be miracle-castable")
	}
	if err := CastWithMiracle(gs, 0, second, nil); err == nil {
		t.Error("CastWithMiracle on a second-drawn card must fail")
	}
}

// Without any grant (and no native miracle), drawing a plain card opens no
// window — the grant is what enables it, and only for the first draw.
func TestGrantedMiracle_NoGrantNoWindow(t *testing.T) {
	gs := newMiracleGame(t, 2)
	card := plainSpell("ordinary")
	gs.Seats[0].Library = []*Card{card}
	gs.drawOne(0)
	if MiracleWindowOpen(gs, card) {
		t.Error("a plain card with no grant and no native miracle must not open a window")
	}
	if CanCastMiracle(gs, 0, card) {
		t.Error("plain card must not be miracle-castable without a grant")
	}
}

// When the granting permanent leaves the battlefield, the grant stops:
// a first draw afterward opens no window. Also covers the source-on-bf
// defensive check in grantedMiracleCost.
func TestGrantedMiracle_SourceLeavesStopsGrant(t *testing.T) {
	gs := newMiracleGame(t, 2)
	src := registerAllHandMiracleZero(gs, 0)

	// Source leaves: unregister + remove from battlefield.
	gs.UnregisterMiracleGrantsForPermanent(src)
	gs.Seats[0].Battlefield = nil

	card := plainSpell("after")
	gs.Seats[0].Library = []*Card{card}
	gs.drawOne(0)
	if MiracleWindowOpen(gs, card) {
		t.Error("no grant should be active after the source left; window must not open")
	}
}

// Defensive: even if a handler forgets to unregister, a grant whose source
// is no longer on the battlefield is ignored.
func TestGrantedMiracle_StaleSourceIgnored(t *testing.T) {
	gs := newMiracleGame(t, 2)
	src := registerAllHandMiracleZero(gs, 0)
	// Source removed from battlefield but grant left registered (stale).
	gs.Seats[0].Battlefield = nil
	_ = src

	card := plainSpell("after-stale")
	gs.Seats[0].Library = []*Card{card}
	gs.drawOne(0)
	if MiracleWindowOpen(gs, card) {
		t.Error("a grant whose source is off the battlefield must be ignored")
	}
}
