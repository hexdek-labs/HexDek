package gameengine

// r63 — 614.1a pressure-test companions. The existing phase-3 test
// (legality_phase3_r62_test.go) proves the check fires past a SYNTHETIC
// always-applies replacement. These pins close the gap to the fuzz
// claim: they run the REAL RegisterRestInPeace / Leyline / Anafenza
// registrations (via RegisterReplacementsForPermanent, the same ETB
// dispatch chaos games use) through both arms:
//
//   - canonical death path (DestroyPermanent → FireDieEvent → §614
//     chain) → card lands in EXILE, zero violations;
//   - chain-bypassing graveyard arrival → 614.1a flags through the real
//     Applies predicate, and the predicate's own exemptions (Leyline
//     spares its controller's cards, Anafenza spares her controller's
//     creatures) stay violation-free.
//
// Together with the seeded loki sweeps (see
// /tmp/fable-review/legality-614-pressure.md) this is what
// "validated-under-pressure" means: the check has teeth on the exact
// boards the fuzz assembled, and 700+ seeded games produced zero hits.

import (
	"testing"
)

func count614(v *LegalityValidator) int {
	n := 0
	for _, viol := range v.Violations {
		if viol.Rule == "614.1a" {
			n++
		}
	}
	return n
}

// Real Rest in Peace: a canonical destroy redirects the card to exile
// with no violations; a chain-bypassing graveyard arrival flags.
func TestReal614_RestInPeace_CanonicalExileCleanBypassFlagged(t *testing.T) {
	gs, v := phase3Game(t)
	rip := addBattlefield(gs, 1, "Rest in Peace", 0, 0, "enchantment")
	RegisterReplacementsForPermanent(gs, rip)

	victim := addBattlefield(gs, 0, "Doomed Bear", 2, 2, "creature")
	if !DestroyPermanent(gs, victim, nil) {
		t.Fatal("DestroyPermanent should succeed")
	}
	inExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == victim.Card {
			inExile = true
		}
	}
	if !inExile {
		t.Fatalf("Rest in Peace should redirect the dying creature to exile; exile=%d graveyard=%d",
			len(gs.Seats[0].Exile), len(gs.Seats[0].Graveyard))
	}
	for _, c := range gs.Seats[0].Graveyard {
		if c == victim.Card {
			t.Fatal("card reached the graveyard despite Rest in Peace")
		}
	}
	if len(v.Violations) != 0 {
		t.Fatalf("canonical redirected death should be violation-free: %v", v.Violations)
	}

	// Bypass arm: a graveyard arrival that skipped the §614 chain.
	ghost := addBattlefield(gs, 0, "Ghost Bear", 2, 2, "creature")
	FireZoneChangeTriggers(gs, ghost, ghost.Card, "battlefield", "graveyard")
	if count614(v) == 0 {
		t.Fatalf("bypassing arrival past real Rest in Peace predicate not flagged: %v", v.Violations)
	}
}

// Real Leyline of the Void: spares its controller's own cards (no
// false positive), flags an opponent's bypassing arrival.
func TestReal614_Leyline_OwnCardExemptOpponentFlagged(t *testing.T) {
	gs, v := phase3Game(t)
	ley := addBattlefield(gs, 1, "Leyline of the Void", 0, 0, "enchantment")
	RegisterReplacementsForPermanent(gs, ley)

	// Controller's own creature dying is NOT covered by Leyline — a
	// graveyard arrival is legitimate and must not flag.
	own := addBattlefield(gs, 1, "Loyal Bear", 2, 2, "creature")
	FireZoneChangeTriggers(gs, own, own.Card, "battlefield", "graveyard")
	if got := count614(v); got != 0 {
		t.Fatalf("Leyline must not flag its controller's own graveyard arrival: %v", v.Violations)
	}

	// An opponent's card bypassing the chain must flag.
	opp := addBattlefield(gs, 0, "Enemy Bear", 2, 2, "creature")
	FireZoneChangeTriggers(gs, opp, opp.Card, "battlefield", "graveyard")
	if count614(v) == 0 {
		t.Fatalf("bypassing arrival past real Leyline predicate not flagged: %v", v.Violations)
	}
}

// Real Anafenza, the Foremost: canonical opponent-creature death
// exiles cleanly; her controller's own creature graveyard arrival is
// exempt; an opponent's bypassing arrival flags via the would_die probe.
func TestReal614_Anafenza_CanonicalExileOwnExemptBypassFlagged(t *testing.T) {
	gs, v := phase3Game(t)
	ana := addBattlefield(gs, 1, "Anafenza, the Foremost", 4, 4, "legendary", "creature")
	RegisterReplacementsForPermanent(gs, ana)

	// Canonical arm: opponent creature destroyed → exiled, clean.
	victim := addBattlefield(gs, 0, "Doomed Bear", 2, 2, "creature")
	if !DestroyPermanent(gs, victim, nil) {
		t.Fatal("DestroyPermanent should succeed")
	}
	inExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == victim.Card {
			inExile = true
		}
	}
	if !inExile {
		t.Fatal("Anafenza should exile the opponent's dying creature")
	}
	if len(v.Violations) != 0 {
		t.Fatalf("canonical redirected death should be violation-free: %v", v.Violations)
	}

	// Her controller's own creature is outside the predicate — clean.
	own := addBattlefield(gs, 1, "Kin Bear", 2, 2, "creature")
	FireZoneChangeTriggers(gs, own, own.Card, "battlefield", "graveyard")
	if got := count614(v); got != 0 {
		t.Fatalf("Anafenza must not flag her controller's own creature: %v", v.Violations)
	}

	// Opponent creature bypassing the chain → flagged.
	ghost := addBattlefield(gs, 0, "Ghost Bear", 2, 2, "creature")
	FireZoneChangeTriggers(gs, ghost, ghost.Card, "battlefield", "graveyard")
	if count614(v) == 0 {
		t.Fatalf("bypassing arrival past real Anafenza predicate not flagged: %v", v.Violations)
	}
}
