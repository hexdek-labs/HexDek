package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R43 stub-batch ports — five gen_*.go pure-stub handlers from the
// alphabetical FIRST half (a-m), avoiding the R36/R37/R41/R42/R42b
// sets.
//
// Picks:
//   - Admiral Brass, Unsinkable      ETB mill + begin-combat Pirate reanim
//   - Hamza, Guardian of Arashin     creature-spell cost reduction
//   - Galea, Kindler of Hope          library zone-cast grant (Aura/Equip)
//   - Indominus Rex, Alpha            ETB discard → keyword counters → draw N
//   - Iroh, Grand Lotus               turn-scoped graveyard flashback grants

// ---------------------------------------------------------------------------
// Admiral Brass, Unsinkable
// ---------------------------------------------------------------------------

func TestAdmiralBrass_ETBMillsFour(t *testing.T) {
	gs := newGame(t, 2)
	brass := stampCreaturePT(addPerm(gs, 0, "Admiral Brass, Unsinkable", "creature", "legendary"), 4, 4)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	admiralBrassETB(gs, brass)

	if got := len(gs.Seats[0].Graveyard); got != 4 {
		t.Errorf("expected 4 milled, got %d", got)
	}
	if got := len(gs.Seats[0].Library); got != 1 {
		t.Errorf("expected 1 card left in library, got %d", got)
	}
}

func TestAdmiralBrass_ETBMillStopsOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	brass := stampCreaturePT(addPerm(gs, 0, "Admiral Brass, Unsinkable", "creature", "legendary"), 4, 4)
	addLibrary(gs, 0, "X", "Y")

	admiralBrassETB(gs, brass)

	if got := len(gs.Seats[0].Graveyard); got != 2 {
		t.Errorf("expected 2 milled with 2-card library, got %d", got)
	}
}

func TestAdmiralBrass_BeginCombatReanimatesBestPirate(t *testing.T) {
	gs := newGame(t, 2)
	brass := stampCreaturePT(addPerm(gs, 0, "Admiral Brass, Unsinkable", "creature", "legendary"), 4, 4)

	// Plant graveyard: a Pirate (CMC 2), a higher-CMC Pirate (CMC 5),
	// a non-Pirate creature (Bear), and an instant.
	smallPirate := &gameengine.Card{
		Name: "Skeleton Pirate", Owner: 0, CMC: 2,
		Types: []string{"creature", "pirate", "cmc:2"},
	}
	bigPirate := &gameengine.Card{
		Name: "Dread Captain", Owner: 0, CMC: 5,
		Types:         []string{"creature", "pirate", "cmc:5"},
		BasePower:     2,
		BaseToughness: 2,
	}
	bear := &gameengine.Card{
		Name: "Bear", Owner: 0, Types: []string{"creature"},
	}
	inst := &gameengine.Card{
		Name: "Bolt", Owner: 0, Types: []string{"instant"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, smallPirate, bigPirate, bear, inst)

	admiralBrassBeginCombatReanim(gs, brass, map[string]interface{}{
		"active_seat": 0,
	})

	// bigPirate should now be on the battlefield (higher CMC wins).
	var ent *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == bigPirate {
			ent = p
			break
		}
	}
	if ent == nil {
		t.Fatal("higher-CMC Pirate should have been reanimated")
	}
	if ent.Counters["finality"] != 1 {
		t.Errorf("expected finality counter on reanimated Pirate, got %d", ent.Counters["finality"])
	}
	if ent.Card.BasePower != 4 || ent.Card.BaseToughness != 4 {
		t.Errorf("reanimated Pirate base P/T = %d/%d, want 4/4",
			ent.Card.BasePower, ent.Card.BaseToughness)
	}
	if ent.Flags["kw:haste"] != 1 {
		t.Errorf("reanimated Pirate should have haste flag")
	}
	if ent.SummoningSick {
		t.Errorf("reanimated Pirate should have summoning sickness cleared (haste)")
	}
	// Bear should still be in graveyard (not a Pirate).
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == bear {
			t.Errorf("non-Pirate Bear should NOT have been reanimated")
		}
	}
}

func TestAdmiralBrass_BeginCombatSkipsOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	brass := stampCreaturePT(addPerm(gs, 0, "Admiral Brass, Unsinkable", "creature", "legendary"), 4, 4)
	pirate := &gameengine.Card{
		Name: "Pirate", Owner: 0, CMC: 2,
		Types: []string{"creature", "pirate", "cmc:2"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, pirate)

	admiralBrassBeginCombatReanim(gs, brass, map[string]interface{}{
		"active_seat": 1, // opponent's combat
	})

	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == pirate {
			t.Errorf("opponent-turn combat should NOT trigger reanim")
		}
	}
}

