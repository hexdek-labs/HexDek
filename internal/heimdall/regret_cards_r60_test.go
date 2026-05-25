package heimdall

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60: ExtractRegretCards flags nonland CMC≥1 cards stranded in a
// seat's hand at game end — the "drew it, never cast it" half of the
// regret signal. Fizzle/no-legal-target/redundant detection is a
// separate signal that requires RetainEvents and is deferred.

func newRegretGS(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
}

func handCard(name string, cmc int, types ...string) *gameengine.Card {
	return &gameengine.Card{Name: name, CMC: cmc, Types: types}
}

// Basic case: one stranded high-CMC spell in hand becomes a regret
// record with reason="stranded_in_hand".
func TestExtractRegretCards_StrandedHighCMCSpell(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Blightsteel Colossus", 12, "artifact", "creature"),
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 1 {
		t.Fatalf("want 1 regret, got %d", len(regrets))
	}
	r := regrets[0]
	if r.Seat != 0 || r.CardName != "Blightsteel Colossus" || r.CMC != 12 || r.Reason != "stranded_in_hand" {
		t.Errorf("unexpected regret: %+v", r)
	}
}

// Lands in hand are not regrets — flood/screw is a separate signal.
func TestExtractRegretCards_LandsExcluded(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Forest", 0, "land", "basic"),
		handCard("Command Tower", 0, "land"),
		handCard("Sol Ring", 1, "artifact"),
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 1 || regrets[0].CardName != "Sol Ring" {
		t.Errorf("want only Sol Ring as regret, got %+v", regrets)
	}
}

// 0-CMC cards (free spells, modal back faces returning 0) are not
// regrets — holding a Lotus Petal isn't a mana-affordability regret.
func TestExtractRegretCards_ZeroCMCExcluded(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Lotus Petal", 0, "artifact"),
		handCard("Mox Diamond", 0, "artifact"),
		handCard("Thoughtseize", 1, "sorcery"),
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 1 || regrets[0].CardName != "Thoughtseize" {
		t.Errorf("want only Thoughtseize as regret, got %+v", regrets)
	}
}

// Multi-seat ordering: sort is seat-ascending then CMC-descending so
// the biggest regret per seat surfaces first.
func TestExtractRegretCards_SortingSeatThenCMCDesc(t *testing.T) {
	gs := newRegretGS(t, 4)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Lightning Bolt", 1, "instant"),
		handCard("Cyclonic Rift", 7, "instant"),
		handCard("Counterspell", 2, "instant"),
	}
	gs.Seats[2].Hand = []*gameengine.Card{
		handCard("Craterhoof Behemoth", 8, "creature"),
		handCard("Llanowar Elves", 1, "creature"),
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 5 {
		t.Fatalf("want 5 regrets, got %d", len(regrets))
	}
	want := []struct {
		seat int
		name string
		cmc  int
	}{
		{0, "Cyclonic Rift", 7},
		{0, "Counterspell", 2},
		{0, "Lightning Bolt", 1},
		{2, "Craterhoof Behemoth", 8},
		{2, "Llanowar Elves", 1},
	}
	for i, w := range want {
		got := regrets[i]
		if got.Seat != w.seat || got.CardName != w.name || got.CMC != w.cmc {
			t.Errorf("position %d: want (seat=%d %q cmc=%d), got %+v", i, w.seat, w.name, w.cmc, got)
		}
	}
}

// Equal-CMC tie-breaker: cards within the same seat at the same CMC
// sort by name ascending for determinism.
func TestExtractRegretCards_EqualCMCTieBreakByName(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Zur the Enchanter", 4, "creature", "legendary"),
		handCard("Atraxa, Praetors' Voice", 4, "creature", "legendary"),
		handCard("Muldrotha, the Gravetide", 6, "creature", "legendary"),
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 3 {
		t.Fatalf("want 3, got %d", len(regrets))
	}
	if regrets[0].CardName != "Muldrotha, the Gravetide" {
		t.Errorf("position 0 should be Muldrotha (highest CMC); got %s", regrets[0].CardName)
	}
	if regrets[1].CardName != "Atraxa, Praetors' Voice" {
		t.Errorf("position 1 should be Atraxa (CMC4, alphabetical); got %s", regrets[1].CardName)
	}
	if regrets[2].CardName != "Zur the Enchanter" {
		t.Errorf("position 2 should be Zur (CMC4, alphabetical); got %s", regrets[2].CardName)
	}
}

// Empty hand → no regrets, nil/empty slice OK.
func TestExtractRegretCards_EmptyHands(t *testing.T) {
	gs := newRegretGS(t, 4)
	if got := ExtractRegretCards(gs); len(got) != 0 {
		t.Errorf("empty hands should produce no regrets, got %+v", got)
	}
}

// Modal DFC: when CastingBackFace is set the back-face CMC is what
// drives the regret tag. A card with front CMC=5 and back CMC=2,
// flipped to back, reports cmc=2.
func TestExtractRegretCards_ModalDFCUsesEffectiveCMC(t *testing.T) {
	gs := newRegretGS(t, 2)
	c := handCard("Valki, God of Lies", 2, "creature", "legendary")
	c.BackFaceCMC = 7
	c.CastingBackFace = true
	gs.Seats[0].Hand = []*gameengine.Card{c}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 1 || regrets[0].CMC != 7 {
		t.Errorf("want back-face CMC=7 from EffectiveCMC; got %+v", regrets)
	}
}

// Defensive: nil game state and nil/empty seats.
func TestExtractRegretCards_NilGameStateReturnsNil(t *testing.T) {
	if got := ExtractRegretCards(nil); got != nil {
		t.Errorf("nil gs should return nil, got %v", got)
	}
}

func TestExtractRegretCards_NilCardEntryIgnored(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		nil,
		handCard("Sol Ring", 1, "artifact"),
		nil,
	}

	regrets := ExtractRegretCards(gs)
	if len(regrets) != 1 || regrets[0].CardName != "Sol Ring" {
		t.Errorf("nil hand entries should be skipped; got %+v", regrets)
	}
}

// All-land hand (mana-screwed-by-flooding pilot): no regrets surface.
// Lands are intentionally filtered; the flood signal lives elsewhere.
func TestExtractRegretCards_AllLandsNoRegrets(t *testing.T) {
	gs := newRegretGS(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		handCard("Forest", 0, "land", "basic"),
		handCard("Mountain", 0, "land", "basic"),
		handCard("Island", 0, "land", "basic"),
	}
	if got := ExtractRegretCards(gs); len(got) != 0 {
		t.Errorf("all-land hand should not produce regrets, got %+v", got)
	}
}
