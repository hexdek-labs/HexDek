package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// beheld_consumers_r60 — 5 handlers wired in beheld_consumers_r60.go
// covering the §701.4 Behold keyword-consumer family (Sarkhan ETB
// rider + TDM Dragon-Exhale cycle + Draconic Fealty + Territorial
// Strike).
// -----------------------------------------------------------------------------

// putDragonInHand adds a Dragon creature card to seat's hand. The
// engine's CardHasBeholdQuality matches against Card.Types so the
// "dragon" subtype tag is enough — the corpus-loaded "creature dragon"
// subtype split happens upstream of these tests.
func putDragonInHand(gs *gameengine.GameState, seat int, name string) *gameengine.Card {
	c := &gameengine.Card{
		Name:  name,
		Owner: seat,
		Types: []string{"creature", "dragon"},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

func plainResolveItem(seat int, cardName, kind string) *gameengine.StackItem {
	return &gameengine.StackItem{
		Controller: seat,
		Card: &gameengine.Card{
			Name:  cardName,
			Owner: seat,
			Types: []string{kind},
		},
	}
}

// -----------------------------------------------------------------------------
// Sarkhan, Dragon Ascendant — ETB rider
// -----------------------------------------------------------------------------

func TestSarkhanDragonAscendant_ETBWithDragonInHandMintsTreasure(t *testing.T) {
	gs := newGame(t, 2)
	putDragonInHand(gs, 0, "Shivan Dragon")
	sarkhan := addPerm(gs, 0, "Sarkhan, Dragon Ascendant", "creature")

	gameengine.InvokeETBHook(gs, sarkhan)

	if !gameengine.HasBeheld(gs, 0, "dragon") {
		t.Errorf("BeholdRegistry must record dragon after Sarkhan ETB; events=%+v", gs.EventLog)
	}
	treasures := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && cardHasType(p.Card, "treasure") {
			treasures++
		}
	}
	if treasures != 1 {
		t.Errorf("expected 1 Treasure minted on successful behold, got %d", treasures)
	}
}

func TestSarkhanDragonAscendant_ETBWithoutDragonFails(t *testing.T) {
	gs := newGame(t, 2)
	// Hand has only non-dragons; battlefield has only the Sarkhan itself
	// (which is a Human Shaman, not a Dragon — does not satisfy behold).
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
	})
	sarkhan := addPerm(gs, 0, "Sarkhan, Dragon Ascendant", "creature")

	gameengine.InvokeETBHook(gs, sarkhan)

	if gameengine.HasBeheld(gs, 0, "dragon") {
		t.Errorf("no behold should be recorded without a Dragon in hand or play")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no Dragon available, events=%+v", gs.EventLog)
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p.Card != nil && cardHasType(p.Card, "treasure") {
			t.Errorf("must NOT mint a Treasure without a successful behold")
		}
	}
}

// -----------------------------------------------------------------------------
// Osseous Exhale — instant with lifegain rider
// -----------------------------------------------------------------------------

func TestOsseousExhale_NoBeholdJustDamages(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 6
	startLife := gs.Seats[0].Life

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Osseous Exhale", "instant"))

	if target.MarkedDamage != 5 {
		t.Errorf("expected 5 damage marked on target, got %d", target.MarkedDamage)
	}
	if gs.Seats[0].Life != startLife {
		t.Errorf("life must not change without a beheld Dragon: before=%d after=%d", startLife, gs.Seats[0].Life)
	}
}

func TestOsseousExhale_BeholdGains2Life(t *testing.T) {
	gs := newGame(t, 2)
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 6
	putDragonInHand(gs, 0, "Shivan Dragon")
	startLife := gs.Seats[0].Life

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Osseous Exhale", "instant"))

	if target.MarkedDamage != 5 {
		t.Errorf("damage must still apply: got %d, want 5", target.MarkedDamage)
	}
	if gs.Seats[0].Life != startLife+2 {
		t.Errorf("expected +2 life from beheld rider, got %d → %d", startLife, gs.Seats[0].Life)
	}
	if !gameengine.HasBeheld(gs, 0, "dragon") {
		t.Errorf("BeheldRegistry must record dragon")
	}
}

