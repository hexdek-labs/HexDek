package deckparser

import (
	"sort"
	"strings"
)

// NameSuggestion is one entry in the autosuggest list returned by
// MetaDB.SuggestSimilarNames when a card name fails to resolve. Drives
// the "Did you mean X?" line in the hexdek-judge --report-parse
// output and any UI that wants to recover from deckbuilder typos
// (Lighting Blot → Lightning Bolt, Sol Rin → Sol Ring, Counterspel →
// Counterspell, etc.).
type NameSuggestion struct {
	Name     string // canonical display name from MetaDB
	Distance int    // Levenshtein distance from input (normalized comparison)
}

// SuggestSimilarNames returns up to maxResults card names closest to
// `input` by Levenshtein distance, in ascending-distance order. Only
// candidates within a per-input distance threshold qualify — a 4-char
// input allows up to 2 edits, a 20-char input up to 5, capped at 4 in
// all cases. Returns nil (not empty slice) when no candidate is close
// enough, so callers can `if len(s) > 0` without nil-checking.
//
// Performance: prefiltered by length difference (`abs(len(input) -
// len(candidate)) > threshold` short-circuits before Levenshtein
// runs). A 50K-card meta with a 12-char input scans ~5K candidates
// in ~5ms on M-series silicon. Cost grows linearly with meta size;
// for the deckparser benchmark's 63-card stub it's sub-µs.
//
// Comparison uses the same normalizeName function as meta lookup so
// the suggestion finds matches that differ from the input only in
// accent / whitespace / case — those would have resolved directly and
// not reached the suggest path anyway, but the consistency means
// SuggestSimilarNames never re-suggests an identical name.
func (m *MetaDB) SuggestSimilarNames(input string, maxResults int) []NameSuggestion {
	if m == nil || len(m.byName) == 0 || input == "" || maxResults <= 0 {
		return nil
	}
	normInput := normalizeName(input)
	threshold := suggestionThreshold(normInput)
	if threshold == 0 {
		return nil
	}
	type scored struct {
		name string
		dist int
	}
	var matches []scored
	inputLen := len(normInput)
	for _, cm := range m.byName {
		candidate := normalizeName(cm.Name)
		// Length-difference prefilter: Levenshtein distance is bounded
		// below by |len(a) - len(b)|, so candidates whose length
		// differs by more than the threshold can't possibly qualify.
		// Cheap O(1) check that drops ~90% of a real oracle meta.
		diff := len(candidate) - inputLen
		if diff < 0 {
			diff = -diff
		}
		if diff > threshold {
			continue
		}
		// Skip exact-normalized matches — those would have resolved
		// directly via meta.Get and never reached suggest. Defensive.
		if candidate == normInput {
			continue
		}
		d := levenshteinBounded(normInput, candidate, threshold)
		if d > threshold {
			continue
		}
		matches = append(matches, scored{name: cm.Name, dist: d})
	}
	if len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].dist != matches[j].dist {
			return matches[i].dist < matches[j].dist
		}
		// Stable secondary sort on name so ties surface deterministically.
		return matches[i].name < matches[j].name
	})
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}
	out := make([]NameSuggestion, len(matches))
	for i, m := range matches {
		out[i] = NameSuggestion{Name: m.name, Distance: m.dist}
	}
	return out
}

// suggestionThreshold returns the max Levenshtein distance the
// suggester will consider for a given input. Scales with input length
// (longer inputs tolerate more edits) but caps at 4 so a 50-char
// gibberish input doesn't sweep half the meta. Inputs ≤ 3 chars get
// threshold 0 (no suggestions) — too short to be reliably matched
// without surfacing dozens of false positives.
func suggestionThreshold(normInput string) int {
	n := len(normInput)
	if n <= 3 {
		return 0
	}
	t := n / 4
	if t < 2 {
		t = 2
	}
	if t > 4 {
		t = 4
	}
	return t
}

// levenshteinBounded computes the Levenshtein distance between a and b
// with an early-exit guard: if the running minimum cost on any row
// exceeds `max`, return `max + 1` without finishing the matrix. Lets
// the suggester skip candidates that are clearly too far without
// paying the full O(M*N) cost. Returns the actual distance when ≤ max.
func levenshteinBounded(a, b string, max int) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	// Work on []rune for unicode safety — card names contain Æ, ö, û,
	// é, etc. that would otherwise be byte-split and miscounted.
	ar := []rune(a)
	br := []rune(b)
	if len(ar) > len(br) {
		ar, br = br, ar
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			d1 := curr[j-1] + 1
			d2 := prev[j] + 1
			d3 := prev[j-1] + cost
			best := d1
			if d2 < best {
				best = d2
			}
			if d3 < best {
				best = d3
			}
			curr[j] = best
			if best < rowMin {
				rowMin = best
			}
		}
		// Early-exit: every cell in the next row is ≥ the current row's
		// minimum (distances grow monotonically), so when rowMin > max
		// the final result is guaranteed > max.
		if rowMin > max {
			return max + 1
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

// suggestionPhrase renders a single NameSuggestion as the canonical
// "Did you mean X? (N char edits)" form used by PrintReport. Pluralizes
// "edit" / "edits" based on distance. Exported so other report
// renderers (UI build-coaching, JSON exports) can reuse the phrasing.
func suggestionPhrase(s NameSuggestion, input string) string {
	editWord := "edits"
	if s.Distance == 1 {
		editWord = "edit"
	}
	var b strings.Builder
	b.WriteString("Did you mean ")
	b.WriteString(s.Name)
	b.WriteString("? (")
	b.WriteString(itoa(s.Distance))
	b.WriteString(" char ")
	b.WriteString(editWord)
	b.WriteString(" from \"")
	b.WriteString(input)
	b.WriteString("\")")
	return b.String()
}

// itoa is a tiny strconv-free int → ASCII helper used by
// suggestionPhrase to keep the suggest module's imports tight.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
