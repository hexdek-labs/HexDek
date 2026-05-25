package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 GraveyardValue rewrite — covers the four new facets layered on
// top of the pre-r60 per-card flat-bonus implementation: (1) recursion-
// keyword value scaled by CMC, (2) reanimator-engine ramp on
// high-CMC creatures in graveyard, (3) delirium gate (4+ distinct
// card types), (4) threshold gate (7+ cards). Each facet has a
// counterfactual sibling so the test fails loudly if the bonus is
// reintroduced as a flat per-card addition or as an always-on scalar.

// ---------------------------------------------------------------------
// Test helpers — graveyard-scoped wrappers around hat_test.go's
// newTestGame / newTestCardMinimal.
// ---------------------------------------------------------------------

func gyCardWithKeyword(name string, types []string, cmc int, keyword string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	if keyword != "" {
		ast.Abilities = append(ast.Abilities, &gameast.Keyword{Name: keyword})
	}
	return newTestCardMinimal(name, types, cmc, ast)
}

func gyCardWithOracle(name string, types []string, cmc int, oracleSnippet string) *gameengine.Card {
	ast := &gameast.CardAST{Name: name}
	ast.Abilities = append(ast.Abilities, &gameast.Static{Raw: oracleSnippet})
	return newTestCardMinimal(name, types, cmc, ast)
}

// gravyardValueFor pulls just the GraveyardValue raw dimension out of
// EvaluateDetailed — keeps test bodies focused on what we're tuning.
func graveyardValueFor(t *testing.T, gs *gameengine.GameState, seatIdx int) float64 {
	t.Helper()
	ev := NewEvaluator(nil)
	return ev.EvaluateDetailed(gs, seatIdx).GraveyardValue
}

// ---------------------------------------------------------------------
// Facet 1 — recursion-keyword value scales with CMC
// ---------------------------------------------------------------------

func TestGraveyardR60_RecursionValueScalesWithCMC(t *testing.T) {
	cheap := newTestGame(t, 2)
	cheap.Seats[0].Graveyard = append(cheap.Seats[0].Graveyard,
		gyCardWithKeyword("Faithless Looting", []string{"sorcery"}, 1, "flashback"))
	cheapScore := graveyardValueFor(t, cheap, 0)

	expensive := newTestGame(t, 2)
	expensive.Seats[0].Graveyard = append(expensive.Seats[0].Graveyard,
		gyCardWithKeyword("Cataclysm", []string{"sorcery"}, 6, "flashback"))
	expensiveScore := graveyardValueFor(t, expensive, 0)

	if expensiveScore <= cheapScore {
		t.Errorf("CMC scaling: 6-mana flashback bomb should score higher than 1-mana cantrip; cheap=%.3f expensive=%.3f", cheapScore, expensiveScore)
	}
	// Saturation guard: 6-mana flashback bomb (max single-card contribution
	// 0.35 by design) should NOT be reading as 6x the 1-mana cantrip.
	if expensiveScore > cheapScore*5 {
		t.Errorf("CMC scaling unbounded: 6-mana card scored %.3fx the 1-mana card (want ≤5x via the 0.35 ceiling)", expensiveScore/cheapScore)
	}
}

func TestGraveyardR60_RecursionValueRecognizesPostAKHKeywords(t *testing.T) {
	// Pre-r60 only recognized flashback / escape / unearth / retrace.
	// r60 extends to: disturb, aftermath, embalm, eternalize, encore,
	// jump-start. Each must register some positive recursion value
	// when present.
	cases := []struct {
		name    string
		keyword string
		types   []string
	}{
		{"flashback baseline", "flashback", []string{"sorcery"}},
		{"escape", "escape", []string{"instant"}},
		{"unearth", "unearth", []string{"creature"}},
		{"retrace", "retrace", []string{"sorcery"}},
		{"disturb", "disturb", []string{"creature"}},
		{"aftermath", "aftermath", []string{"sorcery"}},
		{"embalm", "embalm", []string{"creature"}},
		{"eternalize", "eternalize", []string{"creature"}},
		{"encore", "encore", []string{"creature"}},
		{"jump-start", "jump-start", []string{"instant"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := newTestGame(t, 2)
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
				gyCardWithKeyword(tc.name, tc.types, 2, tc.keyword))
			withKw := graveyardValueFor(t, gs, 0)

			baseline := newTestGame(t, 2)
			baseline.Seats[0].Graveyard = append(baseline.Seats[0].Graveyard,
				gyCardWithKeyword(tc.name, tc.types, 2, ""))
			withoutKw := graveyardValueFor(t, baseline, 0)

			if withKw <= withoutKw {
				t.Errorf("keyword %q must register positive recursion value: with=%.3f without=%.3f", tc.keyword, withKw, withoutKw)
			}
		})
	}
}

