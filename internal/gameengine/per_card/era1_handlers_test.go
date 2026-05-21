package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// dev/era1-unification (PR#32) added 12 hand-written handlers on top of
// the auto-generated gen_*.go stubs but shipped without dedicated test
// coverage. This file backfills one focused test per handler.

// ---------------------------------------------------------------------------
// Mabel, Heir to Cragflame — ETB creates Cragflame Equipment token
// ---------------------------------------------------------------------------

func TestMabelEra1_ETBCreatesCragflameToken(t *testing.T) {
	gs := newGame(t, 2)
	// Stamp P/T so SBA doesn't destroy Mabel when the R50 batchH anthem
	// trigger fan-out runs StateBasedActions after my permanent_etb
	// handler resolves through the stack bridge.
	mabel := stampCreaturePT(addPerm(gs, 0, "Mabel, Heir to Cragflame", "creature", "legendary"), 3, 3)

	bfBefore := len(gs.Seats[0].Battlefield)
	mabelHeirToCragflameETB(gs, mabel)

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Fatalf("expected Cragflame token created; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
	tok := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if tok.Card == nil || tok.Card.DisplayName() != "Cragflame" {
		t.Fatalf("expected Cragflame token at top of battlefield; got %+v", tok.Card)
	}
	if !cardHasType(tok.Card, "equipment") {
		t.Errorf("token should be tagged equipment; types=%v", tok.Card.Types)
	}
}

// ---------------------------------------------------------------------------
// Aesi, Tyrant of Gyre Strait — ETB extra-land flag + landfall draw
// ---------------------------------------------------------------------------

func TestAesiEra1_ETBGrantsExtraLandDrop(t *testing.T) {
	gs := newGame(t, 2)
	aesi := addPerm(gs, 0, "Aesi, Tyrant of Gyre Strait", "creature", "legendary")

	aesiETB(gs, aesi)

	if gs.Seats[0].Flags["extra_land_drops"] != 1 {
		t.Fatalf("expected extra_land_drops=1, got %d", gs.Seats[0].Flags["extra_land_drops"])
	}
}

func TestAesiEra1_LandfallDrawsCard(t *testing.T) {
	gs := newGame(t, 2)
	aesi := addPerm(gs, 0, "Aesi, Tyrant of Gyre Strait", "creature", "legendary")
	addLibrary(gs, 0, "TopCard")
	land := addPerm(gs, 0, "Forest", "land", "basic")

	aesiLandfall(gs, aesi, map[string]interface{}{
		"perm":            land,
		"controller_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 card drawn from landfall, hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Bristly Bill, Spine Sower — landfall counter + activated double
// ---------------------------------------------------------------------------

func TestBristlyBillEra1_LandfallAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	bill := addPerm(gs, 0, "Bristly Bill, Spine Sower", "creature", "legendary")
	bill.Card.BasePower = 2
	bill.Card.BaseToughness = 2
	land := addPerm(gs, 0, "Forest", "land", "basic")

	bristlyBillLandfall(gs, bill, map[string]interface{}{
		"perm":            land,
		"controller_seat": 0,
	})

	// Bill himself is the only creature so he's the target.
	if bill.Counters["+1/+1"] != 1 {
		t.Fatalf("expected +1/+1 counter on Bristly Bill, got %d", bill.Counters["+1/+1"])
	}
}

func TestBristlyBillEra1_ActivatedDoublesCounters(t *testing.T) {
	gs := newGame(t, 2)
	bill := addPerm(gs, 0, "Bristly Bill, Spine Sower", "creature", "legendary")
	bill.Card.BasePower = 2
	bill.Card.BaseToughness = 2
	bill.AddCounter("+1/+1", 3)
	other := addPerm(gs, 0, "Hornet Queen", "creature")
	other.Card.BasePower = 2
	other.Card.BaseToughness = 2
	other.AddCounter("+1/+1", 1)
	uncounter := addPerm(gs, 0, "Llanowar Elves", "creature")
	uncounter.Card.BasePower = 1
	uncounter.Card.BaseToughness = 1
	gs.Seats[0].ManaPool = 5 // {3}{G}{G}

	bristlyBillSpineSowerDouble(gs, bill, 0, nil)

	if bill.Counters["+1/+1"] != 6 {
		t.Errorf("expected Bill's counters doubled to 6; got %d", bill.Counters["+1/+1"])
	}
	if other.Counters["+1/+1"] != 2 {
		t.Errorf("expected other creature counters doubled to 2; got %d", other.Counters["+1/+1"])
	}
	if uncounter.Counters["+1/+1"] != 0 {
		t.Errorf("creatures without counters should be untouched; got %d", uncounter.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Byrke, Long Ear of the Law — ETB up-to-2 counters + attack-double
// ---------------------------------------------------------------------------

func TestByrkeEra1_ETBPlacesCountersOnUpToTwo(t *testing.T) {
	gs := newGame(t, 2)
	byrke := addPerm(gs, 0, "Byrke, Long Ear of the Law", "creature", "legendary")
	byrke.Card.BasePower = 4
	byrke.Card.BaseToughness = 4
	c1 := addPerm(gs, 0, "Bear A", "creature")
	c1.Card.BasePower = 2
	c1.Card.BaseToughness = 2
	c2 := addPerm(gs, 0, "Bear B", "creature")
	c2.Card.BasePower = 2
	c2.Card.BaseToughness = 2
	c3 := addPerm(gs, 0, "Bear C", "creature")
	c3.Card.BasePower = 2
	c3.Card.BaseToughness = 2

	byrkeETB(gs, byrke)

	// "Up to two target creatures" — exactly 2 should receive a counter
	// across Byrke + the three bears.
	gotCounters := 0
	for _, p := range []*gameengine.Permanent{byrke, c1, c2, c3} {
		if p.Counters["+1/+1"] > 0 {
			gotCounters++
		}
	}
	if gotCounters != 2 {
		t.Fatalf("expected exactly 2 creatures to receive +1/+1 counters; got %d", gotCounters)
	}
}

func TestByrkeEra1_AttackDoublesCounters(t *testing.T) {
	gs := newGame(t, 2)
	byrke := addPerm(gs, 0, "Byrke, Long Ear of the Law", "creature", "legendary")
	atk := addPerm(gs, 0, "Counter Bear", "creature")
	atk.AddCounter("+1/+1", 2)

	byrkeAttackDouble(gs, byrke, map[string]interface{}{
		"attacker_perm": atk,
	})

	if atk.Counters["+1/+1"] != 4 {
		t.Fatalf("expected counters doubled 2→4, got %d", atk.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Aminatou, Veil Piercer — upkeep surveil 2
// ---------------------------------------------------------------------------

func TestAminatouEra1_UpkeepSurveils(t *testing.T) {
	gs := newGame(t, 2)
	aminatou := addPerm(gs, 0, "Aminatou, Veil Piercer", "creature", "legendary")
	addLibrary(gs, 0, "L1", "L2", "L3")

	aminatouVeilPiercerUpkeep(gs, aminatou, map[string]interface{}{
		"active_seat": 0,
	})

	// Surveil event must fire for the controller's upkeep (no Hat is wired
	// in tests, so all looked cards stay on top — the surveil event is the
	// observable trace).
	if hasEvent(gs, "surveil") < 1 {
		t.Errorf("expected surveil event from Aminatou upkeep; events=%+v", gs.EventLog)
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Errorf("expected per_card_handler breadcrumb")
	}
}

func TestAminatouEra1_OffturnUpkeepSkipped(t *testing.T) {
	gs := newGame(t, 2)
	aminatou := addPerm(gs, 0, "Aminatou, Veil Piercer", "creature", "legendary")
	addLibrary(gs, 0, "L1", "L2", "L3")

	aminatouVeilPiercerUpkeep(gs, aminatou, map[string]interface{}{
		"active_seat": 1,
	})

	// Off-turn surveil should not happen — slug never emitted.
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_handler" {
			if d, ok := ev.Details["slug"].(string); ok && d == "aminatou_veil_piercer_upkeep_surveil" {
				t.Errorf("upkeep surveil should not fire on opponent's upkeep")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Queen Marchesa — ETB monarch + upkeep Assassin token
// ---------------------------------------------------------------------------

func TestQueenMarchesaEra1_ETBBecomesMonarch(t *testing.T) {
	gs := newGame(t, 2)
	queen := addPerm(gs, 0, "Queen Marchesa", "creature", "legendary")

	queenMarchesaETB(gs, queen)

	if gs.Flags["has_monarch"] != 1 || gs.Flags["monarch_seat"] != 0 {
		t.Fatalf("expected seat 0 to become the monarch; flags=%+v", gs.Flags)
	}
}

func TestQueenMarchesaEra1_UpkeepSpawnsAssassinWhenOpponentMonarch(t *testing.T) {
	gs := newGame(t, 2)
	queen := addPerm(gs, 0, "Queen Marchesa", "creature", "legendary")
	gs.Flags = map[string]int{"has_monarch": 1, "monarch_seat": 1}

	bfBefore := len(gs.Seats[0].Battlefield)
	queenMarchesaUpkeep(gs, queen, map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Fatalf("expected Assassin token created; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
	tok := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if !cardHasType(tok.Card, "assassin") {
		t.Errorf("token should be Assassin; types=%v", tok.Card.Types)
	}
}

func TestQueenMarchesaEra1_NoTokenWhenSelfIsMonarch(t *testing.T) {
	gs := newGame(t, 2)
	queen := addPerm(gs, 0, "Queen Marchesa", "creature", "legendary")
	gs.Flags = map[string]int{"has_monarch": 1, "monarch_seat": 0}

	bfBefore := len(gs.Seats[0].Battlefield)
	queenMarchesaUpkeep(gs, queen, map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != bfBefore {
		t.Fatalf("no token should spawn when self is monarch; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
}

// Kardur, Doomscourge — gen handler was deleted by PR#36 in favor of the
// custom_kardur_doomscourge.go entry from PR#31 (era2). Tests for the
// custom variant live in era2_handlers_test.go (TestKardur_*) so we don't
// duplicate them here.

// ---------------------------------------------------------------------------
// The Swarmweaver — ETB two flying Insect tokens
// ---------------------------------------------------------------------------

func TestTheSwarmweaverEra1_ETBSpawnsTwoInsects(t *testing.T) {
	gs := newGame(t, 2)
	// Stamp P/T so SBA doesn't destroy Swarmweaver when the R50 batchH
	// delirium-anthem trigger fan-out runs StateBasedActions after each
	// Insect-token spawn fires permanent_etb.
	weaver := stampCreaturePT(addPerm(gs, 0, "The Swarmweaver", "creature", "legendary", "artifact"), 2, 3)

	bfBefore := len(gs.Seats[0].Battlefield)
	theSwarmweaverETB(gs, weaver)

	if len(gs.Seats[0].Battlefield)-bfBefore != 2 {
		t.Fatalf("expected 2 Insect tokens; battlefield grew by %d",
			len(gs.Seats[0].Battlefield)-bfBefore)
	}
	tok := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if !cardHasType(tok.Card, "insect") || !cardHasType(tok.Card, "flying") {
		t.Errorf("token should be flying Insect; types=%v", tok.Card.Types)
	}
}

// ---------------------------------------------------------------------------
// Rendmaw, Creaking Nest — ETB and multi-type cast spawn Birds for each player
// ---------------------------------------------------------------------------

func TestRendmawEra1_ETBSpawnsBirdPerPlayer(t *testing.T) {
	gs := newGame(t, 4)
	rendmaw := addPerm(gs, 0, "Rendmaw, Creaking Nest", "creature", "legendary")

	totalsBefore := []int{}
	for _, s := range gs.Seats {
		totalsBefore = append(totalsBefore, len(s.Battlefield))
	}
	rendmawETB(gs, rendmaw)

	for i, s := range gs.Seats {
		grew := len(s.Battlefield) - totalsBefore[i]
		if grew != 1 {
			t.Errorf("seat %d expected 1 bird token; grew by %d", i, grew)
		}
		tok := s.Battlefield[len(s.Battlefield)-1]
		if !tok.Tapped {
			t.Errorf("seat %d bird should be tapped", i)
		}
		if tok.Flags["goaded"] != 1 {
			t.Errorf("seat %d bird should be goaded", i)
		}
	}
}

func TestRendmawEra1_SpellCastSinglyTypedDoesNotTrigger(t *testing.T) {
	gs := newGame(t, 4)
	rendmaw := addPerm(gs, 0, "Rendmaw, Creaking Nest", "creature", "legendary")
	beforeTotal := 0
	for _, s := range gs.Seats {
		beforeTotal += len(s.Battlefield)
	}
	singletype := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
	}
	rendmawSpellCast(gs, rendmaw, map[string]interface{}{
		"caster_seat": 0,
		"card":        singletype,
	})

	afterTotal := 0
	for _, s := range gs.Seats {
		afterTotal += len(s.Battlefield)
	}
	if afterTotal != beforeTotal {
		t.Errorf("single-type spell should not trigger Rendmaw; before=%d after=%d",
			beforeTotal, afterTotal)
	}
}

func TestRendmawEra1_SpellCastMultiTypedTriggers(t *testing.T) {
	gs := newGame(t, 4)
	rendmaw := addPerm(gs, 0, "Rendmaw, Creaking Nest", "creature", "legendary")
	beforeTotal := 0
	for _, s := range gs.Seats {
		beforeTotal += len(s.Battlefield)
	}
	multi := &gameengine.Card{
		Name:  "Dryad Arbor", // creature + land — 2 distinct types
		Owner: 0,
		Types: []string{"creature", "land"},
	}
	rendmawSpellCast(gs, rendmaw, map[string]interface{}{
		"caster_seat": 0,
		"card":        multi,
	})

	afterTotal := 0
	for _, s := range gs.Seats {
		afterTotal += len(s.Battlefield)
	}
	if afterTotal-beforeTotal != 4 {
		t.Errorf("multi-type spell should spawn one bird per player (4); grew %d",
			afterTotal-beforeTotal)
	}
}

// ---------------------------------------------------------------------------
// Kona, Rescue Beastie — Survival cheats highest-CMC permanent from hand
// ---------------------------------------------------------------------------

func TestKonaEra1_SurvivalCheatsHighestCMCPermanent(t *testing.T) {
	gs := newGame(t, 2)
	kona := addPerm(gs, 0, "Kona, Rescue Beastie", "creature", "legendary")
	kona.Tapped = true
	cheap := &gameengine.Card{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature", "cmc:1"}}
	expensive := &gameengine.Card{Name: "Blightsteel Colossus", Owner: 0, Types: []string{"creature", "artifact", "cmc:12"}}
	noncommand := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"}}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, cheap, expensive, noncommand)

	bfBefore := len(gs.Seats[0].Battlefield)
	konaSurvival(gs, kona, map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != bfBefore+1 {
		t.Fatalf("expected one card cheated to battlefield; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
	cheated := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	if cheated.Card != expensive {
		t.Errorf("expected Blightsteel Colossus (highest CMC) cheated; got %s", cheated.Card.DisplayName())
	}
}

func TestKonaEra1_UntappedDoesNotCheat(t *testing.T) {
	gs := newGame(t, 2)
	kona := addPerm(gs, 0, "Kona, Rescue Beastie", "creature", "legendary")
	// Kona untapped — survival shouldn't fire.
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name: "Blightsteel", Owner: 0, Types: []string{"creature", "cmc:12"},
	})

	bfBefore := len(gs.Seats[0].Battlefield)
	konaSurvival(gs, kona, map[string]interface{}{
		"active_seat": 0,
	})

	if len(gs.Seats[0].Battlefield) != bfBefore {
		t.Errorf("untapped Kona should not cheat anything; before=%d after=%d",
			bfBefore, len(gs.Seats[0].Battlefield))
	}
}

// ---------------------------------------------------------------------------
// Ezrim, Agency Chief — ETB investigate twice
// ---------------------------------------------------------------------------

func TestEzrimEra1_ETBCreatesTwoClues(t *testing.T) {
	gs := newGame(t, 2)
	ezrim := addPerm(gs, 0, "Ezrim, Agency Chief", "creature", "legendary")

	bfBefore := len(gs.Seats[0].Battlefield)
	ezrimETB(gs, ezrim)

	grew := len(gs.Seats[0].Battlefield) - bfBefore
	if grew != 2 {
		t.Fatalf("expected 2 Clue tokens from investigate twice; grew %d", grew)
	}
	if hasEvent(gs, "investigate") < 1 {
		t.Errorf("expected investigate event")
	}
}

// ---------------------------------------------------------------------------
// Prime Speaker Zegana — ETB scales with biggest other creature
// ---------------------------------------------------------------------------

func TestPrimeSpeakerZeganaEra1_CountersAndDrawsFromBiggestOther(t *testing.T) {
	gs := newGame(t, 2)
	zegana := addPerm(gs, 0, "Prime Speaker Zegana", "creature", "legendary")
	zegana.Card.BasePower = 1
	zegana.Card.BaseToughness = 1
	beast := addPerm(gs, 0, "Quartzwood Crasher", "creature")
	beast.Card.BasePower = 5
	beast.Card.BaseToughness = 5
	addLibrary(gs, 0, "A", "B", "C", "D", "E", "F", "G")

	primeSpeakerZeganaETB(gs, zegana)

	if zegana.Counters["+1/+1"] != 5 {
		t.Errorf("expected 5 +1/+1 counters from 5-power buddy; got %d", zegana.Counters["+1/+1"])
	}
	// Oracle: "draw X cards" where X = greatest power among other
	// creatures (5), not Zegana's post-counter power.
	if len(gs.Seats[0].Hand) != 5 {
		t.Errorf("expected 5 cards drawn (X=other power); hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestPrimeSpeakerZeganaEra1_NoOtherCreaturesNoCounters(t *testing.T) {
	gs := newGame(t, 2)
	zegana := addPerm(gs, 0, "Prime Speaker Zegana", "creature", "legendary")
	zegana.Card.BasePower = 1
	zegana.Card.BaseToughness = 1
	addLibrary(gs, 0, "A", "B")

	primeSpeakerZeganaETB(gs, zegana)

	if zegana.Counters["+1/+1"] != 0 {
		t.Errorf("no other creatures → no counters; got %d", zegana.Counters["+1/+1"])
	}
	// Oracle gates the draw on "if X is 1 or more"; X=0 → no draw.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected 0 cards drawn when X=0 (no other creatures); hand=%d", len(gs.Seats[0].Hand))
	}
}
