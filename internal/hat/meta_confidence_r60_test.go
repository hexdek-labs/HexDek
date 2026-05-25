package hat

// R60 round 5 — "I've been wrong before" confidence decay. Builds on
// the perceived-archetype confidence curve (#307) by adding fast-
// decay penalties when an observed cast CONTRADICTS the current
// classification. Pre-fix the only decay path was `Confidence *=
// 0.95` when the candidate went "unknown" — a 5% drop per turn,
// equal to the ramp rate. So once the hat snapped to a wrong call
// (e.g., flagged a control deck as combo on the back of one early
// tutor) the bias would persist for many turns even as the opponent
// kept playing CONTROL-shaped cards.
//
// Fix: when a cast event contradicts the classification (combo
// casting Wrath, aggro casting Counterspell, control slamming a
// 5+ CMC creature), subtract contradictDecayStep (0.15) from
// Confidence — 3x the ramp rate, so a single contradiction undoes
// three turns of stickiness. Per-turn cap at contradictMaxPenaltyPerTurn
// (0.30) so a scripted multi-spell turn doesn't snap to zero.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// isMassRemovalText classifier
// ---------------------------------------------------------------------

func TestIsMassRemovalText_PositiveCases(t *testing.T) {
	cases := []string{
		"destroy all creatures.",
		"exile all permanents.",
		"~ deals 4 damage to each creature.",
		"~ deals 5 damage to each opponent.",
		"all creatures get -2/-2 until end of turn.",
		"creatures you don't control get -3/-3.",
	}
	for _, ot := range cases {
		if !isMassRemovalText(ot) {
			t.Errorf("oracle %q must classify as mass removal", ot)
		}
	}
}

func TestIsMassRemovalText_NegativeCases(t *testing.T) {
	cases := []string{
		"destroy target creature.",
		"exile target permanent.",
		"~ deals 3 damage to target creature.",
		"target creature gets -2/-2.",
		"counter target spell.",
		"draw a card.",
		"",
	}
	for _, ot := range cases {
		if isMassRemovalText(ot) {
			t.Errorf("oracle %q must NOT classify as mass removal", ot)
		}
	}
}

// ---------------------------------------------------------------------
// contradictsArchetype dispatcher
// ---------------------------------------------------------------------

func makeOracleCard(name string, types []string, oracle string, cmc int) *gameengine.Card {
	c := &gameengine.Card{
		AST:   &gameast.CardAST{Name: name},
		Name:  name,
		Types: append([]string{}, types...),
		CMC:   cmc,
	}
	if oracle != "" {
		c.Types = append(c.Types, "oracle:"+oracle)
	}
	return c
}

func TestContradictsArchetype_Combo_WrathContradicts(t *testing.T) {
	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	if !contradictsArchetype("combo", wrath) {
		t.Error("combo + Wrath cast must contradict (combo decks don't run mass removal)")
	}
}

func TestContradictsArchetype_Combo_TutorIsCompatible(t *testing.T) {
	tutor := makeOracleCard("Demonic Tutor", []string{"sorcery"}, "search your library for a card.", 2)
	if contradictsArchetype("combo", tutor) {
		t.Error("combo + tutor must NOT contradict (tutors are combo's core line)")
	}
}

func TestContradictsArchetype_Aggro_CounterspellContradicts(t *testing.T) {
	counter := makeOracleCard("Counterspell", []string{"instant"}, "counter target spell.", 2)
	if !contradictsArchetype("aggro", counter) {
		t.Error("aggro + Counterspell cast must contradict (aggro decks pressure, not respond)")
	}
}

func TestContradictsArchetype_Aggro_WrathContradicts(t *testing.T) {
	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	if !contradictsArchetype("aggro", wrath) {
		t.Error("aggro + Wrath cast must contradict (aggro doesn't undo its own board)")
	}
}

func TestContradictsArchetype_Aggro_BurnSpellIsCompatible(t *testing.T) {
	burn := makeOracleCard("Lightning Bolt", []string{"instant"}, "~ deals 3 damage to any target.", 1)
	if contradictsArchetype("aggro", burn) {
		t.Error("aggro + single-target burn must NOT contradict")
	}
}

func TestContradictsArchetype_Control_BigCreatureContradicts(t *testing.T) {
	big := makeOracleCard("Titan", []string{"creature"}, "", 6)
	if !contradictsArchetype("control", big) {
		t.Error("control + 6 CMC creature must contradict (control plays utility / finishers, not slam-and-swing)")
	}
}

