package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R55 batch — 10 cards ported via the R54 primitives.
//
//   Damage replacement (new files):
//     1. Furnace of Rath               universal damage doubling
//     2. Gisela, Blade of Goldnight    doubles damage to opps, halves to self
//     3. Dictate of the Twin Gods      universal damage doubling (Flash variant)
//     4. Quest for Pure Flame          red double for activation turn
//     5. Curse of Bloodletting         doubles damage to enchanted player
//
//   Layer 7b set-PT / Layer 4 add-types:
//     6. The Capitoline Triad emblem   creatures become base 9/9
//     7. Inspirit, Flagship Vessel     8+ charge → flying artifact creature
//     8. Infinite Guideline Station    12+ charge → flying artifact creature
//     9. Phoenix Fleet Airship         8+ named copies → artifact creature
//    10. Toph, the First Metalbender   nontoken artifacts → lands (Layer 4)

// ---------------------------------------------------------------------------
// 1. Furnace of Rath — universal doubling
// ---------------------------------------------------------------------------

func TestFurnaceOfRath_DoublesAllDamage(t *testing.T) {
	gs := newGame(t, 2)
	furnace := addPerm(gs, 0, "Furnace of Rath", "enchantment")
	furnaceOfRathETBRegisterReplacement(gs, furnace)

	ctx := &gameengine.DamageContext{
		Source:     furnace, // any non-nil source
		SourceName: "Lightning Bolt",
		TargetSeat: 1,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     3,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 6 {
		t.Errorf("Furnace of Rath should double 3 → 6; got %d", amt)
	}
}

func TestFurnaceOfRath_LTBClearsClosure(t *testing.T) {
	gs := newGame(t, 2)
	furnace := addPerm(gs, 0, "Furnace of Rath", "enchantment")
	furnaceOfRathETBRegisterReplacement(gs, furnace)
	pre := len(gs.DamageReplacements)
	furnaceOfRathLTBUnregister(gs, furnace, map[string]interface{}{"perm": furnace})
	if len(gs.DamageReplacements) != pre-1 {
		t.Errorf("Furnace LTB should drop closure; %d → %d", pre, len(gs.DamageReplacements))
	}
}

// ---------------------------------------------------------------------------
// 2. Gisela, Blade of Goldnight — double to opps / halve to self
// ---------------------------------------------------------------------------

func TestGiselaBlade_DoublesToOpponent(t *testing.T) {
	gs := newGame(t, 2)
	gisela := addPerm(gs, 0, "Gisela, Blade of Goldnight", "creature", "legendary")
	giselaBladeETBRegisterReplacements(gs, gisela)
	ctx := &gameengine.DamageContext{
		Source:     gisela,
		TargetSeat: 1,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     4,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 8 {
		t.Errorf("Gisela should double 4 → 8 vs opp; got %d", amt)
	}
}

func TestGiselaBlade_HalvesIncomingRoundedUp(t *testing.T) {
	gs := newGame(t, 2)
	gisela := addPerm(gs, 0, "Gisela, Blade of Goldnight", "creature", "legendary")
	giselaBladeETBRegisterReplacements(gs, gisela)

	// Damage 5 to Gisela's controller (seat 0). Halved rounded up = 3.
	ctx := &gameengine.DamageContext{
		Source:     nil,
		SourceName: "Lava Coil",
		TargetSeat: 0,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     5,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 3 {
		t.Errorf("Gisela should halve 5 → 3 (rounded up) to controller; got %d", amt)
	}
}

func TestGiselaBlade_EvenIncomingHalves(t *testing.T) {
	gs := newGame(t, 2)
	gisela := addPerm(gs, 0, "Gisela, Blade of Goldnight", "creature", "legendary")
	giselaBladeETBRegisterReplacements(gs, gisela)
	// Damage 6 to seat 0 → halved rounded up = 3.
	ctx := &gameengine.DamageContext{
		Source:     nil,
		SourceName: "Lava Coil",
		TargetSeat: 0,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     6,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 3 {
		t.Errorf("Gisela should halve 6 → 3 to controller; got %d", amt)
	}
}

// ---------------------------------------------------------------------------
// 3. Dictate of the Twin Gods — same as Furnace
// ---------------------------------------------------------------------------

func TestDictateOfTheTwinGods_Doubles(t *testing.T) {
	gs := newGame(t, 2)
	d := addPerm(gs, 0, "Dictate of the Twin Gods", "enchantment")
	dictateOfTheTwinGodsETBRegisterReplacement(gs, d)
	ctx := &gameengine.DamageContext{
		Source:     d,
		TargetSeat: 1,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     2,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 4 {
		t.Errorf("Dictate should double 2 → 4; got %d", amt)
	}
}

// ---------------------------------------------------------------------------
// 4. Quest for Pure Flame — activation arms turn-scoped doubling
// ---------------------------------------------------------------------------

func TestQuestForPureFlame_ActivationArmsDoublingForRedSource(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Pure Flame", "enchantment")
	quest.AddCounter("quest", 4)

	questForPureFlameActivate(gs, quest, 0, nil)

	// Now a red source we control deals 3 damage — should be 6.
	red := addPerm(gs, 0, "Goblin", "creature")
	red.Card.Colors = []string{"R"}
	ctx := &gameengine.DamageContext{
		Source:     red,
		TargetSeat: 1,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     3,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 6 {
		t.Errorf("Quest should double red damage 3 → 6 this turn; got %d", amt)
	}
}

func TestQuestForPureFlame_NonRedSourceUnaffected(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Pure Flame", "enchantment")
	quest.AddCounter("quest", 4)
	questForPureFlameActivate(gs, quest, 0, nil)

	blue := addPerm(gs, 0, "Wizard", "creature")
	blue.Card.Colors = []string{"U"}
	ctx := &gameengine.DamageContext{
		Source:     blue,
		TargetSeat: 1,
		Kind:       gameengine.DamageNonCombatPlayer,
		Amount:     2,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 2 {
		t.Errorf("Quest should not double non-red source; got %d", amt)
	}
}

func TestQuestForPureFlame_ActivationFailsBelowFour(t *testing.T) {
	gs := newGame(t, 2)
	quest := addPerm(gs, 0, "Quest for Pure Flame", "enchantment")
	quest.AddCounter("quest", 3)
	preCount := len(gs.DamageReplacements)

	questForPureFlameActivate(gs, quest, 0, nil)

	if len(gs.DamageReplacements) != preCount {
		t.Errorf("activation should fail below 4 counters; replacements before=%d after=%d", preCount, len(gs.DamageReplacements))
	}
}

// ---------------------------------------------------------------------------
// 5. Curse of Bloodletting — doubles damage to enchanted player
// ---------------------------------------------------------------------------

func TestCurseOfBloodletting_DoublesToEnchantedPlayer(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[1].Life = 5  // lowest-life → targeted by ETB heuristic
	gs.Seats[2].Life = 20
	curse := addPerm(gs, 0, "Curse of Bloodletting", "enchantment", "aura")
	curseOfBloodlettingETBAttachAndRegister(gs, curse)

	if curse.Flags["curse_enchanted_player_seat"] != 2 { // seat+1 = 1+1 = 2
		t.Errorf("expected enchanted seat 1 (flag = seat+1 = 2); got %d", curse.Flags["curse_enchanted_player_seat"])
	}

	ctx := &gameengine.DamageContext{
		Source:     curse,
		TargetSeat: 1,
		Kind:       gameengine.DamageCombatPlayer,
		Amount:     3,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 6 {
		t.Errorf("Curse should double 3 → 6 to cursed seat 1; got %d", amt)
	}
}

func TestCurseOfBloodletting_OtherSeatsUnaffected(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[1].Life = 5
	gs.Seats[2].Life = 20
	curse := addPerm(gs, 0, "Curse of Bloodletting", "enchantment", "aura")
	curseOfBloodlettingETBAttachAndRegister(gs, curse)

	ctx := &gameengine.DamageContext{
		Source:     curse,
		TargetSeat: 2, // not the cursed seat
		Kind:       gameengine.DamageCombatPlayer,
		Amount:     3,
	}
	amt, _ := gameengine.ApplyDamageReplacement(gs, ctx)
	if amt != 3 {
		t.Errorf("Curse should not double damage to seat 2; got %d", amt)
	}
}

// ---------------------------------------------------------------------------
// 6. Capitoline Triad emblem — base 9/9
// ---------------------------------------------------------------------------

func TestCapitolineTriad_EmblemMakesCreaturesBase9_9(t *testing.T) {
	gs := newGame(t, 2)
	triad := addPerm(gs, 0, "The Capitoline Triad", "artifact", "legendary")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2

	// Stock 30+ MV of historic cards in graveyard. cardCMC reads
	// "cmc:N" type tag — see helpers.go.
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Big Saga", Owner: 0, Types: []string{"enchantment", "saga", "cmc:10"}, CMC: 10},
		{Name: "Big Artifact", Owner: 0, Types: []string{"artifact", "cmc:10"}, CMC: 10},
		{Name: "Legend", Owner: 0, Types: []string{"artifact", "legendary", "cmc:10"}, CMC: 10},
	}
	capitolineTriadEmblemActivate(gs, triad, 0, nil)

	chars := gameengine.GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 9 || chars.Toughness != 9 {
		t.Errorf("Bear should become 9/9 after emblem; got %d/%d", chars.Power, chars.Toughness)
	}
}

// ---------------------------------------------------------------------------
// 7. Inspirit, Flagship Vessel — Spacecraft 8+ → creature
// ---------------------------------------------------------------------------

func TestInspirit_BecomesCreatureAt8Charge(t *testing.T) {
	gs := newGame(t, 2)
	ship := addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact", "spacecraft")
	ship.AddCounter("charge", 8)
	inspiritCheckSpacecraftThreshold(gs, ship, nil)

	if ship.Flags["inspirit_spacecraft_active"] != 1 {
		t.Errorf("Inspirit should mark spacecraft active at 8 charge counters")
	}
	if !gs.HasKeywordOf(ship, "flying") {
		t.Errorf("Inspirit should gain flying via Layer 6 grant")
	}
	if !gs.HasTypeOf(ship, "creature") {
		t.Errorf("Inspirit should gain creature type via Layer 4 add-types")
	}
}

func TestInspirit_StaysVehicleBelow8(t *testing.T) {
	gs := newGame(t, 2)
	ship := addPerm(gs, 0, "Inspirit, Flagship Vessel", "artifact", "spacecraft")
	ship.AddCounter("charge", 7)
	inspiritCheckSpacecraftThreshold(gs, ship, nil)

	if ship.Flags["inspirit_spacecraft_active"] == 1 {
		t.Errorf("Inspirit should NOT be spacecraft-active at 7 counters")
	}
}

// ---------------------------------------------------------------------------
// 8. Infinite Guideline Station — 12+ charge
// ---------------------------------------------------------------------------

func TestInfiniteGuideline_BecomesCreatureAt12Charge(t *testing.T) {
	gs := newGame(t, 2)
	stn := addPerm(gs, 0, "Infinite Guideline Station", "artifact", "spacecraft")
	stn.AddCounter("charge", 12)
	infiniteGuidelineCheckSpacecraftThreshold(gs, stn, nil)

	if stn.Flags["infinite_guideline_spacecraft_active"] != 1 {
		t.Errorf("Station should mark spacecraft active at 12 charge counters")
	}
}

// ---------------------------------------------------------------------------
// 9. Phoenix Fleet Airship — 8+ named copies → creature
// ---------------------------------------------------------------------------

func TestPhoenixFleet_BecomesCreatureAt8Copies(t *testing.T) {
	gs := newGame(t, 2)
	primary := addPerm(gs, 0, "Phoenix Fleet Airship", "artifact", "vehicle")
	// Spawn 7 more copies.
	for i := 0; i < 7; i++ {
		addPerm(gs, 0, "Phoenix Fleet Airship", "artifact", "vehicle")
	}
	phoenixFleetAirshipCheckThreshold(gs, primary)

	if primary.Flags["phoenix_fleet_creature_active"] != 1 {
		t.Errorf("Phoenix Fleet should be creature-active with 8 copies")
	}
}

func TestPhoenixFleet_StaysVehicleBelow8(t *testing.T) {
	gs := newGame(t, 2)
	primary := addPerm(gs, 0, "Phoenix Fleet Airship", "artifact", "vehicle")
	for i := 0; i < 6; i++ {
		addPerm(gs, 0, "Phoenix Fleet Airship", "artifact", "vehicle")
	}
	phoenixFleetAirshipCheckThreshold(gs, primary)

	if primary.Flags["phoenix_fleet_creature_active"] == 1 {
		t.Errorf("Phoenix Fleet should NOT be creature-active with only 7 copies")
	}
}

// ---------------------------------------------------------------------------
// 10. Toph, the First Metalbender — Layer 4 add land to artifacts
// ---------------------------------------------------------------------------

func TestToph1stMB_StampsLandTypeOnNontokenArtifacts(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, the First Metalbender", "creature", "legendary")
	sol := addPerm(gs, 0, "Sol Ring", "artifact")
	bear := addPerm(gs, 0, "Grizzly Bears", "creature")
	_ = bear

	tophFirstMetalbenderETB(gs, toph)
	chars := gameengine.GetEffectiveCharacteristics(gs, sol)

	hasLand := false
	for _, t := range chars.Types {
		if t == "land" {
			hasLand = true
		}
	}
	if !hasLand {
		t.Errorf("Sol Ring should have 'land' added via Layer 4; types=%v", chars.Types)
	}
}

func TestToph1stMB_TokenArtifactNotStamped(t *testing.T) {
	gs := newGame(t, 2)
	toph := addPerm(gs, 0, "Toph, the First Metalbender", "creature", "legendary")
	tok := addPerm(gs, 0, "Treasure Token", "artifact", "token")

	tophFirstMetalbenderETB(gs, toph)
	chars := gameengine.GetEffectiveCharacteristics(gs, tok)

	hasLand := false
	for _, t := range chars.Types {
		if t == "land" {
			hasLand = true
		}
	}
	if hasLand {
		t.Errorf("Token artifact should NOT be stamped with land")
	}
}