// ---------------------------------------------------------------------
// Facet 2 — reanimator-target ramp
// ---------------------------------------------------------------------

func TestGraveyardR60_ReanimatorRamp_FiresOnlyWithEngine(t *testing.T) {
	// Same graveyard, two seats: one with Animate Dead on board, one
	// without. Animate Dead should make the Griselbrand-in-graveyard
	// MUCH more valuable.
	griselbrand := newTestCardMinimal("Griselbrand", []string{"creature"}, 8, nil)

	noEngine := newTestGame(t, 2)
	noEngine.Seats[0].Graveyard = append(noEngine.Seats[0].Graveyard, griselbrand)
	noEngineScore := graveyardValueFor(t, noEngine, 0)

	withEngine := newTestGame(t, 2)
	withEngine.Seats[0].Graveyard = append(withEngine.Seats[0].Graveyard, griselbrand)
	engineCard := gyCardWithOracle("Animate Dead", []string{"enchantment", "aura"}, 2,
		"return target creature card from a graveyard to the battlefield")
	newTestPermanent(withEngine.Seats[0], engineCard, 0, 0)
	withEngineScore := graveyardValueFor(t, withEngine, 0)

	if withEngineScore <= noEngineScore {
		t.Errorf("reanimator ramp must fire when engine is on board; no_engine=%.3f with_engine=%.3f", noEngineScore, withEngineScore)
	}
	// The ramp adds up to 0.40 for an 8-CMC creature (cmc*0.08 = 0.64,
	// clamped to 0.40). Expect a meaningful delta — at least 0.25 — so
	// the test fails if the engine bonus is silently truncated.
	if withEngineScore-noEngineScore < 0.25 {
		t.Errorf("ramp delta too small: %.3f (want ≥0.25 for an 8-CMC creature)", withEngineScore-noEngineScore)
	}
}

func TestGraveyardR60_ReanimatorRamp_CommanderTriggers(t *testing.T) {
	// Meren as commander should trigger the ramp even without a
	// battlefield aura — the engine is the commander.
	griselbrand := newTestCardMinimal("Griselbrand", []string{"creature"}, 8, nil)

	gs := newTestGame(t, 2)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, griselbrand)
	gs.Seats[0].CommanderNames = []string{"Meren of Clan Nel Toth"}
	withMeren := graveyardValueFor(t, gs, 0)

	baseline := newTestGame(t, 2)
	baseline.Seats[0].Graveyard = append(baseline.Seats[0].Graveyard, griselbrand)
	baseline.Seats[0].CommanderNames = []string{"Vanilla Generic Commander"}
	noEngine := graveyardValueFor(t, baseline, 0)

	if withMeren <= noEngine {
		t.Errorf("Meren commander must trigger ramp; with=%.3f without=%.3f", withMeren, noEngine)
	}
}

func TestGraveyardR60_ReanimatorRamp_HighCMCScalesMoreThanLow(t *testing.T) {
	// With the engine on board, a Griselbrand (8 CMC) should ramp more
	// than a Bear Cub (2 CMC). Tests the cmc*0.08 scaling.
	mkGame := func(creatureCMC int) float64 {
		gs := newTestGame(t, 2)
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Body", []string{"creature"}, creatureCMC, nil))
		engineCard := gyCardWithOracle("Animate Dead", []string{"enchantment"}, 2,
			"return target creature card from a graveyard to the battlefield")
		newTestPermanent(gs.Seats[0], engineCard, 0, 0)
		return graveyardValueFor(t, gs, 0)
	}
	low := mkGame(2)
	high := mkGame(8)
	if high <= low {
		t.Errorf("high-CMC ramp should exceed low-CMC ramp; cmc=2 → %.3f, cmc=8 → %.3f", low, high)
	}
}

