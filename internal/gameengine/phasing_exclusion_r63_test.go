package gameengine

// CR §702.26a regressions: a phased-out permanent is treated as though it does
// not exist — it can't be targeted, its static abilities don't function, and
// its triggered abilities can't trigger. r63 fixes:
//   - matchesPermanent excludes phased-out (target enumeration + mass effects)
//   - isTargetStillLegal fizzles a spell whose target phased out
//   - applyLayer skips continuous effects from a phased-out source
//   - FirePhaseTriggers skips phased-out permanents' phase triggers

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// (a) targeting — phased-out permanent is not offered as a legal target.
func TestPhasing_PhasedOutNotTargetable(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")

	f := gameast.Filter{Base: "creature", Targeted: true}
	if got := legalSinglePermanentTargets(gs, f, 0, nil); len(got) != 1 {
		t.Fatalf("phased-in creature should be targetable: got %d targets", len(got))
	}
	PhaseOut(gs, bear)
	if got := legalSinglePermanentTargets(gs, f, 0, nil); len(got) != 0 {
		t.Fatalf("phased-out creature must not be a legal target (CR §702.26a): got %d", len(got))
	}
}

// (a) targeting — a spell whose target phases out before resolution fizzles
// (the target is illegal at resolution).
func TestPhasing_TargetPhasedOutIsIllegalAtResolution(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 1, "Grizzly Bears", 2, 2, "creature")
	tgt := Target{Kind: TargetKindPermanent, Permanent: bear, Seat: 1}

	if !isTargetStillLegal(gs, nil, tgt) {
		t.Fatal("phased-in creature should be a legal target")
	}
	PhaseOut(gs, bear)
	if isTargetStillLegal(gs, nil, tgt) {
		t.Fatal("phased-out creature must be an illegal target at resolution (CR §702.26a)")
	}
}

// (a)/(f) static ability — a continuous effect from a phased-out source does
// not function while phased out, and resumes when it phases back in.
func TestPhasing_StaticAbilityDormantWhilePhasedOut(t *testing.T) {
	gs := newFixtureGame(t)
	anthem := addBattlefield(gs, 0, "Glorious Anthem", 0, 0, "enchantment")
	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	// Anthem: creatures you control get +1/+1 (Layer 7c, sourced from anthem).
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer:      LayerPT,
		Sublayer:   "c",
		Timestamp:  1,
		SourcePerm: anthem,
		HandlerID:  "test_anthem",
		Predicate: func(_ *GameState, p *Permanent) bool {
			return p != nil && p.Controller == 0 && p.IsCreature()
		},
		ApplyFn: func(_ *GameState, _ *Permanent, chars *Characteristics) {
			chars.Power++
			chars.Toughness++
		},
	})
	gs.InvalidateCharacteristicsCache()

	if c := GetEffectiveCharacteristics(gs, bear); c.Power != 3 || c.Toughness != 3 {
		t.Fatalf("anthem should buff bear to 3/3, got %d/%d", c.Power, c.Toughness)
	}

	PhaseOut(gs, anthem)
	gs.InvalidateCharacteristicsCache()
	if c := GetEffectiveCharacteristics(gs, bear); c.Power != 2 || c.Toughness != 2 {
		t.Fatalf("phased-out anthem must not buff (CR §702.26a), expected 2/2 got %d/%d", c.Power, c.Toughness)
	}

	PhaseIn(gs, anthem)
	gs.InvalidateCharacteristicsCache()
	if c := GetEffectiveCharacteristics(gs, bear); c.Power != 3 || c.Toughness != 3 {
		t.Fatalf("phased-in anthem must resume buffing, expected 3/3 got %d/%d", c.Power, c.Toughness)
	}
}

// (a) triggered ability — a phased-out permanent's "at the beginning of your
// upkeep" trigger does not fire.
func TestPhasing_TriggeredAbilityDoesNotFireWhilePhasedOut(t *testing.T) {
	gs := newFixtureGame(t)
	p := eachUpkeepBearer(gs, 0, "at the beginning of your upkeep, you gain 1 life")
	gs.Active = 0

	PhaseOut(gs, p)
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 20 {
		t.Fatalf("phased-out permanent's upkeep trigger must not fire (CR §702.26a): life=%d", gs.Seats[0].Life)
	}

	// Sanity: it fires again once phased in.
	PhaseIn(gs, p)
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 21 {
		t.Fatalf("phased-in permanent's upkeep trigger should fire: life=%d", gs.Seats[0].Life)
	}
}
