package per_card

// r63 — COPY-effect copiable-values audit (CR 707). Token-copy and
// "enters as a copy" handlers that hand-built the copy's Card from only
// P/T+types+colors dropped the source's ABILITIES (AST) entirely — a copy with
// no rules text, whose ETB triggers are blank. CR §707.2: a copy copies the
// source's copiable values, which INCLUDE its abilities/rules text.
//
// Each test gives the copy SOURCE a recognizable ability (a "flying" keyword in
// its AST) and asserts the resulting copy carries it. Before the fix the copy's
// Card.AST was nil.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func astWithFlying() *gameast.CardAST {
	return &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "flying"}}}
}

func astHasKeyword(ast *gameast.CardAST, kw string) bool {
	if ast == nil {
		return false
	}
	for _, ab := range ast.Abilities {
		if k, ok := ab.(*gameast.Keyword); ok && k.Name == kw {
			return true
		}
	}
	return false
}

// Saheeli, Radiant Creator: token copy "except it's a 5/5 artifact … and it has
// haste" — the copy must keep the source's abilities and fire its ETB.
func TestCopy_Saheeli_TokenCopiesAbilities(t *testing.T) {
	gs := newGame(t, 2)
	saheeli := stampCreaturePT(addPerm(gs, 0, "Saheeli, Radiant Creator", "creature", "legendary"), 1, 1)
	gs.Seats[0].Flags = map[string]int{"energy_counters": 3}
	src := stampCreaturePT(addPerm(gs, 0, "Copy Source", "creature"), 3, 3)
	src.Card.CMC = 4
	src.Card.AST = astWithFlying()

	saheeliCombatCopy(gs, saheeli, map[string]interface{}{})

	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p == saheeli || p == src || p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "token") {
			tok = p
			break
		}
	}
	if tok == nil {
		t.Fatalf("Saheeli created no token copy")
	}
	if !astHasKeyword(tok.Card.AST, "flying") {
		t.Fatalf("Saheeli token copy dropped the source's abilities (no flying in AST)")
	}
	if tok.Card.BasePower != 5 || tok.Card.BaseToughness != 5 {
		t.Fatalf("Saheeli token should be 5/5, got %d/%d", tok.Card.BasePower, tok.Card.BaseToughness)
	}
}

// Trostani, Selesnya's Voice — populate: "create a token that's a copy of a
// creature token you control." The copy must carry the source token's abilities.
func TestCopy_Trostani_PopulateCopiesAbilities(t *testing.T) {
	gs := newGame(t, 2)
	trostani := addPerm(gs, 0, "Trostani, Selesnya's Voice", "creature", "legendary")
	srcTok := stampCreaturePT(addPerm(gs, 0, "Beast Token", "creature", "token"), 3, 3)
	srcTok.Card.AST = astWithFlying()

	// Count permanents carrying the source's signature ability (flying) before
	// and after — a populated copy must add one more flying-bearing creature.
	// (Counting flying-AST perms rather than total battlefield size dodges the
	// bare-harness InstanceID-uniqueness sweep that prunes empty-ID test perms.)
	flyersBefore := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && astHasKeyword(p.Card.AST, "flying") {
			flyersBefore++
		}
	}

	trostaniVoicePopulate(gs, trostani, 0, map[string]interface{}{})

	flyersAfter := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && astHasKeyword(p.Card.AST, "flying") {
			flyersAfter++
		}
	}
	if flyersAfter != flyersBefore+1 {
		t.Fatalf("populate copy dropped the source token's abilities: flying-bearing creatures %d → %d (want +1)", flyersBefore, flyersAfter)
	}
}

// The Mimeoplasm — "enters as a copy of one of those [exiled] cards with +1/+1
// counters equal to the power of the other." The copy must carry the chosen
// card's abilities; the counters are the copy's own.
func TestCopy_Mimeoplasm_EntersAsCopyWithAbilities(t *testing.T) {
	gs := newGame(t, 2)
	mime := stampCreaturePT(addPerm(gs, 0, "The Mimeoplasm", "creature", "legendary"), 0, 0)
	big := &gameengine.Card{Name: "Big Beast", Owner: 0, Types: []string{"creature"}, BasePower: 6, BaseToughness: 6, AST: astWithFlying()}
	small := &gameengine.Card{Name: "Small Beast", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	gs.Seats[0].Graveyard = []*gameengine.Card{big, small}

	theMimeoplasmETB(gs, mime)

	if mime.Card.Name != "Big Beast" {
		t.Fatalf("Mimeoplasm should enter as a copy of the higher-power card; got %q", mime.Card.Name)
	}
	if !astHasKeyword(mime.Card.AST, "flying") {
		t.Fatalf("Mimeoplasm copy dropped the chosen card's abilities (no flying in AST)")
	}
	if got := mime.Counters["+1/+1"]; got != 2 {
		t.Fatalf("Mimeoplasm should get +1/+1 counters = power of the other card (2); got %d", got)
	}
}
