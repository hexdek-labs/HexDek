package hexapi

import "testing"

// TestNormalizeCommanderName pins the import-side commander cleaning that
// makes a plain-text paste never error on a leaked quantity / set-code.
// Root cause of the "COMMANDER \"1 KING OF THE OATHBREAKERS\" NOT FOUND"
// false-ILLEGAL: the qty rode the COMMANDER: line into the stored record.
// Casing is preserved for display; a genuine 4-digit-prefixed name
// ("1996 World Champion") is left intact.
func TestNormalizeCommanderName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1 King of the Oathbreakers", "King of the Oathbreakers"},
		{"3x Sol Ring", "Sol Ring"},
		{"Atraxa, Praetors' Voice (CMR) 28", "Atraxa, Praetors' Voice"},
		{"Atraxa, Praetors' Voice (CMR) 28 *F*", "Atraxa, Praetors' Voice"},
		{"1 Gimbal, Gremlin Prodigy (MOC) 3", "Gimbal, Gremlin Prodigy"},
		{"12 Swamp", "Swamp"},
		{"  2  Llanowar Elves ", "Llanowar Elves"},
		{"King of the Oathbreakers", "King of the Oathbreakers"}, // already clean
		{"1996 World Champion", "1996 World Champion"},           // 4-digit prefix is NOT a quantity
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeCommanderName(c.in); got != c.want {
			t.Errorf("normalizeCommanderName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
