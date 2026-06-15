package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// RemoveCounterCostSpec extracts a "remove N <kind> counters from this" cost
// from an activated ability's cost (CR §602.1b). It reads the structured
// Cost.RemoveCountersN / RemoveCountersKnd first, then falls back to parsing
// the raw Cost.Extra string form the parser emits when it doesn't populate the
// structured fields ("remove three spore counters from this creature").
//
// It returns ok=false for any shape it cannot parse with confidence — a
// variable count ("remove X loyalty counters"), a missing kind ("remove a
// counter"), or "remove all …" — so enforcement is purely additive and can
// never wrongly BLOCK a legal activation. The kind is lowercased to match the
// gs.Permanent.Counters key convention ("spore", "+1/+1", "charge", …).
func RemoveCounterCostSpec(c gameast.Cost) (n int, kind string, ok bool) {
	if c.RemoveCountersN != nil && *c.RemoveCountersN > 0 && strings.TrimSpace(c.RemoveCountersKnd) != "" {
		return *c.RemoveCountersN, strings.ToLower(strings.TrimSpace(c.RemoveCountersKnd)), true
	}
	for _, ex := range c.Extra {
		s := strings.ToLower(strings.TrimSpace(ex))
		if !strings.HasPrefix(s, "remove ") || !strings.Contains(s, "counter") {
			continue
		}
		fields := strings.Fields(s)
		// Locate the "counter"/"counters" token.
		ci := -1
		for i, f := range fields {
			if f == "counter" || f == "counters" {
				ci = i
				break
			}
		}
		// Need at least "remove <count> <kind> counter" → ci >= 3 with the
		// kind tokens sitting at fields[2:ci].
		if ci < 3 || ci >= len(fields) {
			continue
		}
		cnt, okc := parseSmallCountWord(fields[1])
		if !okc {
			continue
		}
		kind := strings.TrimSpace(strings.Join(fields[2:ci], " "))
		if kind == "" {
			continue
		}
		return cnt, kind, true
	}
	return 0, "", false
}

// parseSmallCountWord parses a small fixed activation-cost count: a positive
// decimal, or a number word "a"/"an"/"one" through "ten". A variable or
// explicitly-X count returns ok=false (the caller then declines to enforce).
func parseSmallCountWord(w string) (int, bool) {
	switch w {
	case "a", "an", "one":
		return 1, true
	case "two":
		return 2, true
	case "three":
		return 3, true
	case "four":
		return 4, true
	case "five":
		return 5, true
	case "six":
		return 6, true
	case "seven":
		return 7, true
	case "eight":
		return 8, true
	case "nine":
		return 9, true
	case "ten":
		return 10, true
	}
	if n, err := strconv.Atoi(w); err == nil && n > 0 {
		return n, true
	}
	return 0, false
}
