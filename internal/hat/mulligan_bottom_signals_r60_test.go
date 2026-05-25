package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mulligan_bottom_signals_r60_test.go — pins the R60 round 6+ London
// bottom-pile signals added to ChooseBottomCards. Two helpers each get
// unit tests + one integration test verifying they actually change the
// bottom pile in a realistic mulligan shape.

// mkLand builds a basic land card with the typeline / Types fields
// landProducesColorsMask + typeLineContains both need.
func mkLand(name, basicType string) *gameengine.Card {
	return &gameengine.Card{
		Name:     name,
		Types:    []string{"land"},
		TypeLine: "Basic Land — " + basicType,
	}
}

// mkSpell builds a non-land card with the printed ManaCostString our
// color-deadness logic needs. cmc is the integer cost folded into Types
// as "cost:N" so ManaCostOf returns it.
func mkSpell(name string, cmc int, types []string, manaCost string) *gameengine.Card {
	c := &gameengine.Card{
		Name:           name,
		Types:          append([]string{}, types...),
		ManaCostString: manaCost,
		TypeLine:       name,
	}
	if cmc > 0 {
		c.Types = append(c.Types, "cost:"+itoa(cmc))
	}
	return c
}

// -----------------------------------------------------------------------
// handProducedColorMask
// -----------------------------------------------------------------------

func TestHandProducedColorMask_EmptyHand(t *testing.T) {
	if got := handProducedColorMask(nil); got != 0 {
		t.Errorf("nil hand should produce 0 mask, got %b", got)
	}
	if got := handProducedColorMask([]*gameengine.Card{}); got != 0 {
		t.Errorf("empty hand should produce 0 mask, got %b", got)
	}
}

func TestHandProducedColorMask_MonoBlue(t *testing.T) {
	hand := []*gameengine.Card{
		mkLand("Island", "Island"),
		mkLand("Island", "Island"),
		mkSpell("Counterspell", 2, []string{"instant"}, "{U}{U}"),
	}
	mask := handProducedColorMask(hand)
	if mask&colorBitU == 0 {
		t.Errorf("hand with Islands should include U bit; mask=%b", mask)
	}
	if mask&(colorBitW|colorBitB|colorBitR|colorBitG) != 0 {
		t.Errorf("mono-blue hand should not include other color bits; mask=%b", mask)
	}
}

func TestHandProducedColorMask_IgnoresSpells(t *testing.T) {
	// A non-land card with "{U}" in its cost is not a mana source.
	hand := []*gameengine.Card{
		mkSpell("Brainstorm", 1, []string{"instant"}, "{U}"),
	}
	if got := handProducedColorMask(hand); got != 0 {
		t.Errorf("non-land cards should never contribute to produced colors; mask=%b", got)
	}
}

// -----------------------------------------------------------------------
// cardColorDeadAgainstHand
// -----------------------------------------------------------------------

func TestCardColorDead_NoCostStringNeverDead(t *testing.T) {
	c := &gameengine.Card{Name: "Token", Types: []string{"creature"}}
	if cardColorDeadAgainstHand(c, 0) {
		t.Error("card with empty ManaCostString should never be color-dead (engine-minted tokens)")
	}
}

func TestCardColorDead_LandsNeverDead(t *testing.T) {
	c := mkLand("Mountain", "Mountain")
	if cardColorDeadAgainstHand(c, 0) {
		t.Error("lands should never be flagged color-dead")
	}
}

func TestCardColorDead_ColorlessNeverDead(t *testing.T) {
	c := mkSpell("Sol Ring", 1, []string{"artifact"}, "{1}")
	if cardColorDeadAgainstHand(c, 0) {
		t.Error("colorless cards should never be color-dead — generic pips pay from any land")
	}
}

func TestCardColorDead_PureColorMissingFires(t *testing.T) {
	c := mkSpell("Counterspell", 2, []string{"instant"}, "{U}{U}")
	// Hand only produces white.
	if !cardColorDeadAgainstHand(c, colorBitW) {
		t.Error("Counterspell {U}{U} should be color-dead against mono-W hand")
	}
}

func TestCardColorDead_PureColorPresentDoesNotFire(t *testing.T) {
	c := mkSpell("Counterspell", 2, []string{"instant"}, "{U}{U}")
	if cardColorDeadAgainstHand(c, colorBitU) {
		t.Error("Counterspell {U}{U} should NOT be color-dead when hand produces U")
	}
}

func TestCardColorDead_HybridSatisfiedByEitherColor(t *testing.T) {
	// Boros Charm has cost {R}{W}; an Orzhov Charm {W}{B} is partial-hybrid.
	// Use a true hybrid: {R/W} pip.
	c := mkSpell("Boros Reckoner", 3, []string{"creature"}, "{R/W}{R/W}{R/W}")
	// Mono-white hand — each hybrid pip is satisfied by W alone.
	if cardColorDeadAgainstHand(c, colorBitW) {
		t.Error("hybrid {R/W} pip should be satisfied by W alone")
	}
	// Mono-blue hand — none of the hybrid colors available, dead.
	if !cardColorDeadAgainstHand(c, colorBitU) {
		t.Error("hybrid {R/W} pip should be dead vs mono-U hand")
	}
}

func TestCardColorDead_MultiPipPartialPresenceStillDead(t *testing.T) {
	// {U}{B} card; hand produces only U. The {B} pip is unsatisfied.
	c := mkSpell("Negate-ish", 2, []string{"instant"}, "{U}{B}")
	if !cardColorDeadAgainstHand(c, colorBitU) {
		t.Error("multi-color card with one unsatisfied pip should be color-dead")
	}
}

