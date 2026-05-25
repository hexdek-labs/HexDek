package heimdall

import (
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ExtractMulliganStats walks gs.MulliganHistory (populated by
// tournament.RunLondonMulligan) and returns one MulliganStat per seat
// that ran the mulligan procedure. Seats with no recorded history
// (e.g., an engine-internal unit test that hand-builds a GameState
// without going through the mulligan path) are skipped — there's no
// useful signal for "we never asked." Sorted by seat ascending.
//
// The keepable evaluation mirrors Freya's standard rule from
// computeOpeningHandSim: a hand is keepable when it has 2-5 lands AND
// at least one action card (creature / instant / sorcery /
// planeswalker). The function does NOT compute the
// commander-adjusted variant — that needs commander-CMC and
// ramp/synergy tagging the snapshot doesn't carry. Downstream
// consumers that want the adjusted variant can re-derive from the
// raw Lands / ActionCards counts.
func ExtractMulliganStats(gs *gameengine.GameState) []MulliganStat {
	if gs == nil || len(gs.MulliganHistory) == 0 {
		return nil
	}
	out := make([]MulliganStat, 0, len(gs.MulliganHistory))
	for seatIdx, h := range gs.MulliganHistory {
		// Seats whose hand was zero AND that took zero mulligans never
		// reached the mulligan path — skip the false-positive empty row.
		if len(h.OpeningHand) == 0 && h.MulligansTaken == 0 {
			continue
		}
		lands, actions := classifyMulliganHand(h.OpeningHand)
		out = append(out, MulliganStat{
			Seat:            seatIdx,
			MulligansTaken:  h.MulligansTaken,
			OpeningHandSize: len(h.OpeningHand),
			Lands:           lands,
			ActionCards:     actions,
			Keepable:        lands >= 2 && lands <= 5 && actions >= 1,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Seat < out[j].Seat
	})
	return out
}

// classifyMulliganHand counts lands and action cards in one captured
// opening hand. A card counts as "action" when it has any of the
// canonical action types (creature, instant, sorcery, planeswalker).
// A card with the "land" type counts as a land and is NOT also
// counted as an action even if it carries a creature-land secondary
// type (Dryad Arbor, Treetop Village animated state) — the keepable
// rule is asking about MANA-vs-PLAY balance, and a creature-land
// still only produces mana on its ETB turn.
func classifyMulliganHand(hand []gameengine.MulliganHandEntry) (lands, actions int) {
	for _, c := range hand {
		isLand := false
		isAction := false
		for _, t := range c.Types {
			switch t {
			case "land":
				isLand = true
			case "creature", "instant", "sorcery", "planeswalker":
				isAction = true
			}
		}
		if isLand {
			lands++
			continue
		}
		if isAction {
			actions++
		}
	}
	return
}
