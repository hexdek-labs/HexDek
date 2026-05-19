package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIndominusRexAlpha wires Indominus Rex, Alpha.
//
// Oracle text (Scryfall, verified):
//
//	As Indominus Rex enters, discard any number of creature cards. It
//	enters with a flying counter on it if a card discarded this way
//	has flying. The same is true for first strike, double strike,
//	deathtouch, hexproof, haste, indestructible, lifelink, menace,
//	reach, trample, and vigilance.
//	When Indominus Rex enters, draw a card for each counter on it.
//
// Implementation (R43 stub port):
//   - The "as it enters" choice is a replacement effect at would-be-
//     entered; per_card runs at post-ETB, so we approximate the
//     observable effect by discarding from hand inside the OnETB and
//     stamping counters before the draw-N pulls cards. AI policy is
//     greedy-upside: discard every creature card in hand that carries
//     any of the 12 listed keywords (each unique keyword grants one
//     counter, so duplicates among discards still yield the same set
//     of counters — but each "card discarded this way" is still
//     graveyarded, which is correct per oracle).
//   - "Draw a card for each counter on it": after the discard +
//     counter pass, sum every counter type on the permanent and
//     drawOne that many times. Other counter sources that happened
//     during the ETB would also be counted, which matches the printed
//     "for each counter" rather than "for each keyword counter".
func registerIndominusRexAlpha(r *Registry) {
	r.OnETB("Indominus Rex, Alpha", indominusRexAlphaETB)
}

var indominusKeywordCounters = []string{
	"flying", "first strike", "double strike", "deathtouch",
	"hexproof", "haste", "indestructible", "lifelink",
	"menace", "reach", "trample", "vigilance",
}

func indominusRexAlphaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "indominus_rex_alpha_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Walk hand and gather creature cards bearing any of the 12
	// keywords. Each such card gets discarded; the set of keywords
	// represented becomes the set of counters added.
	discarded := []string{}
	gainedCounters := map[string]bool{}
	pending := []*gameengine.Card{}
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") {
			continue
		}
		anyKw := false
		for _, kw := range indominusKeywordCounters {
			if cardHasKeyword(c, kw) {
				gainedCounters[kw] = true
				anyKw = true
			}
		}
		if anyKw {
			pending = append(pending, c)
		}
	}
	for _, c := range pending {
		gameengine.DiscardCard(gs, c, perm.Controller)
		discarded = append(discarded, c.DisplayName())
	}
	for kw := range gainedCounters {
		perm.AddCounter(kw, 1)
		// Engine keyword consumers look at Flags["kw:<name>"] alongside
		// the counter; mirror the pattern other counter-grants use so
		// combat checks see the keyword immediately.
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["kw:"+kw] = 1
	}

	// Draw a card for each counter — sum across all kinds (oracle
	// reading: "each counter on it", not just keyword counters).
	totalCounters := 0
	for _, n := range perm.Counters {
		if n > 0 {
			totalCounters += n
		}
	}
	drawn := 0
	for i := 0; i < totalCounters; i++ {
		c := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		if c == nil {
			break
		}
		drawn++
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"discarded": discarded,
		"counters":  totalCounters,
		"drawn":     drawn,
	})
}
