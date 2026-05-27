package hat

import (
	"math"
	"testing"
)

// r60 follow-up: drawEngineRate now recognizes two virtual-CA classes
// that pure "you draw a card" detection missed:
//
//	0.8 — impulse-draw / exile-cast engines (Outpost Siege,
//	      Wild-Magic Sorcerer, Etali Primal Storm, Prosper Tome-Bound,
//	      Faldorn Dread-Wolf Herald). Recurring "exile the top card of
//	      your library; you may play it" produces ~1 castable card per
//	      cycle. Pre-fix these scored 0 — castableExileCount picks up
//	      the resulting exile cards but only AFTER the engine fires,
//	      so the engine itself got no credit on the turn it landed.
//	0.3 — persistent scry / surveil engines (Path of Ancestry, Ledger
//	      Shredder). Filters draw quality rather than adding a card,
//	      so weighted well below a true engine but >0.

// TestDrawEngineRate_OutpostSiegeImpulse pins the upkeep-cadenced
// impulse engine at 0.8.
func TestDrawEngineRate_OutpostSiegeImpulse(t *testing.T) {
	siege := cardWithStaticText("Outpost Siege", []string{"enchantment"}, 4,
		"at the beginning of your upkeep, choose khans or dragons. khans — exile the top card of your library. until end of turn, you may play that card. dragons — outpost siege deals 1 damage to any target.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], siege, 0, 0)
	got := drawEngineRate(p)
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("Outpost Siege rate = %f, want 0.8 (impulse upkeep)", got)
	}
}

// TestDrawEngineRate_WildMagicSorcererImpulse pins the attack-trigger
// impulse engine at 0.8.
func TestDrawEngineRate_WildMagicSorcererImpulse(t *testing.T) {
	wms := cardWithStaticText("Wild-Magic Sorcerer", []string{"creature"}, 4,
		"whenever wild-magic sorcerer attacks, exile the top card of your library. you may play that card this turn.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], wms, 3, 4)
	got := drawEngineRate(p)
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("Wild-Magic Sorcerer rate = %f, want 0.8 (impulse attack-trigger)", got)
	}
}

