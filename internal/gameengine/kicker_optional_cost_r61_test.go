package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// -----------------------------------------------------------------------------
// r61 PR-4 — cast-time optional-cost framework (kicker / multikicker)
//
// Pins the full plumbing: Hat hook → CostMeta stamp → perm.Flags mirror →
// "if kicked" Static ETB counters (Grunn) + "for each time kicked" variable
// counters (Everflowing Chalice). Plus the hat-affordability guard.
// -----------------------------------------------------------------------------

// kickHat is a test Hat that always pays the maximum affordable kicks.
type kickHat struct{ GreedyHatStub }

func (*kickHat) ChooseKickCount(gs *GameState, seatIdx int, card *Card, kickerCost, maxKicks int) int {
	return maxKicks
}

// noKickHat never kicks (mirrors the conservative default).
type noKickHat struct{ GreedyHatStub }

func (*noKickHat) ChooseKickCount(gs *GameState, seatIdx int, card *Card, kickerCost, maxKicks int) int {
	return 0
}

// makeGrunnCard builds a Grunn-shaped card: single Kicker {3} + a
// conditional Static "if kicked, enters with five +1/+1 counters". Base
// CMC encoded via the cost:N test convention.
func makeGrunnCard(seat int) *Card {
	return &Card{
		Name:          "Grunn, the Lonely King",
		Owner:         seat,
		Types:         []string{"creature", "cost:6"},
		BasePower:     5,
		BaseToughness: 5,
		AST: &gameast.CardAST{
			Name: "Grunn, the Lonely King",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "kicker", Args: []interface{}{float64(3)}},
				&gameast.Static{
					Condition: &gameast.Condition{Kind: "kicked", Args: []interface{}{1}},
					Modification: &gameast.Modification{
						ModKind: "etb_with_counters",
						Args:    []interface{}{float64(5), "+1/+1"},
					},
				},
			},
		},
	}
}

// makeChaliceCard builds an Everflowing Chalice-shaped card: Multikicker {2}
// + a Static "enters with a charge counter for each time it was kicked"
// (variable count encoded as Args[0]=="var").
func makeChaliceCard(seat int) *Card {
	return &Card{
		Name:  "Everflowing Chalice",
		Owner: seat,
		Types: []string{"artifact", "cost:0"},
		AST: &gameast.CardAST{
			Name: "Everflowing Chalice",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "multikicker", Args: []interface{}{float64(2)}},
				&gameast.Static{
					Modification: &gameast.Modification{
						ModKind: "etb_with_counters",
						Args:    []interface{}{"var", "charge", "for_each:time it was kicked"},
					},
				},
			},
		},
	}
}

func castOnlyPerm(gs *GameState, seat int) *Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil {
			return p
		}
	}
	return nil
}

// (a) KICKED Grunn enters as a 10/10 with 5 +1/+1 counters.
func TestGrunn_Kicked_EntersWithFiveCounters(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &kickHat{}
	gs.Seats[0].ManaPool = 9 // 6 base + 3 kicker
	card := makeGrunnCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Grunn not on battlefield")
	}
	if perm.Flags["kicked"] < 1 {
		t.Errorf("expected kicked flag set, got %d", perm.Flags["kicked"])
	}
	if perm.Counters["+1/+1"] != 5 {
		t.Errorf("kicked Grunn should have 5 +1/+1 counters, got %d", perm.Counters["+1/+1"])
	}
	// 5 base + 5 counters = 10/10.
	if perm.Power() != 10 || perm.Toughness() != 10 {
		t.Errorf("kicked Grunn should be 10/10, got %d/%d", perm.Power(), perm.Toughness())
	}
}

// (a) UNKICKED Grunn is a 5/5 with 0 counters.
func TestGrunn_Unkicked_NoCounters(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &noKickHat{}
	gs.Seats[0].ManaPool = 9 // could afford the kick, but the hat declines
	card := makeGrunnCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Grunn not on battlefield")
	}
	if perm.Flags["kicked"] != 0 {
		t.Errorf("unkicked Grunn should have kicked=0, got %d", perm.Flags["kicked"])
	}
	if perm.Counters["+1/+1"] != 0 {
		t.Errorf("unkicked Grunn should have 0 counters, got %d", perm.Counters["+1/+1"])
	}
	if perm.Power() != 5 || perm.Toughness() != 5 {
		t.Errorf("unkicked Grunn should be 5/5, got %d/%d", perm.Power(), perm.Toughness())
	}
}

