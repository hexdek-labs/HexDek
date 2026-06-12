package main

import (
	"strings"

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

// deckCardFromOracle resolves one list entry against the oracle DB,
// applying the same front-face fallbacks the original checks used.
func deckCardFromOracle(name string, qty int, oracle *oracleDB) judge.DeckCard {
	dc := judge.DeckCard{Name: name, Qty: qty}
	if name == "" {
		return dc
	}
	entry := oracle.lookup(name)
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
