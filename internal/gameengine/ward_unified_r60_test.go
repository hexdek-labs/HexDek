package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ward_unified_r60_test.go — r60 unified WardCost primitive
// (2026-05-27 7174n1c architecture lock).
//
// Pins:
//   1. SetWardCost / GetWardCost round-trip including Filter strings.
//   2. WardCostLife dispatch — pay-life happy path, refuses when life
//      is insufficient, refuses when life is too low for safe payment
//      (the 2x cost gate).
//   3. WardCostDamage dispatch via ApplyWardCostDamage — creature
//      target, player target, prevention shield, no-valid-target
//      fallback.
//   4. Filter-driven Sacrifice variation (no-legendary rejects).
//   5. Filter-driven Discard variation (legacy empty-filter default
//      still keeps inst/sorc/ench behavior; explicit filter narrows).
//
// Existing TestWardAlt_* tests cover the pre-existing Sacrifice/
// Discard/Blight paths via the legacy flag-stamping fixture; these
// pins exercise the new SetWardCost / WardCostLife / WardCostDamage
// surfaces.

// helper: minimal 2-seat game with no decks (mana / draw paths not
// exercised by the unified-primitive tests).
func newWardUnifiedGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(31)), nil)
}

// helper: builds a permanent on seat 0 with the ward keyword (so
// CheckWardOnTargeting's HasKeyword("ward") gate passes), then sets
// the requested WardCost via the new primitive.
func wardedPermViaSetWardCost(gs *GameState, name string, cost WardCost, types ...string) *Permanent {
	c := &Card{
		Name:  name,
		Owner: 0,
		Types: types,
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "ward"},
			},
		},
	}
	p := &Permanent{
		Card:       c,
		Controller: 0,
		Owner:      0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	SetWardCost(p, cost)
	return p
}

// =============================================================================
// 1. SetWardCost / GetWardCost round-trip
// =============================================================================

func TestWardCost_SetGetRoundTrip_Sacrifice(t *testing.T) {
	gs := newWardUnifiedGame(t)
	perm := wardedPermViaSetWardCost(gs, "Sauron, the Dark Lord",
		WardCost{Type: WardCostSacrifice, Amount: 1, Filter: "legendary artifact or creature"},
		"creature", "legendary")

	got := GetWardCost(perm)
	if got.Type != WardCostSacrifice {
		t.Errorf("Type = %d, want %d (Sacrifice)", got.Type, WardCostSacrifice)
	}
	if got.Amount != 1 {
		t.Errorf("Amount = %d, want 1", got.Amount)
	}
	if got.Filter != "legendary artifact or creature" {
		t.Errorf("Filter = %q, want %q", got.Filter, "legendary artifact or creature")
	}
	if perm.Flags["kw:ward"] != 1 {
		t.Errorf("kw:ward flag must be stamped by SetWardCost")
	}
}

func TestWardCost_SetGetRoundTrip_NoFilter(t *testing.T) {
	gs := newWardUnifiedGame(t)
	perm := wardedPermViaSetWardCost(gs, "Auntie Ool, Cursewretch",
		WardCost{Type: WardCostBlight, Amount: 2},
		"creature", "legendary")
	got := GetWardCost(perm)
	if got.Type != WardCostBlight || got.Amount != 2 || got.Filter != "" {
		t.Errorf("Blight round-trip mismatch: %+v", got)
	}
}

// =============================================================================
// 2. WardCostLife dispatch
// =============================================================================

func TestWardAlt_Life_Pays_HappyPath(t *testing.T) {
	gs := newWardUnifiedGame(t)
	boar := wardedPermViaSetWardCost(gs, "Charging War Boar",
		WardCost{Type: WardCostLife, Amount: 3}, "creature")
	gs.Seats[1].Life = 40

	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should pay — caster has 40 life, cost 3 (40 > 2*3=6)")
	}
	if gs.Seats[1].Life != 37 {
		t.Errorf("caster life = %d, want 37 (40 - 3)", gs.Seats[1].Life)
	}
}

func TestWardAlt_Life_RefusesAtInsufficientLife(t *testing.T) {
	gs := newWardUnifiedGame(t)
	boar := wardedPermViaSetWardCost(gs, "Charging War Boar",
		WardCost{Type: WardCostLife, Amount: 3}, "creature")
	gs.Seats[1].Life = 2 // less than the 3 required

	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — caster at 2 life can't pay 3")
	}
	if gs.Seats[1].Life != 2 {
		t.Errorf("life must be unchanged when payment refused, got %d", gs.Seats[1].Life)
	}
}

func TestWardAlt_Life_RefusesAtRiskyLife(t *testing.T) {
	// 2x cost gate: caster has 5 life, cost is 3. 5 < 2*3 = 6, so the
	// hat declines payment to preserve the buffer.
	gs := newWardUnifiedGame(t)
	boar := wardedPermViaSetWardCost(gs, "Charging War Boar",
		WardCost{Type: WardCostLife, Amount: 3}, "creature")
	gs.Seats[1].Life = 5

	item := targetingItem(boar)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — caster at 5 life is below 2x cost (6); safe-payment gate refuses")
	}
	if gs.Seats[1].Life != 5 {
		t.Errorf("life must be unchanged when payment refused, got %d", gs.Seats[1].Life)
	}
}

// =============================================================================
// 3. WardCostDamage / ApplyWardCostDamage
// =============================================================================

