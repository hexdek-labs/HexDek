package gameengine

import (
	"testing"
)

// colored_mana_estimate_r60_test.go — R60 color-aware mana tracking.
// Pre-fix the hat saw only the integer AvailableManaEstimate and would
// false-positive a Counterspell {U}{U} against 2 colorless. The new
// AvailableColoredManaEstimate + CanPayColoredCost pair captures the
// per-color picture and supports greedy multi-color allocation.

func newColorTestSeat() *Seat {
	return &Seat{
		Mana:        &ColoredManaPool{},
		Battlefield: []*Permanent{},
	}
}

func addLand(seat *Seat, name string, typeLine string, oracle string) *Permanent {
	c := &Card{
		Name:     name,
		Types:    []string{"land"},
		TypeLine: typeLine,
	}
	if oracle != "" {
		c.OracleTextCache = oracle
		c.oracleTextReady = true
	}
	p := &Permanent{Card: c, Flags: map[string]int{}}
	seat.Battlefield = append(seat.Battlefield, p)
	return p
}

func costCard(name, cost string) *Card {
	return &Card{
		Name:           name,
		ManaCostString: cost,
		Types:          []string{"instant", "cost:" + itoaCMC(cost)},
	}
}

// itoaCMC is a tiny inline CMC parser for the test-fixture cost token.
// Mirrors what the hat tests use; counts every symbol as 1 except for
// pure numerics, which contribute their numeric value.
func itoaCMC(cost string) string {
	cmc := 0
	i := 0
	for i < len(cost) {
		if cost[i] != '{' {
			i++
			continue
		}
		end := -1
		for j := i; j < len(cost); j++ {
			if cost[j] == '}' {
				end = j
				break
			}
		}
		if end < 0 {
			break
		}
		inner := cost[i+1 : end]
		i = end + 1
		// Numeric symbol → add its value; otherwise add 1.
		n, isNum := 0, true
		for _, ch := range inner {
			if ch < '0' || ch > '9' {
				isNum = false
				break
			}
			n = n*10 + int(ch-'0')
		}
		if isNum {
			cmc += n
		} else {
			cmc++
		}
	}
	out := ""
	if cmc == 0 {
		return "0"
	}
	for cmc > 0 {
		out = string(rune('0'+cmc%10)) + out
		cmc /= 10
	}
	return out
}

// -----------------------------------------------------------------------------
// AvailableColoredManaEstimate
// -----------------------------------------------------------------------------

func TestAvailableColoredManaEstimate_PoolContributesToFixed(t *testing.T) {
	seat := newColorTestSeat()
	seat.Mana.U = 2
	seat.Mana.R = 1
	seat.Mana.Any = 1

	est := AvailableColoredManaEstimate(nil, seat)
	if est.Fixed[manaSlotU] != 2 {
		t.Errorf("expected 2 U from pool, got %d", est.Fixed[manaSlotU])
	}
	if est.Fixed[manaSlotR] != 1 {
		t.Errorf("expected 1 R from pool, got %d", est.Fixed[manaSlotR])
	}
	if est.Any != 1 {
		t.Errorf("expected 1 Any from pool, got %d", est.Any)
	}
	if est.Total() != 4 {
		t.Errorf("Total() should be 4, got %d", est.Total())
	}
}

func TestAvailableColoredManaEstimate_MonocolorLandsToFixed(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "Island", "Basic Land — Island", "")
	addLand(seat, "Island #2", "Basic Land — Island", "")
	addLand(seat, "Mountain", "Basic Land — Mountain", "")

	est := AvailableColoredManaEstimate(nil, seat)
	if est.Fixed[manaSlotU] != 2 {
		t.Errorf("expected 2 U fixed, got %d", est.Fixed[manaSlotU])
	}
	if est.Fixed[manaSlotR] != 1 {
		t.Errorf("expected 1 R fixed, got %d", est.Fixed[manaSlotR])
	}
	if len(est.Flex) != 0 {
		t.Errorf("monocolor lands should not appear in Flex, got %d", len(est.Flex))
	}
}

func TestAvailableColoredManaEstimate_TappedLandsExcluded(t *testing.T) {
	seat := newColorTestSeat()
	tapped := addLand(seat, "Island", "Basic Land — Island", "")
	tapped.Tapped = true

	est := AvailableColoredManaEstimate(nil, seat)
	if est.Fixed[manaSlotU] != 0 {
		t.Errorf("tapped Island should not contribute; got %d U", est.Fixed[manaSlotU])
	}
}

func TestAvailableColoredManaEstimate_DualLandToFlex(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "Volcanic Island", "Land — Island Mountain", "")

	est := AvailableColoredManaEstimate(nil, seat)
	if len(est.Flex) != 1 {
		t.Fatalf("dual land should add one Flex entry, got %d", len(est.Flex))
	}
	mask := est.Flex[0]
	if mask&colorBitMaskU == 0 || mask&colorBitMaskR == 0 {
		t.Errorf("Volcanic Island flex mask missing U|R; mask=%08b", mask)
	}
}

func TestAvailableColoredManaEstimate_AnyColorLandToAny(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "City of Brass", "Land", "Tap: Add one mana of any color.")

	est := AvailableColoredManaEstimate(nil, seat)
	if est.Any != 1 {
		t.Errorf("any-color land should fold into Any, got %d", est.Any)
	}
	if len(est.Flex) != 0 {
		t.Errorf("any-color land should NOT be Flex (already all-colors); got %d Flex", len(est.Flex))
	}
}

// -----------------------------------------------------------------------------
// CanPayColoredCost
// -----------------------------------------------------------------------------

