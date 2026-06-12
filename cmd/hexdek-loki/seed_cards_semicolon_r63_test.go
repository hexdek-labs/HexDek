package main

// splitCardList regression — --seed-cards must accept the `;` separator
// so card names containing commas ("Anafenza, the Foremost") survive
// parsing, while comma-only invocations keep working.

import (
	"reflect"
	"testing"
)

func TestSplitCardList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "  ", nil},
		{"comma legacy", "Sol Ring, Mana Crypt", []string{"Sol Ring", "Mana Crypt"}},
		{"semicolon preserves commas in names",
			"Rest in Peace; Leyline of the Void; Anafenza, the Foremost",
			[]string{"Rest in Peace", "Leyline of the Void", "Anafenza, the Foremost"}},
		{"semicolon with empty segments", "A;;B; ", []string{"A", "B"}},
	}
	for _, c := range cases {
		if got := splitCardList(c.raw); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: splitCardList(%q) = %#v, want %#v", c.name, c.raw, got, c.want)
		}
	}
}
