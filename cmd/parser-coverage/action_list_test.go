package main

import (
	"sort"
	"strings"
	"testing"
)

// containsActionTitled reports whether the action list has at least
// one action whose Title contains the given substring (case-sensitive).
// Pattern detection uses fixed phrasing in titles ("ETB triggered-ability",
// "modal-spell", etc.) so substring assertions are the right shape —
// they pin behavior without re-stating the full title.
func containsActionTitled(t *testing.T, actions []Action, substr string) {
	t.Helper()
	for _, a := range actions {
		if strings.Contains(a.Title, substr) {
			return
		}
	}
	titles := make([]string, len(actions))
	for i, a := range actions {
		titles[i] = a.Title
	}
	t.Errorf("expected an action with title containing %q; got titles=%v", substr, titles)
}

func TestGenerateActionList_OKReturnsNil(t *testing.T) {
	r := result{Name: "Lightning Bolt", Class: classOK, OracleText: "Lightning Bolt deals 3 damage to any target."}
	if got := generateActionList(r, "Instant"); got != nil {
		t.Fatalf("OK class must return nil action list, got %v", got)
	}
}

func TestGenerateActionList_OKVanillaReturnsNil(t *testing.T) {
	r := result{Name: "Forest", Class: classOKVanilla, OracleText: ""}
	if got := generateActionList(r, "Basic Land — Forest"); got != nil {
		t.Fatalf("OK_VANILLA class must return nil action list, got %v", got)
	}
}

func TestGenerateActionList_MissingIncludesReingestAction(t *testing.T) {
	r := result{Name: "Some New Card", Class: classMissing, OracleText: "When this creature enters the battlefield, draw a card."}
	got := generateActionList(r, "Creature")
	containsActionTitled(t, got, "Re-ingest card via Thor")
	// Pattern detection still runs for MISSING — the oracle text is
	// known even when the AST entry isn't — so ETB + card-draw should
	// also surface.
	containsActionTitled(t, got, "ETB triggered-ability")
	containsActionTitled(t, got, "card-draw")
}

func TestGenerateActionList_PartialIncludesEachParseError(t *testing.T) {
	r := result{
		Name:        "Wacky Card",
		Class:       classPartial,
		OracleText:  "Some text",
		ParseErrors: []string{"unknown_clause:choose_x", "missing_keyword:cascade"},
	}
	got := generateActionList(r, "")
	containsActionTitled(t, got, "unknown_clause:choose_x")
	containsActionTitled(t, got, "missing_keyword:cascade")
}

func TestGenerateActionList_PartialWithEmptyParseErrorsStillFlagsIt(t *testing.T) {
	// Edge case: PARTIAL with no explicit errors. The fully_parsed flag
	// can be false without parse_errors being populated (parser bug).
	// We still want a flagged TODO so the developer investigates.
	r := result{Name: "Mystery", Class: classPartial, OracleText: "Text", ParseErrors: nil}
	got := generateActionList(r, "")
	containsActionTitled(t, got, "fully_parsed=false")
}

func TestGenerateActionList_EmptyASTGetsExtendParserAction(t *testing.T) {
	r := result{Name: "Empty One", Class: classEmptyAST, OracleText: "Doohickey makes a thing."}
	got := generateActionList(r, "Artifact")
	containsActionTitled(t, got, "Extend Python parser")
}

func TestGenerateActionList_DetectsETBTrigger(t *testing.T) {
	r := result{
		Name:       "Mulldrifter",
		Class:      classEmptyAST,
		OracleText: "Flying\nWhen Mulldrifter enters the battlefield, draw two cards.\nEvoke {2}{U}",
	}
	got := generateActionList(r, "Creature — Elemental")
	containsActionTitled(t, got, "ETB triggered-ability")
	containsActionTitled(t, got, "card-draw")
}

func TestGenerateActionList_DetectsActivatedAbility(t *testing.T) {
	r := result{
		Name:       "Birds of Paradise",
		Class:      classEmptyAST,
		OracleText: "Flying\n{T}: Add one mana of any color.",
	}
	got := generateActionList(r, "Creature — Bird")
	containsActionTitled(t, got, "activated-ability")
	containsActionTitled(t, got, "mana-ability")
}

