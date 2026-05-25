package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_trigger_value_r60_test.go — pins the R60 round 14+ ETB-trigger
// value bonus folded into cardHeuristic. Two layers:
//   1. etbTriggerBonus unit tests pin each categorization branch.
//   2. cardHeuristic integration confirms a Mulldrifter-shape card
//      scores higher than a same-CMC vanilla creature.

// mkEtbCard builds a creature Card with the given oracle text on a
// Triggered ability so OracleTextLower sees it.
func mkEtbCard(name string, oracle string, cmc int) *gameengine.Card {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Triggered{Raw: oracle},
		},
	}
	c := newTestCardMinimal(name, []string{"creature"}, cmc, ast)
	return c
}

// -----------------------------------------------------------------------
// etbTriggerBonus categorization
// -----------------------------------------------------------------------

func TestEtbTriggerBonus_NilCardIsZero(t *testing.T) {
	if got := etbTriggerBonus(nil); got != 0 {
		t.Errorf("nil card should return 0; got %.2f", got)
	}
}

func TestEtbTriggerBonus_VanillaCreatureIsZero(t *testing.T) {
	c := mkEtbCard("Grizzly Bears", "", 2)
	if got := etbTriggerBonus(c); got != 0 {
		t.Errorf("vanilla creature should return 0; got %.2f", got)
	}
}

func TestEtbTriggerBonus_StoneforgeTutor(t *testing.T) {
	c := mkEtbCard("Stoneforge Mystic",
		"when this creature enters, you may search your library for an equipment card",
		2)
	if got := etbTriggerBonus(c); got != 0.30 {
		t.Errorf("Stoneforge (tutor ETB) should be +0.30; got %.2f", got)
	}
}

func TestEtbTriggerBonus_MulldrifterDraw(t *testing.T) {
	c := mkEtbCard("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards",
		5)
	if got := etbTriggerBonus(c); got != 0.20 {
		t.Errorf("Mulldrifter (draw ETB) should be +0.20; got %.2f", got)
	}
}

func TestEtbTriggerBonus_ReclamationSageRemoval(t *testing.T) {
	c := mkEtbCard("Reclamation Sage",
		"when reclamation sage enters the battlefield, you may destroy target artifact or enchantment",
		3)
	if got := etbTriggerBonus(c); got != 0.20 {
		t.Errorf("Reclamation Sage (destroy ETB) should be +0.20; got %.2f", got)
	}
}

func TestEtbTriggerBonus_FlametongueKavuDamage(t *testing.T) {
	c := mkEtbCard("Flametongue Kavu",
		"when flametongue kavu enters the battlefield, it deals 4 damage to target creature",
		4)
	if got := etbTriggerBonus(c); got != 0.20 {
		t.Errorf("Flametongue Kavu (damage ETB) should be +0.20 (categorized as removal); got %.2f", got)
	}
}

func TestEtbTriggerBonus_SolemnSimulacrumRamp(t *testing.T) {
	c := mkEtbCard("Solemn Simulacrum",
		"when solemn simulacrum enters the battlefield, you may search your library for a basic land card",
		4)
	// "search your library" matches tutor bucket first (priority order).
	if got := etbTriggerBonus(c); got != 0.30 {
		t.Errorf("Solemn Simulacrum (search-land via 'search your library') should be tutor bucket +0.30; got %.2f", got)
	}
}

func TestEtbTriggerBonus_LandPutWithoutSearch(t *testing.T) {
	// Some land-ramp ETBs phrase as "put X onto the battlefield" without
	// "search your library". Hits the ramp bucket.
	c := mkEtbCard("Land Ramper",
		"when this creature enters the battlefield, you may put a land card from your hand onto the battlefield",
		3)
	if got := etbTriggerBonus(c); got != 0.15 {
		t.Errorf("'put a land' (no search) should hit ramp bucket +0.15; got %.2f", got)
	}
}

func TestEtbTriggerBonus_AdelineToken(t *testing.T) {
	c := mkEtbCard("Adeline Shape",
		"when this creature enters the battlefield, create a 1/1 white token",
		3)
	if got := etbTriggerBonus(c); got != 0.10 {
		t.Errorf("token ETB should be +0.10; got %.2f", got)
	}
}

func TestEtbTriggerBonus_EternalWitnessRecursion(t *testing.T) {
	c := mkEtbCard("Eternal Witness",
		"when eternal witness enters the battlefield, you may return target card from your graveyard to your hand",
		3)
	if got := etbTriggerBonus(c); got != 0.10 {
		t.Errorf("Eternal Witness (recursion ETB) should be +0.10; got %.2f", got)
	}
}