func TestContradictsArchetype_Control_SmallCreatureCompatible(t *testing.T) {
	utility := makeOracleCard("Snapcaster Mage", []string{"creature"}, "", 2)
	if contradictsArchetype("control", utility) {
		t.Error("control + 2 CMC utility creature must NOT contradict (Snapcaster is a control staple)")
	}
}

func TestContradictsArchetype_Control_CounterspellCompatible(t *testing.T) {
	counter := makeOracleCard("Counterspell", []string{"instant"}, "counter target spell.", 2)
	if contradictsArchetype("control", counter) {
		t.Error("control + Counterspell must NOT contradict")
	}
}

func TestContradictsArchetype_UnknownAndMidrangeAreNoOp(t *testing.T) {
	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	if contradictsArchetype("", wrath) {
		t.Error("empty archetype must never contradict")
	}
	if contradictsArchetype("unknown", wrath) {
		t.Error("unknown archetype must never contradict (nothing to compare against)")
	}
	if contradictsArchetype("midrange", wrath) {
		t.Error("midrange archetype must never contradict (catch-all, no sharp non-plays)")
	}
}

func TestContradictsArchetype_NilCardIsSafe(t *testing.T) {
	if contradictsArchetype("combo", nil) {
		t.Error("nil card must never contradict (no info)")
	}
}

// ---------------------------------------------------------------------
// cardCMC helper
// ---------------------------------------------------------------------

func TestCardCMC_FieldWins(t *testing.T) {
	c := &gameengine.Card{Name: "X", CMC: 4, Types: []string{"creature", "cost:7"}}
	if got := cardCMC(c); got != 4 {
		t.Errorf("CMC field must win over cost: prefix; got %d", got)
	}
}

func TestCardCMC_CostPrefixFallback(t *testing.T) {
	c := &gameengine.Card{Name: "X", Types: []string{"creature", "cost:7"}}
	if got := cardCMC(c); got != 7 {
		t.Errorf("cost:7 must parse as 7; got %d", got)
	}
}

func TestCardCMC_Zero(t *testing.T) {
	if cardCMC(nil) != 0 {
		t.Error("nil card must yield CMC 0")
	}
	c := &gameengine.Card{Name: "Land", Types: []string{"land"}}
	if cardCMC(c) != 0 {
		t.Error("card without CMC or cost: prefix must yield 0")
	}
}

// ---------------------------------------------------------------------
// End-to-end: contradiction observed → confidence drops faster than ramp
// ---------------------------------------------------------------------

func TestContradictionDecay_ComboPlusWrathDropsConfidence(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Establish a high-confidence combo classification by hand.
	h.opponentProfiles[1] = &OpponentProfile{
		Archetype:   "combo",
		Confidence:  0.80,
		TutorsUsed:  2,
		stableTurns: 4,
	}
	confBefore := h.opponentProfiles[1].Confidence

	// Opponent casts Wrath of God — contradicts combo.
	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	h.recordOpponentPlay("cast", wrath.Name, 1, wrath, 5)

	confAfter := h.opponentProfiles[1].Confidence

	drop := confBefore - confAfter
	if drop < contradictDecayStep-0.001 {
		t.Errorf("Wrath-on-combo must drop confidence by >= %.3f; got drop=%.3f", contradictDecayStep, drop)
	}
	if h.opponentProfiles[1].Contradictions != 1 {
		t.Errorf("contradiction counter must increment; got %d", h.opponentProfiles[1].Contradictions)
	}
}

// Decay rate must STRICTLY exceed the ramp rate (the directive's
// headline requirement: "lower confidence faster than ramping up").
func TestContradictionDecay_FasterThanRamp(t *testing.T) {
	const rampPerTurn = 0.05 // from classifyOpponent: +0.05 * stableTurns
	if contradictDecayStep <= rampPerTurn {
		t.Fatalf("contradictDecayStep (%.3f) must exceed rampPerTurn (%.3f) — directive requires faster decay than ramp",
			contradictDecayStep, rampPerTurn)
	}
	// Concretely: 3x the ramp rate.
	if contradictDecayStep < rampPerTurn*3-0.001 {
		t.Errorf("contradictDecayStep (%.3f) should be at least 3x rampPerTurn (%.3f) — single contradiction should undo multiple ramp turns",
			contradictDecayStep, rampPerTurn*3)
	}
}

