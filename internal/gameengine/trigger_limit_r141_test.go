package gameengine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r141 — the once-each-turn trigger limiter, enforced at the single
// chokepoint every AST triggered ability already passed through.
//
// The fixture is Kambal's shape as r140's exporter now emits it: a
// Triggered carrying timing_restriction="once_per_turn", not a detached
// sibling Static.

func r141Trigger(restriction string) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger:           gameast.Trigger{Event: "attacks"},
		Effect:            &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}},
		TimingRestriction: restriction,
		Raw:               "whenever this creature attacks, draw a card",
	}
}

// r141Perm puts a creature on the battlefield carrying trig as its only
// ability. Uses r87Body-style explicit stats — addPerm's 0/0 would die to
// §704.5f before the test got anywhere.
func r141Perm(gs *GameState, seat int, name string, trig *gameast.Triggered) *Permanent {
	p := addBattlefield(gs, seat, name, 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{Name: name, Abilities: []gameast.Ability{trig}}
	return p
}

// r141Fires pushes trig for perm and reports whether it fired.
//
// Stack LENGTH is the wrong probe: outside a batch and a resolve frame the
// legacy path pushes and resolves in one go, so the stack is back to its
// previous depth by the time the call returns. The returned *StackItem is
// the real signal — nil means the ability never reached the stack.
func r141Fires(gs *GameState, perm *Permanent, trig *gameast.Triggered) bool {
	return PushTriggeredAbilityNode(gs, perm, trig) != nil
}

// r141Game is newFixtureGame plus stocked libraries. The fixture's seats
// start with EMPTY libraries, and a draw from an empty library loses the
// game — which would set gs.Flags["ended"] and make every later push
// return nil for a reason that has nothing to do with the limiter.
func r141Game(t *testing.T) *GameState {
	t.Helper()
	gs := newFixtureGame(t)
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for i := 0; i < 40; i++ {
			seat.Library = append(seat.Library, &Card{Name: "R141 Filler"})
		}
	}
	gs.Turn = 1
	return gs
}

func TestR141_ALimitedTriggerFiresOnceThenIsSuppressed(t *testing.T) {
	gs := r141Game(t)
	trig := r141Trigger("once_per_turn")
	p := r141Perm(gs, 0, "R141 Kambal Shape", trig)

	if !r141Fires(gs, p, trig) {
		t.Fatal("the first trigger of the turn must fire")
	}
	if r141Fires(gs, p, trig) {
		t.Fatal("the second trigger in the same turn fired — " +
			"the once-each-turn limiter did not hold")
	}
	if r141Fires(gs, p, trig) {
		t.Fatal("a third trigger fired; the limit is not a limit")
	}
}

func TestR141_TheLimitLiftsOnTheNextTurn(t *testing.T) {
	// The other direction, and the one a too-eager guard gets wrong: a
	// once-per-turn ability is not a once-per-game ability.
	gs := r141Game(t)
	trig := r141Trigger("once_per_turn")
	p := r141Perm(gs, 0, "R141 Next Turn", trig)

	if !r141Fires(gs, p, trig) {
		t.Fatal("first turn, first fire must work")
	}
	if r141Fires(gs, p, trig) {
		t.Fatal("precondition: the second fire this turn should be suppressed")
	}
	gs.Turn = 2
	if !r141Fires(gs, p, trig) {
		t.Fatal("the limiter did not lift on the next turn — " +
			"a once-per-turn limit became once-per-game")
	}
}

func TestR141_AnUnrestrictedTriggerIsNeverLimited(t *testing.T) {
	// The limiter must not invent a limit. Morbid Opportunist is in 97
	// decks WITH the clause; far more cards have none at all.
	gs := r141Game(t)
	trig := r141Trigger("")
	p := r141Perm(gs, 0, "R141 Unrestricted", trig)

	for i := 0; i < 4; i++ {
		if !r141Fires(gs, p, trig) {
			t.Fatalf("fire %d was suppressed on an ability with no restriction", i+1)
		}
	}
}

func TestR141_TwoCopiesOfTheCardHaveIndependentLimits(t *testing.T) {
	// The AST is shared per card NAME, so both permanents hold the same
	// *gameast.Triggered pointer. Identity must therefore be keyed on the
	// permanent as well, or the second copy inherits the first's count.
	gs := r141Game(t)
	trig := r141Trigger("once_per_turn")
	a := r141Perm(gs, 0, "R141 Twin", trig)
	b := r141Perm(gs, 1, "R141 Twin", trig)

	if !r141Fires(gs, a, trig) {
		t.Fatal("copy A must fire")
	}
	if !r141Fires(gs, b, trig) {
		t.Fatal("copy B was suppressed by copy A's count — " +
			"the limiter is keyed on the ability alone, not the object")
	}
	if r141Fires(gs, a, trig) {
		t.Fatal("copy A fired twice in one turn")
	}
	if r141Fires(gs, b, trig) {
		t.Fatal("copy B fired twice in one turn")
	}
}

