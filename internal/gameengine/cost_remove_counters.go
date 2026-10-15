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

// parseSmallCountWord parses a FIXED activation-cost count: a positive
// decimal, or an English number word. A variable count ("x", "any number",
// "all") returns ok=false and the caller declines to enforce.
//
// r123 — THE WORD LIST WAS THE ENFORCEMENT BOUNDARY, AND IT STOPPED AT TEN.
// A counter-removal cost this function cannot parse is not paid at all, so
// the ability becomes FREE — the exact non-termination hazard the comment at
// the call site in activation.go warns about. Two corpus cards sat outside
// the list on a fixed count:
//
//	Fire Nation Turret     "remove fifty charge counters: deal 50 damage to
//	                        any target"   — no mana cost, no tap cost
//	Trenzalore Clocktower  "{1}{U}, {T}, remove twelve time counters …"
//
// Measured before the fix: Fire Nation Turret accepted 5 of 5 activations
// holding ZERO charge counters and took an opponent from 20 to -30 life.
// It is in 3 of 1,758 live decks.
//
// The remaining unparsed shapes are deliberate and counted (r123): 26 "remove
// X", 18 "remove any number of", 9 "remove all", 20 targeting another
// permanent. The first two are self-limiting — removing zero is a legal
// payment and every such effect scales off "counters removed this way", so a
// free activation does nothing. "remove all" and the other-permanent form
// need a count this signature cannot see; see the r123 write-up.
func parseSmallCountWord(w string) (int, bool) {
	if n, ok := englishNumberWords[w]; ok {
		return n, true
	}
	if n, err := strconv.Atoi(w); err == nil && n > 0 {
		return n, true
	}
	return 0, false
}

// englishNumberWords covers the fixed counts that appear in printed costs.
// "a"/"an" are the article form of one.
var englishNumberWords = map[string]int{
	"a": 1, "an": 1, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
	"sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90, "hundred": 100,
}
