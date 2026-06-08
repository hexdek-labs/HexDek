package deckparser

import (
	"strings"
	"testing"
)

// json_deck_test.go — pins the structured-.json → decklist conversion
// added so the engine pool, deck-list, and deck-read endpoints share one
// parse path. Motivating bug (2026-06): pool loaded .txt only, so .json
// decks (the UI/import format) were viewable but un-gauntletable.

func TestLooksLikeJSONDeck(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"plain object", `{"commander":"x"}`, true},
		{"leading whitespace", "  \n\t{\"a\":1}", true},
		{"plaintext decklist", "1 Sol Ring\n1 Arcane Signet", false},
		{"commander directive txt", "COMMANDER: Tergrid\n1 Pox", false},
		{"empty", "", false},
		{"array not object", `["a","b"]`, false},
	}
	for _, c := range cases {
		if got := looksLikeJSONDeck([]byte(c.in)); got != c.want {
			t.Errorf("%s: looksLikeJSONDeck=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestJSONDeckToDecklist(t *testing.T) {
	js := `{"name":"Tergrid","format":"commander","commander":"Tergrid, God of Fright",
		"mainboard":[
			{"name":"Pox","quantity":1},
			{"name":"Swamp","quantity":10},
			{"name":"No Quantity Card"}
		]}`
	out, err := jsonDeckToDecklist([]byte(js))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// Card lines present with quantities; missing quantity defaults to 1.
	for _, want := range []string{"1 Pox", "10 Swamp", "1 No Quantity Card"} {
		if !strings.Contains(out, want) {
			t.Errorf("decklist missing %q\n--- got ---\n%s", want, out)
		}
	}
	// COMMANDER: directive present as a footer so ParseDeckReader picks it up.
	if !strings.Contains(out, "COMMANDER: Tergrid, God of Fright") {
		t.Errorf("decklist missing COMMANDER directive\n--- got ---\n%s", out)
	}
}

func TestJSONDeckToDecklist_EmptyMainboardErrors(t *testing.T) {
	if _, err := jsonDeckToDecklist([]byte(`{"commander":"x","mainboard":[]}`)); err == nil {
		t.Error("expected error for empty mainboard")
	}
}

func TestJSONDeckToDecklist_NoCommanderStillConverts(t *testing.T) {
	// No commander field → no directive; ParseDeckReader falls back to
	// first-resolvable-card-is-commander (same as plaintext).
	out, err := jsonDeckToDecklist([]byte(`{"mainboard":[{"name":"Sol Ring","quantity":1}]}`))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if strings.Contains(out, "COMMANDER:") {
		t.Errorf("did not expect COMMANDER directive when commander empty\n%s", out)
	}
	if !strings.Contains(out, "1 Sol Ring") {
		t.Errorf("missing card line\n%s", out)
	}
}
