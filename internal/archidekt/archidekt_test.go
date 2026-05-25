package archidekt

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// --- ExtractDeckID ---------------------------------------------------------

func TestExtractDeckID_Variants(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://archidekt.com/decks/12345", "12345"},
		{"https://www.archidekt.com/decks/12345", "12345"},
		{"https://archidekt.com/decks/12345/some-deck-slug", "12345"},
		{"http://archidekt.com/decks/9876543210", "9876543210"},
		{"www.archidekt.com/decks/42", "42"},
		{"archidekt.com/decks/7", "7"},
		// Trailing slash + query strings shouldn't fool the regex.
		{"https://archidekt.com/decks/12345/?ref=share", "12345"},
		// Non-numeric IDs: archidekt deck IDs are always digits, so a
		// hash-shaped Moxfield-style ID is treated as malformed rather
		// than silently consumed.
		{"https://archidekt.com/decks/abc123", ""},
		// Different host: no match.
		{"https://www.moxfield.com/decks/abc123", ""},
		// Empty / garbage.
		{"", ""},
		{"not a url at all", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := ExtractDeckID(c.in)
			if got != c.want {
				t.Errorf("ExtractDeckID(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// --- formatDecklist --------------------------------------------------------

func TestFormatDecklist_CommanderMainboardSideboard(t *testing.T) {
	resp := &apiResponse{
		Name: "Test Deck",
		Categories: []apiCategory{
			{Name: "Commander", IncludedInDeck: true},
			{Name: "Maybeboard", IncludedInDeck: false},
			{Name: "Sideboard", IncludedInDeck: false},
		},
		Cards: []apiCard{
			{
				Quantity:   1,
				Categories: []string{"Commander"},
				Card:       cardOf("Atraxa, Praetors' Voice"),
			},
			{Quantity: 1, Categories: []string{}, Card: cardOf("Sol Ring")},
			{Quantity: 8, Categories: []string{}, Card: cardOf("Plains")},
			{Quantity: 1, Categories: []string{"Maybeboard"}, Card: cardOf("Cyclonic Rift")},
			{Quantity: 1, Categories: []string{"Sideboard"}, Card: cardOf("Aether Vial")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	want := strings.Join([]string{
		"COMMANDER: Atraxa, Praetors' Voice",
		"1 Plains was wrong, use 8",
	}, "")
	// Build the expected output explicitly so the assertion failure has
	// the precise diff.
	wantExact := "" +
		"COMMANDER: Atraxa, Praetors' Voice\n" +
		"8 Plains\n" +
		"1 Sol Ring\n" +
		"// Maybeboard: 1 Cyclonic Rift\n" +
		"// Sideboard: 1 Aether Vial\n"
	if out != wantExact {
		t.Errorf("decklist mismatch:\n--- want ---\n%s\n--- got ---\n%s", wantExact, out)
	}
	_ = want
}

func TestFormatDecklist_PartnerCommanders(t *testing.T) {
	// Partner pair: both cards live in the Commander category; both
	// land in COMMANDER lines, in name-sorted order.
	resp := &apiResponse{
		Categories: []apiCategory{{Name: "Commander", IncludedInDeck: true}},
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{"Commander"}, Card: cardOf("Tymna the Weaver")},
			{Quantity: 1, Categories: []string{"Commander", "Partner"}, Card: cardOf("Kraum, Ludevic's Opus")},
			{Quantity: 1, Categories: []string{}, Card: cardOf("Sol Ring")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	want := "COMMANDER: Kraum, Ludevic's Opus\n" +
		"COMMANDER: Tymna the Weaver\n" +
		"1 Sol Ring\n"
	if out != want {
		t.Errorf("partner decklist mismatch:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}

func TestFormatDecklist_CategoryWithIncludedFalseRoutesToBucket(t *testing.T) {
	// A user-named "Considering" category with includedInDeck=false
	// must NOT add the card to the legal mainboard. The output emits
	// it under a comment line so the deckparser drops it from the
	// playable list but the user's intent is preserved.
	resp := &apiResponse{
		Categories: []apiCategory{
			{Name: "Considering", IncludedInDeck: false},
		},
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{}, Card: cardOf("Sol Ring")},
			{Quantity: 1, Categories: []string{"Considering"}, Card: cardOf("Mana Crypt")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !strings.Contains(out, "1 Sol Ring\n") {
		t.Errorf("Sol Ring missing from mainboard:\n%s", out)
	}
	if !strings.Contains(out, "// Considering: 1 Mana Crypt\n") {
		t.Errorf("Mana Crypt should be in Considering bucket, not mainboard:\n%s", out)
	}
	// Anchor with leading newline (or string-start) so this substring
	// doesn't match within "// Considering: 1 Mana Crypt\n".
	if strings.Contains("\n"+out, "\n1 Mana Crypt\n") {
		t.Errorf("Mana Crypt leaked into mainboard:\n%s", out)
	}
}

func TestFormatDecklist_CardInBothInDeckAndExcludedCategoriesCounts(t *testing.T) {
	// Archidekt lets a card live in multiple categories. If ANY of
	// them is includedInDeck, the card is in the deck. This matches
	// the platform's own count.
	resp := &apiResponse{
		Categories: []apiCategory{
			{Name: "Maybeboard", IncludedInDeck: false},
			{Name: "Ramp", IncludedInDeck: true},
		},
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{"Maybeboard", "Ramp"}, Card: cardOf("Sol Ring")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !strings.Contains(out, "1 Sol Ring\n") {
		t.Errorf("Sol Ring should land in mainboard via Ramp:\n%s", out)
	}
	if strings.Contains(out, "Maybeboard") {
		t.Errorf("Sol Ring leaked into Maybeboard despite a Ramp tag:\n%s", out)
	}
}

func TestFormatDecklist_UnknownCategoryDefaultsToInDeck(t *testing.T) {
	// A card in a user-defined category that wasn't declared in the
	// `categories` array (Archidekt allows this) defaults to in-deck —
	// matches the platform's runtime behavior.
	resp := &apiResponse{
		Categories: []apiCategory{}, // empty: every category is "unknown"
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{"Land"}, Card: cardOf("Command Tower")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !strings.Contains(out, "1 Command Tower\n") {
		t.Errorf("unknown-category card should default to mainboard:\n%s", out)
	}
}

func TestFormatDecklist_EmptyDeckErrors(t *testing.T) {
	// No commanders + no mainboard rows → error rather than emit a
	// silent empty file the user later wonders about.
	resp := &apiResponse{
		Name: "Ghost Deck",
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{"Maybeboard"}, Card: cardOf("Sol Ring")},
		},
		Categories: []apiCategory{{Name: "Maybeboard", IncludedInDeck: false}},
	}
	if _, err := formatDecklist(resp); err == nil {
		t.Error("expected error on empty playable deck, got nil")
	}
}

func TestFormatDecklist_DroppedRowsSkipZeroQuantityAndBlankName(t *testing.T) {
	// Defensive: malformed rows must not blow up the importer.
	resp := &apiResponse{
		Categories: []apiCategory{{Name: "Commander", IncludedInDeck: true}},
		Cards: []apiCard{
			{Quantity: 0, Categories: []string{}, Card: cardOf("Ghost Quantity")}, // skip
			{Quantity: 1, Categories: []string{}, Card: cardOf("")},               // skip (blank name)
			{Quantity: 1, Categories: []string{"Commander"}, Card: cardOf("Atraxa")},
			{Quantity: 1, Categories: []string{}, Card: cardOf("Sol Ring")},
		},
	}
	out, err := formatDecklist(resp)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if strings.Contains(out, "Ghost Quantity") {
		t.Errorf("zero-quantity row leaked through:\n%s", out)
	}
	want := "COMMANDER: Atraxa\n1 Sol Ring\n"
	if out != want {
		t.Errorf("decklist mismatch:\n--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}

func TestFormatDecklist_DeterministicOrdering(t *testing.T) {
	// Same deck data presented in arbitrary order should produce the
	// same byte-exact output every time. Important for diff-friendly
	// deck files on disk.
	resp := &apiResponse{
		Categories: []apiCategory{{Name: "Commander", IncludedInDeck: true}},
		Cards: []apiCard{
			{Quantity: 1, Categories: []string{}, Card: cardOf("Zur the Enchanter")},
			{Quantity: 1, Categories: []string{}, Card: cardOf("Anointed Procession")},
			{Quantity: 1, Categories: []string{}, Card: cardOf("Mind Stone")},
			{Quantity: 1, Categories: []string{"Commander"}, Card: cardOf("Atraxa, Praetors' Voice")},
		},
	}
	out1, _ := formatDecklist(resp)
	out2, _ := formatDecklist(resp)
	if out1 != out2 {
		t.Errorf("non-deterministic output:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", out1, out2)
	}
	// Names should appear in ascending order within each section.
	wantOrder := []string{
		"COMMANDER: Atraxa, Praetors' Voice",
		"1 Anointed Procession",
		"1 Mind Stone",
		"1 Zur the Enchanter",
	}
	lines := strings.Split(strings.TrimRight(out1, "\n"), "\n")
	if len(lines) != len(wantOrder) {
		t.Fatalf("line count = %d, want %d:\n%s", len(lines), len(wantOrder), out1)
	}
	for i, w := range wantOrder {
		if lines[i] != w {
			t.Errorf("line %d = %q, want %q", i, lines[i], w)
		}
	}
}

// --- FetchDeck (end-to-end with httptest) ---------------------------------

func TestFetchDeck_EndToEnd(t *testing.T) {
	resetCachesForTest()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/decks/12345/" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"name": "End-to-End Test",
			"categories": [
				{"name": "Commander", "includedInDeck": true},
				{"name": "Maybeboard", "includedInDeck": false}
			],
			"cards": [
				{"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Krenko, Mob Boss"}}},
				{"quantity": 1, "categories": [], "card": {"oracleCard": {"name": "Sol Ring"}}},
				{"quantity": 1, "categories": ["Maybeboard"], "card": {"oracleCard": {"name": "Goblin Chieftain"}}}
			]
		}`)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(SetAPIBaseForTest(server.URL))

	text, err := FetchDeck("https://archidekt.com/decks/12345")
	if err != nil {
		t.Fatalf("FetchDeck: %v", err)
	}
	want := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n// Maybeboard: 1 Goblin Chieftain\n"
	if text != want {
		t.Errorf("FetchDeck output:\n--- want ---\n%s\n--- got ---\n%s", want, text)
	}
}

func TestFetchDeckByID_BadURL(t *testing.T) {
	if _, err := FetchDeck("https://example.com/foo"); err == nil {
		t.Error("expected error on non-archidekt URL, got nil")
	}
}

func TestFetchDeckByID_APIErrorPropagates(t *testing.T) {
	resetCachesForTest()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"detail": "Not found."}`)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(SetAPIBaseForTest(server.URL))

	_, err := FetchDeck("https://archidekt.com/decks/99999")
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code: %v", err)
	}
}

func TestFetchDeck_CacheHitsAvoidSecondAPIRoundTrip(t *testing.T) {
	resetCachesForTest()
	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"name": "Cached",
			"categories": [{"name": "Commander", "includedInDeck": true}],
			"cards": [{"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Krenko"}}}]
		}`)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(SetAPIBaseForTest(server.URL))

	url := "https://archidekt.com/decks/777"
	// Three back-to-back calls: deck, name, deck again. Only the first
	// should hit the API; the rest serve from inProcCache.
	if _, err := FetchDeck(url); err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if _, err := FetchDeckName(url); err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if _, err := FetchDeck(url); err != nil {
		t.Fatalf("call 3: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("API hits = %d, want 1 (cache should serve calls 2+3)", got)
	}
}

// --- helpers ---------------------------------------------------------------

func cardOf(name string) struct {
	OracleCard struct {
		Name string `json:"name"`
	} `json:"oracleCard"`
} {
	var c struct {
		OracleCard struct {
			Name string `json:"name"`
		} `json:"oracleCard"`
	}
	c.OracleCard.Name = name
	return c
}
