package gameengine

import (
	"math/rand"
	"testing"
)

// ward_seat_scope_r60_test.go — pins the r60 anthem-style ward
// extension (Hexing Squelcher / Indomitable Might-class "creatures
// you control have ward N" continuous effects).
//
// Three required pins per the user spec (2026-05-27):
//   1. Global-anthem ward applies to all of the source's controller's
//      creatures (without the creatures needing a printed ward keyword).
//   2. Per-permanent ward stacks with the seat-aggregate — when the
//      target has BOTH a printed ward and inherits one from a seat-
//      scope effect, both fire as separate payments (CR §702.21e).
//   3. Seat ward leaves with the source — when the source LTBs, the
//      anthem effect ends. Also: control change on the source moves
//      the beneficiary seat (the OLD controller's creatures stop
//      inheriting).

func newSeatScopeGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(91)), nil)
}

// Bare creature on seat — no printed ward, no mana.
func plainCreature(gs *GameState, seat int, name string) *Permanent {
	c := &Card{
		Name:          name,
		Owner:         seat,
		Types:         []string{"creature"},
		BasePower:     2,
		BaseToughness: 2,
	}
	p := &Permanent{Card: c, Controller: seat, Owner: seat}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Source permanent on seat that grants the seat-scope ward.
func anthemSource(gs *GameState, seat int, name string) *Permanent {
	c := &Card{Name: name, Owner: seat, Types: []string{"creature"}}
	p := &Permanent{Card: c, Controller: seat, Owner: seat, Timestamp: 1}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// Mock opponent spell targeting `target`.
func oppSpell(target *Permanent) *StackItem {
	return &StackItem{
		Kind:       "spell",
		Controller: 1, // opponent
		Card:       &Card{Name: "Doom Blade", Owner: 1, Types: []string{"instant"}},
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: target}},
	}
}

// -----------------------------------------------------------------------------
// 1. Global anthem — all creatures of source's controller get ward N
// -----------------------------------------------------------------------------

func TestSeatWard_AnthemAppliesToControlledCreatures(t *testing.T) {
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	creature := plainCreature(gs, 0, "Grizzly Bears")

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 2}, nil)

	// Opponent has 2 mana — should pay.
	gs.Seats[1].ManaPool = 2

	item := oppSpell(creature)
	CheckWardOnTargeting(gs, item)

	if item.Countered {
		t.Fatal("ward should pay — caster has 2 mana, cost 2")
	}
	if gs.Seats[1].ManaPool != 0 {
		t.Errorf("caster mana = %d, want 0 (paid 2)", gs.Seats[1].ManaPool)
	}
}

func TestSeatWard_AnthemCountersWhenCantPay(t *testing.T) {
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	creature := plainCreature(gs, 0, "Grizzly Bears")

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 2}, nil)
	gs.Seats[1].ManaPool = 1 // can't pay 2

	item := oppSpell(creature)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("ward should counter — caster only has 1 mana")
	}
}

func TestSeatWard_AnthemFiresWithoutKeywordOnTarget(t *testing.T) {
	// The target creature has NO printed ward keyword — should still
	// fire because the anthem grants it. This is the load-bearing
	// behavior change: pre-r60 CheckWardOnTargeting required
	// perm.HasKeyword("ward"), which would have skipped this entirely.
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	creature := plainCreature(gs, 0, "Grizzly Bears")
	if creature.HasKeyword("ward") {
		t.Fatal("test fixture invalid: Grizzly Bears must not have printed ward")
	}

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 1}, nil)
	gs.Seats[1].ManaPool = 0

	item := oppSpell(creature)
	CheckWardOnTargeting(gs, item)

	if !item.Countered {
		t.Fatal("seat-scope ward must fire even when target has no printed ward keyword")
	}
	_ = squelcher
}

func TestSeatWard_AnthemFiltersOutOpponentCreatures(t *testing.T) {
	// Source on seat 0; opponent creature targeted by an opponent-
	// originated spell. Anthem must NOT apply to opponent-controlled
	// creatures (creatures of seat 0 only).
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	oppCreature := plainCreature(gs, 1, "Opp Bears")

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 5}, nil)
	gs.Seats[1].ManaPool = 0 // caster=seat1, no mana

	// Caster is seat 1, but target is also controlled by seat 1 — outer
	// CheckWardOnTargeting will skip because perm.Controller == item.Controller.
	// Instead simulate a seat 0 spell targeting opp creature. But ward
	// only triggers when an OPPONENT targets — so seat 0 spell vs seat 1
	// creature triggers anthem-of-seat-0? No: the filter is "Source's
	// controller (0) must equal target's controller (1)". They differ →
	// anthem does NOT apply. Pin this.
	spell := &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       &Card{Name: "Doom Blade", Owner: 0, Types: []string{"instant"}},
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: oppCreature}},
	}
	CheckWardOnTargeting(gs, spell)

	if spell.Countered {
		t.Error("anthem from seat 0 must not apply to seat 1's creature even when targeted by seat 0")
	}
}

// -----------------------------------------------------------------------------
// 2. Stacking — per-permanent ward + seat-aggregate fire as separate triggers
// -----------------------------------------------------------------------------

func TestSeatWard_StacksWithPermanentWard(t *testing.T) {
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")

	// Target has printed ward {3}.
	c := &Card{
		Name:          "Sheoldred",
		Owner:         0,
		Types:         []string{"creature"},
		BasePower:     4,
		BaseToughness: 5,
	}
	target := &Permanent{
		Card:       c,
		Controller: 0,
		Owner:      0,
		Flags: map[string]int{
			"kw:ward":   1,
			"ward_cost": 3,
		},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 2}, nil)

	// Caster needs 3 (printed) + 2 (anthem) = 5 mana. Give 5 → both pay.
	gs.Seats[1].ManaPool = 5
	item := oppSpell(target)
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Fatal("with 5 mana caster should pay both 3 + 2 wards")
	}
	if gs.Seats[1].ManaPool != 0 {
		t.Errorf("mana after both ward payments = %d, want 0", gs.Seats[1].ManaPool)
	}
}

