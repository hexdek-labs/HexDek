package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStarCharter wires Star Charter (Muninn parser-gap #55,
// 13,647 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{3}{W}
//	Creature — Bat Cleric
//	Flying
//	At the beginning of your end step, if you gained or lost life this
//	turn, look at the top four cards of your library. You may reveal a
//	creature card with power 3 or less from among them and put it into
//	your hand. Put the rest on the bottom of your library in a random
//	order.
//
// Implementation:
//   - Flying is AST-engine-side.
//   - End-step trigger gated on active_seat == controller AND
//     (seat.Turn.LifeGained > 0 OR seat.Turn.LifeLost > 0).
//   - Look at top 4 (engine: just read the slice prefix). Pick the
//     creature with the lowest power among those with power ≤ 3
//     (lower-power-first preserves higher-CMC ramp-pickup candidates
//     in the deck for later — but the hat's "may" accepts any qualifying
//     card since the alternative is bottoming it blind).
//   - Move pick to hand; bottom the remaining four-minus-pick cards in
//     random order. Random shuffle via the engine's deterministic Rng.
func registerStarCharter(r *Registry) {
	r.OnTrigger("Star Charter", "end_step", starCharterEndStep)
}

func starCharterEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "star_charter_end_step_look4"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Turn.LifeGained <= 0 && seat.Turn.LifeLost <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
		})
		return
	}
	if len(seat.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": true,
			"empty":     true,
		})
		return
	}
	n := 4
	if n > len(seat.Library) {
		n = len(seat.Library)
	}
	top := append([]*gameengine.Card(nil), seat.Library[:n]...)
	bestIdx := -1
	bestPower := 99
	for i, c := range top {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		p := int(c.BasePower)
		if p > 3 {
			continue
		}
		if p < bestPower {
			bestPower = p
			bestIdx = i
		}
	}
	// Wave 2 multi-step migration: keep the picked card in library until
	// MoveCard does its work (source-zone removal + §614 replacements +
	// commander redirect + zone-change triggers all rely on the card
	// still being in library at call time). Pre-r60 shape spliced top n
	// off first, which silently no-op'd MoveCard's source removal and
	// bypassed §614 / zone_change triggers for the picked card.
	var picked *gameengine.Card
	if bestIdx >= 0 {
		picked = top[bestIdx]
		gameengine.MoveCard(gs, picked, perm.Controller, "library", "hand", slug)
	}
	// The non-picked top cards stay in library; we just reorder them to
	// the bottom in random order (within-zone reorder, no zone change).
	// After MoveCard above, library starts with the original top-N minus
	// any picked card, preserved in order. Pull each off the top and
	// drop them into a `rest` slice to shuffle, then re-append to bottom.
	rest := make([]*gameengine.Card, 0, n-1)
	for _, c := range top {
		if c == nil || c == picked {
			continue
		}
		// Defensive: the card should be at library[0]; if not (some
		// upstream trigger snatched it), skip the rotation.
		if len(seat.Library) == 0 || seat.Library[0] != c {
			continue
		}
		seat.Library = seat.Library[1:]
		rest = append(rest, c)
	}
	// "in a random order" — shuffle the bottom batch via the engine's
	// per-game RNG (the seeded deterministic source). Fall back to a
	// turn-derived stream when the game omitted Rng (test fixtures).
	if len(rest) > 1 {
		rng := gs.Rng
		if rng == nil {
			rng = rand.New(rand.NewSource(int64(gs.Turn)*31 + int64(perm.Controller)))
		}
		rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	}
	seat.Library = append(seat.Library, rest...)

	details := map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"looked_at": n,
	}
	if picked != nil {
		details["picked"] = picked.DisplayName()
		details["picked_power"] = bestPower
	} else {
		details["picked"] = "none_qualified"
	}
	emit(gs, slug, perm.Card.DisplayName(), details)
}
