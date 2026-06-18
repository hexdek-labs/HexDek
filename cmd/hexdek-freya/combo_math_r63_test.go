package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// comboInBucket reports whether any combo in `bucket` contains all the named
// cards.
func comboInBucket(bucket []ComboResult, names ...string) bool {
	for _, c := range bucket {
		all := true
		for _, n := range names {
			if !comboContains(c.Cards, n) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// R63 Phase 1 — a known infinite combo (self-contained loop) lands in COMBO.
func TestComboMath_KnownInfiniteIsCombo(t *testing.T) {
	ballista := ClassifyCard("Walking Ballista",
		"Walking Ballista enters the battlefield with X +1/+1 counters on it.\n{4}: Put a +1/+1 counter on Walking Ballista.\nRemove a +1/+1 counter from Walking Ballista: It deals 1 damage to any target.",
		"Artifact Creature — Construct", "{0}", 0, "0")
	heliod := ClassifyCard("Heliod, Sun-Crowned",
		"Indestructible\nAs long as you have devotion to white of five or more, Heliod, Sun-Crowned is a creature.\nWhenever you gain life, put a +1/+1 counter on target creature or enchantment you control.\n{1}{W}: Another target creature or enchantment you control gains lifelink until end of turn.",
		"Legendary Enchantment Creature — God", "{2}{W}", 3, "5")

	report := AnalyzeDeck([]CardProfile{ballista, heliod}, "ballista+heliod", "", "")
	if !comboInBucket(report.Combos, "Walking Ballista", "Heliod, Sun-Crowned") {
		t.Fatalf("known infinite must be tagged COMBO; Combos=%+v", report.Combos)
	}
}

// R63 Phase 1 — two compounding-math pieces assembled (a doubler + a for-each
// scaler) form a COMBO even though they don't make a resource loop.
func TestComboMath_DoublerPlusForEachIsCombo(t *testing.T) {
	doubler := ClassifyCard("Doubling Season",
		"If an effect would create one or more tokens under your control, it creates twice that many of those tokens instead.\nIf an effect would put one or more counters on a permanent you control, it puts twice that many of those counters on that permanent instead.",
		"Enchantment", "{5}{G}", 6, "")
	forEach := ClassifyCard("Verdant Embrace Engine",
		"At the beginning of your end step, create a 1/1 green Saproling creature token for each creature you control.",
		"Enchantment", "{4}{G}", 5, "")

	if len(doubler.MathOps) == 0 {
		t.Fatalf("doubler must carry an oracle MathOp; got none")
	}
	if len(forEach.MathOps) == 0 {
		t.Fatalf("for-each engine must carry an oracle MathOp; got none")
	}

	report := AnalyzeDeck([]CardProfile{doubler, forEach}, "math-assembly", "", "")
	if !comboInBucket(report.Combos, "Doubling Season", "Verdant Embrace Engine") {
		t.Fatalf("doubler + for-each must be tagged COMBO; Combos=%+v", report.Combos)
	}
}

// R63 Phase 1 — an enabler (sacrifice outlet) feeding a compounding-math
// payoff is promoted to COMBO (Q4).
func TestComboMath_EnablerFeedsMathPayoffIsCombo(t *testing.T) {
	outlet := ClassifyCard("Viscera Seer",
		"Sacrifice a creature: Scry 1.",
		"Creature — Vampire Wizard", "{B}", 1, "1")
	payoff := ClassifyCard("Blood Tithe Engine",
		"Whenever a creature you control dies, this deals damage to each opponent equal to the number of creatures you control.",
		"Enchantment", "{2}{B}", 3, "")

	if !outlet.IsOutlet {
		t.Fatalf("sacrifice outlet must set IsOutlet")
	}
	if len(payoff.MathOps) == 0 {
		t.Fatalf("for-each payoff must carry a MathOp")
	}

	report := AnalyzeDeck([]CardProfile{outlet, payoff}, "enabler+payoff", "", "")
	if !comboInBucket(report.Combos, "Viscera Seer", "Blood Tithe Engine") {
		t.Fatalf("enabler→math-payoff must be tagged COMBO; Combos=%+v", report.Combos)
	}
}

// R63 Phase 1 — the canonical false positive must NOT be a combo. Neither card
// carries compounding math; the pair is mere card advantage.
func TestComboMath_EvisceratorOrchardNotCombo(t *testing.T) {
	eviscerator := ClassifyCard("Eviscerator's Insight",
		"As an additional cost to cast this spell, sacrifice an artifact or creature.\nDraw two cards.\nFlashback {4}{B}",
		"Instant", "{2}{B}", 3, "")
	orchard := ClassifyCard("Orchard Strider",
		"When Orchard Strider enters, create two Food tokens.\nLandcycling {1}{G}",
		"Creature — Spider", "{4}{G}", 5, "3")

	if len(eviscerator.MathOps) != 0 || len(orchard.MathOps) != 0 {
		t.Fatalf("neither card should carry compounding math: ev=%+v orchard=%+v",
			eviscerator.MathOps, orchard.MathOps)
	}

	report := AnalyzeDeck([]CardProfile{eviscerator, orchard}, "ev+orchard", "", "")
	if comboInBucket(report.Combos, "Eviscerator's Insight", "Orchard Strider") {
		t.Fatalf("Eviscerator's Insight + Orchard Strider must NOT be a combo; Combos=%+v",
			report.Combos)
	}
}

// fakeASTSource is a fixture mathASTSource for exercising the AST seam without
// the 46 MB ast_dataset.jsonl.
type fakeASTSource map[string]*gameast.CardAST

func (f fakeASTSource) Get(name string) (*gameast.CardAST, bool) {
	c, ok := f[name]
	return c, ok
}

// R63 Phase 1 — the AST seam: a card whose oracle text doesn't trip the
// conservative oracle parser is still detected as a math piece via the Thor
// AST (CounterMod Op=="double" and a compounding ScalingAmount).
func TestComboMath_ASTSourceDetectsMath(t *testing.T) {
	t.Cleanup(func() { SetMathASTSource(nil) })

	// A doubler expressed only structurally (CounterMod double), with oracle
	// text the oracle parser deliberately won't match.
	doublerAST := &gameast.CardAST{
		Name: "Structural Doubler",
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "upkeep"},
				Effect: &gameast.CounterMod{
					Op:          "double",
					CounterKind: "+1/+1",
				},
			},
		},
	}
	// A for-each scaler expressed only structurally (ScalingAmount).
	scalerAST := &gameast.CardAST{
		Name: "Structural Scaler",
		Abilities: []gameast.Ability{
			&gameast.Triggered{
				Trigger: gameast.Trigger{Event: "etb"},
				Effect: &gameast.Damage{
					Amount: *gameast.NumScaling(&gameast.ScalingAmount{
						ScalingKind: "creatures_you_control",
					}),
				},
			},
		},
	}
	SetMathASTSource(fakeASTSource{
		"Structural Doubler": doublerAST,
		"Structural Scaler":  scalerAST,
	})

	doubler := ClassifyCard("Structural Doubler", "Does something each upkeep.", "Enchantment", "{3}", 3, "")
	scaler := ClassifyCard("Structural Scaler", "An effect when it enters.", "Creature — Elemental", "{3}{R}", 4, "2")

	// Oracle parser saw nothing; AST must supply the math.
	if len(doubler.MathOps) != 0 || len(scaler.MathOps) != 0 {
		t.Fatalf("oracle parser should not have matched these fixtures")
	}
	if !buildMathSignature(doubler).HasCompoundingMath() {
		t.Fatalf("AST CounterMod double must register as compounding math")
	}
	if !buildMathSignature(scaler).HasCompoundingMath() {
		t.Fatalf("AST compounding ScalingAmount must register as compounding math")
	}

	report := AnalyzeDeck([]CardProfile{doubler, scaler}, "ast-math", "", "")
	if !comboInBucket(report.Combos, "Structural Doubler", "Structural Scaler") {
		t.Fatalf("two AST-detected math pieces must be tagged COMBO; Combos=%+v", report.Combos)
	}
}
