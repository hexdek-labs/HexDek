package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// addMyriadAttacker builds a creature with myriad plus extra copiable
// keywords/abilities (via AST) and a printed P/T, then puts it on the
// battlefield attacking defSeat. Returns the attacker permanent.
func addMyriadAttacker(gs *GameState, seat int, name string, pow, tough int, defSeat int, keywords ...string) *Permanent {
	abilities := make([]gameast.Ability, len(keywords))
	for i, kw := range keywords {
		abilities[i] = &gameast.Keyword{Name: kw}
	}
	card := &Card{
		Name:          name,
		Owner:         seat,
		BasePower:     pow,
		BaseToughness: tough,
		Types:         []string{"creature"},
		Colors:        []string{"R"},
		AST:           &gameast.CardAST{Name: name, Abilities: abilities},
	}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Timestamp:  gs.NextTimestamp(),
		Counters:   map[string]int{},
		Flags:      map[string]int{},
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	setAttackerDefender(p, defSeat)
	return p
}

func myriadTokensOf(gs *GameState, seat int) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Flags != nil && p.Flags["myriad_token"] == 1 {
			out = append(out, p)
		}
	}
	return out
}

// TestMyriad_TokensAreFaithfulCopies pins CR §702.116a / §707.2 — a myriad
// token is a COPY of the attacker and must carry its copiable values
// (P/T, colors, and especially the rules text / keywords). Before the r63
// fix ApplyMyriad hand-rolled a partial copy that built a bare Card with
// NO AST, dropping every keyword and ability — a myriad copy of a flying
// trampling creature was a vanilla creature. The fix routes through the
// canonical MintTokenAsCopyOf token-copy chokepoint.
//
// Properties exercised: (a) one token per OTHER opponent; (d) copiable
// values preserved; (b) the copy carries the inherited myriad keyword yet
// does NOT re-trigger (token count stays at the opponent count).
func TestMyriad_TokensAreFaithfulCopies(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(42)), nil)
	// Seat 0 attacks seat 1; copies should be made for seats 2 and 3.
	atk := addMyriadAttacker(gs, 0, "Skyfire Wyrm", 4, 4, 1, "myriad", "flying", "trample")

	ApplyMyriad(gs, atk, 0)

	tokens := myriadTokensOf(gs, 0)
	// (a) exactly one token per OTHER opponent (seats 2, 3).
	if len(tokens) != 2 {
		t.Fatalf("(a) expected 2 myriad tokens for the 2 other opponents, got %d", len(tokens))
	}

	for _, tok := range tokens {
		// (d) copiable values preserved.
		if tok.Card == nil || tok.Card.AST == nil {
			t.Fatalf("(d) myriad token lost its AST (copiable rules text) — not a faithful copy")
		}
		if !tok.HasKeyword("flying") || !tok.HasKeyword("trample") {
			t.Errorf("(d) myriad token should inherit flying+trample, has flying=%v trample=%v",
				tok.HasKeyword("flying"), tok.HasKeyword("trample"))
		}
		if tok.Card.BasePower != 4 || tok.Card.BaseToughness != 4 {
			t.Errorf("(d) myriad token P/T should copy 4/4, got %d/%d",
				tok.Card.BasePower, tok.Card.BaseToughness)
		}
		if len(tok.Card.Colors) == 0 || tok.Card.Colors[0] != "R" {
			t.Errorf("(d) myriad token should copy color R, got %v", tok.Card.Colors)
		}
		if !tok.IsToken() {
			t.Errorf("(d) myriad copy must be a token")
		}
		// (b) the copy carries myriad (copiable) but must NOT have
		// re-triggered — guarded by the count==2 check above.
		if !tok.HasKeyword("myriad") {
			t.Errorf("(b/d) faithful copy should carry the inherited myriad keyword")
		}
	}
}

// TestMyriad_TokensEnterTappedAttackingWithoutRetrigger pins property (b):
// each token enters TAPPED and ATTACKING the respective other opponent
// (seats 2 and 3), and because it is PUT into combat (not declared) it does
// not re-trigger attack abilities — even though it now carries myriad.
func TestMyriad_TokensEnterTappedAttackingWithoutRetrigger(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	atk := addMyriadAttacker(gs, 0, "Hydra Broodlord", 5, 5, 1, "myriad")

	ApplyMyriad(gs, atk, 0)

	tokens := myriadTokensOf(gs, 0)
	if len(tokens) != 2 {
		t.Fatalf("(b) expected exactly 2 tokens (no myriad re-trigger cascade), got %d", len(tokens))
	}
	seenDefenders := map[int]bool{}
	for _, tok := range tokens {
		if !tok.Tapped {
			t.Errorf("(b) myriad token should enter tapped")
		}
		if tok.Flags["attacking"] != 1 {
			t.Errorf("(b) myriad token should be attacking")
		}
		def, _ := AttackerDefender(tok)
		if def == 1 {
			t.Errorf("(b) myriad token should attack an OTHER opponent, not the original defender (seat 1)")
		}
		seenDefenders[def] = true
	}
	// One copy each attacking seats 2 and 3.
	if !seenDefenders[2] || !seenDefenders[3] {
		t.Errorf("(b) myriad copies should attack seats 2 and 3, got defenders %v", seenDefenders)
	}
}

// TestMyriad_TokensExiledAtEndOfCombat pins property (c): the delayed
// trigger exiles the myriad tokens at end of combat; the original remains.
func TestMyriad_TokensExiledAtEndOfCombat(t *testing.T) {
	gs := NewGameState(4, rand.New(rand.NewSource(99)), nil)
	atk := addMyriadAttacker(gs, 0, "Comet Drake", 3, 3, 1, "myriad")

	ApplyMyriad(gs, atk, 0)
	if len(myriadTokensOf(gs, 0)) != 2 {
		t.Fatalf("(c) setup: expected 2 tokens, got %d", len(myriadTokensOf(gs, 0)))
	}

	// Fire the end-of-combat delayed trigger.
	FireDelayedTriggers(gs, "combat", "end_of_combat")

	if got := len(myriadTokensOf(gs, 0)); got != 0 {
		t.Errorf("(c) myriad tokens should be exiled at end of combat, %d remain", got)
	}
	// (f) the original myriad creature is untouched — still on the battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == atk {
			found = true
		}
	}
	if !found {
		t.Errorf("(f) the original myriad attacker must NOT be exiled")
	}
}

// TestMyriad_ScalesWithOpponentCount pins property (e): with a single
// opponent no extra tokens are made; it scales to 2 tokens with 3 opponents.
func TestMyriad_ScalesWithOpponentCount(t *testing.T) {
	// 2-player: the only opponent IS the defender → 0 extra tokens.
	gs2 := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	atk2 := addMyriadAttacker(gs2, 0, "Lone Myriad", 2, 2, 1, "myriad")
	ApplyMyriad(gs2, atk2, 0)
	if got := len(myriadTokensOf(gs2, 0)); got != 0 {
		t.Errorf("(e) 1 opponent should make 0 myriad tokens, got %d", got)
	}

	// 4-player: 3 opponents, attacking one → 2 tokens.
	gs4 := NewGameState(4, rand.New(rand.NewSource(2)), nil)
	atk4 := addMyriadAttacker(gs4, 0, "Wide Myriad", 2, 2, 1, "myriad")
	ApplyMyriad(gs4, atk4, 0)
	if got := len(myriadTokensOf(gs4, 0)); got != 2 {
		t.Errorf("(e) 3 opponents should make 2 myriad tokens, got %d", got)
	}
}
