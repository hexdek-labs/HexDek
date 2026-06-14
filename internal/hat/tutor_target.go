package hat

import (
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ChooseTutorTargets implements gameengine.TutorTargetChooser. When a library
// search ("search your library for ...") matches more cards than the tutor
// takes, the engine routes the selection here so the pick is driven by the
// deck's strategy rather than library order or a blunt highest-CMC rule.
//
// The ranking is strategy-aware, in descending priority:
//
//  1. A card that is the SOLE missing piece of a near-assembled combo line
//     (Freya win-line one piece short) — tutoring it sets up the win.
//  2. Cards high on Freya's prioritized TutorTargets list (combo pieces +
//     value-engine keys, already ordered by importance).
//  3. General combo pieces.
//  4. A member of a HIGH-DENSITY synergy cluster — Freya's marker for a real
//     subsystem of THIS deck's gameplan (the landfall-treasure engine the
//     deck is built around), deliberately ranked ABOVE generic value-engine
//     keys and raw card quality so "let the deck shape the wincon" wins over
//     "grab the objectively good card" (e.g. the synergy engine over an
//     Eternal Witness that merely recurs anything).
//  5. A finisher when we're in the execute phase (close the game).
//  6. A removal / board-wipe answer when we're behind or under lethal
//     pressure (the board threat is the thing to find).
//  7. A land when we are mana-screwed (and a penalty for grabbing yet
//     another land when flooded).
//  8. Generic value-engine keys / star cards (raw quality, lowest strategy
//     tier — only above the bare CMC baseline).
//
// CMC breaks ties (Demonic-Tutor-style "grab the biggest bomb"), matching the
// engine's strategy-unaware baseline so a deck with no Freya profile degrades
// gracefully to the old behavior. Constrained tutors (transmute / Trinket /
// Trophy Mage) need no special handling here: the engine has already filtered
// `candidates` to the tutor's legal pool (type + mana value), so this ranks
// the best target WITHIN that pool. Returns nil to defer entirely when there
// is no candidate set or seat is invalid.
func (h *YggdrasilHat) ChooseTutorTargets(gs *gameengine.GameState, seatIdx int, candidates []*gameengine.Card, count int, query gameast.Filter, destination string) []*gameengine.Card {
	if h == nil || gs == nil || count <= 0 || len(candidates) <= count {
		// len <= count: take everything; let the engine handle it.
		return candidates
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) || gs.Seats[seatIdx] == nil {
		return nil
	}

	// Sole-missing combo pieces, computed while this tutor is on the stack
	// (so it is no longer counted as a tutor-in-hand). These complete a win
	// line and dominate every other signal.
	var nearMiss map[string]bool
	if h.comboSeq != nil {
		nearMiss = h.comboSeq.NearMissPieces(gs, seatIdx)
	}

	// Freya TutorTargets priority: earlier in the list = more important.
	tutorRank := map[string]int{}
	if h.Strategy != nil {
		for i, name := range h.Strategy.TutorTargets {
			if _, seen := tutorRank[name]; !seen {
				tutorRank[name] = i
			}
		}
	}

	phase := h.detectPhase(gs, seatIdx)
	underPressure := h.tutorUnderPressure(gs, seatIdx)
	landsInPlay := countLandsInPlay(gs.Seats[seatIdx])
	manaScrewed := landsInPlay <= 2 && gs.Turn >= 3 || landsInPlay < gs.Turn-3
	flooded := landsInPlay >= 7

	// State-aware contextual value: a card whose power scales with the
	// graveyard (delve / escape / flashback / "for each card in your
	// graveyard") is a premium tutor target ONLY when the yard is actually
	// full. Its value is contextual, not static — so we gate the boost on
	// the live graveyard size rather than scoring the card in the abstract.
	graveyardFull := len(gs.Seats[seatIdx].Graveyard) >= 7

	type scored struct {
		card  *gameengine.Card
		score float64
		ord   int
	}
	ranked := make([]scored, 0, len(candidates))
	for i, c := range candidates {
		if c == nil {
			continue
		}
		name := c.DisplayName()
		cmc := float64(gameengine.ManaCostOf(c))
		best := cmc // baseline: highest CMC, identical to the engine fallback.

		consider := func(v float64) {
			if v+cmc > best {
				best = v + cmc
			}
		}

		if nearMiss[name] {
			consider(10000)
		}
		if rank, ok := tutorRank[name]; ok {
			// Higher value for earlier entries; capped so it never beats a
			// live win-line completion.
			consider(5000 + float64(len(h.Strategy.TutorTargets)-rank)*10)
		}
		if h.isComboRelevant(c) {
			consider(3000)
		}
		// Deck-synergy engine: a high-density synergy-cluster member is part
		// of THIS deck's actual subsystem. Ranked above generic value/quality
		// so the deck's win-line shapes the pick, not raw card power.
		if h.isHighDensitySynergyMember(c) {
			consider(2800)
		}
		if phase == PhaseExecute && h.isFinisher(c) {
			consider(2600)
		}
		if underPressure && h.categorizeWithFreya(c) == CatRemoval {
			consider(2400)
		}
		if graveyardFull && scalesWithGraveyard(c) {
			consider(2300)
		}
		if h.isValueEngineKey(c) {
			consider(2000)
		}
		if h.isStarCard(c) {
			consider(1800)
		}
		if h.isFinisher(c) {
			consider(1500)
		}
		if typeLineContains(c, "land") {
			switch {
			case manaScrewed:
				consider(2200)
			case flooded:
				// Penalize: another land is the worst pick when flooded.
				if best > cmc-500 {
					best = cmc - 500
				}
			}
		}
		if h.isCuttable(c) && best <= cmc {
			// Only demote a cuttable card that earned no positive signal.
			best = cmc - 50
		}

		ranked = append(ranked, scored{card: c, score: best, ord: i})
	}

	if len(ranked) <= count {
		out := make([]*gameengine.Card, 0, len(ranked))
		for _, s := range ranked {
			out = append(out, s.card)
		}
		return out
	}

	// Highest score first; stable on original order for ties so behavior is
	// deterministic across runs.
	sort.SliceStable(ranked, func(a, b int) bool {
		if ranked[a].score != ranked[b].score {
			return ranked[a].score > ranked[b].score
		}
		return ranked[a].ord < ranked[b].ord
	})

	out := make([]*gameengine.Card, 0, count)
	for i := 0; i < count && i < len(ranked); i++ {
		out = append(out, ranked[i].card)
	}
	return out
}

// tutorUnderPressure reports whether the searcher is behind or facing
// near-lethal pressure, in which case answers (removal / board wipes) are
// worth tutoring over raw threats.
func (h *YggdrasilHat) tutorUnderPressure(gs *gameengine.GameState, seatIdx int) bool {
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	if seat.Life <= 10 {
		return true
	}
	return h.relativePosition(gs, seatIdx) < 0
}

// isHighDensitySynergyMember reports whether the card belongs to a
// high-density synergy cluster — Freya's marker for a deliberate subsystem of
// the deck's gameplan (≥5 members). Unlike synergyClusterCohesionBoost this
// does NOT require other cluster members to already be on the battlefield: a
// tutor is precisely how a deck assembles its engine, so an unestablished
// engine piece is still a high-value target. Case-insensitive to mirror the
// cohesion-boost membership check.
func (h *YggdrasilHat) isHighDensitySynergyMember(c *gameengine.Card) bool {
	if h == nil || h.Strategy == nil || c == nil || len(h.Strategy.SynergyClusters) == 0 {
		return false
	}
	name := strings.ToLower(c.DisplayName())
	for _, cluster := range h.Strategy.SynergyClusters {
		if !cluster.HighDensity {
			continue
		}
		for _, m := range cluster.Members {
			if strings.ToLower(m) == name {
				return true
			}
		}
	}
	return false
}

// scalesWithGraveyard reports whether a card's value rises with the number of
// cards in the graveyard — delve / escape (cheaper with a full yard),
// flashback / disturb (castable from the yard at all), or an explicit
// "for each card in your graveyard" / "cards in your graveyard" body
// (X-cost / power scaling). Conservative substring scan: a false positive
// only mildly over-ranks a card when the yard is already full, which is a
// reasonable state to want graveyard-synergy cards anyway.
func scalesWithGraveyard(c *gameengine.Card) bool {
	ot := gameengine.OracleTextLower(c)
	if ot == "" {
		return false
	}
	switch {
	case strings.Contains(ot, "delve"),
		strings.Contains(ot, "escape"),
		strings.Contains(ot, "flashback"),
		strings.Contains(ot, "cards in your graveyard"),
		strings.Contains(ot, "card in your graveyard"),
		strings.Contains(ot, "creature cards in your graveyard"):
		return true
	}
	return false
}

// countLandsInPlay counts the lands a seat controls on the battlefield.
func countLandsInPlay(seat *gameengine.Seat) int {
	if seat == nil {
		return 0
	}
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() {
			n++
		}
	}
	return n
}