// A SINGLE contradiction must not snap confidence to zero — the
// classifier is reactive, not panicky. Multiple contradictions can
// stack but the per-turn cap protects against scripted bursts.
func TestContradictionDecay_SingleContradictionDoesNotZero(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	h.opponentProfiles[1] = &OpponentProfile{
		Archetype:  "combo",
		Confidence: 0.80,
	}

	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	h.recordOpponentPlay("cast", wrath.Name, 1, wrath, 5)

	if h.opponentProfiles[1].Confidence <= 0 {
		t.Errorf("single contradiction must not zero confidence; got %.3f", h.opponentProfiles[1].Confidence)
	}
}

// Confidence floors at 0 — never goes negative.
func TestContradictionDecay_FloorsAtZero(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	h.opponentProfiles[1] = &OpponentProfile{
		Archetype:  "combo",
		Confidence: 0.05, // very low — one contradiction will overshoot
	}

	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	h.recordOpponentPlay("cast", wrath.Name, 1, wrath, 5)

	if h.opponentProfiles[1].Confidence < 0 {
		t.Errorf("confidence must floor at 0; got %.3f", h.opponentProfiles[1].Confidence)
	}
}

// Compatible casts do NOT trigger decay.
func TestContradictionDecay_CompatibleCastsDoNotDecay(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	h.opponentProfiles[1] = &OpponentProfile{
		Archetype:  "combo",
		Confidence: 0.80,
	}

	tutor := makeOracleCard("Demonic Tutor", []string{"sorcery"}, "search your library for a card.", 2)
	h.recordOpponentPlay("cast", tutor.Name, 1, tutor, 5)

	if h.opponentProfiles[1].Confidence < 0.80 {
		t.Errorf("compatible cast (combo + tutor) must not decay confidence; got %.3f",
			h.opponentProfiles[1].Confidence)
	}
	if h.opponentProfiles[1].Contradictions != 0 {
		t.Errorf("compatible cast must not increment contradiction counter; got %d",
			h.opponentProfiles[1].Contradictions)
	}
}

// Switching archetypes resets the contradiction counter — the prior
// contradictions actually pointed AT the new archetype, they don't
// continue to count against it.
func TestContradictionDecay_ResetsOnArchetypeSwitch(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	prof := &OpponentProfile{
		Archetype:      "combo",
		Confidence:     0.50,
		Contradictions: 3,
		stableTurns:    2,
		// Stats that would trigger a "control" reclassification.
		RemovalUsed:     2,
		CountersUsed:    2,
		CreaturesPlayed: 1,
	}
	h.opponentProfiles[1] = prof

	gs.Turn = 6
	got := h.classifyOpponent(gs, 1)

	if got.Archetype != "control" {
		t.Fatalf("setup expected reclassification to control; got %q", got.Archetype)
	}
	if got.Contradictions != 0 {
		t.Errorf("contradiction counter must reset on archetype switch; got %d", got.Contradictions)
	}
}

// End-to-end scenario: hat thought combo (high confidence), opponent
// casts Wrath, classifier next pass either reclassifies to control
// OR keeps combo but with much lower confidence. Either way the
// hat's archetype-driven bias should weaken meaningfully.
func TestContradictionDecay_BiasMultiplierDropsAfterContradiction(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	h.opponentProfiles[1] = &OpponentProfile{
		Archetype:  "combo",
		Confidence: 0.85, // super-linear bias zone
	}
	confBefore := h.opponentProfiles[1].Confidence
	multBefore := archetypeBiasMultiplier(confBefore)

	wrath := makeOracleCard("Wrath of God", []string{"sorcery"}, "destroy all creatures.", 4)
	h.recordOpponentPlay("cast", wrath.Name, 1, wrath, 5)

	confAfter := h.opponentProfiles[1].Confidence
	multAfter := archetypeBiasMultiplier(confAfter)

	if multAfter >= multBefore {
		t.Errorf("bias multiplier must drop after a contradicting cast: before=%.3f after=%.3f",
			multBefore, multAfter)
	}
	// And specifically: drop should pull us out of the super-linear
	// zone or noticeably down within it.
	if multBefore-multAfter < 0.15 {
		t.Errorf("multiplier drop should be meaningful (≥ ramp-rate equivalent at the curve); got drop=%.3f",
			multBefore-multAfter)
	}
}