func TestGenerateActionList_DetectsModalSpell(t *testing.T) {
	r := result{
		Name:       "Cryptic Command",
		Class:      classEmptyAST,
		OracleText: "Choose two —\n• Counter target spell.\n• Return target permanent to its owner's hand.",
	}
	got := generateActionList(r, "Instant")
	containsActionTitled(t, got, "modal-spell")
	containsActionTitled(t, got, "Counter target")
	containsActionTitled(t, got, "bounce")
}

func TestGenerateActionList_DetectsReplacementEffect(t *testing.T) {
	r := result{
		Name:       "Doubling Season",
		Class:      classEmptyAST,
		OracleText: "If an effect would create one or more tokens under your control, it creates twice that many of those tokens instead.\nIf an effect would put one or more counters on a permanent you control, it puts twice that many of those counters on that permanent instead.",
	}
	got := generateActionList(r, "Enchantment")
	containsActionTitled(t, got, "replacement-effect")
}

func TestGenerateActionList_DetectsSagaFromTypeLine(t *testing.T) {
	r := result{
		Name:       "The Eldest Reborn",
		Class:      classEmptyAST,
		OracleText: "(As this Saga enters and after your draw step, add a lore counter. Sacrifice after III.)\nI, II — Each opponent chooses a creature they control, then sacrifices it.\nIII — Return target creature card from a graveyard to the battlefield under your control.",
	}
	got := generateActionList(r, "Enchantment — Saga")
	containsActionTitled(t, got, "saga chapter")
}

func TestGenerateActionList_DetectsBattle(t *testing.T) {
	r := result{Name: "Invasion of Ergamon", Class: classEmptyAST, OracleText: "When Invasion of Ergamon enters the battlefield, ..."}
	got := generateActionList(r, "Battle — Siege")
	containsActionTitled(t, got, "battle defense-counter")
}

func TestGenerateActionList_DetectsPlaneswalker(t *testing.T) {
	r := result{
		Name:       "Liliana of the Veil",
		Class:      classEmptyAST,
		OracleText: "+1: Each player discards a card.\n-2: Target player sacrifices a creature.\n-6: Separate all permanents target player controls into two piles. That player sacrifices all permanents in the pile of their choice.",
	}
	got := generateActionList(r, "Legendary Planeswalker — Liliana")
	containsActionTitled(t, got, "planeswalker loyalty-ability")
}

func TestGenerateActionList_DetectsTokenAndCounter(t *testing.T) {
	r := result{
		Name:       "Hangarback Walker",
		Class:      classEmptyAST,
		OracleText: "Hangarback Walker enters the battlefield with X +1/+1 counters on it.\nWhen Hangarback Walker dies, create a 1/1 colorless Thopter artifact creature token with flying for each +1/+1 counter on Hangarback Walker.",
	}
	got := generateActionList(r, "Artifact Creature — Construct")
	containsActionTitled(t, got, "token-creation")
	containsActionTitled(t, got, "+1/+1 counter-placement")
	containsActionTitled(t, got, "LTB / dies")
}

func TestGenerateActionList_KeywordStewSingleLineDetected(t *testing.T) {
	// Short single-line keyword stew should surface its own action so
	// the dev wires the keyword scaffold rather than parsing prose.
	r := result{Name: "Akroma, Angel of Wrath", Class: classEmptyAST, OracleText: "Flying, first strike, vigilance"}
	got := generateActionList(r, "Legendary Creature — Angel")
	containsActionTitled(t, got, "keyword-stew")
}

func TestGenerateActionList_LongTextDoesNotKeywordStew(t *testing.T) {
	// A long-form rules-y body must NOT trigger the keyword-stew rule.
	long := "When this creature attacks, target opponent reveals their hand. You choose a noncreature, nonland card from it. That player discards that card." +
		" If they can't, draw two cards instead, then exile the top three cards of your library."
	r := result{Name: "Long Text Card", Class: classEmptyAST, OracleText: long}
	got := generateActionList(r, "Creature")
	for _, a := range got {
		if strings.Contains(a.Title, "keyword-stew") {
			t.Fatalf("long oracle text incorrectly matched keyword-stew: %v", a)
		}
	}
}

func TestGenerateActionList_ActionsAreSortedByTitle(t *testing.T) {
	r := result{
		Name:       "Multi",
		Class:      classEmptyAST,
		OracleText: "When this enters the battlefield, search your library for a card. Create a 1/1 token. {T}: Add {G}.",
	}
	got := generateActionList(r, "Creature")
	titles := make([]string, len(got))
	for i, a := range got {
		titles[i] = a.Title
	}
	if !sort.StringsAreSorted(titles) {
		t.Fatalf("action list must be sorted by title for stable rendering, got %v", titles)
	}
}

