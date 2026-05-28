package moxfield

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parser_r60_test pins the decision branches in parser.go: LoadDeckFromFile
// I/O paths (missing file / bad JSON / failed validation), the internal
// validate() check (empty Name / Commander / Mainboard / card Name /
// invalid Quantity), and the Deck helpers (ExpandLibrary expansion
// semantics, CardCount aggregation). parser.go landed in the 0%-coverage
// bucket of the r60 audit because the existing moxfield_test.go suite
// targeted ValidateFormat (format-legality) rather than the parser itself.

// TestLoadDeckFromFile_HappyPath verifies the canonical success: a
// well-formed deck JSON parses cleanly and the result carries the
// expected Name / Commander / Mainboard quantities.
func TestLoadDeckFromFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.json")
	body := `{
	  "name": "Test Deck",
	  "commander": "Atraxa, Praetors' Voice",
	  "mainboard": [
	    {"name": "Sol Ring", "quantity": 1},
	    {"name": "Swamp", "quantity": 5}
	  ]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	deck, err := LoadDeckFromFile(path)
	if err != nil {
		t.Fatalf("LoadDeckFromFile: %v", err)
	}
	if deck.Name != "Test Deck" {
		t.Errorf("Name = %q", deck.Name)
	}
	if deck.Commander != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander = %q", deck.Commander)
	}
	if len(deck.Mainboard) != 2 {
		t.Errorf("Mainboard len = %d, want 2", len(deck.Mainboard))
	}
}

// TestLoadDeckFromFile_MissingFileErrors pins the I/O error branch: a
// nonexistent path returns a wrapped read error rather than nil deck +
// nil error.
func TestLoadDeckFromFile_MissingFileErrors(t *testing.T) {
	deck, err := LoadDeckFromFile("/nonexistent/path/never-created.json")
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
	if deck != nil {
		t.Errorf("expected nil deck on error, got %+v", deck)
	}
	if !strings.Contains(err.Error(), "read deck file") {
		t.Errorf("error should include 'read deck file' prefix, got: %v", err)
	}
}

// TestLoadDeckFromFile_BadJSONErrors pins the parse-error branch:
// malformed JSON returns the wrapped "parse deck JSON" error.
func TestLoadDeckFromFile_BadJSONErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := LoadDeckFromFile(path)
	if err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse deck JSON") {
		t.Errorf("error should include 'parse deck JSON' prefix, got: %v", err)
	}
}

// TestLoadDeckFromFile_FailedValidationErrors pins the third error
// path: well-formed JSON that fails validation returns the wrapped
// "validate deck" error. Empty commander is the canonical trigger.
func TestLoadDeckFromFile_FailedValidationErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	body := `{"name": "Test", "commander": "", "mainboard": [{"name": "X", "quantity": 1}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := LoadDeckFromFile(path)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "validate deck") {
		t.Errorf("error should include 'validate deck' prefix, got: %v", err)
	}
}

// TestValidate_EachFailureCondition exercises every decision branch in
// validate(): empty Name, empty Commander, empty Mainboard, empty card
// Name, zero/negative Quantity. The single happy-path case at the top
// pins the "all checks pass" branch.
func TestValidate_EachFailureCondition(t *testing.T) {
	good := &Deck{
		Name: "OK", Commander: "Cmdr",
		Mainboard: []*Card{{Name: "Sol Ring", Quantity: 1}},
	}
	if err := validate(good); err != nil {
		t.Errorf("valid deck rejected: %v", err)
	}

	cases := []struct {
		name string
		deck *Deck
		want string
	}{
		{"empty Name", &Deck{Commander: "C", Mainboard: []*Card{{Name: "X", Quantity: 1}}}, "deck name is empty"},
		{"empty Commander", &Deck{Name: "N", Mainboard: []*Card{{Name: "X", Quantity: 1}}}, "no commander"},
		{"empty Mainboard", &Deck{Name: "N", Commander: "C", Mainboard: nil}, "empty mainboard"},
		{"empty card name", &Deck{Name: "N", Commander: "C", Mainboard: []*Card{{Name: "", Quantity: 1}}}, "empty name"},
		{"zero quantity", &Deck{Name: "N", Commander: "C", Mainboard: []*Card{{Name: "X", Quantity: 0}}}, "invalid quantity"},
		{"negative quantity", &Deck{Name: "N", Commander: "C", Mainboard: []*Card{{Name: "X", Quantity: -3}}}, "invalid quantity"},
	}
	for _, tc := range cases {
		err := validate(tc.deck)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q missing %q", tc.name, err.Error(), tc.want)
		}
	}
}

// TestExpandLibrary_QuantitiesAndIndependence verifies (1) total
// instance count matches the summed quantities, (2) each instance is a
// distinct Card pointer (not aliased to the mainboard entry — different
// zones during a game would otherwise corrupt each other), and (3) each
// instance carries Quantity=1 so the runtime doesn't double-count.
func TestExpandLibrary_QuantitiesAndIndependence(t *testing.T) {
	deck := &Deck{
		Mainboard: []*Card{
			{Name: "Forest", Quantity: 10},
			{Name: "Sol Ring", Quantity: 1},
			{Name: "Mountain", Quantity: 5},
		},
	}
	lib := deck.ExpandLibrary()
	if len(lib) != 16 {
		t.Fatalf("library size = %d, want 16", len(lib))
	}
	for i, c := range lib {
		if c.Quantity != 1 {
			t.Errorf("lib[%d] (%s).Quantity = %d, want 1", i, c.Name, c.Quantity)
		}
	}
	// Distinct-pointer guarantee: mutating one Forest instance must not
	// touch any other Forest instance.
	if lib[0].Name != "Forest" || lib[1].Name != "Forest" {
		t.Fatalf("fixture order broke; expected Forests at idx 0-1")
	}
	lib[0].Notes = "tapped this turn"
	if lib[1].Notes == "tapped this turn" {
		t.Errorf("ExpandLibrary aliases card copies — mutating lib[0] affected lib[1]")
	}
}

// TestCardCount_SumsQuantities pins CardCount semantics — the result is
// the sum across all Mainboard quantities, matching the per-deck 99-card
// commander expectation (for a real commander deck).
func TestCardCount_SumsQuantities(t *testing.T) {
	cases := []struct {
		name string
		deck *Deck
		want int
	}{
		{"empty mainboard", &Deck{}, 0},
		{"single 1-of", &Deck{Mainboard: []*Card{{Quantity: 1}}}, 1},
		{"99-card commander", &Deck{Mainboard: []*Card{
			{Name: "Lands", Quantity: 36}, {Name: "Spells", Quantity: 63},
		}}, 99},
		{"with zero-quantity entry", &Deck{Mainboard: []*Card{
			{Quantity: 4}, {Quantity: 0}, {Quantity: 3},
		}}, 7},
	}
	for _, tc := range cases {
		if got := tc.deck.CardCount(); got != tc.want {
			t.Errorf("%s: CardCount = %d, want %d", tc.name, got, tc.want)
		}
	}
}
