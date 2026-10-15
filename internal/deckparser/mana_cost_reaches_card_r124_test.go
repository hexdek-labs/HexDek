package deckparser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// r124 — the printed mana cost must reach the runtime Card.
//
// Card.ManaCostString's own doc comment claims "the corpus loader populates
// this when available". It did not. Every assignment of the field in the
// whole tree was inside a _test.go, so for every real card in every
// production game it was "". CardMeta.ManaCost carried the printed cost the
// entire way (32,791 of 38,754 oracle cards have one) and both card builders
// copied Name, Types, Power, Toughness, Colors, CMC, TypeLine and the MDFC
// back face while dropping this one field.
//
// Every consumer guards on `!= ""`, so all of them were inert in production:
// devotion counted zero pips, the Jegantha companion check ("no card has
// more than one of the same mana symbol") passed vacuously for every deck,
// colour-specific cost modifiers never applied, and the AI's
// colour-requirement and mulligan-bottom signals were blind.
//
// This drives the REAL loader and the REAL builder rather than asserting on
// the struct, because the bug was precisely that the field was never
// assigned on the path between them.

func r124Meta(t *testing.T, rows ...string) *MetaDB {
	t.Helper()
	p := filepath.Join(t.TempDir(), "meta.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := LoadMetaFromJSONL(p)
	if err != nil {
		t.Fatalf("LoadMetaFromJSONL: %v", err)
	}
	return db
}

const (
	r124Bolt   = `{"name":"Lightning Bolt","type_line":"Instant","mana_cost":"{R}","cmc":1,"colors":["R"],"power":"","toughness":""}`
	r124Thassa = `{"name":"Thassa, God of the Sea","type_line":"Legendary Enchantment Creature — God","mana_cost":"{2}{U}","cmc":3,"colors":["U"],"power":"5","toughness":"5"}`
	r124Land   = `{"name":"Island","type_line":"Basic Land — Island","mana_cost":"","cmc":0,"colors":[],"power":"","toughness":""}`
)

// BuildCardFromName is the exported builder (chaos / tournament path).
func TestManaCostReachesCardR124_ChaosBuilder(t *testing.T) {
	db := r124Meta(t, r124Bolt, r124Thassa, r124Land)
	for _, tc := range []struct{ name, want string }{
		{"Lightning Bolt", "{R}"},
		{"Thassa, God of the Sea", "{2}{U}"},
		{"Island", ""}, // lands have no printed cost — must stay empty
	} {
		c := BuildCardFromName(tc.name, nil, db)
		if c == nil {
			t.Fatalf("%s: builder returned nil", tc.name)
		}
		if c.ManaCostString != tc.want {
			t.Errorf("%s: Card.ManaCostString = %q, want %q (CardMeta.ManaCost = %q)",
				tc.name, c.ManaCostString, tc.want, db.Get(tc.name).ManaCost)
		}
	}
}

// The cost that arrives must be the pip string, not the mana value — a
// builder that wrote strconv.Itoa(md.CMC) would satisfy "non-empty" and be
// useless to every consumer, all of which parse pips.
func TestManaCostReachesCardR124_IsPipsNotManaValue(t *testing.T) {
	db := r124Meta(t, r124Thassa)
	c := BuildCardFromName("Thassa, God of the Sea", nil, db)
	if c == nil {
		t.Fatal("builder returned nil")
	}
	if !strings.Contains(c.ManaCostString, "{U}") {
		t.Errorf("ManaCostString = %q — it must carry the coloured pips, "+
			"since every consumer parses them", c.ManaCostString)
	}
	if c.ManaCostString == "3" || c.ManaCostString == "{3}" {
		t.Errorf("ManaCostString = %q — that is the mana value, not the printed cost",
			c.ManaCostString)
	}
}
