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
//   - r92: this loop used to ALSO write Flags["kw:<name>"] beside each
//     counter, justified in a comment as mirroring "the pattern other
//     counter-grants use so combat checks see the keyword immediately".
//     Both halves of that are wrong, and measured wrong. The flag is
//     redundant on the way in — a keyword counter already answers true
//     on both authorities with no flag (CR 122.1b; HasKeyword surface 4
//     reads Counters, and the layer fold reads the same store). And it
//     is PERMANENT on the way out: the flag has no provenance and
//     nothing removes it, so once a counter is taken off (Vampire
//     Hexmage, Hex Parasite, Thief of Blood, Solemnity) the keyword
//     stays on the creature for the rest of the game. That matters more
//     on this card than on any other, because here the counters ARE the
//     keywords. Nor was it "the pattern": grep found no other handler
//     of this shape, and the canonical placer writes no such flag. What
//     the loop actually needed was the characteristics-cache
//     invalidation it never did.
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
	}
	if len(gainedCounters) > 0 {
		// The counter IS the grant (CR 122.1b) and both keyword
		// authorities already read it: Permanent.HasKeyword surface 4
		// checks Counters directly, and runtimeKeywordSources folds the
		// same store for the layer query. Invalidating is the whole of
		// what the grant needs here.
		gs.InvalidateCharacteristicsCache()
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
