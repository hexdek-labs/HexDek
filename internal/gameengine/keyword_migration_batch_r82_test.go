package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r82 — batch 6 of the keyword call-site migration: keywords_batch.go.
//
// The three reads here are all DEATH triggers, dispatched from the dies
// chokepoint (FireZoneChangeTriggers) after the permanent has already left
// the battlefield. That raised the one question this batch actually had to
// answer: is the LKI permanent still visible to the layer system at that
// point, or does GetEffectiveCharacteristics fall back to the printed base?
//
// Probed rather than reasoned about, both with a pointer-identity grant and
// with an anthem-shaped `controller == 0 && IsCreature()` predicate — the
// layer query sees the grant on the LKI object in both shapes. So the
// migration is live, not vacuous, and CR §603.10 ("the game state
// immediately before the event") is served.
//
// Both directions, per batch discipline:
//
//	GRANT  a §613 layer effect confers persist/undying -> the creature
//	       returns (it did not before)
//	STRIP  a layer-6 ability wipe removes a PRINTED one -> the creature
//	       stays dead (the printed keyword used to win)

func r82Returned(gs *GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == name {
			n++
		}
	}
	return n
}

// CR §702.92 — Persist.
func TestKeywordMigrationR82_PersistGrantAndStrip(t *testing.T) {
	// GRANT: no printed persist, conferred by a layer-6 effect.
	gs := newUPGame(t)
	a := upCreature(gs, 0, "Granted Persister", 3)
	r70Grant(gs, a, "persist", "r82_persist_grant")
	if a.HasKeyword("persist") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	SacrificePermanent(gs, a, "test")
	if n := r82Returned(gs, 0, "Granted Persister"); n != 1 {
		t.Errorf("layer-granted persist did not return the creature; found %d copies", n)
	}

	// STRIP: printed persist wiped by Humility must stay dead.
	gs2 := newUPGame(t)
	b := upCreature(gs2, 0, "Humbled Persister", 3, "persist")
	if !b.HasKeyword("persist") {
		t.Fatal("precondition: the printed keyword should be raw-visible")
	}
	r70Strip(gs2, b, "r82_persist_strip")
	SacrificePermanent(gs2, b, "test")
	if n := r82Returned(gs2, 0, "Humbled Persister"); n != 0 {
		t.Errorf("persist survived a layer-6 ability strip — %d copies returned", n)
	}
}

// CR §702.93 — Undying.
func TestKeywordMigrationR82_UndyingGrantAndStrip(t *testing.T) {
	// GRANT.
	gs := newUPGame(t)
	a := upCreature(gs, 0, "Granted Undier", 2)
	r70Grant(gs, a, "undying", "r82_undying_grant")
	if a.HasKeyword("undying") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}
	SacrificePermanent(gs, a, "test")
	got := creaturesNamed(gs, 0, "Granted Undier")
	if len(got) != 1 {
		t.Fatalf("layer-granted undying did not return the creature; found %d copies", len(got))
	}
	if got[0].Counters["+1/+1"] != 1 {
		t.Errorf("the returned creature should carry one +1/+1 counter; got %d", got[0].Counters["+1/+1"])
	}

	// STRIP.
	gs2 := newUPGame(t)
	b := upCreature(gs2, 0, "Humbled Undier", 2, "undying")
	r70Strip(gs2, b, "r82_undying_strip")
	SacrificePermanent(gs2, b, "test")
	if n := r82Returned(gs2, 0, "Humbled Undier"); n != 0 {
		t.Errorf("undying survived a layer-6 ability strip — %d copies returned", n)
	}
}

// The strip must beat a Flags["kw:"] stamp, not just printed text — a strip
// test built on printed-only text passes vacuously against the stamp path
// that r66 was written to close.
func TestKeywordMigrationR82_StrippedStampedUndyingStaysDead(t *testing.T) {
	gs := newUPGame(t)
	p := upCreature(gs, 0, "Stamped Undier", 2)
	p.Flags["kw:undying"] = 1
	gs.InvalidateCharacteristicsCache()
	if !p.HasKeyword("undying") {
		t.Fatal("precondition: the stamp should be raw-visible")
	}
	r70Strip(gs, p, "r82_stamp_strip")
	SacrificePermanent(gs, p, "test")
	if n := r82Returned(gs, 0, "Stamped Undier"); n != 0 {
		t.Errorf("a Flags-stamped undying survived the ability strip — %d copies returned", n)
	}
}

// CR §702.46 — Soulshift. Only the strip direction is assertable here; see
// the note in CheckSoulshift on why a bare grant cannot carry N.
func TestKeywordMigrationR82_SoulshiftStrip(t *testing.T) {
	gs := newUPGame(t)
	spirit := &Card{Name: "Dead Spirit", Owner: 0, Types: []string{"creature", "spirit"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spirit)

	p := upCreature(gs, 0, "Soulshifter", 2)
	p.Card.AST.Abilities = append(p.Card.AST.Abilities, &gameast.Keyword{Name: "soulshift", Args: []interface{}{3}})
	if soulshiftN(p) != 3 {
		t.Fatalf("precondition: printed soulshift 3 not readable; got %d", soulshiftN(p))
	}
	r70Strip(gs, p, "r82_soulshift_strip")
	SacrificePermanent(gs, p, "test")

	inHand := false
	for _, c := range gs.Seats[0].Hand {
		if c != nil && c.Name == "Dead Spirit" {
			inHand = true
		}
	}
	if inHand {
		t.Error("soulshift survived a layer-6 ability strip — the Spirit was returned to hand")
	}

	t.Log("grant direction not asserted: soulshiftN reads the printed Card.AST for N, " +
		"so a layer-granted bare \"soulshift\" keyword resolves to N=0 and is inert by " +
		"construction. The migrated read gates whether soulshift FUNCTIONS; the printed " +
		"text still supplies the magnitude.")
}
