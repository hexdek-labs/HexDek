package gameengine

import (
	"math/rand"
	"testing"
)

// =============================================================================
// Wave 4 — Phasing tests (CR §702.26).
// =============================================================================

func TestPhaseOut_SetsFlagAndLogs(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Teferi's Stalwart", 3, 3, "creature")
	PhaseOut(gs, p)
	if !p.PhasedOut {
		t.Fatal("PhasedOut should be true after PhaseOut")
	}
	if countEvents(gs, "phase_out") == 0 {
		t.Fatal("missing phase_out event")
	}
}

func TestPhaseOut_IndirectPhasesAttachments(t *testing.T) {
	gs := newFixtureGame(t)
	creature := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	sword := addBattlefield(gs, 0, "Bonesplitter", 0, 0, "artifact", "equipment")
	sword.AttachedTo = creature
	PhaseOut(gs, creature)
	if !creature.PhasedOut {
		t.Fatal("creature should be phased out")
	}
	if !sword.PhasedOut {
		t.Fatal("attached equipment should be indirectly phased out (CR §702.26d)")
	}
}

func TestPhaseIn_ClearsFlag(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	p.PhasedOut = true
	PhaseIn(gs, p)
	if p.PhasedOut {
		t.Fatal("PhasedOut should be false after PhaseIn")
	}
	if countEvents(gs, "phase_in") == 0 {
		t.Fatal("missing phase_in event")
	}
}

func TestPhaseIn_IndirectPhasesAttachments(t *testing.T) {
	gs := newFixtureGame(t)
	creature := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	aura := addBattlefield(gs, 0, "Ethereal Armor", 0, 0, "enchantment", "aura")
	aura.AttachedTo = creature
	creature.PhasedOut = true
	aura.PhasedOut = true
	PhaseIn(gs, creature)
	if creature.PhasedOut {
		t.Fatal("creature should be phased in")
	}
	if aura.PhasedOut {
		t.Fatal("attached aura should be indirectly phased in (CR §702.26d)")
	}
}

func TestPhaseInAll_AtUntapStep(t *testing.T) {
	gs := newFixtureGame(t)
	p1 := addBattlefield(gs, 0, "Creature A", 2, 2, "creature")
	p2 := addBattlefield(gs, 0, "Creature B", 3, 3, "creature")
	p1.PhasedOut = true
	p2.PhasedOut = true
	PhaseInAll(gs, 0)
	if p1.PhasedOut || p2.PhasedOut {
		t.Fatal("all seat 0 permanents should be phased in")
	}
}

func TestPhaseInAll_DoesNotAffectOtherSeat(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 1, "Opponent Creature", 4, 4, "creature")
	p.PhasedOut = true
	PhaseInAll(gs, 0) // only phase in seat 0
	if !p.PhasedOut {
		t.Fatal("seat 1's permanent should still be phased out")
	}
}

func TestUntapAll_PhasesInBeforeUntapping(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Tapped Creature", 2, 2, "creature")
	p.Tapped = true
	p.PhasedOut = true
	UntapAll(gs, 0)
	if p.PhasedOut {
		t.Fatal("creature should phase in during untap step")
	}
	if p.Tapped {
		t.Fatal("creature should be untapped after phasing in")
	}
}

func TestPhaseOut_Idempotent(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	PhaseOut(gs, p)
	PhaseOut(gs, p) // second call should be no-op
	if countEvents(gs, "phase_out") != 1 {
		t.Fatalf("expected 1 phase_out event, got %d", countEvents(gs, "phase_out"))
	}
}

func TestIsEffectivelyOnBattlefield_RespectsPhasing(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	if !IsEffectivelyOnBattlefield(gs, p) {
		t.Fatal("non-phased permanent should be effectively on battlefield")
	}
	p.PhasedOut = true
	if IsEffectivelyOnBattlefield(gs, p) {
		t.Fatal("phased-out permanent should NOT be effectively on battlefield")
	}
}

func TestCanAttack_PhasedOutCannotAttack(t *testing.T) {
	gs := newFixtureGame(t)
	_ = gs
	p := &Permanent{
		Card:       &Card{Name: "Bear", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 0,
		PhasedOut:  true,
	}
	if canAttack(p) {
		t.Fatal("phased-out creature should not be able to attack")
	}
}

func TestCanBlock_PhasedOutCannotBlock(t *testing.T) {
	attacker := &Permanent{
		Card:       &Card{Name: "Attacker", Types: []string{"creature"}, BasePower: 3, BaseToughness: 3},
		Controller: 0,
	}
	blocker := &Permanent{
		Card:       &Card{Name: "Blocker", Types: []string{"creature"}, BasePower: 2, BaseToughness: 2},
		Controller: 1,
		PhasedOut:  true,
	}
	if canBlock(attacker, blocker) {
		t.Fatal("phased-out creature should not be able to block")
	}
}

// =============================================================================
// Wave 4 — CleanupHandSize basic test.
// =============================================================================

func TestCleanupHandSize_DiscardsExcess(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)
	for i := 0; i < 10; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, &Card{
			Name:  "Card",
			Owner: 0,
			CMC:   i,
		})
	}
	CleanupHandSize(gs, 0, 7)
	if len(gs.Seats[0].Hand) != 7 {
		t.Fatalf("expected 7 cards after cleanup, got %d", len(gs.Seats[0].Hand))
	}
}

// =============================================================================
// Wave 5 — Stun counters, DoesNotUntap, SkipUntapStep (nightmare fixes).
// =============================================================================

