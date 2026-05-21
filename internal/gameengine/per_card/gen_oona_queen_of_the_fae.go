package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOonaQueenOfTheFae wires Oona, Queen of the Fae.
//
// Oracle text (Scryfall, verified):
//
//	Flying
//	{X}{U/B}: Choose a color. Target opponent exiles the top X cards
//	of their library. For each card of the chosen color exiled this
//	way, create a 1/1 blue and black Faerie Rogue creature token with
//	flying.
//
// Implementation (R49 stub port):
//   - Flying: AST keyword pipeline.
//   - Mana cost {X}{U/B} = X+1 generic-equivalent. The activation
//     dispatcher pays mana before reaching here; ctx["x"] carries the
//     chosen X (mirrors Walking Ballista / Hydra patterns).
//     Defensive: if no x in ctx, fall back to seat's ManaPool-1 (best
//     effort) so test fixtures and replay paths don't no-op silently.
//   - Color choice: pick the most-represented color among the target
//     opponent's library — maximises expected token yield, matches the
//     deterministic "best-EV" heuristic used by other choose-a-color
//     activations (Crackdown Construct family, Hostage Taker).
//   - Target opponent: lowest-life opponent (kill-priority bias).
//   - Mill: exile top X cards from chosen opponent's library to their
//     own exile (CR §701.27 — exiled "this way" lives in the milled
//     player's exile zone). MoveCard so §614 replacements (Rest in
//     Peace, Leyline of the Void) and zone-cons accounting trigger.
//   - Token spawn: 1/1 blue+black Faerie Rogue with flying; one per
//     exiled card of the chosen color.
func registerOonaQueenOfTheFae(r *Registry) {
	r.OnActivated("Oona, Queen of the Fae", oonaQueenOfTheFaeActivate)
}

func oonaQueenOfTheFaeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "oona_queen_of_the_fae_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	x := 0
	if ctx != nil {
		if v, ok := ctx["x"].(int); ok {
			x = v
		}
	}
	if x <= 0 {
		// Best-effort fallback: dump all available mana minus the {U/B}
		// hybrid pip into X.
		if seat := gs.Seats[seatIdx]; seat != nil && seat.ManaPool > 1 {
			x = seat.ManaPool - 1
		}
	}
	if x <= 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "x_zero_or_no_mana", nil)
		return
	}

	// Target: lowest-life opponent with a non-empty library.
	target := -1
	lowest := 1 << 30
	for i, s := range gs.Seats {
		if i == seatIdx || s == nil || s.Lost || s.LeftGame || len(s.Library) == 0 {
			continue
		}
		if s.Life < lowest {
			lowest = s.Life
			target = i
		}
	}
	if target < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_opponent_with_library", nil)
		return
	}

	// Cap X to library size — we can't exile more than exists.
	if x > len(gs.Seats[target].Library) {
		x = len(gs.Seats[target].Library)
	}

	// Choose color: scan first X cards, pick most common single color.
	colorCounts := map[string]int{}
	for i := 0; i < x; i++ {
		c := gs.Seats[target].Library[i]
		if c == nil {
			continue
		}
		for _, col := range c.Colors {
			colorCounts[col]++
		}
	}
	chosen := ""
	bestN := 0
	for col, n := range colorCounts {
		if n > bestN {
			bestN = n
			chosen = col
		}
	}
	if chosen == "" {
		// All-colorless region — pick "U" as a stable default so the
		// emit log carries the chosen value (no tokens will spawn).
		chosen = "U"
	}

	// Exile top X.
	exiled := []*gameengine.Card{}
	matches := 0
	for i := 0; i < x; i++ {
		if len(gs.Seats[target].Library) == 0 {
			break
		}
		c := gs.Seats[target].Library[0]
		gameengine.MoveCard(gs, c, target, "library", "exile", "oona_exile")
		exiled = append(exiled, c)
		for _, col := range c.Colors {
			if col == chosen {
				matches++
				break
			}
		}
	}

	// Spawn one Faerie Rogue per chosen-color match.
	for i := 0; i < matches; i++ {
		tok := gameengine.CreateCreatureToken(gs, seatIdx, "Faerie Rogue Token",
			[]string{"creature", "faerie", "rogue", "kw:flying"}, 1, 1)
		if tok != nil && tok.Card != nil {
			tok.Card.Colors = []string{"U", "B"}
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seatIdx,
		"target_seat":  target,
		"x":            x,
		"color":        chosen,
		"exiled_count": len(exiled),
		"tokens":       matches,
	})
}
