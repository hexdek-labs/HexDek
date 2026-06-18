package gameengine

import "testing"

// offspringTokensOf returns the freshly-minted token copies on seat's
// battlefield (any permanent whose *Card differs from the parent's).
func offspringTokensOf(gs *GameState, seat int, parent *Permanent) []*Permanent {
	var out []*Permanent
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || p == parent || p.Card == nil {
			continue
		}
		if p.IsToken() {
			out = append(out, p)
		}
	}
	return out
}

// (1)/(2) Offspring mints a FAITHFUL 1/1 token copy: copiable values carried
// (colors, CMC, subtypes — not just name/types the old hand-rolled token set),
// base P/T overridden to 1/1, "token" type present, and a fresh *Card with its
// own InstanceID (no pointer aliasing back to the parent).
func TestOffspring_FaithfulOneOneCopy(t *testing.T) {
	gs := newCombatGame(t)
	parent := addBattlefield(gs, 0, "Mossborn Hydra", 4, 4, "creature", "beast")
	parent.Card.Colors = []string{"G"}
	parent.Card.CMC = 5

	CreateOffspringToken(gs, parent)

	tokens := offspringTokensOf(gs, 0, parent)
	if len(tokens) != 1 {
		t.Fatalf("offspring should mint exactly one token copy, got %d", len(tokens))
	}
	tok := tokens[0]

	// (2) §702.175a "except it's 1/1" — base P/T overridden.
	if tok.Card.BasePower != 1 || tok.Card.BaseToughness != 1 {
		t.Fatalf("offspring token must be 1/1, got %d/%d", tok.Card.BasePower, tok.Card.BaseToughness)
	}
	// (1) Faithful copy: colors carried.
	if len(tok.Card.Colors) != 1 || tok.Card.Colors[0] != "G" {
		t.Fatalf("offspring token should copy colors [G], got %v", tok.Card.Colors)
	}
	// CMC is a copiable value the old hand-rolled token dropped (→ 0).
	if tok.Card.CMC != 5 {
		t.Fatalf("offspring token should copy CMC 5, got %d", tok.Card.CMC)
	}
	// Types: token + the copied creature/subtype.
	for _, want := range []string{"token", "creature", "beast"} {
		found := false
		for _, ty := range tok.Card.Types {
			if ty == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("offspring token Types missing %q: %v", want, tok.Card.Types)
		}
	}
	// No pointer aliasing: fresh *Card, distinct minted InstanceID.
	if tok.Card == parent.Card {
		t.Fatal("offspring token must be a fresh *Card, not alias the parent's")
	}
	if tok.Card.InstanceID == "" || tok.Card.InstanceID == parent.Card.InstanceID {
		t.Fatalf("offspring token must mint its own InstanceID, got %q (parent %q)",
			tok.Card.InstanceID, parent.Card.InstanceID)
	}

	// Canonical-path log event present.
	saw := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "offspring" && ev.Amount == 1 {
			saw = true
		}
	}
	if !saw {
		t.Fatal("expected an 'offspring' event with Amount 1")
	}
}

// (doublers) Token doublers act on the offspring token — Doubling Season makes
// two 1/1 copies. Proves the mint runs through the canonical
// FireCreateTokenEvent path rather than placing one permanent inline.
func TestOffspring_TokenDoublersApply(t *testing.T) {
	gs := newCombatGame(t)
	parent := addBattlefield(gs, 0, "Mossborn Hydra", 4, 4, "creature")
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)

	CreateOffspringToken(gs, parent)

	tokens := offspringTokensOf(gs, 0, parent)
	if len(tokens) != 2 {
		t.Fatalf("offspring with Doubling Season should mint 2 token copies, got %d", len(tokens))
	}
	for _, tok := range tokens {
		if tok.Card.BasePower != 1 || tok.Card.BaseToughness != 1 {
			t.Fatalf("each doubled offspring token must stay 1/1, got %d/%d",
				tok.Card.BasePower, tok.Card.BaseToughness)
		}
	}
}

// (override safety) The 1/1 override is applied to the COPY, never the parent —
// the original entering creature keeps its printed P/T.
func TestOffspring_DoesNotMutateParent(t *testing.T) {
	gs := newCombatGame(t)
	parent := addBattlefield(gs, 0, "Mossborn Hydra", 4, 4, "creature")

	CreateOffspringToken(gs, parent)

	if parent.Card.BasePower != 4 || parent.Card.BaseToughness != 4 {
		t.Fatalf("parent P/T must be unchanged at 4/4, got %d/%d",
			parent.Card.BasePower, parent.Card.BaseToughness)
	}
}
