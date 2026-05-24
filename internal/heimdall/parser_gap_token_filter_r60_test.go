package heimdall

import (
	"reflect"
	"sort"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 regression: ExtractParserGaps was emitting bare token-type names
// like "Token", "Construct Token", and "creature token scorpion dragon
// Token" verbatim — Muninn's parser_gaps.json showed ~4 noise hits per
// report. Tokens carry no parseable oracle text of their own, so any
// permanent whose display name is a runtime-generated "<types> Token"
// is now filtered out before being recorded as a gap.

func TestIsTypeOnlyTokenName_FiltersEngineGeneratedTokenNames(t *testing.T) {
	filtered := []string{
		"Token",
		"Construct Token",
		"creature token scorpion dragon Token",
		"creature token black zombie giant Token",
		"creature token phyrexian mite Token",
		"Soldier Token",
		"Treasure Token",
		"Spirit Token",
		"Servo Token",
		"Phyrexian Germ Token",
		"1/1 Token Token",
		"Loop Token",
		"Cat Warrior Token",
	}
	for _, n := range filtered {
		if !isTypeOnlyTokenName(n) {
			t.Errorf("expected %q to be filtered as a type-only token name", n)
		}
	}
}

func TestIsTypeOnlyTokenName_KeepsRealCardNames(t *testing.T) {
	kept := []string{
		"Lightning Bolt",
		"Snapcaster Mage",
		"Atraxa, Praetors' Voice",
		"Sol Ring",
		"Krenko, Mob Boss",
		"Cruelclaw",
		"Bear",
		// Edge: "Tokenizer" etc. — contains "Token" as a substring
		// but does NOT end in " Token" / equal "Token".
		"Tokenizer",
		"Forbidden Token Library",
	}
	for _, n := range kept {
		if isTypeOnlyTokenName(n) {
			t.Errorf("expected %q NOT to be filtered (real card name)", n)
		}
	}
}

func TestExtractParserGaps_DropsTokenNoiseKeepsRealCards(t *testing.T) {
	mkPerm := func(name string, gap bool) *gameengine.Permanent {
		p := &gameengine.Permanent{
			Card:  &gameengine.Card{Name: name},
			Flags: map[string]int{},
		}
		if gap {
			p.Flags["parser_gap"] = 1
		}
		return p
	}

	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			{
				Battlefield: []*gameengine.Permanent{
					mkPerm("Snapcaster Mage", true),
					mkPerm("Construct Token", true),
					mkPerm("Token", true),
					mkPerm("creature token scorpion dragon Token", true),
				},
			},
			{
				Battlefield: []*gameengine.Permanent{
					mkPerm("Treasure Token", true),
					mkPerm("Atraxa, Praetors' Voice", true),
					// no gap flag — must not appear regardless of name
					mkPerm("Lightning Bolt", false),
				},
			},
		},
	}

	got := ExtractParserGaps(gs)
	sort.Strings(got)
	want := []string{"Atraxa, Praetors' Voice", "Snapcaster Mage"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractParserGaps = %v, want %v", got, want)
	}
}

func TestExtractParserGaps_AllTokenGapsCollapseToEmpty(t *testing.T) {
	// Pre-fix this game would have reported 4 entries; post-fix none.
	mkTok := func(name string) *gameengine.Permanent {
		return &gameengine.Permanent{
			Card:  &gameengine.Card{Name: name},
			Flags: map[string]int{"parser_gap": 1},
		}
	}
	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			{
				Battlefield: []*gameengine.Permanent{
					mkTok("Token"),
					mkTok("Construct Token"),
					mkTok("creature token scorpion dragon Token"),
					mkTok("Treasure Token"),
				},
			},
		},
	}
	if got := ExtractParserGaps(gs); len(got) != 0 {
		t.Errorf("expected empty gap list after token filter, got %v", got)
	}
}

func TestExtractParserGaps_NilSafe(t *testing.T) {
	if got := ExtractParserGaps(nil); got != nil {
		t.Errorf("ExtractParserGaps(nil) = %v, want nil", got)
	}
	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			nil,
			{Battlefield: []*gameengine.Permanent{nil, {Card: nil}}},
		},
	}
	if got := ExtractParserGaps(gs); got != nil {
		t.Errorf("ExtractParserGaps with nil seat/perm/card = %v, want nil", got)
	}
}
