package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R41 stub-batch ports — five gen_*.go pure-stub handlers ported into
// real per-card behaviour. Avoids the r36 set (Ashling, Tannuk, Toph,
// Magnus, Morlun) and the r37 set (Old One Eye, Lara Croft, Maha,
// Norman Osborn, Thrun). Picks span:
//
//   - The Locust God:         draw trigger → 1/1 flying/haste token
//   - Cleopatra, Exiled Pharaoh: end_step counter + creature_dies draw
//   - Rakdos, Lord of Riots:  static cost reduction (cost_modifiers.go)
//   - Iron Man, Titan of Innovation: attack trigger → Treasure + tutor
//   - Yorion, Sky Nomad:      ETB blink + delayed-trigger return

// ---------------------------------------------------------------------------
// The Locust God — draw_card → Insect token (1/1 flying/haste)
// ---------------------------------------------------------------------------

func TestLocustGod_DrawSpawnsInsectToken(t *testing.T) {
	gs := newGame(t, 2)
	locust := stampCreaturePT(addPerm(gs, 0, "The Locust God", "creature"), 4, 4)

	preBF := len(gs.Seats[0].Battlefield)
	theLocustGodOnDraw(gs, locust, map[string]interface{}{
		"seat":          0,
		"drawer_seat":   0,
		"nth_this_turn": 1,
		"source":        "draw",
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF+1 {
		t.Fatalf("battlefield delta = %d, want +1 token", got-preBF)
	}
	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == locust || p.Card == nil {
			continue
		}
		if p.Card.Name == "Insect" {
			tok = p
			break
		}
	}
	if tok == nil {
		t.Fatal("Insect token not found on battlefield")
	}
	if tok.Flags["kw:flying"] != 1 || tok.Flags["kw:haste"] != 1 {
		t.Errorf("Insect token kw flags = %v, want flying+haste set", tok.Flags)
	}
	if tok.SummoningSick {
		t.Error("Insect token should have haste-cleared SummoningSick")
	}
	if tok.Card.BasePower != 1 || tok.Card.BaseToughness != 1 {
		t.Errorf("Insect token P/T = %d/%d, want 1/1",
			tok.Card.BasePower, tok.Card.BaseToughness)
	}
	hasU, hasR := false, false
	for _, c := range tok.Card.Colors {
		if c == "U" {
			hasU = true
		}
		if c == "R" {
			hasR = true
		}
	}
	if !hasU || !hasR {
		t.Errorf("Insect token colors = %v, want includes U and R", tok.Card.Colors)
	}
}

func TestLocustGod_IgnoresOpponentDraws(t *testing.T) {
	gs := newGame(t, 2)
	locust := stampCreaturePT(addPerm(gs, 0, "The Locust God", "creature"), 4, 4)
	preBF := len(gs.Seats[0].Battlefield)

	theLocustGodOnDraw(gs, locust, map[string]interface{}{
		"seat":          1, // opponent drew
		"drawer_seat":   1,
		"nth_this_turn": 1,
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF {
		t.Errorf("opponent draw should NOT mint token; bf delta=%d", got-preBF)
	}
}

// ---------------------------------------------------------------------------
// Cleopatra, Exiled Pharaoh — Allies + Betrayal
// ---------------------------------------------------------------------------

func TestCleopatra_AlliesPutsCounterOnUpToTwo(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	tgt1 := stampCreaturePT(addPerm(gs, 0, "Atraxa", "creature", "legendary"), 4, 4)
	tgt2 := stampCreaturePT(addPerm(gs, 0, "Yuriko", "creature", "legendary"), 1, 3)
	stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1) // nonlegendary skipped

	cleopatraAlliesEndStep(gs, cleo, map[string]interface{}{
		"active_seat": 0,
	})
	if tgt1.Counters["+1/+1"] != 1 {
		t.Errorf("Atraxa counter = %d, want 1", tgt1.Counters["+1/+1"])
	}
	if tgt2.Counters["+1/+1"] != 1 {
		t.Errorf("Yuriko counter = %d, want 1", tgt2.Counters["+1/+1"])
	}
	if cleo.Counters["+1/+1"] != 0 {
		t.Errorf("Cleopatra herself should NOT get a counter (each of up to two OTHER); got %d", cleo.Counters["+1/+1"])
	}
}

func TestCleopatra_AlliesSkipsOpponentEndStep(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	tgt := stampCreaturePT(addPerm(gs, 0, "Atraxa", "creature", "legendary"), 4, 4)

	cleopatraAlliesEndStep(gs, cleo, map[string]interface{}{
		"active_seat": 1, // not your end step
	})
	if tgt.Counters["+1/+1"] != 0 {
		t.Errorf("opponent's end step should NOT trigger Allies; got %d counters", tgt.Counters["+1/+1"])
	}
}

func TestCleopatra_AlliesPicksStrongestTwo(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	weak := stampCreaturePT(addPerm(gs, 0, "Weakling", "creature", "legendary"), 1, 1)
	mid := stampCreaturePT(addPerm(gs, 0, "Middling", "creature", "legendary"), 3, 3)
	strong := stampCreaturePT(addPerm(gs, 0, "Strongling", "creature", "legendary"), 7, 7)

	cleopatraAlliesEndStep(gs, cleo, map[string]interface{}{
		"active_seat": 0,
	})
	if strong.Counters["+1/+1"] != 1 || mid.Counters["+1/+1"] != 1 {
		t.Errorf("strongest two should get counters; strong=%d mid=%d",
			strong.Counters["+1/+1"], mid.Counters["+1/+1"])
	}
	if weak.Counters["+1/+1"] != 0 {
		t.Errorf("weakest legendary should NOT be picked when 3 are available; got %d", weak.Counters["+1/+1"])
	}
}

func TestCleopatra_BetrayalDrawsForCounterCount(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	addLibrary(gs, 0, "A", "B", "C", "D", "E")

	dyingCard := &gameengine.Card{
		Name:  "Atraxa",
		Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	dyingPerm := &gameengine.Permanent{
		Card:     dyingCard,
		Owner:    0,
		Controller: 0,
		Counters: map[string]int{"+1/+1": 3, "loyalty": 0},
		Flags:    map[string]int{},
	}
	preLife := gs.Seats[0].Life

	cleopatraBetrayalDies(gs, cleo, map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 3 {
		t.Errorf("expected to draw 3 (counter total); hand=%d names=%v",
			len(gs.Seats[0].Hand), hand_names(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != preLife-2 {
		t.Errorf("expected 2 life lost; life %d → %d", preLife, gs.Seats[0].Life)
	}
}

func TestCleopatra_BetrayalIgnoresNonLegendaryDeaths(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	addLibrary(gs, 0, "A", "B", "C")

	dyingCard := &gameengine.Card{
		Name:  "Llanowar Elves",
		Owner: 0,
		Types: []string{"creature"},
	}
	dyingPerm := &gameengine.Permanent{
		Card:       dyingCard,
		Owner:      0,
		Controller: 0,
		Counters:   map[string]int{"+1/+1": 5},
		Flags:      map[string]int{},
	}
	preLife := gs.Seats[0].Life
	cleopatraBetrayalDies(gs, cleo, map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("nonlegendary death should NOT trigger Betrayal; hand=%d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Life != preLife {
		t.Errorf("nonlegendary death should NOT cost life; life %d → %d", preLife, gs.Seats[0].Life)
	}
}

func TestCleopatra_BetrayalIgnoresCounterlessDeaths(t *testing.T) {
	gs := newGame(t, 2)
	cleo := stampCreaturePT(addPerm(gs, 0, "Cleopatra, Exiled Pharaoh", "creature", "legendary"), 3, 3)
	addLibrary(gs, 0, "A", "B", "C")

	dyingCard := &gameengine.Card{
		Name:  "Yuriko",
		Owner: 0,
		Types: []string{"creature", "legendary"},
	}
	dyingPerm := &gameengine.Permanent{
		Card:       dyingCard,
		Owner:      0,
		Controller: 0,
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	preLife := gs.Seats[0].Life
	cleopatraBetrayalDies(gs, cleo, map[string]interface{}{
		"perm":            dyingPerm,
		"card":            dyingCard,
		"controller_seat": 0,
	})
	if len(gs.Seats[0].Hand) != 0 || gs.Seats[0].Life != preLife {
		t.Errorf("counterless legendary death should NOT trigger Betrayal; hand=%d life=%d",
			len(gs.Seats[0].Hand), gs.Seats[0].Life)
	}
}

// ---------------------------------------------------------------------------
// Rakdos, Lord of Riots — cost reduction via cost_modifiers.go
// ---------------------------------------------------------------------------

func TestRakdosLordOfRiots_CreatureSpellGetsLifeLossDiscount(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Rakdos, Lord of Riots", "creature", "legendary"), 6, 6)
	// Seat 1 has lost 5 life this turn.
	if gs.Seats[1].Flags == nil {
		gs.Seats[1].Flags = map[string]int{}
	}
	gs.Seats[1].Turn.LifeLost = 5

	creature := &gameengine.Card{
		Name:  "Big Beater",
		Owner: 0,
		Types: []string{"creature"},
	}
	mods := gameengine.ScanCostModifiers(gs, creature, 0)
	total := 0
	for _, m := range mods {
		if m.Source == "Rakdos, Lord of Riots" {
			total += m.Amount
		}
	}
	if total != 5 {
		t.Errorf("expected 5-mana Rakdos discount, got %d (mods=%+v)", total, mods)
	}
}

func TestRakdosLordOfRiots_NonCreatureSpellGetsNoDiscount(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Rakdos, Lord of Riots", "creature", "legendary"), 6, 6)
	gs.Seats[1].Turn.LifeLost = 7

	noncreature := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
	}
	mods := gameengine.ScanCostModifiers(gs, noncreature, 0)
	for _, m := range mods {
		if m.Source == "Rakdos, Lord of Riots" {
			t.Errorf("noncreature spells should NOT get Rakdos discount; mod=%+v", m)
		}
	}
}

func TestRakdosLordOfRiots_OwnLifeLossDoesNotCount(t *testing.T) {
	gs := newGame(t, 2)
	stampCreaturePT(addPerm(gs, 0, "Rakdos, Lord of Riots", "creature", "legendary"), 6, 6)
	gs.Seats[0].Turn.LifeLost = 8 // Rakdos's controller, not opponent
	gs.Seats[1].Turn.LifeLost = 0

	creature := &gameengine.Card{
		Name:  "Tiny",
		Owner: 0,
		Types: []string{"creature"},
	}
	mods := gameengine.ScanCostModifiers(gs, creature, 0)
	for _, m := range mods {
		if m.Source == "Rakdos, Lord of Riots" {
			t.Errorf("self-life-loss should NOT discount Rakdos cost; mod=%+v", m)
		}
	}
}

// ---------------------------------------------------------------------------
// Iron Man, Titan of Innovation — attack → Treasure + maybe tutor
// ---------------------------------------------------------------------------

func TestIronMan_AttackAlwaysCreatesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	iron := stampCreaturePT(addPerm(gs, 0, "Iron Man, Titan of Innovation", "creature", "legendary", "artifact"), 5, 5)
	// No other artifacts; no library hits.
	preBF := len(gs.Seats[0].Battlefield)
	ironManGeniusIndustrialist(gs, iron, map[string]interface{}{
		"attacker_perm": iron,
		"attacker_seat": 0,
	})
	// Treasure should always land.
	foundTreasure := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && (p.Card.Name == "Treasure Token" || p.Card.Name == "Treasure") {
			foundTreasure = true
			break
		}
	}
	if !foundTreasure {
		t.Fatal("Treasure should be created on attack regardless of sac/tutor outcome")
	}
	if got := len(gs.Seats[0].Battlefield); got != preBF+1 {
		t.Errorf("battlefield delta = %d, want exactly +1 (just the Treasure)", got-preBF)
	}
}

func TestIronMan_SacsTreasureForOneMVTutor(t *testing.T) {
	gs := newGame(t, 2)
	iron := stampCreaturePT(addPerm(gs, 0, "Iron Man, Titan of Innovation", "creature", "legendary", "artifact"), 5, 5)

	// Stock library with a 1-MV artifact (matches Treasure CMC 0 + 1).
	oneArtifact := &gameengine.Card{
		Name:  "Sol Ring",
		Owner: 0,
		CMC:   1,
		Types: []string{"artifact", "cmc:1"},
	}
	gs.Seats[0].Library = append(gs.Seats[0].Library, oneArtifact)

	ironManGeniusIndustrialist(gs, iron, map[string]interface{}{
		"attacker_perm": iron,
		"attacker_seat": 0,
	})

	// Sol Ring should now be on the battlefield tapped.
	foundSol := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && p.Card.Name == "Sol Ring" {
			foundSol = true
			if !p.Tapped {
				t.Error("tutored artifact should enter tapped")
			}
			break
		}
	}
	if !foundSol {
		t.Fatal("expected Sol Ring on battlefield after sac-Treasure tutor")
	}
	// Library should be empty (only one card was in it, now tutored out).
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("library should be empty post-tutor, got %d", len(gs.Seats[0].Library))
	}
	// Treasure should have been sacrificed (so 0 Treasures on board).
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && (p.Card.Name == "Treasure Token" || p.Card.Name == "Treasure") {
			t.Errorf("Treasure should have been sacrificed for the tutor; still on bf")
		}
	}
}

func TestIronMan_SkipsTutorWhenNoLibraryHit(t *testing.T) {
	gs := newGame(t, 2)
	iron := stampCreaturePT(addPerm(gs, 0, "Iron Man, Titan of Innovation", "creature", "legendary", "artifact"), 5, 5)

	// Library has only a creature — Iron Man can't tutor it.
	gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{
		Name: "Llanowar Elves", Owner: 0, Types: []string{"creature", "cmc:1"}, CMC: 1,
	})

	ironManGeniusIndustrialist(gs, iron, map[string]interface{}{
		"attacker_perm": iron,
		"attacker_seat": 0,
	})

	// Treasure landed, but nothing sacrificed and nothing put onto bf
	// from library — Llanowar Elves is still in the library.
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("library should still hold Llanowar Elves (no artifact hit); got %d", len(gs.Seats[0].Library))
	}
}

func TestIronMan_IgnoresOtherAttackers(t *testing.T) {
	gs := newGame(t, 2)
	iron := stampCreaturePT(addPerm(gs, 0, "Iron Man, Titan of Innovation", "creature", "legendary", "artifact"), 5, 5)
	other := stampCreaturePT(addPerm(gs, 0, "Llanowar Elves", "creature"), 1, 1)

	preBF := len(gs.Seats[0].Battlefield)
	ironManGeniusIndustrialist(gs, iron, map[string]interface{}{
		"attacker_perm": other,
		"attacker_seat": 0,
	})
	if got := len(gs.Seats[0].Battlefield); got != preBF {
		t.Errorf("non-Iron-Man attacker should NOT trigger; bf delta=%d", got-preBF)
	}
}

// ---------------------------------------------------------------------------
// Yorion, Sky Nomad — ETB blink + delayed return
// ---------------------------------------------------------------------------

func TestYorion_ETBExilesOwnNonlandPermanents(t *testing.T) {
	gs := newGame(t, 2)
	yorion := stampCreaturePT(addPerm(gs, 0, "Yorion, Sky Nomad", "creature", "legendary"), 4, 5)

	addPerm(gs, 0, "Plains", "land", "basic")        // skipped: land
	other := addPerm(gs, 0, "Sol Ring", "artifact")  // exiled
	oppPerm := addPerm(gs, 1, "Lotus Petal", "artifact") // skipped: not owned

	yorionSkyNomadETB(gs, yorion)

	// Yorion himself still on bf.
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == yorion {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Error("Yorion should remain on battlefield; only OTHER permanents blink")
	}

	// Sol Ring should be in exile, not on bf.
	for _, p := range gs.Seats[0].Battlefield {
		if p == other {
			t.Error("Sol Ring should have been exiled by Yorion ETB")
		}
	}
	if len(gs.Seats[0].Exile) < 1 {
		t.Errorf("expected ≥1 card in exile; got %d", len(gs.Seats[0].Exile))
	}

	// Opponent's permanent untouched.
	oppStill := false
	for _, p := range gs.Seats[1].Battlefield {
		if p == oppPerm {
			oppStill = true
		}
	}
	if !oppStill {
		t.Error("opponent's permanents should NOT be exiled by Yorion (you own AND control)")
	}

	// Delayed trigger queued for the return.
	if len(gs.DelayedTriggers) < 1 {
		t.Error("expected a next_end_step delayed trigger queued for the return")
	}
}

func TestYorion_DelayedReturnReinstatesExiledCards(t *testing.T) {
	gs := newGame(t, 2)
	yorion := stampCreaturePT(addPerm(gs, 0, "Yorion, Sky Nomad", "creature", "legendary"), 4, 5)
	rock := addPerm(gs, 0, "Sol Ring", "artifact")

	yorionSkyNomadETB(gs, yorion)

	// Confirm Sol Ring is in exile now.
	foundExiled := false
	for _, c := range gs.Seats[0].Exile {
		if c == rock.Card {
			foundExiled = true
			break
		}
	}
	if !foundExiled {
		t.Fatal("Sol Ring should be in exile after Yorion ETB")
	}
	// Run the delayed trigger directly.
	if len(gs.DelayedTriggers) < 1 {
		t.Fatal("delayed trigger missing")
	}
	dt := gs.DelayedTriggers[len(gs.DelayedTriggers)-1]
	if dt.EffectFn == nil {
		t.Fatal("delayed trigger has no EffectFn")
	}
	dt.EffectFn(gs)

	// Sol Ring should be back on the battlefield, off exile.
	for _, c := range gs.Seats[0].Exile {
		if c == rock.Card {
			t.Error("Sol Ring should have left exile after return")
		}
	}
	backOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card == rock.Card {
			backOnBF = true
			break
		}
	}
	if !backOnBF {
		t.Error("Sol Ring should be back on the battlefield post-return")
	}
}

func TestYorion_SkipsTokens(t *testing.T) {
	gs := newGame(t, 2)
	yorion := stampCreaturePT(addPerm(gs, 0, "Yorion, Sky Nomad", "creature", "legendary"), 4, 5)
	// Add a token permanent. IsToken() checks Card.Types containing "token".
	tok := addPerm(gs, 0, "Treasure", "artifact", "token", "treasure")

	yorionSkyNomadETB(gs, yorion)

	// Token should still be on battlefield (would cease to exist in exile).
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == tok {
			stillThere = true
			break
		}
	}
	if !stillThere {
		t.Error("tokens should NOT be exiled by Yorion (they'd cease to exist in exile)")
	}
}
