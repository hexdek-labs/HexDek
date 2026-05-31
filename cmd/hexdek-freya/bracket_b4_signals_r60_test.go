package main

import (
	"strings"
	"testing"
)

// TestMLDList_BasicShape sanity-checks the curated mass-land-destruction
// list. Catches accidental list collapse from refactors and pins the
// canonical entries that the MLD scoring + floor depend on. Mirrors the
// shape of TestCEDHFreeInteractionList_BasicShape.
func TestMLDList_BasicShape(t *testing.T) {
	if len(mldList) < 12 {
		t.Errorf("mldList collapsed to %d entries (expected 12+)", len(mldList))
	}
	mustHave := []string{
		"armageddon",             // canonical "destroy all lands" anchor
		"ravages of war",         // 4-mana white MLD
		"wildfire",               // red asymmetric (own ramp recovers fastest)
		"jokulhaups",             // grand-scale wipe
		"obliterate",             // all-permanents wipe
		"cataclysm",              // symmetric mass-sacrifice
		"decree of annihilation", // cycle-tutorable MLD
		"apocalypse",             // exile-all-permanents
	}
	for _, name := range mustHave {
		if !mldList[name] {
			t.Errorf("MLD list missing canonical entry %q", name)
		}
	}
	// Negative: single-target land destruction is NOT MLD. The MLD
	// gate is about symmetric mass-reset, not target-removal-of-lands.
	// A B3 deck can run 4 of Strip Mine without being B4.
	mustNotHave := []string{
		"strip mine",
		"wasteland",
		"tectonic edge",
		"ghost quarter",
		"sol ring",
	}
	for _, name := range mustNotHave {
		if mldList[name] {
			t.Errorf("MLD list incorrectly includes single-target / non-MLD %q", name)
		}
	}
}

// TestMLDList_RejectsKnownNonMLDCards is a regression pin from the
// 2026-05-30 power-tier vs bracket reconciliation audit. The initial
// PR #803 MLD list erroneously included three cards that read as MLD
// at a glance but aren't:
//
//   - "avatar of fury": 8/8 flying creature with a red-spell-damage
//     trigger ("Whenever a player casts a red spell, ~ deals damage
//     equal to its power"). No land-destruction text.
//   - "argothian wurm": 6/6 trampler whose ETB lets target player
//     sacrifice ONE land instead of taking the trampler. Single-land
//     asymmetric edict — same shape as Strip Mine, NOT mass-reset.
//   - "plague of vermin": "Beginning with you, each player pays X
//     life; that player creates X 1/1 black Rat tokens." Token
//     creation that incidentally costs life, NOT land destruction.
//
// The false-positive lifted mirror_mastery_2011 to B4 via the MLD
// floor (raw 2 → B4 because Avatar of Fury was counted as MLD),
// creating a B4/T2 contradiction in the PowerTier bridge test.
// Removing the three entries closed the contradiction. This test
// pins the rejection so a future "let's add Avatar of Fury back" PR
// fails fast with a named card to investigate.
func TestMLDList_RejectsKnownNonMLDCards(t *testing.T) {
	knownNonMLD := map[string]string{
		"avatar of fury":   "8/8 flying creature with red-spell-damage trigger; no land destruction text",
		"argothian wurm":   "single-land asymmetric ETB edict; not mass-reset",
		"plague of vermin": "token creation that costs life; not land destruction",
	}
	for name, why := range knownNonMLD {
		if mldList[name] {
			t.Errorf("MLD list re-introduced non-MLD card %q (%s) — removing this card was the fix for the mirror_mastery bridge contradiction in the 2026-05-30 reconciliation audit",
				name, why)
		}
	}
}

