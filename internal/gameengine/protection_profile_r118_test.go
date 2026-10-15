package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r118 — protectionColors and ProtectionTypes were folded into a single
// one-pass, lazily-allocating protectionProfile because DeclareBlockers
// evaluated them once per (attacker, blocker) PAIR. The change is meant to
// be semantics-preserving, so these are characterization tests over every
// input vocabulary the two functions accept.
//
// The AST PLURAL forms are the reason this file exists rather than a
// mutation check alone. Nothing in the suite covered them, and the first
// draft of the fix derived the singular key by substring containment —
// which silently works for "creatures"/"artifacts"/"lands" and silently
// FAILS for "sorceries", because "from sorceries" does not contain
// "from sorcery". The bug passed the whole existing protection suite.

func r118Prot(t *testing.T, flags map[string]int, raws ...string) *Permanent {
	t.Helper()
	c := &Card{Name: "r118 Subject", BasePower: 2, BaseToughness: 2,
		Types: []string{"creature"}}
	if len(raws) > 0 {
		abil := make([]gameast.Ability, 0, len(raws))
		for _, raw := range raws {
			abil = append(abil, &gameast.Keyword{Name: "protection", Raw: raw})
		}
		c.AST = &gameast.CardAST{Name: c.Name, Abilities: abil}
	}
	if flags == nil {
		flags = map[string]int{}
	}
	return &Permanent{Card: c, Counters: map[string]int{}, Flags: flags}
}

func r118HasColor(p *Permanent, want string) bool {
	_, ok := protectionColors(p)[want]
	return ok
}

func r118HasType(p *Permanent, want string) bool {
	_, ok := ProtectionTypes(p)[want]
	return ok
}

// Direction 1: every type word the parser can emit, singular AND plural,
// resolves to the singular key. Direction 2: a word that is not present
// does not appear. Both halves run for all seven types.
func TestProtectionProfileR118_ASTTypeWordsSingularAndPlural(t *testing.T) {
	cases := []struct{ printed, key string }{
		{"creature", "creature"}, {"creatures", "creature"},
		{"instant", "instant"}, {"instants", "instant"},
		{"sorcery", "sorcery"}, {"sorceries", "sorcery"},
		{"enchantment", "enchantment"}, {"enchantments", "enchantment"},
		{"artifact", "artifact"}, {"artifacts", "artifact"},
		{"planeswalker", "planeswalker"}, {"planeswalkers", "planeswalker"},
		{"land", "land"}, {"lands", "land"},
	}
	for _, tc := range cases {
		p := r118Prot(t, nil, "protection from "+tc.printed)
		if !r118HasType(p, tc.key) {
			t.Errorf("%q must yield protection type %q, got %v",
				tc.printed, tc.key, ProtectionTypes(p))
		}
		// The other direction: an unrelated type must NOT be set.
		other := "enchantment"
		if tc.key == "enchantment" {
			other = "land"
		}
		if r118HasType(p, other) {
			t.Errorf("%q must not yield protection type %q", tc.printed, other)
		}
	}
}

// All three flag vocabularies plus the AST colour text, and the >0 gating
// that differs between the two axes (prot_type: has never been gated on a
// positive value; prot:/protection_from_ always have been).
func TestProtectionProfileR118_FlagVocabularies(t *testing.T) {
	// prot:<LETTER>, the combat representation (Akroma, Animar).
	if p := r118Prot(t, map[string]int{"prot:R": 1}); !r118HasColor(p, "R") {
		t.Error("prot:R must yield colour R")
	}
	// protection_from_<colorname>, the targeting representation (Mother of Runes).
	if p := r118Prot(t, map[string]int{"protection_from_red": 1}); !r118HasColor(p, "R") {
		t.Error("protection_from_red must fold into colour R")
	}
	// The everything sentinel, from both the flag and the AST raw text.
	if p := r118Prot(t, map[string]int{"protection_from_everything": 1}); !r118HasColor(p, "*") {
		t.Error("protection_from_everything must yield the * sentinel")
	}
	if p := r118Prot(t, nil, "Protection from everything"); !r118HasColor(p, "*") {
		t.Error("AST 'from everything' must yield the * sentinel")
	}
	// A zeroed colour flag is NOT protection.
	if p := r118Prot(t, map[string]int{"prot:R": 0}); r118HasColor(p, "R") {
		t.Error("prot:R=0 must not yield colour R")
	}
	// prot_type is read regardless of value — preserved from pre-r118.
	if p := r118Prot(t, map[string]int{"prot_type:artifact": 0}); !r118HasType(p, "artifact") {
		t.Error("prot_type: has never been value-gated; that must not change")
	}
	// A creature with no protection at all yields nothing on either axis.
	p := r118Prot(t, map[string]int{"kw:flying": 1})
	if len(protectionColors(p)) != 0 || len(ProtectionTypes(p)) != 0 {
		t.Errorf("a creature with no protection must yield empty sets, got %v / %v",
			protectionColors(p), ProtectionTypes(p))
	}
}

// Both axes come back from ONE pass, and the colour and type halves do not
// contaminate each other — the specific hazard of merging two functions
// that previously each walked Flags alone.
func TestProtectionProfileR118_BothAxesFromOnePass(t *testing.T) {
	p := r118Prot(t, map[string]int{"prot:B": 1, "prot_type:creature": 1},
		"protection from artifacts")
	colors, types := protectionProfile(p)
	if _, ok := colors["B"]; !ok {
		t.Errorf("colour B missing, got %v", colors)
	}
	if _, ok := types["creature"]; !ok {
		t.Errorf("type creature missing, got %v", types)
	}
	if _, ok := types["artifact"]; !ok {
		t.Errorf("type artifact (AST) missing, got %v", types)
	}
	if _, ok := colors["creature"]; ok {
		t.Error("a type key leaked into the colour set")
	}
	if _, ok := types["B"]; ok {
		t.Error("a colour key leaked into the type set")
	}
	// Nil permanent on both axes, and a nil-map read must not panic.
	cn, tn := protectionProfile(nil)
	if len(cn) != 0 || len(tn) != 0 {
		t.Error("nil permanent must yield empty sets")
	}
	if _, ok := cn["R"]; ok {
		t.Error("reading a nil colour set must yield nothing")
	}
}
