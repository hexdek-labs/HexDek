package main

import (
	"strings"
	"testing"
)

// archetype_reanimator_refined_r60_test.go — regressions for the
// r60 Reanimator gate refinement.
//
// The pre-r60 Reanimator gate used `graveyardCount` (a conflated
// counter aggregating IsRecursion + self_mill effects + mass_
// reanimate effects). That counter over-fired on broad-graveyard
// value piles (Muldrotha goodstuff) but under-fired on tight
// Animate-Dead / Reanimate shells whose graveyardCount was below 6
// despite running a clear reanimation engine.
//
// The refined gate adds a parallel branch with the canonical
// reanimator spec: 6+ self-mill/discard enablers + 4+ reanimation
// effects. Either branch qualifies a deck as Reanimator; the legacy
// gate stays for backwards compatibility on broader recursion shells.
//
// Three layers of coverage:
//
//  1. cardIsReanimationEffect / cardIsDiscardOutlet — unit-level
//     detection of the canonical patterns (curated names + oracle
//     phrases + IsRecursion + RecursionDest plumbing).
//  2. buildClassifyContext — pins reanimationCount and
//     discardOutletCount accumulate correctly.
//  3. ClassifyArchetype end-to-end — pins a tight Animate-Dead
//     shell lands on Reanimator via the refined gate, AND that the
//     pure-mill Bruvac counterfactual still doesn't false-positive.

// ---------------------------------------------------------------------------
// 1. cardIsReanimationEffect unit-level shape detection
// ---------------------------------------------------------------------------

func TestCardIsReanimationEffect_CuratedAnimateDead(t *testing.T) {
	if !cardIsReanimationEffect("Animate Dead",
		"enchant creature card in a graveyard. ...", false, "") {
		t.Error("Animate Dead should be detected (curated name)")
	}
}

func TestCardIsReanimationEffect_CuratedReanimate(t *testing.T) {
	if !cardIsReanimationEffect("Reanimate",
		"put target creature card from a graveyard onto the battlefield under your control.", false, "") {
		t.Error("Reanimate should be detected (both curated AND oracle pattern)")
	}
}

func TestCardIsReanimationEffect_CuratedPersistCreature(t *testing.T) {
	if !cardIsReanimationEffect("Murderous Redcap", "", false, "") {
		t.Error("Murderous Redcap should be detected (curated persist creature)")
	}
}

func TestCardIsReanimationEffect_OracleTextReturnToBattlefield(t *testing.T) {
	// Unburial Rites-style — uncurated card with the canonical
	// return-to-battlefield phrasing.
	got := cardIsReanimationEffect("Some Reanimator Spell",
		"return target creature card from a graveyard to the battlefield under your control.",
		false, "")
	if !got {
		t.Error("oracle 'return target creature card from graveyard to battlefield' should match")
	}
}

func TestCardIsReanimationEffect_OracleTextMassReanimation(t *testing.T) {
	// Living Death-style mass reanimation.
	got := cardIsReanimationEffect("Living Death Variant",
		"return all creature cards in all graveyards to the battlefield.",
		false, "")
	if !got {
		t.Error("mass-reanimation 'return all creature cards' shape should match")
	}
}

func TestCardIsReanimationEffect_PersistOracleText(t *testing.T) {
	// Persist on an uncurated card.
	got := cardIsReanimationEffect("Uncurated Persister",
		"persist (when this creature dies, if it had no -1/-1 counters on it, return it to the battlefield under its owner's control with a -1/-1 counter on it.)",
		false, "")
	if !got {
		t.Error("persist creatures should be detected as reanimation effects")
	}
}

func TestCardIsReanimationEffect_IsRecursionBattlefieldDest(t *testing.T) {
	// Card flagged IsRecursion + RecursionDest=battlefield but no
	// curated-name match and no canonical phrasing in oracle text —
	// defense-in-depth path via the parser's classification.
	got := cardIsReanimationEffect("Engine-Tagged Reanimator", "", true, "battlefield")
	if !got {
		t.Error("IsRecursion + RecursionDest=battlefield should match as reanimation")
	}
}

