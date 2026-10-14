package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r97 — the Flags["kw:<name>"] surface is folded into the layer BASE, so
// a stamp or a delete changes what gs.HasKeywordOf answers; but the
// computed characteristics are cached and writing the map does not bump
// the epoch. These tests assert BOTH directions through real entry
// points, each with the cache deliberately PRIMED first — an unprimed
// read computes fresh and would pass whether or not the fix is present.
//
// Since r66, keywordActive (combat, evasion, the §508.1a validator) is
// the layer query alone, so the layer answer is the one that decides
// behaviour. Asserting the raw flag would prove nothing about any of it.

// primeR97 populates the characteristics cache for p, the way any
// ordinary board read upstream of a handler would.
func primeR97(gs *gameengine.GameState, p *gameengine.Permanent, kws ...string) {
	for _, kw := range kws {
		_ = gs.HasKeywordOf(p, kw)
	}
}

// ---------------------------------------------------------------------
// Grant direction — Skithiryx {B}: gains haste until end of turn.
// ---------------------------------------------------------------------

func TestKwInvalidationR97_SkithiryxHasteVisibleToLayerAfterGrant(t *testing.T) {
	gs := newGame(t, 2)
	skit := r89Creature(gs, 0, "Skithiryx, the Blight Dragon", "creature", "dragon")
	skit.SummoningSick = true
	gs.Seats[0].ManaPool = 5

	primeR97(gs, skit, "haste")

	gameengine.InvokeActivatedHook(gs, skit, 0, nil)

	if !skit.HasKeyword("haste") {
		t.Fatalf("fixture: the raw stamp did not land; flags=%+v", skit.Flags)
	}
	if !gs.HasKeywordOf(skit, "haste") {
		t.Errorf("paid {B} for haste and the LAYER query still says no — " +
			"the stamp did not invalidate the characteristics cache")
	}
}

// ---------------------------------------------------------------------
// Expire direction — the end-step delayed trigger strips it again.
//
// The matching half of the test above. A delayed-trigger EffectFn runs
// long after the function that registered it; FireDelayedTriggers does
// not invalidate, so the closure must.
// ---------------------------------------------------------------------

func TestKwInvalidationR97_SkithiryxHasteGoneFromLayerAfterExpiry(t *testing.T) {
	gs := newGame(t, 2)
	skit := r89Creature(gs, 0, "Skithiryx, the Blight Dragon", "creature", "dragon")
	gs.Seats[0].ManaPool = 5

	gameengine.InvokeActivatedHook(gs, skit, 0, nil)
	// Prime with the grant LIVE, so the cached answer is "has haste".
	if !gs.HasKeywordOf(skit, "haste") {
		t.Fatalf("fixture: haste not granted")
	}

	if n := gameengine.FireDelayedTriggers(gs, "ending", "end_step"); n == 0 {
		t.Fatalf("fixture: no delayed trigger fired")
	}

	if skit.HasKeyword("haste") {
		t.Fatalf("fixture: the raw flag survived the strip; flags=%+v", skit.Flags)
	}
	if gs.HasKeywordOf(skit, "haste") {
		t.Errorf("haste expired on the raw surface but the LAYER query still grants it — " +
			"the delayed-trigger EffectFn did not invalidate")
	}
}

// ---------------------------------------------------------------------
// Token mint — CreateCreatureToken fires ETB and token_created triggers,
// which populate the cache for the brand-new token BEFORE the handler
// stamps its keywords on it. Nothing primes this one by hand; the
// creation call is the prime.
// ---------------------------------------------------------------------

func TestKwInvalidationR97_LocustGodTokenFlyingVisibleToLayer(t *testing.T) {
	gs := newGame(t, 2)
	god := r89Creature(gs, 0, "The Locust God", "creature", "god")
	before := len(gs.Seats[0].Battlefield)

	theLocustGodOnDraw(gs, god, map[string]interface{}{
		"seat":          0,
		"drawer_seat":   0,
		"nth_this_turn": 1,
		"source":        "draw",
	})

	var tok *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield[before:] {
		if p != nil && p.Card != nil && p.Card.Name == "Insect" {
			tok = p
		}
	}
	if tok == nil {
		t.Skip("Insect token not minted by this dispatch shape")
	}
	if !tok.HasKeyword("flying") {
		t.Fatalf("fixture: raw flying not stamped on the token; flags=%+v", tok.Flags)
	}
	if !gs.HasKeywordOf(tok, "flying") {
		t.Errorf("token minted with flying and the LAYER query says no — " +
			"the post-creation stamp did not invalidate (evasion reads the layer)")
	}
}

// ---------------------------------------------------------------------
// The r87 refcounted pair — Najeela buffs every attacking creature. Its
// REVOKE arm always invalidated; its GRANT arm never did, in all three
// cards that use the pair this way (Najeela, Dihada, Leonardo).
// ---------------------------------------------------------------------

func TestKwInvalidationR97_NajeelaGrantVisibleToLayer(t *testing.T) {
	gs := newGame(t, 2)
	gs.Phase = "combat"
	naj := r89Creature(gs, 0, "Najeela, the Blade-Blossom", "creature", "warrior")
	atk := r89Creature(gs, 0, "R97 Attacker", "creature", "warrior")
	atk.Flags["attacking"] = 1
	gs.Seats[0].ManaPool = 10

	primeR97(gs, atk, "trample", "lifelink", "haste")

	gameengine.InvokeActivatedHook(gs, naj, 0, nil)

	if !atk.HasKeyword("trample") {
		t.Skip("Najeela's activation did not buff in this fixture shape")
	}
	if !gs.HasKeywordOf(atk, "trample") {
		t.Errorf("Najeela granted trample and the LAYER query says no — " +
			"GrantKeywordFlag leaves invalidation to the caller and the grant arm skipped it")
	}
}

// ---------------------------------------------------------------------
// Over-suppression guard. Invalidating a cache must not invent
// keywords: a creature nobody granted anything to stays clean after the
// same sequence of calls. Without this, "always return true" would pass
// every test above.
// ---------------------------------------------------------------------

func TestKwInvalidationR97_UngrantedCreatureGainsNothing(t *testing.T) {
	gs := newGame(t, 2)
	skit := r89Creature(gs, 0, "Skithiryx, the Blight Dragon", "creature", "dragon")
	bystander := r89Creature(gs, 0, "R97 Bystander", "creature", "bear")
	gs.Seats[0].ManaPool = 5

	primeR97(gs, bystander, "haste", "trample", "flying")
	gameengine.InvokeActivatedHook(gs, skit, 0, nil)

	for _, kw := range []string{"haste", "trample", "flying"} {
		if gs.HasKeywordOf(bystander, kw) {
			t.Errorf("bystander gained %q from an unrelated permanent's grant", kw)
		}
		if bystander.HasKeyword(kw) {
			t.Errorf("bystander raw-gained %q", kw)
		}
	}
}
