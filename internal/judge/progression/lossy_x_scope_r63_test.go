package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// PROGRESSION etb-fire r63 (firespot): the parser flattens X-scaling effects
// ("create X tokens, where X is the number of creatures you control", "draw a
// card for each …", "deal X damage to each creature") to a bare count="x"
// NumberOrRef, dropping the scaling basis. The OUTCOME interpreter's amountVal
// then pins "x" to the fixed spec.XValue, so the expected board state can never
// match the engine's actual output (the engine under-produces these — it makes
// the count-1 default regardless of the real board count). Those (card,
// trigger) pairs are unmodelable from the lossy AST, so CheckAny must mark them
// OUT OF SCOPE rather than emit a miscalibrated finding.

func etbTriggered(raw string, eff gameast.Effect) *gameast.Triggered {
	return &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "etb"},
		Effect:  eff,
		Raw:     raw,
	}
}

func TestLossyX_ScalingTokenETB_OutOfScope(t *testing.T) {
	// Chancellor-of-the-Forge shape: create X tokens where X is a board count.
	tr := etbTriggered(
		"when this creature enters, create x 1/1 red goblin creature tokens, where x is the number of creatures you control",
		&gameast.CreateToken{
			Count: gameast.NumberOrRef{IsStr: true, Str: "x"},
			PT:    &[2]int{1, 1},
			Types: []string{"red", "goblin"},
		})
	if _, ran := CheckAny("Scaling Forge", tr); ran {
		t.Fatal("an X-scaling 'create X tokens where X is …' ETB must be OUT OF SCOPE (lossy count='x'); it ran")
	}
}

func TestLossyX_DamageToEachX_OutOfScope(t *testing.T) {
	// Spiteful-Banditry shape: deal X damage to each creature (fan-out X).
	tr := etbTriggered(
		"when this enchantment enters, it deals x damage to each creature",
		&gameast.Damage{
			Amount: gameast.NumberOrRef{IsStr: true, Str: "x"},
			Target: gameast.Filter{Base: "creature", Quantifier: "each"},
		})
	if _, ran := CheckAny("Scaling Blast", tr); ran {
		t.Fatal("an X-scaling 'deal X damage to each creature' ETB must be OUT OF SCOPE; it ran")
	}
}

func TestLossyX_FixedEffectStillInScope(t *testing.T) {
	// Control: a FIXED-count ETB token (no X) must still be checked — the
	// gate must not over-suppress ordinary effects.
	tr := etbTriggered(
		"when this creature enters, create a 1/1 white soldier creature token",
		&gameast.CreateToken{
			Count: gameast.NumberOrRef{IsInt: true, Int: 1},
			PT:    &[2]int{1, 1},
			Types: []string{"white", "soldier"},
		})
	if _, ran := CheckAny("Fixed Maker", tr); !ran {
		t.Fatal("a fixed-count ETB token effect must remain IN SCOPE; the gate over-suppressed it")
	}
}
