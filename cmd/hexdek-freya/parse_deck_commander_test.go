package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempDeck writes contents to a temp file and returns its path.
func writeTempDeck(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.txt")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp deck: %v", err)
	}
	return path
}

func containsCard(cards []string, name string) bool {
	for _, c := range cards {
		if c == name {
			return true
		}
	}
	return false
}

// TestParseDeckList_CommanderSetCodeAndHeader covers the two deck-import
// defects: (1) the `COMMANDER:` directive must strip a trailing "(SET) N"
// printing suffix like every normal card line does, and (2) a
// `//Commander` section header must cause the FOLLOWING card line to be
// captured as the commander (mirroring internal/deckparser).
func TestParseDeckList_CommanderSetCodeAndHeader(t *testing.T) {
	tests := []struct {
		name          string
		deck          string
		wantCommander string
		wantCardIn    string // a non-commander card expected in the list ("" to skip)
		wantCardOut   string // a name that must NOT be in the list ("" to skip)
	}{
		{
			name:          "directive with set code is stripped",
			deck:          "1 Sol Ring (LTC) 1\nCOMMANDER: Gimbal, Gremlin Prodigy (MOC) 3\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Sol Ring",
		},
		{
			name:          "directive without set code still works",
			deck:          "1 Sol Ring\nCOMMANDER: Gimbal, Gremlin Prodigy\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Sol Ring",
		},
		{
			name:          "//Commander header captures next line and strips set code",
			deck:          "//Commander\n1 Gimbal, Gremlin Prodigy (MOC) 3\n\n//Deck\n1 Sol Ring (LTC) 1\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Sol Ring",
		},
		{
			name:          "// COMMANDER header (spaced, upper) also works",
			deck:          "// COMMANDER\n1 Gimbal, Gremlin Prodigy\n1 Sol Ring\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Sol Ring",
		},
		{
			name:          "genuine comment line is ignored",
			deck:          "// some note about the deck\n1 Sol Ring (LTC) 1\nCOMMANDER: Gimbal, Gremlin Prodigy\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Sol Ring",
			wantCardOut:   "some note about the deck",
		},
		{
			name:          "normal card line with set code still strips (regression guard)",
			deck:          "3 Lightning Bolt (2XM) 129\nCOMMANDER: Gimbal, Gremlin Prodigy\n",
			wantCommander: "Gimbal, Gremlin Prodigy",
			wantCardIn:    "Lightning Bolt",
			wantCardOut:   "Lightning Bolt (2XM) 129",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempDeck(t, tc.deck)
			cards, commander, err := parseDeckList(path)
			if err != nil {
				t.Fatalf("parseDeckList: %v", err)
			}
			if commander != tc.wantCommander {
				t.Errorf("commander = %q, want %q", commander, tc.wantCommander)
			}
			if tc.wantCardIn != "" && !containsCard(cards, tc.wantCardIn) {
				t.Errorf("card %q missing from %v", tc.wantCardIn, cards)
			}
			if tc.wantCardOut != "" && containsCard(cards, tc.wantCardOut) {
				t.Errorf("card %q unexpectedly present in %v", tc.wantCardOut, cards)
			}
			// A captured commander must be reconciled into the card list.
			if tc.wantCommander != "" && !containsCard(cards, tc.wantCommander) {
				t.Errorf("commander %q not reconciled into card list %v", tc.wantCommander, cards)
			}
		})
	}
}

// TestParseDeckListWithQuantities_CommanderHeaderExcluded verifies the
// second parse pass agrees with the first: a `//Commander`-header
// commander is excluded from the quantity map (like the COMMANDER:
// directive form), and set codes are stripped from normal lines.
func TestParseDeckListWithQuantities_CommanderHeaderExcluded(t *testing.T) {
	deck := "//Commander\n1 Gimbal, Gremlin Prodigy (MOC) 3\n1 Sol Ring (LTC) 1\n7 Island\n"
	path := writeTempDeck(t, deck)
	qtys, err := parseDeckListWithQuantities(path)
	if err != nil {
		t.Fatalf("parseDeckListWithQuantities: %v", err)
	}
	if _, ok := qtys["Gimbal, Gremlin Prodigy"]; ok {
		t.Errorf("commander should be excluded from quantity map, got %v", qtys)
	}
	if qtys["Sol Ring"] != 1 {
		t.Errorf("Sol Ring qty = %d, want 1 (set code should strip)", qtys["Sol Ring"])
	}
	if qtys["Island"] != 7 {
		t.Errorf("Island qty = %d, want 7", qtys["Island"])
	}
}
