package main

// Regression for the chaos-harness DFC build gap (legality sweep round 3,
// set-aside #2): buildCardFromName missed on full "Front // Back" oracle
// names when the corpus/meta key the single-face name, and the caller
// then fabricated a bare-bones CMC-0 typeless card — chaos games "cast"
// those DFCs for announced-0 and the typeless cards rode odd cast paths
// (the 117.1a non-active sub-shape pollution). The fix retries the
// lookup per face — front first, then back — before giving up.

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// dfcMeta builds a MetaDB whose entries are keyed by SINGLE-FACE names,
// mirroring the corpus shape that produced the live misses.
func dfcMeta(t *testing.T) *deckparser.MetaDB {
	t.Helper()
	rows := strings.Join([]string{
		// Front face only — full-name lookups must fall back to this.
		`{"name":"Test Packleader","type_line":"Creature — Wolf","mana_cost":"{2}{G}","cmc":3,"colors":["G"],"power":"3","toughness":"3"}`,
		// Back-face-only entry: front face absent, back face present.
		`{"name":"Test Moonform","type_line":"Creature — Werewolf","mana_cost":"","cmc":0,"colors":[],"power":"5","toughness":"5"}`,
	}, "\n")
	meta, err := deckparser.LoadMetaReader(strings.NewReader(rows))
	if err != nil {
		t.Fatalf("LoadMetaReader: %v", err)
	}
	return meta
}

// Full DFC name whose FRONT face is in meta must build the front face as
// a real typed card — not nil (which the caller turns into bare-bones).
func TestBuildCardFromName_DFCFullName_ResolvesFrontFace(t *testing.T) {
	meta := dfcMeta(t)
	c := buildCardFromName("Test Packleader // Test Apexwolf", nil, meta)
	if c == nil {
		t.Fatal("full DFC name returned nil — front-face fallback missing; caller would fabricate a bare-bones CMC-0 typeless card")
	}
	if c.CMC != 3 {
		t.Errorf("CMC = %d (want 3 from the front face) — bare-bones build", c.CMC)
	}
	hasCreature := false
	for _, ty := range c.Types {
		if ty == "creature" {
			hasCreature = true
			break
		}
	}
	if !hasCreature {
		t.Errorf("Types = %v (want creature) — type data lost", c.Types)
	}
	if c.Name != "Test Packleader" {
		t.Errorf("Name = %q (want the resolved front-face name)", c.Name)
	}
}

// When only the BACK face is in meta, the fallback must still resolve
// (front tried first, then back).
func TestBuildCardFromName_DFCFullName_FallsBackToBackFace(t *testing.T) {
	meta := dfcMeta(t)
	c := buildCardFromName("Test Dayform // Test Moonform", nil, meta)
	if c == nil {
		t.Fatal("full DFC name returned nil despite back face present in meta")
	}
	if c.Name != "Test Moonform" || c.BasePower != 5 {
		t.Errorf("resolved %q P=%d (want Test Moonform, P=5)", c.Name, c.BasePower)
	}
}

// A genuinely unknown name (no face hits) must still return nil — the
// fallback must not invent cards.
func TestBuildCardFromName_UnknownDFCName_StillNil(t *testing.T) {
	meta := dfcMeta(t)
	if c := buildCardFromName("Nope // Also Nope", nil, meta); c != nil {
		t.Errorf("unknown DFC name should return nil, got %q", c.Name)
	}
	// And a plain unknown single name stays nil (no behavior change).
	if c := buildCardFromName("Nope", nil, meta); c != nil {
		t.Errorf("unknown single name should return nil, got %q", c.Name)
	}
}