// -----------------------------------------------------------------------
// countCheapPlayables + isEarlyPlayAnchor
// -----------------------------------------------------------------------

func TestCountCheapPlayables_LandsExcluded(t *testing.T) {
	hand := []*gameengine.Card{
		mkLand("Plains", "Plains"),
		mkLand("Island", "Island"),
		mkLand("Forest", "Forest"),
	}
	if got := countCheapPlayables(hand); got != 0 {
		t.Errorf("all-land hand should have 0 cheap playables, got %d", got)
	}
}

func TestCountCheapPlayables_TwoCheapOneExpensive(t *testing.T) {
	hand := []*gameengine.Card{
		mkSpell("Sol Ring", 1, []string{"artifact"}, "{1}"),
		mkSpell("Bear", 2, []string{"creature"}, "{1}{G}"),
		mkSpell("Big Guy", 7, []string{"creature"}, "{5}{B}{B}"),
	}
	if got := countCheapPlayables(hand); got != 2 {
		t.Errorf("expected 2 cheap playables (Sol Ring + Bear), got %d", got)
	}
}

func TestIsEarlyPlayAnchor_FiresOnSoleCheapPlayable(t *testing.T) {
	bear := mkSpell("Bear", 2, []string{"creature"}, "{1}{G}")
	if !isEarlyPlayAnchor(bear, 1) {
		t.Error("the sole cheap playable should anchor")
	}
}

func TestIsEarlyPlayAnchor_QuietWhenMultiple(t *testing.T) {
	bear := mkSpell("Bear", 2, []string{"creature"}, "{1}{G}")
	if isEarlyPlayAnchor(bear, 2) {
		t.Error("anchor should not fire when there are multiple cheap playables")
	}
}

func TestIsEarlyPlayAnchor_QuietWhenZero(t *testing.T) {
	bear := mkSpell("Bear", 2, []string{"creature"}, "{1}{G}")
	if isEarlyPlayAnchor(bear, 0) {
		t.Error("anchor should not fire when the count is 0 (caller logic guards)")
	}
}

func TestIsEarlyPlayAnchor_LandsNeverAnchor(t *testing.T) {
	plains := mkLand("Plains", "Plains")
	if isEarlyPlayAnchor(plains, 1) {
		t.Error("a land should never be flagged as an early-play anchor")
	}
}

func TestIsEarlyPlayAnchor_ExpensiveCardNeverAnchor(t *testing.T) {
	big := mkSpell("Big Guy", 5, []string{"creature"}, "{3}{R}{R}")
	if isEarlyPlayAnchor(big, 1) {
		t.Error("CMC 5 card should never be flagged as an early-play anchor even when alone")
	}
}

// -----------------------------------------------------------------------
// Integration: ChooseBottomCards actually bottoms the color-dead card
// -----------------------------------------------------------------------

// TestChooseBottomCards_PrefersBottomingColorDead pins the integration:
// in a mono-U mulligan hand with both a Counterspell {U}{U} and a
// Lightning Bolt {R} (color-dead), bottoming one card should bottom the
// Lightning Bolt. Without the signal both cards are cheap instants of
// roughly equal cardHeuristic value — the tiebreaker should fall on
// "the one I can never cast" first.
func TestChooseBottomCards_PrefersBottomingColorDead(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeControl}, 0)

	island1 := mkLand("Island1", "Island")
	island2 := mkLand("Island2", "Island")
	counter := mkSpell("Counterspell", 2, []string{"instant"}, "{U}{U}")
	bolt := mkSpell("Lightning Bolt", 1, []string{"instant"}, "{R}")
	// Filler so count=1 has a real choice to make.
	hand := []*gameengine.Card{island1, island2, counter, bolt}

	bottomed := h.ChooseBottomCards(gs, 0, hand, 1)
	if len(bottomed) != 1 {
		t.Fatalf("expected 1 bottomed card, got %d", len(bottomed))
	}
	if bottomed[0].Name != "Lightning Bolt" {
		t.Errorf("expected Lightning Bolt (color-dead in mono-U hand) to be bottomed, got %s",
			bottomed[0].Name)
	}
}

// TestChooseBottomCards_KeepsEarlyPlayAnchor pins the second integration:
// a hand with a single 2-CMC playable (Bear) plus expensive cards should
// NOT bottom the Bear. Without the anchor signal, the Bear and a 6-CMC
// creature would both rank near the floor of cardHeuristic and the
// sort could put either first.
func TestChooseBottomCards_KeepsEarlyPlayAnchor(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)

	forest1 := mkLand("Forest1", "Forest")
	forest2 := mkLand("Forest2", "Forest")
	bear := mkSpell("Bear", 2, []string{"creature"}, "{1}{G}")
	big1 := mkSpell("BigA", 6, []string{"creature"}, "{4}{G}{G}")
	big2 := mkSpell("BigB", 7, []string{"creature"}, "{5}{G}{G}")
	hand := []*gameengine.Card{forest1, forest2, bear, big1, big2}

	bottomed := h.ChooseBottomCards(gs, 0, hand, 2)
	if len(bottomed) != 2 {
		t.Fatalf("expected 2 bottomed cards, got %d", len(bottomed))
	}
	for _, c := range bottomed {
		if c.Name == "Bear" {
			t.Errorf("Bear was the only cheap playable — anchor should have kept it out of the bottom pile; got bottomed=%v",
				bottomNames(bottomed))
		}
	}
}

func bottomNames(cards []*gameengine.Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.Name
	}
	return out
}