// TestEstimateBracket_MLDFloorLifts_GC0 verifies a GC=0 deck with a
// single MLD piece lifts to B4 via the MLD floor. This is the cleanest
// expression of the official rubric: "MLD allowed at Bracket 4, NOT at
// Bracket 3" — no Game Changer corroboration required, the MLD piece
// IS the corroborating deck-shape signal.
func TestEstimateBracket_MLDFloorLifts_GC0(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	// B2-shape ramp deck (no GC, no combo, moderate signals) with one
	// Armageddon — would be B2 without MLD, lifts to B4 with.
	noMLD := &classifyContext{
		roleRatios:    map[RoleTag]float64{},
		avgCMC:        2.4,
		fastManaCount: 4,
		tutorDensity:  0.04,
	}
	withMLD := &classifyContext{
		roleRatios:    map[RoleTag]float64{},
		avgCMC:        2.4,
		fastManaCount: 4,
		tutorDensity:  0.04,
		mldCount:      1,
		mldNames:      []string{"Armageddon"},
	}

	bNoMLD, _, _ := estimateMeasuredBracket(noMLD, report, "")
	bWithMLD, _, brWith := estimateMeasuredBracket(withMLD, report, "")

	if bWithMLD < bNoMLD {
		t.Errorf("MLD shouldn't lower bracket: noMLD=B%d, withMLD=B%d", bNoMLD, bWithMLD)
	}
	if bWithMLD != 4 {
		t.Errorf("MLD floor: want B4, got B%d", bWithMLD)
	}
	// Rationale must record the MLD floor adjustment so report readers
	// see WHY the deck lifted (audit trail for the official-rubric
	// reasoning).
	found := false
	for _, s := range brWith.Signals {
		if s.Name == "MLD floor" && s.Kind == "floor" {
			if !strings.Contains(s.Note, "Armageddon") {
				t.Errorf("MLD floor note should name the evidence card, got: %q", s.Note)
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MLD floor adjustment missing from rationale signals")
	}
}

// TestEstimateBracket_MLDCeilingLift_GC1to3 verifies the GC=1-3 ceiling
// carveout: a deck with 1-3 Game Changers that scores into B4 raw doesn't
// get capped at B3 if it has an MLD piece. Mirrors the existing
// winning-combo and tuned-redundancy carveout entries in the ceiling
// switch.
func TestEstimateBracket_MLDCeilingLift_GC1to3(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	// Deck shape: GC=1, no winning combo, no tuned redundancy. Raw
	// score would push to B4 (GC=1 +1 / fast=6 +2 / fin=4 +1 / tutor=8% +2
	// / low CMC +1 = 7 → B3 raw, but with MLD +1 = 8 → B4 raw). Without
	// the carveout, the GC=1-3 ceiling would re-cap to B3. With MLD,
	// the carveout fires and B4 sticks. NOTE: the MLD floor would also
	// lift here even if the ceiling didn't carve out — this test pins
	// the ceiling-side branch explicitly so a future refactor that
	// removes the floor doesn't silently break the official-rubric
	// invariant.
	report.Finishers = makeComboList(4)
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,
		fastManaCount:    6,
		tutorDensity:     0.08,
		gameChangerCount: 1,
		mldCount:         1,
		mldNames:         []string{"Wildfire"},
	}
	b, _, br := estimateMeasuredBracket(ctx, report, "")
	if b != 4 {
		t.Errorf("MLD ceiling carveout: want B4, got B%d (rationale: %+v)", b, br)
	}
	// Confirm the GC=1-3 ceiling adjustment recorded that the carveout
	// fired (the "capped at B4: winning combo, tuned-redundancy, or MLD"
	// branch — distinct from the "capped at B3" branch that would
	// otherwise fire).
	for _, s := range br.Signals {
		if s.Name == "GC=1-3 ceiling" && s.Kind == "ceiling" {
			if !strings.Contains(s.Note, "MLD") {
				t.Errorf("GC=1-3 ceiling note should mention MLD carveout, got: %q", s.Note)
			}
		}
	}
}

// TestEstimateBracket_MLDScoreContribution pins the raw-score
// contribution of MLD: 1 piece = +1, 2+ pieces = +2. The 2-piece
// threshold is deliberate — a single MLD effect can be a tutored
// toolbox piece, but 2+ pieces is a deck-shape commitment.
func TestEstimateBracket_MLDScoreContribution(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
	}
	base := func() *classifyContext {
		return &classifyContext{
			roleRatios: map[RoleTag]float64{},
			avgCMC:     2.4,
		}
	}
	noMLD := base()
	oneMLD := base()
	oneMLD.mldCount = 1
	oneMLD.mldNames = []string{"Armageddon"}
	twoMLD := base()
	twoMLD.mldCount = 2
	twoMLD.mldNames = []string{"Armageddon", "Ravages of War"}

	_, _, brNone := estimateMeasuredBracket(noMLD, report, "")
	_, _, brOne := estimateMeasuredBracket(oneMLD, report, "")
	_, _, brTwo := estimateMeasuredBracket(twoMLD, report, "")

	if brOne.RawScore-brNone.RawScore != 1 {
		t.Errorf("1 MLD piece should add +1 raw, got delta %d",
			brOne.RawScore-brNone.RawScore)
	}
	if brTwo.RawScore-brNone.RawScore != 2 {
		t.Errorf("2 MLD pieces should add +2 raw, got delta %d",
			brTwo.RawScore-brNone.RawScore)
	}
}