// ---------------------------------------------------------------------
// Facet 3 — delirium gate (4+ distinct card types)
// ---------------------------------------------------------------------

func TestGraveyardR60_DeliriumGate_ActivatesAtFourTypes(t *testing.T) {
	// 3 distinct types → no delirium bonus. 4 distinct types → bonus
	// kicks in. The same 4 cards counted with only 3 distinct types
	// should NOT trigger the bonus, even at the same graveyard size.
	threeTypes := newTestGame(t, 2)
	threeTypes.Seats[0].Graveyard = []*gameengine.Card{
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil),
		newTestCardMinimal("Bolt", []string{"instant"}, 1, nil),
		newTestCardMinimal("Path", []string{"sorcery"}, 1, nil),
		newTestCardMinimal("Path 2", []string{"sorcery"}, 1, nil),
	}
	threeScore := graveyardValueFor(t, threeTypes, 0)

	fourTypes := newTestGame(t, 2)
	fourTypes.Seats[0].Graveyard = []*gameengine.Card{
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil),
		newTestCardMinimal("Bolt", []string{"instant"}, 1, nil),
		newTestCardMinimal("Path", []string{"sorcery"}, 1, nil),
		newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil),
	}
	fourScore := graveyardValueFor(t, fourTypes, 0)

	if fourScore <= threeScore {
		t.Errorf("delirium gate must fire at 4 types; 3-type=%.3f 4-type=%.3f", threeScore, fourScore)
	}
	// The delirium base is 0.40 — expect at least 0.30 delta after
	// netting out the per-card value swap (creature 0.15 vs artifact 0).
	if fourScore-threeScore < 0.30 {
		t.Errorf("delirium gate too quiet: delta %.3f (want ≥0.30)", fourScore-threeScore)
	}
}

func TestGraveyardR60_DeliriumGate_PayoffAmplifies(t *testing.T) {
	// With a delirium payoff on the battlefield, the 4-type bonus
	// should scale 1.5x.
	base := newTestGame(t, 2)
	base.Seats[0].Graveyard = []*gameengine.Card{
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil),
		newTestCardMinimal("Bolt", []string{"instant"}, 1, nil),
		newTestCardMinimal("Path", []string{"sorcery"}, 1, nil),
		newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil),
	}
	baseScore := graveyardValueFor(t, base, 0)

	withPayoff := newTestGame(t, 2)
	withPayoff.Seats[0].Graveyard = []*gameengine.Card{
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil),
		newTestCardMinimal("Bolt", []string{"instant"}, 1, nil),
		newTestCardMinimal("Path", []string{"sorcery"}, 1, nil),
		newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil),
	}
	payoff := gyCardWithOracle("Liliana, the Last Hope", []string{"planeswalker"}, 3,
		"delirium — return target creature card from your graveyard")
	newTestPermanent(withPayoff.Seats[0], payoff, 0, 0)
	withPayoffScore := graveyardValueFor(t, withPayoff, 0)

	if withPayoffScore <= baseScore {
		t.Errorf("delirium payoff must amplify the bonus; base=%.3f with_payoff=%.3f", baseScore, withPayoffScore)
	}
}

// ---------------------------------------------------------------------
// Facet 4 — threshold gate (7+ cards in graveyard)
// ---------------------------------------------------------------------

