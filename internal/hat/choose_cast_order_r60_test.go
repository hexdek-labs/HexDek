package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// choose_cast_order_r60_test.go — regressions for the R60 castHeuristic
// signals: ramp-unlocks-hand and dead-target-creature deprioritization.
//
// castHeuristic is the budget=0 fallback path of ChooseCastFromHand —
// it sees the castable pool and the seat's hand/battlefield and picks
// the next card to cast in main-phase order. The two signals plug into
// the existing turn ≤ 12 ramp-priority rule and the default cardHeuristic
// sort.

// addLandToSeat adds an untapped basic land to the seat's battlefield
// so AvailableManaEstimate counts it. Returns the permanent.
func addLandToSeat(gs *gameengine.GameState, seatIdx int, name string) *gameengine.Permanent {
	land := newTestCardMinimal(name, []string{"land", "basic"}, 0, nil)
	land.TypeLine = "Basic Land — " + name
	return newTestPermanent(gs.Seats[seatIdx], land, 0, 0)
}

// rampArtifact builds a CMC-1 artifact that classifies as ramp via
// isRampCard's CMC≤3 + artifact rule.
func rampArtifact(name string) *gameengine.Card {
	return newTestCardMinimal(name, []string{"artifact"}, 1, nil)
}

// vanillaSpell builds a non-ramp, non-draw spell card. Sorcery type
// chosen so categorizeCard doesn't classify it as ramp or threat.
func vanillaSpell(name string, cmc int) *gameengine.Card {
	return newTestCardMinimal(name, []string{"sorcery"}, cmc, nil)
}

// targetCreatureBuffSpell builds an instant/sorcery whose oracle text
// requires a controlled creature target — castIsDead's primary case.
func targetCreatureBuffSpell(name string, cmc int) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities,
		&gameast.Static{Raw: "Target creature you control gets +3/+3 until end of turn."},
	)
	return newTestCardMinimal(name, []string{"instant"}, cmc, ast)
}

// -----------------------------------------------------------------------------
// rampUnlocksHand helper — pure-ish via the receiver
// -----------------------------------------------------------------------------

func TestRampUnlocksHand_DetectsUnlockTarget(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	// 3 untapped lands → avail = 3.
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	// Big threat at CMC 5 — in [avail+1=4, avail+2=5] window → unlock.
	gs.Seats[0].Hand = []*gameengine.Card{
		vanillaSpell("Big Threat", 5),
	}
	if !h.rampUnlocksHand(gs, 0) {
		t.Fatal("ramp should be flagged as unlocking the CMC-5 hand card at avail=3")
	}
}

func TestRampUnlocksHand_NoUnlockTargetReturnsFalse(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	// Hand cards are all already castable (CMC ≤ avail).
	gs.Seats[0].Hand = []*gameengine.Card{
		vanillaSpell("Cheap Spell", 2),
		vanillaSpell("On-Curve Spell", 3),
	}
	if h.rampUnlocksHand(gs, 0) {
		t.Fatal("no out-of-reach hand card → rampUnlocksHand should be false")
	}
}

func TestRampUnlocksHand_RampCardsItselfNotCountedAsTarget(t *testing.T) {
	// A ramp card in hand at CMC=5 shouldn't count as an unlock target
	// — we don't want to ramp INTO a ramp card.
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	gs.Seats[0].Hand = []*gameengine.Card{
		// Cultivate-shaped ramp: sorcery, CMC 3, but flagged via CardRoles.
		newTestCardMinimal("Big Ramp Spell", []string{"sorcery"}, 5, nil),
	}
	// Inject the role classification so categorizeWithFreya returns CatRamp.
	h.Strategy = &StrategyProfile{
		Archetype: ArchetypeMidrange,
		CardRoles: map[string]string{"Big Ramp Spell": "Ramp"},
	}
	if h.rampUnlocksHand(gs, 0) {
		t.Fatal("ramp-into-ramp should NOT be flagged as an unlock target")
	}
}

func TestRampUnlocksHand_LandsInHandIgnored(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	// Lands shouldn't be treated as unlock targets.
	gs.Seats[0].Hand = []*gameengine.Card{
		newTestCardMinimal("Some Land", []string{"land"}, 0, nil),
	}
	if h.rampUnlocksHand(gs, 0) {
		t.Fatal("lands in hand should not register as unlock targets")
	}
}

// -----------------------------------------------------------------------------
// castHeuristic — late-game ramp priority when unlock target exists
// -----------------------------------------------------------------------------

func TestCastHeuristic_LateGame_RampFirstWhenUnlocksHand(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 15 // well past the turn ≤ 12 cutoff
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	ramp := rampArtifact("Sol Ring")
	spell := vanillaSpell("Lightning Strike", 3)
	bigUnlock := vanillaSpell("Worldspine Wurm", 5) // not in pool (uncastable)
	gs.Seats[0].Hand = []*gameengine.Card{ramp, spell, bigUnlock}

	pool := []*gameengine.Card{ramp, spell}
	pick := h.castHeuristic(gs, 0, pool)
	if pick != ramp {
		t.Fatalf("late-game ramp with hand unlock target should be played first; got %s",
			pick.DisplayName())
	}
}

