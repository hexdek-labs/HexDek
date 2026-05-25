package hat

import "testing"

// eval_weights_tokens_r60_test.go — broader tokens / go-wide profile.
// Pre-fix, Freya's "Aggro / Go Wide" archetype string (and any other
// token-flood deck classified as "tokens" / "go-wide") had no entry in
// archetypeWeights, so DefaultWeightsForArchetype silently fell back to
// midrange — giving the swarm BoardPresence=1.0 and zero combat-closing
// signal. The new ArchetypeTokens profile fixes both axes; these tests
// pin the load-bearing properties.

// -----------------------------------------------------------------------------
// BoardPresence is the signature dimension for token decks.
// -----------------------------------------------------------------------------

func TestTokensWeights_BoardPresenceRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeTokens)
	a := w.AsArray()
	idxBoard := 0 // BoardPresence slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxBoard {
		t.Fatalf("Tokens' highest weight should be BoardPresence (idx %d), got idx %d val %.2f",
			idxBoard, bestIdx, best)
	}
}

func TestTokensWeights_BoardPresenceBeatsAggro(t *testing.T) {
	// Tokens go even wider than mono-creature aggro — Adeline prints a
	// fresh body every attack, Rhys doubles existing tokens, Krenko
	// untaps for a haste swarm. BoardPresence should sit above the
	// already-elevated aggro baseline (1.5).
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	agg := DefaultWeightsForArchetype(ArchetypeAggro)
	if tok.BoardPresence <= agg.BoardPresence {
		t.Errorf("Tokens BoardPresence (%.2f) should exceed Aggro (%.2f) — wider swarm",
			tok.BoardPresence, agg.BoardPresence)
	}
	if tok.BoardPresence < 1.6 {
		t.Errorf("Tokens BoardPresence should be ≥ 1.6 (Adeline / Rhys / Krenko swarm); got %.2f",
			tok.BoardPresence)
	}
}

// -----------------------------------------------------------------------------
// Combat-closing axes (ThreatTrajectory + CommanderProgress) lift over
// midrange — the deck wins by leveraging the next attack, not by
// stabilising on a parity board.
// -----------------------------------------------------------------------------

func TestTokensWeights_ThreatTrajectoryBoosted(t *testing.T) {
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if tok.ThreatTrajectory <= mid.ThreatTrajectory {
		t.Errorf("Tokens ThreatTrajectory (%.2f) should exceed midrange (%.2f) — combat-closing emphasis",
			tok.ThreatTrajectory, mid.ThreatTrajectory)
	}
	if tok.ThreatTrajectory < 0.6 {
		t.Errorf("Tokens ThreatTrajectory should be ≥ 0.6; got %.2f", tok.ThreatTrajectory)
	}
}

func TestTokensWeights_CommanderProgressBoosted(t *testing.T) {
	// Adeline / Krenko / Rhys / Brimaz are engine commanders that print
	// bodies. CommanderProgress needs to sit above midrange so the hat
	// re-casts after a wipe instead of hoarding mana.
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if tok.CommanderProgress <= mid.CommanderProgress {
		t.Errorf("Tokens CommanderProgress (%.2f) should exceed midrange (%.2f) — Adeline/Krenko engine",
			tok.CommanderProgress, mid.CommanderProgress)
	}
}

func TestTokensWeights_PartnerSynergyAnthemDial(t *testing.T) {
	// Glorious Anthem / Intangible Virtue / Beastmaster Ascension /
	// Coat of Arms / Shared Animosity buff the whole swarm. Same dial
	// Tribal uses for lord effects; should sit well above midrange's 0.5.
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if tok.PartnerSynergy <= mid.PartnerSynergy {
		t.Errorf("Tokens PartnerSynergy (%.2f) should exceed midrange (%.2f) — anthem effects",
			tok.PartnerSynergy, mid.PartnerSynergy)
	}
	if tok.PartnerSynergy < 0.8 {
		t.Errorf("Tokens PartnerSynergy should be ≥ 0.8 (anthem stack); got %.2f", tok.PartnerSynergy)
	}
}

