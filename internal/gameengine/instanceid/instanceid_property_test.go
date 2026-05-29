package instanceid

import (
	"testing"
)

// TestProperty_OGMintUniqueness — section 3 invariant: no two InstanceIDs
// collide within a (game, seat). Mints 1000 OG IDs per seat across a
// 4-seat minter and asserts zero collisions.
func TestProperty_OGMintUniqueness(t *testing.T) {
	m := NewMinter(4)
	seen := make(map[string]struct{}, 4000)
	for seat := 0; seat < 4; seat++ {
		for i := 0; i < 1000; i++ {
			id := m.Mint(seat, ProvOG, Visible, "C", 3)
			if id == "" {
				t.Fatalf("seat=%d i=%d: empty id from Mint", seat, i)
			}
			if _, dup := seen[id]; dup {
				t.Fatalf("seat=%d i=%d: duplicate id %q", seat, i, id)
			}
			seen[id] = struct{}{}
		}
	}
	if len(seen) != 4000 {
		t.Fatalf("want 4000 unique ids, got %d", len(seen))
	}
}

// TestProperty_FormatRegex — section 3 invariant: every minted ID
// matches the canonical format regex. Walks every (prov, vis, color, cmc)
// combination across all four seats.
func TestProperty_FormatRegex(t *testing.T) {
	provs := []Provenance{ProvOG, ProvTK, ProvCP, ProvAB}
	viss := []Visibility{Visible, Hidden}
	colors := []string{"W", "U", "B", "R", "G", "C", "WU", "WUBRG", "BR", "UG"}
	cmcs := []int{0, 1, 3, 7, 16}

	for seat := 0; seat < 4; seat++ {
		m := NewMinter(4)
		for _, p := range provs {
			for _, v := range viss {
				for _, c := range colors {
					for _, cmc := range cmcs {
						id := m.Mint(seat, p, v, c, cmc)
						if !FormatRegex.MatchString(id) {
							t.Fatalf("seat=%d prov=%v vis=%v color=%q cmc=%d: id %q does not match FormatRegex",
								seat, p, v, c, cmc, id)
						}
					}
				}
			}
		}
	}
}

// TestProperty_OGLineageEmpty — section 3 invariant: OG-provenance cards
// have empty SourceInstanceID and EnablerInstanceID. This package mints
// IDs; the lineage-empty contract is enforced at the Card-struct callsite
// (deck-load). This test pins the wider invariant by demonstrating that
// FormatID accepts OG without any lineage args (it doesn't take them),
// and Mint produces a valid OG ID with no embedded source/enabler bytes.
func TestProperty_OGLineageEmpty(t *testing.T) {
	m := NewMinter(2)
	id := m.Mint(0, ProvOG, Visible, "R", 1)
	if id == "" {
		t.Fatalf("OG mint returned empty id")
	}
	// FormatRegex.FindStringSubmatch[1] is the provenance group.
	groups := FormatRegex.FindStringSubmatch(id)
	if len(groups) < 2 {
		t.Fatalf("regex did not capture provenance group from %q", id)
	}
	if groups[1] != "OG" {
		t.Fatalf("want provenance OG, got %q from id %q", groups[1], id)
	}
}

// TestProperty_Seq5Monotonic — section 3 invariant: seq5 is per-(game,
// seat) monotonic. Mints 50 IDs in one seat and verifies the embedded
// seq5 strictly increases by 1 each time. Also verifies seats are
// independent — seat 1's counter is unaffected by seat 0 mints.
func TestProperty_Seq5Monotonic(t *testing.T) {
	m := NewMinter(2)
	// 30 seat-0 mints. The interesting bit is the LAST 5 digits of each
	// id — that's the seq5 field.
	prev := -1
	for i := 0; i < 30; i++ {
		id := m.Mint(0, ProvOG, Visible, "C", 2)
		if len(id) < 5 {
			t.Fatalf("i=%d: id %q too short", i, id)
		}
		seqStr := id[len(id)-5:]
		if len(seqStr) != 5 {
			t.Fatalf("i=%d: seq5 slice wrong length: %q", i, seqStr)
		}
		// Manual atoi to keep this test dependency-free.
		got := 0
		for _, ch := range seqStr {
			if ch < '0' || ch > '9' {
				t.Fatalf("i=%d: non-digit in seq5 %q", i, seqStr)
			}
			got = got*10 + int(ch-'0')
		}
		if got != prev+1 {
			t.Fatalf("i=%d: want seq=%d, got %d (id %q)", i, prev+1, got, id)
		}
		prev = got
	}
	// Seat independence — seat 1's first mint must still be seq=00000.
	id := m.Mint(1, ProvOG, Visible, "C", 2)
	if want := "00000"; id[len(id)-5:] != want {
		t.Fatalf("seat 1 first mint: want seq5=%s, got %s (id %q)", want, id[len(id)-5:], id)
	}
}

// TestCanonicalColor — section 3 invariant: color field is canonical
// WUBRG order. Empty/colorless slice collapses to "C".
func TestCanonicalColor(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "C"},
		{[]string{}, "C"},
		{[]string{""}, "C"},
		{[]string{"W"}, "W"},
		{[]string{"r"}, "R"},
		{[]string{"G", "W"}, "WG"},
		{[]string{"R", "G", "U", "B", "W"}, "WUBRG"},
		{[]string{"W", "W", "W"}, "W"}, // dedup
		{[]string{"x", "W"}, "W"},      // invalid ignored
	}
	for _, tc := range cases {
		got := CanonicalColor(tc.in)
		if got != tc.want {
			t.Errorf("CanonicalColor(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestMinter_OutOfRangeSeat — defensive: mint with an invalid seat returns
// empty without crashing or advancing any counter.
func TestMinter_OutOfRangeSeat(t *testing.T) {
	m := NewMinter(2)
	if id := m.Mint(5, ProvOG, Visible, "C", 0); id != "" {
		t.Fatalf("want empty for out-of-range seat, got %q", id)
	}
	if id := m.Mint(-1, ProvOG, Visible, "C", 0); id != "" {
		t.Fatalf("want empty for negative seat, got %q", id)
	}
	// Counters for valid seats should still read 0.
	if m.Peek(0) != 0 || m.Peek(1) != 0 {
		t.Fatalf("expected counters untouched after invalid mints, got %d/%d", m.Peek(0), m.Peek(1))
	}
}

// TestFormatID_Invalid — pure-function validation: bad inputs produce
// the empty string rather than malformed IDs.
func TestFormatID_Invalid(t *testing.T) {
	if FormatID(0, ProvUnknown, Visible, "C", 0, 0) != "" {
		t.Error("ProvUnknown should produce empty id")
	}
	if FormatID(4, ProvOG, Visible, "C", 0, 0) != "" {
		t.Error("seat 4 should produce empty id (max 0..3)")
	}
	if FormatID(0, ProvOG, Visible, "", 0, 0) != "" {
		t.Error("empty color should produce empty id")
	}
	if FormatID(0, ProvOG, Visible, "C", -1, 0) != "" {
		t.Error("negative cmc should produce empty id")
	}
	if FormatID(0, ProvOG, Visible, "C", 0, -1) != "" {
		t.Error("negative seq should produce empty id")
	}
}
