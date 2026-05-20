package deckid

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode"

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

// normalizeName lowercases and folds accents for consistent hashing.
// Duplicated from deckparser to avoid circular imports.
func normalizeName(name string) string {
	out := make([]rune, 0, len(name))
	prevSpace := false
	for _, r := range name {
		r = foldAccent(r)
		if unicode.IsUpper(r) {
			r = unicode.ToLower(r)
		}
		if unicode.IsSpace(r) {
			if prevSpace || len(out) == 0 {
				continue
			}
			out = append(out, ' ')
			prevSpace = true
			continue
		}
		prevSpace = false
		out = append(out, r)
	}
	if n := len(out); n > 0 && out[n-1] == ' ' {
		out = out[:n-1]
	}
	return string(out)
}

func foldAccent(r rune) rune {
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å', 'ā',
		'Á', 'À', 'Â', 'Ä', 'Ã', 'Å', 'Ā':
		return 'a'
	case 'é', 'è', 'ê', 'ë', 'ē',
		'É', 'È', 'Ê', 'Ë', 'Ē':
		return 'e'
	case 'í', 'ì', 'î', 'ï', 'ī',
		'Í', 'Ì', 'Î', 'Ï', 'Ī':
		return 'i'
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'ō',
		'Ó', 'Ò', 'Ô', 'Ö', 'Õ', 'Ō':
		return 'o'
	case 'ú', 'ù', 'û', 'ü', 'ū',
		'Ú', 'Ù', 'Û', 'Ü', 'Ū':
		return 'u'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ç', 'Ç':
		return 'c'
	case 'ß':
		return 's'
	}
	return r
}
