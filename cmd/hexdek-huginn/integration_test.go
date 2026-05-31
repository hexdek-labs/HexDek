package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/huginn"
)

// ---------------------------------------------------------------------------
// HUGINN 2.0 — End-to-end integration test.
//
// Walks the full pipeline across the shipped Huginn 2.0 surfaces:
//
//   1. Load a known-combo "replay" (synthetic raw observations).
//   2. Run huginn.Ingest and verify the pattern tier-graduates to
//      CONFIRMED.
//   3. Build a deck containing both combo cards and verify --predict
//      surfaces the pair as a DirectPair.
//   4. Run huginn.Partners against one combo card and verify the
//      other card is in the returned partner list.
//   5. Build a deck missing one combo card and verify huginn.Extend
//      recommends the missing card.
//
// The replay is synthetic — building it on disk through the canonical
// PersistRawObservationsRaw API exercises the ingestion path the
// tournament runner uses in production. Tier graduation runs against
// the real Ingest implementation, not a stub. Each pipeline step
// reads from the shared on-disk corpus the previous step wrote, so
// any silent breakage in the persistence layer surfaces here.
// ---------------------------------------------------------------------------

// loadKnownComboReplay seeds raw observations representing the
// Blood Artist + Phyrexian Altar drain combo firing across multiple
// games + multiple decks at high impact. Tier 3 thresholds are
// observation_count >= 5 AND avg_impact >= 5.0; the seed provides 8
// observations across 3 decks at impact 8.0 each, well clear of both
// thresholds.
//
// Returns the temp dir holding the corpus.
func loadKnownComboReplay(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	observations := []huginn.RawObservation{
		// 8 firings across 3 distinct decks. All sharing a single
		// canonical effect pattern ("creature_etb → opp_lose_life")
		// so they aggregate into the same LearnedInteraction.
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.0, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g1", DeckNames: []string{"yawgmoth_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.0, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g2", DeckNames: []string{"yawgmoth_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.5, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g3", DeckNames: []string{"yawgmoth_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 7.5, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g4", DeckNames: []string{"meren_reanimator"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.0, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g5", DeckNames: []string{"meren_reanimator"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.2, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g6", DeckNames: []string{"meren_reanimator"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.8, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g7", DeckNames: []string{"korvold_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Phyrexian Altar", ImpactScore: 8.3, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g8", DeckNames: []string{"korvold_aristocrats"}},
		// Add a secondary pair so Partners has a non-trivial result
		// set + Extend has a candidate beyond Phyrexian Altar.
		{CardA: "Blood Artist", CardB: "Zulaport Cutthroat", ImpactScore: 6.5, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g9", DeckNames: []string{"yawgmoth_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Zulaport Cutthroat", ImpactScore: 6.8, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g10", DeckNames: []string{"meren_reanimator"}},
		{CardA: "Blood Artist", CardB: "Zulaport Cutthroat", ImpactScore: 7.0, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g11", DeckNames: []string{"korvold_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Zulaport Cutthroat", ImpactScore: 6.6, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g12", DeckNames: []string{"yawgmoth_aristocrats"}},
		{CardA: "Blood Artist", CardB: "Zulaport Cutthroat", ImpactScore: 6.9, EffectPattern: "creature_etb_into_opp_lose_life", GameID: "g13", DeckNames: []string{"meren_reanimator"}},
	}
	if err := huginn.PersistRawObservationsRaw(dir, observations); err != nil {
		t.Fatalf("seed raw observations: %v", err)
	}
	return dir
}

// TestHuginn2_FullPipeline is the end-to-end integration test: it
// runs every stage of the Huginn 2.0 pipeline against a shared
// in-memory corpus and asserts the contract at each stage. The
// stages share state via the on-disk huginn data dir (just like
// production), so any persistence-layer breakage surfaces as a
// downstream-stage failure rather than a hidden bug.
func TestHuginn2_FullPipeline(t *testing.T) {
	dir := loadKnownComboReplay(t)

	// Stage 1: Ingest the replay → verify tier-graduation to CONFIRMED.
	promotions, err := huginn.Ingest(dir, 0)
	if err != nil {
		t.Fatalf("stage 1: ingest: %v", err)
	}
	if len(promotions) == 0 {
		t.Fatal("stage 1: expected at least one Tier 3 promotion (Blood Artist + Phyrexian Altar at 8 obs / 3 decks / impact ~8) but got 0")
	}

	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		t.Fatalf("stage 1: read learned: %v", err)
	}
	var bloodArtistInteraction *huginn.LearnedInteraction
	for i, li := range interactions {
		if containsExamplePair(li, "Blood Artist", "Phyrexian Altar") {
			bloodArtistInteraction = &interactions[i]
		}
	}
	if bloodArtistInteraction == nil {
		t.Fatal("stage 1: Blood Artist + Phyrexian Altar interaction not found after ingest")
	}
	if bloodArtistInteraction.Tier != huginn.TierConfirmed {
		t.Errorf("stage 1: interaction tier = %d, want %d (CONFIRMED) — obs=%d unique_decks=%d avg_impact=%.2f",
			bloodArtistInteraction.Tier, huginn.TierConfirmed,
			bloodArtistInteraction.ObservationCount,
			bloodArtistInteraction.UniqueDeckCount,
			bloodArtistInteraction.AvgImpactScore)
	}
	if bloodArtistInteraction.ObservationCount < 8 {
		t.Errorf("stage 1: observation count = %d, want >=8", bloodArtistInteraction.ObservationCount)
	}
	if bloodArtistInteraction.UniqueDeckCount != 3 {
		t.Errorf("stage 1: unique deck count = %d, want 3 (yawgmoth, meren, korvold)",
			bloodArtistInteraction.UniqueDeckCount)
	}

	// Stage 2: --predict surfaces the combo as a DirectPair for a
	// deck containing both cards.
	deckWithBoth := &PredictDeckInput{
		DeckName:  "yawgmoth_aristocrats",
		Commander: "Yawgmoth, Thran Physician",
		Cards: []string{
			"Yawgmoth, Thran Physician",
			"Blood Artist",
			"Phyrexian Altar",
			"Zulaport Cutthroat",
			"Sol Ring",
		},
	}
	predictReport, err := Predict(deckWithBoth, dir)
	if err != nil {
		t.Fatalf("stage 2: predict: %v", err)
	}
	foundDirect := false
	for _, p := range predictReport.DirectPairs {
		aLower := strings.ToLower(p.CardA)
		bLower := strings.ToLower(p.CardB)
		if (aLower == "blood artist" && bLower == "phyrexian altar") ||
			(aLower == "phyrexian altar" && bLower == "blood artist") {
			foundDirect = true
			if p.Confidence < 8 {
				t.Errorf("stage 2: DirectPair confidence = %d, want >=8 (observation count)", p.Confidence)
			}
		}
	}
	if !foundDirect {
		t.Errorf("stage 2: --predict did not surface Blood Artist + Phyrexian Altar as DirectPair; got %d direct pairs: %v",
			predictReport.DirectPairCount, predictReport.DirectPairs)
	}

	// Stage 3: Partners(Blood Artist) returns Phyrexian Altar.
	partners, err := huginn.Partners(dir, "Blood Artist", huginn.TierRecurring)
	if err != nil {
		t.Fatalf("stage 3: partners: %v", err)
	}
	if len(partners) == 0 {
		t.Fatal("stage 3: Partners returned zero hits for Blood Artist (Tier 2+)")
	}
	hasPhyrexianAltar := false
	hasZulaport := false
	for _, p := range partners {
		switch strings.ToLower(p.Partner) {
		case "phyrexian altar":
			hasPhyrexianAltar = true
			if p.Tier != huginn.TierConfirmed {
				t.Errorf("stage 3: Phyrexian Altar partner tier = %d, want %d (CONFIRMED)",
					p.Tier, huginn.TierConfirmed)
			}
		case "zulaport cutthroat":
			hasZulaport = true
		}
	}
	if !hasPhyrexianAltar {
		t.Errorf("stage 3: Partners(Blood Artist) missing Phyrexian Altar; got %v", partners)
	}
	if !hasZulaport {
		t.Errorf("stage 3: Partners(Blood Artist) missing Zulaport Cutthroat (Tier 2+ pair seeded); got %v", partners)
	}

	// Stage 4: Extend on a deck containing Blood Artist (but NOT
	// Phyrexian Altar or Zulaport) recommends both missing
	// partners as candidate adds.
	deckMissingPartners := []string{"Blood Artist", "Sol Ring", "Cultivate"}
	extendHits, err := huginn.Extend(dir, deckMissingPartners, huginn.TierRecurring)
	if err != nil {
		t.Fatalf("stage 4: extend: %v", err)
	}
	if len(extendHits) == 0 {
		t.Fatal("stage 4: Extend returned zero candidates for a deck with Blood Artist + no partners")
	}
	hasAltar := false
	hasZulaportExtend := false
	for _, h := range extendHits {
		switch strings.ToLower(h.Card) {
		case "phyrexian altar":
			hasAltar = true
			if h.Tier != huginn.TierConfirmed {
				t.Errorf("stage 4: Phyrexian Altar extend tier = %d, want %d (CONFIRMED)",
					h.Tier, huginn.TierConfirmed)
			}
			if h.Pairs == 0 {
				t.Errorf("stage 4: Phyrexian Altar Pairs = 0, want >=1 (Blood Artist is in deck)")
			}
		case "zulaport cutthroat":
			hasZulaportExtend = true
		}
	}
	if !hasAltar {
		t.Errorf("stage 4: Extend did not recommend Phyrexian Altar; got %v",
			candidateNames(extendHits))
	}
	if !hasZulaportExtend {
		t.Errorf("stage 4: Extend did not recommend Zulaport Cutthroat; got %v",
			candidateNames(extendHits))
	}
}

// TestHuginn2_PipelineConsistency_PartnersMatchPredict pins the
// cross-surface invariant: every partner returned by huginn.Partners
// for a deck card MUST appear as either a DirectPair (if the partner
// is in the deck) or an Extend candidate (if it isn't). The two
// query surfaces (Partners-of-X) and (Predict + Extend) draw from
// the same corpus and should agree on what pairs with what.
func TestHuginn2_PipelineConsistency_PartnersMatchPredict(t *testing.T) {
	dir := loadKnownComboReplay(t)
	if _, err := huginn.Ingest(dir, 0); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	// Two-card deck: Blood Artist + Phyrexian Altar. Partners of
	// Blood Artist should include Phyrexian Altar (in deck) AND
	// Zulaport Cutthroat (not in deck). The first must appear in
	// Predict.DirectPairs; the second must appear in Extend.
	predictDeck := &PredictDeckInput{
		Cards: []string{"Blood Artist", "Phyrexian Altar"},
	}
	predictReport, err := Predict(predictDeck, dir)
	if err != nil {
		t.Fatalf("predict: %v", err)
	}
	predictSet := map[string]bool{}
	for _, p := range predictReport.DirectPairs {
		predictSet[strings.ToLower(p.CardA)] = true
		predictSet[strings.ToLower(p.CardB)] = true
	}

	extendHits, err := huginn.Extend(dir, predictDeck.Cards, huginn.TierRecurring)
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	extendSet := map[string]bool{}
	for _, h := range extendHits {
		extendSet[strings.ToLower(h.Card)] = true
	}

	partners, err := huginn.Partners(dir, "Blood Artist", huginn.TierRecurring)
	if err != nil {
		t.Fatalf("partners: %v", err)
	}
	for _, p := range partners {
		nameLower := strings.ToLower(p.Partner)
		inDeck := nameLower == "phyrexian altar"
		if inDeck {
			if !predictSet[nameLower] {
				t.Errorf("Partner %q is in deck but not in Predict.DirectPairs (predict set: %v)",
					p.Partner, predictSet)
			}
		} else {
			if !extendSet[nameLower] {
				t.Errorf("Partner %q is NOT in deck but not in Extend.Hits (extend set: %v)",
					p.Partner, extendSet)
			}
		}
	}
}

// TestHuginn2_TierGradingThresholdsHonored verifies the Ingest
// gating: 4 observations (below the 5-min threshold) leaves the
// pattern at Tier 1, NOT Tier 3, even though impact would pass.
// Catches accidental threshold drift in tier promotion logic.
func TestHuginn2_TierGradingThresholdsHonored(t *testing.T) {
	dir := t.TempDir()
	observations := []huginn.RawObservation{
		// 4 observations on 1 deck — below both the Tier 3 (5 obs)
		// and Tier 2 (3 obs + 2 decks) thresholds.
		{CardA: "X", CardB: "Y", ImpactScore: 9.0, EffectPattern: "p", GameID: "g1", DeckNames: []string{"deck1"}},
		{CardA: "X", CardB: "Y", ImpactScore: 9.0, EffectPattern: "p", GameID: "g2", DeckNames: []string{"deck1"}},
		{CardA: "X", CardB: "Y", ImpactScore: 9.0, EffectPattern: "p", GameID: "g3", DeckNames: []string{"deck1"}},
		{CardA: "X", CardB: "Y", ImpactScore: 9.0, EffectPattern: "p", GameID: "g4", DeckNames: []string{"deck1"}},
	}
	if err := huginn.PersistRawObservationsRaw(dir, observations); err != nil {
		t.Fatal(err)
	}
	if _, err := huginn.Ingest(dir, 0); err != nil {
		t.Fatal(err)
	}
	interactions, err := huginn.ReadLearnedInteractions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(interactions))
	}
	if interactions[0].Tier >= huginn.TierConfirmed {
		t.Errorf("interaction promoted to Tier %d at 4 obs / 1 deck; want < %d (CONFIRMED)",
			interactions[0].Tier, huginn.TierConfirmed)
	}
}

// containsExamplePair is a small predicate used in stage 1 to find
// the seeded interaction without depending on internal ExampleCards
// ordering. Compares case-insensitively to match Huginn's lookup
// semantics.
func containsExamplePair(li huginn.LearnedInteraction, a, b string) bool {
	wantKey := canonicalPairKey(strings.ToLower(a), strings.ToLower(b))
	for _, ex := range li.ExampleCards {
		parts := strings.SplitN(ex, " + ", 2)
		if len(parts) != 2 {
			continue
		}
		gotKey := canonicalPairKey(strings.ToLower(parts[0]), strings.ToLower(parts[1]))
		if gotKey == wantKey {
			return true
		}
	}
	return false
}

func candidateNames(hits []huginn.ExtendHit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Card)
	}
	return out
}