func TestGraveyardR60_ThresholdGate_ActivatesAtSevenCards(t *testing.T) {
	// 6 cards → no threshold bonus. 7 cards → bonus kicks in. We hold
	// the type set constant (all sorceries) so the delirium gate
	// doesn't muddy the comparison.
	sorcery := func() *gameengine.Card { return newTestCardMinimal("Bolt", []string{"sorcery"}, 1, nil) }

	six := newTestGame(t, 2)
	for i := 0; i < 6; i++ {
		six.Seats[0].Graveyard = append(six.Seats[0].Graveyard, sorcery())
	}
	sixScore := graveyardValueFor(t, six, 0)

	seven := newTestGame(t, 2)
	for i := 0; i < 7; i++ {
		seven.Seats[0].Graveyard = append(seven.Seats[0].Graveyard, sorcery())
	}
	sevenScore := graveyardValueFor(t, seven, 0)

	// The cross-opponent delta also rises with graveyard size — but
	// only by 0.05 per card. The threshold bonus is 0.30 fixed, so
	// the 6→7 delta should be ≥ 0.30 (threshold base) + 0.05 (delta).
	delta := sevenScore - sixScore
	if delta < 0.30 {
		t.Errorf("threshold gate must fire at 7 cards: 6-card=%.3f 7-card=%.3f delta=%.3f (want ≥0.30)", sixScore, sevenScore, delta)
	}
}

func TestGraveyardR60_ThresholdGate_PayoffAmplifies(t *testing.T) {
	sorcery := func() *gameengine.Card { return newTestCardMinimal("Bolt", []string{"sorcery"}, 1, nil) }

	base := newTestGame(t, 2)
	for i := 0; i < 7; i++ {
		base.Seats[0].Graveyard = append(base.Seats[0].Graveyard, sorcery())
	}
	baseScore := graveyardValueFor(t, base, 0)

	withPayoff := newTestGame(t, 2)
	for i := 0; i < 7; i++ {
		withPayoff.Seats[0].Graveyard = append(withPayoff.Seats[0].Graveyard, sorcery())
	}
	payoff := gyCardWithOracle("Werebear", []string{"creature"}, 2,
		"threshold — werebear gets +3/+3 as long as seven or more cards are in your graveyard")
	newTestPermanent(withPayoff.Seats[0], payoff, 0, 0)
	withPayoffScore := graveyardValueFor(t, withPayoff, 0)

	if withPayoffScore <= baseScore {
		t.Errorf("threshold payoff must amplify; base=%.3f with_payoff=%.3f", baseScore, withPayoffScore)
	}
}

// ---------------------------------------------------------------------
// Integration / saturation
// ---------------------------------------------------------------------

func TestGraveyardR60_EmptyGraveyardReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	if got := graveyardValueFor(t, gs, 0); got != 0 {
		t.Errorf("empty graveyard must score 0, got %.3f", got)
	}
}

func TestGraveyardR60_PreR60TestStillPasses(t *testing.T) {
	// Regression for the original TestEvaluator_GraveyardValue
	// assertion: 5 flashback cards still register positive value.
	gs := newTestGame(t, 2)
	for i := 0; i < 5; i++ {
		ast := &gameast.CardAST{Name: "Lingering Souls"}
		ast.Abilities = append(ast.Abilities, &gameast.Keyword{Name: "flashback"})
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			newTestCardMinimal("Lingering Souls", []string{"sorcery"}, 3, ast))
	}
	if got := graveyardValueFor(t, gs, 0); got <= 0 {
		t.Errorf("5 flashback cards must still register positive value, got %.3f", got)
	}
}

func TestGraveyardR60_HelperGates(t *testing.T) {
	// Direct exercise of the package-private helpers.
	if hasGraveyardCastKeyword("draw a card") {
		t.Errorf("draw-a-card oracle should NOT register as graveyard-castable")
	}
	if !hasGraveyardCastKeyword("flashback {2}{u}") {
		t.Errorf("flashback must register")
	}
	if !hasGraveyardCastKeyword("disturb {1}{w}") {
		t.Errorf("disturb must register")
	}

	seat := &gameengine.Seat{CommanderNames: []string{"Meren of Clan Nel Toth"}}
	if !seatHasReanimatorEngine(seat) {
		t.Errorf("Meren must register as a reanimator engine")
	}
	seat2 := &gameengine.Seat{CommanderNames: []string{"Generic Commander"}}
	if seatHasReanimatorEngine(seat2) {
		t.Errorf("generic commander must NOT register as a reanimator engine")
	}
}