func TestCardIsReanimationEffect_RejectsHandRecursion(t *testing.T) {
	// Eternal Witness returns to HAND, not battlefield. Must NOT
	// classify as reanimation.
	got := cardIsReanimationEffect("Eternal Witness",
		"when this creature enters, return target card from your graveyard to your hand.",
		true, "hand")
	if got {
		t.Error("hand-recursion (Eternal Witness) must NOT be classified as reanimation")
	}
}

func TestCardIsReanimationEffect_RejectsGenericArtifact(t *testing.T) {
	if cardIsReanimationEffect("Sol Ring", "{t}: add {c}{c}.", false, "") {
		t.Error("Sol Ring should NOT be classified as reanimation")
	}
}

// ---------------------------------------------------------------------------
// cardIsDiscardOutlet unit-level shape detection
// ---------------------------------------------------------------------------

func TestCardIsDiscardOutlet_CuratedFaithlessLooting(t *testing.T) {
	if !cardIsDiscardOutlet("Faithless Looting",
		"draw two cards, then discard two cards.") {
		t.Error("Faithless Looting should be detected (curated)")
	}
}

func TestCardIsDiscardOutlet_OracleTextLooterPattern(t *testing.T) {
	got := cardIsDiscardOutlet("Some Looter",
		"{t}: draw a card, then discard a card.")
	if !got {
		t.Error("'draw a card, then discard' looter pattern should match")
	}
}

func TestCardIsDiscardOutlet_OracleTextActivatedCost(t *testing.T) {
	got := cardIsDiscardOutlet("Some Outlet",
		"discard a card: this creature gets +1/+1 until end of turn.")
	if !got {
		t.Error("'discard a card:' activated-cost outlet should match")
	}
}

func TestCardIsDiscardOutlet_RejectsOpponentDiscard(t *testing.T) {
	// Mind Rot makes the OPPONENT discard — that doesn't fill our
	// graveyard, so it's not an outlet for the Reanimator engine.
	got := cardIsDiscardOutlet("Mind Rot",
		"target opponent discards two cards.")
	if got {
		t.Error("opponent-targeted discard must NOT be classified as a fill-graveyard outlet")
	}
}

func TestCardIsDiscardOutlet_RejectsGenericRamp(t *testing.T) {
	if cardIsDiscardOutlet("Cultivate",
		"search your library for up to two basic land cards.") {
		t.Error("Cultivate should NOT be classified as a discard outlet")
	}
}

// ---------------------------------------------------------------------------
// 2. buildClassifyContext counter wiring
// ---------------------------------------------------------------------------

func TestBuildClassifyContext_ReanimationAndDiscardCounters(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Animate Dead", TypeLine: "Enchantment — Aura"},
		{Name: "Reanimate", TypeLine: "Sorcery"},
		{Name: "Necromancy", TypeLine: "Enchantment — Aura"},
		{Name: "Unburial Rites", TypeLine: "Sorcery"},
		{Name: "Faithless Looting", TypeLine: "Sorcery"},
		{Name: "Wild Mongrel", TypeLine: "Creature — Hound"},
		{Name: "Putrid Imp", TypeLine: "Creature — Zombie Imp"},
		{Name: "Mind Rot", TypeLine: "Sorcery"}, // opponent discard — NOT outlet
		{Name: "Sol Ring", TypeLine: "Artifact"},
	}
	oracleText := map[string]string{
		"Animate Dead":      "enchant creature card in a graveyard.",
		"Reanimate":         "put target creature card from a graveyard onto the battlefield under your control.",
		"Necromancy":        "enchant creature card in a graveyard.",
		"Unburial Rites":    "return target creature card from your graveyard to the battlefield.",
		"Faithless Looting": "draw two cards, then discard two cards.",
		"Wild Mongrel":      "discard a card: this creature gets +1/+1 and becomes the color of your choice until end of turn.",
		"Putrid Imp":        "discard a card: this creature gains flying until end of turn.",
		"Mind Rot":          "target opponent discards two cards.",
		"Sol Ring":          "{t}: add {c}{c}.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.reanimationCount != 4 {
		t.Errorf("reanimationCount=%d, want 4 (Animate Dead + Reanimate + Necromancy + Unburial Rites)",
			ctx.reanimationCount)
	}
	if ctx.discardOutletCount != 3 {
		t.Errorf("discardOutletCount=%d, want 3 (Faithless Looting + Wild Mongrel + Putrid Imp; Mind Rot excluded)",
			ctx.discardOutletCount)
	}
}

