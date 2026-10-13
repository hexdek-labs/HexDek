package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r87 — the standing regression for the keyword-flag collision filed as
// 6f332cbc.
//
// Flags["kw:<x>"] is one shared boolean. Two sources granting the same
// keyword to the same permanent used to collide: the second source's
// "don't overwrite a native keyword" guard saw the first source's stamp
// through HasKeyword, skipped recording its own grant, and the first
// source's departure then took the keyword away while the second was
// still on the battlefield.
//
// The fixture is the probed one and uses two REAL handlers rather than a
// synthetic pair: Brudiclad ("creature tokens you control have haste")
// and Vihaan ("other outlaws you control have vigilance and haste"), on
// a Pirate creature token, which qualifies for both. Everything is
// driven through the production entry points — InvokeETBHook and a
// permanent_ltb FireCardTrigger — not by calling the sweeps directly.

// r87Body gives a fixture permanent a real body. addPerm leaves
// BasePower/BaseToughness at zero, and a 0/0 creature is destroyed by
// the §704.5f state-based action the moment anything opens a priority
// round — which PushPerCardTrigger does on every dispatch. Without
// this the whole battlefield is gone before the second LTB handler
// resolves, and the test passes for the wrong reason: the keyword is
// "absent" because the permanent is.
func r87Body(p *gameengine.Permanent, pow, tou int) *gameengine.Permanent {
	p.Card.BasePower = pow
	p.Card.BaseToughness = tou
	return p
}

func r87PirateToken(gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	// Token-ness and subtype both live in Card.Types; there is no
	// Permanent.Token field and no Card.Subtypes field.
	return r87Body(addPerm(gs, seat, name, "creature", "pirate", "token"), 2, 2)
}

func r87Commander(gs *gameengine.GameState, seat int, name string, types ...string) *gameengine.Permanent {
	return r87Body(addPerm(gs, seat, name, types...), 3, 3)
}

func r87LeaveBattlefield(t *testing.T, gs *gameengine.GameState, p *gameengine.Permanent) {
	t.Helper()
	gameengine.FireCardTrigger(gs, "permanent_ltb", map[string]interface{}{
		"perm":            p,
		"card":            p.Card,
		"to_zone":         "graveyard",
		"controller_seat": p.Controller,
	})
}

