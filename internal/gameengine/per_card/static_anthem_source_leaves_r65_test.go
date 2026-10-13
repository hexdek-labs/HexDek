package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------------
// r65 — static anthem stamps outlived their own source.
//
// Four handlers applied a "creatures you control have X" anthem by
// stamping Flags["kw:<name>"] plus a provenance marker, and registered
// the SAME refresh function for permanent_etb and permanent_ltb. When
// the anthem source itself left the battlefield the refresh re-ran and
// RE-STAMPED, so the buff outlived its source permanently.
//
// Each test asserts both halves: granted while the source is out, gone
// once it leaves. The last two are don't-over-strip controls.
// ---------------------------------------------------------------------------

func TestAnthemR65_GisaMenaceAndPumpEndWhenGisaLeaves(t *testing.T) {
	gs := newGame(t, 2)
	gisa := addPerm(gs, 0, "Gisa, the Hellraiser", "creature", "legendary", "human", "warlock")
	gisa.Card.BasePower, gisa.Card.BaseToughness = 4, 4
	z := addPerm(gs, 0, "Rotting Regisaur", "creature", "zombie", "dinosaur")
	z.Card.BasePower, z.Card.BaseToughness = 7, 6

	gisaRefreshTribeAnthem(gs, gisa)
	if !z.HasKeyword("menace") || gs.PowerOf(z) != 8 {
		t.Fatalf("setup: zombie should be 8/7 with menace; got menace=%v power=%d",
			z.HasKeyword("menace"), gs.PowerOf(z))
	}

	gameengine.DestroyPermanent(gs, gisa, nil)
	if z.HasKeyword("menace") {
		t.Error("menace must end when Gisa leaves the battlefield")
	}
	if got := gs.PowerOf(z); got != 7 {
		t.Errorf("the +1/+1 anthem must end when Gisa leaves; power=%d want 7", got)
	}
}

func TestAnthemR65_OldOneEyeTrampleEndsWhenItLeaves(t *testing.T) {
	gs := newGame(t, 2)
	ooe := addPerm(gs, 0, "Old One Eye", "creature", "legendary", "dinosaur")
	ooe.Card.BasePower, ooe.Card.BaseToughness = 6, 6
	c := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	c.Card.BasePower, c.Card.BaseToughness = 2, 2

	oldOneEyeRefreshAnthem(gs, ooe)
	if !c.HasKeyword("trample") {
		t.Fatal("setup: creature should have trample while Old One Eye is out")
	}

	gameengine.DestroyPermanent(gs, ooe, nil)
	if c.HasKeyword("trample") {
		t.Error("trample must end when Old One Eye leaves the battlefield")
	}
}

func TestAnthemR65_KolodinHasteEndsWhenItLeaves(t *testing.T) {
	gs := newGame(t, 2)
	k := addPerm(gs, 0, "Kolodin, Triumph Caster", "creature", "legendary", "human", "artificer")
	k.Card.BasePower, k.Card.BaseToughness = 3, 4
	v := addPerm(gs, 0, "Smuggler's Copter", "artifact", "vehicle")
	v.Card.BasePower, v.Card.BaseToughness = 3, 3

	kolodinRefreshMountVehicleHaste(gs, k)
	if !v.HasKeyword("haste") {
		t.Fatal("setup: vehicle should have haste while Kolodin is out")
	}

	gameengine.DestroyPermanent(gs, k, nil)
	if v.HasKeyword("haste") {
		t.Error("haste must end when Kolodin leaves the battlefield")
	}
}

func TestAnthemR65_MustardSoldierAnthemEndsWhenItLeaves(t *testing.T) {
	gs := newGame(t, 2)
	m := addPerm(gs, 0, "Commander Mustard", "creature", "legendary", "human", "soldier")
	m.Card.BasePower, m.Card.BaseToughness = 3, 3
	s := addPerm(gs, 0, "Icatian Javelineers", "creature", "human", "soldier")
	s.Card.BasePower, s.Card.BaseToughness = 1, 1

	mustardRefreshSoldierAnthem(gs, m)
	if !s.HasKeyword("vigilance") || !s.HasKeyword("trample") || !s.HasKeyword("haste") {
		t.Fatal("setup: soldier should have vigilance+trample+haste while Mustard is out")
	}

	gameengine.DestroyPermanent(gs, m, nil)
	for _, kw := range []string{"vigilance", "trample", "haste"} {
		if s.HasKeyword(kw) {
			t.Errorf("%s must end when Commander Mustard leaves the battlefield", kw)
		}
	}
}

// Don't-over-strip control: a creature whose keyword came from
// somewhere else must keep it when an unrelated anthem source dies.
func TestAnthemR65_ForeignGrantSurvivesAnthemSourceLeaving(t *testing.T) {
	gs := newGame(t, 2)
	ooe := addPerm(gs, 0, "Old One Eye", "creature", "legendary", "dinosaur")
	ooe.Card.BasePower, ooe.Card.BaseToughness = 6, 6
	// Trample from an unrelated source — no Old One Eye provenance marker.
	other := addPerm(gs, 0, "Rogue Elephant", "creature", "elephant")
	other.Card.BasePower, other.Card.BaseToughness = 3, 3
	other.Flags["kw:trample"] = 1
	gs.InvalidateCharacteristicsCache()

	oldOneEyeRefreshAnthem(gs, ooe)
	gameengine.DestroyPermanent(gs, ooe, nil)

	if !other.HasKeyword("trample") {
		t.Error("a trample grant from another source must survive Old One Eye leaving")
	}
}

// Don't-over-strip control: the anthem must still end for creatures
// that entered AFTER the source, and must not strip a token's intrinsic
// keyword minted by a different card.
func TestAnthemR65_IntrinsicTokenKeywordSurvives(t *testing.T) {
	gs := newGame(t, 2)
	k := addPerm(gs, 0, "Kolodin, Triumph Caster", "creature", "legendary", "human", "artificer")
	k.Card.BasePower, k.Card.BaseToughness = 3, 4
	// A token minted WITH haste by something else.
	tok := addPerm(gs, 0, "Elemental Token", "creature", "elemental", "token")
	tok.Card.BasePower, tok.Card.BaseToughness = 5, 1
	tok.Flags["kw:haste"] = 1
	gs.InvalidateCharacteristicsCache()

	kolodinRefreshMountVehicleHaste(gs, k)
	gameengine.DestroyPermanent(gs, k, nil)

	if !tok.HasKeyword("haste") {
		t.Error("a token's intrinsic haste must survive Kolodin leaving")
	}
}