// ---------------------------------------------------------------------------
// 3. ClassifyArchetype end-to-end
// ---------------------------------------------------------------------------

// tightReanimatorFixture builds the canonical Animate-Dead-style
// reanimator shell that the refined gate specifically targets:
//   - 4 reanimation effects (Animate Dead, Reanimate, Necromancy,
//     Unburial Rites)
//   - 6 self-mill/discard enablers (3 mill + 3 discard outlets)
//   - 4 big reanimation targets (the bombs being cheated in)
//   - filler for ramp / draw / tutor / threat baseline
func tightReanimatorFixture() ([]CardProfile, map[string]string, map[RoleTag]int) {
	profiles := []CardProfile{}
	oracleText := map[string]string{}

	addCard := func(name, typeLine, ot string, isRecursion bool, recursionDest string, role RoleTag, roleCounts map[RoleTag]int) {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: typeLine,
			IsRecursion: isRecursion, RecursionDest: recursionDest,
		})
		oracleText[name] = ot
		if role != "" {
			roleCounts[role]++
		}
	}

	roleCounts := map[RoleTag]int{
		RoleLand: 36,
	}

	// 4 reanimation effects — all stamped IsRecursion=true with
	// RecursionDest=battlefield so the parser-side path also matches.
	addCard("Animate Dead", "Enchantment — Aura",
		"enchant creature card in a graveyard. when this enters, return target creature card to the battlefield under your control.",
		true, "battlefield", RoleRecursion, roleCounts)
	addCard("Reanimate", "Sorcery",
		"put target creature card from a graveyard onto the battlefield under your control.",
		true, "battlefield", RoleRecursion, roleCounts)
	addCard("Necromancy", "Enchantment — Aura",
		"return target creature card from a graveyard to the battlefield under your control.",
		true, "battlefield", RoleRecursion, roleCounts)
	addCard("Unburial Rites", "Sorcery",
		"return target creature card from your graveyard to the battlefield.",
		true, "battlefield", RoleRecursion, roleCounts)

	// 3 self-mill enablers.
	addCard("Stitcher's Supplier", "Creature — Zombie",
		"when this enters and when it dies, mill three cards.",
		false, "", RoleUtility, roleCounts)
	addCard("Satyr Wayfinder", "Creature — Satyr",
		"when this enters, mill four cards. you may put a land card from those into your hand.",
		false, "", RoleUtility, roleCounts)
	addCard("Grisly Salvage", "Instant",
		"mill the top five cards of your library, then return a creature or land card to your hand.",
		false, "", RoleUtility, roleCounts)

	// 3 discard outlets.
	addCard("Faithless Looting", "Sorcery",
		"draw two cards, then discard two cards.",
		false, "", RoleDraw, roleCounts)
	addCard("Wild Mongrel", "Creature — Hound",
		"discard a card: this creature gets +1/+1 and becomes the color of your choice until end of turn.",
		false, "", RoleUtility, roleCounts)
	addCard("Putrid Imp", "Creature — Zombie Imp",
		"discard a card: this creature gains flying until end of turn.",
		false, "", RoleUtility, roleCounts)

	// 4 reanimation targets.
	for _, n := range []string{"Razaketh, the Foulblooded", "Lord of the Void", "Sheoldred", "Massacre Wurm"} {
		addCard(n, "Legendary Creature", "trample", false, "", RoleThreat, roleCounts)
	}

	// Baseline filler — ramp / draw / tutor.
	for _, n := range []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet"} {
		addCard(n, "Sorcery", "search your library for a basic land", false, "", RoleRamp, roleCounts)
	}
	for _, n := range []string{"Harmonize", "Read the Bones", "Sign in Blood"} {
		addCard(n, "Sorcery", "draw three cards", false, "", RoleDraw, roleCounts)
	}
	for _, n := range []string{"Demonic Tutor", "Buried Alive", "Entomb"} {
		addCard(n, "Sorcery", "search your library for a card", false, "", RoleTutor, roleCounts)
	}
	// Pad to ~99 with utility tags so role ratios stay in-band.
	// Filler is a mix of Instant / Sorcery / Creature so no single
	// archetype's quantitative gate (Artifacts artifactCount>=20,
	// Enchantress enchantmentPct>=0.30) trips and outcompetes the
	// Reanimator classification. Light count (12) so the total stays
	// ~99 cards without diluting the reanimator core signal.
	fillerTypes := []string{"Instant", "Sorcery", "Creature", "Instant"}
	for i := 0; i < 12; i++ {
		addCard("Filler "+string(rune('A'+i%26)), fillerTypes[i%len(fillerTypes)], "", false, "", RoleUtility, roleCounts)
	}

	return profiles, oracleText, roleCounts
}

