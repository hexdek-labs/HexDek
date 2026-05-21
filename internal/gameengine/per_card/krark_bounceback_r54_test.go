package per_card

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R54 regression — Krark, the Thumbless lose-flip bounce no longer
// duplicates the cast card when the spell has already left the stack
// by the time the trigger resolves (CR §608.2c deferred-trigger
// drain via gs.pendingTriggers).
//
// Loki r53 lead 1: Glyph of Destruction (ptr 0xc006c110e0) in game 490
// appeared in both seat 3 hand AND seat 3 graveyard. Root cause: Krark's
// lose branch called MoveCard(card, owner, "stack", "hand") with
// fromZone="stack" — but removeCardFromZone("stack") is intentionally a
// no-op (zone_move.go:239, source removal is caller's responsibility).
// When stackIdx < 0 (spell already resolved into graveyard), the bounce
// appended to hand without removing from graveyard, leaking the *Card
// pointer across two zones.
//
// CR-correct: §608.2b — at resolution, if the target spell is no longer
// on the stack, the bounce does nothing for that target.

// forceKrarkLoseFlip pins the global math/rand source so the next
// rand.Intn(2) returns 0 ("lose"). Seed 1 → first call returns 0 per Go
// stdlib determinism; verified empirically.
func forceKrarkLoseFlip(t *testing.T) {
	t.Helper()
	// R55: krarkTrigger now prefers gs.Rng (per-game RNG) so it isn't
	// at the mercy of global math/rand state. The tests in this file
	// re-seed BOTH the global stream (for any caller that still falls
	// through to it) and overwrite gs.Rng before calling krarkTrigger.
	rand.Seed(2) // legacy global-rand fallback
}

// forceKrarkLoseFlipOn pins gs.Rng to a fresh source so krarkTrigger's
// rand.Intn(2) returns 0 ("lose") deterministically. Empirically:
// NewSource(2).Intn(2) → 0 on the first draw.
func forceKrarkLoseFlipOn(t *testing.T, gs *gameengine.GameState) {
	t.Helper()
	gs.Rng = rand.New(rand.NewSource(2))
}

func TestKrark_R54_LoseFlipNoOpWhenSpellLeftStack(t *testing.T) {
	gs := newGame(t, 2)
	krark := stampCreaturePT(addPerm(gs, 0, "Krark, the Thumbless", "creature"), 2, 2)
	krark.Card.Owner = 0

	// Seed seat 0's graveyard with the would-be-bounced spell — simulating
	// the deferred-trigger scenario where the spell already resolved into
	// graveyard before Krark's trigger drains from pendingTriggers.
	spellCard := &gameengine.Card{
		Name:  "Glyph of Destruction",
		Owner: 0,
		Types: []string{"instant"},
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, spellCard)
	preGY := len(gs.Seats[0].Graveyard)
	preHand := len(gs.Seats[0].Hand)
	preStack := len(gs.Stack)

	forceKrarkLoseFlip(t)
	forceKrarkLoseFlipOn(t, gs)
	krarkTrigger(gs, krark, map[string]interface{}{
		"caster_seat": 0,
		"card":        spellCard,
		"spell_name":  "Glyph of Destruction",
	})

	// Card must remain in graveyard, NOT appear in hand. Stack untouched.
	if got := len(gs.Seats[0].Graveyard); got != preGY {
		t.Errorf("graveyard size changed: before=%d after=%d (card should stay in graveyard)", preGY, got)
	}
	if got := len(gs.Seats[0].Hand); got != preHand {
		t.Errorf("hand size changed: before=%d after=%d (no-op expected)", preHand, got)
	}
	if got := len(gs.Stack); got != preStack {
		t.Errorf("stack size changed: before=%d after=%d", preStack, got)
	}
	// The card must not be duplicated across zones.
	inGY := 0
	for _, c := range gs.Seats[0].Graveyard {
		if c == spellCard {
			inGY++
		}
	}
	inHand := 0
	for _, c := range gs.Seats[0].Hand {
		if c == spellCard {
			inHand++
		}
	}
	if inGY+inHand != 1 {
		t.Errorf("CardIdentity-equivalent leak: card appears %d times in graveyard + %d times in hand (want exactly 1 total)", inGY, inHand)
	}
}

func TestKrark_R54_LoseFlipBouncesWhenSpellOnStack(t *testing.T) {
	gs := newGame(t, 2)
	krark := stampCreaturePT(addPerm(gs, 0, "Krark, the Thumbless", "creature"), 2, 2)
	krark.Card.Owner = 0

	// Spell IS on the stack — normal cast-time flow.
	spellCard := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
	}
	stackItem := &gameengine.StackItem{
		Controller: 0,
		Card:       spellCard,
		CostMeta:   map[string]interface{}{},
	}
	gs.Stack = append(gs.Stack, stackItem)
	preStack := len(gs.Stack)
	preHand := len(gs.Seats[0].Hand)

	forceKrarkLoseFlip(t)
	forceKrarkLoseFlipOn(t, gs)
	krarkTrigger(gs, krark, map[string]interface{}{
		"caster_seat": 0,
		"card":        spellCard,
		"spell_name":  "Lightning Bolt",
	})

	// Spell removed from stack, appended to hand exactly once.
	if got := len(gs.Stack); got != preStack-1 {
		t.Errorf("expected stack to shrink by 1; before=%d after=%d", preStack, got)
	}
	if got := len(gs.Seats[0].Hand); got != preHand+1 {
		t.Errorf("expected hand to grow by 1; before=%d after=%d", preHand, got)
	}
	// Pointer identity: the same *Card now lives in hand.
	last := gs.Seats[0].Hand[len(gs.Seats[0].Hand)-1]
	if last != spellCard {
		t.Errorf("expected bounced *Card pointer in hand; got %p want %p", last, spellCard)
	}
	// And it's gone from the stack.
	for _, si := range gs.Stack {
		if si != nil && si.Card == spellCard {
			t.Errorf("bounced card should no longer have a StackItem; still found one")
		}
	}
}

func TestKrark_R54_LoseFlipNoOpWhenSpellInHandAlready(t *testing.T) {
	// Defense-in-depth: even if the spell has been routed to hand somehow
	// (e.g., a cascade of triggers shuffled it back), the bounce must not
	// double-append.
	gs := newGame(t, 2)
	krark := stampCreaturePT(addPerm(gs, 0, "Krark, the Thumbless", "creature"), 2, 2)
	krark.Card.Owner = 0

	spellCard := &gameengine.Card{
		Name:  "Brainstorm",
		Owner: 0,
		Types: []string{"instant"},
	}
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, spellCard)
	preHand := len(gs.Seats[0].Hand)

	forceKrarkLoseFlip(t)
	forceKrarkLoseFlipOn(t, gs)
	krarkTrigger(gs, krark, map[string]interface{}{
		"caster_seat": 0,
		"card":        spellCard,
		"spell_name":  "Brainstorm",
	})

	// Hand must NOT have duplicated the entry.
	if got := len(gs.Seats[0].Hand); got != preHand {
		t.Errorf("hand size changed: before=%d after=%d (no-op expected, no duplication)", preHand, got)
	}
	count := 0
	for _, c := range gs.Seats[0].Hand {
		if c == spellCard {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected the *Card pointer in hand exactly once; got %d", count)
	}
}
