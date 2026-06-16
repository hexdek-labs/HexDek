package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// suspend_cast_option.go — the read side of the generic "suspend from hand"
// special action (CR §702.62b). The cast-time machinery (TickSuspendCounters /
// CastSuspendedCard) and the exile primitive (SuspendCard) already exist; this
// file lets a caller (the tournament play loop) discover that a hand card CAN
// be suspended and at what price, so all suspend cards — not just Jhoira's
// hardcoded handler — become suspendable.
//
// The Suspend keyword parses as Args:["N","{cost}"] with Raw "suspend N-{cost}"
// (e.g. Errant Ephemeron -> ["4","{1}{u}"], Lotus Bloom -> ["3","{0}"]). N is
// the number of time counters the card enters exile with; {cost} is the mana
// paid to take the special action — which is NOT the card's mana cost (it is
// usually much cheaper, the whole point of the mechanic).

// HasSuspend reports whether the card carries the Suspend keyword (§702.62).
func HasSuspend(card *Card) bool {
	return cardHasKeywordByName(card, "suspend")
}

// SuspendParams extracts the time-counter count N and the suspend mana cost
// (as a CMC integer) from a card's Suspend keyword. Returns ok=false when the
// card has no Suspend keyword or N cannot be determined.
//
// N MUST be read separately from the cost: keywordArgCostStrict would return
// the FIRST cost-parseable arg, and the bare integer "N" parses as an integer
// cost — so a naive reuse of that helper yields N where the suspend cost is
// wanted. This parser distinguishes the bare-integer arg (N) from the
// {…}-shaped arg (the cost), with a Raw "suspend N-{cost}" fallback.
func SuspendParams(card *Card) (n int, cost int, ok bool) {
	if card == nil || card.AST == nil {
		return 0, 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, isKw := ab.(*gameast.Keyword)
		if !isKw {
			continue
		}
		if strings.ToLower(strings.TrimSpace(kw.Name)) != "suspend" {
			continue
		}
		nFound, costFound := false, false
		for _, arg := range kw.Args {
			switch v := arg.(type) {
			case string:
				s := strings.TrimSpace(v)
				if strings.HasPrefix(s, "{") {
					if c, err := mana.Parse(s); err == nil {
						cost, costFound = c.CMC(), true
					}
				} else if iv, err := strconv.Atoi(s); err == nil && !nFound {
					n, nFound = iv, true
				}
			case float64:
				if !nFound {
					n, nFound = int(v), true
				}
			case int:
				if !nFound {
					n, nFound = v, true
				}
			}
		}
		// Raw fallback: "suspend 4-{1}{u}".
		if !nFound || !costFound {
			rn, rc, rok := parseSuspendRaw(kw.Raw)
			if rok {
				if !nFound {
					n, nFound = rn, true
				}
				if !costFound {
					cost, costFound = rc, true
				}
			}
		}
		_ = costFound // cost defaults to 0 (a {0} suspend) when unparseable
		return n, cost, nFound && n > 0
	}
	return 0, 0, false
}

// parseSuspendRaw parses a "suspend N-{cost}" raw keyword string.
func parseSuspendRaw(raw string) (n int, cost int, ok bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "suspend")
	s = strings.TrimSpace(s)
	// Expect "N-{cost}" or "N {cost}".
	sep := strings.IndexAny(s, "-")
	if sep < 0 {
		// Maybe just "N" with no cost.
		if iv, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return iv, 0, true
		}
		return 0, 0, false
	}
	nPart := strings.TrimSpace(s[:sep])
	costPart := strings.TrimSpace(s[sep+1:])
	iv, err := strconv.Atoi(nPart)
	if err != nil {
		return 0, 0, false
	}
	if costPart != "" {
		if c, err := mana.Parse(costPart); err == nil {
			cost = c.CMC()
		}
	}
	return iv, cost, true
}