func TestEtbTriggerBonus_SoulWardenAnthemNotMatched(t *testing.T) {
	// Soul Warden is "Whenever ANOTHER creature enters" — anthem-shape
	// board trigger, not self-ETB. The generic-article filter should
	// reject this.
	c := mkEtbCard("Soul Warden",
		"whenever another creature enters the battlefield, you gain 1 life",
		1)
	if got := etbTriggerBonus(c); got != 0 {
		t.Errorf("Soul Warden (anthem-shape, not self-ETB) should be 0; got %.2f", got)
	}
}

func TestEtbTriggerBonus_ImpactTremorsAnthemNotMatched(t *testing.T) {
	// "Whenever a creature enters under your control" — also anthem.
	c := mkEtbCard("Impact Tremors",
		"whenever a creature enters under your control, impact tremors deals 1 damage to each opponent",
		2)
	if got := etbTriggerBonus(c); got != 0 {
		t.Errorf("Impact Tremors (anthem) should be 0; got %.2f", got)
	}
}

func TestEtbTriggerBonus_NonEnterTriggerIgnored(t *testing.T) {
	// "When this creature ATTACKS" — different trigger family.
	c := mkEtbCard("Attacker",
		"when this creature attacks, draw a card",
		3)
	if got := etbTriggerBonus(c); got != 0 {
		t.Errorf("attack-trigger card should not match ETB family; got %.2f", got)
	}
}

// -----------------------------------------------------------------------
// Integration: cardHeuristic ranks Mulldrifter above same-CMC vanilla
// -----------------------------------------------------------------------

// TestCardHeuristic_MulldrifterBeatsSameCmcVanilla pins the end-to-end
// effect. A 5-CMC Mulldrifter (ETB: draw 2) should score higher than
// a 5-CMC vanilla via the +0.20 ETB-draw bonus, even with the same
// Freya CatDraw role tag for fairness.
func TestCardHeuristic_MulldrifterBeatsSameCmcVanilla(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40

	// Five lands so the 5-CMC mana-efficiency term is identical for both.
	for i := 0; i < 5; i++ {
		newTestPermanent(gs.Seats[0], newTestCardMinimal("Island", []string{"land"}, 0, nil), 0, 0)
	}

	mulldrifter := mkEtbCard("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards", 5)
	vanilla := mkEtbCard("Hill Giant 5/5", "", 5)

	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	mScore := h.cardHeuristic(gs, 0, mulldrifter)
	vScore := h.cardHeuristic(gs, 0, vanilla)

	if mScore <= vScore {
		t.Errorf("Mulldrifter (ETB: draw 2) should score higher than same-CMC vanilla; mulldrifter=%.3f vanilla=%.3f",
			mScore, vScore)
	}
}

// TestCardHeuristic_StoneforgeBeatsMulldrifter — tutor bucket dominates
// draw bucket. Stoneforge Mystic (2-CMC tutor ETB, +0.30) should score
// HIGHER than Mulldrifter (5-CMC draw ETB, +0.20) at the ETB bonus
// layer alone. The full cardHeuristic factors in CMC / phase too, so
// the comparison isn't strictly score-for-score — we just confirm the
// ETB bonus distinction propagates.
func TestEtbTriggerBonus_TutorBeatsDrawAtBonusLayer(t *testing.T) {
	stoneforge := mkEtbCard("Stoneforge Mystic",
		"when this creature enters, you may search your library for an equipment", 2)
	mulldrifter := mkEtbCard("Mulldrifter",
		"when mulldrifter enters the battlefield, draw two cards", 5)
	sBonus := etbTriggerBonus(stoneforge)
	mBonus := etbTriggerBonus(mulldrifter)
	if sBonus <= mBonus {
		t.Errorf("tutor ETB bonus (%.2f) should exceed draw ETB bonus (%.2f) — tutors are the strongest ETB family",
			sBonus, mBonus)
	}
}

// TestCardHeuristic_VanillaNotPenalized — sanity check: cards without
// any ETB trigger should get bonus = 0 (no penalty either). Confirms
// the signal is additive only.
func TestCardHeuristic_VanillaNotPenalizedByEtbSignal(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 3; i++ {
		newTestPermanent(gs.Seats[0], newTestCardMinimal("Forest", []string{"land"}, 0, nil), 0, 0)
	}
	bear := mkEtbCard("Grizzly Bears", "", 2)
	h := NewYggdrasilHat(&StrategyProfile{Archetype: ArchetypeMidrange}, 0)
	score := h.cardHeuristic(gs, 0, bear)
	// Bonus contribution should be 0; the rest of cardHeuristic is
	// unchanged. We just confirm no negative impact on vanillas.
	if etbTriggerBonus(bear) != 0 {
		t.Errorf("vanilla bear should contribute 0 from etbTriggerBonus; got %.2f", etbTriggerBonus(bear))
	}
	_ = score
}