func TestGenerateActionList_DedupesIdenticalTitles(t *testing.T) {
	// Oracle text with two ETB clauses (e.g., a card with a primary ETB
	// trigger and a parenthetical reminder) must not emit two
	// "ETB triggered-ability" actions.
	r := result{
		Name:       "Twin ETB",
		Class:      classEmptyAST,
		OracleText: "When this enters the battlefield, draw a card.\nWhen Twin ETB enters the battlefield, scry 2.",
	}
	got := generateActionList(r, "Creature")
	count := 0
	for _, a := range got {
		if strings.Contains(a.Title, "ETB triggered-ability") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("ETB action must appear exactly once after dedup, got %d", count)
	}
}

func TestRenderActionList_EmitsChecklist(t *testing.T) {
	r := result{
		Name:       "Mulldrifter",
		Class:      classEmptyAST,
		OracleText: "Flying\nWhen Mulldrifter enters the battlefield, draw two cards.",
	}
	actions := generateActionList(r, "Creature — Elemental")
	md := renderActionList(r.Name, "Creature — Elemental", r, actions)

	for _, want := range []string{
		"# Action list — Mulldrifter",
		"Class: **EMPTY_AST**",
		"Type: `Creature — Elemental`",
		"## Oracle text",
		"> Flying",
		"## TODO",
		"- [ ] ",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered markdown missing %q\n----\n%s\n----", want, md)
		}
	}
}

func TestRenderActionList_OKShowsAlreadyCovered(t *testing.T) {
	r := result{Name: "Lightning Bolt", Class: classOK, OracleText: "Lightning Bolt deals 3 damage to any target."}
	md := renderActionList(r.Name, "Instant", r, nil)
	if !strings.Contains(md, "Already covered") {
		t.Errorf("OK class render must say 'Already covered', got:\n%s", md)
	}
	if strings.Contains(md, "- [ ]") {
		t.Errorf("OK class render must not contain checklist items, got:\n%s", md)
	}
}

func TestRenderActionList_EmptyActionsSaysManualInspection(t *testing.T) {
	// A card we can't auto-detect any pattern for (e.g., a one-off
	// oddball whose oracle text doesn't hit any rule). The renderer
	// must NOT emit an empty checklist — it should tell the developer
	// to inspect manually.
	r := result{Name: "Weird Card", Class: classEmptyAST, OracleText: "Xyzzy plugh."}
	md := renderActionList(r.Name, "Artifact", r, []Action{{Title: "Extend Python parser to produce non-empty abilities for this oracle text", Reason: "EMPTY_AST"}})
	if !strings.Contains(md, "- [ ] Extend Python parser") {
		t.Errorf("expected the class-level Extend Python parser TODO, got:\n%s", md)
	}
}

func TestRenderActionList_NoPatternsAndNoClassActionsFallsBackToManual(t *testing.T) {
	// Pathological case: caller passed an empty action slice (e.g.,
	// pre-computed and filtered out). The renderer must produce the
	// manual-inspection fallback message, not an empty TODO section.
	r := result{Name: "Mystery", Class: classEmptyAST, OracleText: "Some text."}
	md := renderActionList(r.Name, "Artifact", r, nil)
	if !strings.Contains(md, "manual inspection required") {
		t.Errorf("expected manual-inspection fallback, got:\n%s", md)
	}
}

func TestFindResultByName_ExactAndCaseInsensitive(t *testing.T) {
	rs := []result{
		{Name: "Sol Ring", Class: classOK},
		{Name: "Sword of Fire and Ice", Class: classOK},
	}
	if r, ok := findResultByName(rs, "Sol Ring"); !ok || r.Name != "Sol Ring" {
		t.Errorf("exact match failed: %+v ok=%v", r, ok)
	}
	if r, ok := findResultByName(rs, "sol ring"); !ok || r.Name != "Sol Ring" {
		t.Errorf("case-insensitive fallback failed: %+v ok=%v", r, ok)
	}
	if _, ok := findResultByName(rs, "Lightning Bolt"); ok {
		t.Errorf("missing card should not be found")
	}
}
