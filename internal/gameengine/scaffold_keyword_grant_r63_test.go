package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffold_keyword_grant_r63_test.go — pins the r63 keyword-ability grant
// sweep: inert empty-ModKind static keyword grants ("creatures you control
// have flying") now register layer-6 grants (producer) AND combat's
// evasion path honors them via keywordActive() (consumer). Each test
// asserts a card granting the keyword now actually HAS it in combat.

// staticGrantCard builds a permanent whose only ability is a raw-text
// static keyword grant (the empty-ModKind scaffold shape the parser emits)
// and registers its continuous effects, mirroring ETB.
func staticGrantCard(gs *GameState, seat int, name, raw string) *Permanent {
	p := addBattlefield(gs, seat, name, 0, 0, "enchantment")
	p.Card.AST = &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: raw, Modification: &gameast.Modification{ModKind: ""}},
		},
	}
	RegisterContinuousEffectsForPermanent(gs, p)
	gs.InvalidateCharacteristicsCache()
	return p
}

// ---- producer: detection ----

func TestKeywordGrant_DetectScopes(t *testing.T) {
	cases := []struct {
		raw       string
		wantScope string
		wantKWs   []string
		wantOK    bool
	}{
		{"creatures you control have flying", "you_control", []string{"flying"}, true},
		{"all creatures have haste", "all", []string{"haste"}, true},
		{"other creatures you control have trample and lifelink", "other_you_control", []string{"trample", "lifelink"}, true},
		{"creatures you control have flying, first strike, vigilance, trample", "you_control", []string{"flying", "first strike", "vigilance", "trample"}, true},
		// Rejections (need richer handling — not plain grants):
		{"creatures you control lose flying", "", nil, false},
		{"creatures your opponents control can't have hexproof", "", nil, false},
		{"creatures you control have hexproof from blue", "", nil, false}, // protection-from family ("from")
		{"creatures you control have flying as long as you control a forest", "", nil, false},
		{"whenever a creature you control attacks, it gains flying", "", nil, false},
		{"creatures your opponents control have menace", "", nil, false}, // opponents scope -> follow-up
		{"you draw a card", "", nil, false},
	}
	for _, tc := range cases {
		scope, kws, ok := detectKeywordGrantStatic(tc.raw)
		if ok != tc.wantOK {
			t.Fatalf("detect(%q) ok=%v want %v", tc.raw, ok, tc.wantOK)
		}
		if !ok {
			continue
		}
		if scope != tc.wantScope {
			t.Fatalf("detect(%q) scope=%q want %q", tc.raw, scope, tc.wantScope)
		}
		if len(kws) != len(tc.wantKWs) {
			t.Fatalf("detect(%q) kws=%v want %v", tc.raw, kws, tc.wantKWs)
		}
		for i := range kws {
			if kws[i] != tc.wantKWs[i] {
				t.Fatalf("detect(%q) kws=%v want %v", tc.raw, kws, tc.wantKWs)
			}
		}
	}
}

// ---- producer + consumer: flying evasion end-to-end ----

func TestKeywordGrant_YouControlHaveFlying_NotBlockableByGround(t *testing.T) {
	gs := newFixtureGame(t)
	staticGrantCard(gs, 0, "Levitation", "creatures you control have flying")
	mine := addBattlefield(gs, 0, "Granted Flyer", 2, 2, "creature")
	ground := addBattlefield(gs, 1, "Ground Blocker", 3, 3, "creature")
	gs.InvalidateCharacteristicsCache()

	// Producer: the grant is visible to the layer-aware accessor.
	if !gs.HasKeywordOf(mine, "flying") {
		t.Fatal("producer: 'creatures you control have flying' did not grant flying")
	}
	// It is NOT printed on the creature (proves the union, not a printed kw).
	if mine.HasKeyword("flying") {
		t.Fatal("fixture: creature should not have PRINTED flying")
	}
	// Consumer: a ground creature can't block the granted flyer.
	if canBlockGS(gs, mine, ground) {
		t.Fatal("consumer: ground creature WRONGLY allowed to block a granted flyer")
	}
}

