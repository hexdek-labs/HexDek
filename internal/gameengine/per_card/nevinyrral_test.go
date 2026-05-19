package per_card

// Loki r45 — Nevinyrral, Urborg Tyrant ZoneConservation regression.
//
// The original handler rebuilt seat.Battlefield by accumulating a `keep`
// slice of non-victim permanents and reassigning, then emitted a
// per-victim "destroy" event but NEVER moved the destroyed *Card to the
// owner's graveyard. The card pointer simply vanished, dropping the
// total real-card count by N. Loki r44/r45 caught this as a
// ZoneConservation cluster (8 real cards disappeared per fire,
// 124 hits total across games 333 / 426 / 472).
//
// The fix routes every destroy through gameengine.DestroyPermanent —
// the canonical battlefield-exit API that runs §614 replacements, fires
// dies/LTB triggers, and lands the card in the owner's graveyard. These
// tests pin the post-ETB zone state.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestNevinyrral_ETBMovesVictimsToGraveyard(t *testing.T) {
	gs := newGame(t, 4)

	// Seat 0: Nevinyrral, plus a Forest (land, should survive).
	nev := addPerm(gs, 0, "Nevinyrral, Urborg Tyrant", "legendary", "creature")
	survivorLand := addPerm(gs, 0, "Forest", "land", "basic")

	// Seat 1: artifact + enchantment + creature (all should die).
	art := addPerm(gs, 1, "Random Artifact", "artifact")
	ench := addPerm(gs, 1, "Random Enchantment", "enchantment")
	crea := addPerm(gs, 1, "Random Creature", "creature")

	// Seat 2: land + planeswalker — both should survive (Nevinyrral
	// only destroys artifacts / creatures / enchantments).
	survivorPW := addPerm(gs, 2, "Random Walker", "planeswalker")
	survivorLand2 := addPerm(gs, 2, "Plains", "land", "basic")

	gameengine.InvokeETBHook(gs, nev)

	// All three victims must now live in their owner's graveyard,
	// not have vanished. This is the zone-conservation contract that
	// the pre-r45 inline-keep loop violated.
	for _, vc := range []struct {
		card *gameengine.Card
		seat int
		name string
	}{
		{art.Card, 1, "artifact"},
		{ench.Card, 1, "enchantment"},
		{crea.Card, 1, "creature"},
	} {
		found := false
		for _, c := range gs.Seats[vc.seat].Graveyard {
			if c == vc.card {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s (%q) in seat %d graveyard after Nevinyrral ETB; not found",
				vc.name, vc.card.Name, vc.seat)
		}
	}

	// Source must still be on its battlefield (Nevinyrral spares itself).
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nev {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Fatal("Nevinyrral, Urborg Tyrant destroyed itself — should spare source")
	}

	// Survivors stay on their battlefields.
	for _, sc := range []struct {
		perm *gameengine.Permanent
		seat int
		kind string
	}{
		{survivorLand, 0, "controller's land"},
		{survivorPW, 2, "opponent's planeswalker"},
		{survivorLand2, 2, "opponent's land"},
	} {
		ok := false
		for _, p := range gs.Seats[sc.seat].Battlefield {
			if p == sc.perm {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%s was unexpectedly destroyed", sc.kind)
		}
	}
}

func TestNevinyrral_ZoneCountConserved(t *testing.T) {
	gs := newGame(t, 4)

	nev := addPerm(gs, 0, "Nevinyrral, Urborg Tyrant", "legendary", "creature")
	addPerm(gs, 1, "Toll Booth", "artifact")
	addPerm(gs, 1, "Curse of Frenzy", "enchantment")
	addPerm(gs, 2, "Lurker", "creature")
	addPerm(gs, 3, "Brawler", "creature")
	addPerm(gs, 3, "Sigil", "enchantment")

	// Pre-ETB census: count every real *Card across all zones.
	pre := totalRealCards(gs)

	gameengine.InvokeETBHook(gs, nev)

	post := totalRealCards(gs)
	if pre != post {
		t.Fatalf("zone conservation violated: pre=%d post=%d (%d cards vanished)",
			pre, post, pre-post)
	}
	_ = nev
}

func totalRealCards(gs *gameengine.GameState) int {
	n := 0
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		n += len(s.Library) + len(s.Hand) + len(s.Graveyard) + len(s.Exile) + len(s.CommandZone)
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && !p.IsToken() {
				n++
			}
		}
	}
	return n
}
