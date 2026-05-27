package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestRejectedCardNames_AnteAnchors pins the 9 ante cards by name. Each
// is keyed lowercase, matching how isRealCard normalizes for lookup.
// Drift on either side (Python adds an ante card we miss, or this list
// re-cases a key) would flip them back into the MISSING bucket.
func TestRejectedCardNames_AnteAnchors(t *testing.T) {
	ante := []string{
		"amulet of quoz",
		"bronze tablet",
		"contract from below",
		"darkpact",
		"demonic attorney",
		"jeweled bird",
		"rebirth",
		"tempest efreet",
		"timmerian fiends",
	}
	for _, name := range ante {
		if !rejectedCardNames[name] {
			t.Errorf("rejectedCardNames missing ante card %q (must mirror scripts/parser.py:_REJECTED_CARDS)", name)
		}
	}
}

// TestRejectedCardNames_WotC2020Anchors pins the 7 cards WotC retired
// in December 2020 for offensive imagery. These are NOT ante — they
// appear in Scryfall bulk data because Scryfall archives historical
// printings, but they're permanently outside any legal pool.
func TestRejectedCardNames_WotC2020Anchors(t *testing.T) {
	removed := []string{
		"cleanse",
		"crusade",
		"imprison",
		"invoke prejudice",
		"jihad",
		"pradesh gypsies",
		"stone-throwing devils",
	}
	for _, name := range removed {
		if !rejectedCardNames[name] {
			t.Errorf("rejectedCardNames missing WotC-removed card %q (must mirror scripts/parser.py:_REJECTED_CARDS)", name)
		}
	}
}

// TestRejectedCardNames_Count guards against silent additions or
// deletions. If Python adds a 17th card to _REJECTED_CARDS, both lists
// drift; this count + the anchor tests together force the maintainer
// to update both sides in lockstep.
func TestRejectedCardNames_Count(t *testing.T) {
	if got := len(rejectedCardNames); got != 16 {
		t.Errorf("rejectedCardNames count: want 16 (9 ante + 7 WotC), got %d", got)
	}
}

// TestRejectedCardNames_MirrorsPython reads the live Python list at
// scripts/parser.py:_REJECTED_CARDS and verifies every entry there is
// present in the Go map. This is the load-bearing drift canary — if
// Python is updated and the Go side isn't, this test fails and points
// the maintainer at the gap.
func TestRejectedCardNames_MirrorsPython(t *testing.T) {
	data, err := os.ReadFile("../../scripts/parser.py")
	if err != nil {
		t.Skipf("could not read parser.py (running outside repo?): %v", err)
	}
	// Find the _REJECTED_CARDS = { ... } block.
	startRe := regexp.MustCompile(`_REJECTED_CARDS\s*=\s*\{`)
	loc := startRe.FindIndex(data)
	if loc == nil {
		t.Fatal("could not find _REJECTED_CARDS literal in parser.py")
	}
	tail := data[loc[1]:]
	endIdx := strings.Index(string(tail), "}")
	if endIdx < 0 {
		t.Fatal("unterminated _REJECTED_CARDS literal in parser.py")
	}
	block := string(tail[:endIdx])
	// Extract every double-quoted string literal.
	stringRe := regexp.MustCompile(`"([^"]+)"`)
	matches := stringRe.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatal("found no quoted entries in _REJECTED_CARDS — parse format changed?")
	}
	pyNames := map[string]bool{}
	for _, m := range matches {
		pyNames[strings.ToLower(m[1])] = true
	}
	for name := range pyNames {
		if !rejectedCardNames[name] {
			t.Errorf("Python _REJECTED_CARDS has %q but Go rejectedCardNames is missing it (drift)", name)
		}
	}
	for name := range rejectedCardNames {
		if !pyNames[name] {
			t.Errorf("Go rejectedCardNames has %q but Python _REJECTED_CARDS is missing it (drift)", name)
		}
	}
}

// TestIsRealCard_FiltersRejectedNames wires the new bucket end-to-end
// through isRealCard. A canonical ante card (Bronze Tablet) and a WotC-
// removed card (Stone-Throwing Devils) must BOTH return false from
// isRealCard so they don't enter the audit denominator.
func TestIsRealCard_FiltersRejectedNames(t *testing.T) {
	cases := []struct {
		name string
		e    oracleEntry
	}{
		{"bronze_tablet_ante", oracleEntry{
			Name:     "Bronze Tablet",
			TypeLine: "Artifact",
			SetName:  "Antiquities",
		}},
		{"stone_throwing_wotc_removed", oracleEntry{
			Name:     "Stone-Throwing Devils",
			TypeLine: "Creature — Human Cleric",
			SetName:  "Limited Edition Alpha",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if isRealCard(tc.e) {
				t.Errorf("%s: isRealCard should return false (rejected by name)", tc.e.Name)
			}
		})
	}
}

// TestIsRealCard_CaseInsensitiveName defends the case-folding in the
// lookup. Scryfall canonically capitalizes card names ("Bronze
// Tablet"); the Python set keys are all lowercase. isRealCard must
// normalize on the way in.
func TestIsRealCard_CaseInsensitiveName(t *testing.T) {
	for _, name := range []string{"Bronze Tablet", "BRONZE TABLET", "bronze tablet", "Bronze tablet"} {
		e := oracleEntry{Name: name, TypeLine: "Artifact"}
		if isRealCard(e) {
			t.Errorf("isRealCard(%q): want false (case-insensitive name match), got true", name)
		}
	}
}

// TestIsRealCard_LegitimateCardsPass guards against the filter
// accidentally swallowing legal cards. A normal Commander-legal card
// must still pass isRealCard regardless of the rejected list.
func TestIsRealCard_LegitimateCardsPass(t *testing.T) {
	cases := []oracleEntry{
		{Name: "Lightning Bolt", TypeLine: "Instant", BorderColor: "black"},
		{Name: "Sol Ring", TypeLine: "Artifact", BorderColor: "black"},
		{Name: "Forest", TypeLine: "Basic Land — Forest", BorderColor: "black"},
		// "Imprison Statue" is a hypothetical that contains "imprison" as a
		// substring of its name but isn't on the rejected list. The lookup
		// is exact-name, so it must pass.
		{Name: "Imprison Statue", TypeLine: "Artifact", BorderColor: "black"},
	}
	for _, e := range cases {
		if !isRealCard(e) {
			t.Errorf("%s: isRealCard should return true (legitimate card), got false", e.Name)
		}
	}
}