// -----------------------------------------------------------------------------
// Piercing Exhale — instant with surveil rider
// -----------------------------------------------------------------------------

func TestPiercingExhale_NoBeholdPingsButNoSurveil(t *testing.T) {
	gs := newGame(t, 2)
	source := addPerm(gs, 0, "Llanowar Elves", "creature")
	source.Card.BasePower, source.Card.BaseToughness = 4, 4
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 6
	addLibrary(gs, 0, "Top1", "Top2", "Top3")

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Piercing Exhale", "instant"))

	if target.MarkedDamage != 4 {
		t.Errorf("expected source.Power=4 damage on target, got %d", target.MarkedDamage)
	}
	if hasEvent(gs, "surveil") > 0 {
		t.Errorf("must not surveil without a beheld Dragon; events=%+v", gs.EventLog)
	}
}

func TestPiercingExhale_BeholdSurveils2(t *testing.T) {
	gs := newGame(t, 2)
	source := addPerm(gs, 0, "Llanowar Elves", "creature")
	source.Card.BasePower, source.Card.BaseToughness = 4, 4
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 6
	putDragonInHand(gs, 0, "Shivan Dragon")
	addLibrary(gs, 0, "Top1", "Top2", "Top3")

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Piercing Exhale", "instant"))

	if target.MarkedDamage != 4 {
		t.Errorf("damage must still apply: got %d, want 4", target.MarkedDamage)
	}
	if hasEvent(gs, "surveil") < 1 {
		t.Errorf("expected surveil event after successful behold; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Draconic Fealty — sorcery with discard + graveyard-exile rider
// -----------------------------------------------------------------------------

func TestDraconicFealty_NoBeholdDiscardsHighestCMC(t *testing.T) {
	gs := newGame(t, 2)
	// Opponent hand: 3 cards at CMC 1, 4, 2 (highest is CMC 4).
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "Bolt", Owner: 1, Types: []string{"instant", "cmc:1"}, CMC: 1},
		{Name: "BigSpell", Owner: 1, Types: []string{"sorcery", "cmc:4"}, CMC: 4},
		{Name: "Counter", Owner: 1, Types: []string{"instant", "cmc:2"}, CMC: 2},
	}
	// Opponent graveyard has something that must NOT be exiled.
	gs.Seats[1].Graveyard = []*gameengine.Card{{Name: "DeadGoblin", Owner: 1}}

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Draconic Fealty", "sorcery"))

	for _, c := range gs.Seats[1].Hand {
		if c.DisplayName() == "BigSpell" {
			t.Errorf("highest-CMC card must have been discarded; still in hand: %s", c.DisplayName())
		}
	}
	if len(gs.Seats[1].Graveyard) < 1 {
		t.Errorf("graveyard must NOT be exiled without a beheld Dragon, got %d cards", len(gs.Seats[1].Graveyard))
	}
}

func TestDraconicFealty_BeholdExilesGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Hand = []*gameengine.Card{
		{Name: "BigSpell", Owner: 1, Types: []string{"sorcery", "cmc:4"}, CMC: 4},
	}
	gs.Seats[1].Graveyard = []*gameengine.Card{
		{Name: "DeadGoblin", Owner: 1},
		{Name: "DeadSorcery", Owner: 1},
	}
	putDragonInHand(gs, 0, "Shivan Dragon")

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Draconic Fealty", "sorcery"))

	if len(gs.Seats[1].Graveyard) != 0 {
		t.Errorf("graveyard must be empty after beheld exile, got %d cards", len(gs.Seats[1].Graveyard))
	}
	// 3 = 2 pre-existing graveyard cards + the just-discarded BigSpell
	// (discard resolves before the exile-graveyard rider, so the
	// discarded card is in the graveyard when the exile sweeps it).
	if len(gs.Seats[1].Exile) != 3 {
		t.Errorf("expected 3 cards in exile (2 pre-existing + 1 discarded), got %d", len(gs.Seats[1].Exile))
	}
}

