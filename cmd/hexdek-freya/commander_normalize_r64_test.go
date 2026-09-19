package main

import "testing"

// TestNormalizeCardLookupName pins the fallback name-cleaning used when a raw
// oracle lookup fails — the fix for false-ILLEGAL commanders that kept a
// leading quantity or a "(SET) N" suffix from a plain-text paste/import.
func TestNormalizeCardLookupName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1 King of the Oathbreakers", "King of the Oathbreakers"},
		{"3x Sol Ring", "Sol Ring"},
		{"Sol Ring (LEA) 1", "Sol Ring"},
		{"1 Gimbal, Gremlin Prodigy (MOC) 3", "Gimbal, Gremlin Prodigy"},
		{"12 Swamp", "Swamp"},
		{"  2  Llanowar Elves ", "Llanowar Elves"},
		{"King of the Oathbreakers", "King of the Oathbreakers"}, // already clean
		{"1996 World Champion", "1996 World Champion"},           // 4-digit prefix is NOT a quantity
	}
	for _, c := range cases {
		if got := normalizeCardLookupName(c.in); got != c.want {
			t.Errorf("normalizeCardLookupName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