// -----------------------------------------------------------------------------
// Distinct from midrange (was the silent-fallback bug) and from the
// neighbouring profiles the broader bucket explicitly excludes.
// -----------------------------------------------------------------------------

func TestTokensWeights_DistinctFromMidrange(t *testing.T) {
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if tok == mid {
		t.Fatal("ArchetypeTokens weights must not equal midrange — was the silent-fallback bug")
	}
}

func TestTokensWeights_DistinctFromVoltronAndAristocrats(t *testing.T) {
	tok := DefaultWeightsForArchetype(ArchetypeTokens)
	if tok == DefaultWeightsForArchetype(ArchetypeVoltron) {
		t.Error("Tokens weights should not equal Voltron — different gameplan (swarm vs. single creature)")
	}
	if tok == DefaultWeightsForArchetype(ArchetypeAristocrats) {
		t.Error("Tokens weights should not equal Aristocrats — broader bucket explicitly excludes sac/drain decks")
	}
}

// -----------------------------------------------------------------------------
// Freya's "Aggro / Go Wide" archetype string (and the related aliases)
// must route to the Tokens profile, not silently to midrange.
// -----------------------------------------------------------------------------

func TestTokensWeights_FreyaAggroGoWideAlias(t *testing.T) {
	// Freya emits primary archetype "Aggro / Go Wide" → lowercased to
	// "aggro / go wide" in strategy_loader.go. Pre-fix this matched no
	// archetypeWeights key and fell through to midrange.
	got := DefaultWeightsForArchetype("aggro / go wide")
	want := DefaultWeightsForArchetype(ArchetypeTokens)
	if got != want {
		t.Errorf("'aggro / go wide' should normalize to Tokens profile; got BoardPresence=%.2f want %.2f",
			got.BoardPresence, want.BoardPresence)
	}
}

func TestTokensWeights_GoWideAliasVariants(t *testing.T) {
	want := DefaultWeightsForArchetype(ArchetypeTokens)
	for _, alias := range []string{"go wide", "go-wide", "gowide", "tokens"} {
		got := DefaultWeightsForArchetype(alias)
		if got != want {
			t.Errorf("alias %q should normalize to Tokens profile; got BoardPresence=%.2f want %.2f",
				alias, got.BoardPresence, want.BoardPresence)
		}
	}
}

func TestTokensWeights_VoltronAndAristocratsNotPoached(t *testing.T) {
	// Even if a compound archetype mentions tokens-style wording, the
	// presence of "voltron" / "aristocrat" must keep the deck on its
	// dedicated profile — broader Tokens should not steal these.
	volMid := DefaultWeightsForArchetype("voltron")
	if got := DefaultWeightsForArchetype("voltron / go wide"); got != volMid {
		t.Errorf("'voltron / go wide' should stay on Voltron, not poach to Tokens")
	}
	ariMid := DefaultWeightsForArchetype("aristocrats")
	if got := DefaultWeightsForArchetype("aristocrats / tokens"); got != ariMid {
		t.Errorf("'aristocrats / tokens' should stay on Aristocrats, not poach to Tokens")
	}
}

// LegacyMidrangeOnly must still collapse Tokens to midrange — the
// tournament A/B baseline must include the new profile in the override.
func TestTokensWeights_LegacyMidrangeOverride(t *testing.T) {
	saved := LegacyMidrangeOnly
	defer func() { LegacyMidrangeOnly = saved }()

	LegacyMidrangeOnly = true
	got := DefaultWeightsForArchetype(ArchetypeTokens)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if got != mid {
		t.Errorf("LegacyMidrangeOnly=true should collapse Tokens to midrange; got BoardPresence=%.2f want %.2f",
			got.BoardPresence, mid.BoardPresence)
	}
}