// TestEstimateBracket_FastManaDensityFloor_Lifts verifies the voja-class
// false-negative fix: a deck with fastMana>=8 + finishers>=4 + GC>=1
// (the Voltron-tribal tempo signature) lifts to B4. Without the floor
// the GC=1-3 ceiling would cap the deck at B3 even though it plays as
// B4 (8+ fast mana means consistent T2-T3 acceleration, the deck-shape
// signal of B4-level optimization).
func TestEstimateBracket_FastManaDensityFloor_Lifts(t *testing.T) {
	// Voja-shape: 8 fast mana, 4 finishers, 1 GC, 5% tutor.
	// hasWinningCombo=false (no true_inf), tunedRedundancy=false (only
	// 4 finishers, threshold 8) → without the new floor, caps at B3.
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(4),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.2,
		fastManaCount:    8,
		tutorDensity:     0.05,
		gameChangerCount: 1,
	}
	b, _, br := estimateMeasuredBracket(ctx, report, "")
	if b != 4 {
		t.Errorf("fast-mana-density floor (voja shape): want B4, got B%d", b)
	}
	// Confirm the floor adjustment was recorded.
	found := false
	for _, s := range br.Signals {
		if s.Name == "Fast-mana-density floor" && s.Kind == "floor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("fast-mana-density floor adjustment missing from rationale")
	}
}

// TestEstimateBracket_FastManaDensityFloor_NeedsCorroborator pins the
// gate: 8+ fast mana + 4+ finishers ALONE is not enough — the deck
// must also have at least one Game Changer OR 6%+ tutor density. A
// no-GC no-tutor ramp pile that ships 8 cheap rocks (theoretically
// possible in a Selvala / Azusa / Wood Elves green-stompy build) is
// NOT inherently B4; it needs the deliberate-optimization signal.
func TestEstimateBracket_FastManaDensityFloor_NeedsCorroborator(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(4),
	}
	// No GC, no tutors — high fast-mana density alone isn't B4.
	noCorrob := &classifyContext{
		roleRatios:    map[RoleTag]float64{},
		avgCMC:        2.4,
		fastManaCount: 8,
	}
	bNo, _, _ := estimateMeasuredBracket(noCorrob, report, "")
	if bNo >= 4 {
		t.Errorf("fast-mana-density floor without corroborator should NOT lift to B4, got B%d", bNo)
	}

	// Same shape + 6% tutor density — corroborator present, floor fires.
	withTutors := &classifyContext{
		roleRatios:    map[RoleTag]float64{},
		avgCMC:        2.4,
		fastManaCount: 8,
		tutorDensity:  0.06,
	}
	bTutor, _, _ := estimateMeasuredBracket(withTutors, report, "")
	if bTutor != 4 {
		t.Errorf("fast-mana-density floor with 6%% tutors: want B4, got B%d", bTutor)
	}
}

// TestEstimateBracket_FastManaDensityFloor_NeedsHighDensity pins the
// 8-piece threshold. A deck with 7 fast mana + 4 finishers + 1 GC
// (nazgul-shape) does NOT trip the floor — the threshold is the
// deliberate marker of T2-T3 acceleration tempo, and 7 is the line
// below which most precons can naturally land.
func TestEstimateBracket_FastManaDensityFloor_NeedsHighDensity(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(4),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,
		fastManaCount:    7, // below the 8-piece threshold
		tutorDensity:     0.04,
		gameChangerCount: 1,
	}
	b, _, _ := estimateMeasuredBracket(ctx, report, "")
	if b >= 4 {
		t.Errorf("7 fast-mana pieces should NOT trip the density floor, got B%d", b)
	}
}

// TestHasCategoricalWinClassInList pins the class-gating helper used by
// hasReliableWinningCombo and hasWinningCombo. The class set must
// match twoCardCategoricalWinClasses exactly — Class="unknown" and
// the value-engine classes (lockdown / mana_sink / blink_engine /
// etb_payoff / etb_doubler / infinite_mana) must NOT match.
func TestHasCategoricalWinClassInList(t *testing.T) {
	// Categorical-win class — must match.
	winClasses := []string{
		ComboClassInfiniteDamage,
		ComboClassInfiniteDrain,
		ComboClassInfiniteMill,
		ComboClassLibraryExileWin,
		ComboClassCombatFinisher,
		ComboClassStormFinisher,
		ComboClassInfiniteTokens,
	}
	for _, cls := range winClasses {
		combos := []ComboResult{{Cards: []string{"x", "y", "z"}, Class: cls}}
		if !hasCategoricalWinClassInList(combos) {
			t.Errorf("class %q should match (categorical win)", cls)
		}
	}

	// Non-categorical / value-engine / unknown — must NOT match.
	nonWinClasses := []string{
		ComboClassUnknown,
		ComboClassLockdown,
		ComboClassManaSink,
		ComboClassBlinkEngine,
		ComboClassETBPayoff,
		ComboClassETBDoubler,
		ComboClassInfiniteMana,
		ComboClassInfiniteETB,
		ComboClassGraveyardLoop,
		"",
	}
	for _, cls := range nonWinClasses {
		combos := []ComboResult{{Cards: []string{"x", "y"}, Class: cls}}
		if hasCategoricalWinClassInList(combos) {
			t.Errorf("class %q should NOT match (not a categorical win)", cls)
		}
	}

	// Empty list — must NOT match.
	if hasCategoricalWinClassInList(nil) {
		t.Errorf("empty combo list should NOT match")
	}
}

