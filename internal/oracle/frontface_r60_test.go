package oracle

import "testing"

// R60 — front-face P/T helpers consumed by downstream renderers + the
// live-game P/T hydration sanity-checks the same parse against raw
// Scryfall string values.

func TestFrontFacePower_SingleFaced(t *testing.T) {
	c := &Card{Name: "Serra Angel", Power: "4", Toughness: "4"}
	if p := c.FrontFacePower(); p != 4 {
		t.Fatalf("Serra Angel power: want 4, got %d", p)
	}
	if t2 := c.FrontFaceToughness(); t2 != 4 {
		t.Fatalf("Serra Angel toughness: want 4, got %d", t2)
	}
}

func TestFrontFacePower_DFCPicksFace0(t *testing.T) {
	// Delver of Secrets — front face 1/1, back face 3/2. Top-level P/T
	// is empty (Scryfall convention for DFCs); helper picks face[0].
	c := &Card{
		Name: "Delver of Secrets // Insectile Aberration",
		CardFaces: []CardFace{
			{Name: "Delver of Secrets", Power: "1", Toughness: "1"},
			{Name: "Insectile Aberration", Power: "3", Toughness: "2"},
		},
	}
	if p := c.FrontFacePower(); p != 1 {
		t.Fatalf("DFC front-face power: want 1, got %d", p)
	}
	if t2 := c.FrontFaceToughness(); t2 != 1 {
		t.Fatalf("DFC front-face toughness: want 1 (not the 2 of the back face), got %d", t2)
	}
}

func TestParsePT_NonIntegerCollapsesToZero(t *testing.T) {
	// Tarmogoyf-style "*" power → 0 (combat layer applies 1/1 fallback).
	// Same for "X" and "1+*".
	for _, s := range []string{"*", "X", "1+*", "", "abc"} {
		if got := parsePT(s); got != 0 {
			t.Fatalf("parsePT(%q): want 0 (non-integer collapse), got %d", s, got)
		}
	}
}

func TestParsePT_IntegerPath(t *testing.T) {
	cases := map[string]int{
		"0":  0,
		"1":  1,
		"12": 12,
		" 4": 4, // tolerates whitespace
		"-1": 0, // negative clamps to 0 (no printed creature has neg base P)
	}
	for in, want := range cases {
		if got := parsePT(in); got != want {
			t.Fatalf("parsePT(%q): want %d, got %d", in, want, got)
		}
	}
}
