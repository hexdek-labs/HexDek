package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// copy_token_mint_coverage_r63_test.go — pins the r63 fishtank-radar
// follow-up that converted the last two "create a token that's a copy of
// X" sites in resolveModificationEffect (ModKind "populate" CR §701.30 and
// "copy_pronoun" CR §707.10f) to route through MintTokenAsCopyOf, the
// Phase-5 mint chokepoint.
//
// Before the fix both built the token with a hand-rolled bare &Card{}
// that (1) never minted an InstanceID — leaving a battlefield permanent
// invisible to the (Minted − Ceased) mint census — and (2) copied only a
// handful of characteristics (Name/P/T/Types/Colors[/CMC]), silently
// dropping the AST, keywords, subtypes, TypeLine and mana cost from the
// copy. Every other token-as-copy path (resolveCopyPermanent §707.10f,
// batch_al_clones Spitting Image, Drafna) already routes through
// MintTokenAsCopyOf with the documented warning that a bare DeepCopy
// carries the source's ID forward and trips the CardIdentity invariant.
//
// These two were the residual unconverted sites surfaced while auditing
// the r63 grinder's recurring basic-land same-seat CardIdentity cluster.

func TestPopulate_TokenIsMintedFaithfulCopy(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(42)), nil)
	if gs.IIDMinter == nil {
		t.Fatal("expected non-nil IIDMinter")
	}

	// Source: a creature token the controller owns, with characteristics
	// the old bare-&Card{} populate path would have dropped.
	srcTok := &Card{
		Name:          "Beast",
		Owner:         0,
		Types:         []string{"token", "creature"},
		Colors:        []string{"G"},
		BasePower:     3,
		BaseToughness: 3,
		CMC:           4,
		TypeLine:      "Token Creature — Beast",
		InstanceID:    "TKbeastSRC",
	}
	srcPerm := &Permanent{Card: srcTok, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, srcPerm)

	// Effect source: a permanent that resolves the populate effect.
	effSrc := &Permanent{Card: &Card{Name: "Trostani", Owner: 0,
		Types: []string{"creature"}, InstanceID: "OGtrostani"},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, effSrc)

	before := len(gs.Seats[0].Battlefield)
	resolveModificationEffect(gs, effSrc, &gameast.ModificationEffect{ModKind: "populate"})

	if got := len(gs.Seats[0].Battlefield); got != before+1 {
		t.Fatalf("populate should add exactly one token: battlefield %d → %d", before, got)
	}
	tok := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	assertMintedCopy(t, gs, tok, srcTok, "populate")
	// Faithfulness: characteristics the old partial copy dropped.
	if tok.Card.TypeLine != srcTok.TypeLine {
		t.Errorf("populate token lost TypeLine: got %q want %q", tok.Card.TypeLine, srcTok.TypeLine)
	}
	if tok.Card.CMC != srcTok.CMC {
		t.Errorf("populate token lost CMC: got %d want %d", tok.Card.CMC, srcTok.CMC)
	}
}

func TestCopyPronoun_TokenIsMintedFaithfulCopy(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	if gs.IIDMinter == nil {
		t.Fatal("expected non-nil IIDMinter")
	}

	// copy_pronoun copies the effect source itself. Give it an OG-provenance
	// InstanceID so a bare DeepCopy (the old bug) would alias an OG id on
	// the same battlefield — the exact CardIdentity same-seat shape.
	srcCard := &Card{
		Name:           "Scute Swarm",
		Owner:          0,
		Types:          []string{"creature", "insect"},
		Colors:         []string{"G"},
		BasePower:      1,
		BaseToughness:  1,
		TypeLine:       "Creature — Insect",
		ManaCostString: "{1}{G}",
		InstanceID:     "h0OGVG000099",
	}
	src := &Permanent{Card: srcCard, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	before := len(gs.Seats[0].Battlefield)
	resolveModificationEffect(gs, src, &gameast.ModificationEffect{ModKind: "copy_pronoun"})

	if got := len(gs.Seats[0].Battlefield); got != before+1 {
		t.Fatalf("copy_pronoun should add exactly one token: battlefield %d → %d", before, got)
	}
	tok := gs.Seats[0].Battlefield[len(gs.Seats[0].Battlefield)-1]
	assertMintedCopy(t, gs, tok, srcCard, "copy_pronoun")
	if tok.Card.TypeLine != srcCard.TypeLine {
		t.Errorf("copy_pronoun token lost TypeLine: got %q want %q", tok.Card.TypeLine, srcCard.TypeLine)
	}
	if tok.Card.ManaCostString != srcCard.ManaCostString {
		t.Errorf("copy_pronoun token lost ManaCostString: got %q want %q", tok.Card.ManaCostString, srcCard.ManaCostString)
	}
}

// assertMintedCopy enforces the core invariants of the Phase-5 mint
// chokepoint: a fresh non-empty InstanceID distinct from the source, the
// "token" type stamped, the source recorded as lineage, and — the bug the
// grinder cluster watches — no CardIdentity duplication on the battlefield.
func assertMintedCopy(t *testing.T, gs *GameState, tok *Permanent, srcCard *Card, label string) {
	t.Helper()
	if tok == nil || tok.Card == nil {
		t.Fatalf("%s: nil token permanent", label)
	}
	if tok.Card.InstanceID == "" {
		t.Errorf("%s: token has empty InstanceID — bypassed the mint chokepoint", label)
	}
	if tok.Card.InstanceID == srcCard.InstanceID {
		t.Errorf("%s: token aliases the source InstanceID %q (DeepCopy-without-remint)", label, srcCard.InstanceID)
	}
	if !tok.IsToken() {
		t.Errorf("%s: copy must be a token per CR §707.10f, Types=%v", label, tok.Card.Types)
	}
	if tok.Card.SourceInstanceID != srcCard.InstanceID {
		t.Errorf("%s: token lineage SourceInstanceID=%q want %q", label, tok.Card.SourceInstanceID, srcCard.InstanceID)
	}
	if err := checkCardIdentity(gs); err != nil {
		t.Errorf("%s: CardIdentity violated after token copy: %v", label, err)
	}
}
