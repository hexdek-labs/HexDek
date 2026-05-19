package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R46 stub-batch ports — five gen_*.go pure-stub / partial-ETB handlers
// from the alphabetical FIRST half (a-m), avoiding R36/R37/R41–R45 sets.
//
// Picks:
//   - Cynette, Jelly Drover            ETB+dies Jellyfish + flying buff
//   - Dargo, the Shipwrecker            self-cast cost reduction by Sacrificed
//   - Dionus, Elvish Archdruid          tap_event Elf untap+counter
//   - Infinite Guideline Station        ETB Robots + attack-draw multicolored
//   - Inspirit, Flagship Vessel         artifact buff + combat-begin counter

// ---------------------------------------------------------------------------
// Cynette, Jelly Drover
// ---------------------------------------------------------------------------

func TestCynette_ETBSpawnsJellyfish(t *testing.T) {
	gs := newGame(t, 2)
	cyn := stampCreaturePT(addPerm(gs, 0, "Cynette, Jelly Drover", "creature", "legendary"), 3, 3)

	preBF := len(gs.Seats[0].Battlefield)
	cynetteJellyDroverETB(gs, cyn)

	jelly := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Jellyfish" {
			jelly++
			if p.Flags["kw:flying"] != 1 {
				t.Errorf("Jellyfish should have kw:flying")
			}
			hasU := false
			for _, c := range p.Card.Colors {
				if c == "U" {
					hasU = true
				}
			}
			if !hasU {
				t.Errorf("Jellyfish should be blue; colors=%v", p.Card.Colors)
			}
		}
	}
	if jelly != 1 {
		t.Errorf("expected 1 Jellyfish, got %d (bf delta=%d)", jelly, len(gs.Seats[0].Battlefield)-preBF)
	}
}

func TestCynette_DiesSpawnsJellyfishToo(t *testing.T) {
	gs := newGame(t, 2)
	cyn := stampCreaturePT(addPerm(gs, 0, "Cynette, Jelly Drover", "creature", "legendary"), 3, 3)

	cynetteOnDies(gs, cyn, map[string]interface{}{
		"perm":            cyn,
		"controller_seat": 0,
	})

	jelly := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Jellyfish" {
			jelly++
		}
	}
	if jelly != 1 {
		t.Errorf("dies trigger should mint Jellyfish; got %d", jelly)
	}
}

func TestCynette_FlyingCreaturesGetBuff(t *testing.T) {
	gs := newGame(t, 2)
	cyn := stampCreaturePT(addPerm(gs, 0, "Cynette, Jelly Drover", "creature", "legendary"), 3, 3)

	flyer := stampCreaturePT(addPerm(gs, 0, "Faerie", "creature"), 2, 2)
	flyer.Flags = map[string]int{"kw:flying": 1}

	noFly := stampCreaturePT(addPerm(gs, 0, "Bear", "creature"), 2, 2)

	cynetteJellyDroverETB(gs, cyn)

	if got := flyer.Power(); got != 3 {
		t.Errorf("flying creature should be +1/+1; power=%d want 3", got)
	}
	if got := flyer.Toughness(); got != 3 {
		t.Errorf("flying creature toughness=%d want 3", got)
	}
	if got := noFly.Power(); got != 2 {
		t.Errorf("non-flying creature should NOT be buffed; power=%d want 2", got)
	}
}