func TestCanPayColoredCost_DoubleBlueFailsOnGenericPool(t *testing.T) {
	seat := newColorTestSeat()
	// 2 colorless from a Wastes-like setup.
	addLand(seat, "Wastes A", "Basic Land — Wastes", "Tap: Add {C}.")
	addLand(seat, "Wastes B", "Basic Land — Wastes", "Tap: Add {C}.")

	counter := costCard("Counterspell", "{U}{U}")
	est := AvailableColoredManaEstimate(nil, seat)
	if CanPayColoredCost(est, counter) {
		t.Fatal("Counterspell {U}{U} must NOT be payable from 2 colorless")
	}
}

func TestCanPayColoredCost_DoubleBlueSucceedsOnTwoIslands(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "Island", "Basic Land — Island", "")
	addLand(seat, "Island #2", "Basic Land — Island", "")

	counter := costCard("Counterspell", "{U}{U}")
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, counter) {
		t.Fatal("Counterspell {U}{U} should be payable from 2 Islands")
	}
}

func TestCanPayColoredCost_DualLandsCoverBothColors(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "Volcanic Island", "Land — Island Mountain", "")
	addLand(seat, "Volcanic Island #2", "Land — Island Mountain", "")

	// {U}{R} cost; both lands can produce either.
	izzet := costCard("Izzet Charm", "{U}{R}")
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, izzet) {
		t.Fatal("{U}{R} should be payable from two U/R dual lands")
	}
}

func TestCanPayColoredCost_GreedyChoosesConstrainedFirst(t *testing.T) {
	// Triple cost {W}{U}{B} with a mix: 1 mono-W, 1 dual U/B.
	// Greedy must consume the mono-W on white (only it can pay), then
	// use the dual on either U or B, leaving the OTHER color short — so
	// the cost is NOT payable. Verifies the constrained-first allocation
	// doesn't accidentally waste the dual on W.
	seat := newColorTestSeat()
	addLand(seat, "Plains", "Basic Land — Plains", "")
	addLand(seat, "Watery Grave", "Land — Island Swamp", "")

	bolas := costCard("Esper Bolas", "{W}{U}{B}")
	est := AvailableColoredManaEstimate(nil, seat)
	if CanPayColoredCost(est, bolas) {
		t.Fatal("{W}{U}{B} cost should NOT be payable with only Plains + one U/B dual")
	}

	// Add a second Plains — still short because no second U or B source.
	addLand(seat, "Plains #2", "Basic Land — Plains", "")
	est = AvailableColoredManaEstimate(nil, seat)
	if CanPayColoredCost(est, bolas) {
		t.Fatal("two Plains + one U/B dual still cannot pay {W}{U}{B}")
	}

	// Add an Island — now we have W (Plains), U (Island), B (dual).
	addLand(seat, "Island", "Basic Land — Island", "")
	est = AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, bolas) {
		t.Fatal("Plains + Island + U/B dual should pay {W}{U}{B}")
	}
}

func TestCanPayColoredCost_GenericAbsorbsExcess(t *testing.T) {
	seat := newColorTestSeat()
	// {2}{U}{U} with 4 Islands — 2 pay UU, 2 pay generic.
	for i := 0; i < 4; i++ {
		addLand(seat, "Island", "Basic Land — Island", "")
	}
	cancel := costCard("Cancel", "{1}{U}{U}")
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, cancel) {
		t.Fatal("4 Islands should pay {1}{U}{U}")
	}
}

func TestCanPayColoredCost_HybridPipPermissive(t *testing.T) {
	seat := newColorTestSeat()
	addLand(seat, "Mountain", "Basic Land — Mountain", "")
	addLand(seat, "Mountain #2", "Basic Land — Mountain", "")

	// Boros Charm cost is {R}{W}, but Char (the hybrid example used
	// here) — actually let's use {R/W}{R/W} which is paid by 2 R.
	charm := costCard("HybridCharm", "{R/W}{R/W}")
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, charm) {
		t.Fatal("{R/W}{R/W} should be payable from 2 Mountains via hybrid")
	}
}

func TestCanPayColoredCost_EmptyCostFallsBackToTotal(t *testing.T) {
	// Token-style card: no ManaCostString, no cost:N tag — CMC is 0,
	// so any non-negative seat capacity must pass.
	token := &Card{Name: "Token", Types: []string{"creature"}}
	seat := newColorTestSeat()
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, token) {
		t.Fatal("zero-cost token should always be payable")
	}
}

func TestCanPayColoredCost_NilCardIsPayable(t *testing.T) {
	seat := newColorTestSeat()
	est := AvailableColoredManaEstimate(nil, seat)
	if !CanPayColoredCost(est, nil) {
		t.Fatal("nil card guard should return true")
	}
}

// -----------------------------------------------------------------------------
// parseColoredPips
// -----------------------------------------------------------------------------

func TestParseColoredPips_Counts(t *testing.T) {
	cases := []struct {
		cost string
		want [6]int // W U B R G C
	}{
		{"{U}{U}", [6]int{0, 2, 0, 0, 0, 0}},
		{"{1}{W}{U}{B}{R}{G}", [6]int{1, 1, 1, 1, 1, 0}},
		{"{X}{R}", [6]int{0, 0, 0, 1, 0, 0}},
		{"{2}{C}{C}", [6]int{0, 0, 0, 0, 0, 2}},
		{"{R/W}", [6]int{1, 0, 0, 1, 0, 0}},
		{"{2/U}", [6]int{0, 1, 0, 0, 0, 0}},
		{"{U/P}", [6]int{0, 1, 0, 0, 0, 0}},
		{"", [6]int{}},
	}
	for _, c := range cases {
		got := parseColoredPips(c.cost)
		if got != c.want {
			t.Errorf("parseColoredPips(%q) = %v, want %v", c.cost, got, c.want)
		}
	}
}