// ---------------------------------------------------------------------------
// Hamza, Guardian of Arashin — cost reduction
// ---------------------------------------------------------------------------

func TestHamza_CreatureSpellGetsReductionPerCounter(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Hamza, Guardian of Arashin", "creature", "legendary"), 3, 3)
	c1 := stampCreaturePT(addPerm(gs, 0, "Warrior 1", "creature"), 2, 2)
	c1.AddCounter("+1/+1", 1)
	c2 := stampCreaturePT(addPerm(gs, 0, "Warrior 2", "creature"), 2, 2)
	c2.AddCounter("+1/+1", 3)
	// A creature with no +1/+1 counters — should not count.
	stampCreaturePT(addPerm(gs, 0, "Warrior 3", "creature"), 2, 2)

	creature := &gameengine.Card{Name: "Big Threat", Owner: 0, Types: []string{"creature"}}
	mods := gameengine.ScanCostModifiers(gs, creature, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Hamza, Guardian of Arashin" {
			total += m.Amount
		}
	}
	// Two creatures with +1/+1 counters → discount of 2 (each contributes 1).
	if total != 2 {
		t.Errorf("expected Hamza discount of 2 (2 counter-bearing creatures), got %d", total)
	}
}

func TestHamza_NonCreatureSpellGetsNoReduction(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Hamza, Guardian of Arashin", "creature", "legendary"), 3, 3)
	c := stampCreaturePT(addPerm(gs, 0, "Warrior", "creature"), 2, 2)
	c.AddCounter("+1/+1", 5)

	inst := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}}
	mods := gameengine.ScanCostModifiers(gs, inst, 0)
	for _, m := range mods {
		if m.Source == "Hamza, Guardian of Arashin" {
			t.Errorf("noncreature spell should NOT get Hamza discount; mod=%+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Galea, Kindler of Hope — library zone-cast for Aura/Equipment
// ---------------------------------------------------------------------------

func TestGalea_ETBRegistersGrantWhenTopIsAura(t *testing.T) {
	gs := newGame(t, 2)
	galea := stampCreaturePT(addPerm(gs, 0, "Galea, Kindler of Hope", "creature", "legendary"), 3, 4)

	aura := &gameengine.Card{
		Name: "Holy Strength", Owner: 0,
		Types: []string{"enchantment", "aura"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, aura)

	galeaKindlerOfHopeETB(gs, galea)

	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[aura] == nil {
		t.Fatal("Aura on top of library should have a grant registered")
	}
	g := gs.ZoneCastGrants[aura]
	if g.Zone != gameengine.ZoneLibrary {
		t.Errorf("grant Zone = %q, want library", g.Zone)
	}
	if g.RequireController != 0 {
		t.Errorf("grant RequireController = %d, want 0", g.RequireController)
	}
}

func TestGalea_ETBRegistersGrantWhenTopIsEquipment(t *testing.T) {
	gs := newGame(t, 2)
	galea := stampCreaturePT(addPerm(gs, 0, "Galea, Kindler of Hope", "creature", "legendary"), 3, 4)

	equip := &gameengine.Card{
		Name: "Bonesplitter", Owner: 0,
		Types: []string{"artifact", "equipment"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, equip)

	galeaKindlerOfHopeETB(gs, galea)

	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[equip] == nil {
		t.Fatal("Equipment on top of library should have a grant registered")
	}
}

func TestGalea_ETBNoGrantForNonEligibleTop(t *testing.T) {
	gs := newGame(t, 2)
	galea := stampCreaturePT(addPerm(gs, 0, "Galea, Kindler of Hope", "creature", "legendary"), 3, 4)

	creat := &gameengine.Card{
		Name: "Bear", Owner: 0, Types: []string{"creature"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, creat)

	galeaKindlerOfHopeETB(gs, galea)

	if gs.ZoneCastGrants != nil && gs.ZoneCastGrants[creat] != nil {
		t.Errorf("non-Aura/Equipment top should NOT receive Galea grant")
	}
}

func TestGalea_RefreshAfterDrawPicksUpNewTop(t *testing.T) {
	gs := newGame(t, 2)
	galea := stampCreaturePT(addPerm(gs, 0, "Galea, Kindler of Hope", "creature", "legendary"), 3, 4)

	bear := &gameengine.Card{Name: "Bear", Owner: 0, Types: []string{"creature"}}
	aura := &gameengine.Card{Name: "Aura X", Owner: 0, Types: []string{"enchantment", "aura"}}
	gs.Seats[0].Library = append(gs.Seats[0].Library, bear, aura)

	// First top is non-eligible.
	galeaKindlerOfHopeETB(gs, galea)
	if gs.ZoneCastGrants != nil && gs.ZoneCastGrants[aura] != nil {
		t.Fatal("Aura is not on top yet")
	}

	// Simulate a draw: pop the bear.
	gs.Seats[0].Library = gs.Seats[0].Library[1:]
	galeaRefreshTopGrant(gs, galea, map[string]interface{}{
		"seat":          0,
		"drawer_seat":   0,
		"nth_this_turn": 1,
	})

	if gs.ZoneCastGrants == nil || gs.ZoneCastGrants[aura] == nil {
		t.Errorf("after draw, Aura now on top should receive a grant")
	}
}

// ---------------------------------------------------------------------------
// Indominus Rex, Alpha — discard + keyword counters + draw N
// ---------------------------------------------------------------------------

// addCardWithKeyword stamps a card in seat's hand and gives it an AST
// Keyword ability with the given name (the engine's cardHasKeyword
// reads kw entries from Card.AST.Abilities).
func addCardWithKeyword(gs *gameengine.GameState, seat int, name string, types []string, kwName string) *gameengine.Card {
	c := &gameengine.Card{
		Name:  name,
		Owner: seat,
		Types: append([]string{}, types...),
	}
	// Build a minimal AST with a Keyword ability.
	if kwName != "" {
		c.AST = &gameast.CardAST{
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: kwName},
			},
		}
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

func TestIndominus_DiscardsCreaturesWithKeywordsStampsCounters(t *testing.T) {
	gs := newGame(t, 2)
	indo := stampCreaturePT(addPerm(gs, 0, "Indominus Rex, Alpha", "creature", "legendary"), 7, 7)

	// Hand: creature with flying + creature with haste + non-creature
	// + creature without any of the 12 keywords (shouldn't discard).
	flyer := addCardWithKeyword(gs, 0, "Flyer", []string{"creature"}, "flying")
	haster := addCardWithKeyword(gs, 0, "Haster", []string{"creature"}, "haste")
	noKw := addCardWithKeyword(gs, 0, "Bear", []string{"creature"}, "")
	inst := addCardWithKeyword(gs, 0, "Bolt", []string{"instant"}, "flying")

	addLibrary(gs, 0, "Draw1", "Draw2", "Draw3", "Draw4")

	indominusRexAlphaETB(gs, indo)

	// Counters: flying + haste = 2 counters.
	if indo.Counters["flying"] != 1 {
		t.Errorf("expected flying counter, got %d", indo.Counters["flying"])
	}
	if indo.Counters["haste"] != 1 {
		t.Errorf("expected haste counter, got %d", indo.Counters["haste"])
	}
	// Draw 2.
	if len(gs.Seats[0].Hand) < 2 {
		t.Errorf("expected to have drawn at least 2 cards; hand size %d", len(gs.Seats[0].Hand))
	}
	// Flyer and haster should be in graveyard. Bear and Bolt should be
	// in graveyard or hand respectively — but Bear was an unkeyworded
	// creature, so it stays in hand.
	gyHas := func(c *gameengine.Card) bool {
		for _, x := range gs.Seats[0].Graveyard {
			if x == c {
				return true
			}
		}
		return false
	}
	if !gyHas(flyer) {
		t.Errorf("flyer should be in graveyard after discard")
	}
	if !gyHas(haster) {
		t.Errorf("haster should be in graveyard after discard")
	}
	if gyHas(noKw) {
		t.Errorf("non-keyword Bear should NOT have been discarded")
	}
	if gyHas(inst) {
		t.Errorf("non-creature Bolt should NOT have been discarded")
	}
}

func TestIndominus_NoCreaturesInHandDrawsZero(t *testing.T) {
	gs := newGame(t, 2)
	indo := stampCreaturePT(addPerm(gs, 0, "Indominus Rex, Alpha", "creature", "legendary"), 7, 7)
	addLibrary(gs, 0, "Draw1", "Draw2")

	indominusRexAlphaETB(gs, indo)

	// No counters → no draws.
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected 0 draws with no discards, hand=%d", len(gs.Seats[0].Hand))
	}
	if len(indo.Counters) != 0 {
		// Counters map may have 0-valued entries; check sum.
		total := 0
		for _, n := range indo.Counters {
			total += n
		}
		if total != 0 {
			t.Errorf("expected 0 counters, got %d (counters: %v)", total, indo.Counters)
		}
	}
}
