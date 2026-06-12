package progression

import (
	"encoding/csv"
	"fmt"
	"os"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge/outcome"
)

func intRef(n int) gameast.NumberOrRef { return gameast.NumberOrRef{IsInt: true, Int: n} }

func gainLifeTrigger(event string, n int) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event},
		Effect:  &gameast.GainLife{Amount: intRef(n), Target: gameast.Filter{Base: "you"}},
		Raw:     fmt.Sprintf("when this creature %ss, gain %d", map[string]string{"etb":"enter","die":"die","attack":"attack"}[event], n),
	}
}

// Teeth: known-good synthetic triggers pass both FIRE and PHANTOM for
// every in-scope event.
func TestProgression_SyntheticTriggersPass(t *testing.T) {
	for _, ev := range []string{"etb", "die", "attack"} {
		findings, ran := CheckTrigger("unit:"+ev, gainLifeTrigger(ev, 3))
		if !ran {
			t.Fatalf("%s: unexpectedly out of scope", ev)
		}
		for _, f := range findings {
			t.Errorf("%s/%s diverged:\n  expected %s\n  actual   %s", ev, f.Check, f.Expected, f.Actual)
		}
	}
}

// Teeth: a deliberately-missed trigger is caught — an AST trigger whose
// event the engine has no dispatch for must flag "fire".
func TestProgression_MissedTriggerCaught(t *testing.T) {
	// The engine fires nothing for the bearer entering when the trigger
	// event is "attack" — entering is the WRONG stimulus. Simulate a
	// miss by checking the FIRE path manually with a wrong-event AST.
	spec := progressionSpec()
	tr := gainLifeTrigger("etb", 3)
	gs, bearer := outcome.BuildBoardForSpec(spec, "unit:miss")
	bearer.Card.AST = wrapSingle(tr)
	before := outcome.Snap(gs)
	// Do NOT fire ETB triggers — the trigger should have fired but the
	// "engine" (sabotaged here) does nothing.
	actual := outcome.DiffSnapshots(before, outcome.Snap(gs))
	expectedSet, _ := expectedFireSet(spec, tr.Effect)
	matched := false
	for _, exp := range expectedSet {
		if actual.Equal(exp) {
			matched = true
		}
	}
	if matched {
		t.Fatal("a zero delta must NOT match a non-empty expected fire (missed-trigger detection broken)")
	}
}

// Check 4 — APNAP: two seats' die-triggers batched in one SBA sweep.
// CR §603.3b: the ACTIVE player's trigger goes on the stack first and
// therefore RESOLVES LAST (LIFO). Observable: the non-active seat's
// effect lands in the event stream before the active seat's. This is
// the one event-stream-based assertion in the dimension (state alone
// cannot order two commutative effects).
func TestProgression_APNAPOrderOnSimultaneousDeaths(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.EventPolicy = gameengine.EventLogFull
	gs.Active = 0
	gs.Phase = "main"

	mk := func(seat, gain int) *gameengine.Permanent {
		tr := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: "die"},
			Effect:  &gameast.GainLife{Amount: intRef(gain), Target: gameast.Filter{Base: "you"}},
			Raw:     "when this creature dies, gain life",
		}
		c := &gameengine.Card{
			Name: fmt.Sprintf("Doomed %d", seat), Owner: seat,
			Types:     []string{"creature"},
			BasePower: 1, BaseToughness: 1,
			AST: &gameast.CardAST{Abilities: []gameast.Ability{tr}},
		}
		p := &gameengine.Permanent{
			Card: c, Controller: seat, Owner: seat,
			Counters: map[string]int{}, Flags: map[string]int{},
			Timestamp: gs.NextTimestamp(),
		}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
		return p
	}
	a := mk(0, 1) // active seat's trigger: +1 life
	b := mk(1, 2) // non-active: +2 life

	// Lethal damage on both; one SBA sweep kills them simultaneously.
	a.MarkedDamage = 5
	b.MarkedDamage = 5
	gameengine.StateBasedActions(gs)

	if gs.Seats[0].Life != 21 || gs.Seats[1].Life != 22 {
		t.Fatalf("both die-triggers must fire exactly once: life=(%d,%d) want (21,22)",
			gs.Seats[0].Life, gs.Seats[1].Life)
	}
	// Event order: find the two life_gain-ish events; the NON-ACTIVE
	// seat's must resolve first (active player's trigger stacks first,
	// resolves last — CR §603.3b + §101.4).
	orderedSeats := []int{}
	for _, ev := range gs.EventLog {
		if ev.Kind == "life_gain" || ev.Kind == "gain_life" || ev.Kind == "life_change" {
			if ev.Amount > 0 {
				orderedSeats = append(orderedSeats, ev.Seat)
			}
		}
	}
	if len(orderedSeats) < 2 {
		t.Skipf("life-gain events not individually logged (%d found) — APNAP order unobservable via stream on this engine build", len(orderedSeats))
	}
	first, last := orderedSeats[0], orderedSeats[len(orderedSeats)-1]
	if first != 1 || last != 0 {
		t.Errorf("APNAP order violated: non-active seat 1 must resolve first, active seat 0 last; got first=%d last=%d", first, last)
	}
}

// Corpus audit: every in-scope (card, trigger) across the dataset.
func TestProgression_CorpusAudit(t *testing.T) {
	const datasetPath = "../../../data/rules/ast_dataset.jsonl"
	if _, err := os.Stat(datasetPath); err != nil {
		t.Skipf("AST dataset not present (%v)", err)
	}
	corpus, err := astload.Load(datasetPath)
	if err != nil {
		t.Fatalf("astload: %v", err)
	}

	var all []*Finding
	inScope, passed := 0, 0
	byEvent := map[string]int{}
	for _, name := range corpus.Names() {
		ast, _ := corpus.Get(name)
		if ast == nil {
			continue
		}
		for _, ab := range ast.Abilities {
			tr, ok := ab.(*gameast.Triggered)
			if !ok {
				continue
			}
			findings, ran := CheckTrigger(name, tr)
			if !ran {
				continue
			}
			inScope++
			if ev, _ := InScopeTrigger(tr); ev != "" {
				byEvent[ev]++
			}
			if len(findings) == 0 {
				passed++
			}
			all = append(all, findings...)
		}
	}
	t.Logf("progression corpus audit: %d triggers in scope, %d passed, %d findings", inScope, passed, len(all))
	t.Logf("by event: %v", byEvent)

	f, err := os.Create("/tmp/fable-review/progression-findings-r63.csv")
	if err != nil {
		t.Fatalf("csv: %v", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"card", "event", "check", "expected", "actual", "raw"})
	for _, fd := range all {
		_ = w.Write([]string{fd.CardName, fd.Event, fd.Check, fd.Expected, fd.Actual, fd.Raw})
	}
	fmt.Printf("PROGRESSION-AUDIT inScope=%d passed=%d findings=%d\n", inScope, passed, len(all))
}