func TestClassifyArchetype_TightReanimatorClassifies(t *testing.T) {
	profiles, oracleText, roleCounts := tightReanimatorFixture()
	ac := classifyFixture(profiles, oracleText, roleCounts, len(profiles)+36)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Reanimator" {
		t.Errorf("tight reanimator fixture: want Primary=Reanimator, got %q (signals=%v)",
			ac.Primary, ac.Signals)
	}
	// Reanimation engine signal must surface.
	sawSignal := false
	for _, s := range ac.Signals {
		if strings.Contains(s, "reanimation engine") {
			sawSignal = true
			break
		}
	}
	if !sawSignal {
		t.Errorf("expected 'reanimation engine' signal in ac.Signals; got %v", ac.Signals)
	}
}

// TestClassifyArchetype_PureDiscardWithoutReanimateFallsThrough pins
// the negative-companion: a deck full of self-mill / discard outlets
// but with ZERO reanimation effects must NOT classify as Reanimator
// under the refined gate. The reanimationCount=0 path keeps the
// counter from false-positiving on a pure-fill graveyard shell.
func TestClassifyArchetype_PureDiscardWithoutReanimateFallsThrough(t *testing.T) {
	profiles := []CardProfile{
		// 8 discard/mill outlets, zero reanimation.
		{Name: "Faithless Looting", TypeLine: "Sorcery"},
		{Name: "Wild Mongrel", TypeLine: "Creature — Hound"},
		{Name: "Putrid Imp", TypeLine: "Creature — Zombie Imp"},
		{Name: "Cathartic Reunion", TypeLine: "Sorcery"},
		{Name: "Stitcher's Supplier", TypeLine: "Creature — Zombie"},
		{Name: "Satyr Wayfinder", TypeLine: "Creature — Satyr"},
		{Name: "Grisly Salvage", TypeLine: "Instant"},
		{Name: "Mulch", TypeLine: "Sorcery"},
		// Some threats but no reanimation spells.
		{Name: "Massacre Wurm", TypeLine: "Legendary Creature"},
		{Name: "Sheoldred", TypeLine: "Legendary Creature"},
	}
	oracleText := map[string]string{
		"Faithless Looting":   "draw two cards, then discard two cards.",
		"Wild Mongrel":        "discard a card: this creature gets +1/+1 until end of turn.",
		"Putrid Imp":          "discard a card: this creature gains flying until end of turn.",
		"Cathartic Reunion":   "as an additional cost to cast this spell, discard two cards. draw three cards.",
		"Stitcher's Supplier": "when this enters and when it dies, mill three cards.",
		"Satyr Wayfinder":     "when this enters, mill four cards.",
		"Grisly Salvage":      "mill five cards.",
		"Mulch":               "mill four cards.",
		"Massacre Wurm":       "whenever a creature an opponent controls enters, it gets -2/-2.",
		"Sheoldred":           "menace, lifelink.",
	}
	roleCounts := map[RoleTag]int{
		RoleLand: 36, RoleThreat: 2, RoleUtility: 6, RoleDraw: 2,
	}
	ac := classifyFixture(profiles, oracleText, roleCounts, len(profiles)+36)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Reanimator" {
		t.Errorf("pure-mill/discard shell without reanimation classified as Reanimator; "+
			"refined gate requires reanimationCount >= 4 (signals=%v)", ac.Signals)
	}
}

