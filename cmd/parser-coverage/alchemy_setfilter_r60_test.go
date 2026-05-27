package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestExcludedSetTypes_AlchemyPresent pins alchemy in the Go-side
// excludedSetTypes map. The motivating case: Teysa of the Ghost
// Council (Alchemy: Outlaws of Thunder Junction) carried the lone
// PARTIAL on the corpus — `[teysa intensifies by 1]` — because the
// "intensity" mechanic is digital-only and the parser doesn't model
// it. All 18 cards using intensity / intensifies are set_type=alchemy
// or set_type=draft_innovation with games=["arena"], so excluding
// alchemy at the denominator drops Teysa cleanly.
func TestExcludedSetTypes_AlchemyPresent(t *testing.T) {
	if !excludedSetTypes["alchemy"] {
		t.Error("excludedSetTypes must include \"alchemy\" — see scripts/parser.py:is_real_card and docs/parser-coverage-r41.md")
	}
}

// TestExcludedSetTypes_MirrorsPython parses scripts/parser.py and
// extracts the live set-type exclusion literal, asserting byte-for-
// byte equality with the Go map. Same drift-canary pattern as
// TestRejectedCardNames_MirrorsPython — adds set_types to the lockstep
// invariant so Python or Go can't silently move the filter.
func TestExcludedSetTypes_MirrorsPython(t *testing.T) {
	data, err := os.ReadFile("../../scripts/parser.py")
	if err != nil {
		t.Skipf("could not read parser.py: %v", err)
	}
	// The literal lives inline at the set_type check:
	//   if c.get("set_type") in {"memorabilia", "token", "minigame", "funny", "alchemy"}:
	// Pin on that exact prefix so we extract the right braced set
	// (the file has other `{...}` blocks).
	re := regexp.MustCompile(`c\.get\("set_type"\)\s+in\s+\{([^}]+)\}`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		t.Fatal("could not find set_type exclusion literal in parser.py")
	}
	// Extract every double-quoted entry from the body.
	pyEntries := map[string]bool{}
	for _, sm := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(m[1], -1) {
		pyEntries[strings.ToLower(sm[1])] = true
	}
	if len(pyEntries) == 0 {
		t.Fatal("found 0 entries in Python set_type literal — parse shape changed?")
	}
	for k := range pyEntries {
		if !excludedSetTypes[k] {
			t.Errorf("Python is_real_card excludes set_type=%q but Go excludedSetTypes is missing it (drift)", k)
		}
	}
	for k := range excludedSetTypes {
		if !pyEntries[k] {
			t.Errorf("Go excludedSetTypes has %q but Python is_real_card is missing it (drift)", k)
		}
	}
}

// TestIsRealCard_RejectsAlchemyCard pins the end-to-end exclusion
// through isRealCard. Teysa of the Ghost Council is the canonical
// post-fix expulsion target — the lone PARTIAL pre-fix.
func TestIsRealCard_RejectsAlchemyCard(t *testing.T) {
	e := oracleEntry{
		Name:     "Teysa of the Ghost Council",
		TypeLine: "Legendary Creature — Spirit Advisor",
		SetName:  "Alchemy: Outlaws of Thunder Junction",
		SetType:  "alchemy",
	}
	if isRealCard(e) {
		t.Error("Teysa of the Ghost Council: isRealCard should return false (set_type=alchemy)")
	}
}

// TestIsRealCard_AcceptsPaperCommanderCard defends against the new
// filter overshooting. A normal paper Commander-legal card with a
// non-excluded set_type must still pass.
func TestIsRealCard_AcceptsPaperCommanderCard(t *testing.T) {
	cases := []oracleEntry{
		{Name: "Lightning Bolt", TypeLine: "Instant", SetType: "core", BorderColor: "black"},
		{Name: "Sol Ring", TypeLine: "Artifact", SetType: "expansion", BorderColor: "black"},
		// "expansion" is the set_type used by both paper-printed sets AND
		// (rarely) a small number of digital-only releases. The filter is
		// scoped to set_type=alchemy specifically so an Outlaws of Thunder
		// Junction printing of a paper card stays in scope.
		{Name: "Slickshot Show-Off", TypeLine: "Creature — Goblin", SetType: "expansion", BorderColor: "black"},
	}
	for _, e := range cases {
		if !isRealCard(e) {
			t.Errorf("%s: isRealCard should return true (paper Commander-legal card), got false", e.Name)
		}
	}
}