func TestR141_ADifferentAbilityOnTheSamePermanentIsCountedSeparately(t *testing.T) {
	gs := r141Game(t)
	first := r141Trigger("once_per_turn")
	second := r141Trigger("once_per_turn")
	p := r141Perm(gs, 0, "R141 Two Abilities", first)

	if !r141Fires(gs, p, first) {
		t.Fatal("ability one must fire")
	}
	if !r141Fires(gs, p, second) {
		t.Fatal("ability two was suppressed by ability one's count")
	}
	if r141Fires(gs, p, first) {
		t.Fatal("ability one fired twice")
	}
}

func TestR141_TheEffectOnlyEntryPointIsNeverLimited(t *testing.T) {
	// PushTriggeredAbility carries no node, so it cannot know of a
	// restriction and must not guess one. Pins that the legacy entry point
	// keeps its old behaviour exactly.
	gs := r141Game(t)
	trig := r141Trigger("once_per_turn")
	p := r141Perm(gs, 0, "R141 Legacy Entry", trig)

	for i := 0; i < 3; i++ {
		if PushTriggeredAbility(gs, p, trig.Effect) == nil {
			t.Fatalf("effect-only push %d was suppressed; an unknown "+
				"restriction must not be treated as a limit", i+1)
		}
	}
}

// TestR142_SuppressionsArePublishedToACounter covers the gap r141 shipped
// with: enforcement whose only signal was a LogEvent, which the grinder
// discards wholesale under EventLogNone. The counter is what makes the
// limiter visible in production, so it gets a test rather than being
// trusted. Deltas rather than absolutes — the counter is process-wide and
// other tests in this package fire triggers too.
func TestR142_SuppressionsArePublishedToACounter(t *testing.T) {
	gs := r141Game(t)
	trig := r141Trigger("once_per_turn")
	p := r141Perm(gs, 0, "R142 Counted", trig)

	before := TriggerLimitSuppressions()
	if !r141Fires(gs, p, trig) {
		t.Fatal("precondition: the first fire must succeed")
	}
	if got := TriggerLimitSuppressions() - before; got != 0 {
		t.Fatalf("a successful fire counted as a suppression: +%d", got)
	}
	if r141Fires(gs, p, trig) {
		t.Fatal("precondition: the second fire must be suppressed")
	}
	if got := TriggerLimitSuppressions() - before; got != 1 {
		t.Fatalf("the suppression was not published: want +1, got +%d — "+
			"the limiter would be invisible in production again", got)
	}
}

func TestR142_AnUnlimitedTriggerNeverTouchesTheCounter(t *testing.T) {
	gs := r141Game(t)
	trig := r141Trigger("")
	p := r141Perm(gs, 0, "R142 Uncounted", trig)

	before := TriggerLimitSuppressions()
	for i := 0; i < 5; i++ {
		if !r141Fires(gs, p, trig) {
			t.Fatalf("fire %d suppressed on an unrestricted ability", i+1)
		}
	}
	if got := TriggerLimitSuppressions() - before; got != 0 {
		t.Fatalf("an unrestricted ability moved the counter by %d", got)
	}
}

// TestR141_TheChokepointIsStillTheOnlyWayOntoTheStack is a derived gate
// (the r134 pattern): it reads the package's own source rather than
// asserting a remembered fact. The limiter is only uniform because every
// AST trigger goes through pushTriggeredAbility. If a future change adds a
// site that builds a triggered StackItem by hand, that site would silently
// skip the limiter — and this test, not a grinder window, should be what
// says so.
func TestR141_TheChokepointIsStillTheOnlyWayOntoTheStack(t *testing.T) {
	// The two legitimate constructors, by file:
	//   stack.go               — pushTriggeredAbility, the chokepoint
	//   trigger_stack_bridge.go — PushPerCardTrigger, the per-card family
	//                             (Go handlers, not *gameast.Triggered)
	allowed := map[string]bool{
		"stack.go":                true,
		"trigger_stack_bridge.go": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	re := regexp.MustCompile(`Kind:\s*"triggered"`)
	var offenders []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		if allowed[n] {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		if re.Match(b) {
			offenders = append(offenders, n)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("these files build a triggered StackItem outside the "+
			"chokepoint, so they bypass the once-each-turn limiter: %v\n"+
			"Route them through PushTriggeredAbilityNode, or add the file "+
			"here with a reason if it is genuinely a different family.",
			offenders)
	}
}
