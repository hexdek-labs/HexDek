package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Third-eye magnitude r60 regressions. Pins the three counters added
// to make the existing opponentTutored bool, archetype-side counter
// classifier, and mass-removal flagging into MAGNITUDE signals (how
// many casts so far). All three feed downstream "they've cast 3 of
// their ~10 counters — next bluff is materially likelier empty"
// predictions.
//
//   - opponentCounterspellsCast — incremented on cast events whose
//     source has counter-target-spell oracle.
//   - opponentMassRemovalCast — incremented on cast events whose
//     source has wrath-shape oracle.
//   - opponentTutorsCast — incremented on tutor / search_library
//     resolution events (covers both spell tutors via the
//     resolve-side event AND activated-ability tutors like Survival
//     of the Fittest). Cast-side intentionally NOT incremented to
//     avoid double-counting spell tutors.

// ---------------------------------------------------------------------
// isCounterspellText classifier — direct tests
// ---------------------------------------------------------------------

func TestIsCounterspellText_KnownShapes(t *testing.T) {
	cases := []struct {
		name   string
		oracle string
		want   bool
	}{
		{"Counterspell", "counter target spell.", true},
		{"Mana Leak", "counter target spell unless its controller pays {3}.", true},
		{"Spell Snare", "counter target spell with mana value 2.", true},
		{"Force of Negation", "counter target noncreature spell. if it's not your turn, you may exile a blue card from your hand rather than pay this spell's mana cost.", true},
		{"Negate", "counter target noncreature spell.", true},
		{"Counterflux", "counter target spell. it can't be countered.", true},
		{"Essence Scatter", "counter target creature spell.", true},
		{"Annul", "counter target artifact or enchantment spell.", true},
		{"Mindbreak Trap", "exile target spell.", false},        // exile, not counter
		{"Lightning Bolt", "deal 3 damage to any target.", false}, // damage
		{"Vandalblast", "destroy target artifact.", false},
		{"Empty oracle", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCounterspellText(tc.oracle); got != tc.want {
				t.Errorf("%s: isCounterspellText = %v want %v\n  oracle: %q",
					tc.name, got, tc.want, tc.oracle)
			}
		})
	}
}

// ---------------------------------------------------------------------
// Accessor bounds + zero-value tests
// ---------------------------------------------------------------------

func TestOpponentCounterspellsCast_BoundsSafe(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	if got := h.OpponentCounterspellsCast(0); got != 0 {
		t.Errorf("pre-init: got %d, want 0", got)
	}
	if got := h.OpponentCounterspellsCast(-1); got != 0 {
		t.Errorf("negative seat: got %d, want 0", got)
	}
	if got := h.OpponentCounterspellsCast(999); got != 0 {
		t.Errorf("oob seat: got %d, want 0", got)
	}
}

func TestOpponentMassRemovalCast_BoundsSafe(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	if got := h.OpponentMassRemovalCast(0); got != 0 {
		t.Errorf("pre-init: got %d, want 0", got)
	}
	if got := h.OpponentMassRemovalCast(-1); got != 0 {
		t.Errorf("negative seat: got %d, want 0", got)
	}
}

func TestOpponentTutorsCast_BoundsSafe(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	if got := h.OpponentTutorsCast(0); got != 0 {
		t.Errorf("pre-init: got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------
// Event-driven increment tests
// ---------------------------------------------------------------------

// thirdEyeTestSetup primes a 4-seat game + hat through ObserveEvent's
// game_start init pathway, plants a counterspell card in opp seat 1's
// hand so the cast handler's findCardByName lookup succeeds, then
// returns the (game, hat) pair ready for cast-event observation.
func thirdEyeTestSetup(t *testing.T) (*gameengine.GameState, *YggdrasilHat) {
	t.Helper()
	gs := newTestGame(t, 4)
	h := NewYggdrasilHat(nil, 0)
	// Fire game_start to init all tracking arrays via ObserveEvent.
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})
	return gs, h
}

func TestObserveEvent_CounterspellCastIncrementsCount(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	// Plant Counterspell in opp seat 1's hand so findCardByName finds it.
	counterspell := gyCardWithOracle("Counterspell", []string{"instant"}, 2,
		"counter target spell.")
	// findCardByName scans Stack first then battlefield/graveyard — NOT
	// hand. Put the spell on the stack since a "cast" event would
	// naturally have the card sitting there mid-resolution.
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: counterspell, Controller: 1})

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Counterspell",
	})
	if got := h.OpponentCounterspellsCast(1); got != 1 {
		t.Errorf("after 1 counterspell cast: got %d, want 1", got)
	}

	// Cast another one.
	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Counterspell",
	})
	if got := h.OpponentCounterspellsCast(1); got != 2 {
		t.Errorf("after 2 counterspell casts: got %d, want 2", got)
	}

	// Other seats untouched.
	if got := h.OpponentCounterspellsCast(2); got != 0 {
		t.Errorf("untouched seat 2: got %d, want 0", got)
	}
}

func TestObserveEvent_NonCounterspellCastDoesNotIncrement(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	bolt := gyCardWithOracle("Lightning Bolt", []string{"instant"}, 1,
		"deal 3 damage to any target.")
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: bolt, Controller: 1})

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Lightning Bolt",
	})
	if got := h.OpponentCounterspellsCast(1); got != 0 {
		t.Errorf("non-counterspell cast: got %d, want 0", got)
	}
}

