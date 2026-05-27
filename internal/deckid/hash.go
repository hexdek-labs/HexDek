package deckid

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

type DeckHash string

// ComputeHash produces a content-addressable hash from commander cards and
// library cards. Two decks with identical card lists produce the same hash
// regardless of ordering, whitespace, or casing in the source file.
func ComputeHash(commanders []*gameengine.Card, library []*gameengine.Card) DeckHash {
	var lines []string
	for _, c := range commanders {
		if c == nil {
			continue
		}
		lines = append(lines, "COMMANDER:"+normalizeName(c.DisplayName()))
	}
	for _, c := range library {
		if c == nil {
			continue
		}
		lines = append(lines, "1x "+normalizeName(c.DisplayName()))
	}
	sort.Strings(lines)
	payload := strings.Join(lines, "\n")
	h := sha256.Sum256([]byte(payload))
	return DeckHash(fmt.Sprintf("%x", h))
}

// ComputeHashFromDeck is a convenience wrapper for TournamentDeck.
func ComputeHashFromDeck(td *deckparser.TournamentDeck) DeckHash {
	if td == nil {
		return ""
	}
	return ComputeHash(td.CommanderCards, td.Library)
}

// CardList returns the canonical sorted card list for a deck (for storage).
func CardList(commanders []*gameengine.Card, library []*gameengine.Card) []string {
	var lines []string
	for _, c := range commanders {
		if c == nil {
			continue
		}
		lines = append(lines, "COMMANDER:"+normalizeName(c.DisplayName()))
	}
	for _, c := range library {
		if c == nil {
			continue
		}
		lines = append(lines, "1x "+normalizeName(c.DisplayName()))
	}
	sort.Strings(lines)
	return lines
}

// CardDelta computes the number of card changes between two sorted card lists.
// Returns added + removed counts (cards added going parent→child, plus cards
// removed). Multiplicity-aware: a deck swapping 30 Plains for 35 Plains is a
// delta of 5, not 0 — Commander allows duplicates only for basics, but those
// basics are the primary mana-base tuning surface.
func CardDelta(parentList, childList []string) int {
	parentCounts := make(map[string]int, len(parentList))
	for _, c := range parentList {
		parentCounts[c]++
	}
	childCounts := make(map[string]int, len(childList))
	for _, c := range childList {
		childCounts[c]++
	}
	delta := 0
	for c, n := range childCounts {
		if extra := n - parentCounts[c]; extra > 0 {
			delta += extra
		}
	}
	for c, n := range parentCounts {
		if extra := n - childCounts[c]; extra > 0 {
			delta += extra
		}
	}
	return delta
}

// normalizeName forwards to deckparser's canonical implementation
// (R60 Phase 2C consolidation — used to be a verbatim copy of the
// deckparser body plus its `foldAccent` helper). deckid already
// imports deckparser, so there's no cycle to avoid — the old
// "duplicated to avoid circular imports" comment on the prior copy
// was stale.
//
// The CleanCardName pre-pass defends against Card values that bypassed
// the parser and still carry printing decoration ("Forest (THB) 270",
// "[MID] Forest", "Forest *F-Etched*"). Without it, the same logical
// content hashes to different deck IDs depending on which import path
// produced the Card — defeating the content-addressing guarantee.
func normalizeName(name string) string {
	return deckparser.NormalizeName(deckparser.CleanCardName(name))
}