// TestDrawEngineRate_EtaliImpulseEachLibrary confirms the "each
// player's library" variant (Etali) still triggers the impulse gate.
func TestDrawEngineRate_EtaliImpulseEachLibrary(t *testing.T) {
	etali := cardWithStaticText("Etali, Primal Storm", []string{"creature"}, 6,
		"whenever etali, primal storm attacks, exile the top card of each player's library, then you may cast any number of spells from among those cards without paying their mana costs.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], etali, 6, 6)
	got := drawEngineRate(p)
	if math.Abs(got-0.8) > 1e-9 {
		t.Errorf("Etali rate = %f, want 0.8 (impulse each-player's-library)", got)
	}
}

// TestDrawEngineRate_PathOfAncestryScry pins a recurring-scry engine
// at the 0.3 pseudo-CA rate.
func TestDrawEngineRate_PathOfAncestryScry(t *testing.T) {
	path := cardWithStaticText("Path of Ancestry", []string{"land"}, 0,
		"whenever you cast a creature spell that shares a creature type with your commander, scry 1. {t}: add one mana of any color in your commander's color identity.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], path, 0, 0)
	got := drawEngineRate(p)
	if math.Abs(got-0.3) > 1e-9 {
		t.Errorf("Path of Ancestry rate = %f, want 0.3 (pseudo-CA scry)", got)
	}
}

// TestDrawEngineRate_LedgerShredderSurveil pins a recurring-surveil
// engine at 0.3.
func TestDrawEngineRate_LedgerShredderSurveil(t *testing.T) {
	shredder := cardWithStaticText("Ledger Shredder", []string{"creature"}, 2,
		"flying. whenever you cast the second spell each turn, surveil 2.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], shredder, 1, 3)
	got := drawEngineRate(p)
	if math.Abs(got-0.3) > 1e-9 {
		t.Errorf("Ledger Shredder rate = %f, want 0.3 (pseudo-CA surveil)", got)
	}
}

// TestDrawEngineRate_ImpulseUnderscoresPhyrexianArena confirms the
// rate ordering: impulse (0.8) is below the upkeep-cadenced "you draw
// a card" baseline (Phyrexian Arena, 1.0). Impulse output is gated on
// mana availability + spell relevance, so a strict-draw engine
// out-scores it.
func TestDrawEngineRate_ImpulseUnderscoresPhyrexianArena(t *testing.T) {
	siege := cardWithStaticText("Outpost Siege", []string{"enchantment"}, 4,
		"at the beginning of your upkeep, choose khans or dragons. khans — exile the top card of your library. until end of turn, you may play that card.")
	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	gs := newTestGame(t, 2)
	pSiege := newTestPermanent(gs.Seats[0], siege, 0, 0)
	pArena := newTestPermanent(gs.Seats[0], arena, 0, 0)
	rSiege := drawEngineRate(pSiege)
	rArena := drawEngineRate(pArena)
	if !(rSiege < rArena) {
		t.Errorf("impulse engine should rate below upkeep-draw; got siege=%f arena=%f", rSiege, rArena)
	}
}

// TestDrawEngineRate_ScryUnderscoresImpulse confirms the rate
// ordering: scry/surveil (0.3) is below impulse (0.8). Filtering one
// card per cycle is real but less valuable than producing a castable
// card per cycle.
func TestDrawEngineRate_ScryUnderscoresImpulse(t *testing.T) {
	shredder := cardWithStaticText("Ledger Shredder", []string{"creature"}, 2,
		"flying. whenever you cast the second spell each turn, surveil 2.")
	siege := cardWithStaticText("Outpost Siege", []string{"enchantment"}, 4,
		"at the beginning of your upkeep, choose khans or dragons. khans — exile the top card of your library. until end of turn, you may play that card.")
	gs := newTestGame(t, 2)
	pShredder := newTestPermanent(gs.Seats[0], shredder, 1, 3)
	pSiege := newTestPermanent(gs.Seats[0], siege, 0, 0)
	rShredder := drawEngineRate(pShredder)
	rSiege := drawEngineRate(pSiege)
	if !(rShredder < rSiege) {
		t.Errorf("scry/surveil should rate below impulse; got shredder=%f siege=%f", rShredder, rSiege)
	}
}

// TestDrawEngineRate_OneShotImpulseDoesNotRegister pins the
// recurring-cue requirement: a one-shot impulse spell (Light Up the
// Stage style) does NOT register as a recurring engine, because the
// "whenever / at the beginning" cue is absent.
func TestDrawEngineRate_OneShotImpulseDoesNotRegister(t *testing.T) {
	lits := cardWithStaticText("Light Up the Stage", []string{"sorcery"}, 1,
		"spectacle {r}. exile the top two cards of your library. until the end of your next turn, you may play those cards.")
	// Sorceries aren't permanents, so this only checks the helper
	// directly via a permanent wrapper to confirm the gate.
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], lits, 0, 0)
	if got := drawEngineRate(p); got != 0 {
		t.Errorf("one-shot Light Up the Stage rate = %f, want 0 (no recurring cue)", got)
	}
}

// TestDrawEngineRate_ActivatedScryDoesNotRegister pins the
// recurring-cue requirement for scry: Crystal Ball (activated cost)
// is NOT a persistent-scry engine. Its throughput depends on mana
// spend and is captured implicitly via ManaAdvantage opportunity
// cost, not double-counted as virtual CA.
func TestDrawEngineRate_ActivatedScryDoesNotRegister(t *testing.T) {
	ball := cardWithStaticText("Crystal Ball", []string{"artifact"}, 2,
		"{1}, {t}: scry 2.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], ball, 0, 0)
	if got := drawEngineRate(p); got != 0 {
		t.Errorf("Crystal Ball rate = %f, want 0 (activated, not recurring trigger)", got)
	}
}

// TestDrawEngineRate_StaticTopOfLibraryDoesNotRegister pins the
// recurring-cue gate against static "you may play the top card of
// your library" effects (Future Sight, Vizier of the Menagerie,
// Augur of Autumn). These are top-of-library-play engines — a
// different family from impulse exile-cast — and are not captured by
// this rate-class table.
func TestDrawEngineRate_StaticTopOfLibraryDoesNotRegister(t *testing.T) {
	future := cardWithStaticText("Future Sight", []string{"enchantment"}, 5,
		"play with the top card of your library revealed. you may play lands and cast spells from the top of your library.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], future, 0, 0)
	if got := drawEngineRate(p); got != 0 {
		t.Errorf("Future Sight rate = %f, want 0 (static top-of-library, no exile cue)", got)
	}
}

// TestDrawEngineCredit_ImpulseSeatBetweenArenaAndHowlingMine confirms
// the rate ordering integrates end-to-end through drawEngineCredit:
// impulse (0.8) sits below Arena (1.0) and above Howling Mine (0.6).
func TestDrawEngineCredit_ImpulseSeatBetweenArenaAndHowlingMine(t *testing.T) {
	gs := newTestGame(t, 3)
	siege := cardWithStaticText("Outpost Siege", []string{"enchantment"}, 4,
		"at the beginning of your upkeep, choose khans or dragons. khans — exile the top card of your library. until end of turn, you may play that card.")
	newTestPermanent(gs.Seats[0], siege, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	mine := cardWithStaticText("Howling Mine", []string{"artifact"}, 2,
		"at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card")
	newTestPermanent(gs.Seats[2], mine, 0, 0)

	cSiege := drawEngineCredit(gs.Seats[0])
	cArena := drawEngineCredit(gs.Seats[1])
	cMine := drawEngineCredit(gs.Seats[2])

	if !(cMine < cSiege && cSiege < cArena) {
		t.Errorf("ordering should be mine < siege < arena; got mine=%f siege=%f arena=%f", cMine, cSiege, cArena)
	}
	if math.Abs(cSiege-0.8) > 1e-9 {
		t.Errorf("seat with single Outpost Siege credit = %f, want 0.8", cSiege)
	}
}

// TestScoreCards_ImpulseSeatOutscoresEmptySeat pins the end-to-end
// integration: a seat with Outpost Siege scores higher CardAdvantage
// than a seat with no engines, holding everything else equal. This is
// the regression the addition fixes — pre-fix the impulse engine
// scored zero CardAdvantage credit on the turn it resolved.
func TestScoreCards_ImpulseSeatOutscoresEmptySeat(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	siege := cardWithStaticText("Outpost Siege", []string{"enchantment"}, 4,
		"at the beginning of your upkeep, choose khans or dragons. khans — exile the top card of your library. until end of turn, you may play that card.")
	newTestPermanent(gs.Seats[0], siege, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage > r1.CardAdvantage) {
		t.Errorf("Outpost Siege seat should out-score empty seat on CardAdvantage; got siege=%.3f empty=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

// TestScoreCards_ScrySeatOutscoresEmptySeat pins the end-to-end
// integration for the scry/surveil class: a Ledger Shredder seat
// scores higher CardAdvantage than a seat with no engines. Pre-fix
// the pseudo-CA was uncredited.
func TestScoreCards_ScrySeatOutscoresEmptySeat(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	shredder := cardWithStaticText("Ledger Shredder", []string{"creature"}, 2,
		"flying. whenever you cast the second spell each turn, surveil 2.")
	newTestPermanent(gs.Seats[0], shredder, 1, 3)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage > r1.CardAdvantage) {
		t.Errorf("Ledger Shredder seat should out-score empty seat on CardAdvantage; got shredder=%.3f empty=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

// TestIsImpulseDrawEngine_NeedsBothExileAndCast pins the conjunction:
// "exile the top card of your library" WITHOUT a play/cast clue does
// NOT register (e.g. mill-self-then-graveyard-payoff effects), and
// "you may play that card" WITHOUT an exile-top-of-library cue does
// NOT register either (e.g. cascade-like effects on resolution).
func TestIsImpulseDrawEngine_NeedsBothExileAndCast(t *testing.T) {
	// Exile without cast: pure self-mill.
	if isImpulseDrawEngine("at the beginning of your upkeep, exile the top card of your library.") {
		t.Errorf("exile-only (no cast) should not register as impulse engine")
	}
	// Cast without exile-top-of-library cue.
	if isImpulseDrawEngine("whenever you cast a spell, you may play an additional land this turn.") {
		t.Errorf("cast-only (no exile-top-of-library) should not register as impulse engine")
	}
	// Full pair must register.
	if !isImpulseDrawEngine("at the beginning of your upkeep, exile the top card of your library. you may play that card this turn.") {
		t.Errorf("full exile + cast + recurring cue should register as impulse engine")
	}
}