func TestCynette_RecomputeDropsBuffWhenPermLeaves(t *testing.T) {
	gs := newGame(t, 2)
	cyn := stampCreaturePT(addPerm(gs, 0, "Cynette, Jelly Drover", "creature", "legendary"), 3, 3)
	flyer := stampCreaturePT(addPerm(gs, 0, "Faerie", "creature"), 2, 2)
	flyer.Flags = map[string]int{"kw:flying": 1}

	cynetteJellyDroverETB(gs, cyn)
	if flyer.Power() != 3 {
		t.Fatalf("initial buff missing; power=%d", flyer.Power())
	}

	// Strip flying and recompute — buff should drop.
	delete(flyer.Flags, "kw:flying")
	cynetteRecompute(gs, cyn, map[string]interface{}{})

	if got := flyer.Power(); got != 2 {
		t.Errorf("after removing flying, power should be 2; got %d", got)
	}
	for _, m := range flyer.Modifications {
		if m.Duration == cynetteFlyingBuffTag {
			t.Errorf("residual cynette_flying_buff after recompute: %+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Dargo, the Shipwrecker
// ---------------------------------------------------------------------------

func TestDargo_SelfCastDiscountFromSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Turn.Sacrificed = 3

	dargo := &gameengine.Card{
		Name: "Dargo, the Shipwrecker", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	mods := gameengine.ScanCostModifiers(gs, dargo, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Dargo, the Shipwrecker (self-cast)" {
			total += m.Amount
		}
	}
	if total != 6 {
		t.Errorf("expected 2 × 3 = 6 mana discount, got %d", total)
	}
}

func TestDargo_NoSacrificeNoDiscount(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Turn.Sacrificed = 0

	dargo := &gameengine.Card{
		Name: "Dargo, the Shipwrecker", Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	mods := gameengine.ScanCostModifiers(gs, dargo, 0)
	for _, m := range mods {
		if m.Source == "Dargo, the Shipwrecker (self-cast)" {
			t.Errorf("no sacrifices → no Dargo discount; mod=%+v", m)
		}
	}
}

func TestDargo_NonDargoSpellGetsNoDiscount(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Turn.Sacrificed = 5

	other := &gameengine.Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, other, 0)
	for _, m := range mods {
		if m.Source == "Dargo, the Shipwrecker (self-cast)" {
			t.Errorf("non-Dargo spell should not get Dargo discount; mod=%+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Dionus, Elvish Archdruid
// ---------------------------------------------------------------------------

func TestDionus_ElfTapUntapsAndAddsCounter(t *testing.T) {
	gs := newGame(t, 2)
	dionus := stampCreaturePT(addPerm(gs, 0, "Dionus, Elvish Archdruid", "creature", "legendary"), 3, 3)
	gs.Active = 0

	elf := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature", "elf"), 1, 1)
	elf.Tapped = true // simulate just-tapped

	dionusElfTapped(gs, dionus, map[string]interface{}{
		"seat": 0,
		"perm": elf,
	})

	if elf.Tapped {
		t.Errorf("Elf should have been untapped")
	}
	if elf.Counters["+1/+1"] != 1 {
		t.Errorf("Elf should have +1/+1 counter; got %d", elf.Counters["+1/+1"])
	}
}

func TestDionus_OncePerTurnPerPerm(t *testing.T) {
	gs := newGame(t, 2)
	dionus := stampCreaturePT(addPerm(gs, 0, "Dionus, Elvish Archdruid", "creature", "legendary"), 3, 3)
	gs.Active = 0

	elf := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature", "elf"), 1, 1)
	elf.Tapped = true

	dionusElfTapped(gs, dionus, map[string]interface{}{"seat": 0, "perm": elf})
	if elf.Counters["+1/+1"] != 1 {
		t.Fatalf("first trigger should fire; counters=%d", elf.Counters["+1/+1"])
	}
	// Re-tap and fire again same turn.
	elf.Tapped = true
	dionusElfTapped(gs, dionus, map[string]interface{}{"seat": 0, "perm": elf})

	if elf.Counters["+1/+1"] != 1 {
		t.Errorf("once-per-turn rider violated; counters=%d (want 1)", elf.Counters["+1/+1"])
	}
	if !elf.Tapped {
		t.Errorf("Elf should remain tapped after suppressed re-trigger")
	}
}

func TestDionus_SkipsOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	dionus := stampCreaturePT(addPerm(gs, 0, "Dionus, Elvish Archdruid", "creature", "legendary"), 3, 3)
	gs.Active = 1 // opponent's turn

	elf := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature", "elf"), 1, 1)
	elf.Tapped = true

	dionusElfTapped(gs, dionus, map[string]interface{}{"seat": 0, "perm": elf})
	if elf.Counters["+1/+1"] != 0 {
		t.Errorf("opponent-turn tap should NOT trigger; counters=%d", elf.Counters["+1/+1"])
	}
}

func TestDionus_NonElfNoTrigger(t *testing.T) {
	gs := newGame(t, 2)
	dionus := stampCreaturePT(addPerm(gs, 0, "Dionus, Elvish Archdruid", "creature", "legendary"), 3, 3)
	gs.Active = 0

	beast := stampCreaturePT(addPerm(gs, 0, "Beast", "creature", "beast"), 4, 4)
	beast.Tapped = true

	dionusElfTapped(gs, dionus, map[string]interface{}{"seat": 0, "perm": beast})
	if beast.Counters["+1/+1"] != 0 {
		t.Errorf("non-Elf should NOT trigger; counters=%d", beast.Counters["+1/+1"])
	}
}

// ---------------------------------------------------------------------------
// Infinite Guideline Station
// ---------------------------------------------------------------------------

func TestInfiniteGuideline_ETBSpawnsRobotPerMulticolor(t *testing.T) {
	gs := newGame(t, 2)
	station := stampCreaturePT(addPerm(gs, 0, "Infinite Guideline Station", "artifact"), 0, 0)

	// Two multicolored perms + one monocolored.
	mc1 := addPerm(gs, 0, "Multi A", "creature")
	mc1.Card.Colors = []string{"U", "R"}
	mc2 := addPerm(gs, 0, "Multi B", "creature")
	mc2.Card.Colors = []string{"W", "B"}
	mono := addPerm(gs, 0, "Mono", "creature")
	mono.Card.Colors = []string{"G"}

	infiniteGuidelineStationETB(gs, station)

	robots := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "Robot" {
			robots++
			if !p.Tapped {
				t.Errorf("Robot should enter tapped")
			}
		}
	}
	if robots != 2 {
		t.Errorf("expected 2 Robots (one per multicolored), got %d", robots)
	}
}

func TestInfiniteGuideline_AttackDrawsPerMulticolor(t *testing.T) {
	gs := newGame(t, 2)
	station := stampCreaturePT(addPerm(gs, 0, "Infinite Guideline Station", "artifact", "creature"), 4, 4)

	mc := addPerm(gs, 0, "Multi", "creature")
	mc.Card.Colors = []string{"U", "R"}
	addLibrary(gs, 0, "D1", "D2", "D3")

	infiniteGuidelineAttackDraw(gs, station, map[string]interface{}{
		"attacker_perm": station,
		"attacker_seat": 0,
	})

	if len(gs.Seats[0].Hand) != 1 {
		t.Errorf("expected to draw 1 (one multicolored perm); hand=%d", len(gs.Seats[0].Hand))
	}
}

func TestInfiniteGuideline_AttackIgnoresOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	station := stampCreaturePT(addPerm(gs, 0, "Infinite Guideline Station", "artifact", "creature"), 4, 4)
	other := stampCreaturePT(addPerm(gs, 0, "Bear", "creature"), 2, 2)
	mc := addPerm(gs, 0, "Multi", "creature")
	mc.Card.Colors = []string{"U", "R"}
	addLibrary(gs, 0, "Top")

	infiniteGuidelineAttackDraw(gs, station, map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("non-Station attacker should not trigger; hand=%d", len(gs.Seats[0].Hand))
	}
}

// ---------------------------------------------------------------------------
// Inspirit, Flagship Vessel
// ---------------------------------------------------------------------------

func TestInspirit_ETBStampsArtifactHexproofIndestructible(t *testing.T) {
	gs := newGame(t, 2)
	insp := stampCreaturePT(addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact"), 0, 0)
	art := addPerm(gs, 0, "Sol Ring", "artifact")
	creat := stampCreaturePT(addPerm(gs, 0, "Bear", "creature"), 2, 2)

	inspiritFlagshipVesselETB(gs, insp)

	if art.Flags["kw:hexproof"] != 1 {
		t.Errorf("artifact should have hexproof; flags=%v", art.Flags)
	}
	if art.Flags["kw:indestructible"] != 1 {
		t.Errorf("artifact should have indestructible")
	}
	if creat.Flags["kw:hexproof"] == 1 {
		t.Errorf("non-artifact should NOT receive grant")
	}
	if insp.Flags["kw:hexproof"] == 1 {
		t.Errorf("Inspirit himself should NOT receive grant (other artifacts only)")
	}
}

func TestInspirit_RefreshOnNewArtifactETB(t *testing.T) {
	gs := newGame(t, 2)
	insp := stampCreaturePT(addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact"), 0, 0)
	inspiritFlagshipVesselETB(gs, insp)

	freshArt := addPerm(gs, 0, "Mind Stone", "artifact")
	inspiritRefreshArtifactGrants(gs, insp, map[string]interface{}{
		"perm":            freshArt,
		"controller_seat": 0,
	})
	if freshArt.Flags["kw:hexproof"] != 1 || freshArt.Flags["kw:indestructible"] != 1 {
		t.Errorf("newly-entered artifact should receive grants; flags=%v", freshArt.Flags)
	}
}

func TestInspirit_CombatBeginPutsCounterOnBestArtifact(t *testing.T) {
	gs := newGame(t, 2)
	insp := stampCreaturePT(addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact"), 0, 0)
	smallArt := stampCreaturePT(addPerm(gs, 0, "Small Artifact", "artifact", "creature"), 1, 1)
	bigArt := stampCreaturePT(addPerm(gs, 0, "Big Artifact", "artifact", "creature"), 5, 5)

	inspiritCombatBeginCounter(gs, insp, map[string]interface{}{
		"active_seat": 0,
	})

	if bigArt.Counters["+1/+1"] != 1 {
		t.Errorf("highest-power artifact should get counter; bigArt=%d", bigArt.Counters["+1/+1"])
	}
	if smallArt.Counters["+1/+1"] != 0 {
		t.Errorf("smaller artifact should not be picked; smallArt=%d", smallArt.Counters["+1/+1"])
	}
}

func TestInspirit_CombatBeginSkipsOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	insp := stampCreaturePT(addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact"), 0, 0)
	art := stampCreaturePT(addPerm(gs, 0, "Artifact", "artifact", "creature"), 2, 2)

	inspiritCombatBeginCounter(gs, insp, map[string]interface{}{
		"active_seat": 1, // opponent's combat
	})
	if art.Counters["+1/+1"] != 0 {
		t.Errorf("opponent-turn combat should NOT trigger; got %d counters", art.Counters["+1/+1"])
	}
}
