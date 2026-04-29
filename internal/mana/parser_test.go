package mana

import "testing"

func TestParseEmpty(t *testing.T) {
	c, err := Parse("")
	if err != nil {
		t.Fatalf("expected nil error for empty cost, got %v", err)
	}
	if c.CMC() != 0 {
		t.Errorf("empty cost CMC: got %d, want 0", c.CMC())
	}
}

func TestParseGeneric(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"{0}", 0},
		{"{1}", 1},
		{"{16}", 16},
		{"{20}", 20},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", c.in, err)
		}
		if got.CMC() != c.want {
			t.Errorf("Parse(%q).CMC() = %d, want %d", c.in, got.CMC(), c.want)
		}
		if got.Generic != c.want {
			t.Errorf("Parse(%q).Generic = %d, want %d", c.in, got.Generic, c.want)
		}
	}
}

func TestParseColors(t *testing.T) {
	cases := []struct {
		in    string
		color Color
		count int
	}{
		{"{W}", White, 1},
		{"{U}", Blue, 1},
		{"{B}", Black, 1},
		{"{R}", Red, 1},
		{"{G}", Green, 1},
		{"{C}", Colorless, 1},
		{"{B}{B}{B}", Black, 3},
		{"{U}{U}", Blue, 2},
	}
	for _, tc := range cases {
		got, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", tc.in, err)
		}
		if got.Colored[tc.color] != tc.count {
			t.Errorf("Parse(%q).Colored[%s] = %d, want %d", tc.in, tc.color, got.Colored[tc.color], tc.count)
		}
	}
}

func TestParseMixed(t *testing.T) {
	c, err := Parse("{2}{U}{B}")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if c.Generic != 2 {
		t.Errorf("Generic = %d, want 2", c.Generic)
	}
	if c.Colored[Blue] != 1 {
		t.Errorf("Blue count = %d, want 1", c.Colored[Blue])
	}
	if c.Colored[Black] != 1 {
		t.Errorf("Black count = %d, want 1", c.Colored[Black])
	}
	if c.CMC() != 4 {
		t.Errorf("CMC = %d, want 4", c.CMC())
	}
}

func TestParseDoomsday(t *testing.T) {
	c, err := Parse("{B}{B}{B}")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if c.CMC() != 3 {
		t.Errorf("Doomsday CMC = %d, want 3", c.CMC())
	}
	if c.Colored[Black] != 3 {
		t.Errorf("Doomsday Black pip count = %d, want 3", c.Colored[Black])
	}
}

func TestParseDraco(t *testing.T) {
	c, err := Parse("{16}")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if c.CMC() != 16 {
		t.Errorf("Draco CMC = %d, want 16", c.CMC())
	}
}

func TestParseXSpell(t *testing.T) {
	c, err := Parse("{X}{B}{B}")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if c.Variable != 1 {
		t.Errorf("Variable = %d, want 1", c.Variable)
	}
	if c.Colored[Black] != 2 {
		t.Errorf("Black = %d, want 2", c.Colored[Black])
	}
	if c.CMC() != 2 {
		t.Errorf("X-spell CMC = %d, want 2 (X treated as 0 for printed CMC)", c.CMC())
	}
}

func TestParseBadInput(t *testing.T) {
	cases := []string{
		"U",          // missing braces
		"{U}{",       // unclosed brace
		"{}",         // empty symbol
		"{HYBRID}",   // unsupported
		"{Z}",        // unknown color
		"{W/U}",      // hybrid (not yet supported)
		"{-1}",       // negative
	}
	for _, tc := range cases {
		_, err := Parse(tc)
		if err == nil {
			t.Errorf("Parse(%q): expected error, got nil", tc)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	cases := []string{
		"{2}{U}{B}",
		"{B}{B}{B}",
		"{16}",
		"{0}",
		"",
	}
	for _, tc := range cases {
		c, err := Parse(tc)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc, err)
		}
		s := c.String()
		c2, err := Parse(s)
		if err != nil {
			t.Fatalf("Re-parse(%q): %v", s, err)
		}
		if c2.CMC() != c.CMC() {
			t.Errorf("round-trip CMC mismatch for %q: original %d, re-parsed %d", tc, c.CMC(), c2.CMC())
		}
	}
}