// TestClassifyArchetype_RefinedGateBranchSelfMillDiscardSplit pins
// that the refined gate counts self-mill AND discard together — a
// deck with only 3 mill enablers but 3 discard outlets satisfies the
// 6+ "fill graveyard" branch via the SUM, not either counter alone.
func TestClassifyArchetype_RefinedGateBranchSelfMillDiscardSplit(t *testing.T) {
	profiles := []CardProfile{}
	oracleText := map[string]string{}
	roleCounts := map[RoleTag]int{RoleLand: 36}

	add := func(name, typeLine, ot string, isRecursion bool, recursionDest string, role RoleTag) {
		profiles = append(profiles, CardProfile{
			Name: name, TypeLine: typeLine,
			IsRecursion: isRecursion, RecursionDest: recursionDest,
		})
		oracleText[name] = ot
		if role != "" {
			roleCounts[role]++
		}
	}

	// 4 reanimation effects.
	add("Animate Dead", "Enchantment — Aura",
		"enchant creature card in a graveyard.", true, "battlefield", RoleRecursion)
	add("Reanimate", "Sorcery",
		"put target creature card from a graveyard onto the battlefield.", true, "battlefield", RoleRecursion)
	add("Necromancy", "Enchantment — Aura",
		"return target creature card from graveyard to the battlefield.", true, "battlefield", RoleRecursion)
	add("Living Death", "Sorcery",
		"return all creature cards in all graveyards to the battlefield.", true, "battlefield", RoleRecursion)

	// 3 self-mill enablers — under the 6-count threshold on its own,
	// but the discard outlets below push the sum over.
	add("Stitcher's Supplier", "Creature — Zombie",
		"mill three cards.", false, "", RoleUtility)
	add("Satyr Wayfinder", "Creature — Satyr",
		"mill four cards.", false, "", RoleUtility)
	add("Grisly Salvage", "Instant",
		"mill five cards.", false, "", RoleUtility)
	// 3 discard outlets — selfMillCount + discardOutletCount = 6.
	add("Faithless Looting", "Sorcery",
		"draw two cards, then discard two cards.", false, "", RoleDraw)
	add("Wild Mongrel", "Creature — Hound",
		"discard a card: this creature gets +1/+1.", false, "", RoleUtility)
	add("Putrid Imp", "Creature — Zombie Imp",
		"discard a card: this creature gains flying.", false, "", RoleUtility)

	// Big targets and filler.
	for _, n := range []string{"Razaketh, the Foulblooded", "Lord of the Void", "Sheoldred", "Massacre Wurm"} {
		add(n, "Legendary Creature", "trample.", false, "", RoleThreat)
	}
	for _, n := range []string{"Cultivate", "Kodama's Reach", "Sol Ring", "Arcane Signet"} {
		add(n, "Sorcery", "search your library.", false, "", RoleRamp)
	}
	for _, n := range []string{"Harmonize", "Read the Bones"} {
		add(n, "Sorcery", "draw three cards.", false, "", RoleDraw)
	}
	for _, n := range []string{"Demonic Tutor", "Buried Alive", "Entomb"} {
		add(n, "Sorcery", "tutor.", false, "", RoleTutor)
	}
	fillerTypes := []string{"Instant", "Sorcery", "Creature", "Instant"}
	for i := 0; i < 12; i++ {
		add("Filler "+string(rune('A'+i%26)), fillerTypes[i%len(fillerTypes)], "", false, "", RoleUtility)
	}

	ac := classifyFixture(profiles, oracleText, roleCounts, len(profiles)+36)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Reanimator" {
		t.Errorf("split fill (3 mill + 3 discard) + 4 reanimation: want Primary=Reanimator, got %q",
			ac.Primary)
	}
}