// TestEstimateBracket_NoOutletInfiniteDoesNotLift is the nazgul-case
// regression pin: a true_infinite with Class="unknown" (the engine's
// fallback for combos with no outlet that loop to a draw rather than
// winning) must NOT trip the Winning-combo floor or the heuristic
// carveout. Prior to the fix the bare `trueInfCount >= 1` arm in
// both hasWinningCombo and hasReliableWinningCombo lifted these
// no-outlet loops as if they were winning combos.
func TestEstimateBracket_NoOutletInfiniteDoesNotLift(t *testing.T) {
	// Nazgul-shape: GC=1, fast=7, 1 true_infinite with Class="unknown"
	// (the Cremate + Unearth no-outlet loop). Raw score = GC(+1) +
	// tutor4%(+1) + combo(+2) + lowCMC(+1) + fast6(+2) = 7 → B3 raw.
	// Without the fix, hasWinningCombo would lift to B4 via the
	// heuristic carveout (GC>=1 + heuristicWinningCombo). With the
	// fix, both predicates class-gate the true_infinite arm, the
	// unknown-class loop is excluded, and the deck stays at B3.
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Cremate", "Unearth"}, Class: ComboClassUnknown},
		},
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.2,
		comboCount:       1,
		fastManaCount:    7,
		tutorDensity:     0.04,
		gameChangerCount: 1,
	}
	b, lbl, br := estimateMeasuredBracket(ctx, report, "")
	if b != 3 {
		t.Errorf("no-outlet Class=unknown true_infinite should NOT lift to B4: got B%d (%s)", b, lbl)
		t.Logf("rationale: %+v", br)
	}
	// Defense-in-depth: no Winning-combo floor adjustment should be
	// recorded (the carveout didn't fire).
	for _, s := range br.Signals {
		if s.Name == "Winning-combo floor" {
			t.Errorf("Winning-combo floor should not fire on unknown-class true_inf, got: %q", s.Note)
		}
	}
}

// TestEstimateBracket_CategoricalClassInfiniteLifts is the positive
// counterpart to the no-outlet test: a true_infinite with a real
// categorical-win class (infinite damage / drain / mill / etc.) MUST
// still trip the Winning-combo floor when GC>=0 and bracket would
// otherwise sit below B4. Pins that the class-gating tightening
// didn't over-narrow.
func TestEstimateBracket_CategoricalClassInfiniteLifts(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Heliod, Sun-Crowned", "Walking Ballista"},
				Class: ComboClassInfiniteDamage},
		},
	}
	// GC=1 + categorical-win true_inf → should lift to B4 via
	// Winning-combo floor.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,
		comboCount:       1,
		fastManaCount:    4,
		tutorDensity:     0.04,
		gameChangerCount: 1,
	}
	b, _, _ := estimateMeasuredBracket(ctx, report, "")
	if b != 4 {
		t.Errorf("categorical-class true_inf should lift to B4: got B%d", b)
	}
}

// TestEstimateBracket_LockdownClassInfiniteDoesNotLift verifies the
// other excluded class: a lockdown true_infinite (Stasis-style
// "table is locked but no one is dead") does NOT count as a winning
// combo. Lockdown is in twoCardCategoricalWinClasses' exclusion set
// per the existing classifier — this pins that the new
// hasCategoricalWinClassInList helper agrees.
func TestEstimateBracket_LockdownClassInfiniteDoesNotLift(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Stasis", "Forsaken Monument"},
				Class: ComboClassLockdown},
		},
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.4,
		comboCount:       1,
		fastManaCount:    3,
		tutorDensity:     0.04,
		gameChangerCount: 1,
	}
	b, _, br := estimateMeasuredBracket(ctx, report, "")
	if b >= 4 {
		t.Errorf("lockdown true_inf should NOT lift to B4: got B%d", b)
	}
	for _, s := range br.Signals {
		if s.Name == "Winning-combo floor" {
			t.Errorf("Winning-combo floor should not fire on lockdown class, got: %q", s.Note)
		}
	}
}