// (b) Everflowing Chalice multikicked N times enters with N charge counters.
func TestEverflowingChalice_Multikicked_NChargeCounters(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &kickHat{} // max kicks
	gs.Seats[0].ManaPool = 6     // 0 base + 6 mana = 3 kicks at {2} each
	card := makeChaliceCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Chalice not on battlefield")
	}
	if perm.Flags["multikick_count"] != 3 {
		t.Errorf("expected multikick_count 3, got %d", perm.Flags["multikick_count"])
	}
	if perm.Counters["charge"] != 3 {
		t.Errorf("3x-kicked Chalice should have 3 charge counters, got %d", perm.Counters["charge"])
	}
}

// (b) Everflowing Chalice cast for {0} (0 kicks) enters with 0 counters.
func TestEverflowingChalice_ZeroKick_NoCounters(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &noKickHat{} // 0 kicks
	gs.Seats[0].ManaPool = 6       // mana available, but hat declines
	card := makeChaliceCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Chalice not on battlefield")
	}
	if perm.Counters["charge"] != 0 {
		t.Errorf("0-kick Chalice should have 0 charge counters, got %d", perm.Counters["charge"])
	}
	if perm.Flags["multikick_count"] != 0 {
		t.Errorf("0-kick Chalice should have multikick_count 0, got %d", perm.Flags["multikick_count"])
	}
}

// (c) Affordability guard — with only 1 mana the max-kick hat cannot pay the
// {2} kicker, so Chalice enters 0-kick even though the hat "wants" to kick.
func TestEverflowingChalice_CannotAffordKick_StaysZero(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &kickHat{} // wants max kicks
	gs.Seats[0].ManaPool = 1     // 0 base, only 1 left — can't afford {2} kicker
	card := makeChaliceCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Chalice not on battlefield")
	}
	if perm.Counters["charge"] != 0 {
		t.Errorf("unaffordable-kick Chalice should have 0 counters, got %d", perm.Counters["charge"])
	}
	// Mana pool untouched by an unaffordable kick (base was {0}).
	if EnsureTypedPool(gs.Seats[0]).Total() != 1 {
		t.Errorf("expected 1 mana left (no kick charged), got %d", EnsureTypedPool(gs.Seats[0]).Total())
	}
}

// Single-kicker affordability: 6 base + only 2 leftover < {3} kicker → unkicked.
func TestGrunn_CannotAffordKick_StaysUnkicked(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Hat = &kickHat{} // wants the kick
	gs.Seats[0].ManaPool = 8     // 6 base + 2 left, kicker is {3} → can't afford
	card := makeGrunnCard(0)
	gs.Seats[0].Hand = []*Card{card}

	if err := CastSpell(gs, 0, card, nil); err != nil {
		t.Fatalf("CastSpell failed: %v", err)
	}
	perm := castOnlyPerm(gs, 0)
	if perm == nil {
		t.Fatalf("Grunn not on battlefield")
	}
	if perm.Flags["kicked"] != 0 {
		t.Errorf("Grunn should be unkicked when kicker unaffordable, got kicked=%d", perm.Flags["kicked"])
	}
	if perm.Counters["+1/+1"] != 0 {
		t.Errorf("unaffordable-kick Grunn should have 0 counters, got %d", perm.Counters["+1/+1"])
	}
}

// Helper-level pins: KickerCost / HasKickerKeyword / IsMultikicker.
func TestKickerHelpers_DetectAndCost(t *testing.T) {
	grunn := makeGrunnCard(0)
	chalice := makeChaliceCard(0)

	if !HasKickerKeyword(grunn) || !HasKickerKeyword(chalice) {
		t.Errorf("both cards should report HasKickerKeyword")
	}
	if IsMultikicker(grunn) {
		t.Errorf("Grunn is a single kicker, not multikicker")
	}
	if !IsMultikicker(chalice) {
		t.Errorf("Chalice should report multikicker")
	}
	if KickerCost(grunn) != 3 {
		t.Errorf("Grunn kicker cost should be 3, got %d", KickerCost(grunn))
	}
	if KickerCost(chalice) != 2 {
		t.Errorf("Chalice multikicker cost should be 2, got %d", KickerCost(chalice))
	}
	// A vanilla card with no kicker keyword is fully inert.
	vanilla := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}}
	if HasKickerKeyword(vanilla) || KickerCost(vanilla) != 0 {
		t.Errorf("vanilla card should have no kicker")
	}
}