// TestKeywordFlagRefcountR87_TwoSourcesOneKeyword is the probe itself.
// Pre-r87 it fails at the second check: Vihaan never recorded a haste
// grant, so Brudiclad's departure removed the shared flag.
func TestKeywordFlagRefcountR87_TwoSourcesOneKeyword(t *testing.T) {
	gs := newGame(t, 2)
	tok := r87PirateToken(gs, 0, "Pirate Token")
	bru := r87Commander(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")
	vih := r87Commander(gs, 0, "Vihaan, Goldwaker", "legendary", "creature", "human", "warlock")

	gameengine.InvokeETBHook(gs, bru)
	gameengine.InvokeETBHook(gs, vih)

	if !tok.HasKeyword("haste") {
		t.Fatalf("setup: both sources grant haste; flags=%+v", tok.Flags)
	}
	// Errorf, not Fatalf: this IS the defect (pre-r87 Vihaan's guard saw
	// Brudiclad's shared flag through HasKeyword and recorded nothing),
	// and aborting here would skip the assertion that actually matters.
	if n := gameengine.KeywordFlagGrantCount(tok, "haste"); n != 2 {
		t.Errorf("expected 2 haste grants (Brudiclad + Vihaan), got %d; flags=%+v", n, tok.Flags)
	}

	// THE BUG. Brudiclad leaves; Vihaan is still on the battlefield and
	// still grants haste to this outlaw.
	r87LeaveBattlefield(t, gs, bru)
	if !tok.HasKeyword("haste") {
		t.Errorf("Brudiclad left but Vihaan still grants haste — keyword must survive; flags=%+v", tok.Flags)
	}
	if n := gameengine.KeywordFlagGrantCount(tok, "haste"); n != 1 {
		t.Errorf("expected Vihaan's grant to remain, got %d grants; flags=%+v", n, tok.Flags)
	}
	// Vihaan's other keyword is untouched by Brudiclad leaving.
	if !tok.HasKeyword("vigilance") {
		t.Errorf("vigilance is Vihaan's alone and must be unaffected; flags=%+v", tok.Flags)
	}

	// THE OTHER DIRECTION, which is the half a one-sided test would
	// miss: once the last holder leaves the keyword really does go.
	r87LeaveBattlefield(t, gs, vih)
	if tok.HasKeyword("haste") {
		t.Errorf("both sources gone — haste must be removed; flags=%+v", tok.Flags)
	}
	if tok.HasKeyword("vigilance") {
		t.Errorf("both sources gone — vigilance must be removed; flags=%+v", tok.Flags)
	}
}

// TestKeywordFlagRefcountR87_ReverseDepartureOrder pins the symmetric
// case. The pre-r87 shape was order-dependent in a way that made a
// single-order test look conclusive when it was not: Vihaan recorded no
// haste grant at all, so Vihaan leaving first did NOT break haste, and
// only the Brudiclad-first order exposed the collision.
func TestKeywordFlagRefcountR87_ReverseDepartureOrder(t *testing.T) {
	gs := newGame(t, 2)
	tok := r87PirateToken(gs, 0, "Pirate Token")
	bru := r87Commander(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")
	vih := r87Commander(gs, 0, "Vihaan, Goldwaker", "legendary", "creature", "human", "warlock")

	gameengine.InvokeETBHook(gs, bru)
	gameengine.InvokeETBHook(gs, vih)

	r87LeaveBattlefield(t, gs, vih)
	if !tok.HasKeyword("haste") {
		t.Errorf("Vihaan left but Brudiclad still grants haste; flags=%+v", tok.Flags)
	}
	if tok.HasKeyword("vigilance") {
		t.Errorf("vigilance had only Vihaan behind it and must go; flags=%+v", tok.Flags)
	}

	r87LeaveBattlefield(t, gs, bru)
	if tok.HasKeyword("haste") {
		t.Errorf("last holder gone — haste must be removed; flags=%+v", tok.Flags)
	}
}

// TestKeywordFlagRefcountR87_LayerQueryAgrees checks the fix at the
// layer-aware query as well as the raw one. runtimeKeywordSources folds
// Flags["kw:<x>"] into the layer BASE, so a shared flag deleted early
// would have disappeared from chars.Keywords too — migrating these call
// sites to keywordActive was never going to fix this, and this pins
// that the refcount fixes BOTH surfaces at once.
func TestKeywordFlagRefcountR87_LayerQueryAgrees(t *testing.T) {
	gs := newGame(t, 2)
	tok := r87PirateToken(gs, 0, "Pirate Token")
	bru := r87Commander(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")
	vih := r87Commander(gs, 0, "Vihaan, Goldwaker", "legendary", "creature", "human", "warlock")

	gameengine.InvokeETBHook(gs, bru)
	gameengine.InvokeETBHook(gs, vih)
	if !gs.HasKeywordOf(tok, "haste") {
		t.Fatalf("setup: layer query should see haste; flags=%+v", tok.Flags)
	}

	r87LeaveBattlefield(t, gs, bru)
	gs.InvalidateCharacteristicsCache()
	if !gs.HasKeywordOf(tok, "haste") {
		t.Errorf("layer query lost haste while Vihaan still grants it; flags=%+v", tok.Flags)
	}

	r87LeaveBattlefield(t, gs, vih)
	gs.InvalidateCharacteristicsCache()
	if gs.HasKeywordOf(tok, "haste") {
		t.Errorf("layer query still reports haste with no source; flags=%+v", tok.Flags)
	}
}

// TestKeywordFlagRefcountR87_UnmigratedRawStampSurvives pins the
// transitional guarantee. ~230 stamp sites are still raw; a migrated
// handler's revoke must not delete a shared flag some other code placed
// without a marker. GrantKeywordFlag adopts such a flag under a
// synthetic marker so it outlives the migrated source.
func TestKeywordFlagRefcountR87_UnmigratedRawStampSurvives(t *testing.T) {
	gs := newGame(t, 2)
	tok := r87PirateToken(gs, 0, "Pirate Token")
	tok.Flags["kw:haste"] = 1 // an unmigrated handler's raw stamp
	bru := r87Commander(gs, 0, "Brudiclad, Telchor Engineer", "legendary", "creature", "human", "artificer")

	gameengine.InvokeETBHook(gs, bru)
	r87LeaveBattlefield(t, gs, bru)

	if !tok.HasKeyword("haste") {
		t.Errorf("a raw stamp Brudiclad did not place must survive its revoke; flags=%+v", tok.Flags)
	}
}
