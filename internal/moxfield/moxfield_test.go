package moxfield

import (
	"testing"
)

func TestExtractDeckID(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.moxfield.com/decks/abc123", "abc123"},
		{"https://moxfield.com/decks/abc123", "abc123"},
		{"www.moxfield.com/decks/abc123", "abc123"},
		{"moxfield.com/decks/abc123", "abc123"},
		{"https://www.moxfield.com/decks/AbC-123_xyz", "AbC-123_xyz"},
		{"https://www.moxfield.com/decks/", ""},           // no ID
		{"https://example.com/decks/abc123", ""},           // wrong domain
		{"not a url", ""},                                  // garbage
		{"", ""},                                           // empty
	}
	for _, tt := range tests {
		got := ExtractDeckID(tt.url)
		if got != tt.want {
			t.Errorf("ExtractDeckID(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestFormatDecklist(t *testing.T) {
	data := &apiResponse{
		Name:   "Test Deck",
		Format: "commander",
		Commanders: map[string]apiCardEntry{
			"Sen Triplets": {Quantity: 1, Card: apiCard{Name: "Sen Triplets"}},
		},
		Mainboard: map[string]apiCardEntry{
			"Sol Ring":       {Quantity: 1, Card: apiCard{Name: "Sol Ring"}},
			"Mana Crypt":     {Quantity: 1, Card: apiCard{Name: "Mana Crypt"}},
			"Island":         {Quantity: 5, Card: apiCard{Name: "Island"}},
		},
	}

	result, err := formatDecklist(data)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}

	// Should contain the commander directive.
	if !containsLine(result, "COMMANDER: Sen Triplets") {
		t.Errorf("missing commander directive in:\n%s", result)
	}
	// Should contain mainboard entries.
	if !containsLine(result, "1 Sol Ring") {
		t.Errorf("missing Sol Ring in:\n%s", result)
	}
	if !containsLine(result, "5 Island") {
		t.Errorf("missing Island in:\n%s", result)
	}
}

func TestFormatDecklist_Empty(t *testing.T) {
	data := &apiResponse{
		Name:      "Empty",
		Mainboard: map[string]apiCardEntry{},
	}
	_, err := formatDecklist(data)
	if err == nil {
		t.Fatal("expected error for empty deck")
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitLines(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
