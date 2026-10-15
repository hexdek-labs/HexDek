package gameengine

import (
	"strings"
	"testing"
)

// r83 — batch 7 of the keyword call-site migration: the UNION sites.
//
// Three call sites still read `p.HasKeyword(kw) || gs.HasKeywordOf(p, kw)`,
// the exact shape r66 deleted from combat. Both were written when neither
// query was a superset of the other, with comments saying so: the union
// existed to stop an anthem-granted keyword from false-positiving a
// detector. r65/r66 folded every positive raw source into the layer base
// and made that intent satisfiable by the layer query alone.
//
// Re-probed before touching it, across all four raw surfaces (AST,
// GrantedAbilities, Flags["kw:"], keyword counters) for haste, menace and
// defender: twelve cells, no raw-true / layer-false anywhere. With the
// superset holding, the raw arm can only contribute where a layer-6 strip
// said NO — so in a DETECTOR it does not merely duplicate the layer query,
// it blinds the detector to the real rules break it was built to catch.
//
// These tests assert the strip direction fires and the grant direction
// still does not, because a migration that traded a false negative for a
// false positive would be no better than the union.

func r83SickAttacker(gs *GameState, name string) *Permanent {
	p := upCreature(gs, 0, name, 2)
	p.SummoningSick = true
	p.Flags[flagAttacking] = 1
	return p
}

// §302.6 via the legality detector — a Humbled creature attacking on a
// stamped haste is an illegal attacker and must be reported.
func TestKeywordMigrationR83_LegalityHasteStripIsCaught(t *testing.T) {
	gs := newUPGame(t)
	p := r83SickAttacker(gs, "Humbled Hasty")
	p.Flags["kw:haste"] = 1
	gs.InvalidateCharacteristicsCache()
	if !p.HasKeyword("haste") {
		t.Fatal("precondition: the stamp should be raw-visible")
	}
	r70Strip(gs, p, "r83_legality_haste_strip")

	obs := &LegalityObservation{Kind: "attack", Seat: 0, Perm: p}
	v := checkLegalityAttackDecl(gs, obs)
	if !r83Has(v, "302.6") {
		t.Errorf("a stripped haste did not reach the legality detector — got %v", r83Rules(v))
	}
}

// The grant direction must stay quiet: a layer-granted haste is real haste
// and flagging it would be the false positive the union was protecting.
func TestKeywordMigrationR83_LegalityHasteGrantStaysQuiet(t *testing.T) {
	gs := newUPGame(t)
	p := r83SickAttacker(gs, "Anthem Hasty")
	r70Grant(gs, p, "haste", "r83_legality_haste_grant")
	if p.HasKeyword("haste") {
		t.Fatal("precondition: the raw query must NOT see a layer grant")
	}

	obs := &LegalityObservation{Kind: "attack", Seat: 0, Perm: p}
	v := checkLegalityAttackDecl(gs, obs)
	if r83Has(v, "302.6") {
		t.Errorf("a layer-granted haste was phantom-flagged as summoning-sick — got %v", r83Rules(v))
	}
}

// §702.111b — a menace stripped by Humility must stop demanding two
// blockers, so a single blocker is legal and must not be reported.
func TestKeywordMigrationR83_LegalityMenaceStripIsNotFlagged(t *testing.T) {
	gs := newUPGame(t)
	atk := upCreature(gs, 0, "Humbled Menacer", 3)
	atk.Flags["kw:menace"] = 1
	gs.InvalidateCharacteristicsCache()
	blk := upCreature(gs, 1, "Lone Blocker", 2)

	obs := &LegalityObservation{Kind: "block", Seat: 1, Attacker: atk, Blockers: []*Permanent{blk}}
	if v := checkLegalityBlockDecl(gs, obs); !r83Has(v, "702.111b") {
		t.Fatalf("precondition: the stamped menace should flag a single blocker — got %v", r83Rules(v))
	}

	r70Strip(gs, atk, "r83_legality_menace_strip")
	v := checkLegalityBlockDecl(gs, obs)
	if r83Has(v, "702.111b") {
		t.Errorf("menace survived a layer-6 ability strip — a single blocker was still reported illegal: %v", r83Rules(v))
	}
}

// invariants.go checkCombatLegality — the defender arm. Its comment said
// the raw read had to stay because the layer query would MISS stamps and
// counters. That has been false since r65/r66.
func TestKeywordMigrationR83_InvariantDefenderStripIsNotFlagged(t *testing.T) {
	gs := newUPGame(t)
	gs.Phase = "combat" // checkCombatLegality only runs during combat
	p := upCreature(gs, 0, "Humbled Wall", 2)
	p.Flags["kw:defender"] = 1
	p.Flags[flagAttacking] = 1
	gs.InvalidateCharacteristicsCache()
	if err := checkCombatLegality(gs); err == nil || !strings.Contains(err.Error(), "defender") {
		t.Fatalf("precondition: a stamped defender attacking should be flagged; got %v", err)
	}

	r70Strip(gs, p, "r83_inv_defender_strip")
	if err := checkCombatLegality(gs); err != nil && strings.Contains(err.Error(), "defender") {
		t.Errorf("defender survived a layer-6 ability strip — still reported as an illegal attacker: %v", err)
	}
}

// invariants.go checkCombatLegality — the haste arm, strip direction.
func TestKeywordMigrationR83_InvariantHasteStripIsCaught(t *testing.T) {
	gs := newUPGame(t)
	gs.Phase = "combat" // checkCombatLegality only runs during combat
	p := r83SickAttacker(gs, "Humbled Sprinter")
	p.Flags["kw:haste"] = 1
	gs.InvalidateCharacteristicsCache()
	if err := checkCombatLegality(gs); err != nil {
		t.Fatalf("precondition: a stamped haste should NOT be flagged; got %v", err)
	}

	r70Strip(gs, p, "r83_inv_haste_strip")
	err := checkCombatLegality(gs)
	if err == nil || !strings.Contains(err.Error(), "summoning sickness") {
		t.Errorf("a stripped haste did not reach the combat-legality invariant; got %v", err)
	}
}

func r83Has(vs []LegalityViolation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func r83Rules(vs []LegalityViolation) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.Rule)
	}
	return out
}
