package main

import (
	"regexp"
	"strings"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/judge"
)

// legality_judge.go — freya is a DRIVER of the Hex Judge's LEGALITY
// dimension (fold r63; the standalone cmd/hexdek-freya/legality.go
// duplicate is deleted). This file only adapts freya's deck data into
// judge.DeckSubmission and re-exports the report type for the existing
// renderers; the five Commander deck checks live in
// internal/judge/legality.go and emit canonical violations through
// LogViolation as they run.

// LegalityReport aliases the canonical deck-legality report so
// analysis.go / report.go / the JSON wire format are unchanged.
type LegalityReport = judge.DeckLegalityReport

// CheckLegality builds the Judge's neutral deck submission from freya's
// oracle-resolved profiles and runs the canonical checks.
func CheckLegality(report *FreyaReport, qtyProfiles []CardProfileQty, oracle *oracleDB) *LegalityReport {
	sub := judge.DeckSubmission{
		Commander:  deckCardFromOracle(report.Commander, 1, oracle),
		TotalCards: report.TotalCards,
	}
	cmdrNorm := normalizeName(report.Commander)
	for _, qp := range qtyProfiles {
		// The commander is carried separately (color-identity check
		// skips it by construction).
		if report.Commander != "" && normalizeName(qp.Profile.Name) == cmdrNorm {
			continue
		}
		sub.Cards = append(sub.Cards, deckCardFromOracle(qp.Profile.Name, qp.Qty, oracle))
	}
	return judge.CheckDeckLegality(sub)
}

// RecomputeLegality re-runs ONLY the deck-legality checks against the
// deck as it is RIGHT NOW, independent of any cached analysis.
//
// Legality is a function of the LITERAL deck text (card count / color
// identity / singleton / banned / commander-exists), but Freya's report
// cache is content-addressed by a key that deliberately normalizes text
// away — normalizeForCacheKey runs deckparser.CleanCardName (strips the
// "(SET) N" printing suffix) then NormalizeName (folds casing /
// punctuation). Two decks that differ only in normalized-away text hash
// to the SAME cache key yet can carry DIFFERENT legality: a deck first
// imported with a malformed commander line that fails to resolve
// ("commander not found") caches an INVALID report, and the corrected
// deck — which normalizes to the same key — would otherwise be served
// that poisoned "illegal" verdict forever (the verdict is baked inside
// the cached FreyaReport).
//
// So legality must never be served from cache. This rebuilds the Judge
// submission from the CURRENT commander + card quantities and runs the
// canonical checks fresh. It is cheap (a handful of oracle lookups)
// next to the full cached analysis, so recomputing per read is nearly
// free.
//
// totalCards is carried from the cached report: the card COUNT is
// invariant under the cache key (quantities participate in the hash and
// the set-code suffix never changes a count), so only the
// name-resolution-dependent checks can actually differ between two
// key-equivalent decks. cardQtys must EXCLUDE the commander, matching
// parseDeckListWithQuantities (CheckLegality also filters the commander
// out by name as a defensive second pass).
func RecomputeLegality(commander string, cardQtys map[string]int, oracle *oracleDB, totalCards int) *LegalityReport {
	qtyProfiles := make([]CardProfileQty, 0, len(cardQtys))
	for name, qty := range cardQtys {
		if qty <= 0 {
			continue
		}
		qtyProfiles = append(qtyProfiles, CardProfileQty{
			Profile: CardProfile{Name: name},
			Qty:     qty,
		})
	}
	// A minimal stub report carries only the two fields CheckLegality
	// reads off the report itself (Commander + TotalCards); every other
	// legality input comes from qtyProfiles + oracle.
	stub := &FreyaReport{Commander: commander, TotalCards: totalCards}
	return CheckLegality(stub, qtyProfiles, oracle)
}

// deckCardFromOracle resolves one list entry against the oracle DB,
// applying the same front-face fallbacks the original checks used.
func deckCardFromOracle(name string, qty int, oracle *oracleDB) judge.DeckCard {
	dc := judge.DeckCard{Name: name, Qty: qty}
	if name == "" {
		return dc
	}
	entry := oracle.lookup(name)
	if entry == nil {
		// Defense-in-depth: a plain-text paste / import can leave a leading
		// deck quantity ("1 King of the Oathbreakers", "3x Sol Ring") or a
		// "(SET) N" printing suffix on the name — which makes a valid card read
		// as ILLEGAL ("not found in oracle database"). Retry with those
		// stripped. Try the RAW name first (above) so legitimate names that
		// begin with digits, e.g. "1996 World Champion", still resolve as-is.
		if norm := normalizeCardLookupName(name); norm != "" && norm != name {
			entry = oracle.lookup(norm)
		}
	}
	if entry == nil {
		return dc
	}
	dc.Resolved = true
	dc.CanonicalName = entry.Name
	dc.ColorIdentity = entry.ColorIdentity
	dc.OracleText = entry.OracleText
	dc.TypeLine = entry.TypeLine
	if len(entry.CardFaces) > 0 {
		if strings.TrimSpace(dc.OracleText) == "" {
			dc.OracleText = entry.CardFaces[0].OracleText
		}
		if strings.TrimSpace(dc.TypeLine) == "" {
			dc.TypeLine = entry.CardFaces[0].TypeLine
		}
	}
	return dc
}


// leadingDeckQtyRE matches a leading deck quantity like "1 ", "12 ", or "3x "
// (Moxfield's "Nx" form). Capped at 3 digits so 4-digit-prefixed card names
// (e.g. "1996 World Champion") are never mistaken for a quantity.
var leadingDeckQtyRE = regexp.MustCompile(`^\s*\d{1,3}x?\s+`)

// normalizeCardLookupName strips a leading deck quantity and a trailing
// "(SET) N" printing suffix from a card line, for a fallback oracle lookup when
// the raw name failed to resolve.
func normalizeCardLookupName(name string) string {
	s := leadingDeckQtyRE.ReplaceAllString(strings.TrimSpace(name), "")
	s = deckparser.CleanCardName(s) // strips a trailing "(SET) N"
	return strings.TrimSpace(s)
}