// -----------------------------------------------------------------------------
// Territorial Strike — sorcery with destroy + perpetual-buff rider
// -----------------------------------------------------------------------------

func TestTerritorialStrike_NoBeholdJustDestroys(t *testing.T) {
	gs := newGame(t, 2)
	doomed := addPerm(gs, 1, "Big Threat", "creature")
	doomed.Card.BasePower, doomed.Card.BaseToughness = 5, 5

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Territorial Strike", "sorcery"))

	for _, p := range gs.Seats[1].Battlefield {
		if p == doomed {
			t.Errorf("Big Threat should have been destroyed")
		}
	}
}

func TestTerritorialStrike_BeholdAlsoBuffsFriendlyDragon(t *testing.T) {
	gs := newGame(t, 2)
	doomed := addPerm(gs, 1, "Big Threat", "creature")
	doomed.Card.BasePower, doomed.Card.BaseToughness = 5, 5

	// Friendly Dragon on the battlefield serves both as the behold
	// target (choose-permanent path) and as the buff recipient.
	dragon := addPerm(gs, 0, "Stormbreath Dragon", "creature", "dragon")
	dragon.Card.BasePower, dragon.Card.BaseToughness = 4, 4
	preP, preT := dragon.Power(), dragon.Toughness()

	gameengine.InvokeResolveHook(gs, plainResolveItem(0, "Territorial Strike", "sorcery"))

	// Destruction still happens.
	for _, p := range gs.Seats[1].Battlefield {
		if p == doomed {
			t.Errorf("Big Threat should have been destroyed")
		}
	}
	// +2/+2 stamped on the Dragon.
	if dragon.Power() != preP+2 || dragon.Toughness() != preT+2 {
		t.Errorf("expected +2/+2 stamp on friendly Dragon, got %d/%d → %d/%d",
			preP, preT, dragon.Power(), dragon.Toughness())
	}
	// Perpetual-approximation breadcrumb so auditors can see the gap.
	if hasEvent(gs, "per_card_partial") < 1 {
		t.Errorf("expected per_card_partial breadcrumb for perpetual approximation; events=%+v", gs.EventLog)
	}
}

// -----------------------------------------------------------------------------
// Quality-agnostic behold attempt — Dragon-only quality matching
// -----------------------------------------------------------------------------

// TestAttemptBeholdQuality_RejectsNonMatchingTypes guards the helper
// from a regression where "behold a Dragon" might accidentally
// satisfy via any creature in hand. Non-Dragon cards must not
// satisfy the behold.
func TestAttemptBeholdQuality_RejectsNonMatchingTypes(t *testing.T) {
	gs := newGame(t, 2)
	// Hand: a non-Dragon creature + a sorcery. Neither satisfies
	// "behold a Dragon".
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Llanowar Elves", Owner: 0, Types: []string{"creature", "elf"}},
		{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}},
	}

	if attemptBeholdQuality(gs, 0, "dragon", "test") {
		t.Errorf("attemptBeholdQuality must reject when hand has no Dragon")
	}
	if gameengine.HasBeheld(gs, 0, "dragon") {
		t.Errorf("BeheldRegistry must stay empty on rejection")
	}
}

// -----------------------------------------------------------------------------
// Registry smoke test — pin that all 5 handlers are wired
// -----------------------------------------------------------------------------

func TestBeheldConsumersR60_AllHandlersRegistered(t *testing.T) {
	if !HasETB("Sarkhan, Dragon Ascendant") {
		t.Errorf("missing ETB handler: Sarkhan, Dragon Ascendant")
	}
	for _, name := range []string{
		"Osseous Exhale",
		"Piercing Exhale",
		"Draconic Fealty",
		"Territorial Strike",
	} {
		if !HasResolve(name) {
			t.Errorf("missing Resolve handler: %s", name)
		}
	}
}