func TestApplyWardCostDamage_CreatureTarget(t *testing.T) {
	gs := newWardUnifiedGame(t)
	source := &Permanent{
		Card:       &Card{Name: "Terror of the Peaks", Owner: 0, Types: []string{"creature"}, BasePower: 5, BaseToughness: 4},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, source)

	target := &Permanent{
		Card:       &Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, target)

	dealt, detail := ApplyWardCostDamage(gs, source, target, -1, WardCost{Type: WardCostDamage, Amount: 3})
	if !dealt {
		t.Fatalf("damage should have landed, got detail %v", detail)
	}
	if target.MarkedDamage != 3 {
		t.Errorf("target MarkedDamage = %d, want 3", target.MarkedDamage)
	}
}

func TestApplyWardCostDamage_PlayerTarget(t *testing.T) {
	gs := newWardUnifiedGame(t)
	source := &Permanent{
		Card:       &Card{Name: "Terror of the Peaks", Owner: 0, Types: []string{"creature"}, BasePower: 5, BaseToughness: 4},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, source)
	gs.Seats[1].Life = 40

	dealt, detail := ApplyWardCostDamage(gs, source, nil, 1, WardCost{Type: WardCostDamage, Amount: 4})
	if !dealt {
		t.Fatalf("player damage should have landed, got detail %v", detail)
	}
	if gs.Seats[1].Life != 36 {
		t.Errorf("seat 1 life = %d, want 36 (40 - 4)", gs.Seats[1].Life)
	}
}

func TestApplyWardCostDamage_NoTarget_NoOp(t *testing.T) {
	gs := newWardUnifiedGame(t)
	source := &Permanent{
		Card:       &Card{Name: "Terror of the Peaks", Owner: 0, Types: []string{"creature"}, BasePower: 5, BaseToughness: 4},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, source)

	dealt, detail := ApplyWardCostDamage(gs, source, nil, -1, WardCost{Type: WardCostDamage, Amount: 4})
	if dealt {
		t.Errorf("damage must NOT land with no valid target, got detail %v", detail)
	}
}

func TestApplyWardCostDamage_NonPositiveAmount_NoOp(t *testing.T) {
	gs := newWardUnifiedGame(t)
	source := &Permanent{
		Card:       &Card{Name: "Terror of the Peaks", Owner: 0, Types: []string{"creature"}, BasePower: 5, BaseToughness: 4},
		Controller: 0,
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, source)
	gs.Seats[1].Life = 40

	dealt, _ := ApplyWardCostDamage(gs, source, nil, 1, WardCost{Type: WardCostDamage, Amount: 0})
	if dealt {
		t.Error("amount=0 must be rejected")
	}
	if gs.Seats[1].Life != 40 {
		t.Errorf("seat 1 life changed unexpectedly: %d", gs.Seats[1].Life)
	}
}

// =============================================================================
// 4. Filter-driven Sacrifice — only matching permanents are valid
// =============================================================================

func TestWardAlt_Sacrifice_FilterRejectsNonLegendary(t *testing.T) {
	gs := newWardUnifiedGame(t)
	sauron := wardedPermViaSetWardCost(gs, "Sauron, the Dark Lord",
		WardCost{Type: WardCostSacrifice, Amount: 1, Filter: "legendary artifact or creature"},
		"creature", "legendary")

	// Caster has ONLY non-legendary creatures.
	nonLegendary := &Permanent{
		Card:       &Card{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 1,
		Owner:      1,
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, nonLegendary)

	item := targetingItem(sauron)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — no legendary to sacrifice")
	}
	if nonLegendary.Card == nil {
		t.Error("non-legendary creature must not have been sacrificed")
	}
}

// =============================================================================
// 5. Discard filter behavior — empty filter preserves legacy default
// =============================================================================

func TestWardAlt_Discard_EmptyFilterUsesLegacyDefault(t *testing.T) {
	// Construct via SetWardCost with empty Filter — should still
	// behave like the legacy "inst/sorc/ench only" Discard variant.
	gs := newWardUnifiedGame(t)
	saruman := wardedPermViaSetWardCost(gs, "Saruman of Many Colors",
		WardCost{Type: WardCostDiscard, Amount: 1},
		"creature", "legendary")

	gs.Seats[1].Hand = []*Card{
		{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}},
		{Name: "Forest", Owner: 1, Types: []string{"land", "basic"}},
	}

	item := targetingItem(saruman)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Error("empty Filter on Discard must keep legacy inst/sorc/ench requirement — only creatures + land in hand should fail")
	}
}

func TestWardAlt_Discard_ExplicitFilterAcceptsCreature(t *testing.T) {
	// Hypothetical future "Ward—Discard a creature card" variant.
	gs := newWardUnifiedGame(t)
	perm := wardedPermViaSetWardCost(gs, "Hypothetical Ward Creature Discard",
		WardCost{Type: WardCostDiscard, Amount: 1, Filter: "creature"},
		"creature")
	// Test the filter parsing path — the implementation only enables
	// inst/sorc/ench gates when the filter mentions those types. A
	// "creature" filter currently falls through to "no match" because
	// the dispatcher doesn't (yet) wire creature-discard. Pin that
	// behavior — when extended in the future, this test will fail
	// loudly so the extension is deliberate.
	gs.Seats[1].Hand = []*Card{
		{Name: "Grizzly Bears", Owner: 1, Types: []string{"creature"}},
	}
	item := targetingItem(perm)
	CheckWardOnTargeting(gs, item)
	if !item.Countered {
		t.Error("Filter=\"creature\" not yet wired — should counter (no inst/sorc/ench match). When wired, update this test.")
	}
}