func TestCastHeuristic_LateGame_NoRampPriorityWithoutUnlock(t *testing.T) {
	// Same shape as above but no unlock target in hand — the late-game
	// ramp priority must NOT fire.
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 15
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	ramp := rampArtifact("Sol Ring")
	spell := vanillaSpell("Cheap Spell", 1) // already castable, in-range
	gs.Seats[0].Hand = []*gameengine.Card{ramp, spell}

	pool := []*gameengine.Card{ramp, spell}
	pick := h.castHeuristic(gs, 0, pool)
	// Without the unlock signal, the default cardHeuristic sort governs.
	// We just assert SOMETHING is picked deterministically and isn't nil.
	if pick == nil {
		t.Fatal("expected a non-nil pick when pool is non-empty")
	}
}

func TestCastHeuristic_EarlyGame_RampStillFiresAtTurn5(t *testing.T) {
	// Pin existing behavior: turn 5 → ramp first (existing rule),
	// unchanged by the late-game extension.
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 5
	for i := 0; i < 2; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	ramp := rampArtifact("Sol Ring")
	spell := vanillaSpell("Lightning Strike", 2)
	gs.Seats[0].Hand = []*gameengine.Card{ramp, spell}

	pool := []*gameengine.Card{ramp, spell}
	pick := h.castHeuristic(gs, 0, pool)
	if pick != ramp {
		t.Fatalf("early-game ramp should still fire first; got %s", pick.DisplayName())
	}
}

// -----------------------------------------------------------------------------
// castIsDead helper
// -----------------------------------------------------------------------------

func TestCastIsDead_TargetCreatureBuffWithNoCreatures(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	buff := targetCreatureBuffSpell("Giant Growth", 1)

	if !h.castIsDead(gs, 0, buff) {
		t.Fatal("target-creature buff with no creatures should be dead")
	}
}

func TestCastIsDead_TargetCreatureBuffWithCreatures(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	newTestPermanent(gs.Seats[0], bear, 2, 2)

	buff := targetCreatureBuffSpell("Giant Growth", 1)
	if h.castIsDead(gs, 0, buff) {
		t.Fatal("buff should not be dead when we control a creature")
	}
}

func TestCastIsDead_CreatureSpellWithSelfReferenceNotDead(t *testing.T) {
	// A creature with "target creature you control gains..." as an ETB
	// ability still resolves usefully because it brings a creature into
	// play that satisfies the requirement.
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)

	ast := &gameast.CardAST{Name: "ETB Creature"}
	ast.Abilities = append(ast.Abilities,
		&gameast.Static{Raw: "When this enters, target creature you control gets +1/+1."},
	)
	creature := newTestCardMinimal("ETB Creature", []string{"creature"}, 3, ast)

	if h.castIsDead(gs, 0, creature) {
		t.Fatal("creature with target-creature ETB should NOT be flagged as dead — itself satisfies the requirement on resolution")
	}
}

func TestCastIsDead_NonTargetingSpellAlwaysAlive(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)

	bolt := newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil)
	if h.castIsDead(gs, 0, bolt) {
		t.Fatal("plain damage spell should never be flagged as dead")
	}
}

// -----------------------------------------------------------------------------
// castHeuristic — dead-card-last in default sort
// -----------------------------------------------------------------------------

func TestCastHeuristic_DefaultSort_DeadCardPushedLast(t *testing.T) {
	// Late turn, no ramp, no unlock target → falls through to the
	// default cardHeuristic sort. With zero creatures, a target-creature
	// buff should not be picked first even if its cardHeuristic ranks
	// high.
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 15
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}

	buff := targetCreatureBuffSpell("Berserk", 1) // dead — no creatures
	bolt := newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil)
	gs.Seats[0].Hand = []*gameengine.Card{buff, bolt}

	pool := []*gameengine.Card{buff, bolt}
	pick := h.castHeuristic(gs, 0, pool)
	if pick == buff {
		t.Fatalf("dead target-creature buff should not be picked when seat has no creatures; got %s",
			pick.DisplayName())
	}
}

func TestCastHeuristic_DefaultSort_BuffPickableWhenCreaturesExist(t *testing.T) {
	// Same shape but the seat now controls a creature — the buff isn't
	// dead and the default sort decides freely. We just assert the call
	// returns a non-nil result (the buff is no longer artificially
	// deprioritized).
	h := NewYggdrasilHatWithNoise(nil, 0, 0)
	gs := newTestGame(t, 2)
	gs.Turn = 15
	for i := 0; i < 3; i++ {
		addLandToSeat(gs, 0, "Forest")
	}
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	newTestPermanent(gs.Seats[0], bear, 2, 2)

	buff := targetCreatureBuffSpell("Berserk", 1)
	pool := []*gameengine.Card{buff}
	gs.Seats[0].Hand = []*gameengine.Card{buff}

	pick := h.castHeuristic(gs, 0, pool)
	if pick == nil {
		t.Fatal("expected a non-nil pick with the buff usable")
	}
}