func TestSeatWard_StackingCountersWhenSecondCantPay(t *testing.T) {
	// Printed ward {3} costs 3; anthem ward {2} costs 2. Caster has 3
	// mana — pays printed ward, then the anthem ward can't be paid
	// (out of mana), so the spell is countered.
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")

	c := &Card{Name: "Sheoldred", Owner: 0, Types: []string{"creature"}, BasePower: 4, BaseToughness: 5}
	target := &Permanent{
		Card:       c,
		Controller: 0,
		Owner:      0,
		Flags:      map[string]int{"kw:ward": 1, "ward_cost": 3},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, target)

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 2}, nil)
	gs.Seats[1].ManaPool = 3

	item := oppSpell(target)
	CheckWardOnTargeting(gs, item)
	if !item.Countered {
		t.Error("second ward (anthem) should counter when caster's mana runs out mid-stack")
	}
}

// -----------------------------------------------------------------------------
// 3. Continuous effect leaves with the source — LTB + control change
// -----------------------------------------------------------------------------

func TestSeatWard_SourceLTB_RemovesEffect(t *testing.T) {
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	creature := plainCreature(gs, 0, "Grizzly Bears")

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 3}, nil)
	if len(gs.SeatWardEffects) != 1 {
		t.Fatalf("expected 1 seat ward effect after AddSeatWardCost, got %d", len(gs.SeatWardEffects))
	}

	// Drive LTB via UnregisterContinuousEffectsForPermanent — the
	// canonical entry point that every LTB path funnels through.
	gs.UnregisterContinuousEffectsForPermanent(squelcher)
	if len(gs.SeatWardEffects) != 0 {
		t.Errorf("expected seat ward to be removed on Source LTB, got %d entries", len(gs.SeatWardEffects))
	}

	// Subsequent targeting should NOT fire any ward.
	gs.Seats[1].ManaPool = 0
	item := oppSpell(creature)
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Error("after Source LTB, no ward should fire on the previously-warded creature")
	}
}

func TestSeatWard_ControlChange_MovesBeneficiary(t *testing.T) {
	// Squelcher starts on seat 0 — anthem benefits seat 0's creatures.
	// Then control flips to seat 1 (e.g. via Threaten / Mind Control).
	// Now seat 1's creatures should inherit the ward; seat 0's
	// creatures should no longer inherit.
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hexing Squelcher")
	seat0Creature := plainCreature(gs, 0, "Grizzly Bears")
	seat1Creature := plainCreature(gs, 1, "Memnite")

	AddSeatWardCost(gs, squelcher, WardCost{Type: WardCostMana, Amount: 3}, nil)

	// Flip control of Squelcher to seat 1.
	squelcher.Controller = 1

	// Seat 0's Grizzly Bears, targeted by seat 1, should NOT inherit
	// the anthem anymore (Squelcher's controller is seat 1, doesn't
	// match Grizzly's controller seat 0). The target's controller
	// equality with the SPELL caster also matters — perm.Controller=0
	// vs item.Controller=1 means ward CAN trigger structurally, but
	// the anthem source's controller no longer matches the target's
	// controller, so no inheritance.
	gs.Seats[1].ManaPool = 0
	item := oppSpell(seat0Creature)
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Error("after Squelcher control-flips to seat 1, seat 0's creatures must lose the anthem")
	}

	// Conversely: seat 1's creature targeted by seat 0 — Source.
	// Controller=1 matches target.Controller=1 → seat 1's creatures
	// inherit. Seat 0 spell vs seat 1 creature with caster mana 0 must
	// counter.
	gs.Seats[0].ManaPool = 0
	spell0 := &StackItem{
		Kind:       "spell",
		Controller: 0,
		Card:       &Card{Name: "Doom Blade", Owner: 0, Types: []string{"instant"}},
		Targets:    []Target{{Kind: TargetKindPermanent, Permanent: seat1Creature}},
	}
	CheckWardOnTargeting(gs, spell0)
	if !spell0.Countered {
		t.Error("after control-change, seat 1's creature should inherit the anthem and counter seat 0's spell")
	}
}

func TestSeatWard_FilterFn_Honored(t *testing.T) {
	// Custom filter — only Beasts inherit ward. Pin that FilterFn is
	// consulted by SeatWardCostsFor.
	gs := newSeatScopeGame(t)
	squelcher := anthemSource(gs, 0, "Hypothetical Beast Anthem")
	bear := plainCreature(gs, 0, "Grizzly Bears")
	beast := plainCreature(gs, 0, "Big Beast")
	beast.Card.Types = []string{"creature", "beast"}

	AddSeatWardCost(gs, squelcher,
		WardCost{Type: WardCostMana, Amount: 2},
		func(perm *Permanent) bool {
			return typeContains(perm.Card.Types, "beast")
		},
	)

	// Bear shouldn't inherit (no "beast" type) — opp spell goes through.
	gs.Seats[1].ManaPool = 0
	item := oppSpell(bear)
	CheckWardOnTargeting(gs, item)
	if item.Countered {
		t.Error("Bear must NOT inherit the beast-filtered anthem")
	}

	// Beast must inherit — opp spell counters.
	item2 := oppSpell(beast)
	CheckWardOnTargeting(gs, item2)
	if !item2.Countered {
		t.Error("Beast must inherit the beast-filtered anthem and counter the spell")
	}
}
