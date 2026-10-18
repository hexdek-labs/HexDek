package gameengine

import (
	"fmt"
	"sort"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r175 — coverage for "if this spell was cast <some way>" conditions.
//
// Asked for directly by the project owner: "we need a test to see how
// often we miss this." There are two independent ways a card can slip
// through, and they need separate tests because they fail in different
// places:
//
//   (a) THE EXPORTER never learned the phrasing, so no cast_mode_check
//       node is emitted at all. Measured separately against the live
//       corpus; see docs/cast-mode-coverage-r175.md.
//
//   (b) THE ENGINE has the node but cannot evaluate the mode, so the
//       condition is assumed TRUE and the card gets its bonus for free.
//       That is what this file measures.
//
// The census in cast_mode_r137.go, taken over the live 32,418-card
// corpus, gives the per-mode card counts used below. Weighting by those
// counts is what turns "4 of 11 modes" into a statement about cards.

// r175Modes is every mode the parser is known to emit, with how many
// cards carry it. Source: cast_mode_r137.go's documented census.
var r175Modes = []struct {
	mode  string
	cards int
}{
	{"kicked", 98},
	{"from_graveyard", 14},
	{"bargained", 9},
	{"five_or_more_mana_spent", 9},
	{"additional_cost_paid", 8},
	{"from_exile", 4},
	{"foretold", 3},
	{"madness_cost_paid", 3},
	{"prowl_cost_paid", 2},
	{"surge_cost_paid", 1},
	{"from_hand", 1},
}

// r175Satisfied / r175Unsatisfied build the two StackItems a mode must
// be able to tell apart. Anything not explicitly stamped is left absent,
// which is the real shape of a spell cast by a path that never
// considered that mode.
func r175Satisfied(mode string) *StackItem {
	it := &StackItem{Card: &Card{Name: "probe"}, CostMeta: map[string]interface{}{}}
	switch mode {
	case "kicked":
		it.CostMeta["kicked"] = 1
	case "bargained":
		it.CostMeta["bargained"] = true
	case "madness_cost_paid":
		it.CostMeta["madness_cast"] = true
	case "surge_cost_paid":
		it.CostMeta["surge_cast"] = true
	case "from_graveyard":
		it.CastZone = "graveyard"
	case "from_exile":
		it.CastZone = "exile"
	case "from_hand":
		it.CastZone = "hand"
	}
	return it
}

func r175Unsatisfied(mode string) *StackItem {
	it := &StackItem{Card: &Card{Name: "probe"}, CostMeta: map[string]interface{}{}}
	switch mode {
	case "kicked":
		it.CostMeta["kicked"] = 0
	case "bargained":
		it.CostMeta["bargained"] = false
	case "madness_cost_paid":
		it.CostMeta["madness_cast"] = false
	case "surge_cost_paid":
		it.CostMeta["surge_cast"] = false
	case "from_graveyard", "from_exile":
		it.CastZone = "hand"
	case "from_hand":
		it.CastZone = "graveyard"
	}
	return it
}

// TestR175_CastModeCoverage is the headline count. It is deliberately
// NOT a pass/fail gate on the undecidable modes — those are a known,
// filed gap, and failing the suite on them would just get the test
// disabled. It FAILS only if coverage gets WORSE than the recorded
// baseline, and it always prints the current number.
func TestR175_CastModeCoverage(t *testing.T) {
	const knownUndecidable = 4 // five_or_more_mana_spent, additional_cost_paid, foretold, prowl_cost_paid

	var decidable, undecidable []string
	cardsDecidable, cardsUndecidable := 0, 0

	for _, m := range r175Modes {
		satTrue, knownTrue := castModeSatisfied(r175Satisfied(m.mode), m.mode)
		satFalse, knownFalse := castModeSatisfied(r175Unsatisfied(m.mode), m.mode)

		// A mode counts as covered only if the engine can answer BOTH
		// ways AND the two answers actually differ. A mode that always
		// says the same thing is not evaluating anything.
		ok := knownTrue && knownFalse && satTrue && !satFalse
		if ok {
			decidable = append(decidable, m.mode)
			cardsDecidable += m.cards
		} else {
			undecidable = append(undecidable,
				fmt.Sprintf("%s (%d cards, known=%v/%v satisfied=%v/%v)",
					m.mode, m.cards, knownTrue, knownFalse, satTrue, satFalse))
			cardsUndecidable += m.cards
		}
	}
	sort.Strings(decidable)

	total := cardsDecidable + cardsUndecidable
	t.Logf("cast-mode coverage: %d/%d modes decidable, %d/%d CARDS covered (%.1f%%)",
		len(decidable), len(r175Modes), cardsDecidable, total,
		100*float64(cardsDecidable)/float64(total))
	t.Logf("decidable: %v", decidable)
	for _, u := range undecidable {
		t.Logf("UNDECIDABLE — condition is assumed TRUE, card gets its bonus free: %s", u)
	}

	if len(undecidable) > knownUndecidable {
		t.Errorf("cast-mode coverage REGRESSED: %d modes undecidable, baseline is %d.\n"+
			"A mode that cannot be evaluated is assumed TRUE, so every card carrying "+
			"it gets its conditional bonus unconditionally.", len(undecidable), knownUndecidable)
	}
	if len(undecidable) < knownUndecidable {
		t.Errorf("cast-mode coverage IMPROVED to %d undecidable modes (baseline %d) — "+
			"good, now lower knownUndecidable to %d so the ratchet holds the gain.",
			len(undecidable), knownUndecidable, len(undecidable))
	}
}

// Each decidable mode must distinguish its two shapes. Split out from
// the census so a specific regression names the specific mode.
func TestR175_EachDecidableModeDistinguishesBothDirections(t *testing.T) {
	for _, m := range r175Modes {
		m := m
		t.Run(m.mode, func(t *testing.T) {
			satTrue, knownTrue := castModeSatisfied(r175Satisfied(m.mode), m.mode)
			if !knownTrue {
				t.Skipf("mode is not evaluable at all (%d cards affected) — "+
					"counted by TestR175_CastModeCoverage, not re-failed here", m.cards)
			}
			if !satTrue {
				t.Errorf("a spell stamped as %s was not recognised as %s", m.mode, m.mode)
			}
			satFalse, knownFalse := castModeSatisfied(r175Unsatisfied(m.mode), m.mode)
			if !knownFalse {
				t.Errorf("%s is evaluable when satisfied but NOT when unsatisfied — "+
					"that asymmetry means the negative case silently assumes true", m.mode)
			}
			if satFalse {
				t.Errorf("a spell NOT stamped as %s was reported as %s", m.mode, m.mode)
			}
		})
	}
}

// An ABSENT stamp must read as unknown, never as false. Answering "no"
// would strip an effect off a card that earned it — the opposite failure
// to the one above, and the reason castModeSatisfied returns two bools.
func TestR175_AbsentStampIsUnknownNotFalse(t *testing.T) {
	bare := &StackItem{Card: &Card{Name: "probe"}, CostMeta: map[string]interface{}{}}
	for _, mode := range []string{"kicked", "bargained", "madness_cost_paid", "surge_cost_paid"} {
		satisfied, known := castModeSatisfied(bare, mode)
		if known {
			t.Errorf("%s: an absent CostMeta stamp reported known=true (satisfied=%v). "+
				"Absence means the cast path never considered this mode, which is "+
				"NOT the same as the spell having been cast without it.", mode, satisfied)
		}
	}
}

// ---------------------------------------------------------------------
// TRANSPORT. The census above measures whether a mode CAN be evaluated
// given a StackItem. That is worthless if the handler never receives
// one. currentResolvingSpell is the only way it gets there, and it
// refuses several shapes — so a mode that is "decidable" in isolation
// can still be undecided in situ. Test the transport, not just the
// destination.
// ---------------------------------------------------------------------

// r175KickedArgs builds the arg slice an if_intervening_tail really
// receives. NOTE: modShape accepts ONLY *gameast.Modification or
// *gameast.ModificationEffect — a map[string]interface{} that merely
// looks like one is silently rejected and every call comes back
// undecided. A first cut of this test used a map and "proved" the whole
// path was inert; it was proving its own bug.
func r175KickedArgs() []interface{} {
	return []interface{}{
		&gameast.Modification{ModKind: "cast_mode_check", Args: []interface{}{"kicked"}},
		"destroy target nonbasic land",
	}
}

// Shape 1: an instant/sorcery mid-resolution. The spell is still the top
// resolving item, so the condition should be answerable.
func TestR175_Transport_SpellMidResolutionIsDecided(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Kicked Sorcery", Owner: 0}
	item := &StackItem{Card: card, CostMeta: map[string]interface{}{"kicked": 1}}
	gs.resolvingItems = append(gs.resolvingItems, item)

	src := &Permanent{Card: card, Controller: 0, Owner: 0,
		Flags: map[string]int{}, Counters: map[string]int{}}

	allow, decided := interveningTailAllows(gs, src, r175KickedArgs())
	t.Logf("spell mid-resolution: allow=%v decided=%v", allow, decided)
	if !decided {
		t.Error("a spell that is the top resolving item must be answerable — " +
			"if this fails the whole cast_mode_check path is inert")
	}
	if !allow {
		t.Error("the spell was stamped kicked=1 but the condition was not satisfied")
	}
}

// Shape 2: Goblin Ruinblaster. "When this enters, if it was kicked,
// destroy target nonbasic land." By the time the ETB trigger resolves
// the spell has LEFT the stack, so the resolving item is the trigger —
// which carries a Source. currentResolvingSpell rejects that.
//
// This test RECORDS the behaviour rather than asserting a fix, because
// which way it should go is a rules decision, not a cleanup: making it
// decide changes real game outcomes.
func TestR175_Transport_PermanentETBTriggerShape(t *testing.T) {
	gs := newFixtureGame(t)
	card := &Card{Name: "Goblin Ruinblaster", Owner: 0}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0,
		Flags: map[string]int{}, Counters: map[string]int{}}

	// The ETB trigger is its own stack item and carries a Source.
	trigger := &StackItem{Card: card, Source: perm, Kind: "trigger"}
	gs.resolvingItems = append(gs.resolvingItems, trigger)

	allow, decided := interveningTailAllows(gs, perm, r175KickedArgs())
	t.Logf("permanent ETB trigger: allow=%v decided=%v", allow, decided)
	if decided {
		t.Logf("DECIDED — the ETB path reaches the cast-mode state. " +
			"If that is new, update the r175 notes; it would RAISE coverage.")
	} else {
		t.Logf("NOT DECIDED — for a permanent's ETB the spell has already left " +
			"the stack, so currentResolvingSpell returns nil and the condition " +
			"is assumed TRUE. Kicker on a PERMANENT is still read correctly by " +
			"the separate Conditional 'kicked' path (resolve.go), which reads " +
			"Flags[\"kicked\"] on the permanent — so this shape is covered " +
			"elsewhere. The gap is any ETB-triggered cast_mode_check whose mode " +
			"has NO permanent-flag equivalent.")
	}
}

// Nothing resolving at all must be undecided, never a confident false.
func TestR175_Transport_NothingResolvingIsUndecided(t *testing.T) {
	gs := newFixtureGame(t)
	src := &Permanent{Card: &Card{Name: "x"}, Controller: 0, Owner: 0,
		Flags: map[string]int{}, Counters: map[string]int{}}
	if _, decided := interveningTailAllows(gs, src, r175KickedArgs()); decided {
		t.Error("with an empty resolving stack the engine cannot know how the " +
			"spell was cast; reporting decided=true would assume an answer")
	}
}
