package gameengine

import "testing"

// r63 — incarnation graveyard-static mechanic (Anger/Brawn/Filth/Wonder/Valor).
// Each test pins one of the five audited properties (a)–(e).

// incarnationInGrave drops an incarnation card into seat's graveyard and
// returns it (so tests can later MoveCard it out).
func incarnationInGrave(gs *GameState, seat int, name string) *Card {
	c := &Card{Name: name, Owner: seat, Types: []string{"creature", "incarnation"}}
	gs.Seats[seat].Graveyard = append(gs.Seats[seat].Graveyard, c)
	return c
}

// addBasicLandPerm puts a basic land of the given subtype on seat's battlefield.
func addBasicLandPerm(gs *GameState, seat int, name, subtype string) *Permanent {
	return addBattlefield(gs, seat, name, 0, 0, "land", subtype)
}

func hasKW(gs *GameState, p *Permanent, kw string) bool {
	gs.InvalidateCharacteristicsCache()
	return gs.HasKeywordOf(p, kw)
}

// (a) The static FUNCTIONS from the graveyard zone and surfaces via the §613
// layer system (GetEffectiveCharacteristics → chars.Keywords → HasKeywordOf).
func TestIncarnation_PropA_FunctionsFromGraveyardViaLayers(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	addBasicLandPerm(gs, 0, "Mountain", "mountain")
	incarnationInGrave(gs, 0, "Anger")

	gs.InvalidateCharacteristicsCache()
	chars := GetEffectiveCharacteristics(gs, bear)
	found := false
	for _, k := range chars.Keywords {
		if k == "haste" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Anger in graveyard + Mountain should grant haste via layer chars; keywords=%v", chars.Keywords)
	}
	if !gs.HasKeywordOf(bear, "haste") {
		t.Errorf("HasKeywordOf should report the layer-granted haste")
	}
	if !keywordActive(gs, bear, "haste") {
		t.Errorf("keywordActive (combat query) should see the graveyard-static haste")
	}
}

// (b) The land-type condition is evaluated DYNAMICALLY — the grant turns on
// and off as the controller's land control changes.
func TestIncarnation_PropB_LandConditionDynamic(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	incarnationInGrave(gs, 0, "Anger")

	// No Mountain yet → no haste.
	if hasKW(gs, bear, "haste") {
		t.Fatalf("haste should be OFF with no Mountain controlled")
	}

	// Play a Mountain → haste turns ON.
	mtn := addBasicLandPerm(gs, 0, "Mountain", "mountain")
	if !hasKW(gs, bear, "haste") {
		t.Fatalf("haste should turn ON once a Mountain is controlled")
	}

	// Lose the Mountain → haste turns OFF.
	removePermanentFromBattlefield(gs, mtn)
	if hasKW(gs, bear, "haste") {
		t.Fatalf("haste should turn OFF when the Mountain leaves")
	}
}

// (c) The granted keyword reaches the correct subset (YOUR creatures) and
// genuinely works in play: layer-granted haste lets a summoning-sick creature
// attack; flying/trample/first strike are visible to the combat query; an
// opponent's creature does NOT receive the grant.
func TestIncarnation_PropC_GrantWorksInPlay(t *testing.T) {
	gs := newFixtureGame(t)

	// Haste lets a summoning-sick creature attack (canAttackGS is the legal
	// attacker gate). Without Anger, the same creature cannot attack.
	sick := addBattlefield(gs, 0, "Hasty Hopeful", 2, 2, "creature")
	sick.SummoningSick = true
	addBasicLandPerm(gs, 0, "Mountain", "mountain")
	gs.InvalidateCharacteristicsCache()
	if canAttackGS(gs, sick) {
		t.Fatalf("summoning-sick creature should NOT be able to attack without haste")
	}
	anger := incarnationInGrave(gs, 0, "Anger")
	gs.InvalidateCharacteristicsCache()
	if !canAttackGS(gs, sick) {
		t.Fatalf("Anger should let the summoning-sick creature attack (layer-granted haste)")
	}
	_ = anger

	// Evasion / combat keywords reach the combat query.
	for _, tc := range []struct{ inc, land, subtype, kw string }{
		{"Wonder", "Island", "island", "flying"},
		{"Brawn", "Forest", "forest", "trample"},
		{"Valor", "Plains", "plains", "first strike"},
		{"Filth", "Swamp", "swamp", "swampwalk"},
	} {
		g := newFixtureGame(t)
		mine := addBattlefield(g, 0, "Mine", 2, 2, "creature")
		opp := addBattlefield(g, 1, "Theirs", 2, 2, "creature")
		addBasicLandPerm(g, 0, tc.land, tc.subtype)
		incarnationInGrave(g, 0, tc.inc)
		g.InvalidateCharacteristicsCache()
		if !keywordActive(g, mine, tc.kw) {
			t.Errorf("%s (gy) + %s should grant %s to my creature", tc.inc, tc.land, tc.kw)
		}
		// Subset: opponent's creature must NOT get my graveyard's grant.
		if keywordActive(g, opp, tc.kw) {
			t.Errorf("%s should NOT grant %s to an OPPONENT's creature", tc.inc, tc.kw)
		}
	}
}

// (d) The grant stops the instant the card leaves the graveyard.
func TestIncarnation_PropD_StopsWhenLeavesGraveyard(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	addBasicLandPerm(gs, 0, "Mountain", "mountain")
	anger := incarnationInGrave(gs, 0, "Anger")

	if !hasKW(gs, bear, "haste") {
		t.Fatalf("precondition: haste should be granted from the graveyard")
	}
	// Anger leaves the graveyard (e.g. recursion to hand / exile). MoveCard
	// invalidates the cache for graveyard moves, so the next read is fresh.
	MoveCard(gs, anger, 0, "graveyard", "exile", "test_leave_gy")
	if gs.HasKeywordOf(bear, "haste") {
		t.Fatalf("haste must stop the instant Anger leaves the graveyard")
	}
}

// (e) The grant does NOT apply while the card is in any other zone — hand,
// library, exile, or on the battlefield as a permanent.
func TestIncarnation_PropE_OnlyFromGraveyard(t *testing.T) {
	zones := []string{"hand", "library", "exile", "battlefield"}
	for _, z := range zones {
		t.Run(z, func(t *testing.T) {
			gs := newFixtureGame(t)
			bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
			addBasicLandPerm(gs, 0, "Mountain", "mountain")
			c := &Card{Name: "Anger", Owner: 0, Types: []string{"creature", "incarnation"}}
			switch z {
			case "hand":
				gs.Seats[0].Hand = append(gs.Seats[0].Hand, c)
			case "library":
				gs.Seats[0].Library = append(gs.Seats[0].Library, c)
			case "exile":
				gs.Seats[0].Exile = append(gs.Seats[0].Exile, c)
			case "battlefield":
				addBattlefield(gs, 0, "Anger", 2, 2, "creature")
			}
			gs.InvalidateCharacteristicsCache()
			if gs.HasKeywordOf(bear, "haste") {
				t.Errorf("Anger in %s must NOT grant haste (graveyard-only static)", z)
			}
		})
	}
}

// Negative: an incarnation in the graveyard with the WRONG land controlled
// grants nothing.
func TestIncarnation_WrongLandNoGrant(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")
	addBasicLandPerm(gs, 0, "Forest", "forest") // not a Mountain
	incarnationInGrave(gs, 0, "Anger")
	if hasKW(gs, bear, "haste") {
		t.Fatalf("Anger should grant nothing without a Mountain (Forest controlled)")
	}
}
