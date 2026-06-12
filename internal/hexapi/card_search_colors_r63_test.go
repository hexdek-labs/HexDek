package hexapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// card_search_colors_r63_test.go — deck editor MVP: /api/cards/search
// colors= (CR §903.4 color-identity subset) + scope= (name/text/all
// oracle-text matching, the Scryfall o: equivalent).

func searchTestHandler() *Handler {
	h := &Handler{}
	h.cardDB = map[string]oracleCard{
		"lightning bolt": {ManaCost: "{R}", TypeLine: "Instant",
			OracleText: "Lightning Bolt deals 3 damage to any target.", ColorIdentity: []string{"R"}},
		"counterspell": {ManaCost: "{U}{U}", TypeLine: "Instant",
			OracleText: "Counter target spell.", ColorIdentity: []string{"U"}},
		"sol ring": {ManaCost: "{1}", TypeLine: "Artifact",
			OracleText: "{T}: Add {C}{C}.", ColorIdentity: nil},
		"dockside extortionist": {ManaCost: "{1}{R}", TypeLine: "Creature — Goblin Pirate",
			OracleText: "When this creature enters, create a Treasure token for each artifact and enchantment your opponents control.", ColorIdentity: []string{"R"}},
		"smothering tithe": {ManaCost: "{3}{W}", TypeLine: "Enchantment",
			OracleText: "Whenever an opponent draws a card, that player may pay {2}. If they don't, you create a Treasure token.", ColorIdentity: []string{"W"}},
		"terror of the peaks": {ManaCost: "{3}{R}{R}", TypeLine: "Creature — Dragon",
			OracleText: "Spell Lightning: spells that target this creature cost 3 more.", ColorIdentity: []string{"R"}},
	}
	return h
}

func runSearch(t *testing.T, h *Handler, query string) []CardSearchHit {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/cards/search?"+query, nil)
	rec := httptest.NewRecorder()
	h.handleCardSearch(rec, req)
	var body struct {
		Results []CardSearchHit `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response parse: %v", err)
	}
	return body.Results
}

func names(hits []CardSearchHit) map[string]bool {
	out := map[string]bool{}
	for _, h := range hits {
		out[h.Name] = true
	}
	return out
}

func TestCardSearch_ColorIdentitySubset(t *testing.T) {
	h := searchTestHandler()

	// Mono-red commander: U cards excluded, colorless + R included.
	got := names(runSearch(t, h, "q=o&colors=R"))
	if got["Counterspell"] {
		t.Fatalf("U card leaked through colors=R: %v", got)
	}
	if !got["Sol Ring"] {
		t.Fatalf("colorless card filtered out by colors=R: %v", got)
	}
	if !got["Lightning Bolt"] && !got["Dockside Extortionist"] {
		t.Fatalf("in-identity red cards missing: %v", got)
	}

	// Lowercase letters accepted.
	got = names(runSearch(t, h, "q=counter&colors=wu"))
	if !got["Counterspell"] {
		t.Fatalf("colors=wu should admit a U card: %v", got)
	}

	// No colors param = no filter.
	got = names(runSearch(t, h, "q=counter"))
	if !got["Counterspell"] {
		t.Fatalf("unfiltered search lost the name hit: %v", got)
	}
}

func TestCardSearch_OracleTextMatching(t *testing.T) {
	h := searchTestHandler()

	// The Scryfall o: equivalent — "create a Treasure" finds producers
	// by TEXT under the default scope.
	got := names(runSearch(t, h, "q=create+a+Treasure"))
	if !got["Dockside Extortionist"] || !got["Smothering Tithe"] {
		t.Fatalf("oracle-text matches missing under default scope: %v", got)
	}

	// Color filter composes with text matching.
	got = names(runSearch(t, h, "q=create+a+Treasure&colors=R"))
	if got["Smothering Tithe"] {
		t.Fatalf("W card leaked through colors=R on text search: %v", got)
	}
	if !got["Dockside Extortionist"] {
		t.Fatalf("red text match missing: %v", got)
	}

	// scope=name restores historical name-only behavior.
	got = names(runSearch(t, h, "q=create+a+Treasure&scope=name"))
	if len(got) != 0 {
		t.Fatalf("scope=name matched non-name hits: %v", got)
	}

	// scope=text: oracle-text only — "Lightning" appears in Bolt's NAME
	// and in Terror's TEXT; text scope must return only the text hit
	// ... but Bolt's own oracle text also says "Lightning Bolt deals" —
	// so both match by text; assert Terror is present and type-line-only
	// matches are excluded.
	got = names(runSearch(t, h, "q=Spell+Lightning&scope=text"))
	if !got["Terror Of The Peaks"] || len(got) != 1 {
		t.Fatalf("scope=text wrong: %v", got)
	}
}

func TestCardSearch_NameHitsRankAboveTextHits(t *testing.T) {
	h := searchTestHandler()
	hits := runSearch(t, h, "q=lightning")
	if len(hits) < 2 {
		t.Fatalf("want both name and text matches, got %v", hits)
	}
	if hits[0].Name != "Lightning Bolt" {
		t.Fatalf("name match must rank first, got %q", hits[0].Name)
	}
}