func TestUntapAll_StunCounterPreventsUntap(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Stunned Bear", 2, 2, "creature")
	p.Tapped = true
	if p.Counters == nil {
		p.Counters = map[string]int{}
	}
	p.Counters["stun"] = 1

	UntapAll(gs, 0)

	// Creature should still be tapped — stun counter removed instead.
	if !p.Tapped {
		t.Fatal("creature with stun counter should remain tapped")
	}
	if p.Counters["stun"] != 0 {
		t.Fatalf("stun counter should be removed, got %d", p.Counters["stun"])
	}
	if countEvents(gs, "stun_counter_removed") == 0 {
		t.Fatal("missing stun_counter_removed event")
	}
}

func TestUntapAll_StunCounterFlagFallback(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Stunned Elf", 1, 1, "creature")
	p.Tapped = true
	p.Flags["stun"] = 1

	UntapAll(gs, 0)

	if !p.Tapped {
		t.Fatal("creature with stun flag should remain tapped")
	}
	if p.Flags["stun"] != 0 {
		t.Fatalf("stun flag should be removed, got %d", p.Flags["stun"])
	}
}

func TestUntapAll_DoesNotUntapFlag(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Mana Vault", 0, 0, "artifact")
	p.Tapped = true
	p.DoesNotUntap = true

	UntapAll(gs, 0)

	if !p.Tapped {
		t.Fatal("permanent with DoesNotUntap should remain tapped")
	}
	if countEvents(gs, "untap_skipped") == 0 {
		t.Fatal("missing untap_skipped event")
	}
}

func TestUntapAll_SkipUntapStep(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Normal Bear", 2, 2, "creature")
	p.Tapped = true
	gs.Seats[0].SkipUntapStep = true

	UntapAll(gs, 0)

	if !p.Tapped {
		t.Fatal("creature should remain tapped when untap step is skipped")
	}
	if countEvents(gs, "untap_step_skipped") == 0 {
		t.Fatal("missing untap_step_skipped event")
	}
	// Summoning sickness should still be cleared.
	if p.SummoningSick {
		t.Fatal("summoning sickness should be cleared even when untap step is skipped")
	}
}

func TestUntapAll_NormalUntap(t *testing.T) {
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Normal Bear", 2, 2, "creature")
	p.Tapped = true

	UntapAll(gs, 0)

	if p.Tapped {
		t.Fatal("normal creature should untap")
	}
	if countEvents(gs, "untap_done") == 0 {
		t.Fatal("missing untap_done event")
	}
}

func TestUntapAll_StunCounterOnSecondUntap(t *testing.T) {
	// After the stun counter is removed, the next untap should untap normally.
	gs := newFixtureGame(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	p.Tapped = true
	if p.Counters == nil {
		p.Counters = map[string]int{}
	}
	p.Counters["stun"] = 1

	// First untap — removes stun counter, stays tapped.
	UntapAll(gs, 0)
	if !p.Tapped {
		t.Fatal("should remain tapped after first untap (stun)")
	}

	// Second untap — no stun counter, should untap.
	UntapAll(gs, 0)
	if p.Tapped {
		t.Fatal("should untap after stun counter was removed")
	}
}

// =============================================================================
// Wave 5 — One-shot event-based delayed triggers.
// =============================================================================

func TestFireEventDelayedTriggers_Basic(t *testing.T) {
	gs := newFixtureGame(t)
	fired := false
	gs.RegisterDelayedTrigger(&DelayedTrigger{
		TriggerAt:      "on_event",
		ControllerSeat: 0,
		SourceCardName: "Test Source",
		OneShot:        true,
		ConditionFn: func(gs *GameState, ev *Event) bool {
			return ev.Kind == "damage"
		},
		EffectFn: func(gs *GameState) {
			fired = true
		},
	})

	// Fire with a non-matching event.
	ev := Event{Kind: "draw"}
	count := FireEventDelayedTriggers(gs, &ev)
	if count != 0 {
		t.Fatalf("expected 0 triggers fired, got %d", count)
	}
	if fired {
		t.Fatal("trigger should not have fired on non-matching event")
	}

	// Fire with a matching event.
	ev2 := Event{Kind: "damage"}
	count = FireEventDelayedTriggers(gs, &ev2)
	if count != 1 {
		t.Fatalf("expected 1 trigger fired, got %d", count)
	}
	if !fired {
		t.Fatal("trigger should have fired on matching event")
	}

	// Fire again — should not fire (consumed).
	fired = false
	ev3 := Event{Kind: "damage"}
	count = FireEventDelayedTriggers(gs, &ev3)
	if count != 0 {
		t.Fatalf("expected 0 triggers fired (consumed), got %d", count)
	}
	if fired {
		t.Fatal("consumed trigger should not fire again")
	}
}

// =============================================================================
// Wave 5 — Face-down cleared on zone change.
// =============================================================================

func TestMoveToZone_ClearsFaceDown(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Morph Creature", Owner: 0, FaceDown: true}

	gs.moveToZone(0, card, "graveyard")
	if card.FaceDown {
		t.Fatal("FaceDown should be cleared on zone change to graveyard")
	}
}

func TestMoveToZone_ClearsFaceDown_Exile(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Morph Creature", Owner: 0, FaceDown: true}

	gs.moveToZone(0, card, "exile")
	if card.FaceDown {
		t.Fatal("FaceDown should be cleared on zone change to exile")
	}
}

func TestMoveToZone_ClearsFaceDown_Hand(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Morph Creature", Owner: 0, FaceDown: true}

	gs.moveToZone(0, card, "hand")
	if card.FaceDown {
		t.Fatal("FaceDown should be cleared on zone change to hand")
	}
}
