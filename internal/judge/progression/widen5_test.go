package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// widen5_test.go — pins the r63c phase-3c scope gates (becomes_blocked +
// ordinal first-main).

func drawEff() gameast.Effect {
	return &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}}
}

// Self "becomes blocked" wordings are in scope; the engine fires them and
// the checker observes the effect delta.
func TestBecomesBlocked_InScopeSelf(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "becomes_blocked"},
		Effect:  drawEff(),
		Raw:     "whenever this creature becomes blocked, draw a card",
	}
	if !InScopeBecomesBlockedTrigger(tr) {
		t.Fatal("self becomes-blocked draw trigger must be in scope")
	}
}

// "each creature blocking it" / "blocking creature" effects scale with the
// actual blocker set — the bare expectation cannot model it (Battle-Scarred
// Goblin). Fail closed.
func TestBecomesBlocked_GatesBlockerScopedEffects(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "becomes_blocked"},
		Effect:  &gameast.Damage{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "creature"}},
		Raw:     "whenever this creature becomes blocked, it deals 1 damage to each creature blocking it",
	}
	if InScopeBecomesBlockedTrigger(tr) {
		t.Fatal(`"blocking it" becomes-blocked effect must be out of scope`)
	}
}

// "becomes blocked by …" carries a dropped blocker condition — out of scope.
func TestBecomesBlocked_GatesBlockedBy(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "becomes_blocked_by"},
		Effect:  drawEff(),
		Raw:     "whenever this creature becomes blocked by a creature, draw a card",
	}
	if InScopeBecomesBlockedTrigger(tr) {
		t.Fatal(`"becomes blocked by" must be out of scope`)
	}
}

// "at the beginning of your first main phase," is in scope; "next end step"
// / "second main phase" are out (delayed / postcombat — separate bands).
func TestOrdinalFirstMain_ScopeGate(t *testing.T) {
	in := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "beginning_of_ordinal_step"},
		Effect:  drawEff(),
		Raw:     "at the beginning of your first main phase, draw a card",
	}
	if !InScopeOrdinalFirstMainTrigger(in) {
		t.Fatal("your first main phase draw trigger must be in scope")
	}
	for _, raw := range []string{
		"at the beginning of the next end step, draw a card",
		"at the beginning of your second main phase, draw a card",
		"at the beginning of your next upkeep, draw a card",
	} {
		out := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: "beginning_of_ordinal_step"},
			Effect:  drawEff(),
			Raw:     raw,
		}
		if InScopeOrdinalFirstMainTrigger(out) {
			t.Fatalf("non-first-main ordinal trigger must be out of scope: %q", raw)
		}
	}
}

// A conditional "if" rider is dropped by the parser — fail closed.
func TestOrdinalFirstMain_GatesIfRider(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "beginning_of_ordinal_step"},
		Effect:  drawEff(),
		Raw:     "at the beginning of your first main phase, if you control a plains, draw a card",
	}
	if InScopeOrdinalFirstMainTrigger(tr) {
		t.Fatal("intervening-if first-main trigger must be out of scope")
	}
}
