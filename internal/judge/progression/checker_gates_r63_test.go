package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// checker_gates_r63_test.go — pins the r63 PROGRESSION oracle gates that
// removed false positives where the ENGINE is correct but the single-
// snapshot scenario cannot soundly model the effect.

// Board-scaling ally-ETB effects ("each creature you control", "creatures
// you control get") scale with the post-entry board (bearer + entering
// ally = 2 creatures); the single-application expectation models 1, so the
// ally checker must skip them (Cathars' Crusade, Goldnight Commander,
// Valor in Akros — the engine correctly affects 2).
func TestAllyETB_GatesBoardScalingEffects(t *testing.T) {
	cases := []string{
		"whenever a creature you control enters, put a +1/+1 counter on each creature you control",
		"whenever another creature you control enters, creatures you control get +1/+1 until end of turn",
		"whenever a creature you control enters, creatures you control get +1/+1 until end of turn",
	}
	for _, raw := range cases {
		tr := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: "tribe_you_control_etb"},
			Effect: &gameast.Buff{Power: 1, Toughness: 1,
				Target: gameast.Filter{Base: "creature", YouControl: true}},
			Raw: raw,
		}
		if _, ran := CheckAllyETBTrigger("Test Anthem", tr); ran {
			t.Errorf("board-scaling ally effect must be out of scope: %q", raw)
		}
	}
}

// "put its counters on target …" moves whatever counters the source had —
// the bare scenario seeds none, and the parser models "its" as a counter
// KIND. The self-LTB-type checker must skip it (Broodguard Elite).
func TestSelfLTBType_GatesItsCounters(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "type_leaves_battlefield"},
		Effect:  &gameast.CounterMod{Op: "move", CounterKind: "its", Target: gameast.Filter{Base: "creature", YouControl: true, Targeted: true}},
		Raw:     "when this creature leaves the battlefield, put its counters on target creature you control",
	}
	if InScopeSelfLTBTypeTrigger(tr) {
		t.Fatal(`"put its counters" LTB trigger must be out of scope`)
	}
}

// Self-recursion die triggers ("when ~ dies, return this card to your
// hand") return the source OUT of the graveyard; the bare-death
// subtraction cannot compose, so the die checker must skip them
// (Michelangelo, On the Scene). The engine itself returns the card
// correctly (gameengine TestDieReturnSelf_GraveyardToHand).
func TestDieChecker_GatesSelfReturn(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "die"},
		Effect:  &gameast.Bounce{Target: gameast.Filter{Base: "self"}, To: "owners_hand"},
		Raw:     "when this creature dies, return this card to your hand",
	}
	if _, ran := CheckTrigger("Self Returner", tr); ran {
		t.Fatal("self-return die trigger must be out of scope")
	}
}