func TestObserveEvent_WrathCastIncrementsMassRemoval(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	wrath := gyCardWithOracle("Wrath of God", []string{"sorcery"}, 4,
		"destroy all creatures. they can't be regenerated.")
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: wrath, Controller: 1})

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Wrath of God",
	})
	if got := h.OpponentMassRemovalCast(1); got != 1 {
		t.Errorf("after Wrath: got %d, want 1", got)
	}
}

func TestObserveEvent_SingleTargetRemovalDoesNotIncrementMassRemoval(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	swords := gyCardWithOracle("Swords to Plowshares", []string{"instant"}, 1,
		"exile target creature. its controller gains life equal to its power.")
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: swords, Controller: 1})

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Swords to Plowshares",
	})
	if got := h.OpponentMassRemovalCast(1); got != 0 {
		t.Errorf("single-target removal: got %d, want 0", got)
	}
}

func TestObserveEvent_TutorResolutionIncrementsTutorsCast(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	// Tutor handler accesses gs.Turn — pass the real game state.
	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "tutor",
		Seat: 1,
	})
	if got := h.OpponentTutorsCast(1); got != 1 {
		t.Errorf("after tutor event: got %d, want 1", got)
	}

	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "search_library",
		Seat: 1,
	})
	if got := h.OpponentTutorsCast(1); got != 2 {
		t.Errorf("after search_library event: got %d, want 2", got)
	}
}

func TestObserveEvent_TutorCastDoesNotDoubleCount(t *testing.T) {
	// Spell tutor (Demonic Tutor) emits BOTH a cast event AND a tutor
	// resolve event. The cast handler must NOT increment opponentTutorsCast
	// — only the resolve handler does, to avoid double counting.
	gs, h := thirdEyeTestSetup(t)
	demonic := gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
		"search your library for a card and put that card into your hand. then shuffle.")
	gs.Stack = append(gs.Stack, &gameengine.StackItem{Card: demonic, Controller: 1})

	// Cast event fires first.
	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind:   "cast",
		Seat:   1,
		Source: "Demonic Tutor",
	})
	if got := h.OpponentTutorsCast(1); got != 0 {
		t.Errorf("cast-only (no resolve yet): got %d, want 0 (resolve-side is the only increment path)", got)
	}

	// Tutor resolve event fires.
	h.ObserveEvent(gs, 0, &gameengine.Event{
		Kind: "tutor",
		Seat: 1,
	})
	if got := h.OpponentTutorsCast(1); got != 1 {
		t.Errorf("after resolve: got %d, want 1 (single increment)", got)
	}
}

// ---------------------------------------------------------------------
// Reset on game_start
// ---------------------------------------------------------------------

func TestGameStartResetsThirdEyeMagnitudeCounters(t *testing.T) {
	gs, h := thirdEyeTestSetup(t)
	// Seed all three counters via direct access (already init'd by setup).
	h.opponentCounterspellsCast[1] = 3
	h.opponentMassRemovalCast[2] = 2
	h.opponentTutorsCast[3] = 5

	// Game start resets.
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	if got := h.OpponentCounterspellsCast(1); got != 0 {
		t.Errorf("counterspell count not reset: got %d", got)
	}
	if got := h.OpponentMassRemovalCast(2); got != 0 {
		t.Errorf("mass removal count not reset: got %d", got)
	}
	if got := h.OpponentTutorsCast(3); got != 0 {
		t.Errorf("tutor count not reset: got %d", got)
	}
}

// ---------------------------------------------------------------------
// PredictedTutorTargetClass
// ---------------------------------------------------------------------

func TestPredictedTutorTargetClass_RoutesByArchetype(t *testing.T) {
	cases := []struct {
		archetype string
		want      string
	}{
		{"combo", "combo_piece"},
		{"control", "interaction"},
		{"aggro", "haste_threat"},
		{"midrange", "value"},
		{"unknown", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.archetype, func(t *testing.T) {
			_, h := thirdEyeTestSetup(t)
			h.opponentTutorsCast[1] = 1 // mark that they've tutored at least once
			h.perceivedArchetype[1] = tc.archetype
			if got := h.PredictedTutorTargetClass(1); got != tc.want {
				t.Errorf("archetype=%q: got %q want %q", tc.archetype, got, tc.want)
			}
		})
	}
}

func TestPredictedTutorTargetClass_EmptyWhenNoTutorsObserved(t *testing.T) {
	_, h := thirdEyeTestSetup(t)
	h.perceivedArchetype[1] = "combo"
	// opponentTutorsCast[1] still 0 → no signal even with archetype set.
	if got := h.PredictedTutorTargetClass(1); got != "" {
		t.Errorf("no tutors observed: got %q, want empty", got)
	}
}

func TestPredictedTutorTargetClass_BoundsSafe(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	if got := h.PredictedTutorTargetClass(0); got != "" {
		t.Errorf("pre-init: got %q, want empty", got)
	}
	if got := h.PredictedTutorTargetClass(-1); got != "" {
		t.Errorf("negative seat: got %q, want empty", got)
	}
	if got := h.PredictedTutorTargetClass(999); got != "" {
		t.Errorf("oob seat: got %q, want empty", got)
	}
}