func TestKeywordGrant_ReachGrant_CanBlockFlyer(t *testing.T) {
	gs := newFixtureGame(t)
	flyer := addBattlefield(gs, 1, "Real Flyer", 2, 2, "creature")
	flyer.Flags["kw:flying"] = 1 // printed-ish flying on the attacker
	staticGrantCard(gs, 0, "Reach Anthem", "creatures you control have reach")
	myReach := addBattlefield(gs, 0, "Granted Reacher", 2, 3, "creature")
	gs.InvalidateCharacteristicsCache()

	if !gs.HasKeywordOf(myReach, "reach") {
		t.Fatal("producer: reach grant missing")
	}
	if !canBlockGS(gs, flyer, myReach) {
		t.Fatal("consumer: granted-reach creature should be able to block a flyer")
	}
}

func TestKeywordGrant_AllCreaturesHaveScope(t *testing.T) {
	gs := newFixtureGame(t)
	staticGrantCard(gs, 0, "Concordant Crossroads", "all creatures have haste")
	mine := addBattlefield(gs, 0, "Mine", 1, 1, "creature")
	theirs := addBattlefield(gs, 1, "Theirs", 1, 1, "creature")
	gs.InvalidateCharacteristicsCache()

	if !gs.HasKeywordOf(mine, "haste") || !gs.HasKeywordOf(theirs, "haste") {
		t.Fatal("'all creatures have haste' must grant haste to every creature")
	}
}

func TestKeywordGrant_OtherYouControl_ExcludesSelf(t *testing.T) {
	gs := newFixtureGame(t)
	lord := addBattlefield(gs, 0, "Menace Lord", 2, 2, "creature")
	lord.Card.AST = &gameast.CardAST{
		Name:      "Menace Lord",
		Abilities: []gameast.Ability{&gameast.Static{Raw: "other creatures you control have menace", Modification: &gameast.Modification{ModKind: ""}}},
	}
	other := addBattlefield(gs, 0, "Other", 2, 2, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)
	gs.InvalidateCharacteristicsCache()

	if !gs.HasKeywordOf(other, "menace") {
		t.Fatal("'other creatures you control have menace' must grant to others")
	}
	if gs.HasKeywordOf(lord, "menace") {
		t.Fatal("'OTHER creatures' grant must NOT include the source itself")
	}
}

// ---- consumer union: direct grant + guardrail ----

func TestKeywordGrant_ConsumerUnionAndGuardrail(t *testing.T) {
	gs := newFixtureGame(t)
	atk := addBattlefield(gs, 0, "Attacker", 2, 2, "creature")
	blk := addBattlefield(gs, 1, "Blocker", 2, 2, "creature")

	// Guardrail: before any grant, a ground creature CAN block.
	if !canBlockGS(gs, atk, blk) {
		t.Fatal("guardrail: plain ground creature should be blockable")
	}
	// Direct layer-6 grant (the canonical producer API) — combat honors it.
	registerKeywordGrant(gs, atk, "flying", "test-direct-flying", func(_ *GameState, t *Permanent) bool { return t == atk })
	gs.InvalidateCharacteristicsCache()

	if !keywordActive(gs, atk, "flying") {
		t.Fatal("keywordActive must see a layer-6 grant")
	}
	if canBlockGS(gs, atk, blk) {
		t.Fatal("consumer: ground creature must not block after attacker gains flying via layer grant")
	}
}

// Guardrail: a non-grant empty-ModKind static must NOT register a keyword.
func TestKeywordGrant_NonGrantStaticNoOp(t *testing.T) {
	gs := newFixtureGame(t)
	p := staticGrantCard(gs, 0, "Random Static", "you draw a card at the beginning of your upkeep")
	cr := addBattlefield(gs, 0, "Creature", 2, 2, "creature")
	gs.InvalidateCharacteristicsCache()
	_ = p
	for _, kw := range []string{"flying", "haste", "trample", "menace"} {
		if gs.HasKeywordOf(cr, kw) {
			t.Fatalf("non-grant static wrongly granted %q", kw)
		}
	}
}
